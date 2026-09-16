//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUpstreamProtocolsOf 固定「账号说哪些协议」的判定来源。
//
// 第三方 key 只认 protocol_endpoints 的键，不做任何平台推导——地址和协议都是
// 管理员显式声明的，推导只会带来「猜错把请求推给不会说该协议的上游」。
// 成品号相反：它本来就是厂商绑定的，协议由厂商决定，没有配置空间。
func TestUpstreamProtocolsOf(t *testing.T) {
	t.Run("配置了协议映射时以映射为准，忽略平台推导", func(t *testing.T) {
		account := Account{
			Platform:          PlatformAnthropic,
			Type:              AccountTypeAPIKey,
			ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://relay.example.com"},
		}
		require.True(t, account.SpeaksUpstreamProtocol(APIProtocolChatCompletions))
		require.False(t, account.SpeaksUpstreamProtocol(APIProtocolAnthropic))
	})

	t.Run("映射里的非法键被忽略", func(t *testing.T) {
		account := Account{
			Platform:          PlatformAnthropic,
			Type:              AccountTypeAPIKey,
			ProtocolEndpoints: map[string]string{"grpc": "https://relay.example.com"},
		}
		require.Empty(t, account.UpstreamProtocolsOf())
	})

	t.Run("成品号按厂商推导", func(t *testing.T) {
		cases := []struct {
			name     string
			account  Account
			speaks   []string
			notSpeak []string
		}{
			{
				name:     "anthropic",
				account:  Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth},
				speaks:   []string{APIProtocolAnthropic},
				notSpeak: []string{APIProtocolChatCompletions, APIProtocolGemini},
			},
			{
				name:     "gemini",
				account:  Account{Platform: PlatformGemini, Type: AccountTypeOAuth},
				speaks:   []string{APIProtocolGemini},
				notSpeak: []string{APIProtocolAnthropic},
			},
			{
				name:     "antigravity 同时暴露 claude 与 gemini",
				account:  Account{Platform: PlatformAntigravity, Type: AccountTypeOAuth},
				speaks:   []string{APIProtocolAnthropic, APIProtocolGemini},
				notSpeak: []string{APIProtocolChatCompletions},
			},
			{
				name:     "openai",
				account:  Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth},
				speaks:   []string{APIProtocolResponses, APIProtocolChatCompletions},
				notSpeak: []string{APIProtocolAnthropic},
			},
		}
		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				account := tt.account
				for _, p := range tt.speaks {
					require.Truef(t, account.SpeaksUpstreamProtocol(p), "应支持 %s", p)
				}
				for _, p := range tt.notSpeak {
					require.Falsef(t, account.SpeaksUpstreamProtocol(p), "不应支持 %s", p)
				}
			})
		}
	})

	t.Run("空协议名恒为否", func(t *testing.T) {
		account := Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}
		require.False(t, account.SpeaksUpstreamProtocol(""))
	})
}
