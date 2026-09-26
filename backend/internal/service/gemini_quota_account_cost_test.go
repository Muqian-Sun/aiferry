package service

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

// Gemini 本地配额的「用量进度」显示的是渠道自己的花费（渠道成本），不是向用户收的钱。
// 原来靠按渠道统计时 actual_cost 被偷换成渠道成本；去掉偷换后显式读 account_cost。
func TestGeminiAggregateUsageUsesAccountCost(t *testing.T) {
	totals := geminiAggregateUsage([]usagestats.ModelStat{
		{Model: "gemini-2.5-pro", Requests: 2, TotalTokens: 10, ActualCost: 1.0, AccountCost: 0.4},
		{Model: "gemini-2.5-flash", Requests: 1, TotalTokens: 5, ActualCost: 0.5, AccountCost: 0.2},
	})
	require.Equal(t, int64(2), totals.ProRequests)
	require.Equal(t, int64(1), totals.FlashRequests)
	require.InDelta(t, 0.4, totals.ProCost, 1e-9)
	require.InDelta(t, 0.2, totals.FlashCost, 1e-9)
}
