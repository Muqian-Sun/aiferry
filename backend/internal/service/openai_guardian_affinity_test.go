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

type guardianAffinityGroupRepo struct {
	GroupRepository
	group *Group
	err   error
}

type guardianAffinityAccountRepo struct {
	schedulerGroupAwareOpenAIAccountRepo
	setErrorCalls int
}

func (r *guardianAffinityAccountRepo) SetError(context.Context, int64, string) error {
	r.setErrorCalls++
	return nil
}

func (r guardianAffinityGroupRepo) GetByID(context.Context, int64) (*Group, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.group, nil
}

func (r guardianAffinityGroupRepo) GetByIDLite(context.Context, int64) (*Group, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.group, nil
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

// guardianTestGroup 一个合法的 openai 分组（Gateway 调度器从 groupRepo 取分组并挂到 ctx；隐私门读 ctx 里的分组）。
func guardianTestGroup(groupID int64, requirePrivacy bool) *Group {
	return &Group{ID: groupID, Name: "guardian", Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true, RequirePrivacySet: requirePrivacy}
}

// newGuardianScheduler 把唯一调度器装在与 OpenAI 服务相同的 repo / cache / 并发上。
func newGuardianScheduler(svc *OpenAIGatewayService, group *Group, groupErr error) *GatewayService {
	return &GatewayService{
		accountRepo:        svc.accountRepo,
		groupRepo:          guardianAffinityGroupRepo{group: group, err: groupErr},
		cache:              svc.cache,
		cfg:                svc.cfg,
		concurrencyService: svc.concurrencyService,
	}
}

// selectWithGuardianAffinity 照 handler 的做法：守护父线程的绑定账号做成预取粘性，再交给唯一调度器。
func selectWithGuardianAffinity(ctx context.Context, svc *OpenAIGatewayService, gw *GatewayService, groupID *int64, sessionHash string, excluded map[int64]struct{}) (*AccountSelectionResult, int64, error) {
	stickyID := svc.ResolveOpenAIGuardianParentAccountID(ctx, groupID)
	if stickyID > 0 {
		ctx = WithPrefetchedStickySession(ctx, stickyID, SchedulingScopeID(ctx, groupID), false)
	}
	selection, err := gw.SelectAccountWithOptions(ctx, groupID, sessionHash, codexAutoReviewModel, excluded, SelectOptions{})
	return selection, stickyID, err
}

// 审查子请求命中父线程的绑定账号（哪怕它优先级更低），父线程的绑定不被删。
func TestOpenAIGatewayService_GuardianParentAffinitySelectsParentAccount(t *testing.T) {
	parentID := "22222222-2222-4222-8222-222222222222"
	parentHash := DeriveSessionHashFromSeed(parentID)
	groupID := int64(102001)

	accounts := []Account{
		{ID: 39001, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10, GroupIDs: []int64{groupID}, Credentials: map[string]any{"plan_type": "team"}},
		{ID: 39002, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, GroupIDs: []int64{groupID}, Credentials: map[string]any{"plan_type": "team"}},
	}
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{parentHash: 39001}}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: accounts}},
		cache:              cache,
		cfg:                &config.Config{},
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquireResults: map[int64]bool{39001: true, 39002: true}}),
	}
	gw := newGuardianScheduler(svc, guardianTestGroup(groupID, false), nil)

	ctx := guardianAffinityTestContext(t, codexAutoReviewModel, "guardian", parentID, "")
	selection, stickyID, err := selectWithGuardianAffinity(ctx, svc, gw, &groupID, "guardian-child-session", nil)
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
func TestOpenAIGatewayService_GuardianParentAffinityFallsBackWithoutCrossGroupOrFailoverBypass(t *testing.T) {
	parentID := "33333333-3333-4333-8333-333333333333"
	parentHash := DeriveSessionHashFromSeed(parentID)
	groupID := int64(102011)
	otherGroupID := int64(102012)

	for name, excluded := range map[string]map[int64]struct{}{
		"parent moved out of group":              nil,
		"parent excluded after upstream failure": {39011: {}},
	} {
		t.Run(name, func(t *testing.T) {
			parentGroups := []int64{groupID}
			if excluded == nil {
				parentGroups = []int64{otherGroupID}
			}
			accounts := []Account{
				{ID: 39011, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, GroupIDs: parentGroups, Credentials: map[string]any{"plan_type": "team"}},
				{ID: 39012, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5, GroupIDs: []int64{groupID}, Credentials: map[string]any{"plan_type": "team"}},
			}
			cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{parentHash: 39011}}
			svc := &OpenAIGatewayService{
				accountRepo:        schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: accounts}},
				cache:              cache,
				cfg:                &config.Config{},
				concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquireResults: map[int64]bool{39011: true, 39012: true}}),
			}
			gw := newGuardianScheduler(svc, guardianTestGroup(groupID, false), nil)

			ctx := guardianAffinityTestContext(t, codexAutoReviewModel, "guardian", parentID, "")
			selection, _, err := selectWithGuardianAffinity(ctx, svc, gw, &groupID, "guardian-fallback-child", excluded)
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
	groupID := int64(102021)
	otherGroupID := int64(102022)

	accounts := []Account{
		{ID: 39021, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, GroupIDs: []int64{otherGroupID}, Credentials: map[string]any{"plan_type": "team"}},
		{ID: 39022, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 5, GroupIDs: []int64{groupID}, Credentials: map[string]any{"plan_type": "team"}},
	}
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{parentHash: 39021}}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: accounts}},
		cache:              cache,
		cfg:                &config.Config{},
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquireResults: map[int64]bool{39021: true, 39022: true}}),
	}
	gw := newGuardianScheduler(svc, guardianTestGroup(groupID, false), nil)

	ctx := guardianAffinityTestContext(t, codexAutoReviewModel, "guardian", parentID, "")
	selection, _, err := selectWithGuardianAffinity(ctx, svc, gw, &groupID, parentHash, nil)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(39022), selection.Account.ID)
	require.NoError(t, gw.BindStickySessionAfterProfitAdmission(ctx, &groupID, parentHash, selection.Account.ID))
	require.Equal(t, int64(39021), cache.sessionBindings[parentHash], "the parent binding must survive the child's admission")
	require.Zero(t, cache.deletedSessions[parentHash])
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

// 简单模式没有分组隔离：续链绑定的账号即使标了别的分组也命中，并作为预取粘性被选中。
func TestOpenAIGatewayService_PreviousResponseSimpleModeIgnoresGroupMembership(t *testing.T) {
	groupID := int64(3905)
	bound := Account{
		ID: 39051, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
		GroupIDs:          []int64{groupID + 1},
		Extra:             map[string]any{"openai_apikey_responses_websockets_v2_enabled": true},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
	}
	fallback := Account{
		ID: 39052, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10,
		GroupIDs:          []int64{groupID},
		Extra:             map[string]any{"openai_apikey_responses_websockets_v2_enabled": true},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
	}
	accounts := []Account{bound, fallback}
	repo := &guardianAffinityAccountRepo{schedulerGroupAwareOpenAIAccountRepo: schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: accounts}}}
	cache := &schedulerTestGatewayCache{}
	store := NewOpenAIWSStateStore(cache)
	cfg := &config.Config{RunMode: config.RunModeSimple}
	group := guardianTestGroup(groupID, false)
	svc := &OpenAIGatewayService{
		accountRepo:        repo,
		cache:              cache,
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(&schedulerTestConcurrencyCache{}),
		openaiWSStateStore: store,
		schedulerSnapshot: &SchedulerSnapshotService{
			accountRepo: repo,
			groupRepo:   guardianAffinityGroupRepo{group: group},
		},
	}
	responseID := "resp_simple_mode_cross_group"
	require.NoError(t, store.BindResponseAccount(context.Background(), groupID, responseID, bound.ID, time.Hour))

	stickyID := svc.ResolveAccountIDByPreviousResponseIDForScheduler(context.Background(), &groupID, responseID, codexAutoReviewModel, nil, OpenAIEndpointCapabilityResponses, false)
	require.Equal(t, bound.ID, stickyID)

	gw := newGuardianScheduler(svc, group, nil)
	ctx := WithPrefetchedStickySession(context.Background(), stickyID, SchedulingScopeID(context.Background(), &groupID), false)
	selection, err := gw.SelectAccountWithOptions(ctx, &groupID, "", codexAutoReviewModel, nil, SelectOptions{Capability: OpenAIEndpointCapabilityResponses})
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, bound.ID, selection.Account.ID)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}
