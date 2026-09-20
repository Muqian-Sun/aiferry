package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// usesAntigravityV1Internal 报告账号是否交给 AntigravityGatewayService 的 v1internal
// 转发实现（Forward / ForwardGemini）。
//
// 只有 Antigravity 成品号走这条路：令牌、项目号与模型映射都是成品号专属。第三方
// key 即使展示标签选了 antigravity，也按协议地址走当前网关的标准转发（Anthropic
// Messages 或 Gemini v1beta），标签不参与路由。
func usesAntigravityV1Internal(account *service.Account) bool {
	return account != nil && !account.IsThirdPartyKey() && account.IsAntigravity()
}

// compatForwardTarget 是 Anthropic 网关（anthropic / gemini / antigravity 分组）上
// Chat Completions 与 Responses 入站请求的转发实现。
type compatForwardTarget int

const (
	// compatForwardSkip 表示该账号在本网关上承接不了这次请求，调用方换号。
	compatForwardSkip compatForwardTarget = iota
	// compatForwardAnthropic 转成 Anthropic Messages，经 GatewayService 转发。
	compatForwardAnthropic
	// compatForwardGemini 转成 Gemini v1beta，经 GeminiMessagesCompatService 转发。
	compatForwardGemini
	// compatForwardAntigravity 经 AntigravityGatewayService 兼容层转发（仅 Antigravity 成品号）。
	compatForwardAntigravity
)

// chatCompletionsForwardTarget 决定 /v1/chat/completions 在 Anthropic 网关上交给谁转发。
//
// 第三方 key 看它在该网关上实际使用的上游协议；成品号按厂商分流，保持原有规则。
func chatCompletionsForwardTarget(gatewayPlatform string, account *service.Account) compatForwardTarget {
	if account.IsThirdPartyKey() {
		return keyCompatForwardTarget(gatewayPlatform, service.APIProtocolChatCompletions, account, true)
	}
	if gatewayPlatform == service.PlatformGemini && account.Platform != service.PlatformGemini {
		return compatForwardSkip
	}
	if account.Platform == service.PlatformGemini {
		return compatForwardGemini
	}
	if shouldUseAntigravityCompat(account) {
		return compatForwardAntigravity
	}
	return compatForwardAnthropic
}

// responsesForwardTarget 决定 /v1/responses 在 Anthropic 网关上交给谁转发。
//
// 这里没有 Responses → Gemini 的转换实现：第三方 key 在该网关上的协议是 gemini 时
// 承接不了，只能换号。成品号保持原有规则。
func responsesForwardTarget(gatewayPlatform string, account *service.Account) compatForwardTarget {
	if account.IsThirdPartyKey() {
		return keyCompatForwardTarget(gatewayPlatform, service.APIProtocolResponses, account, false)
	}
	if shouldUseAntigravityCompat(account) {
		return compatForwardAntigravity
	}
	return compatForwardAnthropic
}

func keyCompatForwardTarget(gatewayPlatform, inboundProtocol string, account *service.Account, geminiSupported bool) compatForwardTarget {
	switch account.KeyUpstreamProtocolFor(anthropicGatewayPlatform(gatewayPlatform), inboundProtocol) {
	case service.APIProtocolAnthropic:
		return compatForwardAnthropic
	case service.APIProtocolGemini:
		if geminiSupported {
			return compatForwardGemini
		}
		return compatForwardSkip
	default:
		return compatForwardSkip
	}
}

// anthropicGatewayPlatform 返回 Anthropic 网关 handler 用来选协议的平台。
//
// 未分组 key（放行未分组调度时）没有分组平台，调度按 anthropic 选号
// （GatewayService.resolvePlatform），这里取同一个值，两边才不会对不上。
func anthropicGatewayPlatform(platform string) string {
	if platform == "" {
		return service.PlatformAnthropic
	}
	return platform
}

// messagesGatewayPlatform 返回 /v1/messages 系请求所在网关的平台，与 service 层
// （错误透传、调度）读同一份 request.Context：强制平台（/antigravity 路由）优先，其次是
// 合成分组解析出的目标平台，最后是分组平台；都没有时为 anthropic。
// 兜底分组重试会把 request.Context 里的强制平台清空，这里跟着变，不再各读各的。
func messagesGatewayPlatform(c *gin.Context, apiKey *service.APIKey) string {
	return service.AnthropicGatewayRequestPlatform(c.Request.Context(), apiKey)
}

// keyServesAnthropicCountTokens 报告第三方 key 能否在当前网关上承接 count_tokens。
//
// count_tokens 只存在于 Anthropic 协议。key 在该网关上走的不是 anthropic 协议时
// （例如 gemini 分组），不能把 Anthropic 请求发到它别的协议地址上。
func keyServesAnthropicCountTokens(gatewayPlatform string, account *service.Account) bool {
	return account.KeyUpstreamProtocolFor(anthropicGatewayPlatform(gatewayPlatform), service.APIProtocolAnthropic) == service.APIProtocolAnthropic
}
