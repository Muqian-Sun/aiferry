package service

import (
	"context"
	"strings"
	"sync"
	"testing"
)

// stubModelCatalogRepo 是内存版目录仓储，用于单测。
type stubModelCatalogRepo struct {
	mu        sync.Mutex
	entries   []ModelCatalogEntry
	listErr   error
	listCalls int
	// listGate 非 nil 时 ListEntries 会阻塞到它被关闭，用来模拟慢库。
	listGate chan struct{}
	// seedAbortAfter > 0 时，播种写入这么多条后返回 ctx 到期错误，模拟中途中止。
	seedAbortAfter int
	// bindings 按条目 ID 存承接关系（ListEntries 装进条目）。
	bindings map[int64][]ModelCatalogBinding
}

// ListEntries 像真实驱动一样尊重 ctx：已取消的 ctx 直接报错。
func (r *stubModelCatalogRepo) ListEntries(ctx context.Context) ([]ModelCatalogEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.listCalls++
	gate := r.listGate
	r.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.listErr != nil {
		return nil, r.listErr
	}
	out := make([]ModelCatalogEntry, len(r.entries))
	copy(out, r.entries)
	for i := range out {
		out[i].Bindings = append([]ModelCatalogBinding(nil), r.bindings[out[i].ID]...)
	}
	return out, nil
}

func (r *stubModelCatalogRepo) ListBindingsByEntry(_ context.Context, entryID int64) ([]ModelCatalogBinding, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]ModelCatalogBinding(nil), r.bindings[entryID]...), nil
}

// SaveEntryPricing 与真仓储同口径：更新条目并整份覆盖它的承接关系。
func (r *stubModelCatalogRepo) SaveEntryPricing(_ context.Context, entry *ModelCatalogEntry, bindings []ModelCatalogBinding) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.entries {
		if r.entries[i].ID == entry.ID {
			r.entries[i] = *entry.Clone()
			if r.bindings == nil {
				r.bindings = map[int64][]ModelCatalogBinding{}
			}
			r.bindings[entry.ID] = append([]ModelCatalogBinding(nil), bindings...)
			return nil
		}
	}
	return ErrModelCatalogEntryNotFound
}

// ReplaceAccountBindings 与真仓储同口径：整份覆盖渠道的承接关系。
func (r *stubModelCatalogRepo) ReplaceAccountBindings(_ context.Context, accountID int64, bindings []ModelCatalogBinding) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.bindings == nil {
		r.bindings = map[int64][]ModelCatalogBinding{}
	}
	for entryID, list := range r.bindings {
		kept := list[:0:0]
		for _, b := range list {
			if b.AccountID != accountID {
				kept = append(kept, b)
			}
		}
		r.bindings[entryID] = kept
	}
	for _, b := range bindings {
		r.bindings[b.EntryID] = append(r.bindings[b.EntryID], b)
	}
	return nil
}

func (r *stubModelCatalogRepo) GetEntryByID(_ context.Context, id int64) (*ModelCatalogEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.entries {
		if r.entries[i].ID == id {
			return r.entries[i].Clone(), nil
		}
	}
	return nil, ErrModelCatalogEntryNotFound
}

func (r *stubModelCatalogRepo) GetEntryByModelID(_ context.Context, modelID string) (*ModelCatalogEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := NormalizeModelCatalogKey(modelID)
	for i := range r.entries {
		if NormalizeModelCatalogKey(r.entries[i].ModelID) == key {
			return r.entries[i].Clone(), nil
		}
	}
	return nil, ErrModelCatalogEntryNotFound
}

func (r *stubModelCatalogRepo) CreateEntry(_ context.Context, entry *ModelCatalogEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := NormalizeModelCatalogKey(entry.ModelID)
	for i := range r.entries {
		if NormalizeModelCatalogKey(r.entries[i].ModelID) == key {
			return ErrModelCatalogEntryExists
		}
	}
	entry.ID = int64(len(r.entries) + 1)
	r.entries = append(r.entries, *entry.Clone())
	return nil
}

func (r *stubModelCatalogRepo) UpdateEntry(_ context.Context, entry *ModelCatalogEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.entries {
		if r.entries[i].ID == entry.ID {
			r.entries[i] = *entry.Clone()
			return nil
		}
	}
	return ErrModelCatalogEntryNotFound
}

func (r *stubModelCatalogRepo) DeleteEntry(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.entries {
		if r.entries[i].ID == id {
			r.entries = append(r.entries[:i], r.entries[i+1:]...)
			return nil
		}
	}
	return ErrModelCatalogEntryNotFound
}

// InsertOrRefreshSeedEntries 复刻真实实现的三分支：不存在插入 / seed 刷新 / admin 跳过。
func (r *stubModelCatalogRepo) InsertOrRefreshSeedEntries(
	_ context.Context,
	entries []ModelCatalogEntry,
) (ModelCatalogSeedResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result ModelCatalogSeedResult
	index := make(map[string]int, len(r.entries))
	for i := range r.entries {
		index[NormalizeModelCatalogKey(r.entries[i].ModelID)] = i
	}
	for i := range entries {
		if r.seedAbortAfter > 0 && i >= r.seedAbortAfter {
			return result, context.DeadlineExceeded
		}
		entry := entries[i]
		key := NormalizeModelCatalogKey(entry.ModelID)
		pos, ok := index[key]
		if !ok {
			entry.ID = int64(len(r.entries) + 1)
			r.entries = append(r.entries, entry)
			index[key] = len(r.entries) - 1
			result.Inserted++
			continue
		}
		if r.entries[pos].ManagedBy != ModelCatalogManagedBySeed {
			result.SkippedAdmin++
			continue
		}
		entry.ID = r.entries[pos].ID
		// 与真仓储同口径：种子带分档时整份覆盖，否则保留；分时不动。
		if len(entry.Intervals) == 0 {
			entry.Intervals = r.entries[pos].Intervals
		}
		entry.TimePricing = r.entries[pos].TimePricing
		r.entries[pos] = entry
		result.Refreshed++
	}
	return result, nil
}

// newTestModelCatalogService 用给定条目构造一个不接 Redis 的目录服务。
func newTestModelCatalogService(entries ...ModelCatalogEntry) (*ModelCatalogService, *stubModelCatalogRepo) {
	repo := &stubModelCatalogRepo{entries: entries}
	return NewModelCatalogService(repo, nil, ModelCatalogSeedInput{}), repo
}

// catalogEntryFromCard 把一份价卡 fixture 投影成目录条目，便于把原来按渠道价卡
// 写的用例平移到目录上。managedBy 决定它算不算运营者定价。
func catalogEntryFromCard(modelID, managedBy string, card PricingCard) ModelCatalogEntry {
	entry := ModelCatalogEntry{
		ModelID:                      modelID,
		BillingMode:                  card.BillingMode,
		Status:                       ModelCatalogStatusListed,
		ManagedBy:                    managedBy,
		InputPrice:                   card.InputPrice,
		OutputPrice:                  card.OutputPrice,
		CacheWritePrice:              card.CacheWritePrice,
		CacheWrite1hPrice:            card.CacheWrite1hPrice,
		CacheReadPrice:               card.CacheReadPrice,
		ImageInputPrice:              card.ImageInputPrice,
		ImageOutputPrice:             card.ImageOutputPrice,
		PerRequestPrice:              card.PerRequestPrice,
		MaxReasoningEffortMultiplier: card.MaxReasoningEffortMultiplier,
		Intervals:                    card.Intervals,
		TimePricing:                  card.TimePricing,
	}
	entry.Normalize()
	return entry
}

// newResolverWithCatalogCards 用一组价卡 fixture 搭一个目录驱动的解析器。
// 条目按 admin 维护记账：它们代表「运营者显式配了价」，与原先的渠道价卡等价。
func newResolverWithCatalogCards(bs *BillingService, cards ...PricingCard) *ModelPricingResolver {
	entries := make([]ModelCatalogEntry, 0, len(cards))
	for _, card := range cards {
		if len(card.Models) == 0 {
			continue
		}
		// 目录不存别名、没有通配：价卡里的每个模型名各建一条同价条目。
		for _, model := range card.Models {
			if strings.Contains(model, "*") {
				panic("catalog has no wildcard model IDs: " + model)
			}
			entries = append(entries, catalogEntryFromCard(model, ModelCatalogManagedByAdmin, card))
		}
	}
	for i := range entries {
		entries[i].ID = int64(i + 1)
	}
	catalog, _ := newTestModelCatalogService(entries...)
	return NewModelPricingResolver(catalog)
}

// newResolverWithSeededEntries 用播种出的目录条目搭解析器：价格数据里的长上下文阶梯经播种换算成按 token 分段，
// 计费走目录这一条路。用来核对「阶梯换算成分段后」的金额与原来按阶梯倍数算的一致。
func newResolverWithSeededEntries(bs *BillingService, entries ...ModelCatalogEntry) *ModelPricingResolver {
	for i := range entries {
		entries[i].ID = int64(i + 1)
	}
	catalog, _ := newTestModelCatalogService(entries...)
	return NewModelPricingResolver(catalog)
}

// seededLiteLLMEntry 按价格文件条目播种一条目录条目（与 ModelCatalogService 播种同一函数）。
func seededLiteLLMEntry(t *testing.T, ps *PricingService, model string) ModelCatalogEntry {
	t.Helper()
	pricing := ps.GetModelPricing(model)
	if pricing == nil {
		t.Fatalf("price file has no %q", model)
	}
	return seedEntryFromLiteLLM(model, pricing)
}

// costViaCatalog 走目录计费（与网关同一入口）。
func costViaCatalog(t *testing.T, bs *BillingService, resolver *ModelPricingResolver, model string, tokens UsageTokens) *CostBreakdown {
	t.Helper()
	got, err := bs.CalculateTokenCostForRequest(TokenCostRequest{
		Ctx: context.Background(), Model: model, Tokens: tokens, RateMultiplier: 1,
		Resolver: resolver,
	})
	if err != nil {
		t.Fatalf("calculate %s: %v", model, err)
	}
	return got
}

// newSeededCatalogEnvFromJSON 用一份价格文件 JSON 搭计费环境，并把 models 按价格文件播种成目录条目（阶梯换算成分段）。
func newSeededCatalogEnvFromJSON(t *testing.T, body string, models ...string) (*BillingService, *ModelPricingResolver) {
	t.Helper()
	ps := newStubPricingServiceFromJSON(t, body)
	bs := NewBillingService()
	entries := make([]ModelCatalogEntry, 0, len(models))
	for _, model := range models {
		entries = append(entries, seededLiteLLMEntry(t, ps, model))
	}
	return bs, newResolverWithSeededEntries(bs, entries...)
}

// builtinPricing 内置价表里这个模型（精确键，不分大小写）的价。计费已不再查内置价表（只认目录），
// 它只用来播种目录；校验价表数据的用例用它。
func builtinPricing(bs *BillingService, model string) (*ModelPricing, error) {
	if pricing := bs.fallbackPrices[strings.ToLower(strings.TrimSpace(model))]; pricing != nil {
		return pricing, nil
	}
	return nil, ErrModelPricingUnavailable
}

// builtinSeededResolver 把内置价表的每个模型按播种规则（seedEntryFromFallback）转成目录条目再建解析器：
// 原来靠内置价表算钱的用例改走与网关相同的目录计费。
func builtinSeededResolver(bs *BillingService) *ModelPricingResolver {
	entries := make([]ModelCatalogEntry, 0, len(bs.fallbackPrices))
	for name, pricing := range bs.SnapshotFallbackPricing() {
		entry := seedEntryFromFallback(name, pricing)
		entry.Status = ModelCatalogStatusListed
		entries = append(entries, entry)
	}
	return newResolverWithSeededEntries(bs, entries...)
}

// builtinCatalogCost 按内置价表播种出的目录计费（替代已删除的 CalculateCost(model) 旧入口）。
func builtinCatalogCost(bs *BillingService, model string, tokens UsageTokens, rateMultiplier float64) (*CostBreakdown, error) {
	return bs.CalculateCostUnified(CostInput{
		Ctx: context.Background(), Model: model, Tokens: tokens, RequestCount: 1,
		RateMultiplier: rateMultiplier, Resolver: builtinSeededResolver(bs),
	})
}
