package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// SchedulingTraceMiddleware 给网关请求挂上调度轨迹记录器，请求结束时合成轨迹交给 writer 异步落库。
// 只有走到调度器的请求才有轨迹；计 token 的请求不是真正的模型调用，不记；视频任务的状态 / 内容
// 轮询只记创建那一次（产品定，轮询会刷出一串同一任务的轨迹，旧 Grok 查询仍经调度器选回归属账号）。
func SchedulingTraceMiddleware(writer *service.SchedulingTraceWriter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if writer == nil || isCountTokensRequest(c) || isVideoTaskLookupRequest(c) {
			c.Next()
			return
		}
		ctx, recorder := service.WithSchedulingTrace(c.Request.Context(), time.Now())
		c.Request = c.Request.WithContext(ctx)
		c.Next()

		input := service.SchedulingTraceInput{
			RequestID:      schedulingTraceRequestID(c),
			StatusCode:     c.Writer.Status(),
			Canceled:       c.Request.Context().Err() != nil,
			StreamFailed:   len(service.GetOpsStreamErrors(c)) > 0,
			End:            time.Now(),
			UpstreamErrors: service.OpsUpstreamErrorEventsFromContext(c),
		}
		if apiKey, ok := middleware2.GetAPIKeyFromContext(c); ok && apiKey != nil {
			keyID, userID := apiKey.ID, apiKey.UserID
			input.APIKeyID, input.UserID = &keyID, &userID
		}
		if trace, ok := recorder.Build(c.Request.Context(), input); ok {
			writer.Enqueue(trace)
		}
	}
}

// isVideoTaskLookupRequest 视频任务的状态 / 内容查询：/videos/… 与 /video/generations/… 下的 GET
// （这两个前缀下的 GET 只有查询，创建都是 POST）。
func isVideoTaskLookupRequest(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.URL == nil || c.Request.Method != http.MethodGet {
		return false
	}
	path := c.Request.URL.Path
	return strings.Contains(path, "/videos/") || strings.Contains(path, "/video/generations/")
}

func schedulingTraceRequestID(c *gin.Context) string {
	ctx := c.Request.Context()
	if id, _ := ctx.Value(ctxkey.RequestID).(string); strings.TrimSpace(id) != "" {
		return id
	}
	id, _ := ctx.Value(ctxkey.ClientRequestID).(string)
	return id
}
