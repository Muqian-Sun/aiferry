package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// 视频生成的规范表示：客户端各家形态（OpenAI Videos / xAI / new-api …）先解析成 VideoTaskRequest，
// 上游各家（Grok 成品号 / xAI 中转 / new-api 中转 …）由适配器把它转成自己的请求体；
// 上游的任务状态同样先转成 VideoTaskResult，再按客户端形态渲染。

// VideoTaskIDPrefix 网关对外任务 ID 的前缀：查询时据此区分新任务与旧的 Grok request_id。
const VideoTaskIDPrefix = "vtask_"

// VideoTaskRetention 任务保留期，过期由服务每小时清理。
const VideoTaskRetention = 7 * 24 * time.Hour

// VideoInputImageRole 输入图在生成里的角色。
type VideoInputImageRole string

const (
	VideoInputImageFirstFrame VideoInputImageRole = "first_frame"
	VideoInputImageLastFrame  VideoInputImageRole = "last_frame"
	VideoInputImageReference  VideoInputImageRole = "reference"
)

// VideoInputImage 输入图：URL 是 http(s) 地址或 data URL。
type VideoInputImage struct {
	URL  string              `json:"url"`
	Role VideoInputImageRole `json:"role"`
}

// VideoTaskRequest 规范化的视频生成请求。Resolution 已归一（NormalizeVideoResolution）；
// Extra 是厂商私有参数，由适配器原样并进上游请求体（不覆盖规范字段）。
type VideoTaskRequest struct {
	Model           string            `json:"model"`
	Prompt          string            `json:"prompt,omitempty"`
	NegativePrompt  string            `json:"negative_prompt,omitempty"`
	Images          []VideoInputImage `json:"images,omitempty"`
	DurationSeconds int               `json:"duration_seconds,omitempty"`
	Resolution      string            `json:"resolution,omitempty"`
	AspectRatio     string            `json:"aspect_ratio,omitempty"`
	Seed            *int64            `json:"seed,omitempty"`
	GenerateAudio   *bool             `json:"generate_audio,omitempty"`
	Extra           map[string]any    `json:"extra,omitempty"`
}

// ImagesWithRole 返回指定角色的输入图。
func (r VideoTaskRequest) ImagesWithRole(role VideoInputImageRole) []VideoInputImage {
	var out []VideoInputImage
	for _, image := range r.Images {
		if image.Role == role {
			out = append(out, image)
		}
	}
	return out
}

// ModerationBody 内容审核的输入（与 Grok 媒体同一形态：prompt + images[].image_url）。
func (r VideoTaskRequest) ModerationBody() []byte {
	payload := map[string]any{}
	prompt := strings.TrimSpace(r.Prompt + "\n" + r.NegativePrompt)
	if prompt != "" {
		payload["prompt"] = prompt
	}
	if len(r.Images) > 0 {
		images := make([]map[string]string, 0, len(r.Images))
		for _, image := range r.Images {
			images = append(images, map[string]string{"image_url": image.URL})
		}
		payload["images"] = images
	}
	if len(payload) == 0 {
		return nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return body
}

// VideoTaskState 任务状态。
type VideoTaskState string

const (
	VideoTaskQueued    VideoTaskState = "queued"
	VideoTaskRunning   VideoTaskState = "running"
	VideoTaskSucceeded VideoTaskState = "succeeded"
	VideoTaskFailed    VideoTaskState = "failed"
	VideoTaskCanceled  VideoTaskState = "canceled"
)

// IsTerminal 终态不再向上游查询状态。
func (s VideoTaskState) IsTerminal() bool {
	return s == VideoTaskSucceeded || s == VideoTaskFailed || s == VideoTaskCanceled
}

// VideoTaskResult 一次上游状态查询的规范化结果。DurationSeconds 为 0 表示上游没报。
type VideoTaskResult struct {
	State           VideoTaskState
	Progress        *int
	VideoURL        string
	CoverURL        string
	DurationSeconds float64
	Resolution      string
	ErrorCode       string
	ErrorMessage    string
}

// VideoInboundShape 客户端调用的 API 形态。
type VideoInboundShape string

const (
	VideoShapeOpenAI VideoInboundShape = "openai"
	VideoShapeXAI    VideoInboundShape = "xai"
	VideoShapeNewAPI VideoInboundShape = "newapi"
)

// InboundEndpoint 创建该任务的入站端点（用量记录在查询时落，入站端点仍记创建时的）。
func (s VideoInboundShape) InboundEndpoint() string {
	switch s {
	case VideoShapeXAI:
		return "/v1/videos/generations"
	case VideoShapeNewAPI:
		return "/v1/video/generations"
	default:
		return "/v1/videos"
	}
}

// VideoUpstreamKind 承接任务的上游适配器。
type VideoUpstreamKind string

const (
	VideoUpstreamGrokOAuth   VideoUpstreamKind = "grok_oauth"
	VideoUpstreamXAIRelay    VideoUpstreamKind = "xai_relay"
	VideoUpstreamNewAPIRelay VideoUpstreamKind = "newapi_relay"
	// 官方视频厂商：key 的 video 地址指向厂商官方域名。
	VideoUpstreamSeedanceOfficial VideoUpstreamKind = "seedance_official"
	VideoUpstreamKlingOfficial    VideoUpstreamKind = "kling_official"
	VideoUpstreamMiniMaxOfficial  VideoUpstreamKind = "minimax_official"
)

// VideoTask 一条视频任务记录。
type VideoTask struct {
	ID              string
	UserID          int64
	APIKeyID        int64
	AccountID       int64
	CatalogEntryID  int64
	Model           string
	UpstreamModel   string
	UpstreamKind    VideoUpstreamKind
	UpstreamTaskID  string
	InboundShape    VideoInboundShape
	Request         VideoTaskRequest
	State           VideoTaskState
	Progress        *int
	VideoURL        string
	ErrorCode       string
	ErrorMessage    string
	DurationSeconds float64
	Resolution      string
	BilledAt        *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ApplyResult 把一次查询结果并进任务（上游没报的字段保留原值）。
func (t *VideoTask) ApplyResult(r VideoTaskResult) {
	if r.State != "" {
		t.State = r.State
	}
	if r.Progress != nil {
		progress := *r.Progress
		t.Progress = &progress
	}
	if r.VideoURL != "" {
		t.VideoURL = r.VideoURL
	}
	if r.DurationSeconds > 0 {
		t.DurationSeconds = r.DurationSeconds
	}
	if r.Resolution != "" {
		t.Resolution = r.Resolution
	}
	if r.ErrorCode != "" {
		t.ErrorCode = r.ErrorCode
	}
	if r.ErrorMessage != "" {
		t.ErrorMessage = r.ErrorMessage
	}
}

// BillingDurationSeconds 计费秒数：上游报的实际时长（四舍五入）优先，没报按请求时长，都没有按默认。
func (t *VideoTask) BillingDurationSeconds() int {
	seconds := 0
	if t.DurationSeconds > 0 {
		seconds = int(math.Round(t.DurationSeconds))
	}
	if seconds <= 0 {
		seconds = t.Request.DurationSeconds
	}
	return NormalizeVideoBillingDurationSecondsOrDefault(seconds)
}

// BillingResolution 计费分辨率档：上游报的优先，没报按请求。
func (t *VideoTask) BillingResolution() string {
	resolution := t.Resolution
	if resolution == "" {
		resolution = t.Request.Resolution
	}
	return VideoBillingTierForResolution(resolution)
}

// UsageResult 首次看到成功时入账用的转发结果：一条视频，分辨率档 × 秒数按目录条目计价。
func (t *VideoTask) UsageResult(now time.Time) *OpenAIForwardResult {
	result := &OpenAIForwardResult{
		RequestID:            t.ID,
		ResponseID:           t.ID,
		Model:                t.Model,
		BillingModel:         t.Model,
		UpstreamModel:        t.UpstreamModel,
		VideoCount:           1,
		VideoResolution:      t.BillingResolution(),
		VideoDurationSeconds: t.BillingDurationSeconds(),
	}
	if !t.CreatedAt.IsZero() && now.After(t.CreatedAt) {
		result.Duration = now.Sub(t.CreatedAt)
	}
	return result
}

// ErrVideoTaskNotFound 任务不存在或不属于这把 key。
var ErrVideoTaskNotFound = errors.New("video task not found")

// VideoTaskRepository 视频任务的存储。
type VideoTaskRepository interface {
	CreateVideoTask(ctx context.Context, task *VideoTask) error
	// GetVideoTask 只返回属于 apiKeyID 的任务，否则 ErrVideoTaskNotFound。
	GetVideoTask(ctx context.Context, id string, apiKeyID int64) (*VideoTask, error)
	// UpdateVideoTaskResult 写回最近一次查询结果；已是终态的任务不再被改写。
	UpdateVideoTaskResult(ctx context.Context, task *VideoTask) error
	// ClaimVideoTaskBilling 原子抢占计费：billed_at 为空时置为当前时刻并返回 true。
	ClaimVideoTaskBilling(ctx context.Context, id string) (bool, error)
	// ReleaseVideoTaskBilling 入账失败时清掉 billed_at，留给下一次查询重试。
	ReleaseVideoTaskBilling(ctx context.Context, id string) error
	DeleteVideoTasksBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

// NewVideoTaskID 生成对外任务 ID。
func NewVideoTaskID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate video task id: %w", err)
	}
	return VideoTaskIDPrefix + hex.EncodeToString(buf[:]), nil
}

// IsVideoTaskID 报告 id 是否是本网关发的任务 ID（否则按旧的 Grok request_id 处理）。
func IsVideoTaskID(id string) bool {
	return strings.HasPrefix(strings.TrimSpace(id), VideoTaskIDPrefix)
}

// 规范分辨率。
const (
	VideoResolution480P  = "480p"
	VideoResolution720P  = "720p"
	VideoResolution768P  = "768p"
	VideoResolution1080P = "1080p"
	VideoResolution2K    = "2k"
	VideoResolution4K    = "4k"
)

// videoResolutionShortSide 各档的短边像素，size ↔ 分辨率换算用。
var videoResolutionShortSide = []struct {
	resolution string
	shortSide  int
}{
	{VideoResolution480P, 480},
	{VideoResolution720P, 720},
	{VideoResolution768P, 768},
	{VideoResolution1080P, 1080},
	{VideoResolution2K, 1440},
	{VideoResolution4K, 2160},
}

// NormalizeVideoResolution 把各家写法归一成规范分辨率（小写）；"1280x720" 这类尺寸按短边取档；
// 认不出返回空串（不猜，交给上游默认）。
func NormalizeVideoResolution(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "":
		return ""
	case "480", "480p", "sd":
		return VideoResolution480P
	case "720", "720p", "hd":
		return VideoResolution720P
	case "768", "768p":
		return VideoResolution768P
	case "1080", "1080p", "fhd", "full_hd", "full-hd":
		return VideoResolution1080P
	case "2k", "1440", "1440p", "qhd":
		return VideoResolution2K
	case "4k", "2160", "2160p", "uhd":
		return VideoResolution4K
	}
	if width, height, ok := ParseVideoSize(value); ok {
		return videoResolutionForShortSide(min(width, height))
	}
	return ""
}

// videoResolutionForShortSide 短边落在哪一档（向上取最近的档，超过 4K 记 4K）。
func videoResolutionForShortSide(shortSide int) string {
	for _, tier := range videoResolutionShortSide {
		if shortSide <= tier.shortSide {
			return tier.resolution
		}
	}
	return VideoResolution4K
}

// VideoBillingTierForResolution 规范分辨率 → 计费档：每个规范分辨率都有同名的档，按实际分辨率计，
// 不拿别的档顶（条目没有这一档时创建就拒绝，见 validateVideoEntryTier）；认不出的返回空串，交给计费默认档。
func VideoBillingTierForResolution(resolution string) string {
	tier, ok := LookupVideoBillingResolution(NormalizeVideoResolution(resolution))
	if !ok {
		return ""
	}
	return tier
}

// validateVideoEntryTier 条目分了视频档却没有这个分辨率的档：拒绝创建（不静默按别的档或基础价计）。
// 没分档的条目按单一的每秒价计，不拦；分辨率为空（交给上游默认）也不拦。
func validateVideoEntryTier(entry *ModelCatalogEntry, resolution string) error {
	tier := VideoBillingTierForResolution(resolution)
	if entry == nil || tier == "" {
		return nil
	}
	tiered := false
	for _, interval := range entry.Intervals {
		if interval.TierLabel == "" {
			continue
		}
		tiered = true
		if interval.TierLabel == tier && interval.PerRequestPrice != nil {
			return nil
		}
	}
	if !tiered {
		return nil
	}
	return newVideoTaskInvalidRequest("resolution %s is not available for model %s", tier, entry.ModelID)
}

// ParseVideoSize 解析 "1280x720"（也接受 "1280*720"、大写 X）。
func ParseVideoSize(raw string) (int, int, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	sep := strings.IndexAny(value, "x*")
	if sep <= 0 || sep == len(value)-1 {
		return 0, 0, false
	}
	width, err1 := strconv.Atoi(strings.TrimSpace(value[:sep]))
	height, err2 := strconv.Atoi(strings.TrimSpace(value[sep+1:]))
	if err1 != nil || err2 != nil || width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

// videoStandardAspectRatios 尺寸换宽高比时吸附的标准比例。
var videoStandardAspectRatios = []struct {
	label string
	w, h  int
}{
	{"16:9", 16, 9}, {"9:16", 9, 16}, {"1:1", 1, 1}, {"4:3", 4, 3}, {"3:4", 3, 4},
	{"3:2", 3, 2}, {"2:3", 2, 3}, {"21:9", 21, 9}, {"9:21", 9, 21},
}

// VideoAspectRatioForSize 尺寸 → 最接近的标准宽高比（按 w/h 的绝对差取最小，并列取表里靠前的）。
func VideoAspectRatioForSize(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	ratio := float64(width) / float64(height)
	best, bestDiff := "", math.MaxFloat64
	for _, candidate := range videoStandardAspectRatios {
		diff := math.Abs(ratio - float64(candidate.w)/float64(candidate.h))
		if diff < bestDiff {
			best, bestDiff = candidate.label, diff
		}
	}
	return best
}

// parseVideoAspectRatio 解析 "16:9"。
func parseVideoAspectRatio(raw string) (int, int, bool) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	w, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return 0, 0, false
	}
	return w, h, true
}

// VideoSizeFor 分辨率 + 宽高比 → "宽x高"：短边取该档像素，长边按比例四舍五入到偶数；缺一项返回空串。
func VideoSizeFor(resolution, aspectRatio string) string {
	width, height, ok := VideoDimensionsFor(resolution, aspectRatio)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%dx%d", width, height)
}

// VideoDimensionsFor 同 VideoSizeFor，返回整数宽高。
func VideoDimensionsFor(resolution, aspectRatio string) (int, int, bool) {
	resolution = NormalizeVideoResolution(resolution)
	w, h, ok := parseVideoAspectRatio(aspectRatio)
	if resolution == "" || !ok {
		return 0, 0, false
	}
	shortSide := 0
	for _, tier := range videoResolutionShortSide {
		if tier.resolution == resolution {
			shortSide = tier.shortSide
		}
	}
	longSide := func(short, num, den int) int {
		v := int(math.Round(float64(short) * float64(num) / float64(den)))
		return v + v%2
	}
	if w >= h {
		return longSide(shortSide, w, h), shortSide, true
	}
	return shortSide, longSide(shortSide, h, w), true
}
