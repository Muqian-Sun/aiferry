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
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		),
	}
}

func requestModelsForTest(h *GatewayHandler, group *service.Group, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)
	if group != nil {
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{GroupID: &group.ID, Group: group})
	}
	h.Models(c)
	return rec
}

// 列表内容 = 目录上架条目（与分组平台无关）；分组平台只决定响应形状。
func TestGatewayModels_ListsListedCatalogEntries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newGatewayModelsHandlerForTest("claude-sonnet-4", "gpt-5.6", "grok-4.5")

	t.Run("anthropic group uses claude shape", func(t *testing.T) {
		rec := requestModelsForTest(h, &service.Group{ID: 1, Platform: service.PlatformAnthropic}, "/v1/models")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var got gatewayModelsResponseForTest
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		require.Equal(t, []string{"claude-sonnet-4", "gpt-5.6", "grok-4.5"}, modelIDsForTest(got.Data))
		require.Equal(t, "2024-01-01T00:00:00Z", got.Data[0].CreatedAt)
	})

	t.Run("openai group uses openai shape", func(t *testing.T) {
		rec := requestModelsForTest(h, &service.Group{ID: 2, Platform: service.PlatformOpenAI}, "/v1/models")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var got gatewayModelsResponseForTest
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		require.Equal(t, []string{"claude-sonnet-4", "gpt-5.6", "grok-4.5"}, modelIDsForTest(got.Data))
		require.Equal(t, "model", got.Data[0].Object)
		require.NotZero(t, got.Data[0].Created)
	})

	t.Run("no group still lists the catalog", func(t *testing.T) {
		rec := requestModelsForTest(h, nil, "/v1/models")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var got gatewayModelsResponseForTest
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		require.Len(t, got.Data, 3)
	})

	t.Run("empty catalog lists nothing", func(t *testing.T) {
		rec := requestModelsForTest(newGatewayModelsHandlerForTest(), &service.Group{ID: 3, Platform: service.PlatformOpenAI}, "/v1/models")
		require.Equal(t, http.StatusOK, rec.Code)
		var got gatewayModelsResponseForTest
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		require.Empty(t, got.Data)
	})
}

// 分组白名单开启时在目录之上再过滤（通配符按目录条目展开）。
func TestGatewayModels_AllowlistFiltersListedCatalog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newGatewayModelsHandlerForTest("gpt-5.4", "gpt-5.5-codex", "gpt-5.5-mini", "other-foo")
	group := &service.Group{ID: 31, Platform: service.PlatformOpenAI,
		ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"gpt-5.5-*", "gpt-5.4", "not-in-catalog"}}}

	rec := requestModelsForTest(h, group, "/v1/models")
	require.Equal(t, http.StatusOK, rec.Code)
	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, []string{"gpt-5.5-codex", "gpt-5.5-mini", "gpt-5.4"}, modelIDsForTest(got.Data),
		"allowlist order is kept and entries absent from the catalog are not invented")
}

func TestGatewayModels_GrokShapeAdvertisesReasoningEffort(t *testing.T) {
	cases := []struct {
		model string
		want  []gatewayReasoningEffortOptionForTest
	}{
		{"grok-4.5", []gatewayReasoningEffortOptionForTest{
			{Value: "low", Label: "Low"}, {Value: "medium", Label: "Medium"}, {Value: "high", Label: "High", Default: true},
		}},
		{"grok-4.6", []gatewayReasoningEffortOptionForTest{
			{Value: "low", Label: "Low"}, {Value: "medium", Label: "Medium"}, {Value: "high", Label: "High", Default: true}, {Value: "xhigh", Label: "xHigh"},
		}},
		{"grok-4.6-latest", []gatewayReasoningEffortOptionForTest{
			{Value: "low", Label: "Low"}, {Value: "medium", Label: "Medium"}, {Value: "high", Label: "High", Default: true}, {Value: "xhigh", Label: "xHigh"},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			h := newGatewayModelsHandlerForTest(tc.model)
			rec := requestModelsForTest(h, &service.Group{ID: 4409, Platform: service.PlatformGrok}, "/v1/models")
			require.Equal(t, http.StatusOK, rec.Code)
			var got gatewayModelsResponseForTest
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
			require.Len(t, got.Data, 1)
			model := got.Data[0]
			require.Equal(t, tc.model, model.ID)
			require.True(t, model.SupportsReasoningEffort)
			require.Equal(t, "high", model.ReasoningEffort)
			require.Equal(t, tc.want, model.ReasoningEfforts)
		})
	}
}

// Codex 清单（生成版）只含目录上架的模型；ETag 按最终响应体计算并支持 304。
func TestGatewayCodexModels_UsesListedCatalogAndFinalBodyETag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newGatewayModelsHandlerForTest("deepseek-v4-pro", "gpt-image-2")
	group := &service.Group{ID: 122, Platform: service.PlatformDeepseek}

	first := httptest.NewRecorder()
	firstContext, _ := gin.CreateTestContext(first)
	firstContext.Request = httptest.NewRequest(http.MethodGet, "/models?client_version=0.147.0", nil)
	firstContext.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{Group: group})
	h.CodexModels(firstContext)

	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	var manifest codexModelsResponseForTest
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &manifest))
	require.Equal(t, []string{"deepseek-v4-pro"}, codexModelSlugsForTest(manifest.Models),
		"dedicated image models are dropped by the Codex filter; the rest comes from the catalog")
	etag := first.Header().Get("ETag")
	require.NotEmpty(t, etag)
	require.Equal(t, service.CodexModelsManifestETag(first.Body.Bytes()), etag)

	second := httptest.NewRecorder()
	secondContext, _ := gin.CreateTestContext(second)
	secondContext.Request = httptest.NewRequest(http.MethodGet, "/models?client_version=0.147.0", nil)
	secondContext.Request.Header.Set("If-None-Match", "W/"+etag)
	secondContext.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{Group: group})
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
