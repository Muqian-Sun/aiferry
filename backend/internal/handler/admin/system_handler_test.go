//go:build unit

package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 在线更新已下线：版本接口只回构建时注入的版本号，不再查询任何远端 release。
func TestSystemHandlerGetVersionReturnsBuildVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewSystemHandler("1.0.0")

	router := gin.New()
	router.GET("/api/v1/admin/system/version", handler.GetVersion)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/version", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Code int `json:"code"`
		Data struct {
			Version string `json:"version"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 0, body.Code)
	require.Equal(t, "1.0.0", body.Data.Version)
}
