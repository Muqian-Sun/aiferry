//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// billableModelWithFallback 是通用安全网：选定计费模型查不到任何价格时回退到
// 实际转发的具体模型；已定价流量（含家族兜底可解析的名字）不受影响。
func TestBillableModelWithFallback(t *testing.T) {
	bs := NewBillingService()
	svc := &GatewayService{billingService: bs, resolver: builtinSeededResolver(bs)}
	apiKey := &APIKey{}
	ctx := context.Background()

	// 完全无价的别名 → 回退到具体转发模型（目录里有 claude-sonnet-4）
	require.Equal(t, "claude-sonnet-4",
		svc.billableModelWithFallback(ctx, apiKey, "team/best", "", "claude-sonnet-4"))

	// 已定价模型不回退，候选被忽略
	require.Equal(t, "claude-sonnet-4",
		svc.billableModelWithFallback(ctx, apiKey, "claude-sonnet-4", "claude-opus-4"))

	// 所有候选都无价 → 保持原值，走既有 warn + 零成本路径
	require.Equal(t, "team/best",
		svc.billableModelWithFallback(ctx, apiKey, "team/best", "another/alias", ""))

	// 空计费模型 + 有价候选 → 取候选
	require.Equal(t, "claude-sonnet-4",
		svc.billableModelWithFallback(ctx, apiKey, "", "claude-sonnet-4"))
}

func TestHasResolvableTokenPricing(t *testing.T) {
	bs := NewBillingService()
	svc := &GatewayService{billingService: bs, resolver: builtinSeededResolver(bs)}
	apiKey := &APIKey{}
	ctx := context.Background()

	require.True(t, svc.hasResolvableTokenPricing(ctx, "claude-sonnet-4", apiKey))
	// 只认目录：含家族词的名字（all/claude）不再被价格表家族兜底解析成「有价」
	require.False(t, svc.hasResolvableTokenPricing(ctx, "all/claude", apiKey))
	require.False(t, svc.hasResolvableTokenPricing(ctx, "team/best", apiKey))
	require.False(t, svc.hasResolvableTokenPricing(ctx, "", apiKey))

	// 没有解析器时 fail-closed（不误判有价）
	empty := &GatewayService{billingService: bs}
	require.False(t, empty.hasResolvableTokenPricing(ctx, "claude-sonnet-4", apiKey))
}
