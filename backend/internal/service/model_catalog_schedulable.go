package service

// 上架提示（2026-10-04 D6，muqian 定：不拦上架，但上架时提示、模型列表标「没有能调度的渠道」）。

// 模型没有能调度的渠道时的原因（给管理站列表与上架确认用）。
const (
	UnschedulableNoBindings       = "no_bindings"       // 没有承接
	UnschedulableChannelsDisabled = "channels_disabled" // 承接的渠道都停用 / 关了调度 / 已删除
	UnschedulableProfitGate       = "profit_gate"       // 开着的渠道都会被利润门跳过
)

// SchedulableBindings 承接这个模型、能派到请求的渠道数；为 0 时给出原因（否则原因为空）。
//
// 渠道要在（accounts 里有）、启用、调度开着；限流 / 过载 / 临时停调这类会自己恢复的状态不算。
// 利润门开着时按新用户默认倍率判断，与调度器的否决点同一个判断（profitGateRejectsBinding）：
// 单独设了更高倍率的用户可能仍派得到，但默认用户派不到，上架给所有人看就是坏的。
func SchedulableBindings(entry *ModelCatalogEntry, accounts map[int64]*Account, profit ProfitControlSettings) (int, string) {
	if entry == nil || len(entry.Bindings) == 0 {
		return 0, UnschedulableNoBindings
	}
	threshold := clampProfitControlThreshold(DefaultSalePriceRatio * (1 - profit.MinMargin))
	enabled, schedulable := 0, 0
	for i := range entry.Bindings {
		b := &entry.Bindings[i]
		account := accounts[b.AccountID]
		if account == nil || account.Status != StatusActive || !account.Schedulable {
			continue
		}
		enabled++
		if profit.Enabled() {
			if rejected, _ := profitGateRejectsBinding(entry, b, threshold); rejected {
				continue
			}
		}
		schedulable++
	}
	switch {
	case schedulable > 0:
		return schedulable, ""
	case enabled == 0:
		return 0, UnschedulableChannelsDisabled
	default:
		return 0, UnschedulableProfitGate
	}
}
