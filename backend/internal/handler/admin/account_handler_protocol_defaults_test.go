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
	handler := NewAccountHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router.GET("/api/v1/admin/accounts/protocol-defaults", handler.GetProtocolDefaults)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/protocol-defaults", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Data struct {
			Protocols        []string                                `json:"protocols"`
			Defaults         map[string]map[string]map[string]string `json:"defaults"`
			VendorHosts      map[string]string                       `json:"vendor_hosts"`
			VideoVendorHosts map[string]string                       `json:"video_vendor_hosts"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotEmpty(t, body.Data.Defaults)
	require.Equal(t, service.OfficialVendorHosts(), body.Data.VendorHosts)
	require.Equal(t, service.PlatformKimi, body.Data.VendorHosts["api.kimi.com"])
	require.Equal(t, service.PlatformKimi, body.Data.VendorHosts["api.moonshot.ai"], "国际站不在预填表里，也要下发")
	// 海外四家不再有官方 key（2026-09-29）：既不预填官方地址，也不按地址提示厂商。
	for _, host := range []string{"api.anthropic.com", "api.openai.com", "generativelanguage.googleapis.com", "api.x.ai"} {
		require.NotContains(t, body.Data.VendorHosts, host)
	}
	for _, platform := range []string{service.PlatformAnthropic, service.PlatformOpenAI, service.PlatformGemini, service.PlatformGrok} {
		require.NotContains(t, body.Data.Defaults, platform)
	}
	require.Contains(t, body.Data.Defaults, service.PlatformDeepseek)
	for host, vendor := range body.Data.VendorHosts {
		require.Equal(t, vendor, service.OfficialVendorOfURL("https://"+host), host)
	}

	// 官方视频 key：video 协议可选，各厂商按站点（cn / intl）预填，video 地址按单独的域名表认厂商。
	require.Contains(t, body.Data.Protocols, service.APIProtocolVideo)
	require.Equal(t, map[string]map[string]string{
		"cn":   {service.APIProtocolVideo: service.DefaultSeedanceCNBaseURL},
		"intl": {service.APIProtocolVideo: service.DefaultSeedanceIntlBaseURL},
	}, body.Data.Defaults[service.PlatformSeedance])
	require.Equal(t, map[string]map[string]string{
		"cn":   {service.APIProtocolVideo: service.DefaultKlingCNBaseURL},
		"intl": {service.APIProtocolVideo: service.DefaultKlingIntlBaseURL},
	}, body.Data.Defaults[service.PlatformKling])
	require.Equal(t, map[string]string{service.APIProtocolVideo: service.DefaultMiniMaxVideoCNBaseURL}, body.Data.Defaults[service.PlatformMiniMax]["cn"])
	require.Equal(t, map[string]string{service.APIProtocolVideo: service.DefaultMiniMaxVideoIntlBaseURL}, body.Data.Defaults[service.PlatformMiniMax]["intl"])
	require.Contains(t, body.Data.Defaults[service.PlatformMiniMax]["default"], service.APIProtocolChatCompletions)
	require.Equal(t, service.OfficialVideoVendorHosts(), body.Data.VideoVendorHosts)
	require.Equal(t, service.PlatformSeedance, body.Data.VideoVendorHosts["ark.cn-beijing.volces.com"])
	require.NotContains(t, body.Data.VendorHosts, "ark.cn-beijing.volces.com", "方舟对话 key 仍按中转")
}
