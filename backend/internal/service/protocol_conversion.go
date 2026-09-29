package service

// 协议转换注册表：网关的本职是「入站协议 → 上游协议」的转换。资源能不能承接一次请求，
// 只看它拥有的上游协议里有没有一个存在从本次入站协议出发的转换实现；先选同协议直连。
// 这里不认「网关族 / 平台」，厂商参数差异归 Vendor() 门控（转发层），不进注册表。

// UpstreamProtocolAntigravity 是 Antigravity 成品号的上游协议（v1internal 封装的 generate）。
// 它不是 protocol_endpoints 的键——第三方 key 配不出它，只有 antigravity 成品号有。
const UpstreamProtocolAntigravity = "antigravity"

// upstreamProtocolOrder[inbound] 列出从该入站协议出发已有转换实现的上游协议，按偏好排序：
// 同协议直连第一，其余按转换保真度。没列出的组合 = 没有实现 = 该资源不能承接。
//
// 已实现的转换（读代码核过）：message → 五种都有（GatewayService.Forward / Gemini 兼容层 /
// Antigravity / OpenAIGatewayService.ForwardAsAnthropic 的 responses 与 chat_completions 分流）；
// completion → 五种（Gateway.ChatCompletions 处理器 + ForwardAsChatCompletions）；
// response → 除 gemini 外四种（没有 Responses → Gemini 的实现）；generate → 只有 gemini / antigravity。
// 入站为空 = OpenAI 扩展端点（图片 / 向量 / 搜索 / 视频…），不是一种协议：第三方 key 只认
// chat_completions 地址（这些端点与 Chat Completions 挂在同一个 API 根地址下），成品号看厂商
// 有没有这些端点——openai / grok 成品号的上游是 responses，也在列。
var upstreamProtocolOrder = map[string][]string{
	APIProtocolAnthropic:       {APIProtocolAnthropic, APIProtocolResponses, APIProtocolChatCompletions, APIProtocolGemini, UpstreamProtocolAntigravity},
	APIProtocolChatCompletions: {APIProtocolChatCompletions, APIProtocolResponses, APIProtocolAnthropic, APIProtocolGemini, UpstreamProtocolAntigravity},
	APIProtocolResponses:       {APIProtocolResponses, APIProtocolChatCompletions, APIProtocolAnthropic, UpstreamProtocolAntigravity},
	APIProtocolGemini:          {APIProtocolGemini, UpstreamProtocolAntigravity},
	"":                         {APIProtocolChatCompletions, APIProtocolResponses},
}

// subscriptionUpstreamProtocol 成品号的上游协议由厂商固定；其他平台没有成品号。
var subscriptionUpstreamProtocol = map[string]string{
	PlatformAnthropic:   APIProtocolAnthropic,
	PlatformGemini:      APIProtocolGemini,
	PlatformAntigravity: UpstreamProtocolAntigravity,
	PlatformOpenAI:      APIProtocolResponses,
	PlatformGrok:        APIProtocolResponses,
}

// ConversionExists 报告入站协议能否转成该上游协议（含同协议直连）。
func ConversionExists(inbound, upstream string) bool {
	for _, candidate := range upstreamProtocolOrder[inbound] {
		if candidate == upstream {
			return true
		}
	}
	return false
}

// UpstreamProtocols 返回资源拥有的上游协议：第三方 key = 配了地址的协议（按 UpstreamProtocols() 固定序），
// 成品号 = 厂商固定的那一个；厂商没有成品号的返回 nil。
func (a *Account) UpstreamProtocols() []string {
	if a == nil {
		return nil
	}
	if a.IsThirdPartyKey() {
		var protocols []string
		for _, protocol := range UpstreamProtocols() {
			if a.ProtocolEndpoint(protocol) != "" {
				protocols = append(protocols, protocol)
			}
		}
		return protocols
	}
	if protocol, ok := subscriptionUpstreamProtocol[a.Vendor()]; ok {
		return []string{protocol}
	}
	return nil
}

// UpstreamProtocolFor 返回资源承接 inbound 实际用的上游协议：注册表偏好序里第一个资源拥有的。
// 空串 = 不能承接，调度必须排除它。偏好序不分厂商：第三方 key 一个资源只有一个协议，成品号
// 的协议由厂商固定，偏好序只决定「能不能承接」。
func (a *Account) UpstreamProtocolFor(inbound string) string {
	owned := a.UpstreamProtocols()
	if len(owned) == 0 {
		return ""
	}
	if inbound == "" && a.IsThirdPartyKey() {
		// 扩展端点对第三方 key 只认 chat_completions 地址（见 upstreamProtocolOrder 的说明）。
		if a.ProtocolEndpoint(APIProtocolChatCompletions) != "" {
			return APIProtocolChatCompletions
		}
		return ""
	}
	for _, candidate := range upstreamProtocolOrder[inbound] {
		for _, protocol := range owned {
			if protocol == candidate {
				return protocol
			}
		}
	}
	return ""
}

// ServesInbound 报告资源能否承接该入站协议。
func (a *Account) ServesInbound(inbound string) bool {
	return a.UpstreamProtocolFor(inbound) != ""
}

// ProtocolMatches 报告资源能以入站协议直连（选号排序的第一键）。
func (a *Account) ProtocolMatches(inbound string) bool {
	return inbound != "" && a.UpstreamProtocolFor(inbound) == inbound
}
