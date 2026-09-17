//go:build unit

package service

import (
	"context"
	"slices"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

// 模型可用性诊断、网关模型列表与管理端模型候选按调度同一套平台准入规则统计第三方 key：
// 看协议地址，不看平台标签。诊断带着请求的入站协议；没有入站请求的模型列表与管理端候选
// 按「该网关上任一入站协议可承接」统计。

func anthropicEndpointKeyWithMapping(id int64, groupID int64) Account {
	key := schedulingTestKey(id, PlatformOpenAI, map[string]string{APIProtocolAnthropic: schedulingTestRelayURL}, groupID)
	key.Credentials = map[string]any{"model_mapping": map[string]any{"claude-relay-custom": "claude-sonnet-4-5"}}
	return key
}

func TestDiagnoseModelAvailabilityForPlatform_CountsKeysByProtocolNotLabel(t *testing.T) {
	groupID := int64(21101)
	key := anthropicEndpointKeyWithMapping(21111, groupID)
	repo := &mockAccountRepoForPlatform{accounts: []Account{key}, accountsByID: map[int64]*Account{}}
	svc := &GatewayService{accountRepo: repo, cfg: testConfig()}
	ctx := WithInboundProtocol(context.Background(), APIProtocolAnthropic)

	diag := svc.DiagnoseModelAvailabilityForPlatform(ctx, &groupID, "claude-relay-custom", PlatformAnthropic)
	require.True(t, diag.HasAccountsInPool)
	require.True(t, diag.HasModelSupport)

	diag = svc.DiagnoseModelAvailabilityForPlatform(ctx, &groupID, "claude-relay-custom", PlatformGemini)
	require.False(t, diag.HasAccountsInPool)
	require.False(t, diag.HasModelSupport)
}

func TestOpenAIDiagnoseModelAvailabilityForPlatform_CountsKeysByInboundProtocol(t *testing.T) {
	groupID := int64(21102)
	key := schedulingTestKey(21112, PlatformAnthropic, map[string]string{APIProtocolChatCompletions: schedulingTestRelayURL}, groupID)
	key.Credentials = map[string]any{"model_mapping": map[string]any{"gpt-relay-custom": "gpt-5.1"}}
	repo := &mockAccountRepoForPlatform{accounts: []Account{key}, accountsByID: map[int64]*Account{}}
	svc := &OpenAIGatewayService{accountRepo: repo, cfg: testConfig()}

	chatCtx := WithInboundProtocol(context.Background(), APIProtocolChatCompletions)
	diag := svc.DiagnoseModelAvailabilityForPlatform(chatCtx, &groupID, "gpt-relay-custom", PlatformOpenAI)
	require.True(t, diag.HasAccountsInPool)
	require.True(t, diag.HasModelSupport)

	geminiCtx := WithInboundProtocol(context.Background(), APIProtocolGemini)
	diag = svc.DiagnoseModelAvailabilityForPlatform(geminiCtx, &groupID, "gpt-relay-custom", PlatformOpenAI)
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
			require.Equal(t, tt.want, accountServesPlatformForAnyInbound(&tt.account, tt.platform))
		})
	}
}

func TestGetAvailableModels_CountsKeysByProtocolNotLabel(t *testing.T) {
	groupID := int64(21103)
	repo := &accountRepoStubForCompositeModelsList{accounts: []Account{anthropicEndpointKeyWithMapping(21113, groupID)}}
	svc := &GatewayService{accountRepo: repo}

	require.Equal(t, []string{"claude-relay-custom"}, svc.GetAvailableModels(context.Background(), &groupID, PlatformAnthropic))
	// OpenAI 网关能把入站 Messages 以 anthropic 协议转发，只有 anthropic 地址的 key 也计入。
	require.Equal(t, []string{"claude-relay-custom"}, svc.GetAvailableModels(context.Background(), &groupID, PlatformOpenAI))
	// Gemini 网关只以 gemini 协议转发。
	require.Nil(t, svc.GetAvailableModels(context.Background(), &groupID, PlatformGemini))
}

func TestGetAvailableModels_CountsResponsesOnlyKeyOnOpenAIGateway(t *testing.T) {
	groupID := int64(21105)
	key := schedulingTestKey(21115, PlatformAnthropic, map[string]string{APIProtocolResponses: schedulingTestRelayURL}, groupID)
	key.Credentials = map[string]any{"model_mapping": map[string]any{"gpt-relay-responses": "gpt-5.1"}}
	repo := &accountRepoStubForCompositeModelsList{accounts: []Account{key}}
	svc := &GatewayService{accountRepo: repo}

	require.Equal(t, []string{"gpt-relay-responses"}, svc.GetAvailableModels(context.Background(), &groupID, PlatformOpenAI))
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

func TestOpenAIConfiguredCodexModelIDs_CountsKeysByProtocolNotLabel(t *testing.T) {
	responsesKey := schedulingTestKey(1, PlatformAnthropic, map[string]string{APIProtocolResponses: schedulingTestRelayURL})
	responsesKey.Credentials = map[string]any{"model_mapping": map[string]any{"gpt-relay-responses": "gpt-5.1"}}
	geminiKey := schedulingTestKey(2, PlatformOpenAI, map[string]string{APIProtocolGemini: schedulingTestRelayURL})
	geminiKey.Credentials = map[string]any{"model_mapping": map[string]any{"gemini-relay-only": "gemini-2.5-pro"}}

	require.Equal(t, []string{"gpt-relay-responses"}, openAIConfiguredCodexModelIDs([]Account{responsesKey, geminiKey}))

	// 分组白名单里的通配映射模型同样按 OpenAI 网关能否承接统计 key。
	wildcardKey := schedulingTestKey(3, PlatformAnthropic, map[string]string{APIProtocolResponses: schedulingTestRelayURL})
	wildcardKey.Credentials = map[string]any{"model_mapping": map[string]any{"gpt-wild-*": "gpt-5.1"}}
	geminiWildcardKey := schedulingTestKey(4, PlatformOpenAI, map[string]string{APIProtocolGemini: schedulingTestRelayURL})
	geminiWildcardKey.Credentials = map[string]any{"model_mapping": map[string]any{"gemini-wild-*": "gemini-2.5-pro"}}
	group := &Group{ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"gpt-wild-one", "gemini-wild-one"}}}
	require.Equal(t, []string{"gpt-wild-one"}, openAIConfiguredCodexModelIDsForGroup([]Account{wildcardKey, geminiWildcardKey}, group))
}
