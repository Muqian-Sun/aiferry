//go:build unit

package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIsDeepSeekModel(t *testing.T) {
	deepseek := []string{
		"deepseek-flash", "deepseek-v4-flash", "deepseek-v4-pro", "deepseek-v4-flash-vision-exp",
		"deepseek-chat", "deepseek-reasoner", "deepseek-v3-2-251201",
		"deepseek-coder", "deepseek-foo", "deepseek-v4-pro-0813",
		"DEEPSEEK-V4-PRO", " deepseek-v4-flash ",
	}
	for _, m := range deepseek {
		require.True(t, isDeepSeekModel(m), "model %q should be deepseek", m)
	}

	nonDeepseek := []string{
		"gpt-5.4", "claude-sonnet-4", "deepseekcoder", // 无连字符不算 deepseek- 前缀
		"", " deepseek", // 无连字符后缀
	}
	for _, m := range nonDeepseek {
		require.False(t, isDeepSeekModel(m), "model %q should not be deepseek", m)
	}
}

// 价格文件里的 DeepSeek 官方价（计费只认目录，目录按价格文件播种）：低谷价；deepseek-v4-pro 自 2026-09-14
// 起被上游路由到 V4.1-Flash 并按 Flash 价收，目录存现价（V4.1 Pro 上线后改回）。
func TestDeepseekPricingFileMatchesOfficialRates(t *testing.T) {
	pricingData := loadBuiltinPricingFile(t)

	_, ok := pricingData["deepseek-v3-2-251201"]
	require.False(t, ok, "deepseek-v3-2-251201（$0 占位条目）必须从价格表中移除")
	for _, discontinued := range []string{"deepseek-chat", "deepseek-reasoner"} {
		_, ok := pricingData[discontinued]
		require.False(t, ok, "%s 已停止服务，必须从价格表中移除", discontinued)
	}

	for _, model := range []string{"deepseek-flash", "deepseek-v4-flash", "deepseek-v4-flash-vision-exp", "deepseek-v4-pro"} {
		t.Run(model, func(t *testing.T) {
			entry, ok := pricingData[model]
			require.True(t, ok, "model %s must exist in pricing file", model)
			require.InDelta(t, 1.5e-7, entry.InputCostPerToken, 1e-15)
			require.InDelta(t, 6e-7, entry.OutputCostPerToken, 1e-15)
			require.InDelta(t, 3e-9, entry.CacheReadInputTokenCost, 1e-15)
		})
	}
}

// DeepSeek 官方高峰（2026-08-23 起）：UTC 01:00–04:00 与 06:00–10:00（半开区间），只在北京时间工作日；
// 高峰价 = 2 × 低谷价。价格文件的 time_pricing 播进目录后逐分钟都要与这条官方规则一致。
func TestDeepseekPricingFileTimePricingMatchesOfficialPeak(t *testing.T) {
	pricingData := loadBuiltinPricingFile(t)
	officialPeak := func(at time.Time) float64 {
		switch at.In(time.FixedZone("Asia/Shanghai", 8*3600)).Weekday() {
		case time.Saturday, time.Sunday:
			return 1
		}
		if h := at.UTC().Hour(); (h >= 1 && h < 4) || (h >= 6 && h < 10) {
			return 2
		}
		return 1
	}
	start := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC) // 周日 UTC，跨上一周末到下一周一
	for _, model := range []string{"deepseek-flash", "deepseek-v4-flash", "deepseek-v4-flash-vision-exp", "deepseek-v4-pro"} {
		entry := seedEntryFromLiteLLM(model, pricingData[model])
		require.NotNil(t, entry.TimePricing, model)
		peakMinutes := 0
		for at := start; at.Before(start.AddDate(0, 0, 8)); at = at.Add(time.Minute) {
			require.Equal(t, officialPeak(at), entry.TimePricing.MultiplierAt(at), "%s at %s", model, at.Format(time.RFC3339))
			if officialPeak(at) > 1 {
				peakMinutes++
			}
		}
		require.Equal(t, 5*7*60, peakMinutes, "每个工作日 7 小时高峰")
	}
}

// loadBuiltinPricingFile 读仓库里的内置价格文件（与生产同一份）。
func loadBuiltinPricingFile(t *testing.T) map[string]*LiteLLMModelPricing {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", "model-pricing", "model_prices_and_context_window.json"))
	require.NoError(t, err)
	pricingData, err := (&PricingService{}).parsePricingData(data)
	require.NoError(t, err)
	return pricingData
}
