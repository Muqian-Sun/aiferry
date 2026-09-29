//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

// 第三方 key 的模型列表同步只看协议地址，不看平台标签。每个用例的账号都刻意让
// 「平台标签」和「协议地址」不一致：标签说 anthropic、地址只有 chat_completions，
// 之类。任何一处回到按标签分派，这里就会打到另一个地址或另一种请求形态而变红。

func keyModelSyncService(body string) (*AccountTestService, *httpUpstreamRecorder) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}}
	return &AccountTestService{httpUpstream: upstream, cfg: upstreamModelSyncTestConfig()}, upstream
}

func keyModelSyncAccount(platform string, endpoints map[string]string) *Account {
	return &Account{
		ID:                701,
		Platform:          platform,
		Type:              AccountTypeAPIKey,
		Credentials:       map[string]any{"api_key": "sk-key"},
		ProtocolEndpoints: endpoints,
	}
}

// 标签 anthropic、只配 chat_completions 地址：走 OpenAI /v1/models + Bearer。
func TestKeyModelSyncAnthropicLabelChatCompletionsEndpointUsesOpenAIShape(t *testing.T) {
	svc, upstream := keyModelSyncService(`{"data":[{"id":"relay-model"}]}`)
	account := keyModelSyncAccount(PlatformAnthropic, map[string]string{
		APIProtocolChatCompletions: "https://relay.example/v1",
	})

	models, err := svc.FetchUpstreamSupportedModels(context.Background(), account)

	require.NoError(t, err)
	require.Equal(t, []string{"relay-model"}, models)
	require.Equal(t, "https://relay.example/v1/models", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-key", upstream.lastReq.Header.Get("Authorization"))
	require.Empty(t, upstream.lastReq.Header.Get("x-api-key"))
	require.Empty(t, upstream.lastReq.Header.Get("anthropic-version"))
}

// 标签 openai、只配 anthropic 地址：走 Anthropic /v1/models + x-api-key。
func TestKeyModelSyncOpenAILabelAnthropicEndpointUsesAnthropicShape(t *testing.T) {
	svc, upstream := keyModelSyncService(`{"data":[{"id":"claude-relay-1"}]}`)
	account := keyModelSyncAccount(PlatformOpenAI, map[string]string{
		APIProtocolAnthropic: "https://relay.example/anthropic",
	})

	models, err := svc.FetchUpstreamSupportedModels(context.Background(), account)

	require.NoError(t, err)
	require.Equal(t, []string{"claude-relay-1"}, models)
	require.Equal(t, "https://relay.example/anthropic/v1/models", upstream.lastReq.URL.String())
	require.Equal(t, "sk-key", upstream.lastReq.Header.Get("x-api-key"))
	require.Equal(t, "2023-06-01", upstream.lastReq.Header.Get("anthropic-version"))
	require.Empty(t, upstream.lastReq.Header.Get("Authorization"))
}

// 标签 anthropic、只配 gemini 地址：走 Gemini /v1beta/models + x-goog-api-key。
func TestKeyModelSyncAnthropicLabelGeminiEndpointUsesGeminiShape(t *testing.T) {
	svc, upstream := keyModelSyncService(`{"models":[{"name":"models/gemini-relay"}]}`)
	account := keyModelSyncAccount(PlatformAnthropic, map[string]string{
		APIProtocolGemini: "https://relay.example/gemini",
	})

	models, err := svc.FetchUpstreamSupportedModels(context.Background(), account)

	require.NoError(t, err)
	require.Equal(t, []string{"gemini-relay"}, models)
	require.Equal(t, "https://relay.example/gemini/v1beta/models", upstream.lastReq.URL.String())
	require.Equal(t, "sk-key", upstream.lastReq.Header.Get("x-goog-api-key"))
}

// 只配 responses 地址的 key 从 responses 根地址拉 /v1/models，不借用没配的
// chat_completions 地址。
func TestKeyModelSyncResponsesOnlyEndpointUsesResponsesBaseURL(t *testing.T) {
	svc, upstream := keyModelSyncService(`{"data":[{"id":"responses-model"}]}`)
	account := keyModelSyncAccount(PlatformAnthropic, map[string]string{
		APIProtocolResponses: "https://responses.example/v1",
	})

	models, err := svc.FetchUpstreamSupportedModels(context.Background(), account)

	require.NoError(t, err)
	require.Equal(t, []string{"responses-model"}, models)
	require.Equal(t, "https://responses.example/v1/models", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-key", upstream.lastReq.Header.Get("Authorization"))
}

// 多协议 key 取 PrimaryUpstreamProtocol：chat_completions 优先，只发一次请求。
func TestKeyModelSyncMultiProtocolSyncsPrimaryProtocolOnly(t *testing.T) {
	svc, upstream := keyModelSyncService(`{"data":[{"id":"primary-model"}]}`)
	account := keyModelSyncAccount(PlatformAnthropic, map[string]string{
		APIProtocolChatCompletions: "https://chat.example/v1",
		APIProtocolAnthropic:       "https://anthropic.example",
		APIProtocolResponses:       "https://responses.example/v1",
	})

	models, err := svc.FetchUpstreamSupportedModels(context.Background(), account)

	require.NoError(t, err)
	require.Equal(t, []string{"primary-model"}, models)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://chat.example/v1/models", upstream.requests[0].URL.String())
}

// 官方 Kimi 地址的 kimi key：与改造前一致，仍是 OpenAI /v1/models + Bearer。
func TestKeyModelSyncOfficialKimiKeyKeepsOpenAIShape(t *testing.T) {
	svc, upstream := keyModelSyncService(`{"data":[{"id":"kimi-k2"}]}`)
	account := keyModelSyncAccount(PlatformKimi, map[string]string{
		APIProtocolChatCompletions: DefaultKimiPayGBaseURL,
		APIProtocolAnthropic:       DefaultKimiPayGAnthropicBaseURL,
	})
	require.Equal(t, PlatformKimi, account.Vendor())

	models, err := svc.FetchUpstreamSupportedModels(context.Background(), account)

	require.NoError(t, err)
	require.Equal(t, []string{"kimi-k2"}, models)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://api.moonshot.cn/v1/models", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-key", upstream.lastReq.Header.Get("Authorization"))
}

// 一个协议地址都没配：报 MISSING_PROTOCOL_ENDPOINT，不静默返回空目录。
func TestKeyModelSyncWithoutAnyEndpointFailsWithClearError(t *testing.T) {
	svc, upstream := keyModelSyncService(`{"data":[{"id":"never-fetched"}]}`)
	account := keyModelSyncAccount(PlatformAnthropic, nil)

	_, err := svc.FetchUpstreamSupportedModels(context.Background(), account)

	require.Error(t, err)
	require.Equal(t, "MISSING_PROTOCOL_ENDPOINT", infraerrors.Reason(err))
	var syncErr *UpstreamModelSyncError
	require.ErrorAs(t, err, &syncErr)
	require.Equal(t, UpstreamModelSyncErrorConfiguration, syncErr.Kind)
	require.Contains(t, syncErr.SafeMessage(), "No upstream address is configured")
	require.Nil(t, upstream.lastReq)
}

// Grok 的目录形态是厂商特化、只对 Grok 成品号：第三方 key 一律按标准 OpenAI 目录解析，贴 grok 标签的中转与
// 指向 api.x.ai 的 key 都一样（2026-09-29 海外四家不再有官方 key）。
func TestKeyModelSyncGrokCatalogShapeFollowsVendorNotLabel(t *testing.T) {
	const body = `{"data":[{"id":"standard-id","model":"grok-4.5"}]}`

	for _, endpoint := range []string{"https://relay.example/v1", "https://api.x.ai/v1"} {
		svc, _ := keyModelSyncService(body)
		account := keyModelSyncAccount(PlatformGrok, map[string]string{APIProtocolChatCompletions: endpoint})
		require.Empty(t, account.Vendor(), endpoint)
		models, err := svc.FetchUpstreamSupportedModels(context.Background(), account)
		require.NoError(t, err)
		require.Equal(t, []string{"standard-id"}, models, endpoint)
	}

	require.True(t, usesGrokModelCatalogShape(&Account{Platform: PlatformGrok, Type: AccountTypeOAuth}))
}

// 跨标签 key 的 /models 端点 404 且没有 model_mapping 可替代时必须报错，
// 不能「同步成功但一个模型都没有」。
func TestKeyModelSyncUnsupportedListEndpointWithoutMappingFails(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader(`{"error":"not found"}`)),
	}}
	svc := &AccountTestService{httpUpstream: upstream, cfg: upstreamModelSyncTestConfig()}
	account := keyModelSyncAccount(PlatformOpenAI, map[string]string{
		APIProtocolAnthropic: "https://relay.example/anthropic",
	})

	catalog, err := svc.SyncUpstreamModelCatalog(context.Background(), account)

	require.Error(t, err)
	require.Nil(t, catalog)
	require.Equal(t, http.StatusNotFound, upstreamModelSyncStatusCode(err))
	require.Equal(t, "https://relay.example/anthropic/v1/models", upstream.lastReq.URL.String())
}
