package service

// 利润控制请求路径矩阵测试：证明所有文本调度路径都经过利润准入过滤，
// 且任何 fallback 都不能把已排除账号重新放回候选。
// 覆盖：高级调度器候选池（openai_profit_control_test.go）、legacy 引擎、
// previous_response WSv2 粘连（跳过复用但保留绑定 + 倍率恢复重粘连）、
// failover 排除不回收、抢槽后终检、倍率恢复重新准入、
// 用户覆盖倍率 D、composite 计费分组与调度分组分离。

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"

	"github.com/stretchr/testify/require"
)

// D 取认证用户的倍率（ctx 里由认证中间件放入），分组倍率不参与；无用户身份（内部调用）按 1。
func TestProfitControl_GateUsesUserRateMultiplier(t *testing.T) {
	svc := &OpenAIGatewayService{}
	groupID := int64(7)
	group := profitControlTestGroup(groupID, 0, 0)
	group.RateMultiplier = 2.0

	ctx := WithUserRateMultiplier(profitControlTestCtx(group), &User{ID: 42, RateMultiplier: 0.5})
	gate := svc.resolveOpenAIProfitControlGate(ctx, &groupID)
	require.NotNil(t, gate)
	require.InDelta(t, 0.5, gate.threshold, 1e-12, "阈值必须基于用户倍率 0.5，而不是分组 2.0")

	// 无用户身份（内部调用）按 1。
	gate = svc.resolveOpenAIProfitControlGate(context.WithValue(context.Background(), ctxkey.Group, group), &groupID)
	require.NotNil(t, gate)
	require.InDelta(t, 1.0, gate.threshold, 1e-12)
}

type profitControlGroupRepo struct {
	GroupRepository
	group *Group
}

func (r profitControlGroupRepo) GetByIDLite(context.Context, int64) (*Group, error) {
	return r.group, nil
}

// GetByID 故意 panic：利润门只需要分组配置，不需要 GetByID 附带的账号计数
// 聚合查询。装门走 GetByID 会在 composite/模型路由/fallback 的每次装门（WS
// 每 turn 一次）上多打一条聚合，且发生在「是否启用利润控制」判定之前。
func (r profitControlGroupRepo) GetByID(context.Context, int64) (*Group, error) {
	panic("profit control gate must read groups via GetByIDLite (no account-count aggregation)")
}

// composite 路由：门配置（margin）取被调度成员分组，D 仍是用户倍率。
func TestProfitControl_CompositeUsesMemberGroupMarginAndUserRate(t *testing.T) {
	memberGroupID := int64(7)
	memberGroup := profitControlTestGroup(memberGroupID, 0.5, 0)
	memberGroup.RateMultiplier = 99 // 分组倍率已无效

	billingGroup := &Group{
		ID:             1001,
		Platform:       PlatformComposite,
		Status:         StatusActive,
		Hydrated:       true,
		RateMultiplier: 1.0,
	}
	svc := &OpenAIGatewayService{
		schedulerSnapshot: &SchedulerSnapshotService{groupRepo: profitControlGroupRepo{group: memberGroup}},
	}

	ctx := WithUserRateMultiplier(profitControlTestCtx(billingGroup), &User{ID: 1, RateMultiplier: 1.0})
	gate := svc.resolveOpenAIProfitControlGate(ctx, &memberGroupID)
	require.NotNil(t, gate)
	require.InDelta(t, 0.5, gate.threshold, 1e-12, "D = 用户倍率 1.0，margin 取成员分组 0.5")
}
