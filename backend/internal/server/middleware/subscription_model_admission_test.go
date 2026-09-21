package middleware

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 订阅模型集准入：挂在 catalogAdmission 之后，只对「ctx 有订阅 且 有目录路由」的请求判套餐模型集。

type subscriptionAdmissionSeen struct {
	calls  int
	reason IngressRejectReason
	rejOK  bool
	opsLim bool
}

// newSubscriptionAdmissionRouter 链：注入订阅（模拟 apiKeyAuth）→ catalogAdmission → subscriptionModelAdmission → handler
func newSubscriptionAdmissionRouter(sub *service.UserSubscription) (*gin.Engine, *subscriptionAdmissionSeen) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	seen := &subscriptionAdmissionSeen{}
	router.Use(func(c *gin.Context) {
		if sub != nil {
			c.Set(string(ContextKeySubscription), sub)
		}
		c.Next()
		seen.reason, seen.rejOK = GetIngressRejectReason(c)
		seen.opsLim = service.HasOpsClientBusinessLimited(c)
	})
	router.Use(CatalogAdmission(newCatalogStub()))
	router.Use(SubscriptionModelAdmission())
	handler := func(c *gin.Context) {
		seen.calls++
		c.Status(http.StatusOK)
	}
	router.POST("/v1/messages", handler)
	router.POST("/v1/chat/completions", handler)
	router.POST("/v1/web_search", handler) // 无模型端点：catalogAdmission 不挂路由
	return router, seen
}

func planWithEntries(ids ...int64) *service.UserSubscription {
	models := make([]service.SubscriptionPlanModel, 0, len(ids))
	for _, id := range ids {
		models = append(models, service.SubscriptionPlanModel{EntryID: id})
	}
	return &service.UserSubscription{ID: 1, UserID: 1, PlanID: 7, Plan: &service.SubscriptionPlan{ID: 7, Models: models}}
}

func TestSubscriptionModelAdmission_RejectsEntryNotInPlan(t *testing.T) {
	router, seen := newSubscriptionAdmissionRouter(planWithEntries(2)) // 套餐只含 gpt-5.6（条目 2）
	w := doJSON(t, router, http.MethodPost, "/v1/messages", `{"model":"sonnet-latest"}`)
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"MODEL_NOT_IN_PLAN"`)
	require.Contains(t, w.Body.String(), `sonnet-latest`, "错误信息带请求的模型名")
	require.Zero(t, seen.calls, "handler 不执行（不落余额、不计用量）")
	require.True(t, seen.rejOK)
	require.Equal(t, IngressRejectModelNotInPlan, seen.reason)
	require.True(t, seen.opsLim, "记为本地模型配置受限")
}

func TestSubscriptionModelAdmission_AllowsCoveredEntry(t *testing.T) {
	router, seen := newSubscriptionAdmissionRouter(planWithEntries(1, 2))
	for _, path := range []string{"/v1/messages", "/v1/chat/completions"} {
		w := doJSON(t, router, http.MethodPost, path, `{"model":"gpt-5.6"}`)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}
	require.Equal(t, 2, seen.calls)
	require.False(t, seen.rejOK)
	require.False(t, seen.opsLim)
}

// 无模型端点没有目录路由：放行（限额已在 apiKeyAuth 检过）
func TestSubscriptionModelAdmission_SkipsWithoutRoute(t *testing.T) {
	router, seen := newSubscriptionAdmissionRouter(planWithEntries(2))
	w := doJSON(t, router, http.MethodPost, "/v1/web_search", `{"query":"x"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, 1, seen.calls)
}

// 余额 key：ctx 无订阅，套餐外模型照样放行
func TestSubscriptionModelAdmission_SkipsBalanceKey(t *testing.T) {
	router, seen := newSubscriptionAdmissionRouter(nil)
	w := doJSON(t, router, http.MethodPost, "/v1/messages", `{"model":"sonnet-latest"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, 1, seen.calls)
	require.False(t, seen.rejOK)
}
