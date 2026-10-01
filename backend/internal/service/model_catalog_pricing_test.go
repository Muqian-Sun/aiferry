//go:build unit

package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// pricingTestAccounts 按 ID 取账号：1 = Chat 中转（能承接），2 = 没配协议地址的 key（承接不了任何入站协议）。
type pricingTestAccounts map[int64]*Account

func (m pricingTestAccounts) GetAccount(_ context.Context, id int64) (*Account, error) {
	if account, ok := m[id]; ok {
		return account, nil
	}
	return nil, ErrAccountNotFound
}

func newPricingTestAccounts() pricingTestAccounts {
	return pricingTestAccounts{
		1: {ID: 1, Name: "fenno-chat", Type: AccountTypeAPIKey, Platform: PlatformOpenAI, Status: StatusActive,
			ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://relay.example.com"}},
		2: {ID: 2, Name: "no-endpoint", Type: AccountTypeAPIKey, Platform: PlatformOpenAI, Status: StatusActive},
		3: {ID: 3, Name: "qiniu-chat", Type: AccountTypeAPIKey, Platform: PlatformOpenAI, Status: StatusActive,
			ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://relay2.example.com"}},
	}
}

func TestModelCatalogBinding_ValidateAgainst(t *testing.T) {
	entry := upstreamCostTestEntry() // 官方有输入、输出、缓存读

	cases := []struct {
		name    string
		mutate  func(b *ModelCatalogBinding, e *ModelCatalogEntry)
		wantErr string
	}{
		{name: "complete", mutate: func(*ModelCatalogBinding, *ModelCatalogEntry) {}},
		{name: "cache read required when official has it",
			mutate:  func(b *ModelCatalogBinding, _ *ModelCatalogEntry) { b.CacheReadPrice = nil },
			wantErr: "upstream cache_read_price is required"},
		{name: "cache write not required when official lacks it",
			mutate: func(b *ModelCatalogBinding, _ *ModelCatalogEntry) { b.CacheWritePrice = nil }},
		{name: "cache write 1h required when official has it",
			mutate: func(b *ModelCatalogBinding, e *ModelCatalogEntry) {
				e.CacheWrite1hPrice = upstreamCostPtr(10e-6)
			},
			wantErr: "upstream cache_write_1h_price is required"},
		{name: "negative price",
			mutate:  func(b *ModelCatalogBinding, _ *ModelCatalogEntry) { b.OutputPrice = -1e-6 },
			wantErr: "upstream output_price must be >= 0"},
		{name: "non-token model",
			mutate:  func(_ *ModelCatalogBinding, e *ModelCatalogEntry) { e.BillingMode = BillingModeImage },
			wantErr: "is not billed by token"},
		{name: "segment with multiplier",
			mutate: func(b *ModelCatalogBinding, _ *ModelCatalogEntry) {
				b.Intervals = []PricingInterval{{MinTokens: 272000, InputMultiplier: upstreamCostPtr(2)}}
			},
			wantErr: "upstream segment 1 must use absolute prices"},
		{name: "segment without price",
			mutate: func(b *ModelCatalogBinding, _ *ModelCatalogEntry) {
				b.Intervals = []PricingInterval{{MinTokens: 272000}}
			},
			wantErr: "upstream segment 1 has no price"},
		{name: "upstream model with wildcard",
			mutate:  func(b *ModelCatalogBinding, _ *ModelCatalogEntry) { b.UpstreamModel = "gpt-*" },
			wantErr: "upstream_model must be a single model name"},
		{name: "upstream model with space",
			mutate:  func(b *ModelCatalogBinding, _ *ModelCatalogEntry) { b.UpstreamModel = "gpt 5" },
			wantErr: "upstream_model must be a single model name"},
		{name: "upstream model too long",
			mutate:  func(b *ModelCatalogBinding, _ *ModelCatalogEntry) { b.UpstreamModel = strings.Repeat("m", 256) },
			wantErr: "upstream_model must be at most 255 characters"},
		{name: "upstream model at the length limit",
			mutate: func(b *ModelCatalogBinding, _ *ModelCatalogEntry) { b.UpstreamModel = strings.Repeat("模", 255) }},
		{name: "unbounded segment not last",
			mutate: func(b *ModelCatalogBinding, _ *ModelCatalogEntry) {
				b.Intervals = []PricingInterval{
					{MinTokens: 100000, InputPrice: upstreamCostPtr(0.2e-6)},
					{MinTokens: 272000, InputPrice: upstreamCostPtr(0.3e-6)},
				}
			},
			wantErr: "upstream interval #1: unbounded interval"},
		{name: "overlapping segments",
			mutate: func(b *ModelCatalogBinding, _ *ModelCatalogEntry) {
				b.Intervals = []PricingInterval{
					{MinTokens: 100000, MaxTokens: intPtr(300000), InputPrice: upstreamCostPtr(0.2e-6)},
					{MinTokens: 272000, InputPrice: upstreamCostPtr(0.3e-6)},
				}
			},
			wantErr: "upstream interval #1 and #2 overlap"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := entry
			b := upstreamCostTestBinding(1)
			tc.mutate(&b, &e)
			err := b.ValidateAgainst(&e)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestModelCatalogService_SaveEntryPricing(t *testing.T) {
	ctx := context.Background()
	accounts := newPricingTestAccounts()
	official := func() OfficialPrices {
		return OfficialPrices{
			InputPrice: upstreamCostPtr(4e-6), OutputPrice: upstreamCostPtr(24e-6), CacheReadPrice: upstreamCostPtr(0.4e-6),
			Intervals: []PricingInterval{{MinTokens: 272000, InputPrice: upstreamCostPtr(8e-6), OutputPrice: upstreamCostPtr(36e-6), SortOrder: 5, ID: 77}},
		}
	}

	t.Run("writes official prices and replaces bindings", func(t *testing.T) {
		seed := upstreamCostTestEntry()
		seed.ManagedBy = ModelCatalogManagedBySeed
		svc, repo := newTestModelCatalogService(seed)
		repo.bindings = map[int64][]ModelCatalogBinding{1: {upstreamCostTestBinding(3)}}

		b := upstreamCostTestBinding(1)
		// 分段乱序传进来，保存时按起点排好
		b.Intervals = []PricingInterval{
			{MinTokens: 272000, InputPrice: upstreamCostPtr(0.3e-6)},
			{MinTokens: 100000, MaxTokens: intPtr(272000), InputPrice: upstreamCostPtr(0.2e-6)},
		}
		got, err := svc.SaveEntryPricing(ctx, 1, official(), []ModelCatalogBinding{b}, accounts)
		require.NoError(t, err)
		require.Equal(t, 4e-6, *got.InputPrice)
		require.Equal(t, 24e-6, *got.OutputPrice)
		require.Nil(t, got.CacheWritePrice)
		require.Equal(t, ModelCatalogManagedByAdmin, got.ManagedBy, "an edited seed entry becomes admin-managed")
		require.Len(t, got.Intervals, 1)
		require.Zero(t, got.Intervals[0].ID, "request ids are dropped")
		require.Zero(t, got.Intervals[0].SortOrder)

		saved := repo.bindings[1]
		require.Len(t, saved, 1, "account 3 is dropped: the block replaces all bindings")
		require.Equal(t, int64(1), saved[0].AccountID)
		require.Equal(t, int64(1), saved[0].EntryID)
		require.Equal(t, 100000, saved[0].Intervals[0].MinTokens, "segments sorted by start")
		require.Equal(t, 272000, saved[0].Intervals[1].MinTokens)
		require.Equal(t, 1, saved[0].Intervals[1].SortOrder)
	})

	reject := []struct {
		name     string
		official func() OfficialPrices
		bindings func() []ModelCatalogBinding
		wantErr  string
	}{
		{name: "official output missing",
			official: func() OfficialPrices { o := official(); o.OutputPrice = nil; return o },
			bindings: func() []ModelCatalogBinding { return nil },
			wantErr:  "official input_price and output_price are required"},
		{name: "official segment with multiplier",
			official: func() OfficialPrices {
				o := official()
				o.Intervals = []PricingInterval{{MinTokens: 272000, OutputMultiplier: upstreamCostPtr(1.5)}}
				return o
			},
			bindings: func() []ModelCatalogBinding { return nil },
			wantErr:  "official segment 1 must use absolute prices"},
		{name: "account cannot serve",
			official: official,
			bindings: func() []ModelCatalogBinding { return []ModelCatalogBinding{upstreamCostTestBinding(2)} },
			wantErr:  "has no upstream protocol"},
		{name: "unknown account",
			official: official,
			bindings: func() []ModelCatalogBinding { return []ModelCatalogBinding{upstreamCostTestBinding(42)} },
			wantErr:  ErrAccountNotFound.Error()},
		{name: "duplicate account",
			official: official,
			bindings: func() []ModelCatalogBinding {
				return []ModelCatalogBinding{upstreamCostTestBinding(1), upstreamCostTestBinding(1)}
			},
			wantErr: "duplicate account 1"},
		{name: "second binding incomplete",
			official: official,
			bindings: func() []ModelCatalogBinding {
				bad := upstreamCostTestBinding(3)
				bad.CacheReadPrice = nil
				return []ModelCatalogBinding{upstreamCostTestBinding(1), bad}
			},
			wantErr: "upstream cache_read_price is required"},
	}
	for _, tc := range reject {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			svc, repo := newTestModelCatalogService(upstreamCostTestEntry())
			before := []ModelCatalogBinding{upstreamCostTestBinding(3)}
			repo.bindings = map[int64][]ModelCatalogBinding{1: before}
			_, err := svc.SaveEntryPricing(ctx, 1, tc.official(), tc.bindings(), accounts)
			require.ErrorContains(t, err, tc.wantErr)
			stored, _ := repo.GetEntryByID(ctx, 1)
			require.Equal(t, 5e-6, *stored.InputPrice, "nothing written")
			require.Equal(t, before, repo.bindings[1], "nothing written")
		})
	}

	// 上游模型名去首尾空白；与目录标识相同的存成空串（= 同名）
	t.Run("upstream model names are normalized", func(t *testing.T) {
		svc, repo := newTestModelCatalogService(upstreamCostTestEntry())
		renamed := upstreamCostTestBinding(1)
		renamed.UpstreamModel = "  auto-review  "
		same := upstreamCostTestBinding(3)
		same.UpstreamModel = "upstream-test-model"
		_, err := svc.SaveEntryPricing(ctx, 1, official(), []ModelCatalogBinding{renamed, same}, accounts)
		require.NoError(t, err)
		require.Equal(t, "auto-review", repo.bindings[1][0].UpstreamModel)
		require.Empty(t, repo.bindings[1][1].UpstreamModel)

		channel := upstreamCostTestBinding(0)
		channel.UpstreamModel = " upstream-test-model "
		_, err = svc.SaveAccountPricing(ctx, 1, []ModelCatalogBinding{channel}, accounts)
		require.NoError(t, err)
		for _, b := range repo.bindings[1] {
			if b.AccountID == 1 {
				require.Empty(t, b.UpstreamModel, "same name as the catalog model is stored as empty")
			}
		}
	})

	// 只改承接渠道、官方价原样提交时不改归属：种子条目仍是平台默认价卡（DeepSeek 强制官方价 / 高峰加价照常、种子照常刷新）
	t.Run("unchanged official prices keep the seed ownership", func(t *testing.T) {
		seed := upstreamCostTestEntry()
		seed.ManagedBy = ModelCatalogManagedBySeed
		svc, _ := newTestModelCatalogService(seed)
		same := OfficialPrices{
			InputPrice: seed.InputPrice, OutputPrice: seed.OutputPrice, CacheReadPrice: seed.CacheReadPrice,
			// 请求里带回来的分段有 ID / 排序号，与库里的不同，不算改价
			Intervals: []PricingInterval{{ID: 9, SortOrder: 3, MinTokens: 272000, InputPrice: upstreamCostPtr(10e-6), OutputPrice: upstreamCostPtr(45e-6)}},
		}
		got, err := svc.SaveEntryPricing(ctx, 1, same, []ModelCatalogBinding{upstreamCostTestBinding(3)}, accounts)
		require.NoError(t, err)
		require.Equal(t, ModelCatalogManagedBySeed, got.ManagedBy)

		changedSegment := same
		changedSegment.Intervals = []PricingInterval{{MinTokens: 272000, InputPrice: upstreamCostPtr(10e-6), OutputPrice: upstreamCostPtr(40e-6)}}
		got, err = svc.SaveEntryPricing(ctx, 1, changedSegment, []ModelCatalogBinding{upstreamCostTestBinding(3)}, accounts)
		require.NoError(t, err)
		require.Equal(t, ModelCatalogManagedByAdmin, got.ManagedBy, "a changed segment price is operator pricing")
	})

	t.Run("rejects non-token model", func(t *testing.T) {
		image := upstreamCostTestEntry()
		image.BillingMode = BillingModeImage
		svc, _ := newTestModelCatalogService(image)
		_, err := svc.SaveEntryPricing(ctx, 1, official(), nil, accounts)
		require.ErrorContains(t, err, "only token models are priced")
	})
}

func TestModelCatalogService_SaveAccountPricing(t *testing.T) {
	ctx := context.Background()
	accounts := newPricingTestAccounts()
	other := upstreamCostTestEntry()
	other.ID, other.ModelID = 2, "upstream-test-model-2"
	image := ModelCatalogEntry{ID: 3, ModelID: "image-model", BillingMode: BillingModeImage, Status: ModelCatalogStatusListed, ManagedBy: ModelCatalogManagedByAdmin}

	newSvc := func() (*ModelCatalogService, *stubModelCatalogRepo) {
		svc, repo := newTestModelCatalogService(upstreamCostTestEntry(), other, image)
		// 渠道 1 承接条目 1；渠道 3 承接条目 1、2
		repo.bindings = map[int64][]ModelCatalogBinding{
			1: {upstreamCostTestBinding(1), upstreamCostTestBinding(3)},
			2: {func() ModelCatalogBinding { b := upstreamCostTestBinding(3); b.EntryID = 2; return b }()},
		}
		return svc, repo
	}

	t.Run("replaces the channel's bindings only", func(t *testing.T) {
		svc, repo := newSvc()
		b := upstreamCostTestBinding(0)
		b.EntryID = 2
		b.InputPrice = 0.1e-6
		saved, err := svc.SaveAccountPricing(ctx, 1, []ModelCatalogBinding{b}, accounts)
		require.NoError(t, err)
		require.Len(t, saved, 1)
		require.Equal(t, int64(1), saved[0].AccountID, "account id comes from the path")

		require.Len(t, repo.bindings[1], 1, "channel 1 left model 1")
		require.Equal(t, int64(3), repo.bindings[1][0].AccountID, "channel 3 untouched")
		require.Len(t, repo.bindings[2], 2)
	})

	reject := []struct {
		name     string
		account  int64
		bindings func() []ModelCatalogBinding
		wantErr  string
	}{
		{name: "non-token model", account: 1,
			bindings: func() []ModelCatalogBinding {
				b := upstreamCostTestBinding(0)
				b.EntryID = 3
				return []ModelCatalogBinding{b}
			},
			wantErr: "is not billed by token"},
		{name: "unknown model", account: 1,
			bindings: func() []ModelCatalogBinding {
				b := upstreamCostTestBinding(0)
				b.EntryID = 9
				return []ModelCatalogBinding{b}
			},
			wantErr: ErrModelCatalogEntryNotFound.Error()},
		{name: "duplicate model", account: 1,
			bindings: func() []ModelCatalogBinding {
				return []ModelCatalogBinding{upstreamCostTestBinding(0), upstreamCostTestBinding(0)}
			},
			wantErr: "duplicate entry 1"},
		{name: "channel cannot serve", account: 2,
			bindings: func() []ModelCatalogBinding { return []ModelCatalogBinding{upstreamCostTestBinding(0)} },
			wantErr:  "has no upstream protocol"},
		{name: "unknown channel", account: 42,
			bindings: func() []ModelCatalogBinding { return nil },
			wantErr:  ErrAccountNotFound.Error()},
	}
	for _, tc := range reject {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			svc, repo := newSvc()
			before := map[int64][]ModelCatalogBinding{1: append([]ModelCatalogBinding(nil), repo.bindings[1]...), 2: append([]ModelCatalogBinding(nil), repo.bindings[2]...)}
			_, err := svc.SaveAccountPricing(ctx, tc.account, tc.bindings(), accounts)
			require.ErrorContains(t, err, tc.wantErr)
			require.Equal(t, before[1], repo.bindings[1], "nothing written")
			require.Equal(t, before[2], repo.bindings[2], "nothing written")
		})
	}
}

func TestModelCatalogService_ListPricingEntriesOnlyTokenModels(t *testing.T) {
	image := ModelCatalogEntry{ID: 3, ModelID: "image-model", BillingMode: BillingModeImage, Status: ModelCatalogStatusListed, ManagedBy: ModelCatalogManagedByAdmin}
	svc, _ := newTestModelCatalogService(upstreamCostTestEntry(), image)
	entries, err := svc.ListPricingEntries(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "upstream-test-model", entries[0].ModelID)

	accounts := newPricingTestAccounts()
	require.True(t, svc.CanBindAccount(&entries[0], accounts[1]))
	require.False(t, svc.CanBindAccount(&entries[0], accounts[2]))
}
