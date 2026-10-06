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

// 价格文件里的 DeepSeek 官方价（计费只认目录，目录按价格文件播种）：低谷价。deepseek-v4-pro 按 V4-Pro 价收——
// 官方更新日志 2026-09-10：「continue providing API services for DeepSeek V4 Pro after September 14, 2026,
// with the billing method remaining unchanged」（同日公告里「路由到 V4.1-Flash 按 Flash 价收」已撤回）。
func TestDeepseekPricingFileMatchesOfficialRates(t *testing.T) {
	pricingData := loadBuiltinPricingFile(t)

	_, ok := pricingData["deepseek-v3-2-251201"]
	require.False(t, ok, "deepseek-v3-2-251201（$0 占位条目）必须从价格表中移除")
	for _, discontinued := range []string{"deepseek-chat", "deepseek-reasoner"} {
		_, ok := pricingData[discontinued]
		require.False(t, ok, "%s 已停止服务，必须从价格表中移除", discontinued)
	}

	for _, tt := range []struct {
		model                    string
		input, output, cacheRead float64
	}{
		{"deepseek-flash", 1.5e-7, 6e-7, 3e-9},
		{"deepseek-v4-flash", 1.5e-7, 6e-7, 3e-9},
		{"deepseek-v4-flash-vision-exp", 1.5e-7, 6e-7, 3e-9},
		{"deepseek-v4-pro", 6.6e-7, 1.98e-6, 2.2e-8},
	} {
		t.Run(tt.model, func(t *testing.T) {
			entry, ok := pricingData[tt.model]
			require.True(t, ok, "model %s must exist in pricing file", tt.model)
			require.InDelta(t, tt.input, entry.InputCostPerToken, 1e-15)
			require.InDelta(t, tt.output, entry.OutputCostPerToken, 1e-15)
			require.InDelta(t, tt.cacheRead, entry.CacheReadInputTokenCost, 1e-15)
		})
	}
}

// DeepSeek 官方高峰（2026-08-23 起）：UTC 01:00–04:00 与 06:00–10:00（半开区间），北京时间周一到周五，
// 不含中国法定节假日（定价页：excluding Chinese public holidays）；高峰价 = 2 × 低谷价。
// 价格文件的 time_pricing 播进目录后逐分钟都要与这条官方规则一致。节假日按国办发明电〔2025〕7号
// （2026 年放假调休日期），覆盖中秋、国庆两段假期。
func TestDeepseekPricingFileTimePricingMatchesOfficialPeak(t *testing.T) {
	pricingData := loadBuiltinPricingFile(t)
	beijing := time.FixedZone("Asia/Shanghai", 8*3600)
	holidays := map[string]bool{}
	for _, span := range [][2]string{{"2026-09-25", "2026-09-27"}, {"2026-10-01", "2026-10-07"}} {
		day, _ := time.Parse(time.DateOnly, span[0])
		last, _ := time.Parse(time.DateOnly, span[1])
		for ; !day.After(last); day = day.AddDate(0, 0, 1) {
			holidays[day.Format(time.DateOnly)] = true
		}
	}
	officialPeak := func(at time.Time) float64 {
		local := at.In(beijing)
		switch local.Weekday() {
		case time.Saturday, time.Sunday:
			return 1
		}
		if holidays[local.Format(time.DateOnly)] {
			return 1
		}
		if h := at.UTC().Hour(); (h >= 1 && h < 4) || (h >= 6 && h < 10) {
			return 2
		}
		return 1
	}
	start := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC) // 跨中秋、国庆到 10 月 10 日（周六调休上班，按官方仍是闲时）
	for _, model := range []string{"deepseek-flash", "deepseek-v4-flash", "deepseek-v4-flash-vision-exp", "deepseek-v4-pro"} {
		entry := seedEntryFromLiteLLM(model, pricingData[model])
		require.NotNil(t, entry.TimePricing, model)
		peakMinutes := 0
		for at := start; at.Before(start.AddDate(0, 0, 21)); at = at.Add(time.Minute) {
			require.Equal(t, officialPeak(at), entry.TimePricing.MultiplierAt(at), "%s at %s", model, at.Format(time.RFC3339))
			if officialPeak(at) > 1 {
				peakMinutes++
			}
		}
		// 21 天里工作日 15 天，去掉 9-25 与 10-1 ～ 10-7 中的 6 个工作日，剩 9 天 × 7 小时
		require.Equal(t, 9*7*60, peakMinutes, model)
	}
	// 国庆假期内的工作日上午（北京 10-06 10:00）按闲时
	require.Equal(t, 1.0, seedEntryFromLiteLLM("deepseek-flash", pricingData["deepseek-flash"]).TimePricing.MultiplierAt(time.Date(2026, 10, 6, 2, 0, 0, 0, time.UTC)))
}

// 官网版本名播种成目录条目的显示名（中转按版本名叫：deepseek-v4.1-flash）。
func TestDeepseekPricingFileDisplayNames(t *testing.T) {
	pricingData := loadBuiltinPricingFile(t)
	require.Equal(t, "DeepSeek-V4.1-Flash", seedEntryFromLiteLLM("deepseek-flash", pricingData["deepseek-flash"]).DisplayName)
	require.Equal(t, "DeepSeek-V4-Pro-0813", seedEntryFromLiteLLM("deepseek-v4-pro", pricingData["deepseek-v4-pro"]).DisplayName)
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
