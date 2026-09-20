//go:build integration

package repository

import (
	"context"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/suite"
)

// SchedulingCandidatesSuite 验证调度候选查询：平台过滤只约束成品号，
// 任意平台标签的第三方 key 都进入候选（能否承接请求由选号时的协议判断决定）。
type SchedulingCandidatesSuite struct {
	suite.Suite
	ctx         context.Context
	client      *dbent.Client
	accountRepo *accountRepository
}

func (s *SchedulingCandidatesSuite) SetupTest() {
	s.ctx = context.Background()
	tx := testEntTx(s.T())
	s.client = tx.Client()
	s.accountRepo = newAccountRepositoryWithSQL(s.client, tx, nil)
}

func TestSchedulingCandidatesSuite(t *testing.T) {
	suite.Run(t, new(SchedulingCandidatesSuite))
}

type schedulingCandidateFixture struct {
	groupID int64

	subAnthropicInGroup   int64
	subOpenAIInGroup      int64
	keyOpenAIInGroup      int64 // 标签 openai 的第三方 key
	keyGeminiInGroup      int64 // 标签 gemini 的第三方 key
	disabledKeyInGroup    int64
	keyUngrouped          int64
	subAnthropicUngrouped int64
	keyInOtherGroup       int64
}

func (s *SchedulingCandidatesSuite) createFixture() schedulingCandidateFixture {
	t := s.T()
	group := mustCreateGroup(t, s.client, &service.Group{Name: "candidates-anthropic", Platform: service.PlatformAnthropic})
	otherGroup := mustCreateGroup(t, s.client, &service.Group{Name: "candidates-other", Platform: service.PlatformGrok})

	create := func(name, platform, accountType string, groupID int64) int64 {
		account := mustCreateAccount(t, s.client, &service.Account{Name: name, Platform: platform, Type: accountType})
		if groupID > 0 {
			mustBindAccountToGroup(t, s.client, account.ID, groupID, 1)
		}
		return account.ID
	}
	f := schedulingCandidateFixture{groupID: group.ID}
	f.subAnthropicInGroup = create("sub-anthropic", service.PlatformAnthropic, service.AccountTypeOAuth, group.ID)
	f.subOpenAIInGroup = create("sub-openai", service.PlatformOpenAI, service.AccountTypeOAuth, group.ID)
	f.keyOpenAIInGroup = create("key-openai-label", service.PlatformOpenAI, service.AccountTypeAPIKey, group.ID)
	f.keyGeminiInGroup = create("key-gemini-label", service.PlatformGemini, service.AccountTypeAPIKey, group.ID)
	f.disabledKeyInGroup = create("disabled-key", service.PlatformOpenAI, service.AccountTypeAPIKey, group.ID)
	s.Require().NoError(s.client.Account.UpdateOneID(f.disabledKeyInGroup).SetSchedulable(false).Exec(s.ctx))
	f.keyUngrouped = create("key-ungrouped", service.PlatformKimi, service.AccountTypeAPIKey, 0)
	f.subAnthropicUngrouped = create("sub-anthropic-ungrouped", service.PlatformAnthropic, service.AccountTypeOAuth, 0)
	f.keyInOtherGroup = create("key-other-group", service.PlatformGrok, service.AccountTypeAPIKey, otherGroup.ID)
	return f
}

func candidateIDs(accounts []service.Account) map[int64]struct{} {
	ids := make(map[int64]struct{}, len(accounts))
	for _, account := range accounts {
		ids[account.ID] = struct{}{}
	}
	return ids
}

func (s *SchedulingCandidatesSuite) TestByGroupIDIncludesKeysOfAnyLabel() {
	f := s.createFixture()

	accounts, err := s.accountRepo.ListSchedulingCandidatesByGroupID(s.ctx, f.groupID, []string{service.PlatformAnthropic})
	s.Require().NoError(err)

	ids := candidateIDs(accounts)
	s.Require().Len(ids, 3)
	for _, id := range []int64{f.subAnthropicInGroup, f.keyOpenAIInGroup, f.keyGeminiInGroup} {
		s.Require().Contains(ids, id)
	}
}

func (s *SchedulingCandidatesSuite) TestUngroupedIncludesUngroupedKeysOnly() {
	f := s.createFixture()

	accounts, err := s.accountRepo.ListSchedulingCandidatesUngrouped(s.ctx, []string{service.PlatformAnthropic})
	s.Require().NoError(err)

	ids := candidateIDs(accounts)
	s.Require().Contains(ids, f.keyUngrouped)
	s.Require().Contains(ids, f.subAnthropicUngrouped)
	for _, id := range []int64{f.subAnthropicInGroup, f.keyOpenAIInGroup, f.keyInOtherGroup} {
		s.Require().NotContains(ids, id)
	}
}

func (s *SchedulingCandidatesSuite) TestAllAccountsIncludesKeysAcrossGroups() {
	f := s.createFixture()

	accounts, err := s.accountRepo.ListSchedulingCandidates(s.ctx, []string{service.PlatformAnthropic})
	s.Require().NoError(err)

	ids := candidateIDs(accounts)
	for _, id := range []int64{f.subAnthropicInGroup, f.keyOpenAIInGroup, f.keyGeminiInGroup, f.keyUngrouped, f.subAnthropicUngrouped, f.keyInOtherGroup} {
		s.Require().Contains(ids, id)
	}
	for _, id := range []int64{f.subOpenAIInGroup, f.disabledKeyInGroup} {
		s.Require().NotContains(ids, id)
	}
}
