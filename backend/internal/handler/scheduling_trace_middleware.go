package handler

import (
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// SchedulingTraceMiddleware 给网关请求挂上调度轨迹记录器，请求结束时合成轨迹交给 writer 异步落库。
// 只有走到调度器的请求才有轨迹；计 token 的请求不是真正的模型调用，不记。
func SchedulingTraceMiddleware(writer *service.SchedulingTraceWriter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if writer == nil || isCountTokensRequest(c) {
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

func schedulingTraceRequestID(c *gin.Context) string {
	ctx := c.Request.Context()
	if id, _ := ctx.Value(ctxkey.RequestID).(string); strings.TrimSpace(id) != "" {
		return id
	}
	id, _ := ctx.Value(ctxkey.ClientRequestID).(string)
	return id
}
