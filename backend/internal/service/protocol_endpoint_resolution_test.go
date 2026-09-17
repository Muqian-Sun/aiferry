//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestProtocolEndpointPreferredOverStoredBaseURL 固定第三方 key 的地址解析：
// 地址只认 protocol_endpoints，credentials.base_url 不再参与。
func TestProtocolEndpointPreferredOverStoredBaseURL(t *testing.T) {
	t.Run("anthropic 配了协议映射时优先使用", func(t *testing.T) {
		account := Account{
			Type:              AccountTypeAPIKey,
			Platform:          PlatformAnthropic,
			Credentials:       map[string]any{"base_url": "https://legacy.example.com"},
			ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://protocol.example.com"},
		}
		require.Equal(t, "https://protocol.example.com", account.GetBaseURL())
	})

	t.Run("非 apikey 账号不受协议映射影响", func(t *testing.T) {
		account := Account{
			Type:              AccountTypeOAuth,
			Platform:          PlatformAnthropic,
			ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://protocol.example.com"},
		}
		require.Equal(t, "", account.GetBaseURL())
	})

	t.Run("openai 优先使用 chat_completions 映射", func(t *testing.T) {
		account := Account{
			Type:              AccountTypeAPIKey,
			Platform:          PlatformOpenAI,
			Credentials:       map[string]any{"base_url": "https://legacy.example.com"},
			ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://cc.example.com"},
		}
		require.Equal(t, "https://cc.example.com", account.GetOpenAIBaseURL())
	})

	t.Run("responses 单独配置时与 chat_completions 分流", func(t *testing.T) {
		account := Account{
			Type:     AccountTypeAPIKey,
			Platform: PlatformOpenAI,
			ProtocolEndpoints: map[string]string{
				APIProtocolChatCompletions: "https://cc.example.com",
				APIProtocolResponses:       "https://resp.example.com",
			},
		}
		require.Equal(t, "https://cc.example.com", account.GetOpenAIBaseURL())
		require.Equal(t, "https://resp.example.com", account.GetOpenAIResponsesBaseURL())
	})

	t.Run("responses 未配置时不借用 chat_completions 地址", func(t *testing.T) {
		account := Account{
			Type:              AccountTypeAPIKey,
			Platform:          PlatformOpenAI,
			ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://cc.example.com"},
		}
		require.Empty(t, account.GetOpenAIResponsesBaseURL())
	})

	t.Run("gemini 优先使用 gemini 映射", func(t *testing.T) {
		account := Account{
			Type:              AccountTypeAPIKey,
			Platform:          PlatformGemini,
			Credentials:       map[string]any{"base_url": "https://legacy.example.com"},
			ProtocolEndpoints: map[string]string{APIProtocolGemini: "https://gemini.example.com"},
		}
		require.Equal(t, "https://gemini.example.com", account.GetGeminiBaseURL("https://default.example.com"))
	})

	t.Run("gemini 第三方 key 未配协议映射时不回落默认值", func(t *testing.T) {
		bare := Account{Type: AccountTypeAPIKey, Platform: PlatformGemini}
		require.Equal(t, "", bare.GetGeminiBaseURL("https://default.example.com"))
	})

	t.Run("gemini 成品号未配 base_url 时使用调用方给的默认值", func(t *testing.T) {
		oauth := Account{Type: AccountTypeOAuth, Platform: PlatformGemini}
		require.Equal(t, "https://default.example.com", oauth.GetGeminiBaseURL("https://default.example.com"))
	})
}
