//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// 过期自动暂停写死开（2026-09-28 P5）：到期扫表不再看 auto_pause_on_expired，
// 库里旧行留着的 false 也照样到期停调；没到期、没设过期时间的不动。
func TestAutoPauseExpiredAccountsIgnoresLegacyOptOut(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := newAccountRepositoryWithSQL(client, tx, nil)
	now := time.Now()

	legacyOptOut := mustCreateAccount(t, client, &service.Account{Name: "expiry-legacy-opt-out", Schedulable: true})
	_, err := client.Account.UpdateOneID(legacyOptOut.ID).SetExpiresAt(now.Add(-time.Hour)).SetAutoPauseOnExpired(false).Save(ctx)
	require.NoError(t, err)
	notYetExpired := mustCreateAccount(t, client, &service.Account{Name: "expiry-future", Schedulable: true})
	_, err = client.Account.UpdateOneID(notYetExpired.ID).SetExpiresAt(now.Add(time.Hour)).Save(ctx)
	require.NoError(t, err)
	noExpiry := mustCreateAccount(t, client, &service.Account{Name: "expiry-none", Schedulable: true})

	updated, err := repo.AutoPauseExpiredAccounts(ctx, now)
	require.NoError(t, err)
	require.GreaterOrEqual(t, updated, int64(1))

	for _, tc := range []struct {
		account     *service.Account
		schedulable bool
	}{
		{legacyOptOut, false},
		{notYetExpired, true},
		{noExpiry, true},
	} {
		loaded, err := client.Account.Get(ctx, tc.account.ID)
		require.NoError(t, err)
		require.Equal(t, tc.schedulable, loaded.Schedulable, tc.account.Name)
	}
}

// 日 / 周限额写死按滚动窗口重置（2026-09-28 P5）：库里旧行留着的 fixed 重置方式、到点时间都不再生效，
// 用量只在「周期起点 + 24 小时 / 7 天」之后才清零，也不再改写 quota_*_reset_at。
// 计费热路径（UsageBillingRepository.Apply）和 IncrementQuotaUsed 共用同一套周期表达式，两条都验。
func TestAccountQuotaIncrementIgnoresLegacyFixedReset(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	now := time.Now().UTC()
	dailyStart := now.Add(-time.Hour).Format(time.RFC3339)
	weeklyStart := now.Add(-48 * time.Hour).Format(time.RFC3339)
	// 旧的固定时间重置：到点时间已过，按旧逻辑这一笔会先清零再累加
	pastResetAt := now.Add(-10 * time.Minute).Format(time.RFC3339)
	legacyExtra := func() map[string]any {
		return map[string]any{
			"quota_daily_limit": 100.0, "quota_daily_used": 5.0, "quota_daily_start": dailyStart,
			"quota_daily_reset_mode": "fixed", "quota_daily_reset_hour": 0.0, "quota_daily_reset_at": pastResetAt,
			"quota_weekly_limit": 500.0, "quota_weekly_used": 7.0, "quota_weekly_start": weeklyStart,
			"quota_weekly_reset_mode": "fixed", "quota_weekly_reset_day": 1.0, "quota_weekly_reset_hour": 0.0,
			"quota_weekly_reset_at": pastResetAt, "quota_reset_timezone": "UTC",
		}
	}
	requireRolling := func(t *testing.T, q sqlExecutor, accountID int64) {
		t.Helper()
		var dailyUsed, weeklyUsed float64
		var gotDailyStart, gotDailyResetAt, gotWeeklyResetAt string
		rows, err := q.QueryContext(ctx, `
			SELECT (extra->>'quota_daily_used')::numeric, (extra->>'quota_weekly_used')::numeric,
				extra->>'quota_daily_start', extra->>'quota_daily_reset_at', extra->>'quota_weekly_reset_at'
			FROM accounts WHERE id = $1`, accountID)
		require.NoError(t, err)
		defer func() { _ = rows.Close() }()
		require.True(t, rows.Next(), "account row missing")
		require.NoError(t, rows.Scan(&dailyUsed, &weeklyUsed, &gotDailyStart, &gotDailyResetAt, &gotWeeklyResetAt))
		require.NoError(t, rows.Err())
		require.InDelta(t, 7.0, dailyUsed, 1e-6, "日用量在 24 小时窗口内累加，不按旧的固定时间清零")
		require.InDelta(t, 9.0, weeklyUsed, 1e-6, "周用量在 7 天窗口内累加")
		require.Equal(t, dailyStart, gotDailyStart, "周期起点不动")
		require.Equal(t, pastResetAt, gotDailyResetAt, "不再改写 quota_daily_reset_at")
		require.Equal(t, pastResetAt, gotWeeklyResetAt, "不再改写 quota_weekly_reset_at")
	}

	t.Run("usage billing apply", func(t *testing.T) {
		// Apply 自己开事务，只能走真实提交的连接：建的行在用例结束时删掉，免得污染按文件顺序排在后面、
		// 按全表计数的仓储用例（TestAccountRepoSuite 的 List 系列等）。
		requestID := uuid.NewString()
		user := mustCreateUser(t, client, &service.User{
			Email:        fmt.Sprintf("rolling-quota-user-%d@example.com", time.Now().UnixNano()),
			PasswordHash: "hash",
		})
		apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-rolling-quota-" + uuid.NewString(), Name: "rolling"})
		account := mustCreateAccount(t, client, &service.Account{
			Name:              "rolling-quota-billing-" + uuid.NewString(),
			Type:              service.AccountTypeAPIKey,
			ProtocolEndpoints: map[string]string{service.APIProtocolAnthropic: "https://api.anthropic.com"},
			Extra:             legacyExtra(),
		})
		t.Cleanup(func() {
			for _, stmt := range []struct {
				query string
				arg   any
			}{
				{"DELETE FROM usage_billing_dedup WHERE request_id = $1", requestID},
				{"DELETE FROM scheduler_outbox WHERE account_id = $1", account.ID},
				{"DELETE FROM accounts WHERE id = $1", account.ID},
				{"DELETE FROM api_keys WHERE id = $1", apiKey.ID},
				{"DELETE FROM users WHERE id = $1", user.ID},
			} {
				_, err := integrationDB.ExecContext(context.Background(), stmt.query, stmt.arg)
				require.NoError(t, err, stmt.query)
			}
		})
		_, err := NewUsageBillingRepository(client, integrationDB).Apply(ctx, &service.UsageBillingCommand{
			RequestID:        requestID,
			APIKeyID:         apiKey.ID,
			UserID:           user.ID,
			AccountID:        account.ID,
			AccountType:      service.AccountTypeAPIKey,
			AccountQuotaCost: 2,
		})
		require.NoError(t, err)
		requireRolling(t, integrationDB, account.ID)
	})

	t.Run("increment quota used", func(t *testing.T) {
		tx := testEntTx(t)
		txClient := tx.Client()
		account := mustCreateAccount(t, txClient, &service.Account{
			Name:              "rolling-quota-increment-" + uuid.NewString(),
			Type:              service.AccountTypeAPIKey,
			ProtocolEndpoints: map[string]string{service.APIProtocolAnthropic: "https://api.anthropic.com"},
			Extra:             legacyExtra(),
		})
		require.NoError(t, newAccountRepositoryWithSQL(txClient, tx, nil).IncrementQuotaUsed(ctx, account.ID, 2))
		requireRolling(t, tx, account.ID)
	})
}
