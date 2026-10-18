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

// MiniMax 视频 V2（platform.minimax.io/docs/api-reference/video-generation-v2-create、-v2-query；
// 国内站 platform.minimaxi.com 同名文档，servers 为 https://api.minimax.cn），认证 Bearer API Key：
//
//	POST {base}/v2/video_generation
//	     {"model":"MiniMax-H3|MiniMax-H3-Max","content":[{"type":"text","text"},{"type":"image_url","image_url":{"url"},
//	      "role":"first_frame|last_frame|reference_image"}],"resolution":"480P|768P|2K","duration":int,"ratio","extra":{…},"callback_url"}
//	     → {"task_id"}；出错时 HTTP 状态即错误码，体为 {"type":"error","error":{"type","message"},"request_id"}
//	GET  {base}/v2/query/video_generation/{task_id}
//	     → {"task":{"id","status":"queued|running|succeeded|failed|cancelled","content":{"url"},"resolution":"2K","duration":5,
//	        "error":{"code","message"}}}；content.url 限时，重新查询拿新地址；只能查 7 天内的任务
//
// 型号：MiniMax-H3 768P / 2K、4–15 秒；MiniMax-H3-Max 480P / 768P（默认 768P）、5–15 秒。resolution 与 duration 必填：
// 没指定时 H3 补 768P（文档没写 H3 的默认，取最低档）、两者都补 5 秒（文档示例的值）——这两个默认是拍的。
// prompt（text）必填；文生视频 ratio 必填且不能是 adaptive，没指定时补 16:9；图生视频（首 / 尾帧）的 ratio 上游恒按
// adaptive，不发；参考图生视频可选发。首尾帧与参考图互斥，参考图最多 9 张。seed / generate_audio / negative_prompt
// 没有对应字段，不发；H3-Max 的 extra.prompt_expansion_mode 经 Extra 透传（{"extra":{…}}）。
const (
	miniMaxVideoCreateEndpoint = "/v2/video_generation"
	miniMaxVideoQueryEndpoint  = "/v2/query/video_generation/"
)

var miniMaxAspectRatios = []string{"adaptive", "21:9", "16:9", "4:3", "1:1", "3:4", "9:16"}

const miniMaxMaxReferenceImages = 9

// miniMaxSpecFor 按模型名（上游模型名，其次目录标识）认 MiniMax 型号；认不出返回 false，不做本地校验。
func miniMaxSpecFor(models ...string) (officialVideoModelSpec, bool) {
	for _, model := range models {
		switch officialVideoModelKey(model) {
		case "minimax-h3":
			return officialVideoModelSpec{resolutions: []string{VideoResolution768P, VideoResolution2K}, defaultResolution: VideoResolution768P,
				minDuration: 4, maxDuration: 15, defaultDuration: 5}, true
		case "minimax-h3-max":
			return officialVideoModelSpec{resolutions: []string{VideoResolution480P, VideoResolution768P}, defaultResolution: VideoResolution768P,
				minDuration: 5, maxDuration: 15, defaultDuration: 5}, true
		}
	}
	return officialVideoModelSpec{}, false
}

type miniMaxVideoUpstream struct{ s *OpenAIGatewayService }

func (u miniMaxVideoUpstream) Kind() VideoUpstreamKind  { return VideoUpstreamMiniMaxOfficial }
func (u miniMaxVideoUpstream) UpstreamEndpoint() string { return miniMaxVideoCreateEndpoint }

func (u miniMaxVideoUpstream) PrepareRequest(req VideoTaskRequest, upstreamModel string) (VideoTaskRequest, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return req, newVideoTaskInvalidRequest("prompt is required by MiniMax video generation")
	}
	if err := validateVideoAspectRatio(req.AspectRatio, miniMaxAspectRatios, "MiniMax"); err != nil {
		return req, err
	}
	counts := countVideoImages(req)
	if err := validateFrameImages(counts); err != nil {
		return req, err
	}
	if counts.reference > miniMaxMaxReferenceImages {
		return req, newVideoTaskInvalidRequest("at most %d reference images are supported by MiniMax", miniMaxMaxReferenceImages)
	}
	if len(req.Images) == 0 {
		switch req.AspectRatio {
		case "":
			req.AspectRatio = "16:9"
		case "adaptive":
			return req, newVideoTaskInvalidRequest("text-to-video on MiniMax needs a concrete aspect ratio, not adaptive")
		}
	}
	spec, ok := miniMaxSpecFor(upstreamModel, req.Model)
	if !ok {
		return req, nil
	}
	return spec.apply(req, upstreamModel)
}

// buildMiniMaxVideoCreateBody 规范请求 → MiniMax 请求体（已过 PrepareRequest）。分辨率按 MiniMax 写法大写（768P / 2K）。
func buildMiniMaxVideoCreateBody(req VideoTaskRequest, upstreamModel string) map[string]any {
	content := []map[string]any{{"type": "text", "text": req.Prompt}}
	frames := false
	for _, image := range req.Images {
		role := string(image.Role)
		switch image.Role {
		case VideoInputImageReference:
			role = "reference_image"
		default:
			frames = true
		}
		content = append(content, map[string]any{"type": "image_url", "image_url": map[string]any{"url": image.URL}, "role": role})
	}
	body := map[string]any{"model": upstreamModel, "content": content}
	if req.Resolution != "" {
		body["resolution"] = strings.ToUpper(req.Resolution)
	}
	if req.DurationSeconds > 0 {
		body["duration"] = req.DurationSeconds
	}
	if req.AspectRatio != "" && !frames {
		body["ratio"] = req.AspectRatio
	}
	mergeVideoExtraDeep(body, req.Extra)
	return body
}

// parseMiniMaxVideoStatus MiniMax 查询体 → 规范结果。
func parseMiniMaxVideoStatus(body []byte) (VideoTaskResult, error) {
	if !gjson.ValidBytes(body) || !gjson.GetBytes(body, "task").IsObject() {
		return VideoTaskResult{}, fmt.Errorf("minimax video status has no task object")
	}
	status := strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "task.status").String()))
	result := VideoTaskResult{
		State:      parseVideoTaskState(status),
		VideoURL:   strings.TrimSpace(gjson.GetBytes(body, "task.content.url").String()),
		Resolution: NormalizeVideoResolution(gjson.GetBytes(body, "task.resolution").String()),
	}
	if duration := gjson.GetBytes(body, "task.duration"); duration.Type == gjson.Number && duration.Float() > 0 {
		result.DurationSeconds = duration.Float()
	}
	if result.State == VideoTaskSucceeded && result.VideoURL == "" {
		result.State = VideoTaskRunning
	}
	if result.State == VideoTaskFailed || result.State == VideoTaskCanceled {
		result.ErrorCode, result.ErrorMessage = videoUpstreamErrorFields(body, "task")
		if result.ErrorCode == "" {
			result.ErrorCode = status
		}
	}
	return result, nil
}

func (u miniMaxVideoUpstream) Create(ctx context.Context, _ *gin.Context, account *Account, req VideoTaskRequest, upstreamModel string) (string, error) {
	upstreamCtx, release := detachUpstreamContext(ctx)
	defer release()
	body, err := u.s.doOfficialVideoJSON(upstreamCtx, account, http.MethodPost, miniMaxVideoCreateEndpoint, nil, buildMiniMaxVideoCreateBody(req, upstreamModel))
	if err != nil {
		return "", err
	}
	return checkVideoUpstreamTaskID(gjson.GetBytes(body, "task_id").String(), body)
}

func (u miniMaxVideoUpstream) Query(ctx context.Context, _ *gin.Context, account *Account, upstreamTaskID string) (VideoTaskResult, error) {
	body, err := u.s.doOfficialVideoJSON(ctx, account, http.MethodGet, miniMaxVideoQueryEndpoint+url.PathEscape(upstreamTaskID), nil, nil)
	if err != nil {
		return VideoTaskResult{}, err
	}
	return parseMiniMaxVideoStatus(body)
}

func (u miniMaxVideoUpstream) OpenContent(ctx context.Context, c *gin.Context, account *Account, _ string, result VideoTaskResult) (*http.Response, error) {
	return u.s.openPublicVideoContent(ctx, c, account, result.VideoURL)
}
