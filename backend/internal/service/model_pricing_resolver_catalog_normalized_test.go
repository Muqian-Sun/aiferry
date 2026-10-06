//go:build unit

package service

// issue #5256 回归测试（价卡已从渠道搬到模型目录，这些用例跟着搬）：
//
// 场景：管理员在目录里把 gpt-5.6-luna 的输入价从官方 $0.2/M 调成 $0.4/M。
// 当请求模型带 effort 后缀（gpt-5.6-luna-high）而目录只配了基名时，目录查找
// 用字面名未命中，官方兜底价却会把后缀名归一化到 gpt-5.6-luna 并命中静态价
// （pricing_service.go 的 gpt-5.6-luna 前缀分支）→ 落库的是官方 0.2 而不是目录 0.4。
//
// 测试走与生产一致的 OpenAIGatewayService.RecordUsage 路径，断言落库 UsageLog 的 InputCost。

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	// 1M 输入 token 下，目录价与官方兜底价的期望费用（USD）
	catalogPricingExpectedCatalogCost  = 0.4
	catalogPricingExpectedOfficialCost = 0.2
	// 用于验证「不相关的目录配置不会被误命中」的对照价
	catalogPricingUnrelatedCost = 0.9
)

// tokenPricingForModels 构造 token 计费模式的价卡；inputPerMillion 单位为 USD/1M token。
func tokenPricingForModels(models []string, inputPerMillion float64) PricingCard {
	return PricingCard{
		Models:          models,
		BillingMode:     BillingModeToken,
		InputPrice:      float64Ptr(inputPerMillion / 1e6),
		OutputPrice:     float64Ptr(2.4e-6),
		CacheWritePrice: float64Ptr(0.5e-6),
		CacheReadPrice:  float64Ptr(0.04e-6),
	}
}

// recordUsageWithCatalogPricing 用给定的目录价卡跑一次 RecordUsage，返回落库的 UsageLog。
func recordUsageWithCatalogPricing(t *testing.T, requestedModel string, _ bool, pricings []PricingCard) *UsageLog {
	t.Helper()

	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newOpenAIRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
	svc.resolver = newResolverWithCatalogCards(svc.billingService, pricings...)

	err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:    "resp_luna_5256",
			Model:        requestedModel,
			BillingModel: requestedModel,
			Usage: OpenAIUsage{
				InputTokens:  1_000_000,
				OutputTokens: 0,
			},
			Duration: time.Second,
		},
		RequestedModel: requestedModel,
		APIKey:         &APIKey{ID: 1},
		User:           &User{ID: 1, RateMultiplier: customRate(1)},
		Account:        &Account{ID: 1, Platform: PlatformOpenAI},
	})
	require.NoError(t, err)
	require.NotNil(t, usageRepo.lastLog)
	return usageRepo.lastLog
}

// 基线：请求模型与目录定价 key 完全一致 → 按目录价计。
func TestCatalogPricing_ExactModelMatch(t *testing.T) {
	log := recordUsageWithCatalogPricing(t, "gpt-5.6-luna", false, []PricingCard{
		tokenPricingForModels([]string{"gpt-5.6-luna"}, catalogPricingExpectedCatalogCost),
	})
	require.InDelta(t, catalogPricingExpectedCatalogCost, log.InputCost, 1e-9)
}

// issue #5256 主回归：请求模型带 effort 后缀、目录只配基名（无通配符）→ 仍应按目录价计。
// 修复前此处得到 0.2（官方兜底价）。
func TestCatalogPricing_SuffixedModelUsesNormalizedCatalogPricing(t *testing.T) {
	log := recordUsageWithCatalogPricing(t, "gpt-5.6-luna-high", false, []PricingCard{
		tokenPricingForModels([]string{"gpt-5.6-luna"}, catalogPricingExpectedCatalogCost),
	})
	require.InDelta(t, catalogPricingExpectedCatalogCost, log.InputCost, 1e-9,
		"suffixed request model should fall back to the normalized catalog pricing; got %v (%v = official fallback)",
		log.InputCost, catalogPricingExpectedOfficialCost)
}

// 同一根因的另一种变体名：上游返回带日期后缀的模型名
// （isCodexDateSuffix，如 gpt-5.6-luna-2026-08-01），目录只配基名 → 仍应按目录价计。
func TestCatalogPricing_DateSuffixedModelUsesNormalizedCatalogPricing(t *testing.T) {
	log := recordUsageWithCatalogPricing(t, "gpt-5.6-luna-2026-08-01", false, []PricingCard{
		tokenPricingForModels([]string{"gpt-5.6-luna"}, catalogPricingExpectedCatalogCost),
	})
	require.InDelta(t, catalogPricingExpectedCatalogCost, log.InputCost, 1e-9,
		"date-suffixed request model should fall back to the normalized catalog pricing; got %v", log.InputCost)
}

// 精确匹配优先：同时配了变体名与基名时，请求变体名必须命中变体的显式配价，
// 不能被归一化后的基名覆盖。
func TestCatalogPricing_ExactVariantWinsOverNormalizedBaseName(t *testing.T) {
	log := recordUsageWithCatalogPricing(t, "gpt-5.6-luna-high", false, []PricingCard{
		tokenPricingForModels([]string{"gpt-5.6-luna-high"}, catalogPricingUnrelatedCost),
		tokenPricingForModels([]string{"gpt-5.6-luna"}, catalogPricingExpectedCatalogCost),
	})
	require.InDelta(t, catalogPricingUnrelatedCost, log.InputCost, 1e-9,
		"explicit per-variant catalog pricing must win over the normalized base name")
}

// 订阅型分组走同一条目录定价解析路径。
func TestCatalogPricing_SuffixedModelSubscriptionGroup(t *testing.T) {
	log := recordUsageWithCatalogPricing(t, "gpt-5.6-luna-high", true, []PricingCard{
		tokenPricingForModels([]string{"gpt-5.6-luna"}, catalogPricingExpectedCatalogCost),
	})
	require.InDelta(t, catalogPricingExpectedCatalogCost, log.InputCost, 1e-9)
}

// 反向保护：目录只配了不相关的模型时，归一化查找不得误命中该配置；
// 计费只认目录，查不到就不计价（不再落回内置价表）。
func TestCatalogPricing_UnrelatedCatalogModelNotMatched(t *testing.T) {
	log := recordUsageWithCatalogPricing(t, "gpt-5.6-luna-high", false, []PricingCard{
		tokenPricingForModels([]string{"gpt-5.4"}, catalogPricingUnrelatedCost),
	})
	require.Zero(t, log.InputCost, "normalized lookup must not match an unrelated catalog entry")
}
