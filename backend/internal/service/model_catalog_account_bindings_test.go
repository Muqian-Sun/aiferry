//go:build unit

package service

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

// 按渠道覆盖承接的目录模型（渠道表单里直接勾选，2026-09-25）：与按条目绑定同一套校验，任一条不过就整份不写。
func TestModelCatalogService_ReplaceAccountBindings(t *testing.T) {
	pricing := &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"gpt-image-2": {Mode: "image_generation", LiteLLMProvider: "openai"},
	}}
	newService := func() (*ModelCatalogService, *stubModelCatalogRepo) {
		priority := 5
		repo := &stubModelCatalogRepo{
			entries: []ModelCatalogEntry{
				{ID: 1, ModelID: "claude-sonnet-4", Vendor: "anthropic", Status: ModelCatalogStatusListed, InputPrice: testPtrFloat64(1e-6)},
				{ID: 2, ModelID: "gpt-5.5", Vendor: "openai", Status: ModelCatalogStatusListed, InputPrice: testPtrFloat64(1e-6)},
				{ID: 3, ModelID: "gpt-image-2", Vendor: "openai", Status: ModelCatalogStatusListed, InputPrice: testPtrFloat64(5e-6)},
			},
			bindings: map[int64][]ModelCatalogBinding{
				// 账号 10 已绑条目 1（带优先级 5）；账号 11 也绑着条目 1，不能被账号 10 的覆盖波及
				1: {{EntryID: 1, AccountID: 10, Priority: &priority}, {EntryID: 1, AccountID: 11}},
			},
		}
		return NewModelCatalogService(repo, nil, ModelCatalogSeedInput{PricingService: pricing}), repo
	}
	accounts := stubCatalogBindingAccounts{
		// 只配 responses 地址的中转 key：能承接对话，接不了经扩展端点调用的生图模型
		10: {ID: 10, Type: AccountTypeAPIKey, Platform: PlatformOpenAI, ProtocolEndpoints: map[string]string{APIProtocolResponses: "https://relay.example.com"}},
		11: {ID: 11, Type: AccountTypeOAuth, Platform: PlatformAnthropic},
		// 没配任何地址的 key：什么都承接不了
		12: {ID: 12, Type: AccountTypeAPIKey, Platform: PlatformOpenAI},
	}
	ctx := context.Background()
	snapshot := func(repo *stubModelCatalogRepo) map[int64][]ModelCatalogBinding {
		out := make(map[int64][]ModelCatalogBinding, len(repo.bindings))
		for entryID, bindings := range repo.bindings {
			out[entryID] = append([]ModelCatalogBinding(nil), bindings...)
		}
		return out
	}

	t.Run("keeps retained priority, drops unlisted, adds new, leaves other accounts alone", func(t *testing.T) {
		svc, repo := newService()
		require.NoError(t, svc.ReplaceAccountBindings(ctx, 10, []int64{1, 2}, accounts))

		ids, err := svc.ListAccountEntryIDs(ctx, 10, accounts)
		require.NoError(t, err)
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		require.Equal(t, []int64{1, 2}, ids)
		for _, binding := range repo.bindings[1] {
			if binding.AccountID == 10 {
				require.NotNil(t, binding.Priority)
				require.Equal(t, 5, *binding.Priority, "保留下来的绑定优先级不变")
			}
		}
		require.Len(t, repo.bindings[1], 2, "账号 11 在条目 1 上的绑定不受影响")

		require.NoError(t, svc.ReplaceAccountBindings(ctx, 10, []int64{2}, accounts))
		ids, err = svc.ListAccountEntryIDs(ctx, 10, accounts)
		require.NoError(t, err)
		require.Equal(t, []int64{2}, ids)
		require.Len(t, repo.bindings[1], 1, "账号 10 从条目 1 上摘掉，账号 11 还在")
		require.Equal(t, int64(11), repo.bindings[1][0].AccountID)
	})

	t.Run("rejects an entry the account cannot serve and writes nothing", func(t *testing.T) {
		svc, repo := newService()
		before := snapshot(repo)
		err := svc.ReplaceAccountBindings(ctx, 12, []int64{2}, accounts)
		require.Error(t, err)
		require.Contains(t, err.Error(), "CATALOG_BINDING_UNSERVABLE")
		require.Equal(t, before, snapshot(repo))
	})

	t.Run("applies the extension endpoint check to image entries", func(t *testing.T) {
		svc, repo := newService()
		before := snapshot(repo)
		err := svc.ReplaceAccountBindings(ctx, 10, []int64{2, 3}, accounts)
		require.Error(t, err)
		require.Contains(t, err.Error(), "chat_completions")
		require.Equal(t, before, snapshot(repo), "一条不过就整份不写")
	})

	t.Run("rejects duplicates, unknown entries and unknown accounts", func(t *testing.T) {
		svc, repo := newService()
		before := snapshot(repo)
		err := svc.ReplaceAccountBindings(ctx, 10, []int64{2, 2}, accounts)
		require.Error(t, err)
		require.Contains(t, err.Error(), "duplicate entry")
		require.ErrorIs(t, svc.ReplaceAccountBindings(ctx, 10, []int64{42}, accounts), ErrModelCatalogEntryNotFound)
		require.ErrorIs(t, svc.ReplaceAccountBindings(ctx, 99, []int64{2}, accounts), ErrAccountNotFound)
		_, err = svc.ListAccountEntryIDs(ctx, 99, accounts)
		require.ErrorIs(t, err, ErrAccountNotFound)
		require.Equal(t, before, snapshot(repo))
	})
}
