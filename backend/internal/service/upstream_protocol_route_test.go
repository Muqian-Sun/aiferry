//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestKeyUpstreamProtocols 固定「分组平台 × 入站协议 → 第三方 key 可用的上游协议」。
func TestKeyUpstreamProtocols(t *testing.T) {
	anthropic, cc, responses, gemini := APIProtocolAnthropic, APIProtocolChatCompletions, APIProtocolResponses, APIProtocolGemini
	cases := []struct {
		name     string
		group    string
		inbound  string
		vendor   string
		expected []string
	}{
		{"anthropic 分组只转 anthropic", PlatformAnthropic, cc, "", []string{anthropic}},
		{"gemini 分组只转 gemini", PlatformGemini, anthropic, "", []string{gemini}},
		{"antigravity 分组入站 gemini", PlatformAntigravity, gemini, "", []string{gemini}},
		{"antigravity 分组入站 messages", PlatformAntigravity, anthropic, "", []string{anthropic}},
		{"OpenAI 网关入站 responses 同协议优先", PlatformDeepseek, responses, "", []string{responses, cc, anthropic}},
		{"OpenAI 网关入站 CC 同协议优先", PlatformOpenAI, cc, "", []string{cc, responses, anthropic}},
		{"OpenAI 网关入站 messages 同协议优先", PlatformGrok, anthropic, PlatformGrok, []string{anthropic, responses, cc}},
		{"官方 OpenAI 入站 CC 先转 Responses", PlatformOpenAI, cc, PlatformOpenAI, []string{responses, cc}},
		{"官方 OpenAI 入站 messages 先转 Responses", PlatformOpenAI, anthropic, PlatformOpenAI, []string{responses, cc}},
		{"OpenAI 扩展端点只认 CC 根地址", PlatformOpenAI, "", "", []string{cc}},
		{"OpenAI 网关不收 gemini 入站", PlatformOpenAI, gemini, "", nil},
		{"未知分组平台没有可用协议", "", cc, "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, KeyUpstreamProtocols(tc.group, tc.inbound, tc.vendor))
		})
	}
}

// TestAccountKeyUpstreamProtocolFor 第三方 key 按配了地址的协议取第一个可用的；
// 平台标签不参与判断。
func TestAccountKeyUpstreamProtocolFor(t *testing.T) {
	relay := &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey, ProtocolEndpoints: map[string]string{
		APIProtocolChatCompletions: "https://relay.example.com/v1",
		APIProtocolResponses:       "https://relay.example.com/v1",
	}}
	keyOnOpenAIHost := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, ProtocolEndpoints: map[string]string{
		APIProtocolChatCompletions: "https://api.openai.com",
		APIProtocolResponses:       "https://api.openai.com",
	}}

	t.Run("中转入站 CC 直连 CC", func(t *testing.T) {
		require.Equal(t, APIProtocolChatCompletions, relay.KeyUpstreamProtocolFor(PlatformOpenAI, APIProtocolChatCompletions))
	})
	t.Run("中转入站 messages 没有 anthropic 地址时转 Responses", func(t *testing.T) {
		require.Equal(t, APIProtocolResponses, relay.KeyUpstreamProtocolFor(PlatformKimi, APIProtocolAnthropic))
	})
	t.Run("指向 api.openai.com 的 key 按中转：入站 CC 直连 CC，不先转 Responses", func(t *testing.T) {
		require.Equal(t, APIProtocolChatCompletions, keyOnOpenAIHost.KeyUpstreamProtocolFor(PlatformOpenAI, APIProtocolChatCompletions))
	})
	t.Run("标签是 anthropic 但没有 anthropic 地址，anthropic 分组不能用", func(t *testing.T) {
		require.Equal(t, "", relay.KeyUpstreamProtocolFor(PlatformAnthropic, APIProtocolAnthropic))
	})
	t.Run("antigravity 分组入站 gemini 需要 gemini 地址", func(t *testing.T) {
		account := &Account{Platform: PlatformAntigravity, Type: AccountTypeAPIKey, ProtocolEndpoints: map[string]string{
			APIProtocolAnthropic: "https://relay.example.com",
		}}
		require.Equal(t, "", account.KeyUpstreamProtocolFor(PlatformAntigravity, APIProtocolGemini))
		require.Equal(t, APIProtocolAnthropic, account.KeyUpstreamProtocolFor(PlatformAntigravity, APIProtocolAnthropic))
	})
	t.Run("成品号不走这条判断", func(t *testing.T) {
		require.Equal(t, "", (&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}).KeyUpstreamProtocolFor(PlatformOpenAI, APIProtocolResponses))
	})
}

func TestIsOpenAIGatewayPlatform(t *testing.T) {
	for _, platform := range []string{PlatformOpenAI, PlatformGrok, PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax, PlatformOpenCodeGo} {
		require.True(t, IsOpenAIGatewayPlatform(platform), platform)
	}
	for _, platform := range []string{PlatformAnthropic, PlatformGemini, PlatformAntigravity, ""} {
		require.False(t, IsOpenAIGatewayPlatform(platform), platform)
	}
}
