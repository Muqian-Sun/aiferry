//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/modelcatalogentry"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// newModelCatalogRepoForTest 返回真实仓储，并在用例结束后清掉本次写入的条目
// （子表由外键级联删除）。
func newModelCatalogRepoForTest(t *testing.T, prefix string) (service.ModelCatalogRepository, func(name string) string) {
	t.Helper()
	client := testEntClient(t)
	repo := NewModelCatalogRepository(client)
	unique := func(name string) string { return fmt.Sprintf("%s-%s", prefix, name) }
	t.Cleanup(func() {
		_, _ = client.ModelCatalogEntry.Delete().
			Where(modelcatalogentry.ModelIDHasPrefix(prefix + "-")).
			Exec(context.Background())
	})
	return repo, unique
}

func float64Value(v float64) *float64 { return &v }

func TestModelCatalogRepository_CreateReadUpdateDelete(t *testing.T) {
	ctx := context.Background()
	repo, unique := newModelCatalogRepoForTest(t, "repo-crud")

	entry := &service.ModelCatalogEntry{
		ModelID:                       unique("sonnet"),
		DisplayName:                   "Sonnet",
		Vendor:                        "anthropic",
		Protocols:                     []string{service.ModelCatalogProtocolAnthropic},
		BillingMode:                   service.BillingModeToken,
		Status:                        service.ModelCatalogStatusListed,
		ManagedBy:                     service.ModelCatalogManagedByAdmin,
		InputPrice:                    float64Value(3e-6),
		OutputPrice:                   float64Value(15e-6),
		CacheWrite1hPrice:             float64Value(6e-6),
		InputPricePriority:            float64Value(6e-6),
		LongContextInputThreshold:     func() *int { v := 200000; return &v }(),
		LongContextThresholdInclusive: true,
		LongContextInputMultiplier:    float64Value(2),
		Intervals: []service.PricingInterval{
			{MinTokens: 0, MaxTokens: func() *int { v := 100000; return &v }(), InputPrice: float64Value(1e-6), SortOrder: 0},
			{MinTokens: 100000, InputPrice: float64Value(2e-6), SortOrder: 1},
		},
		TimePricing: &service.ChannelTimePricing{
			Timezone:     "Asia/Shanghai",
			WeekdaysOnly: true,
			Periods: []service.ChannelTimePricingPeriod{
				{StartTime: "09:00", EndTime: "12:00", Multiplier: 1.5},
			},
		},
	}
	require.NoError(t, repo.CreateEntry(ctx, entry))
	require.NotZero(t, entry.ID)

	loaded, err := repo.GetEntryByID(ctx, entry.ID)
	require.NoError(t, err)
	require.Equal(t, entry.ModelID, loaded.ModelID)
	require.Equal(t, "anthropic", loaded.Vendor)
	require.Equal(t, []string{service.ModelCatalogProtocolAnthropic}, loaded.Protocols)
	require.InDelta(t, 3e-6, *loaded.InputPrice, 1e-15)
	require.InDelta(t, 6e-6, *loaded.CacheWrite1hPrice, 1e-15)
	require.InDelta(t, 6e-6, *loaded.InputPricePriority, 1e-15)
	require.Equal(t, 200000, *loaded.LongContextInputThreshold)
	require.True(t, loaded.LongContextThresholdInclusive)
	require.Len(t, loaded.Intervals, 2)
	require.InDelta(t, 1e-6, *loaded.Intervals[0].InputPrice, 1e-15)
	require.NotNil(t, loaded.TimePricing)
	require.Equal(t, "Asia/Shanghai", loaded.TimePricing.Timezone)
	require.True(t, loaded.TimePricing.WeekdaysOnly)
	require.Len(t, loaded.TimePricing.Periods, 1)
	require.InDelta(t, 1.5, loaded.TimePricing.Periods[0].Multiplier, 1e-9)

	// 大小写不敏感查模型标识。
	byModelID, err := repo.GetEntryByModelID(ctx, unique("SONNET"))
	require.NoError(t, err)
	require.Equal(t, entry.ID, byModelID.ID)

	// 更新是整条覆盖：没给值的列必须被清空，不能留上一版的残值。
	updated := &service.ModelCatalogEntry{
		ID:          entry.ID,
		ModelID:     entry.ModelID,
		BillingMode: service.BillingModeToken,
		Status:      service.ModelCatalogStatusUnlisted,
		ManagedBy:   service.ModelCatalogManagedByAdmin,
		InputPrice:  float64Value(4e-6),
	}
	require.NoError(t, repo.UpdateEntry(ctx, updated))

	reloaded, err := repo.GetEntryByID(ctx, entry.ID)
	require.NoError(t, err)
	require.Equal(t, service.ModelCatalogStatusUnlisted, reloaded.Status)
	require.InDelta(t, 4e-6, *reloaded.InputPrice, 1e-15)
	require.Nil(t, reloaded.OutputPrice, "未给值的价格列必须被清空")
	require.Nil(t, reloaded.CacheWrite1hPrice)
	require.Nil(t, reloaded.LongContextInputThreshold)
	require.Empty(t, reloaded.Intervals, "分档整份覆盖")
	require.Nil(t, reloaded.TimePricing, "分时整份覆盖")

	require.NoError(t, repo.DeleteEntry(ctx, entry.ID))
	_, err = repo.GetEntryByID(ctx, entry.ID)
	require.Error(t, err)
}

func TestModelCatalogRepository_AliasCRUDAndUniqueness(t *testing.T) {
	ctx := context.Background()
	repo, unique := newModelCatalogRepoForTest(t, "repo-alias")

	first := &service.ModelCatalogEntry{
		ModelID: unique("a"), BillingMode: service.BillingModeToken,
		Status: service.ModelCatalogStatusListed, ManagedBy: service.ModelCatalogManagedByAdmin,
		InputPrice: float64Value(1e-6),
	}
	second := &service.ModelCatalogEntry{
		ModelID: unique("b"), BillingMode: service.BillingModeToken,
		Status: service.ModelCatalogStatusListed, ManagedBy: service.ModelCatalogManagedByAdmin,
		InputPrice: float64Value(1e-6),
	}
	require.NoError(t, repo.CreateEntry(ctx, first))
	require.NoError(t, repo.CreateEntry(ctx, second))

	alias := &service.ModelCatalogAlias{
		Alias: unique("Nick"), EntryID: first.ID, Source: service.ModelCatalogAliasSourceManual,
	}
	require.NoError(t, repo.CreateAlias(ctx, alias))
	require.NotZero(t, alias.ID)

	// 同一个别名（大小写不敏感）不能再指向另一个模型。
	dup := &service.ModelCatalogAlias{
		Alias: unique("nick"), EntryID: second.ID, Source: service.ModelCatalogAliasSourceManual,
	}
	require.ErrorIs(t, repo.CreateAlias(ctx, dup), service.ErrModelCatalogAliasExists)

	// entry_id 指向不存在的条目：外键冲突要映射成 404，不能漏成 500。
	orphan := &service.ModelCatalogAlias{
		Alias: unique("orphan"), EntryID: second.ID + 1_000_000, Source: service.ModelCatalogAliasSourceManual,
	}
	require.ErrorIs(t, repo.CreateAlias(ctx, orphan), service.ErrModelCatalogEntryNotFound)
	alias.EntryID = second.ID + 1_000_000
	require.ErrorIs(t, repo.UpdateAlias(ctx, alias), service.ErrModelCatalogEntryNotFound)

	alias.EntryID = second.ID
	require.NoError(t, repo.UpdateAlias(ctx, alias))

	loaded, err := repo.GetEntryByID(ctx, second.ID)
	require.NoError(t, err)
	require.Len(t, loaded.Aliases, 1)
	require.Equal(t, unique("Nick"), loaded.Aliases[0].Alias)

	require.NoError(t, repo.DeleteAlias(ctx, alias.ID))
	loaded, err = repo.GetEntryByID(ctx, second.ID)
	require.NoError(t, err)
	require.Empty(t, loaded.Aliases)
}

func TestModelCatalogRepository_CreateEntryRejectsDuplicateModelID(t *testing.T) {
	ctx := context.Background()
	repo, unique := newModelCatalogRepoForTest(t, "repo-dup")

	first := &service.ModelCatalogEntry{
		ModelID: unique("Model"), BillingMode: service.BillingModeToken,
		Status: service.ModelCatalogStatusListed, ManagedBy: service.ModelCatalogManagedByAdmin,
		InputPrice: float64Value(1e-6),
	}
	require.NoError(t, repo.CreateEntry(ctx, first))

	second := &service.ModelCatalogEntry{
		ModelID: unique("model"), BillingMode: service.BillingModeToken,
		Status: service.ModelCatalogStatusListed, ManagedBy: service.ModelCatalogManagedByAdmin,
		InputPrice: float64Value(1e-6),
	}
	require.ErrorIs(t, repo.CreateEntry(ctx, second), service.ErrModelCatalogEntryExists)
}

// InsertOrRefreshSeedEntries 是播种的真实落盘路径：不存在则插入、seed 则刷新、
// admin 则整条跳过，且重复执行结果稳定。
func TestModelCatalogRepository_InsertOrRefreshSeedEntries(t *testing.T) {
	ctx := context.Background()
	repo, unique := newModelCatalogRepoForTest(t, "repo-seed")

	adminEntry := &service.ModelCatalogEntry{
		ModelID: unique("admin"), BillingMode: service.BillingModeToken,
		Status: service.ModelCatalogStatusListed, ManagedBy: service.ModelCatalogManagedByAdmin,
		InputPrice: float64Value(42e-6),
	}
	require.NoError(t, repo.CreateEntry(ctx, adminEntry))

	candidates := []service.ModelCatalogEntry{
		{
			ModelID: unique("admin"), BillingMode: service.BillingModeToken,
			Status: service.ModelCatalogStatusListed, ManagedBy: service.ModelCatalogManagedBySeed,
			InputPrice: float64Value(1e-6),
		},
		{
			ModelID: unique("fresh"), BillingMode: service.BillingModeToken,
			Status: service.ModelCatalogStatusListed, ManagedBy: service.ModelCatalogManagedBySeed,
			InputPrice: float64Value(2e-6),
		},
	}

	first, err := repo.InsertOrRefreshSeedEntries(ctx, candidates)
	require.NoError(t, err)
	require.Equal(t, 1, first.Inserted)
	require.Zero(t, first.Refreshed)
	require.Equal(t, 1, first.SkippedAdmin)

	kept, err := repo.GetEntryByModelID(ctx, unique("admin"))
	require.NoError(t, err)
	require.InDelta(t, 42e-6, *kept.InputPrice, 1e-15, "admin 条目不得被播种覆盖")

	// 第二次播种：新价格文件把 seed 条目刷新到新价，admin 条目仍然跳过。
	candidates[1].InputPrice = float64Value(3e-6)
	second, err := repo.InsertOrRefreshSeedEntries(ctx, candidates)
	require.NoError(t, err)
	require.Zero(t, second.Inserted)
	require.Equal(t, 1, second.Refreshed)
	require.Equal(t, 1, second.SkippedAdmin)

	refreshed, err := repo.GetEntryByModelID(ctx, unique("fresh"))
	require.NoError(t, err)
	require.InDelta(t, 3e-6, *refreshed.InputPrice, 1e-15, "seed 条目应被刷新到当前价格文件")
}

// 单条写库失败（这里用违反 CHECK 约束的 billing_mode 触发）只跳过这一条：
// 排在它后面的条目照常写入，失败计数与原因带回结果，不返回整体错误。
func TestModelCatalogRepository_SeedSkipsFailedRows(t *testing.T) {
	ctx := context.Background()
	repo, unique := newModelCatalogRepoForTest(t, "repo-seed-rows")

	candidates := []service.ModelCatalogEntry{
		{
			ModelID: unique("bad"), BillingMode: "bogus",
			Status: service.ModelCatalogStatusListed, ManagedBy: service.ModelCatalogManagedBySeed,
			InputPrice: float64Value(1e-6),
		},
		{
			ModelID: unique("good"), BillingMode: service.BillingModeToken,
			Status: service.ModelCatalogStatusListed, ManagedBy: service.ModelCatalogManagedBySeed,
			InputPrice: float64Value(2e-6),
		},
	}

	result, err := repo.InsertOrRefreshSeedEntries(ctx, candidates)
	require.NoError(t, err)
	require.Equal(t, 1, result.Failed)
	require.Len(t, result.Errors, 1)
	require.Contains(t, result.Errors[0], unique("bad"))
	require.Equal(t, 1, result.Inserted, "rows after the failed one must still be written")

	_, err = repo.GetEntryByModelID(ctx, unique("good"))
	require.NoError(t, err)
}

// ctx 到期要整体中止并把 ctx 错误返回，而不是把剩下几百条都记成「写库失败」。
// 用足够多的候选配一个短超时，让截止时刻落在逐条写入的中途：本地 1000 条 upsert
// 远超 30ms，初始查询远低于 30ms。
func TestModelCatalogRepository_SeedAbortsWhenContextExpiresMidway(t *testing.T) {
	repo, unique := newModelCatalogRepoForTest(t, "repo-seed-ctx")
	candidates := make([]service.ModelCatalogEntry, 0, 1000)
	for i := 0; i < cap(candidates); i++ {
		candidates = append(candidates, service.ModelCatalogEntry{
			ModelID: unique(fmt.Sprintf("m%04d", i)), BillingMode: service.BillingModeToken,
			Status: service.ModelCatalogStatusListed, ManagedBy: service.ModelCatalogManagedBySeed,
			InputPrice: float64Value(1e-6),
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	result, err := repo.InsertOrRefreshSeedEntries(ctx, candidates)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, result.Inserted, len(candidates))
	require.Zero(t, result.Failed, "rows skipped because the context expired must not be reported as row failures")
	require.Empty(t, result.Errors)
}

// 播种刷新只动条目本身：真实仓储里别名 / 分档 / 分时必须原样保留。
func TestModelCatalogRepository_SeedRefreshKeepsChildren(t *testing.T) {
	ctx := context.Background()
	repo, unique := newModelCatalogRepoForTest(t, "repo-seed-children")

	entry := &service.ModelCatalogEntry{
		ModelID: unique("m"), BillingMode: service.BillingModeToken,
		Status: service.ModelCatalogStatusListed, ManagedBy: service.ModelCatalogManagedBySeed,
		InputPrice: float64Value(1e-6),
		Intervals: []service.PricingInterval{
			{MinTokens: 0, InputPrice: float64Value(2e-6)},
		},
		TimePricing: &service.ChannelTimePricing{
			Timezone: "UTC",
			Periods:  []service.ChannelTimePricingPeriod{{StartTime: "01:00", EndTime: "02:00", Multiplier: 3}},
		},
	}
	require.NoError(t, repo.CreateEntry(ctx, entry))
	require.NoError(t, repo.CreateAlias(ctx, &service.ModelCatalogAlias{
		Alias: unique("nick"), EntryID: entry.ID, Source: service.ModelCatalogAliasSourceManual,
	}))

	result, err := repo.InsertOrRefreshSeedEntries(ctx, []service.ModelCatalogEntry{{
		ModelID: unique("m"), BillingMode: service.BillingModeToken,
		Status: service.ModelCatalogStatusListed, ManagedBy: service.ModelCatalogManagedBySeed,
		InputPrice: float64Value(5e-6),
	}})
	require.NoError(t, err)
	require.Equal(t, 1, result.Refreshed)

	loaded, err := repo.GetEntryByID(ctx, entry.ID)
	require.NoError(t, err)
	require.InDelta(t, 5e-6, *loaded.InputPrice, 1e-15)
	require.Len(t, loaded.Aliases, 1, "seed refresh must not drop aliases")
	require.Len(t, loaded.Intervals, 1, "seed refresh must not drop price intervals")
	require.NotNil(t, loaded.TimePricing, "seed refresh must not drop time pricing")
}

// ListEntries 必须把别名 / 分档 / 分时挂回对应条目：快照少挂一项，
// 计费就会漏掉分档或分时。
func TestModelCatalogRepository_ListEntriesHydratesChildren(t *testing.T) {
	ctx := context.Background()
	repo, unique := newModelCatalogRepoForTest(t, "repo-list")

	entry := &service.ModelCatalogEntry{
		ModelID: unique("m"), BillingMode: service.BillingModeToken,
		Status: service.ModelCatalogStatusListed, ManagedBy: service.ModelCatalogManagedBySeed,
		InputPrice: float64Value(1e-6),
		Intervals: []service.PricingInterval{
			{MinTokens: 0, InputPrice: float64Value(2e-6)},
		},
		TimePricing: &service.ChannelTimePricing{
			Timezone: "UTC",
			Periods:  []service.ChannelTimePricingPeriod{{StartTime: "01:00", EndTime: "02:00", Multiplier: 3}},
		},
	}
	require.NoError(t, repo.CreateEntry(ctx, entry))
	require.NoError(t, repo.CreateAlias(ctx, &service.ModelCatalogAlias{
		Alias: unique("nick"), EntryID: entry.ID, Source: service.ModelCatalogAliasSourceSeed,
	}))

	entries, err := repo.ListEntries(ctx)
	require.NoError(t, err)

	var found *service.ModelCatalogEntry
	for i := range entries {
		if entries[i].ID == entry.ID {
			found = &entries[i]
			break
		}
	}
	require.NotNil(t, found)
	require.Len(t, found.Aliases, 1)
	require.Len(t, found.Intervals, 1)
	require.NotNil(t, found.TimePricing)
	require.InDelta(t, 3, found.TimePricing.Periods[0].Multiplier, 1e-9)
}
