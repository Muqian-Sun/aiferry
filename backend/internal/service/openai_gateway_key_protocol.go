package service

import (
	"strings"
)

// OpenAI 网关上第三方 key 的上游协议与协议特性判定。
//
// 第三方 key 选的平台只是展示标签：上游协议由协议地址与转换注册表决定（UpstreamProtocolFor），
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
	return "", MissingProtocolEndpointError(account, strings.Join(UpstreamProtocolPreference(inboundProtocol, account.Vendor()), " / "))
}

// openAIGatewayKeyProtocol 返回第三方 key 处理该入站协议的上游协议（协议转换注册表：
// 同协议直连优先，官方 OpenAI 先转 Responses），不能承接时返回空串。
func openAIGatewayKeyProtocol(account *Account, inboundProtocol string) string {
	return account.UpstreamProtocolFor(inboundProtocol)
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

// openAIProtocolFeaturesApply 报告 OpenAI 标准协议层面的特性与错误形态是否对账号生效。
//
// 成品号看厂商是否为 openai（等同平台）；第三方 key 看 Vendor：官方 OpenAI 地址生效，
// 通用中转（Vendor 为空）按标准协议实现对待也生效，其他已知厂商不生效。
func openAIProtocolFeaturesApply(account *Account) bool {
	if account == nil {
		return false
	}
	return openAIProtocolFeaturesApplyToVendor(account.Vendor(), account.IsThirdPartyKey())
}

// openAIProtocolFeaturesApplyToVendor 是 openAIProtocolFeaturesApply 的已算好 Vendor 版本，
// 供热路径上已经取过 Vendor 的调用方复用，避免重复解析协议地址。
func openAIProtocolFeaturesApplyToVendor(vendor string, thirdPartyKey bool) bool {
	switch vendor {
	case PlatformOpenAI:
		return true
	case "":
		return thirdPartyKey
	default:
		return false
	}
}

// keyUsesOpenAIProtocolFeatures 报告第三方 key 专属的 OpenAI Responses 协议处理（续链
// previous_response_id、parallel_tool_calls / store=false 修正、客户端工具降级、compat
// prompt_cache_key 等）是否启用：key 的厂商是官方 OpenAI 或通用中转。
func keyUsesOpenAIProtocolFeatures(account *Account) bool {
	return account.IsThirdPartyKey() && openAIProtocolFeaturesApply(account)
}

// AccountKeepsHTTPPreviousResponseID 报告账号能否在 OpenAI 网关的 HTTP Responses 请求里
// 承接 previous_response_id（续链状态）。
//
// 成品号（OAuth / SetupToken）的续链状态挂在 WSv2 会话上，HTTP 请求一律不承接。
// 第三方 key 要求请求确实以 responses 协议转发（没被转换成别的协议，否则续链状态会被
// 静默丢弃），且厂商是官方 OpenAI 或通用中转。
func AccountKeepsHTTPPreviousResponseID(account *Account) bool {
	return account != nil && keyUsesOpenAIProtocolFeatures(account) &&
		openAIGatewayKeyProtocol(account, APIProtocolResponses) == APIProtocolResponses
}

// openAIToolSchemaPlatform 给工具 schema 修正选择平台口径（null type 修复、正则
// lookaround 剥离的规则按平台区分）。成品号用平台；第三方 key 看实际上游：转成
// Anthropic 协议时按 Anthropic 处理，否则按地址识别的厂商，通用中转按 OpenAI 处理。
func openAIToolSchemaPlatform(account *Account, keyProtocol string) string {
	if !account.IsThirdPartyKey() {
		return account.Platform
	}
	if keyProtocol == APIProtocolAnthropic {
		return PlatformAnthropic
	}
	if vendor := account.Vendor(); vendor != "" {
		return vendor
	}
	return PlatformOpenAI
}
