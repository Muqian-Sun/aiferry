//go:build unit

package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 后台新建用户表单直接显示的默认值，必须就是注册 / 建号实际用的那几个常量
func TestGetNewUserDefaultsReturnsSiteFeatureConstants(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewUserHandler(nil, nil, nil, nil, nil, nil)
	router.GET("/admin/users/defaults", handler.GetNewUserDefaults)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/users/defaults", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	var body struct {
		Code int `json:"code"`
		Data struct {
			Balance        float64 `json:"balance"`
			Concurrency    int     `json:"concurrency"`
			RPMLimit       int     `json:"rpm_limit"`
			RateMultiplier float64 `json:"rate_multiplier"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, 0, body.Code)
	require.Equal(t, service.NewUserBalance, body.Data.Balance)
	require.Equal(t, service.NewUserConcurrency, body.Data.Concurrency)
	require.Equal(t, service.NewUserRPMLimit, body.Data.RPMLimit)
	require.InDelta(t, service.NewUserRateMultiplier, body.Data.RateMultiplier, 1e-12)
}
