package service

import (
	"context"
	"errors"
	"time"
)

// 调度用例共用的内存夹具（原 openai_account_scheduler_test.go；OpenAI 调度器已删，夹具留给 Gateway 调度器用例）。

type schedulerTestOpenAIAccountRepo struct {
	AccountRepository
	accounts []Account
}

func (r schedulerTestOpenAIAccountRepo) GetByID(ctx context.Context, id int64) (*Account, error) {
	for i := range r.accounts {
		if r.accounts[i].ID == id {
			return &r.accounts[i], nil
		}
	}
	return nil, errors.New("account not found")
}

func (r schedulerTestOpenAIAccountRepo) ListSchedulableByPlatform(ctx context.Context, platform string) ([]Account, error) {
	var result []Account
	for _, acc := range r.accounts {
		if acc.Platform == platform {
			result = append(result, acc)
		}
	}
	return result, nil
}

// schedulingCandidateMatchesForTest 是 AccountRepository.ListSchedulingCandidates* 的内存版口径：
// 平台属于 platforms 的成品号，加上任意平台标签的第三方 key。
func schedulingCandidateMatchesForTest(acc Account, platforms []string) bool {
	if acc.IsThirdPartyKey() {
		return true
	}
	for _, platform := range platforms {
		if acc.Platform == platform {
			return true
		}
	}
	return false
}

func (r schedulerTestOpenAIAccountRepo) ListSchedulingCandidates(ctx context.Context, platforms []string) ([]Account, error) {
	var result []Account
	for _, acc := range r.accounts {
		if schedulingCandidateMatchesForTest(acc, platforms) {
			result = append(result, acc)
		}
	}
	return result, nil
}

// ListSchedulingCandidatesByCatalogEntry 返回 CatalogEntryIDs 含该条目的账号（不看平台与分组）。
func (r schedulerTestOpenAIAccountRepo) ListSchedulingCandidatesByCatalogEntry(_ context.Context, entryID int64) ([]Account, error) {
	var out []Account
	for _, account := range r.accounts {
		for _, id := range account.CatalogEntryIDs {
			if id == entryID {
				out = append(out, account)
				break
			}
		}
	}
	return out, nil
}

type schedulerTestConcurrencyCache struct {
	ConcurrencyCache
	loadBatchErr    error
	loadMap         map[int64]*AccountLoadInfo
	acquireResults  map[int64]bool
	waitCounts      map[int64]int
	skipDefaultLoad bool
	acquiredIDs     *[]int64
	releasedIDs     *[]int64
}

func (c schedulerTestConcurrencyCache) AcquireAccountSlot(ctx context.Context, accountID int64, maxConcurrency int, requestID string) (bool, error) {
	if c.acquiredIDs != nil {
		*c.acquiredIDs = append(*c.acquiredIDs, accountID)
	}
	if c.acquireResults != nil {
		if result, ok := c.acquireResults[accountID]; ok {
			return result, nil
		}
	}
	return true, nil
}

func (c schedulerTestConcurrencyCache) ReleaseAccountSlot(ctx context.Context, accountID int64, requestID string) error {
	if c.releasedIDs != nil {
		*c.releasedIDs = append(*c.releasedIDs, accountID)
	}
	return nil
}

func (c schedulerTestConcurrencyCache) GetAccountsLoadBatch(ctx context.Context, accounts []AccountWithConcurrency) (map[int64]*AccountLoadInfo, error) {
	if c.loadBatchErr != nil {
		return nil, c.loadBatchErr
	}
	out := make(map[int64]*AccountLoadInfo, len(accounts))
	if c.skipDefaultLoad && c.loadMap != nil {
		for _, acc := range accounts {
			if load, ok := c.loadMap[acc.ID]; ok {
				out[acc.ID] = load
			}
		}
		return out, nil
	}
	for _, acc := range accounts {
		if c.loadMap != nil {
			if load, ok := c.loadMap[acc.ID]; ok {
				out[acc.ID] = load
				continue
			}
		}
		out[acc.ID] = &AccountLoadInfo{AccountID: acc.ID, LoadRate: 0}
	}
	return out, nil
}

func (c schedulerTestConcurrencyCache) GetAccountWaitingCount(ctx context.Context, accountID int64) (int, error) {
	if c.waitCounts != nil {
		if count, ok := c.waitCounts[accountID]; ok {
			return count, nil
		}
	}
	return 0, nil
}

type schedulerTestGatewayCache struct {
	sessionBindings map[string]int64
	deletedSessions map[string]int
}

func (c *schedulerTestGatewayCache) GetSessionAccountID(ctx context.Context, groupID int64, sessionHash string) (int64, error) {
	if id, ok := c.sessionBindings[sessionHash]; ok {
		return id, nil
	}
	return 0, ErrStickySessionNotFound
}

func (c *schedulerTestGatewayCache) SetSessionAccountID(ctx context.Context, groupID int64, sessionHash string, accountID int64, ttl time.Duration) error {
	if c.sessionBindings == nil {
		c.sessionBindings = make(map[string]int64)
	}
	c.sessionBindings[sessionHash] = accountID
	return nil
}

func (c *schedulerTestGatewayCache) RefreshSessionTTL(ctx context.Context, groupID int64, sessionHash string, ttl time.Duration) error {
	return nil
}

func (c *schedulerTestGatewayCache) DeleteSessionAccountID(ctx context.Context, groupID int64, sessionHash string) error {
	if c.sessionBindings == nil {
		return nil
	}
	if c.deletedSessions == nil {
		c.deletedSessions = make(map[string]int)
	}
	c.deletedSessions[sessionHash]++
	delete(c.sessionBindings, sessionHash)
	return nil
}

func (c *schedulerTestGatewayCache) SetGrokVideoPendingBilling(_ context.Context, _ string, _ []byte, _ time.Duration) error {
	return nil
}
func (c *schedulerTestGatewayCache) GetGrokVideoPendingBilling(_ context.Context, _ string) ([]byte, error) {
	return nil, nil
}
func (c *schedulerTestGatewayCache) ClaimGrokVideoBilled(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return true, nil
}

func (c *schedulerTestGatewayCache) ReleaseGrokVideoBilled(_ context.Context, _ string) error {
	return nil
}

func (c *schedulerTestGatewayCache) SetReasoningContent(_ context.Context, _ string, _ string, _ time.Duration) error {
	return nil
}
func (c *schedulerTestGatewayCache) GetReasoningContent(_ context.Context, _ string) (string, error) {
	return "", ErrReasoningContentNotFound
}

type openAISnapshotCacheStub struct {
	SchedulerCache
	snapshotAccounts []*Account
	accountsByID     map[int64]*Account
}

func (s *openAISnapshotCacheStub) GetSnapshot(ctx context.Context, bucket SchedulerBucket) ([]*Account, bool, error) {
	if len(s.snapshotAccounts) == 0 {
		return nil, false, nil
	}
	out := make([]*Account, 0, len(s.snapshotAccounts))
	for _, account := range s.snapshotAccounts {
		if account == nil {
			continue
		}
		cloned := *account
		out = append(out, &cloned)
	}
	return out, true, nil
}

func (s *openAISnapshotCacheStub) GetAccount(ctx context.Context, accountID int64) (*Account, error) {
	if s.accountsByID == nil {
		return nil, nil
	}
	account := s.accountsByID[accountID]
	if account == nil {
		return nil, nil
	}
	cloned := *account
	return &cloned, nil
}
