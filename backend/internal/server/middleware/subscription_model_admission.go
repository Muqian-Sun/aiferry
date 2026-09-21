package middleware

import (
	"fmt"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// SubscriptionModelAdmission 订阅模型集准入：订阅 key 只能调用套餐模型集里的目录条目。
// 挂在每条网关链的 catalogAdmission 之后、groupModelAllowlist 之前。
//   - 余额 key（ctx 无 subscription）放行；
//   - 无 CatalogRoute（无模型端点）放行：限额已在 apiKeyAuth 检过，这里不查模型集；
//   - WS 的目录路由在 handler 里解析，这里恒放行，由 handler 用同一个 SubscriptionCoversRoute 判。
//
// 拒绝 403 MODEL_NOT_IN_PLAN：不落余额、不计用量（handler 之前中断）。
func SubscriptionModelAdmission() gin.HandlerFunc {
	return func(c *gin.Context) {
		sub, ok := GetSubscriptionFromContext(c)
		if !ok || sub == nil {
			c.Next()
			return
		}
		route, ok := service.CatalogRouteFromContext(c.Request.Context())
		if !ok {
			c.Next()
			return
		}
		if service.SubscriptionCoversRoute(sub, route) {
			c.Next()
			return
		}
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalModelConfiguration)
		MarkIngressRejected(c, IngressRejectModelNotInPlan)
		AbortWithError(c, http.StatusForbidden, "MODEL_NOT_IN_PLAN",
			fmt.Sprintf("Model %q is not included in your subscription plan", route.RequestedModel))
	}
}
