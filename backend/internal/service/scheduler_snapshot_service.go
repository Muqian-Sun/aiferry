package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

var (
	ErrSchedulerCacheNotReady           = errors.New("scheduler cache not ready")
	ErrSchedulerFallbackLimited         = errors.New("scheduler db fallback limited")
	ErrSchedulerGroupLifecycleLeaseBusy = errors.New("scheduler group lifecycle lease busy")
	ErrSchedulerBucketRebuildBusy       = errors.New("scheduler bucket rebuild busy")
)

const (
	outboxEventTimeout                = 2 * time.Minute
	schedulerOutboxCleanupBatch       = 5000
	outboxRebuildRetryBaseDelay       = 5 * time.Second
	outboxRebuildRetryMaxDelay        = 5 * time.Minute
	outboxMaxIDErrorLogSampleInterval = time.Minute
)

// batchSeenKey tracks completed per-platform rebuilds and group lifecycle work
// within one pollOutbox call.
type batchSeenKey struct {
	platform string
}

type schedulerBucketWriteTask struct {
	bucket SchedulerBucket
	token  SchedulerBucketWriteToken
}

type SchedulerSnapshotService struct {
	cache                        SchedulerCache
	outboxRepo                   SchedulerOutboxRepository
	accountRepo                  AccountRepository
	cfg                          *config.Config
	stopCh                       chan struct{}
	stopOnce                     sync.Once
	wg                           sync.WaitGroup
	fallbackLimit                *fallbackLimiter
	lagMu                        sync.Mutex
	lagFailures                  int
	outboxRebuildLatched         bool
	outboxRebuildRunning         bool
	outboxRebuildFailures        int
	outboxRebuildRetryAt         time.Time
	outboxRebuildRetryReason     string
	outboxLagWarningActive       bool
	outboxMaxIDErrorLastLoggedAt time.Time

	fullRebuildRunMu     sync.Mutex
	fullRebuildStateMu   sync.Mutex
	fullRebuildRequested uint64
	fullRebuildCompleted uint64
	fullRebuildLastErr   error
}

func NewSchedulerSnapshotService(
	cache SchedulerCache,
	outboxRepo SchedulerOutboxRepository,
	accountRepo AccountRepository,
	cfg *config.Config,
) *SchedulerSnapshotService {
	maxQPS := 0
	if cfg != nil {
		maxQPS = cfg.Gateway.Scheduling.DbFallbackMaxQPS
	}
	return &SchedulerSnapshotService{
		cache:         cache,
		outboxRepo:    outboxRepo,
		accountRepo:   accountRepo,
		cfg:           cfg,
		stopCh:        make(chan struct{}),
		fallbackLimit: newFallbackLimiter(maxQPS),
	}
}

func (s *SchedulerSnapshotService) Start() {
	if s == nil || s.cache == nil {
		return
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runInitialRebuild()
	}()

	interval := s.outboxPollInterval()
	if s.outboxRepo != nil && interval > 0 {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.runOutboxWorker(interval)
		}()
	}

	fullInterval := s.fullRebuildInterval()
	if fullInterval > 0 {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.runFullRebuildWorker(fullInterval)
		}()
	}
}

func (s *SchedulerSnapshotService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

// ListSchedulableAccounts 返回本次请求在 platform 网关平台上的调度候选。
//
// 桶内容与入站协议无关（第三方 key 进所属分组的每个网关平台桶），这里按请求 context
// 里的入站协议过滤；缓存命中与数据库回源两条路径都要过滤，发布到缓存的仍是未过滤的桶。
func (s *SchedulerSnapshotService) ListSchedulableAccounts(ctx context.Context, platform string) ([]Account, error) {
	bucket := s.bucketForRequest(ctx, platform)
	var writeToken SchedulerBucketWriteToken
	canPublish := false
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if s.cache != nil {
		cached, hit, err := s.cache.GetSnapshot(ctx, bucket)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if err != nil {
			logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] cache read failed: bucket=%s err=%v", bucket.String(), err)
		} else if hit {
			return filterAccountsSchedulableOnPlatform(ctx, derefAccounts(cached), platform), nil
		}
		token, err := s.cache.CaptureBucketWriteToken(ctx, bucket)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if err != nil {
			if errors.Is(err, ErrSchedulerBucketWriteFenced) {
				slog.Debug("[Scheduler] cache publish fenced", "bucket", bucket.String())
			} else {
				logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] cache publish token failed: bucket=%s err=%v", bucket.String(), err)
			}
		} else {
			writeToken = token
			canPublish = true
		}
	}

	if err := s.guardFallback(ctx); err != nil {
		return nil, err
	}

	fallbackCtx, cancel := s.withFallbackTimeout(ctx)
	defer cancel()

	accounts, err := s.loadAccountsFromDB(fallbackCtx, bucket)
	if err != nil {
		return nil, err
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}

	if s.cache != nil && canPublish {
		if err := s.cache.SetSnapshot(fallbackCtx, bucket, writeToken, accounts); err != nil {
			if errors.Is(err, ErrSchedulerBucketWriteFenced) {
				slog.Debug("[Scheduler] cache publish fenced", "bucket", bucket.String())
			} else {
				logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] cache write failed: bucket=%s err=%v", bucket.String(), err)
			}
		}
	}

	return filterAccountsSchedulableOnPlatform(ctx, accounts, platform), nil
}

func (s *SchedulerSnapshotService) GetAccount(ctx context.Context, accountID int64) (*Account, error) {
	if accountID <= 0 {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.cache != nil {
		account, err := s.cache.GetAccount(ctx, accountID)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if err != nil {
			logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] account cache read failed: id=%d err=%v", accountID, err)
		} else if account != nil {
			return account, nil
		}
	}

	if err := s.guardFallback(ctx); err != nil {
		return nil, err
	}
	fallbackCtx, cancel := s.withFallbackTimeout(ctx)
	defer cancel()
	return s.accountRepo.GetByID(fallbackCtx, accountID)
}

// UpdateAccountInCache 立即更新 Redis 中单个账号的数据（用于模型限流后立即生效）
func (s *SchedulerSnapshotService) UpdateAccountInCache(ctx context.Context, account *Account) error {
	if s.cache == nil || account == nil {
		return nil
	}
	return s.cache.SetAccount(ctx, account)
}

func (s *SchedulerSnapshotService) runInitialRebuild() {
	if s.cache == nil {
		return
	}
	_ = s.coalesceFullRebuild(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := s.rebuildFullSnapshot(ctx, "startup"); err != nil {
			logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] rebuild startup failed: %v", err)
			return err
		}
		return nil
	})
}

func (s *SchedulerSnapshotService) runOutboxWorker(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.pollOutbox()
	for {
		select {
		case <-ticker.C:
			s.pollOutbox()
		case <-s.stopCh:
			return
		}
	}
}

func (s *SchedulerSnapshotService) runFullRebuildWorker(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.triggerFullRebuild("interval"); err != nil {
				logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] full rebuild failed: %v", err)
			}
		case <-s.stopCh:
			return
		}
	}
}

func (s *SchedulerSnapshotService) pollOutbox() {
	if s.outboxRepo == nil || s.cache == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	watermark, err := s.cache.GetOutboxWatermark(ctx)
	if err != nil {
		logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] outbox watermark read failed: %v", err)
		return
	}

	events, err := s.outboxRepo.ListAfterAndReleaseDedup(ctx, watermark, 200)
	if err != nil {
		logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] outbox poll failed: %v", err)
		return
	}
	if len(events) == 0 {
		// The outbox query itself proves there is no event after the watermark.
		// Clear degraded/retry state without adding two more repository queries to
		// the healthy one-second poll path.
		s.clearOutboxDegradedEpisode()
		return
	}

	seen := make(map[batchSeenKey]struct{})
	for _, event := range events {
		eventCtx, cancel := context.WithTimeout(context.Background(), outboxEventTimeout)
		err := s.handleOutboxEvent(eventCtx, event, seen)
		cancel()
		if err != nil {
			logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] outbox handle failed: id=%d type=%s err=%v", event.ID, event.EventType, err)
			return
		}
	}

	lastID := events[len(events)-1].ID
	var wmErr error
	for i := range 3 {
		wmCtx, wmCancel := context.WithTimeout(context.Background(), 5*time.Second)
		wmErr = s.cache.SetOutboxWatermark(wmCtx, lastID)
		wmCancel()
		if wmErr == nil {
			break
		}
		if i < 2 {
			time.Sleep(200 * time.Millisecond)
		}
	}
	if wmErr != nil {
		logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] outbox watermark write failed: %v", wmErr)
		return
	}
	s.cleanupConsumedOutbox(lastID)

	// 只有 watermark 成功推进后，当前批次才算已消费。延迟必须按下一条待消费事件计算，
	// 否则本批次处理越慢，越容易误触发一次更慢的全量重建，形成正反馈。
	lagCtx, lagCancel := context.WithTimeout(context.Background(), 5*time.Second)
	s.checkOutboxLag(lagCtx, lastID)
	lagCancel()
}

func (s *SchedulerSnapshotService) cleanupConsumedOutbox(watermark int64) {
	if s == nil || s.outboxRepo == nil || watermark <= 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	lease, acquired, err := s.outboxRepo.TryAcquireCleanupLock(ctx)
	if err != nil {
		logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] outbox cleanup lock failed: %v", err)
		return
	}
	if !acquired {
		return
	}
	defer lease.Release()

	for {
		deleted, err := s.outboxRepo.DeleteConsumedUpTo(ctx, watermark, schedulerOutboxCleanupBatch)
		if err != nil {
			logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] outbox cleanup failed: watermark=%d err=%v", watermark, err)
			return
		}
		if deleted == 0 || deleted < schedulerOutboxCleanupBatch {
			return
		}
	}
}

func (s *SchedulerSnapshotService) handleOutboxEvent(ctx context.Context, event SchedulerOutboxEvent, seen map[batchSeenKey]struct{}) error {
	switch event.EventType {
	case SchedulerOutboxEventAccountLastUsed:
		return s.handleLastUsedEvent(ctx, event.Payload)
	case SchedulerOutboxEventAccountBulkChanged:
		return s.handleBulkAccountEvent(ctx, event.Payload, seen)
	case SchedulerOutboxEventAccountGroupsChanged:
		return s.handleAccountEvent(ctx, event.AccountID, event.Payload, seen)
	case SchedulerOutboxEventAccountChanged:
		return s.handleAccountEvent(ctx, event.AccountID, event.Payload, seen)
	case SchedulerOutboxEventCatalogBindingsChanged:
		return s.handleCatalogBindingsEvent(ctx, event.Payload)
	case SchedulerOutboxEventFullRebuild:
		return s.triggerFullRebuild("outbox")
	default:
		return nil
	}
}

// handleCatalogBindingsEvent 处理目录条目的绑定变更：重建 payload 里每个条目已注册的目录桶。
// 绑定清空或条目已删时重建出的是空快照，而不是退役：退役后只能在分组生命周期租约下
// 重开，目录没有这套生命周期，空快照让请求得到「无可用账号」即可。没注册过的条目
// 什么也不做，首个请求会注册。
func (s *SchedulerSnapshotService) handleCatalogBindingsEvent(ctx context.Context, payload map[string]any) error {
	if s.cache == nil || payload == nil {
		return nil
	}
	entryIDs := parseInt64Slice(payload["entry_ids"])
	if len(entryIDs) == 0 {
		return nil
	}
	buckets, err := s.registeredCatalogBuckets(ctx, entryIDs)
	if err != nil {
		return err
	}
	return s.rebuildBuckets(ctx, buckets, "catalog_bindings_changed")
}

// registeredCatalogBuckets 返回已注册的目录桶里条目 ID 命中 entryIDs 的那些；
// entryIDs 为 nil 时返回全部目录桶。
func (s *SchedulerSnapshotService) registeredCatalogBuckets(ctx context.Context, entryIDs []int64) ([]SchedulerBucket, error) {
	if s.cache == nil {
		return nil, nil
	}
	registered, err := s.cache.ListBuckets(ctx)
	if err != nil {
		return nil, err
	}
	var wanted map[int64]struct{}
	if entryIDs != nil {
		wanted = make(map[int64]struct{}, len(entryIDs))
		for _, id := range entryIDs {
			wanted[id] = struct{}{}
		}
	}
	buckets := make([]SchedulerBucket, 0)
	for _, bucket := range dedupeBuckets(registered) {
		if bucket.Mode != SchedulerModeCatalog {
			continue
		}
		if wanted != nil {
			if _, ok := wanted[bucket.PoolID]; !ok {
				continue
			}
		}
		buckets = append(buckets, bucket)
	}
	return buckets, nil
}

func (s *SchedulerSnapshotService) handleLastUsedEvent(ctx context.Context, payload map[string]any) error {
	if s.cache == nil || payload == nil {
		return nil
	}
	raw, ok := payload["last_used"].(map[string]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	updates := make(map[int64]time.Time, len(raw))
	for key, value := range raw {
		id, err := strconv.ParseInt(key, 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		sec, ok := toInt64(value)
		if !ok || sec <= 0 {
			continue
		}
		updates[id] = time.Unix(sec, 0)
	}
	if len(updates) == 0 {
		return nil
	}
	return s.cache.UpdateLastUsed(ctx, updates)
}

func (s *SchedulerSnapshotService) handleBulkAccountEvent(ctx context.Context, payload map[string]any, seen map[batchSeenKey]struct{}) error {
	if payload == nil {
		return nil
	}
	if s.accountRepo == nil {
		return nil
	}

	rawIDs := parseInt64Slice(payload["account_ids"])
	if len(rawIDs) == 0 {
		return nil
	}

	ids := make([]int64, 0, len(rawIDs))
	seenIDs := make(map[int64]struct{}, len(rawIDs))
	for _, id := range rawIDs {
		if id <= 0 {
			continue
		}
		if _, exists := seenIDs[id]; exists {
			continue
		}
		seenIDs[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil
	}

	accounts, err := s.accountRepo.GetByIDs(ctx, ids)
	if err != nil {
		return err
	}

	found := make(map[int64]struct{}, len(accounts))
	for _, account := range accounts {
		if account == nil || account.ID <= 0 {
			continue
		}
		found[account.ID] = struct{}{}
		if s.cache != nil {
			if err := s.cache.SetAccount(ctx, account); err != nil {
				return err
			}
		}
	}

	allAccountsFound := true
	for _, id := range ids {
		if _, ok := found[id]; ok {
			continue
		}
		allAccountsFound = false
		if s.cache != nil {
			if err := s.cache.DeleteAccount(ctx, id); err != nil {
				return err
			}
		}
	}

	// 缺失账户无法确定原平台，全部平台池都重建以避免遗留旧快照。
	if !allAccountsFound {
		return s.rebuildBuckets(ctx, schedulerCanonicalBuckets(), "account_bulk_change")
	}

	platforms := make(map[string]struct{}, len(accounts))
	for _, account := range accounts {
		if account == nil || account.ID <= 0 || account.Platform == "" {
			continue
		}
		platforms[account.Platform] = struct{}{}
	}
	buckets := make([]SchedulerBucket, 0, len(platforms))
	for _, platform := range schedulerSnapshotPlatforms() {
		if _, ok := platforms[platform]; !ok {
			continue
		}
		buckets = append(buckets, s.bucketsForPlatform(platform, seen)...)
	}
	return s.rebuildBuckets(ctx, buckets, "account_bulk_change")
}

func (s *SchedulerSnapshotService) handleAccountEvent(ctx context.Context, accountID *int64, payload map[string]any, seen map[batchSeenKey]struct{}) error {
	if accountID == nil || *accountID <= 0 {
		return nil
	}
	if s.accountRepo == nil {
		return nil
	}

	account, err := s.accountRepo.GetByID(ctx, *accountID)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			if s.cache != nil {
				if err := s.cache.DeleteAccount(ctx, *accountID); err != nil {
					return err
				}
			}
			// 账号没了就不知道它绑过哪些条目、在哪个平台池，全部目录桶与平台池都重建一遍。
			catalogBuckets, err := s.registeredCatalogBuckets(ctx, nil)
			if err != nil {
				return err
			}
			if err := s.rebuildBuckets(ctx, catalogBuckets, "account_miss"); err != nil {
				return err
			}
			return s.rebuildBuckets(ctx, schedulerCanonicalBuckets(), "account_miss")
		}
		return err
	}
	if s.cache != nil {
		if err := s.cache.SetAccount(ctx, account); err != nil {
			return err
		}
	}
	if len(account.CatalogEntryIDs) > 0 {
		catalogBuckets, err := s.registeredCatalogBuckets(ctx, account.CatalogEntryIDs)
		if err != nil {
			return err
		}
		if err := s.rebuildBuckets(ctx, catalogBuckets, "account_change"); err != nil {
			return err
		}
	}
	return s.rebuildByAccount(ctx, account, "account_change", seen)
}

// rebuildByAccount 重建账号所在的平台池桶：桶只装平台相等的账号（key 与成品号同一条规则），
// 所以只有它自己标签的那一个桶要重建。目录桶由调用方按 CatalogEntryIDs 另行重建。
func (s *SchedulerSnapshotService) rebuildByAccount(ctx context.Context, account *Account, reason string, seen map[batchSeenKey]struct{}) error {
	if account == nil {
		return nil
	}
	return s.rebuildBuckets(ctx, s.bucketsForPlatform(account.Platform, seen), reason)
}

func schedulerSnapshotPlatforms() [10]string {
	return [10]string{PlatformAnthropic, PlatformGemini, PlatformOpenAI, PlatformAntigravity, PlatformGrok, PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax, PlatformOpenCodeGo}
}

// schedulerCanonicalBuckets 常驻桶：每个平台一个平台池（PoolID 0）。目录桶按条目动态注册，不在此列。
func schedulerCanonicalBuckets() []SchedulerBucket {
	platforms := schedulerSnapshotPlatforms()
	buckets := make([]SchedulerBucket, 0, len(platforms))
	for _, platform := range platforms {
		buckets = append(buckets, SchedulerBucket{Platform: platform, Mode: SchedulerModeSingle})
	}
	return buckets
}

// bucketsForPlatform 平台池桶；同一批 outbox 事件里同一平台只重建一次
// （第一次重建就已按最新 DB 数据装载该平台的全部账号）。
func (s *SchedulerSnapshotService) bucketsForPlatform(platform string, seen map[batchSeenKey]struct{}) []SchedulerBucket {
	if platform == "" {
		return nil
	}
	if seen != nil {
		key := batchSeenKey{platform: platform}
		if _, exists := seen[key]; exists {
			return nil
		}
		seen[key] = struct{}{}
	}
	return []SchedulerBucket{{Platform: platform, Mode: SchedulerModeSingle}}
}

func (s *SchedulerSnapshotService) rebuildBuckets(ctx context.Context, buckets []SchedulerBucket, reason string) error {
	tasks, firstErr := s.prepareBucketWriteTasks(ctx, buckets)
	if err := s.rebuildPreparedBucketTasks(ctx, tasks, reason, false); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (s *SchedulerSnapshotService) prepareBucketWriteTasks(ctx context.Context, buckets []SchedulerBucket) ([]schedulerBucketWriteTask, error) {
	if s.cache == nil {
		return nil, ErrSchedulerCacheNotReady
	}
	tasks := make([]schedulerBucketWriteTask, 0, len(buckets))
	var firstErr error
	for _, bucket := range buckets {
		token, err := s.cache.CaptureBucketWriteToken(ctx, bucket)
		if err != nil {
			if errors.Is(err, ErrSchedulerBucketWriteFenced) {
				continue
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		tasks = append(tasks, schedulerBucketWriteTask{bucket: bucket, token: token})
	}
	return tasks, firstErr
}

func (s *SchedulerSnapshotService) rebuildPreparedBucketTasks(
	ctx context.Context,
	tasks []schedulerBucketWriteTask,
	reason string,
	strict bool,
) error {
	var firstErr error
	for _, task := range tasks {
		if err := s.rebuildBucketWithTokenPolicy(ctx, task, reason, strict); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *SchedulerSnapshotService) rebuildBucketWithTokenPolicy(
	ctx context.Context,
	task schedulerBucketWriteTask,
	reason string,
	strict bool,
) error {
	if s.cache == nil {
		return ErrSchedulerCacheNotReady
	}
	bucket := task.bucket
	ok, err := s.cache.TryLockBucket(ctx, bucket, 30*time.Second)
	if err != nil {
		return err
	}
	if !ok {
		if strict {
			return fmt.Errorf("%w: bucket=%s", ErrSchedulerBucketRebuildBusy, bucket.String())
		}
		return nil
	}
	defer func() {
		_ = s.cache.UnlockBucket(ctx, bucket)
	}()

	rebuildCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	accounts, err := s.loadAccountsFromDB(rebuildCtx, bucket)
	if err != nil {
		logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] rebuild failed: bucket=%s reason=%s err=%v", bucket.String(), reason, err)
		return err
	}
	if err := s.cache.SetSnapshot(rebuildCtx, task.bucket, task.token, accounts); err != nil {
		if errors.Is(err, ErrSchedulerBucketWriteFenced) {
			slog.Debug("[Scheduler] rebuild fenced", "bucket", bucket.String(), "reason", reason)
			if strict {
				return err
			}
			return nil
		}
		logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] rebuild cache failed: bucket=%s reason=%s err=%v", bucket.String(), reason, err)
		return err
	}
	slog.Debug("[Scheduler] rebuild ok", "bucket", bucket.String(), "reason", reason, "size", len(accounts))
	return nil
}

func (s *SchedulerSnapshotService) triggerFullRebuild(reason string) error {
	if s.cache == nil {
		return ErrSchedulerCacheNotReady
	}
	return s.coalesceFullRebuild(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		return s.rebuildFullSnapshot(ctx, reason)
	})
}

// rebuildFullSnapshot 重建常驻平台池与注册表里的目录桶。注册表里 ParseSchedulerBucket 不认的
// 成员（旧的 mixed / forced / 分组桶）已在读取时被丢弃，不再重建。
func (s *SchedulerSnapshotService) rebuildFullSnapshot(ctx context.Context, reason string) error {
	if s.cache == nil {
		return ErrSchedulerCacheNotReady
	}

	registered, err := s.cache.ListBuckets(ctx)
	if err != nil {
		return err
	}
	registered = dedupeBuckets(registered)

	canonical := schedulerCanonicalBuckets()
	captured, err := s.captureFullRebuildCanonicalTasks(ctx, canonical)
	if err != nil {
		return err
	}
	ordinary := appendBucketsExcept(nil, registered, canonical)
	return s.prepareAndRebuildFullSnapshot(ctx, captured, ordinary, reason)
}

func (s *SchedulerSnapshotService) prepareAndRebuildFullSnapshot(
	ctx context.Context,
	captured []schedulerBucketWriteTask,
	ordinaryBuckets []SchedulerBucket,
	reason string,
) error {
	// 首个 DB 查询前必须完成全部普通 bucket 的 token 预备；任何预备错误都不会留下部分发布。
	preparedBuckets := make(map[SchedulerBucket]struct{}, len(captured))
	for _, task := range captured {
		preparedBuckets[task.bucket] = struct{}{}
	}

	ordinaryBuckets = dedupeBuckets(ordinaryBuckets)
	toCapture := make([]SchedulerBucket, 0, len(ordinaryBuckets))
	for _, bucket := range ordinaryBuckets {
		if _, ok := preparedBuckets[bucket]; !ok {
			toCapture = append(toCapture, bucket)
		}
	}
	ordinary, firstErr := s.prepareBucketWriteTasks(ctx, toCapture)
	if firstErr != nil {
		return firstErr
	}
	captured = append(captured, ordinary...)
	if err := s.rebuildPreparedBucketTasks(ctx, captured, reason, false); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (s *SchedulerSnapshotService) captureFullRebuildCanonicalTasks(ctx context.Context, buckets []SchedulerBucket) ([]schedulerBucketWriteTask, error) {
	if s.cache == nil {
		return nil, ErrSchedulerCacheNotReady
	}
	tasks := make([]schedulerBucketWriteTask, 0, len(buckets))
	for _, bucket := range buckets {
		token, err := s.cache.CaptureBucketWriteToken(ctx, bucket)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, schedulerBucketWriteTask{bucket: bucket, token: token})
	}
	return tasks, nil
}

func appendBucketsExcept(dst, buckets, excluded []SchedulerBucket) []SchedulerBucket {
	excludedKeys := make(map[SchedulerBucket]struct{}, len(excluded))
	for _, bucket := range excluded {
		excludedKeys[bucket] = struct{}{}
	}
	for _, bucket := range buckets {
		if _, ok := excludedKeys[bucket]; !ok {
			dst = append(dst, bucket)
		}
	}
	return dst
}

func (s *SchedulerSnapshotService) coalesceFullRebuild(run func() error) error {
	s.fullRebuildStateMu.Lock()
	s.fullRebuildRequested++
	requestID := s.fullRebuildRequested
	s.fullRebuildStateMu.Unlock()

	s.fullRebuildRunMu.Lock()
	defer s.fullRebuildRunMu.Unlock()

	s.fullRebuildStateMu.Lock()
	if s.fullRebuildCompleted >= requestID {
		err := s.fullRebuildLastErr
		s.fullRebuildStateMu.Unlock()
		return err
	}
	// 当前轮重建可能早于新 outbox 事件对应事务的提交，不能让后到请求直接复用当前轮。
	// 每轮开始前记录可覆盖的请求代次，执行期间登记的请求统一合并到下一轮。
	coveredThrough := s.fullRebuildRequested
	s.fullRebuildStateMu.Unlock()

	err := run()

	s.fullRebuildStateMu.Lock()
	s.fullRebuildCompleted = coveredThrough
	s.fullRebuildLastErr = err
	s.fullRebuildStateMu.Unlock()
	return err
}

func (s *SchedulerSnapshotService) checkOutboxLag(ctx context.Context, watermark int64) {
	if s.cfg == nil || s.outboxRepo == nil {
		return
	}
	now := time.Now()
	oldestCreatedAt, ok, err := s.outboxRepo.FirstCreatedAtAfter(ctx, watermark)
	if err != nil {
		logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] outbox pending event read failed: %v", err)
		return
	}
	var lag time.Duration
	if ok && !oldestCreatedAt.IsZero() {
		lag = now.Sub(oldestCreatedAt)
	}
	lagSeconds := int(lag.Seconds())
	lagWarning := ok && !oldestCreatedAt.IsZero() &&
		s.cfg.Gateway.Scheduling.OutboxLagWarnSeconds > 0 &&
		lagSeconds >= s.cfg.Gateway.Scheduling.OutboxLagWarnSeconds

	lagDegraded := ok && !oldestCreatedAt.IsZero() &&
		s.cfg.Gateway.Scheduling.OutboxLagRebuildSeconds > 0 &&
		lagSeconds >= s.cfg.Gateway.Scheduling.OutboxLagRebuildSeconds

	backlogThreshold := s.cfg.Gateway.Scheduling.OutboxBacklogRebuildRows
	backlogKnown := true
	var backlog int64
	if backlogThreshold > 0 {
		maxID, maxErr := s.outboxRepo.MaxID(ctx)
		if maxErr != nil {
			backlogKnown = false
			if s.shouldLogOutboxMaxIDError(now) {
				logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] outbox max id read failed: %v", maxErr)
			}
		} else {
			backlog = maxID - watermark
		}
	}
	backlogDegraded := backlogKnown && backlogThreshold > 0 && backlog >= int64(backlogThreshold)

	// A successful rebuild latches the degraded episode until recovery. A failed
	// rebuild remains retryable, but only after an exponentially backed-off
	// cooldown so a one-second poll cannot create a rebuild storm.
	logLagWarning := s.shouldLogOutboxLagWarning(lagWarning)
	s.lagMu.Lock()
	fullyRecovered := !lagDegraded && backlogKnown && !backlogDegraded
	if fullyRecovered {
		s.lagFailures = 0
		s.outboxRebuildLatched = false
		s.outboxRebuildFailures = 0
		s.outboxRebuildRetryAt = time.Time{}
		s.outboxRebuildRetryReason = ""
	}

	if s.outboxRebuildRetryReason != "" {
		retryReasonActive := (s.outboxRebuildRetryReason == "outbox_lag" && lagDegraded) ||
			(s.outboxRebuildRetryReason == "outbox_backlog" && (!backlogKnown || backlogDegraded))
		if !retryReasonActive {
			s.outboxRebuildFailures = 0
			s.outboxRebuildRetryAt = time.Time{}
			s.outboxRebuildRetryReason = ""
		}
	}

	lagRetryPending := s.outboxRebuildRetryReason == "outbox_lag" && !s.outboxRebuildRetryAt.IsZero()
	if lagDegraded {
		if !s.outboxRebuildLatched && !s.outboxRebuildRunning && !lagRetryPending {
			s.lagFailures++
		}
	} else {
		s.lagFailures = 0
	}
	failures := s.lagFailures
	lagReady := lagDegraded && failures >= s.cfg.Gateway.Scheduling.OutboxLagRebuildFailures
	retryDue := s.outboxRebuildRetryReason != "" &&
		!s.outboxRebuildRetryAt.IsZero() && !now.Before(s.outboxRebuildRetryAt)

	reason := ""
	lagCanPreemptRetry := lagReady && s.outboxRebuildRetryReason != "outbox_lag"
	if !s.outboxRebuildLatched && !s.outboxRebuildRunning &&
		(s.outboxRebuildRetryAt.IsZero() || retryDue || lagCanPreemptRetry) {
		switch {
		case lagReady || (retryDue && s.outboxRebuildRetryReason == "outbox_lag" && lagDegraded):
			if s.outboxRebuildRetryReason != "" && s.outboxRebuildRetryReason != "outbox_lag" {
				s.outboxRebuildFailures = 0
				s.outboxRebuildRetryAt = time.Time{}
				s.outboxRebuildRetryReason = ""
			}
			reason = "outbox_lag"
			s.lagFailures = 0
		case backlogDegraded && (s.outboxRebuildRetryReason == "" ||
			(retryDue && s.outboxRebuildRetryReason == "outbox_backlog")):
			reason = "outbox_backlog"
		}
		if reason != "" {
			s.outboxRebuildRunning = true
		}
	}
	s.lagMu.Unlock()

	if logLagWarning {
		logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] outbox lag warning: %ds", lagSeconds)
	}

	if reason == "" {
		return
	}

	var rebuildErr error
	switch reason {
	case "outbox_lag":
		logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] outbox lag rebuild triggered: lag=%s failures=%d", lag, failures)
		rebuildErr = s.triggerFullRebuild(reason)
	case "outbox_backlog":
		logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] outbox backlog rebuild triggered: backlog=%d", backlog)
		rebuildErr = s.triggerFullRebuild(reason)
	}

	s.lagMu.Lock()
	s.outboxRebuildRunning = false
	if rebuildErr == nil {
		s.outboxRebuildLatched = true
		s.outboxRebuildFailures = 0
		s.outboxRebuildRetryAt = time.Time{}
		s.outboxRebuildRetryReason = ""
	} else {
		s.outboxRebuildLatched = false
		s.outboxRebuildFailures++
		s.outboxRebuildRetryAt = time.Now().Add(outboxRebuildRetryDelay(s.outboxRebuildFailures))
		s.outboxRebuildRetryReason = reason
	}
	s.lagMu.Unlock()

	if rebuildErr != nil {
		logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] %s rebuild failed: %v", reason, rebuildErr)
	}
}

func outboxRebuildRetryDelay(failures int) time.Duration {
	delay := outboxRebuildRetryBaseDelay
	for i := 1; i < failures && delay < outboxRebuildRetryMaxDelay; i++ {
		delay *= 2
		if delay >= outboxRebuildRetryMaxDelay {
			return outboxRebuildRetryMaxDelay
		}
	}
	return delay
}

func (s *SchedulerSnapshotService) clearOutboxDegradedEpisode() {
	if s == nil {
		return
	}
	s.lagMu.Lock()
	if s.lagFailures != 0 || s.outboxRebuildLatched || s.outboxRebuildRunning ||
		s.outboxRebuildFailures != 0 || !s.outboxRebuildRetryAt.IsZero() ||
		s.outboxRebuildRetryReason != "" || s.outboxLagWarningActive {
		s.lagFailures = 0
		s.outboxRebuildLatched = false
		s.outboxRebuildFailures = 0
		s.outboxRebuildRetryAt = time.Time{}
		s.outboxRebuildRetryReason = ""
		s.outboxLagWarningActive = false
	}
	s.lagMu.Unlock()
}

func (s *SchedulerSnapshotService) shouldLogOutboxMaxIDError(now time.Time) bool {
	s.lagMu.Lock()
	defer s.lagMu.Unlock()
	if !s.outboxMaxIDErrorLastLoggedAt.IsZero() && now.Sub(s.outboxMaxIDErrorLastLoggedAt) < outboxMaxIDErrorLogSampleInterval {
		return false
	}
	s.outboxMaxIDErrorLastLoggedAt = now
	return true
}

func (s *SchedulerSnapshotService) shouldLogOutboxLagWarning(active bool) bool {
	s.lagMu.Lock()
	defer s.lagMu.Unlock()
	shouldLog := active && !s.outboxLagWarningActive
	s.outboxLagWarningActive = active
	return shouldLog
}

// loadAccountsFromDB 装载一个桶的候选：目录桶 = 条目绑定的账号，平台池 = 该平台的全部可调度资源。
func (s *SchedulerSnapshotService) loadAccountsFromDB(ctx context.Context, bucket SchedulerBucket) ([]Account, error) {
	if s.accountRepo == nil {
		return nil, ErrSchedulerCacheNotReady
	}
	if bucket.Mode == SchedulerModeCatalog {
		return s.accountRepo.ListSchedulingCandidatesByCatalogEntry(ctx, bucket.PoolID)
	}
	accounts, err := s.accountRepo.ListSchedulingCandidates(ctx, []string{bucket.Platform})
	if err != nil {
		return nil, err
	}
	return filterSchedulingBucketAccounts(accounts, bucket.Platform), nil
}

// bucketForRequest 目录路由用目录桶（条目 ID，Platform 恒空）；否则是 platform 的平台池。
// 目录桶的候选之后由 filterAccountsSchedulableOnPlatform 按生效平台与入站协议过滤。
func (s *SchedulerSnapshotService) bucketForRequest(ctx context.Context, platform string) SchedulerBucket {
	if route, ok := CatalogRouteFromContext(ctx); ok {
		return SchedulerBucket{PoolID: route.EntryID, Mode: SchedulerModeCatalog}
	}
	return SchedulerBucket{Platform: platform, Mode: SchedulerModeSingle}
}

func (s *SchedulerSnapshotService) guardFallback(ctx context.Context) error {
	if s.cfg == nil || s.cfg.Gateway.Scheduling.DbFallbackEnabled {
		if s.fallbackLimit == nil || s.fallbackLimit.Allow() {
			return nil
		}
		return ErrSchedulerFallbackLimited
	}
	return ErrSchedulerCacheNotReady
}

func (s *SchedulerSnapshotService) withFallbackTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if s.cfg == nil || s.cfg.Gateway.Scheduling.DbFallbackTimeoutSeconds <= 0 {
		return context.WithCancel(ctx)
	}
	timeout := time.Duration(s.cfg.Gateway.Scheduling.DbFallbackTimeoutSeconds) * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return context.WithCancel(ctx)
		}
		if remaining < timeout {
			timeout = remaining
		}
	}
	return context.WithTimeout(ctx, timeout)
}

func (s *SchedulerSnapshotService) outboxPollInterval() time.Duration {
	if s.cfg == nil {
		return time.Second
	}
	sec := s.cfg.Gateway.Scheduling.OutboxPollIntervalSeconds
	if sec <= 0 {
		return time.Second
	}
	return time.Duration(sec) * time.Second
}

func (s *SchedulerSnapshotService) fullRebuildInterval() time.Duration {
	if s.cfg == nil {
		return 0
	}
	sec := s.cfg.Gateway.Scheduling.FullRebuildIntervalSeconds
	if sec <= 0 {
		return 0
	}
	return time.Duration(sec) * time.Second
}

func dedupeBuckets(in []SchedulerBucket) []SchedulerBucket {
	seen := make(map[string]struct{}, len(in))
	out := make([]SchedulerBucket, 0, len(in))
	for _, bucket := range in {
		key := bucket.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, bucket)
	}
	return out
}

func derefAccounts(accounts []*Account) []Account {
	if len(accounts) == 0 {
		return []Account{}
	}
	out := make([]Account, 0, len(accounts))
	for _, account := range accounts {
		if account == nil {
			continue
		}
		out = append(out, *account)
	}
	return out
}

func parseInt64Slice(value any) []int64 {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]int64, 0, len(raw))
	for _, item := range raw {
		if v, ok := toInt64(item); ok && v > 0 {
			out = append(out, v)
		}
	}
	return out
}

func toInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case int:
		return int64(v), true
	case json.Number:
		parsed, err := strconv.ParseInt(v.String(), 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

type fallbackLimiter struct {
	maxQPS int
	mu     sync.Mutex
	window time.Time
	count  int
}

func newFallbackLimiter(maxQPS int) *fallbackLimiter {
	if maxQPS <= 0 {
		return nil
	}
	return &fallbackLimiter{
		maxQPS: maxQPS,
		window: time.Now(),
	}
}

func (l *fallbackLimiter) Allow() bool {
	if l == nil || l.maxQPS <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	if now.Sub(l.window) >= time.Second {
		l.window = now
		l.count = 0
	}
	if l.count >= l.maxQPS {
		return false
	}
	l.count++
	return true
}
