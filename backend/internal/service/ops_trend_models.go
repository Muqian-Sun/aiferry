package service

import "time"

type OpsThroughputTrendPoint struct {
	BucketStart   time.Time `json:"bucket_start"`
	RequestCount  int64     `json:"request_count"`
	TokenConsumed int64     `json:"token_consumed"`
	SwitchCount   int64     `json:"switch_count"`
	QPS           float64   `json:"qps"`
	TPS           float64   `json:"tps"`
	// 这个时间段成功请求的首字延迟分位数（毫秒），没有样本时为空（运维页「首字延迟」趋势，2026-10-04）。
	TTFTP50Ms *int `json:"ttft_p50_ms"`
	TTFTP99Ms *int `json:"ttft_p99_ms"`
}

type OpsThroughputPlatformBreakdownItem struct {
	Platform      string `json:"platform"`
	RequestCount  int64  `json:"request_count"`
	TokenConsumed int64  `json:"token_consumed"`
}

type OpsThroughputTrendResponse struct {
	Bucket string `json:"bucket"`

	Points []*OpsThroughputTrendPoint `json:"points"`

	// Optional drilldown helper: when no platform is selected, returns totals by platform.
	ByPlatform []*OpsThroughputPlatformBreakdownItem `json:"by_platform,omitempty"`
}

type OpsErrorTrendPoint struct {
	BucketStart time.Time `json:"bucket_start"`

	ErrorCountTotal      int64 `json:"error_count_total"`
	BusinessLimitedCount int64 `json:"business_limited_count"`
	ErrorCountSLA        int64 `json:"error_count_sla"`

	UpstreamErrorCountExcl429529 int64 `json:"upstream_error_count_excl_429_529"`
	Upstream429Count             int64 `json:"upstream_429_count"`
	Upstream529Count             int64 `json:"upstream_529_count"`

	// 运维页「失败 / 换渠道恢复」按环节叠（2026-10-04）：用户收到错误的里面，上游导致的、没选到渠道的；
	// 以及上游出错、换渠道后成功的（不算失败）。
	UpstreamFailedCount int64 `json:"upstream_failed_count"`
	RoutingFailedCount  int64 `json:"routing_failed_count"`
	RecoveredCount      int64 `json:"recovered_count"`
}

type OpsErrorTrendResponse struct {
	Bucket string                `json:"bucket"`
	Points []*OpsErrorTrendPoint `json:"points"`
}

type OpsErrorDistributionItem struct {
	StatusCode      int   `json:"status_code"`
	Total           int64 `json:"total"`
	SLA             int64 `json:"sla"`
	BusinessLimited int64 `json:"business_limited"`
}

type OpsErrorDistributionResponse struct {
	Total int64                       `json:"total"`
	Items []*OpsErrorDistributionItem `json:"items"`
}
