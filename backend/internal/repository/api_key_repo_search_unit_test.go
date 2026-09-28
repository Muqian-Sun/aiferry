package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 管理站选密钥不显示内部 id，同名密钥靠所属用户邮箱区分：搜索结果必须带出所属用户。
func TestAPIKeyRepositorySearchAPIKeysAttachesOwner(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	alice := mustCreateAPIKeyRepoUser(t, ctx, client, "search-owner-alice@test.com")
	bob := mustCreateAPIKeyRepoUser(t, ctx, client, "search-owner-bob@test.com")

	for _, owner := range []*service.User{alice, bob} {
		require.NoError(t, repo.Create(ctx, &service.APIKey{
			UserID: owner.ID,
			Key:    "sk-search-owner-" + owner.Email,
			Name:   "search-owner-default",
			Status: service.StatusActive,
		}))
	}

	keys, err := repo.SearchAPIKeys(ctx, 0, "search-owner-default", 10)
	require.NoError(t, err)
	require.Len(t, keys, 2)

	ownerEmails := make(map[int64]string, len(keys))
	for _, key := range keys {
		require.NotNil(t, key.User, "密钥 %d 没带出所属用户", key.ID)
		ownerEmails[key.UserID] = key.User.Email
	}
	require.Equal(t, map[int64]string{
		alice.ID: "search-owner-alice@test.com",
		bob.ID:   "search-owner-bob@test.com",
	}, ownerEmails)
}
