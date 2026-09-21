package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/dgraph-io/ristretto"
	"github.com/stretchr/testify/require"
)

func TestWithSubscriptionUpdateTx_ReusesExistingTransaction(t *testing.T) {
	existingTx := &dbent.Tx{}
	ctx := dbent.NewTxContext(context.Background(), existingTx)
	svc := &SubscriptionService{entClient: &dbent.Client{}}

	called := false
	err := svc.withSubscriptionUpdateTx(ctx, func(txCtx context.Context) error {
		called = true
		require.Same(t, existingTx, dbent.TxFromContext(txCtx))
		return nil
	})

	require.NoError(t, err)
	require.True(t, called)
}

func TestMaybeInvalidateAssignmentCaches_DefersForOuterTransactionOwner(t *testing.T) {
	cache, err := ristretto.NewCache(&ristretto.Config{NumCounters: 1_000, MaxCost: 100, BufferItems: 64})
	require.NoError(t, err)
	t.Cleanup(cache.Close)

	svc := &SubscriptionService{subCacheL1: cache}
	sub := &UserSubscription{ID: 42, UserID: 7, PlanID: 9}
	key := subCacheKey(sub.ID)
	require.True(t, cache.Set(key, sub, 1))
	cache.Wait()

	svc.maybeInvalidateAssignmentCaches(sub, true)
	_, cachedBeforeCommit := cache.Get(key)
	require.True(t, cachedBeforeCommit, "outer transaction must retain caches until its owner commits")

	svc.maybeInvalidateAssignmentCaches(sub, false)
	cache.Wait()
	_, cachedAfterCommit := cache.Get(key)
	require.False(t, cachedAfterCommit, "post-commit invalidation must remove the cached subscription")
}

// planRepoNoop 套餐仓储桩：任何调用都 panic（用例只想要一个能过编译的依赖时用）
type planRepoNoop struct{}

func (planRepoNoop) GetByID(context.Context, int64) (*SubscriptionPlan, error) {
	panic("unexpected GetByID call")
}
func (planRepoNoop) List(context.Context, bool) ([]SubscriptionPlan, error) {
	panic("unexpected List call")
}
func (planRepoNoop) Create(context.Context, *SubscriptionPlan) error { panic("unexpected Create call") }
func (planRepoNoop) Update(context.Context, *SubscriptionPlan) error { panic("unexpected Update call") }
func (planRepoNoop) Delete(context.Context, int64) error             { panic("unexpected Delete call") }

// subscriptionPlanRepoStub 固定返回一个套餐；plan 为 nil 时返回 ErrPlanNotFound
type subscriptionPlanRepoStub struct {
	planRepoNoop
	plan *SubscriptionPlan
}

// GetByID 像真仓储一样返回请求的 ID（plan.ID 为 0 时回填），名字等字段来自固定套餐
func (s *subscriptionPlanRepoStub) GetByID(_ context.Context, id int64) (*SubscriptionPlan, error) {
	if s.plan == nil {
		return nil, ErrPlanNotFound
	}
	cp := *s.plan
	if cp.ID == 0 || cp.ID != id {
		cp.ID = id
	}
	return &cp, nil
}

// subscriptionKeyRepoStub 记录订阅 key 的生成
type subscriptionKeyRepoStub struct {
	created []*APIKey
	exists  map[int64]bool
	err     error
}

func (s *subscriptionKeyRepoStub) Create(_ context.Context, key *APIKey) error {
	if s.err != nil {
		return s.err
	}
	cp := *key
	s.created = append(s.created, &cp)
	if s.exists == nil {
		s.exists = map[int64]bool{}
	}
	if key.SubscriptionID != nil {
		s.exists[*key.SubscriptionID] = true
	}
	return nil
}

func (s *subscriptionKeyRepoStub) ExistsBySubscriptionID(_ context.Context, id int64) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.exists[id], nil
}

type userSubRepoNoop struct{}

func (userSubRepoNoop) Create(context.Context, *UserSubscription) error {
	panic("unexpected Create call")
}
func (userSubRepoNoop) GetByID(context.Context, int64) (*UserSubscription, error) {
	panic("unexpected GetByID call")
}
func (userSubRepoNoop) GetByIDForUpdate(context.Context, int64) (*UserSubscription, error) {
	panic("unexpected GetByIDForUpdate call")
}
func (userSubRepoNoop) GetByIDIncludeDeleted(context.Context, int64) (*UserSubscription, error) {
	panic("unexpected GetByIDIncludeDeleted call")
}
func (userSubRepoNoop) GetByUserIDAndPlanID(context.Context, int64, int64) (*UserSubscription, error) {
	panic("unexpected GetByUserIDAndPlanID call")
}
func (userSubRepoNoop) GetActiveByUserIDAndPlanID(context.Context, int64, int64) (*UserSubscription, error) {
	panic("unexpected GetActiveByUserIDAndPlanID call")
}
func (userSubRepoNoop) GetActiveByID(context.Context, int64) (*UserSubscription, error) {
	panic("unexpected GetActiveByID call")
}
func (userSubRepoNoop) Update(context.Context, *UserSubscription) error {
	panic("unexpected Update call")
}
func (userSubRepoNoop) Delete(context.Context, int64) error { panic("unexpected Delete call") }
func (userSubRepoNoop) Restore(context.Context, int64, string) (*UserSubscription, error) {
	panic("unexpected Restore call")
}
func (userSubRepoNoop) ListByUserID(context.Context, int64) ([]UserSubscription, error) {
	panic("unexpected ListByUserID call")
}
func (userSubRepoNoop) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	panic("unexpected ListActiveByUserID call")
}
func (userSubRepoNoop) List(context.Context, pagination.PaginationParams, *int64, *int64, string, string, string) ([]UserSubscription, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}
func (userSubRepoNoop) ExistsByUserIDAndPlanID(context.Context, int64, int64) (bool, error) {
	panic("unexpected ExistsByUserIDAndPlanID call")
}
func (userSubRepoNoop) ExtendExpiry(context.Context, int64, time.Time) error {
	panic("unexpected ExtendExpiry call")
}
func (userSubRepoNoop) UpdateStatus(context.Context, int64, string) error {
	panic("unexpected UpdateStatus call")
}
func (userSubRepoNoop) UpdateNotes(context.Context, int64, string) error {
	panic("unexpected UpdateNotes call")
}
func (userSubRepoNoop) ActivateWindows(context.Context, int64, time.Time, time.Time) error {
	panic("unexpected ActivateWindows call")
}
func (userSubRepoNoop) ResetUsageWindows(context.Context, int64, bool, bool, bool, time.Time, time.Time) error {
	panic("unexpected ResetUsageWindows call")
}
func (userSubRepoNoop) ResetDailyUsage(context.Context, int64, *time.Time, time.Time) error {
	panic("unexpected ResetDailyUsage call")
}
func (userSubRepoNoop) ResetWeeklyUsage(context.Context, int64, *time.Time, time.Time) error {
	panic("unexpected ResetWeeklyUsage call")
}
func (userSubRepoNoop) ResetMonthlyUsage(context.Context, int64, *time.Time, time.Time) error {
	panic("unexpected ResetMonthlyUsage call")
}
func (userSubRepoNoop) IncrementUsage(context.Context, int64, float64) error {
	panic("unexpected IncrementUsage call")
}
func (userSubRepoNoop) BatchUpdateExpiredStatus(context.Context) (int64, error) {
	panic("unexpected BatchUpdateExpiredStatus call")
}

type subscriptionUserSubRepoStub struct {
	userSubRepoNoop

	nextID      int64
	byID        map[int64]*UserSubscription
	byUserPlan  map[string]*UserSubscription
	createCalls int
	// deleted 一条软删行（RestoreSubscription 用例）；restored 记录是否真的恢复过
	deleted  *UserSubscription
	restored bool
}

func (s *subscriptionUserSubRepoStub) ExtendExpiry(_ context.Context, id int64, expiresAt time.Time) error {
	sub := s.byID[id]
	if sub == nil {
		return ErrSubscriptionNotFound
	}
	sub.ExpiresAt = expiresAt
	return nil
}

func (s *subscriptionUserSubRepoStub) UpdateStatus(_ context.Context, id int64, status string) error {
	sub := s.byID[id]
	if sub == nil {
		return ErrSubscriptionNotFound
	}
	sub.Status = status
	return nil
}

func (s *subscriptionUserSubRepoStub) UpdateNotes(_ context.Context, id int64, notes string) error {
	sub := s.byID[id]
	if sub == nil {
		return ErrSubscriptionNotFound
	}
	sub.Notes = notes
	return nil
}

func (s *subscriptionUserSubRepoStub) GetByIDIncludeDeleted(_ context.Context, id int64) (*UserSubscription, error) {
	if s.deleted != nil && s.deleted.ID == id {
		cp := *s.deleted
		return &cp, nil
	}
	if sub := s.byID[id]; sub != nil {
		cp := *sub
		return &cp, nil
	}
	return nil, ErrSubscriptionNotFound
}

func (s *subscriptionUserSubRepoStub) Restore(_ context.Context, id int64, restoredStatus string) (*UserSubscription, error) {
	if s.deleted == nil || s.deleted.ID != id {
		return nil, ErrSubscriptionNotFound
	}
	s.restored = true
	cp := *s.deleted
	cp.DeletedAt = nil
	cp.Status = restoredStatus
	s.byID[cp.ID] = &cp
	s.byUserPlan[s.key(cp.UserID, cp.PlanID)] = &cp
	s.deleted = nil
	return &cp, nil
}

func newSubscriptionUserSubRepoStub() *subscriptionUserSubRepoStub {
	return &subscriptionUserSubRepoStub{
		nextID:     1,
		byID:       make(map[int64]*UserSubscription),
		byUserPlan: make(map[string]*UserSubscription),
	}
}

func (s *subscriptionUserSubRepoStub) key(userID, planID int64) string {
	return strconvFormatInt(userID) + ":" + strconvFormatInt(planID)
}

// ListActiveByUserID 单订阅判定读它：status active 且未过期
func (s *subscriptionUserSubRepoStub) ListActiveByUserID(_ context.Context, userID int64) ([]UserSubscription, error) {
	now := time.Now()
	out := make([]UserSubscription, 0)
	for _, sub := range s.byID {
		if sub.UserID == userID && sub.Status == SubscriptionStatusActive && sub.ExpiresAt.After(now) {
			out = append(out, *sub)
		}
	}
	return out, nil
}

func (s *subscriptionUserSubRepoStub) seed(sub *UserSubscription) {
	if sub == nil {
		return
	}
	cp := *sub
	if cp.ID == 0 {
		cp.ID = s.nextID
		s.nextID++
	}
	s.byID[cp.ID] = &cp
	s.byUserPlan[s.key(cp.UserID, cp.PlanID)] = &cp
}

func (s *subscriptionUserSubRepoStub) ExistsByUserIDAndPlanID(_ context.Context, userID, planID int64) (bool, error) {
	_, ok := s.byUserPlan[s.key(userID, planID)]
	return ok, nil
}

func (s *subscriptionUserSubRepoStub) GetByUserIDAndPlanID(_ context.Context, userID, planID int64) (*UserSubscription, error) {
	sub := s.byUserPlan[s.key(userID, planID)]
	if sub == nil {
		return nil, ErrSubscriptionNotFound
	}
	cp := *sub
	return &cp, nil
}

func (s *subscriptionUserSubRepoStub) Create(_ context.Context, sub *UserSubscription) error {
	if sub == nil {
		return nil
	}
	s.createCalls++
	cp := *sub
	if cp.ID == 0 {
		cp.ID = s.nextID
		s.nextID++
	}
	sub.ID = cp.ID
	s.byID[cp.ID] = &cp
	s.byUserPlan[s.key(cp.UserID, cp.PlanID)] = &cp
	return nil
}

func (s *subscriptionUserSubRepoStub) GetByID(_ context.Context, id int64) (*UserSubscription, error) {
	sub := s.byID[id]
	if sub == nil {
		return nil, ErrSubscriptionNotFound
	}
	cp := *sub
	return &cp, nil
}

func (s *subscriptionUserSubRepoStub) GetByIDForUpdate(ctx context.Context, id int64) (*UserSubscription, error) {
	return s.GetByID(ctx, id)
}

func (s *subscriptionUserSubRepoStub) Update(_ context.Context, sub *UserSubscription) error {
	if sub == nil {
		return ErrSubscriptionNilInput
	}
	existing := s.byID[sub.ID]
	if existing == nil {
		return ErrSubscriptionNotFound
	}
	oldKey := s.key(existing.UserID, existing.PlanID)
	cp := *sub
	s.byID[cp.ID] = &cp
	if oldKey != s.key(cp.UserID, cp.PlanID) {
		delete(s.byUserPlan, oldKey)
	}
	s.byUserPlan[s.key(cp.UserID, cp.PlanID)] = &cp
	return nil
}

func TestAssignSubscriptionReuseWhenSemanticsMatch(t *testing.T) {
	start := time.Now().Add(-time.Hour)
	planRepo := &subscriptionPlanRepoStub{plan: &SubscriptionPlan{ID: 1, Name: "plan"}}
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:        10,
		UserID:    1001,
		PlanID:    1,
		StartsAt:  start,
		ExpiresAt: start.AddDate(0, 0, 30),
		Status:    SubscriptionStatusActive,
		Notes:     "init",
	})

	svc := NewSubscriptionService(planRepo, subRepo, &subscriptionKeyRepoStub{}, nil, nil, nil)
	sub, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       1001,
		PlanID:       1,
		ValidityDays: 30,
		Notes:        "init",
	})
	require.NoError(t, err)
	require.Equal(t, int64(10), sub.ID)
	require.Equal(t, 0, subRepo.createCalls, "reuse should not create new subscription")
	require.Equal(t, start, sub.StartsAt)
	require.Equal(t, start.AddDate(0, 0, 30), sub.ExpiresAt)
}

func TestAssignSubscriptionDoesNotReactivateFutureSuspendedSubscription(t *testing.T) {
	start := time.Now().Add(-time.Hour)
	planRepo := &subscriptionPlanRepoStub{plan: &SubscriptionPlan{ID: 1, Name: "plan"}}
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:        13,
		UserID:    1003,
		PlanID:    1,
		StartsAt:  start,
		ExpiresAt: start.AddDate(0, 0, 30),
		Status:    SubscriptionStatusSuspended,
		Notes:     "assignment",
	})

	svc := NewSubscriptionService(planRepo, subRepo, &subscriptionKeyRepoStub{}, nil, nil, nil)
	sub, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       1003,
		PlanID:       1,
		ValidityDays: 30,
		Notes:        "assignment",
	})

	require.NoError(t, err)
	require.Equal(t, int64(13), sub.ID)
	require.Equal(t, SubscriptionStatusSuspended, sub.Status)
	require.Equal(t, start, sub.StartsAt)
	require.Equal(t, start.AddDate(0, 0, 30), sub.ExpiresAt)
	require.Equal(t, "assignment", sub.Notes)
	require.Equal(t, 0, subRepo.createCalls)
}

func TestAssignSubscriptionDoesNotReactivatePastExpirySuspendedSubscription(t *testing.T) {
	start := time.Now().AddDate(0, 0, -31)
	expiresAt := start.AddDate(0, 0, 30)
	windowStart := startOfDay(start)
	planRepo := &subscriptionPlanRepoStub{plan: &SubscriptionPlan{ID: 1, Name: "plan"}}
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:                 15,
		UserID:             1005,
		PlanID:             1,
		StartsAt:           start,
		ExpiresAt:          expiresAt,
		Status:             SubscriptionStatusSuspended,
		DailyWindowStart:   &windowStart,
		WeeklyWindowStart:  &windowStart,
		MonthlyWindowStart: &windowStart,
		DailyUsageUSD:      1,
		WeeklyUsageUSD:     2,
		MonthlyUsageUSD:    3,
		Notes:              "suspended assignment",
	})

	svc := NewSubscriptionService(planRepo, subRepo, &subscriptionKeyRepoStub{}, nil, nil, nil)
	sub, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       1005,
		PlanID:       1,
		ValidityDays: 30,
		Notes:        "suspended assignment",
	})

	require.NoError(t, err)
	require.Equal(t, int64(15), sub.ID)
	require.Equal(t, SubscriptionStatusSuspended, sub.Status)
	require.Equal(t, start, sub.StartsAt)
	require.Equal(t, expiresAt, sub.ExpiresAt)
	require.Equal(t, "suspended assignment", sub.Notes)
	require.Equal(t, &windowStart, sub.DailyWindowStart)
	require.Equal(t, &windowStart, sub.WeeklyWindowStart)
	require.Equal(t, &windowStart, sub.MonthlyWindowStart)
	require.Equal(t, float64(1), sub.DailyUsageUSD)
	require.Equal(t, float64(2), sub.WeeklyUsageUSD)
	require.Equal(t, float64(3), sub.MonthlyUsageUSD)
	require.Equal(t, 0, subRepo.createCalls)
}

func TestAssignSubscriptionRenewsExpiredSemanticMatch(t *testing.T) {
	planRepo := &subscriptionPlanRepoStub{plan: &SubscriptionPlan{ID: 1, Name: "plan"}}
	subRepo := newSubscriptionUserSubRepoStub()
	oldStart := time.Now().Add(-time.Hour)
	oldWindowStart := startOfDay(oldStart)
	subRepo.seed(&UserSubscription{
		ID:                 12,
		UserID:             1002,
		PlanID:             1,
		StartsAt:           oldStart,
		ExpiresAt:          oldStart.AddDate(0, 0, 30),
		Status:             SubscriptionStatusExpired,
		DailyWindowStart:   &oldWindowStart,
		WeeklyWindowStart:  &oldWindowStart,
		MonthlyWindowStart: &oldWindowStart,
		DailyUsageUSD:      1,
		WeeklyUsageUSD:     2,
		MonthlyUsageUSD:    3,
		Notes:              " assignment ",
	})

	svc := NewSubscriptionService(planRepo, subRepo, &subscriptionKeyRepoStub{}, nil, nil, nil)
	before := time.Now()
	sub, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       1002,
		PlanID:       1,
		ValidityDays: 30,
		Notes:        "assignment",
	})
	after := time.Now()

	require.NoError(t, err)
	require.Equal(t, int64(12), sub.ID)
	require.Equal(t, 0, subRepo.createCalls)
	require.Equal(t, SubscriptionStatusActive, sub.Status)
	require.False(t, sub.StartsAt.Before(before))
	require.False(t, sub.StartsAt.After(after))
	require.Equal(t, sub.StartsAt.AddDate(0, 0, 30), sub.ExpiresAt)
	require.Equal(t, timezone.StartOfDay(sub.StartsAt), *sub.DailyWindowStart, "续期后日窗口应锚定当天 0 点")
	require.Equal(t, sub.StartsAt, *sub.WeeklyWindowStart)
	require.Equal(t, sub.StartsAt, *sub.MonthlyWindowStart)
	require.Zero(t, sub.DailyUsageUSD)
	require.Zero(t, sub.WeeklyUsageUSD)
	require.Zero(t, sub.MonthlyUsageUSD)
	require.Equal(t, " assignment ", sub.Notes)
}

func TestAssignSubscriptionRenewsExpiredAndAppendsDifferentNotes(t *testing.T) {
	planRepo := &subscriptionPlanRepoStub{plan: &SubscriptionPlan{ID: 1, Name: "plan"}}
	subRepo := newSubscriptionUserSubRepoStub()
	oldStart := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	subRepo.seed(&UserSubscription{
		ID:        14,
		UserID:    1004,
		PlanID:    1,
		StartsAt:  oldStart,
		ExpiresAt: oldStart.AddDate(0, 0, 30),
		Status:    SubscriptionStatusExpired,
		Notes:     "old assignment",
	})

	svc := NewSubscriptionService(planRepo, subRepo, &subscriptionKeyRepoStub{}, nil, nil, nil)
	sub, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       1004,
		PlanID:       1,
		ValidityDays: 30,
		Notes:        "new assignment",
	})

	require.NoError(t, err)
	require.Equal(t, "old assignment\nnew assignment", sub.Notes)
}

func TestAssignSubscriptionConflictWhenSemanticsMismatch(t *testing.T) {
	start := time.Now().Add(-time.Hour)
	planRepo := &subscriptionPlanRepoStub{plan: &SubscriptionPlan{ID: 1, Name: "plan"}}
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:        11,
		UserID:    2001,
		PlanID:    1,
		StartsAt:  start,
		ExpiresAt: start.AddDate(0, 0, 30),
		Status:    SubscriptionStatusActive,
		Notes:     "old-note",
	})

	svc := NewSubscriptionService(planRepo, subRepo, &subscriptionKeyRepoStub{}, nil, nil, nil)
	_, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       2001,
		PlanID:       1,
		ValidityDays: 30,
		Notes:        "new-note",
	})
	require.Error(t, err)
	require.Equal(t, "SUBSCRIPTION_ASSIGN_CONFLICT", infraerrorsReason(err))
	require.Equal(t, 0, subRepo.createCalls, "conflict should not create or mutate existing subscription")
}

func TestBulkAssignSubscriptionCreatedReusedAndConflict(t *testing.T) {
	start := time.Now().Add(-time.Hour)
	planRepo := &subscriptionPlanRepoStub{plan: &SubscriptionPlan{ID: 1, Name: "plan"}}
	subRepo := newSubscriptionUserSubRepoStub()
	// user 1: 语义一致，可 reused
	subRepo.seed(&UserSubscription{
		ID:        21,
		UserID:    1,
		PlanID:    1,
		StartsAt:  start,
		ExpiresAt: start.AddDate(0, 0, 30),
		Status:    SubscriptionStatusActive,
		Notes:     "same-note",
	})
	// user 3: 语义冲突（有效期不一致），应 failed
	subRepo.seed(&UserSubscription{
		ID:        23,
		UserID:    3,
		PlanID:    1,
		StartsAt:  start,
		ExpiresAt: start.AddDate(0, 0, 60),
		Status:    SubscriptionStatusActive,
		Notes:     "same-note",
	})

	svc := NewSubscriptionService(planRepo, subRepo, &subscriptionKeyRepoStub{}, nil, nil, nil)
	result, err := svc.BulkAssignSubscription(context.Background(), &BulkAssignSubscriptionInput{
		UserIDs:      []int64{1, 2, 3},
		PlanID:       1,
		ValidityDays: 30,
		AssignedBy:   9,
		Notes:        "same-note",
	})
	require.NoError(t, err)
	require.Equal(t, 2, result.SuccessCount)
	require.Equal(t, 1, result.CreatedCount)
	require.Equal(t, 1, result.ReusedCount)
	require.Equal(t, 1, result.FailedCount)
	require.Equal(t, "reused", result.Statuses[1])
	require.Equal(t, "created", result.Statuses[2])
	require.Equal(t, "failed", result.Statuses[3])
	require.Equal(t, 1, subRepo.createCalls)
}

func TestBulkAssignSubscriptionRenewsExpiredSemanticMatch(t *testing.T) {
	planRepo := &subscriptionPlanRepoStub{plan: &SubscriptionPlan{ID: 1, Name: "plan"}}
	subRepo := newSubscriptionUserSubRepoStub()
	oldStart := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	subRepo.seed(&UserSubscription{
		ID:              24,
		UserID:          4,
		PlanID:          1,
		StartsAt:        oldStart,
		ExpiresAt:       oldStart.AddDate(0, 0, 7),
		Status:          SubscriptionStatusExpired,
		DailyUsageUSD:   1,
		WeeklyUsageUSD:  2,
		MonthlyUsageUSD: 3,
		Notes:           "bulk",
	})

	svc := NewSubscriptionService(planRepo, subRepo, &subscriptionKeyRepoStub{}, nil, nil, nil)
	before := time.Now()
	result, err := svc.BulkAssignSubscription(context.Background(), &BulkAssignSubscriptionInput{
		UserIDs:      []int64{4},
		PlanID:       1,
		ValidityDays: 7,
		Notes:        "bulk",
	})
	after := time.Now()

	require.NoError(t, err)
	require.Equal(t, 1, result.SuccessCount)
	require.Equal(t, 0, result.CreatedCount)
	require.Equal(t, 1, result.ReusedCount)
	require.Equal(t, "reused", result.Statuses[4])
	require.Len(t, result.Subscriptions, 1)
	renewed := result.Subscriptions[0]
	require.Equal(t, SubscriptionStatusActive, renewed.Status)
	require.False(t, renewed.StartsAt.Before(before))
	require.False(t, renewed.StartsAt.After(after))
	require.Equal(t, renewed.StartsAt.AddDate(0, 0, 7), renewed.ExpiresAt)
	require.Zero(t, renewed.DailyUsageUSD)
	require.Zero(t, renewed.WeeklyUsageUSD)
	require.Zero(t, renewed.MonthlyUsageUSD)
	require.Equal(t, "bulk", renewed.Notes)
}

func TestAssignSubscriptionKeepsWorkingWhenIdempotencyStoreUnavailable(t *testing.T) {
	planRepo := &subscriptionPlanRepoStub{plan: &SubscriptionPlan{ID: 1, Name: "plan"}}
	subRepo := newSubscriptionUserSubRepoStub()
	SetDefaultIdempotencyCoordinator(NewIdempotencyCoordinator(failingIdempotencyRepo{}, DefaultIdempotencyConfig()))
	t.Cleanup(func() {
		SetDefaultIdempotencyCoordinator(nil)
	})

	svc := NewSubscriptionService(planRepo, subRepo, &subscriptionKeyRepoStub{}, nil, nil, nil)
	sub, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       9001,
		PlanID:       1,
		ValidityDays: 30,
		Notes:        "new",
	})
	require.NoError(t, err)
	require.NotNil(t, sub)
	require.Equal(t, 1, subRepo.createCalls, "semantic idempotent endpoint should not depend on idempotency store availability")
}

func TestNormalizeAssignValidityDays(t *testing.T) {
	require.Equal(t, 30, normalizeAssignValidityDays(0))
	require.Equal(t, 30, normalizeAssignValidityDays(-5))
	require.Equal(t, MaxValidityDays, normalizeAssignValidityDays(MaxValidityDays+100))
	require.Equal(t, 7, normalizeAssignValidityDays(7))
}

func TestDetectAssignSemanticConflictCases(t *testing.T) {
	start := time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC)
	base := &UserSubscription{
		UserID:    1,
		PlanID:    1,
		StartsAt:  start,
		ExpiresAt: start.AddDate(0, 0, 30),
		Notes:     "same",
	}

	reason, conflict := detectAssignSemanticConflict(base, &AssignSubscriptionInput{
		UserID:       1,
		PlanID:       1,
		ValidityDays: 30,
		Notes:        "same",
	})
	require.False(t, conflict)
	require.Equal(t, "", reason)

	reason, conflict = detectAssignSemanticConflict(base, &AssignSubscriptionInput{
		UserID:       1,
		PlanID:       1,
		ValidityDays: 60,
		Notes:        "same",
	})
	require.True(t, conflict)
	require.Equal(t, "validity_days_mismatch", reason)

	reason, conflict = detectAssignSemanticConflict(base, &AssignSubscriptionInput{
		UserID:       1,
		PlanID:       1,
		ValidityDays: 30,
		Notes:        "other",
	})
	require.True(t, conflict)
	require.Equal(t, "notes_mismatch", reason)
}

func TestAssignSubscriptionPlanNotFound(t *testing.T) {
	planRepo := &subscriptionPlanRepoStub{plan: nil}
	subRepo := newSubscriptionUserSubRepoStub()
	svc := NewSubscriptionService(planRepo, subRepo, &subscriptionKeyRepoStub{}, nil, nil, nil)

	_, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       1,
		PlanID:       1,
		ValidityDays: 30,
	})
	require.Error(t, err)
	require.Equal(t, infraerrors.Code(ErrPlanNotFound), infraerrors.Code(err))
	require.Equal(t, 0, subRepo.createCalls)
}

func strconvFormatInt(v int64) string {
	return strconv.FormatInt(v, 10)
}

func infraerrorsReason(err error) string {
	return infraerrors.Reason(err)
}
