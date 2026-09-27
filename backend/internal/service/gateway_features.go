package service

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

// 网关行为（2026-09-27 P4：转发、重试冷却、Claude Code / Codex、余额探测写进代码，后台不再能改）。
// 取值全部等于改之前的默认值；共享 dev 库里这些设置当时也都是默认值（逐项核过）。
// 要改就改这里、重新发版。后台只留最低毛利率（利润门）与 Web Search 模拟。
// 带规则的几项（Beta / Fast 策略、Codex 限制、整流、流超时、停调阈值）是结构体没法写成 const，
// 用包级变量；只有测试会临时替换它们，运行时只读。

// 重试与冷却
const (
	// OverloadCooldownEnabled 上游回 529（过载）时暂停该渠道的调度。
	OverloadCooldownEnabled = true
	// OverloadCooldownMinutes 529 后暂停多久（分钟）。
	OverloadCooldownMinutes = 10
	// RateLimit429FallbackEnabled 上游回 429 但算不出重置时间时，按默认时长回避。
	RateLimit429FallbackEnabled = true
	// RateLimit429FallbackSeconds 429 默认回避时长（秒）。
	RateLimit429FallbackSeconds = 5
	// OpenAIImagesOAuthUnavailableCooldownMinutes OpenAI 成品号的生图工具不可用时，暂停该号生图多久（分钟）。
	OpenAIImagesOAuthUnavailableCooldownMinutes = 30
)

// streamTimeoutPolicy 流式输出中途超时后怎么处理渠道：关（只断开本次请求，不动渠道状态）。
// 超时本身怎么判定由部署配置 gateway.stream_data_interval_timeout 决定。
// 打开时：窗口内累计超时达到次数后按 Action 处理。
var streamTimeoutPolicy = StreamTimeoutSettings{
	Enabled:                false,
	Action:                 StreamTimeoutActionTempUnsched,
	TempUnschedMinutes:     5,
	ThresholdCount:         3,
	ThresholdWindowMinutes: 10,
}

// accountSchedulingThresholds 各平台渠道用量到多少（百分比）就自动停调，100 = 不停（全部关）。
var accountSchedulingThresholds = map[string]int{
	PlatformOpenAI:    100,
	PlatformAnthropic: 100,
	PlatformGrok:      100,
}

// 转发行为
const (
	// OpenAITTFTMode OpenAI Responses 首 token 耗时的统计口径：semantic = 第一个有内容的事件。
	OpenAITTFTMode = OpenAITTFTModeSemantic
	// FingerprintUnificationEnabled Claude 成品号出站统一客户端指纹。
	FingerprintUnificationEnabled = true
	// MetadataPassthroughEnabled 把客户端的 metadata.user_id 原样透传给 Claude 上游（关 = 按渠道重写）。
	MetadataPassthroughEnabled = false

	// ClaudeOAuthSystemPromptInjectionEnabled 非 Claude Code 客户端走 Claude 成品号时注入 Claude Code 的系统提示词块
	// （块用内置默认：计费头 + Claude Code 身份 + 扩展提示词）。
	ClaudeOAuthSystemPromptInjectionEnabled = true
	// AnthropicCacheTTL1hInjectionEnabled 给 Claude 成品号请求的 cache_control 注入 1 小时 ttl。
	AnthropicCacheTTL1hInjectionEnabled = false
	// RewriteMessageCacheControlEnabled 改写 messages 里的 cache_control 断点。
	RewriteMessageCacheControlEnabled = false
	// ClientDatelineNormalizationEnabled Claude 成品号请求体里的客户端日期行归一化。
	ClientDatelineNormalizationEnabled = true
	// IdentityPatchEnabled Claude 请求转 Gemini（Antigravity）时注入身份补丁，用内置模板。
	IdentityPatchEnabled = true
)

// rectifierPolicy 请求整流：上游因 thinking 签名 / budget 报 400 时改写请求重试。
// API Key 渠道的签名整流单独一个开关，关着。
var rectifierPolicy = RectifierSettings{
	Enabled:                  true,
	ThinkingSignatureEnabled: true,
	ThinkingBudgetEnabled:    true,
	APIKeySignatureEnabled:   false,
}

// signatureRectifierEnabled thinking 签名整流（总开关 && 签名子开关）。
func signatureRectifierEnabled() bool {
	return rectifierPolicy.Enabled && rectifierPolicy.ThinkingSignatureEnabled
}

// budgetRectifierEnabled thinking budget 整流（总开关 && budget 子开关）。
func budgetRectifierEnabled() bool {
	return rectifierPolicy.Enabled && rectifierPolicy.ThinkingBudgetEnabled
}

// betaPolicy Anthropic Beta 头策略：fast-mode 一律过滤；1M 上下文只放行 Sonnet 5 系列，其余过滤。
//
// context-1m-2025-08-07 的默认策略：
//   - 仅 claude-sonnet-5 及后续版本（如 claude-sonnet-5-*）在上游默认支持 1M 上下文。
//   - Sonnet 4.x 及以下、Opus、Haiku 上游都不支持该 beta，透传上去会被上游 400 或降级。
//   - 因此默认对 sonnet-5* 放行、其余全部过滤，与上游能力保持一致。
//
// 白名单需要覆盖每个上游路径的模型 ID 变形：
//   - 直连 Anthropic API（OAuth mimic / API Key / SetupToken）：模型保持客户端原样
//     （如 "claude-sonnet-5"、"claude-sonnet-5-YYYYMMDD"、"claude-sonnet-5-thinking"）。
//   - Vertex AI：normalizeVertexAnthropicModelID 会把 "-YYYYMMDD" 后缀转成 "@YYYYMMDD"
//     （如 "claude-sonnet-5@YYYYMMDD"）。
//   - AWS Bedrock：ResolveBedrockModelID 会输出带跨区域前缀的模型 ID
//     （us./eu./apac./jp./au./us-gov./global. 或无前缀的 "anthropic." 形式）。
//
// 白名单只用后缀通配符（matchModelPattern 语义），因此每个路径都需要显式列出前缀。
// 精确匹配 "claude-sonnet-5" + 后缀 "-*" 与 "@*"，可覆盖直连/Vertex 场景，同时避免误伤
// 未来可能出现的 "claude-sonnet-50" 或 "claude-sonnet-5.x" 之类的意外命名。
var betaPolicy = BetaPolicySettings{
	Rules: []BetaPolicyRule{
		{
			BetaToken: "fast-mode-2026-02-01",
			Action:    BetaPolicyActionFilter,
			Scope:     BetaPolicyScopeAll,
		},
		{
			BetaToken: "context-1m-2025-08-07",
			Action:    BetaPolicyActionPass,
			Scope:     BetaPolicyScopeAll,
			ModelWhitelist: []string{
				// 直连 Anthropic API（客户端请求 model 原样）
				"claude-sonnet-5",
				"claude-sonnet-5-*",
				// Vertex AI 走 normalizeVertexAnthropicModelID 后 "@YYYYMMDD" 格式
				"claude-sonnet-5@*",
				// AWS Bedrock cross-region inference profile
				"us.anthropic.claude-sonnet-5*",
				"eu.anthropic.claude-sonnet-5*",
				"apac.anthropic.claude-sonnet-5*",
				"jp.anthropic.claude-sonnet-5*",
				"au.anthropic.claude-sonnet-5*",
				"us-gov.anthropic.claude-sonnet-5*",
				"global.anthropic.claude-sonnet-5*",
				// AWS Bedrock 无 cross-region 前缀
				"anthropic.claude-sonnet-5*",
			},
			FallbackAction: BetaPolicyActionFilter,
		},
	},
}

// openAIFastPolicy OpenAI service_tier（fast / flex）策略：不设规则，客户端传什么档位就按什么转发。
var openAIFastPolicy = OpenAIFastPolicySettings{}

// GrokDefaultBaseURLMode Grok 成品号默认走哪个上游地址：CLI 网关。
// （"grok" / "grok-latest" 别名指向的默认文本模型是 xai.DefaultTextModel。）
const GrokDefaultBaseURLMode = GrokDefaultBaseURLModeCLI

// Claude Code / Codex 客户端限制：版本上下限都不限；codex_cli_only 渠道不设黑白名单、
// 不放行 app-server 类客户端、引擎指纹信号用内置默认。
const (
	MinClaudeCodeVersion = ""
	MaxClaudeCodeVersion = ""
)

var codexRestrictionPolicy = CodexRestrictionPolicy{EngineFingerprintSignals: openai.DefaultEngineFingerprintSignals}

// OpenAICodexVersionAutoSyncEnabled 定时从官方仓库同步 Codex 客户端最新稳定版，出站身份跟着走。
const OpenAICodexVersionAutoSyncEnabled = true

// 余额探测
const (
	// UpstreamBillingProbeEnabled 定时探测上游倍率（渠道自己还有单独的开关）。
	UpstreamBillingProbeEnabled = true
	// UpstreamBillingProbeIntervalMinutes 探测间隔（分钟）。
	UpstreamBillingProbeIntervalMinutes = 30
	// OllamaCloudUsageIntervalMinutes 定时拉 Ollama Cloud 用量时，请求一直不停最长隔多久也要刷新一次（分钟）。
	OllamaCloudUsageIntervalMinutes = 60
	// OllamaCloudUsageDebounceMinutes 最后一次请求之后静默多久再刷新（分钟）。
	OllamaCloudUsageDebounceMinutes = 1
)

// ollamaCloudUsageEnabled 定时拉 Ollama Cloud 用量：关（手动刷新不受影响）。
// 定时刷新整套逻辑挂在这个开关后面，写成变量是为了测试能打开它、让那部分代码保持有测试。
var ollamaCloudUsageEnabled = false
