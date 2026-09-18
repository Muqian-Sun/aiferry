//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// 第三方 key 进入所属分组每个网关平台的调度桶；桶内容与入站协议无关，协议在读取时过滤。

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

func (r *keyBucketAccountRepo) ListSchedulingCandidatesByGroupID(_ context.Context, groupID int64, platforms []string) ([]Account, error) {
	var out []Account
	for _, account := range r.accounts {
		if schedulingCandidateMatchesForTest(account, platforms) && openAIStickyAccountMatchesGroup(&account, &groupID) {
			out = append(out, account)
		}
	}
	return out, nil
}

func keyBucketFixture(groupID int64) (key Account, subscription Account, repo *keyBucketAccountRepo) {
	key = schedulingTestKey(21011, PlatformAnthropic, map[string]string{APIProtocolChatCompletions: schedulingTestRelayURL}, groupID)
	subscription = Account{
		ID: 21012, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
		GroupIDs: []int64{groupID}, AccountGroups: []AccountGroup{{AccountID: 21012, GroupID: groupID}},
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

func TestSchedulerSnapshot_KeyChangeRebuildsEveryGatewayPlatformBucket(t *testing.T) {
	groupID := int64(21001)
	key, subscription, repo := keyBucketFixture(groupID)
	cache := newBulkEventSnapshotCache()
	svc := newBulkEventTestService(cache, repo)

	require.NoError(t, svc.handleAccountEvent(context.Background(), &key.ID, nil, make(map[batchSeenKey]struct{})))

	platforms := schedulerSnapshotPlatforms()
	require.ElementsMatch(t, schedulerBucketsForTest([]int64{groupID}, platforms[:]...), cache.capturedBuckets())
	// 桶不看入站协议与标签：openai 桶里有这个 anthropic 标签的 key 与 openai 成品号；
	// gemini 混合桶里也有这个 key（尽管它没有 gemini 地址），成品号仍按平台归桶。
	require.Equal(t, []int64{key.ID, subscription.ID}, publishedAccountIDs(t, cache, SchedulerBucket{GroupID: groupID, Platform: PlatformOpenAI, Mode: SchedulerModeSingle}))
	require.Equal(t, []int64{key.ID}, publishedAccountIDs(t, cache, SchedulerBucket{GroupID: groupID, Platform: PlatformGemini, Mode: SchedulerModeMixed}))
	require.Equal(t, []int64{key.ID}, publishedAccountIDs(t, cache, SchedulerBucket{GroupID: groupID, Platform: PlatformGrok, Mode: SchedulerModeForced}))
}

func TestSchedulerSnapshot_SubscriptionChangeKeepsLabelScopedRebuild(t *testing.T) {
	groupID := int64(21002)
	_, subscription, repo := keyBucketFixture(groupID)
	cache := newBulkEventSnapshotCache()
	svc := newBulkEventTestService(cache, repo)

	require.NoError(t, svc.handleAccountEvent(context.Background(), &subscription.ID, nil, make(map[batchSeenKey]struct{})))

	require.ElementsMatch(t, schedulerBucketsForTest([]int64{groupID}, PlatformOpenAI), cache.capturedBuckets())
}

func TestSchedulerSnapshot_BulkKeyChangeRebuildsEveryGatewayPlatformBucket(t *testing.T) {
	groupID := int64(21003)
	key, _, repo := keyBucketFixture(groupID)
	cache := newBulkEventSnapshotCache()
	svc := newBulkEventTestService(cache, repo)

	require.NoError(t, svc.handleBulkAccountEvent(context.Background(), bulkEventPayload([]int64{key.ID}, nil), make(map[batchSeenKey]struct{})))

	platforms := schedulerSnapshotPlatforms()
	require.ElementsMatch(t, schedulerBucketsForTest([]int64{groupID}, platforms[:]...), cache.capturedBuckets())
}

func TestSchedulerSnapshot_ListSchedulableAccountsFiltersKeysByInboundProtocol(t *testing.T) {
	groupID := int64(21004)
	key, subscription, repo := keyBucketFixture(groupID)

	services := map[string]*SchedulerSnapshotService{
		// 缓存命中：桶里是未过滤的候选。
		"cache hit": NewSchedulerSnapshotService(&openAISnapshotCacheStub{snapshotAccounts: []*Account{&key, &subscription}}, nil, nil, nil, &config.Config{RunMode: config.RunModeStandard}),
		// 缓存缺失回源数据库。
		"db fallback": NewSchedulerSnapshotService(nil, nil, repo, nil, nil),
	}
	for name, svc := range services {
		t.Run(name, func(t *testing.T) {
			chatCtx := WithInboundProtocol(context.Background(), APIProtocolChatCompletions)
			accounts, _, err := svc.ListSchedulableAccounts(chatCtx, &groupID, PlatformOpenAI, false)
			require.NoError(t, err)
			require.Equal(t, []int64{key.ID, subscription.ID}, accountIDs(accounts))

			geminiCtx := WithInboundProtocol(context.Background(), APIProtocolGemini)
			accounts, _, err = svc.ListSchedulableAccounts(geminiCtx, &groupID, PlatformOpenAI, false)
			require.NoError(t, err)
			require.Equal(t, []int64{subscription.ID}, accountIDs(accounts))
		})
	}
}
