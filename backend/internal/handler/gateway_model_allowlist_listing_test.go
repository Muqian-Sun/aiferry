package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 通配条目在 /v1/models 中展开为候选来源中所有匹配项，保持来源顺序。
// geminiAllowlistAccountRepoStub 让 Gemini 兼容层选不到账号，触发回落静态列表。
type geminiAllowlistAccountRepoStub struct {
	service.AccountRepository
}

func (s *geminiAllowlistAccountRepoStub) ListSchedulingCandidatesByCatalogEntry(context.Context, int64) ([]service.Account, error) {
	return nil, nil
}

// 无模型端点的池 = 全部资源（PR-7a）：只有一个 antigravity 成品号，Gemini 选号失败后回落静态模型列表。
func (s *geminiAllowlistAccountRepoStub) ListSchedulingCandidates(context.Context, []string) ([]service.Account, error) {
	return []service.Account{{ID: 2, Platform: service.PlatformAntigravity, Status: service.StatusActive, Schedulable: true}}, nil
}

// 白名单关闭时只按目录过滤。
func TestGeminiV1BetaListModels_FiltersFallbackByCatalog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &GatewayHandler{
		geminiCompatService: service.NewGeminiMessagesCompatService(&geminiAllowlistAccountRepoStub{}, nil, nil, nil, nil, nil, nil, nil),
		modelCatalog:        listedCatalogStub{ids: []string{"gemini-2.5-pro"}},
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)
	geminiGroupID := int64(42)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{GroupID: &geminiGroupID})

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

func TestFilterUpstreamGeminiModelsBody(t *testing.T) {
	keep := func(name string) bool { return strings.TrimPrefix(name, "models/") == "gemini-2.5-pro" }

	t.Run("full hit returns original body with dropped=false", func(t *testing.T) {
		body := []byte(`{"models":[{"name":"models/gemini-2.5-pro"},{"name":"models/gemini-2.5-pro"}]}`)
		filtered, dropped, ok := filterUpstreamGeminiModelsBody(body, keep)
		require.True(t, ok)
		require.False(t, dropped, "full hit must keep the original response so headers pass through")
		require.Equal(t, string(body), string(filtered))
	})

	t.Run("partial hit filters models and preserves envelope fields", func(t *testing.T) {
		body := []byte(`{"nextPageToken":"tok","models":[{"name":"models/gemini-2.5-pro"},{"name":"models/gemini-2.5-flash"}],"other":"kept"}`)
		filtered, dropped, ok := filterUpstreamGeminiModelsBody(body, keep)
		require.True(t, ok)
		require.True(t, dropped)
		require.JSONEq(t, `{"nextPageToken":"tok","other":"kept","models":[{"name":"models/gemini-2.5-pro"}]}`, string(filtered))
	})

	t.Run("models prefix stripped candidate form matches entry", func(t *testing.T) {
		body := []byte(`{"models":[{"name":"gemini-2.5-pro"}]}`)
		filtered, dropped, ok := filterUpstreamGeminiModelsBody(body, keep)
		require.True(t, ok)
		require.False(t, dropped)
		require.Equal(t, string(body), string(filtered))
	})

	t.Run("missing models field passes through", func(t *testing.T) {
		body := []byte(`{"error":"odd upstream"}`)
		_, dropped, ok := filterUpstreamGeminiModelsBody(body, keep)
		require.True(t, ok)
		require.False(t, dropped)
	})

	t.Run("invalid JSON signals passthrough", func(t *testing.T) {
		_, dropped, ok := filterUpstreamGeminiModelsBody([]byte(`not-json`), keep)
		require.False(t, ok)
		require.False(t, dropped)
	})
}
