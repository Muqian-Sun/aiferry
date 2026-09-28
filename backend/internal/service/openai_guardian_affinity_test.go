package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type guardianAffinityAccountRepo struct {
	schedulerTestOpenAIAccountRepo
	setErrorCalls int
}

func (r *guardianAffinityAccountRepo) SetError(context.Context, int64, string) error {
	r.setErrorCalls++
	return nil
}

func guardianAffinityTestContext(t *testing.T, model, subagent, parentHeader, metadata string) context.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set(openAISubagentHeader, subagent)
	if parentHeader != "" {
		c.Request.Header.Set(codexParentThreadIDHeader, parentHeader)
	}
	if metadata != "" {
		c.Request.Header.Set(codexTurnMetadataHeader, metadata)
	}
	return WithOpenAIGuardianParentAffinity(context.Background(), c, nil, model)
}

func TestWithOpenAIGuardianParentAffinity_RequiresUnambiguousReviewLineage(t *testing.T) {
	parentID := "11111111-1111-4111-8111-111111111111"
	wantHash := DeriveSessionHashFromSeed(parentID)

	for _, subagent := range []string{"guardian", "review", "GUARDIAN"} {
		t.Run(subagent, func(t *testing.T) {
			ctx := guardianAffinityTestContext(t, codexAutoReviewModel, subagent, parentID, `{"parent_thread_id":"`+parentID+`"}`)
			affinity, ok := openAIGuardianParentAffinityFromContext(ctx)
			require.True(t, ok)
			require.Equal(t, wantHash, affinity.currentSessionHash)
		})
	}

	t.Run("metadata only", func(t *testing.T) {
		ctx := guardianAffinityTestContext(t, codexAutoReviewModel, "guardian", "", `{"parent_thread_id":"`+parentID+`"}`)
		_, ok := openAIGuardianParentAffinityFromContext(ctx)
		require.True(t, ok)
	})

	t.Run("websocket envelope metadata", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/openai/v1/responses", nil)
		body := []byte(`{"type":"response.create","response":{"model":"codex-auto-review","client_metadata":{"x-codex-turn-metadata":"{\"parent_thread_id\":\"` + parentID + `\",\"subagent_kind\":\"guardian\"}"}}}`)
		ctx := WithOpenAIGuardianParentAffinity(context.Background(), c, body, codexAutoReviewModel)
		affinity, ok := openAIGuardianParentAffinityFromContext(ctx)
		require.True(t, ok)
		require.Equal(t, wantHash, affinity.currentSessionHash)
	})

	for name, ctx := range map[string]context.Context{
		"ordinary model":       guardianAffinityTestContext(t, "gpt-5.6-sol", "guardian", parentID, ""),
		"ordinary subagent":    guardianAffinityTestContext(t, codexAutoReviewModel, "collab_spawn", parentID, ""),
		"missing parent":       guardianAffinityTestContext(t, codexAutoReviewModel, "guardian", "", ""),
		"conflicting lineage":  guardianAffinityTestContext(t, codexAutoReviewModel, "guardian", parentID, `{"parent_thread_id":"different-parent"}`),
		"conflicting subagent": guardianAffinityTestContext(t, codexAutoReviewModel, "guardian", parentID, `{"parent_thread_id":"`+parentID+`","subagent_kind":"collab_spawn"}`),
	} {
		t.Run(name, func(t *testing.T) {
			_, ok := openAIGuardianParentAffinityFromContext(ctx)
			require.False(t, ok)
		})
	}
}

// newGuardianScheduler 把唯一调度器装在与 OpenAI 服务相同的 repo / cache / 并发上。
func newGuardianScheduler(svc *OpenAIGatewayService) *GatewayService {
	return &GatewayService{
		accountRepo:        svc.accountRepo,
		cache:              svc.cache,
		cfg:                svc.cfg,
		concurrencyService: svc.concurrencyService,
	}
}

// selectWithGuardianAffinity 照 handler 的做法：守护父线程的绑定账号做成预取粘性，再交给唯一调度器。
func selectWithGuardianAffinity(ctx context.Context, svc *OpenAIGatewayService, gw *GatewayService, sessionHash string, excluded map[int64]struct{}) (*AccountSelectionResult, int64, error) {
	stickyID := svc.ResolveOpenAIGuardianParentAccountID(ctx)
	if stickyID > 0 {
		ctx = WithPrefetchedStickySession(ctx, stickyID, SchedulingScopeID(ctx), false)
	}
	// 审查请求在生产里带目录路由；这里用 openai 平台池代替（成品号平台相等）。
	selection, err := gw.SelectAccountWithOptions(ctx, sessionHash, codexAutoReviewModel, excluded, SelectOptions{Platform: PlatformOpenAI})
	return selection, stickyID, err
}

// 审查子请求命中父线程的绑定账号（哪怕它优先级更低），父线程的绑定不被删。
func TestOpenAIGatewayService_GuardianParentAffinitySelectsParentAccount(t *testing.T) {
	parentID := "22222222-2222-4222-8222-222222222222"
	parentHash := DeriveSessionHashFromSeed(parentID)

	accounts := []Account{
		{ID: 39001, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10, Credentials: map[string]any{"plan_type": "team"}},
		{ID: 39002, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, Credentials: map[string]any{"plan_type": "team"}},
	}
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{parentHash: 39001}}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              cache,
		cfg:                &config.Config{},
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquireResults: map[int64]bool{39001: true, 39002: true}}),
	}
	gw := newGuardianScheduler(svc)

	ctx := guardianAffinityTestContext(t, codexAutoReviewModel, "guardian", parentID, "")
	selection, stickyID, err := selectWithGuardianAffinity(ctx, svc, gw, "guardian-child-session", nil)
	require.NoError(t, err)
	require.Equal(t, int64(39001), stickyID)
	require.NotNil(t, selection)
	require.Equal(t, int64(39001), selection.Account.ID)
	require.Zero(t, cache.deletedSessions[parentHash])
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

// 父线程账号不在本分组 / 已因上游失败被排除：回落到正常选号，且绝不删父线程的绑定。
// 父线程账号在本次换号中已被排除（上游失败）时不绕过排除：落到别的账号，父线程绑定不动。
func TestOpenAIGatewayService_GuardianParentAffinityFallsBackWhenParentExcluded(t *testing.T) {
	parentID := "33333333-3333-4333-8333-333333333333"
	parentHash := DeriveSessionHashFromSeed(parentID)

	for name, excluded := range map[string]map[int64]struct{}{
		"parent excluded after upstream failure": {39011: {}},
	} {
		t.Run(name, func(t *testing.T) {
			accounts := []Account{
				{ID: 39011, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, Credentials: map[string]any{"plan_type": "team"}},
				{ID: 39012, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5, Credentials: map[string]any{"plan_type": "team"}},
			}
			cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{parentHash: 39011}}
			svc := &OpenAIGatewayService{
				accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
				cache:              cache,
				cfg:                &config.Config{},
				concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquireResults: map[int64]bool{39011: true, 39012: true}}),
			}
			gw := newGuardianScheduler(svc)

			ctx := guardianAffinityTestContext(t, codexAutoReviewModel, "guardian", parentID, "")
			selection, _, err := selectWithGuardianAffinity(ctx, svc, gw, "guardian-fallback-child", excluded)
			require.NoError(t, err)
			require.NotNil(t, selection)
			require.Equal(t, int64(39012), selection.Account.ID)
			require.Zero(t, cache.deletedSessions[parentHash], "a child request must never delete its parent's binding")
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
		})
	}
}

// 子请求的 session hash 与父线程相同（hash 碰撞）时，准入后绑定不得覆写父线程的绑定。
func TestOpenAIGatewayService_GuardianParentHashCollisionPreservesParentBinding(t *testing.T) {
	parentID := "44444444-4444-4444-8444-444444444444"
	parentHash := DeriveSessionHashFromSeed(parentID)

	accounts := []Account{
		{ID: 39021, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, Credentials: map[string]any{"plan_type": "team"}},
		{ID: 39022, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5, Credentials: map[string]any{"plan_type": "team"}},
	}
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{parentHash: 39021}}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              cache,
		cfg:                &config.Config{},
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquireResults: map[int64]bool{39021: true, 39022: true}}),
	}
	gw := newGuardianScheduler(svc)

	ctx := guardianAffinityTestContext(t, codexAutoReviewModel, "guardian", parentID, "")
	// 父账号本次被排除（上游失败换号），子请求落到 39022；它的 session hash 与父线程碰撞
	selection, _, err := selectWithGuardianAffinity(ctx, svc, gw, parentHash, map[int64]struct{}{39021: {}})
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(39022), selection.Account.ID)
	require.NoError(t, gw.BindStickySessionAfterProfitAdmission(ctx, parentHash, selection.Account.ID))
	require.Equal(t, int64(39021), cache.sessionBindings[parentHash], "the parent binding must survive the child's admission")
	require.Zero(t, cache.deletedSessions[parentHash])
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

// 续链绑定的账号（不论它绑着什么分组——池是全部资源）命中，并作为预取粘性被选中。
func TestOpenAIGatewayService_PreviousResponseBoundAccountSelectedAsPrefetch(t *testing.T) {
	bound := Account{
		ID: 39051, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
		Extra:             map[string]any{},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
	}
	fallback := Account{
		ID: 39052, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10,
		Extra:             map[string]any{},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
	}
	accounts := []Account{bound, fallback}
	repo := &guardianAffinityAccountRepo{schedulerTestOpenAIAccountRepo: schedulerTestOpenAIAccountRepo{accounts: accounts}}
	cache := &schedulerTestGatewayCache{}
	store := NewOpenAIWSStateStore(cache)
	cfg := &config.Config{}
	svc := &OpenAIGatewayService{
		accountRepo:        repo,
		cache:              cache,
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(&schedulerTestConcurrencyCache{}),
		openaiWSStateStore: store,
		schedulerSnapshot: &SchedulerSnapshotService{
			accountRepo: repo,
		},
	}
	responseID := "resp_simple_mode_cross_group"
	require.NoError(t, store.BindResponseAccount(context.Background(), SchedulingScopeID(context.Background()), responseID, bound.ID, time.Hour))

	stickyID := svc.ResolveAccountIDByPreviousResponseIDForScheduler(context.Background(), responseID, codexAutoReviewModel, nil, OpenAIEndpointCapabilityResponses, false)
	require.Equal(t, bound.ID, stickyID)

	gw := newGuardianScheduler(svc)
	ctx := WithPrefetchedStickySession(context.Background(), stickyID, SchedulingScopeID(context.Background()), false)
	selection, err := gw.SelectAccountWithOptions(ctx, "", codexAutoReviewModel, nil, SelectOptions{Capability: OpenAIEndpointCapabilityResponses, Platform: PlatformOpenAI})
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, bound.ID, selection.Account.ID)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}
