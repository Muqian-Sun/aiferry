package repository

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"slices"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/modelcatalogalias"
	"github.com/Wei-Shaw/sub2api/ent/modelcatalogbinding"
	"github.com/Wei-Shaw/sub2api/ent/modelcatalogentry"
	"github.com/Wei-Shaw/sub2api/ent/modelcatalogpriceinterval"
	"github.com/Wei-Shaw/sub2api/ent/modelcatalogtimepricing"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type modelCatalogRepository struct {
	client *dbent.Client
	// sql 用于向 scheduler_outbox 投递「绑定变了」事件，调度快照据此重建目录桶。
	sql *sql.DB
}

// NewModelCatalogRepository 创建模型目录仓储。
func NewModelCatalogRepository(client *dbent.Client, sqlDB *sql.DB) service.ModelCatalogRepository {
	return &modelCatalogRepository{client: client, sql: sqlDB}
}

// ListEntries 读取全量目录条目，并把别名、分档、分时一次性挂上去。
// 目录是计费热路径的价格来源，服务层按整份快照缓存，分开读会让快照内部不自洽。
func (r *modelCatalogRepository) ListEntries(ctx context.Context) ([]service.ModelCatalogEntry, error) {
	client := clientFromContext(ctx, r.client)

	rows, err := client.ModelCatalogEntry.Query().
		Order(dbent.Asc(modelcatalogentry.FieldModelID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	entries := make([]service.ModelCatalogEntry, 0, len(rows))
	byID := make(map[int64]*service.ModelCatalogEntry, len(rows))
	for _, row := range rows {
		entries = append(entries, *modelCatalogEntryToService(row))
	}
	for i := range entries {
		byID[entries[i].ID] = &entries[i]
	}
	if len(entries) == 0 {
		return entries, nil
	}

	aliases, err := client.ModelCatalogAlias.Query().
		Order(dbent.Asc(modelcatalogalias.FieldAlias)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range aliases {
		if entry, ok := byID[row.EntryID]; ok {
			entry.Aliases = append(entry.Aliases, *modelCatalogAliasToService(row))
		}
	}

	intervals, err := client.ModelCatalogPriceInterval.Query().
		Order(
			dbent.Asc(modelcatalogpriceinterval.FieldSortOrder),
			dbent.Asc(modelcatalogpriceinterval.FieldID),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range intervals {
		if entry, ok := byID[row.EntryID]; ok {
			entry.Intervals = append(entry.Intervals, modelCatalogIntervalToService(row))
		}
	}

	timePricings, err := client.ModelCatalogTimePricing.Query().All(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range timePricings {
		if entry, ok := byID[row.EntryID]; ok {
			entry.TimePricing = modelCatalogTimePricingToService(row)
		}
	}

	bindings, err := client.ModelCatalogBinding.Query().
		Order(
			dbent.Asc(modelcatalogbinding.FieldEntryID),
			dbent.Asc(modelcatalogbinding.FieldAccountID),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range bindings {
		if entry, ok := byID[row.EntryID]; ok {
			entry.Bindings = append(entry.Bindings, modelCatalogBindingToService(row))
		}
	}

	return entries, nil
}

func (r *modelCatalogRepository) GetEntryByID(ctx context.Context, id int64) (*service.ModelCatalogEntry, error) {
	client := clientFromContext(ctx, r.client)
	row, err := client.ModelCatalogEntry.Get(ctx, id)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrModelCatalogEntryNotFound, nil)
	}
	return r.hydrateEntry(ctx, modelCatalogEntryToService(row))
}

func (r *modelCatalogRepository) GetEntryByModelID(ctx context.Context, modelID string) (*service.ModelCatalogEntry, error) {
	client := clientFromContext(ctx, r.client)
	row, err := client.ModelCatalogEntry.Query().
		Where(modelcatalogentry.ModelIDEqualFold(modelID)).
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrModelCatalogEntryNotFound, nil)
	}
	return r.hydrateEntry(ctx, modelCatalogEntryToService(row))
}

func (r *modelCatalogRepository) hydrateEntry(ctx context.Context, entry *service.ModelCatalogEntry) (*service.ModelCatalogEntry, error) {
	client := clientFromContext(ctx, r.client)

	aliases, err := client.ModelCatalogAlias.Query().
		Where(modelcatalogalias.EntryIDEQ(entry.ID)).
		Order(dbent.Asc(modelcatalogalias.FieldAlias)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range aliases {
		entry.Aliases = append(entry.Aliases, *modelCatalogAliasToService(row))
	}

	intervals, err := client.ModelCatalogPriceInterval.Query().
		Where(modelcatalogpriceinterval.EntryIDEQ(entry.ID)).
		Order(
			dbent.Asc(modelcatalogpriceinterval.FieldSortOrder),
			dbent.Asc(modelcatalogpriceinterval.FieldID),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range intervals {
		entry.Intervals = append(entry.Intervals, modelCatalogIntervalToService(row))
	}

	timePricing, err := client.ModelCatalogTimePricing.Query().
		Where(modelcatalogtimepricing.EntryIDEQ(entry.ID)).
		Only(ctx)
	if err != nil && !dbent.IsNotFound(err) {
		return nil, err
	}
	if timePricing != nil {
		entry.TimePricing = modelCatalogTimePricingToService(timePricing)
	}

	bindings, err := r.ListBindingsByEntry(ctx, entry.ID)
	if err != nil {
		return nil, err
	}
	entry.Bindings = bindings
	return entry, nil
}

// ListBindingsByEntry 返回条目的承接关系（带上游价，按账号 ID 排序）。
func (r *modelCatalogRepository) ListBindingsByEntry(ctx context.Context, entryID int64) ([]service.ModelCatalogBinding, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.ModelCatalogBinding.Query().
		Where(modelcatalogbinding.EntryIDEQ(entryID)).
		Order(dbent.Asc(modelcatalogbinding.FieldAccountID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	bindings := make([]service.ModelCatalogBinding, 0, len(rows))
	for _, row := range rows {
		bindings = append(bindings, modelCatalogBindingToService(row))
	}
	return bindings, nil
}

// SaveEntryPricing 价格页按模型保存：更新条目（官方价与分段）并整份覆盖它的承接关系，同一事务；
// 提交后通知调度重建这个条目的桶。
func (r *modelCatalogRepository) SaveEntryPricing(ctx context.Context, entry *service.ModelCatalogEntry, bindings []service.ModelCatalogBinding) error {
	if entry == nil {
		return service.ErrModelCatalogEntryNotFound
	}
	err := r.withTx(ctx, func(tx *dbent.Tx) error {
		updated, err := applyCatalogEntryUpdate(tx.ModelCatalogEntry.UpdateOneID(entry.ID), entry).Save(ctx)
		if err != nil {
			return translatePersistenceError(err, service.ErrModelCatalogEntryNotFound, service.ErrModelCatalogEntryExists)
		}
		entry.CreatedAt = updated.CreatedAt
		entry.UpdatedAt = updated.UpdatedAt
		if err := replaceCatalogChildren(ctx, tx, entry); err != nil {
			return err
		}
		if _, err := tx.ModelCatalogBinding.Delete().Where(modelcatalogbinding.EntryIDEQ(entry.ID)).Exec(ctx); err != nil {
			return err
		}
		for _, binding := range bindings {
			if err := createCatalogBinding(ctx, tx, entry.ID, binding.AccountID, binding); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return r.enqueueCatalogBindingsChanged(ctx, entry.ID)
}

// ReplaceAccountBindings 价格页按渠道保存：整份覆盖渠道的承接关系（删掉不在列表里的、改价、新增），
// 同一事务；提交后按受影响的条目（原有 ∪ 新）通知调度。
func (r *modelCatalogRepository) ReplaceAccountBindings(ctx context.Context, accountID int64, bindings []service.ModelCatalogBinding) error {
	affected := make(map[int64]struct{})
	err := r.withTx(ctx, func(tx *dbent.Tx) error {
		existing, err := tx.ModelCatalogBinding.Query().Where(modelcatalogbinding.AccountIDEQ(accountID)).All(ctx)
		if err != nil {
			return err
		}
		for _, row := range existing {
			affected[row.EntryID] = struct{}{}
		}
		if _, err := tx.ModelCatalogBinding.Delete().Where(modelcatalogbinding.AccountIDEQ(accountID)).Exec(ctx); err != nil {
			return err
		}
		for _, binding := range bindings {
			if err := createCatalogBinding(ctx, tx, binding.EntryID, accountID, binding); err != nil {
				return err
			}
			affected[binding.EntryID] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return err
	}
	entryIDs := make([]int64, 0, len(affected))
	for id := range affected {
		entryIDs = append(entryIDs, id)
	}
	slices.Sort(entryIDs)
	for _, id := range entryIDs {
		if err := r.enqueueCatalogBindingsChanged(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// createCatalogBinding 写一条承接关系（带上游价）；上游分段存成 domain.PriceSegment 的 JSONB 数组，按传入顺序。
func createCatalogBinding(ctx context.Context, tx *dbent.Tx, entryID, accountID int64, binding service.ModelCatalogBinding) error {
	segments := make([]domain.PriceSegment, 0, len(binding.Intervals))
	for _, iv := range binding.Intervals {
		segments = append(segments, domain.PriceSegment{
			MinTokens:         iv.MinTokens,
			MaxTokens:         iv.MaxTokens,
			InputPrice:        iv.InputPrice,
			OutputPrice:       iv.OutputPrice,
			CacheWritePrice:   iv.CacheWritePrice,
			CacheWrite1hPrice: iv.CacheWrite1hPrice,
			CacheReadPrice:    iv.CacheReadPrice,
		})
	}
	_, err := tx.ModelCatalogBinding.Create().
		SetEntryID(entryID).
		SetAccountID(accountID).
		SetInputPrice(binding.InputPrice).
		SetOutputPrice(binding.OutputPrice).
		SetNillableCacheWritePrice(binding.CacheWritePrice).
		SetNillableCacheWrite1hPrice(binding.CacheWrite1hPrice).
		SetNillableCacheReadPrice(binding.CacheReadPrice).
		SetPriceIntervals(segments).
		Save(ctx)
	return translatePersistenceError(err, service.ErrModelCatalogEntryNotFound, nil)
}

func (r *modelCatalogRepository) enqueueCatalogBindingsChanged(ctx context.Context, entryID int64) error {
	payload := map[string]any{"entry_ids": []int64{entryID}}
	return enqueueSchedulerOutbox(ctx, r.sql, service.SchedulerOutboxEventCatalogBindingsChanged, nil, payload)
}

func (r *modelCatalogRepository) CreateEntry(ctx context.Context, entry *service.ModelCatalogEntry) error {
	if entry == nil {
		return service.ErrModelCatalogEntryNotFound
	}
	return r.withTx(ctx, func(tx *dbent.Tx) error {
		created, err := applyCatalogEntryCreate(tx.ModelCatalogEntry.Create(), entry).Save(ctx)
		if err != nil {
			return translatePersistenceError(err, nil, service.ErrModelCatalogEntryExists)
		}
		entry.ID = created.ID
		entry.CreatedAt = created.CreatedAt
		entry.UpdatedAt = created.UpdatedAt
		return replaceCatalogChildren(ctx, tx, entry)
	})
}

func (r *modelCatalogRepository) UpdateEntry(ctx context.Context, entry *service.ModelCatalogEntry) error {
	if entry == nil {
		return service.ErrModelCatalogEntryNotFound
	}
	return r.withTx(ctx, func(tx *dbent.Tx) error {
		updated, err := applyCatalogEntryUpdate(tx.ModelCatalogEntry.UpdateOneID(entry.ID), entry).Save(ctx)
		if err != nil {
			return translatePersistenceError(err, service.ErrModelCatalogEntryNotFound, service.ErrModelCatalogEntryExists)
		}
		entry.CreatedAt = updated.CreatedAt
		entry.UpdatedAt = updated.UpdatedAt
		return replaceCatalogChildren(ctx, tx, entry)
	})
}

// DeleteEntry 删除条目；绑定由外键级联删除，所以同样要通知调度退役该条目的桶。
func (r *modelCatalogRepository) DeleteEntry(ctx context.Context, id int64) error {
	client := clientFromContext(ctx, r.client)
	if err := client.ModelCatalogEntry.DeleteOneID(id).Exec(ctx); err != nil {
		return translatePersistenceError(err, service.ErrModelCatalogEntryNotFound, nil)
	}
	return r.enqueueCatalogBindingsChanged(ctx, id)
}

func (r *modelCatalogRepository) CreateAlias(ctx context.Context, alias *service.ModelCatalogAlias) error {
	if alias == nil {
		return service.ErrModelCatalogAliasNotFound
	}
	client := clientFromContext(ctx, r.client)
	builder := client.ModelCatalogAlias.Create().
		SetAlias(alias.Alias).
		SetEntryID(alias.EntryID).
		SetSource(alias.Source)
	if alias.Notes != nil {
		builder = builder.SetNotes(*alias.Notes)
	}
	created, err := builder.Save(ctx)
	if err != nil {
		return translateModelCatalogAliasError(err)
	}
	*alias = *modelCatalogAliasToService(created)
	return nil
}

// translateModelCatalogAliasError 把别名写入的库错误映射成业务错误：
// 别名重名 → 409；entry_id 指向不存在的条目（外键冲突）→ 404，而不是 500。
func translateModelCatalogAliasError(err error) error {
	if isForeignKeyViolation(err) {
		return service.ErrModelCatalogEntryNotFound.WithCause(err)
	}
	return translatePersistenceError(err, service.ErrModelCatalogAliasNotFound, service.ErrModelCatalogAliasExists)
}

func (r *modelCatalogRepository) UpdateAlias(ctx context.Context, alias *service.ModelCatalogAlias) error {
	if alias == nil {
		return service.ErrModelCatalogAliasNotFound
	}
	client := clientFromContext(ctx, r.client)
	builder := client.ModelCatalogAlias.UpdateOneID(alias.ID).
		SetAlias(alias.Alias).
		SetEntryID(alias.EntryID).
		SetSource(alias.Source)
	if alias.Notes != nil {
		builder = builder.SetNotes(*alias.Notes)
	} else {
		builder = builder.ClearNotes()
	}
	updated, err := builder.Save(ctx)
	if err != nil {
		return translateModelCatalogAliasError(err)
	}
	*alias = *modelCatalogAliasToService(updated)
	return nil
}

func (r *modelCatalogRepository) DeleteAlias(ctx context.Context, id int64) error {
	client := clientFromContext(ctx, r.client)
	if err := client.ModelCatalogAlias.DeleteOneID(id).Exec(ctx); err != nil {
		return translatePersistenceError(err, service.ErrModelCatalogAliasNotFound, nil)
	}
	return nil
}

// InsertOrRefreshSeedEntries 按「不存在则插入 / seed 则刷新 / admin 则跳过」写入播种条目。
//
// 种子自带分档（Intervals）时随条目一起整份覆盖（seed 条目的分档跟着种子走）；
// 种子自带别名（SeedAliases）时逐个补齐，已被占用的别名跳过并打 warn。
// 其它条目（价格文件 / 兜底表）不带这两样，播种既不写也不清空它们。
func (r *modelCatalogRepository) InsertOrRefreshSeedEntries(
	ctx context.Context,
	entries []service.ModelCatalogEntry,
) (service.ModelCatalogSeedResult, error) {
	var result service.ModelCatalogSeedResult
	if len(entries) == 0 {
		return result, nil
	}
	client := clientFromContext(ctx, r.client)

	existing, err := client.ModelCatalogEntry.Query().
		Select(modelcatalogentry.FieldID, modelcatalogentry.FieldModelID, modelcatalogentry.FieldManagedBy).
		All(ctx)
	if err != nil {
		return result, err
	}
	type existingEntry struct {
		id        int64
		managedBy string
	}
	byKey := make(map[string]existingEntry, len(existing))
	for _, row := range existing {
		byKey[service.NormalizeModelCatalogKey(row.ModelID)] = existingEntry{id: row.ID, managedBy: row.ManagedBy}
	}

	// 单条写失败只跳过这一条：一条坏数据不该让后面几百条都播不进去。
	// ctx 到期（库挂死 / 启动超时）才整体中止。
	for i := range entries {
		entry := entries[i]
		key := service.NormalizeModelCatalogKey(entry.ModelID)
		current, ok := byKey[key]
		if !ok {
			created, createErr := applyCatalogEntryCreate(client.ModelCatalogEntry.Create(), &entry).Save(ctx)
			if createErr != nil {
				// 并发播种（多实例同时启动）会撞唯一索引。此时另一边已经写进去了，
				// 记为跳过而不是失败。
				if isUniqueConstraintViolation(createErr) {
					result.SkippedAdmin++
					continue
				}
				if abort := seedRowFailed(ctx, &result, "insert", entry.ModelID, createErr); abort != nil {
					return result, abort
				}
				continue
			}
			if abort := r.writeSeedChildren(ctx, &result, created.ID, &entry); abort != nil {
				return result, abort
			}
			result.Inserted++
			continue
		}
		if current.managedBy != service.ModelCatalogManagedBySeed {
			result.SkippedAdmin++
			continue
		}
		if _, updateErr := applyCatalogEntryUpdate(client.ModelCatalogEntry.UpdateOneID(current.id), &entry).Save(ctx); updateErr != nil {
			if abort := seedRowFailed(ctx, &result, "refresh", entry.ModelID, updateErr); abort != nil {
				return result, abort
			}
			continue
		}
		if abort := r.writeSeedChildren(ctx, &result, current.id, &entry); abort != nil {
			return result, abort
		}
		result.Refreshed++
	}
	return result, nil
}

// writeSeedChildren 写种子条目自带的分档与别名；失败按单条播种失败处理（ctx 到期才整体中止）。
func (r *modelCatalogRepository) writeSeedChildren(ctx context.Context, result *service.ModelCatalogSeedResult, entryID int64, entry *service.ModelCatalogEntry) error {
	if len(entry.Intervals) > 0 {
		entry.ID = entryID
		err := r.withTx(ctx, func(tx *dbent.Tx) error {
			return replaceCatalogChildren(ctx, tx, entry)
		})
		if err != nil {
			if abort := seedRowFailed(ctx, result, "intervals", entry.ModelID, err); abort != nil {
				return abort
			}
		}
	}
	for _, alias := range entry.SeedAliases {
		record := &service.ModelCatalogAlias{EntryID: entryID, Alias: alias, Source: service.ModelCatalogAliasSourceSeed}
		err := r.CreateAlias(ctx, record)
		if err == nil {
			continue
		}
		if errors.Is(err, service.ErrModelCatalogAliasExists) {
			// 管理员已手建同名别名（可能指向别的条目）：不覆盖，但要有声音。
			existing, lookupErr := r.client.ModelCatalogAlias.Query().
				Where(modelcatalogalias.AliasEqualFold(alias)).
				Only(ctx)
			existingEntryID := int64(0)
			if lookupErr == nil && existing != nil {
				existingEntryID = existing.EntryID
			}
			if existingEntryID == entryID {
				continue // 上次播种已写过，同一条目
			}
			slog.Warn("model catalog seed: alias already taken, skipped",
				"alias", alias, "wanted_entry_id", entryID, "existing_entry_id", existingEntryID, "model_id", entry.ModelID)
			continue
		}
		if abort := seedRowFailed(ctx, result, "alias", entry.ModelID, err); abort != nil {
			return abort
		}
	}
	return nil
}

// seedRowFailed 处理一条播种写入失败：ctx 已到期说明失败不是这条数据的问题
// （库挂死 / 启动超时），返回 ctx 错误让整批中止；否则记数、打日志、继续下一条。
func seedRowFailed(ctx context.Context, result *service.ModelCatalogSeedResult, op, modelID string, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	slog.Warn("model catalog seed: "+op+" failed", "model_id", modelID, "error", err)
	result.RecordFailure(modelID, err)
	return nil
}

func (r *modelCatalogRepository) withTx(ctx context.Context, fn func(tx *dbent.Tx) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(tx)
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		return err
	}
	return tx.Commit()
}

// replaceCatalogChildren 用整份配置覆盖条目的分档与分时（别名走独立端点，不在此处动）。
func replaceCatalogChildren(ctx context.Context, tx *dbent.Tx, entry *service.ModelCatalogEntry) error {
	if _, err := tx.ModelCatalogPriceInterval.Delete().
		Where(modelcatalogpriceinterval.EntryIDEQ(entry.ID)).Exec(ctx); err != nil {
		return err
	}
	for i := range entry.Intervals {
		interval := entry.Intervals[i]
		builder := tx.ModelCatalogPriceInterval.Create().
			SetEntryID(entry.ID).
			SetMinTokens(interval.MinTokens).
			SetTierLabel(interval.TierLabel).
			SetSortOrder(interval.SortOrder).
			SetNillableMaxTokens(interval.MaxTokens).
			SetNillableInputPrice(interval.InputPrice).
			SetNillableOutputPrice(interval.OutputPrice).
			SetNillableCacheWritePrice(interval.CacheWritePrice).
			SetNillableCacheWrite1hPrice(interval.CacheWrite1hPrice).
			SetNillableCacheReadPrice(interval.CacheReadPrice).
			SetNillablePerRequestPrice(interval.PerRequestPrice).
			SetNillableInputMultiplier(interval.InputMultiplier).
			SetNillableOutputMultiplier(interval.OutputMultiplier).
			SetNillableCacheWriteMultiplier(interval.CacheWriteMultiplier).
			SetNillableCacheReadMultiplier(interval.CacheReadMultiplier)
		created, err := builder.Save(ctx)
		if err != nil {
			return err
		}
		entry.Intervals[i].ID = created.ID
	}

	if _, err := tx.ModelCatalogTimePricing.Delete().
		Where(modelcatalogtimepricing.EntryIDEQ(entry.ID)).Exec(ctx); err != nil {
		return err
	}
	if entry.TimePricing == nil {
		return nil
	}
	periods := make([]map[string]any, 0, len(entry.TimePricing.Periods))
	for _, period := range entry.TimePricing.Periods {
		periods = append(periods, map[string]any{
			"start_time": period.StartTime,
			"end_time":   period.EndTime,
			"multiplier": period.Multiplier,
		})
	}
	_, err := tx.ModelCatalogTimePricing.Create().
		SetEntryID(entry.ID).
		SetTimezone(entry.TimePricing.Timezone).
		SetWeekdaysOnly(entry.TimePricing.WeekdaysOnly).
		SetPeriods(periods).
		Save(ctx)
	return err
}

func applyCatalogEntryCreate(builder *dbent.ModelCatalogEntryCreate, entry *service.ModelCatalogEntry) *dbent.ModelCatalogEntryCreate {
	builder = builder.
		SetModelID(entry.ModelID).
		SetDisplayName(entry.DisplayName).
		SetVendor(entry.Vendor).
		SetBillingMode(string(entry.EffectiveBillingMode())).
		SetStatus(entry.Status).
		SetManagedBy(entry.ManagedBy).
		SetNillableInputPrice(entry.InputPrice).
		SetNillableOutputPrice(entry.OutputPrice).
		SetNillableCacheWritePrice(entry.CacheWritePrice).
		SetNillableCacheWrite1hPrice(entry.CacheWrite1hPrice).
		SetNillableCacheReadPrice(entry.CacheReadPrice).
		SetNillableImageInputPrice(entry.ImageInputPrice).
		SetNillableImageOutputPrice(entry.ImageOutputPrice).
		SetNillableImageCacheReadPrice(entry.ImageCacheReadPrice).
		SetNillableAudioInputPrice(entry.AudioInputPrice).
		SetNillableAudioOutputPrice(entry.AudioOutputPrice).
		SetNillablePerRequestPrice(entry.PerRequestPrice).
		SetNillableSearchPricePerCall(entry.SearchPricePerCall).
		SetNillableMaxReasoningEffortMultiplier(entry.MaxReasoningEffortMultiplier)
	if len(entry.Protocols) > 0 {
		builder = builder.SetProtocols(entry.Protocols)
	}
	if entry.Notes != nil {
		builder = builder.SetNotes(*entry.Notes)
	}
	return builder
}

// applyCatalogEntryUpdate 整条覆盖：没给值的列一律清空，避免旧值在部分更新后残留。
func applyCatalogEntryUpdate(builder *dbent.ModelCatalogEntryUpdateOne, entry *service.ModelCatalogEntry) *dbent.ModelCatalogEntryUpdateOne {
	builder = builder.
		SetModelID(entry.ModelID).
		SetDisplayName(entry.DisplayName).
		SetVendor(entry.Vendor).
		SetBillingMode(string(entry.EffectiveBillingMode())).
		SetStatus(entry.Status).
		SetManagedBy(entry.ManagedBy).
		SetProtocols(entry.Protocols)

	setPrice := func(
		set func(float64) *dbent.ModelCatalogEntryUpdateOne,
		clear func() *dbent.ModelCatalogEntryUpdateOne,
		value *float64,
	) {
		if value != nil {
			builder = set(*value)
			return
		}
		builder = clear()
	}

	setPrice(builder.SetInputPrice, builder.ClearInputPrice, entry.InputPrice)
	setPrice(builder.SetOutputPrice, builder.ClearOutputPrice, entry.OutputPrice)
	setPrice(builder.SetCacheWritePrice, builder.ClearCacheWritePrice, entry.CacheWritePrice)
	setPrice(builder.SetCacheWrite1hPrice, builder.ClearCacheWrite1hPrice, entry.CacheWrite1hPrice)
	setPrice(builder.SetCacheReadPrice, builder.ClearCacheReadPrice, entry.CacheReadPrice)
	setPrice(builder.SetImageInputPrice, builder.ClearImageInputPrice, entry.ImageInputPrice)
	setPrice(builder.SetImageOutputPrice, builder.ClearImageOutputPrice, entry.ImageOutputPrice)
	setPrice(builder.SetImageCacheReadPrice, builder.ClearImageCacheReadPrice, entry.ImageCacheReadPrice)
	setPrice(builder.SetAudioInputPrice, builder.ClearAudioInputPrice, entry.AudioInputPrice)
	setPrice(builder.SetAudioOutputPrice, builder.ClearAudioOutputPrice, entry.AudioOutputPrice)
	setPrice(builder.SetPerRequestPrice, builder.ClearPerRequestPrice, entry.PerRequestPrice)
	setPrice(builder.SetSearchPricePerCall, builder.ClearSearchPricePerCall, entry.SearchPricePerCall)
	setPrice(builder.SetMaxReasoningEffortMultiplier, builder.ClearMaxReasoningEffortMultiplier, entry.MaxReasoningEffortMultiplier)

	if entry.Notes != nil {
		builder = builder.SetNotes(*entry.Notes)
	} else {
		builder = builder.ClearNotes()
	}
	return builder
}

func modelCatalogEntryToService(row *dbent.ModelCatalogEntry) *service.ModelCatalogEntry {
	if row == nil {
		return nil
	}
	return &service.ModelCatalogEntry{
		ID:          row.ID,
		ModelID:     row.ModelID,
		DisplayName: row.DisplayName,
		Vendor:      row.Vendor,
		Protocols:   row.Protocols,
		BillingMode: service.BillingMode(row.BillingMode),
		Status:      row.Status,
		ManagedBy:   row.ManagedBy,

		InputPrice:          row.InputPrice,
		OutputPrice:         row.OutputPrice,
		CacheWritePrice:     row.CacheWritePrice,
		CacheWrite1hPrice:   row.CacheWrite1hPrice,
		CacheReadPrice:      row.CacheReadPrice,
		ImageInputPrice:     row.ImageInputPrice,
		ImageOutputPrice:    row.ImageOutputPrice,
		ImageCacheReadPrice: row.ImageCacheReadPrice,
		AudioInputPrice:     row.AudioInputPrice,
		AudioOutputPrice:    row.AudioOutputPrice,

		PerRequestPrice:    row.PerRequestPrice,
		SearchPricePerCall: row.SearchPricePerCall,

		MaxReasoningEffortMultiplier: row.MaxReasoningEffortMultiplier,

		Notes:     row.Notes,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

// modelCatalogBindingToService 把承接关系行（带上游价）转成服务层结构。上游分段价存成
// domain.PriceSegment 的 JSONB 数组，按数组下标排序：SortOrder = 下标，分段没有档位名。
func modelCatalogBindingToService(row *dbent.ModelCatalogBinding) service.ModelCatalogBinding {
	binding := service.ModelCatalogBinding{
		EntryID:           row.EntryID,
		AccountID:         row.AccountID,
		InputPrice:        row.InputPrice,
		OutputPrice:       row.OutputPrice,
		CacheWritePrice:   row.CacheWritePrice,
		CacheWrite1hPrice: row.CacheWrite1hPrice,
		CacheReadPrice:    row.CacheReadPrice,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
	if len(row.PriceIntervals) > 0 {
		binding.Intervals = make([]service.PricingInterval, 0, len(row.PriceIntervals))
		for i, segment := range row.PriceIntervals {
			binding.Intervals = append(binding.Intervals, service.PricingInterval{
				MinTokens:         segment.MinTokens,
				MaxTokens:         segment.MaxTokens,
				InputPrice:        segment.InputPrice,
				OutputPrice:       segment.OutputPrice,
				CacheWritePrice:   segment.CacheWritePrice,
				CacheWrite1hPrice: segment.CacheWrite1hPrice,
				CacheReadPrice:    segment.CacheReadPrice,
				SortOrder:         i,
			})
		}
	}
	return binding
}

func modelCatalogAliasToService(row *dbent.ModelCatalogAlias) *service.ModelCatalogAlias {
	if row == nil {
		return nil
	}
	return &service.ModelCatalogAlias{
		ID:        row.ID,
		EntryID:   row.EntryID,
		Alias:     row.Alias,
		Source:    row.Source,
		Notes:     row.Notes,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func modelCatalogIntervalToService(row *dbent.ModelCatalogPriceInterval) service.PricingInterval {
	return service.PricingInterval{
		ID:                   row.ID,
		MinTokens:            row.MinTokens,
		MaxTokens:            row.MaxTokens,
		TierLabel:            row.TierLabel,
		InputPrice:           row.InputPrice,
		OutputPrice:          row.OutputPrice,
		CacheWritePrice:      row.CacheWritePrice,
		CacheWrite1hPrice:    row.CacheWrite1hPrice,
		CacheReadPrice:       row.CacheReadPrice,
		InputMultiplier:      row.InputMultiplier,
		OutputMultiplier:     row.OutputMultiplier,
		CacheWriteMultiplier: row.CacheWriteMultiplier,
		CacheReadMultiplier:  row.CacheReadMultiplier,
		PerRequestPrice:      row.PerRequestPrice,
		SortOrder:            row.SortOrder,
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
	}
}

func modelCatalogTimePricingToService(row *dbent.ModelCatalogTimePricing) *service.TimePricing {
	if row == nil {
		return nil
	}
	cfg := &service.TimePricing{
		Timezone:     row.Timezone,
		WeekdaysOnly: row.WeekdaysOnly,
	}
	for _, raw := range row.Periods {
		period := service.TimePricingPeriod{}
		if value, ok := raw["start_time"].(string); ok {
			period.StartTime = value
		}
		if value, ok := raw["end_time"].(string); ok {
			period.EndTime = value
		}
		if value, ok := raw["multiplier"].(float64); ok {
			period.Multiplier = value
		}
		cfg.Periods = append(cfg.Periods, period)
	}
	return cfg
}
