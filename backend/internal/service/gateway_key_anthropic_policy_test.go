//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/websearch"
	"github.com/stretchr/testify/require"
)

// Anthropic 协议上的 beta 策略、fast 计费档与 web search 模拟：按协议、厂商与网关平台
// 判断，不看第三方 key 的展示标签。

func TestGatewayServiceForward_BetaPolicyBlockAppliesToKeysOfAnyLabel(t *testing.T) {
	raw, err := json.Marshal(&BetaPolicySettings{Rules: []BetaPolicyRule{{
		BetaToken:    "context-1m-2025-08-07",
		Action:       BetaPolicyActionBlock,
		Scope:        BetaPolicyScopeAPIKey,
		ErrorMessage: "1m context is blocked",
	}}})
	require.NoError(t, err)
	c, svc, upstream := newAnthropicKeyForwardFixture(`{}`)
	svc.settingService = NewSettingService(&betaPolicySettingRepoStub{values: map[string]string{
		SettingKeyBetaPolicySettings: string(raw),
	}}, &config.Config{})
	c.Request.Header.Set("Anthropic-Beta", "context-1m-2025-08-07")
	body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	parsed := &ParsedRequest{Body: NewRequestBodyRef(body), Model: "claude-sonnet-4-5"}

	_, err = svc.Forward(context.Background(), c, openAILabelledAnthropicKey(nil), parsed)

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

func TestAnthropicSpeedServiceTier_KeysNeedOfficialVendorOrUpstreamConfirmation(t *testing.T) {
	const model = "claude-opus-5"
	relayAnthropicLabel := anthropicKeyWithEndpoint(PlatformAnthropic, "https://anthropic-relay.example.com")
	relayOpenAILabel := anthropicKeyWithEndpoint(PlatformOpenAI, "https://anthropic-relay.example.com")
	officialOpenAILabel := anthropicKeyWithEndpoint(PlatformOpenAI, "https://api.anthropic.com")

	// 中转：标签是 anthropic 也不凭请求侧 speed=fast 计 2x。
	require.Nil(t, anthropicSpeedServiceTier(relayAnthropicLabel, "fast", model, ""))
	require.Nil(t, anthropicSpeedServiceTier(relayOpenAILabel, "fast", model, "standard"))
	// 中转回了 usage.speed=fast，任何标签都按 fast 计。
	tier := anthropicSpeedServiceTier(relayOpenAILabel, "fast", model, "fast")
	require.NotNil(t, tier)
	require.Equal(t, "fast", *tier)
	// 官方 Anthropic 地址：标签是 openai 也按请求侧档位计，由响应只降不升。
	tier = anthropicSpeedServiceTier(officialOpenAILabel, "fast", model, "")
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

func TestGetWebSearchEmulationMode_KeysOfAnyLabelSubscriptionsFollowChannel(t *testing.T) {
	key := &Account{
		Platform:          PlatformOpenAI,
		Type:              AccountTypeAPIKey,
		Extra:             map[string]any{featureKeyWebSearchEmulation: WebSearchModeEnabled},
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://anthropic-relay.example.com"},
	}
	require.Equal(t, WebSearchModeEnabled, key.GetWebSearchEmulationMode())

	subscription := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Extra:    map[string]any{featureKeyWebSearchEmulation: WebSearchModeEnabled},
	}
	require.Equal(t, WebSearchModeDefault, subscription.GetWebSearchEmulationMode())
}

func TestShouldEmulateWebSearch_ChannelSwitchFollowsGatewayPlatformNotLabel(t *testing.T) {
	mgr := websearch.NewManager([]websearch.ProviderConfig{{Type: "brave", APIKey: "k"}}, nil)
	SetWebSearchManager(mgr)
	defer SetWebSearchManager(nil)
	setGlobalWebSearchConfig(&WebSearchEmulationConfig{
		Enabled:   true,
		Providers: []WebSearchProviderConfig{{Type: "brave", APIKey: "k"}},
	})
	defer clearGlobalWebSearchConfig()

	channel := &Channel{
		ID:     11,
		Status: StatusActive,
		FeaturesConfig: map[string]any{
			featureKeyWebSearchEmulation: map[string]any{PlatformAnthropic: true},
		},
	}
	key := func(label string) *Account {
		return &Account{
			Platform:          label,
			Type:              AccountTypeAPIKey,
			ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://anthropic-relay.example.com"},
		}
	}
	newSvc := func(groupPlatform string) *GatewayService {
		channelSvc := newChannelServiceWithCache(43, channel)
		channelSvc.cache.Load().(*channelCache).groupPlatform[43] = groupPlatform
		return &GatewayService{settingService: newSettingServiceForWebSearchTest(true), channelService: channelSvc}
	}
	groupID := int64(43)

	require.True(t, newSvc(PlatformAnthropic).shouldEmulateWebSearch(context.Background(), key(PlatformOpenAI), &groupID, webSearchToolBody),
		"openai-labelled key in an anthropic group follows the anthropic switch")
	require.False(t, newSvc(PlatformAntigravity).shouldEmulateWebSearch(context.Background(), key(PlatformAnthropic), &groupID, webSearchToolBody),
		"anthropic label does not borrow the anthropic switch in an antigravity group")

	forced := context.WithValue(context.Background(), ctxkey.ForcePlatform, PlatformAntigravity)
	require.False(t, newSvc(PlatformAnthropic).shouldEmulateWebSearch(forced, key(PlatformAnthropic), &groupID, webSearchToolBody),
		"forced antigravity route looks up the antigravity switch")
}
