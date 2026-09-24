package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestShouldForwardOpenAIResponsesViaRawChatCompletions_OpenCodeKey：按模型分流删掉后，
// OpenCode 官方地址与其他第三方 key 同一口径——没配 responses 地址就判定为「Responses
// 转 Chat Completions」，不再因为「不带模型判断不了」而例外。
func TestShouldForwardOpenAIResponsesViaRawChatCompletions_OpenCodeKey(t *testing.T) {
	t.Parallel()
	chatOnly := &Account{
		Platform:          PlatformOpenCodeGo,
		Type:              AccountTypeAPIKey,
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: DefaultOpenCodeZenBaseURL},
	}
	require.True(t, shouldForwardOpenAIResponsesViaRawChatCompletions(chatOnly))

	responsesOnly := &Account{
		Platform:          PlatformOpenCodeGo,
		Type:              AccountTypeAPIKey,
		ProtocolEndpoints: map[string]string{APIProtocolResponses: DefaultOpenCodeZenBaseURL},
	}
	require.False(t, shouldForwardOpenAIResponsesViaRawChatCompletions(responsesOnly))
}

// TestOpenCodeEndpointMode：Go 套餐与 Zen 按量只按协议地址区分，不看凭据里的 account_mode。
func TestOpenCodeEndpointMode(t *testing.T) {
	t.Parallel()
	goPlan := &Account{
		Platform:          PlatformOpenCodeGo,
		Type:              AccountTypeAPIKey,
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: DefaultOpenCodeGoBaseURL},
	}
	require.Equal(t, AccountModeGo, goPlan.openCodeEndpointMode())

	zen := &Account{
		Platform:          PlatformOpenCodeGo,
		Type:              AccountTypeAPIKey,
		Credentials:       map[string]any{"account_mode": AccountModeGo},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: DefaultOpenCodeZenBaseURL},
	}
	require.Equal(t, AccountModeZen, zen.openCodeEndpointMode())
}

func TestStampOpenAIResponsesUpstreamEndpoint(t *testing.T) {
	t.Parallel()
	result := &OpenAIForwardResult{}
	stampOpenAIResponsesUpstreamEndpoint(nil, result)
	require.Equal(t, "/v1/responses", result.UpstreamEndpoint)

	existing := &OpenAIForwardResult{UpstreamEndpoint: "/v1/messages"}
	stampOpenAIResponsesUpstreamEndpoint(nil, existing)
	require.Equal(t, "/v1/messages", existing.UpstreamEndpoint)
}

func TestOpenCodeGoQuotaURL(t *testing.T) {
	t.Parallel()
	require.Equal(t, "https://opencode.ai/zen/go/v1/usage", openCodeGoQuotaURL(DefaultOpenCodeGoBaseURL+"/"))
	require.Equal(t, "https://custom.example/v1/usage", openCodeGoQuotaURL("https://custom.example/v1"))
}

func TestDefaultOpenCodeGoModelIDsCoverDocumentedCatalog(t *testing.T) {
	t.Parallel()
	ids := DefaultOpenCodeGoModelIDs()
	require.Contains(t, ids, "grok-4.6")
	require.Contains(t, ids, "minimax-m3")
	require.Contains(t, ids, "glm-5.3")
	require.NotEmpty(t, ids)
}
