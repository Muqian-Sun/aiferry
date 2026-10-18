package service

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// Seedance 官方 API（BytePlus ModelArk「Create a video generation task」/「Retrieve a video generation task」，
// docs.byteplus.com/en/docs/ModelArk/1520757、1521309，2026-10-17 版；火山方舟同名文档，根地址不同）：
//
//	POST {base}/contents/generations/tasks
//	     {"model","content":[{"type":"text","text"},{"type":"image_url","image_url":{"url"},"role":"first_frame|last_frame|reference_image"}],
//	      "resolution":"480p|720p|1080p|4k","ratio":"16:9|4:3|1:1|3:4|9:16|21:9|adaptive","duration":int,"generate_audio":bool,"seed":int,…}
//	     → {"id":"cgt-…"}
//	GET  {base}/contents/generations/tasks/{id}
//	     → {"id","model","status":"queued|running|succeeded|failed|cancelled|expired","content":{"video_url","last_frame_url"},
//	        "duration"|"frames","framespersecond","resolution","ratio","error":{"code","message"}}
//
// 模型（文档「resolution」「duration」两节）：2.5 默认 720p，480p / 720p / 1080p，4–30 秒；2.0 默认 720p，480p / 720p / 1080p / 4k，
// 4–15 秒；2.0 fast / mini 只有 480p / 720p，4–15 秒。2.5 的 duration 默认 -1（模型自定时长），这里不传 -1：
// 没指定时补 5 秒（2.0 系列的官方默认），计费时长才确定。seed 只有 1.x 支持，2.x 不发；negative_prompt 没有对应字段，不发。
// 参考图 2.0 系列最多 9 张、2.5 最多 30 张。content.video_url 24 小时有效（重新查询不刷新）。
const seedanceVideoTasksEndpoint = "/contents/generations/tasks"

var seedanceAspectRatios = []string{"16:9", "4:3", "1:1", "3:4", "9:16", "21:9", "adaptive"}

type seedanceModelSpec struct {
	officialVideoModelSpec
	maxReferenceImages int
	seed               bool
}

// seedanceSpecFor 按模型名认 Seedance 型号（上游模型名认不出时再看目录标识）；都认不出返回 false，不做本地校验。
func seedanceSpecFor(models ...string) (seedanceModelSpec, bool) {
	series20 := func(resolutions ...string) seedanceModelSpec {
		return seedanceModelSpec{officialVideoModelSpec: officialVideoModelSpec{resolutions: resolutions, defaultResolution: VideoResolution720P,
			minDuration: 4, maxDuration: 15, defaultDuration: 5}, maxReferenceImages: 9}
	}
	for _, model := range models {
		key := officialVideoModelKey(model)
		switch {
		case strings.Contains(key, "seedance-2-5"):
			return seedanceModelSpec{officialVideoModelSpec: officialVideoModelSpec{
				resolutions:       []string{VideoResolution480P, VideoResolution720P, VideoResolution1080P},
				defaultResolution: VideoResolution720P, minDuration: 4, maxDuration: 30, defaultDuration: 5}, maxReferenceImages: 30}, true
		case strings.Contains(key, "seedance-2-0-fast"), strings.Contains(key, "seedance-2-0-mini"):
			return series20(VideoResolution480P, VideoResolution720P), true
		case strings.Contains(key, "seedance-2-0"):
			return series20(VideoResolution480P, VideoResolution720P, VideoResolution1080P, VideoResolution4K), true
		}
	}
	return seedanceModelSpec{}, false
}

type seedanceVideoUpstream struct{ s *OpenAIGatewayService }

func (u seedanceVideoUpstream) Kind() VideoUpstreamKind  { return VideoUpstreamSeedanceOfficial }
func (u seedanceVideoUpstream) UpstreamEndpoint() string { return seedanceVideoTasksEndpoint }

func (u seedanceVideoUpstream) PrepareRequest(req VideoTaskRequest, upstreamModel string) (VideoTaskRequest, error) {
	if err := validateVideoAspectRatio(req.AspectRatio, seedanceAspectRatios, "Seedance"); err != nil {
		return req, err
	}
	counts := countVideoImages(req)
	if err := validateFrameImages(counts); err != nil {
		return req, err
	}
	if counts.last > 0 && counts.first == 0 {
		return req, newVideoTaskInvalidRequest("a last_frame image needs a first_frame image")
	}
	spec, ok := seedanceSpecFor(upstreamModel, req.Model)
	if !ok {
		return req, nil
	}
	if counts.reference > spec.maxReferenceImages {
		return req, newVideoTaskInvalidRequest("at most %d reference images are supported by %s", spec.maxReferenceImages, upstreamModel)
	}
	return spec.apply(req, upstreamModel)
}

// buildSeedanceVideoCreateBody 规范请求 → Seedance 请求体（已过 PrepareRequest）。
func buildSeedanceVideoCreateBody(req VideoTaskRequest, upstreamModel string) map[string]any {
	content := make([]map[string]any, 0, len(req.Images)+1)
	if req.Prompt != "" {
		content = append(content, map[string]any{"type": "text", "text": req.Prompt})
	}
	for _, image := range req.Images {
		role := string(image.Role)
		if image.Role == VideoInputImageReference {
			role = "reference_image"
		}
		content = append(content, map[string]any{"type": "image_url", "image_url": map[string]any{"url": image.URL}, "role": role})
	}
	body := map[string]any{"model": upstreamModel, "content": content}
	if req.Resolution != "" {
		body["resolution"] = req.Resolution
	}
	if req.AspectRatio != "" {
		body["ratio"] = req.AspectRatio
	}
	if req.DurationSeconds > 0 {
		body["duration"] = req.DurationSeconds
	}
	if req.GenerateAudio != nil {
		body["generate_audio"] = *req.GenerateAudio
	}
	spec, known := seedanceSpecFor(upstreamModel, req.Model)
	if req.Seed != nil && (!known || spec.seed) {
		body["seed"] = *req.Seed
	}
	mergeVideoExtraDeep(body, req.Extra)
	return body
}

// parseSeedanceVideoStatus Seedance 任务体 → 规范结果。传了 frames 的任务不回 duration，按帧数 ÷ 帧率折算。
func parseSeedanceVideoStatus(body []byte) (VideoTaskResult, error) {
	if !gjson.ValidBytes(body) {
		return VideoTaskResult{}, fmt.Errorf("seedance video status is not valid json")
	}
	status := strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "status").String()))
	result := VideoTaskResult{
		State:      parseVideoTaskState(status),
		VideoURL:   strings.TrimSpace(gjson.GetBytes(body, "content.video_url").String()),
		Resolution: NormalizeVideoResolution(gjson.GetBytes(body, "resolution").String()),
	}
	if duration := gjson.GetBytes(body, "duration"); duration.Type == gjson.Number && duration.Float() > 0 {
		result.DurationSeconds = duration.Float()
	} else if frames := gjson.GetBytes(body, "frames").Float(); frames > 0 {
		fps := gjson.GetBytes(body, "framespersecond").Float()
		if fps <= 0 {
			fps = 24
		}
		result.DurationSeconds = frames / fps
	}
	if result.State == VideoTaskSucceeded && result.VideoURL == "" {
		result.State = VideoTaskRunning
	}
	if result.State == VideoTaskFailed || result.State == VideoTaskCanceled {
		result.ErrorCode, result.ErrorMessage = videoUpstreamErrorFields(body, "")
		if result.ErrorCode == "" {
			result.ErrorCode = status
		}
	}
	return result, nil
}

func (u seedanceVideoUpstream) Create(ctx context.Context, _ *gin.Context, account *Account, req VideoTaskRequest, upstreamModel string) (string, error) {
	upstreamCtx, release := detachUpstreamContext(ctx)
	defer release()
	body, err := u.s.doOfficialVideoJSON(upstreamCtx, account, http.MethodPost, seedanceVideoTasksEndpoint, nil, buildSeedanceVideoCreateBody(req, upstreamModel))
	if err != nil {
		return "", err
	}
	return checkVideoUpstreamTaskID(gjson.GetBytes(body, "id").String(), body)
}

func (u seedanceVideoUpstream) Query(ctx context.Context, _ *gin.Context, account *Account, upstreamTaskID string) (VideoTaskResult, error) {
	body, err := u.s.doOfficialVideoJSON(ctx, account, http.MethodGet, seedanceVideoTasksEndpoint+"/"+url.PathEscape(upstreamTaskID), nil, nil)
	if err != nil {
		return VideoTaskResult{}, err
	}
	return parseSeedanceVideoStatus(body)
}

func (u seedanceVideoUpstream) OpenContent(ctx context.Context, c *gin.Context, account *Account, _ string, result VideoTaskResult) (*http.Response, error) {
	return u.s.openPublicVideoContent(ctx, c, account, result.VideoURL)
}
