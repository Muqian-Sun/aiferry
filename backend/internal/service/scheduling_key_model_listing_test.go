//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 模型可用性诊断、网关模型列表与管理端模型候选按调度同一套平台准入规则统计第三方 key：
// 看协议地址，不看平台标签。没有入站请求（模型列表、管理端）时按空入站协议判断。

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

func TestGetAvailableModels_CountsKeysByProtocolNotLabel(t *testing.T) {
	groupID := int64(21103)
	repo := &accountRepoStubForCompositeModelsList{accounts: []Account{anthropicEndpointKeyWithMapping(21113, groupID)}}
	svc := &GatewayService{accountRepo: repo}

	require.Equal(t, []string{"claude-relay-custom"}, svc.GetAvailableModels(context.Background(), &groupID, PlatformAnthropic))
	// 同一个 openai 标签的 key 没有 chat_completions 地址，不计入 OpenAI 分组的模型列表。
	require.Nil(t, svc.GetAvailableModels(context.Background(), &groupID, PlatformOpenAI))
}

func TestAdminGroupModelsListCandidates_CountsKeysByProtocolNotLabel(t *testing.T) {
	groupID := int64(21104)
	accountRepo := &accountRepoStubForCompositeModelsList{accounts: []Account{anthropicEndpointKeyWithMapping(21114, groupID)}}
	groupRepo := &groupRepoStubForAdmin{getByIDByID: map[int64]*Group{
		groupID: {ID: groupID, Platform: PlatformAnthropic},
	}}
	svc := &adminServiceImpl{accountRepo: accountRepo, groupRepo: groupRepo}

	candidates, err := svc.GetGroupModelsListCandidates(context.Background(), groupID, PlatformAnthropic)
	require.NoError(t, err)
	require.Contains(t, candidates, "claude-relay-custom")

	candidates, err = svc.GetGroupModelsListCandidates(context.Background(), groupID, PlatformOpenAI)
	require.NoError(t, err)
	require.NotContains(t, candidates, "claude-relay-custom")
}
