//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type schedulerScorePoolRepoStub struct {
	accountRepoStub
	accounts       []Account
	groupedCalls   []int64
	ungroupedCalls int
	queryPlatforms [][]string
}

func (s *schedulerScorePoolRepoStub) ListSchedulingCandidatesByCatalogEntry(context.Context, int64) ([]Account, error) {
	return nil, nil
}

func (s *schedulerScorePoolRepoStub) ListSchedulingCandidatesByGroupID(_ context.Context, groupID int64, platforms []string) ([]Account, error) {
	s.groupedCalls = append(s.groupedCalls, groupID)
	s.queryPlatforms = append(s.queryPlatforms, platforms)
	return s.accounts, nil
}

func (s *schedulerScorePoolRepoStub) ListSchedulingCandidatesUngrouped(_ context.Context, platforms []string) ([]Account, error) {
	s.ungroupedCalls++
	s.queryPlatforms = append(s.queryPlatforms, platforms)
	return s.accounts, nil
}

// 管理端调度分候选池与调度同一口径：候选查询装载任意标签的 key，只保留能在 OpenAI 网关承接请求的。
func TestListOpenAISchedulableAccountsForSchedulerScore_UsesSchedulingCandidates(t *testing.T) {
	openAIOAuth := Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	relayResponsesKey := Account{ID: 2, Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
		ProtocolEndpoints: map[string]string{APIProtocolResponses: "https://relay.example.com/v1"}}
	geminiOnlyKey := Account{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		ProtocolEndpoints: map[string]string{APIProtocolGemini: "https://relay.example.com"}}
	repo := &schedulerScorePoolRepoStub{accounts: []Account{openAIOAuth, relayResponsesKey, geminiOnlyKey}}
	svc := &adminServiceImpl{accountRepo: repo}

	groupID := int64(77)
	pool, err := svc.ListOpenAISchedulableAccountsForSchedulerScore(context.Background(), &groupID)
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, accountIDs(pool))
	require.Equal(t, []int64{groupID}, repo.groupedCalls)

	pool, err = svc.ListOpenAISchedulableAccountsForSchedulerScore(context.Background(), nil)
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, accountIDs(pool))
	require.Equal(t, 1, repo.ungroupedCalls)
	require.Equal(t, [][]string{{PlatformOpenAI}, {PlatformOpenAI}}, repo.queryPlatforms)
}
