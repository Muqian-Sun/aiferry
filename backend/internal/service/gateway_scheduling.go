package service

// 账号选择：唯一入口 SelectAccountWithOptions → 负载感知调度（粘性 → 负载分层 → 兜底排队），
// 窗口费用与 RPM 预取、候选排序 / 过滤。池由目录路由（条目绑定）或端点声明的平台
// （SelectOptions.Platform，无模型端点）决定；分组已不存在。

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	mathrand "math/rand"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

// SelectAccountWithOptions 唯一的选号入口：负载感知 + 等待计划，opts 是本次请求对资源的额外要求。
// 全池只剩被隔离代理后面的账号时，隔离降级成偏好：宁可用坏代理也不回 502。
func (s *GatewayService) SelectAccountWithOptions(ctx context.Context, sessionHash string, requestedModel string, excludedIDs map[int64]struct{}, opts SelectOptions) (*AccountSelectionResult, error) {
	ctx = WithSelectOptions(ctx, opts)
	result, err := s.selectAccountWithLoadAwareness(ctx, sessionHash, requestedModel, excludedIDs)
	if err == nil || openAIProxyStreamQuarantineBypassed(ctx) || (!errors.Is(err, ErrNoAvailableAccounts) && !errors.Is(err, ErrNoAvailableCompactAccounts)) {
		return result, err
	}
	blocked := s.rateLimitService.ActiveProxyQuarantines(time.Now())
	if blocked == 0 {
		return result, err
	}
	s.rateLimitService.logProxyStreamQuarantineFailOpen(requestedModel, blocked)
	return s.selectAccountWithLoadAwareness(withOpenAIProxyStreamQuarantineBypass(ctx), sessionHash, requestedModel, excludedIDs)
}

// SelectAccountWithLoadAwareness 零要求的选号（/v1/messages、Gemini 入站）。
func (s *GatewayService) SelectAccountWithLoadAwareness(ctx context.Context, sessionHash string, requestedModel string, excludedIDs map[int64]struct{}) (*AccountSelectionResult, error) {
	return s.SelectAccountWithOptions(ctx, sessionHash, requestedModel, excludedIDs, SelectOptions{})
}

// errLoadBatchDisabled 负载图未启用（配置关了或没有并发服务）：L2 走优先级 + LRU 的回退排序。
var errLoadBatchDisabled = errors.New("load batch disabled")

func (s *GatewayService) selectAccountWithLoadAwareness(ctx context.Context, sessionHash string, requestedModel string, excludedIDs map[int64]struct{}) (*AccountSelectionResult, error) {
	// 调试日志：记录调度入口参数
	excludedIDsList := make([]int64, 0, len(excludedIDs))
	for id := range excludedIDs {
		excludedIDsList = append(excludedIDsList, id)
	}
	slog.Debug("account_scheduling_starting",
		"scope_id", SchedulingScopeID(ctx),
		"model", requestedModel,
		"session", shortSessionHash(sessionHash),
		"excluded_ids", excludedIDsList)

	cfg := s.schedulingConfig()
	opts := selectOptionsFromContext(ctx)
	ctx = s.withGatewayProfitControlGate(ctx)

	var stickyAccountID int64
	var stickySource string
	if prefetch := prefetchedStickyAccountIDFromContext(ctx); prefetch > 0 {
		stickyAccountID = prefetch
		stickySource = "prefetch"
	} else if sessionHash != "" && s.cache != nil {
		if accountID, err := s.cache.GetSessionAccountID(ctx, SchedulingScopeID(ctx), sessionHash); err == nil {
			stickyAccountID = accountID
			stickySource = "cache"
		}
	}

	// [DEBUG-STICKY] 调度器入口日志
	slog.Info("sticky.scheduler_entry",
		"scope_id", SchedulingScopeID(ctx),
		"session_hash", shortSessionHash(sessionHash),
		"sticky_account_id", stickyAccountID,
		"sticky_source", stickySource,
		"model", requestedModel,
		"load_batch", cfg.LoadBatchEnabled,
		"has_concurrency_svc", s.concurrencyService != nil,
		"excluded_count", len(excludedIDs),
	)

	platform, hasForcePlatform := resolvePlatform(ctx)
	inbound := InboundProtocolFromContext(ctx)

	accounts, err := s.listSchedulableAccounts(ctx, platform, hasForcePlatform)
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, ErrNoAvailableAccounts
	}
	ctx = s.withRPMPrefetch(ctx, accounts)

	// 提前构建 accountByID（供粘性层使用）
	accountByID := make(map[int64]*Account, len(accounts))
	for i := range accounts {
		accountByID[accounts[i].ID] = &accounts[i]
	}
	isExcluded := func(accountID int64) bool {
		if excludedIDs == nil {
			return false
		}
		_, excluded := excludedIDs[accountID]
		return excluded
	}

	// ============ Layer 1.5: 粘性会话 ============
	if sessionHash != "" && stickyAccountID > 0 && !isExcluded(stickyAccountID) {
		accountID := stickyAccountID
		if accountID > 0 && !isExcluded(accountID) {
			account, ok := accountByID[accountID]
			if ok {
				// 检查账户是否需要清理粘性会话绑定
				clearSticky := shouldClearStickySession(account, requestedModel)
				if clearSticky {
					slog.Debug("sticky.layer1_5_no_routing_clear",
						"account_id", accountID,
						"reason", "should_clear_sticky_session",
						"session", shortSessionHash(sessionHash),
					)
					s.deleteStickySession(ctx, sessionHash)
				}

				platformOK := isAccountSchedulableOnPlatform(ctx, account, platform)
				admitOK, admitReason := s.candidateAdmits(ctx, account, requestedModel)
				rpmOK := s.isAccountSchedulableForRPM(ctx, account, true)

				slog.Debug("sticky.layer1_5_no_routing_checks",
					"account_id", accountID,
					"session", shortSessionHash(sessionHash),
					"clear_sticky", clearSticky,
					"platform_ok", platformOK,
					"admit_ok", admitOK,
					"admit_reason", admitReason,
					"rpm_ok", rpmOK,
				)

				if !clearSticky && platformOK && admitOK && rpmOK {
					if opts.NoSlot {
						// 计 token：粘性命中即用，不抢槽、不等待
						return s.newSelectionResult(ctx, account, false, nil, nil)
					}
					result, err := s.tryAcquireAccountSlot(ctx, accountID, account.Concurrency)
					if err == nil && result.Acquired {
						// 会话数量限制检查
						if !s.checkAndRegisterSession(ctx, account, sessionHash) {
							result.ReleaseFunc() // 释放槽位，继续到 Layer 2
							slog.Debug("sticky.layer1_5_no_routing_miss",
								"account_id", accountID,
								"reason", "session_limit",
								"session", shortSessionHash(sessionHash),
							)
						} else {
							slog.Debug("sticky.layer1_5_no_routing_hit",
								"account_id", accountID,
								"session", shortSessionHash(sessionHash),
								"result", "slot_acquired",
							)
							if s.cache != nil {
								_ = s.cache.RefreshSessionTTL(ctx, SchedulingScopeID(ctx), sessionHash, stickySessionTTL)
							}
							return s.newSelectionResult(ctx, account, true, result.ReleaseFunc, nil)
						}
					} else {
						slog.Debug("sticky.layer1_5_no_routing_slot_busy",
							"account_id", accountID,
							"session", shortSessionHash(sessionHash),
						)
					}

					waitingCount := cfg.StickySessionMaxWaiting
					if s.concurrencyService != nil {
						waitingCount, _ = s.concurrencyService.GetAccountWaitingCount(ctx, accountID)
					}
					if waitingCount < cfg.StickySessionMaxWaiting {
						// 会话数量限制检查（等待计划也需要占用会话配额）
						if !s.checkAndRegisterSession(ctx, account, sessionHash) {
							// 会话限制已满，继续到 Layer 2
						} else {
							slog.Debug("sticky.layer1_5_no_routing_hit",
								"account_id", accountID,
								"session", shortSessionHash(sessionHash),
								"result", "wait_plan",
							)
							return s.newSelectionResult(ctx, account, false, nil, &AccountWaitPlan{
								AccountID:      accountID,
								MaxConcurrency: account.Concurrency,
								Timeout:        cfg.StickySessionWaitTimeout,
								MaxWaiting:     cfg.StickySessionMaxWaiting,
							})
						}
					}
				} else if !clearSticky {
					slog.Debug("sticky.layer1_5_no_routing_miss",
						"account_id", accountID,
						"reason", "gate_check_failed",
						"session", shortSessionHash(sessionHash),
					)
				}
			} else {
				slog.Debug("sticky.layer1_5_no_routing_miss",
					"account_id", accountID,
					"reason", "account_not_in_map",
					"session", shortSessionHash(sessionHash),
				)
			}
		}
	} else if sessionHash != "" {
		slog.Debug("sticky.layer1_5_no_routing_skip",
			"sticky_account_id", stickyAccountID,
			"is_excluded", func() bool { return stickyAccountID > 0 && isExcluded(stickyAccountID) }(),
			"session", shortSessionHash(sessionHash),
			"reason", func() string {
				if stickyAccountID == 0 {
					return "no_sticky_binding"
				}
				return "sticky_account_excluded"
			}(),
		)
	}

	// ============ Layer 2: 负载感知选择 ============
	slog.Debug("sticky.layer2_fallback",
		"session", shortSessionHash(sessionHash),
		"sticky_account_id", stickyAccountID,
		"reason", "sticky_not_used_falling_back_to_load_balance",
		"total_accounts", len(accounts),
	)
	candidates := make([]*Account, 0, len(accounts))
	compactRejected := 0
	for i := range accounts {
		acc := &accounts[i]
		if isExcluded(acc.ID) {
			continue
		}
		if !isAccountSchedulableOnPlatform(ctx, acc, platform) {
			continue
		}
		// Scheduler snapshots can be temporarily stale (bucket rebuild is throttled);
		// candidateAdmits re-checks schedulability so recently rate-limited/overloaded
		// accounts are not selected again before the bucket is rebuilt.
		if ok, reason := s.candidateAdmits(ctx, acc, requestedModel); !ok {
			if reason == admitReasonCompactUnsupported {
				compactRejected++
			}
			continue
		}
		// RPM 检查（非粘性会话路径）
		if !s.isAccountSchedulableForRPM(ctx, acc, false) {
			continue
		}
		candidates = append(candidates, acc)
	}

	if len(candidates) == 0 {
		if compactRejected > 0 {
			return nil, ErrNoAvailableCompactAccounts
		}
		return nil, ErrNoAvailableAccounts
	}

	if opts.NoSlot {
		// 计 token 不抢槽、不绑粘性、不等待：粘性命中且在候选里就用它，否则优先级 + LRU 首个。
		if stickyAccountID > 0 {
			for _, acc := range candidates {
				if acc.ID == stickyAccountID {
					return s.newSelectionResult(ctx, acc, false, nil, nil)
				}
			}
		}
		ordered := append([]*Account(nil), candidates...)
		sortAccountsByPriorityAndLastUsed(ordered, inbound)
		return s.newSelectionResult(ctx, ordered[0], false, nil, nil)
	}

	accountLoads := make([]AccountWithConcurrency, 0, len(candidates))
	for _, acc := range candidates {
		accountLoads = append(accountLoads, AccountWithConcurrency{
			ID:             acc.ID,
			MaxConcurrency: acc.EffectiveLoadFactor(),
		})
	}

	// 负载图：配置关了或没有并发服务时（与取负载图失败同一条路）按优先级 + LRU 回退排序。
	var loadMap map[int64]*AccountLoadInfo
	loadErr := errLoadBatchDisabled
	if s.concurrencyService != nil && cfg.LoadBatchEnabled {
		loadMap, loadErr = s.concurrencyService.GetAccountsLoadBatch(ctx, accountLoads)
	}
	if loadErr != nil {
		if result, ok, legacyErr := s.tryAcquireByLegacyOrder(ctx, candidates, sessionHash, inbound); legacyErr != nil {
			return nil, legacyErr
		} else if ok {
			return result, nil
		}
	} else {
		var available []accountWithLoad
		for _, acc := range candidates {
			loadInfo := loadMap[acc.ID]
			if loadInfo == nil {
				loadInfo = &AccountLoadInfo{AccountID: acc.ID}
			}
			if loadInfo.LoadRate < 100 {
				available = append(available, accountWithLoad{
					account:  acc,
					loadInfo: loadInfo,
				})
			}
		}

		// 分层过滤选择：协议直连 → 优先级 →（可选）最早重置 → 负载率 → LRU
		for len(available) > 0 {
			// 0. 有能以入站协议直连的就只在它们里挑（协议匹配优先，系统特色之一）
			candidates := filterByProtocolMatch(available, inbound)
			// 0.5 /responses/compact：明确支持 compact 的先于「未探测」的（tier 2 > tier 1）
			if selectOptionsFromContext(ctx).RequireCompact {
				candidates = filterByMaxCompactTier(candidates)
			}
			// 1. 取优先级最小的集合
			candidates = filterByMinPriority(candidates)
			// 2. （可选）use-it-or-lose-it：优先选用会话窗口最早重置的账号
			if cfg.PreferSoonestReset {
				candidates = filterBySoonestReset(candidates)
			}
			// 3. 取负载率最低的集合
			candidates = filterByMinLoadRate(candidates)
			// 4. LRU 选择最久未用的账号
			selected := selectByLRU(candidates)
			if selected == nil {
				break
			}

			result, err := s.tryAcquireAccountSlot(ctx, selected.account.ID, selected.account.Concurrency)
			if err == nil && result.Acquired {
				// 会话数量限制检查
				if !s.checkAndRegisterSession(ctx, selected.account, sessionHash) {
					result.ReleaseFunc() // 释放槽位，继续尝试下一个账号
				} else {
					if sessionHash != "" && s.cache != nil {
						_ = s.bindGatewayStickySessionDuringSelection(ctx, sessionHash, selected.account.ID)
					}
					return s.newSelectionResult(ctx, selected.account, true, result.ReleaseFunc, nil)
				}
			}

			// 移除已尝试的账号，重新进行分层过滤
			selectedID := selected.account.ID
			newAvailable := make([]accountWithLoad, 0, len(available)-1)
			for _, acc := range available {
				if acc.account.ID != selectedID {
					newAvailable = append(newAvailable, acc)
				}
			}
			available = newAvailable
		}
	}

	// ============ Layer 3: 兜底排队 ============
	s.sortCandidatesForFallback(candidates, inbound, cfg.FallbackSelectionMode)
	for _, acc := range candidates {
		// 会话数量限制检查（等待计划也需要占用会话配额）
		if !s.checkAndRegisterSession(ctx, acc, sessionHash) {
			continue // 会话限制已满，尝试下一个账号
		}
		return s.newSelectionResult(ctx, acc, false, nil, &AccountWaitPlan{
			AccountID:      acc.ID,
			MaxConcurrency: acc.Concurrency,
			Timeout:        cfg.FallbackWaitTimeout,
			MaxWaiting:     cfg.FallbackMaxWaiting,
		})
	}
	return nil, ErrNoAvailableAccounts
}

// stickyAccountIDForSelection 本次请求的粘性账号：handler 预取的（续链 / 守护父线程亲和 / Messages 提前查的）优先，
// 其次缓存里的会话绑定；没有返回 0。
func (s *GatewayService) stickyAccountIDForSelection(ctx context.Context, sessionHash string) int64 {
	if prefetch := prefetchedStickyAccountIDFromContext(ctx); prefetch > 0 {
		return prefetch
	}
	if sessionHash == "" || s.cache == nil {
		return 0
	}
	accountID, err := s.cache.GetSessionAccountID(ctx, SchedulingScopeID(ctx), sessionHash)
	if err != nil {
		return 0
	}
	return accountID
}

// deleteStickySession 清缓存里的会话绑定（缓存未配置时 no-op）。
func (s *GatewayService) deleteStickySession(ctx context.Context, sessionHash string) {
	if s.cache == nil || sessionHash == "" {
		return
	}
	_ = s.cache.DeleteSessionAccountID(ctx, SchedulingScopeID(ctx), sessionHash)
}

func (s *GatewayService) tryAcquireByLegacyOrder(ctx context.Context, candidates []*Account, sessionHash string, inbound string) (*AccountSelectionResult, bool, error) {
	ordered := append([]*Account(nil), candidates...)
	sortAccountsByPriorityAndLastUsed(ordered, inbound)

	for _, acc := range ordered {
		result, err := s.tryAcquireAccountSlot(ctx, acc.ID, acc.Concurrency)
		if err == nil && result.Acquired {
			// 会话数量限制检查
			if !s.checkAndRegisterSession(ctx, acc, sessionHash) {
				result.ReleaseFunc() // 释放槽位，继续尝试下一个账号
				continue
			}
			if sessionHash != "" && s.cache != nil {
				_ = s.bindGatewayStickySessionDuringSelection(ctx, sessionHash, acc.ID)
			}
			selection, err := s.newSelectionResult(ctx, acc, true, result.ReleaseFunc, nil)
			if err != nil {
				return nil, false, err
			}
			return selection, true, nil
		}
	}

	return nil, false, nil
}

func (s *GatewayService) schedulingConfig() config.GatewaySchedulingConfig {
	if s.cfg != nil {
		return s.cfg.Gateway.Scheduling
	}
	return config.GatewaySchedulingConfig{
		StickySessionMaxWaiting:  3,
		StickySessionWaitTimeout: 45 * time.Second,
		FallbackWaitTimeout:      30 * time.Second,
		FallbackMaxWaiting:       100,
		LoadBatchEnabled:         true,
		SlotCleanupInterval:      30 * time.Second,
	}
}

// resolvePlatform 本次请求的调度平台：强制平台（/antigravity 路由）→ 端点声明的平台
// （SelectOptions.Platform，无模型端点）→ 空串。目录路由不需要平台（池 = 条目绑定，强制平台只
// 用来把池收窄到 antigravity 成品号）；无路由又无平台的请求没有池，listSchedulableAccounts 回无可用账号。
func resolvePlatform(ctx context.Context) (platform string, forced bool) {
	if forcePlatform, ok := ctx.Value(ctxkey.ForcePlatform).(string); ok && forcePlatform != "" {
		return forcePlatform, true
	}
	return selectOptionsFromContext(ctx).Platform, false
}

// listSchedulableAccounts 本次请求的池：目录路由 = 条目绑定的账号；否则 = platform 平台的全部资源。
// 快照在就读快照（桶由快照按同一规则定），否则直接查库；两条路都再按生效平台与入站协议过滤。
func (s *GatewayService) listSchedulableAccounts(ctx context.Context, platform string, hasForcePlatform bool) ([]Account, error) {
	route, routed := CatalogRouteFromContext(ctx)
	if !routed && platform == "" {
		slog.Debug("account_scheduling_no_platform", "inbound", InboundProtocolFromContext(ctx))
		return nil, ErrNoAvailableAccounts
	}
	var accounts []Account
	var err error
	switch {
	case s.schedulerSnapshot != nil:
		accounts, err = s.schedulerSnapshot.ListSchedulableAccounts(ctx, platform)
	case routed:
		accounts, err = s.accountRepo.ListSchedulingCandidatesByCatalogEntry(ctx, route.EntryID)
		if err == nil {
			accounts = filterAccountsSchedulableOnPlatform(ctx, accounts, platform)
		}
	default:
		accounts, err = s.accountRepo.ListSchedulingCandidates(ctx, []string{platform})
		if err == nil {
			accounts = filterAccountsSchedulableOnPlatform(ctx, accounts, platform)
		}
	}
	if err != nil {
		slog.Debug("account_scheduling_list_failed", "scope_id", SchedulingScopeID(ctx), "platform", platform, "error", err)
		return nil, err
	}
	slog.Debug("account_scheduling_list", "scope_id", SchedulingScopeID(ctx), "platform", platform, "snapshot", s.schedulerSnapshot != nil, "count", len(accounts))
	if slog.Default().Enabled(ctx, slog.LevelDebug) {
		for _, acc := range accounts {
			slog.Debug("account_scheduling_account_detail",
				"account_id", acc.ID,
				"name", acc.Name,
				"platform", acc.Platform,
				"type", acc.Type,
				"status", acc.Status,
				"tls_fingerprint", acc.IsTLSFingerprintEnabled())
		}
	}
	return accounts, nil
}

// IsSinglePool 本次请求的池是否只有一个可调度资源（目录路由按条目绑定数，不看平台；无路由 = false）。
// Handler 层在首次请求时据此提前设置 SingleAccountRetry context，避免单资源池收到 503 时
// 错误地设置模型限流标记导致后续请求连续快速失败。
func (s *GatewayService) IsSinglePool(ctx context.Context) bool {
	if _, ok := CatalogRouteFromContext(ctx); !ok {
		return false
	}
	accounts, err := s.listSchedulableAccounts(ctx, "", false)
	if err != nil {
		return false
	}
	return len(accounts) == 1
}

// isAccountSchedulableForSelection 选号准入：只读资源的调度状态（整体），额度评估不在这里。
const admitReasonCompactUnsupported = "compact_unsupported"

// candidateAdmits 是所有层共用的候选准入：状态 → 进程内熔断 → 利润 → 模型 →
// 请求要求（SelectOptions）→ 影子母账号。平台 / 池成员 / 渠道限制 / RPM / 会话数仍在各层自己判
// （它们按层不同：粘性层 RPM 用 isSticky=true）。reason 只给诊断用。
func (s *GatewayService) candidateAdmits(ctx context.Context, account *Account, requestedModel string) (bool, string) {
	now := time.Now()
	if !account.SchedulingAllows(ctx, requestedModel, now) {
		return false, "not_schedulable"
	}
	if s.rateLimitService.ModelTransientBlocked(account, requestedModel, now) {
		return false, "model_transient_blocked"
	}
	if grokModelRuntimeBlocked(account, requestedModel, now) {
		return false, "grok_model_blocked"
	}
	if s.rateLimitService.ProxyStreamQuarantined(ctx, account) {
		return false, "proxy_quarantined"
	}
	if !s.isGatewayAccountProfitEligible(ctx, account) {
		return false, "profit_threshold"
	}
	if requestedModel != "" && !s.isModelSupportedByAccountWithContext(ctx, account, requestedModel) {
		return false, "model_not_supported"
	}
	if ok, reason := selectOptionsFromContext(ctx).admits(s.cfg, s.wsProtocolResolver(), account); !ok {
		return false, reason
	}
	if !parentHealthyForShadow(account, s.shadowParentLookup(ctx)) {
		return false, "shadow_parent_unhealthy"
	}
	return true, ""
}

// admits 是 candidateAdmits 的布尔版（粘性层的长条件链用）。
func (s *GatewayService) admits(ctx context.Context, account *Account, requestedModel string) bool {
	ok, _ := s.candidateAdmits(ctx, account, requestedModel)
	return ok
}

// shadowParentLookup 影子账号的母账号解析：快照优先（与选中后的 hydrate 同源）。
func (s *GatewayService) shadowParentLookup(ctx context.Context) func(int64) *Account {
	return func(id int64) *Account {
		account, err := s.getSchedulableAccount(ctx, id)
		if err != nil {
			return nil
		}
		return account
	}
}

func (s *GatewayService) tryAcquireAccountSlot(ctx context.Context, accountID int64, maxConcurrency int) (*AcquireResult, error) {
	if s.concurrencyService == nil {
		return &AcquireResult{Acquired: true, ReleaseFunc: func() {}}, nil
	}
	return s.concurrencyService.AcquireAccountSlot(ctx, accountID, maxConcurrency)
}

// rpmPrefetchContextKey is the context key for prefetched RPM counts.
type rpmPrefetchContextKeyType struct{}

var rpmPrefetchContextKey = rpmPrefetchContextKeyType{}

func rpmFromPrefetchContext(ctx context.Context, accountID int64) (int, bool) {
	if v, ok := ctx.Value(rpmPrefetchContextKey).(map[int64]int); ok {
		count, found := v[accountID]
		return count, found
	}
	return 0, false
}

// withRPMPrefetch 批量预取所有候选账号的 RPM 计数
func (s *GatewayService) withRPMPrefetch(ctx context.Context, accounts []Account) context.Context {
	if s.rpmCache == nil {
		return ctx
	}

	var ids []int64
	for i := range accounts {
		if accounts[i].GetBaseRPM() > 0 {
			ids = append(ids, accounts[i].ID)
		}
	}
	if len(ids) == 0 {
		return ctx
	}

	counts, err := s.rpmCache.GetRPMBatch(ctx, ids)
	if err != nil {
		return ctx // 失败开放
	}
	return context.WithValue(ctx, rpmPrefetchContextKey, counts)
}

// isAccountSchedulableForRPM 检查账号是否可根据 RPM 进行调度：任何设了 base_rpm 的资源都算，不问类型。
func (s *GatewayService) isAccountSchedulableForRPM(ctx context.Context, account *Account, isSticky bool) bool {
	baseRPM := account.GetBaseRPM()
	if baseRPM <= 0 {
		return true
	}

	// 尝试从预取缓存获取
	var currentRPM int
	if count, ok := rpmFromPrefetchContext(ctx, account.ID); ok {
		currentRPM = count
	} else if s.rpmCache != nil {
		if count, err := s.rpmCache.GetRPM(ctx, account.ID); err == nil {
			currentRPM = count
		}
		// 失败开放：GetRPM 错误时允许调度
	}

	schedulability := account.CheckRPMSchedulability(currentRPM)
	switch schedulability {
	case WindowCostSchedulable:
		return true
	case WindowCostStickyOnly:
		return isSticky
	case WindowCostNotSchedulable:
		return false
	}
	return true
}

// IncrementAccountRPM increments the RPM counter for the given account.
// 已知 TOCTOU 竞态：调度时读取 RPM 计数与此处递增之间存在时间窗口，
// 高并发下可能短暂超出 RPM 限制。这是与 WindowCost 一致的 soft-limit
// 设计权衡——可接受的少量超额优于加锁带来的延迟和复杂度。
func (s *GatewayService) IncrementAccountRPM(ctx context.Context, accountID int64) error {
	if s.rpmCache == nil {
		return nil
	}
	_, err := s.rpmCache.IncrementRPM(ctx, accountID)
	return err
}

// checkAndRegisterSession 检查并注册会话，用于会话数量限制：任何设了 max_sessions 的资源都算，不问类型。
// sessionID: 会话标识符（使用粘性会话的 hash）
// 返回 true 表示允许（在限制内或会话已存在），false 表示拒绝（超出限制且是新会话）
func (s *GatewayService) checkAndRegisterSession(ctx context.Context, account *Account, sessionID string) bool {
	maxSessions := account.GetMaxSessions()
	if maxSessions <= 0 || sessionID == "" {
		return true // 未启用会话限制或无会话ID
	}

	if s.sessionLimitCache == nil {
		return true // 缓存不可用时允许通过
	}

	idleTimeout := time.Duration(account.GetSessionIdleTimeoutMinutes()) * time.Minute

	allowed, err := s.sessionLimitCache.RegisterSession(ctx, account.ID, sessionID, maxSessions, idleTimeout)
	if err != nil {
		// 失败开放：缓存错误时允许通过
		return true
	}
	return allowed
}

// ReleaseAccountSession 立即释放会话槽（不等待空闲超时）
// 供 handler 在请求最终失败（选号成功但转发失败/客户端中断）时调用：
// 上游从未真正服务该会话，若继续占槽，max_sessions 受限的账号会被失败请求的
// session hash 卡满整个空闲窗口，后续新会话全部被拒。
// 适用条件与 checkAndRegisterSession 对齐；不适用账号为 no-op，幂等可安全重复调用。
func (s *GatewayService) ReleaseAccountSession(ctx context.Context, account *Account, sessionID string) {
	if s == nil || s.sessionLimitCache == nil || account == nil || sessionID == "" {
		return
	}
	if account.GetMaxSessions() <= 0 {
		return
	}
	if err := s.sessionLimitCache.UnregisterSession(ctx, account.ID, sessionID); err != nil {
		slog.Debug("session_limit.release_failed",
			"account_id", account.ID,
			"error", err)
	}
}

func (s *GatewayService) getSchedulableAccount(ctx context.Context, accountID int64) (*Account, error) {
	var (
		account *Account
		err     error
	)
	if s.schedulerSnapshot != nil {
		account, err = s.schedulerSnapshot.GetAccount(ctx, accountID)
	} else {
		account, err = s.accountRepo.GetByID(ctx, accountID)
	}
	if err != nil || account == nil {
		return account, err
	}
	return account, nil
}

func (s *GatewayService) hydrateSelectedAccount(ctx context.Context, account *Account) (*Account, error) {
	if account == nil || s.schedulerSnapshot == nil {
		return account, nil
	}
	hydrated, err := s.schedulerSnapshot.GetAccount(ctx, account.ID)
	if err != nil {
		return nil, err
	}
	if hydrated == nil {
		return nil, fmt.Errorf("selected gateway account %d not found during hydration", account.ID)
	}
	return hydrated, nil
}

func (s *GatewayService) newSelectionResult(ctx context.Context, account *Account, acquired bool, release func(), waitPlan *AccountWaitPlan) (*AccountSelectionResult, error) {
	hydrated, err := s.hydrateSelectedAccount(ctx, account)
	if err != nil {
		return nil, err
	}
	return attachSelectionProfitGate(ctx, &AccountSelectionResult{
		Account:     hydrated,
		Acquired:    acquired,
		ReleaseFunc: release,
		WaitPlan:    waitPlan,
	}), nil
}

// filterByMaxCompactTier 有明确支持 compact（tier 2）的就只留它们，否则原样（未探测的 tier 1 仍是候选）。
func filterByMaxCompactTier(accounts []accountWithLoad) []accountWithLoad {
	var known []accountWithLoad
	for _, acc := range accounts {
		if openAICompactSupportTier(acc.account) == 2 {
			known = append(known, acc)
		}
	}
	if len(known) == 0 {
		return accounts
	}
	return known
}

// filterByMinPriority 过滤出优先级最小的账号集合
func filterByMinPriority(accounts []accountWithLoad) []accountWithLoad {
	if len(accounts) == 0 {
		return accounts
	}
	minPriority := accounts[0].account.Priority
	for _, acc := range accounts[1:] {
		if acc.account.Priority < minPriority {
			minPriority = acc.account.Priority
		}
	}
	result := make([]accountWithLoad, 0, len(accounts))
	for _, acc := range accounts {
		if acc.account.Priority == minPriority {
			result = append(result, acc)
		}
	}
	return result
}

// filterByMinLoadRate 过滤出负载率最低的账号集合
func filterByMinLoadRate(accounts []accountWithLoad) []accountWithLoad {
	if len(accounts) == 0 {
		return accounts
	}
	minLoadRate := accounts[0].loadInfo.LoadRate
	for _, acc := range accounts[1:] {
		if acc.loadInfo.LoadRate < minLoadRate {
			minLoadRate = acc.loadInfo.LoadRate
		}
	}
	result := make([]accountWithLoad, 0, len(accounts))
	for _, acc := range accounts {
		if acc.loadInfo.LoadRate == minLoadRate {
			result = append(result, acc)
		}
	}
	return result
}

// filterBySoonestReset 过滤出「会话窗口最早重置」的账号集合（use-it-or-lose-it）。
// 仅保留拥有未来重置时间（SessionWindowEnd 在当前时间之后）且最早的账号；
// 窗口为空或已过期的账号视为无活跃窗口、优先级最低。
// 当所有账号都没有活跃窗口时，返回原集合（不改变后续 LRU 选择）。
func filterBySoonestReset(accounts []accountWithLoad) []accountWithLoad {
	if len(accounts) <= 1 {
		return accounts
	}
	now := time.Now()
	var minEnd *time.Time
	for _, acc := range accounts {
		end := acc.account.SessionWindowEnd
		if end == nil || !now.Before(*end) {
			continue
		}
		if minEnd == nil || end.Before(*minEnd) {
			minEnd = end
		}
	}
	if minEnd == nil {
		// 没有任何账号拥有活跃窗口，保持原集合
		return accounts
	}
	result := make([]accountWithLoad, 0, len(accounts))
	for _, acc := range accounts {
		end := acc.account.SessionWindowEnd
		if end != nil && now.Before(*end) && end.Equal(*minEnd) {
			result = append(result, acc)
		}
	}
	return result
}

// selectByLRU 从集合中选择最久未用的账号
// 如果有多个账号具有相同的最小 LastUsedAt，则随机选择一个
func selectByLRU(accounts []accountWithLoad) *accountWithLoad {
	if len(accounts) == 0 {
		return nil
	}
	if len(accounts) == 1 {
		return &accounts[0]
	}

	// 1. 找到最小的 LastUsedAt（nil 被视为最小）
	var minTime *time.Time
	hasNil := false
	for _, acc := range accounts {
		if acc.account.LastUsedAt == nil {
			hasNil = true
			break
		}
		if minTime == nil || acc.account.LastUsedAt.Before(*minTime) {
			minTime = acc.account.LastUsedAt
		}
	}

	// 2. 收集所有具有最小 LastUsedAt 的账号索引
	var candidateIdxs []int
	for i, acc := range accounts {
		if hasNil {
			if acc.account.LastUsedAt == nil {
				candidateIdxs = append(candidateIdxs, i)
			}
		} else {
			if acc.account.LastUsedAt != nil && acc.account.LastUsedAt.Equal(*minTime) {
				candidateIdxs = append(candidateIdxs, i)
			}
		}
	}

	// 3. 如果只有一个候选，直接返回
	if len(candidateIdxs) == 1 {
		return &accounts[candidateIdxs[0]]
	}

	// 4. 随机选择一个
	selectedIdx := candidateIdxs[mathrand.Intn(len(candidateIdxs))]
	return &accounts[selectedIdx]
}

func sortAccountsByPriorityAndLastUsed(accounts []*Account, inbound string) {
	sort.SliceStable(accounts, func(i, j int) bool {
		a, b := accounts[i], accounts[j]
		if ra, rb := protocolRank(a, inbound), protocolRank(b, inbound); ra != rb {
			return ra < rb
		}
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		switch {
		case a.LastUsedAt == nil && b.LastUsedAt != nil:
			return true
		case a.LastUsedAt != nil && b.LastUsedAt == nil:
			return false
		case a.LastUsedAt == nil && b.LastUsedAt == nil:
			return false
		default:
			return a.LastUsedAt.Before(*b.LastUsedAt)
		}
	})
	shuffleWithinPriorityAndLastUsed(accounts, inbound)
}

// shuffleWithinSortGroups 对排序后的 accountWithLoad 切片，按 (Priority, LoadRate, LastUsedAt) 分组后组内随机打乱。
// 防止并发请求读取同一快照时，确定性排序导致所有请求命中相同账号。
func shuffleWithinSortGroups(accounts []accountWithLoad) {
	if len(accounts) <= 1 {
		return
	}
	i := 0
	for i < len(accounts) {
		j := i + 1
		for j < len(accounts) && sameAccountWithLoadGroup(accounts[i], accounts[j]) {
			j++
		}
		if j-i > 1 {
			mathrand.Shuffle(j-i, func(a, b int) {
				accounts[i+a], accounts[i+b] = accounts[i+b], accounts[i+a]
			})
		}
		i = j
	}
}

// sameAccountWithLoadGroup 判断两个 accountWithLoad 是否属于同一排序组
func sameAccountWithLoadGroup(a, b accountWithLoad) bool {
	if a.account.Priority != b.account.Priority {
		return false
	}
	if a.loadInfo.LoadRate != b.loadInfo.LoadRate {
		return false
	}
	return sameLastUsedAt(a.account.LastUsedAt, b.account.LastUsedAt)
}

// shuffleWithinPriorityAndLastUsed 对排序后的 []*Account 切片，按 (协议直连, Priority, LastUsedAt) 分组后
// 组内随机打乱，避免并发请求读同一快照时全部命中同一个账号。
func shuffleWithinPriorityAndLastUsed(accounts []*Account, inbound string) {
	if len(accounts) <= 1 {
		return
	}
	i := 0
	for i < len(accounts) {
		j := i + 1
		for j < len(accounts) && sameAccountGroup(accounts[i], accounts[j], inbound) {
			j++
		}
		if j-i > 1 {
			mathrand.Shuffle(j-i, func(a, b int) {
				accounts[i+a], accounts[i+b] = accounts[i+b], accounts[i+a]
			})
		}
		i = j
	}
}

// sameAccountGroup 判断两个 Account 是否属于同一排序组（协议直连 + Priority + LastUsedAt）
func sameAccountGroup(a, b *Account, inbound string) bool {
	if protocolRank(a, inbound) != protocolRank(b, inbound) {
		return false
	}
	if a.Priority != b.Priority {
		return false
	}
	return sameLastUsedAt(a.LastUsedAt, b.LastUsedAt)
}

// protocolRank 选号排序第一键：能以入站协议直连的资源排前（0），需要转换的排后（1）。
// 协议匹配优先是系统特色之一，排在配置的优先级之前。
func protocolRank(account *Account, inbound string) int {
	if account.ProtocolMatches(inbound) {
		return 0
	}
	return 1
}

// filterByProtocolMatch 有能以入站协议直连的候选就只留它们，否则原样返回（全部要转换时按优先级挑）。
func filterByProtocolMatch(accounts []accountWithLoad, inbound string) []accountWithLoad {
	if inbound == "" || len(accounts) == 0 {
		return accounts
	}
	var matched []accountWithLoad
	for _, acc := range accounts {
		if acc.account.ProtocolMatches(inbound) {
			matched = append(matched, acc)
		}
	}
	if len(matched) == 0 {
		return accounts
	}
	return matched
}

// candidatePrecedes 报告 candidate 应排在 current 前：协议直连 → 优先级 → 从未用过 → 更久未用。
// 非负载感知路径（legacy / 单平台 / 混合）逐个比较时用它，与排序函数同一套键。
func candidatePrecedes(candidate, current *Account, inbound string) bool {
	if rc, ru := protocolRank(candidate, inbound), protocolRank(current, inbound); rc != ru {
		return rc < ru
	}
	if candidate.Priority != current.Priority {
		return candidate.Priority < current.Priority
	}
	switch {
	case candidate.LastUsedAt == nil && current.LastUsedAt != nil:
		return true
	case candidate.LastUsedAt == nil || current.LastUsedAt == nil:
		return false
	default:
		return candidate.LastUsedAt.Before(*current.LastUsedAt)
	}
}

// sameLastUsedAt 判断两个 LastUsedAt 是否相同（精度到秒）
func sameLastUsedAt(a, b *time.Time) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return a.Unix() == b.Unix()
	}
}

// sortCandidatesForFallback 根据配置选择排序策略
// mode: "last_used"(按最后使用时间) 或 "random"(随机)
func (s *GatewayService) sortCandidatesForFallback(accounts []*Account, inbound string, mode string) {
	if mode == "random" {
		// 先按协议直连 + 优先级排序，然后在同组内随机打乱
		sortAccountsByPriorityOnly(accounts, inbound)
		shuffleWithinPriority(accounts, inbound)
	} else {
		// 默认按最后使用时间排序
		sortAccountsByPriorityAndLastUsed(accounts, inbound)
	}
}

// sortAccountsByPriorityOnly 按协议直连 + 优先级排序
func sortAccountsByPriorityOnly(accounts []*Account, inbound string) {
	sort.SliceStable(accounts, func(i, j int) bool {
		a, b := accounts[i], accounts[j]
		if ra, rb := protocolRank(a, inbound), protocolRank(b, inbound); ra != rb {
			return ra < rb
		}
		return a.Priority < b.Priority
	})
}

// shuffleWithinPriority 在同（协议直连, 优先级）组内随机打乱顺序
func shuffleWithinPriority(accounts []*Account, inbound string) {
	if len(accounts) <= 1 {
		return
	}
	r := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
	start := 0
	for start < len(accounts) {
		priority := accounts[start].Priority
		rank := protocolRank(accounts[start], inbound)
		end := start + 1
		for end < len(accounts) && accounts[end].Priority == priority && protocolRank(accounts[end], inbound) == rank {
			end++
		}
		// 对 [start, end) 范围内的账户随机打乱
		if end-start > 1 {
			r.Shuffle(end-start, func(i, j int) {
				accounts[start+i], accounts[start+j] = accounts[start+j], accounts[start+i]
			})
		}
		start = end
	}
}

// isModelSupportedByAccountWithContext 根据账户上游厂商检查模型支持（带 context）
// 对于 Antigravity 上游，会先获取映射后的最终模型名（包括 thinking 后缀）再检查支持。
// Antigravity 只有成品号（第三方 key 的 Vendor 不会是 antigravity），标签为
// antigravity 的 key 按普通账号的映射判定。
func (s *GatewayService) isModelSupportedByAccountWithContext(ctx context.Context, account *Account, requestedModel string) bool {
	if account.Vendor() == PlatformAntigravity {
		if strings.TrimSpace(requestedModel) == "" {
			return true
		}
		// 使用与转发阶段一致的映射逻辑：自定义映射优先 → 默认映射兜底
		mapped := mapAntigravityModel(account, requestedModel)
		if mapped == "" {
			return false
		}
		// 应用 thinking 后缀后检查最终模型是否在账号映射中
		if enabled, ok := ThinkingEnabledFromContext(ctx); ok {
			finalModel := applyThinkingModelSuffix(mapped, enabled)
			if finalModel == mapped {
				return true // thinking 后缀未改变模型名，映射已通过
			}
			return account.IsModelSupported(finalModel)
		}
		return true
	}
	return s.isModelSupportedByAccount(account, requestedModel)
}

// isModelSupportedByAccount 根据账户上游厂商检查模型支持（无 context，用于非 Antigravity 上游）
func (s *GatewayService) isModelSupportedByAccount(account *Account, requestedModel string) bool {
	if account.Vendor() == PlatformAntigravity {
		if strings.TrimSpace(requestedModel) == "" {
			return true
		}
		return mapAntigravityModel(account, requestedModel) != ""
	}
	if account.IsBedrock() {
		_, ok := ResolveBedrockModelID(account, requestedModel)
		return ok
	}
	// OpenAI 透传模式：仅替换认证，允许所有模型。透传是 OpenAI 标准协议特性，
	// 只对官方 OpenAI 与通用中转生效。
	if openAIProtocolFeaturesApply(account) && account.IsOpenAIPassthroughEnabled() {
		return true
	}
	// OAuth/SetupToken/Vertex 成品号使用 Anthropic 标准映射（短ID → 长ID）。
	// 第三方 key 不论标签都不走这条：它的模型名由管理员映射决定，不做官方短名展开。
	if !account.IsThirdPartyKey() && account.Platform == PlatformAnthropic {
		if account.Type == AccountTypeServiceAccount {
			requestedModel = normalizeVertexAnthropicModelID(claude.NormalizeModelID(requestedModel))
		} else {
			requestedModel = claude.NormalizeModelID(requestedModel)
		}
	}
	// 其他平台使用账户的模型支持检查
	return account.IsModelSupported(requestedModel)
}
