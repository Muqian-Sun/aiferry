package service

import "time"

// 上游忙闲时（muqian 2026-10-06：算成本按我们填的渠道价，上游有没有忙闲时在填承接时定；忙时整单乘倍数；
// 利润门按请求当时的价判断）。承接上的 TimePricing 只影响渠道成本与利润门，不影响向用户收的钱。

// deepseekOfficialPeakTimePricing DeepSeek 官方忙闲时：北京时间工作日 09:00–12:00、14:00–18:00 × 2，
// 与向用户收钱的 deepseekPeakMultiplierAt 是同一条规则（测试锁住两者一致）。价格页给 DeepSeek 模型新加承接时
// 默认带上它作上游忙闲时；算忙时毛利时也用它找时段切点。
var deepseekOfficialPeakTimePricing = TimePricing{
	Timezone:     "Asia/Shanghai",
	WeekdaysOnly: true,
	Periods: []TimePricingPeriod{
		{StartTime: "09:00", EndTime: "12:00", Multiplier: 2},
		{StartTime: "14:00", EndTime: "18:00", Multiplier: 2},
	},
}

// DeepSeekOfficialPeakTimePricing 返回 DeepSeek 官方忙闲时的副本（给价格页当默认值 / 快捷选项）。
func DeepSeekOfficialPeakTimePricing() TimePricing {
	cp := deepseekOfficialPeakTimePricing
	cp.Periods = append([]TimePricingPeriod(nil), deepseekOfficialPeakTimePricing.Periods...)
	return cp
}

// deepseekPeakApplies 向用户收钱时这个条目套不套 DeepSeek 官方忙时翻倍（与计费同口径：平台默认价卡才套）。
func deepseekPeakApplies(entry *ModelCatalogEntry) bool {
	return entry != nil && !entry.IsOperatorAuthored() && isDeepSeekModel(entry.ModelID)
}

// saleTimeMultiplierAt 我们向用户收钱在 at 时刻整单乘的倍数：条目的分时 × DeepSeek 官方忙时。at 为零值 = 平时。
func saleTimeMultiplierAt(entry *ModelCatalogEntry, at time.Time) float64 {
	if entry == nil || at.IsZero() {
		return 1
	}
	m := entry.TimePricing.MultiplierAt(at)
	if deepseekPeakApplies(entry) {
		m *= deepseekPeakMultiplierAt(at)
	}
	return m
}

// bindingCostRatioAt at 时刻的上游成本比：平时的比值（bindingCostRatio）× 上游当时的倍数 ÷ 我们当时的倍数。
// at 为零值 = 平时（两边都不加）。
func bindingCostRatioAt(entry *ModelCatalogEntry, b *ModelCatalogBinding, at time.Time) (float64, bool) {
	ratio, ok := bindingCostRatio(entry, b)
	if !ok || at.IsZero() {
		return ratio, ok
	}
	sale := saleTimeMultiplierAt(entry, at)
	if sale <= 0 {
		return ratio, ok
	}
	return ratio * b.TimePricing.MultiplierAt(at) / sale, true
}

// bindingPeakCostRatio 一周里最差的上游成本比（价格页的「忙时毛利」）：在上游忙闲时、条目分时、DeepSeek 忙时的
// 每个时段切点上各算一遍取最大。比平时还高时 ok=true；两边都不分时、或忙时不比平时差时 ok=false。
func bindingPeakCostRatio(entry *ModelCatalogEntry, b *ModelCatalogBinding) (float64, bool) {
	base, ok := bindingCostRatio(entry, b)
	if !ok || b == nil {
		return 0, false
	}
	var schedules []*TimePricing
	if b.TimePricing != nil {
		schedules = append(schedules, b.TimePricing)
	}
	if entry.TimePricing != nil {
		schedules = append(schedules, entry.TimePricing)
	}
	if deepseekPeakApplies(entry) {
		schedules = append(schedules, &deepseekOfficialPeakTimePricing)
	}
	worst, worse := base, false
	for _, at := range weeklyBreakpoints(schedules) {
		if r, _ := bindingCostRatioAt(entry, b, at); r > worst*(1+1e-9) {
			worst, worse = r, true
		}
	}
	return worst, worse
}

// weeklyBreakpoints 各分时配置在一周里每个时段的起止与每天零点（换算成绝对时刻）。分时按周重复，
// 倍数在两个相邻切点之间不变，所以在这些时刻上取值就覆盖了一周里的所有情况（多取一天防时区跨日）。
func weeklyBreakpoints(schedules []*TimePricing) []time.Time {
	monday := time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC)
	var out []time.Time
	for _, tp := range schedules {
		if tp == nil || len(tp.Periods) == 0 {
			continue
		}
		location, err := loadChannelTimePricingLocation(tp.Timezone)
		if err != nil {
			continue
		}
		periods, err := parseChannelTimePeriods(tp.Periods)
		if err != nil {
			continue
		}
		for d := -1; d <= 8; d++ {
			day := monday.AddDate(0, 0, d).In(location)
			midnight := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, location)
			out = append(out, midnight)
			for _, p := range periods {
				out = append(out, midnight.Add(time.Duration(p.start)*time.Second), midnight.Add(time.Duration(p.end)*time.Second))
			}
		}
	}
	return out
}
