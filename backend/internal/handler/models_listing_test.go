//go:build unit

package handler

import (
	"context"
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

// 用户拿自己的 key 查模型（2026-09-29）：三种形状都列目录里上架的模型；Anthropic 形状与官方一致，
// Gemini 不再转发上游。

type entriesCatalogStub struct{ entries []service.ModelCatalogEntry }

func (s entriesCatalogStub) ListListedEntries(context.Context) []service.ModelCatalogEntry {
	return append([]service.ModelCatalogEntry(nil), s.entries...)
}

func (s entriesCatalogStub) ResolveRoute(context.Context, string) (service.CatalogRoute, bool) {
	return service.CatalogRoute{}, false
}

var modelsListingCreated = time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)

func modelsListingHandler() *GatewayHandler {
	return &GatewayHandler{modelCatalog: entriesCatalogStub{entries: []service.ModelCatalogEntry{
		{ID: 1, ModelID: "claude-sonnet-4-6", DisplayName: "Claude Sonnet 4.6", Vendor: "anthropic", Status: service.ModelCatalogStatusListed, CreatedAt: modelsListingCreated},
		{ID: 2, ModelID: "deepseek-chat", Vendor: "deepseek", Status: service.ModelCatalogStatusListed, CreatedAt: modelsListingCreated},
	}}}
}

func modelsListingRequest(t *testing.T, path string, params gin.Params, header map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range header {
		c.Request.Header.Set(k, v)
	}
	c.Params = params
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 1})
	return c, rec
}

func TestModels_OpenAIShapeUsesCatalogVendorAndTime(t *testing.T) {
	c, rec := modelsListingRequest(t, "/v1/models", nil, nil)
	modelsListingHandler().Models(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var got struct {
		Object string `json:"object"`
		Data   []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
			Created int64  `json:"created"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "list", got.Object)
	require.Len(t, got.Data, 2)
	require.Equal(t, "claude-sonnet-4-6", got.Data[0].ID)
	require.Equal(t, "anthropic", got.Data[0].OwnedBy, "owned_by 取目录厂商，不再一律 openai")
	require.Equal(t, modelsListingCreated.Unix(), got.Data[0].Created)
}

func TestModels_AnthropicShapeMatchesOfficialAPI(t *testing.T) {
	c, rec := modelsListingRequest(t, "/v1/models", nil, map[string]string{"anthropic-version": "2023-06-01"})
	modelsListingHandler().Models(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.NotContains(t, got, "object", "Anthropic 列表没有 object 字段")
	require.JSONEq(t, `false`, string(got["has_more"]))
	require.JSONEq(t, `"claude-sonnet-4-6"`, string(got["first_id"]))
	require.JSONEq(t, `"deepseek-chat"`, string(got["last_id"]))
	require.JSONEq(t, `[
		{"id":"claude-sonnet-4-6","type":"model","display_name":"Claude Sonnet 4.6","created_at":"2026-09-01T08:00:00Z"},
		{"id":"deepseek-chat","type":"model","display_name":"deepseek-chat","created_at":"2026-09-01T08:00:00Z"}
	]`, string(got["data"]))

	c, rec = modelsListingRequest(t, "/v1/models/claude-sonnet-4-6", gin.Params{{Key: "model", Value: "claude-sonnet-4-6"}}, map[string]string{"anthropic-version": "2023-06-01"})
	modelsListingHandler().Models(c)
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"id":"claude-sonnet-4-6","type":"model","display_name":"Claude Sonnet 4.6","created_at":"2026-09-01T08:00:00Z"}`, rec.Body.String())

	c, rec = modelsListingRequest(t, "/v1/models/nope", gin.Params{{Key: "model", Value: "nope"}}, map[string]string{"anthropic-version": "2023-06-01"})
	modelsListingHandler().Models(c)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.JSONEq(t, `{"type":"error","error":{"type":"not_found_error","message":"model: nope"}}`, rec.Body.String())
}

// Gemini 形状列全部上架模型（网关能把任一模型转成 Gemini 协议），handler 没装 Gemini 服务也能回——不再转发上游。
func TestGeminiV1BetaModels_ListFromCatalogWithoutUpstream(t *testing.T) {
	c, rec := modelsListingRequest(t, "/v1beta/models", nil, nil)
	modelsListingHandler().GeminiV1BetaListModels(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.JSONEq(t, `{"models":[
		{"name":"models/claude-sonnet-4-6","displayName":"Claude Sonnet 4.6","supportedGenerationMethods":["generateContent","streamGenerateContent"]},
		{"name":"models/deepseek-chat","displayName":"deepseek-chat","supportedGenerationMethods":["generateContent","streamGenerateContent"]}
	]}`, rec.Body.String())

	c, rec = modelsListingRequest(t, "/v1beta/models/claude-sonnet-4-6", gin.Params{{Key: "model", Value: "claude-sonnet-4-6"}}, nil)
	modelsListingHandler().GeminiV1BetaGetModel(c)
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"name":"models/claude-sonnet-4-6","displayName":"Claude Sonnet 4.6","supportedGenerationMethods":["generateContent","streamGenerateContent"]}`, rec.Body.String())

	c, rec = modelsListingRequest(t, "/v1beta/models/nope", gin.Params{{Key: "model", Value: "nope"}}, nil)
	modelsListingHandler().GeminiV1BetaGetModel(c)
	require.Equal(t, http.StatusNotFound, rec.Code)
}
