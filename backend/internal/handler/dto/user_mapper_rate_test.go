//go:build unit

package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 用户自己看得到的 User（/auth/me、个人资料）不带倍率：用户站只给售价与实付（2026-10-04 D1）；
// 管理站的 AdminUser 照旧带生效倍率与单独设的倍率。
func TestUserDTOHidesRateMultiplierAdminKeepsIt(t *testing.T) {
	custom := 0.1
	u := &service.User{ID: 1, Email: "a@example.com", RateMultiplier: &custom}

	userJSON, err := json.Marshal(UserFromService(u))
	require.NoError(t, err)
	require.NotContains(t, string(userJSON), "rate_multiplier")

	admin := UserFromServiceAdmin(u)
	require.InDelta(t, 0.1, admin.RateMultiplier, 1e-12)
	require.Equal(t, &custom, admin.CustomRateMultiplier)
	adminJSON, err := json.Marshal(admin)
	require.NoError(t, err)
	require.Contains(t, string(adminJSON), `"rate_multiplier":0.1`)
	require.Contains(t, string(adminJSON), `"custom_rate_multiplier":0.1`)

	// 没单独设的用户：管理站看到售价折扣 1（按售价收）、单独设的为 null（muqian 2026-10-06：倍率 = 在售价上再打折）
	def := UserFromServiceAdmin(&service.User{ID: 2})
	require.InDelta(t, 1.0, def.RateMultiplier, 1e-12)
	require.Nil(t, def.CustomRateMultiplier)
}

// 用户站用量行只给实付口径：分项费用折成实付（= 官方价口径 × token 实付 / token 官方价），
// 标准计费与用户倍率只在管理站的 AdminUsageLog 里；管理站的分项仍是官方价口径。
func TestUsageLogDTOBilledItemsForUsersOfficialForAdmin(t *testing.T) {
	// token 官方价 0.30、联网搜索 0.01（按原价收）、实付 0.03 → token 实付 0.02，系数 0.02 / 0.30
	l := &service.UsageLog{ID: 1, InputCost: 0.1, OutputCost: 0.2, TotalCost: 0.31, WebSearchCost: 0.01, ActualCost: 0.03, RateMultiplier: 1.0 / 15, ImageInputCost: 0.03}

	user := UsageLogFromService(l)
	require.InDelta(t, 0.1*0.02/0.30, user.InputCost, 1e-12)
	require.InDelta(t, 0.2*0.02/0.30, user.OutputCost, 1e-12)
	require.InDelta(t, 0.03*0.02/0.30, user.ImageInputCost, 1e-12)
	require.InDelta(t, 0.03, user.ActualCost, 1e-12)
	userJSON, err := json.Marshal(user)
	require.NoError(t, err)
	require.NotContains(t, string(userJSON), "total_cost")
	require.NotContains(t, string(userJSON), "rate_multiplier")

	adminJSON, err := json.Marshal(UsageLogFromServiceAdmin(l))
	require.NoError(t, err)
	for _, kept := range []string{`"input_cost":0.1`, `"output_cost":0.2`, `"total_cost":0.31`, `"actual_cost":0.03`, `"image_input_cost":0.03`} {
		require.Contains(t, string(adminJSON), kept)
	}
	require.Contains(t, string(adminJSON), `"rate_multiplier":0.0666`)

	// token 官方价为 0（只有联网搜索）：分项本来是 0，不除以 0
	only := UsageLogFromService(&service.UsageLog{TotalCost: 0.01, WebSearchCost: 0.01, ActualCost: 0.01})
	require.Zero(t, only.InputCost)
}
