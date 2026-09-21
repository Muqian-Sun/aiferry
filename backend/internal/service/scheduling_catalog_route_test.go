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

// 绑定校验仍按网关族矩阵（3b-5 删 route_platform 时换成注册表）：运行时放宽了，绑定没放宽。
func TestBindingAdmitsFamily_KeepsFamilyMatrix(t *testing.T) {
	oauth := func(platform string) *Account {
		return &Account{ID: 1, Type: AccountTypeOAuth, Platform: platform, Status: StatusActive, Schedulable: true}
	}
	require.True(t, bindingAdmitsFamily(oauth(PlatformAnthropic), PlatformAnthropic, APIProtocolAnthropic))
	require.False(t, bindingAdmitsFamily(oauth(PlatformAnthropic), PlatformGemini, APIProtocolAnthropic), "anthropic subscription cannot be bound to a gemini-family entry")
	require.False(t, bindingAdmitsFamily(oauth(PlatformGrok), PlatformOpenAI, APIProtocolResponses), "openai family requires the exact platform")
	require.True(t, bindingAdmitsFamily(oauth(PlatformAntigravity), PlatformGemini, APIProtocolGemini))
	require.False(t, bindingAdmitsFamily(oauth(PlatformGemini), PlatformGemini, APIProtocolResponses))
	chatOnly := schedulingTestKey(1, PlatformAnthropic, map[string]string{APIProtocolChatCompletions: schedulingTestRelayURL})
	require.False(t, bindingAdmitsFamily(&chatOnly, PlatformAnthropic, APIProtocolAnthropic), "anthropic family needs an anthropic address")
	require.True(t, bindingAdmitsFamily(&chatOnly, PlatformOpenAI, APIProtocolAnthropic))
	require.False(t, bindingAdmitsFamily(&chatOnly, PlatformOpenAI, APIProtocolGemini), "gemini inbound never reaches the openai family")
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
