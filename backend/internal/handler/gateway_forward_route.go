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
// Messages / Chat Completions / Responses 入站请求的转发实现。
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

// compatForwardTargetFor 决定 Anthropic 网关上一次请求交给谁转发，按选中的账号而不是网关平台：
//   - 第三方 key：看它在该网关上承接 inboundProtocol 实际用的协议（anthropic → 标准转发，gemini → Gemini 兼容转发）；
//   - Antigravity 成品号：v1internal 的 Claude 形态（内部 Claude→Gemini 转换，任何模型都走它，包括 gemini 族条目）；
//   - Gemini 成品号：Gemini 兼容转发；该入站没有 Gemini 实现（responses）时承接不了；
//   - 其余成品号（anthropic）：标准转发。
//
// Skip 在调度矩阵（accountServesCatalogRoute / 分组协议过滤）正确时走不到；留着是两套口径的对账点，
// 调用方打 warn 日志并排除该账号，不静默换号。
func compatForwardTargetFor(gatewayPlatform, inboundProtocol string, account *service.Account, geminiSupported bool) compatForwardTarget {
	if account.IsThirdPartyKey() {
		return keyCompatForwardTarget(gatewayPlatform, inboundProtocol, account, geminiSupported)
	}
	if usesAntigravityV1Internal(account) {
		return compatForwardAntigravity
	}
	if account.IsGemini() {
		if geminiSupported {
			return compatForwardGemini
		}
		return compatForwardSkip
	}
	return compatForwardAnthropic
}

// messagesForwardTarget 决定 /v1/messages 交给谁转发。
func messagesForwardTarget(gatewayPlatform string, account *service.Account) compatForwardTarget {
	return compatForwardTargetFor(gatewayPlatform, service.APIProtocolAnthropic, account, true)
}

// chatCompletionsForwardTarget 决定 /v1/chat/completions 交给谁转发。
func chatCompletionsForwardTarget(gatewayPlatform string, account *service.Account) compatForwardTarget {
	return compatForwardTargetFor(gatewayPlatform, service.APIProtocolChatCompletions, account, true)
}

// responsesForwardTarget 决定 /v1/responses 交给谁转发；没有 Responses → Gemini 的实现。
func responsesForwardTarget(gatewayPlatform string, account *service.Account) compatForwardTarget {
	return compatForwardTargetFor(gatewayPlatform, service.APIProtocolResponses, account, false)
}

// keyCompatForwardTarget 第三方 key 按协议转换注册表选上游协议（同协议直连优先）：
// anthropic 地址走标准转发，gemini 地址走 Gemini 兼容转发；responses / chat_completions 地址的
// 转换实现在 OpenAI 网关服务里，本 handler 还接不进来（3b-3 接），先 Skip。
func keyCompatForwardTarget(_ string, inboundProtocol string, account *service.Account, geminiSupported bool) compatForwardTarget {
	switch account.UpstreamProtocolFor(inboundProtocol) {
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

// messagesGatewayPlatform 返回 /v1/messages 系请求所在网关的平台，与 service 层
// （错误透传、调度）读同一份 request.Context：强制平台（/antigravity 路由）优先，其次是
// 合成分组解析出的目标平台，最后是分组平台；都没有时为 anthropic。
// 兜底分组重试会把 request.Context 里的强制平台清空，这里跟着变，不再各读各的。
func messagesGatewayPlatform(c *gin.Context, apiKey *service.APIKey) string {
	return service.AnthropicGatewayRequestPlatform(c.Request.Context(), apiKey)
}

// keyServesAnthropicCountTokens 报告第三方 key 能否承接 count_tokens。
//
// count_tokens 只存在于 Anthropic 协议，没有转换：key 必须以 anthropic 协议直连。
func keyServesAnthropicCountTokens(_ string, account *service.Account) bool {
	return account.ProtocolMatches(service.APIProtocolAnthropic)
}
