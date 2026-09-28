//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 管理端渠道健康的错误明细（GET /admin/.../channel-monitor-v2/errors 带管理员明细）走 loadErrorDetails：
// 去分组时 SELECT 删了 group_id，但 GROUP BY 仍按 1..9 编号（第 9 列成了 COUNT(*)）且 Scan 仍多一个 groupID，
// 这条查询在 PostgreSQL 上直接报「aggregate functions are not allowed in GROUP BY」。
func TestChannelMonitorV2LoadErrorDetails_ReturnsSamples(t *testing.T) {
	ctx := context.Background()
	_, _ = integrationDB.ExecContext(ctx, "TRUNCATE ops_error_logs RESTART IDENTITY CASCADE")
	repo := NewChannelMonitorV2Repository(integrationDB).(*channelMonitorV2Repository)

	now := time.Now()
	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO ops_error_logs (
			request_id, platform, model, error_phase, error_type, severity, status_code,
			upstream_status_code, error_message, created_at
		) VALUES (
			'req-cm-detail', 'anthropic', 'claude-sonnet-4-6', 'upstream', 'upstream_error', 'error', 502,
			503, 'service temporarily unavailable', $1
		)`, now)
	require.NoError(t, err)

	cfg := service.ChannelMonitorV2Config{
		Platforms: []service.ChannelMonitorV2PlatformConfig{{Platform: "anthropic", Enabled: true}},
	}
	filter := service.ChannelMonitorV2Filter{Start: now.Add(-time.Hour), End: now.Add(time.Hour)}

	details, err := repo.loadErrorDetails(ctx, filter, cfg)
	require.NoError(t, err)
	var samples []service.ChannelMonitorV2ErrorDetail
	for _, list := range details {
		samples = append(samples, list...)
	}
	require.Len(t, samples, 1)
	require.Equal(t, "anthropic", samples[0].Platform)
	require.Equal(t, "claude-sonnet-4-6", samples[0].Model)
	require.Equal(t, 502, samples[0].StatusCode)
	require.Equal(t, 503, samples[0].UpstreamStatusCode)
	require.EqualValues(t, 1, samples[0].Count)
}
