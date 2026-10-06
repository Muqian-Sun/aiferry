//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Codex alpha/search 记 1 次 web 搜索：走 token 路径（没有 token），搜索费按官方原价、不乘用户倍率。
func TestCalculateOpenAIRecordUsageCostAlphaSearch(t *testing.T) {
	t.Parallel()
	bs := NewBillingService()
	inputPrice := 1e-6
	svc := &OpenAIGatewayService{billingService: bs, resolver: newResolverWithSeededEntries(bs,
		ModelCatalogEntry{ModelID: "gpt-5.6-sol", Vendor: "openai", BillingMode: BillingModeToken, InputPrice: &inputPrice, OutputPrice: &inputPrice, ManagedBy: ModelCatalogManagedByAdmin},
		ModelCatalogEntry{ModelID: "gpt-6-astra", Vendor: "openai", BillingMode: BillingModeToken, InputPrice: &inputPrice, OutputPrice: &inputPrice, ManagedBy: ModelCatalogManagedByAdmin, SearchPricePerCall: testPtrFloat64(0.005)},
	)}
	apiKey := &APIKey{ID: 1}

	// 条目没填搜索价：只认目录，不收搜索费（厂商公开价由播种写进目录）
	result := &OpenAIForwardResult{Model: "gpt-5.6-sol", WebSearch: WebSearchUsage{WebSearchCalls: 1}}
	cost, tokenPath, err := svc.calculateOpenAIRecordUsageCost(context.Background(), result, apiKey, []string{"gpt-5.6-sol"}, 2.0, UsageTokens{}, time.Time{})
	require.NoError(t, err)
	require.True(t, tokenPath, "渠道成本按承接关系算（搜索部分按官方价）")
	require.Equal(t, 1, cost.WebSearchCount)
	require.Zero(t, cost.TotalCost)
	require.Zero(t, cost.ActualCost)

	// 条目填了搜索价 0.005：按它收
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
