package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/dgraph-io/ristretto"
	"golang.org/x/sync/singleflight"
)

// MaxExpiresAt is the maximum allowed expiration date (year 2099)
// This prevents time.Time JSON serialization errors (RFC 3339 requires year <= 9999)
var MaxExpiresAt = time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC)

// MaxValidityDays is the maximum allowed validity days for subscriptions (100 years)
const MaxValidityDays = 36500

var (
	ErrSubscriptionNotFound        = infraerrors.NotFound("SUBSCRIPTION_NOT_FOUND", "subscription not found")
	ErrSubscriptionExpired         = infraerrors.Forbidden("SUBSCRIPTION_EXPIRED", "subscription has expired")
	ErrSubscriptionSuspended       = infraerrors.Forbidden("SUBSCRIPTION_SUSPENDED", "subscription is suspended")
	ErrSubscriptionAlreadyExists   = infraerrors.Conflict("SUBSCRIPTION_ALREADY_EXISTS", "subscription already exists for this user and plan")
	ErrSubscriptionAlreadyActive   = infraerrors.Conflict("SUBSCRIPTION_ALREADY_ACTIVE", "user already has an active subscription for another plan")
	ErrSubscriptionAssignConflict  = infraerrors.Conflict("SUBSCRIPTION_ASSIGN_CONFLICT", "subscription exists but request conflicts with existing assignment semantics")
	ErrSubscriptionNotRevoked      = infraerrors.Conflict("SUBSCRIPTION_NOT_REVOKED", "subscription is not revoked")
	ErrSubscriptionRestoreConflict = infraerrors.Conflict("SUBSCRIPTION_RESTORE_CONFLICT", "subscription already exists for this user and plan")
	ErrInvalidInput                = infraerrors.BadRequest("INVALID_INPUT", "at least one of resetDaily, resetWeekly, or resetMonthly must be true")
	ErrDailyLimitExceeded          = infraerrors.TooManyRequests("DAILY_LIMIT_EXCEEDED", "daily usage limit exceeded")
	ErrWeeklyLimitExceeded         = infraerrors.TooManyRequests("WEEKLY_LIMIT_EXCEEDED", "weekly usage limit exceeded")
	ErrMonthlyLimitExceeded        = infraerrors.TooManyRequests("MONTHLY_LIMIT_EXCEEDED", "monthly usage limit exceeded")
	ErrSubscriptionNilInput        = infraerrors.BadRequest("SUBSCRIPTION_NIL_INPUT", "subscription input cannot be nil")
	ErrAdjustWouldExpire           = infraerrors.BadRequest("ADJUST_WOULD_EXPIRE", "adjustment would result in expired subscription (remaining days must be > 0)")
)

// SubscriptionKeyRepository 是订阅服务生成订阅 key 需要的最小仓储面（APIKeyRepository 隐式满足）。
type SubscriptionKeyRepository interface {
	Create(ctx context.Context, key *APIKey) error
	ExistsBySubscriptionID(ctx context.Context, subscriptionID int64) (bool, error)
}

// SubscriptionService 订阅服务
type SubscriptionService struct {
	planRepo            SubscriptionPlanRepository
	userSubRepo         UserSubscriptionRepository
	apiKeyRepo          SubscriptionKeyRepository
	billingCacheService *BillingCacheService
	entClient           *dbent.Client
	keyPrefix           string // 订阅 key 前缀（cfg.Default.APIKeyPrefix，空则 sk-）

	// L1 缓存：加速中间件热路径的订阅 / 套餐查询（同一实例，键前缀 sub: / plan: 区分）
	subCacheL1     *ristretto.Cache
	subCacheGroup  singleflight.Group
	subCacheTTL    time.Duration
	subCacheJitter int // 抖动百分比

	maintenanceQueue *SubscriptionMaintenanceQueue
	now              func() time.Time
}

// NewSubscriptionService 创建订阅服务
func NewSubscriptionService(planRepo SubscriptionPlanRepository, userSubRepo UserSubscriptionRepository, apiKeyRepo SubscriptionKeyRepository, billingCacheService *BillingCacheService, entClient *dbent.Client, cfg *config.Config) *SubscriptionService {
	svc := &SubscriptionService{
		planRepo:            planRepo,
		userSubRepo:         userSubRepo,
		apiKeyRepo:          apiKeyRepo,
		billingCacheService: billingCacheService,
		entClient:           entClient,
		keyPrefix:           "sk-",
		now:                 time.Now,
	}
	if cfg != nil && cfg.Default.APIKeyPrefix != "" {
		svc.keyPrefix = cfg.Default.APIKeyPrefix
	}
	svc.initSubCache(cfg)
	svc.initMaintenanceQueue(cfg)
	svc.StartSubCacheInvalidationSubscriber(context.Background())
	return svc
}

func (s *SubscriptionService) initMaintenanceQueue(cfg *config.Config) {
	if cfg == nil {
		return
	}
	mc := cfg.SubscriptionMaintenance
	if mc.WorkerCount <= 0 || mc.QueueSize <= 0 {
		return
	}
	s.maintenanceQueue = NewSubscriptionMaintenanceQueue(mc.WorkerCount, mc.QueueSize)
}

// Stop stops the maintenance worker pool.
func (s *SubscriptionService) Stop() {
	if s == nil {
		return
	}
	if s.maintenanceQueue != nil {
		s.maintenanceQueue.Stop()
	}
}

// initSubCache 初始化订阅 L1 缓存
func (s *SubscriptionService) initSubCache(cfg *config.Config) {
	if cfg == nil {
		return
	}
	sc := cfg.SubscriptionCache
	if sc.L1Size <= 0 || sc.L1TTLSeconds <= 0 {
		return
	}
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: int64(sc.L1Size) * 10,
		MaxCost:     int64(sc.L1Size),
		BufferItems: 64,
	})
	if err != nil {
		log.Printf("Warning: failed to init subscription L1 cache: %v", err)
		return
	}
	s.subCacheL1 = cache
	s.subCacheTTL = time.Duration(sc.L1TTLSeconds) * time.Second
	s.subCacheJitter = sc.JitterPercent
}

// subCacheKey 订阅 L1 键（按订阅 ID：订阅 key 鉴权时 ID 已知；热路径避免 fmt.Sprintf）
func subCacheKey(subscriptionID int64) string {
	return "sub:" + strconv.FormatInt(subscriptionID, 10)
}

// planCacheKey 套餐 L1 键（限额 + 模型集）；改套餐只失效这一个键
func planCacheKey(planID int64) string {
	return "plan:" + strconv.FormatInt(planID, 10)
}

// jitteredTTL 为 TTL 添加抖动，避免集中过期
func (s *SubscriptionService) jitteredTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 || s.subCacheJitter <= 0 {
		return ttl
	}
	pct := s.subCacheJitter
	if pct > 100 {
		pct = 100
	}
	delta := float64(pct) / 100
	factor := 1 - delta + rand.Float64()*(2*delta)
	if factor <= 0 {
		return ttl
	}
	return time.Duration(float64(ttl) * factor)
}

// InvalidateSubCache 失效一条订阅的 L1 缓存
func (s *SubscriptionService) InvalidateSubCache(subscriptionID int64) {
	if s.subCacheL1 == nil {
		return
	}
	s.subCacheL1.Del(subCacheKey(subscriptionID))
}

// InvalidateSubCacheSync 失效订阅 L1 缓存并等待 Ristretto 删除操作生效。
func (s *SubscriptionService) InvalidateSubCacheSync(subscriptionID int64) {
	s.invalidateSubCacheKeySync(subCacheKey(subscriptionID))
}

// InvalidatePlanCache 套餐被改 / 删后失效本实例与其它实例的套餐 L1。
func (s *SubscriptionService) InvalidatePlanCache(ctx context.Context, planID int64) error {
	s.invalidateSubCacheKeySync(planCacheKey(planID))
	if s.billingCacheService == nil {
		return nil
	}
	if err := s.billingCacheService.PublishSubscriptionCacheInvalidation(ctx, planCacheKey(planID)); err != nil {
		return fmt.Errorf("publish plan cache invalidation: %w", err)
	}
	return nil
}

func (s *SubscriptionService) invalidateSubCacheKeySync(key string) {
	if s.subCacheL1 == nil {
		return
	}
	s.subCacheL1.Del(key)
	s.subCacheL1.Wait()
}

// StartSubCacheInvalidationSubscriber 启动跨实例订阅 L1 缓存失效订阅。
func (s *SubscriptionService) StartSubCacheInvalidationSubscriber(ctx context.Context) {
	if s.billingCacheService == nil || s.subCacheL1 == nil {
		return
	}
	if err := s.billingCacheService.SubscribeSubscriptionCacheInvalidation(ctx, func(cacheKey string) {
		s.invalidateSubCacheKeySync(cacheKey)
	}); err != nil {
		log.Printf("Warning: failed to start subscription cache invalidation subscriber: %v", err)
	}
}

// invalidateSubscriptionCaches 订阅变更后的全部失效：本实例 L1、Redis 计费缓存、跨实例广播。
// 所有写路径收口到这里。
func (s *SubscriptionService) invalidateSubscriptionCaches(sub *UserSubscription) error {
	s.InvalidateSubCacheSync(sub.ID)
	if s.billingCacheService == nil {
		return nil
	}

	cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.billingCacheService.InvalidateSubscription(cacheCtx, sub.UserID, sub.PlanID); err != nil {
		return fmt.Errorf("invalidate billing subscription cache: %w", err)
	}
	if err := s.billingCacheService.PublishSubscriptionCacheInvalidation(cacheCtx, subCacheKey(sub.ID)); err != nil {
		return fmt.Errorf("publish subscription cache invalidation: %w", err)
	}
	return nil
}

// invalidateSubscriptionCachesAsync 请求路径 / 分配路径用：L1 同步删，Redis 异步删，不广播。
func (s *SubscriptionService) invalidateSubscriptionCachesAsync(sub *UserSubscription) {
	s.InvalidateSubCache(sub.ID)
	if s.billingCacheService == nil {
		return
	}
	userID, planID := sub.UserID, sub.PlanID
	go func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.billingCacheService.InvalidateSubscription(cacheCtx, userID, planID)
	}()
}

// AssignSubscriptionInput 分配订阅输入
type AssignSubscriptionInput struct {
	UserID       int64
	PlanID       int64
	ValidityDays int
	AssignedBy   int64
	Notes        string
}

// AssignSubscription 分配订阅给用户（不允许重复分配）
func (s *SubscriptionService) AssignSubscription(ctx context.Context, input *AssignSubscriptionInput) (*UserSubscription, error) {
	sub, _, err := s.assignSubscriptionWithReuse(ctx, input)
	if err != nil {
		return nil, err
	}
	return sub, nil
}

// AssignOrExtendSubscription 分配或续期订阅（用于兑换码等场景）
// 如果用户已有同套餐的订阅：
//   - 未过期：从当前过期时间累加天数
//   - 已过期：从当前时间开始计算新的过期时间，并激活订阅
//
// 如果没有订阅：创建新订阅
func (s *SubscriptionService) AssignOrExtendSubscription(ctx context.Context, input *AssignSubscriptionInput) (*UserSubscription, bool, error) {
	return s.assignOrExtendSubscription(ctx, input, false)
}

func (s *SubscriptionService) assignOrExtendSubscription(ctx context.Context, input *AssignSubscriptionInput, deferCacheInvalidation bool) (*UserSubscription, bool, error) {
	plan, err := s.planRepo.GetByID(ctx, input.PlanID)
	if err != nil {
		return nil, false, err
	}
	if err := s.rejectOtherActivePlan(ctx, input.UserID, input.PlanID); err != nil {
		return nil, false, err
	}

	// 查询是否已有同套餐订阅（未软删，可能已过期）
	existingSub, err := s.userSubRepo.GetByUserIDAndPlanID(ctx, input.UserID, input.PlanID)
	if err != nil {
		// 不存在记录是正常情况，其他错误需要返回
		existingSub = nil
	}

	validityDays := input.ValidityDays
	if validityDays <= 0 {
		validityDays = 30
	}
	if validityDays > MaxValidityDays {
		validityDays = MaxValidityDays
	}

	// 已有订阅，执行续期（在事务中完成所有更新）
	if existingSub != nil {
		if err := s.updateExistingSubscriptionTerm(ctx, existingSub.ID, plan.Name, validityDays, input.Notes, false); err != nil {
			return nil, false, err
		}

		// 失效订阅缓存
		s.maybeInvalidateAssignmentCaches(existingSub, deferCacheInvalidation)

		// 返回更新后的订阅
		sub, err := s.userSubRepo.GetByID(ctx, existingSub.ID)
		return sub, true, err // true 表示是续期
	}

	// 没有订阅，创建新订阅
	sub, err := s.createSubscription(ctx, input, plan)
	if err != nil {
		return nil, false, err
	}

	// 失效订阅缓存
	s.maybeInvalidateAssignmentCaches(sub, deferCacheInvalidation)

	return sub, false, nil // false 表示是新建
}

// rejectOtherActivePlan 同一用户同一时间只允许一条有效订阅：已有别的套餐有效 → ErrSubscriptionAlreadyActive。
// 同套餐走续期。有效 = status active 且未过期（status 由过期批处理延迟翻转，所以看 expires_at）。
func (s *SubscriptionService) rejectOtherActivePlan(ctx context.Context, userID, planID int64) error {
	// ListActiveByUserID 只回 status=active 且未过期的行（含过期批处理还没翻状态的不算）
	active, err := s.userSubRepo.ListActiveByUserID(ctx, userID)
	if err != nil {
		return err
	}
	for i := range active {
		if active[i].PlanID != planID {
			return ErrSubscriptionAlreadyActive
		}
	}
	return nil
}

func (s *SubscriptionService) maybeInvalidateAssignmentCaches(sub *UserSubscription, deferred bool) {
	// Payment fulfillment owns an outer transaction and performs a synchronous
	// invalidation after commit. Invalidating inside that transaction can reload
	// the pre-commit subscription into cache.
	if deferred {
		return
	}
	s.invalidateSubscriptionCachesAsync(sub)
}

// ensureSubscriptionKey 同一订阅行至多一把未删除的订阅 key；新建 / 续期 / 恢复时补齐（管理端删过就再生成）。
// 必须在持有订阅行锁的事务里调用（updateExistingSubscriptionTerm / createSubscription 的 tx），并发靠行锁串行。
func (s *SubscriptionService) ensureSubscriptionKey(ctx context.Context, sub *UserSubscription, planName string) error {
	exists, err := s.apiKeyRepo.ExistsBySubscriptionID(ctx, sub.ID)
	if err != nil {
		return fmt.Errorf("check subscription key: %w", err)
	}
	if exists {
		return nil
	}
	key, err := generateAPIKey(s.keyPrefix)
	if err != nil {
		return err
	}
	now := s.now()
	subscriptionID := sub.ID
	if err := s.apiKeyRepo.Create(ctx, &APIKey{
		UserID:         sub.UserID,
		Key:            key,
		Name:           planName,
		Status:         StatusActive,
		SubscriptionID: &subscriptionID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil {
		return fmt.Errorf("create subscription key: %w", err)
	}
	return nil
}

func (s *SubscriptionService) updateExistingSubscriptionTerm(
	ctx context.Context,
	subscriptionID int64,
	planName string,
	validityDays int,
	notes string,
	assignmentSemantics bool,
) error {
	return s.withSubscriptionUpdateTx(ctx, func(txCtx context.Context) error {
		existingSub, err := s.userSubRepo.GetByIDForUpdate(txCtx, subscriptionID)
		if err != nil {
			return fmt.Errorf("lock subscription for renewal: %w", err)
		}
		if err := s.ensureSubscriptionKey(txCtx, existingSub, planName); err != nil {
			return err
		}
		if assignmentSemantics && existingSub.Status == SubscriptionStatusSuspended {
			return nil
		}

		now := time.Now()
		if s.now != nil {
			now = s.now()
		}
		isExpired := !existingSub.ExpiresAt.After(now)
		if assignmentSemantics {
			isExpired = existingSub.Status == SubscriptionStatusExpired ||
				(existingSub.Status != SubscriptionStatusSuspended && !existingSub.ExpiresAt.After(now))
		}
		newExpiresAt := existingSub.ExpiresAt.AddDate(0, 0, validityDays)
		if isExpired {
			newExpiresAt = now.AddDate(0, 0, validityDays)
		}
		if newExpiresAt.After(MaxExpiresAt) {
			newExpiresAt = MaxExpiresAt
		}
		if assignmentSemantics && strings.TrimSpace(existingSub.Notes) == strings.TrimSpace(notes) {
			notes = ""
		}

		if isExpired {
			renewed := renewedSubscriptionTerm(existingSub, notes, now, newExpiresAt)
			if err := s.userSubRepo.Update(txCtx, renewed); err != nil {
				return fmt.Errorf("renew expired subscription: %w", err)
			}
			return nil
		}

		// 更新过期时间
		if err := s.userSubRepo.ExtendExpiry(txCtx, existingSub.ID, newExpiresAt); err != nil {
			return fmt.Errorf("extend subscription: %w", err)
		}

		// 如果订阅被暂停，恢复为 active 状态
		if existingSub.Status != SubscriptionStatusActive {
			if err := s.userSubRepo.UpdateStatus(txCtx, existingSub.ID, SubscriptionStatusActive); err != nil {
				return fmt.Errorf("update subscription status: %w", err)
			}
		}

		// 追加备注
		if notes != "" {
			if err := s.userSubRepo.UpdateNotes(txCtx, existingSub.ID, appendSubscriptionNotes(existingSub.Notes, notes)); err != nil {
				return fmt.Errorf("update subscription notes: %w", err)
			}
		}

		return nil
	})
}

func (s *SubscriptionService) withSubscriptionUpdateTx(ctx context.Context, fn func(context.Context) error) error {
	if dbent.TxFromContext(ctx) != nil {
		return fn(ctx)
	}
	if s.entClient == nil {
		return fn(ctx)
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	txCtx := dbent.NewTxContext(ctx, tx)

	if err := fn(txCtx); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func renewedSubscriptionTerm(existingSub *UserSubscription, notes string, startsAt, expiresAt time.Time) *UserSubscription {
	renewed := *existingSub
	// 日窗口按日历日对齐（0 点刷新）；周/月窗口按订阅期限对齐（锚点为新周期起点）。
	dailyWindowStart := timezone.StartOfDay(startsAt)
	periodicWindowStart := startsAt
	renewed.StartsAt = startsAt
	renewed.ExpiresAt = expiresAt
	renewed.Status = SubscriptionStatusActive
	renewed.DailyWindowStart = &dailyWindowStart
	renewed.WeeklyWindowStart = &periodicWindowStart
	renewed.MonthlyWindowStart = &periodicWindowStart
	renewed.DailyUsageUSD = 0
	renewed.WeeklyUsageUSD = 0
	renewed.MonthlyUsageUSD = 0
	renewed.Notes = appendSubscriptionNotes(existingSub.Notes, notes)
	return &renewed
}

func appendSubscriptionNotes(existingNotes, newNotes string) string {
	if newNotes == "" {
		return existingNotes
	}
	if existingNotes == "" {
		return newNotes
	}
	return existingNotes + "\n" + newNotes
}

// createSubscription 创建新订阅并生成订阅 key（同一事务）
func (s *SubscriptionService) createSubscription(ctx context.Context, input *AssignSubscriptionInput, plan *SubscriptionPlan) (*UserSubscription, error) {
	validityDays := input.ValidityDays
	if validityDays <= 0 {
		validityDays = 30
	}
	if validityDays > MaxValidityDays {
		validityDays = MaxValidityDays
	}

	now := time.Now()
	expiresAt := now.AddDate(0, 0, validityDays)
	if expiresAt.After(MaxExpiresAt) {
		expiresAt = MaxExpiresAt
	}

	sub := &UserSubscription{
		UserID:     input.UserID,
		PlanID:     plan.ID,
		StartsAt:   now,
		ExpiresAt:  expiresAt,
		Status:     SubscriptionStatusActive,
		AssignedAt: now,
		Notes:      input.Notes,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	// 只有当 AssignedBy > 0 时才设置（0 表示系统分配，如兑换码）
	if input.AssignedBy > 0 {
		sub.AssignedBy = &input.AssignedBy
	}

	err := s.withSubscriptionUpdateTx(ctx, func(txCtx context.Context) error {
		if err := s.userSubRepo.Create(txCtx, sub); err != nil {
			return err
		}
		return s.ensureSubscriptionKey(txCtx, sub, plan.Name)
	})
	if err != nil {
		return nil, err
	}

	// 重新获取完整订阅信息（包含关联）
	return s.userSubRepo.GetByID(ctx, sub.ID)
}

// BulkAssignSubscriptionInput 批量分配订阅输入
type BulkAssignSubscriptionInput struct {
	UserIDs      []int64
	PlanID       int64
	ValidityDays int
	AssignedBy   int64
	Notes        string
}

// BulkAssignResult 批量分配结果
type BulkAssignResult struct {
	SuccessCount  int
	CreatedCount  int
	ReusedCount   int
	FailedCount   int
	Subscriptions []UserSubscription
	Errors        []string
	Statuses      map[int64]string
}

// BulkAssignSubscription 批量分配订阅
func (s *SubscriptionService) BulkAssignSubscription(ctx context.Context, input *BulkAssignSubscriptionInput) (*BulkAssignResult, error) {
	result := &BulkAssignResult{
		Subscriptions: make([]UserSubscription, 0),
		Errors:        make([]string, 0),
		Statuses:      make(map[int64]string),
	}

	for _, userID := range input.UserIDs {
		sub, reused, err := s.assignSubscriptionWithReuse(ctx, &AssignSubscriptionInput{
			UserID:       userID,
			PlanID:       input.PlanID,
			ValidityDays: input.ValidityDays,
			AssignedBy:   input.AssignedBy,
			Notes:        input.Notes,
		})
		if err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, fmt.Sprintf("user %d: %v", userID, err))
			result.Statuses[userID] = "failed"
		} else {
			result.SuccessCount++
			result.Subscriptions = append(result.Subscriptions, *sub)
			if reused {
				result.ReusedCount++
				result.Statuses[userID] = "reused"
			} else {
				result.CreatedCount++
				result.Statuses[userID] = "created"
			}
		}
	}

	return result, nil
}

func (s *SubscriptionService) assignSubscriptionWithReuse(ctx context.Context, input *AssignSubscriptionInput) (*UserSubscription, bool, error) {
	plan, err := s.planRepo.GetByID(ctx, input.PlanID)
	if err != nil {
		return nil, false, err
	}
	if err := s.rejectOtherActivePlan(ctx, input.UserID, input.PlanID); err != nil {
		return nil, false, err
	}

	// 检查是否已存在订阅；若已存在，则按幂等成功返回现有订阅
	exists, err := s.userSubRepo.ExistsByUserIDAndPlanID(ctx, input.UserID, input.PlanID)
	if err != nil {
		return nil, false, err
	}
	if exists {
		sub, getErr := s.userSubRepo.GetByUserIDAndPlanID(ctx, input.UserID, input.PlanID)
		if getErr != nil {
			return nil, false, getErr
		}
		now := time.Now()
		if sub.Status == SubscriptionStatusExpired ||
			(sub.Status != SubscriptionStatusSuspended && !sub.ExpiresAt.After(now)) {
			validityDays := normalizeAssignValidityDays(input.ValidityDays)
			if err := s.updateExistingSubscriptionTerm(ctx, sub.ID, plan.Name, validityDays, input.Notes, true); err != nil {
				return nil, false, err
			}
			s.maybeInvalidateAssignmentCaches(sub, false)
			renewed, getErr := s.userSubRepo.GetByID(ctx, sub.ID)
			return renewed, true, getErr
		}
		if conflictReason, conflict := detectAssignSemanticConflict(sub, input); conflict {
			return nil, false, ErrSubscriptionAssignConflict.WithMetadata(map[string]string{
				"conflict_reason": conflictReason,
			})
		}
		return sub, true, nil
	}

	sub, err := s.createSubscription(ctx, input, plan)
	if err != nil {
		return nil, false, err
	}

	// 失效订阅缓存
	s.invalidateSubscriptionCachesAsync(sub)

	return sub, false, nil
}

func detectAssignSemanticConflict(existing *UserSubscription, input *AssignSubscriptionInput) (string, bool) {
	if existing == nil || input == nil {
		return "", false
	}

	normalizedDays := normalizeAssignValidityDays(input.ValidityDays)
	if !existing.StartsAt.IsZero() {
		expectedExpiresAt := existing.StartsAt.AddDate(0, 0, normalizedDays)
		if expectedExpiresAt.After(MaxExpiresAt) {
			expectedExpiresAt = MaxExpiresAt
		}
		if !existing.ExpiresAt.Equal(expectedExpiresAt) {
			return "validity_days_mismatch", true
		}
	}

	existingNotes := strings.TrimSpace(existing.Notes)
	inputNotes := strings.TrimSpace(input.Notes)
	if existingNotes != inputNotes {
		return "notes_mismatch", true
	}

	return "", false
}

func normalizeAssignValidityDays(days int) int {
	if days <= 0 {
		days = 30
	}
	if days > MaxValidityDays {
		days = MaxValidityDays
	}
	return days
}

// RevokeSubscription 撤销订阅
func (s *SubscriptionService) RevokeSubscription(ctx context.Context, subscriptionID int64) error {
	// 先获取订阅信息用于失效缓存
	sub, err := s.userSubRepo.GetByID(ctx, subscriptionID)
	if err != nil {
		return err
	}

	if err := s.userSubRepo.Delete(ctx, subscriptionID); err != nil {
		return err
	}

	if err := s.invalidateSubscriptionCaches(sub); err != nil {
		return err
	}

	return nil
}

// RestoreSubscription 恢复已撤销订阅
func (s *SubscriptionService) RestoreSubscription(ctx context.Context, subscriptionID int64) (*UserSubscription, error) {
	sub, err := s.userSubRepo.GetByIDIncludeDeleted(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	if sub.DeletedAt == nil {
		return nil, ErrSubscriptionNotRevoked
	}

	// 单订阅：恢复时用户不能已有别的套餐有效
	if err := s.rejectOtherActivePlan(ctx, sub.UserID, sub.PlanID); err != nil {
		return nil, err
	}
	// 同套餐已有未删行（软删后重新分配过）→ 撞部分唯一索引，直接拒
	exists, err := s.userSubRepo.ExistsByUserIDAndPlanID(ctx, sub.UserID, sub.PlanID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrSubscriptionRestoreConflict
	}

	restoredStatus := sub.Status
	now := time.Now()
	if restoredStatus == SubscriptionStatusActive && !sub.ExpiresAt.After(now) {
		restoredStatus = SubscriptionStatusExpired
	}

	restored, err := s.userSubRepo.Restore(ctx, subscriptionID, restoredStatus)
	if err != nil {
		return nil, err
	}
	planName := ""
	if restored.Plan != nil {
		planName = restored.Plan.Name
	} else if plan, err := s.planRepo.GetByID(ctx, restored.PlanID); err == nil {
		planName = plan.Name
	}
	if err := s.withSubscriptionUpdateTx(ctx, func(txCtx context.Context) error {
		return s.ensureSubscriptionKey(txCtx, restored, planName)
	}); err != nil {
		return nil, err
	}

	if err := s.invalidateSubscriptionCaches(restored); err != nil {
		return nil, err
	}
	return restored, nil
}

// ExtendSubscription 调整订阅时长（正数延长，负数缩短）
func (s *SubscriptionService) ExtendSubscription(ctx context.Context, subscriptionID int64, days int) (*UserSubscription, error) {
	err := s.withSubscriptionUpdateTx(ctx, func(txCtx context.Context) error {
		// Lock the row before reading its expiry. Without this lock, concurrent
		// adjustments can both calculate from the same stale expiry and lose one
		// of the updates when they write the absolute timestamp.
		sub, err := s.userSubRepo.GetByIDForUpdate(txCtx, subscriptionID)
		if err != nil {
			return ErrSubscriptionNotFound
		}

		// 限制调整天数范围
		if days > MaxValidityDays {
			days = MaxValidityDays
		}
		if days < -MaxValidityDays {
			days = -MaxValidityDays
		}

		now := time.Now()
		if s.now != nil {
			now = s.now()
		}
		isExpired := !sub.ExpiresAt.After(now)

		// 如果订阅已过期，不允许负向调整
		if isExpired && days < 0 {
			return infraerrors.BadRequest("CANNOT_SHORTEN_EXPIRED", "cannot shorten an expired subscription")
		}

		// 计算新的过期时间
		var newExpiresAt time.Time
		if isExpired {
			// 已过期：从当前时间开始增加天数
			newExpiresAt = now.AddDate(0, 0, days)
		} else {
			// 未过期：从原过期时间增加/减少天数
			newExpiresAt = sub.ExpiresAt.AddDate(0, 0, days)
		}

		if newExpiresAt.After(MaxExpiresAt) {
			newExpiresAt = MaxExpiresAt
		}

		// 检查新的过期时间必须大于当前时间
		if !newExpiresAt.After(now) {
			return ErrAdjustWouldExpire
		}

		if err := s.userSubRepo.ExtendExpiry(txCtx, subscriptionID, newExpiresAt); err != nil {
			return err
		}

		// 如果订阅已过期，恢复为active状态
		if sub.Status == SubscriptionStatusExpired {
			if err := s.userSubRepo.UpdateStatus(txCtx, subscriptionID, SubscriptionStatusActive); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sub, err := s.userSubRepo.GetByID(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}

	// 失效订阅缓存
	s.invalidateSubscriptionCachesAsync(sub)

	return sub, nil
}

// GetByID 根据ID获取订阅
func (s *SubscriptionService) GetByID(ctx context.Context, id int64) (*UserSubscription, error) {
	return s.userSubRepo.GetByID(ctx, id)
}

// GetActiveSubscription 按订阅 ID 取有效订阅（订阅 key 鉴权热路径），并挂上套餐（限额 + 模型集）。
// 订阅与套餐各走一层 L1 + singleflight；返回浅拷贝，调用方可安全修改字段。
// 查不到 / 已过期 / 非 active → ErrSubscriptionNotFound；基础设施错误原样返回（调用方分 403 / 503）。
func (s *SubscriptionService) GetActiveSubscription(ctx context.Context, subscriptionID int64) (*UserSubscription, error) {
	key := subCacheKey(subscriptionID)
	var sub *UserSubscription
	if cached, ok := s.l1Get(key); ok {
		sub, _ = cached.(*UserSubscription)
	}
	if sub == nil {
		value, err, _ := s.subCacheGroup.Do(key, func() (any, error) {
			fresh, err := s.userSubRepo.GetActiveByID(ctx, subscriptionID)
			if err != nil {
				return nil, err // 透传 repo 已翻译的错误（NotFound → ErrSubscriptionNotFound）
			}
			s.l1Set(key, fresh)
			return fresh, nil
		})
		if err != nil {
			return nil, err
		}
		sub, _ = value.(*UserSubscription)
		if sub == nil {
			return nil, ErrSubscriptionNotFound
		}
	}
	plan, err := s.GetPlan(ctx, sub.PlanID)
	if err != nil {
		return nil, err
	}
	cp := *sub
	cp.Plan = plan
	return &cp, nil
}

// GetPlan 按 ID 取套餐（限额 + 模型集），L1 + singleflight；不存在 → ErrPlanNotFound。
// 返回的是缓存指针，调用方只读。
func (s *SubscriptionService) GetPlan(ctx context.Context, planID int64) (*SubscriptionPlan, error) {
	key := planCacheKey(planID)
	if cached, ok := s.l1Get(key); ok {
		if plan, ok := cached.(*SubscriptionPlan); ok && plan != nil {
			return plan, nil
		}
	}
	value, err, _ := s.subCacheGroup.Do(key, func() (any, error) {
		plan, err := s.planRepo.GetByID(ctx, planID)
		if err != nil {
			return nil, err
		}
		s.l1Set(key, plan)
		return plan, nil
	})
	if err != nil {
		return nil, err
	}
	plan, _ := value.(*SubscriptionPlan)
	if plan == nil {
		return nil, ErrPlanNotFound
	}
	return plan, nil
}

// IsSubscriptionInactiveError 报告 GetActiveSubscription 的错误是不是「订阅本身无效」（而非基础设施故障）。
func IsSubscriptionInactiveError(err error) bool {
	return errors.Is(err, ErrSubscriptionNotFound) ||
		errors.Is(err, ErrSubscriptionExpired) ||
		errors.Is(err, ErrSubscriptionSuspended)
}

func (s *SubscriptionService) l1Get(key string) (any, bool) {
	if s.subCacheL1 == nil {
		return nil, false
	}
	return s.subCacheL1.Get(key)
}

func (s *SubscriptionService) l1Set(key string, value any) {
	if s.subCacheL1 == nil {
		return
	}
	_ = s.subCacheL1.SetWithTTL(key, value, 1, s.jitteredTTL(s.subCacheTTL))
}

// ListUserSubscriptions 获取用户的所有订阅
func (s *SubscriptionService) ListUserSubscriptions(ctx context.Context, userID int64) ([]UserSubscription, error) {
	subs, err := s.userSubRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	normalizeExpiredWindows(subs)
	normalizeSubscriptionStatus(subs)
	return subs, nil
}

// ListActiveUserSubscriptions 获取用户的所有有效订阅
func (s *SubscriptionService) ListActiveUserSubscriptions(ctx context.Context, userID int64) ([]UserSubscription, error) {
	subs, err := s.userSubRepo.ListActiveByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	normalizeExpiredWindows(subs)
	return subs, nil
}

// List 获取所有订阅（分页，支持筛选和排序）
func (s *SubscriptionService) List(ctx context.Context, page, pageSize int, userID, planID *int64, status, sortBy, sortOrder string) ([]UserSubscription, *pagination.PaginationResult, error) {
	params := pagination.PaginationParams{Page: page, PageSize: pageSize}
	subs, pag, err := s.userSubRepo.List(ctx, params, userID, planID, status, sortBy, sortOrder)
	if err != nil {
		return nil, nil, err
	}
	normalizeExpiredWindows(subs)
	normalizeSubscriptionStatus(subs)
	return subs, pag, nil
}

// normalizeExpiredWindows 将已过期窗口的数据清零（仅影响返回数据，不影响数据库）
// 这确保前端显示正确的当前窗口状态，而不是过期窗口的历史数据
func normalizeExpiredWindows(subs []UserSubscription) {
	normalizeExpiredWindowsAt(subs, time.Now())
}

func normalizeExpiredWindowsAt(subs []UserSubscription, now time.Time) {
	for i := range subs {
		sub := &subs[i]
		// 日窗口过期：清零展示数据
		if sub.canAutomaticallyResetDailyAt(now) {
			sub.DailyWindowStart = nil
			sub.DailyUsageUSD = 0
		}
		// 周窗口过期：清零展示数据
		if sub.canAutomaticallyResetWeeklyAt(now) {
			sub.WeeklyWindowStart = nil
			sub.WeeklyUsageUSD = 0
		}
		// 月窗口过期：清零展示数据
		if sub.canAutomaticallyResetMonthlyAt(now) {
			sub.MonthlyWindowStart = nil
			sub.MonthlyUsageUSD = 0
		}
	}
}

// normalizeSubscriptionStatus 根据实际过期时间修正状态（仅影响返回数据，不影响数据库）
// 这确保前端显示正确的状态，即使定时任务尚未更新数据库
func normalizeSubscriptionStatus(subs []UserSubscription) {
	now := time.Now()
	for i := range subs {
		sub := &subs[i]
		if sub.Status == SubscriptionStatusActive && !sub.ExpiresAt.After(now) {
			sub.Status = SubscriptionStatusExpired
		}
	}
}

// startOfDay 返回给定时间所在日期的零点（保持原时区）
func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// CheckAndActivateWindow 检查并激活窗口（首次使用时）
func (s *SubscriptionService) CheckAndActivateWindow(ctx context.Context, sub *UserSubscription) error {
	return s.checkAndActivateWindowAt(ctx, sub, s.now())
}

func (s *SubscriptionService) checkAndActivateWindowAt(ctx context.Context, sub *UserSubscription, now time.Time) error {
	if sub.IsWindowActivated() {
		return nil
	}

	// 日窗口锚定当天 0 点（日历日语义）；周/月窗口锚定首次使用时刻（期限对齐语义，
	// 锚点不得早于 StartsAt，否则最后一个不完整周期会重复发放额度，见 issue #5051）。
	return s.userSubRepo.ActivateWindows(ctx, sub.ID, timezone.StartOfDay(now), now)
}

// AdminResetQuota manually resets the daily, weekly, and/or monthly usage windows.
func (s *SubscriptionService) AdminResetQuota(ctx context.Context, subscriptionID int64, resetDaily, resetWeekly, resetMonthly bool) (*UserSubscription, error) {
	if !resetDaily && !resetWeekly && !resetMonthly {
		return nil, ErrInvalidInput
	}
	sub, err := s.userSubRepo.GetByID(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	// 日窗口锚点取当天 0 点：手动重置只清空用量，不改变“每天 0 点刷新”的节奏。
	// 周/月窗口保持锚定重置时刻（期限对齐滚动窗口语义）。
	if err := s.userSubRepo.ResetUsageWindows(ctx, sub.ID, resetDaily, resetWeekly, resetMonthly, timezone.StartOfDay(now), now); err != nil {
		return nil, err
	}
	// Invalidate L1 ristretto cache. Ristretto's Del() is asynchronous by design,
	// so call Wait() immediately after to flush pending operations and guarantee
	// the deleted key is not returned on the very next Get() call.
	s.InvalidateSubCacheSync(sub.ID)
	if s.billingCacheService != nil {
		_ = s.billingCacheService.InvalidateSubscription(ctx, sub.UserID, sub.PlanID)
	}
	// Return the refreshed subscription from DB
	return s.userSubRepo.GetByID(ctx, subscriptionID)
}

// CheckAndResetWindows 检查并重置过期的窗口
func (s *SubscriptionService) CheckAndResetWindows(ctx context.Context, sub *UserSubscription) error {
	now := s.now()
	needsInvalidateCache := false

	// 日窗口重置（每天 0 点刷新，按日历日对齐）
	if windowStart, ok := sub.automaticDailyWindowStartAt(now); ok {
		expectedWindowStart := sub.DailyWindowStart
		if err := s.userSubRepo.ResetDailyUsage(ctx, sub.ID, expectedWindowStart, windowStart); err != nil {
			return err
		}
		sub.DailyWindowStart = &windowStart
		sub.DailyUsageUSD = 0
		needsInvalidateCache = true
	}

	// 周窗口重置（7天）
	if windowStart, ok := sub.automaticWindowStartAt(sub.WeeklyWindowStart, 7*24*time.Hour, now); ok {
		expectedWindowStart := sub.WeeklyWindowStart
		if err := s.userSubRepo.ResetWeeklyUsage(ctx, sub.ID, expectedWindowStart, windowStart); err != nil {
			return err
		}
		sub.WeeklyWindowStart = &windowStart
		sub.WeeklyUsageUSD = 0
		needsInvalidateCache = true
	}

	// 月窗口重置（30天）
	if windowStart, ok := sub.automaticWindowStartAt(sub.MonthlyWindowStart, 30*24*time.Hour, now); ok {
		expectedWindowStart := sub.MonthlyWindowStart
		if err := s.userSubRepo.ResetMonthlyUsage(ctx, sub.ID, expectedWindowStart, windowStart); err != nil {
			return err
		}
		sub.MonthlyWindowStart = &windowStart
		sub.MonthlyUsageUSD = 0
		needsInvalidateCache = true
	}

	// 如果有窗口被重置，失效缓存以保持一致性
	if needsInvalidateCache {
		s.InvalidateSubCache(sub.ID)
		if s.billingCacheService != nil {
			_ = s.billingCacheService.InvalidateSubscription(ctx, sub.UserID, sub.PlanID)
		}
	}

	return nil
}

// EnsureWindowMaintenance advances expired usage windows before a request is
// allowed to proceed. It returns a fresh database snapshot because a competing
// request may have won one of the conditional resets.
func (s *SubscriptionService) EnsureWindowMaintenance(ctx context.Context, sub *UserSubscription) (*UserSubscription, error) {
	if sub == nil {
		return nil, ErrSubscriptionNilInput
	}
	if !sub.IsWindowActivated() {
		if err := s.CheckAndActivateWindow(ctx, sub); err != nil {
			return nil, err
		}
	}
	if err := s.CheckAndResetWindows(ctx, sub); err != nil {
		return nil, err
	}

	// GetByID bypasses the service caches. This prevents a stale loser of the
	// CAS from validating limits against zeroed in-memory usage.
	refreshed, err := s.userSubRepo.GetByID(ctx, sub.ID)
	if err != nil {
		return nil, err
	}
	// 回读不带套餐（GetByID 的边只到套餐主表），沿用请求里已挂好的套餐
	refreshed.Plan = sub.Plan
	s.InvalidateSubCacheSync(sub.ID)
	return refreshed, nil
}

// ValidateAndCheckLimits 合并验证+限额检查（中间件热路径专用）
// 仅做内存检查，不触发 DB 写入。调用方必须在放行请求前同步完成窗口维护。
// 返回 needsMaintenance 表示是否需要执行窗口维护并回读数据库快照。
func (s *SubscriptionService) ValidateAndCheckLimits(sub *UserSubscription) (needsMaintenance bool, err error) {
	now := s.now()
	// 1. 验证订阅状态
	if sub.Status == SubscriptionStatusExpired {
		return false, ErrSubscriptionExpired
	}
	if sub.Status == SubscriptionStatusSuspended {
		return false, ErrSubscriptionSuspended
	}
	if !sub.ExpiresAt.After(now) {
		return false, ErrSubscriptionExpired
	}

	// 2. 内存中修正过期窗口的用量，确保预检查不会误拒绝用户。
	//    调用方随后同步推进 DB 窗口，并用回读快照重新校验。
	if sub.canAutomaticallyResetDailyAt(now) {
		sub.DailyUsageUSD = 0
		needsMaintenance = true
	}
	if sub.canAutomaticallyResetWeeklyAt(now) {
		sub.WeeklyUsageUSD = 0
		needsMaintenance = true
	}
	if sub.canAutomaticallyResetMonthlyAt(now) {
		sub.MonthlyUsageUSD = 0
		needsMaintenance = true
	}
	if !sub.IsWindowActivated() {
		needsMaintenance = true
	}

	// 3. 检查用量限额（套餐限额）
	if !sub.CheckDailyLimit(sub.Plan, 0) {
		return needsMaintenance, ErrDailyLimitExceeded
	}
	if !sub.CheckWeeklyLimit(sub.Plan, 0) {
		return needsMaintenance, ErrWeeklyLimitExceeded
	}
	if !sub.CheckMonthlyLimit(sub.Plan, 0) {
		return needsMaintenance, ErrMonthlyLimitExceeded
	}

	return needsMaintenance, nil
}

// DoWindowMaintenance 异步执行窗口维护（激活+重置）
// 使用独立 context，不受请求取消影响。
// 注意：此方法仅在 ValidateAndCheckLimits 返回 needsMaintenance=true 时调用，
// 而 IsExpired()=true 的订阅在 ValidateAndCheckLimits 中已被拦截返回错误，
// 因此进入此方法的订阅一定未过期，无需处理过期状态同步。
func (s *SubscriptionService) DoWindowMaintenance(sub *UserSubscription) {
	if s == nil {
		return
	}
	if s.maintenanceQueue != nil {
		err := s.maintenanceQueue.TryEnqueue(func() {
			s.doWindowMaintenance(sub)
		})
		if err != nil {
			log.Printf("Subscription maintenance enqueue failed: %v", err)
		}
		return
	}

	s.doWindowMaintenance(sub)
}

func (s *SubscriptionService) doWindowMaintenance(sub *UserSubscription) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 激活窗口（首次使用时）
	if !sub.IsWindowActivated() {
		if err := s.CheckAndActivateWindow(ctx, sub); err != nil {
			log.Printf("Failed to activate subscription windows: %v", err)
		}
	}

	// 重置过期窗口
	if err := s.CheckAndResetWindows(ctx, sub); err != nil {
		log.Printf("Failed to reset subscription windows: %v", err)
	}

	// 失效 L1 缓存，确保后续请求拿到更新后的数据
	s.InvalidateSubCache(sub.ID)
}

// RecordUsage 记录使用量到订阅
func (s *SubscriptionService) RecordUsage(ctx context.Context, subscriptionID int64, costUSD float64) error {
	return s.userSubRepo.IncrementUsage(ctx, subscriptionID, costUSD)
}

// SubscriptionProgress 订阅进度
type SubscriptionProgress struct {
	ID            int64                `json:"id"`
	PlanName      string               `json:"plan_name"`
	ExpiresAt     time.Time            `json:"expires_at"`
	ExpiresInDays int                  `json:"expires_in_days"`
	Daily         *UsageWindowProgress `json:"daily,omitempty"`
	Weekly        *UsageWindowProgress `json:"weekly,omitempty"`
	Monthly       *UsageWindowProgress `json:"monthly,omitempty"`
}

// UsageWindowProgress 使用窗口进度
type UsageWindowProgress struct {
	LimitUSD        float64   `json:"limit_usd"`
	UsedUSD         float64   `json:"used_usd"`
	RemainingUSD    float64   `json:"remaining_usd"`
	Percentage      float64   `json:"percentage"`
	WindowStart     time.Time `json:"window_start"`
	ResetsAt        time.Time `json:"resets_at"`
	ResetsInSeconds int64     `json:"resets_in_seconds"`
}

// GetSubscriptionProgress 获取订阅使用进度
func (s *SubscriptionService) GetSubscriptionProgress(ctx context.Context, subscriptionID int64) (*SubscriptionProgress, error) {
	sub, err := s.userSubRepo.GetByID(ctx, subscriptionID)
	if err != nil {
		return nil, ErrSubscriptionNotFound
	}

	plan := sub.Plan
	if plan == nil {
		plan, err = s.planRepo.GetByID(ctx, sub.PlanID)
		if err != nil {
			return nil, err
		}
	}

	return s.calculateProgress(sub, plan), nil
}

// calculateProgress 根据已加载的订阅和套餐数据计算使用进度（纯内存计算，无 DB 查询）
func (s *SubscriptionService) calculateProgress(sub *UserSubscription, plan *SubscriptionPlan) *SubscriptionProgress {
	progress := &SubscriptionProgress{
		ID:            sub.ID,
		PlanName:      plan.Name,
		ExpiresAt:     sub.ExpiresAt,
		ExpiresInDays: sub.DaysRemaining(),
	}

	// 日进度
	if plan.HasDailyLimit() && sub.DailyWindowStart != nil {
		limit := *plan.DailyLimitUSD
		resetsAt := sub.DailyWindowStart.Add(24 * time.Hour)
		if dailyResetTime := sub.DailyResetTime(); dailyResetTime != nil {
			resetsAt = *dailyResetTime
		}
		progress.Daily = &UsageWindowProgress{
			LimitUSD:        limit,
			UsedUSD:         sub.DailyUsageUSD,
			RemainingUSD:    limit - sub.DailyUsageUSD,
			Percentage:      (sub.DailyUsageUSD / limit) * 100,
			WindowStart:     *sub.DailyWindowStart,
			ResetsAt:        resetsAt,
			ResetsInSeconds: int64(time.Until(resetsAt).Seconds()),
		}
		if progress.Daily.RemainingUSD < 0 {
			progress.Daily.RemainingUSD = 0
		}
		if progress.Daily.Percentage > 100 {
			progress.Daily.Percentage = 100
		}
		if progress.Daily.ResetsInSeconds < 0 {
			progress.Daily.ResetsInSeconds = 0
		}
	}

	// 周进度
	if plan.HasWeeklyLimit() && sub.WeeklyWindowStart != nil {
		limit := *plan.WeeklyLimitUSD
		resetsAt := sub.WeeklyWindowStart.Add(7 * 24 * time.Hour)
		if weeklyResetTime := sub.WeeklyResetTime(); weeklyResetTime != nil {
			resetsAt = *weeklyResetTime
		}
		progress.Weekly = &UsageWindowProgress{
			LimitUSD:        limit,
			UsedUSD:         sub.WeeklyUsageUSD,
			RemainingUSD:    limit - sub.WeeklyUsageUSD,
			Percentage:      (sub.WeeklyUsageUSD / limit) * 100,
			WindowStart:     *sub.WeeklyWindowStart,
			ResetsAt:        resetsAt,
			ResetsInSeconds: int64(time.Until(resetsAt).Seconds()),
		}
		if progress.Weekly.RemainingUSD < 0 {
			progress.Weekly.RemainingUSD = 0
		}
		if progress.Weekly.Percentage > 100 {
			progress.Weekly.Percentage = 100
		}
		if progress.Weekly.ResetsInSeconds < 0 {
			progress.Weekly.ResetsInSeconds = 0
		}
	}

	// 月进度
	if plan.HasMonthlyLimit() && sub.MonthlyWindowStart != nil {
		limit := *plan.MonthlyLimitUSD
		resetsAt := sub.MonthlyWindowStart.Add(30 * 24 * time.Hour)
		if monthlyResetTime := sub.MonthlyResetTime(); monthlyResetTime != nil {
			resetsAt = *monthlyResetTime
		}
		progress.Monthly = &UsageWindowProgress{
			LimitUSD:        limit,
			UsedUSD:         sub.MonthlyUsageUSD,
			RemainingUSD:    limit - sub.MonthlyUsageUSD,
			Percentage:      (sub.MonthlyUsageUSD / limit) * 100,
			WindowStart:     *sub.MonthlyWindowStart,
			ResetsAt:        resetsAt,
			ResetsInSeconds: int64(time.Until(resetsAt).Seconds()),
		}
		if progress.Monthly.RemainingUSD < 0 {
			progress.Monthly.RemainingUSD = 0
		}
		if progress.Monthly.Percentage > 100 {
			progress.Monthly.Percentage = 100
		}
		if progress.Monthly.ResetsInSeconds < 0 {
			progress.Monthly.ResetsInSeconds = 0
		}
	}

	return progress
}
