//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/modelcatalogentry"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionplan"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/stretchr/testify/require"
)

// 套餐仓储走真实 tx（主表 + 模型集同事务、外键冲突翻译），所以不用 testEntTx 的事务隔离：
// 外键冲突会把外层事务打成 aborted，之后什么都查不了。用非事务 client + 按前缀清理。

type planRepoFixture struct {
	t      *testing.T
	ctx    context.Context
	client *dbent.Client
	repo   service.SubscriptionPlanRepository
	prefix string
}

func newPlanRepoFixture(t *testing.T) *planRepoFixture {
	t.Helper()
	client := testEntClient(t)
	f := &planRepoFixture{t: t, ctx: context.Background(), client: client,
		repo: NewSubscriptionPlanRepository(client), prefix: fmt.Sprintf("planrepo-%d", time.Now().UnixNano())}
	t.Cleanup(func() {
		ctx := context.Background()
		// 订阅行引用套餐（RESTRICT）：先删订阅再删套餐；模型集随套餐 CASCADE
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM user_subscriptions WHERE plan_id IN (SELECT id FROM subscription_plans WHERE name LIKE $1)", f.prefix+"%")
		_, _ = client.SubscriptionPlan.Delete().Where(subscriptionplan.NameHasPrefix(f.prefix)).Exec(ctx)
		_, _ = client.ModelCatalogEntry.Delete().Where(modelcatalogentry.ModelIDHasPrefix(f.prefix)).Exec(ctx)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM users WHERE email LIKE $1", f.prefix+"%")
	})
	return f
}

func (f *planRepoFixture) name(s string) string { return f.prefix + "-" + s }

func (f *planRepoFixture) entry(model, display string) int64 {
	f.t.Helper()
	e, err := f.client.ModelCatalogEntry.Create().
		SetModelID(f.name(model)).SetDisplayName(display).
		SetStatus(service.ModelCatalogStatusListed).SetManagedBy(service.ModelCatalogManagedByAdmin).
		Save(f.ctx)
	require.NoError(f.t, err, "create catalog entry")
	return e.ID
}

func (f *planRepoFixture) plan(name string, entryIDs ...int64) *service.SubscriptionPlan {
	f.t.Helper()
	models := make([]service.SubscriptionPlanModel, 0, len(entryIDs))
	for _, id := range entryIDs {
		models = append(models, service.SubscriptionPlanModel{EntryID: id})
	}
	limit := 2.5
	p := &service.SubscriptionPlan{Name: f.name(name), Price: 9.9, ValidityDays: 30, ValidityUnit: "day", ForSale: true,
		DailyLimitUSD: &limit, Models: models}
	require.NoError(f.t, f.repo.Create(f.ctx, p))
	require.NotZero(f.t, p.ID)
	return p
}

// 建套餐时模型集同事务落库；GetByID 回读带条目名
func TestSubscriptionPlanRepo_CreateWithModels_GetByIDLoadsNames(t *testing.T) {
	f := newPlanRepoFixture(t)
	e1 := f.entry("gpt", "GPT 5.6")
	e2 := f.entry("sonnet", "Sonnet 4")
	created := f.plan("pro", e1, e2)

	got, err := f.repo.GetByID(f.ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.Name, got.Name)
	require.NotNil(t, got.DailyLimitUSD)
	require.InDelta(t, 2.5, *got.DailyLimitUSD, 1e-9)
	require.Nil(t, got.WeeklyLimitUSD)
	require.ElementsMatch(t, []int64{e1, e2}, got.EntryIDs())
	byEntry := map[int64]service.SubscriptionPlanModel{}
	for _, m := range got.Models {
		byEntry[m.EntryID] = m
	}
	require.Equal(t, "GPT 5.6", byEntry[e1].DisplayName)
	require.Equal(t, f.name("gpt"), byEntry[e1].ModelID)
	require.Equal(t, "Sonnet 4", byEntry[e2].DisplayName)

	// List 也带模型集；forSaleOnly 过滤
	plans, err := f.repo.List(f.ctx, true)
	require.NoError(t, err)
	var found *service.SubscriptionPlan
	for i := range plans {
		if plans[i].ID == created.ID {
			found = &plans[i]
		}
	}
	require.NotNil(t, found)
	require.Len(t, found.Models, 2)

	_, err = f.repo.GetByID(f.ctx, -1)
	require.ErrorIs(t, err, service.ErrPlanNotFound)
}

// Update 整份覆盖模型集，nil 限额清成 NULL
func TestSubscriptionPlanRepo_Update_ReplacesModels(t *testing.T) {
	f := newPlanRepoFixture(t)
	e1 := f.entry("a", "A")
	e2 := f.entry("b", "B")
	e3 := f.entry("c", "C")
	p := f.plan("pro", e1, e2)

	p.Name = f.name("pro-renamed")
	p.DailyLimitUSD = nil
	weekly := 7.0
	p.WeeklyLimitUSD = &weekly
	p.Models = []service.SubscriptionPlanModel{{EntryID: e3}}
	require.NoError(t, f.repo.Update(f.ctx, p))

	got, err := f.repo.GetByID(f.ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, f.name("pro-renamed"), got.Name)
	require.Nil(t, got.DailyLimitUSD, "nil 限额清成 NULL")
	require.InDelta(t, 7.0, *got.WeeklyLimitUSD, 1e-9)
	require.Equal(t, []int64{e3}, got.EntryIDs(), "旧模型集整份被覆盖")
}

// 引用不存在的目录条目：PLAN_MODEL_NOT_FOUND，且主表不留孤行（同事务回滚）
func TestSubscriptionPlanRepo_Create_UnknownEntry_ErrPlanModelNotFound(t *testing.T) {
	f := newPlanRepoFixture(t)
	p := &service.SubscriptionPlan{Name: f.name("bad"), Price: 9.9, ValidityDays: 30, ValidityUnit: "day",
		Models: []service.SubscriptionPlanModel{{EntryID: -404}}}
	err := f.repo.Create(f.ctx, p)
	require.ErrorIs(t, err, service.ErrPlanModelNotFound)

	n, err := f.client.SubscriptionPlan.Query().Where(subscriptionplan.NameEQ(f.name("bad"))).Count(f.ctx)
	require.NoError(t, err)
	require.Zero(t, n, "主表与模型集同事务：条目不存在时套餐行也不能留下")
}

// 有订阅行引用（哪怕已软删）→ PLAN_IN_USE（外键 RESTRICT）
func TestSubscriptionPlanRepo_Delete_ReferencedBySoftDeletedSubscription_ErrPlanInUse(t *testing.T) {
	f := newPlanRepoFixture(t)
	e1 := f.entry("a", "A")
	p := f.plan("used", e1)
	user := mustCreateUser(t, f.client, &service.User{Email: f.name("u") + "@test.com", Role: service.RoleUser, Status: service.StatusActive})
	sub := mustCreateSubscription(t, f.client, &service.UserSubscription{UserID: user.ID, PlanID: p.ID})
	_, err := integrationDB.ExecContext(f.ctx, "UPDATE user_subscriptions SET deleted_at = NOW() WHERE id = $1", sub.ID)
	require.NoError(t, err)

	err = f.repo.Delete(f.ctx, p.ID)
	require.ErrorIs(t, err, service.ErrPlanInUse)
	_, err = f.repo.GetByID(f.ctx, p.ID)
	require.NoError(t, err, "套餐仍在")
}

// 无引用：删掉，模型集随之级联；再删 → PLAN_NOT_FOUND
func TestSubscriptionPlanRepo_Delete_Unreferenced_OK(t *testing.T) {
	f := newPlanRepoFixture(t)
	e1 := f.entry("a", "A")
	p := f.plan("free", e1)

	require.NoError(t, f.repo.Delete(f.ctx, p.ID))
	_, err := f.repo.GetByID(f.ctx, p.ID)
	require.ErrorIs(t, err, service.ErrPlanNotFound)
	var n int
	require.NoError(t, integrationDB.QueryRowContext(f.ctx, "SELECT COUNT(*) FROM subscription_plan_models WHERE plan_id = $1", p.ID).Scan(&n))
	require.Zero(t, n, "模型集随套餐级联删除")
	require.ErrorIs(t, f.repo.Delete(f.ctx, p.ID), service.ErrPlanNotFound)
}
