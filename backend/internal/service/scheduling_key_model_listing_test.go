//go:build unit

package service

import (
	"context"
	"slices"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

// 模型可用性诊断与管理端模型候选：诊断走调度那套池规则（平台池 = 平台相等 + 入站协议可承接，
// 目录路由 = 条目绑定 + 协议转换注册表）；没有入站请求的管理端候选按「能不能承接」统计，不看标签。

func anthropicEndpointKeyWithMapping(id int64, groupID int64) Account {
	key := schedulingTestKey(id, PlatformOpenAI, map[string]string{APIProtocolAnthropic: schedulingTestRelayURL}, groupID)
	key.Credentials = map[string]any{"model_mapping": map[string]any{"claude-relay-custom": "claude-sonnet-4-5"}}
	return key
}

// 无路由的诊断按平台池：key 只算在自己标签的池里（D18）。
func TestDiagnoseModelAvailabilityForPlatform_PlatformPoolCountsKeysByLabel(t *testing.T) {
	key := anthropicEndpointKeyWithMapping(21111, 0)
	repo := &mockAccountRepoForPlatform{accounts: []Account{key}, accountsByID: map[int64]*Account{}}
	svc := &GatewayService{accountRepo: repo, cfg: testConfig()}
	ctx := WithInboundProtocol(context.Background(), APIProtocolAnthropic)

	// key 标签是 openai、配 anthropic 地址：openai 池里算数（message 入站转 anthropic）
	diag := svc.DiagnoseModelAvailabilityForPlatform(ctx, "claude-relay-custom", PlatformOpenAI)
	require.True(t, diag.HasAccountsInPool)
	require.True(t, diag.HasModelSupport)

	// anthropic 池里没有它（标签不符）
	diag = svc.DiagnoseModelAvailabilityForPlatform(ctx, "claude-relay-custom", PlatformAnthropic)
	require.False(t, diag.HasAccountsInPool)
	require.False(t, diag.HasModelSupport)

	diag = svc.DiagnoseModelAvailabilityForPlatform(ctx, "claude-relay-custom", PlatformGemini)
	require.False(t, diag.HasAccountsInPool)
	require.False(t, diag.HasModelSupport)
}

// 目录路由下诊断按条目绑定：绑了但不支持该模型 → {true,false}（404）；没绑任何账号 → {false,false}（503）；
// 没绑到条目的账号哪怕支持模型也不算。
func TestDiagnoseModelAvailabilityForPlatform_CatalogRouteUsesBindings(t *testing.T) {
	const entryID = int64(21120)
	bound := anthropicEndpointKeyWithMapping(21121, 0)
	bound.CatalogEntryIDs = []int64{entryID}
	unbound := anthropicEndpointKeyWithMapping(21122, 0)
	unbound.Credentials = map[string]any{"model_mapping": map[string]any{"claude-other": "claude-sonnet-4-5"}}
	repo := &mockAccountRepoForPlatform{accounts: []Account{bound, unbound}, accountsByID: map[int64]*Account{}}
	svc := &GatewayService{accountRepo: repo, cfg: testConfig()}
	ctx := catalogRouteCtx(entryID, APIProtocolAnthropic)

	diag := svc.DiagnoseModelAvailabilityForPlatform(ctx, "claude-relay-custom", PlatformAnthropic)
	require.True(t, diag.HasAccountsInPool)
	require.True(t, diag.HasModelSupport, "绑定账号的映射含该模型")

	diag = svc.DiagnoseModelAvailabilityForPlatform(ctx, "claude-other", PlatformAnthropic)
	require.True(t, diag.HasAccountsInPool)
	require.False(t, diag.HasModelSupport, "只有没绑到条目的账号支持该模型：按绑定看是不支持 → 404")

	diag = svc.DiagnoseModelAvailabilityForPlatform(catalogRouteCtx(entryID+1, APIProtocolAnthropic), "claude-relay-custom", PlatformAnthropic)
	require.False(t, diag.HasAccountsInPool, "条目没有绑定 → 503")
	require.False(t, diag.HasModelSupport)
}

func TestOpenAIDiagnoseModelAvailabilityForPlatform_CountsKeysByInboundProtocol(t *testing.T) {
	groupID := int64(21102)
	key := schedulingTestKey(21112, PlatformOpenAI, map[string]string{APIProtocolChatCompletions: schedulingTestRelayURL}, groupID)
	key.Credentials = map[string]any{"model_mapping": map[string]any{"gpt-relay-custom": "gpt-5.1"}}
	repo := &mockAccountRepoForPlatform{accounts: []Account{key}, accountsByID: map[int64]*Account{}}
	svc := &OpenAIGatewayService{accountRepo: repo, cfg: testConfig()}

	chatCtx := WithInboundProtocol(context.Background(), APIProtocolChatCompletions)
	diag := svc.DiagnoseModelAvailabilityForPlatform(chatCtx, "gpt-relay-custom", PlatformOpenAI)
	require.True(t, diag.HasAccountsInPool)
	require.True(t, diag.HasModelSupport)

	geminiCtx := WithInboundProtocol(context.Background(), APIProtocolGemini)
	diag = svc.DiagnoseModelAvailabilityForPlatform(geminiCtx, "gpt-relay-custom", PlatformOpenAI)
	require.False(t, diag.HasAccountsInPool)
}

func TestAccountServesPlatformForAnyInbound(t *testing.T) {
	responsesOnlyAnthropicLabel := schedulingTestKey(1, PlatformAnthropic, map[string]string{APIProtocolResponses: schedulingTestRelayURL})
	geminiOnlyOpenAILabel := schedulingTestKey(2, PlatformOpenAI, map[string]string{APIProtocolGemini: schedulingTestRelayURL})
	antigravityMixed := Account{Platform: PlatformAntigravity, Type: AccountTypeOAuth, Extra: map[string]any{"mixed_scheduling": true}}
	openAIOAuth := Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	tests := []struct {
		name     string
		account  Account
		platform string
		want     bool
	}{
		// 只有 responses 地址的 key 承接不了扩展端点（入站协议为空只认 chat_completions），
		// 但能承接 OpenAI 网关上的 Responses 入站，模型列表要算上它。
		{"key responses endpoint on openai gateway", responsesOnlyAnthropicLabel, PlatformOpenAI, true},
		{"key responses endpoint on grok gateway", responsesOnlyAnthropicLabel, PlatformGrok, true},
		{"key responses endpoint on its label gateway", responsesOnlyAnthropicLabel, PlatformAnthropic, false},
		{"key gemini endpoint on antigravity gateway", geminiOnlyOpenAILabel, PlatformAntigravity, true},
		{"key gemini endpoint on its label gateway", geminiOnlyOpenAILabel, PlatformOpenAI, false},
		{"subscription same platform", openAIOAuth, PlatformOpenAI, true},
		{"subscription other platform", openAIOAuth, PlatformGrok, false},
		{"antigravity mixed subscription not listed on anthropic", antigravityMixed, PlatformAnthropic, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, AccountServesPlatformForAnyInbound(&tt.account, tt.platform))
		})
	}
}

func TestAdminGroupModelsListCandidates_CountsKeysByProtocolNotLabel(t *testing.T) {
	groupID := int64(21104)
	responsesKey := schedulingTestKey(21116, PlatformAnthropic, map[string]string{APIProtocolResponses: schedulingTestRelayURL}, groupID)
	responsesKey.Credentials = map[string]any{"model_mapping": map[string]any{"gpt-relay-responses": "gpt-5.1"}}
	accountRepo := &accountRepoStubForCompositeModelsList{accounts: []Account{anthropicEndpointKeyWithMapping(21114, groupID), responsesKey}}
	groupRepo := &groupRepoStubForAdmin{getByIDByID: map[int64]*Group{
		groupID: {ID: groupID, Platform: PlatformAnthropic},
	}}
	svc := &adminServiceImpl{accountRepo: accountRepo, groupRepo: groupRepo}

	candidates, err := svc.GetGroupModelsListCandidates(context.Background(), groupID, PlatformAnthropic)
	require.NoError(t, err)
	require.Contains(t, candidates, "claude-relay-custom")
	require.NotContains(t, candidates, "gpt-relay-responses")

	candidates, err = svc.GetGroupModelsListCandidates(context.Background(), groupID, PlatformOpenAI)
	require.NoError(t, err)
	require.Contains(t, candidates, "claude-relay-custom")
	require.Contains(t, candidates, "gpt-relay-responses")

	candidates, err = svc.GetGroupModelsListCandidates(context.Background(), groupID, PlatformGemini)
	require.NoError(t, err)
	require.NotContains(t, candidates, "claude-relay-custom")
	require.NotContains(t, candidates, "gpt-relay-responses")
}

// 空映射账号按 OpenAI 默认目录补齐：官方 OpenAI 与通用中转补，已知其他厂商与 OpenAI 网关
// 承接不了的 key 不补，平台标签不参与。
func TestSupplementUnmappedOpenAIModels_ByVendorNotLabel(t *testing.T) {
	mappedModels := []string{"gpt-mapped-only"}
	defaultModel := openai.DefaultModelIDs()[0]

	tests := []struct {
		name    string
		account Account
		want    bool
	}{
		{"relay key with anthropic label", schedulingTestKey(1, PlatformAnthropic, map[string]string{APIProtocolChatCompletions: schedulingTestRelayURL}), true},
		{"official openai key with deepseek label", schedulingTestKey(2, PlatformDeepseek, map[string]string{APIProtocolResponses: "https://api.openai.com"}), true},
		{"official deepseek key with openai label", schedulingTestKey(3, PlatformOpenAI, map[string]string{APIProtocolChatCompletions: DefaultDeepseekBaseURL}), false},
		{"relay key without openai gateway endpoint", schedulingTestKey(4, PlatformOpenAI, map[string]string{APIProtocolGemini: schedulingTestRelayURL}), false},
		{"openai oauth", Account{ID: 5, Platform: PlatformOpenAI, Type: AccountTypeOAuth}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := supplementUnmappedOpenAIModels([]Account{tt.account}, mappedModels)
			require.Equal(t, tt.want, slices.Contains(got, defaultModel))
			require.Contains(t, got, "gpt-mapped-only")
		})
	}
}
