//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

// catalogBucketCache 在生命周期测试缓存上补齐账号事件路径要用的账号缓存方法。
type catalogBucketCache struct {
	*fullRebuildLifecycleCache
}

func newCatalogBucketCache(buckets ...SchedulerBucket) *catalogBucketCache {
	return &catalogBucketCache{fullRebuildLifecycleCache: newFullRebuildLifecycleCache(buckets...)}
}

func (c *catalogBucketCache) SetAccount(context.Context, *Account) error { return nil }

func (c *catalogBucketCache) DeleteAccount(context.Context, int64) error { return nil }

// catalogBucketAccountRepo 记录按条目读候选的调用，并按条目返回预设账号。
type catalogBucketAccountRepo struct {
	fullRebuildAccountRepo

	mu         sync.Mutex
	byEntry    map[int64][]Account
	entryCalls []int64
	accounts   map[int64]*Account
}

func (r *catalogBucketAccountRepo) ListSchedulingCandidatesByCatalogEntry(_ context.Context, entryID int64) ([]Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entryCalls = append(r.entryCalls, entryID)
	return append([]Account(nil), r.byEntry[entryID]...), nil
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

func catalogBucket(entryID int64, platform string) SchedulerBucket {
	return SchedulerBucket{GroupID: entryID, Platform: platform, Mode: SchedulerModeCatalog}
}

// 目录桶与分组桶共用数字 ID：分组 7 被删掉时它的桶退役，条目 7 的目录桶必须原地重建。
func TestSchedulerFullRebuildKeepsCatalogBucketWhenSameIDGroupIsRetired(t *testing.T) {
	const id int64 = 7
	groupBucket := SchedulerBucket{GroupID: id, Platform: PlatformAnthropic, Mode: SchedulerModeSingle}
	entryBucket := catalogBucket(id, PlatformAnthropic)
	cache := newCatalogBucketCache(groupBucket, entryBucket)
	groups := &fullRebuildLifecycleGroupRepo{fresh: make(map[int64]*Group), freshErr: make(map[int64]error)}
	accounts := &catalogBucketAccountRepo{byEntry: map[int64][]Account{
		id: {{ID: 1, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true}},
	}}
	svc := newFullRebuildLifecycleService(cache, nil, accounts, groups, config.RunModeStandard)

	require.NoError(t, svc.rebuildFullSnapshot(context.Background(), "test"))

	retired := bucketStrings(cache.retiredBuckets())
	require.Contains(t, retired, groupBucket.String(), "the deleted group's bucket is retired")
	require.NotContains(t, retired, entryBucket.String(), "the catalog bucket with the same id survives")
	_, published := cache.counts(entryBucket)
	require.Equal(t, 1, published, "the catalog bucket is rebuilt in place")
	require.Equal(t, []int64{id}, accounts.catalogCalls())
	require.Zero(t, accounts.groupCallCount(id), "no group query is issued for the catalog bucket")
}

// simple 模式把分组 ID 归零，但目录桶的 ID 是条目 ID，不能被归零。
func TestSchedulerFullRebuildSimpleModeLoadsCatalogBucketByEntryID(t *testing.T) {
	entryBucket := catalogBucket(42, PlatformOpenAI)
	cache := newCatalogBucketCache(entryBucket)
	groups := &fullRebuildLifecycleGroupRepo{
		activeIDsErr: errors.New("simple mode must not query groups"),
		fresh:        make(map[int64]*Group), freshErr: make(map[int64]error),
	}
	accounts := &catalogBucketAccountRepo{byEntry: map[int64][]Account{
		42: {{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true}},
	}}
	svc := newFullRebuildLifecycleService(cache, nil, accounts, groups, config.RunModeSimple)

	require.NoError(t, svc.rebuildFullSnapshot(context.Background(), "test"))

	require.Equal(t, []int64{42}, accounts.catalogCalls())
	_, published := cache.counts(entryBucket)
	require.Equal(t, 1, published)
	require.Empty(t, cache.retiredBuckets())
}

func TestHandleCatalogBindingsEvent(t *testing.T) {
	newFixture := func(byEntry map[int64][]Account) (*SchedulerSnapshotService, *catalogBucketCache, *catalogBucketAccountRepo) {
		cache := newCatalogBucketCache(catalogBucket(7, PlatformAnthropic), catalogBucket(8, PlatformOpenAI))
		accounts := &catalogBucketAccountRepo{byEntry: byEntry}
		groups := &fullRebuildLifecycleGroupRepo{fresh: make(map[int64]*Group), freshErr: make(map[int64]error)}
		return newFullRebuildLifecycleService(cache, nil, accounts, groups, config.RunModeStandard), cache, accounts
	}
	payload := map[string]any{"entry_ids": []any{float64(7), float64(9)}}

	t.Run("rebuilds the registered bucket that still has candidates", func(t *testing.T) {
		svc, cache, accounts := newFixture(map[int64][]Account{
			7: {{ID: 1, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true}},
		})
		require.NoError(t, svc.handleCatalogBindingsEvent(context.Background(), payload))
		_, published := cache.counts(catalogBucket(7, PlatformAnthropic))
		require.Equal(t, 1, published)
		require.Empty(t, cache.retiredBuckets())
		require.NotContains(t, accounts.catalogCalls(), int64(8), "entries outside the payload are untouched")
		require.NotContains(t, accounts.catalogCalls(), int64(9), "unregistered entries need no query")
	})

	t.Run("publishes an empty snapshot when the bindings are gone instead of retiring", func(t *testing.T) {
		svc, cache, accounts := newFixture(map[int64][]Account{})
		require.NoError(t, svc.handleCatalogBindingsEvent(context.Background(), payload))
		require.Empty(t, cache.retiredBuckets())
		_, published := cache.counts(catalogBucket(7, PlatformAnthropic))
		require.Equal(t, 1, published)
		require.Equal(t, []int64{7}, accounts.catalogCalls())
	})

	t.Run("ignores an empty payload", func(t *testing.T) {
		svc, _, accounts := newFixture(map[int64][]Account{})
		require.NoError(t, svc.handleCatalogBindingsEvent(context.Background(), nil))
		require.NoError(t, svc.handleCatalogBindingsEvent(context.Background(), map[string]any{"entry_ids": []any{}}))
		require.Empty(t, accounts.catalogCalls())
	})
}

// 账号变更只重建它绑定的条目的目录桶；账号消失则全部目录桶重建。
func TestHandleAccountEventRebuildsCatalogBuckets(t *testing.T) {
	newFixture := func(accountsByID map[int64]*Account) (*SchedulerSnapshotService, *catalogBucketCache, *catalogBucketAccountRepo) {
		cache := newCatalogBucketCache(catalogBucket(7, PlatformAnthropic), catalogBucket(8, PlatformOpenAI))
		accounts := &catalogBucketAccountRepo{
			accounts: accountsByID,
			byEntry: map[int64][]Account{
				7: {{ID: 1, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true}},
				8: {{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true}},
			},
		}
		groups := &fullRebuildLifecycleGroupRepo{fresh: make(map[int64]*Group), freshErr: make(map[int64]error)}
		return newFullRebuildLifecycleService(cache, nil, accounts, groups, config.RunModeStandard), cache, accounts
	}

	t.Run("account change rebuilds only its entries", func(t *testing.T) {
		svc, cache, accounts := newFixture(map[int64]*Account{
			1: {ID: 1, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, CatalogEntryIDs: []int64{7}},
		})
		accountID := int64(1)
		require.NoError(t, svc.handleAccountEvent(context.Background(), &accountID, nil, map[batchSeenKey]struct{}{}))
		require.Equal(t, []int64{7}, accounts.catalogCalls())
		_, published := cache.counts(catalogBucket(7, PlatformAnthropic))
		require.Equal(t, 1, published)
		_, published = cache.counts(catalogBucket(8, PlatformOpenAI))
		require.Zero(t, published)
	})

	t.Run("account miss rebuilds every catalog bucket", func(t *testing.T) {
		svc, cache, accounts := newFixture(map[int64]*Account{})
		accountID := int64(1)
		require.NoError(t, svc.handleAccountEvent(context.Background(), &accountID, nil, map[batchSeenKey]struct{}{}))
		require.ElementsMatch(t, []int64{7, 8}, accounts.catalogCalls())
		_, published := cache.counts(catalogBucket(8, PlatformOpenAI))
		require.Equal(t, 1, published)
	})
}

// catalogListCache 只实现 ListSchedulableAccounts 用到的读写：快照永远未命中，记录写入的桶。
type catalogListCache struct {
	SchedulerCache

	mu     sync.Mutex
	epoch  int64
	writes map[SchedulerBucket][]Account
}

func newCatalogListCache() *catalogListCache {
	return &catalogListCache{writes: make(map[SchedulerBucket][]Account)}
}

func (c *catalogListCache) GetSnapshot(context.Context, SchedulerBucket) ([]*Account, bool, error) {
	return nil, false, nil
}

func (c *catalogListCache) CaptureBucketWriteToken(_ context.Context, bucket SchedulerBucket) (SchedulerBucketWriteToken, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.epoch++
	return SchedulerBucketWriteToken{Bucket: bucket, Epoch: c.epoch}, nil
}

func (c *catalogListCache) SetSnapshot(_ context.Context, bucket SchedulerBucket, _ SchedulerBucketWriteToken, accounts []Account) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writes[bucket] = append([]Account(nil), accounts...)
	return nil
}

func (c *catalogListCache) written() []SchedulerBucket {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]SchedulerBucket, 0, len(c.writes))
	for bucket := range c.writes {
		out = append(out, bucket)
	}
	return out
}

// 带 CatalogRoute 的请求读写目录桶（条目 ID + 条目网关族），候选来自绑定；
// 同一服务无 route 时仍走分组桶与分组查询。
func TestListSchedulableAccounts_CatalogRouteUsesCatalogBucket(t *testing.T) {
	groupID := int64(3)
	cache := newCatalogListCache()
	accounts := &catalogBucketAccountRepo{byEntry: map[int64][]Account{
		7: {
			{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, CatalogEntryIDs: []int64{7}},
			{ID: 2, Platform: PlatformGemini, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, CatalogEntryIDs: []int64{7}},
		},
	}}
	groups := &fullRebuildLifecycleGroupRepo{fresh: make(map[int64]*Group), freshErr: make(map[int64]error)}
	cfg := &config.Config{RunMode: config.RunModeStandard}
	cfg.Gateway.Scheduling.DbFallbackEnabled = true
	svc := NewSchedulerSnapshotService(cache, nil, accounts, groups, cfg)

	routed := catalogRouteCtx(7, PlatformAnthropic, APIProtocolAnthropic)
	got, useMixed, err := svc.ListSchedulableAccounts(routed, &groupID, PlatformAnthropic, false)
	require.NoError(t, err)
	require.False(t, useMixed, "catalog buckets never mix")
	require.Len(t, got, 1, "the gemini oauth cannot serve the anthropic family and is filtered out")
	require.Equal(t, int64(1), got[0].ID)
	require.Equal(t, []int64{7}, accounts.catalogCalls())
	require.Equal(t, []SchedulerBucket{catalogBucket(7, PlatformAnthropic)}, cache.written())

	// 强制 antigravity（/antigravity 路由）：桶仍是目录桶，但过滤按 antigravity 只留 antigravity 成品号。
	forced := context.WithValue(routed, ctxkey.ForcePlatform, PlatformAntigravity)
	got, _, err = svc.ListSchedulableAccounts(forced, &groupID, PlatformAntigravity, true)
	require.NoError(t, err)
	require.Empty(t, got)

	unrouted := WithInboundProtocol(context.Background(), APIProtocolAnthropic)
	_, useMixed, err = svc.ListSchedulableAccounts(unrouted, &groupID, PlatformAnthropic, false)
	require.NoError(t, err)
	require.True(t, useMixed)
	require.Equal(t, 1, accounts.groupCallCount(groupID), "without a route the group query runs")
	require.Contains(t, cache.written(), SchedulerBucket{GroupID: groupID, Platform: PlatformAnthropic, Mode: SchedulerModeMixed})
}
