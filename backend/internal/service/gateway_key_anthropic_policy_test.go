//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// Anthropic 协议上的 beta 策略、fast 计费档与 web search 模拟：按协议、厂商与网关平台
// 判断，不看第三方 key 的展示标签。

func TestGatewayServiceForward_BetaPolicyBlockAppliesToKeysOfAnyLabel(t *testing.T) {
	setGatewayPolicyForTest(t, &betaPolicy, BetaPolicySettings{Rules: []BetaPolicyRule{{
		BetaToken:    "context-1m-2025-08-07",
		Action:       BetaPolicyActionBlock,
		Scope:        BetaPolicyScopeAPIKey,
		ErrorMessage: "1m context is blocked",
	}}})
	c, svc, upstream := newAnthropicKeyForwardFixture(`{}`)
	c.Request.Header.Set("Anthropic-Beta", "context-1m-2025-08-07")
	body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	parsed := &ParsedRequest{Body: NewRequestBodyRef(body), Model: "claude-sonnet-4-5"}

	_, err := svc.Forward(context.Background(), c, openAILabelledAnthropicKey(nil), parsed)

	var blocked *BetaBlockedError
	require.True(t, errors.As(err, &blocked), "expected *BetaBlockedError, got %v", err)
	require.Equal(t, "1m context is blocked", blocked.Message)
	require.Nil(t, upstream.lastReq)
}

func anthropicKeyWithEndpoint(label, endpoint string) *Account {
	return &Account{
		Platform:          label,
		Type:              AccountTypeAPIKey,
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: endpoint},
	}
}

func TestAnthropicSpeedServiceTier_KeysNeedUpstreamConfirmation(t *testing.T) {
	const model = "claude-opus-5"
	relayAnthropicLabel := anthropicKeyWithEndpoint(PlatformAnthropic, "https://anthropic-relay.example.com")
	relayOpenAILabel := anthropicKeyWithEndpoint(PlatformOpenAI, "https://anthropic-relay.example.com")
	keyOnOfficialHost := anthropicKeyWithEndpoint(PlatformAnthropic, "https://api.anthropic.com")
	subscription := &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}

	// 中转：标签是 anthropic 也不凭请求侧 speed=fast 计 2x。
	require.Nil(t, anthropicSpeedServiceTier(relayAnthropicLabel, "fast", model, ""))
	require.Nil(t, anthropicSpeedServiceTier(relayOpenAILabel, "fast", model, "standard"))
	// 指向 api.anthropic.com 的 key 也按中转（2026-09-29 海外四家不再有官方 key）。
	require.Nil(t, anthropicSpeedServiceTier(keyOnOfficialHost, "fast", model, ""))
	// 中转回了 usage.speed=fast，任何标签都按 fast 计。
	tier := anthropicSpeedServiceTier(relayOpenAILabel, "fast", model, "fast")
	require.NotNil(t, tier)
	require.Equal(t, "fast", *tier)
	// Anthropic 成品号：按请求侧档位计，由响应只降不升。
	tier = anthropicSpeedServiceTier(subscription, "fast", model, "")
	require.NotNil(t, tier)
	require.Equal(t, "fast", *tier)
	// 未请求 fast 时响应声明 fast 也不升档。
	require.Nil(t, anthropicSpeedServiceTier(relayOpenAILabel, "standard", model, "fast"))
}

func TestGatewayServiceForward_RelayKeyFastTierFollowsUpstreamSpeed(t *testing.T) {
	fast := "fast"
	cases := []struct {
		name     string
		response string
		want     *string
	}{
		{"upstream confirms fast", `{"id":"msg_1","type":"message","model":"claude-opus-5","usage":{"input_tokens":3,"output_tokens":1,"speed":"fast"}}`, &fast},
		{"upstream silent", `{"id":"msg_1","type":"message","model":"claude-opus-5","usage":{"input_tokens":3,"output_tokens":1}}`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, svc, _ := newAnthropicKeyForwardFixture(tc.response)
			body := []byte(`{"model":"claude-opus-5","max_tokens":16,"speed":"fast","messages":[{"role":"user","content":"hello"}]}`)
			parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
			require.NoError(t, err)
			require.Equal(t, "fast", parsed.Speed)

			result, err := svc.Forward(context.Background(), c, openAILabelledAnthropicKey(nil), parsed)

			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, tc.want, result.ServiceTier)
		})
	}
}
