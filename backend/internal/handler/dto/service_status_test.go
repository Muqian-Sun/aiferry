package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 可用率按「计入错误率的失败」算：被忽略的客户端原因失败不拉低可用率。
func TestServiceStatusAvailabilityUsesScoredErrorRate(t *testing.T) {
	// 100 个请求：90 成功、10 失败，其中 7 个是被忽略的客户端原因 → 计入的错误率 3%
	got := serviceStatusMetric(service.ChannelMonitorV2Metric{SuccessRate: 0.9, ErrorRate: 0.03})
	require.NotNil(t, got.Availability)
	require.InDelta(t, 0.97, *got.Availability, 1e-9)
}

// 区间没有请求（两个比率都是 0）时可用率是 null，不能显示成 100%。
func TestServiceStatusAvailabilityIsNullWithoutRequests(t *testing.T) {
	got := serviceStatusMetric(service.ChannelMonitorV2Metric{})
	require.Nil(t, got.Availability)

	// 全部失败且都计入错误率：可用率 0，不是 null
	allFailed := serviceStatusMetric(service.ChannelMonitorV2Metric{ErrorRate: 1})
	require.NotNil(t, allFailed.Availability)
	require.InDelta(t, 0, *allFailed.Availability, 1e-9)
}

// 模型维度的值带平台前缀、同名模型每个平台一条：用户侧只留去重后的模型名。
func TestServiceStatusDimensionsStripPlatformPrefix(t *testing.T) {
	got := ServiceStatusDimensionsFromService(&service.ChannelMonitorV2Dimensions{
		Platforms: []service.ChannelMonitorV2Dimension{{Value: "openai"}},
		Models: []service.ChannelMonitorV2Dimension{
			{Value: "openai\x00gpt-5"},
			{Value: "azure\x00gpt-5"},
			{Value: "openai\x00__other__"},
			{Value: "claude-sonnet-4"},
		},
	})
	require.Equal(t, []string{"gpt-5", "__other__", "claude-sonnet-4"}, got.Models)
}

// 首次补历史数据时给进度，补齐后为 null。
func TestServiceStatusCoverageBackfillPercent(t *testing.T) {
	active := serviceStatusCoverage(service.ChannelMonitorV2Coverage{Bootstrap: &service.ChannelMonitorV2Bootstrap{Active: true, ProgressPercent: 42}})
	require.NotNil(t, active.BackfillPercent)
	require.Equal(t, 42, *active.BackfillPercent)
	require.Nil(t, serviceStatusCoverage(service.ChannelMonitorV2Coverage{}).BackfillPercent)
}
