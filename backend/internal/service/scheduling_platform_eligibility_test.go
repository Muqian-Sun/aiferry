//go:build unit

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

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

// 粘性会话路径（负载感知 Layer 1.5 与无负载图的回退顺序）同样按协议判断跨标签 key。
// 跨标签 key 只在目录路由下有意义（池 = 条目绑定、资格 = 协议转换注册表）；平台池要求账号平台相等。
func TestGatewayService_StickySessionKeepsCrossLabelKey(t *testing.T) {
	const sessionHash = "gateway-cross-label-sticky"
	const entryID = int64(20950)
	for _, subscriptionPlatform := range []string{PlatformAnthropic, PlatformAntigravity} {
		for _, loadBatch := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s load batch=%v", subscriptionPlatform, loadBatch), func(t *testing.T) {
				sticky := schedulingTestKey(20961, PlatformOpenAI, map[string]string{APIProtocolAnthropic: schedulingTestRelayURL})
				sticky.Priority = 5
				sticky.CatalogEntryIDs = []int64{entryID}
				preferred := Account{
					ID: 20962, Platform: subscriptionPlatform, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
					Concurrency: 5, Priority: 0, CatalogEntryIDs: []int64{entryID},
				}
				repo := &mockAccountRepoForPlatform{accounts: []Account{preferred, sticky}, accountsByID: map[int64]*Account{}}
				for i := range repo.accounts {
					repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
				}
				cfg := testConfig()
				cfg.Gateway.Scheduling.LoadBatchEnabled = loadBatch
				svc := &GatewayService{
					accountRepo:        repo,
					cache:              &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{sessionHash: sticky.ID}},
					cfg:                cfg,
					concurrencyService: NewConcurrencyService(&mockConcurrencyCache{}),
				}
				ctx := catalogRouteCtx(entryID, APIProtocolAnthropic)

				result, err := svc.SelectAccountWithLoadAwareness(ctx, "", "claude-sonnet-4-5", nil)
				require.NoError(t, err)
				// 协议直连第一键：anthropic 成品号与 key 都直连 → 优先级小的成品号赢；
				// antigravity 成品号要转换（v1internal），配了 anthropic 地址的 key 直连 → key 赢。
				wantWithoutSession := preferred.ID
				if subscriptionPlatform == PlatformAntigravity {
					wantWithoutSession = sticky.ID
				}
				require.Equal(t, wantWithoutSession, result.Account.ID, "without a session: protocol match first, then priority")

				result, err = svc.SelectAccountWithLoadAwareness(ctx, sessionHash, "claude-sonnet-4-5", nil)
				require.NoError(t, err)
				require.Equal(t, sticky.ID, result.Account.ID)
			})
		}
	}
}

// 目录路由下第三方 key 按入站协议承接：只有 anthropic 地址的 openai 标签 key 承接 message 入站，不承接 gemini 入站。
func TestGatewayService_SelectAccountWithLoadAwareness_CrossLabelKeyByInboundProtocol(t *testing.T) {
	const entryID = int64(20940)
	key := schedulingTestKey(20941, PlatformOpenAI, map[string]string{APIProtocolAnthropic: schedulingTestRelayURL})
	key.CatalogEntryIDs = []int64{entryID}

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
			cfg := testConfig()
			cfg.Gateway.Scheduling.LoadBatchEnabled = loadBatch
			svc := &GatewayService{
				accountRepo:        repo,
				cache:              &mockGatewayCacheForPlatform{},
				cfg:                cfg,
				concurrencyService: NewConcurrencyService(&mockConcurrencyCache{}),
			}

			result, err := svc.SelectAccountWithLoadAwareness(catalogRouteCtx(entryID, APIProtocolAnthropic), "", "claude-sonnet-4-5", nil)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, result.Account)
			require.Equal(t, key.ID, result.Account.ID, "message 入站：只配 anthropic 地址的 openai 标签 key 直连")

			// gemini 入站：这把 key 没有 gemini 地址，承接不了
			result, err = svc.SelectAccountWithLoadAwareness(catalogRouteCtx(entryID, APIProtocolGemini), "", "gemini-2.5-pro", nil)
			require.ErrorIs(t, err, ErrNoAvailableAccounts)
			require.Nil(t, result)
		})
	}
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
			require.Equal(t, tt.want, AccountKeepsHTTPPreviousResponseID(&tt.account))
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

func TestOpenAIQuotaPauseDecision_KeysIgnoreLabel(t *testing.T) {
	codexExtra := func() map[string]any {
		return map[string]any{
			"codex_5h_used_percent":   96.0,
			"auto_pause_5h_threshold": 0.95,
			"codex_usage_updated_at":  time.Now().UTC().Format(time.RFC3339),
		}
	}
	anthropicLabelledKey := schedulingTestKey(1, PlatformAnthropic, map[string]string{APIProtocolResponses: schedulingTestRelayURL})
	anthropicLabelledKey.Extra = codexExtra()
	_, _, paused := openAIQuotaPauseDecision(&anthropicLabelledKey, OpsOpenAIAccountQuotaAutoPauseSettings{}, time.Now())
	require.True(t, paused)

	anthropicOAuth := Account{ID: 2, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Extra: codexExtra()}
	_, _, paused = openAIQuotaPauseDecision(&anthropicOAuth, OpsOpenAIAccountQuotaAutoPauseSettings{}, time.Now())
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

	selected, err := svc.SelectAccountForAIStudioEndpoints(context.Background())
	require.NoError(t, err)
	require.Equal(t, geminiEndpointAnthropicLabel.ID, selected.ID)

	repo.accounts = []Account{anthropicEndpointGeminiLabel}
	_, err = svc.SelectAccountForAIStudioEndpoints(context.Background())
	require.Error(t, err)
}
