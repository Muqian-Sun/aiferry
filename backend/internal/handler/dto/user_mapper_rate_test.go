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

	// 跟全站默认的用户：管理站看到生效倍率 1/15、单独倍率为 null
	def := UserFromServiceAdmin(&service.User{ID: 2})
	require.InDelta(t, 1.0/15, def.RateMultiplier, 1e-12)
	require.Nil(t, def.CustomRateMultiplier)
}

// 用户站用量行只给实付：按官方价的分项费用、标准计费、用户倍率只在管理站的 AdminUsageLog 里。
func TestUsageLogDTOHidesOfficialCostsAdminKeepsThem(t *testing.T) {
	l := &service.UsageLog{ID: 1, InputCost: 0.1, OutputCost: 0.2, TotalCost: 0.3, ActualCost: 0.02, RateMultiplier: 1.0 / 15, ImageInputCost: 0.01}

	userJSON, err := json.Marshal(UsageLogFromService(l))
	require.NoError(t, err)
	require.Contains(t, string(userJSON), `"actual_cost":0.02`)
	for _, hidden := range []string{"input_cost", "output_cost", "total_cost", "rate_multiplier", "image_input_cost"} {
		require.NotContains(t, string(userJSON), hidden)
	}

	adminJSON, err := json.Marshal(UsageLogFromServiceAdmin(l))
	require.NoError(t, err)
	for _, kept := range []string{`"input_cost":0.1`, `"output_cost":0.2`, `"total_cost":0.3`, `"actual_cost":0.02`, `"image_input_cost":0.01`} {
		require.Contains(t, string(adminJSON), kept)
	}
	require.Contains(t, string(adminJSON), `"rate_multiplier":0.0666`)
}
