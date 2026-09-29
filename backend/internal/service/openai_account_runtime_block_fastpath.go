package service

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	openAIAccountStateUpdateTimeout       = 5 * time.Second
	openAIOAuth429RetryWindow             = 2 * time.Minute
	openAIOAuth429RetryDelay              = 500 * time.Millisecond
	openAIOAuth429MaxRetryDelay           = 8 * time.Second
	openAIOAuth429MaxAccountAttempts      = 3
	openAIStopSchedulingBridgeCooldown    = 2 * time.Minute
	openAIOAuth429StormWindow             = 10 * time.Second
	openAIOAuth429StormMaxAccountSwitches = 1
)

// OpenAIOAuth429FailoverState tracks the request-local follow-up budget after
// the first Grok OAuth 429. Once that 429 occurs, exactly one different account
// may be attempted; any failure from that follow-up account ends failover.
type OpenAIOAuth429FailoverState struct {
	grokOAuth429FollowupPending bool
}

type openAIOAuth429Disposition uint8

const (
	openAIOAuth429Transient openAIOAuth429Disposition = iota
	openAIOAuth429Quota5h
	openAIOAuth429Quota7d
	openAIOAuth429QuotaReset
)

// classifyOpenAIOAuth429 区分账号配额耗尽信号与普通瞬时 429。只有窗口达到
// 100% 或响应体明确给出 reset 时间时，才视为配额限流信号。
func classifyOpenAIOAuth429(headers http.Header, responseBody []byte) (openAIOAuth429Disposition, *time.Time) {
	if snapshot := ParseCodexRateLimitHeaders(headers); snapshot != nil {
		if normalized := snapshot.Normalize(); normalized != nil {
			if normalized.Used7dPercent != nil && *normalized.Used7dPercent >= 100 {
				if normalized.Reset7dSeconds != nil {
					now := time.Now()
					resetAt := now.Add(time.Duration(*normalized.Reset7dSeconds) * time.Second)
					return openAIOAuth429Quota7d, &resetAt
				}
				return openAIOAuth429Quota7d, nil
			}
			if normalized.Used5hPercent != nil && *normalized.Used5hPercent >= 100 {
				if normalized.Reset5hSeconds != nil {
					now := time.Now()
					resetAt := now.Add(time.Duration(*normalized.Reset5hSeconds) * time.Second)
					return openAIOAuth429Quota5h, &resetAt
				}
				return openAIOAuth429Quota5h, nil
			}
		}
	}
	if resetAt := calculateOpenAI429ResetTime(headers); resetAt != nil {
		return openAIOAuth429QuotaReset, resetAt
	}
	if resetUnix := parseOpenAIRateLimitResetTime(responseBody); resetUnix != nil {
		resetAt := time.Unix(*resetUnix, 0)
		return openAIOAuth429QuotaReset, &resetAt
	}
	return openAIOAuth429Transient, nil
}

func openAIAccountStateContext(ctx context.Context) (context.Context, context.CancelFunc) {
	base := context.Background()
	if ctx != nil {
		base = context.WithoutCancel(ctx)
	}
	return context.WithTimeout(base, openAIAccountStateUpdateTimeout)
}

func isOpenAIOAuthAccount(account *Account) bool {
	return account != nil && account.IsOpenAIOAuthLike()
}

func isGrokOAuthAccount(account *Account) bool {
	return account != nil && account.Platform == PlatformGrok && account.Type == AccountTypeOAuth
}

// isOpenAIAccount 报告账号是否参与 OpenAI 网关的进程内调度阻断。
//
// 第三方 key 不论标签都可能被 OpenAI 网关的分组调度到，阻断状态必须对它们生效，
// 否则标签不是 openai/grok 的 key 在认证失败后仍会被继续选中。成品号保持原口径。
func isOpenAIAccount(account *Account) bool {
	if account == nil {
		return false
	}
	if account.IsThirdPartyKey() {
		return true
	}
	return account.Platform == PlatformOpenAI || account.Platform == PlatformGrok
}

// handleOpenAIAccountUpstreamError expects canonicalModel to be the model used
// for scheduling after applying account mapping exactly once.
func (s *OpenAIGatewayService) handleOpenAIAccountUpstreamError(ctx context.Context, account *Account, statusCode int, headers http.Header, responseBody []byte, canonicalModel ...string) bool {
	// xAI 的内容安全拒绝文案是厂商私有语义，按 Vendor 识别。
	if account != nil && account.Vendor() == PlatformGrok && isGrokContentPolicyRejection(statusCode, responseBody) {
		return false
	}
	// Any non-2xx upstream HTTP response means the model request was actually sent.
	if s != nil {
		scheduleOllamaCloudUsageActivity(s.deferredService, account)
	}
	// Capacity shedding describes this request, not account health. Keep the
	// account schedulable while the request-local retry budget handles recovery.
	// 过载文案是 OpenAI 协议错误形态，中转会原样透传；豁免只免除账号处罚，
	// 对通用中转同样安全。
	if openAIProtocolFeaturesApply(account) && isOpenAIRequestScopedCapacityShed("", responseBody) {
		return false
	}
	stateCtx, cancel := openAIAccountStateContext(ctx)
	defer cancel()
	// 账号/工作区停用码描述的是 OpenAI 官方凭据本身，直接永久停用。中转透传的同类
	// 错误码说的是中转背后某个上游账号，不能据此停用整把中转 key，只认官方地址。
	if account != nil && account.Vendor() == PlatformOpenAI && isOpenAIHTTPUpstreamAccessStateError(statusCode, "", responseBody) {
		message := "OpenAI upstream account or workspace is unavailable"
		if upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(responseBody)); upstreamMsg != "" {
			message = upstreamMsg
		}
		if s != nil && s.rateLimitService != nil {
			s.rateLimitService.handleAuthError(stateCtx, account, message)
		}
		if s != nil {
			s.BlockAccountScheduling(account, time.Time{}, "openai_access_state")
		}
		return true
	}

	// context_length_exceeded 是 OpenAI 协议的标准错误码，属于请求本身的问题；
	// 豁免只免除账号处罚，对通用中转同样适用。
	if openAIProtocolFeaturesApply(account) && isOpenAIContextWindowError("", responseBody) {
		return false
	}

	if isOpenAIImageRateLimitError(statusCode, responseBody) {
		if s != nil && s.rateLimitService != nil {
			_ = s.rateLimitService.HandleOpenAIImageRateLimit(stateCtx, account, statusCode, headers, responseBody)
		}
		return false
	}

	// Self-built images requests always carry a matching image_generation tool, so a
	// "tool choice not found in 'tools'" 400 means upstream revoked this account's
	// image capability. Gated on the self-built marker: passthrough clients control
	// their own tools/tool_choice and could otherwise poison a healthy account.
	if isOpenAIImagesSelfBuiltRequest(ctx) && isOpenAIImageCapabilityLossError(statusCode, responseBody) {
		if s != nil && s.rateLimitService != nil {
			_ = s.rateLimitService.HandleOpenAIImageCapabilityLoss(stateCtx, account, statusCode, responseBody)
		}
		return false
	}

	if s == nil || account == nil {
		return false
	}
	// Team 联动熔断必须先于 model-not-found 的早退。
	if s.rateLimitService != nil {
		s.rateLimitService.maybeHandleOpenAITeamLinkedError(stateCtx, account, statusCode, responseBody)
	}
	if s.rateLimitService != nil && len(canonicalModel) > 0 && s.rateLimitService.HandleUpstreamModelNotFound(stateCtx, account, canonicalModel[0], statusCode, responseBody) {
		return true
	}
	if statusCode == http.StatusTooManyRequests && s.rateLimitService != nil && len(canonicalModel) > 0 &&
		s.rateLimitService.HandleOpenAICodexSparkRateLimit(stateCtx, account, canonicalModel[0], statusCode, headers, responseBody) {
		return false
	}
	if statusCode == http.StatusTooManyRequests {
		s.markOpenAIOAuth429RateLimited(stateCtx, account, headers, responseBody)
	}
	if s.rateLimitService == nil {
		return false
	}
	shouldDisable := s.rateLimitService.HandleUpstreamError(stateCtx, account, statusCode, headers, responseBody)
	if shouldDisable {
		s.BlockAccountScheduling(account, time.Time{}, "upstream_disable")
	}
	// Pool-mode retryable upstream errors are already bounded by the request-local
	// same-account retry budget. Recording the generic account+model transient
	// cooldown here would block the next approved retry before that budget is used.
	poolModeRetryable := account.IsPoolMode() && account.IsPoolModeRetryableStatus(statusCode)
	// 账号×模型的瞬时冷却针对第三方 key：5xx/52x 与 OpenAI 协议的处理中错误，
	// 官方 OpenAI 地址与通用中转都适用；成品号不参与。
	if !shouldDisable && account.IsThirdPartyKey() && openAIProtocolFeaturesApply(account) &&
		shouldCooldownOpenAITransientUpstreamError(statusCode, responseBody) && !poolModeRetryable {
		model := ""
		if len(canonicalModel) > 0 {
			model = canonicalModel[0]
		}
		decision := s.recordOpenAIAccountModelTransientFailure(account, model, time.Now())
		if decision.FailureStreak > 0 {
			slog.Warn("openai_model_transient_state",
				"account_id", account.ID,
				"model", openAIAccountModelTransientModel(model),
				"failure_streak", decision.FailureStreak,
				"cooldown_ms", decision.Cooldown.Milliseconds(),
				"block_scope", "account_model",
			)
		}
	}
	return shouldDisable
}

// isTransientUpstreamServerStatus 计入「渠道 × 模型」连续失败的上游 5xx / 52x。
// OpenAI 协议与 Messages 协议的 key 共用这张表（529 不在内：过载只按成品号冷却处理）。
func isTransientUpstreamServerStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout, 520, 521, 522, 523, 524:
		return true
	default:
		return false
	}
}

func shouldCooldownOpenAITransientUpstreamError(statusCode int, responseBody []byte) bool {
	if isTransientUpstreamServerStatus(statusCode) {
		return true
	}
	return statusCode == http.StatusBadRequest && isOpenAITransientProcessingError(statusCode, "", responseBody)
}

func (s *OpenAIGatewayService) markOpenAIOAuth429RateLimited(ctx context.Context, account *Account, headers http.Header, responseBody []byte) {
	if s == nil || !isOpenAIOAuthAccount(account) {
		return
	}
	// Spark 影子：不按 /responses 429 的 global x-codex-* 信号做内存运行时熔断(同 handle429,外审第8轮 P1)。
	// 同时避免把 spark 的 429 计入全局 429 storm 计数(recordOpenAIOAuth429),否则会误伤母账号 failover 决策。
	if account.IsShadow() {
		return
	}
	s.recordOpenAIOAuth429()
	disposition, resetAt := classifyOpenAIOAuth429(headers, responseBody)
	if disposition == openAIOAuth429Transient && s.openAIOAuth429RetryWindowActive(account) {
		return
	}

	now := time.Now()
	var cooldownUntil time.Time
	if resetAt != nil && resetAt.After(now) {
		cooldownUntil = *resetAt
	} else {
		cooldown, ok := rateLimit429FallbackCooldown()
		if !ok {
			s.openaiOAuth429RetryStartedAt.Delete(account.ID)
			return
		}
		cooldownUntil = now.Add(cooldown)
	}
	s.BlockAccountScheduling(account, cooldownUntil, "429")
	s.openaiOAuth429RetryStartedAt.Delete(account.ID)
}

func (s *OpenAIGatewayService) shouldRetryOpenAIOAuth429OnSameAccount(account *Account, statusCode int, shouldDisable bool) bool {
	return s.shouldRetryOpenAIOAuth429OnSameAccountWithResponse(account, statusCode, shouldDisable, nil, nil)
}

func (s *OpenAIGatewayService) shouldRetryOpenAIOAuth429OnSameAccountWithResponse(account *Account, statusCode int, shouldDisable bool, headers http.Header, responseBody []byte) bool {
	if shouldDisable || statusCode != http.StatusTooManyRequests || !isOpenAIOAuthAccount(account) || account.IsShadow() {
		return false
	}
	disposition, _ := classifyOpenAIOAuth429(headers, responseBody)
	if disposition != openAIOAuth429Transient {
		return false
	}
	// markOpenAIOAuth429RateLimited parks the account once the window expires.
	// Do not accidentally create a fresh window after that transition.
	if s.isOpenAIAccountRuntimeBlocked(account) {
		return false
	}
	return s.openAIOAuth429RetryWindowActive(account)
}

// ShouldRetryOpenAIOAuth429 lets RateLimitService defer persistent account
// cooldown until the gateway's same-account retry window is exhausted.
func (s *OpenAIGatewayService) ShouldRetryOpenAIOAuth429(account *Account, headers http.Header, responseBody []byte) bool {
	if s == nil || !isOpenAIOAuthAccount(account) || account.IsShadow() || s.isOpenAIAccountRuntimeBlocked(account) {
		return false
	}
	disposition, _ := classifyOpenAIOAuth429(headers, responseBody)
	if disposition != openAIOAuth429Transient {
		return false
	}
	return s.openAIOAuth429RetryWindowActive(account)
}

func (s *OpenAIGatewayService) openAIOAuth429RetryWindowActive(account *Account) bool {
	if s == nil || !isOpenAIOAuthAccount(account) || account.IsShadow() {
		return false
	}
	now := time.Now()
	value, _ := s.openaiOAuth429RetryStartedAt.LoadOrStore(account.ID, now)
	startedAt, ok := value.(time.Time)
	if !ok {
		s.openaiOAuth429RetryStartedAt.Store(account.ID, now)
		startedAt = now
	}
	return now.Before(startedAt.Add(openAIOAuth429RetryWindow))
}

func (s *OpenAIGatewayService) openAIOAuth429RetryDeadline(account *Account) time.Time {
	if s == nil || !isOpenAIOAuthAccount(account) || account.IsShadow() {
		return time.Time{}
	}
	value, ok := s.openaiOAuth429RetryStartedAt.Load(account.ID)
	if !ok {
		return time.Time{}
	}
	startedAt, ok := value.(time.Time)
	if !ok {
		return time.Time{}
	}
	return startedAt.Add(openAIOAuth429RetryWindow)
}

func openAIOAuth429SameAccountRetryDelay(headers http.Header, deadline time.Time) time.Duration {
	delay := openAIOAuth429RetryDelay
	now := time.Now()
	if resetAt := parseRetryAfterResetTime(headers, now); resetAt != nil && resetAt.After(now) {
		delay = resetAt.Sub(now)
	}
	if delay > openAIOAuth429MaxRetryDelay {
		delay = openAIOAuth429MaxRetryDelay
	}
	if remaining := time.Until(deadline); !deadline.IsZero() && delay > remaining {
		delay = remaining
	}
	if delay < 0 {
		return 0
	}
	return delay
}

func (s *OpenAIGatewayService) BlockAccountScheduling(account *Account, until time.Time, reason string) {
	if s == nil || !isOpenAIAccount(account) {
		return
	}
	mu := s.openAIAccountRuntimeBlockLock(account.ID)
	mu.Lock()
	defer mu.Unlock()
	_, _ = s.blockAccountSchedulingLocked(account, until, reason)
}

func (s *OpenAIGatewayService) openAIAccountRuntimeBlockLock(accountID int64) *sync.Mutex {
	actual, _ := s.openaiAccountRuntimeBlockLocks.LoadOrStore(accountID, &sync.Mutex{})
	mu, ok := actual.(*sync.Mutex)
	if !ok {
		mu = &sync.Mutex{}
		s.openaiAccountRuntimeBlockLocks.Store(accountID, mu)
	}
	return mu
}

func (s *OpenAIGatewayService) blockAccountSchedulingLocked(account *Account, until time.Time, _ string) (uint64, bool) {
	generation := s.openaiAccountRuntimeBlockSequence.Add(1)
	s.openaiAccountRuntimeBlockGeneration.Store(account.ID, generation)
	now := time.Now()
	blockUntil := until
	if blockUntil.IsZero() || !blockUntil.After(now) {
		blockUntil = now.Add(openAIStopSchedulingBridgeCooldown)
	}

	for {
		current, loaded := s.openaiAccountRuntimeBlockUntil.Load(account.ID)
		if !loaded {
			actual, stored := s.openaiAccountRuntimeBlockUntil.LoadOrStore(account.ID, blockUntil)
			if !stored {
				return generation, true
			}
			current = actual
		}

		currentUntil, ok := current.(time.Time)
		if !ok || currentUntil.IsZero() {
			if s.openaiAccountRuntimeBlockUntil.CompareAndSwap(account.ID, current, blockUntil) {
				return generation, true
			}
			continue
		}
		if !blockUntil.After(currentUntil) {
			return generation, false
		}
		if s.openaiAccountRuntimeBlockUntil.CompareAndSwap(account.ID, current, blockUntil) {
			return generation, true
		}
	}
}

func (s *OpenAIGatewayService) ClearAccountSchedulingBlock(accountID int64) {
	if s == nil || accountID <= 0 {
		return
	}
	mu := s.openAIAccountRuntimeBlockLock(accountID)
	mu.Lock()
	defer mu.Unlock()
	s.openaiAccountRuntimeBlockUntil.Delete(accountID)
	s.openaiOAuth429RetryStartedAt.Delete(accountID)
	s.openaiAccountRuntimeBlockGeneration.Store(accountID, s.openaiAccountRuntimeBlockSequence.Add(1))
}

func (s *OpenAIGatewayService) isOpenAIAccountRuntimeBlocked(account *Account) bool {
	if s == nil || !isOpenAIAccount(account) {
		return false
	}
	mu := s.openAIAccountRuntimeBlockLock(account.ID)
	mu.Lock()
	defer mu.Unlock()
	value, ok := s.openaiAccountRuntimeBlockUntil.Load(account.ID)
	if !ok {
		return false
	}
	cooldownUntil, ok := value.(time.Time)
	if !ok || cooldownUntil.IsZero() {
		s.openaiAccountRuntimeBlockUntil.Delete(account.ID)
		s.openaiAccountRuntimeBlockGeneration.Store(account.ID, s.openaiAccountRuntimeBlockSequence.Add(1))
		return false
	}
	if time.Now().Before(cooldownUntil) {
		return true
	}
	s.openaiAccountRuntimeBlockUntil.Delete(account.ID)
	s.openaiAccountRuntimeBlockGeneration.Store(account.ID, s.openaiAccountRuntimeBlockSequence.Add(1))
	return false
}

func canonicalOpenAIAccountSchedulingModel(account *Account, requestedModel string) string {
	model := strings.TrimSpace(requestedModel)
	if account == nil || model == "" {
		return model
	}
	// 第三方 key 在 OpenAI 网关上不论标签都走同一条转发模型解析链（映射、透传、
	// 上游归一），调度与瞬时冷却的模型键必须与之一致，不能按标签分叉。
	if account.IsOpenAI() || account.IsThirdPartyKey() {
		return resolveOpenAIAccountUpstreamModelForRequest(account, model)
	}
	if mapped := strings.TrimSpace(account.GetMappedModel(model)); mapped != "" {
		return mapped
	}
	return model
}

func openAIAccountModelTransientModel(canonicalModel string) string {
	return normalizeOpenAIAccountModelTransientModel(canonicalModel)
}

func (s *OpenAIGatewayService) recordOpenAIAccountModelTransientFailure(account *Account, canonicalModel string, now time.Time) openAIAccountModelTransientDecision {
	if s == nil {
		return openAIAccountModelTransientDecision{}
	}
	return s.rateLimitService.RecordModelTransientFailure(account, canonicalModel, now)
}

func (s *OpenAIGatewayService) clearOpenAIAccountModelTransientState(accountID int64, model string) {
	if s == nil {
		return
	}
	s.rateLimitService.ClearModelTransient(accountID, model)
}

func (s *OpenAIGatewayService) isOpenAIAccountModelRuntimeBlocked(account *Account, requestedModel string) bool {
	if s == nil {
		return false
	}
	return s.rateLimitService.ModelTransientBlocked(account, requestedModel, time.Now())
}

func accountPersistedSchedulingCooldownActive(account *Account) bool {
	if account == nil {
		return false
	}
	now := time.Now()
	if account.TempUnschedulableUntil != nil && now.Before(*account.TempUnschedulableUntil) {
		return true
	}
	if account.RateLimitResetAt != nil && now.Before(*account.RateLimitResetAt) {
		return true
	}
	if account.OverloadUntil != nil && now.Before(*account.OverloadUntil) {
		return true
	}
	return false
}

type openAIAccountRuntimeBlockSnapshot struct {
	until      time.Time
	generation uint64
	blocked    bool
}

func (s *OpenAIGatewayService) peekOpenAIAccountRuntimeBlock(account *Account) openAIAccountRuntimeBlockSnapshot {
	if s == nil || !isOpenAIAccount(account) {
		return openAIAccountRuntimeBlockSnapshot{}
	}
	mu := s.openAIAccountRuntimeBlockLock(account.ID)
	mu.Lock()
	defer mu.Unlock()
	value, ok := s.openaiAccountRuntimeBlockUntil.Load(account.ID)
	if !ok {
		return openAIAccountRuntimeBlockSnapshot{}
	}
	until, isTime := value.(time.Time)
	if !isTime || until.IsZero() || !time.Now().Before(until) {
		s.openaiAccountRuntimeBlockUntil.Delete(account.ID)
		s.openaiOAuth429RetryStartedAt.Delete(account.ID)
		s.openaiAccountRuntimeBlockGeneration.Store(account.ID, s.openaiAccountRuntimeBlockSequence.Add(1))
		return openAIAccountRuntimeBlockSnapshot{}
	}
	generation, _ := s.openaiAccountRuntimeBlockGeneration.Load(account.ID)
	gen, _ := generation.(uint64)
	return openAIAccountRuntimeBlockSnapshot{until: until, generation: gen, blocked: true}
}

// clearOpenAIAccountRuntimeBlockIfUnchanged deletes the in-process account block
// only when generation and deadline are unchanged. A newer block installed after
// peek must be kept even if its deadline happens to match.
func (s *OpenAIGatewayService) clearOpenAIAccountRuntimeBlockIfUnchanged(accountID int64, snapshot openAIAccountRuntimeBlockSnapshot) {
	if s == nil || accountID <= 0 || !snapshot.blocked {
		return
	}
	mu := s.openAIAccountRuntimeBlockLock(accountID)
	mu.Lock()
	defer mu.Unlock()
	generation, ok := s.openaiAccountRuntimeBlockGeneration.Load(accountID)
	if !ok || generation != snapshot.generation {
		return
	}
	current, ok := s.openaiAccountRuntimeBlockUntil.Load(accountID)
	currentUntil, isTime := current.(time.Time)
	if !ok || !isTime || !currentUntil.Equal(snapshot.until) {
		return
	}
	s.openaiAccountRuntimeBlockUntil.Delete(accountID)
	s.openaiOAuth429RetryStartedAt.Delete(accountID)
	s.openaiAccountRuntimeBlockGeneration.Store(accountID, s.openaiAccountRuntimeBlockSequence.Add(1))
}

// isOpenAIAccountRequestRuntimeBlocked treats persisted cooldown fields on the
// scheduling Account as source of truth. When TempUnschedulableUntil,
// RateLimitResetAt, and OverloadUntil are all inactive, a stale local account
// block is dropped with generation+deadline CAS. Model-scoped transient blocks
// are left alone. This is fail-open if a DB write failed or the snapshot has
// not caught up yet: empty cooldown fields drop the local account-level block.
func (s *OpenAIGatewayService) isOpenAIAccountRequestRuntimeBlocked(account *Account, requestedModel string) bool {
	if s == nil {
		return false
	}
	snapshot := s.peekOpenAIAccountRuntimeBlock(account)
	if snapshot.blocked {
		if accountPersistedSchedulingCooldownActive(account) {
			return true
		}
		s.clearOpenAIAccountRuntimeBlockIfUnchanged(account.ID, snapshot)
	}
	return s.isOpenAIAccountModelRuntimeBlocked(account, requestedModel)
}

func (s *OpenAIGatewayService) recordOpenAIOAuth429() {
	if s == nil {
		return
	}
	now := time.Now()
	windowStart := s.openaiOAuth429WindowStartUnixNano.Load()
	if windowStart == 0 || now.Sub(time.Unix(0, windowStart)) >= openAIOAuth429StormWindow {
		if s.openaiOAuth429WindowStartUnixNano.CompareAndSwap(windowStart, now.UnixNano()) {
			s.openaiOAuth429WindowCount.Store(1)
			return
		}
	}
	s.openaiOAuth429WindowCount.Add(1)
}

func (s *OpenAIGatewayService) ShouldStopOpenAIOAuth429Failover(account *Account, statusCode int, failedSwitches int, state *OpenAIOAuth429FailoverState) bool {
	if failedSwitches < openAIOAuth429StormMaxAccountSwitches {
		return false
	}
	if state != nil && state.grokOAuth429FollowupPending {
		// The follow-up budget was armed by a Grok OAuth 429. Consume it on
		// any failing follow-up account, even if a mixed pool selected an API-key
		// account next.
		return true
	}
	if isGrokOAuthAccount(account) {
		if state == nil {
			// Preserve the old threshold for callers that have not adopted the
			// request-local state contract yet.
			return statusCode == http.StatusTooManyRequests && failedSwitches >= 2
		}
		if statusCode == http.StatusTooManyRequests {
			state.grokOAuth429FollowupPending = true
		}
		return false
	}
	if statusCode != http.StatusTooManyRequests || !isOpenAIOAuthAccount(account) {
		return false
	}
	// Each OpenAI OAuth candidate has already consumed its full same-account
	// retry window before reaching this switch point. A global storm is useful
	// telemetry, but must not prevent trying the bounded next-account budget.
	return failedSwitches >= openAIOAuth429MaxAccountAttempts
}
