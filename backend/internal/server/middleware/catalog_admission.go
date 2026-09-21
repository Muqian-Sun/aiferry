package middleware

import (
	"fmt"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// CatalogAdmissionSource 是目录准入需要的最小接口；ModelCatalogService 满足它。
type CatalogAdmissionSource = service.CatalogRouteResolver

// CatalogAdmission 是目录准入中间件：客户端写的模型名必须解析到一条上架（listed）
// 的目录条目，否则按入口协议格式返回 404。命中后把 CatalogRoute 挂到 request.Context，
// 下游据此定资源池。条目不分入口：/v1beta 上调到 anthropic 厂商的条目也放行，池里没有
// 能承接 gemini 入站的资源时由调度回 503（资格在协议转换注册表里）。
//
// 挂载位置：每条网关链的 apiKeyAuth 之后、其余准入之前。
//
// 行为：
//   - Responses WebSocket 入口跳过（模型在升级后的帧里，由 handler 逐帧校验）。
//   - 提取不到模型名的请求放行：无模型的端点按端点路由，handler 自己决定是否报
//     「model is required」。
//   - 请求体里多个候选（重复键、大小写变体）必须解析到同一条目，否则拒绝。
//   - 拒绝时标记运维业务限流原因 local_model_configuration 与 ingress 拒绝原因
//     model_not_listed。
func CatalogAdmission(catalog CatalogAdmissionSource) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request == nil || isResponsesWebSocketRoute(c) {
			c.Next()
			return
		}
		models, done := requestModelCandidates(c)
		if !done {
			return
		}
		if len(models) == 0 {
			c.Next()
			return
		}

		route, blocked, ok := service.ResolveCatalogRouteForCandidates(c.Request.Context(), catalog, models)
		if !ok {
			rejectCatalogAdmission(c, fmt.Sprintf("Model %q is not available", blocked))
			return
		}
		c.Request = c.Request.WithContext(service.WithCatalogRoute(c.Request.Context(), route))
		c.Next()
	}
}

func rejectCatalogAdmission(c *gin.Context, message string) {
	service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalModelConfiguration)
	MarkIngressRejected(c, IngressRejectModelNotListed)
	gatewayModelErrorWriter(c)(c, http.StatusNotFound, message)
	c.Abort()
}
