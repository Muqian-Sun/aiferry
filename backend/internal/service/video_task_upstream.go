package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// VideoTaskUpstream 上游适配器：把规范请求提交到一种上游、把上游任务状态转回规范结果、取成片内容。
// c 只用于记录运维与调度轨迹需要的上游错误事件，适配器不写客户端响应。
type VideoTaskUpstream interface {
	Kind() VideoUpstreamKind
	// UpstreamEndpoint 用量记录里的上游端点。
	UpstreamEndpoint() string
	Create(ctx context.Context, c *gin.Context, account *Account, req VideoTaskRequest, upstreamModel string) (upstreamTaskID string, err error)
	Query(ctx context.Context, c *gin.Context, account *Account, upstreamTaskID string) (VideoTaskResult, error)
	// OpenContent 发起成片下载请求；result 是刚查到的成功状态（含上游给的视频地址）。
	OpenContent(ctx context.Context, c *gin.Context, account *Account, upstreamTaskID string, result VideoTaskResult) (*http.Response, error)
}

// 账号形态：上游适配器按（条目厂商, 账号形态）选。
const (
	videoAccountKindGrokOAuth = "grok_oauth"
	// videoAccountKindRelayKey 通用中转 key（地址不是任何已知厂商的官方域名）。
	videoAccountKindRelayKey = "relay_key"
	// videoAccountKindVendorKeyPrefix 指向厂商官方域名的 key，后接厂商（vendor_key:minimax）。
	videoAccountKindVendorKeyPrefix = "vendor_key:"
)

// videoTaskAnyVendor 注册表里匹配任意条目厂商的通配。
const videoTaskAnyVendor = "*"

type videoTaskRouteKey struct {
	vendor      string
	accountKind string
}

// videoTaskRoutes 上游适配器注册表：（条目厂商, 账号形态）→ 适配器。先查精确厂商，再查通配。
// 官方厂商适配器按 {厂商, vendor_key:厂商} 一行，并在 NewVideoTaskService 里注册实现。
var videoTaskRoutes = map[videoTaskRouteKey]VideoUpstreamKind{
	{PlatformGrok, videoAccountKindGrokOAuth}:                              VideoUpstreamGrokOAuth,
	{PlatformGrok, videoAccountKindRelayKey}:                               VideoUpstreamXAIRelay,
	{videoTaskAnyVendor, videoAccountKindRelayKey}:                         VideoUpstreamNewAPIRelay,
	{PlatformSeedance, videoAccountKindVendorKeyPrefix + PlatformSeedance}: VideoUpstreamSeedanceOfficial,
	{PlatformKling, videoAccountKindVendorKeyPrefix + PlatformKling}:       VideoUpstreamKlingOfficial,
	{PlatformMiniMax, videoAccountKindVendorKeyPrefix + PlatformMiniMax}:   VideoUpstreamMiniMaxOfficial,
}

// videoAccountKind 账号在视频注册表里的形态；没有视频能力的形态返回空串。
// 第三方 key：配了 video 地址的按地址认出的厂商（认不出没有形态）；指向厂商官方域名的对话 key 没有视频形态；
// 其余是通用中转。
func videoAccountKind(account *Account) string {
	if account == nil {
		return ""
	}
	if account.IsThirdPartyKey() {
		vendor := account.Vendor()
		if account.ProtocolEndpoint(APIProtocolVideo) != "" {
			if vendor == "" {
				return ""
			}
			return videoAccountKindVendorKeyPrefix + vendor
		}
		if vendor != "" {
			return ""
		}
		return videoAccountKindRelayKey
	}
	if account.Vendor() == PlatformGrok && account.Type == AccountTypeOAuth {
		return videoAccountKindGrokOAuth
	}
	return ""
}

// VideoTaskVendorFor 条目在视频注册表里的厂商键：认得出的厂商用平台常量（xai → grok），
// 其余用条目厂商原文（小写），没有厂商时为空（只能落到通配）。
func VideoTaskVendorFor(entry *ModelCatalogEntry) string {
	if platform := CatalogVendorPlatform(entry); platform != "" {
		return platform
	}
	if entry == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(entry.Vendor))
}

// VideoTaskUpstreamKindFor 选（条目厂商, 账号）对应的上游适配器；没有时 ok=false，调度据此排除该账号。
func VideoTaskUpstreamKindFor(vendor string, account *Account) (VideoUpstreamKind, bool) {
	accountKind := videoAccountKind(account)
	if accountKind == "" {
		return "", false
	}
	if kind, ok := videoTaskRoutes[videoTaskRouteKey{vendor, accountKind}]; ok && vendor != "" {
		return kind, true
	}
	kind, ok := videoTaskRoutes[videoTaskRouteKey{videoTaskAnyVendor, accountKind}]
	return kind, ok
}

// videoUpstreamHTTPError 上游回了非 2xx。由服务层按账号形态决定 failover 与账号副作用。
type videoUpstreamHTTPError struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
	Message    string
	URL        string
	RequestID  string
}

func (e *videoUpstreamHTTPError) Error() string {
	return fmt.Sprintf("video upstream returned status %d: %s", e.StatusCode, e.Message)
}

// videoUpstreamTransportError 请求没拿到 HTTP 响应（代理 / DNS / TCP / TLS）。
type videoUpstreamTransportError struct {
	Err error
	URL string
}

func (e *videoUpstreamTransportError) Error() string { return e.Err.Error() }
func (e *videoUpstreamTransportError) Unwrap() error { return e.Err }

// VideoTaskClientError 不换号、直接回给客户端的错误（请求本身有问题，或上游明确拒绝）。
type VideoTaskClientError struct {
	StatusCode int
	Type       string
	Message    string
}

func (e *VideoTaskClientError) Error() string { return e.Message }

func newVideoTaskInvalidRequest(format string, args ...any) *VideoTaskClientError {
	return &VideoTaskClientError{StatusCode: http.StatusBadRequest, Type: "invalid_request_error", Message: fmt.Sprintf(format, args...)}
}

// errVideoTaskNoVideoURL 上游报成功却没给视频地址：当作未完成，等下一次查询。
var errVideoTaskNoVideoURL = errors.New("video task has no downloadable url yet")

// videoTaskStatusBodyLimit 状态响应体的读取上限（状态是小 JSON，留足余量）。
const videoTaskStatusBodyLimit = 4 << 20

// doVideoUpstreamJSON 发一个 JSON 请求并读回 2xx 响应体；非 2xx 返回 videoUpstreamHTTPError，
// 没拿到响应返回 videoUpstreamTransportError。
func (s *OpenAIGatewayService) doVideoUpstreamJSON(req *http.Request, account *Account) ([]byte, http.Header, error) {
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return nil, nil, &videoUpstreamTransportError{Err: err, URL: safeUpstreamURL(req.URL.String())}
	}
	defer func() { _ = resp.Body.Close() }()
	requestID := firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("xai-request-id"), resp.Header.Get("request-id"))
	if resp.StatusCode >= 300 {
		body := s.readUpstreamErrorBody(resp)
		message := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(body)))
		if message == "" {
			message = fmt.Sprintf("upstream returned status %d", resp.StatusCode)
		}
		return nil, nil, &videoUpstreamHTTPError{
			StatusCode: resp.StatusCode, Body: body, Headers: resp.Header.Clone(),
			Message: message, URL: safeUpstreamURL(req.URL.String()), RequestID: requestID,
		}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, videoTaskStatusBodyLimit))
	if err != nil {
		return nil, nil, &videoUpstreamTransportError{Err: err, URL: safeUpstreamURL(req.URL.String())}
	}
	return body, resp.Header.Clone(), nil
}

func newVideoUpstreamJSONRequest(ctx context.Context, method, targetURL string, payload any) (*http.Request, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := marshalOpenAIUpstreamJSON(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, targetURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// mergeVideoExtra 把 Extra 并进上游请求体，规范字段已写的键不被覆盖。
func mergeVideoExtra(body map[string]any, extra map[string]any) {
	for key, value := range extra {
		if _, exists := body[key]; !exists {
			body[key] = value
		}
	}
}

// parseVideoTaskState 把各家写法的任务状态归一；认不出的当作仍在进行（继续查，不误判终态）。
func parseVideoTaskState(raw string) VideoTaskState {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "queued", "pending", "submitted", "not_start", "not_started", "waiting", "created":
		return VideoTaskQueued
	case "succeeded", "success", "successful", "completed", "complete", "done", "finished":
		return VideoTaskSucceeded
	case "failed", "failure", "error", "expired", "rejected":
		return VideoTaskFailed
	case "canceled", "cancelled":
		return VideoTaskCanceled
	default:
		return VideoTaskRunning
	}
}

// parseVideoProgress 进度：百分数（45 或 "45%"），小于 1 的小数按比例（0.45）；夹到 0..100，没有返回 nil。
func parseVideoProgress(value gjson.Result) *int {
	var progress float64
	switch value.Type {
	case gjson.Number:
		progress = value.Float()
		if progress > 0 && progress < 1 {
			progress *= 100
		}
	case gjson.String:
		parsed, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(value.String()), "%"), 64)
		if err != nil {
			return nil
		}
		progress = parsed
	default:
		return nil
	}
	out := int(math.Max(0, math.Min(100, progress)))
	return &out
}

// firstGJSONString 依次取第一个非空字符串字段。
func firstGJSONString(body []byte, paths ...string) string {
	for _, path := range paths {
		if value := gjson.GetBytes(body, path); value.Type == gjson.String {
			if s := strings.TrimSpace(value.String()); s != "" {
				return s
			}
		}
	}
	return ""
}

// videoUpstreamErrorFields 状态体里的失败原因：error 可能是字符串或 {code,message}，也可能在 fail_reason 里。
func videoUpstreamErrorFields(body []byte, root string) (code, message string) {
	prefix := ""
	if root != "" {
		prefix = root + "."
	}
	errValue := gjson.GetBytes(body, prefix+"error")
	switch {
	case errValue.Type == gjson.String:
		message = errValue.String()
	case errValue.IsObject():
		message = errValue.Get("message").String()
		code = errValue.Get("code").String()
		if code == "" {
			code = errValue.Get("type").String()
		}
	}
	if message == "" {
		message = firstGJSONString(body, prefix+"fail_reason", prefix+"failure_reason", prefix+"message")
	}
	return strings.TrimSpace(code), sanitizeUpstreamErrorMessage(strings.TrimSpace(message))
}

// keyVideoBaseURL 第三方 key 的视频端点根地址：与图片 / 向量同一个扩展端点根（chat_completions 地址）。
func (s *OpenAIGatewayService) keyVideoBaseURL(account *Account) (string, error) {
	baseURL, err := ResolveUpstreamBaseURL(account, openAIGatewayKeyExtensionBaseURL(account), APIProtocolChatCompletions, "")
	if err != nil {
		return "", err
	}
	return s.validateUpstreamBaseURL(baseURL)
}

// applyVideoKeyHeaders 第三方 key 的认证与账号级请求头。
func applyVideoKeyHeaders(req *http.Request, account *Account) error {
	apiKey := strings.TrimSpace(account.GetOpenAIProtocolAPIKey())
	if apiKey == "" {
		return fmt.Errorf("account %d missing api_key", account.ID)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if customUA := account.GetOpenAIUserAgent(); customUA != "" {
		req.Header.Set("User-Agent", customUA)
	}
	account.ApplyHeaderOverrides(req.Header)
	return nil
}

// openKeyVideoContent 中转上游的成片下载：相对地址按根地址补全；与根地址同主机时带 key（上游自己的代理下载），
// 其余视为公开 / 签名地址，不带凭据，且只允许公网目的地（地址来自上游响应，不可信）。
func (s *OpenAIGatewayService) openKeyVideoContent(ctx context.Context, c *gin.Context, account *Account, rawURL string) (*http.Response, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, errVideoTaskNoVideoURL
	}
	root, err := s.keyVideoBaseURL(account)
	if err != nil {
		return nil, err
	}
	rootURL, err := url.Parse(root)
	if err != nil {
		return nil, err
	}
	target, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid video url: %w", err)
	}
	target = rootURL.ResolveReference(target)
	sameHost := strings.EqualFold(target.Host, rootURL.Host)
	contentCtx := WithHTTPUpstreamRedirectsDisabled(ctx)
	downloadURL, err := s.validateOutboundURL(target.String())
	if err != nil {
		return nil, fmt.Errorf("invalid video url: %w", err)
	}
	if !sameHost {
		if err := rejectPrivateImageHost(downloadURL); err != nil {
			return nil, err
		}
		contentCtx = WithHTTPUpstreamPublicHostsOnly(contentCtx)
	}
	req, err := http.NewRequestWithContext(contentCtx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	copyVideoRangeHeader(c, req)
	if sameHost {
		if err := applyVideoKeyHeaders(req, account); err != nil {
			return nil, err
		}
	}
	return s.doVideoContentRequest(req, account)
}

func copyVideoRangeHeader(c *gin.Context, req *http.Request) {
	if c == nil || c.Request == nil {
		return
	}
	if rangeHeader := strings.TrimSpace(c.GetHeader("Range")); rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
}

// doVideoContentRequest 发成片下载请求；2xx / 206 / 416 原样交给调用方流式转发，其余按上游错误返回。
func (s *OpenAIGatewayService) doVideoContentRequest(req *http.Request, account *Account) (*http.Response, error) {
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return nil, &videoUpstreamTransportError{Err: err, URL: safeUpstreamURL(req.URL.String())}
	}
	if resp.StatusCode < 300 || resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		return resp, nil
	}
	defer func() { _ = resp.Body.Close() }()
	body := s.readUpstreamErrorBody(resp)
	message := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(body)))
	if message == "" {
		message = fmt.Sprintf("video content returned status %d", resp.StatusCode)
	}
	return nil, &videoUpstreamHTTPError{StatusCode: resp.StatusCode, Body: body, Headers: resp.Header.Clone(), Message: message, URL: safeUpstreamURL(req.URL.String())}
}
