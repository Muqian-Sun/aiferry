//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 复制账号落库的原子性：账号行与 scheduler_outbox 必须同生同死。
// 去分组后没有 account_groups 一起写，原子边界由 accountRepo.Create 自己兜。
func TestCreateAccountPersistsPausedCopyAtomically(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := newAccountRepositoryWithSQL(client, integrationDB, nil)
	suffix := time.Now().UnixNano()

	success := &service.Account{
		Name:              fmt.Sprintf("duplicate-success-%d", suffix),
		Platform:          service.PlatformAnthropic,
		Type:              service.AccountTypeAPIKey,
		ProtocolEndpoints: map[string]string{service.APIProtocolAnthropic: "https://api.anthropic.com"},
		Status:            service.StatusActive,
		Schedulable:       false,
		Credentials:       map[string]any{"api_key": "secret"},
		Extra:             map[string]any{},
	}
	require.NoError(t, repo.Create(ctx, success))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM scheduler_outbox WHERE account_id = $1", success.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = $1", success.ID)
	})

	var schedulable bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT schedulable FROM accounts WHERE id = $1", success.ID).Scan(&schedulable))
	require.False(t, schedulable, "复制出来的副本必须是暂停态")
	var outboxCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduler_outbox WHERE account_id = $1", success.ID).Scan(&outboxCount))
	require.Equal(t, 1, outboxCount, "建号必须且只能写一条调度 outbox")

	// 协议地址缺失：Create 在落库前就挡住，账号行与 outbox 都不能留下痕迹。
	failure := &service.Account{
		Name:        fmt.Sprintf("duplicate-failure-%d", suffix),
		Platform:    service.PlatformAnthropic,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: false,
		Credentials: map[string]any{"api_key": "secret"},
		Extra:       map[string]any{},
	}
	require.Error(t, repo.Create(ctx, failure))

	var accountCount, failedOutboxCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM accounts WHERE name = $1", failure.Name).Scan(&accountCount))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduler_outbox WHERE account_id = $1", failure.ID).Scan(&failedOutboxCount))
	require.Zero(t, accountCount)
	require.Zero(t, failedOutboxCount)
}
