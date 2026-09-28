package service

// Anthropic 成品号 / Bedrock / Vertex / Anthropic 协议 key 的渠道级行为（2026-09-28 P5 渠道表单收口）。
// 这些原来是渠道表单里的开关或输入框，现在取值写在这里，库里各渠道存的旧值不再读。
// 要改就改这里、重新发版。
//
// 同一轮直接删掉、代码里只剩一种行为、不需要常量的：
//   - Anthropic 自动透传（anthropic_passthrough）：写死关，透传分支已删，第三方 key 一律走兼容链路。
//   - 5h 窗口费用阈值 / 粘性预留（window_cost_limit / window_cost_sticky_reserve）：删，成品号不再有渠道级费用闸门。
//   - RPM 策略（rpm_strategy）：只剩 tiered（到 base_rpm 只放粘性会话，再超出粘性缓冲就不调度）。
//   - RPM 粘性缓冲（rpm_sticky_buffer）：只剩自动（并发数 + 最大会话数，下限 base_rpm/5）。
//   - 用户消息限速（user_msg_queue_mode）：删渠道级，走部署配置 gateway.user_message_queue.mode
//     （环境变量 GATEWAY_USER_MESSAGE_QUEUE_MODE）。
//   - TLS 指纹模板（tls_fingerprint_profile_id）：删，只用内置模板。
//   - 缓存 TTL 强制替换（cache_ttl_override_enabled / cache_ttl_override_target）：写死关，渠道级分支已删。
//   - Bedrock 池模式（pool_mode 及重试次数 / 状态码）：删，池模式只对第三方 key。
//   - AWS Session Token（aws_session_token）：删，Bedrock SigV4 只用长期 Access Key。
//   - Vertex 的 project_id / client_email 副本：删，一律从 Service Account JSON 里取。

const (
	// SessionIdleTimeoutMinutes 最大会话数按「多久没请求算会话结束」数活跃会话：5 分钟。
	// 依据：原来渠道不填时就是 5（原 Account.GetSessionIdleTimeoutMinutes 的默认值）。
	SessionIdleTimeoutMinutes = 5

	// AnthropicSubscriptionTLSFingerprintEnabled Anthropic 成品号（OAuth / setup-token）出站模拟
	// Claude Code（Node.js）的 TLS 握手：开，一律用内置模板。其它渠道不模拟。
	// 依据：2026-09-28 muqian 定写死开（原来是渠道开关，默认关）。
	AnthropicSubscriptionTLSFingerprintEnabled = true

	// SessionIDMaskingEnabled Anthropic 成品号的会话 ID 伪装（15 分钟内固定 metadata.user_id 里的 session）：关。
	// 依据：原来渠道开关默认关，2026-09-28 定写死关。
	SessionIDMaskingEnabled = false
)

// builtinTLSFingerprintProfileName 内置 TLS 模板的名字（Profile 其余字段为空 = dialer 用内置默认值）。
const builtinTLSFingerprintProfileName = "Built-in Default (Node.js 24.x)"

// webSearchEmulationAppliesTo 渠道是否做 web_search 模拟：第三方 key 且上游不是 Anthropic 官方地址。
// 模拟只在 Anthropic Messages 转发路径上判定（走到那里用的就是 Anthropic 协议）；全站是否生效另看
// 「配了带 Key 的服务商」（SettingService.IsWebSearchEmulationEnabled）。成品号与官方地址的 key 不模拟：
// 官方上游自己支持 web_search。依据：2026-09-28 muqian 定（原来是渠道开关，默认关）。
func webSearchEmulationAppliesTo(account *Account) bool {
	return account != nil && account.IsThirdPartyKey() && account.Vendor() != PlatformAnthropic
}
