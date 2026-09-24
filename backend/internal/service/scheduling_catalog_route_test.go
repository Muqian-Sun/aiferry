//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 目录路由下的资格：协议转换注册表 × 入站协议（条目没有网关族）。

func catalogRouteCtx(entryID int64, inbound string) context.Context {
	ctx := WithCatalogRoute(context.Background(), CatalogRoute{EntryID: entryID, CanonicalModel: "m", RequestedModel: "m"})
	if inbound != "" {
		ctx = WithInboundProtocol(ctx, inbound)
	}
	return ctx
}

// 运行时资格 = 协议转换注册表，不分成品号 / key、不看条目网关族；platform 只剩强制 antigravity 一个用途。
func TestAccountServesCatalogRoute_ByConversion(t *testing.T) {
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
		{"anthropic subscription", oauth(PlatformAnthropic), PlatformAnthropic, [4]bool{true, true, true, false}},
		{"anthropic subscription on a gemini-family entry (family no longer matters)", oauth(PlatformAnthropic), PlatformGemini, [4]bool{true, true, true, false}},
		{"anthropic subscription on forced antigravity", oauth(PlatformAnthropic), PlatformAntigravity, [4]bool{false, false, false, false}},
		{"antigravity subscription", oauth(PlatformAntigravity), PlatformAnthropic, [4]bool{true, true, true, true}},
		{"antigravity subscription on forced antigravity", oauth(PlatformAntigravity), PlatformAntigravity, [4]bool{true, true, true, true}},
		{"gemini subscription", oauth(PlatformGemini), PlatformGemini, [4]bool{true, true, false, true}},
		{"openai subscription", oauth(PlatformOpenAI), PlatformOpenAI, [4]bool{true, true, true, false}},
		{"grok subscription", oauth(PlatformGrok), PlatformGrok, [4]bool{true, true, true, false}},
		{"kimi has no subscription upstream", oauth(PlatformKimi), PlatformKimi, [4]bool{false, false, false, false}},
		{"responses-only key serves message / chat / responses", &schedulingTestKeyPtr, PlatformAnthropic, [4]bool{true, true, true, false}},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			for i, inbound := range allInbound {
				require.Equal(t, r.want[i], accountServesCatalogRoute(r.account, r.platform, inbound), "inbound=%s", inbound)
			}
		})
	}

	t.Run("empty inbound is the OpenAI extension endpoints", func(t *testing.T) {
		require.True(t, accountServesCatalogRoute(oauth(PlatformOpenAI), PlatformOpenAI, ""))
		require.True(t, accountServesCatalogRoute(oauth(PlatformGrok), PlatformGrok, ""))
		require.False(t, accountServesCatalogRoute(oauth(PlatformGemini), PlatformGemini, ""))
		chatKey := schedulingTestKey(3, PlatformOpenAI, map[string]string{APIProtocolChatCompletions: schedulingTestRelayURL})
		require.True(t, accountServesCatalogRoute(&chatKey, PlatformOpenAI, ""))
	})
}

var schedulingTestKeyPtr = schedulingTestKey(2, PlatformOpenAI, map[string]string{APIProtocolResponses: schedulingTestRelayURL})

func TestAccountServesCatalogRoute_KeysFollowUpstreamAddresses(t *testing.T) {
	chatOnly := schedulingTestKey(1, PlatformAnthropic, map[string]string{APIProtocolChatCompletions: schedulingTestRelayURL})
	anthropicAddr := schedulingTestKey(2, PlatformOpenAI, map[string]string{APIProtocolAnthropic: schedulingTestRelayURL})
	geminiOnly := schedulingTestKey(3, PlatformGemini, map[string]string{APIProtocolGemini: schedulingTestRelayURL})

	require.True(t, accountServesCatalogRoute(&chatOnly, PlatformOpenAI, APIProtocolAnthropic), "message converts to chat")
	require.True(t, accountServesCatalogRoute(&chatOnly, PlatformAnthropic, APIProtocolAnthropic), "no longer family-gated")
	require.True(t, accountServesCatalogRoute(&anthropicAddr, PlatformAnthropic, APIProtocolChatCompletions))
	require.True(t, accountServesCatalogRoute(&anthropicAddr, PlatformOpenAI, APIProtocolResponses), "responses convert to anthropic")
	require.False(t, accountServesCatalogRoute(&anthropicAddr, PlatformGemini, APIProtocolGemini), "no generate → anthropic conversion")
	require.True(t, accountServesCatalogRoute(&geminiOnly, PlatformAnthropic, APIProtocolAnthropic), "message converts to gemini")
	require.False(t, accountServesCatalogRoute(&geminiOnly, PlatformAnthropic, APIProtocolResponses), "no responses → gemini conversion")
	require.False(t, accountServesCatalogRoute(&anthropicAddr, PlatformAntigravity, APIProtocolAnthropic), "forced antigravity only admits antigravity subscriptions")
	require.False(t, accountServesCatalogRoute(nil, PlatformOpenAI, APIProtocolChatCompletions))
}

// 目录路由下资格只看协议转换注册表（antigravity 成品号能承接 message 入站）；
// 无路由时按平台池规则，平台不相等就出局。
func TestIsAccountSchedulableOnPlatform_CatalogRouteVersusPlatformPool(t *testing.T) {
	antigravity := &Account{ID: 9, Type: AccountTypeOAuth, Platform: PlatformAntigravity, Status: StatusActive, Schedulable: true}

	routed := catalogRouteCtx(7, APIProtocolAnthropic)
	require.True(t, isAccountSchedulableOnPlatform(routed, antigravity, PlatformAnthropic))

	unrouted := WithInboundProtocol(context.Background(), APIProtocolAnthropic)
	require.False(t, isAccountSchedulableOnPlatform(unrouted, antigravity, PlatformAnthropic))
	require.True(t, isAccountSchedulableOnPlatform(unrouted, antigravity, PlatformAntigravity))

	geminiOAuth := &Account{ID: 10, Type: AccountTypeOAuth, Platform: PlatformGemini, Status: StatusActive, Schedulable: true}
	filtered := filterAccountsSchedulableOnPlatform(catalogRouteCtx(7, APIProtocolResponses), []Account{*antigravity, *geminiOAuth}, PlatformGemini)
	require.Len(t, filtered, 1, "gemini oauth cannot serve responses; antigravity can")
	require.Equal(t, int64(9), filtered[0].ID)
}

// 目录路由下：网关族由条目决定（分组是 anthropic 也走 openai 平台候选），池 = 条目绑定；
// 同一请求去掉 route 后回到分组语义。
func TestGatewayService_SelectAccountWithLoadAwareness_CatalogRouteOrEndpointPlatform(t *testing.T) {
	const entryID = int64(77)
	openAIOAuth := Account{
		ID: 30102, Name: "openai-oauth", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive,
		Schedulable: true, Concurrency: 5, Priority: 1, CatalogEntryIDs: []int64{entryID},
	}
	anthropicInGroup := Account{
		ID: 30103, Name: "anthropic-oauth", Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive,
		Schedulable: true, Concurrency: 5, Priority: 1}
	for _, loadBatch := range []bool{true, false} {
		name := "load aware"
		if !loadBatch {
			name = "legacy"
		}
		t.Run(name, func(t *testing.T) {
			repo := &mockAccountRepoForPlatform{accounts: []Account{openAIOAuth, anthropicInGroup}, accountsByID: map[int64]*Account{}}
			for i := range repo.accounts {
				repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
			}
			cfg := testConfig()
			cfg.Gateway.Scheduling.LoadBatchEnabled = loadBatch
			svc := &GatewayService{
				accountRepo:        repo,
				cache:              &mockGatewayCacheForPlatform{},
				cfg:                cfg,
				concurrencyService: NewConcurrencyService(&mockConcurrencyCache{}),
			}

			routed := catalogRouteCtx(entryID, APIProtocolAnthropic)
			result, err := svc.SelectAccountWithLoadAwareness(routed, "", "gpt-5.6", nil)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, openAIOAuth.ID, result.Account.ID, "the bound openai account is chosen")

			// 无路由又无端点平台：没有池（原来按分组平台 / anthropic 兜底）
			unrouted := WithInboundProtocol(context.Background(), APIProtocolAnthropic)
			result, err = svc.SelectAccountWithLoadAwareness(unrouted, "", "claude-sonnet-4-5", nil)
			require.ErrorIs(t, err, ErrNoAvailableAccounts, "no route and no endpoint platform: no pool")
			require.Nil(t, result)

			// 端点声明平台：anthropic 平台池只有平台相等的成品号
			result, err = svc.SelectAccountWithOptions(unrouted, "", "claude-sonnet-4-5", nil, SelectOptions{Platform: PlatformAnthropic})
			require.NoError(t, err)
			require.Equal(t, anthropicInGroup.ID, result.Account.ID, "endpoint platform pool applies without a route")
		})
	}
}
