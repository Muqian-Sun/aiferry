//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// profitControlTestService 造一个只带利润门全局设置（最低毛利率；0 = 关）的 OpenAI 网关服务。
func profitControlTestService(t *testing.T, minMargin float64) *OpenAIGatewayService {
	t.Helper()
	return &OpenAIGatewayService{settingService: profitControlTestSettingService(t, minMargin)}
}

// profitControlTestCtx 模拟认证后的请求上下文：D = 用户倍率。
func profitControlTestCtx(userRate float64) context.Context {
	return WithUserRateMultiplier(context.Background(), &User{ID: 1, RateMultiplier: officialRate(userRate)})
}

// upstreamCostTestAccount / upstreamCostTestOAuthAccount 只有 unit 标签的用例（本文件与
// openai_profit_control_pricing_test.go）用；放在无标签的 scheduler_test_stubs_test.go 里会被默认标签下的 lint 判成未使用。
func upstreamCostTestAccount(id int64) *Account {
	return &Account{
		ID:                id,
		Platform:          PlatformOpenAI,
		Type:              AccountTypeAPIKey,
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
	}
}

func upstreamCostTestOAuthAccount(id int64) *Account {
	return &Account{ID: id, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
}

// profitTestEntryID 利润门用例的目录条目：官方价输入 1、输出 2（$ / 百万 Token）。
const profitTestEntryID int64 = 9001

// profitTestPrices 登记「渠道 → 上游成本比」：withRoute 把它们挂成目录路由里这个条目的承接关系，
// 上游价 = 比值 × 官方价，所以利润门算出的上游成本比（bindingCostRatio）就是登记的比值。
// 路由在 withRoute 时定型，之后再登记的比值要重新 withRoute 才生效。
type profitTestPrices map[int64]float64

func (p profitTestPrices) withRoute(ctx context.Context) context.Context {
	in, out := 1e-6, 2e-6
	entry := &ModelCatalogEntry{ID: profitTestEntryID, ModelID: "profit-test-model", Status: ModelCatalogStatusListed, InputPrice: &in, OutputPrice: &out}
	for id, rate := range p {
		entry.Bindings = append(entry.Bindings, ModelCatalogBinding{EntryID: entry.ID, AccountID: id, InputPrice: rate * in, OutputPrice: rate * out})
	}
	return WithCatalogRoute(ctx, CatalogRoute{EntryID: entry.ID, CanonicalModel: entry.ModelID, RequestedModel: entry.ModelID, Entry: entry})
}

func TestResolveOpenAIProfitControlGate(t *testing.T) {
	t.Run("no setting service yields no gate", func(t *testing.T) {
		svc := &OpenAIGatewayService{}
		require.Nil(t, svc.resolveOpenAIProfitControlGate(profitControlTestCtx(1)))
	})

	t.Run("zero min margin yields no gate", func(t *testing.T) {
		svc := profitControlTestService(t, 0)
		require.Nil(t, svc.resolveOpenAIProfitControlGate(profitControlTestCtx(1)))
	})

	t.Run("threshold is the user rate times one minus min margin", func(t *testing.T) {
		svc := profitControlTestService(t, 0.3)
		gate := svc.resolveOpenAIProfitControlGate(profitControlTestCtx(2.0))
		require.NotNil(t, gate)
		require.InDelta(t, 2.0*(1-0.3), gate.threshold, 1e-12)
		require.False(t, gate.pricingAt.IsZero())
	})

	t.Run("no user identity prices at rate 1", func(t *testing.T) {
		svc := profitControlTestService(t, 0.5)
		gate := svc.resolveOpenAIProfitControlGate(context.Background())
		require.NotNil(t, gate)
		require.InDelta(t, 0.5, gate.threshold, 1e-12)
	})

	t.Run("settings change is visible after cache invalidation", func(t *testing.T) {
		svc := profitControlTestService(t, 0.5)
		require.InDelta(t, 0.5, svc.resolveOpenAIProfitControlGate(profitControlTestCtx(1)).threshold, 1e-12)
		require.NoError(t, svc.settingService.settingRepo.Set(context.Background(), SettingKeyProfitMinMargin, "0.1"))
		require.InDelta(t, 0.5, svc.resolveOpenAIProfitControlGate(profitControlTestCtx(1)).threshold, 1e-12, "60s 缓存内仍是旧值")
		InvalidateProfitControlSettingsCache()
		require.InDelta(t, 0.9, svc.resolveOpenAIProfitControlGate(profitControlTestCtx(1)).threshold, 1e-12)
	})
}

func TestOpenAIProfitControlVetoReason(t *testing.T) {
	now := time.Now()
	gateCtx := func(threshold float64, prices profitTestPrices) context.Context {
		return context.WithValue(prices.withRoute(context.Background()), openAIProfitControlGateCtxKey{}, &openAIProfitControlGate{
			threshold: threshold,
			pricingAt: now,
		})
	}

	t.Run("no gate admits everything", func(t *testing.T) {
		vetoed, reason := openAIProfitControlVetoReason(context.Background(), upstreamCostTestOAuthAccount(1))
		require.False(t, vetoed)
		require.Empty(t, reason)
	})

	t.Run("cost ratio below threshold admits", func(t *testing.T) {
		vetoed, _ := openAIProfitControlVetoReason(gateCtx(0.7, profitTestPrices{1: 0.5}), upstreamCostTestAccount(1))
		require.False(t, vetoed)
	})

	t.Run("cost ratio exactly at threshold admits via epsilon", func(t *testing.T) {
		vetoed, _ := openAIProfitControlVetoReason(gateCtx(0.7, profitTestPrices{1: 0.7}), upstreamCostTestAccount(1))
		require.False(t, vetoed)
	})

	t.Run("cost ratio within float noise above threshold admits", func(t *testing.T) {
		vetoed, _ := openAIProfitControlVetoReason(gateCtx(0.7, profitTestPrices{1: 0.7 + 1e-12}), upstreamCostTestAccount(1))
		require.False(t, vetoed)
	})

	t.Run("cost ratio above threshold is vetoed", func(t *testing.T) {
		vetoed, reason := openAIProfitControlVetoReason(gateCtx(0.7, profitTestPrices{1: 0.8}), upstreamCostTestAccount(1))
		require.True(t, vetoed)
		require.Equal(t, openAIProfitFilterReasonThreshold, reason)
	})

	t.Run("zero threshold only admits free upstream", func(t *testing.T) {
		prices := profitTestPrices{1: 0, 2: 0.01}
		vetoed, _ := openAIProfitControlVetoReason(gateCtx(0, prices), upstreamCostTestAccount(1))
		require.False(t, vetoed)
		vetoed, reason := openAIProfitControlVetoReason(gateCtx(0, prices), upstreamCostTestAccount(2))
		require.True(t, vetoed)
		require.Equal(t, openAIProfitFilterReasonThreshold, reason)
	})

	t.Run("channel without upstream price for the model is vetoed", func(t *testing.T) {
		vetoed, reason := openAIProfitControlVetoReason(gateCtx(0.7, profitTestPrices{2: 0.1}), upstreamCostTestAccount(1))
		require.True(t, vetoed)
		require.Equal(t, openAIProfitFilterReasonMissingUpstreamPrice, reason)
	})

	t.Run("oauth account is priced by its binding like a key", func(t *testing.T) {
		vetoed, _ := openAIProfitControlVetoReason(gateCtx(0.7, profitTestPrices{1: 0.2}), upstreamCostTestOAuthAccount(1))
		require.False(t, vetoed)
	})

	t.Run("request without catalog route is not gated", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), openAIProfitControlGateCtxKey{}, &openAIProfitControlGate{threshold: 0.7, pricingAt: now})
		vetoed, reason := openAIProfitControlVetoReason(ctx, upstreamCostTestAccount(1))
		require.False(t, vetoed)
		require.Empty(t, reason)
	})
}
