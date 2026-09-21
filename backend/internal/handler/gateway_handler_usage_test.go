package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 订阅 key 的 /v1/usage：套餐名与限额来自套餐，窗口起点来自订阅。
func TestUsageUnrestrictedSubscriptionKeyUsesPlan(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/usage", nil)

	weeklyLimit := 42.0
	weeklyWindowStart := time.Date(2026, time.July, 13, 0, 30, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	c.Set(string(middleware.ContextKeySubscription), &service.UserSubscription{
		WeeklyWindowStart: &weeklyWindowStart,
		WeeklyUsageUSD:    2,
		Plan:              &service.SubscriptionPlan{ID: 9, Name: "Weekly plan", WeeklyLimitUSD: &weeklyLimit},
	})

	subscriptionID := int64(55)
	handler := &GatewayHandler{}
	handler.usageUnrestricted(
		c,
		context.Background(),
		&service.APIKey{SubscriptionID: &subscriptionID},
		middleware.AuthSubject{},
		nil,
		nil,
		nil,
	)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		IsValid      bool    `json:"isValid"`
		PlanName     string  `json:"planName"`
		Remaining    float64 `json:"remaining"`
		Subscription struct {
			WeeklyWindowStart *time.Time `json:"weekly_window_start"`
			WeeklyLimitUSD    *float64   `json:"weekly_limit_usd"`
		} `json:"subscription"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.IsValid)
	require.Equal(t, "Weekly plan", response.PlanName)
	require.Equal(t, 40.0, response.Remaining)
	require.NotNil(t, response.Subscription.WeeklyWindowStart)
	require.True(t, weeklyWindowStart.Equal(*response.Subscription.WeeklyWindowStart))
	require.NotNil(t, response.Subscription.WeeklyLimitUSD)
	require.Equal(t, weeklyLimit, *response.Subscription.WeeklyLimitUSD)
}

// 订阅已失效（ctx 无订阅）时：isValid=false，不再解引用分组。
func TestUsageUnrestrictedSubscriptionKeyInactive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/usage", nil)

	subscriptionID := int64(55)
	handler := &GatewayHandler{}
	handler.usageUnrestricted(c, context.Background(), &service.APIKey{SubscriptionID: &subscriptionID}, middleware.AuthSubject{}, nil, nil, nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		IsValid  bool   `json:"isValid"`
		PlanName string `json:"planName"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.False(t, response.IsValid)
	require.Equal(t, "", response.PlanName)
}
