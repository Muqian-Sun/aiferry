//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSchedulingTraceRepositoryRoundTripFiltersAndCounts(t *testing.T) {
	ctx := context.Background()
	repo := NewSchedulingTraceRepository(integrationDB)
	entryID := time.Now().UnixNano() % 1_000_000_000
	prefix := fmt.Sprintf("sched-trace-%d-", entryID)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM scheduling_traces WHERE request_id LIKE $1`, prefix+"%")
	})
	now := time.Now().UTC().Truncate(time.Millisecond)
	account := func(id int64) *int64 { return &id }
	traces := []service.SchedulingTrace{
		{
			CreatedAt: now.Add(-time.Minute), RequestID: prefix + "a", Model: "m", CatalogEntryID: &entryID,
			InboundProtocol: service.APIProtocolAnthropic, Outcome: service.SchedulingTraceOutcomeSuccess, StatusCode: 200,
			FinalAccountID: account(12), SwitchCount: 1, DurationMs: 900,
			Picks: []service.SchedulingTracePick{
				{AtMs: 1, AccountID: 11, Layer: service.SchedulingTraceLayerLoad, Candidates: []service.SchedulingTraceCandidate{{AccountID: 11, Decision: service.SchedulingTracePicked}}},
				{AtMs: 500, AccountID: 12, Layer: service.SchedulingTraceLayerLoad, Candidates: []service.SchedulingTraceCandidate{{AccountID: 11, Decision: service.SchedulingTraceExcluded}, {AccountID: 12, Decision: service.SchedulingTracePicked}}},
			},
			Attempts: []service.SchedulingTraceAttempt{{AtMs: 480, AccountID: 11, StatusCode: 429, Kind: "failover"}},
		},
		{
			CreatedAt: now.Add(-2 * time.Minute), RequestID: prefix + "b", Model: "m", CatalogEntryID: &entryID,
			InboundProtocol: service.APIProtocolChatCompletions, Outcome: service.SchedulingTraceOutcomeNoChannel, StatusCode: 503,
			Picks: []service.SchedulingTracePick{{AtMs: 1, Error: "no_available_accounts", Candidates: []service.SchedulingTraceCandidate{}}}, Attempts: []service.SchedulingTraceAttempt{},
		},
		{
			CreatedAt: now.Add(-3 * time.Hour), RequestID: prefix + "old", Model: "m", CatalogEntryID: &entryID,
			InboundProtocol: service.APIProtocolAnthropic, Outcome: service.SchedulingTraceOutcomeSuccess, StatusCode: 200, FinalAccountID: account(11),
			Picks: []service.SchedulingTracePick{}, Attempts: []service.SchedulingTraceAttempt{},
		},
	}
	require.NoError(t, repo.InsertSchedulingTraces(ctx, traces))

	since := now.Add(-time.Hour)
	items, total, err := repo.ListSchedulingTraces(ctx, service.SchedulingTraceFilter{Since: since, CatalogEntryID: entryID, Limit: 10})
	require.NoError(t, err)
	require.Equal(t, int64(2), total, "窗外的不算")
	require.Len(t, items, 2)
	require.Equal(t, prefix+"a", items[0].RequestID, "新的在前")
	require.Equal(t, traces[0].Picks, items[0].Picks)
	require.Equal(t, traces[0].Attempts, items[0].Attempts)
	require.Equal(t, int64(12), *items[0].FinalAccountID)
	require.Nil(t, items[1].FinalAccountID)
	require.True(t, items[0].CreatedAt.Equal(traces[0].CreatedAt))

	filtered := func(filter service.SchedulingTraceFilter) []string {
		filter.Since, filter.CatalogEntryID, filter.Limit = since, entryID, 10
		items, _, err := repo.ListSchedulingTraces(ctx, filter)
		require.NoError(t, err)
		ids := []string{}
		for _, item := range items {
			ids = append(ids, item.RequestID)
		}
		return ids
	}
	require.Equal(t, []string{prefix + "a"}, filtered(service.SchedulingTraceFilter{Outcome: "switched"}))
	require.Equal(t, []string{prefix + "b"}, filtered(service.SchedulingTraceFilter{Outcome: service.SchedulingTraceOutcomeNoChannel}))
	require.Equal(t, []string{prefix + "b"}, filtered(service.SchedulingTraceFilter{InboundProtocol: service.APIProtocolChatCompletions}))
	require.Equal(t, []string{prefix + "a"}, filtered(service.SchedulingTraceFilter{AccountID: 12}))

	page, _, err := repo.ListSchedulingTraces(ctx, service.SchedulingTraceFilter{Since: since, CatalogEntryID: entryID, Limit: 1, Offset: 1})
	require.NoError(t, err)
	require.Len(t, page, 1)
	require.Equal(t, prefix+"b", page[0].RequestID)

	counts, err := repo.CountSchedulingTraces(ctx, since)
	require.NoError(t, err)
	var mine []service.SchedulingTraceCount
	for _, c := range counts {
		if c.CatalogEntryID == entryID {
			mine = append(mine, c)
		}
	}
	require.ElementsMatch(t, []service.SchedulingTraceCount{
		{CatalogEntryID: entryID, InboundProtocol: service.APIProtocolAnthropic, AccountID: 12, Outcome: service.SchedulingTraceOutcomeSuccess, Requests: 1},
		{CatalogEntryID: entryID, InboundProtocol: service.APIProtocolChatCompletions, AccountID: 0, Outcome: service.SchedulingTraceOutcomeNoChannel, Requests: 1},
	}, mine)

	_, err = repo.DeleteSchedulingTracesBefore(ctx, now.Add(-time.Hour))
	require.NoError(t, err)
	var left int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM scheduling_traces WHERE request_id LIKE $1`, prefix+"%").Scan(&left))
	require.Equal(t, 2, left, "只删保留期之前的")
}
