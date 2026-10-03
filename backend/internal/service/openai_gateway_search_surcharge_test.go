//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 搜索费叠加在 token 费上：官方原价、不乘用户倍率。
func TestCalculateOpenAIRecordUsageCost_SearchIsAdditiveToTokens(t *testing.T) {
	t.Parallel()

	svc := &OpenAIGatewayService{
		billingService: newTestBillingService(),
	}
	apiKey := &APIKey{}

	// claude-sonnet-4 fallback: Input $3/MTok, Output $15/MTok
	// 1000 in + 500 out → 0.003 + 0.0075 = 0.0105，× 倍率 2 = 0.021
	// + 100 次搜索 × 内置 $0.01（不乘倍率）= 1.0
	cost, _, err := svc.calculateOpenAIRecordUsageCost(
		context.Background(),
		&OpenAIForwardResult{WebSearch: WebSearchUsage{WebSearchCalls: 100}},
		apiKey,
		[]string{"claude-sonnet-4"},
		2.0,
		UsageTokens{InputTokens: 1000, OutputTokens: 500},
		time.Time{},
	)
	require.NoError(t, err)
	require.NotNil(t, cost)
	require.Equal(t, 100, cost.WebSearchCount)
	require.InDelta(t, 1.0, cost.WebSearchCost, 1e-9)
	require.InDelta(t, 1.021, cost.ActualCost, 1e-9)
	require.InDelta(t, 1.0105, cost.TotalCost, 1e-9)
}

func TestCalculateOpenAIRecordUsageCost_TokenPricingErrorNotSwallowedBySearch(t *testing.T) {
	t.Parallel()

	svc := &OpenAIGatewayService{
		billingService: newTestBillingService(),
	}
	apiKey := &APIKey{}
	// Unknown model → token pricing fails; search must not replace that with $0/$search bill.
	cost, _, err := svc.calculateOpenAIRecordUsageCost(
		context.Background(),
		&OpenAIForwardResult{WebSearch: WebSearchUsage{WebSearchCalls: 100}},
		apiKey,
		[]string{"totally-unknown-model-xyz-no-pricing"},
		1.0,
		UsageTokens{InputTokens: 1000, OutputTokens: 500},
		time.Time{},
	)
	require.Error(t, err)
	require.Nil(t, cost)
}
