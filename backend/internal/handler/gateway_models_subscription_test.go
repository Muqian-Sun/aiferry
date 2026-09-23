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

// 订阅 key（无分组）的模型列表只列套餐模型集里的条目；套餐外的条目单查 404。

func requestModelsAsSubscriptionKey(h *GatewayHandler, sub *service.UserSubscription, modelID string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	if modelID != "" {
		c.Params = gin.Params{{Key: "model", Value: modelID}}
	}
	subID := sub.ID
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 5, SubscriptionID: &subID})
	c.Set(string(middleware2.ContextKeySubscription), sub)
	h.Models(c)
	return rec
}

func TestModels_SubscriptionKeyListsOnlyPlanEntries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// listedCatalogStub 条目 ID = 下标 + 1：claude-sonnet-4=1、gpt-5.6=2、grok-4.5=3
	h := newGatewayModelsHandlerForTest("claude-sonnet-4", "gpt-5.6", "grok-4.5")
	sub := &service.UserSubscription{ID: 9, PlanID: 3, Plan: &service.SubscriptionPlan{ID: 3, Models: []service.SubscriptionPlanModel{{EntryID: 2, ModelID: "gpt-5.6"}}}}

	list := requestModelsAsSubscriptionKey(h, sub, "")
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &got))
	require.Equal(t, []string{"gpt-5.6"}, modelIDsForTest(got.Data), "只列套餐内条目")

	require.Equal(t, http.StatusOK, requestModelsAsSubscriptionKey(h, sub, "gpt-5.6").Code)
	notInPlan := requestModelsAsSubscriptionKey(h, sub, "claude-sonnet-4")
	require.Equal(t, http.StatusNotFound, notInPlan.Code, notInPlan.Body.String())
	require.Contains(t, notInPlan.Body.String(), `"code":"model_not_found"`)

	// 余额 key（ctx 无订阅）照旧列全部
	all := requestModelsForTest(h, "/v1/models")
	require.NoError(t, json.Unmarshal(all.Body.Bytes(), &got))
	require.Len(t, got.Data, 3)
}

// 订阅 key 的 Codex 清单同样按套餐过滤，且不再因为没有分组而 401（§6-9）。
func TestCodexModels_SubscriptionKeyFilteredByPlanNot401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newGatewayModelsHandlerForTest("gpt-5.6", "gpt-5.6-mini", "claude-sonnet-4")
	codexRequest := func(sub *service.UserSubscription) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/backend-api/codex/models", nil)
		subID := sub.ID
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 5, SubscriptionID: &subID})
		c.Set(string(middleware2.ContextKeySubscription), sub)
		h.CodexModels(c)
		return rec
	}

	inPlan := codexRequest(&service.UserSubscription{ID: 9, PlanID: 3, Plan: &service.SubscriptionPlan{ID: 3, Models: []service.SubscriptionPlanModel{{EntryID: 2, ModelID: "gpt-5.6-mini"}}}})
	require.Equal(t, http.StatusOK, inPlan.Code, inPlan.Body.String())
	var manifest codexModelsResponseForTest
	require.NoError(t, json.Unmarshal(inPlan.Body.Bytes(), &manifest))
	require.Equal(t, []string{"gpt-5.6-mini"}, codexModelSlugsForTest(manifest.Models))

	empty := codexRequest(&service.UserSubscription{ID: 10, PlanID: 4, Plan: &service.SubscriptionPlan{ID: 4, Models: []service.SubscriptionPlanModel{{EntryID: 3, ModelID: "claude-sonnet-4"}}}})
	require.Equal(t, http.StatusOK, empty.Code, "套餐里没有 OpenAI 条目 → 空清单，不是 401")
	require.NoError(t, json.Unmarshal(empty.Body.Bytes(), &manifest))
	require.Empty(t, manifest.Models)
}
