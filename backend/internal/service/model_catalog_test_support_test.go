package service

import (
	"context"
	"errors"
	"strings"
	"sync"
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
	// bindings 按条目 ID 存绑定；replaceCalls 记 ReplaceBindings 被调了几次。
	bindings     map[int64][]ModelCatalogBinding
	replaceCalls int
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

func (r *stubModelCatalogRepo) ReplaceBindings(_ context.Context, entryID int64, bindings []ModelCatalogBinding) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.replaceCalls++
	if r.bindings == nil {
		r.bindings = make(map[int64][]ModelCatalogBinding)
	}
	r.bindings[entryID] = append([]ModelCatalogBinding(nil), bindings...)
	return nil
}

func (r *stubModelCatalogRepo) ListEntryIDsByAccount(_ context.Context, accountID int64) ([]int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var ids []int64
	for entryID, bindings := range r.bindings {
		for _, binding := range bindings {
			if binding.AccountID == accountID {
				ids = append(ids, entryID)
				break
			}
		}
	}
	return ids, nil
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

func (r *stubModelCatalogRepo) CreateAlias(_ context.Context, alias *ModelCatalogAlias) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := NormalizeModelCatalogKey(alias.Alias)
	for i := range r.entries {
		for _, existing := range r.entries[i].Aliases {
			if NormalizeModelCatalogKey(existing.Alias) == key {
				return ErrModelCatalogAliasExists
			}
		}
	}
	for i := range r.entries {
		if r.entries[i].ID == alias.EntryID {
			alias.ID = int64(len(r.entries[i].Aliases) + 1)
			r.entries[i].Aliases = append(r.entries[i].Aliases, *alias)
			return nil
		}
	}
	return ErrModelCatalogEntryNotFound
}

func (r *stubModelCatalogRepo) UpdateAlias(context.Context, *ModelCatalogAlias) error {
	return errors.New("not implemented in stub")
}

func (r *stubModelCatalogRepo) DeleteAlias(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.entries {
		for j := range r.entries[i].Aliases {
			if r.entries[i].Aliases[j].ID == id {
				r.entries[i].Aliases = append(r.entries[i].Aliases[:j], r.entries[i].Aliases[j+1:]...)
				return nil
			}
		}
	}
	return ErrModelCatalogAliasNotFound
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
			entry.Aliases = r.seedAliases(entry.ID, nil, entry.SeedAliases)
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
		// 与真仓储同口径：种子带分档时整份覆盖，否则保留；别名只补不删；分时不动。
		entry.Aliases = r.seedAliases(entry.ID, r.entries[pos].Aliases, entry.SeedAliases)
		if len(entry.Intervals) == 0 {
			entry.Intervals = r.entries[pos].Intervals
		}
		entry.TimePricing = r.entries[pos].TimePricing
		r.entries[pos] = entry
		result.Refreshed++
	}
	return result, nil
}

// seedAliases 模拟仓储写种子别名：全局（跨条目）已被占用的别名跳过。
func (r *stubModelCatalogRepo) seedAliases(entryID int64, existing []ModelCatalogAlias, seeds []string) []ModelCatalogAlias {
	out := append([]ModelCatalogAlias(nil), existing...)
	taken := make(map[string]bool)
	for i := range r.entries {
		for _, alias := range r.entries[i].Aliases {
			taken[strings.ToLower(alias.Alias)] = true
		}
	}
	for _, alias := range existing {
		taken[strings.ToLower(alias.Alias)] = true
	}
	for _, alias := range seeds {
		if taken[strings.ToLower(alias)] {
			continue
		}
		taken[strings.ToLower(alias)] = true
		out = append(out, ModelCatalogAlias{EntryID: entryID, Alias: alias, Source: ModelCatalogAliasSourceSeed})
	}
	return out
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
		SearchPricePerCall:           card.SearchPricePerCall,
		FastMultiplier:               card.FastMultiplier,
		FlexMultiplier:               card.FlexMultiplier,
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
		// 第一个非通配名做模型标识，其余（含 "foo-*" 这类模式）落成别名。
		primary := ""
		for _, model := range card.Models {
			if !strings.Contains(model, "*") {
				primary = model
				break
			}
		}
		if primary == "" {
			primary = strings.TrimSuffix(card.Models[0], "*")
		}
		entry := catalogEntryFromCard(primary, ModelCatalogManagedByAdmin, card)
		for _, model := range card.Models {
			if model == primary {
				continue
			}
			entry.Aliases = append(entry.Aliases, ModelCatalogAlias{
				Alias:  model,
				Source: ModelCatalogAliasSourceManual,
			})
		}
		entries = append(entries, entry)
	}
	for i := range entries {
		entries[i].ID = int64(i + 1)
	}
	catalog, _ := newTestModelCatalogService(entries...)
	return NewModelPricingResolver(catalog, bs)
}
