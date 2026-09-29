package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// /v1beta/models 只列目录里上架的模型。
func TestGeminiV1BetaListModels_FiltersFallbackByCatalog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &GatewayHandler{modelCatalog: listedCatalogStub{ids: []string{"gemini-2.5-pro"}}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{})

	h.GeminiV1BetaListModels(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var got struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Models, 1)
	require.Equal(t, "models/gemini-2.5-pro", got.Models[0].Name)
}

// 白名单关闭时 /antigravity/models 只按目录过滤。
func TestAntigravityModels_FiltersByCatalog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &GatewayHandler{modelCatalog: listedCatalogStub{ids: []string{"gemini-2.5-flash", "gemini-2.5-flash-thinking"}}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/antigravity/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{})

	h.AntigravityModels(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var got struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Data, 2)
	require.Equal(t, "gemini-2.5-flash", got.Data[0].ID)
	require.Equal(t, "gemini-2.5-flash-thinking", got.Data[1].ID)
}
