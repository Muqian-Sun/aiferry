//go:build unit

package handler

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 3b-4b：/v1/chat/completions 由 Gateway handler 承接全部资源，responses / chat_completions 上游经 OpenAI 服务转换。

func TestGatewayHandlerChatCompletions_ResponsesKeyForwardsViaOpenAIService(t *testing.T) {
	const entryID = 199
	group := keyRouteGroup(2301, service.PlatformAnthropic)
	key := keyRouteAccount(1301, group.ID, service.PlatformOpenAI,
		map[string]string{service.APIProtocolResponses: "https://relay.example.com"}, "gpt-5.6")
	key.CatalogEntryIDs = []int64{entryID}
	hs := newKeyRouteHarness(t, group, []*service.Account{key})

	body := []byte(`{"model":"gpt-5.6","messages":[{"role":"user","content":"hello"}]}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/chat/completions", body, group, service.APIProtocolChatCompletions, "")
	openAIRouteEntry(c, entryID, "gpt-5.6")

	hs.handler.ChatCompletions(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"chat.completion"`)
	got := hs.openAIUpstream.recorded()
	require.Len(t, got, 1)
	require.True(t, strings.HasSuffix(got[0].url, "/v1/responses"), got[0].url)
	require.Empty(t, hs.geminiUpstream.recorded())
}

func TestGatewayHandlerChatCompletions_ChatKeyDirect(t *testing.T) {
	const entryID = 199
	group := keyRouteGroup(2302, service.PlatformAnthropic)
	key := keyRouteAccount(1302, group.ID, service.PlatformOpenAI,
		map[string]string{service.APIProtocolChatCompletions: "https://relay.example.com"}, "gpt-5.6")
	key.CatalogEntryIDs = []int64{entryID}
	hs := newKeyRouteHarness(t, group, []*service.Account{key})
	hs.openAIUpstream.respBody = openAIChatCompletionOK
	hs.openAIUpstream.contentType = "application/json"

	body := []byte(`{"model":"gpt-5.6","messages":[{"role":"user","content":"hello"}]}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/chat/completions", body, group, service.APIProtocolChatCompletions, "")
	openAIRouteEntry(c, entryID, "gpt-5.6")

	hs.handler.ChatCompletions(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := hs.openAIUpstream.recorded()
	require.Len(t, got, 1)
	require.True(t, strings.HasSuffix(got[0].url, "/v1/chat/completions"), got[0].url)
}

// 协议直连是第一排序键：chat 地址的 key 即使优先级更低也赢过要转换的 responses key。
func TestGatewayHandlerChatCompletions_ProtocolMatchBeatsPriority(t *testing.T) {
	const entryID = 199
	group := keyRouteGroup(2303, service.PlatformAnthropic)
	chatKey := keyRouteAccount(1303, group.ID, service.PlatformOpenAI,
		map[string]string{service.APIProtocolChatCompletions: "https://chat.example.com"}, "gpt-5.6")
	chatKey.Priority = 50
	chatKey.CatalogEntryIDs = []int64{entryID}
	responsesKey := keyRouteAccount(1304, group.ID, service.PlatformOpenAI,
		map[string]string{service.APIProtocolResponses: "https://responses.example.com"}, "gpt-5.6")
	responsesKey.Priority = 1
	responsesKey.CatalogEntryIDs = []int64{entryID}
	hs := newKeyRouteHarness(t, group, []*service.Account{chatKey, responsesKey})
	hs.openAIUpstream.respBody = openAIChatCompletionOK
	hs.openAIUpstream.contentType = "application/json"

	body := []byte(`{"model":"gpt-5.6","messages":[{"role":"user","content":"hello"}]}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/chat/completions", body, group, service.APIProtocolChatCompletions, "")
	openAIRouteEntry(c, entryID, "gpt-5.6")

	hs.handler.ChatCompletions(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := hs.openAIUpstream.recorded()
	require.Len(t, got, 1)
	require.True(t, strings.HasPrefix(got[0].url, "https://chat.example.com/"), got[0].url)
}

func TestGatewayHandlerChatCompletions_OpenAITargetRecordsOpenAIUsage(t *testing.T) {
	const entryID = 199
	group := keyRouteGroup(2304, service.PlatformAnthropic)
	key := keyRouteAccount(1305, group.ID, service.PlatformOpenAI,
		map[string]string{service.APIProtocolResponses: "https://relay.example.com"}, "gpt-5.6")
	key.CatalogEntryIDs = []int64{entryID}
	hs := newKeyRouteHarness(t, group, []*service.Account{key})

	body := []byte(`{"model":"gpt-5.6","messages":[{"role":"user","content":"hello"}]}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/chat/completions", body, group, service.APIProtocolChatCompletions, "")
	openAIRouteEntry(c, entryID, "gpt-5.6")

	hs.handler.ChatCompletions(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	logs := hs.usageLogs.recorded()
	require.Len(t, logs, 1)
	require.Equal(t, "gpt-5.6", logs[0].Model)
	require.Equal(t, key.ID, logs[0].AccountID)
}
