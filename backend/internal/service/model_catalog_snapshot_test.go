//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 这两个访问器只有本文件（unit 标签）用；放在无标签的 support 文件里会被默认标签下的 lint 判成未使用。
func (r *stubModelCatalogRepo) calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.listCalls
}

func (r *stubModelCatalogRepo) appendEntry(entry ModelCatalogEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, entry)
}

func catalogSnapshotEntry(id int64, modelID string) ModelCatalogEntry {
	entry := catalogEntryFromCard(modelID, ModelCatalogManagedBySeed, PricingCard{InputPrice: float64Ptr(1e-6)})
	entry.ID = id
	return entry
}

// expireSnapshot 把已装好的快照标成过期，模拟 pub/sub 丢失后到达 TTL。
func expireSnapshot(svc *ModelCatalogService) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	svc.snapshot.loadedAt = time.Now().Add(-2 * modelCatalogCacheTTL)
}

// 冷启动时的并发查表只打一次库，其余请求等同一次重建的结果。
func TestModelCatalogSnapshot_ColdStartLoadsOnce(t *testing.T) {
	svc, repo := newTestModelCatalogService(catalogSnapshotEntry(1, "team/a"))
	repo.listGate = make(chan struct{})

	const readers = 8
	results := make([]*ModelCatalogEntry, readers)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = svc.LookupPricingEntry(context.Background(), "team/a")
		}(i)
	}
	require.Eventually(t, func() bool { return repo.calls() == 1 }, time.Second, 5*time.Millisecond)
	close(repo.listGate)
	wg.Wait()

	require.Equal(t, 1, repo.calls())
	for _, got := range results {
		require.NotNil(t, got)
		require.Equal(t, "team/a", got.ModelID)
	}
}

// 快照过期但库很慢时，查表立刻拿陈旧快照，刷新在后台完成；计费热路径不能挂在库上。
func TestModelCatalogSnapshot_StaleServedWhileReloading(t *testing.T) {
	svc, repo := newTestModelCatalogService(catalogSnapshotEntry(1, "team/a"))
	require.NotNil(t, svc.LookupPricingEntry(context.Background(), "team/a"))
	require.Equal(t, 1, repo.calls())

	expireSnapshot(svc)
	repo.appendEntry(catalogSnapshotEntry(2, "team/b"))
	repo.listGate = make(chan struct{})

	done := make(chan *ModelCatalogEntry, 1)
	go func() { done <- svc.LookupPricingEntry(context.Background(), "team/a") }()
	select {
	case got := <-done:
		require.NotNil(t, got, "stale snapshot must be served")
	case <-time.After(time.Second):
		t.Fatal("lookup blocked on the slow reload instead of serving the stale snapshot")
	}
	require.Eventually(t, func() bool { return repo.calls() == 2 }, time.Second, 5*time.Millisecond)

	close(repo.listGate)
	require.Eventually(t, func() bool {
		return svc.LookupPricingEntry(context.Background(), "team/b") != nil
	}, time.Second, 5*time.Millisecond, "background reload must eventually install the fresh snapshot")
}

// 重建失败后进入退避：期间的查表继续用陈旧快照，不再每个请求都打库。
func TestModelCatalogSnapshot_FailedReloadBacksOff(t *testing.T) {
	svc, repo := newTestModelCatalogService(catalogSnapshotEntry(1, "team/a"))
	require.NotNil(t, svc.LookupPricingEntry(context.Background(), "team/a"))
	expireSnapshot(svc)
	repo.mu.Lock()
	repo.listErr = errors.New("db down")
	repo.mu.Unlock()

	require.NotNil(t, svc.LookupPricingEntry(context.Background(), "team/a"), "stale snapshot survives a failed reload")
	require.Eventually(t, func() bool { return repo.calls() == 2 }, time.Second, 5*time.Millisecond)
	// 后台重建已失败并记下退避；等它结束再查，不应再打库。
	require.Eventually(t, func() bool {
		svc.mu.RLock()
		defer svc.mu.RUnlock()
		return svc.reloading == nil && !svc.retryAfter.IsZero()
	}, time.Second, 5*time.Millisecond)
	for i := 0; i < 5; i++ {
		require.NotNil(t, svc.LookupPricingEntry(context.Background(), "team/a"))
	}
	// 没有退避时这些查表会各自起一次后台重建；观察一段时间确认库没再被打。
	require.Never(t, func() bool { return repo.calls() > 2 }, 200*time.Millisecond, 10*time.Millisecond,
		"lookups inside the backoff window must not hit the repository")
}

// 重建用独立 ctx：触发重建的请求已被取消，也不能让所有人都拿不到目录。
func TestModelCatalogSnapshot_ReloadIgnoresCallerCancellation(t *testing.T) {
	svc, repo := newTestModelCatalogService(catalogSnapshotEntry(1, "team/a"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got := svc.LookupPricingEntry(ctx, "team/a")

	require.NotNil(t, got, "reload must not run on the caller's cancelled ctx")
	require.Equal(t, 1, repo.calls())
}

// 重建进行中发生写入：读到的是写入前的数据，结果必须作废并重来，不能把旧数据装回去。
func TestModelCatalogSnapshot_WriteDuringReloadDiscardsStaleResult(t *testing.T) {
	svc, repo := newTestModelCatalogService(catalogSnapshotEntry(1, "team/a"))
	repo.listGate = make(chan struct{})

	done := make(chan *ModelCatalogEntry, 1)
	go func() { done <- svc.LookupPricingEntry(context.Background(), "team/b") }()
	require.Eventually(t, func() bool { return repo.calls() == 1 }, time.Second, 5*time.Millisecond)

	// 重建卡在库读取时来了一次写入。
	repo.appendEntry(catalogSnapshotEntry(2, "team/b"))
	svc.invalidateLocal()
	close(repo.listGate)

	select {
	case got := <-done:
		require.NotNil(t, got, "the reader must see the entry written during the reload")
		require.Equal(t, "team/b", got.ModelID)
	case <-time.After(time.Second):
		t.Fatal("lookup did not finish")
	}
	require.Equal(t, 2, repo.calls(), "the pre-write result must be discarded and reloaded")
}
