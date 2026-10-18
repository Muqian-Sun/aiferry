package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// xAI 视频 API（docs.x.ai「Video Generation」）：
//
//	POST /v1/videos/generations  {"model","prompt","image":{"url"},"duration","aspect_ratio","resolution"} → {"request_id"}
//	GET  /v1/videos/{request_id} → {"status":"pending|done|failed|expired","model","video":{"url","duration"}}
//
// Grok 成品号走 xAI 原生端点（buildGrokMediaURL）；xAI 形态的中转 key 走 {chat_completions 根}/v1/videos/*。
const xaiVideoGenerationsEndpoint = "/v1/videos/generations"

// buildXAIVideoCreateBody 规范请求 → xAI 请求体。首帧放 image、参考图放 reference_images；xAI 没有尾帧，
// negative_prompt / seed / generate_audio 也没有对应字段，不发（避免上游按未知字段拒绝）。
func buildXAIVideoCreateBody(req VideoTaskRequest, upstreamModel string) (map[string]any, error) {
	if len(req.ImagesWithRole(VideoInputImageLastFrame)) > 0 {
		return nil, newVideoTaskInvalidRequest("last_frame images are not supported by this video model")
	}
	first := req.ImagesWithRole(VideoInputImageFirstFrame)
	if len(first) > 1 {
		return nil, newVideoTaskInvalidRequest("only one first_frame image is supported")
	}
	body := map[string]any{"model": upstreamModel}
	if req.Prompt != "" {
		body["prompt"] = req.Prompt
	}
	if len(first) == 1 {
		body["image"] = map[string]any{"url": first[0].URL}
	}
	if refs := req.ImagesWithRole(VideoInputImageReference); len(refs) > 0 {
		items := make([]map[string]any, 0, len(refs))
		for _, ref := range refs {
			items = append(items, map[string]any{"url": ref.URL})
		}
		body["reference_images"] = items
	}
	if req.DurationSeconds > 0 {
		body["duration"] = req.DurationSeconds
	}
	if req.AspectRatio != "" {
		body["aspect_ratio"] = req.AspectRatio
	}
	if req.Resolution != "" {
		body["resolution"] = req.Resolution
	}
	mergeVideoExtra(body, req.Extra)
	return body, nil
}

// parseXAIVideoStatus xAI 状态体 → 规范结果。done 却没有 video.url 时不算完成（老路径同样不计费），继续查。
func parseXAIVideoStatus(body []byte) (VideoTaskResult, error) {
	if !gjson.ValidBytes(body) {
		return VideoTaskResult{}, fmt.Errorf("xai video status is not valid json")
	}
	status := strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "status").String()))
	result := VideoTaskResult{
		State:    parseVideoTaskState(status),
		Progress: parseVideoProgress(gjson.GetBytes(body, "progress")),
		VideoURL: strings.TrimSpace(gjson.GetBytes(body, "video.url").String()),
	}
	if result.State == VideoTaskQueued && result.Progress != nil && *result.Progress > 0 {
		result.State = VideoTaskRunning
	}
	if duration := gjson.GetBytes(body, "video.duration"); duration.Type == gjson.Number {
		result.DurationSeconds = duration.Float()
	}
	result.Resolution = NormalizeVideoResolution(gjson.GetBytes(body, "video.resolution").String())
	if result.State == VideoTaskSucceeded && result.VideoURL == "" {
		result.State = VideoTaskRunning
	}
	if result.State == VideoTaskFailed {
		result.ErrorCode, result.ErrorMessage = videoUpstreamErrorFields(body, "")
		if result.ErrorCode == "" {
			result.ErrorCode = status
		}
	}
	return result, nil
}

// extractVideoUpstreamTaskID 创建响应里的上游任务 ID；不能作为 URL 路径段的 ID 当作无效。
func extractVideoUpstreamTaskID(body []byte) (string, error) {
	id := extractGrokMediaVideoRequestID(body)
	if id == "" {
		return "", &videoUpstreamHTTPError{StatusCode: http.StatusBadGateway, Body: body, Message: "upstream returned no video task id"}
	}
	if len(id) > 255 || strings.ContainsAny(id, "/?#% \t\r\n") {
		return "", &videoUpstreamHTTPError{StatusCode: http.StatusBadGateway, Body: body, Message: "upstream returned an invalid video task id"}
	}
	return id, nil
}

type grokOAuthVideoUpstream struct{ s *OpenAIGatewayService }

func (u grokOAuthVideoUpstream) Kind() VideoUpstreamKind  { return VideoUpstreamGrokOAuth }
func (u grokOAuthVideoUpstream) UpstreamEndpoint() string { return xaiVideoGenerationsEndpoint }

// applyAuth 成品号的认证与 CLI 网关头；账号级请求头覆写最后应用。
func (u grokOAuthVideoUpstream) applyAuth(req *http.Request, account *Account, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
	if account.IsGrokOAuth() && isGrokCLIProxyTarget(req.URL.String()) {
		applyGrokCLIHeaders(req.Header)
	}
	account.ApplyHeaderOverrides(req.Header)
}

func (u grokOAuthVideoUpstream) Create(ctx context.Context, c *gin.Context, account *Account, req VideoTaskRequest, upstreamModel string) (string, error) {
	if account.Vendor() != PlatformGrok {
		return "", fmt.Errorf("account %d is not a grok account", account.ID)
	}
	token, _, err := u.s.getRequestCredential(ctx, c, account)
	if err != nil {
		return "", err
	}
	targetURL, err := buildGrokMediaURL(account, u.s.cfg, GrokMediaEndpointVideosGenerations, "")
	if err != nil {
		return "", err
	}
	model := NormalizeGrokMediaModelForEndpoint(GrokMediaEndpointVideosGenerations, upstreamModel, len(req.Images) > 0)
	payload, err := buildXAIVideoCreateBody(req, model)
	if err != nil {
		return "", err
	}
	upstreamCtx, release := detachUpstreamContext(ctx)
	defer release()
	httpReq, err := newVideoUpstreamJSONRequest(upstreamCtx, http.MethodPost, targetURL, payload)
	if err != nil {
		return "", err
	}
	u.applyAuth(httpReq, account, token)
	body, headers, err := u.s.doVideoUpstreamJSON(httpReq, account)
	if err != nil {
		return "", err
	}
	u.s.updateGrokUsageFromResponse(withGrokTeamRateLimitModel(ctx, model), account, headers, http.StatusOK)
	return extractVideoUpstreamTaskID(body)
}

func (u grokOAuthVideoUpstream) Query(ctx context.Context, c *gin.Context, account *Account, upstreamTaskID string) (VideoTaskResult, error) {
	token, _, err := u.s.getRequestCredential(ctx, c, account)
	if err != nil {
		return VideoTaskResult{}, err
	}
	targetURL, err := buildGrokMediaURL(account, u.s.cfg, GrokMediaEndpointVideoStatus, upstreamTaskID)
	if err != nil {
		return VideoTaskResult{}, err
	}
	httpReq, err := newVideoUpstreamJSONRequest(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return VideoTaskResult{}, err
	}
	u.applyAuth(httpReq, account, token)
	body, _, err := u.s.doVideoUpstreamJSON(httpReq, account)
	if err != nil {
		return VideoTaskResult{}, err
	}
	return parseXAIVideoStatus(body)
}

// OpenContent xAI 给的是 vidgen.x.ai 签名地址时直接下载（不带凭据）；否则走带凭据的 /videos/{id}/content。
func (u grokOAuthVideoUpstream) OpenContent(ctx context.Context, c *gin.Context, account *Account, upstreamTaskID string, result VideoTaskResult) (*http.Response, error) {
	contentURL, err := grokSignedVideoURL(result.VideoURL, upstreamTaskID)
	if err != nil {
		return nil, err
	}
	signed := contentURL != ""
	if !signed {
		if contentURL, err = buildGrokMediaURL(account, u.s.cfg, GrokMediaEndpointVideoContent, upstreamTaskID); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(WithHTTPUpstreamRedirectsDisabled(ctx), http.MethodGet, contentURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	copyVideoRangeHeader(c, req)
	if !signed {
		token, _, err := u.s.getRequestCredential(ctx, c, account)
		if err != nil {
			return nil, err
		}
		u.applyAuth(req, account, token)
	}
	return u.s.doVideoContentRequest(req, account)
}

type xaiRelayVideoUpstream struct{ s *OpenAIGatewayService }

func (u xaiRelayVideoUpstream) Kind() VideoUpstreamKind  { return VideoUpstreamXAIRelay }
func (u xaiRelayVideoUpstream) UpstreamEndpoint() string { return xaiVideoGenerationsEndpoint }

func (u xaiRelayVideoUpstream) Create(ctx context.Context, _ *gin.Context, account *Account, req VideoTaskRequest, upstreamModel string) (string, error) {
	root, err := u.s.keyVideoBaseURL(account)
	if err != nil {
		return "", err
	}
	payload, err := buildXAIVideoCreateBody(req, upstreamModel)
	if err != nil {
		return "", err
	}
	upstreamCtx, release := detachUpstreamContext(ctx)
	defer release()
	httpReq, err := newVideoUpstreamJSONRequest(WithHTTPUpstreamProfile(upstreamCtx, HTTPUpstreamProfileOpenAI), http.MethodPost,
		joinUpstreamEndpointURL(root, xaiVideoGenerationsEndpoint), payload)
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

func (u xaiRelayVideoUpstream) Query(ctx context.Context, _ *gin.Context, account *Account, upstreamTaskID string) (VideoTaskResult, error) {
	root, err := u.s.keyVideoBaseURL(account)
	if err != nil {
		return VideoTaskResult{}, err
	}
	httpReq, err := newVideoUpstreamJSONRequest(WithHTTPUpstreamProfile(ctx, HTTPUpstreamProfileOpenAI), http.MethodGet,
		joinUpstreamEndpointURL(root, "/v1/videos/"+upstreamTaskID), nil)
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
	return parseXAIVideoStatus(body)
}

func (u xaiRelayVideoUpstream) OpenContent(ctx context.Context, c *gin.Context, account *Account, upstreamTaskID string, result VideoTaskResult) (*http.Response, error) {
	contentURL := result.VideoURL
	if contentURL == "" {
		contentURL = "/v1/videos/" + upstreamTaskID + "/content"
	}
	return u.s.openKeyVideoContent(ctx, c, account, contentURL)
}
