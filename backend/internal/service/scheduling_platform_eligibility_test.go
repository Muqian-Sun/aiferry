//go:build unit

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// 第三方 key 按协议地址、不按平台标签参与调度的回归用例。

const schedulingTestRelayURL = "https://relay.example.com/v1"

func schedulingTestKey(id int64, label string, endpoints map[string]string, groupIDs ...int64) Account {
	account := Account{
		ID:                id,
		Name:              "key",
		Platform:          label,
		Type:              AccountTypeAPIKey,
		Status:            StatusActive,
		Schedulable:       true,
		Concurrency:       5,
		Priority:          1,
		ProtocolEndpoints: endpoints,
		GroupIDs:          groupIDs,
	}
	for _, groupID := range groupIDs {
		account.AccountGroups = append(account.AccountGroups, AccountGroup{AccountID: id, GroupID: groupID})
	}
	return account
}

func TestAccountServesSchedulingPlatform(t *testing.T) {
	antigravityMixed := Account{Platform: PlatformAntigravity, Type: AccountTypeOAuth, Extra: map[string]any{"mixed_scheduling": true}}
	antigravityNotMixed := Account{Platform: PlatformAntigravity, Type: AccountTypeOAuth}
	anthropicOAuth := Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}
	chatOnlyAnthropicLabel := schedulingTestKey(1, PlatformAnthropic, map[string]string{APIProtocolChatCompletions: schedulingTestRelayURL})
	anthropicOnlyOpenAILabel := schedulingTestKey(2, PlatformOpenAI, map[string]string{APIProtocolAnthropic: schedulingTestRelayURL})
	anthropicOnlyAntigravityLabel := schedulingTestKey(3, PlatformAntigravity, map[string]string{APIProtocolAnthropic: schedulingTestRelayURL})

	tests := []struct {
		name     string
		account  Account
		platform string
		inbound  string
		useMixed bool
		want     bool
	}{
		// 成品号：规则不变。
		{"subscription same platform", anthropicOAuth, PlatformAnthropic, APIProtocolAnthropic, true, true},
		{"subscription other platform", anthropicOAuth, PlatformGemini, APIProtocolGemini, true, false},
		{"antigravity mixed enabled in mixed bucket", antigravityMixed, PlatformAnthropic, APIProtocolAnthropic, true, true},
		{"antigravity mixed enabled outside mixed bucket", antigravityMixed, PlatformAnthropic, APIProtocolAnthropic, false, false},
		{"antigravity mixed disabled", antigravityNotMixed, PlatformGemini, APIProtocolGemini, true, false},
		// 第三方 key：只看网关平台与入站协议能否选出一个已配地址的上游协议。
		{"key chat endpoint on openai gateway", chatOnlyAnthropicLabel, PlatformOpenAI, APIProtocolChatCompletions, false, true},
		{"key chat endpoint converts responses", chatOnlyAnthropicLabel, PlatformKimi, APIProtocolResponses, false, true},
		{"key chat endpoint gemini inbound", chatOnlyAnthropicLabel, PlatformOpenAI, APIProtocolGemini, false, false},
		{"key chat endpoint on its label gateway", chatOnlyAnthropicLabel, PlatformAnthropic, APIProtocolAnthropic, true, false},
		{"key anthropic endpoint on anthropic gateway", anthropicOnlyOpenAILabel, PlatformAnthropic, APIProtocolAnthropic, true, true},
		{"key anthropic endpoint on gemini gateway", anthropicOnlyOpenAILabel, PlatformGemini, APIProtocolGemini, true, false},
		{"key anthropic endpoint extension endpoint", anthropicOnlyOpenAILabel, PlatformOpenAI, "", false, false},
		{"antigravity-labelled key ignores mixed flag", anthropicOnlyAntigravityLabel, PlatformAnthropic, APIProtocolAnthropic, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, accountServesSchedulingPlatform(&tt.account, tt.platform, tt.inbound, tt.useMixed))
			ctx := WithInboundProtocol(context.Background(), tt.inbound)
			require.Equal(t, tt.want, isAccountSchedulableOnPlatform(ctx, &tt.account, tt.platform, tt.useMixed))
		})
	}
}

func TestSchedulingBucketAdmitsKeysOfAnyLabel(t *testing.T) {
	key := schedulingTestKey(1, PlatformAnthropic, map[string]string{APIProtocolChatCompletions: schedulingTestRelayURL})
	for _, platform := range schedulerSnapshotPlatforms() {
		require.True(t, schedulingBucketAdmits(&key, platform, false), platform)
	}
	antigravityNotMixed := Account{Platform: PlatformAntigravity, Type: AccountTypeOAuth}
	require.False(t, schedulingBucketAdmits(&antigravityNotMixed, PlatformAnthropic, true))
	require.True(t, schedulingBucketAdmits(&antigravityNotMixed, PlatformAntigravity, false))
}

func TestOpenAISchedulers_SelectCrossLabelKeyByInboundProtocol(t *testing.T) {
	groupID := int64(20901)
	key := schedulingTestKey(20911, PlatformAnthropic, map[string]string{APIProtocolChatCompletions: schedulingTestRelayURL}, groupID)

	newService := func(t *testing.T, scoring, loadBatch bool) *OpenAIGatewayService {
		cfg := &config.Config{}
		cfg.Gateway.Scheduling.LoadBatchEnabled = loadBatch
		svc := &OpenAIGatewayService{
			accountRepo:        schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{key}}},
			cache:              &schedulerTestGatewayCache{},
			cfg:                cfg,
			concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
		}
		if scoring {
			svc.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("true")
		}
		require.Equal(t, scoring, svc.isOpenAIAdvancedSchedulerEnabled(context.Background()))
		return svc
	}

	cases := []struct {
		name      string
		scoring   bool
		loadBatch bool
	}{
		{name: "legacy load batch", loadBatch: true},
		{name: "legacy without load batch"},
		{name: "scoring", scoring: true, loadBatch: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			defer resetOpenAIAdvancedSchedulerSettingCacheForTest()
			svc := newService(t, tc.scoring, tc.loadBatch)

			chatCtx := WithInboundProtocol(context.Background(), APIProtocolChatCompletions)
			selection, _, err := svc.SelectAccountWithSchedulerForCapability(
				chatCtx, &groupID, "", "", "gpt-5.1", nil,
				OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityChatCompletions,
				false, false, false, PlatformOpenAI,
			)
			require.NoError(t, err)
			require.NotNil(t, selection)
			require.NotNil(t, selection.Account)
			require.Equal(t, key.ID, selection.Account.ID)

			geminiCtx := WithInboundProtocol(context.Background(), APIProtocolGemini)
			selection, _, err = svc.SelectAccountWithSchedulerForCapability(
				geminiCtx, &groupID, "", "", "gpt-5.1", nil,
				OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityChatCompletions,
				false, false, false, PlatformOpenAI,
			)
			require.ErrorIs(t, err, ErrNoAvailableAccounts)
			require.Nil(t, selection)
		})
	}
}

// 粘性会话路径同样按协议判断：会话绑定到跨标签 key 时继续命中它，而不是改选优先级更高的账号。
func TestOpenAISchedulers_StickySessionKeepsCrossLabelKey(t *testing.T) {
	groupID := int64(20902)
	sticky := schedulingTestKey(20912, PlatformAnthropic, map[string]string{APIProtocolChatCompletions: schedulingTestRelayURL}, groupID)
	sticky.Priority = 5
	preferred := schedulingTestKey(20913, PlatformOpenAI, map[string]string{APIProtocolChatCompletions: schedulingTestRelayURL}, groupID)
	preferred.Priority = 0
	const sessionHash = "cross-label-sticky"

	for _, scoring := range []bool{false, true} {
		name := "legacy"
		if scoring {
			name = "scoring"
		}
		t.Run(name, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			defer resetOpenAIAdvancedSchedulerSettingCacheForTest()
			cfg := newSchedulerTestSubscriptionPriorityConfig() // Top-1 按优先级打分，结果确定
			cfg.Gateway.Scheduling.LoadBatchEnabled = true
			svc := &OpenAIGatewayService{
				accountRepo:        schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{preferred, sticky}}},
				cache:              &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:" + sessionHash: sticky.ID}},
				cfg:                cfg,
				concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
			}
			if scoring {
				svc.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("true")
			}
			require.Equal(t, scoring, svc.isOpenAIAdvancedSchedulerEnabled(context.Background()))

			ctx := WithInboundProtocol(context.Background(), APIProtocolChatCompletions)
			selection, _, err := svc.SelectAccountWithSchedulerForCapability(
				ctx, &groupID, "", "", "gpt-5.1", nil,
				OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityChatCompletions,
				false, false, false, PlatformOpenAI,
			)
			require.NoError(t, err)
			require.Equal(t, preferred.ID, selection.Account.ID, "without a session the higher-priority key wins")
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}

			selection, _, err = svc.SelectAccountWithSchedulerForCapability(
				ctx, &groupID, "", sessionHash, "gpt-5.1", nil,
				OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityChatCompletions,
				false, false, false, PlatformOpenAI,
			)
			require.NoError(t, err)
			require.Equal(t, sticky.ID, selection.Account.ID)
		})
	}
}

// 粘性会话路径（负载感知 Layer 1.5、传统单平台与混合调度）同样按协议判断跨标签 key。
func TestGatewayService_StickySessionKeepsCrossLabelKey(t *testing.T) {
	const sessionHash = "gateway-cross-label-sticky"
	groups := map[string]int64{PlatformAnthropic: 20951, PlatformAntigravity: 20952}
	for groupPlatform, groupID := range groups {
		for _, loadBatch := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s load batch=%v", groupPlatform, loadBatch), func(t *testing.T) {
				sticky := schedulingTestKey(20961, PlatformOpenAI, map[string]string{APIProtocolAnthropic: schedulingTestRelayURL}, groupID)
				sticky.Priority = 5
				preferred := Account{
					ID: 20962, Platform: groupPlatform, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
					Concurrency: 5, Priority: 0, GroupIDs: []int64{groupID}, AccountGroups: []AccountGroup{{AccountID: 20962, GroupID: groupID}},
				}
				repo := &mockAccountRepoForPlatform{accounts: []Account{preferred, sticky}, accountsByID: map[int64]*Account{}}
				for i := range repo.accounts {
					repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
				}
				cfg := testConfig()
				cfg.Gateway.Scheduling.LoadBatchEnabled = loadBatch
				svc := &GatewayService{
					accountRepo: repo,
					groupRepo: &mockGroupRepoForGateway{groups: map[int64]*Group{
						groupID: {ID: groupID, Platform: groupPlatform, Status: StatusActive, Hydrated: true},
					}},
					cache:              &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{sessionHash: sticky.ID}},
					cfg:                cfg,
					concurrencyService: NewConcurrencyService(&mockConcurrencyCache{}),
				}
				ctx := WithInboundProtocol(context.Background(), APIProtocolAnthropic)

				result, err := svc.SelectAccountWithLoadAwareness(ctx, &groupID, "", "claude-sonnet-4-5", nil, "", 0)
				require.NoError(t, err)
				require.Equal(t, preferred.ID, result.Account.ID, "without a session the higher-priority account wins")

				result, err = svc.SelectAccountWithLoadAwareness(ctx, &groupID, sessionHash, "claude-sonnet-4-5", nil, "", 0)
				require.NoError(t, err)
				require.Equal(t, sticky.ID, result.Account.ID)
			})
		}
	}
}

func TestGatewayService_SelectAccountWithLoadAwareness_CrossLabelKeyByGroupProtocol(t *testing.T) {
	anthropicGroupID := int64(20931)
	geminiGroupID := int64(20932)
	key := schedulingTestKey(20941, PlatformOpenAI, map[string]string{APIProtocolAnthropic: schedulingTestRelayURL}, anthropicGroupID, geminiGroupID)

	for _, loadBatch := range []bool{true, false} {
		name := "load aware"
		if !loadBatch {
			name = "legacy"
		}
		t.Run(name, func(t *testing.T) {
			repo := &mockAccountRepoForPlatform{accounts: []Account{key}, accountsByID: map[int64]*Account{}}
			for i := range repo.accounts {
				repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
			}
			groupRepo := &mockGroupRepoForGateway{groups: map[int64]*Group{
				anthropicGroupID: {ID: anthropicGroupID, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true},
				geminiGroupID:    {ID: geminiGroupID, Platform: PlatformGemini, Status: StatusActive, Hydrated: true},
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

			ctx := WithInboundProtocol(context.Background(), APIProtocolAnthropic)
			result, err := svc.SelectAccountWithLoadAwareness(ctx, &anthropicGroupID, "", "claude-sonnet-4-5", nil, "", 0)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, result.Account)
			require.Equal(t, key.ID, result.Account.ID)

			result, err = svc.SelectAccountWithLoadAwareness(ctx, &geminiGroupID, "", "gemini-2.5-pro", nil, "", 0)
			require.ErrorIs(t, err, ErrNoAvailableAccounts)
			require.Nil(t, result)
		})
	}
}

// antigravity 分组按入站协议只走一种上游协议：只有 gemini 地址的 key 不承接 /v1/messages，
// 但承接 Gemini 原生请求。传统单平台选号循环依赖候选列表已按协议过滤。
func TestGatewayService_AntigravityGroupKeyNeedsInboundProtocolEndpoint(t *testing.T) {
	groupID := int64(20991)
	key := schedulingTestKey(20992, PlatformAntigravity, map[string]string{APIProtocolGemini: schedulingTestRelayURL}, groupID)
	for _, loadBatch := range []bool{true, false} {
		t.Run(fmt.Sprintf("load batch=%v", loadBatch), func(t *testing.T) {
			repo := &mockAccountRepoForPlatform{accounts: []Account{key}, accountsByID: map[int64]*Account{}}
			for i := range repo.accounts {
				repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
			}
			cfg := testConfig()
			cfg.Gateway.Scheduling.LoadBatchEnabled = loadBatch
			svc := &GatewayService{
				accountRepo: repo,
				groupRepo: &mockGroupRepoForGateway{groups: map[int64]*Group{
					groupID: {ID: groupID, Platform: PlatformAntigravity, Status: StatusActive, Hydrated: true},
				}},
				cache:              &mockGatewayCacheForPlatform{},
				cfg:                cfg,
				concurrencyService: NewConcurrencyService(&mockConcurrencyCache{}),
			}

			anthropicCtx := WithInboundProtocol(context.Background(), APIProtocolAnthropic)
			result, err := svc.SelectAccountWithLoadAwareness(anthropicCtx, &groupID, "", "", nil, "", 0)
			require.ErrorIs(t, err, ErrNoAvailableAccounts)
			require.Nil(t, result)

			geminiCtx := WithInboundProtocol(context.Background(), APIProtocolGemini)
			result, err = svc.SelectAccountWithLoadAwareness(geminiCtx, &groupID, "", "", nil, "", 0)
			require.NoError(t, err)
			require.Equal(t, key.ID, result.Account.ID)
		})
	}
}

// 成品号的混合调度与平台匹配不受第三方 key 规则影响：anthropic 分组照旧选中启用了
// mixed_scheduling 的 antigravity 成品号，跳过未启用的 antigravity 成品号与其他平台成品号。
func TestGatewayService_SelectAccountWithLoadAwareness_SubscriptionMixedSchedulingUnchanged(t *testing.T) {
	groupID := int64(20971)
	accounts := []Account{
		{ID: 20981, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Priority: 0, Status: StatusActive, Schedulable: true, Concurrency: 5},
		{ID: 20982, Platform: PlatformAntigravity, Type: AccountTypeOAuth, Priority: 0, Status: StatusActive, Schedulable: true, Concurrency: 5},
		{ID: 20983, Platform: PlatformAntigravity, Type: AccountTypeOAuth, Priority: 1, Status: StatusActive, Schedulable: true, Concurrency: 5, Extra: map[string]any{"mixed_scheduling": true}},
		{ID: 20984, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Priority: 2, Status: StatusActive, Schedulable: true, Concurrency: 5},
	}
	for _, loadBatch := range []bool{true, false} {
		repo := &mockAccountRepoForPlatform{accounts: accounts, accountsByID: map[int64]*Account{}}
		for i := range repo.accounts {
			repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
		}
		cfg := testConfig()
		cfg.Gateway.Scheduling.LoadBatchEnabled = loadBatch
		svc := &GatewayService{
			accountRepo: repo,
			groupRepo: &mockGroupRepoForGateway{groups: map[int64]*Group{
				groupID: {ID: groupID, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true},
			}},
			cache:              &mockGatewayCacheForPlatform{},
			cfg:                cfg,
			concurrencyService: NewConcurrencyService(&mockConcurrencyCache{}),
		}
		ctx := WithInboundProtocol(context.Background(), APIProtocolAnthropic)
		excluded := map[int64]struct{}{}
		var picked []int64
		for {
			result, err := svc.SelectAccountWithLoadAwareness(ctx, &groupID, "", "claude-sonnet-4-5", excluded, "", 0)
			if err != nil {
				require.ErrorIs(t, err, ErrNoAvailableAccounts)
				break
			}
			picked = append(picked, result.Account.ID)
			excluded[result.Account.ID] = struct{}{}
		}
		require.Equal(t, []int64{20983, 20984}, picked, "load batch=%v", loadBatch)
	}
}

func TestPreferGeminiOAuthInMixedScheduling(t *testing.T) {
	geminiOAuth := &Account{Platform: PlatformGemini, Type: AccountTypeOAuth}
	geminiServiceAccount := &Account{Platform: PlatformGemini, Type: AccountTypeServiceAccount}
	antigravityOAuth := &Account{Platform: PlatformAntigravity, Type: AccountTypeOAuth, Extra: map[string]any{"mixed_scheduling": true}}
	anthropicLabelledKey := &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey}

	require.True(t, preferGeminiOAuthInMixedScheduling(geminiOAuth, geminiServiceAccount))
	require.True(t, preferGeminiOAuthInMixedScheduling(geminiOAuth, anthropicLabelledKey))
	require.False(t, preferGeminiOAuthInMixedScheduling(geminiOAuth, antigravityOAuth))
	require.False(t, preferGeminiOAuthInMixedScheduling(geminiServiceAccount, anthropicLabelledKey))
	require.False(t, preferGeminiOAuthInMixedScheduling(anthropicLabelledKey, geminiServiceAccount))
}

func TestAccountKeepsHTTPPreviousResponseID(t *testing.T) {
	tests := []struct {
		name    string
		account Account
		want    bool
	}{
		{"official openai key", schedulingTestKey(1, PlatformAnthropic, map[string]string{APIProtocolResponses: "https://api.openai.com", APIProtocolChatCompletions: "https://api.openai.com"}), true},
		{"relay key with responses", schedulingTestKey(2, PlatformDeepseek, map[string]string{APIProtocolResponses: schedulingTestRelayURL}), true},
		{"relay key converted to chat completions", schedulingTestKey(3, PlatformOpenAI, map[string]string{APIProtocolChatCompletions: schedulingTestRelayURL}), false},
		{"other known vendor", schedulingTestKey(4, PlatformOpenAI, map[string]string{APIProtocolResponses: DefaultDeepseekBaseURL}), false},
		{"openai oauth", Account{ID: 5, Platform: PlatformOpenAI, Type: AccountTypeOAuth}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, AccountKeepsHTTPPreviousResponseID(&tt.account, PlatformOpenAI))
		})
	}
}

func TestAccountSupportsOpenAIEndpointCapability_KeysIgnoreLabel(t *testing.T) {
	relayChatOnly := schedulingTestKey(1, PlatformOpenAI, map[string]string{APIProtocolChatCompletions: schedulingTestRelayURL})
	relayResponses := schedulingTestKey(2, PlatformAnthropic, map[string]string{APIProtocolResponses: schedulingTestRelayURL, APIProtocolChatCompletions: schedulingTestRelayURL})
	grokLabelledRelay := schedulingTestKey(3, PlatformGrok, map[string]string{APIProtocolChatCompletions: schedulingTestRelayURL})
	officialXAIOpenAILabel := schedulingTestKey(4, PlatformOpenAI, map[string]string{APIProtocolChatCompletions: "https://api.x.ai/v1"})
	grokLabelledRelayOverride := schedulingTestKey(5, PlatformGrok, map[string]string{APIProtocolChatCompletions: schedulingTestRelayURL})
	grokLabelledRelayOverride.Extra = map[string]any{GrokMediaEligibleExtraKey: true}
	embeddingsOnly := schedulingTestKey(6, PlatformKimi, map[string]string{APIProtocolChatCompletions: schedulingTestRelayURL})
	embeddingsOnly.Credentials = map[string]any{"openai_capabilities": []any{"embeddings"}}

	// Responses 看有没有 responses 地址，不看平台标签，也不看已退役的探测标记。
	require.False(t, relayChatOnly.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityResponses))
	relayResponses.Extra = map[string]any{"openai_responses_supported": false}
	require.True(t, relayResponses.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityResponses))

	// grok 标签的中转 key 与其他 key 一样具备通用能力。
	require.True(t, grokLabelledRelay.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions))
	require.True(t, grokLabelledRelay.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityEmbeddings))
	require.True(t, relayChatOnly.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityAlphaSearch))
	require.False(t, relayChatOnly.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityLive))

	// Grok 媒体生成是 xAI 厂商能力：看地址，管理员显式开关优先。
	require.False(t, grokLabelledRelay.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityGrokMediaGeneration))
	require.True(t, officialXAIOpenAILabel.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityGrokMediaGeneration))
	require.True(t, grokLabelledRelayOverride.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityGrokMediaGeneration))

	// 管理员配置的能力集照旧生效。
	require.False(t, embeddingsOnly.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions))
	require.True(t, embeddingsOnly.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityEmbeddings))
}

func TestOpenAICompactSupportTier_KeysByProtocolAndVendor(t *testing.T) {
	relayResponses := schedulingTestKey(1, PlatformAnthropic, map[string]string{APIProtocolResponses: schedulingTestRelayURL})
	relayChatOnly := schedulingTestKey(2, PlatformOpenAI, map[string]string{APIProtocolChatCompletions: schedulingTestRelayURL})
	officialXAI := schedulingTestKey(3, PlatformOpenAI, map[string]string{APIProtocolResponses: "https://api.x.ai/v1"})
	grokLabelledRelay := schedulingTestKey(4, PlatformGrok, map[string]string{APIProtocolResponses: schedulingTestRelayURL})
	kimiOfficial := schedulingTestKey(5, PlatformOpenAI, map[string]string{APIProtocolResponses: DefaultKimiPayGBaseURL})

	require.Equal(t, 1, openAICompactSupportTier(&relayResponses))
	require.Equal(t, 0, openAICompactSupportTier(&relayChatOnly))
	require.Equal(t, 2, openAICompactSupportTier(&officialXAI))
	require.Equal(t, 1, openAICompactSupportTier(&grokLabelledRelay))
	require.Equal(t, 0, openAICompactSupportTier(&kimiOfficial))

	// 成品号不变。
	require.Equal(t, 2, openAICompactSupportTier(&Account{Platform: PlatformGrok, Type: AccountTypeOAuth}))
	require.Equal(t, 0, openAICompactSupportTier(&Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}))
}

func TestShouldAutoPauseOpenAIAccountByQuota_KeysIgnoreLabel(t *testing.T) {
	codexExtra := func() map[string]any {
		return map[string]any{
			"codex_5h_used_percent":   96.0,
			"auto_pause_5h_threshold": 0.95,
			"codex_usage_updated_at":  time.Now().UTC().Format(time.RFC3339),
		}
	}
	anthropicLabelledKey := schedulingTestKey(1, PlatformAnthropic, map[string]string{APIProtocolResponses: schedulingTestRelayURL})
	anthropicLabelledKey.Extra = codexExtra()
	paused, _ := shouldAutoPauseOpenAIAccountByQuota(context.Background(), &anthropicLabelledKey)
	require.True(t, paused)

	anthropicOAuth := Account{ID: 2, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Extra: codexExtra()}
	paused, _ = shouldAutoPauseOpenAIAccountByQuota(context.Background(), &anthropicOAuth)
	require.False(t, paused)
}

// Gemini AI Studio 端点（GET /v1beta/models 等）只转发到 Gemini 协议：第三方 key 按有无
// gemini 地址参与选号，不看平台标签。
func TestGeminiSelectAccountForAIStudioEndpoints_KeysByGeminiEndpoint(t *testing.T) {
	groupID := int64(21201)
	geminiEndpointAnthropicLabel := schedulingTestKey(21211, PlatformAnthropic, map[string]string{APIProtocolGemini: schedulingTestRelayURL}, groupID)
	geminiEndpointAnthropicLabel.Priority = 2
	geminiEndpointAnthropicLabel.Credentials = map[string]any{"api_key": "relay-key"}
	anthropicEndpointGeminiLabel := schedulingTestKey(21212, PlatformGemini, map[string]string{APIProtocolAnthropic: schedulingTestRelayURL}, groupID)
	anthropicEndpointGeminiLabel.Priority = 1
	anthropicEndpointGeminiLabel.Credentials = map[string]any{"api_key": "relay-key"}

	repo := &mockAccountRepoForGemini{
		accounts:     []Account{anthropicEndpointGeminiLabel, geminiEndpointAnthropicLabel},
		accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
	}
	svc := &GeminiMessagesCompatService{accountRepo: repo, groupRepo: &mockGroupRepoForGemini{groups: map[int64]*Group{}}}

	selected, err := svc.SelectAccountForAIStudioEndpoints(context.Background(), &groupID)
	require.NoError(t, err)
	require.Equal(t, geminiEndpointAnthropicLabel.ID, selected.ID)

	repo.accounts = []Account{anthropicEndpointGeminiLabel}
	_, err = svc.SelectAccountForAIStudioEndpoints(context.Background(), &groupID)
	require.Error(t, err)
}
