package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestAccountFromServiceShallow_RedactsSensitiveCredentials(t *testing.T) {
	src := &service.Account{
		ID:       42,
		Name:     "demo",
		Platform: "anthropic",
		Type:     "oauth",
		Credentials: map[string]any{
			"access_token":  "at-secret",
			"refresh_token": "rt-secret",
			"id_token":      "id-secret",
			"api_key":       "sk-secret",
			"base_url":      "https://api.example.com",
			"model_mapping": map[string]any{"foo": "bar"},
		},
	}

	got := AccountFromServiceShallow(src)
	require.NotNil(t, got)

	// 敏感键不在 Credentials 里
	require.NotContains(t, got.Credentials, "access_token")
	require.NotContains(t, got.Credentials, "refresh_token")
	require.NotContains(t, got.Credentials, "id_token")
	require.NotContains(t, got.Credentials, "api_key")
	// 非敏感键保留
	require.Equal(t, "https://api.example.com", got.Credentials["base_url"])
	require.Equal(t, map[string]any{"foo": "bar"}, got.Credentials["model_mapping"])

	// 状态 map 标记敏感键存在
	require.True(t, got.CredentialsStatus["has_access_token"])
	require.True(t, got.CredentialsStatus["has_refresh_token"])
	require.True(t, got.CredentialsStatus["has_id_token"])
	require.True(t, got.CredentialsStatus["has_api_key"])

	// JSON 序列化校验：响应体里不会出现敏感子串
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "rt-secret")
	require.NotContains(t, string(raw), "at-secret")
	require.NotContains(t, string(raw), "sk-secret")
	require.NotContains(t, string(raw), "id-secret")
	// 状态标识应序列化进 JSON
	require.Contains(t, string(raw), "credentials_status")
	require.Contains(t, string(raw), "has_refresh_token")

	// 原始 service.Account 不应被改动
	require.Equal(t, "rt-secret", src.Credentials["refresh_token"])
}

func TestAccountFromServiceShallow_RedactsOllamaCloudManagedExtra(t *testing.T) {
	snapshot := map[string]any{
		"status":          service.OllamaCloudUsageStatusOK,
		"last_attempt_at": "2026-07-22T12:00:00Z",
		"next_refresh_at": "2026-07-22T13:00:00Z",
		"data":            map[string]any{"plan": "Pro"},
	}
	src := &service.Account{
		ID: 9, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": "https://ollama.com", "api_key": "secret-key"},
		// 第三方 key 的上游地址来自协议映射，Ollama Cloud 判定读的是它。
		ProtocolEndpoints: map[string]string{
			service.APIProtocolChatCompletions: "https://ollama.com",
		},
		Extra: map[string]any{
			service.OllamaCloudUsageSessionExtraKey:  "ciphertext-secret",
			service.OllamaCloudUsageSnapshotExtraKey: snapshot,
			"ordinary":                               "kept",
		},
	}

	got := AccountFromServiceShallow(src)
	require.NotContains(t, got.Extra, service.OllamaCloudUsageSessionExtraKey)
	require.NotContains(t, got.Extra, service.OllamaCloudUsageSnapshotExtraKey)
	require.Equal(t, "kept", got.Extra["ordinary"])
	require.NotNil(t, got.OllamaCloudUsage)
	require.True(t, got.OllamaCloudUsage.Configured)
	require.Equal(t, "Pro", got.OllamaCloudUsage.Snapshot.Data.Plan)

	raw, err := json.Marshal(got)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "ciphertext-secret")
	require.NotContains(t, string(raw), "secret-key")
	require.Contains(t, src.Extra, service.OllamaCloudUsageSessionExtraKey)
}

func TestAccountFromServiceShallow_NilCredentialsOmitsStatus(t *testing.T) {
	src := &service.Account{ID: 1, Name: "n", Platform: "anthropic", Type: "oauth"}
	got := AccountFromServiceShallow(src)
	require.NotNil(t, got)
	require.Nil(t, got.Credentials)
	require.Nil(t, got.CredentialsStatus)
}

// vendor 按上游地址识别：第三方 key 的 platform 只是展示标签，前端要靠 vendor 知道
// 这把 key 实际直连的是哪家官方，中转 / 聚合平台为空。
func TestAccountFromServiceShallow_ExposesVendorByAddress(t *testing.T) {
	official := AccountFromServiceShallow(&service.Account{
		ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		ProtocolEndpoints: map[string]string{service.APIProtocolChatCompletions: "https://api.deepseek.com"},
	})
	require.Equal(t, service.PlatformDeepseek, official.Vendor, "an openai-labelled key on api.deepseek.com is a DeepSeek key")

	relay := AccountFromServiceShallow(&service.Account{
		ID: 2, Platform: service.PlatformKimi, Type: service.AccountTypeAPIKey,
		ProtocolEndpoints: map[string]string{service.APIProtocolChatCompletions: "https://relay.example/v1"},
	})
	require.Empty(t, relay.Vendor, "a relay is no vendor, whatever the label")

	subscription := AccountFromServiceShallow(&service.Account{ID: 3, Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth})
	require.Equal(t, service.PlatformAnthropic, subscription.Vendor)
}
