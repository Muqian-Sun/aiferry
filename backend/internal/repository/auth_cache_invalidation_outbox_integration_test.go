//go:build integration

package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAuthCacheInvalidationTriggers_CoverSecurityMutationsOnly(t *testing.T) {
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	user := mustCreateUser(t, integrationEntClient, &service.User{
		Email: fmt.Sprintf("auth-outbox-%d@example.com", suffix), Concurrency: 5,
	})
	keyValue := fmt.Sprintf("sk-auth-outbox-%d", suffix)
	apiKeyRepo := NewAPIKeyRepository(integrationEntClient, integrationDB)
	key := &service.APIKey{UserID: user.ID, Key: keyValue, Name: "outbox", Status: service.StatusActive}
	require.NoError(t, apiKeyRepo.Create(ctx, key))

	sum := sha256.Sum256([]byte(keyValue))
	cacheKey := hex.EncodeToString(sum[:])
	clear := func() {
		_, err := integrationDB.ExecContext(ctx, "DELETE FROM auth_cache_invalidation_outbox WHERE cache_key = $1", cacheKey)
		require.NoError(t, err)
	}
	count := func() int {
		var value int
		require.NoError(t, integrationDB.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM auth_cache_invalidation_outbox WHERE cache_key = $1", cacheKey).Scan(&value))
		return value
	}
	clear()
	t.Cleanup(clear)
	t.Cleanup(func() {
		// 共享集成库：用例结束后硬删自己造的行，最后一轮 clear 会清掉硬删触发的失效记录。
		_, err := integrationDB.ExecContext(ctx, "DELETE FROM api_keys WHERE id = $1", key.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, "DELETE FROM users WHERE id = $1", user.ID)
		require.NoError(t, err)
	})

	_, err := integrationDB.ExecContext(ctx, `
		UPDATE api_keys
		SET quota_used = quota_used + 1,
			usage_5h = usage_5h + 1,
			last_used_at = NOW()
		WHERE id = $1`, key.ID)
	require.NoError(t, err)
	require.Zero(t, count(), "usage-only key updates must not enqueue")

	_, err = integrationDB.ExecContext(ctx, "UPDATE api_keys SET status = 'disabled' WHERE id = $1", key.ID)
	require.NoError(t, err)
	require.Equal(t, 1, count(), "key disable must enqueue")
	clear()
	_, err = integrationDB.ExecContext(ctx, "UPDATE api_keys SET status = 'active' WHERE id = $1", key.ID)
	require.NoError(t, err)
	clear()

	userRepo := NewUserRepository(integrationEntClient, integrationDB)
	loadedUser, err := userRepo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	_, err = userRepo.AdjustBalance(ctx, loadedUser.ID, 10)
	require.NoError(t, err)
	require.Zero(t, count(), "balance-only user updates must not enqueue")

	_, err = integrationDB.ExecContext(ctx, "UPDATE users SET status = 'disabled' WHERE id = $1", user.ID)
	require.NoError(t, err)
	require.Equal(t, 1, count(), "user disable must enqueue all active keys")
	clear()
	_, err = integrationDB.ExecContext(ctx, "UPDATE users SET status = 'active' WHERE id = $1", user.ID)
	require.NoError(t, err)
	clear()

	require.NoError(t, apiKeyRepo.DeleteWithAudit(ctx, key.ID))
	require.Equal(t, 1, count(), "tombstone delete must hash OLD.key exactly once")
	var stored string
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT cache_key FROM auth_cache_invalidation_outbox WHERE cache_key = $1 LIMIT 1", cacheKey).Scan(&stored))
	require.Equal(t, cacheKey, stored)
	require.NotContains(t, stored, keyValue)
}
