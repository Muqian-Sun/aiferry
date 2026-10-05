package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// 建渠道时的「探测协议」（2026-09-29 muqian 定）：对管理员填的同一个基础地址，逐个试四个上游协议，
// 列出支持 / 不支持 / 不确定，管理员在结果旁边的选择框里挑一个（一个 key 只承接一个协议）。
//
// 先发不带模型的空请求看状态码（不花钱）：不存在的端点回 404 / 405 或落到网页，直接判不支持；
// 其余拿不准的（400 / 422 参数校验、401 / 403 / 429 / 5xx / 网络错误 / 2xx 但内容不像这个协议）
// 再用上游模型列表里挑的模型发一次最小的真实请求确认。
//
// 真实请求挑模型（2026-10-06 muqian：生产上按字母挑中上游列出、实际调不通的 gpt-5.3-codex-spark，四个协议全是不确定，
// 还让上游把这个模型冷却了 30 分钟）：先挑和协议对口、目录里认识的；被拒的不是 key / 限流问题就换下一个，最多试两个。

// ProtocolProbeStatus 一个协议的探测结论。
type ProtocolProbeStatus string

const (
	ProtocolProbeSupported   ProtocolProbeStatus = "supported"
	ProtocolProbeUnsupported ProtocolProbeStatus = "unsupported"
	ProtocolProbeUnknown     ProtocolProbeStatus = "unknown"
)

// ProtocolProbeReason 结论的依据，前端据此给出说明。
type ProtocolProbeReason string

const (
	ProtocolProbeReasonAccepted        ProtocolProbeReason = "accepted"         // 2xx JSON
	ProtocolProbeReasonValidationError ProtocolProbeReason = "validation_error" // 400 / 422：端点在、只是请求不完整
	ProtocolProbeReasonNotFound        ProtocolProbeReason = "not_found"        // 404 / 405 / 501
	ProtocolProbeReasonNotAPI          ProtocolProbeReason = "not_api"          // 回的不是 JSON（多半是网页）
	ProtocolProbeReasonUnexpectedBody  ProtocolProbeReason = "unexpected_body"  // 2xx JSON 但不像这个协议的响应（兜底的健康检查之类）
	ProtocolProbeReasonRealRejected    ProtocolProbeReason = "real_rejected"    // 最小的真实请求被回 400 / 422：端点在，但这个 key / 模型走不通
	ProtocolProbeReasonAuthRejected    ProtocolProbeReason = "auth_rejected"    // 401 / 403
	ProtocolProbeReasonRateLimited     ProtocolProbeReason = "rate_limited"     // 429
	ProtocolProbeReasonUpstreamError   ProtocolProbeReason = "upstream_error"   // 5xx 与其他状态码
	ProtocolProbeReasonNetworkError    ProtocolProbeReason = "network_error"
	ProtocolProbeReasonNoModel         ProtocolProbeReason = "no_model" // 空请求拿不准，又拿不到模型名做真实请求
)

// ProbedUpstreamProtocol 一个协议的探测结果。BaseURL 是选中后填进表单的地址。
type ProbedUpstreamProtocol struct {
	Protocol   string              `json:"protocol"`
	BaseURL    string              `json:"base_url"`
	Status     ProtocolProbeStatus `json:"status"`
	Reason     ProtocolProbeReason `json:"reason"`
	HTTPStatus int                 `json:"http_status,omitempty"`
	// Model 真实请求用的模型（试了两个时是后一个）；只发了空请求时为空。
	Model string `json:"model,omitempty"`
	// Detail 上游出错时回的原话（截断、去掉 key），让管理员看到被拒的真实原因。
	Detail string `json:"detail,omitempty"`
}

// protocolProbeOutcome 一次探测请求的结论。
type protocolProbeOutcome struct {
	status     ProtocolProbeStatus
	reason     ProtocolProbeReason
	httpStatus int
	detail     string
}

const (
	protocolProbeTimeout      = 20 * time.Second
	protocolProbeBodyReadSize = 64 << 10
	// protocolProbeMaxModels 每个协议最多用几个模型发真实请求（muqian 2026-10-06 定 2 个）。
	protocolProbeMaxModels = 2
	// protocolProbeDetailMaxRunes 上游原话最多留多少字。拍的：一句报错够用，再长就是堆栈或网页。
	protocolProbeDetailMaxRunes = 200
)

// ProbeUpstreamProtocols 探测 baseURL 支持哪些上游协议。account 只提供 key、代理与 TLS 指纹，地址以 baseURL 为准。
// isKnownModel 判断模型名是不是目录里认识的（挑真实请求的模型时优先）；为 nil 时都当不认识。
func (s *AccountTestService) ProbeUpstreamProtocols(ctx context.Context, account *Account, baseURL string, isKnownModel func(string) bool) ([]ProbedUpstreamProtocol, error) {
	if s == nil || s.httpUpstream == nil {
		return nil, newUpstreamModelSyncConfigError("Upstream HTTP client is not configured", nil)
	}
	apiKey := strings.TrimSpace(account.GetOpenAIProtocolAPIKey())
	if apiKey == "" {
		return nil, newUpstreamModelSyncConfigError("api_key is required", nil)
	}
	base, err := s.validateUpstreamBaseURL(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, newUpstreamModelSyncConfigError("Invalid upstream address: "+err.Error(), err)
	}

	protocols := UpstreamProtocols()
	results := make([]ProbedUpstreamProtocol, len(protocols))
	var wg sync.WaitGroup
	for i, protocol := range protocols {
		results[i] = ProbedUpstreamProtocol{Protocol: protocol, BaseURL: protocolProbeBaseURL(protocol, base)}
		wg.Add(1)
		go func(r *ProbedUpstreamProtocol) {
			defer wg.Done()
			r.apply(s.sendProtocolProbe(ctx, account, apiKey, r.Protocol, r.BaseURL, ""))
		}(&results[i])
	}
	wg.Wait()

	// 空请求拿不准的，用上游模型列表里的模型发真实请求；模型列表只拉一次。真实请求逐个发，不并发，
	// 免得同一个 key 短时间内打太多请求被上游限流。
	var models []string
	modelsLoaded := false
	for i := range results {
		r := &results[i]
		if r.Status != ProtocolProbeUnknown {
			continue
		}
		if !modelsLoaded {
			models = s.protocolProbeModels(ctx, account, base)
			modelsLoaded = true
		}
		candidates := protocolProbeCandidates(r.Protocol, models, isKnownModel, protocolProbeMaxModels)
		if len(candidates) == 0 {
			// 401 / 403、内容不像这个协议、连不上这类空请求就拿到的原因比「没有模型」更有用，保留
			// （连不上原来也被改写成「拿不到模型名」，2026-10-04 UI E2E）
			if r.Reason == ProtocolProbeReasonUpstreamError {
				r.Reason = ProtocolProbeReasonNoModel
			}
			continue
		}
		for _, model := range candidates {
			r.Model = model
			outcome := s.sendProtocolProbe(ctx, account, apiKey, r.Protocol, r.BaseURL, model)
			r.apply(outcome)
			if !protocolProbeTryAnotherModel(outcome) {
				break
			}
		}
	}
	return results, nil
}

func (r *ProbedUpstreamProtocol) apply(o protocolProbeOutcome) {
	r.Status, r.Reason, r.HTTPStatus, r.Detail = o.status, o.reason, o.httpStatus, o.detail
}

// protocolProbeTryAnotherModel 真实请求失败时要不要换个模型再试：被拒的原因可能是模型（上游没有这个模型、
// 这个模型的上游出错）才换；key 被拒（401 / 403）、限流（429）换模型也一样，连不上与 2xx 都不换。
func protocolProbeTryAnotherModel(o protocolProbeOutcome) bool {
	switch o.httpStatus {
	case 0, http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests:
		return false
	}
	return o.httpStatus >= 400
}

// protocolProbeBaseURL 选中后填进表单的地址。Anthropic / Chat Completions / Responses 的端点拼接
// （joinUpstreamEndpointURL）带不带 /v1 结果一样，原样用；Gemini 直接在地址后接 /v1beta/...，
// 地址末尾的版本段要去掉，否则拼成 /v1/v1beta。
func protocolProbeBaseURL(protocol string, base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if protocol != APIProtocolGemini {
		return base
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return base
	}
	path := strings.TrimRight(parsed.Path, "/")
	if i := strings.LastIndex(path, "/"); i >= 0 && isUpstreamAPIVersionSegment(path[i+1:]) {
		parsed.Path = path[:i]
		parsed.RawPath = ""
		return strings.TrimRight(parsed.String(), "/")
	}
	return base
}

// sendProtocolProbe 发一次探测请求并分类。model 为空发空请求，否则发最小的真实请求。
//
// 400 / 422 不直接算支持：空请求被参数校验拦下只说明端点多半存在，但有的上游路由在、这个 key 却用不了
// （2026-09-29 真实上游 fenno：Gemini 路由对非 Gemini 分组的 key 一律回 400「API key group platform is not
// gemini」），所以归为不确定、再用真实请求确认；真实请求（带模型、格式完整）还被回 400 / 422 就是走不通。
func (s *AccountTestService) sendProtocolProbe(ctx context.Context, account *Account, apiKey, protocol, base, model string) protocolProbeOutcome {
	ctx, cancel := context.WithTimeout(ctx, protocolProbeTimeout)
	defer cancel()
	req, err := buildProtocolProbeRequest(ctx, apiKey, protocol, base, model)
	if err != nil {
		return protocolProbeOutcome{status: ProtocolProbeUnknown, reason: ProtocolProbeReasonNetworkError}
	}
	resp, err := s.doKeyProbeRequest(req, account)
	if err != nil {
		return protocolProbeOutcome{status: ProtocolProbeUnknown, reason: ProtocolProbeReasonNetworkError}
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, protocolProbeBodyReadSize))
	contentType := resp.Header.Get("Content-Type")
	status, reason := classifyProtocolProbeResponse(resp.StatusCode, contentType, body)
	if status == ProtocolProbeSupported && reason == ProtocolProbeReasonAccepted && !protocolProbeBodyMatches(protocol, model != "", body) {
		status, reason = ProtocolProbeUnknown, ProtocolProbeReasonUnexpectedBody
	}
	if reason == ProtocolProbeReasonValidationError {
		status = ProtocolProbeUnknown
		if model != "" {
			reason = ProtocolProbeReasonRealRejected
		}
	}
	outcome := protocolProbeOutcome{status: status, reason: reason, httpStatus: resp.StatusCode}
	// 空请求被参数校验拦下的原话（「model is required」之类）是我们故意发空请求换来的，给人看只会误导
	if resp.StatusCode >= 400 && reason != ProtocolProbeReasonValidationError {
		outcome.detail = protocolProbeDetail(contentType, body, apiKey)
	}
	return outcome
}

// 上游原话里像 key 的片段：sk-/ak- 开头的，或 32 位以上连续的字母数字（token、哈希）。
var protocolProbeSecretPattern = regexp.MustCompile(`(?i)\b(?:sk|ak|key)-[A-Za-z0-9_\-]{4,}|[A-Za-z0-9_\-]{32,}`)

// protocolProbeDetail 从出错响应里取一句给人看的原话：JSON 取常见的 message / error / detail 字段，
// 纯文本原样取，网页不取；去掉这次用的 key 与像 key 的片段，截到 protocolProbeDetailMaxRunes。
func protocolProbeDetail(contentType string, body []byte, apiKey string) string {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return ""
	}
	text := ""
	var obj any
	if json.Unmarshal(trimmed, &obj) == nil {
		text = protocolProbeMessageFrom(obj)
	} else if !strings.Contains(strings.ToLower(contentType), "html") && !bytes.HasPrefix(trimmed, []byte("<")) {
		text = string(trimmed)
	}
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return ""
	}
	if apiKey != "" {
		text = strings.ReplaceAll(text, apiKey, "***")
	}
	text = protocolProbeSecretPattern.ReplaceAllString(text, "***")
	if utf8.RuneCountInString(text) > protocolProbeDetailMaxRunes {
		text = string([]rune(text)[:protocolProbeDetailMaxRunes]) + "…"
	}
	return text
}

// protocolProbeMessageFrom 按 OpenAI / Anthropic / Gemini / 常见中转的错误形状取报错句子。
func protocolProbeMessageFrom(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case map[string]any:
		for _, key := range []string{"message", "error", "detail", "msg", "error_msg", "errorMessage"} {
			if inner, ok := value[key]; ok {
				if text := protocolProbeMessageFrom(inner); text != "" {
					return text
				}
			}
		}
	}
	return ""
}

// protocolProbeBodyMatches 2xx 的响应要像这个协议才算支持：有的上游对任意路径回 200 JSON（健康检查兜底），
// 只看状态码会把不存在的端点当成支持。Gemini 空请求是模型列表，要有 models；其余看各协议响应的标志字段。
func protocolProbeBodyMatches(protocol string, realRequest bool, body []byte) bool {
	var obj map[string]json.RawMessage
	if json.Unmarshal(body, &obj) != nil {
		return false
	}
	has := func(key string) bool { _, ok := obj[key]; return ok }
	switch protocol {
	case APIProtocolAnthropic:
		return has("content")
	case APIProtocolChatCompletions:
		return has("choices")
	case APIProtocolResponses:
		return has("output")
	case APIProtocolGemini:
		if realRequest {
			return has("candidates")
		}
		return has("models")
	}
	return false
}

func classifyProtocolProbeResponse(statusCode int, contentType string, body []byte) (ProtocolProbeStatus, ProtocolProbeReason) {
	isJSON := strings.Contains(strings.ToLower(contentType), "json") || json.Valid(bytes.TrimSpace(body))
	switch {
	case statusCode == http.StatusNotFound || statusCode == http.StatusMethodNotAllowed || statusCode == http.StatusNotImplemented:
		return ProtocolProbeUnsupported, ProtocolProbeReasonNotFound
	case statusCode >= 200 && statusCode < 300:
		if isJSON {
			return ProtocolProbeSupported, ProtocolProbeReasonAccepted
		}
		return ProtocolProbeUnsupported, ProtocolProbeReasonNotAPI
	case statusCode == http.StatusBadRequest || statusCode == http.StatusUnprocessableEntity:
		if isJSON {
			return ProtocolProbeSupported, ProtocolProbeReasonValidationError
		}
		return ProtocolProbeUnsupported, ProtocolProbeReasonNotAPI
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return ProtocolProbeUnknown, ProtocolProbeReasonAuthRejected
	case statusCode == http.StatusTooManyRequests:
		return ProtocolProbeUnknown, ProtocolProbeReasonRateLimited
	default:
		return ProtocolProbeUnknown, ProtocolProbeReasonUpstreamError
	}
}

// buildProtocolProbeRequest 各协议的探测请求。认证头与真实转发同形：Anthropic 同时带 x-api-key 与 Bearer
// （中转两种都有人用，多带一个无害），OpenAI 两个协议 Bearer，Gemini x-goog-api-key。
func buildProtocolProbeRequest(ctx context.Context, apiKey, protocol, base, model string) (*http.Request, error) {
	var (
		method  = http.MethodPost
		target  string
		payload any = map[string]any{}
		err     error
	)
	switch protocol {
	case APIProtocolAnthropic:
		target = joinUpstreamEndpointURL(base, "/v1/messages")
		if model != "" {
			payload = map[string]any{"model": model, "max_tokens": 1, "messages": []map[string]any{{"role": "user", "content": "hi"}}}
		}
	case APIProtocolChatCompletions:
		target = buildOpenAIChatCompletionsURL(base)
		if model != "" {
			payload = map[string]any{"model": model, "max_tokens": 1, "messages": []map[string]any{{"role": "user", "content": "hi"}}}
		}
	case APIProtocolResponses:
		target = buildOpenAIResponsesURL(base)
		if model != "" {
			// OpenAI 要求 max_output_tokens ≥ 16
			payload = map[string]any{"model": model, "max_output_tokens": 16, "input": "hi"}
		}
	case APIProtocolGemini:
		if model == "" {
			// Gemini 的动作端点必须带模型名，空请求改用模型列表（同样不花钱）
			method, payload = http.MethodGet, nil
			target = base + "/v1beta/models"
		} else {
			if target, err = buildGeminiAIStudioModelActionURL(base, model, "generateContent", false); err != nil {
				return nil, err
			}
			payload = map[string]any{
				"contents":         []map[string]any{{"role": "user", "parts": []map[string]any{{"text": "hi"}}}},
				"generationConfig": map[string]any{"maxOutputTokens": 1},
			}
		}
	default:
		return nil, errors.New("unsupported protocol: " + protocol)
	}

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	switch protocol {
	case APIProtocolAnthropic:
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	case APIProtocolGemini:
		req.Header.Set("x-goog-api-key", apiKey)
	default:
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	return req, nil
}

// protocolProbeModels 真实请求要用的模型名：按 OpenAI 形态拉 {base}/v1/models（中转普遍提供）。拉不到返回空。
func (s *AccountTestService) protocolProbeModels(ctx context.Context, account *Account, base string) []string {
	temp := *account
	temp.ProtocolEndpoints = map[string]string{APIProtocolChatCompletions: base}
	models, _, err := s.fetchUpstreamModelList(ctx, &temp)
	if err != nil {
		return nil
	}
	return dedupeAndSortModelIDs(models)
}

// protocolProbeCandidates 按协议给真实请求排模型，取前 limit 个：和协议对口（Anthropic 的 claude、Gemini 的
// gemini、OpenAI 两个协议的 gpt）且目录里认识的最先，其次对口的，再次目录里认识的，最后其余的（中转多数能跨协议
// 转换，没有对口的也照样试）。同一档内保持上游名单的顺序。目录里认识的优先，是为了避开上游列出、实际调不通的冷门模型。
func protocolProbeCandidates(protocol string, models []string, isKnownModel func(string) bool, limit int) []string {
	prefer := "gpt"
	switch protocol {
	case APIProtocolAnthropic:
		prefer = "claude"
	case APIProtocolGemini:
		prefer = "gemini"
	}
	rank := func(model string) int {
		score := 0
		if strings.Contains(strings.ToLower(model), prefer) {
			score += 2
		}
		if isKnownModel != nil && isKnownModel(model) {
			score++
		}
		return score
	}
	ranked := append([]string(nil), models...)
	sort.SliceStable(ranked, func(i, j int) bool { return rank(ranked[i]) > rank(ranked[j]) })
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked
}
