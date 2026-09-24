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

// 去分组后候选集不再按分组切分：夹具只区分成品号与第三方 key、可调度与不可调度。
type schedulingCandidateFixture struct {
	subAnthropic   int64
	subOpenAI      int64
	keyOpenAILabel int64 // 标签 openai 的第三方 key
	keyGeminiLabel int64 // 标签 gemini 的第三方 key
	keyKimiLabel   int64
	keyGrokLabel   int64
	disabledKey    int64
}

func (s *SchedulingCandidatesSuite) createFixture() schedulingCandidateFixture {
	t := s.T()
	create := func(name, platform, accountType string) int64 {
		account := mustCreateAccount(t, s.client, &service.Account{Name: name, Platform: platform, Type: accountType})
		return account.ID
	}
	var f schedulingCandidateFixture
	f.subAnthropic = create("sub-anthropic", service.PlatformAnthropic, service.AccountTypeOAuth)
	f.subOpenAI = create("sub-openai", service.PlatformOpenAI, service.AccountTypeOAuth)
	f.keyOpenAILabel = create("key-openai-label", service.PlatformOpenAI, service.AccountTypeAPIKey)
	f.keyGeminiLabel = create("key-gemini-label", service.PlatformGemini, service.AccountTypeAPIKey)
	f.keyKimiLabel = create("key-kimi-label", service.PlatformKimi, service.AccountTypeAPIKey)
	f.keyGrokLabel = create("key-grok-label", service.PlatformGrok, service.AccountTypeAPIKey)
	f.disabledKey = create("disabled-key", service.PlatformOpenAI, service.AccountTypeAPIKey)
	s.Require().NoError(s.client.Account.UpdateOneID(f.disabledKey).SetSchedulable(false).Exec(s.ctx))
	return f
}

func candidateIDs(accounts []service.Account) map[int64]struct{} {
	ids := make(map[int64]struct{}, len(accounts))
	for _, account := range accounts {
		ids[account.ID] = struct{}{}
	}
	return ids
}

// 平台过滤只约束成品号：任意平台标签的第三方 key 都进候选（能否承接由选号时的协议判断定）。
func (s *SchedulingCandidatesSuite) TestPlatformFilterOnlyConstrainsSubscriptions() {
	f := s.createFixture()

	accounts, err := s.accountRepo.ListSchedulingCandidates(s.ctx, []string{service.PlatformAnthropic})
	s.Require().NoError(err)

	ids := candidateIDs(accounts)
	for _, id := range []int64{f.subAnthropic, f.keyOpenAILabel, f.keyGeminiLabel, f.keyKimiLabel, f.keyGrokLabel} {
		s.Require().Contains(ids, id)
	}
	for _, id := range []int64{f.subOpenAI, f.disabledKey} {
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
