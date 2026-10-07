//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/modelcatalogentry"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// newModelCatalogRepoForTest 返回真实仓储，并在用例结束后清掉本次写入的条目
// （子表由外键级联删除）。
func newModelCatalogRepoForTest(t *testing.T, prefix string) (service.ModelCatalogRepository, func(name string) string) {
	t.Helper()
	client := testEntClient(t)
	repo := NewModelCatalogRepository(client, integrationDB)
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
		ModelID:           unique("sonnet"),
		DisplayName:       "Sonnet",
		Vendor:            "anthropic",
		Protocols:         []string{service.ModelCatalogProtocolAnthropic},
		BillingMode:       service.BillingModeToken,
		Status:            service.ModelCatalogStatusListed,
		ManagedBy:         service.ModelCatalogManagedByAdmin,
		InputPrice:        float64Value(3e-6),
		OutputPrice:       float64Value(15e-6),
		CacheWrite1hPrice: float64Value(6e-6),
		Intervals: []service.PricingInterval{
			{MinTokens: 0, MaxTokens: func() *int { v := 100000; return &v }(), InputPrice: float64Value(1e-6), SortOrder: 0},
			{MinTokens: 100000, InputPrice: float64Value(2e-6), SortOrder: 1},
		},
		TimePricing: &service.TimePricing{
			Timezone:     "Asia/Shanghai",
			WeekdaysOnly: true,
			Periods: []service.TimePricingPeriod{
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
	require.Empty(t, reloaded.Intervals, "分档整份覆盖")
	require.Nil(t, reloaded.TimePricing, "分时整份覆盖")

	require.NoError(t, repo.DeleteEntry(ctx, entry.ID))
	_, err = repo.GetEntryByID(ctx, entry.ID)
	require.Error(t, err)
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
		TimePricing: &service.TimePricing{
			Timezone: "UTC",
			Periods:  []service.TimePricingPeriod{{StartTime: "01:00", EndTime: "02:00", Multiplier: 3}},
		},
	}
	require.NoError(t, repo.CreateEntry(ctx, entry))

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
	require.Len(t, loaded.Intervals, 1, "seed refresh must not drop price intervals")
	require.NotNil(t, loaded.TimePricing, "seed refresh must not drop time pricing")
}

// ListEntries 必须把分档 / 分时挂回对应条目：快照少挂一项，
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
		TimePricing: &service.TimePricing{
			Timezone: "UTC",
			Periods:  []service.TimePricingPeriod{{StartTime: "01:00", EndTime: "02:00", Multiplier: 3}},
		},
	}
	require.NoError(t, repo.CreateEntry(ctx, entry))

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
	require.Len(t, found.Intervals, 1)
	require.NotNil(t, found.TimePricing)
	require.InDelta(t, 3, found.TimePricing.Periods[0].Multiplier, 1e-9)
}

// 承接关系带上游价：读出时输入 / 输出 / 三项缓存价原样带出（缓存价没填的保持 nil），
// 上游分段（price_intervals JSONB）按数组顺序转成 PricingInterval（SortOrder = 下标、无档位名）；
// 按账号 ID 排序；GetEntryByID / ListEntries 都挂上；随账号删除级联消失；删条目投递 catalog_bindings_changed。
// 承接关系的写入在价格页接口里，这里直接用 ent 落行。
func TestModelCatalogRepository_BindingsCarryUpstreamPrices(t *testing.T) {
	ctx := context.Background()
	repo, unique := newModelCatalogRepoForTest(t, "repo-bind")
	client := testEntClient(t)

	entry := &service.ModelCatalogEntry{
		ModelID: unique("sonnet"), Vendor: "anthropic", BillingMode: service.BillingModeToken,
		Status: service.ModelCatalogStatusListed, ManagedBy: service.ModelCatalogManagedByAdmin,
		InputPrice: float64Value(1e-6),
	}
	require.NoError(t, repo.CreateEntry(ctx, entry))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(),
			"DELETE FROM scheduler_outbox WHERE event_type = $1 AND payload->'entry_ids' @> $2::jsonb",
			service.SchedulerOutboxEventCatalogBindingsChanged, fmt.Sprintf("[%d]", entry.ID))
	})

	accountA := mustCreateAccount(t, client, &service.Account{Name: unique("a"), Priority: 10})
	accountB := mustCreateAccount(t, client, &service.Account{Name: unique("b"), Priority: 20})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = ANY($1)", pq.Array([]int64{accountA.ID, accountB.ID}))
	})

	// 先写 B（只填必填的输入 / 输出、没有分段），再写 A（全填、两段），读出来要按账号 ID 排。
	segmentEnd := 200000
	_, err := client.ModelCatalogBinding.Create().
		SetEntryID(entry.ID).
		SetAccountID(accountB.ID).
		SetInputPrice(2e-6).
		SetOutputPrice(8e-6).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.ModelCatalogBinding.Create().
		SetEntryID(entry.ID).
		SetAccountID(accountA.ID).
		SetInputPrice(3e-6).
		SetOutputPrice(1.5e-5).
		SetCacheWritePrice(3.75e-6).
		SetCacheWrite1hPrice(6e-6).
		SetCacheReadPrice(3e-7).
		SetPriceIntervals([]domain.PriceSegment{
			{MinTokens: 0, MaxTokens: &segmentEnd, InputPrice: float64Value(3e-6), OutputPrice: float64Value(1.5e-5)},
			{MinTokens: 200000, InputPrice: float64Value(6e-6), OutputPrice: float64Value(2.25e-5), CacheReadPrice: float64Value(6e-7)},
		}).
		Save(ctx)
	require.NoError(t, err)

	assertBindings := func(name string, bindings []service.ModelCatalogBinding) {
		t.Helper()
		require.Len(t, bindings, 2, name)

		a := bindings[0]
		require.Equal(t, accountA.ID, a.AccountID, name+": sorted by account id")
		require.Equal(t, entry.ID, a.EntryID, name)
		require.Equal(t, 3e-6, a.InputPrice, name)
		require.Equal(t, 1.5e-5, a.OutputPrice, name)
		require.Equal(t, float64Value(3.75e-6), a.CacheWritePrice, name)
		require.Equal(t, float64Value(6e-6), a.CacheWrite1hPrice, name)
		require.Equal(t, float64Value(3e-7), a.CacheReadPrice, name)
		require.False(t, a.CreatedAt.IsZero(), name)
		require.False(t, a.UpdatedAt.IsZero(), name)
		require.Equal(t, []service.PricingInterval{
			{MinTokens: 0, MaxTokens: &segmentEnd, InputPrice: float64Value(3e-6), OutputPrice: float64Value(1.5e-5), SortOrder: 0},
			{MinTokens: 200000, InputPrice: float64Value(6e-6), OutputPrice: float64Value(2.25e-5), CacheReadPrice: float64Value(6e-7), SortOrder: 1},
		}, a.Intervals, name)

		b := bindings[1]
		require.Equal(t, accountB.ID, b.AccountID, name)
		require.Equal(t, 2e-6, b.InputPrice, name)
		require.Equal(t, 8e-6, b.OutputPrice, name)
		require.Nil(t, b.CacheWritePrice, name)
		require.Nil(t, b.CacheWrite1hPrice, name)
		require.Nil(t, b.CacheReadPrice, name)
		require.Empty(t, b.Intervals, name+": no segments")
	}

	bindings, err := repo.ListBindingsByEntry(ctx, entry.ID)
	require.NoError(t, err)
	assertBindings("ListBindingsByEntry", bindings)

	got, err := repo.GetEntryByID(ctx, entry.ID)
	require.NoError(t, err)
	assertBindings("GetEntryByID", got.Bindings)

	all, err := repo.ListEntries(ctx)
	require.NoError(t, err)
	var listed *service.ModelCatalogEntry
	for i := range all {
		if all[i].ID == entry.ID {
			listed = &all[i]
		}
	}
	require.NotNil(t, listed)
	assertBindings("ListEntries", listed.Bindings)

	// ent 的 Account.Delete 是软删除（只写 deleted_at），级联要看真正的行删除。
	_, err = integrationDB.ExecContext(ctx, "DELETE FROM accounts WHERE id = $1", accountB.ID)
	require.NoError(t, err)
	bindings, err = repo.ListBindingsByEntry(ctx, entry.ID)
	require.NoError(t, err)
	require.Len(t, bindings, 1, "deleting the account cascades the binding")
	require.Equal(t, accountA.ID, bindings[0].AccountID)

	outboxCount := func() int {
		var n int
		require.NoError(t, integrationDB.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM scheduler_outbox WHERE event_type = $1 AND payload->'entry_ids' @> $2::jsonb",
			service.SchedulerOutboxEventCatalogBindingsChanged, fmt.Sprintf("[%d]", entry.ID)).Scan(&n))
		return n
	}
	before := outboxCount()
	require.NoError(t, repo.DeleteEntry(ctx, entry.ID))
	require.Equal(t, before+1, outboxCount(), "deleting the entry enqueues the event")
	bindings, err = repo.ListBindingsByEntry(ctx, entry.ID)
	require.NoError(t, err)
	require.Empty(t, bindings, "deleting the entry cascades its bindings")
}

// 种子自带分档时随条目落库：插入写、刷新整份覆盖分档。
func TestModelCatalogRepository_SeedWritesIntervals(t *testing.T) {
	ctx := context.Background()
	repo, unique := newModelCatalogRepoForTest(t, "repo-seed-children")

	seed := service.ModelCatalogEntry{
		ModelID: unique("imagine"), Vendor: "xai", BillingMode: service.BillingModeImage,
		Status: service.ModelCatalogStatusUnlisted, ManagedBy: service.ModelCatalogManagedBySeed,
		PerRequestPrice: float64Value(0.05),
		Intervals: []service.PricingInterval{
			{TierLabel: service.ImageBillingSize1K, PerRequestPrice: float64Value(0.05), SortOrder: 0},
			{TierLabel: service.ImageBillingSize2K, PerRequestPrice: float64Value(0.07), SortOrder: 1},
		},
	}

	first, err := repo.InsertOrRefreshSeedEntries(ctx, []service.ModelCatalogEntry{seed})
	require.NoError(t, err)
	require.Equal(t, 1, first.Inserted)
	require.Zero(t, first.Failed)

	got, err := repo.GetEntryByModelID(ctx, unique("imagine"))
	require.NoError(t, err)
	require.Len(t, got.Intervals, 2)
	require.Equal(t, service.ImageBillingSize1K, got.Intervals[0].TierLabel)
	require.InDelta(t, 0.07, *got.Intervals[1].PerRequestPrice, 1e-12)

	// 重播：分档随种子整份覆盖（改成三档）
	seed.Intervals = append(seed.Intervals, service.PricingInterval{TierLabel: service.ImageBillingSize4K, PerRequestPrice: float64Value(0.10), SortOrder: 2})
	second, err := repo.InsertOrRefreshSeedEntries(ctx, []service.ModelCatalogEntry{seed})
	require.NoError(t, err)
	require.Equal(t, 1, second.Refreshed)
	require.Zero(t, second.Failed)

	again, err := repo.GetEntryByModelID(ctx, unique("imagine"))
	require.NoError(t, err)
	require.Len(t, again.Intervals, 3)
}

// 价格页的两种整块保存：
//   - SaveEntryPricing：同一事务里改官方价、整份覆盖分段与承接关系（删掉不在列表里的渠道），提交后投递这个条目；
//   - ReplaceAccountBindings：整份覆盖一个渠道的承接关系，别的渠道不动，提交后按「原有 ∪ 新」的条目投递；
//   - 事务里任何一行写失败，整块回滚（条目价格、别的承接行都不变）。
func TestModelCatalogRepository_SavePricing(t *testing.T) {
	ctx := context.Background()
	repo, unique := newModelCatalogRepoForTest(t, "repo-pricing")
	client := testEntClient(t)

	newEntry := func(name string) *service.ModelCatalogEntry {
		entry := &service.ModelCatalogEntry{
			ModelID: unique(name), Vendor: "openai", BillingMode: service.BillingModeToken,
			Status: service.ModelCatalogStatusListed, ManagedBy: service.ModelCatalogManagedBySeed,
			InputPrice: float64Value(5e-6), OutputPrice: float64Value(3e-5),
			Intervals: []service.PricingInterval{{MinTokens: 272000, InputPrice: float64Value(1e-5)}},
		}
		require.NoError(t, repo.CreateEntry(ctx, entry))
		t.Cleanup(func() {
			_, _ = integrationDB.ExecContext(context.Background(),
				"DELETE FROM scheduler_outbox WHERE event_type = $1 AND payload->'entry_ids' @> $2::jsonb",
				service.SchedulerOutboxEventCatalogBindingsChanged, fmt.Sprintf("[%d]", entry.ID))
		})
		return entry
	}
	e1 := newEntry("gpt")
	e2 := newEntry("gpt-mini")

	accountA := mustCreateAccount(t, client, &service.Account{Name: unique("a"), Priority: 10})
	accountB := mustCreateAccount(t, client, &service.Account{Name: unique("b"), Priority: 20})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = ANY($1)", pq.Array([]int64{accountA.ID, accountB.ID}))
	})

	outboxCount := func(entryID int64) int {
		var n int
		require.NoError(t, integrationDB.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM scheduler_outbox WHERE event_type = $1 AND payload->'entry_ids' @> $2::jsonb",
			service.SchedulerOutboxEventCatalogBindingsChanged, fmt.Sprintf("[%d]", entryID)).Scan(&n))
		return n
	}
	bindingsOf := func(entryID int64) map[int64]service.ModelCatalogBinding {
		list, err := repo.ListBindingsByEntry(ctx, entryID)
		require.NoError(t, err)
		out := make(map[int64]service.ModelCatalogBinding, len(list))
		for _, b := range list {
			out[b.AccountID] = b
		}
		return out
	}
	segmentEnd := 272000

	// 1. 按模型保存：官方价、分段、两条承接关系一起写。
	before := outboxCount(e1.ID)
	updated := e1.Clone()
	updated.InputPrice = float64Value(4e-6)
	updated.CacheReadPrice = float64Value(4e-7)
	updated.ManagedBy = service.ModelCatalogManagedByAdmin
	updated.Intervals = []service.PricingInterval{{MinTokens: 200000, InputPrice: float64Value(8e-6), SortOrder: 0}}
	updated.SearchPricePerCall = float64Value(0.01)
	updated.XPostPrice = float64Value(0.005)
	updated.XUserPrice = float64Value(0.01)
	require.NoError(t, repo.SaveEntryPricing(ctx, updated, []service.ModelCatalogBinding{
		{AccountID: accountA.ID, InputPrice: 1.2e-7, OutputPrice: 9e-7, CacheReadPrice: float64Value(1.2e-8),
			Intervals: []service.PricingInterval{
				{MinTokens: 0, MaxTokens: &segmentEnd, InputPrice: float64Value(1.2e-7), CacheWritePrice: float64Value(1.5e-7), CacheWrite1hPrice: float64Value(2.4e-7)},
				{MinTokens: 272000, InputPrice: float64Value(3e-7), OutputPrice: float64Value(1.35e-6), CacheReadPrice: float64Value(3e-8)},
			}},
		{AccountID: accountB.ID, UpstreamModel: "gpt-relay", InputPrice: 2e-7, OutputPrice: 1.2e-6, CacheReadPrice: float64Value(2e-8),
			SearchPricePerCall: float64Value(0.008), XPostPrice: float64Value(0.004), XUserPrice: float64Value(0.009)},
	}))
	require.Equal(t, before+1, outboxCount(e1.ID), "saving a model enqueues its entry")

	got, err := repo.GetEntryByID(ctx, e1.ID)
	require.NoError(t, err)
	require.Equal(t, float64Value(4e-6), got.InputPrice)
	require.Equal(t, float64Value(3e-5), got.OutputPrice, "untouched official price kept")
	require.Equal(t, float64Value(4e-7), got.CacheReadPrice)
	require.Equal(t, float64Value(0.01), got.SearchPricePerCall)
	require.Equal(t, float64Value(0.005), got.XPostPrice)
	require.Equal(t, float64Value(0.01), got.XUserPrice)
	require.Equal(t, service.ModelCatalogManagedByAdmin, got.ManagedBy)
	require.Len(t, got.Intervals, 1, "official segments replaced")
	require.Equal(t, 200000, got.Intervals[0].MinTokens)
	b := bindingsOf(e1.ID)
	require.Len(t, b, 2)
	require.Equal(t, 1.2e-7, b[accountA.ID].InputPrice)
	require.Equal(t, float64Value(1.2e-8), b[accountA.ID].CacheReadPrice)
	require.Equal(t, []service.PricingInterval{
		{MinTokens: 0, MaxTokens: &segmentEnd, InputPrice: float64Value(1.2e-7), CacheWritePrice: float64Value(1.5e-7), CacheWrite1hPrice: float64Value(2.4e-7), SortOrder: 0},
		{MinTokens: 272000, InputPrice: float64Value(3e-7), OutputPrice: float64Value(1.35e-6), CacheReadPrice: float64Value(3e-8), SortOrder: 1},
	}, b[accountA.ID].Intervals)
	require.Equal(t, 1.2e-6, b[accountB.ID].OutputPrice)
	require.Equal(t, "gpt-relay", b[accountB.ID].UpstreamModel)
	require.Empty(t, b[accountA.ID].UpstreamModel, "empty = same name as the catalog model")
	require.Equal(t, float64Value(0.008), b[accountB.ID].SearchPricePerCall)
	require.Equal(t, float64Value(0.004), b[accountB.ID].XPostPrice)
	require.Equal(t, float64Value(0.009), b[accountB.ID].XUserPrice)
	require.Nil(t, b[accountA.ID].SearchPricePerCall, "unset = charge at the official search price")

	// 2. 再保存一次只留 B：A 的承接行被删，B 改价。
	require.NoError(t, repo.SaveEntryPricing(ctx, got, []service.ModelCatalogBinding{
		{AccountID: accountB.ID, InputPrice: 2.5e-7, OutputPrice: 1.5e-6, CacheReadPrice: float64Value(2.5e-8)},
	}))
	b = bindingsOf(e1.ID)
	require.Len(t, b, 1, "bindings not in the list are removed")
	require.Equal(t, 2.5e-7, b[accountB.ID].InputPrice)

	// 3. 按渠道保存：B 改成只承接 e2；e1 上 B 的行被删，别的渠道（e1、e2 上的 A）不动。
	require.NoError(t, repo.SaveEntryPricing(ctx, got, []service.ModelCatalogBinding{
		{AccountID: accountA.ID, InputPrice: 1e-7, OutputPrice: 8e-7, CacheReadPrice: float64Value(1e-8)},
		{AccountID: accountB.ID, InputPrice: 2.5e-7, OutputPrice: 1.5e-6, CacheReadPrice: float64Value(2.5e-8)},
	}))
	require.NoError(t, repo.SaveEntryPricing(ctx, e2, []service.ModelCatalogBinding{
		{AccountID: accountA.ID, InputPrice: 4e-8, OutputPrice: 2e-7},
	}))
	beforeE1, beforeE2 := outboxCount(e1.ID), outboxCount(e2.ID)
	require.NoError(t, repo.ReplaceAccountBindings(ctx, accountB.ID, []service.ModelCatalogBinding{
		{EntryID: e2.ID, InputPrice: 5e-8, OutputPrice: 3e-7},
	}))
	b = bindingsOf(e1.ID)
	require.Len(t, b, 1)
	require.Contains(t, b, accountA.ID, "other channels' bindings untouched")
	b2 := bindingsOf(e2.ID)
	require.Len(t, b2, 2, "A stays on e2 when B joins")
	require.Equal(t, 5e-8, b2[accountB.ID].InputPrice)
	require.Equal(t, 4e-8, b2[accountA.ID].InputPrice)
	require.Equal(t, beforeE1+1, outboxCount(e1.ID), "entry the channel left is enqueued")
	require.Equal(t, beforeE2+1, outboxCount(e2.ID), "entry the channel joined is enqueued")

	// 4. 事务回滚：第二行指向不存在的条目，整块不生效。
	err = repo.ReplaceAccountBindings(ctx, accountB.ID, []service.ModelCatalogBinding{
		{EntryID: e1.ID, InputPrice: 1e-7, OutputPrice: 1e-6},
		{EntryID: 1 << 40, InputPrice: 1e-7, OutputPrice: 1e-6},
	})
	require.Error(t, err)
	require.Len(t, bindingsOf(e1.ID), 1, "rolled back: B not added to e1")
	require.Contains(t, bindingsOf(e2.ID), accountB.ID, "rolled back: B still on e2")

	// 5. 事务回滚：承接行指向不存在的渠道，条目改价也不生效。
	broken := got.Clone()
	broken.InputPrice = float64Value(9e-6)
	err = repo.SaveEntryPricing(ctx, broken, []service.ModelCatalogBinding{
		{AccountID: 1 << 40, InputPrice: 1e-7, OutputPrice: 1e-6},
	})
	require.Error(t, err)
	got, err = repo.GetEntryByID(ctx, e1.ID)
	require.NoError(t, err)
	require.Equal(t, float64Value(4e-6), got.InputPrice, "rolled back: official price unchanged")
	require.Len(t, got.Bindings, 1, "rolled back: bindings unchanged")
}

// 售价（muqian 2026-10-06）只在价格页保存时写：播种刷新官方价、编辑模型都不碰它；新建条目默认一项都没定。
func TestModelCatalogRepository_SalePricesSurviveSeedRefreshAndEdits(t *testing.T) {
	ctx := context.Background()
	repo, unique := newModelCatalogRepoForTest(t, "repo-sale")

	entry := &service.ModelCatalogEntry{
		ModelID: unique("gpt"), Vendor: "openai", BillingMode: service.BillingModeToken,
		Status: service.ModelCatalogStatusUnlisted, ManagedBy: service.ModelCatalogManagedBySeed,
		InputPrice: float64Value(5e-6), OutputPrice: float64Value(3e-5),
		Intervals: []service.PricingInterval{{MinTokens: 272000, InputPrice: float64Value(1e-5)}},
	}
	require.NoError(t, repo.CreateEntry(ctx, entry))
	created, err := repo.GetEntryByID(ctx, entry.ID)
	require.NoError(t, err)
	require.True(t, created.SalePrices.IsZero(), "新建条目一项售价都没定")

	sale := service.CatalogSalePrices{
		InputPrice: float64Value(0.5e-6),
		Segments:   []service.CatalogSaleSegment{{MinTokens: 272000, OutputPrice: float64Value(4e-6)}},
		// 售价忙闲时存在同一个 JSONB 里
		TimePricing: &service.TimePricingSpec{Timezone: "Asia/Shanghai", WeekdaysOnly: true,
			Periods:      []service.TimePricingSpecPeriod{{StartTime: "09:00", EndTime: "18:00", Multiplier: 1.5}},
			ExcludeDates: []string{"2026-10-01"}},
	}
	created.SalePrices = sale
	require.NoError(t, repo.SaveEntryPricing(ctx, created, nil))

	// 播种刷新：官方价跟着价格文件变，售价不动
	refreshed, err := repo.InsertOrRefreshSeedEntries(ctx, []service.ModelCatalogEntry{{
		ModelID: unique("gpt"), Vendor: "openai", BillingMode: service.BillingModeToken,
		Status: service.ModelCatalogStatusUnlisted, ManagedBy: service.ModelCatalogManagedBySeed,
		InputPrice: float64Value(6e-6), OutputPrice: float64Value(3e-5),
	}})
	require.NoError(t, err)
	require.Equal(t, 1, refreshed.Refreshed)
	got, err := repo.GetEntryByID(ctx, entry.ID)
	require.NoError(t, err)
	require.InDelta(t, 6e-6, *got.InputPrice, 1e-15)
	require.Equal(t, sale, got.SalePrices)

	// 编辑模型（名称等整条写回）：售价不动
	got.DisplayName = "GPT"
	got.SalePrices = service.CatalogSalePrices{}
	require.NoError(t, repo.UpdateEntry(ctx, got))
	edited, err := repo.GetEntryByID(ctx, entry.ID)
	require.NoError(t, err)
	require.Equal(t, "GPT", edited.DisplayName)
	require.Equal(t, sale, edited.SalePrices)

	// 列表快照也带上售价（计费从快照读）
	all, err := repo.ListEntries(ctx)
	require.NoError(t, err)
	for i := range all {
		if all[i].ID == entry.ID {
			require.Equal(t, sale, all[i].SalePrices)
			return
		}
	}
	t.Fatal("entry missing from ListEntries")
}

// 承接上的上游忙闲时（muqian 2026-10-06）：按模型保存、按渠道保存都写进 time_pricing，三条读路径都带回；
// 不分忙闲时存 NULL。
func TestModelCatalogRepository_BindingTimePricingRoundTrips(t *testing.T) {
	ctx := context.Background()
	repo, unique := newModelCatalogRepoForTest(t, "repo-peak")
	client := testEntClient(t)

	entry := &service.ModelCatalogEntry{
		ModelID: unique("deepseek"), Vendor: "deepseek", BillingMode: service.BillingModeToken,
		Status: service.ModelCatalogStatusUnlisted, ManagedBy: service.ModelCatalogManagedByAdmin,
		InputPrice: float64Value(1e-6), OutputPrice: float64Value(2e-6),
	}
	require.NoError(t, repo.CreateEntry(ctx, entry))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(),
			"DELETE FROM scheduler_outbox WHERE event_type = $1 AND payload->'entry_ids' @> $2::jsonb",
			service.SchedulerOutboxEventCatalogBindingsChanged, fmt.Sprintf("[%d]", entry.ID))
	})
	accountA := mustCreateAccount(t, client, &service.Account{Name: unique("a")})
	accountB := mustCreateAccount(t, client, &service.Account{Name: unique("b")})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = ANY($1)", pq.Array([]int64{accountA.ID, accountB.ID}))
	})

	peak := service.TimePricing{Timezone: "Asia/Shanghai", WeekdaysOnly: true, Periods: []service.TimePricingPeriod{
		{StartTime: "09:00", EndTime: "12:00", Multiplier: 2},
		{StartTime: "14:00", EndTime: "18:00", Multiplier: 2},
	}, ExcludeDates: []string{"2026-10-01"}}
	require.NoError(t, repo.SaveEntryPricing(ctx, entry, []service.ModelCatalogBinding{
		{AccountID: accountA.ID, InputPrice: 0.5e-6, OutputPrice: 1e-6, TimePricing: &peak},
		{AccountID: accountB.ID, InputPrice: 0.5e-6, OutputPrice: 1e-6},
	}))

	assertPeak := func(name string, bindings []service.ModelCatalogBinding) {
		t.Helper()
		require.Len(t, bindings, 2, name)
		require.Equal(t, accountA.ID, bindings[0].AccountID, name)
		require.Equal(t, &peak, bindings[0].TimePricing, name)
		require.Nil(t, bindings[1].TimePricing, name)
	}
	bindings, err := repo.ListBindingsByEntry(ctx, entry.ID)
	require.NoError(t, err)
	assertPeak("ListBindingsByEntry", bindings)
	got, err := repo.GetEntryByID(ctx, entry.ID)
	require.NoError(t, err)
	assertPeak("GetEntryByID", got.Bindings)
	all, err := repo.ListEntries(ctx)
	require.NoError(t, err)
	found := false
	for i := range all {
		if all[i].ID == entry.ID {
			assertPeak("ListEntries", all[i].Bindings)
			found = true
		}
	}
	require.True(t, found, "entry missing from ListEntries")

	var nulls int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM model_catalog_bindings WHERE entry_id = $1 AND time_pricing IS NULL", entry.ID).Scan(&nulls))
	require.Equal(t, 1, nulls, "不分忙闲时存 NULL")

	// 按渠道保存：A 改成不分忙闲时
	require.NoError(t, repo.ReplaceAccountBindings(ctx, accountA.ID, []service.ModelCatalogBinding{
		{EntryID: entry.ID, InputPrice: 0.5e-6, OutputPrice: 1e-6},
	}))
	bindings, err = repo.ListBindingsByEntry(ctx, entry.ID)
	require.NoError(t, err)
	require.Nil(t, bindings[0].TimePricing)
}

// 最高推理倍率三套（muqian 2026-10-07）：官方写条目列、售价写 sale_prices、上游写承接列；没填存 NULL / 不写键。
func TestModelCatalogRepository_MaxReasoningMultipliersRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo, unique := newModelCatalogRepoForTest(t, "repo-max-effort")
	client := testEntClient(t)

	entry := &service.ModelCatalogEntry{
		ModelID: unique("fable"), Vendor: "anthropic", BillingMode: service.BillingModeToken,
		Status: service.ModelCatalogStatusUnlisted, ManagedBy: service.ModelCatalogManagedByAdmin,
		InputPrice: float64Value(10e-6), OutputPrice: float64Value(50e-6),
	}
	require.NoError(t, repo.CreateEntry(ctx, entry))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(),
			"DELETE FROM scheduler_outbox WHERE event_type = $1 AND payload->'entry_ids' @> $2::jsonb",
			service.SchedulerOutboxEventCatalogBindingsChanged, fmt.Sprintf("[%d]", entry.ID))
	})
	accountA := mustCreateAccount(t, client, &service.Account{Name: unique("a")})
	accountB := mustCreateAccount(t, client, &service.Account{Name: unique("b")})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = ANY($1)", pq.Array([]int64{accountA.ID, accountB.ID}))
	})

	entry.MaxReasoningEffortMultiplier = float64Value(3)
	entry.SalePrices = service.CatalogSalePrices{MaxReasoningEffortMultiplier: float64Value(1)}
	require.NoError(t, repo.SaveEntryPricing(ctx, entry, []service.ModelCatalogBinding{
		{AccountID: accountA.ID, InputPrice: 0.5e-6, OutputPrice: 1e-6, MaxReasoningEffortMultiplier: float64Value(2)},
		{AccountID: accountB.ID, InputPrice: 0.5e-6, OutputPrice: 1e-6},
	}))

	got, err := repo.GetEntryByID(ctx, entry.ID)
	require.NoError(t, err)
	require.Equal(t, float64Value(3), got.MaxReasoningEffortMultiplier)
	require.Equal(t, float64Value(1), got.SalePrices.MaxReasoningEffortMultiplier)
	require.Len(t, got.Bindings, 2)
	require.Equal(t, accountA.ID, got.Bindings[0].AccountID)
	require.Equal(t, float64Value(2), got.Bindings[0].MaxReasoningEffortMultiplier)
	require.Nil(t, got.Bindings[1].MaxReasoningEffortMultiplier)

	var nulls int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM model_catalog_bindings WHERE entry_id = $1 AND max_reasoning_effort_multiplier IS NULL", entry.ID).Scan(&nulls))
	require.Equal(t, 1, nulls, "没填存 NULL = 跟官方")

	// 按渠道保存：A 改成跟官方
	require.NoError(t, repo.ReplaceAccountBindings(ctx, accountA.ID, []service.ModelCatalogBinding{
		{EntryID: entry.ID, InputPrice: 0.5e-6, OutputPrice: 1e-6},
	}))
	bindings, err := repo.ListBindingsByEntry(ctx, entry.ID)
	require.NoError(t, err)
	require.Nil(t, bindings[0].MaxReasoningEffortMultiplier)

	_, err = integrationDB.ExecContext(ctx,
		"UPDATE model_catalog_bindings SET max_reasoning_effort_multiplier = 0 WHERE entry_id = $1 AND account_id = $2", entry.ID, accountB.ID)
	require.ErrorContains(t, err, "chk_model_catalog_bindings_max_reasoning_positive", "库里也挡住 <= 0")
}

// 播种条目带忙闲时（价格文件的 time_pricing，如 DeepSeek 高峰 × 2）：插入与刷新都写进目录。
func TestModelCatalogRepository_SeedWritesTimePricing(t *testing.T) {
	ctx := context.Background()
	repo, unique := newModelCatalogRepoForTest(t, "repo-seed-peak")
	peak := &service.TimePricing{Timezone: "Asia/Shanghai", WeekdaysOnly: true, Periods: []service.TimePricingPeriod{
		{StartTime: "09:00", EndTime: "12:00", Multiplier: 2},
	}, ExcludeDates: []string{"2026-10-01", "2026-10-02"}}
	seed := func(tp *service.TimePricing) {
		_, err := repo.InsertOrRefreshSeedEntries(ctx, []service.ModelCatalogEntry{{
			ModelID: unique("deepseek"), BillingMode: service.BillingModeToken,
			Status: service.ModelCatalogStatusUnlisted, ManagedBy: service.ModelCatalogManagedBySeed,
			InputPrice: float64Value(1e-6), TimePricing: tp,
		}})
		require.NoError(t, err)
	}
	load := func() *service.ModelCatalogEntry {
		got, err := repo.GetEntryByModelID(ctx, unique("deepseek"))
		require.NoError(t, err)
		return got
	}

	seed(peak)
	require.Equal(t, peak, load().TimePricing, "insert writes the seed's time pricing")

	changed := &service.TimePricing{Timezone: "Asia/Shanghai", WeekdaysOnly: true, Periods: []service.TimePricingPeriod{
		{StartTime: "14:00", EndTime: "18:00", Multiplier: 2},
	}}
	seed(changed)
	require.Equal(t, changed, load().TimePricing, "refresh follows the price file")
}

// 一键上架只改上架状态（muqian 2026-10-07）：归属不变；播种刷新官方价时保留上架状态，不把它冲回未上架。
func TestModelCatalogRepository_SetEntriesStatusSurvivesSeedRefresh(t *testing.T) {
	ctx := context.Background()
	repo, unique := newModelCatalogRepoForTest(t, "repo-status")

	seedEntry := func(input float64) service.ModelCatalogEntry {
		return service.ModelCatalogEntry{
			ModelID: unique("gpt"), Vendor: "openai", BillingMode: service.BillingModeToken,
			Status: service.ModelCatalogStatusUnlisted, ManagedBy: service.ModelCatalogManagedBySeed,
			InputPrice: float64Value(input), OutputPrice: float64Value(3e-5),
		}
	}
	inserted, err := repo.InsertOrRefreshSeedEntries(ctx, []service.ModelCatalogEntry{seedEntry(5e-6)})
	require.NoError(t, err)
	require.Equal(t, 1, inserted.Inserted)
	created, err := repo.GetEntryByModelID(ctx, unique("gpt"))
	require.NoError(t, err)

	require.NoError(t, repo.SetEntriesStatus(ctx, []int64{created.ID}, service.ModelCatalogStatusListed))
	listed, err := repo.GetEntryByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, service.ModelCatalogStatusListed, listed.Status)
	require.Equal(t, service.ModelCatalogManagedBySeed, listed.ManagedBy, "只改上架状态，不改归属")

	refreshed, err := repo.InsertOrRefreshSeedEntries(ctx, []service.ModelCatalogEntry{seedEntry(6e-6)})
	require.NoError(t, err)
	require.Equal(t, 1, refreshed.Refreshed)
	got, err := repo.GetEntryByID(ctx, created.ID)
	require.NoError(t, err)
	require.InDelta(t, 6e-6, *got.InputPrice, 1e-15, "官方价跟着价格文件刷新")
	require.Equal(t, service.ModelCatalogStatusListed, got.Status, "刷新不把上架冲回未上架")
}
