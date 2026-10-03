package service

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

// RecordUsageInput 记录使用量的输入参数。
// 异步 worker 只接收计费所需快照，不能持有 ParsedRequest/RequestBodyRef 这类大请求体引用。
type RecordUsageInput struct {
	Result             *ForwardResult
	APIKey             *APIKey
	User               *User
	Account            *Account
	Subscription       *UserSubscription  // 可选：订阅信息
	PricingAt          time.Time          // token 售价固定时刻；零值保持既有的记录时刻语义
	InboundEndpoint    string             // 入站端点（客户端请求路径）
	UpstreamEndpoint   string             // 上游端点（标准化后的上游路径）
	UserAgent          string             // 请求的 User-Agent
	IPAddress          string             // 请求的客户端 IP 地址
	SessionID          string             // 客户端显式会话标识（session_id / X-Session-Id 等请求头），仅用于用量行会话关联
	RequestPayloadHash string             // 请求体语义哈希，用于降低 request_id 误复用时的静默误去重风险
	ForceCacheBilling  bool               // 强制缓存计费：将 input_tokens 转为 cache_read 计费（用于粘性会话切换）
	APIKeyService      APIKeyQuotaUpdater // 可选：用于更新API Key配额

	// RequestedModel 是客户端写的模型名（目录别名归一前），进 usage_logs.requested_model；
	// 空则用 result.Model。
	RequestedModel string
	// WebSearchDelegated 这次是 Claude Code 配第三方模型时交给 Haiku 代执行的搜索请求（见 web_search_delegate.go）。
	WebSearchDelegated bool
}

// APIKeyQuotaUpdater defines the interface for updating API Key quota and rate limit usage
type APIKeyQuotaUpdater interface {
	UpdateQuotaUsed(ctx context.Context, apiKeyID int64, cost float64) error
	UpdateRateLimitUsage(ctx context.Context, apiKeyID int64, cost float64) error
}

type apiKeyAuthCacheInvalidator interface {
	InvalidateAuthCacheByKey(ctx context.Context, key string)
}

type usageLogBestEffortWriter interface {
	CreateBestEffort(ctx context.Context, log *UsageLog) error
}

// postUsageBillingParams 统一扣费所需的参数
type postUsageBillingParams struct {
	Cost               *CostBreakdown
	User               *User
	APIKey             *APIKey
	Account            *Account
	Subscription       *UserSubscription
	RequestPayloadHash string
	IsSubscriptionBill bool
	// AccountCost 渠道成本（用量 × 上游价）；渠道额度按它累计。
	AccountCost   float64
	APIKeyService APIKeyQuotaUpdater
}

func (p *postUsageBillingParams) shouldDeductAPIKeyQuota() bool {
	return p.Cost.ActualCost > 0 && p.APIKey.Quota > 0 && p.APIKeyService != nil
}

func (p *postUsageBillingParams) shouldUpdateRateLimits() bool {
	return p.Cost.ActualCost > 0 && p.APIKey.HasRateLimits() && p.APIKeyService != nil
}

func (p *postUsageBillingParams) shouldUpdateAccountQuota() bool {
	return p.AccountCost > 0 && p.Account.IsAPIKeyOrBedrock() && p.Account.HasAnyQuotaLimit()
}

// postUsageBilling is the legacy fallback billing path used when the unified
// billing repo is unavailable (nil). Production uses applyUsageBilling → repo.Apply
// for atomic billing. This path only runs in tests or degraded mode.
func postUsageBilling(ctx context.Context, p *postUsageBillingParams, deps *billingDeps) {
	billingCtx, cancel := detachedBillingContext(ctx)
	defer cancel()

	cost := p.Cost

	if p.IsSubscriptionBill {
		// Subscription usage tracked by ActualCost so group rate multiplier
		// consumes the quota at the expected speed.
		if cost.ActualCost > 0 {
			if err := deps.userSubRepo.IncrementUsage(billingCtx, p.Subscription.ID, cost.ActualCost); err != nil {
				slog.Error("increment subscription usage failed", "subscription_id", p.Subscription.ID, "error", err)
			}
		}
	} else {
		if cost.ActualCost > 0 {
			if err := deps.userRepo.DeductBalance(billingCtx, p.User.ID, cost.ActualCost); err != nil {
				slog.Error("deduct balance failed", "user_id", p.User.ID, "error", err)
			} else if deps.billingCacheService != nil {
				if err := deps.billingCacheService.InvalidateUserBalance(billingCtx, p.User.ID); err != nil {
					slog.Warn("invalidate balance cache after legacy deduction failed", "user_id", p.User.ID, "error", err)
				}
			}
		}
	}

	if p.shouldDeductAPIKeyQuota() {
		if err := p.APIKeyService.UpdateQuotaUsed(billingCtx, p.APIKey.ID, cost.ActualCost); err != nil {
			slog.Error("update api key quota failed", "api_key_id", p.APIKey.ID, "error", err)
		}
	}

	if p.shouldUpdateRateLimits() {
		if err := p.APIKeyService.UpdateRateLimitUsage(billingCtx, p.APIKey.ID, cost.ActualCost); err != nil {
			slog.Error("update api key rate limit usage failed", "api_key_id", p.APIKey.ID, "error", err)
		}
	}

	if p.shouldUpdateAccountQuota() {
		if err := deps.accountRepo.IncrementQuotaUsed(billingCtx, p.Account.ID, p.AccountCost); err != nil {
			slog.Error("increment account quota used failed", "account_id", p.Account.ID, "cost", p.AccountCost, "error", err)
		}
	}

	// NOTE: finalizePostUsageBilling is NOT called here to avoid double-queuing
	// cache updates. The legacy path does DB writes directly; the finalize path
	// does cache queue + notifications. Notifications are dispatched separately
	// by the caller after recording the usage log.
}

func resolveUsageBillingRequestID(ctx context.Context, upstreamRequestID string) string {
	// Forced durable money-event IDs must win over client/local context IDs so
	// standalone web_search / async video cannot collapse under a reused client id.
	if requestID := strings.TrimSpace(upstreamRequestID); requestID != "" {
		if isForcedUsageBillingRequestID(requestID) {
			return requestID
		}
	}
	if ctx != nil {
		if clientRequestID, _ := ctx.Value(ctxkey.ClientRequestID).(string); strings.TrimSpace(clientRequestID) != "" {
			return "client:" + strings.TrimSpace(clientRequestID)
		}
		if requestID, _ := ctx.Value(ctxkey.RequestID).(string); strings.TrimSpace(requestID) != "" {
			return "local:" + strings.TrimSpace(requestID)
		}
	}
	if requestID := strings.TrimSpace(upstreamRequestID); requestID != "" {
		return requestID
	}
	return "generated:" + generateRequestID()
}

func isForcedUsageBillingRequestID(requestID string) bool {
	id := strings.TrimSpace(requestID)
	return strings.HasPrefix(id, "web_search:") ||
		strings.HasPrefix(id, "grok-video:") ||
		strings.HasPrefix(id, "grok_audio:") ||
		strings.HasPrefix(id, "grok_realtime:")
}

// StableGrokAudioBillingRequestID is the durable usage_logs / dedup key for one
// voice HTTP call (TTS/STT). Prefer an upstream request id when present.
func StableGrokAudioBillingRequestID(upstreamRequestID string) string {
	upstreamRequestID = strings.TrimSpace(upstreamRequestID)
	if strings.HasPrefix(upstreamRequestID, "grok_audio:") {
		return upstreamRequestID
	}
	if upstreamRequestID == "" {
		upstreamRequestID = generateRequestID()
	}
	return "grok_audio:" + upstreamRequestID
}

// StableGrokRealtimeBillingRequestID is the durable usage_logs / dedup key for
// one realtime WebSocket session.
func StableGrokRealtimeBillingRequestID(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if strings.HasPrefix(sessionID, "grok_realtime:") {
		return sessionID
	}
	if sessionID == "" {
		sessionID = generateRequestID()
	}
	return "grok_realtime:" + sessionID
}

func resolveUsageBillingPayloadFingerprint(ctx context.Context, requestPayloadHash string) string {
	if payloadHash := strings.TrimSpace(requestPayloadHash); payloadHash != "" {
		return payloadHash
	}
	if ctx != nil {
		if clientRequestID, _ := ctx.Value(ctxkey.ClientRequestID).(string); strings.TrimSpace(clientRequestID) != "" {
			return "client:" + strings.TrimSpace(clientRequestID)
		}
		if requestID, _ := ctx.Value(ctxkey.RequestID).(string); strings.TrimSpace(requestID) != "" {
			return "local:" + strings.TrimSpace(requestID)
		}
	}
	return ""
}

func buildUsageBillingCommand(requestID string, usageLog *UsageLog, p *postUsageBillingParams) *UsageBillingCommand {
	if p == nil || p.Cost == nil || p.APIKey == nil || p.User == nil || p.Account == nil {
		return nil
	}

	cmd := &UsageBillingCommand{
		RequestID:          requestID,
		APIKeyID:           p.APIKey.ID,
		UserID:             p.User.ID,
		AccountID:          p.Account.ID,
		AccountType:        p.Account.Type,
		RequestPayloadHash: strings.TrimSpace(p.RequestPayloadHash),
	}
	if usageLog != nil {
		cmd.Model = usageLog.Model
		cmd.BillingType = usageLog.BillingType
		cmd.InputTokens = usageLog.InputTokens
		cmd.OutputTokens = usageLog.OutputTokens
		cmd.CacheCreationTokens = usageLog.CacheCreationTokens
		cmd.CacheReadTokens = usageLog.CacheReadTokens
		cmd.ImageCount = usageLog.ImageCount
		if usageLog.ServiceTier != nil {
			cmd.ServiceTier = *usageLog.ServiceTier
		}
		if usageLog.ReasoningEffort != nil {
			cmd.ReasoningEffort = *usageLog.ReasoningEffort
		}
		if usageLog.SubscriptionID != nil {
			cmd.SubscriptionID = usageLog.SubscriptionID
		}
	}

	// Record subscription / balance cost using ActualCost so the group (and any
	// user-specific) rate multiplier consumes subscription quota at the expected
	// speed. TotalCost remains the raw (pre-multiplier) value; downstream guards
	// on "> 0" still correctly skip free subscriptions (RateMultiplier == 0).
	if p.IsSubscriptionBill && p.Subscription != nil && p.Cost.TotalCost > 0 {
		cmd.SubscriptionID = &p.Subscription.ID
		cmd.SubscriptionCost = p.Cost.ActualCost
	} else if p.Cost.ActualCost > 0 {
		cmd.BalanceCost = p.Cost.ActualCost
	}

	if p.shouldDeductAPIKeyQuota() {
		cmd.APIKeyQuotaCost = p.Cost.ActualCost
	}
	if p.shouldUpdateRateLimits() {
		cmd.APIKeyRateLimitCost = p.Cost.ActualCost
	}
	if p.shouldUpdateAccountQuota() {
		cmd.AccountQuotaCost = p.AccountCost
	}

	cmd.Normalize()
	return cmd
}

func applyUsageBilling(ctx context.Context, requestID string, usageLog *UsageLog, p *postUsageBillingParams, deps *billingDeps, repo UsageBillingRepository) (bool, error) {
	if p == nil || deps == nil {
		return false, nil
	}

	cmd := buildUsageBillingCommand(requestID, usageLog, p)
	if cmd == nil || cmd.RequestID == "" || repo == nil {
		postUsageBilling(ctx, p, deps)
		return true, nil
	}

	billingCtx, cancel := detachedBillingContext(ctx)
	defer cancel()

	result, err := repo.Apply(billingCtx, cmd)
	if err != nil {
		return false, err
	}

	if result == nil || !result.Applied {
		deps.deferredService.ScheduleLastUsedUpdate(p.Account.ID)
		return false, nil
	}

	if result.APIKeyQuotaExhausted {
		if invalidator, ok := p.APIKeyService.(apiKeyAuthCacheInvalidator); ok && p.APIKey != nil && p.APIKey.Key != "" {
			invalidator.InvalidateAuthCacheByKey(billingCtx, p.APIKey.Key)
		}
	}

	finalizePostUsageBilling(billingCtx, p, deps, result)
	return true, nil
}

func finalizePostUsageBilling(ctx context.Context, p *postUsageBillingParams, deps *billingDeps, result *UsageBillingApplyResult) {
	if p == nil || p.Cost == nil || deps == nil {
		return
	}

	if p.IsSubscriptionBill {
		// 订阅用量缓存的键是 (user, plan)——读侧 GetSubscriptionStatus 用 subscription.PlanID 拼键。
		// 这里原来传的是 apiKey.GroupID：4b 把订阅与分组解耦（迁移 248）后两者不是同一 ID 空间，
		// 增量会落到 billing:sub:<user>:<groupID> 这个没人读的键上，无分组 key 更是整个跳过。
		if p.Cost.ActualCost > 0 && p.User != nil && p.Subscription != nil {
			deps.billingCacheService.QueueUpdateSubscriptionUsage(p.User.ID, p.Subscription.PlanID, p.Cost.ActualCost)
		}
	} else if p.Cost.ActualCost > 0 && p.User != nil {
		syncBalanceCacheAfterDeduction(ctx, p, deps, result)
	}

	if p.Cost.ActualCost > 0 && p.APIKey != nil && p.APIKey.HasRateLimits() {
		deps.billingCacheService.QueueUpdateAPIKeyRateLimitUsage(p.APIKey.ID, p.Cost.ActualCost)
	}

	deps.deferredService.ScheduleLastUsedUpdate(p.Account.ID)

	// Notification checks run async — all parameters are already captured,
	// no dependency on the request context or upstream connection.
	go notifyBalanceLow(p, deps, result)
	go notifyAccountQuota(p, deps, result)
}

func syncBalanceCacheAfterDeduction(ctx context.Context, p *postUsageBillingParams, deps *billingDeps, result *UsageBillingApplyResult) {
	if p == nil || p.Cost == nil || p.User == nil || deps == nil || deps.billingCacheService == nil {
		return
	}
	if result != nil && result.NewBalance != nil && deps.billingCacheService.balanceBelowEligibilityThreshold(*result.NewBalance) {
		if err := deps.billingCacheService.InvalidateUserBalance(ctx, p.User.ID); err != nil {
			slog.Warn("invalidate balance cache after exhausted deduction failed",
				"user_id", p.User.ID,
				"new_balance", *result.NewBalance,
				"balance_overdrafted", result.BalanceOverdrafted,
				"error", err,
			)
		}
		return
	}
	deps.billingCacheService.QueueDeductBalance(p.User.ID, p.Cost.ActualCost)
}

// notifyBalanceLow sends balance low notification after deduction.
// When result.NewBalance is available (from DB transaction RETURNING), it is used directly
// to reconstruct oldBalance, avoiding stale Redis reads and concurrent-deduction races.
func notifyBalanceLow(p *postUsageBillingParams, deps *billingDeps, result *UsageBillingApplyResult) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic in notifyBalanceLow", "recover", r)
		}
	}()
	if p.IsSubscriptionBill || p.Cost.ActualCost <= 0 || p.User == nil || deps.balanceNotifyService == nil {
		slog.Debug("notifyBalanceLow: skipped",
			"is_subscription", p.IsSubscriptionBill,
			"actual_cost", p.Cost.ActualCost,
			"user_nil", p.User == nil,
			"service_nil", deps.balanceNotifyService == nil,
		)
		return
	}

	oldBalance := resolveOldBalance(p, result)
	slog.Debug("notifyBalanceLow: calling CheckBalanceAfterDeduction",
		"user_id", p.User.ID,
		"old_balance", oldBalance,
		"cost", p.Cost.ActualCost,
		"notify_enabled", p.User.BalanceNotifyEnabled,
		"threshold", p.User.BalanceNotifyThreshold,
		"result_has_new_balance", result != nil && result.NewBalance != nil,
	)
	deps.balanceNotifyService.CheckBalanceAfterDeduction(context.Background(), p.User, oldBalance, p.Cost.ActualCost)
}

// resolveOldBalance returns the pre-deduction balance.
// Prefers the DB transaction result (newBalance + cost) over snapshot.
func resolveOldBalance(p *postUsageBillingParams, result *UsageBillingApplyResult) float64 {
	if result != nil && result.NewBalance != nil {
		return *result.NewBalance + p.Cost.ActualCost
	}
	// Legacy fallback: snapshot balance from request context
	return p.User.Balance
}

// notifyAccountQuota sends account quota threshold notification after increment.
// When result.QuotaState is available (from DB transaction RETURNING), it is passed directly
// to avoid a separate DB read that may see stale or concurrently-modified data.
func notifyAccountQuota(p *postUsageBillingParams, deps *billingDeps, result *UsageBillingApplyResult) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic in notifyAccountQuota", "recover", r)
		}
	}()
	if p.AccountCost <= 0 || p.Account == nil || !p.Account.IsAPIKeyOrBedrock() || deps.balanceNotifyService == nil {
		slog.Debug("notifyAccountQuota: skipped",
			"account_cost", p.AccountCost,
			"account_nil", p.Account == nil,
			"is_apikey_or_bedrock", p.Account != nil && p.Account.IsAPIKeyOrBedrock(),
			"service_nil", deps.balanceNotifyService == nil,
		)
		return
	}
	accountCost := p.AccountCost
	var quotaState *AccountQuotaState
	if result != nil {
		quotaState = result.QuotaState
	}
	slog.Debug("notifyAccountQuota: calling CheckAccountQuotaAfterIncrement",
		"account_id", p.Account.ID,
		"account_cost", accountCost,
		"has_quota_state", quotaState != nil,
	)
	deps.balanceNotifyService.CheckAccountQuotaAfterIncrement(context.Background(), p.Account, accountCost, quotaState)
}

func detachedBillingContext(ctx context.Context) (context.Context, context.CancelFunc) {
	base := context.Background()
	if ctx != nil {
		base = context.WithoutCancel(ctx)
	}
	return context.WithTimeout(base, postUsageBillingTimeout)
}

func detachStreamUpstreamContext(ctx context.Context, stream bool) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.Background(), func() {}
	}
	if !stream {
		return ctx, func() {}
	}
	return context.WithoutCancel(ctx), func() {}
}

func detachUpstreamContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.Background(), func() {}
	}
	return context.WithoutCancel(ctx), func() {}
}

// billingDeps 扣费逻辑依赖的服务（由各 gateway service 提供）
type billingDeps struct {
	accountRepo          AccountRepository
	userRepo             UserRepository
	userSubRepo          UserSubscriptionRepository
	billingCacheService  *BillingCacheService
	deferredService      *DeferredService
	balanceNotifyService *BalanceNotifyService
	cfg                  *config.Config
}

func (s *GatewayService) billingDeps() *billingDeps {
	return &billingDeps{
		accountRepo:          s.accountRepo,
		userRepo:             s.userRepo,
		userSubRepo:          s.userSubRepo,
		billingCacheService:  s.billingCacheService,
		deferredService:      s.deferredService,
		balanceNotifyService: s.balanceNotifyService,
		cfg:                  s.cfg,
	}
}

func writeUsageLogBestEffort(ctx context.Context, repo UsageLogRepository, usageLog *UsageLog, logKey string) {
	if repo == nil || usageLog == nil {
		return
	}
	usageCtx, cancel := detachedBillingContext(ctx)
	defer cancel()

	if writer, ok := repo.(usageLogBestEffortWriter); ok {
		if err := writer.CreateBestEffort(usageCtx, usageLog); err != nil {
			logger.LegacyPrintf(logKey, "Create usage log failed: %v", err)
			// 计费已在此前完成，日志必须落库：dropped（批处理队列超时）同样走同步兜底，
			// 否则会出现“已扣费但无 usage_log”的对账缺口（issue #3656）。
			// 重复写入由 usage_logs 的 ON CONFLICT (request_id, api_key_id) DO NOTHING 防护。
			fallbackCtx := usageCtx
			if usageCtx.Err() != nil {
				// usageCtx 已耗尽（best-effort 入队阻塞到期限）：换新的 detached 窗口，避免兜底必然失败。
				var fallbackCancel context.CancelFunc
				fallbackCtx, fallbackCancel = detachedBillingContext(context.Background())
				defer fallbackCancel()
			}
			if _, syncErr := repo.Create(fallbackCtx, usageLog); syncErr != nil {
				logger.LegacyPrintf(logKey, "Create usage log sync fallback failed: %v", syncErr)
			}
		}
		return
	}

	if _, err := repo.Create(usageCtx, usageLog); err != nil {
		logger.LegacyPrintf(logKey, "Create usage log failed: %v", err)
	}
}

// RecordUsage 记录使用量并扣费（或更新订阅用量）
func (s *GatewayService) RecordUsage(ctx context.Context, input *RecordUsageInput) error {
	return s.recordUsageCore(ctx, &recordUsageCoreInput{
		Result:             input.Result,
		APIKey:             input.APIKey,
		User:               input.User,
		Account:            input.Account,
		Subscription:       input.Subscription,
		PricingAt:          input.PricingAt,
		InboundEndpoint:    input.InboundEndpoint,
		UpstreamEndpoint:   input.UpstreamEndpoint,
		UserAgent:          input.UserAgent,
		IPAddress:          input.IPAddress,
		SessionID:          input.SessionID,
		RequestPayloadHash: input.RequestPayloadHash,
		ForceCacheBilling:  input.ForceCacheBilling,
		APIKeyService:      input.APIKeyService,
		RequestedModel:     input.RequestedModel,
		WebSearchDelegated: input.WebSearchDelegated,
	})
}

// recordUsageCoreInput 是 recordUsageCore 的公共输入字段，从两种输入结构体中提取。
type recordUsageCoreInput struct {
	Result             *ForwardResult
	APIKey             *APIKey
	User               *User
	Account            *Account
	Subscription       *UserSubscription
	PricingAt          time.Time
	InboundEndpoint    string
	UpstreamEndpoint   string
	UserAgent          string
	IPAddress          string
	SessionID          string
	RequestPayloadHash string
	ForceCacheBilling  bool
	APIKeyService      APIKeyQuotaUpdater
	RequestedModel     string
	WebSearchDelegated bool
}

// recordUsageCore 是 RecordUsage 的核心实现。
func (s *GatewayService) recordUsageCore(ctx context.Context, input *recordUsageCoreInput) error {
	result := input.Result
	apiKey := input.APIKey
	user := input.User
	account := input.Account
	subscription := input.Subscription
	ApplyForwardImageBillingResolution(result)
	logServiceTierBillingDowngrade("service.gateway", account, result.RequestID, ApplyForwardServiceTierBillingResolution(result))

	// 强制缓存计费：将 input_tokens 转为 cache_read_input_tokens
	// 用于粘性会话切换时的特殊计费处理
	if input.ForceCacheBilling && result.Usage.InputTokens > 0 {
		logger.LegacyPrintf("service.gateway", "force_cache_billing: %d input_tokens → cache_read_input_tokens (account=%d)",
			result.Usage.InputTokens, account.ID)
		result.Usage.CacheReadInputTokens += result.Usage.InputTokens
		result.Usage.InputTokens = 0
	}

	// Cache TTL Override: 确保计费时 token 分类与响应改写一致。
	// 只在全局 1h 请求注入开启时把 usage 计费归回 5m（渠道级强制替换已删）。
	cacheTTLOverridden := false
	if overrideTarget, ok := s.resolveCacheTTLUsageOverrideTarget(ctx, account); ok {
		applyCacheTTLOverride(&result.Usage, overrideTarget)
		cacheTTLOverridden = (result.Usage.CacheCreation5mTokens + result.Usage.CacheCreation1hTokens) > 0
	}

	// 用户价 = 目录价 × 用户倍率；图片按次倍率与 token 倍率是同一个数。
	multiplier := UserRateMultiplier(user)
	pricingAt := input.PricingAt
	if pricingAt.IsZero() {
		pricingAt = timezone.Now()
	}

	// 确定计费模型：请求模型（目录别名归一由 resolver 完成）。
	billingModel := forwardResultBillingModel(result.Model, result.UpstreamModel)
	// 选定模型查不到任何价格时回退到实际转发的具体模型（账号级 model_mapping 的上游名）。
	billingModel = s.billableModelWithFallback(ctx, apiKey, billingModel, result.UpstreamModel, result.Model)

	// RequestedModel：客户端原始请求模型（别名归一前）
	requestedModel := result.Model
	if input.RequestedModel != "" {
		requestedModel = input.RequestedModel
	}

	// 计算费用；渠道成本 = token 用量 × 这个渠道给这个模型的上游价（媒体用量记 0）
	cost, tokenPath := s.calculateRecordUsageCost(ctx, result, apiKey, billingModel, multiplier, pricingAt)
	accountCost := 0.0
	if tokenPath {
		accountCost = recordUsageAccountCost(ctx, s.billingService, s.resolver, account.ID, []string{billingModel},
			recordUsageTokens(result), result.webSearchUsage(), pricingAt, optionalStringValue(result.ReasoningEffort))
	}

	// 判断计费方式：订阅模式 vs 余额模式
	isSubscriptionBilling := subscription != nil
	billingType := BillingTypeBalance
	if isSubscriptionBilling {
		billingType = BillingTypeSubscription
	}

	// 创建使用日志
	usageLog := s.buildRecordUsageLog(ctx, input, result, apiKey, user, account, subscription,
		requestedModel, multiplier, accountCost, billingType, cacheTTLOverridden, cost)

	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		writeUsageLogBestEffort(ctx, s.usageLogRepo, usageLog, "service.gateway")
		logger.LegacyPrintf("service.gateway", "[SIMPLE MODE] Usage recorded (not billed): user=%d, tokens=%d", usageLog.UserID, usageLog.TotalTokens())
		s.deferredService.ScheduleLastUsedUpdate(account.ID)
		s.rateLimitService.ApplyAccountUsageState(ctx, account, usageLog.Model)
		return nil
	}

	requestID := usageLog.RequestID
	_, billingErr := applyUsageBilling(ctx, requestID, usageLog, &postUsageBillingParams{
		Cost:               cost,
		User:               user,
		APIKey:             apiKey,
		Account:            account,
		Subscription:       subscription,
		RequestPayloadHash: resolveUsageBillingPayloadFingerprint(ctx, input.RequestPayloadHash),
		IsSubscriptionBill: isSubscriptionBilling,
		AccountCost:        accountCost,
		APIKeyService:      input.APIKeyService,
	}, s.billingDeps(), s.usageBillingRepo)

	if billingErr != nil {
		usageLog.ActualCost = 0
		writeUsageLogBestEffort(ctx, s.usageLogRepo, usageLog, "service.gateway")
		return billingErr
	}
	writeUsageLogBestEffort(ctx, s.usageLogRepo, usageLog, "service.gateway")
	// 用量入账是「由我们自己的用量驱动」的额度（配额计数 / 免费档 / Gemini 本地配额）的状态写入点。
	s.rateLimitService.ApplyAccountUsageState(ctx, account, usageLog.Model)

	return nil
}

// calculateRecordUsageCost 根据请求类型计算费用：媒体用量（图片 / 音频）按目录条目，
// 其余走 token 计费，联网搜索费叠加在 token 费上。tokenPath 报告是否走了
// token 计费（渠道成本只在 token 路径上按上游价算）。
func (s *GatewayService) calculateRecordUsageCost(
	ctx context.Context,
	result *ForwardResult,
	apiKey *APIKey,
	billingModel string,
	multiplier float64,
	pricingAt time.Time,
) (cost *CostBreakdown, tokenPath bool) {
	if cost, handled, err := s.billingService.CalculateMediaCost(ctx, s.resolver, billingModel, mediaUsageFromForwardResult(result), multiplier); handled {
		if err != nil {
			logger.LegacyPrintf("service.gateway", "Calculate media cost failed: %v", err)
			return &CostBreakdown{ActualCost: 0}, false
		}
		return cost, false
	}

	// Token 计费，再叠加联网搜索费（官方原价、不乘用户倍率）。
	tokenCost := s.calculateTokenCost(ctx, result, apiKey, billingModel, multiplier, pricingAt)
	return addWebSearchCharge(ctx, s.resolver, billingModel, result.webSearchUsage(), tokenCost), true
}

// billableModelWithFallback 在选定计费模型（可能是 composite 公开别名或未定价的映射名）
// 查不到任何价格（渠道价与全局价均无）时，按序回退到实际转发的具体模型，避免静默 $0 计费。
// 所有候选都无价时保持原值，走既有的 warn + 零成本路径。
func (s *GatewayService) billableModelWithFallback(ctx context.Context, apiKey *APIKey, billingModel string, fallbacks ...string) string {
	if s.hasResolvableTokenPricing(ctx, billingModel, apiKey) {
		return billingModel
	}
	for _, fallback := range fallbacks {
		fallback = strings.TrimSpace(fallback)
		if fallback == "" || fallback == billingModel {
			continue
		}
		if s.hasResolvableTokenPricing(ctx, fallback, apiKey) {
			logger.LegacyPrintf("service.gateway", "[Billing] billing model %q has no pricing, falling back to concrete model %q", billingModel, fallback)
			return fallback
		}
	}
	return billingModel
}

// hasResolvableTokenPricing 判断模型能否沿定价解析链（模型目录 → 价格表）解析出可计费的价格。
func (s *GatewayService) hasResolvableTokenPricing(ctx context.Context, model string, apiKey *APIKey) bool {
	if strings.TrimSpace(model) == "" {
		return false
	}
	if s.resolver != nil {
		return s.resolver.Resolve(ctx, PricingInput{Model: model}).hasUsablePricing()
	}
	if s.billingService == nil {
		return false
	}
	_, err := s.billingService.GetModelPricing(model)
	return err == nil
}

// calculateTokenCost 计算 Token 计费：路径选择（分组/渠道定价 → 内置定价）
// 统一交给 BillingService.CalculateTokenCostForRequest，与模型广场的阶梯表查询同源。
func (s *GatewayService) calculateTokenCost(
	ctx context.Context,
	result *ForwardResult,
	apiKey *APIKey,
	billingModel string,
	multiplier float64,
	pricingAt time.Time,
) *CostBreakdown {
	tokens := recordUsageTokens(result)

	var resolved *ResolvedPricing
	if s.resolver != nil {
		resolved = s.resolver.Resolve(ctx, PricingInput{Model: billingModel})
	}

	cost, err := s.billingService.CalculateTokenCostForRequest(TokenCostRequest{
		Ctx:             ctx,
		Model:           billingModel,
		Tokens:          tokens,
		RateMultiplier:  multiplier,
		PricingAt:       pricingAt,
		ReasoningEffort: optionalStringValue(result.ReasoningEffort),
		Resolver:        s.resolver,
		Resolved:        resolved,
	})
	if err != nil {
		logger.LegacyPrintf("service.gateway", "Calculate cost failed: %v", err)
		return &CostBreakdown{ActualCost: 0}
	}
	return cost
}

// recordUsageTokens 这次请求按 token 计费的用量；用户价与渠道成本用同一份。
func recordUsageTokens(result *ForwardResult) UsageTokens {
	return UsageTokens{
		InputTokens:           result.Usage.InputTokens,
		OutputTokens:          result.Usage.OutputTokens,
		CacheCreationTokens:   result.Usage.CacheCreationInputTokens,
		CacheReadTokens:       result.Usage.CacheReadInputTokens,
		CacheCreation5mTokens: result.Usage.CacheCreation5mTokens,
		CacheCreation1hTokens: result.Usage.CacheCreation1hTokens,
		ImageOutputTokens:     result.Usage.ImageOutputTokens,
		// 音频 token 在 InputTokens / OutputTokens 之内（Gemini 的 AUDIO 模态）；超出部分由
		// computeTokenBreakdown 截到文本 token 数（如 ForceCacheBilling 把输入转成缓存读取后）。
		AudioInputTokens:  result.Usage.AudioInputTokens,
		AudioOutputTokens: result.Usage.AudioOutputTokens,
	}
}

// buildRecordUsageLog 构建使用日志并设置计费模式。
func (s *GatewayService) buildRecordUsageLog(
	ctx context.Context,
	input *recordUsageCoreInput,
	result *ForwardResult,
	apiKey *APIKey,
	user *User,
	account *Account,
	subscription *UserSubscription,
	requestedModel string,
	multiplier float64,
	accountCost float64,
	billingType int8,
	cacheTTLOverridden bool,
	cost *CostBreakdown,
) *UsageLog {
	durationMs := int(result.Duration.Milliseconds())
	requestID := resolveUsageBillingRequestID(ctx, result.RequestID)
	sentModel := upstreamSentModel(result.Model, result.UpstreamModel)
	if result.UpstreamResponseModelConflict {
		slog.Warn("upstream_response_model_conflict",
			"platform", account.Platform,
			"account_id", account.ID,
			"request_id", requestID,
			"sent_model", sentModel,
			"selected_response_model", strings.TrimSpace(result.UpstreamResponseModel),
		)
	}
	usageLog := &UsageLog{
		UserID:                   user.ID,
		APIKeyID:                 apiKey.ID,
		AccountID:                account.ID,
		RequestID:                requestID,
		UpstreamRequestID:        usageUpstreamRequestIDPtr(result.UpstreamHeaders, false),
		Model:                    result.Model,
		RequestedModel:           requestedModel,
		UpstreamModel:            optionalTrimmedStringPtr(result.UpstreamModel),
		UpstreamResponseModel:    optionalTrimmedStringPtr(result.UpstreamResponseModel),
		UpstreamModelMismatch:    upstreamModelMismatch(sentModel, result.UpstreamResponseModel),
		WebSearchDelegated:       input.WebSearchDelegated,
		ServiceTier:              result.ServiceTier,
		ReasoningEffort:          result.ReasoningEffort,
		RequestedReasoningEffort: coalesceRequestedReasoningEffort(result.RequestedReasoningEffort, result.ReasoningEffort),
		InboundEndpoint:          optionalTrimmedStringPtr(input.InboundEndpoint),
		UpstreamEndpoint:         optionalTrimmedStringPtr(input.UpstreamEndpoint),
		InputTokens:              result.Usage.InputTokens,
		OutputTokens:             result.Usage.OutputTokens,
		CacheCreationTokens:      result.Usage.CacheCreationInputTokens,
		CacheReadTokens:          result.Usage.CacheReadInputTokens,
		CacheCreation5mTokens:    result.Usage.CacheCreation5mTokens,
		CacheCreation1hTokens:    result.Usage.CacheCreation1hTokens,
		ImageOutputTokens:        result.Usage.ImageOutputTokens,
		RateMultiplier:           multiplier,
		AccountCost:              accountCost,
		BillingType:              billingType,
		BillingMode:              resolveBillingMode(result, cost),
		Stream:                   result.Stream,
		DurationMs:               &durationMs,
		FirstTokenMs:             result.FirstTokenMs,
		ImageCount:               result.ImageCount,
		ImageSize:                optionalTrimmedStringPtr(result.ImageSize),
		ImageInputSize:           optionalTrimmedStringPtr(result.ImageInputSize),
		ImageOutputSize:          optionalTrimmedStringPtr(result.ImageOutputSize),
		ImageSizeSource:          optionalTrimmedStringPtr(result.ImageSizeSource),
		ImageSizeBreakdown:       result.ImageSizeBreakdown,
		CacheTTLOverridden:       cacheTTLOverridden,
		UserAgent:                optionalTrimmedStringPtr(input.UserAgent),
		IPAddress:                optionalTrimmedStringPtr(input.IPAddress),
		SessionID:                optionalTrimmedStringPtr(input.SessionID),
		SubscriptionID:           optionalSubscriptionID(subscription),
		CreatedAt:                time.Now(),
	}
	if usageLog.WebSearchDelegated && usageLog.UpstreamModel == nil {
		// 代执行的搜索：请求模型记客户端写的，发往上游的模型（Haiku）明确记下来，管理站对账用
		usageLog.UpstreamModel = optionalTrimmedStringPtr(sentModel)
	}
	if cost != nil {
		usageLog.InputCost = cost.InputCost
		usageLog.OutputCost = cost.OutputCost
		usageLog.ImageOutputCost = cost.ImageOutputCost
		usageLog.CacheCreationCost = cost.CacheCreationCost
		usageLog.CacheReadCost = cost.CacheReadCost
		usageLog.TotalCost = cost.TotalCost
		usageLog.ActualCost = cost.ActualCost
		usageLog.WebSearchCount = cost.WebSearchCount
		usageLog.WebSearchCost = cost.WebSearchCost
	}

	return usageLog
}

// resolveBillingMode 根据计费结果和请求类型确定计费模式。
func resolveBillingMode(result *ForwardResult, cost *CostBreakdown) *string {
	var mode string
	switch {
	case cost != nil && cost.BillingMode != "":
		mode = cost.BillingMode
	case result.ImageCount > 0:
		mode = string(BillingModeImage)
	default:
		mode = string(BillingModeToken)
	}
	return &mode
}

func optionalSubscriptionID(subscription *UserSubscription) *int64 {
	if subscription != nil {
		return &subscription.ID
	}
	return nil
}
