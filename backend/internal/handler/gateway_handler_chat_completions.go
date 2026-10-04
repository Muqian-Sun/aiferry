package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// ChatCompletions handles OpenAI Chat Completions API endpoint for Anthropic platform groups.
// POST /v1/chat/completions
// This converts Chat Completions requests to Anthropic format (via Responses format chain),
// forwards to Anthropic upstream, and converts responses back to Chat Completions format.
func (h *GatewayHandler) ChatCompletions(c *gin.Context) {
	streamStarted := false

	requestStart := time.Now()

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.chatCompletionsErrorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.chatCompletionsErrorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	reqLog := requestLogger(
		c,
		"handler.gateway.chat_completions",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
	)

	// Read request body
	body, err := readLenientJSONRequestBodyWithPrealloc(c.Request, h.cfg)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.chatCompletionsErrorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		logRequestBodyReadFailure(reqLog, c.Request, err)
		h.chatCompletionsErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}

	if len(body) == 0 {
		h.chatCompletionsErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}

	setOpsRequestContext(c, "", false)

	// Validate JSON
	if !gjson.ValidBytes(body) {
		logRequestBodyParseFailure(reqLog, body, nil)
		h.chatCompletionsErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return
	}

	// Extract model and stream
	modelResult := gjson.GetBytes(body, "model")
	if !modelResult.Exists() || modelResult.Type != gjson.String || modelResult.String() == "" {
		h.chatCompletionsErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	reqModel := modelResult.String()
	bindRequestedReasoningEffort(c, body, reqModel)
	reqStream, ok := parseOpenAICompatibleStream(body)
	if !ok {
		h.chatCompletionsErrorResponse(c, http.StatusBadRequest, "invalid_request_error", invalidStreamFieldTypeMessage)
		return
	}
	if _, err := service.ValidateOpenAIServiceTierField(body); err != nil {
		h.chatCompletionsErrorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	if service.IsGPTImageGenerationModel(reqModel) {
		h.chatCompletionsErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "This model is not supported on the Chat Completions endpoint")
		return
	}
	// 发到本端点的 Responses 形状请求体（Cursor 等：有 input、无 messages）会原样转发到 Responses
	// 上游，image_generation 工具能带过去，与 /v1/responses 同样把关。Chat 形状的请求体在
	// Chat→Responses 转换里只保留 function / web_search / code_execution / x_search 工具，带不过去。
	if !gjson.GetBytes(body, "messages").Exists() && gjson.GetBytes(body, "input").Exists() {
		body, ok = gateOpenAIImageGenerationTool(c, h.cfg, reqLog, reqModel, body, h.chatCompletionsErrorResponse)
		if !ok {
			return
		}
	}
	reqLog = reqLog.With(zap.String("model", reqModel), zap.Bool("stream", reqStream))

	setOpsRequestContext(c, reqModel, reqStream)
	setOpsEndpointContext(c, "", int16(service.RequestTypeFromLegacy(reqStream, false)))
	pricingCtx, pricingAt := service.WithGatewayTokenRequestPricing(c.Request.Context())
	c.Request = c.Request.WithContext(pricingCtx)

	// 解析渠道级模型映射

	if decision := h.checkSecurityAudit(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIChat, reqModel, body); decision != nil && !decision.AllowNextStage {
		h.openAISecurityAuditError(c, decision)
		return
	}
	if rejectIfCyberSessionBlocked(c, h.cyberPolicyDeps(), apiKey, body, reqModel, cyberBlockFormatChat) {
		return
	}

	// Error passthrough binding
	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)

	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())

	userReleaseFunc, err := h.concurrencyHelper.AcquireUserSlotWithWait(c, subject.UserID, subject.Concurrency, reqStream, &streamStarted)
	if err != nil {
		reqLog.Warn("gateway.cc.user_slot_acquire_failed", zap.Error(err))
		h.handleConcurrencyError(c, err, "user", streamStarted)
		return
	}
	userReleaseFunc = wrapReleaseOnDone(c.Request.Context(), userReleaseFunc)
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}

	// 2. Re-check billing
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, subscription); err != nil {
		reqLog.Info("gateway.cc.billing_check_failed", zap.Error(err))
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.chatCompletionsErrorResponse(c, status, code, message)
		return
	}

	// Parse request for session hash
	bodyRef := service.NewRequestBodyRef(body)
	parsedReq, _ := service.ParseGatewayRequest(bodyRef, "chat_completions")
	if parsedReq == nil {
		parsedReq = &service.ParsedRequest{Model: reqModel, Stream: reqStream, Body: bodyRef}
	}
	parsedReq.SessionContext = &service.SessionContext{
		ClientIP:  ip.GetClientIP(c),
		UserAgent: c.GetHeader("User-Agent"),
		APIKeyID:  apiKey.ID,
	}
	// 粘性键按 OpenAI 协议派生：会话头 / prompt_cache_key / 稳定的内容摘要（对 anthropic 池同样生效）。
	sessionHash := h.openAIGatewayService.GenerateSessionHash(c, body)
	// OpenAI 上游的 prompt cache 键（Responses prompt_cache_key）
	promptCacheKey := h.openAIGatewayService.ExtractSessionID(c, body)
	requestPlatform := service.OpenAICompatibleRequestPlatform(c.Request.Context())
	selectionSessionHash := sessionHash
	// 3. Account selection + failover loop
	fs := NewFailoverState(h.maxAccountSwitches, false)

	for {
		if c.Request.Context().Err() != nil {
			return
		}
		selection, err := h.gatewayService.SelectAccountWithOptions(c.Request.Context(), selectionSessionHash, reqModel, fs.FailedAccountIDs, service.SelectOptions{Capability: service.OpenAIEndpointCapabilityChatCompletions})
		if err != nil {
			if len(fs.FailedAccountIDs) == 0 {
				cls := classifyNoAccountErrorFromGin(c, h.gatewayService, reqModel, reqModel, requestPlatform)
				cls = classifySelectionFailureError(err, cls)
				if !cls.ModelNotFound {
					markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
				}
				message := cls.Message
				// 选不到上游：说「这个模型现在没有上游可用」；限流（429）与模型不存在（404）用分类给的说法（D5）
				if !cls.ModelNotFound && cls.Status != http.StatusTooManyRequests {
					message = noUpstreamMessage(reqModel)
				}
				h.chatCompletionsErrorResponse(c, cls.Status, cls.ErrType, message)
				return
			}
			if fs.HandleSelectionExhausted(c.Request.Context()) == FailoverCanceled {
				failoverClientGone(c)
				return
			}
			if fs.LastFailoverErr != nil {
				h.handleCCFailoverExhausted(c, fs.LastFailoverErr, requestPlatform, streamStarted)
			} else {
				h.chatCompletionsErrorResponse(c, http.StatusBadGateway, "server_error", "All available accounts exhausted")
			}
			return
		}
		account := selection.Account
		setOpsSelectedAccount(c, account.ID, account.Platform)

		// 4. Acquire account concurrency slot
		accountReleaseFunc := selection.ReleaseFunc
		if !selection.Acquired {
			if selection.WaitPlan == nil {
				markOpsRoutingCapacityLimited(c)
				h.chatCompletionsErrorResponse(c, http.StatusServiceUnavailable, "api_error", upstreamBusyMessage)
				return
			}
			accountReleaseFunc, err = h.concurrencyHelper.AcquireAccountSlotWithWaitTimeout(
				c,
				account.ID,
				selection.WaitPlan.MaxConcurrency,
				selection.WaitPlan.Timeout,
				reqStream,
				&streamStarted,
			)
			if err != nil {
				reqLog.Warn("gateway.cc.account_slot_acquire_failed", zap.Int64("account_id", account.ID), zap.Error(err))
				h.handleConcurrencyError(c, err, "account", streamStarted)
				return
			}
		}
		// 终检与准入后绑定使用选号结果携带的门（见 responses 同名注释）。
		admissionCtx := service.ContextWithSelectionProfitGate(c.Request.Context(), selection)
		latest, vetoed, reason := h.gatewayService.GatewayProfitControlVetoLatest(admissionCtx, account)
		if vetoed {
			if accountReleaseFunc != nil {
				accountReleaseFunc()
			}
			reqLog.Debug("gateway.cc.account_slot_profit_vetoed", zap.Int64("account_id", account.ID), zap.String("reason", reason))
			if fs.RecordProfitVeto(account.ID) == FailoverExhausted {
				reqLog.Warn("gateway.cc.profit_veto_attempts_exhausted", zap.Int("profit_veto_count", fs.ProfitVetoCount()))
				h.chatCompletionsErrorResponse(c, http.StatusServiceUnavailable, "api_error", profitVetoExhaustedMessage)
				return
			}
			continue
		}
		account = latest
		selection.Account = latest
		if selection.ProfitGateActive() {
			if err := h.gatewayService.BindStickySessionAfterProfitAdmission(admissionCtx, selectionSessionHash, account.ID); err != nil {
				reqLog.Warn("gateway.cc.bind_sticky_session_after_profit_admission_failed", zap.Int64("account_id", account.ID), zap.Error(err))
			}
		}
		accountReleaseFunc = wrapReleaseOnDone(c.Request.Context(), accountReleaseFunc)

		forwardTarget := chatCompletionsForwardTarget(account)
		if forwardTarget == compatForwardSkip {
			if accountReleaseFunc != nil {
				accountReleaseFunc()
			}
			if account.IsThirdPartyKey() {
				// 调度按协议地址放行 key，这里仍对不上说明两边口径不一致，留日志而不是静默换号。
				reqLog.Warn("gateway.cc.key_protocol_unavailable",
					zap.Int64("account_id", account.ID),
					zap.String("request_platform", requestPlatform),
				)
			}
			fs.FailedAccountIDs[account.ID] = struct{}{}
			continue
		}

		// 5. Forward request
		writerSizeBeforeForward := c.Writer.Size()
		forwardBody := body
		var result *service.ForwardResult
		var oaResult *service.OpenAIForwardResult
		setActualUpstreamEndpoint(c, "")
		switch forwardTarget {
		case compatForwardOpenAI:
			// responses / chat_completions 上游：OpenAI 网关服务转换（目录模型按请求名转发）
			oaResult, err = h.openAIGatewayService.ForwardAsChatCompletions(c.Request.Context(), c, account, forwardBody, promptCacheKey)
		case compatForwardGemini:
			if h.geminiCompatService == nil {
				h.chatCompletionsErrorResponse(c, http.StatusBadGateway, "upstream_error", "Gemini compatibility service is not configured")
				if accountReleaseFunc != nil {
					accountReleaseFunc()
				}
				return
			}
			result, err = h.geminiCompatService.ForwardAsChatCompletions(c.Request.Context(), c, account, forwardBody)
			h.gatewayService.ObserveRelayKeyResult(account, reqModel, err)
		case compatForwardAntigravity:
			if h.antigravityGatewayService == nil {
				h.chatCompletionsErrorResponse(c, http.StatusBadGateway, "upstream_error", "Antigravity compatibility service is not configured")
				if accountReleaseFunc != nil {
					accountReleaseFunc()
				}
				return
			}
			setActualUpstreamEndpoint(c, EndpointAntigravityGenerateContent)
			result, err = h.antigravityGatewayService.ForwardAsChatCompletions(c.Request.Context(), c, account, forwardBody, parsedReq)
		default:
			result, err = h.gatewayService.ForwardAsChatCompletions(c.Request.Context(), c, account, forwardBody, parsedReq)
			h.gatewayService.ObserveRelayKeyResult(account, reqModel, err)
		}

		if accountReleaseFunc != nil {
			accountReleaseFunc()
		}

		// OpenAI 上游透传 cyber_policy 后的风控记录（未标记时 no-op）
		var cyberBlockBody []byte
		if service.GetOpsCyberPolicy(c) != nil {
			cyberBlockBody = body
		}
		recordCyberPolicyIfMarked(c, h.cyberPolicyDeps(), apiKey, account, subscription, reqModel, err != nil, cyberBlockBody, clientRequestedModel(c, reqModel), service.HashUsageRequestPayload(body))

		// 入账：两个网关服务的结果类型不同，按转发实现二选一。
		submitForwardUsage := func(result *service.ForwardResult) {
			userAgent := c.GetHeader("User-Agent")
			clientIP := ip.GetClientIP(c)
			requestPayloadHash := service.HashUsageRequestPayload(body)
			inboundEndpoint := GetInboundEndpoint(c)
			upstreamEndpoint := GetUpstreamEndpoint(c, account.Platform)
			sessionID := service.ExtractClientSessionID(c)
			stampForwardRequestedReasoningEffort(result, service.RequestedReasoningEffortFromContext(c.Request.Context()))
			h.submitUsageRecordTask(c.Request.Context(), func(ctx context.Context) {
				if err := h.gatewayService.RecordUsage(ctx, &service.RecordUsageInput{
					Result:             result,
					APIKey:             apiKey,
					User:               apiKey.User,
					Account:            account,
					Subscription:       subscription,
					PricingAt:          pricingAt,
					InboundEndpoint:    inboundEndpoint,
					UpstreamEndpoint:   upstreamEndpoint,
					UserAgent:          userAgent,
					IPAddress:          clientIP,
					RequestPayloadHash: requestPayloadHash,
					APIKeyService:      h.apiKeyService,
					SessionID:          sessionID,
					RequestedModel:     clientRequestedModel(c, reqModel),
				}); err != nil {
					reqLog.Error("gateway.cc.record_usage_failed",
						zap.Int64("account_id", account.ID),
						zap.Error(err),
					)
				}
			})
		}
		submitOpenAIForwardUsage := func(res *service.OpenAIForwardResult) {
			stampOpenAIRequestedReasoningEffort(res, c)
			userAgent := c.GetHeader("User-Agent")
			clientIP := ip.GetClientIP(c)
			requestPayloadHash := service.HashUsageRequestPayload(body)
			inboundEndpoint := GetInboundEndpoint(c)
			upstreamEndpoint := resolveOpenAIUpstreamEndpoint(c, account, res)
			sessionID := service.ExtractClientSessionID(c)
			cyberBlocked := service.GetOpsCyberPolicy(c) != nil
			task := func(ctx context.Context) {
				if err := h.openAIGatewayService.RecordUsage(ctx, &service.OpenAIRecordUsageInput{
					Result:             res,
					APIKey:             apiKey,
					User:               apiKey.User,
					Account:            account,
					Subscription:       subscription,
					InboundEndpoint:    inboundEndpoint,
					UpstreamEndpoint:   upstreamEndpoint,
					UserAgent:          userAgent,
					IPAddress:          clientIP,
					RequestPayloadHash: requestPayloadHash,
					APIKeyService:      h.apiKeyService,
					SessionID:          sessionID,
					RequestedModel:     clientRequestedModel(c, reqModel),
					PricingAt:          pricingAt,
					CyberBlocked:       cyberBlocked,
				}); err != nil {
					reqLog.Error("gateway.cc.record_openai_usage_failed", zap.Int64("account_id", account.ID), zap.Error(err))
				}
			}
			// 媒体 / 搜索 / 语音结果不能因池溢出丢单
			if res.ImageCount > 0 || res.VideoCount > 0 || !res.WebSearch.IsZero() || res.AudioUsage != nil {
				h.submitMandatoryUsageRecordTask(c.Request.Context(), task)
			} else {
				h.submitUsageRecordTask(c.Request.Context(), task)
			}
		}
		submitAttemptUsage := func() {
			if oaResult != nil {
				submitOpenAIForwardUsage(oaResult)
				return
			}
			if result != nil {
				submitForwardUsage(result)
			}
		}

		if err != nil {
			// 上游协议表达不了的内容分片：400，不换号、不计账号健康（见 handleUnsupportedContentError）
			if h.handleUnsupportedContentError(c, reqLog, account, err,
				c.Writer.Size() != writerSizeBeforeForward, streamStarted || c.Writer.Written(), h.chatCompletionsErrorResponse) {
				return
			}
			var failoverErr *service.UpstreamFailoverError
			if errors.As(err, &failoverErr) {
				if c.Writer.Size() != writerSizeBeforeForward {
					if forwardTarget == compatForwardOpenAI {
						h.openAIGatewayService.ObserveOpenAIAccountHealthFailure(c.Request.Context(), account, err)
					}
					h.handleCCFailoverExhausted(c, failoverErr, service.ErrorPassthroughRulePlatform(account, requestPlatform), true)
					return
				}
				if forwardTarget == compatForwardOpenAI && failoverErr.ShouldReportAccountScheduleFailure() {
					h.openAIGatewayService.ObserveOpenAIAccountResult(account, openAIAccountScheduleModel(c, account, reqModel, nil), false, err)
				}
				switchCountBefore := fs.SwitchCount
				action := fs.HandleFailoverError(c.Request.Context(), h.gatewayService, account, account.GetPoolModeRetryCount(), failoverErr)
				switch action {
				case FailoverContinue:
					// OAuth 429 风暴刹车：只在真正换号（不是同账号重试）后判断
					if fs.SwitchCount > switchCountBefore && h.openAIGatewayService.ShouldStopOpenAIOAuth429Failover(account, failoverErr.StatusCode, fs.SwitchCount, &fs.OAuth429) {
						h.handleCCFailoverExhausted(c, failoverErr, service.ErrorPassthroughRulePlatform(account, requestPlatform), streamStarted)
						return
					}
					continue
				case FailoverExhausted:
					h.handleCCFailoverExhausted(c, fs.LastFailoverErr, service.ErrorPassthroughRulePlatform(account, requestPlatform), streamStarted)
					return
				case FailoverCanceled:
					failoverClientGone(c)
					return
				}
			}
			if forwardTarget == compatForwardOpenAI {
				h.openAIGatewayService.ObserveOpenAIAccountResult(account, openAIAccountScheduleModel(c, account, reqModel, oaResult), false, err)
			}
			var upstreamErrorAlreadyCommunicated bool
			if forwardTarget == compatForwardOpenAI {
				upstreamErrorAlreadyCommunicated = openAIForwardErrorAlreadyCommunicated(c, writerSizeBeforeForward, err)
			} else {
				upstreamErrorAlreadyCommunicated = gatewayForwardErrorAlreadyCommunicated(c, writerSizeBeforeForward, err)
			}
			wroteFallback := false
			if !upstreamErrorAlreadyCommunicated {
				if forwardTarget == compatForwardOpenAI {
					wroteFallback = ensureOpenAIStreamReadErrorResponse(c, err, streamStarted)
				}
				if !wroteFallback {
					wroteFallback = h.ensureForwardErrorResponse(c, streamStarted)
				}
			}
			reqLog.Error("gateway.cc.forward_failed",
				zap.Int64("account_id", account.ID),
				zap.Bool("fallback_error_response_written", wroteFallback),
				zap.Bool("upstream_error_response_already_written", upstreamErrorAlreadyCommunicated),
				zap.Error(err),
			)
			// 错误返回携带的部分结果（流中断前上游已计量的 usage）照常入账；failover 错误恒定无结果。
			submitAttemptUsage()
			return
		}

		if oaResult != nil {
			// key 健康熔断 / 调度统计的成功观测
			h.openAIGatewayService.ObserveOpenAIAccountResult(account, openAIAccountScheduleModel(c, account, reqModel, oaResult), true)
		}
		// 6. Record usage
		submitAttemptUsage()
		return
	}
}

// chatCompletionsErrorResponse writes an error in OpenAI Chat Completions format.
func (h *GatewayHandler) chatCompletionsErrorResponse(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}

// handleCCFailoverExhausted 换号耗尽：分类同三个入站，按 Chat Completions 形状写；流已开始时写 SSE 错误帧。
func (h *GatewayHandler) handleCCFailoverExhausted(c *gin.Context, lastErr *service.UpstreamFailoverError, platform string, streamStarted bool) {
	resp := classifyFailoverExhausted(c, lastErr, exhaustedClassifyOptions{
		Platform:       platform,
		Passthrough:    h.errorPassthroughService,
		MapUpstream:    openAIMapUpstreamError,
		StreamStarted:  streamStarted,
		RawUpstream400: true,
	})
	if resp.Written {
		return
	}
	h.handleStreamingAwareError(c, resp.Status, resp.ErrType, resp.Message, streamStarted)
}
