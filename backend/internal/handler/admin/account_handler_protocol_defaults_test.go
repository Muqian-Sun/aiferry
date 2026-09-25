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

// 建第三方 key 不选平台（muqian 2026-09-25）：前端按地址提示厂商，官方域名表随官方地址一起下发，
// 与后端 OfficialVendorOfURL 同一张表（含国际站等预填表之外的官方站点）。
func TestAccountHandler_GetProtocolDefaults_VendorHosts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewAccountHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router.GET("/api/v1/admin/accounts/protocol-defaults", handler.GetProtocolDefaults)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/protocol-defaults", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Data struct {
			Defaults    map[string]map[string]map[string]string `json:"defaults"`
			VendorHosts map[string]string                       `json:"vendor_hosts"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotEmpty(t, body.Data.Defaults)
	require.Equal(t, service.OfficialVendorHosts(), body.Data.VendorHosts)
	require.Equal(t, service.PlatformKimi, body.Data.VendorHosts["api.kimi.com"])
	require.Equal(t, service.PlatformKimi, body.Data.VendorHosts["api.moonshot.ai"], "国际站不在预填表里，也要下发")
	require.Equal(t, service.PlatformOpenAI, body.Data.VendorHosts["api.openai.com"])
	for host, vendor := range body.Data.VendorHosts {
		require.Equal(t, vendor, service.OfficialVendorOfURL("https://"+host), host)
	}
}
