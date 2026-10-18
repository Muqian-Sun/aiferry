//go:build unit

package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func traceDecisions(pick SchedulingTracePick) map[int64]string {
	out := map[int64]string{}
	for _, candidate := range pick.Candidates {
		out[candidate.AccountID] = candidate.Decision
		if candidate.Reason != "" {
			out[candidate.AccountID] += ":" + candidate.Reason
		}
	}
	return out
}

func tracedSelectCtx(inbound string) (context.Context, *SchedulingTraceRecorder) {
	return WithSchedulingTrace(catalogRouteCtx(protocolMatchEntryID, inbound), time.Now())
}

// 按负载挑：记下被排除的、准入不过的（带原因）、选中的和没轮到的。
func TestSchedulingTrace_RecordsLoadAwarePick(t *testing.T) {
	responsesKey, anthropicOAuth, geminiOAuth, _ := protocolMatchPool()
	disabled := anthropicOAuth
	disabled.ID, disabled.Status = 93001, StatusDisabled
	svc := newProtocolMatchService(t, true, nil, responsesKey, anthropicOAuth, geminiOAuth, disabled)

	ctx, recorder := tracedSelectCtx(APIProtocolAnthropic)
	excluded := map[int64]struct{}{geminiOAuth.ID: {}}
	result, err := svc.SelectAccountWithLoadAwareness(ctx, "", "claude-sonnet-4-5", excluded)
	require.NoError(t, err)
	require.Equal(t, anthropicOAuth.ID, result.Account.ID)

	picks := recorder.Picks()
	require.Len(t, picks, 1)
	require.Equal(t, SchedulingTraceLayerLoad, picks[0].Layer)
	require.Equal(t, anthropicOAuth.ID, picks[0].AccountID)
	require.Empty(t, picks[0].Error)
	require.Equal(t, map[int64]string{
		responsesKey.ID:   SchedulingTraceEligible,
		anthropicOAuth.ID: SchedulingTracePicked,
		geminiOAuth.ID:    SchedulingTraceExcluded,
		disabled.ID:       SchedulingTraceSkipped + ":not_schedulable",
	}, traceDecisions(picks[0]))
	for _, candidate := range picks[0].Candidates {
		if candidate.AccountID == anthropicOAuth.ID {
			require.NotNil(t, candidate.LoadRate, "按负载挑时记下负载率")
		}
	}
}

func TestSchedulingTrace_RecordsStickyHit(t *testing.T) {
	responsesKey, anthropicOAuth, _, _ := protocolMatchPool()
	cache := &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{"session-1": responsesKey.ID}}
	svc := newProtocolMatchService(t, true, cache, responsesKey, anthropicOAuth)

	ctx, recorder := tracedSelectCtx(APIProtocolAnthropic)
	result, err := svc.SelectAccountWithLoadAwareness(ctx, "session-1", "claude-sonnet-4-5", nil)
	require.NoError(t, err)
	require.Equal(t, responsesKey.ID, result.Account.ID)

	picks := recorder.Picks()
	require.Len(t, picks, 1)
	require.Equal(t, SchedulingTraceLayerSticky, picks[0].Layer)
	require.Equal(t, &SchedulingTraceSticky{AccountID: responsesKey.ID, Outcome: SchedulingTraceStickyHit}, picks[0].Sticky)
	require.Empty(t, picks[0].Candidates, "粘性命中不走候选筛选")
}

// 粘性绑定的渠道这次被排除（本请求里失败过）：记 miss 与原因，转去按负载挑。
func TestSchedulingTrace_RecordsStickyMissWhenExcluded(t *testing.T) {
	responsesKey, anthropicOAuth, _, _ := protocolMatchPool()
	cache := &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{"session-1": responsesKey.ID}}
	svc := newProtocolMatchService(t, true, cache, responsesKey, anthropicOAuth)

	ctx, recorder := tracedSelectCtx(APIProtocolAnthropic)
	result, err := svc.SelectAccountWithLoadAwareness(ctx, "session-1", "claude-sonnet-4-5", map[int64]struct{}{responsesKey.ID: {}})
	require.NoError(t, err)
	require.Equal(t, anthropicOAuth.ID, result.Account.ID)

	pick := recorder.Picks()[0]
	require.Equal(t, &SchedulingTraceSticky{AccountID: responsesKey.ID, Outcome: SchedulingTraceStickyMiss, Reason: SchedulingTraceExcluded}, pick.Sticky)
	require.Equal(t, SchedulingTraceLayerLoad, pick.Layer)
}

func TestSchedulingTrace_RecordsLegacyOrder(t *testing.T) {
	responsesKey, anthropicOAuth, _, _ := protocolMatchPool()
	svc := newProtocolMatchService(t, false, nil, responsesKey, anthropicOAuth)

	ctx, recorder := tracedSelectCtx(APIProtocolAnthropic)
	_, err := svc.SelectAccountWithLoadAwareness(ctx, "", "claude-sonnet-4-5", nil)
	require.NoError(t, err)
	pick := recorder.Picks()[0]
	require.Equal(t, SchedulingTraceLayerLegacy, pick.Layer)
	require.Equal(t, SchedulingTracePicked, traceDecisions(pick)[anthropicOAuth.ID])
}

// 槽都抢不到：记下 slot_busy，最后排队的那个记 queued。
func TestSchedulingTrace_RecordsQueueFallback(t *testing.T) {
	responsesKey, anthropicOAuth, _, _ := protocolMatchPool()
	svc := newProtocolMatchService(t, true, nil, responsesKey, anthropicOAuth)
	svc.concurrencyService = NewConcurrencyService(&mockConcurrencyCache{acquireResults: map[int64]bool{responsesKey.ID: false, anthropicOAuth.ID: false}})

	ctx, recorder := tracedSelectCtx(APIProtocolAnthropic)
	result, err := svc.SelectAccountWithLoadAwareness(ctx, "", "claude-sonnet-4-5", nil)
	require.NoError(t, err)
	require.NotNil(t, result.WaitPlan)

	pick := recorder.Picks()[0]
	require.Equal(t, SchedulingTraceLayerQueue, pick.Layer)
	decisions := traceDecisions(pick)
	require.Equal(t, SchedulingTraceQueued, decisions[result.Account.ID])
	for id, decision := range decisions {
		if id != result.Account.ID {
			require.Equal(t, SchedulingTraceSlotBusy, decision)
		}
	}
}

func TestSchedulingTrace_RecordsNoAvailableChannel(t *testing.T) {
	responsesKey, anthropicOAuth, _, _ := protocolMatchPool()
	svc := newProtocolMatchService(t, true, nil, responsesKey, anthropicOAuth)

	ctx, recorder := tracedSelectCtx(APIProtocolAnthropic)
	_, err := svc.SelectAccountWithLoadAwareness(ctx, "", "claude-sonnet-4-5", map[int64]struct{}{responsesKey.ID: {}, anthropicOAuth.ID: {}})
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	pick := recorder.Picks()[0]
	require.Zero(t, pick.AccountID)
	require.Equal(t, "no_available_accounts", pick.Error)
	require.Equal(t, map[int64]string{responsesKey.ID: SchedulingTraceExcluded, anthropicOAuth.ID: SchedulingTraceExcluded}, traceDecisions(pick))
}

func newBuildRecorder(picks ...SchedulingTracePick) (context.Context, *SchedulingTraceRecorder) {
	ctx := WithCatalogRoute(context.Background(), CatalogRoute{EntryID: 41, CanonicalModel: "m", RequestedModel: "m-alias"})
	ctx = WithInboundProtocol(ctx, APIProtocolChatCompletions)
	ctx, recorder := WithSchedulingTrace(ctx, time.Now().Add(-2*time.Second))
	for _, pick := range picks {
		recorder.add(pick)
	}
	return ctx, recorder
}

// 换过一次渠道后成功：最终渠道是最后选中的，失败的尝试按发生时间换算，错误信息截断。
func TestSchedulingTraceBuild_SwitchedThenSucceeded(t *testing.T) {
	ctx, recorder := newBuildRecorder(
		SchedulingTracePick{AccountID: 11, Layer: SchedulingTraceLayerSticky},
		SchedulingTracePick{AccountID: 12, Layer: SchedulingTraceLayerLoad},
	)
	userID, keyID := int64(7), int64(8)
	trace, ok := recorder.Build(ctx, SchedulingTraceInput{
		RequestID: "req-1", UserID: &userID, APIKeyID: &keyID, StatusCode: 200, End: time.Now(),
		UpstreamErrors: []*OpsUpstreamErrorEvent{nil, {
			AtUnixMs: recorder.Start().UnixMilli() + 1500, AccountID: 11, UpstreamStatusCode: 429, Kind: "failover",
			Message: strings.Repeat("x", schedulingTraceMessageLimit+50),
		}},
	})
	require.True(t, ok)
	require.Equal(t, SchedulingTraceOutcomeSuccess, trace.Outcome)
	require.Equal(t, int64(12), *trace.FinalAccountID)
	require.Equal(t, 1, trace.SwitchCount)
	require.True(t, trace.StickyHit)
	require.Equal(t, "m-alias", trace.Model)
	require.Equal(t, int64(41), *trace.CatalogEntryID)
	require.Equal(t, APIProtocolChatCompletions, trace.InboundProtocol)
	require.Equal(t, "req-1", trace.RequestID)
	require.Equal(t, &userID, trace.UserID)
	require.GreaterOrEqual(t, trace.DurationMs, int64(2000))
	require.Len(t, trace.Attempts, 1)
	require.Equal(t, SchedulingTraceAttempt{AtMs: 1500, AccountID: 11, StatusCode: 429, Kind: "failover", Message: strings.Repeat("x", schedulingTraceMessageLimit)}, trace.Attempts[0])
}

func TestSchedulingTraceBuild_Outcomes(t *testing.T) {
	picked := SchedulingTracePick{AccountID: 11, Layer: SchedulingTraceLayerLoad}
	none := SchedulingTracePick{Error: "no_available_accounts"}
	cases := []struct {
		name  string
		picks []SchedulingTracePick
		input SchedulingTraceInput
		want  string
	}{
		{"upstream error status", []SchedulingTracePick{picked}, SchedulingTraceInput{StatusCode: 502}, SchedulingTraceOutcomeFailed},
		{"stream failed after 200", []SchedulingTracePick{picked}, SchedulingTraceInput{StatusCode: 200, StreamFailed: true}, SchedulingTraceOutcomeFailed},
		{"client gone", []SchedulingTracePick{picked}, SchedulingTraceInput{StatusCode: 499}, SchedulingTraceOutcomeCanceled},
		{"context canceled", []SchedulingTracePick{picked}, SchedulingTraceInput{StatusCode: 200, Canceled: true}, SchedulingTraceOutcomeCanceled},
		{"never got a channel", []SchedulingTracePick{none}, SchedulingTraceInput{StatusCode: 503}, SchedulingTraceOutcomeNoChannel},
		{"ran out after trying one", []SchedulingTracePick{picked, none}, SchedulingTraceInput{StatusCode: 503}, SchedulingTraceOutcomeFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, recorder := newBuildRecorder(tc.picks...)
			tc.input.End = time.Now()
			trace, ok := recorder.Build(ctx, tc.input)
			require.True(t, ok)
			require.Equal(t, tc.want, trace.Outcome)
		})
	}
}

func TestSchedulingTraceBuild_SkipsRequestsThatNeverReachedScheduler(t *testing.T) {
	ctx, recorder := newBuildRecorder()
	_, ok := recorder.Build(ctx, SchedulingTraceInput{StatusCode: 400, End: time.Now()})
	require.False(t, ok)
}

// Stop 先把队列里剩下的写完。
func TestSchedulingTraceWriter_FlushesOnStop(t *testing.T) {
	repo := &stubSchedulingTraceRepo{}
	writer := NewSchedulingTraceWriter(repo)
	for i := 0; i < 3; i++ {
		writer.Enqueue(SchedulingTrace{RequestID: "r", Outcome: SchedulingTraceOutcomeSuccess})
	}
	writer.Stop()
	require.Len(t, repo.insertedTraces(), 3)
	require.Zero(t, writer.Dropped())
}

// 队列满时丢弃并计数，不阻塞请求。
func TestSchedulingTraceWriter_DropsWhenQueueFull(t *testing.T) {
	writer := &SchedulingTraceWriter{queue: make(chan SchedulingTrace, 1)}
	writer.Enqueue(SchedulingTrace{})
	writer.Enqueue(SchedulingTrace{})
	writer.Enqueue(SchedulingTrace{})
	require.Equal(t, int64(2), writer.Dropped())
}

// 没挂记录器时调度照常，什么也不记。
func TestSchedulingTrace_NoRecorderIsNoop(t *testing.T) {
	responsesKey, anthropicOAuth, _, _ := protocolMatchPool()
	svc := newProtocolMatchService(t, true, nil, responsesKey, anthropicOAuth)
	result, err := svc.SelectAccountWithLoadAwareness(catalogRouteCtx(protocolMatchEntryID, APIProtocolAnthropic), "", "claude-sonnet-4-5", nil)
	require.NoError(t, err)
	require.Equal(t, anthropicOAuth.ID, result.Account.ID)
	require.Nil(t, beginSchedulingTracePick(context.Background()))
}
