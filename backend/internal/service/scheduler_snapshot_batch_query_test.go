//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// 一个平台一个池桶之后，同一批重建里不再有两个桶共用一次查询；本文件盯住的是重建批次的策略：
// 所有 token 必须在第一次 DB 查询前备好、锁忙 / 栅栏对普通与严格任务的不同处理、查询失败不影响后续桶。

type batchAccountQueryRepo struct {
	AccountRepository

	mu        sync.Mutex
	calls     map[string]int
	results   map[string][]batchAccountQueryResult
	beforeRun func(platform string)
}

type batchAccountQueryResult struct {
	accounts []Account
	err      error
}

func newBatchAccountQueryRepo() *batchAccountQueryRepo {
	return &batchAccountQueryRepo{
		calls:   make(map[string]int),
		results: make(map[string][]batchAccountQueryResult),
	}
}

func (r *batchAccountQueryRepo) ListSchedulingCandidatesByCatalogEntry(context.Context, int64) ([]Account, error) {
	return nil, nil
}

func (r *batchAccountQueryRepo) ListModelAvailabilityCandidates(context.Context, []string) ([]Account, error) {
	panic("unexpected ListModelAvailabilityCandidates call")
}

func (r *batchAccountQueryRepo) ListSchedulingCandidates(_ context.Context, platforms []string) ([]Account, error) {
	platform := ""
	if len(platforms) > 0 {
		platform = platforms[0]
	}
	r.mu.Lock()
	r.calls[platform]++
	call := r.calls[platform]
	results := r.results[platform]
	beforeRun := r.beforeRun
	r.mu.Unlock()

	if beforeRun != nil {
		beforeRun(platform)
	}
	if call <= len(results) {
		result := results[call-1]
		return append([]Account(nil), result.accounts...), result.err
	}
	return []Account{{
		ID:          int64(call),
		Name:        "source",
		Platform:    platform,
		Status:      StatusActive,
		Schedulable: true,
	}}, nil
}

func (r *batchAccountQueryRepo) callCount(platform string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls[platform]
}

type batchSnapshotWrite struct {
	token    SchedulerBucketWriteToken
	accounts []Account
}

type batchSnapshotCache struct {
	SchedulerCache

	mu          sync.Mutex
	nextEpoch   int64
	captures    []SchedulerBucket
	captured    map[SchedulerBucket]SchedulerBucketWriteToken
	locks       map[SchedulerBucket]int
	lockBusy    map[SchedulerBucket]bool
	lockErrors  map[SchedulerBucket]error
	setErrors   map[SchedulerBucket]error
	setAttempts map[SchedulerBucket]int
	writes      map[SchedulerBucket][]batchSnapshotWrite
	versions    map[SchedulerBucket]int
}

func newBatchSnapshotCache() *batchSnapshotCache {
	return &batchSnapshotCache{
		captured:    make(map[SchedulerBucket]SchedulerBucketWriteToken),
		locks:       make(map[SchedulerBucket]int),
		lockBusy:    make(map[SchedulerBucket]bool),
		lockErrors:  make(map[SchedulerBucket]error),
		setErrors:   make(map[SchedulerBucket]error),
		setAttempts: make(map[SchedulerBucket]int),
		writes:      make(map[SchedulerBucket][]batchSnapshotWrite),
		versions:    make(map[SchedulerBucket]int),
	}
}

func (c *batchSnapshotCache) CaptureBucketWriteToken(_ context.Context, bucket SchedulerBucket) (SchedulerBucketWriteToken, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextEpoch++
	token := SchedulerBucketWriteToken{Bucket: bucket, Epoch: c.nextEpoch}
	c.captures = append(c.captures, bucket)
	c.captured[bucket] = token
	return token, nil
}

func (c *batchSnapshotCache) TryLockBucket(_ context.Context, bucket SchedulerBucket, _ time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.locks[bucket]++
	if err := c.lockErrors[bucket]; err != nil {
		return false, err
	}
	return !c.lockBusy[bucket], nil
}

func (c *batchSnapshotCache) UnlockBucket(context.Context, SchedulerBucket) error { return nil }

func (c *batchSnapshotCache) SetSnapshot(_ context.Context, bucket SchedulerBucket, token SchedulerBucketWriteToken, accounts []Account) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setAttempts[bucket]++
	if token != c.captured[bucket] || !token.ValidFor(bucket) {
		return ErrSchedulerBucketWriteFenced
	}
	if err := c.setErrors[bucket]; err != nil {
		return err
	}
	c.versions[bucket]++
	c.writes[bucket] = append(c.writes[bucket], batchSnapshotWrite{
		token:    token,
		accounts: append([]Account(nil), accounts...),
	})
	return nil
}

func (c *batchSnapshotCache) captureCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.captures)
}

func (c *batchSnapshotCache) bucketState(bucket SchedulerBucket) (locks, attempts, version int, writes []batchSnapshotWrite) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.locks[bucket], c.setAttempts[bucket], c.versions[bucket], append([]batchSnapshotWrite(nil), c.writes[bucket]...)
}

func newBatchQueryTestService(cache SchedulerCache, accounts AccountRepository) *SchedulerSnapshotService {
	return NewSchedulerSnapshotService(cache, nil, accounts, &config.Config{RunMode: config.RunModeStandard})
}

// 每个平台池独立查询与发布；所有 token 在第一次 DB 查询前就位。
func TestSchedulerRebuildBatchCapturesAllTokensBeforeFirstQuery(t *testing.T) {
	openai := platformPoolBucket(PlatformOpenAI)
	anthropic := platformPoolBucket(PlatformAnthropic)
	cache := newBatchSnapshotCache()
	repo := newBatchAccountQueryRepo()
	repo.beforeRun = func(string) {
		require.Equal(t, 2, cache.captureCount(), "all tokens must be prepared before the first DB query")
	}
	svc := newBatchQueryTestService(cache, repo)

	require.NoError(t, svc.rebuildBuckets(context.Background(), []SchedulerBucket{openai, anthropic}, "first"))
	require.Equal(t, 1, repo.callCount(PlatformOpenAI))
	require.Equal(t, 1, repo.callCount(PlatformAnthropic))
	for _, bucket := range []SchedulerBucket{openai, anthropic} {
		locks, attempts, version, writes := cache.bucketState(bucket)
		require.Equal(t, 1, locks, bucket.String())
		require.Equal(t, 1, attempts, bucket.String())
		require.Equal(t, 1, version, bucket.String())
		require.Len(t, writes, 1, bucket.String())
		require.Equal(t, bucket.Platform, writes[0].accounts[0].Platform, bucket.String())
		require.Equal(t, bucket, writes[0].token.Bucket)
	}

	repo.beforeRun = nil
	require.NoError(t, svc.rebuildBuckets(context.Background(), []SchedulerBucket{openai, anthropic}, "second"))
	require.Equal(t, 2, repo.callCount(PlatformOpenAI), "每轮重建都重新查库")
}

// 查询失败只影响它自己的桶，后续桶照常重建。
func TestSchedulerRebuildBatchQueryFailureDoesNotBlockNextBucket(t *testing.T) {
	openai := platformPoolBucket(PlatformOpenAI)
	anthropic := platformPoolBucket(PlatformAnthropic)
	wantErr := errors.New("query failed")
	cache := newBatchSnapshotCache()
	repo := newBatchAccountQueryRepo()
	repo.results[PlatformOpenAI] = []batchAccountQueryResult{{err: wantErr}}
	svc := newBatchQueryTestService(cache, repo)

	err := svc.rebuildBuckets(context.Background(), []SchedulerBucket{openai, anthropic}, "test")
	require.ErrorIs(t, err, wantErr)
	_, openaiAttempts, _, _ := cache.bucketState(openai)
	require.Zero(t, openaiAttempts)
	_, anthropicAttempts, anthropicVersion, _ := cache.bucketState(anthropic)
	require.Equal(t, 1, anthropicAttempts)
	require.Equal(t, 1, anthropicVersion)
}

func TestSchedulerRebuildBatchPreservesLockBusyAndFencingPolicy(t *testing.T) {
	openai := platformPoolBucket(PlatformOpenAI)
	anthropic := platformPoolBucket(PlatformAnthropic)

	t.Run("lock busy skips only that bucket", func(t *testing.T) {
		cache := newBatchSnapshotCache()
		cache.lockBusy[openai] = true
		repo := newBatchAccountQueryRepo()
		svc := newBatchQueryTestService(cache, repo)

		require.NoError(t, svc.rebuildBuckets(context.Background(), []SchedulerBucket{openai, anthropic}, "test"))
		require.Zero(t, repo.callCount(PlatformOpenAI), "锁忙的桶不查库")
		_, openaiAttempts, _, _ := cache.bucketState(openai)
		_, anthropicAttempts, anthropicVersion, _ := cache.bucketState(anthropic)
		require.Zero(t, openaiAttempts)
		require.Equal(t, 1, anthropicAttempts)
		require.Equal(t, 1, anthropicVersion)
	})

	t.Run("lock error is returned while other buckets continue", func(t *testing.T) {
		wantErr := errors.New("lock failed")
		cache := newBatchSnapshotCache()
		cache.lockErrors[openai] = wantErr
		repo := newBatchAccountQueryRepo()
		svc := newBatchQueryTestService(cache, repo)

		err := svc.rebuildBuckets(context.Background(), []SchedulerBucket{openai, anthropic}, "test")
		require.ErrorIs(t, err, wantErr)
		_, anthropicAttempts, anthropicVersion, _ := cache.bucketState(anthropic)
		require.Equal(t, 1, anthropicAttempts)
		require.Equal(t, 1, anthropicVersion)
	})

	t.Run("fencing stays non-fatal", func(t *testing.T) {
		cache := newBatchSnapshotCache()
		cache.setErrors[openai] = ErrSchedulerBucketWriteFenced
		repo := newBatchAccountQueryRepo()
		svc := newBatchQueryTestService(cache, repo)

		require.NoError(t, svc.rebuildBuckets(context.Background(), []SchedulerBucket{openai, anthropic}, "test"))
		_, openaiAttempts, openaiVersion, _ := cache.bucketState(openai)
		_, anthropicAttempts, anthropicVersion, _ := cache.bucketState(anthropic)
		require.Equal(t, 1, openaiAttempts)
		require.Zero(t, openaiVersion)
		require.Equal(t, 1, anthropicAttempts)
		require.Equal(t, 1, anthropicVersion)
	})

	t.Run("strict lock busy is returned to the caller", func(t *testing.T) {
		cache := newBatchSnapshotCache()
		cache.lockBusy[openai] = true
		token, err := cache.CaptureBucketWriteToken(context.Background(), openai)
		require.NoError(t, err)
		repo := newBatchAccountQueryRepo()
		svc := newBatchQueryTestService(cache, repo)

		err = svc.rebuildPreparedBucketTasks(
			context.Background(),
			[]schedulerBucketWriteTask{{bucket: openai, token: token}},
			"test",
			true,
		)
		require.ErrorIs(t, err, ErrSchedulerBucketRebuildBusy)
	})
}
