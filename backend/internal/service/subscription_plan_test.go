//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSubscriptionPlan_Covers(t *testing.T) {
	plan := &SubscriptionPlan{Models: []SubscriptionPlanModel{{EntryID: 199, ModelID: "gpt-5.6"}, {EntryID: 57}}}
	require.True(t, plan.Covers(199))
	require.True(t, plan.Covers(57))
	require.False(t, plan.Covers(27))
	require.Equal(t, []int64{199, 57}, plan.EntryIDs())

	empty := &SubscriptionPlan{}
	require.False(t, empty.Covers(199), "空模型集什么都不覆盖（没有隐含的「全部」）")
	require.Empty(t, empty.EntryIDs())
}

func TestSubscriptionPlan_HasLimits(t *testing.T) {
	zero, positive := 0.0, 1.5
	require.False(t, (&SubscriptionPlan{}).HasDailyLimit())
	require.False(t, (&SubscriptionPlan{DailyLimitUSD: &zero}).HasDailyLimit(), "0 = 不限")
	require.True(t, (&SubscriptionPlan{DailyLimitUSD: &positive}).HasDailyLimit())
	require.True(t, (&SubscriptionPlan{WeeklyLimitUSD: &positive}).HasWeeklyLimit())
	require.True(t, (&SubscriptionPlan{MonthlyLimitUSD: &positive}).HasMonthlyLimit())
}

func TestSubscriptionCoversRoute(t *testing.T) {
	route := CatalogRoute{EntryID: 199, RequestedModel: "gpt-5.6"}
	require.True(t, SubscriptionCoversRoute(nil, route), "余额 key 没有订阅，恒放行")
	inPlan := &UserSubscription{Plan: &SubscriptionPlan{Models: []SubscriptionPlanModel{{EntryID: 199}}}}
	outOfPlan := &UserSubscription{Plan: &SubscriptionPlan{Models: []SubscriptionPlanModel{{EntryID: 27}}}}
	require.True(t, SubscriptionCoversRoute(inPlan, route))
	require.False(t, SubscriptionCoversRoute(outOfPlan, route))
}
