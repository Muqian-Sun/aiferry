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

const previewModel = "claude-sonnet-4-5"

func previewEntry() *ModelCatalogEntry {
	return &ModelCatalogEntry{ID: protocolMatchEntryID, ModelID: previewModel, Status: ModelCatalogStatusListed}
}

func previewAccounts(accounts ...Account) []*Account {
	out := make([]*Account, 0, len(accounts))
	for i := range accounts {
		out = append(out, &accounts[i])
	}
	return out
}

func previewItem(t *testing.T, items []SchedulingPreviewItem, accountID int64) SchedulingPreviewItem {
	t.Helper()
	for _, item := range items {
		if item.AccountID == accountID {
			return item
		}
	}
	t.Fatalf("account %d missing from preview", accountID)
	return SchedulingPreviewItem{}
}

// 预览的第 1 名就是调度器此刻会选的渠道：两边走同一套准入与分层规则。
func TestPreviewCatalogScheduling_FirstRankMatchesScheduler(t *testing.T) {
	responsesKey, anthropicOAuth, _, _ := protocolMatchPool()
	for _, loadBatch := range []bool{true, false} {
		for _, inbound := range []string{APIProtocolAnthropic, APIProtocolChatCompletions} {
			t.Run(map[bool]string{true: "load aware ", false: "legacy "}[loadBatch]+inbound, func(t *testing.T) {
				svc := newProtocolMatchService(t, loadBatch, nil, responsesKey, anthropicOAuth)
				result, err := svc.SelectAccountWithLoadAwareness(catalogRouteCtx(protocolMatchEntryID, inbound), "", previewModel, nil)
				require.NoError(t, err)

				accounts := previewAccounts(responsesKey, anthropicOAuth)
				loads, err := svc.SchedulingLoads(context.Background(), accounts)
				if loadBatch {
					require.NoError(t, err)
				} else {
					require.ErrorIs(t, err, errLoadBatchDisabled)
				}
				items := svc.PreviewCatalogScheduling(context.Background(), previewEntry(), accounts, inbound, loads)
				require.Len(t, items, 2)
				require.Equal(t, SchedulingPreviewReady, items[0].State)
				require.Equal(t, 1, items[0].Rank)
				require.Equal(t, result.Account.ID, items[0].AccountID)
				require.Equal(t, 2, items[1].Rank)
			})
		}
	}
}

func TestPreviewCatalogScheduling_DirectFlagFollowsInbound(t *testing.T) {
	responsesKey, anthropicOAuth, _, _ := protocolMatchPool()
	svc := newProtocolMatchService(t, true, nil, responsesKey, anthropicOAuth)
	accounts := previewAccounts(responsesKey, anthropicOAuth)
	items := svc.PreviewCatalogScheduling(context.Background(), previewEntry(), accounts, APIProtocolAnthropic, nil)
	require.True(t, previewItem(t, items, anthropicOAuth.ID).Direct)
	require.False(t, previewItem(t, items, responsesKey.ID).Direct)
}

// 不被选的渠道写清原因与解除时间；ready 在前、queue 其次、blocked 最后。
func TestPreviewCatalogScheduling_BlockedReasonsAndQueue(t *testing.T) {
	responsesKey, anthropicOAuth, _, _ := protocolMatchPool()
	future := time.Now().Add(30 * time.Minute).Truncate(time.Second)

	rateLimited := anthropicOAuth
	rateLimited.ID, rateLimited.Name = 92001, "rate-limited"
	rateLimited.RateLimitResetAt = &future

	modelLimited := anthropicOAuth
	modelLimited.ID, modelLimited.Name = 92002, "model-limited"
	modelLimited.Extra = map[string]any{
		"model_rate_limits": map[string]any{
			previewModel: map[string]any{"rate_limit_reset_at": future.Format(time.RFC3339)},
		},
	}

	disabled := anthropicOAuth
	disabled.ID, disabled.Name, disabled.Status = 92003, "disabled", StatusDisabled

	errored := anthropicOAuth
	errored.ID, errored.Name, errored.Status, errored.ErrorMessage = 92005, "errored", StatusError, " 401 invalid api key "

	full := anthropicOAuth
	full.ID, full.Name = 92004, "full"

	pool := []Account{responsesKey, rateLimited, modelLimited, disabled, full, errored}
	svc := newProtocolMatchService(t, true, nil, pool...)
	svc.concurrencyService = NewConcurrencyService(&mockConcurrencyCache{
		skipDefaultLoad: true,
		loadMap:         map[int64]*AccountLoadInfo{full.ID: {AccountID: full.ID, CurrentConcurrency: 5, LoadRate: 100}},
	})
	accounts := previewAccounts(pool...)
	loads, err := svc.SchedulingLoads(context.Background(), accounts)
	require.NoError(t, err)
	items := svc.PreviewCatalogScheduling(context.Background(), previewEntry(), accounts, APIProtocolAnthropic, loads)

	ready := previewItem(t, items, responsesKey.ID)
	require.Equal(t, SchedulingPreviewReady, ready.State)
	require.Equal(t, 1, ready.Rank)

	queued := previewItem(t, items, full.ID)
	require.Equal(t, SchedulingPreviewQueue, queued.State)
	require.Zero(t, queued.Rank)

	limited := previewItem(t, items, rateLimited.ID)
	require.Equal(t, SchedulingPreviewBlocked, limited.State)
	require.Equal(t, "rate_limited", limited.Reason)
	require.NotNil(t, limited.Until)
	require.True(t, limited.Until.Equal(future))

	byModel := previewItem(t, items, modelLimited.ID)
	require.Equal(t, schedulingPreviewModelRateLimited, byModel.Reason)
	require.NotNil(t, byModel.Until)
	require.True(t, byModel.Until.Equal(future))
	require.Equal(t, previewModel, byModel.Detail)

	require.Equal(t, "disabled", previewItem(t, items, disabled.ID).Reason)

	// 出错停用与手动停用分开：渠道页显示为异常，并带上错误信息
	erroredItem := previewItem(t, items, errored.ID)
	require.Equal(t, schedulingPreviewError, erroredItem.Reason)
	require.Equal(t, "401 invalid api key", erroredItem.Detail)

	states := make([]string, 0, len(items))
	for _, item := range items {
		states = append(states, item.State)
	}
	require.Equal(t, []string{SchedulingPreviewReady, SchedulingPreviewQueue, SchedulingPreviewBlocked, SchedulingPreviewBlocked, SchedulingPreviewBlocked, SchedulingPreviewBlocked}, states)
}

// 承接不了本次入站协议的渠道（responses key 没有 gemini 入站的转换）标 protocol_unsupported。
func TestPreviewCatalogScheduling_ProtocolUnsupported(t *testing.T) {
	responsesKey, _, _, geminiKey := protocolMatchPool()
	svc := newProtocolMatchService(t, true, nil, responsesKey, geminiKey)
	items := svc.PreviewCatalogScheduling(context.Background(), previewEntry(), previewAccounts(responsesKey, geminiKey), APIProtocolGemini, nil)
	require.Equal(t, admitReasonProtocolUnsupported, previewItem(t, items, responsesKey.ID).Reason)
	require.Equal(t, SchedulingPreviewReady, previewItem(t, items, geminiKey.ID).State)
}

type stubSchedulingTraceRepo struct {
	counts   []SchedulingTraceCount
	since    time.Time
	listed   SchedulingTraceFilter
	items    []SchedulingTrace
	inserted []SchedulingTrace
	mu       sync.Mutex
}

func (s *stubSchedulingTraceRepo) InsertSchedulingTraces(_ context.Context, traces []SchedulingTrace) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inserted = append(s.inserted, traces...)
	return nil
}

func (s *stubSchedulingTraceRepo) DeleteSchedulingTracesBefore(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (s *stubSchedulingTraceRepo) ListSchedulingTraces(_ context.Context, filter SchedulingTraceFilter) ([]SchedulingTrace, int64, error) {
	s.listed = filter
	return s.items, int64(len(s.items)), nil
}

func (s *stubSchedulingTraceRepo) CountSchedulingTraces(_ context.Context, since time.Time) ([]SchedulingTraceCount, error) {
	s.since = since
	return s.counts, nil
}

func (s *stubSchedulingTraceRepo) insertedTraces() []SchedulingTrace {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]SchedulingTrace(nil), s.inserted...)
}

type stubStickyCounter struct {
	counts map[int64]map[int64]int
	err    error
}

func (s *stubStickyCounter) CountStickySessions(context.Context) (map[int64]map[int64]int, error) {
	return s.counts, s.err
}

func newSchedulingOverviewFixture(t *testing.T, traces *stubSchedulingTraceRepo, sticky *stubStickyCounter) (*SchedulingOverviewService, Account, Account) {
	t.Helper()
	responsesKey, anthropicOAuth, _, _ := protocolMatchPool()
	entry := ModelCatalogEntry{ID: protocolMatchEntryID, ModelID: previewModel, Status: ModelCatalogStatusListed}
	unbound := ModelCatalogEntry{ID: 93, ModelID: "unbound-model", Status: ModelCatalogStatusListed}
	retired := ModelCatalogEntry{ID: 94, ModelID: "retired-model", Status: "retired"}
	catalog, repo := newTestModelCatalogService(entry, unbound, retired)
	repo.bindings = map[int64][]ModelCatalogBinding{
		protocolMatchEntryID: {{EntryID: protocolMatchEntryID, AccountID: responsesKey.ID}, {EntryID: protocolMatchEntryID, AccountID: anthropicOAuth.ID}},
		retired.ID:           {{EntryID: retired.ID, AccountID: anthropicOAuth.ID}},
	}
	gateway := newProtocolMatchService(t, true, nil, responsesKey, anthropicOAuth)
	return NewSchedulingOverviewService(catalog, gateway.accountRepo, gateway, traces, sticky), responsesKey, anthropicOAuth
}

// 请求数来自轨迹：按入站协议分开；失败只算 failed，没选到渠道算进 unrouted。
func TestSchedulingOverview_ComposesLanesTracesAndSticky(t *testing.T) {
	traces := &stubSchedulingTraceRepo{}
	_, oauth, _, _ := protocolMatchPool()
	svc, responsesKey, anthropicOAuth := newSchedulingOverviewFixture(t, traces, &stubStickyCounter{counts: map[int64]map[int64]int{protocolMatchEntryID: {oauth.ID: 2}}})
	traces.counts = []SchedulingTraceCount{
		{CatalogEntryID: protocolMatchEntryID, InboundProtocol: APIProtocolAnthropic, AccountID: anthropicOAuth.ID, Outcome: SchedulingTraceOutcomeSuccess, Requests: 3},
		{CatalogEntryID: protocolMatchEntryID, InboundProtocol: APIProtocolAnthropic, AccountID: anthropicOAuth.ID, Outcome: SchedulingTraceOutcomeFailed, Requests: 1},
		{CatalogEntryID: protocolMatchEntryID, InboundProtocol: APIProtocolChatCompletions, AccountID: anthropicOAuth.ID, Outcome: SchedulingTraceOutcomeCanceled, Requests: 2},
		{CatalogEntryID: protocolMatchEntryID, InboundProtocol: APIProtocolChatCompletions, AccountID: 0, Outcome: SchedulingTraceOutcomeNoChannel, Requests: 2},
		{CatalogEntryID: 0, InboundProtocol: APIProtocolResponses, AccountID: responsesKey.ID, Outcome: SchedulingTraceOutcomeSuccess, Requests: 5},
	}

	overview, err := svc.Overview(context.Background(), "15m")
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().Add(-15*time.Minute), traces.since, 5*time.Second)
	require.Equal(t, "15m", overview.Window)
	require.Empty(t, overview.Warnings)

	require.Len(t, overview.Models, 1, "没有绑定也没有请求的条目、没上架的条目都不列")
	model := overview.Models[0]
	require.Equal(t, previewModel, model.ModelID)
	require.Equal(t, int64(8), model.Requests)
	require.Equal(t, int64(1), model.Failures, "取消不算失败")
	require.Equal(t, int64(2), model.Unrouted)
	require.Equal(t, map[string]SchedulingCounts{
		APIProtocolAnthropic:       {Requests: 4, Failures: 1},
		APIProtocolChatCompletions: {Requests: 4, Unrouted: 2},
	}, model.ByProtocol)

	perChannel := map[int64]SchedulingModelChannel{}
	for _, channel := range model.Channels {
		perChannel[channel.AccountID] = channel
	}
	require.Equal(t, SchedulingModelChannel{
		AccountID: anthropicOAuth.ID, Requests: 6, Failures: 1, StickySessions: 2,
		ByProtocol: map[string]SchedulingCounts{APIProtocolAnthropic: {Requests: 4, Failures: 1}, APIProtocolChatCompletions: {Requests: 2}},
	}, perChannel[anthropicOAuth.ID])
	require.Equal(t, SchedulingModelChannel{AccountID: responsesKey.ID, ByProtocol: map[string]SchedulingCounts{}}, perChannel[responsesKey.ID], "没命中目录条目的请求不归到条目")

	lanes := map[string]SchedulingLane{}
	for _, lane := range model.Lanes {
		lanes[lane.Protocol] = lane
	}
	require.Contains(t, lanes, APIProtocolAnthropic)
	require.Equal(t, anthropicOAuth.ID, lanes[APIProtocolAnthropic].Items[0].AccountID)

	channels := map[int64]SchedulingChannel{}
	for _, channel := range overview.Channels {
		channels[channel.ID] = channel
	}
	require.Equal(t, int64(5), channels[responsesKey.ID].Requests, "渠道合计含没命中条目的请求")
	require.NotNil(t, channels[anthropicOAuth.ID].InUse)
}

func TestSchedulingOverview_RejectsUnknownWindow(t *testing.T) {
	svc, _, _ := newSchedulingOverviewFixture(t, &stubSchedulingTraceRepo{}, &stubStickyCounter{})
	_, err := svc.Overview(context.Background(), "7d")
	require.ErrorIs(t, err, ErrSchedulingWindowInvalid)
}

// 粘性会话数拿不到时其余照常返回，并带上提示，不静默显示成 0。
func TestSchedulingOverview_StickyFailureBecomesWarning(t *testing.T) {
	svc, _, _ := newSchedulingOverviewFixture(t, &stubSchedulingTraceRepo{}, &stubStickyCounter{err: errors.New("redis down")})
	overview, err := svc.Overview(context.Background(), "1h")
	require.NoError(t, err)
	require.Equal(t, []string{SchedulingWarningStickyUnavailable}, overview.Warnings)
	require.Len(t, overview.Models, 1)
}

func TestSchedulingListTraces_ValidatesAndPages(t *testing.T) {
	traces := &stubSchedulingTraceRepo{}
	svc, _, _ := newSchedulingOverviewFixture(t, traces, &stubStickyCounter{})

	page, err := svc.ListTraces(context.Background(), "24h", SchedulingTraceFilter{Outcome: "switched", InboundProtocol: APIProtocolResponses}, 3, 500)
	require.NoError(t, err)
	require.Equal(t, schedulingTraceMaxPageSize, page.PageSize)
	require.Equal(t, 3, page.Page)
	require.Equal(t, schedulingTraceMaxPageSize, traces.listed.Limit)
	require.Equal(t, 2*schedulingTraceMaxPageSize, traces.listed.Offset)
	require.WithinDuration(t, time.Now().Add(-24*time.Hour), traces.listed.Since, 5*time.Second)

	_, err = svc.ListTraces(context.Background(), "1h", SchedulingTraceFilter{Outcome: "exploded"}, 1, 20)
	require.ErrorIs(t, err, ErrSchedulingTraceFilterInvalid)
	_, err = svc.ListTraces(context.Background(), "1h", SchedulingTraceFilter{InboundProtocol: "smtp"}, 1, 20)
	require.ErrorIs(t, err, ErrSchedulingTraceFilterInvalid)
	_, err = svc.ListTraces(context.Background(), "7d", SchedulingTraceFilter{}, 1, 20)
	require.ErrorIs(t, err, ErrSchedulingWindowInvalid)
}
