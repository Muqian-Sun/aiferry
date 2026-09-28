package service

// 渠道表单收口 P5（2026-09-28，A3：Gemini / Antigravity / OpenAI / Grok）：
// 下面这些原来是每个渠道自己的开关或数值，现在写进代码，库里旧值不再生效。
// 要改就改这里、重新发版。
//
// 直接删掉、没有常量的（功能分支一起删了，或删后等于不限制）：
//   - OpenAI 自动透传（openai_passthrough）：写死关，透传转发分支已删，一律走常规兼容链路。
//   - 仅允许 Codex 官方客户端 / 允许 app-server（codex_cli_only*）：删，连同探测器与拒绝点。
//   - Codex 指纹收敛（codex_fingerprint_mode / _seed）：写死关，收敛与种子逻辑已删。
//   - 渠道级 WS mode（openai_*_responses_websockets_v2_*）：legacy 路径恒走 HTTP；
//     要开 WS 用 GATEWAY_OPENAI_WS_MODE_ROUTER_V2_ENABLED（取全局 ingress_mode_default）。
//   - 摊平 Codex namespace：删（只有 compact 请求照旧摊平）。
//   - Codex 图片桥接渠道级覆盖：删，只看全局 GATEWAY_CODEX_IMAGE_GENERATION_BRIDGE_ENABLED。
//   - 端点能力（openai_capabilities）：删，不按渠道限制。
//   - Compact 模式（openai_compact_mode）：写死 auto，只看探测结果 openai_compact_supported。
//   - Compact 专属模型映射（compact_model_mapping）：渠道级删；全局 GATEWAY_OPENAI_COMPACT_MODEL 不动。
//   - 渠道级 5h / 7d 自动暂停阈值与禁用开关：删，只用运维设置里的全局阈值。
//   - Grok 媒体生成资格手动覆盖（grok_media_eligible）：删，只按探测结果自动判断（接口一起删）。

const (
	// OpenAIImagesURLToB64JSONEnabled Images 端点非流式响应里只有 url、没有 b64_json 的图片项，
	// 由网关下载后回填 b64_json：开（原渠道开关 images_url_to_b64_json 默认关，P5 定写死开；
	// 下载拒私网 / 回环、限大小与格式，见 openai_images_b64_backfill.go）。
	OpenAIImagesURLToB64JSONEnabled = true

	// OpenAIAutoResetCreditThreshold5h 自动用卡的 5h 窗口触发阈值（0-1 比例）：1.0 = 用满 100% 才用卡
	// （原渠道级阈值默认就是 100%，P5 写死）。
	OpenAIAutoResetCreditThreshold5h = 1.0
	// OpenAIAutoResetCreditThreshold7d 自动用卡的 7d 窗口触发阈值（0-1 比例）：1.0，同上。
	OpenAIAutoResetCreditThreshold7d = 1.0

	// GrokClientToolCacheEnabled Grok 客户端工具缓存路由：开。只对已识别的 Grok Free 成品号生效
	// （与原来渠道没设这个键时一致）；请求头 opt-in / opt-out 仍按请求生效。
	GrokClientToolCacheEnabled = true
)
