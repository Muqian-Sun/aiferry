//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 上架提示的「能调度的渠道」：渠道在、启用、调度开着，且按新用户默认倍率过得了利润门（与调度器同一判断）。
func TestSchedulableBindings(t *testing.T) {
	in, out := 3e-6, 15e-6
	entry := func(bindings ...ModelCatalogBinding) *ModelCatalogEntry {
		return &ModelCatalogEntry{ID: 1, ModelID: "m", InputPrice: &in, OutputPrice: &out, Bindings: bindings}
	}
	// 上游价 = 官方价 × ratio
	binding := func(accountID int64, ratio float64) ModelCatalogBinding {
		return ModelCatalogBinding{EntryID: 1, AccountID: accountID, InputPrice: in * ratio, OutputPrice: out * ratio}
	}
	active := func(id int64) *Account { return &Account{ID: id, Status: StatusActive, Schedulable: true} }
	gateOn := ProfitControlSettings{MinMargin: 0.3} // 阈值 = 1/15 × 0.7 ≈ 0.0467
	noLoss := ProfitControlSettings{}               // 最低毛利率 0 = 不能亏本：阈值 = 1/15 ≈ 0.0667

	n, reason := SchedulableBindings(entry(), nil, gateOn)
	require.Equal(t, 0, n)
	require.Equal(t, UnschedulableNoBindings, reason)

	// 渠道停用、关了调度、已删除（不在 accounts 里）都不算
	disabled := &Account{ID: 2, Status: StatusDisabled, Schedulable: true}
	paused := &Account{ID: 3, Status: StatusActive, Schedulable: false}
	n, reason = SchedulableBindings(entry(binding(2, 0.03), binding(3, 0.03), binding(4, 0.03)), map[int64]*Account{2: disabled, 3: paused}, gateOn)
	require.Equal(t, 0, n)
	require.Equal(t, UnschedulableChannelsDisabled, reason)

	// 开着的渠道都超过阈值：利润门会跳过
	n, reason = SchedulableBindings(entry(binding(5, 0.10)), map[int64]*Account{5: active(5)}, gateOn)
	require.Equal(t, 0, n)
	require.Equal(t, UnschedulableProfitGate, reason)

	// 最低毛利率 0：不亏的派得到，亏本的照样跳过
	n, reason = SchedulableBindings(entry(binding(5, 0.05)), map[int64]*Account{5: active(5)}, noLoss)
	require.Equal(t, 1, n)
	require.Empty(t, reason)
	n, reason = SchedulableBindings(entry(binding(5, 0.10)), map[int64]*Account{5: active(5)}, noLoss)
	require.Equal(t, 0, n)
	require.Equal(t, UnschedulableProfitGate, reason)

	// 一个过门、一个被跳过：数 1 个
	n, reason = SchedulableBindings(entry(binding(5, 0.10), binding(6, 0.03)), map[int64]*Account{5: active(5), 6: active(6)}, gateOn)
	require.Equal(t, 1, n)
	require.Empty(t, reason)
}
