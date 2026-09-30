//go:build unit

package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func catalogEntryForTest(id int64, modelID string, aliases ...string) ModelCatalogEntry {
	entry := ModelCatalogEntry{
		ID:          id,
		ModelID:     modelID,
		BillingMode: BillingModeToken,
		Status:      ModelCatalogStatusListed,
		ManagedBy:   ModelCatalogManagedBySeed,
		InputPrice:  testPtrFloat64(1e-6),
	}
	for i, alias := range aliases {
		entry.Aliases = append(entry.Aliases, ModelCatalogAlias{
			ID:      int64(i + 1),
			EntryID: id,
			Alias:   alias,
			Source:  ModelCatalogAliasSourceManual,
		})
	}
	return entry
}

func TestModelCatalogLookup_ExactModelIDIsCaseInsensitive(t *testing.T) {
	svc, _ := newTestModelCatalogService(catalogEntryForTest(1, "claude-sonnet-4"))

	entry := svc.LookupPricingEntry(context.Background(), "Claude-Sonnet-4")
	require.NotNil(t, entry)
	require.Equal(t, "claude-sonnet-4", entry.ModelID)
}

// 目录查表复用渠道定价的模型名归一化：claude-* 里的 "." 与 "-" 等价。
func TestModelCatalogLookup_NormalizesClaudeDotSpelling(t *testing.T) {
	svc, _ := newTestModelCatalogService(catalogEntryForTest(1, "claude-opus-4-5"))

	entry := svc.LookupPricingEntry(context.Background(), "claude-opus-4.5")
	require.NotNil(t, entry)
	require.Equal(t, "claude-opus-4-5", entry.ModelID)
}

func TestModelCatalogLookup_ExactAlias(t *testing.T) {
	svc, _ := newTestModelCatalogService(catalogEntryForTest(1, "claude-sonnet-4", "sonnet"))

	entry := svc.LookupPricingEntry(context.Background(), "SONNET")
	require.NotNil(t, entry)
	require.Equal(t, "claude-sonnet-4", entry.ModelID)
}

// 前缀别名按最长前缀胜出，结果不依赖行序。
func TestModelCatalogLookup_LongestPrefixAliasWins(t *testing.T) {
	broad := catalogEntryForTest(1, "broad-model", "claude-*")
	narrow := catalogEntryForTest(2, "narrow-model", "claude-sonnet-*")

	for _, order := range [][]ModelCatalogEntry{{broad, narrow}, {narrow, broad}} {
		svc, _ := newTestModelCatalogService(order...)
		entry := svc.LookupPricingEntry(context.Background(), "claude-sonnet-4")
		require.NotNil(t, entry)
		require.Equal(t, "narrow-model", entry.ModelID)
	}
}

// 精确模型标识优先于别名：别名不能盖掉一个真实存在的模型。
func TestModelCatalogLookup_ExactModelIDBeatsAlias(t *testing.T) {
	aliasHolder := catalogEntryForTest(1, "alias-holder", "claude-sonnet-4")
	exact := catalogEntryForTest(2, "claude-sonnet-4")

	svc, _ := newTestModelCatalogService(aliasHolder, exact)
	entry := svc.LookupPricingEntry(context.Background(), "claude-sonnet-4")
	require.NotNil(t, entry)
	require.Equal(t, "claude-sonnet-4", entry.ModelID)
}

func TestModelCatalogLookup_MissReturnsNil(t *testing.T) {
	svc, _ := newTestModelCatalogService(catalogEntryForTest(1, "claude-sonnet-4"))
	require.Nil(t, svc.LookupPricingEntry(context.Background(), "gpt-5.1"))
}

// 目录是基准价：只写显式配置的项，没配的项保留价格文件里的值。
func TestApplyToModelPricing_OnlyWritesConfiguredFields(t *testing.T) {
	base := &ModelPricing{
		InputPricePerToken:       3e-6,
		OutputPricePerToken:      15e-6,
		ImageOutputPricePerToken: 40e-6,
		ImageInputPricePerToken:  2e-6,
	}
	entry := &ModelCatalogEntry{InputPrice: testPtrFloat64(10e-6)}

	entry.ApplyToModelPricing(base)

	require.InDelta(t, 10e-6, base.InputPricePerToken, 1e-12)
	require.InDelta(t, 15e-6, base.OutputPricePerToken, 1e-12, "未配置的项必须保留基准价")
	require.InDelta(t, 40e-6, base.ImageOutputPricePerToken, 1e-12, "图片输出价不得被归零")
	require.False(t, base.ImageOutputPriceExplicit)
	require.InDelta(t, 2e-6, base.ImageInputPricePerToken, 1e-12, "图片输入价不得被归零")
}

// 5m/1h 分档只在 1h 价严格高于 5m 价时成立：写反了必须不分档，
// 否则 1h 缓存会按更低的价算。
func TestApplyToModelPricing_CacheBreakdownRequiresHigher1hPrice(t *testing.T) {
	t.Run("1h higher enables breakdown", func(t *testing.T) {
		base := &ModelPricing{}
		entry := &ModelCatalogEntry{
			CacheWritePrice:   testPtrFloat64(3.75e-6),
			CacheWrite1hPrice: testPtrFloat64(6e-6),
		}
		entry.ApplyToModelPricing(base)
		require.True(t, base.SupportsCacheBreakdown)
		require.InDelta(t, 6e-6, base.CacheCreation1hPrice, 1e-12)
	})

	t.Run("1h not higher disables breakdown", func(t *testing.T) {
		base := &ModelPricing{SupportsCacheBreakdown: true}
		entry := &ModelCatalogEntry{
			CacheWritePrice:   testPtrFloat64(3.75e-6),
			CacheWrite1hPrice: testPtrFloat64(1e-6),
		}
		entry.ApplyToModelPricing(base)
		require.False(t, base.SupportsCacheBreakdown)
	})
}

func TestModelCatalogEntry_Validate(t *testing.T) {
	valid := func() *ModelCatalogEntry {
		entry := &ModelCatalogEntry{ModelID: "m", InputPrice: testPtrFloat64(1e-6)}
		entry.Normalize()
		return entry
	}

	require.NoError(t, valid().Validate())

	t.Run("empty model id", func(t *testing.T) {
		entry := valid()
		entry.ModelID = ""
		require.Error(t, entry.Validate())
	})

	t.Run("negative price", func(t *testing.T) {
		entry := valid()
		entry.OutputPrice = testPtrFloat64(-1)
		require.Error(t, entry.Validate())
	})

	t.Run("non-positive multiplier", func(t *testing.T) {
		entry := valid()
		entry.MaxReasoningEffortMultiplier = testPtrFloat64(0)
		require.Error(t, entry.Validate())
	})

	t.Run("unknown protocol", func(t *testing.T) {
		entry := valid()
		entry.Protocols = []string{"grpc"}
		require.Error(t, entry.Validate())
	})

	t.Run("unknown status", func(t *testing.T) {
		entry := valid()
		entry.Status = "draft"
		require.Error(t, entry.Validate())
	})

	// 上架即可调用，所以上架条目必须能独立定价；下架条目可以先建骨架再补价。
	t.Run("listed without price", func(t *testing.T) {
		entry := valid()
		entry.Status = ModelCatalogStatusListed
		entry.InputPrice = nil
		require.Error(t, entry.Validate())
	})

	t.Run("unlisted without price", func(t *testing.T) {
		entry := valid()
		entry.Status = ModelCatalogStatusUnlisted
		entry.InputPrice = nil
		require.NoError(t, entry.Validate())
	})

	t.Run("per_request listed with per_request_price", func(t *testing.T) {
		entry := valid()
		entry.Status = ModelCatalogStatusListed
		entry.BillingMode = BillingModePerRequest
		entry.InputPrice = nil
		entry.PerRequestPrice = testPtrFloat64(0.01)
		require.NoError(t, entry.Validate())
	})

	t.Run("per_request listed without any price", func(t *testing.T) {
		entry := valid()
		entry.Status = ModelCatalogStatusListed
		entry.BillingMode = BillingModePerRequest
		entry.InputPrice = nil
		require.Error(t, entry.Validate())
	})

	t.Run("normalize lowercases vendor and defaults status to unlisted", func(t *testing.T) {
		entry := &ModelCatalogEntry{ModelID: "m", Vendor: " OpenAI "}
		entry.Normalize()
		require.Equal(t, "openai", entry.Vendor)
		require.Equal(t, ModelCatalogStatusUnlisted, entry.Status)
	})

	t.Run("overlapping intervals", func(t *testing.T) {
		entry := valid()
		entry.Intervals = []PricingInterval{
			{MinTokens: 0, MaxTokens: testPtrInt(100), InputPrice: testPtrFloat64(1e-6)},
			{MinTokens: 50, MaxTokens: testPtrInt(200), InputPrice: testPtrFloat64(2e-6)},
		}
		require.Error(t, entry.Validate())
	})

	t.Run("invalid time pricing", func(t *testing.T) {
		entry := valid()
		entry.TimePricing = &TimePricing{
			Timezone: "Not/AZone",
			Periods:  []TimePricingPeriod{{StartTime: "09:00", EndTime: "10:00", Multiplier: 2}},
		}
		require.Error(t, entry.Validate())
	})

	// 长度上限与 243 号迁移的 VARCHAR(n) 一致；超长要在校验层拦住，不能等到落库时
	// 让整次播种半途而废。按字符数计：200 个汉字是合法的 display_name。
	t.Run("column lengths", func(t *testing.T) {
		cjk := strings.Repeat("模", 200)
		entry := valid()
		entry.DisplayName = cjk
		require.NoError(t, entry.Validate(), "length is counted in characters, not bytes")

		for name, mutate := range map[string]func(*ModelCatalogEntry){
			"model_id":     func(e *ModelCatalogEntry) { e.ModelID = strings.Repeat("m", 201) },
			"display_name": func(e *ModelCatalogEntry) { e.DisplayName = cjk + "模" },
			"vendor":       func(e *ModelCatalogEntry) { e.Vendor = strings.Repeat("v", 51) },
			"tier_label": func(e *ModelCatalogEntry) {
				e.Intervals = []PricingInterval{{MinTokens: 0, InputPrice: testPtrFloat64(1e-6), TierLabel: strings.Repeat("t", 51)}}
			},
			"timezone": func(e *ModelCatalogEntry) {
				e.TimePricing = &TimePricing{
					Timezone: strings.Repeat("Z", 65),
					Periods:  []TimePricingPeriod{{StartTime: "09:00", EndTime: "10:00", Multiplier: 2}},
				}
			},
		} {
			entry := valid()
			mutate(entry)
			require.ErrorContains(t, entry.Validate(), name+" must be at most", name)
		}
	})
}

func TestValidateModelCatalogAlias(t *testing.T) {
	require.NoError(t, ValidateModelCatalogAlias("claude-sonnet-4", ModelCatalogAliasSourceManual))
	require.NoError(t, ValidateModelCatalogAlias("claude-sonnet-*", ModelCatalogAliasSourceManual))
	require.Error(t, ValidateModelCatalogAlias("", ModelCatalogAliasSourceManual))
	// 全量通配会让任意模型名都拿到同一份价卡，等于关掉"查不到价"这个信号。
	require.Error(t, ValidateModelCatalogAlias("*", ModelCatalogAliasSourceManual))
	require.Error(t, ValidateModelCatalogAlias("claude-*-4", ModelCatalogAliasSourceManual))
	require.Error(t, ValidateModelCatalogAlias("claude", "importer"))
	require.NoError(t, ValidateModelCatalogAlias(strings.Repeat("别", 200), ModelCatalogAliasSourceManual))
	require.ErrorContains(t, ValidateModelCatalogAlias(strings.Repeat("a", 201), ModelCatalogAliasSourceManual), "alias must be at most")
}

// 管理端任何一次写入都把 managed_by 翻成 admin，之后播种器不再覆盖。
func TestModelCatalogService_WritesMarkEntryAsAdminManaged(t *testing.T) {
	svc, repo := newTestModelCatalogService()
	ctx := context.Background()

	entry := &ModelCatalogEntry{ModelID: "m", ManagedBy: ModelCatalogManagedBySeed, InputPrice: testPtrFloat64(1e-6)}
	require.NoError(t, svc.CreateEntry(ctx, entry))
	require.Equal(t, ModelCatalogManagedByAdmin, entry.ManagedBy)
	require.Equal(t, ModelCatalogManagedByAdmin, repo.entries[0].ManagedBy)

	entry.ManagedBy = ModelCatalogManagedBySeed
	require.NoError(t, svc.UpdateEntry(ctx, entry))
	require.Equal(t, ModelCatalogManagedByAdmin, repo.entries[0].ManagedBy)
}

// 写入后必须立刻失效本地快照，否则管理员改完价要等 TTL 才生效。
func TestModelCatalogService_WriteInvalidatesSnapshot(t *testing.T) {
	svc, repo := newTestModelCatalogService(catalogEntryForTest(1, "m1"))
	ctx := context.Background()

	require.NotNil(t, svc.LookupPricingEntry(ctx, "m1"))
	loadsAfterFirstLookup := repo.listCalls

	require.NoError(t, svc.CreateEntry(ctx, &ModelCatalogEntry{ModelID: "m2", InputPrice: testPtrFloat64(1e-6)}))

	require.NotNil(t, svc.LookupPricingEntry(ctx, "m2"))
	require.Greater(t, repo.listCalls, loadsAfterFirstLookup, "write must drop the cached snapshot")
}

// 未命中时不该每次都回库：快照在 TTL 内复用。
func TestModelCatalogService_SnapshotIsCachedBetweenLookups(t *testing.T) {
	svc, repo := newTestModelCatalogService(catalogEntryForTest(1, "m1"))
	ctx := context.Background()

	svc.LookupPricingEntry(ctx, "m1")
	svc.LookupPricingEntry(ctx, "m1")
	require.Equal(t, 1, repo.listCalls)
}

// 目录条目没有任何 token 价、底下价格文件也定不到价时，必须保持"无价"，
// 让计费链路走既有的 ErrModelPricingUnavailable 路径，而不是悄悄变成全 0 价卡。
func TestResolve_CatalogEntryWithoutPricesKeepsModelUnpriced(t *testing.T) {
	bs := &BillingService{fallbackPrices: map[string]*ModelPricing{}}
	svc, _ := newTestModelCatalogService(ModelCatalogEntry{
		ID: 1, ModelID: "ghost-model", BillingMode: BillingModeToken,
		Status: ModelCatalogStatusListed, ManagedBy: ModelCatalogManagedByAdmin,
	})
	r := NewModelPricingResolver(svc, bs)

	resolved := r.Resolve(context.Background(), PricingInput{Model: "ghost-model"})
	require.Equal(t, PricingSourceCatalog, resolved.Source)
	require.Nil(t, resolved.BasePricing)

	_, err := bs.CalculateCostUnified(CostInput{
		Ctx: context.Background(), Model: "ghost-model",
		Tokens: UsageTokens{InputTokens: 1000}, RateMultiplier: 1,
		Resolver: r, Resolved: resolved,
	})
	require.ErrorIs(t, err, ErrModelPricingUnavailable)
}
