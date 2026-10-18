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

// 时间窗内按模型 × 渠道合计（跨平台相加），窗外的分钟不算。
func TestSchedulingTrafficSumsWindowByModelAndAccount(t *testing.T) {
	ctx := context.Background()
	repo := NewSchedulingTrafficRepository(integrationDB)
	model := fmt.Sprintf("scheduling-traffic-%d", time.Now().UnixNano())
	now := time.Now().UTC().Truncate(time.Minute)
	insert := func(bucket time.Time, platform string, accountID, success, errors int64) {
		_, err := integrationDB.ExecContext(ctx, `
			INSERT INTO channel_monitor_v2_metrics_1m (bucket_start, platform, model, account_id, success_requests, error_requests)
			VALUES ($1, $2, $3, $4, $5, $6)`, bucket, platform, model, accountID, success, errors)
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM channel_monitor_v2_metrics_1m WHERE model = $1`, model)
	})
	insert(now.Add(-5*time.Minute), "anthropic", 11, 3, 1)
	insert(now.Add(-5*time.Minute), "openai", 11, 2, 0)
	insert(now.Add(-2*time.Minute), "anthropic", 0, 0, 4)
	insert(now.Add(-40*time.Minute), "anthropic", 11, 100, 100)

	rows, _, err := repo.SchedulingTraffic(ctx, now.Add(-15*time.Minute))
	require.NoError(t, err)
	got := map[int64]service.SchedulingTrafficRow{}
	for _, row := range rows {
		if row.Model == model {
			got[row.AccountID] = row
		}
	}
	require.Equal(t, map[int64]service.SchedulingTrafficRow{
		11: {Model: model, AccountID: 11, Success: 5, Errors: 1},
		0:  {Model: model, AccountID: 0, Success: 0, Errors: 4},
	}, got)
}
