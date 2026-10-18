package service

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// new-api 统一视频接口（docs.newapi.ai「Create / Get Video Generation」，源码 QuantumNous/new-api
// router/video-router.go、dto/video.go、relay/relay_task.go、controller/relay.go）：
//
//	POST /v1/video/generations  {"model","prompt","image","duration","width","height","seed","metadata":{…}}
//	     → {"task_id","status"}（文档）；源码兜底回 {"id","task_id","status":"queued","model","created_at"}，
//	       走 openai_video 插件时回 OpenAI video 对象（id / status）
//	GET  /v1/video/generations/{task_id}
//	     → 文档：{"task_id","status":"queued|in_progress|completed|failed","url","metadata":{"duration"…},"error":{…}}
//	     → 源码：{"code":"success","data":{"task_id","status":"SUCCESS|FAILURE|IN_PROGRESS|QUEUED|SUBMITTED|NOT_START",
//	             "result_url","fail_reason","progress":"100%"}}；Gemini / Vertex 渠道 data.status 为 succeeded / processing，data.url
//
// 两种都可能遇到，解析按宽松口径：先看 data 对象，再看顶层；状态、地址、失败原因都接受常见别名。
const newAPIVideoGenerationsEndpoint = "/v1/video/generations"

// buildNewAPIVideoCreateBody 规范请求 → new-api 请求体。首帧 / 参考图都放 image（第一张）与 images（全部），
// 厂商私有参数、宽高比、分辨率、反向提示词等进 metadata（new-api 把 metadata 交给各渠道适配器）。
func buildNewAPIVideoCreateBody(req VideoTaskRequest, upstreamModel string) map[string]any {
	body := map[string]any{"model": upstreamModel}
	if req.Prompt != "" {
		body["prompt"] = req.Prompt
	}
	var urls []string
	var lastFrame string
	for _, image := range req.Images {
		if image.Role == VideoInputImageLastFrame {
			lastFrame = image.URL
			continue
		}
		urls = append(urls, image.URL)
	}
	if len(urls) > 0 {
		body["image"] = urls[0]
	}
	if len(urls) > 1 {
		body["images"] = urls
	}
	if req.DurationSeconds > 0 {
		body["duration"] = req.DurationSeconds
		body["seconds"] = strconv.Itoa(req.DurationSeconds)
	}
	if width, height, ok := VideoDimensionsFor(req.Resolution, req.AspectRatio); ok {
		body["width"], body["height"] = width, height
		body["size"] = fmt.Sprintf("%dx%d", width, height)
	}
	if req.Resolution != "" {
		body["resolution"] = req.Resolution
	}
	if req.Seed != nil {
		body["seed"] = *req.Seed
	}
	metadata := map[string]any{}
	if req.NegativePrompt != "" {
		metadata["negative_prompt"] = req.NegativePrompt
	}
	if req.AspectRatio != "" {
		metadata["aspect_ratio"] = req.AspectRatio
	}
	if req.Resolution != "" {
		metadata["resolution"] = req.Resolution
	}
	if req.GenerateAudio != nil {
		metadata["generate_audio"] = *req.GenerateAudio
	}
	if lastFrame != "" {
		metadata["last_frame_image"] = lastFrame
	}
	mergeVideoExtra(metadata, req.Extra)
	if len(metadata) > 0 {
		body["metadata"] = metadata
	}
	return body
}

// parseNewAPIVideoStatus new-api 状态体 → 规范结果（宽松口径，见文件头）。
func parseNewAPIVideoStatus(body []byte) (VideoTaskResult, error) {
	if !gjson.ValidBytes(body) {
		return VideoTaskResult{}, fmt.Errorf("new-api video status is not valid json")
	}
	root := ""
	if data := gjson.GetBytes(body, "data"); data.IsObject() {
		root = "data"
	}
	path := func(field string) string {
		if root == "" {
			return field
		}
		return root + "." + field
	}
	status := firstGJSONString(body, path("status"), path("state"), path("task_status"))
	result := VideoTaskResult{
		State:    parseVideoTaskState(status),
		Progress: parseVideoProgress(gjson.GetBytes(body, path("progress"))),
		VideoURL: firstGJSONString(body, path("url"), path("result_url"), path("video_url"), path("video.url"),
			path("output.video_url"), path("metadata.url")),
	}
	if result.State == VideoTaskQueued && result.Progress != nil && *result.Progress > 0 {
		result.State = VideoTaskRunning
	}
	for _, field := range []string{"metadata.duration", "duration", "video.duration", "seconds"} {
		value := gjson.GetBytes(body, path(field))
		if value.Type == gjson.Number && value.Float() > 0 {
			result.DurationSeconds = value.Float()
			break
		}
		if value.Type == gjson.String {
			if seconds, err := strconv.ParseFloat(strings.TrimSpace(value.String()), 64); err == nil && seconds > 0 {
				result.DurationSeconds = seconds
				break
			}
		}
	}
	width := gjson.GetBytes(body, path("metadata.width")).Int()
	height := gjson.GetBytes(body, path("metadata.height")).Int()
	if width > 0 && height > 0 {
		result.Resolution = videoResolutionForShortSide(int(min(width, height)))
	}
	if result.State == VideoTaskSucceeded && result.VideoURL == "" {
		result.State = VideoTaskRunning
	}
	if result.State == VideoTaskFailed || result.State == VideoTaskCanceled {
		result.ErrorCode, result.ErrorMessage = videoUpstreamErrorFields(body, root)
		if result.ErrorCode == "" || result.ErrorCode == "0" {
			result.ErrorCode = strings.ToLower(status)
		}
	}
	return result, nil
}

type newAPIRelayVideoUpstream struct{ s *OpenAIGatewayService }

func (u newAPIRelayVideoUpstream) Kind() VideoUpstreamKind  { return VideoUpstreamNewAPIRelay }
func (u newAPIRelayVideoUpstream) UpstreamEndpoint() string { return newAPIVideoGenerationsEndpoint }

func (u newAPIRelayVideoUpstream) Create(ctx context.Context, _ *gin.Context, account *Account, req VideoTaskRequest, upstreamModel string) (string, error) {
	root, err := u.s.keyVideoBaseURL(account)
	if err != nil {
		return "", err
	}
	upstreamCtx, release := detachUpstreamContext(ctx)
	defer release()
	httpReq, err := newVideoUpstreamJSONRequest(WithHTTPUpstreamProfile(upstreamCtx, HTTPUpstreamProfileOpenAI), http.MethodPost,
		joinUpstreamEndpointURL(root, newAPIVideoGenerationsEndpoint), buildNewAPIVideoCreateBody(req, upstreamModel))
	if err != nil {
		return "", err
	}
	if err := applyVideoKeyHeaders(httpReq, account); err != nil {
		return "", err
	}
	body, _, err := u.s.doVideoUpstreamJSON(httpReq, account)
	if err != nil {
		return "", err
	}
	return extractVideoUpstreamTaskID(body)
}

func (u newAPIRelayVideoUpstream) Query(ctx context.Context, _ *gin.Context, account *Account, upstreamTaskID string) (VideoTaskResult, error) {
	root, err := u.s.keyVideoBaseURL(account)
	if err != nil {
		return VideoTaskResult{}, err
	}
	httpReq, err := newVideoUpstreamJSONRequest(WithHTTPUpstreamProfile(ctx, HTTPUpstreamProfileOpenAI), http.MethodGet,
		joinUpstreamEndpointURL(root, newAPIVideoGenerationsEndpoint+"/"+upstreamTaskID), nil)
	if err != nil {
		return VideoTaskResult{}, err
	}
	if err := applyVideoKeyHeaders(httpReq, account); err != nil {
		return VideoTaskResult{}, err
	}
	body, _, err := u.s.doVideoUpstreamJSON(httpReq, account)
	if err != nil {
		return VideoTaskResult{}, err
	}
	return parseNewAPIVideoStatus(body)
}

// OpenContent new-api 的 result_url 可能是它自己的代理下载地址（{server}/v1/videos/{task_id}/content，要带 key），
// 也可能是厂商的公开地址；同主机判定在 openKeyVideoContent 里。
func (u newAPIRelayVideoUpstream) OpenContent(ctx context.Context, c *gin.Context, account *Account, _ string, result VideoTaskResult) (*http.Response, error) {
	return u.s.openKeyVideoContent(ctx, c, account, result.VideoURL)
}
