package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newKeyBillingContext(apiKey *service.APIKey) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/sub2api/billing", nil)
	if apiKey != nil {
		c.Set(string(middleware2.ContextKeyAPIKey), apiKey)
	}
	return c, w
}

// 计费自省只报用户倍率：分组倍率 / 峰值都不再参与，无分组的 key 同样能查。
func TestGatewayHandlerKeyBillingInfoUsesUserMultiplier(t *testing.T) {
	c, w := newKeyBillingContext(&service.APIKey{
		UserID: 11,
		User:   &service.User{ID: 11, RateMultiplier: officialRate(0.8)},
	})
	(&GatewayHandler{}).KeyBillingInfo(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))

	var got keyBillingInfoResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, "sub2api.key_billing", got.Object)
	require.Equal(t, keyBillingInfoSchemaVersion, got.SchemaVersion)
	require.Equal(t, "token", got.BillingScope)
	require.Equal(t, 0.8, got.GroupRateMultiplier)
	require.Equal(t, 0.8, got.ResolvedRateMultiplier)
	require.Equal(t, 0.8, got.EffectiveRateMultiplier)
	require.Nil(t, got.UserRateMultiplier)
	require.False(t, got.PeakRateEnabled)
	require.Nil(t, got.AppliedPeakMultiplier)
	require.NotContains(t, w.Body.String(), `"peak_start"`)

	c, w = newKeyBillingContext(&service.APIKey{UserID: 12, User: &service.User{ID: 12, RateMultiplier: officialRate(1.5)}})
	(&GatewayHandler{}).KeyBillingInfo(c)
	require.Equal(t, http.StatusOK, w.Code, "ungrouped keys still report their multiplier")
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, 1.5, got.EffectiveRateMultiplier)
}

// 线上形状沿用 schema 1：原版 sub2api 的探针要求四个倍率字段齐全且 resolved 与 group 一致。
func TestBuildKeyBillingInfoKeepsSchemaOneShape(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	payload, err := json.Marshal(buildKeyBillingInfo(0.5, now))
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(payload, &got))
	for _, key := range []string{"group_rate_multiplier", "resolved_rate_multiplier", "effective_rate_multiplier"} {
		require.Equal(t, 0.5, got[key], key)
	}
	require.Equal(t, false, got["peak_rate_enabled"])
	require.Equal(t, float64(1), got["schema_version"])
	require.Equal(t, "2026-09-20T12:00:00Z", got["observed_at"])
	require.NotContains(t, got, "user_rate_multiplier")
}

func TestGatewayHandlerKeyBillingInfoErrorsAreSafe(t *testing.T) {
	t.Run("missing API key", func(t *testing.T) {
		c, w := newKeyBillingContext(nil)
		(&GatewayHandler{}).KeyBillingInfo(c)
		require.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("API key without user", func(t *testing.T) {
		c, w := newKeyBillingContext(&service.APIKey{UserID: 11})
		(&GatewayHandler{}).KeyBillingInfo(c)
		require.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
