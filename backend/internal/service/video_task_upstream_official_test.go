//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func videoTestOfficialKey(id int64, platform, base string) *Account {
	return &Account{ID: id, Name: platform, Platform: platform, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Concurrency: 5, Credentials: map[string]any{"api_key": "sk-" + platform},
		ProtocolEndpoints: map[string]string{APIProtocolVideo: base}}
}

// officialVideoSeed 播种表里的官方视频条目（测试直接用真实的价与分档）。
func officialVideoSeed(t *testing.T, modelID string) *ModelCatalogEntry {
	t.Helper()
	for _, entry := range officialVideoSeeds() {
		if entry.ModelID == modelID {
			clone := entry
			return &clone
		}
	}
	t.Fatalf("no official video seed %s", modelID)
	return nil
}

type officialVideoHarness struct {
	upstream *videoFakeUpstream
	mux      *http.ServeMux
	svc      *VideoTaskService
	errors   *videoAuthErrorRepo
}

// videoAuthErrorRepo 记录 401 时的 SetError（账号被标成凭据失效）。
type videoAuthErrorRepo struct {
	AccountRepository
	setErrorIDs []int64
}

func (r *videoAuthErrorRepo) SetError(_ context.Context, id int64, _ string) error {
	r.setErrorIDs = append(r.setErrorIDs, id)
	return nil
}

func (r *videoAuthErrorRepo) SetRateLimited(context.Context, int64, time.Time) error { return nil }

func (r *videoAuthErrorRepo) SetTempUnschedulable(context.Context, int64, time.Time, string) error {
	return nil
}

func (r *videoAuthErrorRepo) SetModelRateLimit(context.Context, int64, string, time.Time, ...string) error {
	return nil
}

func (r *videoAuthErrorRepo) UpdateExtra(context.Context, int64, map[string]any) error { return nil }

func newOfficialVideoHarness(t *testing.T) *officialVideoHarness {
	t.Helper()
	mux := http.NewServeMux()
	upstream := &videoFakeUpstream{handler: mux, transportErr: map[string]error{}}
	repo := &videoAuthErrorRepo{}
	gateway := &OpenAIGatewayService{httpUpstream: upstream, rateLimitService: NewRateLimitService(repo, nil, nil, nil, nil)}
	svc := NewVideoTaskService(nil, gateway)
	t.Cleanup(svc.Stop)
	return &officialVideoHarness{upstream: upstream, mux: mux, svc: svc, errors: repo}
}

func (h *officialVideoHarness) lastCall(t *testing.T) videoFakeCall {
	t.Helper()
	calls := h.upstream.recorded()
	require.NotEmpty(t, calls)
	return calls[len(calls)-1]
}

func requireVideoClientError(t *testing.T, err error, contains string) {
	t.Helper()
	var clientErr *VideoTaskClientError
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.StatusCode)
	require.Contains(t, clientErr.Message, contains)
}

func TestSeedanceVideoUpstreamCreate(t *testing.T) {
	h := newOfficialVideoHarness(t)
	h.mux.HandleFunc("POST ark.cn-beijing.volces.com/api/v3/contents/generations/tasks", func(w http.ResponseWriter, _ *http.Request) {
		writeVideoJSON(w, 200, `{"id":"cgt-20261018-a1"}`)
	})
	account := videoTestOfficialKey(30, PlatformSeedance, DefaultSeedanceCNBaseURL)
	account.CatalogUpstreamModels = map[string]string{"seedance-2.0": "doubao-seedance-2-0-260128", "seedance-2.0-fast": "doubao-seedance-2-0-fast-260128"}
	seed, audio := int64(42), true
	req := VideoTaskRequest{Model: "seedance-2.0", Prompt: "waves", NegativePrompt: "blur", Resolution: VideoResolution1080P, AspectRatio: "21:9",
		DurationSeconds: 12, Seed: &seed, GenerateAudio: &audio,
		Images: []VideoInputImage{{URL: "https://example.com/f.png", Role: VideoInputImageFirstFrame}, {URL: "https://example.com/l.png", Role: VideoInputImageLastFrame}},
		Extra:  map[string]any{"camera_fixed": false, "resolution": "480p", "return_last_frame": true}}

	submission, err := h.svc.Submit(context.Background(), videoTestGinContext(), account, officialVideoSeed(t, "seedance-2.0"), req)
	require.NoError(t, err)
	require.Equal(t, VideoUpstreamSeedanceOfficial, submission.UpstreamKind)
	require.Equal(t, "cgt-20261018-a1", submission.UpstreamTaskID)
	require.Equal(t, "doubao-seedance-2-0-260128", submission.UpstreamModel, "上游模型名取承接上的映射")
	call := h.lastCall(t)
	require.Equal(t, "https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks", call.URL)
	require.Equal(t, "Bearer sk-seedance", call.Auth)
	// seed 只有 1.x 支持、negative_prompt 没有字段：都不发；Extra 不覆盖规范字段（resolution 仍是 1080p）。
	require.JSONEq(t, `{"model":"doubao-seedance-2-0-260128","content":[
		{"type":"text","text":"waves"},
		{"type":"image_url","image_url":{"url":"https://example.com/f.png"},"role":"first_frame"},
		{"type":"image_url","image_url":{"url":"https://example.com/l.png"},"role":"last_frame"}],
		"resolution":"1080p","ratio":"21:9","duration":12,"generate_audio":true,"camera_fixed":false,"return_last_frame":true}`, string(call.Body))

	// 没给分辨率 / 时长：按型号补默认（720p、5 秒），落库的请求带上补齐的值（没报实际值时按它计费）。
	submission, err = h.svc.Submit(context.Background(), videoTestGinContext(), account, officialVideoSeed(t, "seedance-2.0-fast"), VideoTaskRequest{
		Model: "seedance-2.0-fast", Prompt: "cat", Images: []VideoInputImage{{URL: "https://example.com/r.png", Role: VideoInputImageReference}}})
	require.NoError(t, err)
	require.Equal(t, VideoResolution720P, submission.Request.Resolution)
	require.Equal(t, 5, submission.Request.DurationSeconds)
	require.JSONEq(t, `{"model":"doubao-seedance-2-0-fast-260128","content":[{"type":"text","text":"cat"},
		{"type":"image_url","image_url":{"url":"https://example.com/r.png"},"role":"reference_image"}],"resolution":"720p","duration":5}`, string(h.lastCall(t).Body))

	before := len(h.upstream.recorded())
	refs := make([]VideoInputImage, 10)
	for i := range refs {
		refs[i] = VideoInputImage{URL: "https://example.com/r.png", Role: VideoInputImageReference}
	}
	for _, tc := range []struct {
		name, entry, contains string
		req                   VideoTaskRequest
	}{
		{"fast 没有 1080p", "seedance-2.0-fast", "resolution 1080p is not supported", VideoTaskRequest{Model: "seedance-2.0-fast", Prompt: "x", Resolution: VideoResolution1080P}},
		{"2.0 最长 15 秒，不收敛", "seedance-2.0", "between 4 and 15 seconds", VideoTaskRequest{Model: "seedance-2.0", Prompt: "x", DurationSeconds: 20}},
		{"2.0 最短 4 秒", "seedance-2.0", "between 4 and 15 seconds", VideoTaskRequest{Model: "seedance-2.0", Prompt: "x", DurationSeconds: 3}},
		{"2.5 没有 4k", "seedance-2.5", "resolution 4k is not supported", VideoTaskRequest{Model: "seedance-2.5", Prompt: "x", Resolution: VideoResolution4K}},
		{"不支持的宽高比", "seedance-2.0", "aspect ratio 3:2", VideoTaskRequest{Model: "seedance-2.0", Prompt: "x", AspectRatio: "3:2"}},
		{"首帧与参考图互斥", "seedance-2.0", "cannot be combined", VideoTaskRequest{Model: "seedance-2.0", Prompt: "x", Images: []VideoInputImage{
			{URL: "https://example.com/f.png", Role: VideoInputImageFirstFrame}, {URL: "https://example.com/r.png", Role: VideoInputImageReference}}}},
		{"只有尾帧", "seedance-2.0", "needs a first_frame", VideoTaskRequest{Model: "seedance-2.0", Prompt: "x", Images: []VideoInputImage{
			{URL: "https://example.com/l.png", Role: VideoInputImageLastFrame}}}},
		{"2.0 参考图最多 9 张", "seedance-2.0", "at most 9 reference images", VideoTaskRequest{Model: "seedance-2.0", Prompt: "x", Images: refs}},
	} {
		_, err := h.svc.Submit(context.Background(), videoTestGinContext(), account, officialVideoSeed(t, tc.entry), tc.req)
		requireVideoClientError(t, err, tc.contains)
	}
	require.Len(t, h.upstream.recorded(), before, "请求不合型号时不发上游")

	// 2.5 允许 30 秒（框架上限），参考图可到 30 张。
	_, err = h.svc.Submit(context.Background(), videoTestGinContext(), account, officialVideoSeed(t, "seedance-2.5"), VideoTaskRequest{
		Model: "seedance-2.5", Prompt: "x", DurationSeconds: 30, Images: refs})
	require.NoError(t, err)
	require.Equal(t, int64(30), gjson.GetBytes(h.lastCall(t).Body, "duration").Int())
}

func TestSeedanceVideoUpstreamQueryAndContent(t *testing.T) {
	h := newOfficialVideoHarness(t)
	statusBody := ""
	h.mux.HandleFunc("GET ark.ap-southeast.bytepluses.com/api/v3/contents/generations/tasks/cgt-1", func(w http.ResponseWriter, _ *http.Request) {
		writeVideoJSON(w, 200, statusBody)
	})
	h.mux.HandleFunc("GET ark-content.example.com/out.mp4", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = io.WriteString(w, "SEEDANCE")
	})
	account := videoTestOfficialKey(31, PlatformSeedance, DefaultSeedanceIntlBaseURL)
	adapter := seedanceVideoUpstream{s: h.svc.gateway}
	for _, tc := range []struct {
		name, body, url, code, message, resolution string
		state                                      VideoTaskState
		duration                                   float64
	}{
		{"queued", `{"id":"cgt-1","status":"queued"}`, "", "", "", "", VideoTaskQueued, 0},
		{"running", `{"id":"cgt-1","status":"running"}`, "", "", "", "", VideoTaskRunning, 0},
		{"succeeded", `{"id":"cgt-1","status":"succeeded","content":{"video_url":"https://ark-content.example.com/out.mp4"},"duration":10,"resolution":"1080p","ratio":"16:9","framespersecond":24}`,
			"https://ark-content.example.com/out.mp4", "", "", VideoResolution1080P, VideoTaskSucceeded, 10},
		{"succeeded by frames", `{"status":"succeeded","content":{"video_url":"https://ark-content.example.com/out.mp4"},"frames":121,"framespersecond":24,"resolution":"720p"}`,
			"https://ark-content.example.com/out.mp4", "", "", VideoResolution720P, VideoTaskSucceeded, 121.0 / 24},
		{"succeeded without url keeps polling", `{"status":"succeeded","content":{}}`, "", "", "", "", VideoTaskRunning, 0},
		{"failed", `{"status":"failed","error":{"code":"OutputVideoSensitiveContentDetected","message":"sensitive"}}`, "", "OutputVideoSensitiveContentDetected", "sensitive", "", VideoTaskFailed, 0},
		{"cancelled", `{"status":"cancelled"}`, "", "cancelled", "", "", VideoTaskCanceled, 0},
		{"expired", `{"status":"expired"}`, "", "expired", "", "", VideoTaskFailed, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			statusBody = tc.body
			result, err := adapter.Query(context.Background(), videoTestGinContext(), account, "cgt-1")
			require.NoError(t, err)
			require.Equal(t, tc.state, result.State)
			require.Equal(t, tc.url, result.VideoURL)
			require.InDelta(t, tc.duration, result.DurationSeconds, 1e-9)
			require.Equal(t, tc.resolution, result.Resolution)
			require.Equal(t, tc.code, result.ErrorCode)
			require.Equal(t, tc.message, result.ErrorMessage)
			require.Equal(t, "Bearer sk-seedance", h.lastCall(t).Auth)
		})
	}

	resp, err := adapter.OpenContent(context.Background(), videoTestGinContext(), account, "cgt-1",
		VideoTaskResult{State: VideoTaskSucceeded, VideoURL: "https://ark-content.example.com/out.mp4"})
	require.NoError(t, err)
	content, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, "SEEDANCE", string(content))
	require.Empty(t, h.lastCall(t).Auth, "签名地址下载不带 key")
	before := len(h.upstream.recorded())
	_, err = adapter.OpenContent(context.Background(), videoTestGinContext(), account, "cgt-1", VideoTaskResult{State: VideoTaskSucceeded, VideoURL: "https://10.0.0.8/v.mp4"})
	require.Error(t, err)
	_, err = adapter.OpenContent(context.Background(), videoTestGinContext(), account, "cgt-1", VideoTaskResult{State: VideoTaskSucceeded, VideoURL: "/relative.mp4"})
	require.Error(t, err)
	require.Len(t, h.upstream.recorded(), before, "私网 / 相对地址不发请求")
}

func TestKlingVideoUpstreamCreate(t *testing.T) {
	h := newOfficialVideoHarness(t)
	h.mux.HandleFunc("POST api-beijing.klingai.com/text-to-video/kling-3.0", func(w http.ResponseWriter, _ *http.Request) {
		writeVideoJSON(w, 200, `{"code":0,"message":"SUCCEED","request_id":"req-should-not-be-used","data":{"id":"kt-1","status":"submitted"}}`)
	})
	h.mux.HandleFunc("POST api-beijing.klingai.com/image-to-video/kling-2.6", func(w http.ResponseWriter, _ *http.Request) {
		writeVideoJSON(w, 200, `{"code":0,"request_id":"r2","data":{"id":"ki-1","status":"submitted"}}`)
	})
	account := videoTestOfficialKey(32, PlatformKling, DefaultKlingCNBaseURL)
	audio := true
	seed := int64(9)

	submission, err := h.svc.Submit(context.Background(), videoTestGinContext(), account, officialVideoSeed(t, "kling-3.0"), VideoTaskRequest{
		Model: "kling-3.0", Prompt: "a girl on a train", NegativePrompt: "blur", Resolution: VideoResolution4K, AspectRatio: "9:16",
		DurationSeconds: 15, GenerateAudio: &audio, Seed: &seed, Extra: map[string]any{"settings": map[string]any{"multi_shot": false, "duration": 3}}})
	require.NoError(t, err)
	require.Equal(t, "kt-1", submission.UpstreamTaskID, "任务 ID 取 data.id，不是 request_id")
	call := h.lastCall(t)
	require.Equal(t, "https://api-beijing.klingai.com/text-to-video/kling-3.0", call.URL)
	require.Equal(t, "Bearer sk-kling", call.Auth)
	require.JSONEq(t, `{"prompt":"a girl on a train","settings":{"resolution":"4k","aspect_ratio":"9:16","duration":15,"audio":"native","multi_shot":false},
		"options":{"watermark_info":{"enabled":false}}}`, string(call.Body))

	audio = false
	submission, err = h.svc.Submit(context.Background(), videoTestGinContext(), account, officialVideoSeed(t, "kling-2.6"), VideoTaskRequest{
		Model: "kling-2.6", Prompt: "smile", AspectRatio: "16:9", GenerateAudio: &audio,
		Images: []VideoInputImage{{URL: "https://example.com/l.png", Role: VideoInputImageLastFrame}, {URL: "https://example.com/f.png", Role: VideoInputImageFirstFrame}}})
	require.NoError(t, err)
	require.Equal(t, "ki-1", submission.UpstreamTaskID)
	require.Equal(t, VideoResolution720P, submission.Request.Resolution)
	require.Equal(t, 5, submission.Request.DurationSeconds)
	call = h.lastCall(t)
	require.Equal(t, "https://api-beijing.klingai.com/image-to-video/kling-2.6", call.URL)
	// 图生视频没有 aspect_ratio（按图片定），首帧在前。
	require.JSONEq(t, `{"contents":[{"type":"prompt","text":"smile"},{"type":"first_frame","url":"https://example.com/f.png"},{"type":"last_frame","url":"https://example.com/l.png"}],
		"settings":{"resolution":"720p","duration":5,"audio":"off"},"options":{"watermark_info":{"enabled":false}}}`, string(call.Body))

	// 2.5 Turbo 没有 audio：不发。
	h.mux.HandleFunc("POST api-beijing.klingai.com/text-to-video/kling-2.5-turbo", func(w http.ResponseWriter, _ *http.Request) {
		writeVideoJSON(w, 200, `{"code":0,"data":{"id":"kt-2"}}`)
	})
	audio = true
	_, err = h.svc.Submit(context.Background(), videoTestGinContext(), account, officialVideoSeed(t, "kling-2.5-turbo"), VideoTaskRequest{
		Model: "kling-2.5-turbo", Prompt: "x", DurationSeconds: 10, Resolution: VideoResolution1080P, GenerateAudio: &audio})
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(h.lastCall(t).Body, "settings.audio").Exists())

	before := len(h.upstream.recorded())
	for _, tc := range []struct {
		name, entry, contains string
		req                   VideoTaskRequest
	}{
		{"参考图不支持", "kling-3.0", "reference images are not supported", VideoTaskRequest{Model: "kling-3.0", Prompt: "x",
			Images: []VideoInputImage{{URL: "https://example.com/r.png", Role: VideoInputImageReference}}}},
		{"3.0 Turbo 没有尾帧", "kling-3.0-turbo", "last_frame images are not supported", VideoTaskRequest{Model: "kling-3.0-turbo", Prompt: "x",
			Images: []VideoInputImage{{URL: "https://example.com/f.png", Role: VideoInputImageFirstFrame}, {URL: "https://example.com/l.png", Role: VideoInputImageLastFrame}}}},
		{"只有尾帧", "kling-3.0", "needs a first_frame", VideoTaskRequest{Model: "kling-3.0", Prompt: "x",
			Images: []VideoInputImage{{URL: "https://example.com/l.png", Role: VideoInputImageLastFrame}}}},
		{"2.6 只有 5 / 10 秒", "kling-2.6", "one of 5, 10 seconds", VideoTaskRequest{Model: "kling-2.6", Prompt: "x", DurationSeconds: 7}},
		{"3.0 最长 15 秒", "kling-3.0", "between 3 and 15 seconds", VideoTaskRequest{Model: "kling-3.0", Prompt: "x", DurationSeconds: 16}},
		{"3.0 Turbo 没有 4k", "kling-3.0-turbo", "resolution 4k is not supported", VideoTaskRequest{Model: "kling-3.0-turbo", Prompt: "x", Resolution: VideoResolution4K}},
		{"没有 480p", "kling-3.0", "resolution 480p is not supported", VideoTaskRequest{Model: "kling-3.0", Prompt: "x", Resolution: VideoResolution480P}},
		{"文生视频宽高比", "kling-3.0", "aspect ratio 4:3", VideoTaskRequest{Model: "kling-3.0", Prompt: "x", AspectRatio: "4:3"}},
	} {
		_, err := h.svc.Submit(context.Background(), videoTestGinContext(), account, officialVideoSeed(t, tc.entry), tc.req)
		requireVideoClientError(t, err, tc.contains)
	}
	require.Len(t, h.upstream.recorded(), before, "请求不合型号时不发上游")
}

func TestKlingVideoUpstreamQueryAndContent(t *testing.T) {
	h := newOfficialVideoHarness(t)
	statusBody := ""
	h.mux.HandleFunc("GET api-singapore.klingai.com/tasks", func(w http.ResponseWriter, _ *http.Request) { writeVideoJSON(w, 200, statusBody) })
	h.mux.HandleFunc("GET kling-cdn.example.com/v.mp4", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "KLING") })
	account := videoTestOfficialKey(33, PlatformKling, DefaultKlingIntlBaseURL)
	adapter := klingVideoUpstream{s: h.svc.gateway}
	for _, tc := range []struct {
		name, body, url, message string
		state                    VideoTaskState
		duration                 float64
	}{
		{"submitted", `{"code":0,"data":[{"id":"kt-1","status":"submitted"}]}`, "", "", VideoTaskQueued, 0},
		{"processing", `{"code":0,"data":[{"id":"kt-1","status":"processing"}]}`, "", "", VideoTaskRunning, 0},
		{"succeeded", `{"code":0,"data":[{"id":"other","status":"failed"},{"id":"kt-1","status":"succeeded","outputs":[
			{"type":"image","url":"https://kling-cdn.example.com/cover.png"},{"type":"video","url":"https://kling-cdn.example.com/v.mp4","watermark_url":"https://kling-cdn.example.com/w.mp4","duration":"5.04"}]}]}`,
			"https://kling-cdn.example.com/v.mp4", "", VideoTaskSucceeded, 5.04},
		{"succeeded without video keeps polling", `{"code":0,"data":[{"id":"kt-1","status":"succeeded","outputs":[]}]}`, "", "", VideoTaskRunning, 0},
		{"failed", `{"code":0,"data":[{"id":"kt-1","status":"failed","message":"content risk control"}]}`, "", "content risk control", VideoTaskFailed, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			statusBody = tc.body
			result, err := adapter.Query(context.Background(), videoTestGinContext(), account, "kt-1")
			require.NoError(t, err)
			require.Equal(t, tc.state, result.State)
			require.Equal(t, tc.url, result.VideoURL)
			require.InDelta(t, tc.duration, result.DurationSeconds, 1e-9)
			require.Equal(t, tc.message, result.ErrorMessage)
			require.Equal(t, "https://api-singapore.klingai.com/tasks?task_ids=kt-1", h.lastCall(t).URL)
		})
	}
	statusBody = `{"code":0,"data":[]}`
	_, err := adapter.Query(context.Background(), videoTestGinContext(), account, "kt-1")
	require.ErrorContains(t, err, "not in the query result", "查不到当临时错误，不判失败")
	var httpErr *videoUpstreamHTTPError
	require.False(t, errors.As(err, &httpErr))

	resp, err := adapter.OpenContent(context.Background(), videoTestGinContext(), account, "kt-1", VideoTaskResult{State: VideoTaskSucceeded, VideoURL: "https://kling-cdn.example.com/v.mp4"})
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Empty(t, h.lastCall(t).Auth)
}

func TestMiniMaxVideoUpstreamCreate(t *testing.T) {
	h := newOfficialVideoHarness(t)
	h.mux.HandleFunc("POST api.minimax.io/v2/video_generation", func(w http.ResponseWriter, _ *http.Request) {
		writeVideoJSON(w, 200, `{"task_id":"424010985738629"}`)
	})
	account := videoTestOfficialKey(34, PlatformMiniMax, DefaultMiniMaxVideoIntlBaseURL)
	seed, audio := int64(1), true

	submission, err := h.svc.Submit(context.Background(), videoTestGinContext(), account, officialVideoSeed(t, "MiniMax-H3"), VideoTaskRequest{
		Model: "MiniMax-H3", Prompt: "space opera", NegativePrompt: "blur", Resolution: VideoResolution2K, DurationSeconds: 6, Seed: &seed, GenerateAudio: &audio,
		Extra: map[string]any{"callback_url": "https://cb.example.com/x"}})
	require.NoError(t, err)
	require.Equal(t, "424010985738629", submission.UpstreamTaskID)
	require.Equal(t, "16:9", submission.Request.AspectRatio, "文生视频 ratio 必填，没给补 16:9")
	call := h.lastCall(t)
	require.Equal(t, "https://api.minimax.io/v2/video_generation", call.URL)
	require.Equal(t, "Bearer sk-minimax", call.Auth)
	require.JSONEq(t, `{"model":"MiniMax-H3","content":[{"type":"text","text":"space opera"}],"resolution":"2K","duration":6,"ratio":"16:9",
		"callback_url":"https://cb.example.com/x"}`, string(call.Body))

	// 图生视频不发 ratio；H3-Max 默认 768P、5 秒；Extra 的 extra.prompt_expansion_mode 原样透传。
	submission, err = h.svc.Submit(context.Background(), videoTestGinContext(), account, officialVideoSeed(t, "MiniMax-H3-Max"), VideoTaskRequest{
		Model: "MiniMax-H3-Max", Prompt: "ramen", AspectRatio: "9:16",
		Images: []VideoInputImage{{URL: "https://example.com/f.png", Role: VideoInputImageFirstFrame}, {URL: "https://example.com/l.png", Role: VideoInputImageLastFrame}},
		Extra:  map[string]any{"extra": map[string]any{"prompt_expansion_mode": "quality"}}})
	require.NoError(t, err)
	require.Equal(t, VideoResolution768P, submission.Request.Resolution)
	require.Equal(t, 5, submission.Request.DurationSeconds)
	require.JSONEq(t, `{"model":"MiniMax-H3-Max","content":[{"type":"text","text":"ramen"},
		{"type":"image_url","image_url":{"url":"https://example.com/f.png"},"role":"first_frame"},
		{"type":"image_url","image_url":{"url":"https://example.com/l.png"},"role":"last_frame"}],
		"resolution":"768P","duration":5,"extra":{"prompt_expansion_mode":"quality"}}`, string(h.lastCall(t).Body))

	_, err = h.svc.Submit(context.Background(), videoTestGinContext(), account, officialVideoSeed(t, "MiniMax-H3"), VideoTaskRequest{
		Model: "MiniMax-H3", Prompt: "ref", AspectRatio: "1:1", Images: []VideoInputImage{{URL: "https://example.com/r.png", Role: VideoInputImageReference}}})
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"MiniMax-H3","content":[{"type":"text","text":"ref"},{"type":"image_url","image_url":{"url":"https://example.com/r.png"},"role":"reference_image"}],
		"resolution":"768P","duration":5,"ratio":"1:1"}`, string(h.lastCall(t).Body))

	before := len(h.upstream.recorded())
	for _, tc := range []struct {
		name, entry, contains string
		req                   VideoTaskRequest
	}{
		{"H3 没有 1080p", "MiniMax-H3", "resolution 1080p is not supported", VideoTaskRequest{Model: "MiniMax-H3", Prompt: "x", Resolution: VideoResolution1080P}},
		{"H3-Max 没有 2K", "MiniMax-H3-Max", "resolution 2k is not supported", VideoTaskRequest{Model: "MiniMax-H3-Max", Prompt: "x", Resolution: VideoResolution2K}},
		{"H3-Max 不支持 4 秒", "MiniMax-H3-Max", "between 5 and 15 seconds", VideoTaskRequest{Model: "MiniMax-H3-Max", Prompt: "x", DurationSeconds: 4}},
		{"H3 最长 15 秒", "MiniMax-H3", "between 4 and 15 seconds", VideoTaskRequest{Model: "MiniMax-H3", Prompt: "x", DurationSeconds: 16}},
		{"prompt 必填", "MiniMax-H3", "prompt is required", VideoTaskRequest{Model: "MiniMax-H3", Images: []VideoInputImage{{URL: "https://example.com/f.png", Role: VideoInputImageFirstFrame}}}},
		{"文生视频不能 adaptive", "MiniMax-H3", "not adaptive", VideoTaskRequest{Model: "MiniMax-H3", Prompt: "x", AspectRatio: "adaptive"}},
		{"不支持的宽高比", "MiniMax-H3", "aspect ratio 2:3", VideoTaskRequest{Model: "MiniMax-H3", Prompt: "x", AspectRatio: "2:3"}},
		{"首帧与参考图互斥", "MiniMax-H3", "cannot be combined", VideoTaskRequest{Model: "MiniMax-H3", Prompt: "x", Images: []VideoInputImage{
			{URL: "https://example.com/f.png", Role: VideoInputImageFirstFrame}, {URL: "https://example.com/r.png", Role: VideoInputImageReference}}}},
	} {
		_, err := h.svc.Submit(context.Background(), videoTestGinContext(), account, officialVideoSeed(t, tc.entry), tc.req)
		requireVideoClientError(t, err, tc.contains)
	}
	require.Len(t, h.upstream.recorded(), before, "请求不合型号时不发上游")
}

func TestMiniMaxVideoUpstreamQueryAndContent(t *testing.T) {
	h := newOfficialVideoHarness(t)
	statusBody := ""
	h.mux.HandleFunc("GET api.minimax.cn/v2/query/video_generation/424010985738629", func(w http.ResponseWriter, _ *http.Request) {
		writeVideoJSON(w, 200, statusBody)
	})
	h.mux.HandleFunc("GET video-product.cdn.example.com/output.mp4", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "HAILUO") })
	account := videoTestOfficialKey(35, PlatformMiniMax, DefaultMiniMaxVideoCNBaseURL)
	adapter := miniMaxVideoUpstream{s: h.svc.gateway}
	for _, tc := range []struct {
		name, body, url, code, message, resolution string
		state                                      VideoTaskState
		duration                                   float64
	}{
		{"queued", `{"task":{"id":"424010985738629","status":"queued"}}`, "", "", "", "", VideoTaskQueued, 0},
		{"running", `{"task":{"status":"running","resolution":"2K","duration":5}}`, "", "", "", VideoResolution2K, VideoTaskRunning, 5},
		{"succeeded", `{"task":{"status":"succeeded","content":{"url":"https://video-product.cdn.example.com/output.mp4"},"resolution":"768P","duration":6,"usage":{"total_seconds":6}}}`,
			"https://video-product.cdn.example.com/output.mp4", "", "", VideoResolution768P, VideoTaskSucceeded, 6},
		{"succeeded without url keeps polling", `{"task":{"status":"succeeded","content":{}}}`, "", "", "", "", VideoTaskRunning, 0},
		{"failed", `{"task":{"status":"failed","error":{"code":"1026","message":"video description contains sensitive content"},"resolution":"2K","duration":5}}`,
			"", "1026", "video description contains sensitive content", VideoResolution2K, VideoTaskFailed, 5},
		{"cancelled", `{"task":{"status":"cancelled"}}`, "", "cancelled", "", "", VideoTaskCanceled, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			statusBody = tc.body
			result, err := adapter.Query(context.Background(), videoTestGinContext(), account, "424010985738629")
			require.NoError(t, err)
			require.Equal(t, tc.state, result.State)
			require.Equal(t, tc.url, result.VideoURL)
			require.InDelta(t, tc.duration, result.DurationSeconds, 1e-9)
			require.Equal(t, tc.resolution, result.Resolution)
			require.Equal(t, tc.code, result.ErrorCode)
			require.Equal(t, tc.message, result.ErrorMessage)
		})
	}
	statusBody = `{"base_resp":{"status_code":2013}}`
	_, err := adapter.Query(context.Background(), videoTestGinContext(), account, "424010985738629")
	require.ErrorContains(t, err, "no task object")

	resp, err := adapter.OpenContent(context.Background(), videoTestGinContext(), account, "424010985738629",
		VideoTaskResult{State: VideoTaskSucceeded, VideoURL: "https://video-product.cdn.example.com/output.mp4"})
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, "HAILUO", string(body))
	require.Empty(t, h.lastCall(t).Auth)
}

// 三家官方适配器创建失败的归类：401 凭据失效（换号并把这把 key 标成出错）、429 限流换号、4xx 请求错误原样回客户端、
// 5xx 与网络错误换号。
func TestOfficialVideoUpstreamCreateErrorMapping(t *testing.T) {
	type vendorCase struct {
		name, entry, path, badRequest, badMessage string
		account                                   *Account
	}
	vendors := []vendorCase{
		{"seedance", "seedance-2.0", "POST ark.cn-beijing.volces.com/api/v3/contents/generations/tasks",
			`{"error":{"code":"InvalidParameter","message":"the parameter ratio is invalid","type":"BadRequest"}}`, "the parameter ratio is invalid",
			videoTestOfficialKey(40, PlatformSeedance, DefaultSeedanceCNBaseURL)},
		{"kling", "kling-3.0", "POST api-beijing.klingai.com/text-to-video/kling-3.0",
			`{"code":1201,"message":"prompt is too long","request_id":"r"}`, "prompt is too long",
			videoTestOfficialKey(41, PlatformKling, DefaultKlingCNBaseURL)},
		{"minimax", "MiniMax-H3", "POST api.minimax.cn/v2/video_generation",
			`{"type":"error","error":{"type":"bad_request_error","message":"invalid params (2013)","http_code":"400"},"request_id":"r"}`, "invalid params (2013)",
			videoTestOfficialKey(42, PlatformMiniMax, DefaultMiniMaxVideoCNBaseURL)},
	}
	for _, vendor := range vendors {
		t.Run(vendor.name, func(t *testing.T) {
			h := newOfficialVideoHarness(t)
			status, body := 0, ""
			h.mux.HandleFunc(vendor.path, func(w http.ResponseWriter, _ *http.Request) { writeVideoJSON(w, status, body) })
			req := VideoTaskRequest{Model: vendor.entry, Prompt: "waves"}
			submit := func() (*gin.Context, error) {
				c := videoTestGinContext()
				_, err := h.svc.Submit(context.Background(), c, vendor.account, officialVideoSeed(t, vendor.entry), req)
				return c, err
			}
			opsKinds := func(c *gin.Context) []string {
				var kinds []string
				for _, event := range OpsUpstreamErrorEventsFromContext(c) {
					kinds = append(kinds, event.Kind)
				}
				return kinds
			}

			status, body = http.StatusUnauthorized, `{"error":{"message":"invalid api key"}}`
			c, err := submit()
			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, err, &failoverErr)
			require.Equal(t, http.StatusUnauthorized, failoverErr.StatusCode)
			require.Equal(t, []string{"failover"}, opsKinds(c))
			require.Equal(t, []int64{vendor.account.ID}, h.errors.setErrorIDs, "401 把这把 key 标成凭据失效")

			status, body = http.StatusTooManyRequests, `{"code":1303,"message":"parallel task over resource pack limit"}`
			_, err = submit()
			require.ErrorAs(t, err, &failoverErr)
			require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)

			status, body = http.StatusBadRequest, vendor.badRequest
			c, err = submit()
			var clientErr *VideoTaskClientError
			require.ErrorAs(t, err, &clientErr)
			require.Equal(t, http.StatusBadRequest, clientErr.StatusCode)
			require.Equal(t, vendor.badMessage, clientErr.Message)
			require.Equal(t, []string{"http_error"}, opsKinds(c))

			status, body = http.StatusInternalServerError, `{"error":{"message":"internal error"}}`
			_, err = submit()
			require.ErrorAs(t, err, &failoverErr)
			require.Equal(t, http.StatusInternalServerError, failoverErr.StatusCode)

			status, body = http.StatusOK, `{"code":0}`
			_, err = submit()
			require.ErrorAs(t, err, &failoverErr, "没回任务 ID 当上游故障换号")

			host := strings.Fields(vendor.path)[1]
			h.upstream.transportErr[host[:strings.Index(host, "/")]] = errors.New("dial tcp: i/o timeout")
			_, err = submit()
			require.ErrorAs(t, err, &failoverErr)
			require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
		})
	}
}

// 条目分了档却没有请求的分辨率：创建时拒绝，不按别的档计；没分档的条目不拦。
func TestOfficialVideoSubmitRejectsMissingTier(t *testing.T) {
	h := newOfficialVideoHarness(t)
	h.mux.HandleFunc("POST api.minimax.cn/v2/video_generation", func(w http.ResponseWriter, _ *http.Request) {
		writeVideoJSON(w, 200, `{"task_id":"t-1"}`)
	})
	account := videoTestOfficialKey(50, PlatformMiniMax, DefaultMiniMaxVideoCNBaseURL)
	entry := officialVideoSeed(t, "MiniMax-H3")
	entry.Intervals = entry.Intervals[:1] // 只留 768p

	_, err := h.svc.Submit(context.Background(), videoTestGinContext(), account, entry, VideoTaskRequest{Model: "MiniMax-H3", Prompt: "x", Resolution: VideoResolution2K})
	requireVideoClientError(t, err, "resolution 2k is not available for model MiniMax-H3")
	require.Empty(t, h.upstream.recorded())

	_, err = h.svc.Submit(context.Background(), videoTestGinContext(), account, entry, VideoTaskRequest{Model: "MiniMax-H3", Prompt: "x"})
	require.NoError(t, err, "补的默认 768p 有档")

	entry.Intervals = nil
	_, err = h.svc.Submit(context.Background(), videoTestGinContext(), account, entry, VideoTaskRequest{Model: "MiniMax-H3", Prompt: "x", Resolution: VideoResolution2K})
	require.NoError(t, err, "没分档的条目按单一每秒价计")
}

// 框架的时长上限放到 30 秒（Seedance 2.5）后，xAI 形态的上游仍按 xAI 的 1–15 秒拦（中转 / 成品号同一条）。
func TestVideoTaskXAIDurationLimit(t *testing.T) {
	h := newOfficialVideoHarness(t)
	h.mux.HandleFunc("POST relay.example.com/v1/videos/generations", func(w http.ResponseWriter, _ *http.Request) {
		writeVideoJSON(w, 200, `{"request_id":"x-1"}`)
	})
	relay := videoTestRelayKey(2, "https://relay.example.com/v1")
	grokEntry := &ModelCatalogEntry{ModelID: "grok-imagine-video", Vendor: "xai", BillingMode: BillingModeVideo}
	_, err := h.svc.Submit(context.Background(), videoTestGinContext(), relay, grokEntry, VideoTaskRequest{Model: "grok-imagine-video", Prompt: "x", DurationSeconds: 16})
	requireVideoClientError(t, err, "between 1 and 15 seconds")
	require.Empty(t, h.upstream.recorded())
	_, err = h.svc.Submit(context.Background(), videoTestGinContext(), relay, grokEntry, VideoTaskRequest{Model: "grok-imagine-video", Prompt: "x", DurationSeconds: 15})
	require.NoError(t, err)
	_, err = grokOAuthVideoUpstream{}.PrepareRequest(VideoTaskRequest{DurationSeconds: 16}, "grok-imagine-video")
	requireVideoClientError(t, err, "between 1 and 15 seconds")
}

// 注册表：官方视频 key 只承接同厂商的条目；指向厂商官方域名的对话 key 没有视频形态；中转 key 仍按 new-api 形态承接。
func TestOfficialVideoUpstreamRegistry(t *testing.T) {
	seedanceKey := videoTestOfficialKey(1, PlatformSeedance, DefaultSeedanceCNBaseURL)
	klingKey := videoTestOfficialKey(2, PlatformKling, DefaultKlingIntlBaseURL)
	miniMaxVideoKey := videoTestOfficialKey(3, PlatformMiniMax, DefaultMiniMaxVideoCNBaseURL)
	miniMaxChatKey := videoTestRelayKey(4, "https://api.minimaxi.com/v1")
	kimiChatKey := videoTestRelayKey(5, "https://api.moonshot.cn/v1")
	arkChatKey := videoTestRelayKey(6, DefaultSeedanceCNBaseURL)
	relay := videoTestRelayKey(7, "https://relay.example.com/v1")
	for _, tc := range []struct {
		vendor  string
		account *Account
		want    VideoUpstreamKind
	}{
		{PlatformSeedance, seedanceKey, VideoUpstreamSeedanceOfficial},
		{PlatformKling, klingKey, VideoUpstreamKlingOfficial},
		{PlatformMiniMax, miniMaxVideoKey, VideoUpstreamMiniMaxOfficial},
		{PlatformKling, seedanceKey, ""},
		{PlatformSeedance, miniMaxVideoKey, ""},
		{PlatformGrok, seedanceKey, ""},
		{PlatformOpenAI, klingKey, ""},
		{PlatformMiniMax, miniMaxChatKey, ""},
		{PlatformKling, kimiChatKey, ""},
		{PlatformSeedance, arkChatKey, VideoUpstreamNewAPIRelay},
		{PlatformSeedance, relay, VideoUpstreamNewAPIRelay},
		{PlatformKling, relay, VideoUpstreamNewAPIRelay},
	} {
		kind, ok := VideoTaskUpstreamKindFor(tc.vendor, tc.account)
		require.Equal(t, tc.want != "", ok, "%s/%d", tc.vendor, tc.account.ID)
		require.Equal(t, tc.want, kind, "%s/%d", tc.vendor, tc.account.ID)
		admitted, _ := SelectOptions{Capability: OpenAIEndpointCapabilityVideoGeneration, VideoVendor: tc.vendor}.admits(nil, nil, tc.account)
		require.Equal(t, tc.want != "", admitted, "调度准入与注册表同口径 %s/%d", tc.vendor, tc.account.ID)
	}
	require.Equal(t, PlatformSeedance, VideoTaskVendorFor(&ModelCatalogEntry{Vendor: "seedance"}))
	require.Equal(t, PlatformKling, VideoTaskVendorFor(&ModelCatalogEntry{Vendor: "Kling"}))
	require.Equal(t, PlatformMiniMax, VideoTaskVendorFor(&ModelCatalogEntry{Vendor: "minimax"}))
	require.Equal(t, "volcengine", VideoTaskVendorFor(&ModelCatalogEntry{Vendor: "volcengine"}), "豆包对话的厂商串不归 Seedance")
}

// video key 只承接视频：四个对话入站协议、扩展端点（图片 / 向量 / 搜索）都承接不了，对话与生图条目也绑不上。
func TestVideoKeyServesOnlyVideo(t *testing.T) {
	for _, key := range []*Account{
		videoTestOfficialKey(1, PlatformSeedance, DefaultSeedanceCNBaseURL),
		videoTestOfficialKey(2, PlatformKling, DefaultKlingCNBaseURL),
		videoTestOfficialKey(3, PlatformMiniMax, DefaultMiniMaxVideoIntlBaseURL),
	} {
		require.Equal(t, []string{APIProtocolVideo}, key.UpstreamProtocols(), key.Platform)
		for _, inbound := range append(ConversationProtocols(), "") {
			require.False(t, key.ServesInbound(inbound), "%s 不承接入站 %q", key.Platform, inbound)
			require.Empty(t, key.KeyUpstreamProtocolFor(PlatformOpenAI, inbound), "%s 不进平台池 %q", key.Platform, inbound)
		}
		require.False(t, AccountServesPlatformForAnyInbound(key, key.Platform))
		for _, capability := range []OpenAIEndpointCapability{OpenAIEndpointCapabilityChatCompletions, OpenAIEndpointCapabilityResponses,
			OpenAIEndpointCapabilityEmbeddings, OpenAIEndpointCapabilityAlphaSearch, OpenAIEndpointCapabilityGrokImageGeneration,
			OpenAIEndpointCapabilityGrokMediaGeneration} {
			if capability == OpenAIEndpointCapabilityChatCompletions {
				// 对话能力在承接判定里由协议注册表把关（上面 ServesInbound 全为 false），这里只看扩展端点。
				continue
			}
			require.False(t, key.SupportsOpenAIEndpointCapability(capability), "%s %s", key.Platform, capability)
		}
		require.True(t, key.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityVideoGeneration), key.Platform)
		require.Empty(t, openAIGatewayKeyExtensionBaseURL(key))
		for _, served := range CatalogBindingServes(key) {
			require.False(t, served)
		}
		require.Error(t, AccountServesCatalogEntry(&ModelCatalogEntry{ModelID: "glm-5"}, key))
	}
	unknown := &Account{ID: 9, Type: AccountTypeAPIKey, ProtocolEndpoints: map[string]string{APIProtocolVideo: "https://video-relay.example.com"}}
	require.False(t, unknown.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityVideoGeneration), "认不出厂商的 video 地址没有视频能力")
	require.Empty(t, videoAccountKind(unknown))
}

// 目录路由下的平台准入：视频创建按视频注册表放行官方视频 key（它没有对话协议），对话请求照旧按协议注册表挡住它。
func TestVideoCreateRoutePredicateAdmitsVideoKeys(t *testing.T) {
	routed := WithCatalogRoute(context.Background(), CatalogRoute{EntryID: 1, Entry: &ModelCatalogEntry{ID: 1, ModelID: "seedance-2.0", Vendor: "seedance"}})
	videoCtx := WithSelectOptions(routed, SelectOptions{Capability: OpenAIEndpointCapabilityVideoGeneration, VideoVendor: PlatformSeedance})
	key := videoTestOfficialKey(1, PlatformSeedance, DefaultSeedanceCNBaseURL)
	require.True(t, isAccountSchedulableOnPlatform(videoCtx, key, ""))
	require.False(t, isAccountSchedulableOnPlatform(routed, key, ""), "非视频请求（扩展端点 / 对话）不派给 video key")
	require.False(t, isAccountSchedulableOnPlatform(WithInboundProtocol(routed, APIProtocolChatCompletions), key, ""))
	require.True(t, isAccountSchedulableOnPlatform(videoCtx, videoTestRelayKey(2, "https://relay.example.com/v1"), ""))
	require.True(t, isAccountSchedulableOnPlatform(videoCtx, videoTestGrokOAuth(3), ""))
	require.False(t, isAccountSchedulableOnPlatform(videoCtx, videoTestRelayKey(4, "https://api.minimaxi.com/v1"), ""), "厂商官方域名上的对话 key")
	require.False(t, isAccountSchedulableOnPlatform(videoCtx, key, PlatformAntigravity))
	require.Equal(t, routed, WithInboundProtocol(routed, APIProtocolVideo), "video 不是入站协议")
}

// 视频条目的绑定校验与选号同一条规则：同厂商的官方视频 key、中转 key 能绑；别家的视频 key、厂商官方域名上的对话 key 不能绑。
func TestCatalogVideoEntryBindingValidation(t *testing.T) {
	svc, _ := newTestModelCatalogService()
	seedance := officialVideoSeed(t, "seedance-2.0")
	kling := officialVideoSeed(t, "kling-3.0")
	grokVideo := &ModelCatalogEntry{ModelID: "grok-imagine-video", Vendor: "xai", BillingMode: BillingModeVideo}
	seedanceKey := videoTestOfficialKey(1, PlatformSeedance, DefaultSeedanceCNBaseURL)
	relay := videoTestRelayKey(2, "https://relay.example.com/v1")
	anthropicRelay := &Account{ID: 3, Type: AccountTypeAPIKey, ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://relay.example.com"}}
	miniMaxChat := videoTestRelayKey(4, "https://api.minimaxi.com/v1")
	grok := videoTestGrokOAuth(5)

	require.True(t, svc.CanBindAccount(seedance, seedanceKey))
	require.True(t, svc.CanBindAccount(seedance, relay), "中转按 new-api 形态承接")
	require.False(t, svc.CanBindAccount(kling, seedanceKey), "别家的视频 key")
	require.False(t, svc.CanBindAccount(seedance, anthropicRelay), "中转没有扩展端点根地址")
	require.False(t, svc.CanBindAccount(officialVideoSeed(t, "MiniMax-H3"), miniMaxChat), "MiniMax 对话 key 没有视频地址")
	require.False(t, svc.CanBindAccount(seedance, grok), "Grok 成品号只承接 xAI 视频")
	require.True(t, svc.CanBindAccount(grokVideo, grok))
	require.True(t, svc.CanBindAccount(grokVideo, relay))
	require.False(t, svc.CanBindAccount(grokVideo, seedanceKey))
	require.False(t, svc.CanBindAccount(&ModelCatalogEntry{ModelID: "glm-5", Vendor: "zhipu"}, seedanceKey), "对话条目绑不上 video key")
	require.False(t, svc.CanBindAccount(&ModelCatalogEntry{ModelID: "img", Vendor: "openai", BillingMode: BillingModeImage}, seedanceKey), "生图条目绑不上 video key")
}

// 建号时没带平台：video key 按地址推导出视频厂商平台。
func TestResolveCreateAccountPlatformForVideoKeys(t *testing.T) {
	for _, tc := range []struct{ url, want string }{
		{DefaultSeedanceIntlBaseURL, PlatformSeedance},
		{DefaultKlingCNBaseURL, PlatformKling},
		{DefaultMiniMaxVideoCNBaseURL, PlatformMiniMax},
	} {
		input := &CreateAccountInput{Type: AccountTypeAPIKey, ProtocolEndpoints: map[string]string{APIProtocolVideo: tc.url}}
		require.NoError(t, resolveCreateAccountPlatform(input))
		require.Equal(t, tc.want, input.Platform, tc.url)
	}
	input := &CreateAccountInput{Type: AccountTypeAPIKey, ProtocolEndpoints: map[string]string{APIProtocolVideo: "https://relay.example.com"}}
	require.ErrorContains(t, resolveCreateAccountPlatform(input), "official address of a video vendor")
}
