package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// OpenAI 协议入站（/v1/chat/completions、/v1/responses、扩展端点）的请求处理与错误写出，
// Gateway handler 与 OpenAI handler 共用：这些都是入站协议的事，与选到的上游资源无关。

// openAIStreamingAwareError OpenAI 形状的错误写出：流已开始（或 compact 心跳已提交 200）写流内错误帧，
// 否则写 JSON。Responses 入站的流内错误用 response.failed 终止事件。
func openAIStreamingAwareError(c *gin.Context, status int, errType string, code string, message string, streamStarted bool, countTowardsSLA bool) {
	// body-signal compact 心跳可能已把响应头提交为 200：先停心跳（建立
	// happens-before，接管 ResponseWriter），并升级为流内错误处理。
	if service.StopOpenAICompactSSEKeepaliveCommitted(c) {
		streamStarted = true
	}
	if streamStarted {
		if countTowardsSLA {
			service.MarkOpsStreamFailure(c, errType, code, message, status)
		} else {
			service.MarkOpsStreamError(c, errType, message, status)
		}
		// /v1/responses 的严格 SDK（Codex CLI）要求终止事件必须属于
		// response.completed/failed/incomplete/cancelled 集合。
		// 通用 `event: error` 帧不被识别为终止事件，会导致
		// "stream closed before response.completed"。
		if inboundIsResponses(c) {
			if writeResponsesFailedSSE(c, errType, code, message) {
				return
			}
		}
		// Stream already started, send error as SSE event then close
		flusher, ok := c.Writer.(http.Flusher)
		if ok {
			errorObject := gin.H{"type": errType, "message": message}
			if code != "" {
				errorObject["code"] = code
			}
			payload, err := json.Marshal(gin.H{"error": errorObject})
			if err != nil {
				payload = []byte(`{"error":{"type":"upstream_error","message":"Upstream request failed"}}`)
			}
			errorEvent := "event: error\ndata: " + string(payload) + "\n\n"
			if _, err := fmt.Fprint(c.Writer, errorEvent); err != nil {
				_ = c.Error(err)
			}
			flusher.Flush()
		}
		return
	}

	// Normal case: return JSON response with proper status code
	writeOpenAIError(c, status, errType, code, message)
}

// writeOpenAIError OpenAI 形状的 JSON 错误；code 为空时省略。
func writeOpenAIError(c *gin.Context, status int, errType, code, message string) {
	errorObject := gin.H{"type": errType, "message": message}
	if code != "" {
		errorObject["code"] = code
	}
	c.JSON(status, gin.H{"error": errorObject})
}

// ensureOpenAIStreamReadErrorResponse 上游流读错误有明确编码时按它写出（计入 SLA 失败）。
func ensureOpenAIStreamReadErrorResponse(c *gin.Context, err error, streamStarted bool) bool {
	code, message, ok := service.OpenAIUpstreamStreamReadErrorDetails(err)
	if !ok || c == nil || c.Writer == nil || service.IsResponseCommitted(c) {
		return false
	}
	if c.Writer.Written() {
		streamStarted = true
	}
	openAIStreamingAwareError(c, http.StatusBadGateway, "upstream_error", code, message, streamStarted, true)
	return true
}

// normalizeOpenAIResponsesCompactRequest keeps Codex remote compaction v2 on
// its native streaming /responses wire and preserves the legacy body-signal
// promotion for non-streaming requests.
// 返回归一化后的 body；ok=false 表示错误响应已写出，调用方应直接 return。
// normalizeOpenAIResponsesCompactRequest body-signal 的 compact 请求提升为 /compact 路径并归一请求体；失败时已写 400。
func normalizeOpenAIResponsesCompactRequest(c *gin.Context, reqLog *zap.Logger, body []byte) ([]byte, bool) {
	isCompactRequest := isOpenAILegacyCompactPath(c)
	if !isCompactRequest && isBareOpenAIResponsesPath(c) && service.HasCompactionTriggerInInput(body) {
		if normalized, changed, err := service.NormalizeCompactionTriggerInputOrder(body); err != nil {
			reqLog.Warn("codex.remote_compact.trigger_order_normalization_failed", zap.Error(err))
		} else if changed {
			body = normalized
		}
		if isOpenAIRemoteCompactionV2Request(body) {
			return body, true
		}
		c.Request.URL.Path = strings.TrimRight(c.Request.URL.Path, "/") + "/compact"
		isCompactRequest = true
		clientStream := gjson.GetBytes(body, "stream").Bool()
		if clientStream {
			service.MarkOpenAICompactClientStream(c)
		}
		reqLog.Info("codex.remote_compact.detected_body_signal", zap.Bool("client_stream", clientStream))
	}
	if !isCompactRequest {
		return body, true
	}
	if compactSeed := strings.TrimSpace(gjson.GetBytes(body, "prompt_cache_key").String()); compactSeed != "" {
		c.Set(service.OpenAICompactSessionSeedKeyForTest(), compactSeed)
	}
	normalizedCompactBody, normalizedCompact, compactErr := service.NormalizeOpenAICompactRequestBodyForTest(body)
	if compactErr != nil {
		writeOpenAIError(c, http.StatusBadRequest, "invalid_request_error", "", "Failed to normalize compact request body")
		return nil, false
	}
	if normalizedCompact {
		body = normalizedCompactBody
	}
	return body, true
}

// logOpenAIRemoteCompactOutcome /responses/compact 请求结束时记一条结局日志。
func logOpenAIRemoteCompactOutcome(c *gin.Context, cfg *config.Config, startedAt time.Time) {
	if !isOpenAILegacyCompactPath(c) {
		return
	}

	var (
		ctx    = context.Background()
		path   string
		status int
	)
	if c != nil {
		if c.Request != nil {
			ctx = c.Request.Context()
			if c.Request.URL != nil {
				path = strings.TrimSpace(c.Request.URL.Path)
			}
		}
		if c.Writer != nil {
			status = c.Writer.Status()
		}
	}

	outcome := "failed"
	if status >= 200 && status < 300 {
		outcome = "succeeded"
	}
	// compact 心跳提交后失败的 wire 状态码固化为 200，真实结局以流内错误
	// 标记为准（response.failed 降级路径会 MarkOpsStreamError）。
	if outcome == "succeeded" && c != nil {
		if _, hasStreamErr := service.GetOpsStreamError(c); hasStreamErr {
			outcome = "failed"
		}
	}
	latencyMs := time.Since(startedAt).Milliseconds()
	if latencyMs < 0 {
		latencyMs = 0
	}

	fields := []zap.Field{
		zap.String("component", "handler.gateway.responses"),
		zap.Bool("remote_compact", true),
		zap.String("compact_outcome", outcome),
		zap.Int("status_code", status),
		zap.Int64("latency_ms", latencyMs),
		zap.String("path", path),
		zap.Bool("force_codex_cli", cfg != nil && cfg.Gateway.ForceCodexCLI),
	}

	if c != nil {
		if userAgent := strings.TrimSpace(c.GetHeader("User-Agent")); userAgent != "" {
			fields = append(fields, zap.String("request_user_agent", userAgent))
		}
		if v, ok := c.Get(opsModelKey); ok {
			if model, ok := v.(string); ok && strings.TrimSpace(model) != "" {
				fields = append(fields, zap.String("request_model", strings.TrimSpace(model)))
			}
		}
		if v, ok := c.Get(opsAccountIDKey); ok {
			if accountID, ok := v.(int64); ok && accountID > 0 {
				fields = append(fields, zap.Int64("account_id", accountID))
			}
		}
		if c.Writer != nil {
			if upstreamRequestID := strings.TrimSpace(c.Writer.Header().Get("x-request-id")); upstreamRequestID != "" {
				fields = append(fields, zap.String("upstream_request_id", upstreamRequestID))
			} else if upstreamRequestID := strings.TrimSpace(c.Writer.Header().Get("X-Request-Id")); upstreamRequestID != "" {
				fields = append(fields, zap.String("upstream_request_id", upstreamRequestID))
			}
		}
	}

	log := logger.FromContext(ctx).With(fields...)
	if outcome == "succeeded" {
		log.Info("codex.remote_compact.succeeded")
		return
	}
	log.Warn("codex.remote_compact.failed")
}

// validateFunctionCallOutputRequest HTTP 上的 function_call_output 必须带 call_id 与 item_reference（续链只在 WS v2 上）；不满足时已写 400。
func validateFunctionCallOutputRequest(c *gin.Context, body []byte, reqLog *zap.Logger) bool {
	if !gjson.GetBytes(body, `input.#(type=="function_call_output")`).Exists() {
		return true
	}

	validation := service.ValidateFunctionCallOutputContextBytes(body)
	if !validation.HasFunctionCallOutput {
		return true
	}

	previousResponseID := gjson.GetBytes(body, "previous_response_id").String()
	if strings.TrimSpace(previousResponseID) != "" || validation.HasToolCallContext {
		return true
	}

	if validation.HasFunctionCallOutputMissingCallID {
		reqLog.Warn("openai.request_validation_failed",
			zap.String("reason", "function_call_output_missing_call_id"),
		)
		writeOpenAIError(c, http.StatusBadRequest, "invalid_request_error", "", "function_call_output requires call_id on HTTP requests; continuation via previous_response_id is only supported on Responses WebSocket v2")
		return false
	}
	if validation.HasItemReferenceForAllCallIDs {
		return true
	}

	reqLog.Warn("openai.request_validation_failed",
		zap.String("reason", "function_call_output_missing_item_reference"),
	)
	writeOpenAIError(c, http.StatusBadRequest, "invalid_request_error", "", "function_call_output requires item_reference ids matching each call_id on HTTP requests; continuation via previous_response_id is only supported on Responses WebSocket v2")
	return false
}

// acquireImageGenerationSlot 生图并发槽；拿不到时已写 429。
func acquireImageGenerationSlot(c *gin.Context, cfg *config.Config, limiter *ImageConcurrencyLimiter, streamStarted bool) (func(), bool) {
	if cfg == nil || limiter == nil {
		return nil, true
	}
	imageConcurrency := cfg.Gateway.ImageConcurrency
	wait := strings.TrimSpace(imageConcurrency.OverflowMode) == config.ImageConcurrencyOverflowModeWait
	release, acquired := limiter.Acquire(
		c.Request.Context(),
		imageConcurrency.Enabled,
		imageConcurrency.MaxConcurrentRequests,
		wait,
		time.Duration(imageConcurrency.WaitTimeoutSeconds)*time.Second,
		imageConcurrency.MaxWaitingRequests,
	)
	if acquired {
		return release, true
	}
	openAIStreamingAwareError(c, http.StatusTooManyRequests, "rate_limit_error", gatewayConcurrencyLimitCode, "Image generation concurrency limit exceeded, please retry later", streamStarted, false)
	return nil, false
}

// openAICompactKeepaliveInterval 复用流式 keepalive 配置作为 compact 下游
// 心跳间隔；0 表示禁用（与流式路径语义一致）。
// openAICompactKeepaliveInterval compact 等待期的 SSE 注释行心跳间隔；0 = 不发。
func openAICompactKeepaliveInterval(cfg *config.Config) time.Duration {
	if cfg == nil || cfg.Gateway.StreamKeepaliveInterval <= 0 {
		return 0
	}
	return time.Duration(cfg.Gateway.StreamKeepaliveInterval) * time.Second
}

// recoverForwardPanic 转发 handler 的局部兜底：recover 的值由调用方在 defer 里取（recover 只在直接被 defer 的
// 函数里有效），这里补错误响应并记日志。
func recoverForwardPanic(c *gin.Context, streamStarted *bool, recovered any, ensure func(*gin.Context, bool) bool, component, event string) {
	if recovered == nil {
		return
	}
	started := false
	if streamStarted != nil {
		started = *streamStarted
	}
	wroteFallback := ensure(c, started)
	requestLogger(c, component).Error(
		event,
		zap.Bool("fallback_error_response_written", wroteFallback),
		zap.Any("panic", recovered),
		zap.ByteString("stack", debug.Stack()),
	)
}
