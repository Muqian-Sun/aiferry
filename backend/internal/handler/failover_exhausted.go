package handler

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// failoverExhaustedResponse 是换号耗尽后写给客户端的错误；格式由各入站自己写。
// Written 为 true 表示分类过程中已把上游 body 原样写出（OpenAI 模型不存在 400），调用方不再写。
type failoverExhaustedResponse struct {
	Kind    string // 归类：body_too_large / continuation_unsupported / credential / capacity_shed / raw_upstream_400 / silent_refusal / passthrough_rule / mapped
	Status  int
	ErrType string // OpenAI 形状的 error.type；Anthropic 形状的入站按 Kind 自己换成 api_error 等
	Message string
	Written bool
}

// exhaustedClassifyOptions 各入站的差异：透传规则用的平台、默认映射表、流是否已开始、
// 是否允许把 OpenAI 上游的「模型不存在 400」原样写给客户端（只有 OpenAI 形状的入站允许）。
type exhaustedClassifyOptions struct {
	Platform       string
	Passthrough    *service.ErrorPassthroughService
	MapUpstream    func(int) (int, string, string)
	StreamStarted  bool
	RawUpstream400 bool
}

// classifyFailoverExhausted 把 UpstreamFailoverError 归成一条客户端可见的错误，三个聊天入站共用同一套顺序：
// 请求体过大 → HTTP 续链不支持 → 凭据失败 → 容量降载 → OpenAI 模型不存在 400（原样写出）→ 静默拒绝 →
// 错误透传规则（按账号 / 平台）→ 上游状态码默认映射。顺带透传 Retry-After、记 ops 上游错误。
// 流已开始时不能再原样写出上游 body（响应已提交），模型不存在 400 落到默认映射。
func classifyFailoverExhausted(c *gin.Context, failoverErr *service.UpstreamFailoverError, opts exhaustedClassifyOptions) failoverExhaustedResponse {
	mapUpstream := opts.MapUpstream
	if failoverErr == nil {
		status, errType, message := mapUpstream(http.StatusBadGateway)
		service.SetOpsUpstreamError(c, http.StatusBadGateway, message, "")
		return failoverExhaustedResponse{Kind: "mapped", Status: status, ErrType: errType, Message: message}
	}
	if failoverErr.IsOpenAIRequestBodyTooLarge() {
		service.SetOpsUpstreamError(c, http.StatusRequestEntityTooLarge, service.OpenAIRequestBodyTooLargeClientMessage, "")
		return failoverExhaustedResponse{Kind: "body_too_large", Status: http.StatusRequestEntityTooLarge, ErrType: "invalid_request_error", Message: service.OpenAIRequestBodyTooLargeClientMessage}
	}
	if failoverErr.Reason == service.OpenAIHTTPContinuationUnsupportedReason {
		message := strings.TrimSpace(failoverErr.ClientMessage)
		if message == "" {
			message = "previous_response_id requires an OpenAI API-key account for HTTP requests"
		}
		return failoverExhaustedResponse{Kind: "continuation_unsupported", Status: http.StatusBadRequest, ErrType: "invalid_request_error", Message: message}
	}
	copyFailoverRetryAfter(c, failoverErr.ResponseHeaders)
	if failoverErr.IsCredentialFailure() {
		status, message := credentialFailoverClientResponse(failoverErr)
		return failoverExhaustedResponse{Kind: "credential", Status: status, ErrType: "upstream_error", Message: message}
	}
	if failoverErr.IsOpenAICapacityShed() && strings.TrimSpace(failoverErr.ClientMessage) != "" {
		status := failoverErr.ClientStatusCode
		if status <= 0 {
			status = http.StatusServiceUnavailable
		}
		return failoverExhaustedResponse{Kind: "capacity_shed", Status: status, ErrType: "server_error", Message: failoverErr.ClientMessage}
	}
	statusCode := failoverErr.StatusCode
	responseBody := failoverErr.ResponseBody
	if opts.RawUpstream400 && !opts.StreamStarted && statusCode == http.StatusBadRequest && service.IsOpenAICompatibleModelNotFound400(responseBody) {
		upstreamMsg := service.SanitizeUpstreamErrorMessage(service.ExtractUpstreamErrorMessage(responseBody))
		service.SetOpsUpstreamError(c, statusCode, upstreamMsg, "")
		service.WriteOpenAIUpstreamClientError(c, statusCode, responseBody, upstreamMsg)
		return failoverExhaustedResponse{Kind: "raw_upstream_400", Status: statusCode, ErrType: "invalid_request_error", Message: upstreamMsg, Written: true}
	}
	if service.IsOpenAISilentRefusalErrorBody(responseBody) {
		service.SetOpsUpstreamError(c, statusCode, service.OpenAISilentRefusalClientMessage(), "")
		return failoverExhaustedResponse{Kind: "silent_refusal", Status: http.StatusBadGateway, ErrType: "upstream_error", Message: service.OpenAISilentRefusalClientMessage()}
	}
	if opts.Passthrough != nil && len(responseBody) > 0 {
		if rule := opts.Passthrough.MatchRule(opts.Platform, statusCode, responseBody); rule != nil {
			respCode := statusCode
			if !rule.PassthroughCode && rule.ResponseCode != nil {
				respCode = *rule.ResponseCode
			}
			msg := service.ExtractUpstreamErrorMessage(responseBody)
			if !rule.PassthroughBody && rule.CustomMessage != nil {
				msg = *rule.CustomMessage
			}
			if rule.SkipMonitoring {
				c.Set(service.OpsSkipPassthroughKey, true)
			}
			return failoverExhaustedResponse{Kind: "passthrough_rule", Status: respCode, ErrType: "upstream_error", Message: msg}
		}
	}
	upstreamMsg := service.ExtractUpstreamErrorMessage(responseBody)
	service.SetOpsUpstreamError(c, statusCode, upstreamMsg, "")
	status, errType, message := mapUpstream(statusCode)
	return failoverExhaustedResponse{Kind: "mapped", Status: status, ErrType: errType, Message: message}
}

// anthropicErrType 把 OpenAI 形状的错误类型换成 Anthropic 形状能认的：凭据 / 容量降载是 api_error。
func (r failoverExhaustedResponse) anthropicErrType() string {
	switch r.Kind {
	case "credential", "capacity_shed":
		return "api_error"
	default:
		return r.ErrType
	}
}

// openAIMapUpstreamError OpenAI 形状入站的上游状态码默认映射（与原 OpenAI handler 的 mapUpstreamError 相同）。
func openAIMapUpstreamError(statusCode int) (int, string, string) {
	switch statusCode {
	case 401:
		return http.StatusBadGateway, "upstream_error", "Upstream authentication failed, please contact administrator"
	case 403:
		return http.StatusBadGateway, "upstream_error", "Upstream access forbidden, please contact administrator"
	case 429:
		return http.StatusTooManyRequests, "rate_limit_error", "Upstream rate limit exceeded, please retry later"
	case 529:
		return http.StatusServiceUnavailable, "upstream_error", "Upstream service overloaded, please retry later"
	case 500, 502, 503, 504:
		return http.StatusBadGateway, "upstream_error", "Upstream service temporarily unavailable"
	default:
		return http.StatusBadGateway, "upstream_error", "Upstream request failed"
	}
}
