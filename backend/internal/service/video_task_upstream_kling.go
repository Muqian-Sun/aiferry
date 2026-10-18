package service

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// 可灵新版 API（kling.ai/document-api：api/video/{3-0-turbo,3-0-omni,2-6,2-5-turbo}/{text,image}-to-video、
// get-started/authentication、get-started/error-codes；国内站 klingai.com/document-api 同一套），认证 Bearer API Key：
//
//	POST {base}/text-to-video/{model}   {"prompt","settings":{"resolution","aspect_ratio","duration","audio"},"options":{…}}
//	POST {base}/image-to-video/{model}  {"contents":[{"type":"prompt","text"},{"type":"first_frame","url"},{"type":"last_frame","url"}],
//	                                     "settings":{"resolution","duration","audio"},"options":{…}}
//	     → {"code":0,"message","request_id","data":{"id","status","external_id"}}
//	GET  {base}/tasks?task_ids={id}
//	     → {"code":0,"data":[{"id","status":"submitted|processing|succeeded|failed","message",
//	        "outputs":[{"type":"video","url","watermark_url","duration":"5"}]}]}
//
// 型号（各文档 settings 表）：3.0 Turbo 720p / 1080p、3–15 秒、图生视频只认首帧、没有 audio；3.0 720p / 1080p / 4k、3–15 秒、
// 首帧 + 可选尾帧、audio native / off；2.6 720p / 1080p、只有 5 / 10 秒、首帧 + 可选尾帧、audio；2.5 Turbo 同 2.6 但没有 audio。
// 默认 720p、5 秒。宽高比只有文生视频有（16:9 / 9:16 / 1:1），图生视频按图片定、不发。参考图（Element 另有接口）、seed、
// negative_prompt 都没有对应字段：参考图拒绝，seed / negative_prompt 不发（prompt 里可写反向描述）。成片 30 天后清理。
const (
	klingTextToVideoEndpoint  = "/text-to-video/"
	klingImageToVideoEndpoint = "/image-to-video/"
	klingTasksEndpoint        = "/tasks"
)

var klingAspectRatios = []string{"16:9", "9:16", "1:1"}

type klingModelSpec struct {
	officialVideoModelSpec
	lastFrame bool
	audio     bool
}

// klingSpecFor 按模型名（上游模型名，其次目录标识）认可灵型号；认不出返回 false，不做本地校验。
func klingSpecFor(models ...string) (klingModelSpec, bool) {
	hd := []string{VideoResolution720P, VideoResolution1080P}
	spec := func(resolutions []string, durations []int, lastFrame, audio bool) klingModelSpec {
		return klingModelSpec{officialVideoModelSpec: officialVideoModelSpec{resolutions: resolutions, defaultResolution: VideoResolution720P,
			durations: durations, minDuration: 3, maxDuration: 15, defaultDuration: 5}, lastFrame: lastFrame, audio: audio}
	}
	for _, model := range models {
		switch officialVideoModelKey(model) {
		case "kling-3-0-turbo":
			return spec(hd, nil, false, false), true
		case "kling-3-0":
			return spec([]string{VideoResolution720P, VideoResolution1080P, VideoResolution4K}, nil, true, true), true
		case "kling-2-6":
			return spec(hd, []int{5, 10}, true, true), true
		case "kling-2-5-turbo":
			return spec(hd, []int{5, 10}, true, false), true
		}
	}
	return klingModelSpec{}, false
}

type klingVideoUpstream struct{ s *OpenAIGatewayService }

func (u klingVideoUpstream) Kind() VideoUpstreamKind  { return VideoUpstreamKlingOfficial }
func (u klingVideoUpstream) UpstreamEndpoint() string { return "/{text|image}-to-video" }

func (u klingVideoUpstream) PrepareRequest(req VideoTaskRequest, upstreamModel string) (VideoTaskRequest, error) {
	counts := countVideoImages(req)
	if counts.reference > 0 {
		return req, newVideoTaskInvalidRequest("Kling image-to-video takes first_frame / last_frame images only; reference images are not supported")
	}
	if err := validateFrameImages(counts); err != nil {
		return req, err
	}
	if counts.last > 0 && counts.first == 0 {
		return req, newVideoTaskInvalidRequest("a last_frame image needs a first_frame image")
	}
	if counts.first == 0 {
		if err := validateVideoAspectRatio(req.AspectRatio, klingAspectRatios, "Kling"); err != nil {
			return req, err
		}
	}
	spec, ok := klingSpecFor(upstreamModel, req.Model)
	if !ok {
		return req, nil
	}
	if counts.last > 0 && !spec.lastFrame {
		return req, newVideoTaskInvalidRequest("last_frame images are not supported by %s", upstreamModel)
	}
	return spec.apply(req, upstreamModel)
}

// buildKlingVideoCreate 规范请求 → 可灵端点与请求体（已过 PrepareRequest）：有首帧走图生视频，否则文生视频。
func buildKlingVideoCreate(req VideoTaskRequest, upstreamModel string) (string, map[string]any) {
	settings := map[string]any{}
	if req.Resolution != "" {
		settings["resolution"] = req.Resolution
	}
	if req.DurationSeconds > 0 {
		settings["duration"] = req.DurationSeconds
	}
	spec, known := klingSpecFor(upstreamModel, req.Model)
	if req.GenerateAudio != nil && (!known || spec.audio) {
		settings["audio"] = "off"
		if *req.GenerateAudio {
			settings["audio"] = "native"
		}
	}
	body := map[string]any{
		"settings": settings,
		"options":  map[string]any{"watermark_info": map[string]any{"enabled": false}},
	}
	endpoint := klingTextToVideoEndpoint
	if len(req.ImagesWithRole(VideoInputImageFirstFrame)) > 0 {
		endpoint = klingImageToVideoEndpoint
		contents := make([]map[string]any, 0, 3)
		if req.Prompt != "" {
			contents = append(contents, map[string]any{"type": "prompt", "text": req.Prompt})
		}
		for _, role := range []VideoInputImageRole{VideoInputImageFirstFrame, VideoInputImageLastFrame} {
			for _, image := range req.ImagesWithRole(role) {
				contents = append(contents, map[string]any{"type": string(role), "url": image.URL})
			}
		}
		body["contents"] = contents
	} else {
		body["prompt"] = req.Prompt
		if req.AspectRatio != "" {
			settings["aspect_ratio"] = req.AspectRatio
		}
	}
	mergeVideoExtraDeep(body, req.Extra)
	return endpoint + url.PathEscape(upstreamModel), body
}

// parseKlingVideoStatus 可灵任务查询体 → 规范结果。data 里没有这个任务时报错（不当作任务不存在：刚创建的任务
// 可能还查不到，按临时错误下次再查）。
func parseKlingVideoStatus(body []byte, upstreamTaskID string) (VideoTaskResult, error) {
	if !gjson.ValidBytes(body) {
		return VideoTaskResult{}, fmt.Errorf("kling video status is not valid json")
	}
	var task gjson.Result
	gjson.GetBytes(body, "data").ForEach(func(_, item gjson.Result) bool {
		if item.Get("id").String() == upstreamTaskID {
			task = item
			return false
		}
		return true
	})
	if !task.Exists() {
		return VideoTaskResult{}, fmt.Errorf("kling task %s is not in the query result", upstreamTaskID)
	}
	status := strings.ToLower(strings.TrimSpace(task.Get("status").String()))
	result := VideoTaskResult{State: parseVideoTaskState(status)}
	task.Get("outputs").ForEach(func(_, output gjson.Result) bool {
		if output.Get("type").String() != "video" {
			return true
		}
		result.VideoURL = strings.TrimSpace(output.Get("url").String())
		if seconds, err := strconv.ParseFloat(strings.TrimSpace(output.Get("duration").String()), 64); err == nil && seconds > 0 {
			result.DurationSeconds = seconds
		}
		return false
	})
	if result.State == VideoTaskSucceeded && result.VideoURL == "" {
		result.State = VideoTaskRunning
	}
	if result.State == VideoTaskFailed {
		result.ErrorCode = status
		result.ErrorMessage = sanitizeUpstreamErrorMessage(strings.TrimSpace(task.Get("message").String()))
	}
	return result, nil
}

func (u klingVideoUpstream) Create(ctx context.Context, _ *gin.Context, account *Account, req VideoTaskRequest, upstreamModel string) (string, error) {
	endpoint, payload := buildKlingVideoCreate(req, upstreamModel)
	upstreamCtx, release := detachUpstreamContext(ctx)
	defer release()
	body, err := u.s.doOfficialVideoJSON(upstreamCtx, account, http.MethodPost, endpoint, nil, payload)
	if err != nil {
		return "", err
	}
	return checkVideoUpstreamTaskID(gjson.GetBytes(body, "data.id").String(), body)
}

func (u klingVideoUpstream) Query(ctx context.Context, _ *gin.Context, account *Account, upstreamTaskID string) (VideoTaskResult, error) {
	body, err := u.s.doOfficialVideoJSON(ctx, account, http.MethodGet, klingTasksEndpoint, url.Values{"task_ids": {upstreamTaskID}}, nil)
	if err != nil {
		return VideoTaskResult{}, err
	}
	return parseKlingVideoStatus(body, upstreamTaskID)
}

func (u klingVideoUpstream) OpenContent(ctx context.Context, c *gin.Context, account *Account, _ string, result VideoTaskResult) (*http.Response, error) {
	return u.s.openPublicVideoContent(ctx, c, account, result.VideoURL)
}
