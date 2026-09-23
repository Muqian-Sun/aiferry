package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrPlanNotFound       = infraerrors.NotFound("PLAN_NOT_FOUND", "subscription plan not found")
	ErrPlanModelsRequired = infraerrors.BadRequest("PLAN_MODELS_REQUIRED", "plan must include at least one model")
	ErrPlanModelNotFound  = infraerrors.BadRequest("PLAN_MODEL_NOT_FOUND", "model catalog entry not found")
	ErrPlanInUse          = infraerrors.Conflict("PLAN_IN_USE", "plan is referenced by subscriptions")
)

// SubscriptionPlanModel 是套餐模型集里的一个目录条目（订阅 key 只能调用这些条目）。
type SubscriptionPlanModel struct {
	EntryID     int64
	ModelID     string
	DisplayName string
}

// SubscriptionPlan 套餐：售卖信息 + 自带的日 / 周 / 月限额 + 模型集。
// 订阅（UserSubscription）挂在套餐上，限额从这里读；不再与分组有任何关系。
type SubscriptionPlan struct {
	ID            int64
	Name          string
	Description   string
	Price         float64
	OriginalPrice *float64
	ValidityDays  int
	ValidityUnit  string
	Features      string
	ProductName   string
	ForSale       bool
	SortOrder     int

	// nil 或 <= 0 = 不限
	DailyLimitUSD   *float64
	WeeklyLimitUSD  *float64
	MonthlyLimitUSD *float64

	Models []SubscriptionPlanModel

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (p *SubscriptionPlan) HasDailyLimit() bool {
	return p.DailyLimitUSD != nil && *p.DailyLimitUSD > 0
}

func (p *SubscriptionPlan) HasWeeklyLimit() bool {
	return p.WeeklyLimitUSD != nil && *p.WeeklyLimitUSD > 0
}

func (p *SubscriptionPlan) HasMonthlyLimit() bool {
	return p.MonthlyLimitUSD != nil && *p.MonthlyLimitUSD > 0
}

// Covers 报告目录条目是否在套餐模型集里。
func (p *SubscriptionPlan) Covers(entryID int64) bool {
	for i := range p.Models {
		if p.Models[i].EntryID == entryID {
			return true
		}
	}
	return false
}

// EntryIDs 模型集的条目 ID（DTO 用）。
func (p *SubscriptionPlan) EntryIDs() []int64 {
	ids := make([]int64, 0, len(p.Models))
	for i := range p.Models {
		ids = append(ids, p.Models[i].EntryID)
	}
	return ids
}

// SubscriptionCoversRoute 报告订阅是否覆盖目录路由；无订阅（余额 key）恒为 true。
// 中间件（HTTP）与 WS 首帧 / 换模型共用这一处判定。
func SubscriptionCoversRoute(sub *UserSubscription, route CatalogRoute) bool {
	if sub == nil {
		return true
	}
	return sub.Plan.Covers(route.EntryID)
}

// SubscriptionPlanRepository 套餐仓储。单独抽出而不放进支付配置服务：
// 订阅热路径要按 ID 取套餐（限额 + 模型集），ent→领域转换只在 repository 包一份，
// 外键冲突在仓储翻译成业务错误。
type SubscriptionPlanRepository interface {
	// GetByID 带模型集；不存在 → ErrPlanNotFound
	GetByID(ctx context.Context, id int64) (*SubscriptionPlan, error)
	// List 按 sort_order 升序；forSaleOnly 只列在售
	List(ctx context.Context, forSaleOnly bool) ([]SubscriptionPlan, error)
	// Create 写主表与模型集（同一事务）；条目不存在 → ErrPlanModelNotFound；回填 plan.ID
	Create(ctx context.Context, plan *SubscriptionPlan) error
	// Update 整行回写并用 plan.Models 覆盖模型集（同一事务）
	Update(ctx context.Context, plan *SubscriptionPlan) error
	// Delete 硬删；仍被订阅行（含软删）引用 → ErrPlanInUse；不存在 → ErrPlanNotFound
	Delete(ctx context.Context, id int64) error
}
