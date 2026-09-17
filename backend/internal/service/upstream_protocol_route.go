package service

// IsOpenAIGatewayPlatform 报告该分组平台的请求是否由 OpenAI 网关转发。
func IsOpenAIGatewayPlatform(platform string) bool {
	switch platform {
	case PlatformOpenAI, PlatformGrok,
		PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax, PlatformOpenCodeGo:
		return true
	default:
		return false
	}
}

// KeyUpstreamProtocols 返回第三方 key 在某分组平台的网关上处理某入站协议时，
// 可以使用的上游协议，按偏好排序。
//
// 分组平台决定网关，网关决定能做哪些协议转换：
//   - anthropic 分组（Anthropic 网关）只以 anthropic 协议转发；
//   - gemini 分组（Gemini 网关）只以 gemini 协议转发；
//   - antigravity 分组：入站 gemini 走 gemini，其余走 anthropic；
//   - OpenAI 网关的分组能在 responses、chat_completions、anthropic 之间互转。
//
// OpenAI 网关上同协议直连优先，没有才转换。例外是官方 OpenAI 地址：入站
// Chat Completions 与 Messages 也先转 Responses——codex 系列模型只在 Responses
// 上提供，这是 OpenAI 的厂商特性，不套到中转上。
//
// 入站协议为空（图片、向量、搜索等 OpenAI 扩展端点）时只认 chat_completions
// 地址：这些端点与 Chat Completions 挂在同一个 OpenAI API 根地址下。
func KeyUpstreamProtocols(groupPlatform, inboundProtocol, vendor string) []string {
	switch {
	case groupPlatform == PlatformAnthropic:
		return []string{APIProtocolAnthropic}
	case groupPlatform == PlatformGemini:
		return []string{APIProtocolGemini}
	case groupPlatform == PlatformAntigravity:
		if inboundProtocol == APIProtocolGemini {
			return []string{APIProtocolGemini}
		}
		return []string{APIProtocolAnthropic}
	case IsOpenAIGatewayPlatform(groupPlatform):
		return openAIGatewayKeyUpstreamProtocols(inboundProtocol, vendor)
	default:
		return nil
	}
}

func openAIGatewayKeyUpstreamProtocols(inboundProtocol, vendor string) []string {
	if inboundProtocol == "" {
		return []string{APIProtocolChatCompletions}
	}
	if vendor == PlatformOpenAI {
		return []string{APIProtocolResponses, APIProtocolChatCompletions}
	}
	switch inboundProtocol {
	case APIProtocolResponses:
		return []string{APIProtocolResponses, APIProtocolChatCompletions, APIProtocolAnthropic}
	case APIProtocolChatCompletions:
		return []string{APIProtocolChatCompletions, APIProtocolResponses, APIProtocolAnthropic}
	case APIProtocolAnthropic:
		return []string{APIProtocolAnthropic, APIProtocolResponses, APIProtocolChatCompletions}
	default:
		return nil
	}
}

// KeyUpstreamProtocolFor 返回第三方 key 在该网关上处理该入站协议时实际使用的上游
// 协议：按 KeyUpstreamProtocols 的偏好取第一个配了地址的。返回空串表示这个 key
// 不能承接该请求，调度必须排除它。
//
// 只对第三方 key 有意义；成品号的协议由厂商决定，返回空串。
func (a *Account) KeyUpstreamProtocolFor(groupPlatform, inboundProtocol string) string {
	if a == nil || !a.IsThirdPartyKey() {
		return ""
	}
	for _, protocol := range KeyUpstreamProtocols(groupPlatform, inboundProtocol, a.Vendor()) {
		if a.ProtocolEndpoint(protocol) != "" {
			return protocol
		}
	}
	return ""
}
