package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// Responses handles OpenAI Responses API endpoint for Anthropic platform groups.
// POST /v1/responses
// This converts Responses API requests to Anthropic format, forwards to Anthropic
// upstream, and converts responses back to Responses format.
func (h *GatewayHandler) Responses(c *gin.Context) {
	streamStarted := false
	defer func() {
		recoverForwardPanic(c, &streamStarted, recover(), h.ensureForwardErrorResponse, "handler.gateway.responses", "gateway.responses_panic_recovered")
	}()
	requestStart := time.Now()
	defer logOpenAIRemoteCompactOutcome(c, h.cfg, requestStart)
	setOpenAIClientTransportHTTP(c)

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.responsesErrorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.responsesErrorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	reqLog := requestLogger(
		c,
		"handler.gateway.responses",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
	)

	// Read request body
	body, err := readLenientJSONRequestBodyWithPrealloc(c.Request, h.cfg)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.responsesErrorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		logRequestBodyReadFailure(reqLog, c.Request, err)
		h.responsesErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}

	if len(body) == 0 {
		h.responsesErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}

	setOpsRequestContext(c, "", false)
	// 粘性键与 cyber 拦截用归一前的原始 body；compact 归一可能改写请求体。
	sessionHashBody := body
	body, ok = normalizeOpenAIResponsesCompactRequest(c, reqLog, body)
	if !ok {
		return
	}
	legacyCompact := service.IsOpenAIResponsesCompactPath(c)
	nativeV2 := isBareOpenAIResponsesPath(c) && isOpenAIRemoteCompactionV2Request(body)
	if nativeV2 {
		// 原生 v2 压缩出站前补注 x-codex-beta-features: remote_compaction_v2（网关链剥头后本级负责恢复）
		service.MarkOpenAINativeCompactionV2(c)
	}
	// body-signal compact：上游 unary 等待期间向下游发 SSE 注释行心跳，防止反向代理空闲超时掐断长压缩连接
	stopCompactKeepalive := service.StartOpenAICompactSSEKeepalive(c, openAICompactKeepaliveInterval(h.cfg))
	defer stopCompactKeepalive()

	// Validate JSON
	if !gjson.ValidBytes(body) {
		logRequestBodyParseFailure(reqLog, body, nil)
		h.responsesErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return
	}

	// Extract model and stream using gjson (like OpenAI handler)
	modelResult := gjson.GetBytes(body, "model")
	if !modelResult.Exists() || modelResult.Type != gjson.String || modelResult.String() == "" {
		h.responsesErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	reqModel := modelResult.String()
	bindRequestedReasoningEffort(c, body, reqModel)
	if normalizedBody, changed := normalizeCodexAutomationBootstrap(body); changed {
		body = normalizedBody
		reqLog.Info("gateway.responses.codex_automation_bootstrap_normalized", zap.String("normalization", "call_output_to_user_message"))
	}
	if normalizedBody, changed := normalizeCodexDelegationBootstrap(body); changed {
		body = normalizedBody
		reqLog.Info("gateway.responses.codex_delegation_bootstrap_normalized", zap.String("normalization", "call_output_to_user_message"))
	}
	reqStream, ok := parseOpenAICompatibleStream(body)
	if !ok {
		h.responsesErrorResponse(c, http.StatusBadRequest, "invalid_request_error", invalidStreamFieldTypeMessage)
		return
	}
	if _, err := service.ValidateOpenAIServiceTierField(body); err != nil {
		h.responsesErrorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	// 生图未开放：带 image_generation 工具的语言模型请求在选号前处理（Codex 官方客户端剥掉工具，
	// 其余客户端 400），不打上游、不算账号失败。
	body, ok = gateOpenAIImageGenerationTool(c, h.cfg, reqLog, reqModel, body, h.responsesErrorResponse)
	if !ok {
		return
	}
	reqLog = reqLog.With(zap.String("model", reqModel), zap.Bool("stream", reqStream))
	// previous_response_id：只认 resp_*，且必须是本用户的续链
	previousResponseID := strings.TrimSpace(gjson.GetBytes(body, "previous_response_id").String())
	if previousResponseID != "" {
		previousResponseIDKind := service.ClassifyOpenAIPreviousResponseIDKind(previousResponseID)
		reqLog = reqLog.With(zap.Bool("has_previous_response_id", true), zap.String("previous_response_id_kind", previousResponseIDKind))
		if previousResponseIDKind == service.OpenAIPreviousResponseIDKindMessageID {
			reqLog.Warn("gateway.responses.request_validation_failed", zap.String("reason", "previous_response_id_looks_like_message_id"))
			h.responsesErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "previous_response_id must be a response.id (resp_*), not a message id")
			return
		}
		owned, ownershipErr := h.openAIGatewayService.ValidateOpenAIHTTPResponseOwner(c.Request.Context(), service.SchedulingScopeID(c.Request.Context()), previousResponseID, subject.UserID, apiKey.ID)
		if ownershipErr != nil {
			reqLog.Warn("gateway.responses.previous_response_owner_lookup_failed", zap.Error(ownershipErr))
		}
		if !owned {
			reqLog.Warn("gateway.responses.request_validation_failed", zap.String("reason", "previous_response_owner_mismatch"))
			h.responsesErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "previous_response_id is not available for this user")
			return
		}
	}
	service.SetOpenAIHTTPResponseOwner(c, subject.UserID, apiKey.ID)

	setOpsRequestContext(c, reqModel, reqStream)
	setOpsEndpointContext(c, "", int16(service.RequestTypeFromLegacy(reqStream, false)))
	requestCtx := c.Request.Context()
	// 定价上下文无条件装配：/v1/responses 是 token 计费端点，声明生图工具的
	// 混合请求同样按 token 计费（外加图片部分），其 token 利润保护不因请求体
	// 里的任何工具声明（含 Codex 被动 image_gen namespace）而关闭。生图意图
	// 仅用于能力路由与图片计费；独立图片/视频端点才在利润门范围之外。
	requestCtx, pricingAt := service.WithGatewayTokenRequestPricing(requestCtx)
	// 显式生图意图（排除 Codex 被动 image_gen namespace）：能力路由与图片计费用；分组开关关着则 403
	imageIntent := service.IsExplicitImageGenerationIntent("/v1/responses", reqModel, body)
	if imageIntent {
		requestCtx = service.WithOpenAIImageGenerationIntent(requestCtx)
	}
	c.Request = c.Request.WithContext(requestCtx)

	service.SetOpenAIImageIntentHint(c, imageIntent)
	// 提前校验 function_call_output 是否具备可关联上下文，避免上游 400
	if !validateFunctionCallOutputRequest(c, body, reqLog) {
		return
	}

	if decision := h.checkSecurityAudit(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIResponses, reqModel, body); decision != nil && !decision.AllowNextStage {
		h.responsesSecurityAuditError(c, decision)
		return
	}
	if rejectIfCyberSessionBlocked(c, h.cyberPolicyDeps(), apiKey, sessionHashBody, reqModel, cyberBlockFormatResponses) {
		return
	}
	c.Request = c.Request.WithContext(service.WithOpenAIGuardianParentAffinity(c.Request.Context(), c, sessionHashBody, reqModel))

	// Error passthrough binding
	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)

	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())

	userReleaseFunc, err := h.concurrencyHelper.AcquireUserSlotWithWait(c, subject.UserID, subject.Concurrency, reqStream, &streamStarted)
	if err != nil {
		reqLog.Warn("gateway.responses.user_slot_acquire_failed", zap.Error(err))
		h.handleConcurrencyError(c, err, "user", streamStarted)
		return
	}
	userReleaseFunc = wrapReleaseOnDone(c.Request.Context(), userReleaseFunc)
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}
	if imageIntent {
		imageReleaseFunc, imageAcquired := acquireImageGenerationSlot(c, h.cfg, h.imageLimiter, streamStarted)
		if !imageAcquired {
			return
		}
		if imageReleaseFunc != nil {
			defer imageReleaseFunc()
		}
	}

	// 2. Re-check billing
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, subscription); err != nil {
		reqLog.Info("gateway.responses.billing_check_failed", zap.Error(err))
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.responsesErrorResponse(c, status, code, message)
		return
	}

	// Parse request for session hash
	bodyRef := service.NewRequestBodyRef(body)
	parsedReq, _ := service.ParseGatewayRequest(bodyRef, "responses")
	if parsedReq == nil {
		parsedReq = &service.ParsedRequest{Model: reqModel, Stream: reqStream, Body: bodyRef}
	}
	parsedReq.SessionContext = &service.SessionContext{
		ClientIP:  ip.GetClientIP(c),
		UserAgent: c.GetHeader("User-Agent"),
		APIKeyID:  apiKey.ID,
	}
	// 粘性键按 OpenAI 协议派生：会话头 / prompt_cache_key / 稳定的内容摘要（对 anthropic 池同样生效）。
	sessionHash := h.openAIGatewayService.GenerateSessionHash(c, sessionHashBody)
	requireCompact := legacyCompact
	requestPlatform := service.OpenAICompatibleRequestPlatform(c.Request.Context())
	// 生图 / compact / 原生 v2 压缩必须调度到确实提供 Responses 的资源
	capability := openAIResponsesRequiredCapabilityForRequest(imageIntent, nativeV2 || legacyCompact, requestPlatform)
	// 续链与守护父线程亲和都是「已绑定的资源」：做成预取粘性，选号时优先于缓存里的会话绑定。
	stickyID := int64(0)
	if previousResponseID != "" {
		stickyID = h.openAIGatewayService.ResolveAccountIDByPreviousResponseIDForScheduler(c.Request.Context(), previousResponseID, reqModel, nil, capability, requireCompact)
	}
	if stickyID == 0 {
		stickyID = h.openAIGatewayService.ResolveOpenAIGuardianParentAccountID(c.Request.Context())
	}
	if stickyID > 0 {
		c.Request = c.Request.WithContext(service.WithPrefetchedStickySession(c.Request.Context(), stickyID, service.SchedulingScopeID(c.Request.Context()), h.metadataBridgeEnabled()))
	}
	requestCtx = c.Request.Context()

	// 3. Account selection + failover loop
	fs := NewFailoverState(h.maxAccountSwitches, false)
	firstOutputSwitches := 0

	for {
		if requestCtx.Err() != nil {
			return
		}
		selection, err := h.gatewayService.SelectAccountWithOptions(requestCtx, sessionHash, reqModel, fs.FailedAccountIDs, service.SelectOptions{Capability: capability, RequireCompact: requireCompact})
		if err != nil {
			if len(fs.FailedAccountIDs) == 0 {
				if legacyCompact && errors.Is(err, service.ErrNoAvailableCompactAccounts) {
					markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
					h.responsesErrorResponse(c, http.StatusServiceUnavailable, "compact_not_supported", "No available accounts support /responses/compact")
					return
				}
				cls := classifyNoAccountErrorFromGin(c, h.gatewayService, reqModel, reqModel, requestPlatform)
				cls = classifySelectionFailureError(err, cls)
				if !cls.ModelNotFound {
					markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
				}
				message := cls.Message
				if !cls.ModelNotFound {
					message = "No available accounts: " + err.Error()
				}
				h.responsesErrorResponse(c, cls.Status, cls.ErrType, message)
				return
			}
			if fs.HandleSelectionExhausted(requestCtx) == FailoverCanceled {
				failoverClientGone(c)
				return
			}
			if fs.LastFailoverErr != nil {
				h.handleResponsesFailoverExhausted(c, fs.LastFailoverErr, requestPlatform, streamStarted)
			} else {
				h.responsesErrorResponse(c, http.StatusBadGateway, "server_error", "All available accounts exhausted")
			}
			return
		}
		account := selection.Account
		if previousResponseID != "" && requestPlatform == service.PlatformOpenAI && !service.AccountKeepsHTTPPreviousResponseID(account) {
			// HTTP 续链只有以 responses 协议直连的官方 / 通用 key 承接；成品号的续链状态挂在 WS v2 会话上。
			// 混合池里换下一个而不是静默丢掉续链状态。
			fs.FailedAccountIDs[account.ID] = struct{}{}
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			fs.LastFailoverErr = &service.UpstreamFailoverError{
				StatusCode:       http.StatusBadRequest,
				Stage:            service.GatewayFailureStageInference,
				Scope:            service.GatewayFailureScopeRequest,
				Reason:           service.OpenAIHTTPContinuationUnsupportedReason,
				ClientStatusCode: http.StatusBadRequest,
				ClientMessage:    "previous_response_id requires an OpenAI API-key account for HTTP requests",
			}
			reqLog.Debug("gateway.responses.account_skipped_http_continuation_unsupported", zap.Int64("account_id", account.ID), zap.String("account_type", account.Type))
			continue
		}
		setOpsSelectedAccount(c, account.ID, account.Platform)

		// 4. Acquire account concurrency slot
		accountReleaseFunc := selection.ReleaseFunc
		if !selection.Acquired {
			if selection.WaitPlan == nil {
				markOpsRoutingCapacityLimited(c)
				h.responsesErrorResponse(c, http.StatusServiceUnavailable, "api_error", "No available accounts")
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
				reqLog.Warn("gateway.responses.account_slot_acquire_failed", zap.Int64("account_id", account.ID), zap.Error(err))
				h.handleConcurrencyError(c, err, "account", streamStarted)
				return
			}
		}
		// 终检与准入后绑定必须使用选号结果携带的门：门安装在调度栈的局部
		// ctx 上（composite/fallback 还可能解析出与入口分组不同的门），直接用
		// requestCtx 会退化为空操作。
		admissionCtx := service.ContextWithSelectionProfitGate(requestCtx, selection)
		latest, vetoed, reason := h.gatewayService.GatewayProfitControlVetoLatest(admissionCtx, account)
		if vetoed {
			if accountReleaseFunc != nil {
				accountReleaseFunc()
			}
			reqLog.Debug("gateway.responses.account_slot_profit_vetoed", zap.Int64("account_id", account.ID), zap.String("reason", reason))
			if fs.RecordProfitVeto(account.ID) == FailoverExhausted {
				reqLog.Warn("gateway.responses.profit_veto_attempts_exhausted", zap.Int("profit_veto_count", fs.ProfitVetoCount()))
				h.responsesErrorResponse(c, http.StatusServiceUnavailable, "api_error", profitVetoExhaustedMessage)
				return
			}
			continue
		}
		account = latest
		selection.Account = latest
		if selection.ProfitGateActive() {
			if err := h.gatewayService.BindStickySessionAfterProfitAdmission(admissionCtx, sessionHash, account.ID); err != nil {
				reqLog.Warn("gateway.responses.bind_sticky_session_after_profit_admission_failed", zap.Int64("account_id", account.ID), zap.Error(err))
			}
		}
		accountReleaseFunc = wrapReleaseOnDone(c.Request.Context(), accountReleaseFunc)

		forwardTarget := responsesForwardTarget(account)
		if forwardTarget == compatForwardSkip {
			if accountReleaseFunc != nil {
				accountReleaseFunc()
			}
			reqLog.Warn("gateway.responses.key_protocol_unavailable",
				zap.Int64("account_id", account.ID),
				zap.String("request_platform", requestPlatform),
			)
			fs.FailedAccountIDs[account.ID] = struct{}{}
			continue
		}

		// 5. Forward request
		// 扣除非语义心跳字节的口径快照：心跳注释不构成语义响应，不能因心跳字节变化而放弃 failover 换号
		writerSizeBeforeForward := service.OpenAICompactKeepaliveAdjustedWrittenSize(c)
		forwardBody := body
		var result *service.ForwardResult
		var oaResult *service.OpenAIForwardResult
		setActualUpstreamEndpoint(c, "")
		switch forwardTarget {
		case compatForwardOpenAI:
			oaResult, err = h.openAIGatewayService.Forward(requestCtx, c, account, forwardBody)
		case compatForwardAntigravity:
			if h.antigravityGatewayService == nil {
				h.responsesErrorResponse(c, http.StatusBadGateway, "upstream_error", "Antigravity compatibility service is not configured")
				if accountReleaseFunc != nil {
					accountReleaseFunc()
				}
				return
			}
			setActualUpstreamEndpoint(c, EndpointAntigravityGenerateContent)
			result, err = h.antigravityGatewayService.ForwardAsResponses(requestCtx, c, account, forwardBody, parsedReq)
		default:
			result, err = h.gatewayService.ForwardAsResponses(requestCtx, c, account, forwardBody, parsedReq)
		}

		if accountReleaseFunc != nil {
			accountReleaseFunc()
		}

		// OpenAI 上游透传 cyber_policy 后的风控记录（未标记时 no-op）
		var cyberBlockBody []byte
		if service.GetOpsCyberPolicy(c) != nil {
			cyberBlockBody = sessionHashBody
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
					reqLog.Error("gateway.responses.record_usage_failed",
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
					NativeCompactionV2: nativeV2,
				}); err != nil {
					reqLog.Error("gateway.responses.record_openai_usage_failed", zap.Int64("account_id", account.ID), zap.Error(err))
				}
			}
			// 媒体 / 搜索 / 语音结果不能因池溢出丢单
			if res.ImageCount > 0 || res.VideoCount > 0 || res.SearchCount > 0 || res.WebSearchCalls > 0 || res.AudioUsage != nil {
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
			// 客户端断开：断开排水期间上游已计量的 usage 照常入账
			if (oaResult != nil && oaResult.ClientDisconnect) || failoverClientGone(c) {
				reqLog.Info("gateway.responses.client_disconnected", zap.Int64("account_id", account.ID), zap.Error(err))
				submitAttemptUsage()
				return
			}
			// 上游协议表达不了的内容分片：400，不换号、不计账号健康（见 handleUnsupportedContentError）。
			// 口径与 failover 判定相同：扣除 compact 心跳字节；心跳提交的 200 由 responsesErrorResponse 降级处理。
			if h.handleUnsupportedContentError(c, reqLog, account, err,
				service.OpenAICompactKeepaliveAdjustedWrittenSize(c) != writerSizeBeforeForward, streamStarted, h.responsesErrorResponse) {
				return
			}
			var failoverErr *service.UpstreamFailoverError
			if errors.As(err, &failoverErr) {
				// 已写出语义字节（心跳不算）就不能换号；SafeToFailoverAfterWrite 的错误除外
				if !openAIForwardMayFailover(c, writerSizeBeforeForward, failoverErr) {
					if forwardTarget == compatForwardOpenAI {
						h.openAIGatewayService.ObserveOpenAIAccountHealthFailure(c.Request.Context(), account, err)
					}
					h.handleResponsesFailoverExhausted(c, failoverErr, service.ErrorPassthroughRulePlatform(account, requestPlatform), true)
					return
				}
				// 写出的字节不含语义输出，但重试耗尽时仍须按已提交的 SSE 响应返回流内错误
				if c.Writer.Written() {
					streamStarted = true
				}
				if forwardTarget == compatForwardOpenAI && failoverErr.ShouldReportAccountScheduleFailure() {
					h.openAIGatewayService.ObserveOpenAIAccountResult(account, openAIAccountScheduleModel(c, account, reqModel, nil), false, err)
				}
				if openAIFirstOutputFailoverExhausted(failoverErr, &firstOutputSwitches) {
					h.handleResponsesFailoverExhausted(c, failoverErr, service.ErrorPassthroughRulePlatform(account, requestPlatform), streamStarted)
					return
				}
				switchCountBefore := fs.SwitchCount
				action := fs.HandleFailoverError(requestCtx, h.gatewayService, account, account.GetPoolModeRetryCount(), failoverErr)
				switch action {
				case FailoverContinue:
					// OAuth 429 风暴刹车：只在真正换号（不是同账号重试）后判断
					if fs.SwitchCount > switchCountBefore && h.openAIGatewayService.ShouldStopOpenAIOAuth429Failover(account, failoverErr.StatusCode, fs.SwitchCount, &fs.OAuth429) {
						h.handleResponsesFailoverExhausted(c, failoverErr, service.ErrorPassthroughRulePlatform(account, requestPlatform), streamStarted)
						return
					}
					continue
				case FailoverExhausted:
					h.handleResponsesFailoverExhausted(c, fs.LastFailoverErr, service.ErrorPassthroughRulePlatform(account, requestPlatform), streamStarted)
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
				wroteFallback = h.ensureForwardErrorResponse(c, streamStarted)
			}
			fields := []zap.Field{
				zap.Int64("account_id", account.ID),
				zap.Bool("fallback_error_response_written", wroteFallback),
				zap.Bool("upstream_error_response_already_written", upstreamErrorAlreadyCommunicated),
				zap.Error(err),
			}
			// 错误返回携带的部分结果（流中断前上游已计量的 usage）照常入账；failover 错误恒定无结果。
			submitAttemptUsage()
			if shouldLogOpenAIForwardFailureAsWarn(c, wroteFallback) {
				reqLog.Warn("gateway.responses.forward_failed", fields...)
				return
			}
			reqLog.Error("gateway.responses.forward_failed", fields...)
			return
		}

		if oaResult != nil {
			// 排除 spark 影子：其 codex_* 仅由 QueryUsage 更新
			if account.Type == service.AccountTypeOAuth && !account.IsShadow() {
				h.openAIGatewayService.UpdateCodexUsageSnapshotFromHeaders(c.Request.Context(), account.ID, oaResult.ResponseHeaders)
			}
			// key 健康熔断 / 调度统计的成功观测
			h.openAIGatewayService.ObserveOpenAIAccountResult(account, openAIAccountScheduleModel(c, account, reqModel, oaResult), openAIForwardSucceededForScheduling(oaResult))
		}
		// 6. Record usage
		submitAttemptUsage()
		return
	}
}

// responsesErrorResponse 按 OpenAI Responses 形状写错误（error.type + error.message）。
// body-signal compact 心跳可能已把响应头提交为 200：JSON 错误体会与已提交的 SSE 流交错，
// 必须降级为 response.failed 终止事件（#3887）。
func (h *GatewayHandler) responsesErrorResponse(c *gin.Context, status int, errType, message string) {
	if service.StopOpenAICompactSSEKeepaliveCommitted(c) {
		service.MarkOpsStreamError(c, errType, message, status)
		if writeResponsesFailedSSE(c, errType, "", message) {
			return
		}
	}
	c.JSON(status, gin.H{
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}

// handleResponsesFailoverExhausted 换号耗尽：分类同三个入站，按 Responses 形状写；流已开始时补 response.failed。
func (h *GatewayHandler) handleResponsesFailoverExhausted(c *gin.Context, lastErr *service.UpstreamFailoverError, platform string, streamStarted bool) {
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
	if streamStarted {
		// A slot-wait heartbeat commits HTTP 200 before any upstream response.
		// In that case a terminal frame is still required; once any semantic or
		// official terminal bytes exist, preserve them without appending a second
		// generic response.failed.
		service.MarkOpsStreamError(c, resp.ErrType, resp.Message, resp.Status)
		if c != nil && c.Writer != nil && (c.Writer.Size() <= 0 || gatewayStreamHasOnlyHeartbeats(c)) {
			writeResponsesFailedSSE(c, resp.ErrType, "", resp.Message)
		}
		return
	}
	h.responsesErrorResponse(c, resp.Status, resp.ErrType, resp.Message)
}
