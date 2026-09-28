package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// Account management implementations
func (s *adminServiceImpl) ListAccounts(ctx context.Context, page, pageSize int, platform, accountType, status, search string, privacyMode string, sortBy, sortOrder string) ([]Account, int64, error) {
	params := pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: sortBy, SortOrder: sortOrder}
	accounts, result, err := s.accountRepo.ListWithFilters(ctx, params, platform, accountType, status, search, privacyMode)
	if err != nil {
		return nil, 0, err
	}
	return accounts, result.Total, nil
}

func (s *adminServiceImpl) GetAccount(ctx context.Context, id int64) (*Account, error) {
	return s.accountRepo.GetByID(ctx, id)
}

func (s *adminServiceImpl) GetAccountsByIDs(ctx context.Context, ids []int64) ([]*Account, error) {
	if len(ids) == 0 {
		return []*Account{}, nil
	}

	accounts, err := s.accountRepo.GetByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts by IDs: %w", err)
	}

	return accounts, nil
}

const maxAccountNameRunes = 100
const duplicateAccountOperationIDExtraKey = "duplicate_operation_id"

func duplicateAccountName(sourceName string) string {
	const suffix = " (Copy)"
	nameRunes := []rune(strings.TrimSpace(sourceName))
	maxBaseRunes := maxAccountNameRunes - len([]rune(suffix))
	if len(nameRunes) > maxBaseRunes {
		nameRunes = nameRunes[:maxBaseRunes]
	}
	return string(nameRunes) + suffix
}

func cloneAccountJSONMap(value map[string]any) (map[string]any, error) {
	if value == nil {
		return nil, nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	cloned := make(map[string]any, len(value))
	if err := json.Unmarshal(payload, &cloned); err != nil {
		return nil, err
	}
	return cloned, nil
}

var duplicateAccountDiscardedExtraKeys = map[string]struct{}{
	// A retry identity belongs to the operation that created one copy, not to later copies.
	duplicateAccountOperationIDExtraKey: {},
	// Local quota usage and derived window timestamps must start fresh.
	"quota_used":         {},
	"quota_daily_used":   {},
	"quota_weekly_used":  {},
	"quota_daily_start":  {},
	"quota_weekly_start": {},
	// Provider observations, capability probes, and transient scheduling state.
	"model_rate_limits":                      {},
	"session_window_utilization":             {},
	"passive_usage_7d_utilization":           {},
	"passive_usage_7d_reset":                 {},
	"passive_usage_7d_oi_utilization":        {},
	"passive_usage_7d_oi_reset":              {},
	"passive_usage_sampled_at":               {},
	"grok_usage_snapshot":                    {},
	"grok_billing_snapshot":                  {},
	"openai_compact_supported":               {},
	"openai_compact_checked_at":              {},
	"openai_compact_last_status":             {},
	"openai_compact_last_error":              {},
	"antigravity_credits_overages":           {},
	"antigravity_force_token_refresh":        {},
	"antigravity_force_token_refresh_at":     {},
	"antigravity_force_token_refresh_reason": {},
	"drive_storage_limit":                    {},
	"drive_storage_usage":                    {},
	"drive_tier_updated_at":                  {},
	"codex_primary_used_percent":             {},
	"codex_primary_reset_after_seconds":      {},
	"codex_primary_window_minutes":           {},
	"codex_secondary_used_percent":           {},
	"codex_secondary_reset_after_seconds":    {},
	"codex_secondary_window_minutes":         {},
	"codex_primary_over_secondary_percent":   {},
	"codex_usage_updated_at":                 {},
	"codex_5h_used_percent":                  {},
	"codex_5h_reset_after_seconds":           {},
	"codex_5h_window_minutes":                {},
	"codex_5h_reset_at":                      {},
	"codex_7d_used_percent":                  {},
	"codex_7d_reset_after_seconds":           {},
	"codex_7d_window_minutes":                {},
	"codex_7d_reset_at":                      {},
}

func duplicateAccountExtra(value map[string]any) (map[string]any, error) {
	cloned, err := cloneAccountJSONMap(value)
	if err != nil {
		return nil, err
	}
	for key := range duplicateAccountDiscardedExtraKeys {
		delete(cloned, key)
	}
	return cloned, nil
}

func canDuplicateAccountType(accountType string) bool {
	switch accountType {
	case AccountTypeAPIKey, AccountTypeBedrock, AccountTypeServiceAccount:
		return true
	default:
		return false
	}
}

func duplicateAccountOperationID(sourceID int64, actorScope, operationKey string) string {
	operationKey = strings.TrimSpace(operationKey)
	if operationKey == "" {
		return ""
	}
	actorScope = strings.TrimSpace(actorScope)
	if actorScope == "" {
		actorScope = "admin:0"
	}
	payload := "admin.accounts.duplicate\x00" + actorScope + "\x00" + strconv.FormatInt(sourceID, 10) + "\x00" + operationKey
	digest := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("%x", digest)
}

func (s *adminServiceImpl) findDuplicateByOperationID(ctx context.Context, operationID string) (*Account, error) {
	if operationID == "" {
		return nil, nil
	}
	accounts, err := s.accountRepo.FindByExtraField(ctx, duplicateAccountOperationIDExtraKey, operationID)
	if err != nil {
		return nil, fmt.Errorf("find duplicate account operation: %w", err)
	}
	if len(accounts) == 0 {
		return nil, nil
	}
	account := accounts[0]
	return &account, nil
}

// RecoverDuplicateAccount performs a read-only lookup for an already committed duplicate.
// It is used when the idempotency coordinator cannot confirm whether response persistence
// succeeded, and deliberately never repeats the create side effect.
func (s *adminServiceImpl) RecoverDuplicateAccount(ctx context.Context, id int64, actorScope, operationKey string) (*Account, error) {
	return s.findDuplicateByOperationID(ctx, duplicateAccountOperationID(id, actorScope, operationKey))
}

func cloneAccountValuePointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

// DuplicateAccount creates a paused account from source configuration without carrying first-class
// runtime state. Credentials and extra configuration are deep-copied so normalization of the new
// account cannot mutate the in-memory source. Linked credential shadows are excluded because they
// intentionally do not own credentials and must be created through CreateShadow.
func (s *adminServiceImpl) DuplicateAccount(ctx context.Context, id int64, actorScope, operationKey string) (*Account, error) {
	operationID := duplicateAccountOperationID(id, actorScope, operationKey)
	existing, err := s.RecoverDuplicateAccount(ctx, id, actorScope, operationKey)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	source, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if source.IsCredentialShadow() {
		return nil, infraerrors.BadRequest(
			"ACCOUNT_DUPLICATE_SHADOW_UNSUPPORTED",
			"linked credential shadow accounts cannot be duplicated; duplicate the parent account instead",
		)
	}
	if !canDuplicateAccountType(source.Type) {
		return nil, infraerrors.BadRequest(
			"ACCOUNT_DUPLICATE_CREDENTIAL_TYPE_UNSUPPORTED",
			"accounts with rotating or unsupported credential types cannot be duplicated",
		)
	}

	credentials, err := cloneAccountJSONMap(source.Credentials)
	if err != nil {
		return nil, fmt.Errorf("clone account credentials: %w", err)
	}
	extra, err := duplicateAccountExtra(source.Extra)
	if err != nil {
		return nil, fmt.Errorf("clone account extra configuration: %w", err)
	}
	if operationID != "" {
		if extra == nil {
			extra = make(map[string]any, 1)
		}
		extra[duplicateAccountOperationIDExtraKey] = operationID
	}

	var expiresAt *int64
	if source.ExpiresAt != nil {
		unix := source.ExpiresAt.Unix()
		expiresAt = &unix
	}
	proxyID := source.ProxyID
	if source.ProxyFallbackOriginID != nil {
		// Proxy fallback is transient runtime state; duplicate the configured origin.
		proxyID = source.ProxyFallbackOriginID
	}
	input := &CreateAccountInput{
		Name:           duplicateAccountName(source.Name),
		Notes:          cloneAccountValuePointer(source.Notes),
		Platform:       source.Platform,
		Type:           source.Type,
		Credentials:    credentials,
		Extra:          extra,
		ProxyID:        cloneAccountValuePointer(proxyID),
		Concurrency:    source.Concurrency,
		Priority:       source.Priority,
		RateMultiplier: cloneAccountValuePointer(source.RateMultiplier),
		ExpiresAt:      expiresAt,
	}
	accountExtra := input.Extra
	if err := NormalizeHeaderOverrideCredentials(input.Credentials); err != nil {
		return nil, err
	}
	duplicate, err := buildAccountForCreate(input, accountExtra)
	if err != nil {
		return nil, err
	}
	// A copied credential must be reviewed before it can share live traffic with its source.
	duplicate.Schedulable = false
	if err := s.accountRepo.Create(ctx, duplicate); err != nil {
		return nil, fmt.Errorf("create duplicate account: %w", err)
	}
	return duplicate, nil
}

func normalizeAccountConcurrency(platform, accountType string, concurrency int) int {
	if platform == PlatformGrok && accountType == AccountTypeOAuth {
		if concurrency <= 0 {
			return 1
		}
	}
	return concurrency
}

// Grok media eligibility helpers live in account_grok_media_eligibility.go.

// resolveCreateAccountPlatform 第三方 key 不再要求填平台（管理端添加渠道先选「第三方 key / 成品号」，
// key 只填地址 + Key，2026-09-25）：没带平台时按地址推导——认得出官方厂商就是该厂商，指向中转的按主协议归族
// （AccountModelFamily）。这个标签仍用于无模型接口（联网搜索 / 语音 / live）按平台归池。成品号的厂商决定授权流程，必须填。
func resolveCreateAccountPlatform(input *CreateAccountInput) error {
	input.Platform = strings.TrimSpace(input.Platform)
	if input.Platform != "" {
		return nil
	}
	if input.Type != AccountTypeAPIKey {
		return infraerrors.BadRequest("ACCOUNT_PLATFORM_REQUIRED", "platform is required for subscription accounts")
	}
	endpoints, err := NormalizeProtocolEndpoints(input.ProtocolEndpoints)
	if err != nil {
		return infraerrors.BadRequest("INVALID_PROTOCOL_ENDPOINTS", err.Error())
	}
	if len(endpoints) == 0 {
		return infraerrors.BadRequest("INVALID_PROTOCOL_ENDPOINTS", "a third-party key needs one upstream protocol endpoint")
	}
	input.Platform = AccountModelFamily(&Account{Type: AccountTypeAPIKey, ProtocolEndpoints: endpoints})
	return nil
}

func buildAccountForCreate(input *CreateAccountInput, accountExtra map[string]any) (*Account, error) {
	// Probe/session state is system-managed. New accounts always start with automatic refresh disabled.
	delete(accountExtra, UpstreamBillingProbeEnabledExtraKey)
	delete(accountExtra, UpstreamBillingRateSyncEnabledExtraKey)
	delete(accountExtra, UpstreamBillingProbeExtraKey)
	delete(accountExtra, OllamaCloudUsageSessionExtraKey)
	delete(accountExtra, OllamaCloudUsageSnapshotExtraKey)
	protocolEndpoints, err := NormalizeProtocolEndpoints(input.ProtocolEndpoints)
	if err != nil {
		return nil, infraerrors.BadRequest("INVALID_PROTOCOL_ENDPOINTS", err.Error())
	}

	account := &Account{
		Name:              input.Name,
		Notes:             normalizeAccountNotes(input.Notes),
		Platform:          input.Platform,
		Type:              input.Type,
		Credentials:       input.Credentials,
		Extra:             accountExtra,
		ProxyID:           input.ProxyID,
		Concurrency:       normalizeAccountConcurrency(input.Platform, input.Type, input.Concurrency),
		Priority:          input.Priority,
		Status:            StatusActive,
		Schedulable:       true,
		ProtocolEndpoints: protocolEndpoints,
	}
	if input.ProbeEnabled != nil && *input.ProbeEnabled {
		if !isUpstreamBillingProbeAccount(account) {
			return nil, ErrUpstreamBillingProbeAccountInvalid
		}
		if account.Extra == nil {
			account.Extra = make(map[string]any)
		}
		account.Extra[UpstreamBillingProbeEnabledExtraKey] = true
	}
	if input.ExpiresAt != nil && *input.ExpiresAt > 0 {
		expiresAt := time.Unix(*input.ExpiresAt, 0)
		account.ExpiresAt = &expiresAt
	}
	if input.RateMultiplier != nil {
		if *input.RateMultiplier < 0 {
			return nil, errors.New("rate_multiplier must be >= 0")
		}
		account.RateMultiplier = input.RateMultiplier
	}
	return account, nil
}

func (s *adminServiceImpl) CreateAccount(ctx context.Context, input *CreateAccountInput) (*Account, error) {
	if err := resolveCreateAccountPlatform(input); err != nil {
		return nil, err
	}
	accountExtra, err := normalizeOpenAIAutoResetCreditExtra(input.Platform, input.Type, false, input.Extra)
	if err != nil {
		return nil, err
	}

	// 校验并规范化请求头覆写配置（header 名小写化、格式检查）
	if err := NormalizeHeaderOverrideCredentials(input.Credentials); err != nil {
		return nil, err
	}
	// Never persist ephemeral SSO/password secrets after OAuth conversion.
	input.Credentials = SanitizeStoredCredentials(input.Platform, input.Credentials)

	account, err := buildAccountForCreate(input, accountExtra)
	if err != nil {
		return nil, err
	}
	if err := s.accountRepo.Create(ctx, account); err != nil {
		return nil, err
	}

	// OAuth 账号：创建后异步设置隐私。
	// 使用 Ensure（幂等）而非 Force：新建账号 Extra 为空时效果相同，但更安全。
	if account.Type == AccountTypeOAuth {
		switch account.Platform {
		case PlatformOpenAI:
			go func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("create_account_openai_privacy_panic", "account_id", account.ID, "recover", r)
					}
				}()
				s.EnsureOpenAIPrivacy(context.Background(), account)
			}()
		case PlatformAntigravity:
			go func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("create_account_antigravity_privacy_panic", "account_id", account.ID, "recover", r)
					}
				}()
				s.EnsureAntigravityPrivacy(context.Background(), account)
			}()
		}
	}

	return account, nil
}

func (s *adminServiceImpl) UpdateAccount(ctx context.Context, id int64, input *UpdateAccountInput) (*Account, error) {
	account, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	var normalizedExtra map[string]any
	if input.Extra != nil {
		effectiveType := account.Type
		if input.Type != "" {
			effectiveType = input.Type
		}
		normalizedExtra, err = normalizeOpenAIAutoResetCreditExtra(account.Platform, effectiveType, account.IsShadow(), input.Extra)
		if err != nil {
			return nil, err
		}
	}
	previousProbeIdentity := upstreamBillingProbeIdentity(account)
	previousOllamaUsageIdentity := ollamaCloudUsageIdentity(account)
	// 安全/身份不变量(影子账号):通用更新路径被 edit/re-auth/refresh/batch 共用,
	// 必须在此守住,否则仅在创建时的保证可被这些路径绕过。
	if account.IsCredentialShadow() {
		// 影子绝不持有凭据(凭据只在母账号)——外审 F5。
		if !isAllowedSparkShadowCredentialsUpdate(input.Credentials) {
			return nil, infraerrors.Newf(http.StatusBadRequest, "SPARK_SHADOW_NO_CREDENTIALS",
				"spark shadow accounts do not hold auth credentials; only model mapping can be configured on the shadow account")
		}
		// 影子 type 不可变——很多上游逻辑按 account.Type 分支(OAuth transform / ChatGPT
		// header 注入 / WS OAuth 决策),改成 apikey 会让 spark 影子被选中后按错误协议转发(外审 G7)。
		if input.Type != "" && input.Type != account.Type {
			return nil, infraerrors.Newf(http.StatusBadRequest, "SPARK_SHADOW_IMMUTABLE_TYPE",
				"spark shadow account type cannot be changed; it must remain an OpenAI OAuth shadow")
		}
	} else if input.Type != "" && input.Type != account.Type && input.Type != AccountTypeOAuth {
		// 母账号守卫(外审 D/P1):有 spark 影子的账号不能把 type 改出 OpenAI OAuth——影子读透母
		// 凭据,母变成 apikey/setup_token 会让影子被调度后按错协议失败(resolveCredentialAccount
		// 必报错)。须先删影子再改 type。
		shadows, serr := s.accountRepo.ListShadowsByParent(ctx, id)
		if serr != nil {
			return nil, serr
		}
		if len(shadows) > 0 {
			return nil, infraerrors.New(http.StatusBadRequest, "SPARK_SHADOW_PARENT_IMMUTABLE_TYPE",
				"cannot change account type while it has a spark shadow; delete the shadow first")
		}
	}
	wasOveragesEnabled := account.IsOveragesEnabled()

	if input.ProtocolEndpoints != nil {
		normalizedEndpoints, perr := NormalizeProtocolEndpoints(*input.ProtocolEndpoints)
		if perr != nil {
			return nil, infraerrors.BadRequest("INVALID_PROTOCOL_ENDPOINTS", perr.Error())
		}
		account.ProtocolEndpoints = normalizedEndpoints
	}
	if input.Name != "" {
		account.Name = input.Name
	}
	if input.Type != "" {
		account.Type = input.Type
	}
	if input.Notes != nil {
		account.Notes = normalizeAccountNotes(input.Notes)
	}
	if account.IsCredentialShadow() && input.Credentials != nil {
		account.Credentials = sanitizeSparkShadowCredentials(input.Credentials)
	} else if len(input.Credentials) > 0 {
		// 敏感子键采用"incoming 没提供就保留"的合并语义：前端响应已脱敏，
		// 全对象 PUT 编辑时不会再带回 token，避免覆盖时清空已有凭证。
		account.Credentials = MergePreservingSensitiveCreds(account.Credentials, input.Credentials)
		// 校验并规范化请求头覆写配置（header 名小写化、格式检查）
		if err := NormalizeHeaderOverrideCredentials(account.Credentials); err != nil {
			return nil, err
		}
		// Strip SSO/password residue that must never sit next to OAuth tokens.
		account.Credentials = SanitizeStoredCredentials(account.Platform, account.Credentials)
	}
	// Extra 使用 map：需要区分“未提供(nil)”与“显式清空({})”。
	// 关闭配额限制时前端会删除 quota_* 键并提交 extra:{}，此时也必须落库。
	requestedProbeEnabledUpdate := input.ProbeEnabled
	requestedRateSyncEnabledUpdate := input.RateSyncEnabled
	if input.Extra != nil {
		requestedProbeEnabled, hasRequestedProbeEnabled := normalizedExtra[UpstreamBillingProbeEnabledExtraKey]
		if hasRequestedProbeEnabled {
			enabled, ok := requestedProbeEnabled.(bool)
			if !ok {
				return nil, infraerrors.BadRequest("INVALID_UPSTREAM_BILLING_PROBE_ENABLED", "upstream_billing_probe_enabled must be a boolean")
			}
			if requestedProbeEnabledUpdate != nil && *requestedProbeEnabledUpdate != enabled {
				return nil, infraerrors.BadRequest("CONFLICTING_UPSTREAM_BILLING_PROBE_ENABLED", "conflicting upstream_billing_probe_enabled values")
			}
			requestedProbeEnabledUpdate = &enabled
		}
		delete(normalizedExtra, UpstreamBillingProbeEnabledExtraKey)
		delete(normalizedExtra, UpstreamBillingRateSyncEnabledExtraKey)
		delete(normalizedExtra, UpstreamBillingProbeExtraKey)
		delete(normalizedExtra, OllamaCloudUsageSessionExtraKey)
		delete(normalizedExtra, OllamaCloudUsageSnapshotExtraKey)
		// 保留配额用量和专用服务受管字段，防止普通账号编辑意外覆盖。
		for _, key := range []string{
			"quota_used",
			"quota_daily_used",
			"quota_daily_start",
			"quota_weekly_used",
			"quota_weekly_start",
			grokBillingExtraKey,
			UpstreamBillingProbeEnabledExtraKey,
			UpstreamBillingRateSyncEnabledExtraKey,
			UpstreamBillingProbeExtraKey,
			OllamaCloudUsageSessionExtraKey,
			OllamaCloudUsageSnapshotExtraKey,
			OpenAIAutoResetCreditStateExtraKey,
		} {
			if v, ok := account.Extra[key]; ok {
				normalizedExtra[key] = v
			}
		}
		account.Extra = normalizedExtra
		if account.IsAntigravity() && wasOveragesEnabled && !account.IsOveragesEnabled() {
			delete(account.Extra, "antigravity_credits_overages") // 清理旧版 overages 运行态
			// 清除 AICredits 限流 key
			if rawLimits, ok := account.Extra[modelRateLimitsKey].(map[string]any); ok {
				delete(rawLimits, creditsExhaustedKey)
			}
		}
		if account.IsAntigravity() && !wasOveragesEnabled && account.IsOveragesEnabled() {
			delete(account.Extra, modelRateLimitsKey)
			delete(account.Extra, "antigravity_credits_overages") // 清理旧版 overages 运行态
		}
	}
	if requestedRateSyncEnabledUpdate != nil && *requestedRateSyncEnabledUpdate {
		if requestedProbeEnabledUpdate != nil && !*requestedProbeEnabledUpdate {
			return nil, infraerrors.BadRequest(
				"UPSTREAM_BILLING_RATE_SYNC_REQUIRES_PROBE",
				"upstream billing rate sync requires upstream billing probe",
			)
		}
		enabled := true
		requestedProbeEnabledUpdate = &enabled
	}
	if requestedProbeEnabledUpdate != nil && !*requestedProbeEnabledUpdate {
		disabled := false
		requestedRateSyncEnabledUpdate = &disabled
	}
	if (requestedProbeEnabledUpdate != nil && *requestedProbeEnabledUpdate) ||
		(requestedRateSyncEnabledUpdate != nil && *requestedRateSyncEnabledUpdate) {
		if !isUpstreamBillingProbeAccount(account) {
			return nil, ErrUpstreamBillingProbeAccountInvalid
		}
	}
	if account.Extra == nil && (requestedProbeEnabledUpdate != nil || requestedRateSyncEnabledUpdate != nil) {
		account.Extra = make(map[string]any)
	}
	if requestedProbeEnabledUpdate != nil {
		account.Extra[UpstreamBillingProbeEnabledExtraKey] = *requestedProbeEnabledUpdate
	}
	if requestedRateSyncEnabledUpdate != nil {
		account.Extra[UpstreamBillingRateSyncEnabledExtraKey] = *requestedRateSyncEnabledUpdate
	}
	// 影子代理恒继承母账号(由 propagateProxyToShadows 同步),不接受独立编辑——外审 B/P1;
	// 否则要等母账号下次改 proxy 才被覆盖,期间影子会出现"有时继承、有时独立"的漂移。
	if input.ProxyID != nil && !account.IsCredentialShadow() {
		// 0 表示清除代理（前端发送 0 而不是 null 来表达清除意图）
		if *input.ProxyID == 0 {
			account.ProxyID = nil
		} else {
			account.ProxyID = input.ProxyID
		}
		account.Proxy = nil // 清除关联对象，防止 GORM Save 时根据 Proxy.ID 覆盖 ProxyID
	}
	if !reflect.DeepEqual(previousProbeIdentity, upstreamBillingProbeIdentity(account)) && account.Extra != nil {
		delete(account.Extra, UpstreamBillingProbeExtraKey)
		if !isUpstreamBillingProbeAccount(account) {
			delete(account.Extra, UpstreamBillingProbeEnabledExtraKey)
			delete(account.Extra, UpstreamBillingRateSyncEnabledExtraKey)
		}
	}
	if account.Extra != nil {
		if !IsOllamaCloudUsageAccount(account) {
			delete(account.Extra, OllamaCloudUsageSessionExtraKey)
			delete(account.Extra, OllamaCloudUsageSnapshotExtraKey)
		} else if !reflect.DeepEqual(previousOllamaUsageIdentity, ollamaCloudUsageIdentity(account)) {
			delete(account.Extra, OllamaCloudUsageSessionExtraKey)
			delete(account.Extra, OllamaCloudUsageSnapshotExtraKey)
		}
	}
	// 只在指针非 nil 时更新 Concurrency（支持设置为 0）
	if input.Concurrency != nil {
		account.Concurrency = normalizeAccountConcurrency(account.Platform, account.Type, *input.Concurrency)
	}
	// 只在指针非 nil 时更新 Priority（支持设置为 0）
	if input.Priority != nil {
		account.Priority = *input.Priority
	}
	if input.RateMultiplier != nil {
		if *input.RateMultiplier < 0 {
			return nil, errors.New("rate_multiplier must be >= 0")
		}
		// 同步开启时倍率归上游所有，手工值活不过下一次成功探测（表现为"改了又自己
		// 变回去"），与批量路径一样直接拒绝。判断的是本次请求生效后的状态：上面
		// 已把请求携带的两个开关落进 account.Extra，所以"同一请求关闭同步 + 改倍率"
		// （用户显式收回所有权）会走到这里时读到 false，正常放行。
		if upstreamBillingRateSyncEnabled(account) {
			return nil, ErrUpstreamBillingRateSyncConflict
		}
		account.RateMultiplier = input.RateMultiplier
	}
	if input.Status != "" {
		account.Status = input.Status
	}
	if input.ExpiresAt != nil {
		if *input.ExpiresAt <= 0 {
			account.ExpiresAt = nil
		} else {
			expiresAt := time.Unix(*input.ExpiresAt, 0)
			account.ExpiresAt = &expiresAt
		}
	}
	billingSettingsAppliedAtomically := false
	updater := s.accountBillingRepo
	if updater == nil {
		// Unit tests and narrow internal callers may construct adminServiceImpl
		// directly; production wiring requires this capability through
		// AdminAccountRepository.
		updater, _ = s.accountRepo.(AccountBillingSettingsRepository)
	}
	if updater != nil {
		if err := updater.UpdateWithAccountBillingSettings(
			ctx,
			account,
			requestedProbeEnabledUpdate,
			requestedRateSyncEnabledUpdate,
			input.RateMultiplier,
		); err != nil {
			return nil, err
		}
		billingSettingsAppliedAtomically = true
	}
	if !billingSettingsAppliedAtomically {
		if err := s.accountRepo.Update(ctx, account); err != nil {
			return nil, err
		}
		if (requestedProbeEnabledUpdate != nil || requestedRateSyncEnabledUpdate != nil) &&
			isUpstreamBillingProbeAccount(account) {
			settings := make(map[string]any, 2)
			if requestedProbeEnabledUpdate != nil {
				settings[UpstreamBillingProbeEnabledExtraKey] = *requestedProbeEnabledUpdate
			}
			if requestedRateSyncEnabledUpdate != nil {
				settings[UpstreamBillingRateSyncEnabledExtraKey] = *requestedRateSyncEnabledUpdate
			}
			if err := s.accountRepo.UpdateExtra(ctx, account.ID, settings); err != nil {
				return nil, err
			}
		}
	}

	// 将 proxy 变更传播到 spark 影子账号（同步；Update 内部已触发调度快照）。
	// 影子自身 proxy 不可独立编辑(见上),故对影子的更新不触发传播。
	if input.ProxyID != nil && !account.IsCredentialShadow() {
		if err := s.propagateProxyToShadows(ctx, id, account.ProxyID); err != nil {
			return nil, err
		}
	}

	// 重新查询以确保返回完整数据（包括正确的 Proxy 关联对象）
	updated, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// UpdateAccountExtra 仅对 Extra JSONB 做 key 级合并，避免覆盖其它运行态键
// （如 model_rate_limits / passive_usage_* 等）。
func (s *adminServiceImpl) UpdateAccountExtra(ctx context.Context, id int64, updates map[string]any) error {
	updates = stripOpenAIAutoResetCreditManagedExtra(updates, true)
	delete(updates, UpstreamBillingProbeEnabledExtraKey)
	delete(updates, UpstreamBillingRateSyncEnabledExtraKey)
	delete(updates, UpstreamBillingProbeExtraKey)
	delete(updates, OllamaCloudUsageSessionExtraKey)
	delete(updates, OllamaCloudUsageSnapshotExtraKey)
	if len(updates) == 0 {
		return nil
	}
	return s.accountRepo.UpdateExtra(ctx, id, updates)
}

// BulkUpdateAccounts updates multiple accounts in one request.
// It merges credentials/extra keys instead of overwriting the whole object.
func (s *adminServiceImpl) BulkUpdateAccounts(ctx context.Context, input *BulkUpdateAccountsInput) (*BulkUpdateAccountsResult, error) {
	// Managed probe/session state may only enter through dedicated typed endpoints.
	input.Extra = stripOpenAIAutoResetCreditManagedExtra(input.Extra, true)
	delete(input.Extra, UpstreamBillingProbeEnabledExtraKey)
	delete(input.Extra, UpstreamBillingRateSyncEnabledExtraKey)
	delete(input.Extra, UpstreamBillingProbeExtraKey)
	delete(input.Extra, OllamaCloudUsageSessionExtraKey)
	delete(input.Extra, OllamaCloudUsageSnapshotExtraKey)

	if len(input.AccountIDs) == 0 && input.Filters != nil {
		accountIDs, err := s.resolveBulkUpdateTargetIDs(ctx, input.Filters)
		if err != nil {
			return nil, err
		}
		input.AccountIDs = accountIDs
	}

	result := &BulkUpdateAccountsResult{
		SuccessIDs: make([]int64, 0, len(input.AccountIDs)),
		FailedIDs:  make([]int64, 0, len(input.AccountIDs)),
		Results:    make([]BulkUpdateAccountResult, 0, len(input.AccountIDs)),
	}

	if len(input.AccountIDs) == 0 {
		return result, nil
	}

	// 预取所有目标账号，供凭据守卫/代理守卫/混合渠道检查共用，避免多次 DB 查询。
	var cachedTargets []*Account
	if len(input.Credentials) > 0 || input.ProxyID != nil || input.ProbeEnabled != nil || input.RateMultiplier != nil {
		loaded, err := s.accountRepo.GetByIDs(ctx, input.AccountIDs)
		if err != nil {
			return nil, err
		}
		cachedTargets = loaded
	}
	targetsByID := make(map[int64]*Account, len(cachedTargets))
	for _, account := range cachedTargets {
		if account != nil {
			targetsByID[account.ID] = account
		}
	}
	if input.ProbeEnabled != nil {
		for _, accountID := range input.AccountIDs {
			account, ok := targetsByID[accountID]
			if !ok {
				return nil, ErrAccountNotFound
			}
			if !isUpstreamBillingProbeAccount(account) {
				return nil, ErrUpstreamBillingProbeAccountInvalid
			}
		}
	}
	// 影子账号绝不持有凭据:批量更新携带凭据时,目标中不得含影子(外审 G5,与单账号
	// UpdateAccount 守卫对齐)。覆盖显式 IDs 与 filter 解析出的 IDs(此处 AccountIDs 已解析完成)。
	if len(input.Credentials) > 0 {
		for _, acc := range cachedTargets {
			if acc != nil && acc.IsCredentialShadow() {
				return nil, infraerrors.Newf(http.StatusBadRequest, "SPARK_SHADOW_NO_CREDENTIALS",
					"spark shadow account %d cannot hold credentials; manage credentials on the parent account", acc.ID)
			}
		}
	}

	// 影子账号 proxy 恒继承母账号(与单账号 UpdateAccount 守卫对齐——外审第4轮 P1):批量携带 proxy
	// 时目标不得含影子,否则影子会获得独立 proxy、破坏继承不变量(网关按所选影子自身 proxy 出站,
	// 要等母账号下次改 proxy 才覆盖→漂移)。含影子即整体拒绝,提示从选择中剔除影子。
	if input.ProxyID != nil {
		for _, acc := range cachedTargets {
			if acc != nil && acc.IsCredentialShadow() {
				return nil, infraerrors.Newf(http.StatusBadRequest, "SPARK_SHADOW_PROXY_INHERITED",
					"spark shadow account %d proxy is inherited from its parent and cannot be set in bulk; manage it on the parent account", acc.ID)
			}
		}
	}

	if input.RateMultiplier != nil {
		if *input.RateMultiplier < 0 {
			return nil, errors.New("rate_multiplier must be >= 0")
		}
		syncEnabledCount := 0
		for _, account := range cachedTargets {
			if account == nil || account.Extra == nil {
				continue
			}
			enabled, _ := account.Extra[UpstreamBillingRateSyncEnabledExtraKey].(bool)
			if enabled {
				syncEnabledCount++
			}
		}
		if syncEnabledCount > 0 {
			return nil, ErrUpstreamBillingRateSyncBulkConflict.WithMetadata(map[string]string{
				"count": strconv.Itoa(syncEnabledCount),
			})
		}
	}

	// 校验并规范化请求头覆写配置（批量路径为 JSONB 顶层 key 合并，直接校验增量即可）
	if err := NormalizeHeaderOverrideCredentials(input.Credentials); err != nil {
		return nil, err
	}
	// Bulk may mix platforms; always drop ephemeral SSO/password keys (cookie
	// only when platform is known Grok — empty platform still strips password/*).
	if input.Credentials != nil {
		input.Credentials = SanitizeStoredCredentials("", input.Credentials)
	}

	// Prepare bulk updates for columns and JSONB fields.
	repoUpdates := AccountBulkUpdate{
		Credentials:  input.Credentials,
		Extra:        input.Extra,
		ProbeEnabled: input.ProbeEnabled,
	}
	if input.ProbeEnabled != nil {
		if repoUpdates.Extra == nil {
			repoUpdates.Extra = make(map[string]any)
		}
		repoUpdates.Extra[UpstreamBillingProbeEnabledExtraKey] = *input.ProbeEnabled
		if !*input.ProbeEnabled {
			repoUpdates.Extra[UpstreamBillingRateSyncEnabledExtraKey] = false
		}
	}
	if updatesUpstreamBillingProbeIdentity(input.Credentials) || input.ProxyID != nil {
		if repoUpdates.Extra == nil {
			repoUpdates.Extra = make(map[string]any)
		}
		// JSON null makes every reader treat the old snapshot as absent and lets the
		// next enabled runner cycle probe the new upstream identity immediately.
		repoUpdates.Extra[UpstreamBillingProbeExtraKey] = nil
	}
	if input.Name != "" {
		repoUpdates.Name = &input.Name
	}
	if input.ProxyID != nil {
		repoUpdates.ProxyID = input.ProxyID
	}
	if input.Concurrency != nil {
		repoUpdates.Concurrency = input.Concurrency
	}
	if input.Priority != nil {
		repoUpdates.Priority = input.Priority
	}
	if input.RateMultiplier != nil {
		repoUpdates.RateMultiplier = input.RateMultiplier
	}
	if input.Status != "" {
		repoUpdates.Status = &input.Status
	}
	if input.Schedulable != nil {
		repoUpdates.Schedulable = input.Schedulable
	}

	// Run bulk update for column/jsonb fields first.
	if _, err := s.accountRepo.BulkUpdate(ctx, input.AccountIDs, repoUpdates); err != nil {
		return nil, err
	}

	// 将 proxy 变更传播到每个目标账号的 spark 影子账号
	if repoUpdates.ProxyID != nil {
		var effectiveProxyID *int64
		if *repoUpdates.ProxyID != 0 {
			effectiveProxyID = repoUpdates.ProxyID
		}
		for _, accountID := range input.AccountIDs {
			if err := s.propagateProxyToShadows(ctx, accountID, effectiveProxyID); err != nil {
				return nil, err
			}
		}
	}

	// 逐个账号汇总批量更新结果。
	for _, accountID := range input.AccountIDs {
		entry := BulkUpdateAccountResult{AccountID: accountID}

		entry.Success = true
		result.Success++
		result.SuccessIDs = append(result.SuccessIDs, accountID)
		result.Results = append(result.Results, entry)
	}

	return result, nil
}

func updatesUpstreamBillingProbeIdentity(credentials map[string]any) bool {
	for _, key := range []string{"api_key", credKeyHeaderOverrides} {
		if _, ok := credentials[key]; ok {
			return true
		}
	}
	return false
}

func upstreamBillingProbeIdentity(account *Account) map[string]any {
	if account == nil {
		return nil
	}
	identity := map[string]any{"platform": account.Platform, "type": account.Type, "proxy_id": nil}
	if account.ProxyID != nil {
		identity["proxy_id"] = *account.ProxyID
	}
	for _, key := range []string{"api_key", credKeyHeaderOverrides} {
		if value, ok := account.Credentials[key]; ok {
			identity[key] = value
		}
	}
	// 第三方 key 的上游坐标在协议映射里；只在非空时放入，避免 nil 与空 map 被 DeepEqual 判成不同。
	if len(account.ProtocolEndpoints) > 0 {
		identity["protocol_endpoints"] = account.ProtocolEndpoints
	}
	return identity
}

func (s *adminServiceImpl) resolveBulkUpdateTargetIDs(ctx context.Context, filters *BulkUpdateAccountFilters) ([]int64, error) {
	if filters == nil {
		return nil, nil
	}

	const pageSize = 500
	page := 1
	accountIDs := make([]int64, 0, pageSize)

	for {
		accounts, total, err := s.ListAccounts(
			ctx,
			page,
			pageSize,
			filters.Platform,
			filters.Type,
			filters.Status,
			filters.Search,
			filters.PrivacyMode,
			"",
			"",
		)
		if err != nil {
			return nil, err
		}
		for _, account := range accounts {
			accountIDs = append(accountIDs, account.ID)
		}
		if int64(len(accountIDs)) >= total || len(accounts) == 0 {
			return accountIDs, nil
		}
		page++
	}
}

func (s *adminServiceImpl) DeleteAccount(ctx context.Context, id int64) error {
	// 级联删除 spark 影子账号（先删影子，再删母账号）
	shadows, err := s.accountRepo.ListShadowsByParent(ctx, id)
	if err != nil {
		return fmt.Errorf("list spark shadows for cascade delete: %w", err)
	}
	for _, shadow := range shadows {
		if err := s.accountRepo.Delete(ctx, shadow.ID); err != nil {
			return fmt.Errorf("cascade delete spark shadow %d: %w", shadow.ID, err)
		}
	}
	if err := s.accountRepo.Delete(ctx, id); err != nil {
		return err
	}
	return nil
}

func (s *adminServiceImpl) RefreshAccountCredentials(ctx context.Context, id int64) (*Account, error) {
	account, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// TODO: Implement refresh logic
	return account, nil
}

func (s *adminServiceImpl) ClearAccountError(ctx context.Context, id int64) (*Account, error) {
	if err := s.accountRepo.ClearError(ctx, id); err != nil {
		return nil, err
	}
	if err := s.accountRepo.ClearRateLimit(ctx, id); err != nil {
		return nil, err
	}
	if err := s.accountRepo.ClearAntigravityQuotaScopes(ctx, id); err != nil {
		return nil, err
	}
	if err := s.accountRepo.ClearModelRateLimits(ctx, id); err != nil {
		return nil, err
	}
	if err := s.accountRepo.ClearTempUnschedulable(ctx, id); err != nil {
		return nil, err
	}
	if s.runtimeBlocker != nil {
		s.runtimeBlocker.ClearAccountSchedulingBlock(id)
	}
	return s.accountRepo.GetByID(ctx, id)
}

func (s *adminServiceImpl) SetAccountError(ctx context.Context, id int64, errorMsg string) error {
	return s.accountRepo.SetError(ctx, id, errorMsg)
}

func (s *adminServiceImpl) SetAccountSchedulable(ctx context.Context, id int64, schedulable bool) (*Account, error) {
	if err := s.accountRepo.SetSchedulable(ctx, id, schedulable); err != nil {
		return nil, err
	}
	updated, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *adminServiceImpl) RevertAccountProxyFallback(ctx context.Context, id int64) error {
	if err := s.accountRepo.RevertProxyFallback(ctx, id); err != nil {
		return err
	}
	// 加载回退后的账号以获取实际 ProxyID，再传播到影子账号
	account, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get account after proxy revert: %w", err)
	}
	return s.propagateProxyToShadows(ctx, id, account.ProxyID)
}

// CreateShadow 为指定 OpenAI OAuth 母账号创建 spark 维度影子账号（一母一影）。
// 安全不变量：Credentials 恒不含 auth token（仅 model_mapping，守卫 isAllowedSparkShadowCredentialsUpdate 放行）。
func (s *adminServiceImpl) CreateShadow(ctx context.Context, parentID int64, opts ShadowOptions) (*Account, error) {
	// 1. 加载母账号并校验平台/类型
	parent, err := s.accountRepo.GetByID(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("get parent account: %w", err)
	}
	if !parent.IsOpenAIOAuth() {
		return nil, infraerrors.New(http.StatusBadRequest, "SPARK_SHADOW_INVALID_PARENT",
			"spark shadow requires an OpenAI OAuth parent account")
	}
	// G6:母账号本身不能是影子,否则会建出二级影子——resolveCredentialAccount 只解一层,
	// 会解析到无凭据的一级影子,进入坏调度/上游失败。
	if parent.IsCredentialShadow() {
		return nil, infraerrors.New(http.StatusBadRequest, "SPARK_SHADOW_PARENT_IS_SHADOW",
			"spark shadow parent must be a real account, not another spark shadow")
	}

	// 2. 一母一影校验
	shadows, err := s.accountRepo.ListShadowsByParent(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("check existing spark shadows: %w", err)
	}
	if len(shadows) > 0 {
		return nil, infraerrors.New(http.StatusConflict, "SPARK_SHADOW_ALREADY_EXISTS",
			"parent account already has a spark shadow account")
	}

	// 4. 构造影子账号（安全不变量：Credentials 恒不含 auth token，仅含 model_mapping）。
	// name 为空时默认 "<母账号名> (Spark)"——否则空 name 会在 ent(name NotEmpty)处变成裸 500
	// (外审 E/P2);并 rune 安全截断到 ent MaxLen(100)。
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		name = parent.Name + " (Spark)"
	}
	if runes := []rune(name); len(runes) > 100 {
		name = string(runes[:100])
	}
	// 并发未指定(<=0)时继承母账号，避免 0 被限流器解读为"无限并发"（外审 F3）。
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = parent.Concurrency
	}
	// 优先级未指定(<=0)时继承母账号——前端一键创建只传 name,opts.Priority 省略即 0,而调度
	// 比较是「数值越小越优先」(openai_account_scheduler.isOpenAIAccountCandidateBetter),且 repo
	// 显式 SetPriority 会绕过 ent 默认 50,直写 0 会让影子意外抢到最高优先级(外审第5轮 P1)。
	// 与上方 Concurrency 一致采用「省略继承母账号」语义(影子的 proxy/分组/并发亦全部继承母账号)。
	priority := opts.Priority
	if priority <= 0 {
		priority = parent.Priority
	}
	shadow := &Account{
		Name:            name,
		Platform:        PlatformOpenAI,
		Type:            AccountTypeOAuth,
		Status:          StatusActive,
		Credentials:     map[string]any{"model_mapping": defaultSparkShadowModelMapping()},
		ParentAccountID: &parentID,
		QuotaDimension:  QuotaDimensionSpark,
		ProxyID:         parent.ProxyID,
		Priority:        priority,
		Concurrency:     concurrency,
		Schedulable:     true,
	}

	// 5. 持久化（Create 填充 shadow.ID）。并发竞态:预查(步骤2)放行后另一请求抢先建成,本次会撞
	// 一母一影唯一索引。复查确认确为"已存在"竞态时返回结构化 409 而非裸 500——外审 A/P1。
	if err := s.accountRepo.Create(ctx, shadow); err != nil {
		if existing, qerr := s.accountRepo.ListShadowsByParent(ctx, parentID); qerr == nil && len(existing) > 0 {
			return nil, infraerrors.New(http.StatusConflict, "SPARK_SHADOW_ALREADY_EXISTS",
				"parent account already has a spark shadow account")
		}
		return nil, fmt.Errorf("create spark shadow: %w", err)
	}

	return shadow, nil
}

// propagateProxyToShadows syncs proxyID to all spark shadow accounts of parentID.
// It is called synchronously so that proxy changes are immediately consistent;
// accountRepo.Update triggers the scheduler outbox + cache propagation internally.
// Calling this for a non-parent account is a harmless no-op.
func (s *adminServiceImpl) propagateProxyToShadows(ctx context.Context, parentID int64, proxyID *int64) error {
	return propagateAccountProxyToShadows(ctx, s.accountRepo, parentID, proxyID)
}

// propagateAccountProxyToShadows 把母账号的 proxy 同步到其所有 spark 影子(影子 proxy 恒继承母账号)。
// 母账号 proxy 改动后必须传播,否则影子保留旧 proxy 出现出站漂移(外审第8轮)。
func propagateAccountProxyToShadows(ctx context.Context, repo AccountRepository, parentID int64, proxyID *int64) error {
	shadows, err := repo.ListShadowsByParent(ctx, parentID)
	if err != nil {
		return fmt.Errorf("list spark shadows for proxy propagation: %w", err)
	}
	for _, shadow := range shadows {
		shadow.ProxyID = proxyID
		if err := repo.Update(ctx, shadow); err != nil {
			return fmt.Errorf("update spark shadow %d proxy: %w", shadow.ID, err)
		}
	}
	return nil
}

func (s *adminServiceImpl) ResetAccountQuota(ctx context.Context, id int64) error {
	account, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	// spark 影子账号不持自有配额(凭据透传母账号、spark 用量走独立 codex_* 维度由 QueryUsage 维护),
	// 通用 quota 重置对其无意义且语义不一致——明确 400 拒绝(与 OpenAI reset-credit 对影子一致)(外审第7轮 P2)。
	if account.IsCredentialShadow() {
		return infraerrors.New(http.StatusBadRequest, "SPARK_SHADOW_NO_QUOTA_RESET",
			"cannot reset quota for a spark shadow account; manage it on the parent account")
	}
	if err := s.accountRepo.ResetQuotaUsedAndClearRateLimitCooldown(ctx, id); err != nil {
		return err
	}
	// 配额计数超限是状态服务写的 temp_unschedulable（总额度停到管理员重置）：计数清零后一并解除；
	// 别的原因写的停调不动。
	if payload, ok := parseTempUnschedReasonPayload(account.TempUnschedulableReason); ok && payload.Source == quotaCounterSource {
		if err := s.accountRepo.ClearTempUnschedulable(ctx, id); err != nil {
			return err
		}
		if s.runtimeBlocker != nil {
			s.runtimeBlocker.ClearAccountSchedulingBlock(id)
		}
	}
	return nil
}

// EnsureOpenAIPrivacy 检查 OpenAI OAuth 账号是否已设置 privacy_mode，
// 未设置则调用 disableOpenAITraining 并持久化到 Extra，返回设置的 mode 值。
func (s *adminServiceImpl) EnsureOpenAIPrivacy(ctx context.Context, account *Account) string {
	// 影子账号不持凭据，隐私设置由母账号管理，直接跳过。
	if account.IsCredentialShadow() {
		return ""
	}
	if account.Platform != PlatformOpenAI || account.Type != AccountTypeOAuth {
		return ""
	}
	if s.privacyClientFactory == nil {
		return ""
	}
	if shouldSkipOpenAIPrivacyEnsure(account.Extra) {
		return ""
	}

	token, _ := account.Credentials["access_token"].(string)
	if token == "" {
		return ""
	}

	var proxyURL string
	if account.ProxyID != nil {
		if p, err := s.proxyRepo.GetByID(ctx, *account.ProxyID); err == nil && p != nil {
			proxyURL = p.URL()
		}
	}

	mode := disableOpenAITraining(ctx, s.privacyClientFactory, token, proxyURL)
	if mode == "" {
		return ""
	}

	_ = s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{"privacy_mode": mode})
	return mode
}

// ForceOpenAIPrivacy 强制重新设置 OpenAI OAuth 账号隐私，无论当前状态。
func (s *adminServiceImpl) ForceOpenAIPrivacy(ctx context.Context, account *Account) string {
	// 影子账号不持凭据,隐私由母账号管理,直接跳过(与 EnsureOpenAIPrivacy 一致——外审第4轮)。
	if account.IsCredentialShadow() {
		return ""
	}
	if account.Platform != PlatformOpenAI || account.Type != AccountTypeOAuth {
		return ""
	}
	if s.privacyClientFactory == nil {
		return ""
	}

	token, _ := account.Credentials["access_token"].(string)
	if token == "" {
		return ""
	}

	var proxyURL string
	if account.ProxyID != nil {
		if p, err := s.proxyRepo.GetByID(ctx, *account.ProxyID); err == nil && p != nil {
			proxyURL = p.URL()
		}
	}

	mode := disableOpenAITraining(ctx, s.privacyClientFactory, token, proxyURL)
	if mode == "" {
		return ""
	}

	if err := s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{"privacy_mode": mode}); err != nil {
		logger.LegacyPrintf("service.admin", "force_update_openai_privacy_mode_failed: account_id=%d err=%v", account.ID, err)
		return mode
	}
	if account.Extra == nil {
		account.Extra = make(map[string]any)
	}
	account.Extra["privacy_mode"] = mode
	return mode
}

// EnsureAntigravityPrivacy 检查 Antigravity OAuth 账号隐私状态。
// 仅当 privacy_mode 已成功设置（"privacy_set"）时跳过；
// 未设置或之前失败（"privacy_set_failed"）均会重试。
func (s *adminServiceImpl) EnsureAntigravityPrivacy(ctx context.Context, account *Account) string {
	if !account.IsAntigravity() || account.Type != AccountTypeOAuth {
		return ""
	}
	if account.Extra != nil {
		if existing, ok := account.Extra["privacy_mode"].(string); ok && existing == AntigravityPrivacySet {
			return existing
		}
	}

	token, _ := account.Credentials["access_token"].(string)
	if token == "" {
		return ""
	}

	projectID, _ := account.Credentials["project_id"].(string)

	var proxyURL string
	if account.ProxyID != nil {
		if p, err := s.proxyRepo.GetByID(ctx, *account.ProxyID); err == nil && p != nil {
			proxyURL = p.URL()
		}
	}

	mode := setAntigravityPrivacy(ctx, token, projectID, proxyURL)
	if mode == "" {
		return ""
	}

	if err := s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{"privacy_mode": mode}); err != nil {
		logger.LegacyPrintf("service.admin", "update_antigravity_privacy_mode_failed: account_id=%d err=%v", account.ID, err)
		return mode
	}
	applyAntigravityPrivacyMode(account, mode)
	return mode
}

// ForceAntigravityPrivacy 强制重新设置 Antigravity OAuth 账号隐私，无论当前状态。
func (s *adminServiceImpl) ForceAntigravityPrivacy(ctx context.Context, account *Account) string {
	if !account.IsAntigravity() || account.Type != AccountTypeOAuth {
		return ""
	}

	token, _ := account.Credentials["access_token"].(string)
	if token == "" {
		return ""
	}

	projectID, _ := account.Credentials["project_id"].(string)

	var proxyURL string
	if account.ProxyID != nil {
		if p, err := s.proxyRepo.GetByID(ctx, *account.ProxyID); err == nil && p != nil {
			proxyURL = p.URL()
		}
	}

	mode := setAntigravityPrivacy(ctx, token, projectID, proxyURL)
	if mode == "" {
		return ""
	}

	if err := s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{"privacy_mode": mode}); err != nil {
		logger.LegacyPrintf("service.admin", "force_update_antigravity_privacy_mode_failed: account_id=%d err=%v", account.ID, err)
		return mode
	}
	applyAntigravityPrivacyMode(account, mode)
	return mode
}
