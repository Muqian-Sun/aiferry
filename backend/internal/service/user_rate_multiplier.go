package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

// UserRateMultiplier 本次请求生效的计费倍率：用户价 = 目录价 × 它。负数按 0（免费）。
func UserRateMultiplier(user *User) float64 {
	if user.RateMultiplier < 0 {
		return 0
	}
	return user.RateMultiplier
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
