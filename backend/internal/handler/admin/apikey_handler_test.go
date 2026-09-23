package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupAPIKeyHandler(adminSvc service.AdminService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := NewAdminAPIKeyHandler(adminSvc)
	router.PUT("/api/v1/admin/api-keys/:id", h.UpdateKey)
	return router
}

func TestAdminAPIKeyHandler_ResetRateLimitUsage(t *testing.T) {
	svc := newStubAdminService()
	now := time.Now()
	svc.apiKeys[0].Usage5h = 1.2
	svc.apiKeys[0].Usage1d = 3.4
	svc.apiKeys[0].Usage7d = 5.6
	svc.apiKeys[0].Window5hStart = &now
	svc.apiKeys[0].Window1dStart = &now
	svc.apiKeys[0].Window7dStart = &now
	router := setupAPIKeyHandler(svc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/api-keys/10", bytes.NewBufferString(`{"reset_rate_limit_usage":true}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Data struct {
			APIKey struct {
				Usage5h       float64    `json:"usage_5h"`
				Usage1d       float64    `json:"usage_1d"`
				Usage7d       float64    `json:"usage_7d"`
				Window5hStart *time.Time `json:"window_5h_start"`
				Window1dStart *time.Time `json:"window_1d_start"`
				Window7dStart *time.Time `json:"window_7d_start"`
			} `json:"api_key"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Zero(t, resp.Data.APIKey.Usage5h)
	require.Zero(t, resp.Data.APIKey.Usage1d)
	require.Zero(t, resp.Data.APIKey.Usage7d)
	require.Nil(t, resp.Data.APIKey.Window5hStart)
	require.Nil(t, resp.Data.APIKey.Window1dStart)
	require.Nil(t, resp.Data.APIKey.Window7dStart)
}
