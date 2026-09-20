//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 目录路由下成品号的资格矩阵：厂商 × 生效平台 × 入站协议。

func catalogRouteCtx(entryID int64, platform, inbound string) context.Context {
	ctx := WithCatalogRoute(context.Background(), CatalogRoute{EntryID: entryID, CanonicalModel: "m", RequestedModel: "m", Platform: platform})
	if inbound != "" {
		ctx = WithInboundProtocol(ctx, inbound)
	}
	return ctx
}

func TestSubscriptionServesCatalogRoute(t *testing.T) {
	oauth := func(platform string) *Account {
		return &Account{ID: 1, Type: AccountTypeOAuth, Platform: platform, Status: StatusActive, Schedulable: true}
	}
	allInbound := []string{APIProtocolAnthropic, APIProtocolChatCompletions, APIProtocolResponses, APIProtocolGemini}
	type row struct {
		name     string
		account  *Account
		platform string
		want     [4]bool // anthropic, chat_completions, responses, gemini
	}
	rows := []row{
		{"anthropic on anthropic family", oauth(PlatformAnthropic), PlatformAnthropic, [4]bool{true, true, true, false}},
		{"anthropic on gemini family", oauth(PlatformAnthropic), PlatformGemini, [4]bool{false, false, false, false}},
		{"anthropic on forced antigravity", oauth(PlatformAnthropic), PlatformAntigravity, [4]bool{false, false, false, false}},
		{"antigravity on anthropic family", oauth(PlatformAntigravity), PlatformAnthropic, [4]bool{true, true, true, true}},
		{"antigravity on gemini family", oauth(PlatformAntigravity), PlatformGemini, [4]bool{true, true, true, true}},
		{"antigravity on forced antigravity", oauth(PlatformAntigravity), PlatformAntigravity, [4]bool{true, true, true, true}},
		{"antigravity on openai family", oauth(PlatformAntigravity), PlatformOpenAI, [4]bool{false, false, false, false}},
		{"gemini on gemini family", oauth(PlatformGemini), PlatformGemini, [4]bool{true, true, false, true}},
		{"gemini on anthropic family", oauth(PlatformGemini), PlatformAnthropic, [4]bool{false, false, false, false}},
		{"openai on openai family", oauth(PlatformOpenAI), PlatformOpenAI, [4]bool{true, true, true, false}},
		{"openai on grok family", oauth(PlatformOpenAI), PlatformGrok, [4]bool{false, false, false, false}},
		{"grok on grok family", oauth(PlatformGrok), PlatformGrok, [4]bool{true, true, true, false}},
		{"grok on openai family", oauth(PlatformGrok), PlatformOpenAI, [4]bool{false, false, false, false}},
		{"kimi on kimi family", oauth(PlatformKimi), PlatformKimi, [4]bool{true, true, true, false}},
		{"openai on anthropic family", oauth(PlatformOpenAI), PlatformAnthropic, [4]bool{false, false, false, false}},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			for i, inbound := range allInbound {
				require.Equal(t, r.want[i], subscriptionServesCatalogRoute(r.account, r.platform, inbound), "inbound=%s", inbound)
			}
		})
	}

	t.Run("empty inbound is treated as chat_completions", func(t *testing.T) {
		require.True(t, subscriptionServesCatalogRoute(oauth(PlatformOpenAI), PlatformOpenAI, ""))
		require.True(t, subscriptionServesCatalogRoute(oauth(PlatformGemini), PlatformGemini, ""))
		require.False(t, subscriptionServesCatalogRoute(oauth(PlatformOpenAI), PlatformGrok, ""))
	})
}

func TestAccountServesCatalogRoute_KeysFollowUpstreamAddresses(t *testing.T) {
	chatOnly := schedulingTestKey(1, PlatformAnthropic, map[string]string{APIProtocolChatCompletions: schedulingTestRelayURL})
	anthropicAddr := schedulingTestKey(2, PlatformOpenAI, map[string]string{APIProtocolAnthropic: schedulingTestRelayURL})

	require.True(t, accountServesCatalogRoute(&chatOnly, PlatformOpenAI, APIProtocolAnthropic), "openai family converts messages to chat")
	require.False(t, accountServesCatalogRoute(&chatOnly, PlatformAnthropic, APIProtocolAnthropic), "anthropic family needs an anthropic address")
	require.True(t, accountServesCatalogRoute(&anthropicAddr, PlatformAnthropic, APIProtocolChatCompletions))
	require.True(t, accountServesCatalogRoute(&anthropicAddr, PlatformOpenAI, APIProtocolResponses), "openai family can convert responses to anthropic")
	require.False(t, accountServesCatalogRoute(&anthropicAddr, PlatformGemini, APIProtocolGemini))
	require.False(t, accountServesCatalogRoute(nil, PlatformOpenAI, APIProtocolChatCompletions))
}

// 目录路由下 antigravity 成品号不再看 mixed_scheduling 开关；同一账号无 route 时按分组规则被排除。
func TestIsAccountSchedulableOnPlatform_CatalogRouteIgnoresMixedFlag(t *testing.T) {
	antigravity := &Account{ID: 9, Type: AccountTypeOAuth, Platform: PlatformAntigravity, Status: StatusActive, Schedulable: true}
	require.False(t, antigravity.IsMixedSchedulingEnabled())

	routed := catalogRouteCtx(7, PlatformAnthropic, APIProtocolAnthropic)
	require.True(t, isAccountSchedulableOnPlatform(routed, antigravity, PlatformAnthropic, true))

	unrouted := WithInboundProtocol(context.Background(), APIProtocolAnthropic)
	require.False(t, isAccountSchedulableOnPlatform(unrouted, antigravity, PlatformAnthropic, true))

	geminiOAuth := &Account{ID: 10, Type: AccountTypeOAuth, Platform: PlatformGemini, Status: StatusActive, Schedulable: true}
	filtered := filterAccountsSchedulableOnPlatform(catalogRouteCtx(7, PlatformGemini, APIProtocolResponses), []Account{*antigravity, *geminiOAuth}, PlatformGemini, false)
	require.Len(t, filtered, 1, "gemini oauth cannot serve responses; antigravity can")
	require.Equal(t, int64(9), filtered[0].ID)
}
