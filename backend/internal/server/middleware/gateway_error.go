package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// 网关（推理入口）鉴权与计费前置检查的错误，按客户端认得的格式返回（2026-10-04 D5，muqian 定）：
//   - Anthropic 入口（/v1/messages…）：{"type":"error","error":{"type","message"}}
//   - OpenAI 兼容入口（/v1/chat/completions、/v1/responses、Codex 直连…）：{"error":{"message","type","param","code"}}
//   - 本站自己的接口（/v1/usage、/v1/sub2api/billing…）照旧 {code, message}，前端按 code 翻译
//
// 两种协议格式的 error 对象里都带本站的 code（Anthropic 格式里是多出来的字段，客户端不读）：
// 运维错误分类按它认（ops_error_logger 的 parseOpsErrorResponse 读 error.code）。
// Gemini 入口（/v1beta）有自己的中间件与 Google 格式，不走这里。

type gatewayErrorFormat int

const (
	gatewayErrorPlain gatewayErrorFormat = iota
	gatewayErrorAnthropic
	gatewayErrorOpenAI
)

func gatewayErrorFormatOf(c *gin.Context) gatewayErrorFormat {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return gatewayErrorPlain
	}
	path := strings.TrimRight(c.Request.URL.Path, "/")
	path = strings.TrimPrefix(path, "/antigravity")
	hasPrefix := func(root string) bool { return path == root || strings.HasPrefix(path, root+"/") }
	switch {
	case hasPrefix("/v1/usage"), hasPrefix("/v1/sub2api"):
		return gatewayErrorPlain
	case hasPrefix("/v1/messages"):
		return gatewayErrorAnthropic
	case hasPrefix("/v1/models"):
		// /v1/models 按 anthropic-version 头决定返回哪家的格式，报错也跟着走
		if strings.TrimSpace(c.GetHeader("anthropic-version")) != "" {
			return gatewayErrorAnthropic
		}
		return gatewayErrorOpenAI
	case hasPrefix("/v1"), hasPrefix("/backend-api/codex"), hasPrefix("/openai/v1"), hasPrefix("/responses"):
		return gatewayErrorOpenAI
	}
	return gatewayErrorPlain
}

// abortGatewayError 按入口协议写错误并中断请求。
func abortGatewayError(c *gin.Context, status int, code, message string) {
	switch gatewayErrorFormatOf(c) {
	case gatewayErrorAnthropic:
		c.JSON(status, gin.H{
			"type":  "error",
			"error": gin.H{"type": anthropicGatewayErrorType(status), "message": message, "code": code},
		})
	case gatewayErrorOpenAI:
		c.JSON(status, gin.H{
			"error": gin.H{"message": message, "type": openAIGatewayErrorType(status, code), "param": nil, "code": code},
		})
	default:
		c.JSON(status, NewErrorResponse(code, message))
	}
	c.Abort()
}

// anthropicGatewayErrorType Anthropic 官方的错误类型（按状态码）。
func anthropicGatewayErrorType(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "invalid_request_error"
	case http.StatusUnauthorized:
		return "authentication_error"
	case http.StatusForbidden:
		return "permission_error"
	case http.StatusNotFound:
		return "not_found_error"
	case http.StatusTooManyRequests:
		return "rate_limit_error"
	}
	return "api_error"
}

// openAIGatewayErrorType OpenAI 官方的错误类型：没钱 / 额度用完是 insufficient_quota（Codex 等客户端据此不再重试）。
func openAIGatewayErrorType(status int, code string) string {
	switch code {
	case "INSUFFICIENT_BALANCE", "API_KEY_QUOTA_EXHAUSTED", "USAGE_LIMIT_EXCEEDED":
		return "insufficient_quota"
	}
	switch {
	case status == http.StatusTooManyRequests:
		return "rate_limit_error"
	case status >= 400 && status < 500:
		return "invalid_request_error"
	}
	return "api_error"
}
