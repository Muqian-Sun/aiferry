package middleware

import (
	"fmt"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// GroupModelAllowlist 是分组级模型白名单准入中间件。
//
// 挂载位置：每条网关链的 apiKeyAuth 之后、compositeTarget 之前——
// 保证校验发生在合成路由改写与调度之前，且只看客户端书写的公开模型名。
//
// 行为：
//   - 快速路径：未绑定分组或白名单未开启时直接放行，不读请求体。
//   - Responses WebSocket 入口跳过（首帧与后续 turn 由 ResponsesWebSocket 逐帧
//     校验）；Grok Realtime 的升级请求模型固定在查询参数里，仍走中间件校验，
//     其他路由伪造 Upgrade 头不得绕过校验。
//   - GET/DELETE：从路由参数（model / modelAction）或查询参数（/realtime 的
//     model）提取模型；提取不到放行，由 handler 决定是否报「model is required」。
//   - 带请求体的方法：读体（PrereadBody 回填，下游零拷贝）、提取 JSON
//     `model`/`session.model` 或 multipart `model`/`session` 后回填请求体。
//   - 拒绝：按入口协议格式返回 404，并标记运维业务限流原因
//     local_model_configuration 与 ingress 拒绝原因 model_not_allowed。
func GroupModelAllowlist() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey, ok := GetAPIKeyFromContext(c)
		if !ok || apiKey == nil || apiKey.Group == nil || !apiKey.Group.ModelAllowlistEnabled() {
			c.Next()
			return
		}
		allowlist := apiKey.Group.ModelAllowlist
		if c.Request == nil {
			c.Next()
			return
		}

		if isResponsesWebSocketRoute(c) {
			// Responses WS 长连接由 ResponsesWebSocket 校验首帧与每个 response.create。
			c.Next()
			return
		}
		// models 收集该请求全部可被下游解析器绑定到的模型值（路径参数/查询参数
		// 单值；请求体候选集含重复键与大小写变体），逐一校验，任一未命中即拒绝。
		models, done := requestModelCandidates(c)
		if !done {
			// 请求体读取失败（如超限 413）已写出响应。
			return
		}

		blocked := ""
		for _, candidate := range models {
			if !allowlist.Allows(candidate) {
				blocked = candidate
				break
			}
		}
		if blocked == "" {
			c.Next()
			return
		}

		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalModelConfiguration)
		MarkIngressRejected(c, IngressRejectModelNotAllowed)
		gatewayModelErrorWriter(c)(c, http.StatusNotFound, fmt.Sprintf("Model %q is not available for this group", blocked))
		c.Abort()
	}
}
