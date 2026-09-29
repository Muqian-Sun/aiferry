//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func conversionTestKey(endpoints map[string]string) *Account {
	return &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, ProtocolEndpoints: endpoints}
}

func conversionTestSubscription(platform string) *Account {
	return &Account{ID: 2, Platform: platform, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true}
}

func TestConversionExists(t *testing.T) {
	tests := []struct {
		inbound, upstream string
		want              bool
	}{
		{APIProtocolAnthropic, APIProtocolAnthropic, true},
		{APIProtocolAnthropic, APIProtocolGemini, true},
		{APIProtocolAnthropic, UpstreamProtocolAntigravity, true},
		{APIProtocolChatCompletions, APIProtocolAnthropic, true},
		{APIProtocolChatCompletions, APIProtocolGemini, true},
		{APIProtocolResponses, APIProtocolAnthropic, true},
		{APIProtocolResponses, APIProtocolGemini, false},
		{APIProtocolGemini, APIProtocolGemini, true},
		{APIProtocolGemini, UpstreamProtocolAntigravity, true},
		{APIProtocolGemini, APIProtocolAnthropic, false},
		{"", APIProtocolChatCompletions, true},
		{"", APIProtocolAnthropic, false},
		{"unknown", APIProtocolAnthropic, false},
	}
	for _, tt := range tests {
		t.Run(tt.inbound+"->"+tt.upstream, func(t *testing.T) {
			require.Equal(t, tt.want, ConversionExists(tt.inbound, tt.upstream))
		})
	}
}

func TestUpstreamProtocols(t *testing.T) {
	require.Equal(t, []string{APIProtocolChatCompletions, APIProtocolResponses},
		conversionTestKey(map[string]string{APIProtocolResponses: "https://r.example.com", APIProtocolChatCompletions: "https://c.example.com"}).UpstreamProtocols(),
		"key 按固定序列出配了地址的协议")
	require.Nil(t, conversionTestKey(nil).UpstreamProtocols(), "没配地址的 key 什么都不能接")
	require.Equal(t, []string{APIProtocolAnthropic}, conversionTestSubscription(PlatformAnthropic).UpstreamProtocols())
	require.Equal(t, []string{APIProtocolGemini}, conversionTestSubscription(PlatformGemini).UpstreamProtocols())
	require.Equal(t, []string{UpstreamProtocolAntigravity}, conversionTestSubscription(PlatformAntigravity).UpstreamProtocols())
	require.Equal(t, []string{APIProtocolResponses}, conversionTestSubscription(PlatformOpenAI).UpstreamProtocols())
	require.Equal(t, []string{APIProtocolResponses}, conversionTestSubscription(PlatformGrok).UpstreamProtocols())
	require.Nil(t, conversionTestSubscription(PlatformKimi).UpstreamProtocols(), "kimi 没有成品号")
	var nilAccount *Account
	require.Nil(t, nilAccount.UpstreamProtocols())
}

func TestUpstreamProtocolFor(t *testing.T) {
	relayCC := "https://relay.example.com"
	tests := []struct {
		name    string
		account *Account
		inbound string
		want    string
	}{
		{name: "responses-only key serves message via responses", account: conversionTestKey(map[string]string{APIProtocolResponses: relayCC}), inbound: APIProtocolAnthropic, want: APIProtocolResponses},
		{name: "gemini-only key cannot serve responses", account: conversionTestKey(map[string]string{APIProtocolGemini: relayCC}), inbound: APIProtocolResponses, want: ""},
		{name: "gemini-only key serves message via gemini", account: conversionTestKey(map[string]string{APIProtocolGemini: relayCC}), inbound: APIProtocolAnthropic, want: APIProtocolGemini},
		{name: "same protocol wins for generic relay", account: conversionTestKey(map[string]string{APIProtocolChatCompletions: relayCC, APIProtocolResponses: relayCC}), inbound: APIProtocolChatCompletions, want: APIProtocolChatCompletions},
		// 指向 api.openai.com 的 key 按中转（2026-09-29）：同协议直连优先，不再先转 Responses。
		{name: "key on api.openai.com keeps chat inbound direct", account: conversionTestKey(map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"}), inbound: APIProtocolChatCompletions, want: APIProtocolChatCompletions},
		{name: "key on api.openai.com serves message via responses like any relay", account: conversionTestKey(map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"}), inbound: APIProtocolAnthropic, want: APIProtocolResponses},
		{name: "key on api.openai.com without responses address serves message via chat", account: conversionTestKey(map[string]string{APIProtocolChatCompletions: "https://api.openai.com"}), inbound: APIProtocolAnthropic, want: APIProtocolChatCompletions},
		{name: "key on api.openai.com with anthropic address serves message directly", account: conversionTestKey(map[string]string{APIProtocolAnthropic: "https://api.openai.com"}), inbound: APIProtocolAnthropic, want: APIProtocolAnthropic},
		{name: "anthropic key with anthropic+responses serves message directly", account: conversionTestKey(map[string]string{APIProtocolAnthropic: relayCC, APIProtocolResponses: relayCC}), inbound: APIProtocolAnthropic, want: APIProtocolAnthropic},
		{name: "extension endpoint needs chat address on keys", account: conversionTestKey(map[string]string{APIProtocolResponses: relayCC}), inbound: "", want: ""},
		{name: "extension endpoint chat key", account: conversionTestKey(map[string]string{APIProtocolChatCompletions: relayCC}), inbound: "", want: APIProtocolChatCompletions},
		{name: "extension endpoint openai subscription", account: conversionTestSubscription(PlatformOpenAI), inbound: "", want: APIProtocolResponses},
		{name: "extension endpoint anthropic subscription", account: conversionTestSubscription(PlatformAnthropic), inbound: "", want: ""},
		{name: "anthropic subscription serves chat via conversion", account: conversionTestSubscription(PlatformAnthropic), inbound: APIProtocolChatCompletions, want: APIProtocolAnthropic},
		{name: "anthropic subscription cannot serve generate", account: conversionTestSubscription(PlatformAnthropic), inbound: APIProtocolGemini, want: ""},
		{name: "gemini subscription cannot serve responses", account: conversionTestSubscription(PlatformGemini), inbound: APIProtocolResponses, want: ""},
		{name: "antigravity subscription serves everything", account: conversionTestSubscription(PlatformAntigravity), inbound: APIProtocolGemini, want: UpstreamProtocolAntigravity},
		{name: "grok subscription serves message via responses", account: conversionTestSubscription(PlatformGrok), inbound: APIProtocolAnthropic, want: APIProtocolResponses},
		{name: "kimi subscription serves nothing", account: conversionTestSubscription(PlatformKimi), inbound: APIProtocolChatCompletions, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.account.UpstreamProtocolFor(tt.inbound))
			require.Equal(t, tt.want != "", tt.account.ServesInbound(tt.inbound))
		})
	}
}

func TestProtocolMatches(t *testing.T) {
	relay := "https://relay.example.com"
	require.True(t, conversionTestKey(map[string]string{APIProtocolChatCompletions: relay}).ProtocolMatches(APIProtocolChatCompletions))
	require.False(t, conversionTestKey(map[string]string{APIProtocolResponses: relay}).ProtocolMatches(APIProtocolChatCompletions), "要转换的不算直连")
	require.True(t, conversionTestSubscription(PlatformAnthropic).ProtocolMatches(APIProtocolAnthropic))
	require.False(t, conversionTestSubscription(PlatformAntigravity).ProtocolMatches(APIProtocolGemini), "antigravity 的上游是 v1internal 封装，不算 generate 直连")
	require.False(t, conversionTestKey(map[string]string{APIProtocolChatCompletions: relay}).ProtocolMatches(""), "扩展端点没有直连概念")
	require.True(t, conversionTestKey(map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"}).ProtocolMatches(APIProtocolChatCompletions), "指向 api.openai.com 的 key 按中转，chat 入站直连")
}
