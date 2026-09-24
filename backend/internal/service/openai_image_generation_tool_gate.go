package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// 生图未开放期间，语言模型请求不能经 Responses 的 image_generation 工具在上游出图。
// 全局开关 gateway.image_generation_tool_enabled（默认 false）关着时：
//   - Codex 官方客户端（或 force_codex_cli）：沿用账号级 strip 策略的剥离路径，剥掉工具后照常转发；
//   - 其余客户端：请求里声明了该工具就 400 invalid_request_error，不选号、不打上游；
//   - Codex 桥接注入（全局 codex_image_generation_bridge_enabled 与账号级覆盖）一律不生效。
//
// 请求的模型本身就是生图模型（model=gpt-image-* / grok-imagine-*）时属于图片产品，不在这里拦。

// OpenAIImageGenerationToolUnavailableMessage 是拒绝 image_generation 工具时返回给客户端的文案。
const OpenAIImageGenerationToolUnavailableMessage = "the image_generation tool is not available"

// ErrOpenAIImageGenerationToolUnavailable 表示请求带了 image_generation 工具而网关没开放生图。
// 它是客户端请求错误：调用方回 400，不算账号失败、不换号。
var ErrOpenAIImageGenerationToolUnavailable = errors.New(OpenAIImageGenerationToolUnavailableMessage)

// OpenAIImageGenerationToolEnabled 报告全局开关是否允许语言模型请求使用 image_generation 工具。
func OpenAIImageGenerationToolEnabled(cfg *config.Config) bool {
	return cfg != nil && cfg.Gateway.ImageGenerationToolEnabled
}

// IsOpenAICodexCLIClient 与转发层判定 Codex 官方客户端的口径一致（UA / originator，或 force_codex_cli）。
func IsOpenAICodexCLIClient(cfg *config.Config, userAgent, originator string) bool {
	return openai.IsCodexOfficialClientByHeaders(userAgent, originator) || (cfg != nil && cfg.Gateway.ForceCodexCLI)
}

// GateOpenAIImageGenerationTool 在入站时处理请求里的 Responses image_generation 工具。
//
// 返回处理后的 body 与是否剥离过；err 为 ErrOpenAIImageGenerationToolUnavailable 时调用方
// 应直接回 400，不选号、不打上游。开关开着、请求没带该工具、或请求的模型本身就是生图模型时原样返回。
func GateOpenAIImageGenerationTool(cfg *config.Config, userAgent, originator, requestedModel string, body []byte) ([]byte, bool, error) {
	if OpenAIImageGenerationToolEnabled(cfg) || !openAIRequestDeclaresHostedImageGenerationTool(requestedModel, body) {
		return body, false, nil
	}
	if !IsOpenAICodexCLIClient(cfg, userAgent, originator) {
		return body, false, ErrOpenAIImageGenerationToolUnavailable
	}
	stripped, changed, err := stripOpenAIImageGenerationToolsFromRawPayload(body)
	if err != nil {
		return body, false, err
	}
	return stripped, changed, nil
}

// openAIRequestDeclaresHostedImageGenerationTool 报告请求是否声明了会让上游出图的 Responses
// image_generation 工具：顶层 tools、Responses Lite 的 input[].additional_tools，或指向它的 tool_choice。
// Codex 的 image_gen namespace 是客户端函数工具，上游只会回函数调用、不会出图，不算。
// 请求的模型本身是生图模型时返回 false（图片产品，不归语言模型的开关管）。
func openAIRequestDeclaresHostedImageGenerationTool(requestedModel string, body []byte) bool {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return false
	}
	if isOpenAIImageGenerationModel(strings.TrimSpace(requestedModel)) ||
		isOpenAIImageGenerationModel(openAIJSONString(gjson.GetBytes(body, "model"))) {
		return false
	}
	if openAIJSONToolsContainNativeImageGeneration(gjson.GetBytes(body, "tools")) {
		return true
	}
	if openAIJSONToolChoiceSelectsHostedImageGeneration(gjson.GetBytes(body, "tool_choice")) {
		return true
	}
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		return false
	}
	found := false
	input.ForEach(func(_, item gjson.Result) bool {
		if openAIJSONString(item.Get("type")) == "additional_tools" &&
			openAIJSONToolsContainNativeImageGeneration(item.Get("tools")) {
			found = true
		}
		return !found
	})
	return found
}

// openAIJSONToolChoiceSelectsHostedImageGeneration 只认指向托管 image_generation 工具的 tool_choice
// （"image_generation" / {"type":"image_generation"} / {"tool":{"type":"image_generation"}}），
// 不含 image_gen namespace。
func openAIJSONToolChoiceSelectsHostedImageGeneration(choice gjson.Result) bool {
	switch {
	case !choice.Exists():
		return false
	case choice.Type == gjson.String:
		return isOpenAIImageGenerationType(choice.String())
	case !choice.IsObject():
		return false
	case isOpenAIImageGenerationType(openAIJSONString(choice.Get("type"))):
		return true
	default:
		tool := choice.Get("tool")
		return tool.IsObject() && isOpenAIImageGenerationType(openAIJSONString(tool.Get("type")))
	}
}

// writeOpenAIImageGenerationToolUnavailable 按 OpenAI 错误形状回 400。
func writeOpenAIImageGenerationToolUnavailable(c *gin.Context) {
	if c == nil {
		return
	}
	MarkOpsClientBusinessLimited(c, OpsClientBusinessLimitedReasonLocalFeatureGate)
	c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{
		"type":    "invalid_request_error",
		"message": OpenAIImageGenerationToolUnavailableMessage,
		"param":   "tools",
	}})
}

// BuildOpenAIImageGenerationToolUnavailableWSEvent 构造 WS 入站拒绝该工具时先发给客户端的 error 事件。
func BuildOpenAIImageGenerationToolUnavailableWSEvent() []byte {
	payload, err := json.Marshal(map[string]any{
		"event_id": "evt_image_generation_tool_unavailable",
		"type":     "error",
		"error": map[string]any{
			"type":    "invalid_request_error",
			"code":    "image_generation_tool_unavailable",
			"message": OpenAIImageGenerationToolUnavailableMessage,
			"param":   "tools",
		},
	})
	if err != nil {
		return nil
	}
	return payload
}

// codexImageGenerationToolPolicy 返回本次 Codex 请求对显式 image_generation 工具的有效策略：
// 全局开关关着时一律 strip（账号级 allow 不能放开），开着时沿用账号级策略。
func (s *OpenAIGatewayService) codexImageGenerationToolPolicy(account *Account) string {
	if s == nil || !OpenAIImageGenerationToolEnabled(s.cfg) {
		return codexImageGenerationExplicitToolPolicyStrip
	}
	return account.CodexImageGenerationExplicitToolPolicy()
}
