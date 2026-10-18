package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

// 客户端形态（入站适配器）：把各家创建请求解析成 VideoTaskRequest，把任务渲染成各家的创建 / 状态响应。
//
// OpenAI Videos API（platform.openai.com/docs/api-reference/videos，2026-09-24 起官方已下线 Sora 2，形态仍有大量客户端在用）：
//
//	POST /v1/videos  JSON 或 multipart：model, prompt, seconds（"4"/"8"/"12"）, size（"1280x720"…）, input_reference（文件或 {image_url}）
//	→ video 对象 {id, object:"video", model, status:queued|in_progress|completed|failed, progress, created_at, completed_at,
//	  expires_at, seconds, size, prompt, error, remixed_from_video_id}；GET /v1/videos/{id} 同一对象；/content 是 mp4。
//
// size ↔ 规范分辨率 + 宽高比（确定性）：短边定档（≤480 → 480p，≤720 → 720p，≤768 → 768p，≤1080 → 1080p，
// ≤1440 → 2k，其余 4k），宽高比吸附到最近的标准比例；反向按档的短边像素（480/720/768/1080/1440/2160）
// 与比例算长边（四舍五入到偶数）：720x1280 ↔ 720p 9:16，1280x720 ↔ 720p 16:9，1024x1792 → 1080p 9:16 → 1080x1920。

// videoTaskMaxRequestImages 一次请求最多带的输入图。
const videoTaskMaxRequestImages = 16

// ParseVideoTaskCreate 按入站形态解析创建请求；请求本身有问题时返回 *VideoTaskClientError。
func ParseVideoTaskCreate(shape VideoInboundShape, contentType string, body []byte) (VideoTaskRequest, error) {
	var req VideoTaskRequest
	var err error
	switch shape {
	case VideoShapeOpenAI:
		if isMultipartVideoRequest(contentType) {
			req, err = parseOpenAIVideoMultipart(contentType, body)
		} else {
			req, err = parseOpenAIVideoJSON(body)
		}
	case VideoShapeXAI:
		req, err = parseXAIVideoJSON(body)
	case VideoShapeNewAPI:
		req, err = parseNewAPIVideoJSON(body)
	default:
		return VideoTaskRequest{}, newVideoTaskInvalidRequest("unsupported video API shape")
	}
	if err != nil {
		return VideoTaskRequest{}, err
	}
	return req, validateVideoTaskRequest(req)
}

func validateVideoTaskRequest(req VideoTaskRequest) error {
	if req.Model == "" {
		return newVideoTaskInvalidRequest("model is required")
	}
	if req.Prompt == "" && len(req.Images) == 0 {
		return newVideoTaskInvalidRequest("prompt is required")
	}
	// 计费按秒且封顶 VideoBillingMaxDurationSeconds：放行更长的请求会让超出部分不计费。各适配器按模型再收窄。
	if req.DurationSeconds < 0 || req.DurationSeconds > VideoBillingMaxDurationSeconds {
		return newVideoTaskInvalidRequest("duration must be between %d and %d seconds", VideoBillingMinDurationSeconds, VideoBillingMaxDurationSeconds)
	}
	if len(req.Images) > videoTaskMaxRequestImages {
		return newVideoTaskInvalidRequest("a maximum of %d input images is supported", videoTaskMaxRequestImages)
	}
	for _, image := range req.Images {
		if !isValidVideoInputImageURL(image.URL) {
			return newVideoTaskInvalidRequest("input images must be http(s) URLs or base64 data URLs")
		}
	}
	return nil
}

func isMultipartVideoRequest(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	return err == nil && strings.EqualFold(mediaType, "multipart/form-data")
}

func isValidVideoInputImageURL(raw string) bool {
	if strings.HasPrefix(strings.ToLower(raw), "data:image/") {
		return strings.Contains(raw, ";base64,")
	}
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != ""
}

// normalizeVideoImageInput 图片输入：URL / data URL 原样；裸 base64 按内容嗅探补成 data URL。
func normalizeVideoImageInput(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || isValidVideoInputImageURL(raw) || strings.Contains(raw, "://") {
		return raw
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(decoded) == 0 {
		return raw
	}
	contentType := http.DetectContentType(decoded)
	if !strings.HasPrefix(contentType, "image/") {
		return raw
	}
	return "data:" + contentType + ";base64," + raw
}

// jsonVideoDuration 时长字段：数字或数字字符串；不存在返回 0，格式不对返回错误。
func jsonVideoDuration(value gjson.Result, field string) (int, error) {
	switch value.Type {
	case gjson.Null:
		return 0, nil
	case gjson.Number:
		return int(math.Round(value.Float())), nil
	case gjson.String:
		if strings.TrimSpace(value.String()) == "" {
			return 0, nil
		}
		seconds, err := strconv.ParseFloat(strings.TrimSpace(value.String()), 64)
		if err != nil {
			return 0, newVideoTaskInvalidRequest("%s must be a number of seconds", field)
		}
		return int(math.Round(seconds)), nil
	default:
		return 0, newVideoTaskInvalidRequest("%s must be a number of seconds", field)
	}
}

// applyVideoSize 把 "宽x高" 拆成分辨率与宽高比（客户端显式给了的不覆盖）。
func applyVideoSize(req *VideoTaskRequest, size string) error {
	size = strings.TrimSpace(size)
	if size == "" {
		return nil
	}
	width, height, ok := ParseVideoSize(size)
	if !ok {
		return newVideoTaskInvalidRequest("size must be formatted as WIDTHxHEIGHT")
	}
	if req.Resolution == "" {
		req.Resolution = videoResolutionForShortSide(min(width, height))
	}
	if req.AspectRatio == "" {
		req.AspectRatio = VideoAspectRatioForSize(width, height)
	}
	return nil
}

// applyCanonicalVideoOptions 各形态都接受的规范扩展字段（非 OpenAI 官方字段，方便客户端直接指定）。
func applyCanonicalVideoOptions(req *VideoTaskRequest, get func(string) gjson.Result) error {
	if v := strings.TrimSpace(get("negative_prompt").String()); v != "" {
		req.NegativePrompt = v
	}
	if v := strings.TrimSpace(get("aspect_ratio").String()); v != "" {
		req.AspectRatio = v
	}
	if v := strings.TrimSpace(get("resolution").String()); v != "" {
		req.Resolution = NormalizeVideoResolution(v)
		if req.Resolution == "" {
			return newVideoTaskInvalidRequest("unsupported resolution %q", v)
		}
	}
	if seed := get("seed"); seed.Type == gjson.Number {
		v := seed.Int()
		req.Seed = &v
	}
	if audio := get("generate_audio"); audio.IsBool() {
		v := audio.Bool()
		req.GenerateAudio = &v
	}
	return nil
}

func parseVideoJSONObject(body []byte) (gjson.Result, error) {
	if !gjson.ValidBytes(body) || !gjson.ParseBytes(body).IsObject() {
		return gjson.Result{}, newVideoTaskInvalidRequest("request body must be a JSON object")
	}
	return gjson.ParseBytes(body), nil
}

func parseOpenAIVideoJSON(body []byte) (VideoTaskRequest, error) {
	root, err := parseVideoJSONObject(body)
	if err != nil {
		return VideoTaskRequest{}, err
	}
	req := VideoTaskRequest{
		Model:  strings.TrimSpace(root.Get("model").String()),
		Prompt: strings.TrimSpace(root.Get("prompt").String()),
	}
	if req.DurationSeconds, err = jsonVideoDuration(root.Get("seconds"), "seconds"); err != nil {
		return VideoTaskRequest{}, err
	}
	if err := applyCanonicalVideoOptions(&req, root.Get); err != nil {
		return VideoTaskRequest{}, err
	}
	if err := applyVideoSize(&req, root.Get("size").String()); err != nil {
		return VideoTaskRequest{}, err
	}
	reference := root.Get("input_reference")
	switch {
	case !reference.Exists() || reference.Type == gjson.Null:
	case reference.Type == gjson.String:
		req.Images = append(req.Images, VideoInputImage{URL: strings.TrimSpace(reference.String()), Role: VideoInputImageFirstFrame})
	case reference.IsObject():
		if strings.TrimSpace(reference.Get("file_id").String()) != "" {
			return VideoTaskRequest{}, newVideoTaskInvalidRequest("input_reference.file_id is not supported; use image_url or upload the file")
		}
		imageURL := strings.TrimSpace(firstNonEmpty(reference.Get("image_url").String(), reference.Get("url").String()))
		if imageURL == "" {
			return VideoTaskRequest{}, newVideoTaskInvalidRequest("input_reference must provide image_url")
		}
		req.Images = append(req.Images, VideoInputImage{URL: imageURL, Role: VideoInputImageFirstFrame})
	default:
		return VideoTaskRequest{}, newVideoTaskInvalidRequest("input_reference must be an object with image_url")
	}
	return req, nil
}

func parseOpenAIVideoMultipart(contentType string, body []byte) (VideoTaskRequest, error) {
	_, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil || strings.TrimSpace(params["boundary"]) == "" {
		return VideoTaskRequest{}, newVideoTaskInvalidRequest("invalid multipart/form-data body")
	}
	fields := map[string]string{}
	var reference *VideoInputImage
	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return VideoTaskRequest{}, newVideoTaskInvalidRequest("invalid multipart/form-data body")
		}
		name := strings.TrimSpace(part.FormName())
		data, readErr := io.ReadAll(io.LimitReader(part, openAIImageMaxUploadPartSize+1))
		fileName := strings.TrimSpace(part.FileName())
		partContentType := strings.TrimSpace(part.Header.Get("Content-Type"))
		_ = part.Close()
		if readErr != nil {
			return VideoTaskRequest{}, newVideoTaskInvalidRequest("invalid multipart/form-data body")
		}
		if len(data) > openAIImageMaxUploadPartSize {
			return VideoTaskRequest{}, newVideoTaskInvalidRequest("multipart part %q exceeds %d bytes", name, openAIImageMaxUploadPartSize)
		}
		if name == "input_reference" && fileName != "" {
			dataURL, err := openAIImageUploadToDataURL(OpenAIImagesUpload{FieldName: name, FileName: fileName, ContentType: partContentType, Data: data})
			if err != nil {
				return VideoTaskRequest{}, newVideoTaskInvalidRequest("input_reference: %s", err.Error())
			}
			if !strings.HasPrefix(dataURL, "data:image/") {
				return VideoTaskRequest{}, newVideoTaskInvalidRequest("input_reference must be an image")
			}
			reference = &VideoInputImage{URL: dataURL, Role: VideoInputImageFirstFrame}
			continue
		}
		if name != "" && fileName == "" {
			fields[name] = strings.TrimSpace(string(data))
		}
	}
	req := VideoTaskRequest{Model: fields["model"], Prompt: fields["prompt"]}
	if raw := fields["seconds"]; raw != "" {
		if req.DurationSeconds, err = jsonVideoDuration(gjson.Result{Type: gjson.String, Str: raw}, "seconds"); err != nil {
			return VideoTaskRequest{}, err
		}
	}
	get := func(key string) gjson.Result {
		value, ok := fields[key]
		if !ok {
			return gjson.Result{}
		}
		if key == "seed" || key == "generate_audio" {
			return gjson.Parse(value)
		}
		return gjson.Result{Type: gjson.String, Str: value}
	}
	if err := applyCanonicalVideoOptions(&req, get); err != nil {
		return VideoTaskRequest{}, err
	}
	if err := applyVideoSize(&req, fields["size"]); err != nil {
		return VideoTaskRequest{}, err
	}
	switch {
	case reference != nil:
		req.Images = append(req.Images, *reference)
	case fields["input_reference"] != "":
		req.Images = append(req.Images, VideoInputImage{URL: fields["input_reference"], Role: VideoInputImageFirstFrame})
	}
	return req, nil
}

// xaiVideoKnownFields xAI 形态里映射到规范字段的键；其余键作为厂商私有参数原样透传（Extra）。
var xaiVideoKnownFields = map[string]bool{
	"model": true, "prompt": true, "image": true, "image_url": true, "images": true, "reference_images": true,
	"duration": true, "aspect_ratio": true, "resolution": true, "negative_prompt": true, "seed": true,
	"generate_audio": true, "n": true, "user": true, "stream": true,
}

func parseXAIVideoJSON(body []byte) (VideoTaskRequest, error) {
	root, err := parseVideoJSONObject(body)
	if err != nil {
		return VideoTaskRequest{}, err
	}
	req := VideoTaskRequest{
		Model:  strings.TrimSpace(root.Get("model").String()),
		Prompt: strings.TrimSpace(root.Get("prompt").String()),
	}
	if n := root.Get("n"); n.Exists() && n.Int() > 1 {
		return VideoTaskRequest{}, newVideoTaskInvalidRequest("n > 1 is not supported for video generation")
	}
	if req.DurationSeconds, err = jsonVideoDuration(root.Get("duration"), "duration"); err != nil {
		return VideoTaskRequest{}, err
	}
	if err := applyCanonicalVideoOptions(&req, root.Get); err != nil {
		return VideoTaskRequest{}, err
	}
	if first := extractGrokMediaImageURL(root.Get("image")); first != "" {
		req.Images = append(req.Images, VideoInputImage{URL: first, Role: VideoInputImageFirstFrame})
	} else if first := strings.TrimSpace(root.Get("image_url").String()); first != "" {
		req.Images = append(req.Images, VideoInputImage{URL: first, Role: VideoInputImageFirstFrame})
	}
	for _, field := range []string{"images", "reference_images"} {
		for _, item := range root.Get(field).Array() {
			if imageURL := extractGrokMediaImageURL(item); imageURL != "" {
				req.Images = append(req.Images, VideoInputImage{URL: imageURL, Role: VideoInputImageReference})
			}
		}
	}
	root.ForEach(func(key, value gjson.Result) bool {
		if !xaiVideoKnownFields[key.String()] {
			if req.Extra == nil {
				req.Extra = map[string]any{}
			}
			req.Extra[key.String()] = value.Value()
		}
		return true
	})
	return req, nil
}

// newAPIMetadataCanonicalFields new-api metadata 里映射到规范字段的键，不再进 Extra。
var newAPIMetadataCanonicalFields = map[string]bool{
	"negative_prompt": true, "aspect_ratio": true, "resolution": true, "seed": true, "generate_audio": true,
}

func parseNewAPIVideoJSON(body []byte) (VideoTaskRequest, error) {
	root, err := parseVideoJSONObject(body)
	if err != nil {
		return VideoTaskRequest{}, err
	}
	req := VideoTaskRequest{
		Model:  strings.TrimSpace(root.Get("model").String()),
		Prompt: strings.TrimSpace(root.Get("prompt").String()),
	}
	if n := root.Get("n"); n.Exists() && n.Int() > 1 {
		return VideoTaskRequest{}, newVideoTaskInvalidRequest("n > 1 is not supported for video generation")
	}
	if req.DurationSeconds, err = jsonVideoDuration(root.Get("duration"), "duration"); err != nil {
		return VideoTaskRequest{}, err
	}
	if req.DurationSeconds == 0 {
		if req.DurationSeconds, err = jsonVideoDuration(root.Get("seconds"), "seconds"); err != nil {
			return VideoTaskRequest{}, err
		}
	}
	metadata := root.Get("metadata")
	if metadata.Exists() && metadata.Type != gjson.Null && !metadata.IsObject() {
		return VideoTaskRequest{}, newVideoTaskInvalidRequest("metadata must be an object")
	}
	lookup := func(key string) gjson.Result {
		if value := root.Get(key); value.Exists() {
			return value
		}
		return metadata.Get(key)
	}
	if err := applyCanonicalVideoOptions(&req, lookup); err != nil {
		return VideoTaskRequest{}, err
	}
	width, height := root.Get("width").Int(), root.Get("height").Int()
	if width > 0 && height > 0 {
		if err := applyVideoSize(&req, strconv.FormatInt(width, 10)+"x"+strconv.FormatInt(height, 10)); err != nil {
			return VideoTaskRequest{}, err
		}
	} else if err := applyVideoSize(&req, root.Get("size").String()); err != nil {
		return VideoTaskRequest{}, err
	}
	if first := normalizeVideoImageInput(extractGrokMediaImageURL(root.Get("image"))); first != "" {
		req.Images = append(req.Images, VideoInputImage{URL: first, Role: VideoInputImageFirstFrame})
	}
	for _, item := range root.Get("images").Array() {
		if imageURL := normalizeVideoImageInput(extractGrokMediaImageURL(item)); imageURL != "" {
			req.Images = append(req.Images, VideoInputImage{URL: imageURL, Role: VideoInputImageReference})
		}
	}
	metadata.ForEach(func(key, value gjson.Result) bool {
		if !newAPIMetadataCanonicalFields[key.String()] {
			if req.Extra == nil {
				req.Extra = map[string]any{}
			}
			req.Extra[key.String()] = value.Value()
		}
		return true
	})
	return req, nil
}

// VideoStatusRenderShape 状态查询按哪种形态回包：/v1/videos/{id} 是 OpenAI 与 xAI 共用的路径，按任务创建时的形态
// （创建于 new-api 形态的按 OpenAI）；/v1/videos/generations/{id} 恒为 xAI，/v1/video/generations/{id} 恒为 new-api。
func VideoStatusRenderShape(pathShape, createdShape VideoInboundShape) VideoInboundShape {
	if pathShape == VideoShapeOpenAI && createdShape == VideoShapeXAI {
		return VideoShapeXAI
	}
	return pathShape
}

// RenderVideoTaskCreated 创建成功的响应体。
func RenderVideoTaskCreated(shape VideoInboundShape, task *VideoTask) any {
	switch shape {
	case VideoShapeXAI:
		return map[string]any{"request_id": task.ID}
	case VideoShapeNewAPI:
		return map[string]any{
			"id": task.ID, "task_id": task.ID, "status": newAPIVideoStatus(task.State),
			"model": task.Model, "created_at": task.CreatedAt.Unix(),
		}
	default:
		return openAIVideoObject(task)
	}
}

// RenderVideoTaskStatus 状态响应体；contentURL 是网关自己的成片下载地址（上游地址不外露）。
func RenderVideoTaskStatus(shape VideoInboundShape, task *VideoTask, contentURL string) any {
	switch shape {
	case VideoShapeXAI:
		return xaiVideoStatusObject(task, contentURL)
	case VideoShapeNewAPI:
		return newAPIVideoStatusObject(task, contentURL)
	default:
		return openAIVideoObject(task)
	}
}

// videoTaskDisplaySeconds 对外展示的时长：上游报的优先，没有按请求的；都没有为 0。
func videoTaskDisplaySeconds(task *VideoTask) int {
	if task.DurationSeconds > 0 {
		return int(math.Round(task.DurationSeconds))
	}
	return task.Request.DurationSeconds
}

func videoTaskProgress(task *VideoTask) int {
	if task.State == VideoTaskSucceeded {
		return 100
	}
	if task.Progress != nil {
		return *task.Progress
	}
	return 0
}

func videoTaskErrorCode(task *VideoTask) string {
	if task.ErrorCode != "" {
		return task.ErrorCode
	}
	if task.State == VideoTaskCanceled {
		return "canceled"
	}
	return "video_generation_failed"
}

func videoTaskErrorMessage(task *VideoTask) string {
	if task.ErrorMessage != "" {
		return task.ErrorMessage
	}
	if task.State == VideoTaskCanceled {
		return "video generation was canceled"
	}
	return "video generation failed"
}

// openAIVideoStatus 规范状态 → OpenAI 状态（取消没有对应值，记 failed）。
func openAIVideoStatus(state VideoTaskState) string {
	switch state {
	case VideoTaskRunning:
		return "in_progress"
	case VideoTaskSucceeded:
		return "completed"
	case VideoTaskFailed, VideoTaskCanceled:
		return "failed"
	default:
		return "queued"
	}
}

func openAIVideoObject(task *VideoTask) map[string]any {
	resolution := task.Resolution
	if resolution == "" {
		resolution = task.Request.Resolution
	}
	seconds := ""
	if s := videoTaskDisplaySeconds(task); s > 0 {
		seconds = strconv.Itoa(s)
	}
	var prompt any
	if task.Request.Prompt != "" {
		prompt = task.Request.Prompt
	}
	obj := map[string]any{
		"id": task.ID, "object": "video", "model": task.Model,
		"status": openAIVideoStatus(task.State), "progress": videoTaskProgress(task),
		"created_at": task.CreatedAt.Unix(), "completed_at": nil, "expires_at": nil,
		"seconds": seconds, "size": VideoSizeFor(resolution, task.Request.AspectRatio),
		"prompt": prompt, "error": nil, "remixed_from_video_id": nil,
	}
	if task.State.IsTerminal() {
		obj["completed_at"] = task.UpdatedAt.Unix()
	}
	if task.State == VideoTaskFailed || task.State == VideoTaskCanceled {
		obj["error"] = map[string]any{"code": videoTaskErrorCode(task), "message": videoTaskErrorMessage(task)}
	}
	return obj
}

// xaiVideoStatus 规范状态 → xAI 状态（pending / done / failed / expired）。
func xaiVideoStatus(task *VideoTask) string {
	switch task.State {
	case VideoTaskSucceeded:
		return "done"
	case VideoTaskFailed, VideoTaskCanceled:
		if task.ErrorCode == "expired" {
			return "expired"
		}
		return "failed"
	default:
		return "pending"
	}
}

func xaiVideoStatusObject(task *VideoTask, contentURL string) map[string]any {
	obj := map[string]any{"request_id": task.ID, "status": xaiVideoStatus(task), "model": task.Model}
	if task.Progress != nil || task.State == VideoTaskSucceeded {
		obj["progress"] = videoTaskProgress(task)
	}
	switch task.State {
	case VideoTaskSucceeded:
		video := map[string]any{"url": contentURL}
		if seconds := videoTaskDisplaySeconds(task); seconds > 0 {
			video["duration"] = seconds
		}
		obj["video"] = video
	case VideoTaskFailed, VideoTaskCanceled:
		obj["error"] = map[string]any{"code": videoTaskErrorCode(task), "message": videoTaskErrorMessage(task)}
	}
	return obj
}

// newAPIVideoStatus 规范状态 → new-api 文档里的状态（queued / in_progress / completed / failed）。
func newAPIVideoStatus(state VideoTaskState) string {
	return openAIVideoStatus(state)
}

// newAPITaskDtoStatus 规范状态 → new-api 源码 TaskDto 的状态（QUEUED / IN_PROGRESS / SUCCESS / FAILURE）。
func newAPITaskDtoStatus(state VideoTaskState) string {
	switch state {
	case VideoTaskRunning:
		return "IN_PROGRESS"
	case VideoTaskSucceeded:
		return "SUCCESS"
	case VideoTaskFailed, VideoTaskCanceled:
		return "FAILURE"
	default:
		return "QUEUED"
	}
}

// newAPIVideoStatusObject 顶层按 new-api 文档的形态；另带源码实际返回的 code / data（TaskDto 子集），
// 两种客户端解析口径都能读到状态与地址。
func newAPIVideoStatusObject(task *VideoTask, contentURL string) map[string]any {
	obj := map[string]any{"task_id": task.ID, "status": newAPIVideoStatus(task.State)}
	data := map[string]any{
		"task_id": task.ID, "status": newAPITaskDtoStatus(task.State),
		"progress": strconv.Itoa(videoTaskProgress(task)) + "%", "created_at": task.CreatedAt.Unix(),
	}
	switch task.State {
	case VideoTaskSucceeded:
		obj["url"], obj["format"] = contentURL, "mp4"
		data["result_url"] = contentURL
		if seconds := videoTaskDisplaySeconds(task); seconds > 0 {
			obj["metadata"] = map[string]any{"duration": seconds}
		}
	case VideoTaskFailed, VideoTaskCanceled:
		obj["error"] = map[string]any{"code": 0, "message": videoTaskErrorMessage(task)}
		data["fail_reason"] = videoTaskErrorMessage(task)
	}
	obj["code"], obj["data"] = "success", data
	return obj
}

// MarshalVideoTaskRequest 规范请求的存储格式（video_tasks.request）。
func MarshalVideoTaskRequest(req VideoTaskRequest) ([]byte, error) {
	return json.Marshal(req)
}

// UnmarshalVideoTaskRequest 读回存储的规范请求；数字按 json.Number 保留精度（Extra 原样回传上游）。
func UnmarshalVideoTaskRequest(raw []byte) (VideoTaskRequest, error) {
	var req VideoTaskRequest
	if len(raw) == 0 {
		return req, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	err := decoder.Decode(&req)
	return req, err
}
