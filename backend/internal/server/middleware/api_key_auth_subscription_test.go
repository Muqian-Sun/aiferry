//go:build unit

package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 订阅 key 鉴权：按 apiKey.SubscriptionID 取订阅；订阅无效 403、基础设施错误 503；余额 key 完全不碰订阅服务。

type subscriptionAuthFixture struct {
	apiKey     *service.APIKey
	apiKeyRepo *stubApiKeyRepo
	subCalls   atomic.Int64
}

func newSubscriptionAuthFixture(subscriptionID *int64, balance float64) *subscriptionAuthFixture {
	user := &service.User{ID: 7, Role: service.RoleUser, Status: service.StatusActive, Balance: balance, Concurrency: 3}
	apiKey := &service.APIKey{ID: 100, UserID: user.ID, Key: "sub-key", Status: service.StatusActive, User: user, SubscriptionID: subscriptionID}
	return &subscriptionAuthFixture{
		apiKey: apiKey,
		apiKeyRepo: &stubApiKeyRepo{
			getByKey: func(_ context.Context, key string) (*service.APIKey, error) {
				if key != apiKey.Key {
					return nil, service.ErrAPIKeyNotFound
				}
				clone := *apiKey
				return &clone, nil
			},
		},
	}
}

func (f *subscriptionAuthFixture) serve(t *testing.T, getActiveByID func(context.Context, int64) (*service.UserSubscription, error)) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{RunMode: config.RunModeStandard}
	apiKeyService := service.NewAPIKeyService(f.apiKeyRepo, nil, nil, nil, nil, cfg)
	subRepo := &stubUserSubscriptionRepo{
		getActiveByID: func(ctx context.Context, id int64) (*service.UserSubscription, error) {
			f.subCalls.Add(1)
			return getActiveByID(ctx, id)
		},
	}
	subscriptionService := service.NewSubscriptionService(&stubPlanRepo{plan: &service.SubscriptionPlan{ID: 42, Name: "sub"}}, subRepo, nil, nil, nil, cfg)
	t.Cleanup(subscriptionService.Stop)
	router := newAuthTestRouter(apiKeyService, subscriptionService, cfg)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	req.Header.Set("x-api-key", f.apiKey.Key)
	router.ServeHTTP(w, req)
	return w
}

// 订阅被撤销 / 过期 / 不存在：GetActiveByID 查不到 → 403 SUBSCRIPTION_INVALID（不是 503）
func TestAPIKeyAuth_SubscriptionKeyMissingSubscription403(t *testing.T) {
	subscriptionID := int64(55)
	f := newSubscriptionAuthFixture(&subscriptionID, 100) // 余额充足也无关：订阅 key 不看余额
	w := f.serve(t, func(context.Context, int64) (*service.UserSubscription, error) {
		return nil, service.ErrSubscriptionNotFound
	})
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"SUBSCRIPTION_INVALID"`)
	require.EqualValues(t, 1, f.subCalls.Load())
}

// Redis / DB 不通不是「你的订阅无效」：503 BILLING_SERVICE_UNAVAILABLE
func TestAPIKeyAuth_SubscriptionLookupInfraError503(t *testing.T) {
	subscriptionID := int64(55)
	f := newSubscriptionAuthFixture(&subscriptionID, 100)
	w := f.serve(t, func(context.Context, int64) (*service.UserSubscription, error) {
		return nil, errors.New("redis down")
	})
	require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"BILLING_SERVICE_UNAVAILABLE"`)
}

// 订阅有效：放行，ctx 挂上订阅（套餐已附上）
func TestAPIKeyAuth_SubscriptionKeyActivePasses(t *testing.T) {
	subscriptionID := int64(55)
	f := newSubscriptionAuthFixture(&subscriptionID, 0) // 余额 0 也放行：订阅 key 不看余额
	now := time.Now()
	w := f.serve(t, func(_ context.Context, id int64) (*service.UserSubscription, error) {
		return &service.UserSubscription{ID: id, UserID: 7, PlanID: 42, Status: service.SubscriptionStatusActive,
			ExpiresAt: now.Add(24 * time.Hour), DailyWindowStart: &now, WeeklyWindowStart: &now, MonthlyWindowStart: &now}, nil
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// 余额 key：余额 0 → 403 INSUFFICIENT_BALANCE，订阅仓储零调用
func TestAPIKeyAuth_BalanceKeyIgnoresSubscriptionService(t *testing.T) {
	f := newSubscriptionAuthFixture(nil, 0)
	w := f.serve(t, func(context.Context, int64) (*service.UserSubscription, error) {
		t.Fatal("balance key must not touch subscription repo")
		return nil, nil
	})
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"INSUFFICIENT_BALANCE"`)
	require.Zero(t, f.subCalls.Load())
}
