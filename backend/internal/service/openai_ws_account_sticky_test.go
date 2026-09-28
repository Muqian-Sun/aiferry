package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAIGatewayService_ResolvePreviousResponseAccount_Hit(t *testing.T) {
	scopeID := int64(23)
	ctx := WithCatalogRoute(context.Background(), CatalogRoute{EntryID: scopeID})
	account := Account{
		ID:                2,
		Platform:          PlatformOpenAI,
		Type:              AccountTypeAPIKey,
		Status:            StatusActive,
		Schedulable:       true,
		Concurrency:       2,
		Extra:             map[string]any{},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
	}
	cache := &stubGatewayCache{}
	store := NewOpenAIWSStateStore(cache)
	cfg := newOpenAIWSV2TestConfig()

	svc := &OpenAIGatewayService{
		accountRepo:        stubOpenAIAccountRepo{accounts: []Account{account}},
		cache:              cache,
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{}),
		openaiWSStateStore: store,
	}

	require.NoError(t, store.BindResponseAccount(ctx, scopeID, "resp_prev_1", account.ID, time.Hour))

	accountID := svc.ResolveAccountIDByPreviousResponseIDForScheduler(ctx, "resp_prev_1", "gpt-5.1", nil, "", false)
	require.Equal(t, account.ID, accountID)
}

// 额度超限由状态服务写成 temp_unschedulable；previous_response_id 粘连和其他停调一样按状态判、清绑定。
func TestOpenAIGatewayService_ResolvePreviousResponseAccount_QuotaPausedMiss(t *testing.T) {
	scopeID := int64(23)
	ctx := WithCatalogRoute(context.Background(), CatalogRoute{EntryID: scopeID})
	pausedUntil := time.Now().Add(time.Hour)
	account := Account{
		ID:                      77,
		Platform:                PlatformOpenAI,
		Type:                    AccountTypeAPIKey,
		Status:                  StatusActive,
		Schedulable:             true,
		Concurrency:             2,
		Extra:                   map[string]any{},
		TempUnschedulableUntil:  &pausedUntil,
		TempUnschedulableReason: BuildTempUnschedReasonPayload(openAIQuotaAutoPauseSource, "codex 5h window 96.0% used"),
		ProtocolEndpoints:       map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
	}
	cache := &stubGatewayCache{}
	store := NewOpenAIWSStateStore(cache)
	cfg := newOpenAIWSV2TestConfig()
	svc := &OpenAIGatewayService{
		accountRepo:        stubOpenAIAccountRepo{accounts: []Account{account}},
		cache:              cache,
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{}),
		openaiWSStateStore: store,
	}

	require.NoError(t, store.BindResponseAccount(ctx, scopeID, "resp_prev_quota", account.ID, time.Hour))

	accountID := svc.ResolveAccountIDByPreviousResponseIDForScheduler(ctx, "resp_prev_quota", "gpt-5.1", nil, "", false)
	require.Zero(t, accountID, "被状态服务停调的账号不应继续命中 previous_response_id 粘连")
	boundAccountID, getErr := store.GetResponseAccount(ctx, scopeID, "resp_prev_quota")
	require.NoError(t, getErr)
	require.Zero(t, boundAccountID, "与限流等停调同处理：清绑定")
}

func TestOpenAIGatewayService_ResolvePreviousResponseAccount_RateLimitedMiss(t *testing.T) {
	scopeID := int64(23)
	ctx := WithCatalogRoute(context.Background(), CatalogRoute{EntryID: scopeID})
	rateLimitedUntil := time.Now().Add(30 * time.Minute)
	account := Account{
		ID:                12,
		Platform:          PlatformOpenAI,
		Type:              AccountTypeAPIKey,
		Status:            StatusActive,
		Schedulable:       true,
		Concurrency:       1,
		RateLimitResetAt:  &rateLimitedUntil,
		Extra:             map[string]any{},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
	}
	cache := &stubGatewayCache{}
	store := NewOpenAIWSStateStore(cache)
	cfg := newOpenAIWSV2TestConfig()
	svc := &OpenAIGatewayService{
		accountRepo:        stubOpenAIAccountRepo{accounts: []Account{account}},
		cache:              cache,
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{}),
		openaiWSStateStore: store,
	}

	require.NoError(t, store.BindResponseAccount(ctx, scopeID, "resp_prev_rl", account.ID, time.Hour))

	accountID := svc.ResolveAccountIDByPreviousResponseIDForScheduler(ctx, "resp_prev_rl", "gpt-5.1", nil, "", false)
	require.Zero(t, accountID, "限额中的账号不应继续命中 previous_response_id 粘连")
	boundAccountID, getErr := store.GetResponseAccount(ctx, scopeID, "resp_prev_rl")
	require.NoError(t, getErr)
	require.Zero(t, boundAccountID)
}

func TestOpenAIGatewayService_ResolvePreviousResponseAccount_DBRuntimeRecheckRateLimitedMiss(t *testing.T) {
	scopeID := int64(24)
	ctx := WithCatalogRoute(context.Background(), CatalogRoute{EntryID: scopeID})
	rateLimitedUntil := time.Now().Add(30 * time.Minute)
	staleAccount := &Account{
		ID:                13,
		Platform:          PlatformOpenAI,
		Type:              AccountTypeAPIKey,
		Status:            StatusActive,
		Schedulable:       true,
		Concurrency:       1,
		Extra:             map[string]any{},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
	}
	dbAccount := Account{
		ID:                13,
		Platform:          PlatformOpenAI,
		Type:              AccountTypeAPIKey,
		Status:            StatusActive,
		Schedulable:       true,
		Concurrency:       1,
		RateLimitResetAt:  &rateLimitedUntil,
		Extra:             map[string]any{},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
	}
	cache := &stubGatewayCache{}
	store := NewOpenAIWSStateStore(cache)
	cfg := newOpenAIWSV2TestConfig()
	snapshotCache := &openAISnapshotCacheStub{
		accountsByID: map[int64]*Account{dbAccount.ID: staleAccount},
	}
	svc := &OpenAIGatewayService{
		accountRepo:        stubOpenAIAccountRepo{accounts: []Account{dbAccount}},
		cache:              cache,
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{}),
		openaiWSStateStore: store,
		schedulerSnapshot:  &SchedulerSnapshotService{cache: snapshotCache},
	}

	require.NoError(t, store.BindResponseAccount(ctx, scopeID, "resp_prev_db_rl", dbAccount.ID, time.Hour))

	accountID := svc.ResolveAccountIDByPreviousResponseIDForScheduler(ctx, "resp_prev_db_rl", "gpt-5.1", nil, "", false)
	require.Zero(t, accountID, "DB 中已限流的账号不应继续命中 previous_response_id 粘连")
	boundAccountID, getErr := store.GetResponseAccount(ctx, scopeID, "resp_prev_db_rl")
	require.NoError(t, getErr)
	require.Zero(t, boundAccountID)
}

func TestOpenAIGatewayService_ResolvePreviousResponseAccount_Excluded(t *testing.T) {
	scopeID := int64(23)
	ctx := WithCatalogRoute(context.Background(), CatalogRoute{EntryID: scopeID})
	account := Account{
		ID:                8,
		Platform:          PlatformOpenAI,
		Type:              AccountTypeAPIKey,
		Status:            StatusActive,
		Schedulable:       true,
		Concurrency:       1,
		Extra:             map[string]any{},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
	}
	cache := &stubGatewayCache{}
	store := NewOpenAIWSStateStore(cache)
	cfg := newOpenAIWSV2TestConfig()
	svc := &OpenAIGatewayService{
		accountRepo:        stubOpenAIAccountRepo{accounts: []Account{account}},
		cache:              cache,
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{}),
		openaiWSStateStore: store,
	}

	require.NoError(t, store.BindResponseAccount(ctx, scopeID, "resp_prev_2", account.ID, time.Hour))

	accountID := svc.ResolveAccountIDByPreviousResponseIDForScheduler(ctx, "resp_prev_2", "gpt-5.1", map[int64]struct{}{account.ID: {}}, "", false)
	require.Zero(t, accountID)
}

func TestOpenAIGatewayService_ResolvePreviousResponseAccount_APIKeyForceHTTPHit(t *testing.T) {
	scopeID := int64(23)
	ctx := WithCatalogRoute(context.Background(), CatalogRoute{EntryID: scopeID})
	account := Account{
		ID:          11,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Extra: map[string]any{
			"openai_ws_force_http": true,
		},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
	}
	cache := &stubGatewayCache{}
	store := NewOpenAIWSStateStore(cache)
	cfg := newOpenAIWSV2TestConfig()
	svc := &OpenAIGatewayService{
		accountRepo:        stubOpenAIAccountRepo{accounts: []Account{account}},
		cache:              cache,
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{}),
		openaiWSStateStore: store,
	}

	require.NoError(t, store.BindResponseAccount(ctx, scopeID, "resp_prev_force_http", account.ID, time.Hour))

	accountID := svc.ResolveAccountIDByPreviousResponseIDForScheduler(ctx, "resp_prev_force_http", "gpt-5.1", nil, "", false)
	require.Equal(t, account.ID, accountID, "API-key HTTP continuation must retain the key/project that created the response")
}

func TestOpenAIGatewayService_ResolvePreviousResponseAccount_OAuthForceHTTPIgnored(t *testing.T) {
	scopeID := int64(23)
	ctx := WithCatalogRoute(context.Background(), CatalogRoute{EntryID: scopeID})
	account := Account{
		ID:          12,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Extra: map[string]any{
			"openai_ws_force_http": true,
		},
	}
	cache := &stubGatewayCache{}
	store := NewOpenAIWSStateStore(cache)
	svc := &OpenAIGatewayService{
		accountRepo:        stubOpenAIAccountRepo{accounts: []Account{account}},
		cache:              cache,
		cfg:                newOpenAIWSV2TestConfig(),
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{}),
		openaiWSStateStore: store,
	}

	require.NoError(t, store.BindResponseAccount(ctx, scopeID, "resp_prev_oauth_force_http", account.ID, time.Hour))

	accountID := svc.ResolveAccountIDByPreviousResponseIDForScheduler(ctx, "resp_prev_oauth_force_http", "gpt-5.1", nil, "", false)
	require.Zero(t, accountID, "OAuth HTTP fallback cannot preserve WSv2 continuation state")
}

func TestOpenAIGatewayService_ResolvePreviousResponseAccount_BusyKeepsSticky(t *testing.T) {
	scopeID := int64(23)
	ctx := WithCatalogRoute(context.Background(), CatalogRoute{EntryID: scopeID})
	accounts := []Account{
		{
			ID:                21,
			Platform:          PlatformOpenAI,
			Type:              AccountTypeAPIKey,
			Status:            StatusActive,
			Schedulable:       true,
			Concurrency:       1,
			Priority:          0,
			Extra:             map[string]any{},
			ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
		},
		{
			ID:                22,
			Platform:          PlatformOpenAI,
			Type:              AccountTypeAPIKey,
			Status:            StatusActive,
			Schedulable:       true,
			Concurrency:       1,
			Priority:          9,
			Extra:             map[string]any{},
			ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
		},
	}

	cache := &stubGatewayCache{}
	store := NewOpenAIWSStateStore(cache)
	cfg := newOpenAIWSV2TestConfig()
	cfg.Gateway.Scheduling.StickySessionMaxWaiting = 2
	cfg.Gateway.Scheduling.StickySessionWaitTimeout = 30 * time.Second

	concurrencyCache := stubConcurrencyCache{
		acquireResults: map[int64]bool{
			21: false, // previous_response 命中的账号繁忙
			22: true,  // 次优账号可用（若回退会命中）
		},
		waitCounts: map[int64]int{
			21: 999,
		},
	}

	svc := &OpenAIGatewayService{
		accountRepo:        stubOpenAIAccountRepo{accounts: accounts},
		cache:              cache,
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(concurrencyCache),
		openaiWSStateStore: store,
	}

	require.NoError(t, store.BindResponseAccount(ctx, scopeID, "resp_prev_busy", 21, time.Hour))

	accountID := svc.ResolveAccountIDByPreviousResponseIDForScheduler(ctx, "resp_prev_busy", "gpt-5.1", nil, "", false)
	require.Equal(t, int64(21), accountID, "busy previous_response sticky account should remain resolved; the scheduler queues on it")
}

func TestOpenAIGatewayService_ResolvePreviousResponseAccount_CapabilityMismatchKeepsSticky(t *testing.T) {
	scopeID := int64(25)
	ctx := WithCatalogRoute(context.Background(), CatalogRoute{EntryID: scopeID})
	// 能力不匹配用成品号不接 embeddings 来构造（渠道级 openai_capabilities 2026-09-28 P5 删了）。
	account := Account{
		ID:          31,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Extra:       map[string]any{},
	}
	require.False(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityEmbeddings))
	cache := &stubGatewayCache{}
	store := NewOpenAIWSStateStore(cache)
	cfg := newOpenAIWSV2TestConfig()
	svc := &OpenAIGatewayService{
		accountRepo:        stubOpenAIAccountRepo{accounts: []Account{account}},
		cache:              cache,
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{}),
		openaiWSStateStore: store,
	}

	require.NoError(t, store.BindResponseAccount(ctx, scopeID, "resp_prev_capability", account.ID, time.Hour))

	accountID := svc.ResolveAccountIDByPreviousResponseIDForScheduler(ctx, "resp_prev_capability", "text-embedding-3-small", nil, OpenAIEndpointCapabilityEmbeddings, false)
	require.Zero(t, accountID)
	boundAccountID, getErr := store.GetResponseAccount(ctx, scopeID, "resp_prev_capability")
	require.NoError(t, getErr)
	require.Equal(t, account.ID, boundAccountID)
}

func newOpenAIWSV2TestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	// 渠道级 WS 开关 2026-09-28 P5 删了，WS 只能经 mode_router_v2 的全局默认模式打开。
	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	cfg.Gateway.OpenAIWS.IngressModeDefault = OpenAIWSIngressModeCtxPool
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600
	return cfg
}
