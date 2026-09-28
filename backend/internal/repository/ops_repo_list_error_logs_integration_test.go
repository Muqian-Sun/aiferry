//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 管理端错误请求列表（用量页「错误」页签、运维错误列表）走这条查询：SELECT 列与 Scan 目标必须一一对应，
// 去分组时 SELECT 删了分组列而 Scan 漏删，列表整页 500 却被前端当成「暂无数据」。
func TestListErrorLogs_ReturnsInsertedRow(t *testing.T) {
	ctx := context.Background()
	_, _ = integrationDB.ExecContext(ctx, "TRUNCATE ops_error_logs RESTART IDENTITY CASCADE")
	repo := NewOpsRepository(integrationDB).(*opsRepository)

	_, err := repo.InsertErrorLog(ctx, &service.OpsInsertErrorLogInput{
		RequestID:  "req-list-error-logs",
		ErrorPhase: "upstream",
		ErrorType:  "upstream_error",
		Severity:   "error",
		StatusCode: 502,
		Model:      "claude-sonnet-4-6",
		CreatedAt:  time.Now(),
	})
	require.NoError(t, err)

	list, err := repo.ListErrorLogs(ctx, &service.OpsErrorLogFilter{View: "all"})
	require.NoError(t, err)
	require.Equal(t, 1, list.Total)
	require.Len(t, list.Errors, 1)
	got := list.Errors[0]
	require.Equal(t, "req-list-error-logs", got.RequestID)
	require.Equal(t, "upstream", got.Phase)
	require.Equal(t, 502, got.StatusCode)
	require.Equal(t, "claude-sonnet-4-6", got.Model)
}
