//go:build integration

package repository

import (
	"context"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbmodelcatalogbinding "github.com/Wei-Shaw/sub2api/ent/modelcatalogbinding"
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

// 按目录条目取候选：只有绑定的账号进入，按账号自己的优先级、再按 ID 排序（承接关系上不再有优先级），
// 不活跃 / 不可调度的账号排除，且不看平台标签；账号上装载 CatalogEntryIDs 与承接关系上改了名的上游名。
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

	// 承接关系必须带上游价（输入 / 输出 NOT NULL）；这里只关心候选集与排序，价随便填。
	bind := func(accountID, entryID int64) {
		_, err := s.client.ModelCatalogBinding.Create().
			SetEntryID(entryID).
			SetAccountID(accountID).
			SetInputPrice(1e-6).
			SetOutputPrice(2e-6).
			Save(s.ctx)
		s.Require().NoError(err)
	}
	bound := func(name, platform, accountType string, accountPriority int, entryID int64) int64 {
		account := mustCreateAccount(t, s.client, &service.Account{Name: name, Platform: platform, Type: accountType, Priority: accountPriority})
		bind(account.ID, entryID)
		return account.ID
	}
	// 先建的账号优先级 90，排最后；两个优先级 20 的按 ID 排。
	keyLowPriority := bound("key-low-priority", service.PlatformOpenAI, service.AccountTypeAPIKey, 90, entry.ID)
	subAnthropic := bound("sub-anthropic-bound", service.PlatformAnthropic, service.AccountTypeOAuth, 10, entry.ID)
	keyGemini := bound("key-gemini-bound", service.PlatformGemini, service.AccountTypeAPIKey, 20, entry.ID)
	keyKimi := bound("key-kimi-bound", service.PlatformKimi, service.AccountTypeAPIKey, 20, entry.ID)
	inactive := mustCreateAccount(t, s.client, &service.Account{Name: "inactive-bound", Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth, Status: service.StatusDisabled})
	bind(inactive.ID, entry.ID)
	unschedulable := mustCreateAccount(t, s.client, &service.Account{Name: "unschedulable-bound", Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth})
	_, err = s.client.Account.UpdateOneID(unschedulable.ID).SetSchedulable(false).Save(s.ctx)
	s.Require().NoError(err)
	bind(unschedulable.ID, entry.ID)
	unbound := mustCreateAccount(t, s.client, &service.Account{Name: "unbound", Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth})
	_ = bound("bound-elsewhere", service.PlatformAnthropic, service.AccountTypeOAuth, 5, otherEntry.ID)

	// keyGemini 在两个条目上都改了名（上游名的键 = 条目的模型标识），其余账号没改名
	_, err = s.client.ModelCatalogBinding.Update().
		Where(dbmodelcatalogbinding.EntryIDEQ(entry.ID), dbmodelcatalogbinding.AccountIDEQ(keyGemini)).
		SetUpstreamModel("relay-main").Save(s.ctx)
	s.Require().NoError(err)
	bind(keyGemini, otherEntry.ID)
	_, err = s.client.ModelCatalogBinding.Update().
		Where(dbmodelcatalogbinding.EntryIDEQ(otherEntry.ID), dbmodelcatalogbinding.AccountIDEQ(keyGemini)).
		SetUpstreamModel("relay-other").Save(s.ctx)
	s.Require().NoError(err)

	accounts, err := s.accountRepo.ListSchedulingCandidatesByCatalogEntry(s.ctx, entry.ID)
	s.Require().NoError(err)
	ids := make([]int64, 0, len(accounts))
	for _, account := range accounts {
		ids = append(ids, account.ID)
	}
	s.Require().Equal([]int64{subAnthropic, keyGemini, keyKimi, keyLowPriority}, ids,
		"sorted by account priority then id; inactive / unschedulable / unbound / other-entry accounts are absent")
	priorities := make([]int, 0, len(accounts))
	for _, account := range accounts {
		priorities = append(priorities, account.Priority)
	}
	s.Require().Equal([]int{10, 20, 20, 90}, priorities, "account's own priority, untouched")
	for _, account := range accounts {
		if account.ID == keyGemini {
			s.Require().Equal([]int64{entry.ID, otherEntry.ID}, account.CatalogEntryIDs)
			s.Require().Equal(map[string]string{
				"candidates-catalog-entry": "relay-main",
				"candidates-catalog-other": "relay-other",
			}, account.CatalogUpstreamModels, "keyed by the entry's catalog model ID")
			continue
		}
		s.Require().Equal([]int64{entry.ID}, account.CatalogEntryIDs)
		s.Require().Nil(account.CatalogUpstreamModels, "no renamed binding → no upstream names")
	}

	fresh, err := s.accountRepo.GetByID(s.ctx, unbound.ID)
	s.Require().NoError(err)
	s.Require().Empty(fresh.CatalogEntryIDs)

	empty, err := s.accountRepo.ListSchedulingCandidatesByCatalogEntry(s.ctx, otherEntry.ID+1000)
	s.Require().NoError(err)
	s.Require().Empty(empty)
}
