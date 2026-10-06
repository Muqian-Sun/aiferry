package service

import "time"

// 上游忙闲时（muqian 2026-10-06：算成本按我们填的渠道价，上游有没有忙闲时在填承接时定；忙时整单乘倍数；
// 利润门按请求当时的价判断）。承接上的 TimePricing 只影响渠道成本与利润门，不影响向用户收的钱。

// saleTimeMultiplierAt 我们向用户收钱在 at 时刻整单乘的倍数：售价忙闲时（没单独定跟官方忙闲时，如 DeepSeek
// 工作日高峰 × 2，播种时从价格文件写进目录）。at 为零值 = 平时。
func saleTimeMultiplierAt(entry *ModelCatalogEntry, at time.Time) float64 {
	if entry == nil {
		return 1
	}
	return entry.SaleTimePricing().MultiplierAt(at)
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

// bindingPeakCostRatio 一周里最差的上游成本比（价格页的「忙时毛利」）：在上游忙闲时、条目分时的
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
	if sale := entry.SaleTimePricing(); sale != nil {
		schedules = append(schedules, sale)
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
