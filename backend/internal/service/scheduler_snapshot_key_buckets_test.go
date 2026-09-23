//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// 平台池桶只装平台相等的账号：第三方 key 与成品号同一条规则（D18），
// key 的展示标签决定它进哪个池；桶内容与入站协议无关，协议在读取时过滤。

type keyBucketAccountRepo struct {
	*batchAccountQueryRepo
	accounts []Account
}

func (r *keyBucketAccountRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	for i := range r.accounts {
		if r.accounts[i].ID == id {
			account := r.accounts[i]
			return &account, nil
		}
	}
	return nil, ErrAccountNotFound
}

func (r *keyBucketAccountRepo) GetByIDs(ctx context.Context, ids []int64) ([]*Account, error) {
	out := make([]*Account, 0, len(ids))
	for _, id := range ids {
		if account, err := r.GetByID(ctx, id); err == nil {
			out = append(out, account)
		}
	}
	return out, nil
}

func (r *keyBucketAccountRepo) ListSchedulingCandidatesByCatalogEntry(context.Context, int64) ([]Account, error) {
	return nil, nil
}

func (r *keyBucketAccountRepo) ListSchedulingCandidates(_ context.Context, platforms []string) ([]Account, error) {
	var out []Account
	for _, account := range r.accounts {
		if schedulingCandidateMatchesForTest(account, platforms) {
			out = append(out, account)
		}
	}
	return out, nil
}

func keyBucketFixture() (key Account, subscription Account, repo *keyBucketAccountRepo) {
	// openai 标签的第三方 key（只配了 chat 地址）与 openai 成品号：两者都在 openai 池。
	key = schedulingTestKey(21011, PlatformOpenAI, map[string]string{APIProtocolChatCompletions: schedulingTestRelayURL})
	subscription = Account{
		ID: 21012, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
	}
	repo = &keyBucketAccountRepo{batchAccountQueryRepo: newBatchAccountQueryRepo(), accounts: []Account{key, subscription}}
	return key, subscription, repo
}

func publishedAccountIDs(t *testing.T, cache *bulkEventSnapshotCache, bucket SchedulerBucket) []int64 {
	t.Helper()
	_, _, _, writes := cache.bucketState(bucket)
	require.Len(t, writes, 1, bucket.String())
	return accountIDs(writes[0].accounts)
}

// key 变更只重建它自己标签的平台池（原来是所属分组的每个网关平台桶）。
func TestSchedulerSnapshot_KeyChangeRebuildsItsOwnPlatformPool(t *testing.T) {
	key, subscription, repo := keyBucketFixture()
	cache := newBulkEventSnapshotCache()
	svc := newBulkEventTestService(cache, repo)

	require.NoError(t, svc.handleAccountEvent(context.Background(), &key.ID, nil, make(map[batchSeenKey]struct{})))

	require.ElementsMatch(t, platformPoolBuckets(PlatformOpenAI), cache.capturedBuckets())
	// 桶不看入站协议：openai 池里有这把 key 与 openai 成品号。
	require.Equal(t, []int64{key.ID, subscription.ID}, publishedAccountIDs(t, cache, platformPoolBucket(PlatformOpenAI)))
}

func TestSchedulerSnapshot_SubscriptionChangeRebuildsItsOwnPlatformPool(t *testing.T) {
	_, subscription, repo := keyBucketFixture()
	cache := newBulkEventSnapshotCache()
	svc := newBulkEventTestService(cache, repo)

	require.NoError(t, svc.handleAccountEvent(context.Background(), &subscription.ID, nil, make(map[batchSeenKey]struct{})))

	require.ElementsMatch(t, platformPoolBuckets(PlatformOpenAI), cache.capturedBuckets())
}

func TestSchedulerSnapshot_BulkKeyChangeRebuildsItsOwnPlatformPool(t *testing.T) {
	key, _, repo := keyBucketFixture()
	cache := newBulkEventSnapshotCache()
	svc := newBulkEventTestService(cache, repo)

	require.NoError(t, svc.handleBulkAccountEvent(context.Background(), bulkEventPayload([]int64{key.ID}, nil), make(map[batchSeenKey]struct{})))

	require.ElementsMatch(t, platformPoolBuckets(PlatformOpenAI), cache.capturedBuckets())
}

// 读取时按入站协议过滤：只配 chat 地址的 key 不承接 gemini 入站。
func TestSchedulerSnapshot_ListSchedulableAccountsFiltersKeysByInboundProtocol(t *testing.T) {
	key, subscription, repo := keyBucketFixture()

	services := map[string]*SchedulerSnapshotService{
		// 缓存命中：桶里是未过滤的候选。
		"cache hit": NewSchedulerSnapshotService(&openAISnapshotCacheStub{snapshotAccounts: []*Account{&key, &subscription}}, nil, nil, &config.Config{RunMode: config.RunModeStandard}),
		// 缓存缺失回源数据库。
		"db fallback": NewSchedulerSnapshotService(nil, nil, repo, nil),
	}
	for name, svc := range services {
		t.Run(name, func(t *testing.T) {
			chatCtx := WithInboundProtocol(context.Background(), APIProtocolChatCompletions)
			accounts, err := svc.ListSchedulableAccounts(chatCtx, PlatformOpenAI)
			require.NoError(t, err)
			require.Equal(t, []int64{key.ID, subscription.ID}, accountIDs(accounts))

			geminiCtx := WithInboundProtocol(context.Background(), APIProtocolGemini)
			accounts, err = svc.ListSchedulableAccounts(geminiCtx, PlatformOpenAI)
			require.NoError(t, err)
			require.Equal(t, []int64{subscription.ID}, accountIDs(accounts))
		})
	}
}
