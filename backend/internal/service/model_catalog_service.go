package service

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"
)

// ModelCatalogRepository 是模型目录的数据访问接口。
// 条目自带别名、分档与分时配置：目录是计费热路径的价格来源，一次读全比分开读
// 更容易保证快照内部一致。
type ModelCatalogRepository interface {
	ListEntries(ctx context.Context) ([]ModelCatalogEntry, error)
	GetEntryByID(ctx context.Context, id int64) (*ModelCatalogEntry, error)
	GetEntryByModelID(ctx context.Context, modelID string) (*ModelCatalogEntry, error)
	CreateEntry(ctx context.Context, entry *ModelCatalogEntry) error
	UpdateEntry(ctx context.Context, entry *ModelCatalogEntry) error
	DeleteEntry(ctx context.Context, id int64) error

	CreateAlias(ctx context.Context, alias *ModelCatalogAlias) error
	UpdateAlias(ctx context.Context, alias *ModelCatalogAlias) error
	DeleteAlias(ctx context.Context, id int64) error

	// InsertOrRefreshSeedEntries 写入播种条目：模型标识不存在则插入，
	// 已存在且 managed_by = 'seed' 则刷新价格，managed_by = 'admin' 则整条跳过。
	InsertOrRefreshSeedEntries(ctx context.Context, entries []ModelCatalogEntry) (ModelCatalogSeedResult, error)
}

// ModelCatalogCachePubSub 在多实例之间广播目录缓存失效。
type ModelCatalogCachePubSub interface {
	NotifyUpdate(ctx context.Context) error
	SubscribeUpdates(ctx context.Context, handler func())
}

// ModelCatalogSeedResult 播种结果。
type ModelCatalogSeedResult struct {
	Inserted        int `json:"inserted"`
	Refreshed       int `json:"refreshed"`
	SkippedAdmin    int `json:"skipped_admin"`
	SkippedInvalid  int `json:"skipped_invalid"`
	CandidateModels int `json:"candidate_models"`
}

// modelCatalogCacheTTL 是本地快照的最长陈旧时间。
// 目录写入会在本实例立即失效缓存，并通过 pub/sub 通知其它实例；TTL 只是
// pub/sub 不可用（未接 Redis、消息丢失）时的兜底刷新周期。
const modelCatalogCacheTTL = 60 * time.Second

type wildcardCatalogAlias struct {
	prefix string
	entry  *ModelCatalogEntry
}

// modelCatalogSnapshot 是目录的只读查表快照。
type modelCatalogSnapshot struct {
	entries   []ModelCatalogEntry
	byModelID map[string]*ModelCatalogEntry
	byAlias   map[string]*ModelCatalogEntry
	// wildcards 按前缀长度降序：最长前缀胜出，结果不依赖行序。
	wildcards []wildcardCatalogAlias
	loadedAt  time.Time
}

// ModelCatalogPricingSource 是定价解析器依赖的最小接口。
type ModelCatalogPricingSource interface {
	LookupPricingEntry(ctx context.Context, model string) *ModelCatalogEntry
}

// ModelCatalogService 提供目录的读写与热路径查表。
type ModelCatalogService struct {
	repo      ModelCatalogRepository
	cachePub  ModelCatalogCachePubSub
	seedInput ModelCatalogSeedInput

	mu       sync.RWMutex
	snapshot *modelCatalogSnapshot
}

// NewModelCatalogService 创建目录服务并订阅跨实例失效通知。
func NewModelCatalogService(
	repo ModelCatalogRepository,
	cachePub ModelCatalogCachePubSub,
	seedInput ModelCatalogSeedInput,
) *ModelCatalogService {
	svc := &ModelCatalogService{repo: repo, cachePub: cachePub, seedInput: seedInput}
	if cachePub != nil {
		cachePub.SubscribeUpdates(context.Background(), func() {
			svc.invalidateLocal()
		})
	}
	return svc
}

func (s *ModelCatalogService) invalidateLocal() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.snapshot = nil
	s.mu.Unlock()
}

// invalidate 清掉本实例缓存并通知其它实例。
func (s *ModelCatalogService) invalidate(ctx context.Context) {
	s.invalidateLocal()
	if s.cachePub == nil {
		return
	}
	if err := s.cachePub.NotifyUpdate(ctx); err != nil {
		slog.Warn("failed to broadcast model catalog cache invalidation", "error", err)
	}
}

// loadSnapshot 返回当前快照，必要时从库里重建。
func (s *ModelCatalogService) loadSnapshot(ctx context.Context) *modelCatalogSnapshot {
	if s == nil || s.repo == nil {
		return nil
	}

	s.mu.RLock()
	current := s.snapshot
	s.mu.RUnlock()
	if current != nil && time.Since(current.loadedAt) < modelCatalogCacheTTL {
		return current
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// 双检：等锁期间可能已有人重建过。
	if s.snapshot != nil && time.Since(s.snapshot.loadedAt) < modelCatalogCacheTTL {
		return s.snapshot
	}

	entries, err := s.repo.ListEntries(ctx)
	if err != nil {
		// 重建失败时宁可继续用陈旧快照，也不要让整条计费链路查不到价（静默 $0）。
		slog.Warn("failed to reload model catalog", "error", err)
		return s.snapshot
	}
	s.snapshot = buildModelCatalogSnapshot(entries)
	return s.snapshot
}

func buildModelCatalogSnapshot(entries []ModelCatalogEntry) *modelCatalogSnapshot {
	snapshot := &modelCatalogSnapshot{
		entries:   entries,
		byModelID: make(map[string]*ModelCatalogEntry, len(entries)),
		byAlias:   make(map[string]*ModelCatalogEntry),
		loadedAt:  time.Now(),
	}
	for i := range entries {
		entry := &snapshot.entries[i]
		snapshot.byModelID[NormalizeModelCatalogKey(entry.ModelID)] = entry
		for _, alias := range entry.Aliases {
			key := NormalizeModelCatalogKey(alias.Alias)
			if prefix, wild := splitWildcardSuffix(key); wild {
				snapshot.wildcards = append(snapshot.wildcards, wildcardCatalogAlias{prefix: prefix, entry: entry})
				continue
			}
			snapshot.byAlias[key] = entry
		}
	}
	sort.SliceStable(snapshot.wildcards, func(i, j int) bool {
		return len(snapshot.wildcards[i].prefix) > len(snapshot.wildcards[j].prefix)
	})
	return snapshot
}

func (snapshot *modelCatalogSnapshot) lookupNormalized(key string) *ModelCatalogEntry {
	if snapshot == nil || key == "" {
		return nil
	}
	if entry, ok := snapshot.byModelID[key]; ok {
		return entry
	}
	if entry, ok := snapshot.byAlias[key]; ok {
		return entry
	}
	for _, wildcard := range snapshot.wildcards {
		if strings.HasPrefix(key, wildcard.prefix) {
			return wildcard.entry
		}
	}
	return nil
}

// LookupPricingEntry 按模型名查目录条目：先精确模型标识，再精确别名，
// 最后按最长前缀匹配通配别名。全部未命中时再用 OpenAI/Codex 的归一化基名重试一次
// （与渠道定价此前的 lookupChannelPricingNormalized 同口径，见 issue #5256）。
//
// 上架状态（listed/unlisted）不参与查表：本阶段目录只换价格来源，不改准入。
func (s *ModelCatalogService) LookupPricingEntry(ctx context.Context, model string) *ModelCatalogEntry {
	if s == nil {
		return nil
	}
	snapshot := s.loadSnapshot(ctx)
	if snapshot == nil {
		return nil
	}
	literal := strings.TrimSpace(model)
	if entry := snapshot.lookupNormalized(NormalizeModelCatalogKey(literal)); entry != nil {
		return entry
	}
	normalized := normalizeKnownOpenAICodexModel(literal)
	if normalized == "" || strings.EqualFold(normalized, literal) {
		return nil
	}
	return snapshot.lookupNormalized(NormalizeModelCatalogKey(normalized))
}

// ListEntries 返回目录全量条目（按模型标识排序）。
func (s *ModelCatalogService) ListEntries(ctx context.Context) ([]ModelCatalogEntry, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	return s.repo.ListEntries(ctx)
}

// GetEntry 按 ID 取条目。
func (s *ModelCatalogService) GetEntry(ctx context.Context, id int64) (*ModelCatalogEntry, error) {
	if s == nil || s.repo == nil {
		return nil, ErrModelCatalogEntryNotFound
	}
	return s.repo.GetEntryByID(ctx, id)
}

// CreateEntry 创建条目。管理员建的条目一律记为 admin，播种器不会再覆盖它。
func (s *ModelCatalogService) CreateEntry(ctx context.Context, entry *ModelCatalogEntry) error {
	if s == nil || s.repo == nil {
		return ErrModelCatalogEntryNotFound
	}
	entry.ManagedBy = ModelCatalogManagedByAdmin
	entry.Normalize()
	if err := entry.Validate(); err != nil {
		return err
	}
	if err := s.repo.CreateEntry(ctx, entry); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}

// UpdateEntry 更新条目。任何一次管理端写入都会把 managed_by 翻成 admin，
// 之后播种器不再覆盖——这是「播种不能悄悄盖掉管理员改动」的落点。
func (s *ModelCatalogService) UpdateEntry(ctx context.Context, entry *ModelCatalogEntry) error {
	if s == nil || s.repo == nil {
		return ErrModelCatalogEntryNotFound
	}
	entry.ManagedBy = ModelCatalogManagedByAdmin
	entry.Normalize()
	if err := entry.Validate(); err != nil {
		return err
	}
	if err := s.repo.UpdateEntry(ctx, entry); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}

// DeleteEntry 删除条目（别名、分档、分时由外键级联删除）。
func (s *ModelCatalogService) DeleteEntry(ctx context.Context, id int64) error {
	if s == nil || s.repo == nil {
		return ErrModelCatalogEntryNotFound
	}
	if err := s.repo.DeleteEntry(ctx, id); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}

// CreateAlias 新增别名。
func (s *ModelCatalogService) CreateAlias(ctx context.Context, alias *ModelCatalogAlias) error {
	if s == nil || s.repo == nil {
		return ErrModelCatalogAliasNotFound
	}
	alias.Alias = NormalizeModelCatalogAlias(alias.Alias)
	if alias.Source == "" {
		alias.Source = ModelCatalogAliasSourceManual
	}
	if err := ValidateModelCatalogAlias(alias.Alias, alias.Source); err != nil {
		return err
	}
	if err := s.repo.CreateAlias(ctx, alias); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}

// UpdateAlias 更新别名。
func (s *ModelCatalogService) UpdateAlias(ctx context.Context, alias *ModelCatalogAlias) error {
	if s == nil || s.repo == nil {
		return ErrModelCatalogAliasNotFound
	}
	alias.Alias = NormalizeModelCatalogAlias(alias.Alias)
	if alias.Source == "" {
		alias.Source = ModelCatalogAliasSourceManual
	}
	if err := ValidateModelCatalogAlias(alias.Alias, alias.Source); err != nil {
		return err
	}
	if err := s.repo.UpdateAlias(ctx, alias); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}

// DeleteAlias 删除别名。
func (s *ModelCatalogService) DeleteAlias(ctx context.Context, id int64) error {
	if s == nil || s.repo == nil {
		return ErrModelCatalogAliasNotFound
	}
	if err := s.repo.DeleteAlias(ctx, id); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}
