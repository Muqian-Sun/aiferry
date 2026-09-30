package dto

import "github.com/Wei-Shaw/sub2api/internal/service"

// 管理站「渠道状态」页（/admin/channel-status）的响应：与用户站服务状态同一套指标（可用率、首字延迟、缓存命中率），
// 按渠道分行，另外给管理员看请求数与渠道身份（名字、平台、类型、状态）。

// AdminChannelStatusMetric 服务状态的统计格 + 请求数。
type AdminChannelStatusMetric struct {
	ServiceStatusMetric
	RequestCount int64 `json:"request_count"`
}

// AdminChannelStatusRow 一个渠道；account_id 0 = 没选到渠道就失败的请求（没有名字）。
type AdminChannelStatusRow struct {
	AccountID int64                    `json:"account_id"`
	Name      string                   `json:"name"`
	Platform  string                   `json:"platform"`
	Type      string                   `json:"type"`
	Status    string                   `json:"status"`
	Deleted   bool                     `json:"deleted"`
	Metrics   AdminChannelStatusMetric `json:"metrics"`
	Health    ServiceStatusHealth      `json:"health"`
	Buckets   []ServiceStatusPoint     `json:"buckets"`
}

type AdminChannelStatus struct {
	RefreshIntervalSeconds int                      `json:"refresh_interval_seconds"`
	Coverage               ServiceStatusCoverage    `json:"coverage"`
	Metrics                AdminChannelStatusMetric `json:"metrics"`
	Health                 ServiceStatusHealth      `json:"health"`
	Trend                  []ServiceStatusPoint     `json:"trend"`
	Items                  []AdminChannelStatusRow  `json:"items"`
	// Models 上架模型，给按模型筛选用
	Models []string `json:"models"`
}

func adminChannelStatusMetric(m service.ChannelMonitorV2Metric) AdminChannelStatusMetric {
	return AdminChannelStatusMetric{ServiceStatusMetric: serviceStatusMetric(m), RequestCount: m.RequestCount}
}

func AdminChannelStatusFromService(c *service.ChannelMonitorV2Channels) *AdminChannelStatus {
	if c == nil {
		return nil
	}
	items := make([]AdminChannelStatusRow, 0, len(c.Items))
	for _, row := range c.Items {
		items = append(items, AdminChannelStatusRow{
			AccountID: row.AccountID, Name: row.Name, Platform: row.Platform, Type: row.Type, Status: row.Status, Deleted: row.Deleted,
			Metrics: adminChannelStatusMetric(row.Metrics),
			Health:  serviceStatusHealth(row.Health),
			Buckets: serviceStatusPoints(row.Buckets),
		})
	}
	return &AdminChannelStatus{
		RefreshIntervalSeconds: service.ChannelMonitorV2RefreshIntervalSeconds,
		Coverage:               serviceStatusCoverage(c.Coverage),
		Metrics:                adminChannelStatusMetric(c.Metrics),
		Health:                 serviceStatusHealth(c.Health),
		Trend:                  serviceStatusPoints(c.Trend),
		Items:                  items,
		Models:                 c.Models,
	}
}
