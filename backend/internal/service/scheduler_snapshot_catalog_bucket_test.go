//go:build unit

package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// catalogBucketCache 记录每个桶的发布次数；快照永远未命中，走 DB 回填。
type catalogBucketCache struct {
	SchedulerCache

	mu         sync.Mutex
	registered []SchedulerBucket
	epoch      int64
	published  map[SchedulerBucket]int
	writes     map[SchedulerBucket][]Account
}

func newCatalogBucketCache(buckets ...SchedulerBucket) *catalogBucketCache {
	return &catalogBucketCache{
		registered: append([]SchedulerBucket(nil), buckets...),
		published:  make(map[SchedulerBucket]int),
		writes:     make(map[SchedulerBucket][]Account),
	}
}

// ListBuckets 照真实仓储的做法：注册表存的是桶键字符串，解析不回来的成员被丢弃。
func (c *catalogBucketCache) ListBuckets(context.Context) ([]SchedulerBucket, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]SchedulerBucket, 0, len(c.registered))
	for _, bucket := range c.registered {
		if parsed, ok := ParseSchedulerBucket(bucket.String()); ok {
			out = append(out, parsed)
		}
	}
	return out, nil
}

func (c *catalogBucketCache) GetSnapshot(context.Context, SchedulerBucket) ([]*Account, bool, error) {
	return nil, false, nil
}

func (c *catalogBucketCache) CaptureBucketWriteToken(_ context.Context, bucket SchedulerBucket) (SchedulerBucketWriteToken, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.epoch++
	return SchedulerBucketWriteToken{Bucket: bucket, Epoch: c.epoch}, nil
}

func (c *catalogBucketCache) SetSnapshot(_ context.Context, bucket SchedulerBucket, _ SchedulerBucketWriteToken, accounts []Account) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.published[bucket]++
	c.writes[bucket] = append([]Account(nil), accounts...)
	return nil
}

func (c *catalogBucketCache) TryLockBucket(context.Context, SchedulerBucket, time.Duration) (bool, error) {
	return true, nil
}

func (c *catalogBucketCache) UnlockBucket(context.Context, SchedulerBucket) error { return nil }

func (c *catalogBucketCache) SetAccount(context.Context, *Account) error { return nil }

func (c *catalogBucketCache) DeleteAccount(context.Context, int64) error { return nil }

func (c *catalogBucketCache) publishCount(bucket SchedulerBucket) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.published[bucket]
}

func (c *catalogBucketCache) written() []SchedulerBucket {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]SchedulerBucket, 0, len(c.writes))
	for bucket := range c.writes {
		out = append(out, bucket)
	}
	return out
}

// catalogBucketAccountRepo 记录按条目 / 按平台读候选的调用，并按条目返回预设账号。
type catalogBucketAccountRepo struct {
	AccountRepository

	mu             sync.Mutex
	byEntry        map[int64][]Account
	byPlatform     map[string][]Account
	entryCalls     []int64
	platformCalls  []string
	accounts       map[int64]*Account
	accountsByIDsN int
}

func (r *catalogBucketAccountRepo) ListSchedulingCandidatesByCatalogEntry(_ context.Context, entryID int64) ([]Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entryCalls = append(r.entryCalls, entryID)
	return append([]Account(nil), r.byEntry[entryID]...), nil
}

func (r *catalogBucketAccountRepo) ListSchedulingCandidates(_ context.Context, platforms []string) ([]Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Account, 0)
	for _, platform := range platforms {
		r.platformCalls = append(r.platformCalls, platform)
		out = append(out, r.byPlatform[platform]...)
	}
	return out, nil
}

func (r *catalogBucketAccountRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if account, ok := r.accounts[id]; ok {
		copied := *account
		return &copied, nil
	}
	return nil, ErrAccountNotFound
}

func (r *catalogBucketAccountRepo) catalogCalls() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]int64(nil), r.entryCalls...)
}

func (r *catalogBucketAccountRepo) platformQueryCount(platform string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, called := range r.platformCalls {
		if called == platform {
			count++
		}
	}
	return count
}

func catalogBucket(entryID int64) SchedulerBucket {
	return SchedulerBucket{PoolID: entryID, Mode: SchedulerModeCatalog}
}

func platformPoolBucket(platform string) SchedulerBucket {
	return SchedulerBucket{Platform: platform, Mode: SchedulerModeSingle}
}

func newCatalogBucketService(cache SchedulerCache, accounts AccountRepository) *SchedulerSnapshotService {
	cfg := &config.Config{RunMode: config.RunModeStandard}
	cfg.Gateway.Scheduling.DbFallbackEnabled = true
	return NewSchedulerSnapshotService(cache, nil, accounts, cfg)
}

func TestHandleCatalogBindingsEvent(t *testing.T) {
	newFixture := func(byEntry map[int64][]Account) (*SchedulerSnapshotService, *catalogBucketCache, *catalogBucketAccountRepo) {
		cache := newCatalogBucketCache(catalogBucket(7), catalogBucket(8))
		accounts := &catalogBucketAccountRepo{byEntry: byEntry}
		return newCatalogBucketService(cache, accounts), cache, accounts
	}
	payload := map[string]any{"entry_ids": []any{float64(7), float64(9)}}

	t.Run("rebuilds the registered bucket that still has candidates", func(t *testing.T) {
		svc, cache, accounts := newFixture(map[int64][]Account{
			7: {{ID: 1, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true}},
		})
		require.NoError(t, svc.handleCatalogBindingsEvent(context.Background(), payload))
		require.Equal(t, 1, cache.publishCount(catalogBucket(7)))
		require.NotContains(t, accounts.catalogCalls(), int64(8), "entries outside the payload are untouched")
		require.NotContains(t, accounts.catalogCalls(), int64(9), "unregistered entries need no query")
	})

	t.Run("publishes an empty snapshot when the bindings are gone", func(t *testing.T) {
		svc, cache, accounts := newFixture(map[int64][]Account{})
		require.NoError(t, svc.handleCatalogBindingsEvent(context.Background(), payload))
		require.Equal(t, 1, cache.publishCount(catalogBucket(7)))
		require.Equal(t, []int64{7}, accounts.catalogCalls())
	})

	t.Run("ignores an empty payload", func(t *testing.T) {
		svc, _, accounts := newFixture(map[int64][]Account{})
		require.NoError(t, svc.handleCatalogBindingsEvent(context.Background(), nil))
		require.NoError(t, svc.handleCatalogBindingsEvent(context.Background(), map[string]any{"entry_ids": []any{}}))
		require.Empty(t, accounts.catalogCalls())
	})
}

// 账号变更只重建它绑定的条目的目录桶 + 它自己平台的平台池；账号消失则全部目录桶与全部平台池重建。
func TestHandleAccountEventRebuildsCatalogAndPlatformBuckets(t *testing.T) {
	newFixture := func(accountsByID map[int64]*Account) (*SchedulerSnapshotService, *catalogBucketCache, *catalogBucketAccountRepo) {
		cache := newCatalogBucketCache(catalogBucket(7), catalogBucket(8))
		accounts := &catalogBucketAccountRepo{
			accounts: accountsByID,
			byEntry: map[int64][]Account{
				7: {{ID: 1, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true}},
				8: {{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true}},
			},
			byPlatform: map[string][]Account{},
		}
		return newCatalogBucketService(cache, accounts), cache, accounts
	}

	t.Run("account change rebuilds its entries and its own platform pool", func(t *testing.T) {
		svc, cache, accounts := newFixture(map[int64]*Account{
			1: {ID: 1, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, CatalogEntryIDs: []int64{7}},
		})
		accountID := int64(1)
		require.NoError(t, svc.handleAccountEvent(context.Background(), &accountID, nil, map[batchSeenKey]struct{}{}))
		require.Equal(t, []int64{7}, accounts.catalogCalls())
		require.Equal(t, 1, cache.publishCount(catalogBucket(7)))
		require.Zero(t, cache.publishCount(catalogBucket(8)))
		require.Equal(t, 1, cache.publishCount(platformPoolBucket(PlatformAnthropic)))
		require.Zero(t, cache.publishCount(platformPoolBucket(PlatformOpenAI)), "其它平台池不受影响")
	})

	t.Run("account miss rebuilds every catalog bucket and every platform pool", func(t *testing.T) {
		svc, cache, accounts := newFixture(map[int64]*Account{})
		accountID := int64(1)
		require.NoError(t, svc.handleAccountEvent(context.Background(), &accountID, nil, map[batchSeenKey]struct{}{}))
		require.ElementsMatch(t, []int64{7, 8}, accounts.catalogCalls())
		require.Equal(t, 1, cache.publishCount(catalogBucket(8)))
		for _, platform := range schedulerSnapshotPlatforms() {
			require.Equal(t, 1, cache.publishCount(platformPoolBucket(platform)), platform)
		}
	})
}

// 带 CatalogRoute 的请求读写目录桶（条目 ID），候选来自绑定；无 route 时走平台池。
func TestListSchedulableAccounts_CatalogRouteUsesCatalogBucket(t *testing.T) {
	cache := newCatalogBucketCache()
	accounts := &catalogBucketAccountRepo{
		byEntry: map[int64][]Account{
			7: {
				{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, CatalogEntryIDs: []int64{7}},
				{ID: 2, Platform: PlatformGemini, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, CatalogEntryIDs: []int64{7}},
			},
		},
		byPlatform: map[string][]Account{
			PlatformAnthropic: {{ID: 3, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true}},
		},
	}
	svc := newCatalogBucketService(cache, accounts)

	routed := catalogRouteCtx(7, APIProtocolAnthropic)
	got, err := svc.ListSchedulableAccounts(routed, PlatformAnthropic)
	require.NoError(t, err)
	require.Len(t, got, 2, "message converts to gemini: both subscriptions serve it (no family gate)")
	require.Equal(t, []int64{7}, accounts.catalogCalls())
	require.Equal(t, []SchedulerBucket{catalogBucket(7)}, cache.written())

	got, err = svc.ListSchedulableAccounts(catalogRouteCtx(7, APIProtocolResponses), PlatformAnthropic)
	require.NoError(t, err)
	require.Len(t, got, 1, "no responses → gemini conversion: the gemini oauth is filtered out")
	require.Equal(t, int64(1), got[0].ID)

	// 强制 antigravity（/antigravity 路由）：桶仍是目录桶，但过滤按 antigravity 只留 antigravity 成品号。
	got, err = svc.ListSchedulableAccounts(routed, PlatformAntigravity)
	require.NoError(t, err)
	require.Empty(t, got)

	unrouted := WithInboundProtocol(context.Background(), APIProtocolAnthropic)
	got, err = svc.ListSchedulableAccounts(unrouted, PlatformAnthropic)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, int64(3), got[0].ID, "无路由走 anthropic 平台池")
	require.Equal(t, 1, accounts.platformQueryCount(PlatformAnthropic))
	require.Contains(t, cache.written(), platformPoolBucket(PlatformAnthropic))
}

// 全量重建 = 10 个平台池 + 注册表里的目录桶；注册表里残留的 mixed / forced / 分组桶被解析拒绝，不再重建。
func TestSchedulerFullRebuildIsPlatformPoolsPlusCatalogBuckets(t *testing.T) {
	cache := newCatalogBucketCache(catalogBucket(7), catalogBucket(8))
	cache.registered = append(cache.registered,
		SchedulerBucket{PoolID: 3, Platform: PlatformAnthropic, Mode: "mixed"},
		SchedulerBucket{PoolID: 3, Platform: PlatformAnthropic, Mode: "forced"},
		SchedulerBucket{PoolID: 3, Platform: PlatformAnthropic, Mode: SchedulerModeSingle},
	)
	accounts := &catalogBucketAccountRepo{
		byEntry:    map[int64][]Account{7: {{ID: 1, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true}}},
		byPlatform: map[string][]Account{},
	}
	svc := newCatalogBucketService(cache, accounts)

	require.NoError(t, svc.rebuildFullSnapshot(context.Background(), "test"))

	want := append(schedulerCanonicalBuckets(), catalogBucket(7), catalogBucket(8))
	require.ElementsMatch(t, want, cache.written())
	require.Len(t, schedulerCanonicalBuckets(), 10)
}

// group 事件不再有任何处理：既不重建也不报错（发送端随 7b-3 的分组 repo 删）。
func TestSchedulerSnapshot_GroupEventIsIgnored(t *testing.T) {
	cache := newCatalogBucketCache(catalogBucket(7))
	accounts := &catalogBucketAccountRepo{byEntry: map[int64][]Account{}, byPlatform: map[string][]Account{}}
	svc := newCatalogBucketService(cache, accounts)

	require.NoError(t, svc.handleOutboxEvent(
		context.Background(),
		SchedulerOutboxEvent{EventType: SchedulerOutboxEventGroupChanged},
		map[batchSeenKey]struct{}{},
	))
	require.Empty(t, cache.written())
	require.Empty(t, accounts.catalogCalls())
}
