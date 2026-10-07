//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 复制渠道（muqian 2026-10-07：只复制 key 和端点，其余同新建）：key 留空时由后端从源渠道取存着的 key。
func TestCreateAccountCopiesKeyFromSourceChannel(t *testing.T) {
	ctx := context.Background()
	newEnv := func(t *testing.T) (*adminServiceImpl, *sparkShadowRepoStub, *Account) {
		t.Helper()
		repo := newSparkShadowRepoStub()
		source := &Account{
			Name: "fenno", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
			Credentials:       map[string]any{"api_key": "sk-source-secret"},
			ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://relay.example.com"},
		}
		require.NoError(t, repo.Create(ctx, source))
		return &adminServiceImpl{accountRepo: repo}, repo, source
	}
	input := func(sourceID int64, credentials map[string]any) *CreateAccountInput {
		return &CreateAccountInput{
			Name: "fenno 2", Type: AccountTypeAPIKey, Credentials: credentials,
			ProtocolEndpoints:    map[string]string{APIProtocolResponses: "https://relay.example.com/v1"},
			CopyKeyFromAccountID: sourceID,
		}
	}

	t.Run("empty key uses the source key", func(t *testing.T) {
		svc, _, source := newEnv(t)
		created, err := svc.CreateAccount(ctx, input(source.ID, map[string]any{}))
		require.NoError(t, err)
		require.Equal(t, "sk-source-secret", created.Credentials["api_key"])
		require.Equal(t, map[string]string{APIProtocolResponses: "https://relay.example.com/v1"}, created.ProtocolEndpoints, "端点按表单的")
		require.True(t, created.Schedulable, "其余同新建：不再像旧的复制那样停调")
	})

	t.Run("typed key wins", func(t *testing.T) {
		svc, _, source := newEnv(t)
		created, err := svc.CreateAccount(ctx, input(source.ID, map[string]any{"api_key": "sk-new"}))
		require.NoError(t, err)
		require.Equal(t, "sk-new", created.Credentials["api_key"])
	})

	t.Run("source must be an API key channel", func(t *testing.T) {
		svc, repo, _ := newEnv(t)
		oauth := &Account{Name: "claude", Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive,
			Credentials: map[string]any{"access_token": "at"}}
		require.NoError(t, repo.Create(ctx, oauth))
		_, err := svc.CreateAccount(ctx, input(oauth.ID, map[string]any{}))
		require.ErrorContains(t, err, "only API key channels can be copied")
	})

	t.Run("source without a key", func(t *testing.T) {
		svc, repo, source := newEnv(t)
		stored, _ := repo.GetByID(ctx, source.ID)
		stored.Credentials = map[string]any{}
		_, err := svc.CreateAccount(ctx, input(source.ID, map[string]any{}))
		require.ErrorContains(t, err, "the source channel has no API key")
	})

	t.Run("only API key channels can copy", func(t *testing.T) {
		svc, _, source := newEnv(t)
		in := input(source.ID, map[string]any{"access_token": "at"})
		in.Type, in.Platform = AccountTypeOAuth, PlatformAnthropic
		_, err := svc.CreateAccount(ctx, in)
		require.ErrorContains(t, err, "only API key channels can copy a key")
	})
}
