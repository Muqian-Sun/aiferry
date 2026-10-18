package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newStickyCountCache(t *testing.T) (*gatewayCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return &gatewayCache{rdb: rdb}, mr
}

// 数的是网关写入的同一组 key：按 scope × 渠道计数，超过一批的 key 也全部数到。
func TestCountStickySessions_CountsPerScopeAndAccount(t *testing.T) {
	cache, _ := newStickyCountCache(t)
	ctx := context.Background()
	const many = stickySessionCountBatch*2 + 3
	for i := 0; i < many; i++ {
		require.NoError(t, cache.SetSessionAccountID(ctx, 7, fmt.Sprintf("hash-%d", i), 101, time.Hour))
	}
	require.NoError(t, cache.SetSessionAccountID(ctx, 7, "other", 102, time.Hour))
	require.NoError(t, cache.SetSessionAccountID(ctx, 9, "x", 101, time.Hour))
	require.NoError(t, cache.rdb.Set(ctx, "unrelated:7:abc", "5", time.Hour).Err())

	counts, err := cache.CountStickySessions(ctx)
	require.NoError(t, err)
	require.Equal(t, map[int64]map[int64]int{7: {101: many, 102: 1}, 9: {101: 1}}, counts)
}

func TestCountStickySessions_EmptyWhenNoBindings(t *testing.T) {
	cache, _ := newStickyCountCache(t)
	counts, err := cache.CountStickySessions(context.Background())
	require.NoError(t, err)
	require.Empty(t, counts)
}

// 不是网关写入格式的 key 报错，不静默少数。
func TestCountStickySessions_RejectsMalformedEntry(t *testing.T) {
	cache, _ := newStickyCountCache(t)
	ctx := context.Background()
	require.NoError(t, cache.rdb.Set(ctx, stickySessionPrefix+"7:abc", "not-a-number", time.Hour).Err())
	_, err := cache.CountStickySessions(ctx)
	require.ErrorContains(t, err, "unexpected sticky session entry")

	cache, _ = newStickyCountCache(t)
	require.NoError(t, cache.rdb.Set(ctx, stickySessionPrefix+"no-scope", "101", time.Hour).Err())
	_, err = cache.CountStickySessions(ctx)
	require.ErrorContains(t, err, "unexpected sticky session entry")
}
