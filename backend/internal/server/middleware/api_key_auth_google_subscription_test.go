package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Google 鉴权同构：订阅无效 403 PERMISSION_DENIED；基础设施错误 503 UNAVAILABLE。

func serveGoogleSubscriptionKey(t *testing.T, getActiveByID func(context.Context, int64) (*service.UserSubscription, error)) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	subscriptionID := int64(601)
	apiKey := &service.APIKey{
		ID: 501, UserID: 999, Key: "google-sub", Status: service.StatusActive,
		User:           &service.User{ID: 999, Role: service.RoleUser, Status: service.StatusActive, Balance: 10, Concurrency: 3},
		SubscriptionID: &subscriptionID,
	}
	apiKeyService := newTestAPIKeyService(fakeAPIKeyRepo{
		getByKey: func(_ context.Context, key string) (*service.APIKey, error) {
			if key != apiKey.Key {
				return nil, service.ErrAPIKeyNotFound
			}
			clone := *apiKey
			return &clone, nil
		},
	})
	cfg := &config.Config{RunMode: config.RunModeStandard}
	subscriptionService := service.NewSubscriptionService(&stubPlanRepo{plan: &service.SubscriptionPlan{ID: 7, Name: "plan"}},
		fakeGoogleSubscriptionRepo{getActiveByID: getActiveByID}, nil, nil, nil, cfg)
	t.Cleanup(subscriptionService.Stop)

	r := gin.New()
	r.Use(APIKeyAuthWithSubscriptionGoogle(apiKeyService, subscriptionService, cfg))
	r.GET("/v1beta/test", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	req := httptest.NewRequest(http.MethodGet, "/v1beta/test", nil)
	req.Header.Set("x-goog-api-key", apiKey.Key)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestApiKeyAuthWithSubscriptionGoogle_SubscriptionKeyMissingSubscription403(t *testing.T) {
	rec := serveGoogleSubscriptionKey(t, func(context.Context, int64) (*service.UserSubscription, error) {
		return nil, service.ErrSubscriptionNotFound
	})
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	var resp googleErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "PERMISSION_DENIED", resp.Error.Status)
	require.Equal(t, "Subscription is not active", resp.Error.Message)
}

func TestApiKeyAuthWithSubscriptionGoogle_SubscriptionLookupInfraError503(t *testing.T) {
	rec := serveGoogleSubscriptionKey(t, func(context.Context, int64) (*service.UserSubscription, error) {
		return nil, errors.New("redis down")
	})
	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	var resp googleErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, http.StatusServiceUnavailable, resp.Error.Code)
	require.Equal(t, "Subscription service unavailable", resp.Error.Message)
}
