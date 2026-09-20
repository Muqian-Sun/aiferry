//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
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

// 目录路由下：网关族由条目决定（分组是 anthropic 也走 openai 平台候选），池 = 条目绑定；
// 同一请求去掉 route 后回到分组语义。
func TestGatewayService_SelectAccountWithLoadAwareness_CatalogRouteOverridesGroup(t *testing.T) {
	groupID := int64(30101)
	const entryID = int64(77)
	openAIOAuth := Account{
		ID: 30102, Name: "openai-oauth", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive,
		Schedulable: true, Concurrency: 5, Priority: 1, CatalogEntryIDs: []int64{entryID},
	}
	anthropicInGroup := Account{
		ID: 30103, Name: "anthropic-oauth", Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive,
		Schedulable: true, Concurrency: 5, Priority: 1, GroupIDs: []int64{groupID},
		AccountGroups: []AccountGroup{{AccountID: 30103, GroupID: groupID}},
	}
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
			groupRepo := &mockGroupRepoForGateway{groups: map[int64]*Group{
				groupID: {ID: groupID, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true},
			}}
			cfg := testConfig()
			cfg.Gateway.Scheduling.LoadBatchEnabled = loadBatch
			svc := &GatewayService{
				accountRepo:        repo,
				groupRepo:          groupRepo,
				cache:              &mockGatewayCacheForPlatform{},
				cfg:                cfg,
				concurrencyService: NewConcurrencyService(&mockConcurrencyCache{}),
			}

			routed := catalogRouteCtx(entryID, PlatformOpenAI, APIProtocolAnthropic)
			result, err := svc.SelectAccountWithLoadAwareness(routed, &groupID, "", "gpt-5.6", nil, "", 0)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, openAIOAuth.ID, result.Account.ID, "the bound openai account is chosen although the group is anthropic")

			unrouted := WithInboundProtocol(context.Background(), APIProtocolAnthropic)
			result, err = svc.SelectAccountWithLoadAwareness(unrouted, &groupID, "", "claude-sonnet-4-5", nil, "", 0)
			require.NoError(t, err)
			require.Equal(t, anthropicInGroup.ID, result.Account.ID, "without a route the group pool still applies")
		})
	}
}

// 目录路由下分组的主备路由规则不参与选号。
func TestRoutingAccountIDsSkippedUnderCatalogRoute(t *testing.T) {
	groupID := int64(30201)
	groupRepo := &mockGroupRepoForGateway{groups: map[int64]*Group{
		groupID: {ID: groupID, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true,
			ModelRoutingEnabled: true, ModelRouting: map[string][]int64{"claude-sonnet-4-5": {1, 2}}},
	}}
	svc := &GatewayService{groupRepo: groupRepo, cfg: testConfig()}

	require.Equal(t, []int64{1, 2}, svc.routingAccountIDsForRequest(context.Background(), &groupID, "claude-sonnet-4-5", PlatformAnthropic))
	require.Nil(t, svc.routingAccountIDsForRequest(catalogRouteCtx(5, PlatformAnthropic, APIProtocolAnthropic), &groupID, "claude-sonnet-4-5", PlatformAnthropic))
}

// OpenAI 网关：目录路由下池 = 条目绑定（不看分组），grok 条目只能选到 grok 成品号 / 能承接的 key。
func TestOpenAISchedulers_CatalogRoutePoolFromBindings(t *testing.T) {
	groupID := int64(30301)
	const entryID = int64(88)
	grokOAuth := Account{
		ID: 30302, Name: "grok-oauth", Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive,
		Schedulable: true, Concurrency: 5, Priority: 1, CatalogEntryIDs: []int64{entryID},
	}
	openAIOAuthInGroup := Account{
		ID: 30303, Name: "openai-oauth", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive,
		Schedulable: true, Concurrency: 5, Priority: 1, GroupIDs: []int64{groupID},
	}
	for _, loadBatch := range []bool{true, false} {
		name := "load batch"
		if !loadBatch {
			name = "legacy"
		}
		t.Run(name, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			defer resetOpenAIAdvancedSchedulerSettingCacheForTest()
			cfg := &config.Config{}
			cfg.Gateway.Scheduling.LoadBatchEnabled = loadBatch
			svc := &OpenAIGatewayService{
				accountRepo:        schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{grokOAuth, openAIOAuthInGroup}}},
				cache:              &schedulerTestGatewayCache{},
				cfg:                cfg,
				concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
			}

			routed := catalogRouteCtx(entryID, PlatformGrok, APIProtocolChatCompletions)
			selection, _, err := svc.SelectAccountWithSchedulerForCapability(
				routed, &groupID, "", "", "grok-4.6", nil,
				OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityChatCompletions,
				false, false, false, PlatformGrok,
			)
			require.NoError(t, err)
			require.NotNil(t, selection)
			require.Equal(t, grokOAuth.ID, selection.Account.ID, "bound grok account is chosen although it is not in the key's group")

			unrouted := WithInboundProtocol(context.Background(), APIProtocolChatCompletions)
			selection, _, err = svc.SelectAccountWithSchedulerForCapability(
				unrouted, &groupID, "", "", "gpt-5.1", nil,
				OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityChatCompletions,
				false, false, false, PlatformOpenAI,
			)
			require.NoError(t, err)
			require.Equal(t, openAIOAuthInGroup.ID, selection.Account.ID, "without a route the group pool still applies")
		})
	}
}
