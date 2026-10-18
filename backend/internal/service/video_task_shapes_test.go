//go:build unit

package service

import (
	"bytes"
	"encoding/base64"
	"mime/multipart"
	"net/textproto"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestVideoSizeMappingTable(t *testing.T) {
	for _, tc := range []struct {
		size, resolution, aspect, roundTrip string
	}{
		{"720x1280", VideoResolution720P, "9:16", "720x1280"},
		{"1280x720", VideoResolution720P, "16:9", "1280x720"},
		{"1024x1792", VideoResolution1080P, "9:16", "1080x1920"},
		{"1792x1024", VideoResolution1080P, "16:9", "1920x1080"},
		{"854x480", VideoResolution480P, "16:9", "854x480"},
		{"1366x768", VideoResolution768P, "16:9", "1366x768"},
		{"1920x1080", VideoResolution1080P, "16:9", "1920x1080"},
		{"1080x1080", VideoResolution1080P, "1:1", "1080x1080"},
		{"2560x1440", VideoResolution2K, "16:9", "2560x1440"},
		{"3840x2160", VideoResolution4K, "16:9", "3840x2160"},
	} {
		t.Run(tc.size, func(t *testing.T) {
			var req VideoTaskRequest
			require.NoError(t, applyVideoSize(&req, tc.size))
			require.Equal(t, tc.resolution, req.Resolution)
			require.Equal(t, tc.aspect, req.AspectRatio)
			require.Equal(t, tc.roundTrip, VideoSizeFor(req.Resolution, req.AspectRatio))
		})
	}
	require.Empty(t, VideoSizeFor("", "16:9"), "缺分辨率不猜")
	require.Empty(t, VideoSizeFor(VideoResolution720P, ""), "缺宽高比不猜")
}

func TestNormalizeVideoResolutionAndBillingTier(t *testing.T) {
	for raw, want := range map[string][2]string{
		"480P":  {VideoResolution480P, VideoBillingResolution480P},
		"720p":  {VideoResolution720P, VideoBillingResolution720P},
		"768P":  {VideoResolution768P, VideoBillingResolution1080P},
		"1080":  {VideoResolution1080P, VideoBillingResolution1080P},
		"2K":    {VideoResolution2K, VideoBillingResolution1080P},
		"4k":    {VideoResolution4K, VideoBillingResolution1080P},
		"bogus": {"", ""},
	} {
		require.Equal(t, want[0], NormalizeVideoResolution(raw), raw)
		require.Equal(t, want[1], VideoBillingTierForResolution(raw), raw)
	}
}

func TestParseOpenAIVideoJSON(t *testing.T) {
	req, err := ParseVideoTaskCreate(VideoShapeOpenAI, "application/json", []byte(
		`{"model":"sora-2","prompt":"waves","seconds":"8","size":"1280x720","input_reference":{"image_url":"https://example.com/a.png"}}`))
	require.NoError(t, err)
	require.Equal(t, "sora-2", req.Model)
	require.Equal(t, "waves", req.Prompt)
	require.Equal(t, 8, req.DurationSeconds)
	require.Equal(t, VideoResolution720P, req.Resolution)
	require.Equal(t, "16:9", req.AspectRatio)
	require.Equal(t, []VideoInputImage{{URL: "https://example.com/a.png", Role: VideoInputImageFirstFrame}}, req.Images)

	req, err = ParseVideoTaskCreate(VideoShapeOpenAI, "application/json", []byte(`{"model":"sora-2","prompt":"x","seconds":4,"input_reference":"https://example.com/b.png"}`))
	require.NoError(t, err)
	require.Equal(t, 4, req.DurationSeconds)
	require.Equal(t, "https://example.com/b.png", req.Images[0].URL)

	for name, body := range map[string]string{
		"file_id":        `{"model":"sora-2","prompt":"x","input_reference":{"file_id":"file_1"}}`,
		"bad size":       `{"model":"sora-2","prompt":"x","size":"big"}`,
		"too long":       `{"model":"sora-2","prompt":"x","seconds":"20"}`,
		"no model":       `{"prompt":"x"}`,
		"no prompt":      `{"model":"sora-2"}`,
		"bad image":      `{"model":"sora-2","prompt":"x","input_reference":"ftp://example.com/a.png"}`,
		"bad seconds":    `{"model":"sora-2","prompt":"x","seconds":"eight"}`,
		"not an object":  `[1,2]`,
		"bad resolution": `{"model":"sora-2","prompt":"x","resolution":"9000p"}`,
	} {
		_, err := ParseVideoTaskCreate(VideoShapeOpenAI, "application/json", []byte(body))
		var clientErr *VideoTaskClientError
		require.ErrorAs(t, err, &clientErr, name)
		require.Equal(t, 400, clientErr.StatusCode, name)
	}
}

func TestParseOpenAIVideoMultipart(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n0000000000")
	build := func(withFile bool) ([]byte, string) {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		_ = w.WriteField("model", "sora-2")
		_ = w.WriteField("prompt", "a cat")
		_ = w.WriteField("seconds", "12")
		_ = w.WriteField("size", "720x1280")
		if withFile {
			header := textproto.MIMEHeader{}
			header.Set("Content-Disposition", `form-data; name="input_reference"; filename="ref.png"`)
			header.Set("Content-Type", "image/png")
			part, _ := w.CreatePart(header)
			_, _ = part.Write(png)
		} else {
			_ = w.WriteField("input_reference", "https://example.com/ref.png")
		}
		_ = w.Close()
		return buf.Bytes(), w.FormDataContentType()
	}

	body, contentType := build(true)
	req, err := ParseVideoTaskCreate(VideoShapeOpenAI, contentType, body)
	require.NoError(t, err)
	require.Equal(t, "sora-2", req.Model)
	require.Equal(t, 12, req.DurationSeconds)
	require.Equal(t, VideoResolution720P, req.Resolution)
	require.Equal(t, "9:16", req.AspectRatio)
	require.Equal(t, "data:image/png;base64,"+base64.StdEncoding.EncodeToString(png), req.Images[0].URL)
	require.Equal(t, VideoInputImageFirstFrame, req.Images[0].Role)

	body, contentType = build(false)
	req, err = ParseVideoTaskCreate(VideoShapeOpenAI, contentType, body)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/ref.png", req.Images[0].URL)
}

func TestParseXAIVideoJSONKeepsVendorExtras(t *testing.T) {
	req, err := ParseVideoTaskCreate(VideoShapeXAI, "application/json", []byte(`{
		"model":"grok-imagine-video","prompt":"animate","image":{"image_url":"https://example.com/first.png"},
		"reference_images":[{"url":"https://example.com/r1.png"}],"duration":6,"aspect_ratio":"16:9","resolution":"720p",
		"camera_motion":"pan","n":1}`))
	require.NoError(t, err)
	require.Equal(t, 6, req.DurationSeconds)
	require.Equal(t, VideoResolution720P, req.Resolution)
	require.Equal(t, "16:9", req.AspectRatio)
	require.Equal(t, []VideoInputImage{
		{URL: "https://example.com/first.png", Role: VideoInputImageFirstFrame},
		{URL: "https://example.com/r1.png", Role: VideoInputImageReference},
	}, req.Images)
	require.Equal(t, map[string]any{"camera_motion": "pan"}, req.Extra)

	_, err = ParseVideoTaskCreate(VideoShapeXAI, "application/json", []byte(`{"model":"m","prompt":"x","n":2}`))
	require.Error(t, err)
}

func TestParseNewAPIVideoJSON(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n0000000000")
	req, err := ParseVideoTaskCreate(VideoShapeNewAPI, "application/json", []byte(`{
		"model":"kling-v1","prompt":"walk","image":"`+base64.StdEncoding.EncodeToString(png)+`","duration":5.0,
		"width":1280,"height":720,"seed":42,"metadata":{"negative_prompt":"blur","style":"anime","generate_audio":true}}`))
	require.NoError(t, err)
	require.Equal(t, 5, req.DurationSeconds)
	require.Equal(t, VideoResolution720P, req.Resolution)
	require.Equal(t, "16:9", req.AspectRatio)
	require.Equal(t, "blur", req.NegativePrompt)
	require.NotNil(t, req.Seed)
	require.Equal(t, int64(42), *req.Seed)
	require.NotNil(t, req.GenerateAudio)
	require.True(t, *req.GenerateAudio)
	require.Equal(t, map[string]any{"style": "anime"}, req.Extra)
	require.Equal(t, "data:image/png;base64,"+base64.StdEncoding.EncodeToString(png), req.Images[0].URL, "裸 base64 补成 data URL")

	req, err = ParseVideoTaskCreate(VideoShapeNewAPI, "application/json", []byte(`{"model":"m","prompt":"x","seconds":"10"}`))
	require.NoError(t, err)
	require.Equal(t, 10, req.DurationSeconds)
}

func videoTaskForRender(state VideoTaskState) *VideoTask {
	created := time.Unix(1_700_000_000, 0)
	return &VideoTask{
		ID: "vtask_abc", Model: "sora-2", State: state, CreatedAt: created, UpdatedAt: created.Add(time.Minute),
		Request: VideoTaskRequest{Model: "sora-2", Prompt: "waves", DurationSeconds: 8, Resolution: VideoResolution720P, AspectRatio: "16:9"},
	}
}

func videoRenderMap(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	require.True(t, ok, "%T", v)
	return m
}

func TestRenderVideoTaskStatusMappingPerShape(t *testing.T) {
	cases := []struct {
		state                          VideoTaskState
		openAI, xai, newAPI, newAPIDto string
	}{
		{VideoTaskQueued, "queued", "pending", "queued", "QUEUED"},
		{VideoTaskRunning, "in_progress", "pending", "in_progress", "IN_PROGRESS"},
		{VideoTaskSucceeded, "completed", "done", "completed", "SUCCESS"},
		{VideoTaskFailed, "failed", "failed", "failed", "FAILURE"},
		{VideoTaskCanceled, "failed", "failed", "failed", "FAILURE"},
	}
	for _, tc := range cases {
		task := videoTaskForRender(tc.state)
		openAI := videoRenderMap(t, RenderVideoTaskStatus(VideoShapeOpenAI, task, "/v1/videos/vtask_abc/content"))
		require.Equal(t, tc.openAI, openAI["status"], tc.state)
		require.Equal(t, "video", openAI["object"])
		require.Equal(t, "1280x720", openAI["size"])
		require.Equal(t, "8", openAI["seconds"])
		xai := videoRenderMap(t, RenderVideoTaskStatus(VideoShapeXAI, task, "/v1/videos/vtask_abc/content"))
		require.Equal(t, tc.xai, xai["status"], tc.state)
		newAPI := videoRenderMap(t, RenderVideoTaskStatus(VideoShapeNewAPI, task, "/v1/videos/vtask_abc/content"))
		require.Equal(t, tc.newAPI, newAPI["status"], tc.state)
		require.Equal(t, tc.newAPIDto, videoRenderMap(t, newAPI["data"])["status"], tc.state)
		if tc.state == VideoTaskSucceeded {
			require.Equal(t, 100, openAI["progress"])
			require.Equal(t, map[string]any{"url": "/v1/videos/vtask_abc/content", "duration": 8}, xai["video"])
			require.Equal(t, "/v1/videos/vtask_abc/content", newAPI["url"])
			require.Equal(t, "/v1/videos/vtask_abc/content", videoRenderMap(t, newAPI["data"])["result_url"])
		}
		if tc.state == VideoTaskFailed || tc.state == VideoTaskCanceled {
			require.NotNil(t, openAI["error"])
			require.NotNil(t, xai["error"])
		} else {
			require.Nil(t, openAI["error"])
		}
	}

	expired := videoTaskForRender(VideoTaskFailed)
	expired.ErrorCode = "expired"
	require.Equal(t, "expired", videoRenderMap(t, RenderVideoTaskStatus(VideoShapeXAI, expired, ""))["status"])

	created := videoTaskForRender(VideoTaskQueued)
	require.Equal(t, map[string]any{"request_id": "vtask_abc"}, RenderVideoTaskCreated(VideoShapeXAI, created))
	require.Equal(t, "vtask_abc", videoRenderMap(t, RenderVideoTaskCreated(VideoShapeNewAPI, created))["task_id"])
	require.Equal(t, "queued", videoRenderMap(t, RenderVideoTaskCreated(VideoShapeOpenAI, created))["status"])
}

func TestVideoStatusRenderShape(t *testing.T) {
	require.Equal(t, VideoShapeOpenAI, VideoStatusRenderShape(VideoShapeOpenAI, VideoShapeOpenAI))
	require.Equal(t, VideoShapeXAI, VideoStatusRenderShape(VideoShapeOpenAI, VideoShapeXAI), "/videos/{id} 是两家共用的路径，按创建形态")
	require.Equal(t, VideoShapeOpenAI, VideoStatusRenderShape(VideoShapeOpenAI, VideoShapeNewAPI))
	require.Equal(t, VideoShapeXAI, VideoStatusRenderShape(VideoShapeXAI, VideoShapeOpenAI), "跨家查询按路径的形态")
	require.Equal(t, VideoShapeNewAPI, VideoStatusRenderShape(VideoShapeNewAPI, VideoShapeXAI))
}

func TestVideoTaskBillingUnits(t *testing.T) {
	task := videoTaskForRender(VideoTaskSucceeded)
	require.Equal(t, 8, task.BillingDurationSeconds(), "上游没报时长按请求")
	require.Equal(t, VideoBillingResolution720P, task.BillingResolution())
	task.DurationSeconds = 5.6
	task.Resolution = VideoResolution1080P
	require.Equal(t, 6, task.BillingDurationSeconds(), "上游报的实际时长优先")
	require.Equal(t, VideoBillingResolution1080P, task.BillingResolution())
	result := task.UsageResult(task.CreatedAt.Add(90 * time.Second))
	require.Equal(t, 1, result.VideoCount)
	require.Equal(t, 6, result.VideoDurationSeconds)
	require.Equal(t, "vtask_abc", result.RequestID)
	require.Equal(t, 90*time.Second, result.Duration)
	require.Equal(t, "vtask_abc", StableGrokVideoBillingRequestID("vtask_abc"), "本网关的任务 ID 直接作为去重键")
}
