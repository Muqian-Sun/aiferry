//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// Codex alpha/search 记 1 次 web 搜索：走 token 路径（没有 token），搜索费按官方原价、不乘用户倍率。
func TestCalculateOpenAIRecordUsageCostAlphaSearch(t *testing.T) {
	t.Parallel()
	bs := NewBillingService(&config.Config{}, nil)
	inputPrice := 1e-6
	svc := &OpenAIGatewayService{billingService: bs, resolver: newResolverWithSeededEntries(bs,
		ModelCatalogEntry{ModelID: "gpt-5.6-sol", Vendor: "openai", BillingMode: BillingModeToken, InputPrice: &inputPrice, OutputPrice: &inputPrice, ManagedBy: ModelCatalogManagedByAdmin},
		ModelCatalogEntry{ModelID: "gpt-6-astra", Vendor: "openai", BillingMode: BillingModeToken, InputPrice: &inputPrice, OutputPrice: &inputPrice, ManagedBy: ModelCatalogManagedByAdmin, SearchPricePerCall: testPtrFloat64(0.005)},
	)}
	apiKey := &APIKey{ID: 1}

	// 条目没填搜索价：按 OpenAI 公开价 0.01
	result := &OpenAIForwardResult{Model: "gpt-5.6-sol", WebSearch: WebSearchUsage{WebSearchCalls: 1}}
	cost, tokenPath, err := svc.calculateOpenAIRecordUsageCost(context.Background(), result, apiKey, []string{"gpt-5.6-sol"}, 2.0, UsageTokens{}, time.Time{})
	require.NoError(t, err)
	require.True(t, tokenPath, "渠道成本按承接关系算（搜索部分按官方价）")
	require.Equal(t, 1, cost.WebSearchCount)
	require.InDelta(t, 0.01, cost.TotalCost, 1e-12)
	require.InDelta(t, 0.01, cost.ActualCost, 1e-12)

	// 条目填了搜索价 0.005：覆盖公开价
	result = &OpenAIForwardResult{Model: "gpt-6-astra", WebSearch: WebSearchUsage{WebSearchCalls: 1}}
	cost, _, err = svc.calculateOpenAIRecordUsageCost(context.Background(), result, apiKey, []string{"gpt-6-astra"}, 2.0, UsageTokens{}, time.Time{})
	require.NoError(t, err)
	require.InDelta(t, 0.005, cost.TotalCost, 1e-12)
	require.InDelta(t, 0.005, cost.ActualCost, 1e-12)

	// 没有搜索：只有 token 费
	result = &OpenAIForwardResult{Model: "gpt-6-astra"}
	cost, _, err = svc.calculateOpenAIRecordUsageCost(context.Background(), result, apiKey, []string{"gpt-6-astra"}, 1.0, UsageTokens{InputTokens: 10}, time.Time{})
	require.NoError(t, err)
	require.Zero(t, cost.WebSearchCount)
	require.InDelta(t, 10*inputPrice, cost.TotalCost, 1e-12)
}
