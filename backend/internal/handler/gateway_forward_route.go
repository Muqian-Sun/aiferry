package handler

import (
	"strings"

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

// compatForwardTarget 是 Anthropic 网关上 Messages / Chat Completions / Responses 入站请求的转发实现。
type compatForwardTarget int

const (
	// compatForwardSkip 表示该账号承接不了这次请求，调用方换号。
	compatForwardSkip compatForwardTarget = iota
	// compatForwardAnthropic 转成 Anthropic Messages，经 GatewayService 转发。
	compatForwardAnthropic
	// compatForwardGemini 转成 Gemini v1beta，经 GeminiMessagesCompatService 转发。
	compatForwardGemini
	// compatForwardAntigravity 经 AntigravityGatewayService 兼容层转发（仅 Antigravity 成品号）。
	compatForwardAntigravity
	// compatForwardOpenAI 转成 OpenAI Responses / Chat Completions，经 OpenAIGatewayService 转发；
	// 对应资源的上游协议是 responses 或 chat_completions。
	compatForwardOpenAI
)

// compatForwardTargetFor 决定一次请求交给谁转发：只看资源承接该入站协议实际用的上游协议
// （协议转换注册表），不看资源种类、不看网关族。
//
// Skip 在调度矩阵（accountServesCatalogRoute）正确时走不到；留着是两套口径的对账点，
// 调用方打 warn 日志并排除该账号，不静默换号。
func compatForwardTargetFor(inboundProtocol string, account *service.Account) compatForwardTarget {
	switch account.UpstreamProtocolFor(inboundProtocol) {
	case service.APIProtocolAnthropic:
		return compatForwardAnthropic
	case service.APIProtocolGemini:
		return compatForwardGemini
	case service.UpstreamProtocolAntigravity:
		return compatForwardAntigravity
	case service.APIProtocolResponses, service.APIProtocolChatCompletions:
		return compatForwardOpenAI
	default:
		return compatForwardSkip
	}
}

// messagesForwardTarget 决定 /v1/messages 交给谁转发。
func messagesForwardTarget(account *service.Account) compatForwardTarget {
	return compatForwardTargetFor(service.APIProtocolAnthropic, account)
}

// chatCompletionsForwardTarget 决定 /v1/chat/completions 交给谁转发。
func chatCompletionsForwardTarget(account *service.Account) compatForwardTarget {
	return compatForwardTargetFor(service.APIProtocolChatCompletions, account)
}

// responsesForwardTarget 决定 /v1/responses 交给谁转发；Responses → Gemini 没有转换，
// 注册表对 gemini 资源返回空上游协议，落到 Skip。
func responsesForwardTarget(account *service.Account) compatForwardTarget {
	return compatForwardTargetFor(service.APIProtocolResponses, account)
}

// messagesGatewayPlatform 返回 /v1/messages 系请求所在网关的平台，与 service 层
// （错误透传、调度）读同一份 request.Context：强制平台（/antigravity 路由）优先，其次是
// 目录条目解析出的目标平台，最后是分组平台；都没有时为 anthropic。
func messagesGatewayPlatform(c *gin.Context) string {
	return service.AnthropicGatewayRequestPlatform(c.Request.Context())
}

// keyServesAnthropicCountTokens 报告第三方 key 能否承接 count_tokens。
//
// count_tokens 只存在于 Anthropic 协议，没有转换：key 必须以 anthropic 协议直连。
func keyServesAnthropicCountTokens(_ string, account *service.Account) bool {
	return account.ProtocolMatches(service.APIProtocolAnthropic)
}

// resolveOpenAIUpstreamEndpoint returns the actual upstream endpoint for an
// OpenAI-compatible account. A forwarding result is authoritative because a
// single inbound route may choose raw Chat or a Responses bridge at runtime.
// The account-based derivation remains as a fallback for existing callers and
// forwarding paths that do not report their endpoint yet.
func resolveOpenAIUpstreamEndpoint(c *gin.Context, account *service.Account, result *service.OpenAIForwardResult) string {
	if result != nil {
		if endpoint := strings.TrimSpace(result.UpstreamEndpoint); endpoint != "" {
			return endpoint
		}
	}
	if endpoint := service.GetActualOpenAIUpstreamEndpoint(c); endpoint != "" {
		return endpoint
	}
	return GetUpstreamEndpoint(c, account.Platform)
}
