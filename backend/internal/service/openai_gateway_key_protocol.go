package service

import (
	"strings"
)

// OpenAI 网关上第三方 key 的上游协议与协议特性判定。
//
// 第三方 key 选的平台只是展示标签：上游协议由协议地址决定（KeyUpstreamProtocolFor），
// 厂商特化由地址识别出的厂商决定（Vendor），这里不读 Platform。成品号（OpenAI OAuth /
// Codex、Grok OAuth）的协议由厂商决定，不经过这些函数。

// resolveOpenAIGatewayKeyProtocol 返回第三方 key 在 OpenAI 网关上承接本次请求实际
// 使用的上游协议。inboundProtocol 由转发入口固定：Forward 是 responses，
// ForwardAsChatCompletions 是 chat_completions，ForwardAsAnthropic 是 anthropic。
//
// OpenCode 官方地址是厂商特化：同一个 key 按模型分流到不同原生端点。模型规则选出的
// 协议配了地址就用它，没配则回到通用选择——规则只在 key 已配置的协议之间挑。
// upstreamModel 只在需要时才求值（解析请求体有成本）。
//
// 取不到协议时返回 MissingProtocolEndpointError：调度已排除这类 key，走到这里说明
// 调度与转发的判定不一致，直接报错比静默改走别的协议或官方地址更容易排查。
func resolveOpenAIGatewayKeyProtocol(account *Account, inboundProtocol string, upstreamModel func() string) (string, error) {
	if account.Vendor() == PlatformOpenCodeGo && upstreamModel != nil {
		if protocol := account.ResolveOpenCodeGoUpstreamProtocol(upstreamModel()); protocol != "" && account.ProtocolEndpoint(protocol) != "" {
			return protocol, nil
		}
	}
	if protocol := openAIGatewayKeyProtocol(account, inboundProtocol); protocol != "" {
		return protocol, nil
	}
	return "", MissingProtocolEndpointError(account, strings.Join(KeyUpstreamProtocols(PlatformOpenAI, inboundProtocol, account.Vendor()), " / "))
}

// openAIGatewayKeyProtocol 返回第三方 key 在 OpenAI 网关上处理该入站协议的首选上游协议，
// 不能承接时返回空串。
//
// OpenAI 网关上各分组平台（openai / grok / 国产供应商 / OpenCode）的协议候选相同，
// 转发层拿不到分组平台，用 PlatformOpenAI 代表这个网关。
func openAIGatewayKeyProtocol(account *Account, inboundProtocol string) string {
	return account.KeyUpstreamProtocolFor(PlatformOpenAI, inboundProtocol)
}

// openAIGatewayKeyExtensionBaseURL 返回第三方 key 承接图片、向量、搜索等 OpenAI 扩展
// 端点的上游根地址（入站协议为空时的候选协议地址），未配置时返回空串。
func openAIGatewayKeyExtensionBaseURL(account *Account) string {
	return account.ProtocolEndpoint(openAIGatewayKeyProtocol(account, ""))
}

// shouldForwardOpenAIResponsesViaRawChatCompletions 报告入站 Responses 请求是否会被
// 转成 Chat Completions 发往上游。只有第三方 key 会走这条路：它的 Responses 入站
// 首选协议是 chat_completions（没配 responses 地址）。
//
// OpenCode 官方地址按模型分流，不带模型时判断不了，按不转换处理。
func shouldForwardOpenAIResponsesViaRawChatCompletions(account *Account) bool {
	if account == nil || !account.IsThirdPartyKey() || account.Vendor() == PlatformOpenCodeGo {
		return false
	}
	return openAIGatewayKeyProtocol(account, APIProtocolResponses) == APIProtocolChatCompletions
}

// hasStatelessVendorResponses 报告该厂商的官方 Responses 端点是否为无状态实现
// （不接受 store=true 与 previous_response_id）。
func hasStatelessVendorResponses(vendor string) bool {
	switch vendor {
	case PlatformDeepseek, PlatformKimi, PlatformMiniMax, PlatformOpenCodeGo:
		return true
	default:
		return false
	}
}
