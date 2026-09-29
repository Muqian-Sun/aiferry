package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// 建渠道时的「探测协议」（2026-09-29 muqian 定）：对管理员填的同一个基础地址，逐个试四个上游协议，
// 列出支持 / 不支持 / 不确定，管理员在结果旁边的选择框里挑一个（一个 key 只承接一个协议）。
//
// 先发不带模型的空请求看状态码（不花钱）：端点存在的上游会做参数校验回 400 / 422，不存在的回 404 / 405
// 或落到网页；拿不准的（401 / 403 / 429 / 5xx / 网络错误 / 2xx 但内容不像这个协议）再用上游模型列表里挑的模型
// 发一次最小的真实请求确认。

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
	// Model 真实请求用的模型；只发了空请求时为空。
	Model string `json:"model,omitempty"`
}

const (
	protocolProbeTimeout      = 20 * time.Second
	protocolProbeBodyReadSize = 64 << 10
)

// ProbeUpstreamProtocols 探测 baseURL 支持哪些上游协议。account 只提供 key、代理与 TLS 指纹，地址以 baseURL 为准。
func (s *AccountTestService) ProbeUpstreamProtocols(ctx context.Context, account *Account, baseURL string) ([]ProbedUpstreamProtocol, error) {
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
			r.Status, r.Reason, r.HTTPStatus = s.sendProtocolProbe(ctx, account, apiKey, r.Protocol, r.BaseURL, "")
		}(&results[i])
	}
	wg.Wait()

	// 空请求拿不准的，用上游模型列表里的模型发一次真实请求；模型列表只拉一次。
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
		model := pickProtocolProbeModel(r.Protocol, models)
		if model == "" {
			// 401 / 403、内容不像这个协议这类空请求就拿到的原因比「没有模型」更有用，保留
			if r.Reason == ProtocolProbeReasonUpstreamError || r.Reason == ProtocolProbeReasonNetworkError {
				r.Reason = ProtocolProbeReasonNoModel
			}
			continue
		}
		r.Model = model
		r.Status, r.Reason, r.HTTPStatus = s.sendProtocolProbe(ctx, account, apiKey, r.Protocol, r.BaseURL, model)
	}
	return results, nil
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
func (s *AccountTestService) sendProtocolProbe(ctx context.Context, account *Account, apiKey, protocol, base, model string) (ProtocolProbeStatus, ProtocolProbeReason, int) {
	ctx, cancel := context.WithTimeout(ctx, protocolProbeTimeout)
	defer cancel()
	req, err := buildProtocolProbeRequest(ctx, apiKey, protocol, base, model)
	if err != nil {
		return ProtocolProbeUnknown, ProtocolProbeReasonNetworkError, 0
	}
	resp, err := s.doKeyProbeRequest(req, account)
	if err != nil {
		return ProtocolProbeUnknown, ProtocolProbeReasonNetworkError, 0
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, protocolProbeBodyReadSize))
	status, reason := classifyProtocolProbeResponse(resp.StatusCode, resp.Header.Get("Content-Type"), body)
	if status == ProtocolProbeSupported && reason == ProtocolProbeReasonAccepted && !protocolProbeBodyMatches(protocol, model != "", body) {
		status, reason = ProtocolProbeUnknown, ProtocolProbeReasonUnexpectedBody
	}
	return status, reason, resp.StatusCode
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

// pickProtocolProbeModel 按协议挑一个最可能走得通的模型：Anthropic 挑 claude、Gemini 挑 gemini、
// OpenAI 两个协议挑 gpt；没有就用第一个（中转多数能跨协议转换）。
func pickProtocolProbeModel(protocol string, models []string) string {
	if len(models) == 0 {
		return ""
	}
	prefer := "gpt"
	switch protocol {
	case APIProtocolAnthropic:
		prefer = "claude"
	case APIProtocolGemini:
		prefer = "gemini"
	}
	for _, m := range models {
		if strings.Contains(strings.ToLower(m), prefer) {
			return m
		}
	}
	return models[0]
}
