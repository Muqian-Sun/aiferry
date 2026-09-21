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

// 按目录条目取候选：只有绑定的账号进入，绑定优先级覆盖账号优先级，不活跃 / 不可调度的
// 账号排除，且不看平台标签；账号上装载 CatalogEntryIDs。
func (s *SchedulingCandidatesSuite) TestListSchedulingCandidatesByCatalogEntry() {
	t := s.T()
	entry, err := s.client.ModelCatalogEntry.Create().
		SetModelID("candidates-catalog-entry").
		SetStatus(service.ModelCatalogStatusListed).
		SetManagedBy(service.ModelCatalogManagedByAdmin).
		SetInputPrice(1e-6).
		Save(s.ctx)
	s.Require().NoError(err)
	otherEntry, err := s.client.ModelCatalogEntry.Create().
		SetModelID("candidates-catalog-other").
		SetStatus(service.ModelCatalogStatusUnlisted).
		SetManagedBy(service.ModelCatalogManagedByAdmin).
		Save(s.ctx)
	s.Require().NoError(err)

	bound := func(name, platform, accountType string, accountPriority int, bindingPriority *int, entryID int64) int64 {
		account := mustCreateAccount(t, s.client, &service.Account{Name: name, Platform: platform, Type: accountType, Priority: accountPriority})
		create := s.client.ModelCatalogBinding.Create().SetEntryID(entryID).SetAccountID(account.ID)
		if bindingPriority != nil {
			create.SetPriority(*bindingPriority)
		}
		_, err := create.Save(s.ctx)
		s.Require().NoError(err)
		return account.ID
	}
	one := 1
	// 账号优先级 90，但绑定优先级 1，应排到最前。
	keyLowAccountHighBinding := bound("key-binding-priority", service.PlatformOpenAI, service.AccountTypeAPIKey, 90, &one, entry.ID)
	subAnthropic := bound("sub-anthropic-bound", service.PlatformAnthropic, service.AccountTypeOAuth, 10, nil, entry.ID)
	keyGemini := bound("key-gemini-bound", service.PlatformGemini, service.AccountTypeAPIKey, 20, nil, entry.ID)
	inactive := mustCreateAccount(t, s.client, &service.Account{Name: "inactive-bound", Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth, Status: service.StatusDisabled})
	_, err = s.client.ModelCatalogBinding.Create().SetEntryID(entry.ID).SetAccountID(inactive.ID).Save(s.ctx)
	s.Require().NoError(err)
	unschedulable := mustCreateAccount(t, s.client, &service.Account{Name: "unschedulable-bound", Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth})
	_, err = s.client.Account.UpdateOneID(unschedulable.ID).SetSchedulable(false).Save(s.ctx)
	s.Require().NoError(err)
	_, err = s.client.ModelCatalogBinding.Create().SetEntryID(entry.ID).SetAccountID(unschedulable.ID).Save(s.ctx)
	s.Require().NoError(err)
	unbound := mustCreateAccount(t, s.client, &service.Account{Name: "unbound", Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth})
	_ = bound("bound-elsewhere", service.PlatformAnthropic, service.AccountTypeOAuth, 5, nil, otherEntry.ID)

	accounts, err := s.accountRepo.ListSchedulingCandidatesByCatalogEntry(s.ctx, entry.ID)
	s.Require().NoError(err)
	ids := make([]int64, 0, len(accounts))
	for _, account := range accounts {
		ids = append(ids, account.ID)
	}
	s.Require().Equal([]int64{keyLowAccountHighBinding, subAnthropic, keyGemini}, ids,
		"binding priority overrides account priority; inactive / unschedulable / unbound / other-entry accounts are absent")
	s.Require().Equal(1, accounts[0].Priority, "effective priority is written back")
	s.Require().Equal(10, accounts[1].Priority)
	for _, account := range accounts {
		s.Require().Equal([]int64{entry.ID}, account.CatalogEntryIDs)
	}

	fresh, err := s.accountRepo.GetByID(s.ctx, unbound.ID)
	s.Require().NoError(err)
	s.Require().Empty(fresh.CatalogEntryIDs)

	empty, err := s.accountRepo.ListSchedulingCandidatesByCatalogEntry(s.ctx, otherEntry.ID+1000)
	s.Require().NoError(err)
	s.Require().Empty(empty)
}
