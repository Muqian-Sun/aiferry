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
//   - 提取不到模型名的请求：路由在 catalogAdmissionRouteDefaults 里的（/v1/images/*）按
//     该端点的默认模型准入——handler 反正会用同一个默认值转发，不能让它绕过准入；
//     其余无模型端点按端点路由放行，handler 自己决定是否报「model is required」。
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
			if fallback := catalogAdmissionRouteDefaults[c.FullPath()]; fallback != "" {
				models = []string{fallback}
			} else {
				c.Next()
				return
			}
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

// catalogAdmissionRouteDefaults 是「请求不带 model 时 handler 会用的默认模型」，按路由模板
// （c.FullPath()）查；只有 images 端点有默认值（service.DefaultImageGenerationModel）。
var catalogAdmissionRouteDefaults = func() map[string]string {
	out := make(map[string]string)
	for _, prefix := range []string{"/v1", ""} {
		for _, path := range []string{"/images/generations", "/images/edits", "/images/generations/async", "/images/edits/async"} {
			out[prefix+path] = service.DefaultImageGenerationModel
		}
	}
	return out
}()

func rejectCatalogAdmission(c *gin.Context, message string) {
	service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalModelConfiguration)
	MarkIngressRejected(c, IngressRejectModelNotListed)
	gatewayModelErrorWriter(c)(c, http.StatusNotFound, message)
	c.Abort()
}
