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

	subAnthropicInGroup     int64
	subOpenAIInGroup        int64
	keyNullSourceInGroup    int64 // source_kind 为 NULL，按类型推导为第三方 key
	keyExplicitInGroup      int64 // source_kind 显式为 api_key
	explicitSubscriptionKey int64 // 类型是 apikey，但 source_kind 显式为 subscription
	disabledKeyInGroup      int64
	keyUngrouped            int64
	subAnthropicUngrouped   int64
	keyInOtherGroup         int64
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
	setSourceKind := func(id int64, kind string) {
		s.Require().NoError(s.client.Account.UpdateOneID(id).SetSourceKind(kind).Exec(s.ctx))
	}

	f := schedulingCandidateFixture{groupID: group.ID}
	f.subAnthropicInGroup = create("sub-anthropic", service.PlatformAnthropic, service.AccountTypeOAuth, group.ID)
	f.subOpenAIInGroup = create("sub-openai", service.PlatformOpenAI, service.AccountTypeOAuth, group.ID)
	f.keyNullSourceInGroup = create("key-openai-label", service.PlatformOpenAI, service.AccountTypeAPIKey, group.ID)
	f.keyExplicitInGroup = create("key-gemini-label", service.PlatformGemini, service.AccountTypeAPIKey, group.ID)
	setSourceKind(f.keyExplicitInGroup, service.AccountSourceAPIKey)
	f.explicitSubscriptionKey = create("apikey-typed-subscription", service.PlatformKimi, service.AccountTypeAPIKey, group.ID)
	setSourceKind(f.explicitSubscriptionKey, service.AccountSourceSubscription)
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
	for _, id := range []int64{f.subAnthropicInGroup, f.keyNullSourceInGroup, f.keyExplicitInGroup} {
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
	for _, id := range []int64{f.subAnthropicInGroup, f.keyNullSourceInGroup, f.keyInOtherGroup, f.explicitSubscriptionKey} {
		s.Require().NotContains(ids, id)
	}
}

func (s *SchedulingCandidatesSuite) TestAllAccountsIncludesKeysAcrossGroups() {
	f := s.createFixture()

	accounts, err := s.accountRepo.ListSchedulingCandidates(s.ctx, []string{service.PlatformAnthropic})
	s.Require().NoError(err)

	ids := candidateIDs(accounts)
	for _, id := range []int64{f.subAnthropicInGroup, f.keyNullSourceInGroup, f.keyExplicitInGroup, f.keyUngrouped, f.subAnthropicUngrouped, f.keyInOtherGroup} {
		s.Require().Contains(ids, id)
	}
	for _, id := range []int64{f.subOpenAIInGroup, f.explicitSubscriptionKey, f.disabledKeyInGroup} {
		s.Require().NotContains(ids, id)
	}
}
