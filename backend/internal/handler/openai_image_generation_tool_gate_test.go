//go:build unit

package handler

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 生图未开放（gateway.image_generation_tool_enabled=false，默认）：带 Responses image_generation 工具的
// 语言模型请求在选号前处理——非 Codex 客户端 400、不打上游；Codex 官方客户端剥掉工具照常转发。

const imageToolGateEntryID = 199

func imageToolGateResponsesBody() []byte {
	return []byte(`{"model":"gpt-5.6","input":"draw a cat","stream":false,"tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}},{"type":"image_generation"}]}`)
}

func TestGatewayHandlerResponses_ImageToolSwitchOffRejectsNonCodexBeforeUpstream(t *testing.T) {
	hs := newKeyRouteHarness(t, []*service.Account{responsesKey(1501, "https://relay.example.com", 1, imageToolGateEntryID)})
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/responses", imageToolGateResponsesBody(), service.APIProtocolResponses, "")
	c.Request.Header.Set("User-Agent", "curl/8.0")
	openAIRouteEntry(c, imageToolGateEntryID, "gpt-5.6")

	hs.handler.Responses(c)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Equal(t, "invalid_request_error", gjson.Get(rec.Body.String(), "error.type").String())
	require.Equal(t, service.OpenAIImageGenerationToolUnavailableMessage, gjson.Get(rec.Body.String(), "error.message").String())
	require.Empty(t, hs.openAIUpstream.recorded(), "rejected before any upstream call")
	require.Empty(t, hs.usageLogs.recorded())
}

func TestGatewayHandlerResponses_ImageToolSwitchOffStripsForCodexCLI(t *testing.T) {
	hs := newKeyRouteHarness(t, []*service.Account{responsesKey(1502, "https://relay.example.com", 1, imageToolGateEntryID)})
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/responses", imageToolGateResponsesBody(), service.APIProtocolResponses, "")
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.98.0")
	openAIRouteEntry(c, imageToolGateEntryID, "gpt-5.6")

	hs.handler.Responses(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := hs.openAIUpstream.recorded()
	require.Len(t, got, 1)
	require.False(t, gjson.GetBytes(got[0].body, `tools.#(type=="image_generation")`).Exists())
	require.True(t, gjson.GetBytes(got[0].body, `tools.#(name=="lookup")`).Exists())
}

func TestGatewayHandlerResponses_ImageToolSwitchOnForwardsUnchanged(t *testing.T) {
	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Gateway.ImageGenerationToolEnabled = true
	hs := newKeyRouteHarnessWithConfig(t, []*service.Account{responsesKey(1503, "https://relay.example.com", 1, imageToolGateEntryID)}, cfg)
	hs.handler.cfg = cfg
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/responses", imageToolGateResponsesBody(), service.APIProtocolResponses, "")
	c.Request.Header.Set("User-Agent", "curl/8.0")
	openAIRouteEntry(c, imageToolGateEntryID, "gpt-5.6")

	hs.handler.Responses(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := hs.openAIUpstream.recorded()
	require.Len(t, got, 1)
	require.True(t, gjson.GetBytes(got[0].body, `tools.#(type=="image_generation")`).Exists())
}

// /v1/chat/completions 收到 Responses 形状的请求体（Cursor 等）会原样转发到 Responses 上游，同样把关。
func TestGatewayHandlerChatCompletions_ResponsesShapedImageToolSwitchOffRejected(t *testing.T) {
	key := keyRouteAccount(1504, service.PlatformOpenAI,
		map[string]string{service.APIProtocolResponses: "https://relay.example.com"}, "gpt-5.6")
	key.CatalogEntryIDs = []int64{imageToolGateEntryID}
	hs := newKeyRouteHarness(t, []*service.Account{key})
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/chat/completions", imageToolGateResponsesBody(), service.APIProtocolChatCompletions, "")
	c.Request.Header.Set("User-Agent", "curl/8.0")
	openAIRouteEntry(c, imageToolGateEntryID, "gpt-5.6")

	hs.handler.ChatCompletions(c)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Equal(t, service.OpenAIImageGenerationToolUnavailableMessage, gjson.Get(rec.Body.String(), "error.message").String())
	require.Empty(t, hs.openAIUpstream.recorded())
}

// Chat 形状的请求体里 image_generation 工具在 Chat→Responses 转换中带不过去，不拦。
func TestGatewayHandlerChatCompletions_ChatShapedImageToolNotGated(t *testing.T) {
	key := keyRouteAccount(1505, service.PlatformOpenAI,
		map[string]string{service.APIProtocolResponses: "https://relay.example.com"}, "gpt-5.6")
	key.CatalogEntryIDs = []int64{imageToolGateEntryID}
	hs := newKeyRouteHarness(t, []*service.Account{key})
	body := []byte(`{"model":"gpt-5.6","messages":[{"role":"user","content":"hello"}],"tools":[{"type":"image_generation"}]}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/chat/completions", body, service.APIProtocolChatCompletions, "")
	c.Request.Header.Set("User-Agent", "curl/8.0")
	openAIRouteEntry(c, imageToolGateEntryID, "gpt-5.6")

	hs.handler.ChatCompletions(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := hs.openAIUpstream.recorded()
	require.Len(t, got, 1)
	require.False(t, gjson.GetBytes(got[0].body, `tools.#(type=="image_generation")`).Exists())
}
