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

func requestModelForTest(h *GatewayHandler, group *service.Group, modelID, etag string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Request.Header.Set("If-None-Match", etag)
	if modelID != "" {
		c.Params = gin.Params{{Key: "model", Value: modelID}}
	}
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{GroupID: &group.ID, Group: group})
	h.Models(c)
	return rec
}

func TestRetrieveModelMatchesVisibleCatalogue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, platform := range []string{service.PlatformOpenAI, service.PlatformAnthropic, service.PlatformGemini, service.PlatformGrok, service.PlatformComposite} {
		t.Run(platform, func(t *testing.T) {
			group := &service.Group{ID: 71, Platform: platform}
			h := newGatewayModelsHandlerForTest("custom-model", "second-model")
			list := requestModelForTest(h, group, "", "")
			require.Equal(t, http.StatusOK, list.Code, list.Body.String())
			var catalog struct {
				Data []json.RawMessage `json:"data"`
			}
			require.NoError(t, json.Unmarshal(list.Body.Bytes(), &catalog))
			require.Len(t, catalog.Data, 2)
			var model struct {
				ID string `json:"id"`
			}
			require.NoError(t, json.Unmarshal(catalog.Data[0], &model))
			retrieved := requestModelForTest(h, group, model.ID, "")
			require.Equal(t, http.StatusOK, retrieved.Code, retrieved.Body.String())
			require.JSONEq(t, string(catalog.Data[0]), retrieved.Body.String())
			require.Equal(t, http.StatusNotFound, requestModelForTest(h, group, "unknown-model", "").Code)
			group.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"absent-from-source"}}
			hidden := requestModelForTest(h, group, model.ID, "")
			require.Equal(t, http.StatusNotFound, hidden.Code, hidden.Body.String())
			require.Contains(t, hidden.Body.String(), `"code":"model_not_found"`)
		})
	}
}

func TestRetrievePinnedModelPreservesMetadataFilteringAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		selected   []string
		wantStatus int
	}{
		{name: "metadata", wantStatus: http.StatusOK},
		{name: "hidden", selected: []string{"other-model"}, wantStatus: http.StatusNotFound},
		{name: "upstream failure", status: http.StatusServiceUnavailable, wantStatus: http.StatusBadGateway},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := &codexModelsPinnedHTTPUpstream{bodies: map[int64]string{
				2: `{"data":[{"id":"special-model","ID":"not-a-model-id","owned_by":"source-owner","created":123,"extra":{"context":999}}]}`,
			}, statuses: map[int64]int{}}
			if tc.status != 0 {
				upstream.statuses[2] = tc.status
			}
			codex := newPinnedCodexTestHandler([]service.Account{newPinnedCodexAccount(2, service.StatusActive, true, false)}, upstream, 3)
			h := &GatewayHandler{openAIGatewayService: codex.gatewayService, maxAccountSwitches: 3, modelCatalog: listedCatalogStub{ids: []string{"special-model", "other-model"}}}
			group := &service.Group{ID: 72, Platform: service.PlatformOpenAI,
				ModelAllowlist:            service.GroupModelAllowlist{Enabled: len(tc.selected) > 0, Models: tc.selected},
				CodexModelsManifestConfig: service.GroupCodexModelsManifestConfig{Enabled: true, AccountIDs: []int64{2}}}
			got := requestModelForTest(h, group, "special-model", "collection-etag")
			require.Equal(t, tc.wantStatus, got.Code, got.Body.String())
			if tc.wantStatus == http.StatusOK {
				list := requestModelForTest(h, group, "", "")
				var catalog struct {
					Data []json.RawMessage `json:"data"`
				}
				require.NoError(t, json.Unmarshal(list.Body.Bytes(), &catalog))
				require.JSONEq(t, string(catalog.Data[0]), got.Body.String())
				require.Contains(t, got.Body.String(), `"owned_by":"source-owner"`)
				require.Contains(t, got.Body.String(), `"created":123`)
				again := requestModelForTest(h, group, "special-model", list.Header().Get("ETag"))
				require.Equal(t, http.StatusOK, again.Code)
				require.Empty(t, again.Header().Get("ETag"))
				require.Equal(t, http.StatusNotFound, requestModelForTest(h, group, "not-a-model-id", "").Code)
			}
		})
	}
}

// 固定账号清单也只露出目录上架的模型：未上架的模型列表里没有、单取 404。
func TestPinnedModelsHideUnlistedCatalogModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &codexModelsPinnedHTTPUpstream{bodies: map[int64]string{
		2: `{"data":[{"id":"special-model"},{"id":"hidden-model"}]}`,
	}, statuses: map[int64]int{}}
	codex := newPinnedCodexTestHandler([]service.Account{newPinnedCodexAccount(2, service.StatusActive, true, false)}, upstream, 3)
	h := &GatewayHandler{openAIGatewayService: codex.gatewayService, maxAccountSwitches: 3, modelCatalog: listedCatalogStub{ids: []string{"special-model"}}}
	group := &service.Group{ID: 73, Platform: service.PlatformOpenAI,
		CodexModelsManifestConfig: service.GroupCodexModelsManifestConfig{Enabled: true, AccountIDs: []int64{2}}}

	list := requestModelForTest(h, group, "", "")
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	require.Contains(t, list.Body.String(), "special-model")
	require.NotContains(t, list.Body.String(), "hidden-model")
	require.Equal(t, http.StatusNotFound, requestModelForTest(h, group, "hidden-model", "").Code)
}
