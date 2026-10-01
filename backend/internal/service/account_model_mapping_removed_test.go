//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 渠道上的「模型改名」已删（D4）：管理端建号 / 改号 / 批量改号带 model_mapping 或 model_mapping_rename_only
// 直接 400，不静默存下也不静默丢掉；spark 影子号的模型列表走影子号自己的路径，照旧可写。
func TestAdminAccountWritesRejectRemovedModelMapping(t *testing.T) {
	ctx := context.Background()
	relayCredentials := func(extra map[string]any) map[string]any {
		creds := map[string]any{"api_key": "sk-test"}
		for k, v := range extra {
			creds[k] = v
		}
		return creds
	}

	t.Run("create", func(t *testing.T) {
		repo := newSparkShadowRepoStub()
		svc := &adminServiceImpl{accountRepo: repo}
		_, err := svc.CreateAccount(ctx, &CreateAccountInput{
			Name:              "relay",
			Type:              AccountTypeAPIKey,
			ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://relay.example.com/v1"},
			Credentials:       relayCredentials(map[string]any{"model_mapping": map[string]any{"gpt-5.5": "auto"}}),
		})
		require.ErrorContains(t, err, "ACCOUNT_MODEL_MAPPING_REMOVED")
		require.Empty(t, repo.accounts, "nothing created")
	})

	t.Run("update", func(t *testing.T) {
		repo := newSparkShadowRepoStub()
		svc := &adminServiceImpl{accountRepo: repo}
		existing := &Account{Name: "relay", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive,
			Credentials: relayCredentials(nil)}
		require.NoError(t, repo.Create(ctx, existing))

		_, err := svc.UpdateAccount(ctx, existing.ID, &UpdateAccountInput{
			Credentials: relayCredentials(map[string]any{"model_mapping_rename_only": true}),
		})
		require.ErrorContains(t, err, "ACCOUNT_MODEL_MAPPING_REMOVED")
		stored, _ := repo.GetByID(ctx, existing.ID)
		require.NotContains(t, stored.Credentials, "model_mapping_rename_only", "nothing written")
	})

	t.Run("bulk update", func(t *testing.T) {
		repo := newSparkShadowRepoStub()
		svc := &adminServiceImpl{accountRepo: repo}
		existing := &Account{Name: "relay", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive,
			Credentials: relayCredentials(nil)}
		require.NoError(t, repo.Create(ctx, existing))

		_, err := svc.BulkUpdateAccounts(ctx, &BulkUpdateAccountsInput{
			AccountIDs:  []int64{existing.ID},
			Credentials: map[string]any{"model_mapping": map[string]any{}},
		})
		require.ErrorContains(t, err, "ACCOUNT_MODEL_MAPPING_REMOVED")
	})

	t.Run("spark shadow keeps its model list", func(t *testing.T) {
		repo := newSparkShadowRepoStub()
		svc := &adminServiceImpl{accountRepo: repo}
		parent := &Account{Name: "parent", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive}
		require.NoError(t, repo.Create(ctx, parent))
		shadow := &Account{Name: "shadow", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive,
			ParentAccountID: &parent.ID, QuotaDimension: QuotaDimensionSpark,
			Credentials: map[string]any{"model_mapping": defaultSparkShadowModelMapping()}}
		require.NoError(t, repo.Create(ctx, shadow))

		_, err := svc.UpdateAccount(ctx, shadow.ID, &UpdateAccountInput{
			Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5.3-codex-spark": "gpt-5.3-codex-spark"}},
		})
		require.NoError(t, err)
	})
}
