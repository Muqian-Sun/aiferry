package handler

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// handleUnsupportedContentError 拦截跨协议转换拒收的内容分片（apicompat.UnsupportedContentError）。
//
// 这类错误是客户端请求本身无法在所选资源的上游协议上忠实表达（如 input_audio 发往
// Anthropic、URL 图片发往 Gemini），属于 400 invalid_request_error：转换发生在发出任何上游
// 请求之前，与上游 / 账号状态无关。调用方必须在 failover 判定与账号健康观测
// （ObserveOpenAIAccountResult 等）之前拦下它直接返回——不换号、不标记账号异常、不计入
// 账号健康，也不落到通用的 502 "Upstream request failed" 兜底。
//
// 部分兼容层（Gemini / Antigravity / OpenAI 的 chat 回退）在 service 层已按入站协议写出了
// 400（serviceWroteResponse），此时不重复写；槽位等待心跳已把 200 提交为 SSE 时
// （streamStarted）以协议合规的流内错误帧终止。
//
// 返回 false 表示 err 不是这类错误，调用方照常处理。
func (h *GatewayHandler) handleUnsupportedContentError(
	c *gin.Context,
	reqLog *zap.Logger,
	account *service.Account,
	err error,
	serviceWroteResponse bool,
	streamStarted bool,
	writeJSON func(c *gin.Context, status int, errType, message string),
) bool {
	unsupported, ok := apicompat.AsUnsupportedContentError(err)
	if !ok {
		return false
	}
	message := unsupported.Error()
	if reqLog != nil {
		fields := []zap.Field{zap.String("reason", message)}
		if account != nil {
			fields = append(fields, zap.Int64("account_id", account.ID))
		}
		reqLog.Info("gateway.unsupported_content_rejected", fields...)
	}
	switch {
	case serviceWroteResponse || service.IsResponseCommitted(c):
		// service 层已写出 400 错误体。
	case streamStarted:
		h.handleStreamingAwareError(c, http.StatusBadRequest, "invalid_request_error", message, true)
	default:
		writeJSON(c, http.StatusBadRequest, "invalid_request_error", message)
	}
	return true
}
