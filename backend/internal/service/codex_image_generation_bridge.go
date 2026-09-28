package service

// Codex /responses 请求里客户端自带 image_generation 工具的处理策略。
// 渠道级覆盖（codex_image_generation_bridge / codex_image_generation_explicit_tool_policy）
// 2026-09-28 P5 删了：生图开着就放行（allow），关着就剥掉（strip），见 codexImageGenerationToolPolicy；
// 桥接注入只看全局 GATEWAY_CODEX_IMAGE_GENERATION_BRIDGE_ENABLED，见 isCodexImageGenerationBridgeEnabled。
const (
	codexImageGenerationExplicitToolPolicyAllow = "allow"
	codexImageGenerationExplicitToolPolicyStrip = "strip"
)
