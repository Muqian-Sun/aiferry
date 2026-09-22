package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type gatewayModelsResponseForTest struct {
	Object string                    `json:"object"`
	Data   []gatewayModelItemForTest `json:"data"`
}

type codexModelsResponseForTest struct {
	Models []struct {
		Slug                     string                       `json:"slug"`
		SupportedReasoningLevels []codexReasoningLevelForTest `json:"supported_reasoning_levels"`
		InputModalities          []string                     `json:"input_modalities"`
		ModelMessages            map[string]json.RawMessage   `json:"model_messages"`
		TruncationPolicy         map[string]json.RawMessage   `json:"truncation_policy"`
		AvailabilityNUX          json.RawMessage              `json:"availability_nux"`
		Upgrade                  json.RawMessage              `json:"upgrade"`
	} `json:"models"`
}

type codexReasoningLevelForTest struct {
	Effort string `json:"effort"`
}

type gatewayModelItemForTest struct {
	ID                      string                                `json:"id"`
	Object                  string                                `json:"object"`
	Created                 int64                                 `json:"created"`
	OwnedBy                 string                                `json:"owned_by"`
	CreatedAt               string                                `json:"created_at"`
	SupportsReasoningEffort bool                                  `json:"supportsReasoningEffort"`
	ReasoningEffort         string                                `json:"reasoningEffort"`
	ReasoningEfforts        []gatewayReasoningEffortOptionForTest `json:"reasoningEfforts"`
}

type gatewayReasoningEffortOptionForTest struct {
	Value   string `json:"value"`
	Label   string `json:"label"`
	Default bool   `json:"default"`
}

// newGatewayModelsHandlerForTest 构造只带目录来源的 handler：列表内容来自目录，
// 与分组账号无关；Codex 清单生成还需要一个 GatewayService（不访问账号）。
func newGatewayModelsHandlerForTest(listed ...string) *GatewayHandler {
	return &GatewayHandler{
		modelCatalog: listedCatalogStub{ids: listed},
		gatewayService: service.NewGatewayService(
			nil,
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		),
	}
}

func requestModelsForTest(h *GatewayHandler, group *service.Group, path string, headers ...string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)
	for i := 0; i+1 < len(headers); i += 2 {
		c.Request.Header.Set(headers[i], headers[i+1])
	}
	if group != nil {
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{GroupID: &group.ID, Group: group})
	}
	h.Models(c)
	return rec
}

// 列表内容 = 目录上架条目；响应形状只看客户端：带 anthropic-version 头 → Claude 形状，否则 OpenAI 形状。
func TestGatewayModels_ListsListedCatalogEntries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newGatewayModelsHandlerForTest("claude-sonnet-4", "gpt-5.6", "grok-4.5")

	t.Run("anthropic-version header uses claude shape", func(t *testing.T) {
		rec := requestModelsForTest(h, nil, "/v1/models", "anthropic-version", "2023-06-01")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var got gatewayModelsResponseForTest
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		require.Equal(t, []string{"claude-sonnet-4", "gpt-5.6", "grok-4.5"}, modelIDsForTest(got.Data))
		require.Equal(t, "2024-01-01T00:00:00Z", got.Data[0].CreatedAt)
	})

	t.Run("no header uses openai shape even for an anthropic group", func(t *testing.T) {
		rec := requestModelsForTest(h, &service.Group{ID: 1, Platform: service.PlatformAnthropic}, "/v1/models")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var got gatewayModelsResponseForTest
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		require.Equal(t, []string{"claude-sonnet-4", "gpt-5.6", "grok-4.5"}, modelIDsForTest(got.Data))
		require.Equal(t, "model", got.Data[0].Object)
		require.NotZero(t, got.Data[0].Created)
		require.Empty(t, got.Data[0].CreatedAt)
	})

	t.Run("grok group gets the openai shape too", func(t *testing.T) {
		rec := requestModelsForTest(h, &service.Group{ID: 4, Platform: service.PlatformGrok}, "/v1/models")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var got gatewayModelsResponseForTest
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		require.Equal(t, "model", got.Data[0].Object)
		require.False(t, got.Data[2].SupportsReasoningEffort, "Grok 专有形状已并入 OpenAI 形状")
	})

	t.Run("empty catalog lists nothing", func(t *testing.T) {
		rec := requestModelsForTest(newGatewayModelsHandlerForTest(), nil, "/v1/models")
		require.Equal(t, http.StatusOK, rec.Code)
		var got gatewayModelsResponseForTest
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		require.Empty(t, got.Data)
	})
}

// Codex 清单由目录生成：只含上架的 OpenAI 厂商条目（生图专用模型被 Codex 过滤掉），无分组也能拿；
// ETag 按最终响应体计算并支持 304。
func TestGatewayCodexModels_UsesListedCatalogAndFinalBodyETag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newGatewayModelsHandlerForTest("gpt-5.6", "gpt-image-2", "claude-sonnet-4", "deepseek-v4-pro")

	first := httptest.NewRecorder()
	firstContext, _ := gin.CreateTestContext(first)
	firstContext.Request = httptest.NewRequest(http.MethodGet, "/models?client_version=0.147.0", nil)
	firstContext.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{})
	h.CodexModels(firstContext)

	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	var manifest codexModelsResponseForTest
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &manifest))
	require.Equal(t, []string{"gpt-5.6"}, codexModelSlugsForTest(manifest.Models),
		"only listed OpenAI-vendor entries appear; image models are dropped by the Codex filter, other vendors are not Codex models")
	etag := first.Header().Get("ETag")
	require.NotEmpty(t, etag)
	require.Equal(t, service.CodexModelsManifestETag(first.Body.Bytes()), etag)

	second := httptest.NewRecorder()
	secondContext, _ := gin.CreateTestContext(second)
	secondContext.Request = httptest.NewRequest(http.MethodGet, "/models?client_version=0.147.0", nil)
	secondContext.Request.Header.Set("If-None-Match", "W/"+etag)
	secondContext.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{})
	h.CodexModels(secondContext)

	require.Equal(t, http.StatusNotModified, second.Code)
	require.Empty(t, second.Body.Bytes())
	require.Equal(t, etag, second.Header().Get("ETag"))
}

func codexModelSlugsForTest(models []struct {
	Slug                     string                       `json:"slug"`
	SupportedReasoningLevels []codexReasoningLevelForTest `json:"supported_reasoning_levels"`
	InputModalities          []string                     `json:"input_modalities"`
	ModelMessages            map[string]json.RawMessage   `json:"model_messages"`
	TruncationPolicy         map[string]json.RawMessage   `json:"truncation_policy"`
	AvailabilityNUX          json.RawMessage              `json:"availability_nux"`
	Upgrade                  json.RawMessage              `json:"upgrade"`
}) []string {
	slugs := make([]string, 0, len(models))
	for _, model := range models {
		slugs = append(slugs, model.Slug)
	}
	return slugs
}

func modelIDsForTest(models []gatewayModelItemForTest) []string {
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	return ids
}
