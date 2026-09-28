package dto

import (
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// 用户站「服务状态」页（/channel-monitor-v2/* 的普通用户路由）的响应。
//
// 只回答「各模型现在能不能用、快不快」，字段一律白名单：
// 不含平台 / 渠道、上游状态码与错误原文、请求量与 Token 量、RPM / TPM、缓存率、
// 健康分的阈值与权重、其他用户的任何信息。管理员路由仍返回 service 层的完整结构。

// ServiceStatusMetric 一个统计格：可用率与首字延迟。
type ServiceStatusMetric struct {
	// Availability = 1 − 计入错误率的失败占比（内容审核、余额不足这类客户端原因的失败不计）。
	// 该区间没有请求时为 null。
	Availability *float64 `json:"availability"`
	TTFTP50Ms    *int64   `json:"ttft_p50_ms"`
	TTFTP90Ms    *int64   `json:"ttft_p90_ms"`
}

// ServiceStatusHealth 状态档位：healthy / warning / critical / unknown（样本不足）。
type ServiceStatusHealth struct {
	Overall      string `json:"overall"`
	Availability string `json:"availability"`
	TTFT         string `json:"ttft"`
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

type ServiceStatusModel struct {
	Model   string              `json:"model"`
	Metrics ServiceStatusMetric `json:"metrics"`
	Health  ServiceStatusHealth `json:"health"`
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

type ServiceStatusDimensions struct {
	Models []string `json:"models"`
}

type ServiceStatusErrorCategory struct {
	Category string  `json:"category"`
	Rate     float64 `json:"rate"`
	Ignored  bool    `json:"ignored"`
}

// ServiceStatusSelf 当前用户自己的请求统计（排行里的其他用户一律不返回）。
type ServiceStatusSelf struct {
	Metrics ServiceStatusMetric `json:"metrics"`
}

func serviceStatusMetric(m service.ChannelMonitorV2Metric) ServiceStatusMetric {
	out := ServiceStatusMetric{TTFTP50Ms: m.TTFT.P50Ms, TTFTP90Ms: m.TTFT.P90Ms}
	// 普通用户拿到的指标已被 service 清掉请求数，只能从比率判断「有没有请求」：
	// 有请求时成功率与错误率至少一个大于 0（失败全是被忽略的客户端原因且没有成功时也按无数据处理）。
	if m.SuccessRate > 0 || m.ErrorRate > 0 {
		availability := 1 - m.ErrorRate
		out.Availability = &availability
	}
	return out
}

func serviceStatusHealth(h service.ChannelMonitorV2Health) ServiceStatusHealth {
	return ServiceStatusHealth{Overall: h.Overall, Availability: h.ErrorRate, TTFT: h.TTFT}
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
		RefreshIntervalSeconds: s.Config.RefreshIntervalSeconds,
		Coverage:               serviceStatusCoverage(s.Coverage),
		Metrics:                serviceStatusMetric(s.Metrics),
		Health:                 serviceStatusHealth(s.Health),
		Trend:                  serviceStatusPoints(s.Trend),
	}
}

func ServiceStatusModelsFromService(l *service.ChannelMonitorV2List[service.ChannelMonitorV2ModelRow]) *ServiceStatusList[ServiceStatusModel] {
	if l == nil {
		return nil
	}
	items := make([]ServiceStatusModel, 0, len(l.Items))
	for _, row := range l.Items {
		items = append(items, ServiceStatusModel{Model: row.Model, Metrics: serviceStatusMetric(row.Metrics), Health: serviceStatusHealth(row.Health)})
	}
	return &ServiceStatusList[ServiceStatusModel]{Coverage: serviceStatusCoverage(l.Coverage), Items: items}
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

// ServiceStatusDimensionsFromService 只留模型名。仓储里模型维度的值是「平台\x00模型」
// （同名模型在每个平台各一条），这里截掉平台前缀并按原顺序去重。
func ServiceStatusDimensionsFromService(d *service.ChannelMonitorV2Dimensions) *ServiceStatusDimensions {
	if d == nil {
		return nil
	}
	models := make([]string, 0, len(d.Models))
	seen := make(map[string]struct{}, len(d.Models))
	for _, dim := range d.Models {
		model := dim.Value
		if i := strings.IndexByte(model, 0); i >= 0 {
			model = model[i+1:]
		}
		if _, ok := seen[model]; ok || model == "" {
			continue
		}
		seen[model] = struct{}{}
		models = append(models, model)
	}
	return &ServiceStatusDimensions{Models: models}
}

func ServiceStatusErrorsFromService(l *service.ChannelMonitorV2List[service.ChannelMonitorV2ErrorRow]) *ServiceStatusList[ServiceStatusErrorCategory] {
	if l == nil {
		return nil
	}
	items := make([]ServiceStatusErrorCategory, 0, len(l.Items))
	for _, row := range l.Items {
		items = append(items, ServiceStatusErrorCategory{Category: row.Category, Rate: row.Rate, Ignored: row.Ignored})
	}
	return &ServiceStatusList[ServiceStatusErrorCategory]{Coverage: serviceStatusCoverage(l.Coverage), Items: items}
}

// ServiceStatusSelfFromService 只留当前用户自己那一行（service 已标 IsSelf），不带名次。
func ServiceStatusSelfFromService(l *service.ChannelMonitorV2List[service.ChannelMonitorV2UserRow]) *ServiceStatusList[ServiceStatusSelf] {
	if l == nil {
		return nil
	}
	items := make([]ServiceStatusSelf, 0, 1)
	for _, row := range l.Items {
		if row.IsSelf {
			items = append(items, ServiceStatusSelf{Metrics: serviceStatusMetric(row.Metrics)})
		}
	}
	return &ServiceStatusList[ServiceStatusSelf]{Coverage: serviceStatusCoverage(l.Coverage), Items: items}
}
