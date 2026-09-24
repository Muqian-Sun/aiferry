//go:build unit

package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 跨协议转换拒收的内容分片（apicompat.UnsupportedContentError）在 handler 层收尾为
// 400 invalid_request_error：转换发生在任何上游请求之前，不换号、不落通用 502 兜底。
//
// "不换号"的证明方式：池里放两个资源，A（优先级高）的上游协议表达不了该分片，B（优先级低）
// 能表达。A 先被选中并以 400 拒收；若 handler 换号，B 的上游会收到请求。对照组只放 B，
// 证明 B 确实能承接同一请求。

func requireInvalidRequest400(t *testing.T, status int, body []byte, wantSubstrings ...string) {
	t.Helper()
	require.Equal(t, http.StatusBadRequest, status, string(body))
	require.True(t, json.Valid(body), "error body must be a single JSON document (no SSE fallback appended): %s", body)
	var payload struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(body, &payload))
	require.Equal(t, "invalid_request_error", payload.Error.Type)
	for _, s := range wantSubstrings {
		require.Contains(t, payload.Error.Message, s)
	}
}

// /v1/chat/completions + input_audio：非 chat 上游都表达不了音频。responses 资源由 handler 写 400
// （service 层不写，旧行为是通用 502）；Anthropic / Gemini 资源走 Chat→Responses→… 链，
// 错误必须指向客户端实际打到的上游协议而不是网关内部的中转格式。
func TestGatewayHandlerChatCompletions_UnsupportedAudioReturns400WithoutUpstreamCall(t *testing.T) {
	const entryID = 199
	tests := []struct {
		name         string
		label        string
		endpoints    map[string]string
		wantUpstream string
	}{
		{"responses key", service.PlatformOpenAI, map[string]string{service.APIProtocolResponses: "https://relay.example.com"}, "OpenAI Responses protocol"},
		{"anthropic key", service.PlatformAnthropic, map[string]string{service.APIProtocolAnthropic: "https://anthropic-relay.example.com"}, "Anthropic Messages protocol"},
		{"gemini key", service.PlatformGemini, map[string]string{service.APIProtocolGemini: "https://gemini-relay.example.com"}, "Gemini protocol"},
	}
	body := []byte(`{"model":"gpt-5.6","messages":[{"role":"user","content":[{"type":"text","text":"transcribe"},{"type":"input_audio","input_audio":{"data":"UklGRg==","format":"wav"}}]}]}`)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := keyRouteAccount(1401, tt.label, tt.endpoints, "gpt-5.6")
			key.CatalogEntryIDs = []int64{entryID}
			hs := newKeyRouteHarness(t, []*service.Account{key})

			c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/chat/completions", body, service.APIProtocolChatCompletions, "")
			openAIRouteEntry(c, entryID, "gpt-5.6")

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("request was forwarded upstream: %v", r)
					}
				}()
				hs.handler.ChatCompletions(c)
			}()

			requireInvalidRequest400(t, rec.Code, rec.Body.Bytes(), "input_audio", tt.wantUpstream)
			require.Empty(t, hs.openAIUpstream.recorded(), "no upstream request may be sent")
			require.Empty(t, hs.geminiUpstream.recorded(), "no upstream request may be sent")
			require.Empty(t, hs.usageLogs.recorded(), "a rejected request must not be billed")
		})
	}
}

// /v1/chat/completions + file_id → Anthropic 资源 A（先选中）拒收；能承接 file_id 的
// responses 资源 B 不得被换上。
func TestGatewayHandlerChatCompletions_UnsupportedFileIDDoesNotFailOver(t *testing.T) {
	const entryID = 199
	newPool := func(includeAnthropic bool) (*keyRouteHarness, *service.Account) {
		anthropicKey := keyRouteAccount(1402, service.PlatformAnthropic,
			map[string]string{service.APIProtocolAnthropic: "https://anthropic-relay.example.com"}, "gpt-5.6")
		anthropicKey.Priority = 1
		anthropicKey.CatalogEntryIDs = []int64{entryID}
		responsesKey := keyRouteAccount(1403, service.PlatformOpenAI,
			map[string]string{service.APIProtocolResponses: "https://relay.example.com"}, "gpt-5.6")
		responsesKey.Priority = 50
		responsesKey.CatalogEntryIDs = []int64{entryID}
		accounts := []*service.Account{responsesKey}
		if includeAnthropic {
			accounts = append(accounts, anthropicKey)
		}
		return newKeyRouteHarness(t, accounts), responsesKey
	}
	body := []byte(`{"model":"gpt-5.6","messages":[{"role":"user","content":[{"type":"text","text":"summarise"},{"type":"file","file":{"file_id":"file-abc"}}]}]}`)

	t.Run("control: the responses key alone serves the request", func(t *testing.T) {
		hs, _ := newPool(false)
		c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/chat/completions", body, service.APIProtocolChatCompletions, "")
		openAIRouteEntry(c, entryID, "gpt-5.6")
		hs.handler.ChatCompletions(c)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Len(t, hs.openAIUpstream.recorded(), 1)
	})

	t.Run("anthropic key rejects with 400 and no failover", func(t *testing.T) {
		hs, _ := newPool(true)
		c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/chat/completions", body, service.APIProtocolChatCompletions, "")
		openAIRouteEntry(c, entryID, "gpt-5.6")
		func() {
			// GatewayService 在测试装配里没有 HTTP 上游：若请求被发往 Anthropic 上游会 panic。
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("request was forwarded to the anthropic upstream: %v", r)
				}
			}()
			hs.handler.ChatCompletions(c)
		}()
		requireInvalidRequest400(t, rec.Code, rec.Body.Bytes(), "file referenced by file_id", "Anthropic Messages protocol")
		require.Empty(t, hs.openAIUpstream.recorded(), "must not fail over to the next account")
		require.Empty(t, hs.usageLogs.recorded())
	})
}

// /v1/messages + URL 图片 → Gemini 资源 A（service 层已写 400）拒收；能承接 URL 图片的
// responses 资源 B 不得被换上。
func TestGatewayHandlerMessages_UnsupportedURLImageOnGeminiDoesNotFailOver(t *testing.T) {
	const entryID = 199
	newPool := func(includeGemini bool) *keyRouteHarness {
		geminiKey := keyRouteAccount(1404, service.PlatformGemini,
			map[string]string{service.APIProtocolGemini: "https://gemini-relay.example.com"}, "gpt-5.6")
		geminiKey.Priority = 1
		geminiKey.CatalogEntryIDs = []int64{entryID}
		responsesKey := keyRouteAccount(1405, service.PlatformOpenAI,
			map[string]string{service.APIProtocolResponses: "https://relay.example.com"}, "gpt-5.6")
		responsesKey.Priority = 50
		responsesKey.CatalogEntryIDs = []int64{entryID}
		accounts := []*service.Account{responsesKey}
		if includeGemini {
			accounts = append(accounts, geminiKey)
		}
		return newKeyRouteHarness(t, accounts)
	}
	body := []byte(`{"model":"gpt-5.6","max_tokens":16,"messages":[{"role":"user","content":[{"type":"text","text":"what is this"},{"type":"image","source":{"type":"url","url":"https://example.com/cat.png"}}]}]}`)

	t.Run("control: the responses key alone serves the request", func(t *testing.T) {
		hs := newPool(false)
		c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/messages", body, service.APIProtocolAnthropic, "")
		openAIRouteEntry(c, entryID, "gpt-5.6")
		hs.handler.Messages(c)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		got := hs.openAIUpstream.recorded()
		require.Len(t, got, 1)
		require.Contains(t, string(got[0].body), "https://example.com/cat.png", "URL image must reach the Responses upstream")
	})

	t.Run("gemini key rejects with 400 and no failover", func(t *testing.T) {
		hs := newPool(true)
		c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/messages", body, service.APIProtocolAnthropic, "")
		openAIRouteEntry(c, entryID, "gpt-5.6")
		hs.handler.Messages(c)
		requireInvalidRequest400(t, rec.Code, rec.Body.Bytes(), "image with a URL source", "Gemini protocol")
		require.Empty(t, hs.geminiUpstream.recorded(), "no upstream request may be sent")
		require.Empty(t, hs.openAIUpstream.recorded(), "must not fail over to the next account")
		require.Empty(t, hs.usageLogs.recorded())
	})
}

// /v1/messages + Anthropic file_id 图片 → responses 资源：OpenAI 服务的 Anthropic→Responses 转换
// 不写响应，handler 按 Anthropic 形状写 400（旧行为是通用 502）。
func TestGatewayHandlerMessages_UnsupportedFileSourceOnResponsesKeyReturns400(t *testing.T) {
	const entryID = 199
	key := keyRouteAccount(1408, service.PlatformOpenAI,
		map[string]string{service.APIProtocolResponses: "https://relay.example.com"}, "gpt-5.6")
	key.CatalogEntryIDs = []int64{entryID}
	hs := newKeyRouteHarness(t, []*service.Account{key})

	body := []byte(`{"model":"gpt-5.6","max_tokens":16,"messages":[{"role":"user","content":[{"type":"text","text":"describe"},{"type":"image","source":{"type":"file","file_id":"file_011"}}]}]}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/messages", body, service.APIProtocolAnthropic, "")
	openAIRouteEntry(c, entryID, "gpt-5.6")

	hs.handler.Messages(c)

	requireInvalidRequest400(t, rec.Code, rec.Body.Bytes(), "image with an Anthropic file_id source", "OpenAI Responses protocol")
	require.Contains(t, rec.Body.String(), `"type":"error"`, "Anthropic error envelope")
	require.Empty(t, hs.openAIUpstream.recorded(), "no upstream request may be sent")
	require.Empty(t, hs.usageLogs.recorded())
}

// /v1/responses + input_file(file_url) → chat 资源（service 层已写 400）：handler 不得再追加
// SSE 兜底帧，也不得换到能承接 file_url 的 Anthropic 资源。
func TestGatewayHandlerResponses_UnsupportedFileURLOnChatKeyReturns400(t *testing.T) {
	const entryID = 199
	chatKey := keyRouteAccount(1406, service.PlatformOpenAI,
		map[string]string{service.APIProtocolChatCompletions: "https://relay.example.com"}, "gpt-5.6")
	chatKey.Priority = 1
	chatKey.CatalogEntryIDs = []int64{entryID}
	anthropicKey := keyRouteAccount(1407, service.PlatformAnthropic,
		map[string]string{service.APIProtocolAnthropic: "https://anthropic-relay.example.com"}, "gpt-5.6")
	anthropicKey.Priority = 50
	anthropicKey.CatalogEntryIDs = []int64{entryID}
	hs := newKeyRouteHarness(t, []*service.Account{chatKey, anthropicKey})

	body := []byte(`{"model":"gpt-5.6","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"read"},{"type":"input_file","file_url":"https://example.com/report.pdf"}]}]}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/responses", body, service.APIProtocolResponses, "")
	openAIRouteEntry(c, entryID, "gpt-5.6")

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("request failed over to the anthropic upstream: %v", r)
			}
		}()
		hs.handler.Responses(c)
	}()

	requireInvalidRequest400(t, rec.Code, rec.Body.Bytes(), "file referenced by URL (file_url)", "OpenAI Chat Completions protocol")
	require.Empty(t, hs.openAIUpstream.recorded(), "no upstream request may be sent")
	require.Empty(t, hs.usageLogs.recorded())
}
