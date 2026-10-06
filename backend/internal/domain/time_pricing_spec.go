package domain

// TimePricingSpec 是分时倍率的存储形状（与 service.TimePricing 同字段）：按 Timezone 的本地时间，
// 落在某个时段 [StartTime, EndTime) 里时整单 × Multiplier；WeekdaysOnly 为 true 时周末不加。
// 承接关系上的上游忙闲时（model_catalog_bindings.time_pricing）存成这个形状的 JSONB。
type TimePricingSpec struct {
	Timezone     string                  `json:"timezone"`
	WeekdaysOnly bool                    `json:"weekdays_only,omitempty"`
	Periods      []TimePricingSpecPeriod `json:"periods"`
}

// TimePricingSpecPeriod 一个时段：HH:mm 或 HH:mm:ss，左闭右开，结束 00:00 表示到当天结束。
type TimePricingSpecPeriod struct {
	StartTime  string  `json:"start_time"`
	EndTime    string  `json:"end_time"`
	Multiplier float64 `json:"multiplier"`
}
