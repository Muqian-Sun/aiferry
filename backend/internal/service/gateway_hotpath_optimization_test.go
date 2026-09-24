package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

type stickyGatewayCacheHotpathStub struct {
	GatewayCache

	stickyID int64
	getCalls atomic.Int64
}

func (s *stickyGatewayCacheHotpathStub) GetSessionAccountID(ctx context.Context, groupID int64, sessionHash string) (int64, error) {
	s.getCalls.Add(1)
	if s.stickyID > 0 {
		return s.stickyID, nil
	}
	return 0, errors.New("not found")
}

func (s *stickyGatewayCacheHotpathStub) SetSessionAccountID(ctx context.Context, groupID int64, sessionHash string, accountID int64, ttl time.Duration) error {
	return nil
}

func (s *stickyGatewayCacheHotpathStub) RefreshSessionTTL(ctx context.Context, groupID int64, sessionHash string, ttl time.Duration) error {
	return nil
}

func (s *stickyGatewayCacheHotpathStub) DeleteSessionAccountID(ctx context.Context, groupID int64, sessionHash string) error {
	return nil
}

func (s *stickyGatewayCacheHotpathStub) SetGrokVideoPendingBilling(_ context.Context, _ string, _ []byte, _ time.Duration) error {
	return nil
}
func (s *stickyGatewayCacheHotpathStub) GetGrokVideoPendingBilling(_ context.Context, _ string) ([]byte, error) {
	return nil, nil
}
func (s *stickyGatewayCacheHotpathStub) ClaimGrokVideoBilled(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return true, nil
}

func (s *stickyGatewayCacheHotpathStub) ReleaseGrokVideoBilled(_ context.Context, _ string) error {
	return nil
}

func (s *stickyGatewayCacheHotpathStub) SetReasoningContent(_ context.Context, _ string, _ string, _ time.Duration) error {
	return nil
}
func (s *stickyGatewayCacheHotpathStub) GetReasoningContent(_ context.Context, _ string) (string, error) {
	return "", ErrReasoningContentNotFound
}

func TestGatewayHotpathHelpers_StickyContext(t *testing.T) {
	t.Run("prefetched_sticky_account_id_from_context", func(t *testing.T) {
		require.Equal(t, int64(0), prefetchedStickyAccountIDFromContext(context.TODO()))
		require.Equal(t, int64(0), prefetchedStickyAccountIDFromContext(context.Background()))

		// 无路由：作用域 0
		ctx := context.WithValue(context.Background(), ctxkey.PrefetchedStickyAccountID, int64(123))
		ctx = context.WithValue(ctx, ctxkey.PrefetchedStickyScopeID, int64(0))
		require.Equal(t, int64(123), prefetchedStickyAccountIDFromContext(ctx))

		// 目录路由：作用域 = 条目 ID，预取的作用域要一致
		routed := WithCatalogRoute(context.Background(), CatalogRoute{EntryID: 9, Entry: &ModelCatalogEntry{ID: 9, ModelID: "m"}})
		ctx2 := context.WithValue(routed, ctxkey.PrefetchedStickyAccountID, 456)
		ctx2 = context.WithValue(ctx2, ctxkey.PrefetchedStickyScopeID, int64(9))
		require.Equal(t, int64(456), prefetchedStickyAccountIDFromContext(ctx2))

		ctx3 := context.WithValue(routed, ctxkey.PrefetchedStickyAccountID, "invalid")
		ctx3 = context.WithValue(ctx3, ctxkey.PrefetchedStickyScopeID, int64(9))
		require.Equal(t, int64(0), prefetchedStickyAccountIDFromContext(ctx3))

		ctx4 := context.WithValue(routed, ctxkey.PrefetchedStickyAccountID, int64(789))
		ctx4 = context.WithValue(ctx4, ctxkey.PrefetchedStickyScopeID, int64(10))
		require.Equal(t, int64(0), prefetchedStickyAccountIDFromContext(ctx4), "预取作用域与请求作用域不一致 → 不认")
	})
}

func TestSelectAccountWithLoadAwareness_StickyReadReuse(t *testing.T) {
	now := time.Now().Add(-time.Minute)
	account := Account{
		ID:                88,
		Platform:          PlatformAnthropic,
		Type:              AccountTypeAPIKey,
		Status:            StatusActive,
		Schedulable:       true,
		Concurrency:       4,
		Priority:          1,
		LastUsedAt:        &now,
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"},
	}

	repo := stubOpenAIAccountRepo{accounts: []Account{account}}
	concurrency := NewConcurrencyService(stubConcurrencyCache{})

	cfg := &config.Config{
		RunMode: config.RunModeStandard,
		Gateway: config.GatewayConfig{
			Scheduling: config.GatewaySchedulingConfig{
				LoadBatchEnabled:         true,
				StickySessionMaxWaiting:  3,
				StickySessionWaitTimeout: time.Second,
				FallbackWaitTimeout:      time.Second,
				FallbackMaxWaiting:       10,
			},
		},
	}

	baseCtx := context.WithValue(context.Background(), ctxkey.ForcePlatform, PlatformAnthropic)

	t.Run("without_prefetch_reads_cache_once", func(t *testing.T) {
		cache := &stickyGatewayCacheHotpathStub{stickyID: account.ID}
		svc := &GatewayService{
			accountRepo:        repo,
			cache:              cache,
			cfg:                cfg,
			concurrencyService: concurrency,
		}

		result, err := svc.SelectAccountWithLoadAwareness(baseCtx, "sess-hash", "", nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.Account)
		require.Equal(t, account.ID, result.Account.ID)
		require.Equal(t, int64(1), cache.getCalls.Load())
	})

	t.Run("with_prefetch_skips_cache_read", func(t *testing.T) {
		cache := &stickyGatewayCacheHotpathStub{stickyID: account.ID}
		svc := &GatewayService{
			accountRepo:        repo,
			cache:              cache,
			cfg:                cfg,
			concurrencyService: concurrency,
		}

		ctx := context.WithValue(baseCtx, ctxkey.PrefetchedStickyAccountID, account.ID)
		ctx = context.WithValue(ctx, ctxkey.PrefetchedStickyScopeID, int64(0))
		result, err := svc.SelectAccountWithLoadAwareness(ctx, "sess-hash", "", nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.Account)
		require.Equal(t, account.ID, result.Account.ID)
		require.Equal(t, int64(0), cache.getCalls.Load())
	})

	t.Run("with_prefetch_group_mismatch_reads_cache", func(t *testing.T) {
		cache := &stickyGatewayCacheHotpathStub{stickyID: account.ID}
		svc := &GatewayService{
			accountRepo:        repo,
			cache:              cache,
			cfg:                cfg,
			concurrencyService: concurrency,
		}

		ctx := context.WithValue(baseCtx, ctxkey.PrefetchedStickyAccountID, int64(999))
		ctx = context.WithValue(ctx, ctxkey.PrefetchedStickyScopeID, int64(77))
		result, err := svc.SelectAccountWithLoadAwareness(ctx, "sess-hash", "", nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.Account)
		require.Equal(t, account.ID, result.Account.ID)
		require.Equal(t, int64(1), cache.getCalls.Load())
	})
}
