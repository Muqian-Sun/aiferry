//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestProtocolEndpointPreferredOverStoredBaseURL 固定地址解析的优先级：
// 协议映射里配了就用它，没配才回落到 credentials.base_url 或官方默认地址。
// 回落是过渡期的关键——存量账号没有协议映射，行为必须与切换前完全一致。
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

	t.Run("anthropic 未配协议映射时回落 base_url", func(t *testing.T) {
		account := Account{
			Type:        AccountTypeAPIKey,
			Platform:    PlatformAnthropic,
			Credentials: map[string]any{"base_url": "https://legacy.example.com"},
		}
		require.Equal(t, "https://legacy.example.com", account.GetBaseURL())
	})

	t.Run("两者都没有时仍回落官方地址", func(t *testing.T) {
		account := Account{Type: AccountTypeAPIKey, Platform: PlatformAnthropic}
		require.Equal(t, "https://api.anthropic.com", account.GetBaseURL())
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

	t.Run("responses 未单独配置时回落 chat_completions", func(t *testing.T) {
		account := Account{
			Type:              AccountTypeAPIKey,
			Platform:          PlatformOpenAI,
			ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://cc.example.com"},
		}
		require.Equal(t, "https://cc.example.com", account.GetOpenAIResponsesBaseURL())
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

	t.Run("gemini 未配时回落 base_url，再回落默认值", func(t *testing.T) {
		withBase := Account{
			Type:        AccountTypeAPIKey,
			Platform:    PlatformGemini,
			Credentials: map[string]any{"base_url": "https://legacy.example.com"},
		}
		require.Equal(t, "https://legacy.example.com", withBase.GetGeminiBaseURL("https://default.example.com"))

		bare := Account{Type: AccountTypeAPIKey, Platform: PlatformGemini}
		require.Equal(t, "https://default.example.com", bare.GetGeminiBaseURL("https://default.example.com"))
	})
}
