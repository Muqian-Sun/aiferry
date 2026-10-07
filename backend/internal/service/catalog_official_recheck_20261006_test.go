//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 2026-10-06 晚按 11 家官网复核目录（muqian：「11 家再核一遍」「只补对话模型」「国际站没有的按国内站换算」）。
func TestCatalogOfficialRecheck20261006(t *testing.T) {
	byID := seededBuiltinCatalog(t)

	// OpenAI：Pro 系列官网没有缓存价；只剩已宣布停服快照的不进目录；gpt-5.6 别名指向 gpt-5.6-sol、同价
	for _, model := range []string{"gpt-5.4-pro", "gpt-5.4-pro-2026-03-05", "gpt-5.5-pro", "gpt-5.5-pro-2026-04-23"} {
		require.Nil(t, byID[model].CacheReadPrice, model)
		require.NotNil(t, byID[model].InputPrice, model)
	}
	for _, model := range []string{"gpt-4o-mini-tts", "gpt-5.1-2025-11-13", "gpt-5.4-nano-2026-03-17", "gemini-3.1-flash-lite"} {
		_, ok := byID[model]
		require.False(t, ok, "%s 官网已宣布停服，不进目录", model)
	}
	alias, sol := byID["gpt-5.6"], byID["gpt-5.6-sol"]
	requirePrice(t, *sol.InputPrice, alias.InputPrice)
	requirePrice(t, *sol.OutputPrice, alias.OutputPrice)
	requirePrice(t, *sol.CacheReadPrice, alias.CacheReadPrice)
	requirePrice(t, *sol.CacheWritePrice, alias.CacheWritePrice)
	require.Equal(t, sol.Intervals, alias.Intervals)

	// Kimi（国内站人民币价 ÷ 6.8）：K3 缓存写入 5 分钟 ¥20 / 1 小时 ¥40；K2.6 缓存命中 ¥1.1
	k3 := byID["kimi-k3"]
	requirePrice(t, cnyPerMillion(20), k3.InputPrice)
	requirePrice(t, cnyPerMillion(20), k3.CacheWritePrice)
	requirePrice(t, cnyPerMillion(40), k3.CacheWrite1hPrice)
	requirePrice(t, cnyPerMillion(1.1), byID["kimi-k2.6"].CacheReadPrice)

	// 智谱（国内站人民币价 ÷ 6.8）：GLM-5-Turbo / GLM-5V-Turbo 分 [0, 32K) 与 ≥32K 两段
	for _, model := range []string{"glm-5-turbo", "glm-5v-turbo"} {
		entry := byID[model]
		require.Equal(t, "zhipu", entry.Vendor, model)
		requirePrice(t, cnyPerMillion(5), entry.InputPrice, model)
		requirePrice(t, cnyPerMillion(22), entry.OutputPrice, model)
		requirePrice(t, cnyPerMillion(1.2), entry.CacheReadPrice, model)
		require.Len(t, entry.Intervals, 1, model)
		requirePrice(t, cnyPerMillion(7), entry.Intervals[0].InputPrice, model)
		requirePrice(t, cnyPerMillion(26), entry.Intervals[0].OutputPrice, model)
		requirePrice(t, cnyPerMillion(1.8), entry.Intervals[0].CacheReadPrice, model)
	}
	requirePrice(t, cnyPerMillion(1), byID["glm-4-long"].InputPrice)
	requirePrice(t, cnyPerMillion(0.5), byID["glm-4-long"].CacheReadPrice)
	requirePrice(t, cnyPerMillion(0.1), byID["glm-4-flashx-250414"].OutputPrice)
	requirePrice(t, cnyPerMillion(2), byID["glm-4.1v-thinking-flashx"].InputPrice)
	require.Nil(t, byID["glm-4.1v-thinking-flashx"].CacheReadPrice, "国内站标「不支持」缓存")

	// 豆包滚动迭代模型 ID 不固定指向，不收（muqian 10-06）
	_, evolving := byID["doubao-seed-evolving"]
	require.False(t, evolving)

	// 通义角色扮演（国内站人民币价 ÷ 6.8）：在隐式缓存支持列表里，命中价按国内站单模型页；-ja 国内站没有，不收
	requirePrice(t, cnyPerMillion(0.16), byID["qwen-plus-character"].CacheReadPrice)
	requirePrice(t, cnyPerMillion(0.05), byID["qwen-flash-character"].CacheReadPrice)
	_, ja := byID["qwen-plus-character-ja"]
	require.False(t, ja)
}

// 智谱国内站分段写的是「[0, 32K)」「≥32K」：恰好 32000 就进高段（分段左开右闭，价格文件第一段上限写 31999）。
func TestGLM5TurboEntersHighTierAt32K(t *testing.T) {
	ps := newStubPricingServiceFromJSON(t, string(readBuiltinPricingFile(t)))
	bs := NewBillingService()
	resolver := newResolverWithSeededEntries(bs, seededLiteLLMEntry(t, ps, "glm-5-turbo"))

	below := costViaCatalog(t, bs, resolver, "glm-5-turbo", UsageTokens{InputTokens: 31_999})
	require.InEpsilon(t, 31_999*cnyPerMillion(5), below.InputCost, 1e-5)
	at := costViaCatalog(t, bs, resolver, "glm-5-turbo", UsageTokens{InputTokens: 32_000})
	require.InEpsilon(t, 32_000*cnyPerMillion(7), at.InputCost, 1e-5)
}

// 只有显式缓存的通义模型：上游报的命中 / 创建 token 按官网显式价收（原来缓存价空着，命中与创建都按 0 收）。
func TestQwenExplicitCacheBilledAtOfficialPrices(t *testing.T) {
	ps := newStubPricingServiceFromJSON(t, string(readBuiltinPricingFile(t)))
	bs := NewBillingService()
	resolver := newResolverWithSeededEntries(bs, seededLiteLLMEntry(t, ps, "qwen3.6-plus"))

	cost := costViaCatalog(t, bs, resolver, "qwen3.6-plus", UsageTokens{
		InputTokens: 10_000, CacheReadTokens: 50_000, CacheCreationTokens: 20_000, OutputTokens: 1_000,
	})
	require.InEpsilon(t, 50_000*cnyPerMillion(0.2), cost.CacheReadCost, 1e-5)
	require.InEpsilon(t, 20_000*cnyPerMillion(2.5), cost.CacheCreationCost, 1e-5)
}

// 官网免费的文本模型（muqian 10-06「免费的文本模型加入」）：存显式 0 价——能上架、按 0 计费；
// 利润门开着时上游也免费就放行，上游收钱照旧挡掉。
func TestFreeTextModelsSeedAsZeroPriced(t *testing.T) {
	byID := seededBuiltinCatalog(t)
	bs := NewBillingService()
	for _, model := range []string{"glm-4.7-flash", "glm-4-flash-250414"} { // glm-4.5-flash 国内站没有，不收（10-07）
		entry, ok := byID[model]
		require.True(t, ok, model)
		require.Equal(t, "zhipu", entry.Vendor, model)
		require.NotNil(t, entry.InputPrice, model)
		require.NotNil(t, entry.OutputPrice, model)
		require.Zero(t, *entry.InputPrice, model)
		require.Zero(t, *entry.OutputPrice, model)

		entry.Status = ModelCatalogStatusListed
		entry.Normalize()
		require.NoError(t, entry.Validate(), "免费模型要能上架：%s", model)

		cost, err := builtinCatalogCost(bs, model, UsageTokens{InputTokens: 1000, OutputTokens: 1000}, 1)
		require.NoError(t, err, model)
		require.Zero(t, cost.TotalCost, model)

		free := &ModelCatalogBinding{}
		ratio, ok := entry.UpstreamCostRatio(free)
		require.True(t, ok, model)
		require.Zero(t, ratio, model)
		paid := &ModelCatalogBinding{InputPrice: 1e-7}
		_, ok = entry.UpstreamCostRatio(paid)
		require.False(t, ok, "上游收钱、官方免费：比不出，利润门按缺价挡掉")
	}
}
