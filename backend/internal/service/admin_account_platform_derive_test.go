//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 添加渠道时第三方 key 不再选平台（2026-09-25）：没带平台就按地址推导，成品号必须带。
func TestResolveCreateAccountPlatform(t *testing.T) {
	derive := func(endpoints map[string]string) string {
		input := &CreateAccountInput{Type: AccountTypeAPIKey, ProtocolEndpoints: endpoints}
		require.NoError(t, resolveCreateAccountPlatform(input))
		return input.Platform
	}

	require.Equal(t, PlatformKimi, derive(map[string]string{APIProtocolChatCompletions: DefaultKimiPayGBaseURL}), "国产厂商官方地址认出厂商")
	require.Equal(t, PlatformDeepseek, derive(map[string]string{APIProtocolAnthropic: DefaultDeepseekAnthropicBaseURL}), "国产厂商的 Anthropic 兼容地址也认厂商，不按协议归族")
	// 海外四家的官方地址按中转、按主协议归族（2026-09-29）：xAI 地址归 openai 兼容族，不会推出 grok 标签。
	require.Equal(t, PlatformAnthropic, derive(map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"}))
	require.Equal(t, PlatformOpenAI, derive(map[string]string{APIProtocolResponses: "https://api.openai.com"}))
	require.Equal(t, PlatformGemini, derive(map[string]string{APIProtocolGemini: "https://generativelanguage.googleapis.com"}))
	require.Equal(t, PlatformOpenAI, derive(map[string]string{APIProtocolResponses: "https://api.x.ai/v1"}), "xAI 官方地址不再推出 grok")
	require.Equal(t, PlatformOpenAI, derive(map[string]string{APIProtocolChatCompletions: "https://us-east-1.api.x.ai/v1"}))
	require.Equal(t, PlatformAnthropic, derive(map[string]string{APIProtocolAnthropic: "https://relay.example.com"}), "中转按主协议归族")
	require.Equal(t, PlatformGemini, derive(map[string]string{APIProtocolGemini: "https://relay.example.com"}))
	require.Equal(t, PlatformOpenAI, derive(map[string]string{APIProtocolChatCompletions: "https://relay.example.com/v1"}))

	explicit := &CreateAccountInput{Type: AccountTypeAPIKey, Platform: "  grok ", ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://relay.example.com"}}
	require.NoError(t, resolveCreateAccountPlatform(explicit))
	require.Equal(t, PlatformGrok, explicit.Platform, "带了平台就用带的（去掉首尾空白）")

	noEndpoint := &CreateAccountInput{Type: AccountTypeAPIKey}
	require.ErrorContains(t, resolveCreateAccountPlatform(noEndpoint), "INVALID_PROTOCOL_ENDPOINTS")

	twoProtocols := &CreateAccountInput{Type: AccountTypeAPIKey, ProtocolEndpoints: map[string]string{
		APIProtocolResponses:       "https://api.openai.com",
		APIProtocolChatCompletions: "https://api.openai.com",
	}}
	require.ErrorContains(t, resolveCreateAccountPlatform(twoProtocols), "INVALID_PROTOCOL_ENDPOINTS", "一个资源只承接一个上游协议")

	subscription := &CreateAccountInput{Type: AccountTypeOAuth}
	require.ErrorContains(t, resolveCreateAccountPlatform(subscription), "ACCOUNT_PLATFORM_REQUIRED", "成品号的厂商决定授权流程")
}
