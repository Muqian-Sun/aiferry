package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// 视频任务框架的入口：三种客户端形态（OpenAI Videos / xAI / new-api）共用一套创建、查询、取成片与计费。
// 创建按目录条目选号并 failover；查询 / 取成片按任务记录直接找回承接账号，不经调度器（不记调度轨迹）。
// 计费只在首次看到成功时发生一次（video_tasks.billed_at 抢占），创建不计费。

// VideoTaskCreate 创建视频任务，shape 是这个入口的客户端形态。
func (h *OpenAIGatewayHandler) VideoTaskCreate(shape service.VideoInboundShape) gin.HandlerFunc {
	return func(c *gin.Context) { h.createVideoTask(c, shape) }
}

// VideoTaskStatus 查询视频任务。pathShape 是这条查询路径所属的形态（/v1/videos/{id} 记 openai，
// 回包形态见 service.VideoStatusRenderShape）；不是本网关任务 ID 的交给旧的 Grok 查询。
func (h *OpenAIGatewayHandler) VideoTaskStatus(pathShape service.VideoInboundShape) gin.HandlerFunc {
	return func(c *gin.Context) { h.videoTaskStatus(c, pathShape) }
}

// VideoTaskContent 取成片（流式代理上游地址，不落盘）；不是本网关任务 ID 的交给旧的 Grok 内容下载。
func (h *OpenAIGatewayHandler) VideoTaskContent(c *gin.Context) {
	id := videoTaskIDParam(c)
	if !service.IsVideoTaskID(id) {
		h.GrokVideoContent(c)
		return
	}
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)
	task, apiKey, reqLog, ok := h.loadVideoTask(c, id)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	result, account, err := h.videoTasks.Poll(ctx, c, task)
	if err != nil {
		reqLog.Warn("video_task.content_poll_failed", zap.Error(err))
		if task.State != service.VideoTaskSucceeded {
			writeVideoTaskError(c, http.StatusBadGateway, "upstream_error", "Failed to query the video task")
			return
		}
		result = service.VideoTaskResult{State: task.State, VideoURL: task.VideoURL}
		if account, err = h.videoTasks.LoadAccount(ctx, task.AccountID); err != nil {
			writeVideoTaskError(c, http.StatusBadGateway, "upstream_error", "The account that created this video is unavailable")
			return
		}
	}
	switch task.State {
	case service.VideoTaskSucceeded:
	case service.VideoTaskFailed, service.VideoTaskCanceled:
		writeVideoTaskError(c, http.StatusBadRequest, "invalid_request_error", "Video generation did not succeed: "+firstNonEmptyString(task.ErrorMessage, string(task.State)))
		return
	default:
		writeVideoTaskError(c, http.StatusBadRequest, "invalid_request_error", "Video is not ready yet (status: "+string(task.State)+")")
		return
	}
	h.billVideoTaskOnce(c, reqLog, apiKey, task, account)
	if err := h.videoTasks.StreamContent(ctx, c, task, account, result); err != nil {
		reqLog.Warn("video_task.content_stream_failed", zap.Error(err))
		if !c.Writer.Written() {
			writeVideoTaskError(c, http.StatusBadGateway, "upstream_error", "Failed to download the video")
		}
	}
}

func videoTaskIDParam(c *gin.Context) string {
	return strings.TrimSpace(firstNonEmptyString(c.Param("request_id"), c.Param("task_id")))
}

func writeVideoTaskError(c *gin.Context, status int, errType, message string) {
	if c.Writer.Written() {
		return
	}
	c.JSON(status, gin.H{"error": gin.H{"type": errType, "message": message}})
}

func writeVideoTaskClientError(c *gin.Context, err *service.VideoTaskClientError) {
	errType := strings.TrimSpace(err.Type)
	if errType == "" {
		errType = "invalid_request_error"
	}
	writeVideoTaskError(c, err.StatusCode, errType, err.Message)
}

func (h *OpenAIGatewayHandler) createVideoTask(c *gin.Context, shape service.VideoInboundShape) {
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)

	requestStart := time.Now()
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		writeVideoTaskError(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		writeVideoTaskError(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	reqLog := requestLogger(c, "handler.openai_gateway.video_task",
		zap.Int64("user_id", subject.UserID), zap.Int64("api_key_id", apiKey.ID), zap.String("shape", string(shape)))
	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}
	if h.videoTasks == nil {
		writeVideoTaskError(c, http.StatusServiceUnavailable, "api_error", "Service temporarily unavailable")
		return
	}

	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			writeVideoTaskError(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		writeVideoTaskError(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(body) == 0 {
		writeVideoTaskError(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}
	req, err := service.ParseVideoTaskCreate(shape, c.GetHeader("Content-Type"), body)
	if err != nil {
		var clientErr *service.VideoTaskClientError
		if errors.As(err, &clientErr) {
			writeVideoTaskClientError(c, clientErr)
			return
		}
		writeVideoTaskError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	route, ok := service.CatalogRouteFromContext(c.Request.Context())
	if !ok || route.Entry == nil {
		writeVideoTaskError(c, http.StatusNotFound, "not_found_error", fmt.Sprintf("Model %q is not available", req.Model))
		return
	}
	// 视频按目录的按次价计费；token 计价的条目算不出视频价，不能放行（否则成片白送）。
	if !route.Entry.IsPerRequestBilled() {
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		writeVideoTaskError(c, http.StatusNotFound, "not_found_error", fmt.Sprintf("Model %q does not support video generation", req.Model))
		return
	}
	req.Model = route.CanonicalModel
	vendor := service.VideoTaskVendorFor(route.Entry)
	reqLog = reqLog.With(zap.String("model", req.Model), zap.String("vendor", vendor))
	setOpsRequestContext(c, req.Model, false)
	setOpsEndpointContext(c, "", int16(service.RequestTypeSync))

	if moderationBody := req.ModerationBody(); len(moderationBody) > 0 {
		decision := h.checkSecurityAudit(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIImages, req.Model, moderationBody)
		if decision != nil && !decision.AllowNextStage {
			h.openAISecurityAuditError(c, decision)
			return
		}
	}
	mediaReleaseFunc, acquired := h.acquireImageGenerationSlot(c, streamStarted)
	if !acquired {
		return
	}
	if mediaReleaseFunc != nil {
		defer mediaReleaseFunc()
	}
	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}
	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())

	userReleaseFunc, acquired := h.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, false, &streamStarted, reqLog)
	if !acquired {
		return
	}
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, subscription); err != nil {
		reqLog.Info("video_task.billing_eligibility_check_failed", zap.Error(err))
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		writeVideoTaskError(c, status, code, message)
		return
	}

	// 视频与 Grok 媒体一样按目录按次价计费，豁免按 token 价算的利润门（同 handleGrokMedia）。
	requestCtx := service.WithOpenAIProfitControlSuppressed(c.Request.Context())
	opts := service.SelectOptions{
		Capability:  service.OpenAIEndpointCapabilityVideoGeneration,
		Transport:   service.OpenAIUpstreamTransportHTTPSSE,
		VideoVendor: vendor,
	}
	failedAccountIDs := make(map[int64]struct{})
	var lastFailoverErr *service.UpstreamFailoverError
	profitVetoCount := 0
	switchCount := 0
	maxAccountSwitches := h.maxAccountSwitches
	if maxAccountSwitches <= 0 {
		maxAccountSwitches = 3
	}
	routingStart := time.Now()
	var accountReleaseFunc func()
	releaseAccount := func() {
		if accountReleaseFunc != nil {
			accountReleaseFunc()
			accountReleaseFunc = nil
		}
	}
	defer releaseAccount()

	for {
		releaseAccount()
		if failoverClientGone(c) {
			return
		}
		selection, err := h.gatewayService.Scheduler().SelectAccountWithOptions(requestCtx, "", route.RequestedModel, failedAccountIDs, opts)
		if selection != nil && selection.Acquired {
			selection.ReleaseFunc = wrapReleaseOnDone(requestCtx, selection.ReleaseFunc)
			accountReleaseFunc = selection.ReleaseFunc
		}
		if err != nil || selection == nil || selection.Account == nil {
			if failoverClientGone(c) {
				return
			}
			reqLog.Warn("video_task.account_select_failed", zap.Error(err), zap.Int("excluded_account_count", len(failedAccountIDs)))
			if lastFailoverErr != nil {
				h.handleFailoverExhausted(c, lastFailoverErr, false)
				return
			}
			if err != nil {
				markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
			} else {
				markOpsRoutingCapacityLimited(c)
			}
			writeVideoTaskError(c, http.StatusServiceUnavailable, "video_no_eligible_account", "No available accounts for video generation")
			return
		}
		account := selection.Account
		// 成品号的 xAI 付费资格探测（与 Grok 媒体同一套）；第三方 key 没有这个资格面。
		if !account.IsThirdPartyKey() {
			if eligible, reason, probeErr := h.ensureGrokMediaAccountEligibility(requestCtx, account); !eligible {
				releaseAccount()
				failedAccountIDs[account.ID] = struct{}{}
				reqLog.Warn("video_task.account_eligibility_rejected", zap.Int64("account_id", account.ID),
					zap.String("reason", reason), zap.Bool("probe_failed", probeErr != nil))
				if switchCount >= maxAccountSwitches {
					markOpsRoutingCapacityLimited(c)
					writeVideoTaskError(c, http.StatusServiceUnavailable, "video_no_eligible_account", "No available accounts for video generation")
					return
				}
				switchCount++
				continue
			}
		}
		if failoverClientGone(c) {
			return
		}
		setOpsSelectedAccount(c, account.ID, account.Platform)
		var slotResult openAISlotAcquireResult
		accountReleaseFunc, slotResult = h.acquireResponsesAccountSlot(c, "", selection, false, &streamStarted, reqLog)
		if slotResult == openAISlotAcquireProfitVetoed {
			if !recordOpenAIProfitVeto(failedAccountIDs, account.ID, &profitVetoCount) {
				h.handleOpenAIProfitVetoExhausted(c, streamStarted, reqLog, profitVetoCount)
				return
			}
			continue
		}
		if slotResult != openAISlotAcquireOK {
			return
		}
		service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(routingStart).Milliseconds())

		submission, err := func() (service.VideoTaskSubmission, error) {
			defer releaseAccount()
			return h.videoTasks.Submit(requestCtx, c, account, route.Entry, req)
		}()
		if err != nil {
			var clientErr *service.VideoTaskClientError
			var failoverErr *service.UpstreamFailoverError
			switch {
			case errors.As(err, &clientErr):
				writeVideoTaskClientError(c, clientErr)
				return
			case errors.As(err, &failoverErr):
				if failoverClientGone(c) {
					return
				}
				if failoverErr.ShouldReportAccountScheduleFailure() {
					h.gatewayService.ObserveOpenAIAccountResult(account, account.GetMappedModel(req.Model), false)
				}
				if !failoverErr.ShouldRetryNextAccount() {
					h.handleFailoverExhausted(c, failoverErr, false)
					return
				}
				failedAccountIDs[account.ID] = struct{}{}
				lastFailoverErr = failoverErr
				if switchCount >= maxAccountSwitches {
					h.handleFailoverExhausted(c, failoverErr, false)
					return
				}
				switchCount++
				reqLog.Warn("video_task.upstream_failover_switching", zap.Int64("account_id", account.ID),
					zap.Int("upstream_status", failoverErr.StatusCode), zap.Int("switch_count", switchCount))
				continue
			default:
				reqLog.Warn("video_task.create_failed", zap.Int64("account_id", account.ID), zap.Error(err))
				if !failoverClientGone(c) {
					writeVideoTaskError(c, http.StatusBadGateway, "upstream_error", "Upstream request failed")
				}
				return
			}
		}
		h.gatewayService.ObserveOpenAIAccountResult(account, submission.UpstreamModel, true)

		taskID, err := service.NewVideoTaskID()
		if err != nil {
			reqLog.Error("video_task.id_failed", zap.Error(err))
			writeVideoTaskError(c, http.StatusInternalServerError, "api_error", "Failed to record the video task")
			return
		}
		now := time.Now()
		task := &service.VideoTask{
			ID: taskID, UserID: subject.UserID, APIKeyID: apiKey.ID, AccountID: account.ID, CatalogEntryID: route.EntryID,
			Model: req.Model, UpstreamModel: submission.UpstreamModel, UpstreamKind: submission.UpstreamKind,
			UpstreamTaskID: submission.UpstreamTaskID, InboundShape: shape, Request: submission.Request,
			State: service.VideoTaskQueued, CreatedAt: now, UpdatedAt: now,
		}
		// 上游已接单：客户端此时断开也要把任务记下来，否则后续查询找不回、成片无法交付。
		if err := h.videoTasks.CreateTask(context.WithoutCancel(requestCtx), task); err != nil {
			reqLog.Error("video_task.persist_failed", zap.Int64("account_id", account.ID),
				zap.String("upstream_task_id", submission.UpstreamTaskID), zap.Error(err))
			writeVideoTaskError(c, http.StatusInternalServerError, "api_error", "Failed to record the video task")
			return
		}
		reqLog.Info("video_task.created", zap.String("task_id", task.ID), zap.Int64("account_id", account.ID),
			zap.String("upstream_kind", string(task.UpstreamKind)), zap.Int("switch_count", switchCount))
		c.JSON(http.StatusOK, service.RenderVideoTaskCreated(shape, task))
		return
	}
}

// loadVideoTask 查询类请求的公共前置：鉴权、依赖、按 (id, api key) 取任务；不属于这把 key 一律 404。
func (h *OpenAIGatewayHandler) loadVideoTask(c *gin.Context, id string) (*service.VideoTask, *service.APIKey, *zap.Logger, bool) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		writeVideoTaskError(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return nil, nil, nil, false
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		writeVideoTaskError(c, http.StatusInternalServerError, "api_error", "User context not found")
		return nil, nil, nil, false
	}
	reqLog := requestLogger(c, "handler.openai_gateway.video_task",
		zap.Int64("user_id", subject.UserID), zap.Int64("api_key_id", apiKey.ID), zap.String("task_id", id))
	if h.videoTasks == nil {
		writeVideoTaskError(c, http.StatusServiceUnavailable, "api_error", "Service temporarily unavailable")
		return nil, nil, nil, false
	}
	task, err := h.videoTasks.GetTask(c.Request.Context(), id, apiKey.ID)
	if err == nil && task.UserID != subject.UserID {
		err = service.ErrVideoTaskNotFound
	}
	if err != nil {
		if !errors.Is(err, service.ErrVideoTaskNotFound) {
			reqLog.Warn("video_task.load_failed", zap.Error(err))
			writeVideoTaskError(c, http.StatusInternalServerError, "api_error", "Failed to load the video task")
			return nil, nil, nil, false
		}
		writeVideoTaskError(c, http.StatusNotFound, "not_found_error", "Video task not found")
		return nil, nil, nil, false
	}
	setOpsRequestContext(c, task.Model, false)
	return task, apiKey, reqLog, true
}

func (h *OpenAIGatewayHandler) videoTaskStatus(c *gin.Context, pathShape service.VideoInboundShape) {
	id := videoTaskIDParam(c)
	if !service.IsVideoTaskID(id) {
		if pathShape == service.VideoShapeNewAPI {
			writeVideoTaskError(c, http.StatusNotFound, "not_found_error", "Video task not found")
			return
		}
		h.GrokVideoStatus(c)
		return
	}
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)
	task, apiKey, reqLog, ok := h.loadVideoTask(c, id)
	if !ok {
		return
	}
	var account *service.Account
	if !task.State.IsTerminal() {
		var err error
		if _, account, err = h.videoTasks.Poll(c.Request.Context(), c, task); err != nil {
			// 查询失败不改任务状态：回最近一次的状态，客户端下次再查。
			reqLog.Warn("video_task.poll_failed", zap.Error(err))
		}
	}
	h.billVideoTaskOnce(c, reqLog, apiKey, task, account)
	renderShape := service.VideoStatusRenderShape(pathShape, task.InboundShape)
	c.JSON(http.StatusOK, service.RenderVideoTaskStatus(renderShape, task, videoTaskContentURL(c, task.ID)))
}

// videoTaskContentURL 网关自己的成片地址（与旧 Grok 路径同一口径：只给路径，/v1 前缀跟随本次请求）。
func videoTaskContentURL(c *gin.Context, id string) string {
	prefix := ""
	if c.Request != nil && c.Request.URL != nil && strings.HasPrefix(c.Request.URL.Path, "/v1/") {
		prefix = "/v1"
	}
	return prefix + "/videos/" + url.PathEscape(id) + "/content"
}

// billVideoTaskOnce 任务首次被看到成功时入账一次：先在任务表抢占 billed_at，抢到的才写用量；
// 写用量失败释放抢占，下一次查询重试（用量去重键是任务 ID，重试不会重复扣费）。
func (h *OpenAIGatewayHandler) billVideoTaskOnce(c *gin.Context, reqLog *zap.Logger, apiKey *service.APIKey, task *service.VideoTask, account *service.Account) {
	if task.State != service.VideoTaskSucceeded || task.BilledAt != nil {
		return
	}
	ctx := context.WithoutCancel(c.Request.Context())
	claimed, err := h.videoTasks.ClaimBilling(ctx, task.ID)
	if err != nil {
		reqLog.Warn("video_task.billing_claim_failed", zap.Error(err))
		return
	}
	if !claimed {
		return
	}
	now := time.Now()
	task.BilledAt = &now
	if account == nil {
		if account, err = h.videoTasks.LoadAccount(ctx, task.AccountID); err != nil {
			reqLog.Warn("video_task.billing_account_missing", zap.Int64("account_id", task.AccountID), zap.Error(err))
			account = &service.Account{ID: task.AccountID}
		}
	}
	result := task.UsageResult(now)
	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	input := &service.OpenAIRecordUsageInput{
		Result:             result,
		APIKey:             apiKey,
		User:               apiKey.User,
		Account:            account,
		Subscription:       subscription,
		InboundEndpoint:    task.InboundShape.InboundEndpoint(),
		UpstreamEndpoint:   h.videoTasks.UpstreamEndpoint(task.UpstreamKind),
		UserAgent:          c.GetHeader("User-Agent"),
		IPAddress:          ip.GetClientIP(c),
		RequestPayloadHash: service.HashUsageRequestPayload([]byte(task.ID)),
		APIKeyService:      h.apiKeyService,
		SessionID:          service.ExtractClientSessionID(c),
		RequestedModel:     task.Model,
	}
	taskID := task.ID
	h.submitOpenAIUsageRecordTask(ctx, result, func(recordCtx context.Context) {
		if err := h.gatewayService.RecordUsage(recordCtx, input); err != nil {
			if releaseErr := h.videoTasks.ReleaseBilling(recordCtx, taskID); releaseErr != nil {
				reqLog.Warn("video_task.billing_release_failed", zap.String("task_id", taskID), zap.Error(releaseErr))
			}
			reqLog.Error("video_task.record_usage_failed", zap.String("task_id", taskID), zap.Error(err))
		}
	})
}
