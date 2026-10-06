package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

// UserSaleDiscount 这个用户在售价上的折扣（管理员在用户管理里单独设的倍率，muqian 2026-10-06：在售价上再打折）：
// 1 = 按售价收；没单独设（nil）= 1；负数按 0（免费）。
func UserSaleDiscount(user *User) float64 {
	if user.RateMultiplier == nil {
		return 1
	}
	if *user.RateMultiplier < 0 {
		return 0
	}
	return *user.RateMultiplier
}

// UserRateMultiplier 计费用的「官方价口径」倍率 = 售价折扣 × 默认售价比例：没单独填售价的项按
// 「官方价 × 它」收；填了售价的项在计费里先换算成官方口径（售价 ÷ 默认售价比例），乘它正好是「售价 × 折扣」。
func UserRateMultiplier(user *User) float64 {
	return UserSaleDiscount(user) * DefaultSalePriceRatio
}

// WithUserRateMultiplier 把认证后用户的计费倍率放进 request.Context（认证中间件调用）。
func WithUserRateMultiplier(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, ctxkey.UserRateMultiplier, UserRateMultiplier(user))
}

// UserRateMultiplierFromContext 读认证中间件放进来的用户倍率；没有（内部调用）时按 1。
func UserRateMultiplierFromContext(ctx context.Context) float64 {
	if v, ok := ctx.Value(ctxkey.UserRateMultiplier).(float64); ok {
		return v
	}
	return 1
}
