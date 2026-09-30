package dto

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// 用户站「服务状态」页（/channel-monitor-v2/snapshot 与 /matrix，对未登录访客公开）的响应。
//
// 只回答「各模型现在能不能用、快不快、缓存命中多少」，字段一律白名单：
// 不含平台 / 渠道、上游状态码与错误原文、请求量与 Token 量、RPM / TPM、
// 健康分的阈值与权重、任何用户的信息。

// ServiceStatusMetric 一个统计格：可用率、首字延迟与缓存命中率。
type ServiceStatusMetric struct {
	// Availability = 1 − 计入错误率的失败占比（内容审核、余额不足这类客户端原因的失败不计）。
	// 该区间没有请求时为 null。
	Availability *float64 `json:"availability"`
	TTFTP50Ms    *int64   `json:"ttft_p50_ms"`
	TTFTP90Ms    *int64   `json:"ttft_p90_ms"`
	// CacheHitRate 输入里命中缓存的 token 占比（缓存读 ÷（输入 + 缓存写 + 缓存读））；没有输入时为 null。
	CacheHitRate *float64 `json:"cache_hit_rate"`
}

// ServiceStatusHealth 状态档位：healthy / warning / critical / unknown（样本不足）。
type ServiceStatusHealth struct {
	Overall      string `json:"overall"`
	Availability string `json:"availability"`
	TTFT         string `json:"ttft"`
	Cache        string `json:"cache"`
}

// ServiceStatusCoverage 统计窗口与数据新鲜度。
type ServiceStatusCoverage struct {
	RequestedStart   time.Time `json:"requested_start"`
	RequestedEnd     time.Time `json:"requested_end"`
	DataThrough      time.Time `json:"data_through"`
	BucketSeconds    int       `json:"bucket_seconds"`
	CoverageComplete bool      `json:"coverage_complete"`
	// BackfillPercent 首次启用后后台补历史数据的进度；补齐后为 null。
	BackfillPercent *int `json:"backfill_percent"`
}

type ServiceStatusPoint struct {
	BucketStart time.Time           `json:"bucket_start"`
	Metrics     ServiceStatusMetric `json:"metrics"`
	Health      ServiceStatusHealth `json:"health"`
}

type ServiceStatusSnapshot struct {
	RefreshIntervalSeconds int                   `json:"refresh_interval_seconds"`
	Coverage               ServiceStatusCoverage `json:"coverage"`
	Metrics                ServiceStatusMetric   `json:"metrics"`
	Health                 ServiceStatusHealth   `json:"health"`
	Trend                  []ServiceStatusPoint  `json:"trend"`
}

type ServiceStatusModelTrend struct {
	Model   string               `json:"model"`
	Metrics ServiceStatusMetric  `json:"metrics"`
	Health  ServiceStatusHealth  `json:"health"`
	Buckets []ServiceStatusPoint `json:"buckets"`
}

type ServiceStatusList[T any] struct {
	Coverage ServiceStatusCoverage `json:"coverage"`
	Items    []T                   `json:"items"`
}

func serviceStatusMetric(m service.ChannelMonitorV2Metric) ServiceStatusMetric {
	out := ServiceStatusMetric{TTFTP50Ms: m.TTFT.P50Ms, TTFTP90Ms: m.TTFT.P90Ms}
	// 按比率判断「有没有请求」：有请求时成功率与错误率至少一个大于 0
	// （失败全是被忽略的客户端原因且没有成功时按无数据处理）。
	if m.SuccessRate > 0 || m.ErrorRate > 0 {
		availability := 1 - m.ErrorRate
		out.Availability = &availability
	}
	if m.CacheRateDenominator > 0 {
		rate := m.CacheRate
		out.CacheHitRate = &rate
	}
	return out
}

func serviceStatusHealth(h service.ChannelMonitorV2Health) ServiceStatusHealth {
	return ServiceStatusHealth{Overall: h.Overall, Availability: h.ErrorRate, TTFT: h.TTFT, Cache: h.Cache}
}

func serviceStatusCoverage(c service.ChannelMonitorV2Coverage) ServiceStatusCoverage {
	out := ServiceStatusCoverage{
		RequestedStart:   c.RequestedStart,
		RequestedEnd:     c.RequestedEnd,
		DataThrough:      c.DataThrough,
		BucketSeconds:    c.BucketSeconds,
		CoverageComplete: c.CoverageComplete,
	}
	if c.Bootstrap != nil && c.Bootstrap.Active {
		percent := c.Bootstrap.ProgressPercent
		out.BackfillPercent = &percent
	}
	return out
}

func serviceStatusPoints(points []service.ChannelMonitorV2TrendPoint) []ServiceStatusPoint {
	out := make([]ServiceStatusPoint, 0, len(points))
	for _, p := range points {
		out = append(out, ServiceStatusPoint{BucketStart: p.BucketStart, Metrics: serviceStatusMetric(p.Metrics), Health: serviceStatusHealth(p.Health)})
	}
	return out
}

func ServiceStatusSnapshotFromService(s *service.ChannelMonitorV2Snapshot) *ServiceStatusSnapshot {
	if s == nil {
		return nil
	}
	return &ServiceStatusSnapshot{
		RefreshIntervalSeconds: service.ChannelMonitorV2RefreshIntervalSeconds,
		Coverage:               serviceStatusCoverage(s.Coverage),
		Metrics:                serviceStatusMetric(s.Metrics),
		Health:                 serviceStatusHealth(s.Health),
		Trend:                  serviceStatusPoints(s.Trend),
	}
}

func ServiceStatusMatrixFromService(m *service.ChannelMonitorV2Matrix) *ServiceStatusList[ServiceStatusModelTrend] {
	if m == nil {
		return nil
	}
	items := make([]ServiceStatusModelTrend, 0, len(m.Items))
	for _, row := range m.Items {
		items = append(items, ServiceStatusModelTrend{
			Model:   row.Model,
			Metrics: serviceStatusMetric(row.Metrics),
			Health:  serviceStatusHealth(row.Health),
			Buckets: serviceStatusPoints(row.Buckets),
		})
	}
	return &ServiceStatusList[ServiceStatusModelTrend]{Coverage: serviceStatusCoverage(m.Coverage), Items: items}
}
