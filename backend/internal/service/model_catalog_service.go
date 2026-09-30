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

	// ListBindingsByEntry 返回条目的承接关系（带上游价）。
	ListBindingsByEntry(ctx context.Context, entryID int64) ([]ModelCatalogBinding, error)
	// SaveEntryPricing 价格页按模型保存：更新条目（官方价与分段）并整份覆盖它的承接关系，同一事务；
	// 提交后向调度 outbox 投递 catalog_bindings_changed。
	SaveEntryPricing(ctx context.Context, entry *ModelCatalogEntry, bindings []ModelCatalogBinding) error
	// ReplaceAccountBindings 价格页按渠道保存：整份覆盖渠道的承接关系（带上游价），同一事务；
	// 提交后按受影响的条目（原有 ∪ 新）投递 catalog_bindings_changed。
	ReplaceAccountBindings(ctx context.Context, accountID int64, bindings []ModelCatalogBinding) error
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
	// Failed 是写库失败的条目数：单条失败不拖垮整批，但要计数并把前几条原因带回来。
	Failed int      `json:"failed"`
	Errors []string `json:"errors,omitempty"`
}

// modelCatalogSeedErrorSamples 是播种结果里最多带回的失败原因条数。
const modelCatalogSeedErrorSamples = 10

// RecordFailure 记一条写库失败：计数，并保留前几条原因供日志与管理端展示。
func (r *ModelCatalogSeedResult) RecordFailure(modelID string, err error) {
	r.Failed++
	if len(r.Errors) < modelCatalogSeedErrorSamples {
		r.Errors = append(r.Errors, modelID+": "+err.Error())
	}
}

// modelCatalogCacheTTL 是本地快照的最长陈旧时间。
// 目录写入会在本实例立即失效缓存，并通过 pub/sub 通知其它实例；TTL 只是
// pub/sub 不可用（未接 Redis、消息丢失）时的兜底刷新周期。
const modelCatalogCacheTTL = 60 * time.Second

// modelCatalogReloadTimeout 限制一次快照重建的库读取时长；重建用独立 ctx，
// 不随触发它的那个请求一起被取消。几百行的全表读是毫秒级，10s 只防挂死（拍的）。
const modelCatalogReloadTimeout = 10 * time.Second

// modelCatalogReloadBackoff 是重建失败后的退避：期间继续用陈旧快照，不再每个请求都打库。
// 5s 是拍的：比 TTL 短一个量级，库恢复后很快跟上；比单次请求长，能压住失败风暴。
const modelCatalogReloadBackoff = 5 * time.Second

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
	// reloading 非 nil 表示有一次重建在进行，重建结束时关闭；同一时刻只跑一次。
	reloading chan struct{}
	// retryAfter 是上次重建失败后允许再次重建的时刻。
	retryAfter time.Time
	// generation 每次失效 +1：失效之前开始的重建读到的是旧数据，结果作废。
	generation uint64
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
	s.generation++
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

func (snapshot *modelCatalogSnapshot) fresh() bool {
	return snapshot != nil && time.Since(snapshot.loadedAt) < modelCatalogCacheTTL
}

// loadSnapshot 返回当前快照，必要时从库里重建。
//
// 重建不持锁、不占用请求的 ctx、同一时刻只跑一次：
//   - 有陈旧快照（TTL 过期）：立刻返回陈旧快照，在后台刷新；
//   - 没有快照（首次 / 写入后失效）：由第一个到达的请求同步重建，其余请求等它完成；
//   - 重建失败：记退避时刻，期间继续用陈旧快照，不再每个请求都打库。
//
// 宁可继续用陈旧快照，也不要让整条计费链路查不到价（静默 $0）。
func (s *ModelCatalogService) loadSnapshot(ctx context.Context) *modelCatalogSnapshot {
	if s == nil || s.repo == nil {
		return nil
	}
	for {
		s.mu.RLock()
		current := s.snapshot
		s.mu.RUnlock()
		if current.fresh() {
			return current
		}

		s.mu.Lock()
		if s.snapshot.fresh() {
			snapshot := s.snapshot
			s.mu.Unlock()
			return snapshot
		}
		stale := s.snapshot
		done := s.reloading
		if done != nil {
			s.mu.Unlock()
			if stale != nil {
				return stale
			}
			select {
			case <-done:
				// 重建结果（成功 / 失败退避 / 被失效作废）交给下一轮判定。
				continue
			case <-ctx.Done():
				return nil
			}
		}
		if time.Now().Before(s.retryAfter) {
			s.mu.Unlock()
			return stale
		}
		done = make(chan struct{})
		s.reloading = done
		generation := s.generation
		s.mu.Unlock()

		if stale != nil {
			go s.reloadSnapshot(done, generation)
			return stale
		}
		s.reloadSnapshot(done, generation)
	}
}

// reloadSnapshot 从库里重建快照并关闭 done。用独立 ctx：触发重建的请求可能随时被取消，
// 不能让它把所有人的快照重建一起带走。generation 与开始时不一致说明中途发生过写入，
// 读到的是旧数据，丢弃不装，由下一个读请求重来。
func (s *ModelCatalogService) reloadSnapshot(done chan struct{}, generation uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), modelCatalogReloadTimeout)
	defer cancel()
	entries, err := s.repo.ListEntries(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()
	defer close(done)
	s.reloading = nil
	if err != nil {
		slog.Warn("failed to reload model catalog", "error", err)
		s.retryAfter = time.Now().Add(modelCatalogReloadBackoff)
		return
	}
	if s.generation != generation {
		return
	}
	s.retryAfter = time.Time{}
	s.snapshot = buildModelCatalogSnapshot(entries)
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

// lookupExactModelID 按模型标识逐字查条目（只去首尾空白），不认别名与大小写变体。
func (s *ModelCatalogService) lookupExactModelID(ctx context.Context, model string) *ModelCatalogEntry {
	if s == nil {
		return nil
	}
	snapshot := s.loadSnapshot(ctx)
	if snapshot == nil {
		return nil
	}
	literal := strings.TrimSpace(model)
	entry, ok := snapshot.byModelID[NormalizeModelCatalogKey(literal)]
	if !ok || entry.ModelID != literal {
		return nil
	}
	return entry
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
