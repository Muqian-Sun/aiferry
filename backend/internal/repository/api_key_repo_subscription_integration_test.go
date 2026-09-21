//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/apikey"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/stretchr/testify/require"
)

// 订阅 key：在建订阅的同一事务里插入（外键指向未提交的订阅行）；ExistsBySubscriptionID 不算软删的 key。

// Create 必须走 ctx 里的事务：订阅行在 tx 内刚建、未提交，用仓储自己的 client 插 key 会外键失败。
// 回滚后订阅与 key 都不存在。
func TestAPIKeyRepo_CreateInTx_WithSubscriptionID_SameTx(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAPIKeyRepository(client, integrationDB)
	prefix := fmt.Sprintf("keytx-%d", time.Now().UnixNano())

	user := mustCreateUser(t, client, &service.User{Email: prefix + "@test.com"})
	plan := mustCreatePlan(t, client, &service.SubscriptionPlan{Name: prefix})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM api_keys WHERE user_id = $1", user.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM user_subscriptions WHERE user_id = $1", user.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM subscription_plans WHERE id = $1", plan.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
	})

	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	// Cleanup 后进先出：这条先于上面的 DELETE 跑，断言失败时事务也会释放订阅行的锁，DELETE 不会等死
	t.Cleanup(func() { _ = tx.Rollback() })
	txCtx := dbent.NewTxContext(ctx, tx)
	sub := mustCreateSubscription(t, tx.Client(), &service.UserSubscription{UserID: user.ID, PlanID: plan.ID})

	key := &service.APIKey{UserID: user.ID, Key: prefix + "-key", Name: plan.Name, Status: service.StatusActive, SubscriptionID: &sub.ID}
	// 若 Create 走了仓储自己的连接，外键检查会等这条未提交订阅行的锁：不是报错而是挂死，用超时把挂死变成失败
	createCtx, cancel := context.WithTimeout(txCtx, 5*time.Second)
	defer cancel()
	require.NoError(t, repo.Create(createCtx, key), "同事务内建 key（外键指向未提交的订阅行）")
	require.NotZero(t, key.ID)

	exists, err := repo.ExistsBySubscriptionID(txCtx, sub.ID)
	require.NoError(t, err)
	require.True(t, exists, "事务内可见")
	got, err := tx.Client().APIKey.Get(txCtx, key.ID)
	require.NoError(t, err)
	require.NotNil(t, got.SubscriptionID)
	require.Equal(t, sub.ID, *got.SubscriptionID)

	require.NoError(t, tx.Rollback())
	n, err := client.APIKey.Query().Where(apikey.KeyEQ(key.Key)).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, n, "回滚后 key 不存在")
	exists, err = repo.ExistsBySubscriptionID(ctx, sub.ID)
	require.NoError(t, err)
	require.False(t, exists)
}

func (s *APIKeyRepoSuite) TestExistsBySubscriptionID_IgnoresSoftDeleted() {
	user := s.mustCreateUser("subkey@test.com")
	plan := mustCreatePlan(s.T(), s.client, &service.SubscriptionPlan{Name: "p-subkey"})
	sub := mustCreateSubscription(s.T(), s.client, &service.UserSubscription{UserID: user.ID, PlanID: plan.ID})

	exists, err := s.repo.ExistsBySubscriptionID(s.ctx, sub.ID)
	s.Require().NoError(err)
	s.Require().False(exists, "还没发 key")

	key := &service.APIKey{UserID: user.ID, Key: "sk-subkey", Name: plan.Name, Status: service.StatusActive, SubscriptionID: &sub.ID}
	s.Require().NoError(s.repo.Create(s.ctx, key))
	exists, err = s.repo.ExistsBySubscriptionID(s.ctx, sub.ID)
	s.Require().NoError(err)
	s.Require().True(exists)

	got, err := s.repo.GetByID(s.ctx, key.ID)
	s.Require().NoError(err)
	s.Require().NotNil(got.SubscriptionID)
	s.Require().Equal(sub.ID, *got.SubscriptionID)
	s.Require().True(got.IsSubscriptionKey())

	// 管理端软删后 → 不存在（续期 / 恢复时据此补发）
	s.Require().NoError(s.repo.Delete(s.ctx, key.ID))
	exists, err = s.repo.ExistsBySubscriptionID(s.ctx, sub.ID)
	s.Require().NoError(err)
	s.Require().False(exists, "软删的 key 不算")
}
