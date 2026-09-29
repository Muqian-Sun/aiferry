// Package dto provides data transfer objects for HTTP handlers.
package dto

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func UserFromServiceShallow(u *service.User) *User {
	if u == nil {
		return nil
	}
	return &User{
		ID:                         u.ID,
		Email:                      u.Email,
		Username:                   u.Username,
		Role:                       u.Role,
		Balance:                    u.Balance,
		FrozenBalance:              u.FrozenBalance,
		Concurrency:                u.Concurrency,
		Status:                     u.Status,
		LastActiveAt:               u.LastActiveAt,
		CreatedAt:                  u.CreatedAt,
		UpdatedAt:                  u.UpdatedAt,
		BalanceNotifyEnabled:       u.BalanceNotifyEnabled,
		BalanceNotifyThresholdType: u.BalanceNotifyThresholdType,
		BalanceNotifyThreshold:     u.BalanceNotifyThreshold,
		BalanceNotifyExtraEmails:   NotifyEmailEntriesFromService(u.BalanceNotifyExtraEmails),
		TotalRecharged:             u.TotalRecharged,
		RPMLimit:                   u.RPMLimit,
		RateMultiplier:             service.UserRateMultiplier(u),
		CustomRateMultiplier:       u.RateMultiplier,
		DeletedAt:                  u.DeletedAt,
	}
}

func UserFromService(u *service.User) *User {
	if u == nil {
		return nil
	}
	out := UserFromServiceShallow(u)
	if len(u.APIKeys) > 0 {
		out.APIKeys = make([]APIKey, 0, len(u.APIKeys))
		for i := range u.APIKeys {
			k := u.APIKeys[i]
			out.APIKeys = append(out.APIKeys, *APIKeyFromService(&k))
		}
	}
	if len(u.Subscriptions) > 0 {
		out.Subscriptions = make([]UserSubscription, 0, len(u.Subscriptions))
		for i := range u.Subscriptions {
			s := u.Subscriptions[i]
			out.Subscriptions = append(out.Subscriptions, *UserSubscriptionFromService(&s))
		}
	}
	return out
}

// UserFromServiceAdmin converts a service User to DTO for admin users.
// It includes notes - user-facing endpoints must not use this.
func UserFromServiceAdmin(u *service.User) *AdminUser {
	if u == nil {
		return nil
	}
	base := UserFromService(u)
	if base == nil {
		return nil
	}
	return &AdminUser{
		User:       *base,
		Notes:      u.Notes,
		LastUsedAt: u.LastUsedAt,
	}
}

func APIKeyFromService(k *service.APIKey) *APIKey {
	if k == nil {
		return nil
	}
	out := &APIKey{
		ID:                 k.ID,
		UserID:             k.UserID,
		Key:                k.Key,
		Name:               k.Name,
		SubscriptionID:     k.SubscriptionID,
		Status:             k.Status,
		IPWhitelist:        k.IPWhitelist,
		IPBlacklist:        k.IPBlacklist,
		LastUsedAt:         k.LastUsedAt,
		LastUsedIP:         k.LastUsedIP,
		Quota:              k.Quota,
		QuotaUsed:          k.QuotaUsed,
		ExpiresAt:          k.ExpiresAt,
		CreatedAt:          k.CreatedAt,
		UpdatedAt:          k.UpdatedAt,
		CurrentConcurrency: k.CurrentConcurrency,
		RateLimit5h:        k.RateLimit5h,
		RateLimit1d:        k.RateLimit1d,
		RateLimit7d:        k.RateLimit7d,
		Usage5h:            k.EffectiveUsage5h(),
		Usage1d:            k.EffectiveUsage1d(),
		Usage7d:            k.EffectiveUsage7d(),
		Window5hStart:      k.Window5hStart,
		Window1dStart:      k.Window1dStart,
		Window7dStart:      k.Window7dStart,
		User:               UserFromServiceShallow(k.User),
	}
	if k.Subscription != nil && k.Subscription.Plan != nil {
		out.SubscriptionPlanName = k.Subscription.Plan.Name
	}
	if k.Window5hStart != nil && !service.IsWindowExpired(k.Window5hStart, service.RateLimitWindow5h) {
		t := k.Window5hStart.Add(service.RateLimitWindow5h)
		out.Reset5hAt = &t
	}
	if k.Window1dStart != nil && !service.IsWindowExpired(k.Window1dStart, service.RateLimitWindow1d) {
		t := k.Window1dStart.Add(service.RateLimitWindow1d)
		out.Reset1dAt = &t
	}
	if k.Window7dStart != nil && !service.IsWindowExpired(k.Window7dStart, service.RateLimitWindow7d) {
		t := k.Window7dStart.Add(service.RateLimitWindow7d)
		out.Reset7dAt = &t
	}
	return out
}

func AccountFromServiceShallow(a *service.Account) *Account {
	if a == nil {
		return nil
	}
	redactedCreds, credsStatus := RedactCredentials(a.Credentials)
	extra := redactAccountManagedExtra(a.Extra)
	var ollamaCloudUsage *service.OllamaCloudUsageState
	if state := service.OllamaCloudUsageStateFromAccount(a); state.Eligible {
		ollamaCloudUsage = state
	}
	out := &Account{
		ID:                      a.ID,
		Name:                    a.Name,
		Notes:                   a.Notes,
		Platform:                a.Platform,
		Type:                    a.Type,
		Credentials:             redactedCreds,
		CredentialsStatus:       credsStatus,
		Extra:                   extra,
		OllamaCloudUsage:        ollamaCloudUsage,
		ProxyID:                 a.ProxyID,
		ProxyFallbackOriginID:   a.ProxyFallbackOriginID,
		ProxyFallbackOriginName: a.ProxyFallbackOriginName,
		Concurrency:             a.Concurrency,
		Priority:                a.Priority,
		RateMultiplier:          a.BillingRateMultiplier(),
		Status:                  a.Status,
		ErrorMessage:            a.ErrorMessage,
		LastUsedAt:              a.LastUsedAt,
		ExpiresAt:               timeToUnixSeconds(a.ExpiresAt),
		CreatedAt:               a.CreatedAt,
		UpdatedAt:               a.UpdatedAt,
		Schedulable:             a.Schedulable,
		RateLimitedAt:           a.RateLimitedAt,
		RateLimitResetAt:        a.RateLimitResetAt,
		OverloadUntil:           a.OverloadUntil,
		TempUnschedulableUntil:  a.TempUnschedulableUntil,
		TempUnschedulableReason: a.TempUnschedulableReason,
		SessionWindowStart:      a.SessionWindowStart,
		SessionWindowEnd:        a.SessionWindowEnd,
		SessionWindowStatus:     a.SessionWindowStatus,
		ParentAccountID:         a.ParentAccountID,
		QuotaDimension:          a.QuotaDimension,
		ProtocolEndpoints:       a.ProtocolEndpoints,
		Vendor:                  a.Vendor(),
	}

	// 提取会话数量与 RPM 限制配置（仅 Anthropic OAuth/SetupToken 账号有效）。
	// 空闲超时、RPM 策略、TLS 指纹、会话 ID 伪装等已写死在代码里（channel_features_anthropic.go），不回显；
	// 粘性缓冲只回显按并发 / 会话数自动算出的值，供容量展示用（渠道级手填已删）。
	if a.IsAnthropicOAuthOrSetupToken() {
		if maxSessions := a.GetMaxSessions(); maxSessions > 0 {
			out.MaxSessions = &maxSessions
		}
		if rpm := a.GetBaseRPM(); rpm > 0 {
			out.BaseRPM = &rpm
			buffer := a.GetRPMStickyBuffer()
			out.RPMStickyBuffer = &buffer
		}
	}

	// 提取账号配额限制（apikey / bedrock 类型有效）
	if a.IsAPIKeyOrBedrock() {
		if limit := a.GetQuotaLimit(); limit > 0 {
			out.QuotaLimit = &limit
			used := a.GetQuotaUsed()
			out.QuotaUsed = &used
		}
		if limit := a.GetQuotaDailyLimit(); limit > 0 {
			out.QuotaDailyLimit = &limit
			used := a.GetQuotaDailyUsed()
			if a.IsDailyQuotaPeriodExpired() {
				used = 0
			}
			out.QuotaDailyUsed = &used
		}
		if limit := a.GetQuotaWeeklyLimit(); limit > 0 {
			out.QuotaWeeklyLimit = &limit
			used := a.GetQuotaWeeklyUsed()
			if a.IsWeeklyQuotaPeriodExpired() {
				used = 0
			}
			out.QuotaWeeklyUsed = &used
		}
	}

	return out
}

func redactAccountManagedExtra(extra map[string]any) map[string]any {
	if extra == nil {
		return nil
	}
	redacted := make(map[string]any, len(extra))
	for key, value := range extra {
		switch key {
		case service.OllamaCloudUsageSessionExtraKey,
			service.OllamaCloudUsageSnapshotExtraKey:
			continue
		default:
			redacted[key] = value
		}
	}
	return redacted
}

func AccountFromService(a *service.Account) *Account {
	if a == nil {
		return nil
	}
	out := AccountFromServiceShallow(a)
	out.Proxy = ProxyFromService(a.Proxy)
	return out
}

// AccountListItemFromAccount projects a full account response into the
// compact shape used by the paginated admin account list. Keeping this
// projection separate from Account preserves the existing detail API.
func AccountListItemFromAccount(a *Account) *AccountListItem {
	if a == nil {
		return nil
	}
	return &AccountListItem{
		ID: a.ID, Name: a.Name, Notes: a.Notes, Platform: a.Platform, Type: a.Type,
		Credentials: a.Credentials, CredentialsStatus: a.CredentialsStatus, Extra: a.Extra,
		OllamaCloudUsage: a.OllamaCloudUsage,
		ProxyID:          a.ProxyID, ProxyFallbackOriginID: a.ProxyFallbackOriginID, ProxyFallbackOriginName: a.ProxyFallbackOriginName,
		Concurrency: a.Concurrency, Priority: a.Priority, RateMultiplier: a.RateMultiplier,
		Status: a.Status, ErrorMessage: a.ErrorMessage, LastUsedAt: a.LastUsedAt, ExpiresAt: a.ExpiresAt,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
		Schedulable: a.Schedulable, RateLimitedAt: a.RateLimitedAt, RateLimitResetAt: a.RateLimitResetAt,
		OverloadUntil: a.OverloadUntil, TempUnschedulableUntil: a.TempUnschedulableUntil,
		TempUnschedulableReason: a.TempUnschedulableReason, SessionWindowStart: a.SessionWindowStart,
		SessionWindowEnd: a.SessionWindowEnd, SessionWindowStatus: a.SessionWindowStatus,
		MaxSessions: a.MaxSessions, BaseRPM: a.BaseRPM, RPMStickyBuffer: a.RPMStickyBuffer,
		QuotaLimit: a.QuotaLimit, QuotaUsed: a.QuotaUsed,
		QuotaDailyLimit: a.QuotaDailyLimit, QuotaDailyUsed: a.QuotaDailyUsed, QuotaWeeklyLimit: a.QuotaWeeklyLimit,
		QuotaWeeklyUsed: a.QuotaWeeklyUsed, ParentAccountID: a.ParentAccountID,
		QuotaDimension: a.QuotaDimension, ParentEmail: a.ParentEmail, ParentPlanType: a.ParentPlanType,
		ParentPrivacyMode: a.ParentPrivacyMode, ParentSubscriptionExpiresAt: a.ParentSubscriptionExpiresAt,
		ParentChatGPTAccountID: a.ParentChatGPTAccountID, Proxy: a.Proxy,
	}
}

func timeToUnixSeconds(value *time.Time) *int64 {
	if value == nil {
		return nil
	}
	ts := value.Unix()
	return &ts
}

func ProxyFromService(p *service.Proxy) *Proxy {
	if p == nil {
		return nil
	}
	return &Proxy{
		ID:             p.ID,
		Name:           p.Name,
		Protocol:       p.Protocol,
		Host:           p.Host,
		Port:           p.Port,
		Username:       p.Username,
		Status:         p.Status,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
		ExpiresAt:      p.ExpiresAt,
		FallbackMode:   p.FallbackMode,
		BackupProxyID:  p.BackupProxyID,
		ExpiryWarnDays: p.ExpiryWarnDays,
	}
}

func ProxyWithAccountCountFromService(p *service.ProxyWithAccountCount) *ProxyWithAccountCount {
	if p == nil {
		return nil
	}
	return &ProxyWithAccountCount{
		Proxy:          *ProxyFromService(&p.Proxy),
		AccountCount:   p.AccountCount,
		LatencyMs:      p.LatencyMs,
		LatencyStatus:  p.LatencyStatus,
		LatencyMessage: p.LatencyMessage,
		IPAddress:      p.IPAddress,
		Country:        p.Country,
		CountryCode:    p.CountryCode,
		Region:         p.Region,
		City:           p.City,
		QualityStatus:  p.QualityStatus,
		QualityScore:   p.QualityScore,
		QualityGrade:   p.QualityGrade,
		QualitySummary: p.QualitySummary,
		QualityChecked: p.QualityChecked,
	}
}

// ProxyFromServiceAdmin converts a service Proxy to AdminProxy DTO for admin users.
// It includes the password field - user-facing endpoints must not use this.
func ProxyFromServiceAdmin(p *service.Proxy) *AdminProxy {
	if p == nil {
		return nil
	}
	base := ProxyFromService(p)
	if base == nil {
		return nil
	}
	return &AdminProxy{
		Proxy:    *base,
		Password: p.Password,
	}
}

// ProxyWithAccountCountFromServiceAdmin converts a service ProxyWithAccountCount to AdminProxyWithAccountCount DTO.
// It includes the password field - user-facing endpoints must not use this.
func ProxyWithAccountCountFromServiceAdmin(p *service.ProxyWithAccountCount) *AdminProxyWithAccountCount {
	if p == nil {
		return nil
	}
	admin := ProxyFromServiceAdmin(&p.Proxy)
	if admin == nil {
		return nil
	}
	return &AdminProxyWithAccountCount{
		AdminProxy:     *admin,
		AccountCount:   p.AccountCount,
		LatencyMs:      p.LatencyMs,
		LatencyStatus:  p.LatencyStatus,
		LatencyMessage: p.LatencyMessage,
		IPAddress:      p.IPAddress,
		Country:        p.Country,
		CountryCode:    p.CountryCode,
		Region:         p.Region,
		City:           p.City,
		QualityStatus:  p.QualityStatus,
		QualityScore:   p.QualityScore,
		QualityGrade:   p.QualityGrade,
		QualitySummary: p.QualitySummary,
		QualityChecked: p.QualityChecked,
	}
}

func ProxyAccountSummaryFromService(a *service.ProxyAccountSummary) *ProxyAccountSummary {
	if a == nil {
		return nil
	}
	return &ProxyAccountSummary{
		ID:       a.ID,
		Name:     a.Name,
		Platform: a.Platform,
		Type:     a.Type,
		Notes:    a.Notes,
	}
}

func RedeemCodeFromService(rc *service.RedeemCode) *RedeemCode {
	if rc == nil {
		return nil
	}
	out := redeemCodeFromServiceBase(rc)
	return &out
}

// RedeemCodeFromServiceAdmin converts a service RedeemCode to DTO for admin users.
// It includes notes - user-facing endpoints must not use this.
func RedeemCodeFromServiceAdmin(rc *service.RedeemCode) *AdminRedeemCode {
	if rc == nil {
		return nil
	}
	return &AdminRedeemCode{
		RedeemCode: redeemCodeFromServiceBase(rc),
		Notes:      rc.Notes,
	}
}

func redeemCodeFromServiceBase(rc *service.RedeemCode) RedeemCode {
	out := RedeemCode{
		ID:           rc.ID,
		Code:         rc.Code,
		Type:         rc.Type,
		Value:        rc.Value,
		Status:       rc.Status,
		UsedBy:       rc.UsedBy,
		UsedAt:       rc.UsedAt,
		CreatedAt:    rc.CreatedAt,
		ExpiresAt:    rc.ExpiresAt,
		PlanID:       rc.PlanID,
		ValidityDays: rc.ValidityDays,
		User:         UserFromServiceShallow(rc.User),
		Plan:         SubscriptionPlanRefFromService(rc.Plan),
	}
	if rc.IsExpired() {
		out.Status = service.StatusExpired
	}

	// For admin_balance/admin_concurrency types, include notes so users can see
	// why they were charged or credited by admin
	if (rc.Type == "admin_balance" || rc.Type == "admin_concurrency") && rc.Notes != "" {
		out.Notes = &rc.Notes
	}

	return out
}

// AccountSummaryFromService returns a minimal AccountSummary for usage log display.
// Only includes ID and Name - no sensitive fields like Credentials, Proxy, etc.
func AccountSummaryFromService(a *service.Account) *AccountSummary {
	if a == nil {
		return nil
	}
	return &AccountSummary{
		ID:   a.ID,
		Name: a.Name,
	}
}

func usageLogFromServiceUser(l *service.UsageLog) UsageLog {
	// 普通用户 DTO：严禁包含管理员字段（例如 account_id、account、upstream_model、session_id）。
	requestType := l.EffectiveRequestType()
	stream, openAIWSMode := service.ApplyLegacyRequestFields(requestType, l.Stream, l.OpenAIWSMode)
	requestedModel := l.RequestedModel
	if requestedModel == "" {
		requestedModel = l.Model
	}
	return UsageLog{
		ID:                    l.ID,
		UserID:                l.UserID,
		APIKeyID:              l.APIKeyID,
		RequestID:             l.RequestID,
		Model:                 requestedModel,
		ServiceTier:           l.ServiceTier,
		ReasoningEffort:       userFacingReasoningEffort(l),
		InboundEndpoint:       l.InboundEndpoint,
		SubscriptionID:        l.SubscriptionID,
		InputTokens:           l.InputTokens,
		OutputTokens:          l.OutputTokens,
		CacheCreationTokens:   l.CacheCreationTokens,
		CacheReadTokens:       l.CacheReadTokens,
		CacheCreation5mTokens: l.CacheCreation5mTokens,
		CacheCreation1hTokens: l.CacheCreation1hTokens,
		InputCost:             l.InputCost,
		OutputCost:            l.OutputCost,
		CacheCreationCost:     l.CacheCreationCost,
		CacheReadCost:         l.CacheReadCost,
		TotalCost:             l.TotalCost,
		ActualCost:            l.ActualCost,
		RateMultiplier:        l.RateMultiplier,
		BillingType:           l.BillingType,
		RequestType:           requestType.String(),
		Stream:                stream,
		OpenAIWSMode:          openAIWSMode,
		NativeCompactionV2:    l.NativeCompactionV2,
		DurationMs:            l.DurationMs,
		FirstTokenMs:          l.FirstTokenMs,
		ImageCount:            l.ImageCount,
		ImageSize:             l.ImageSize,
		ImageInputSize:        l.ImageInputSize,
		ImageOutputSize:       l.ImageOutputSize,
		ImageInputTokens:      l.ImageInputTokens,
		ImageInputCost:        l.ImageInputCost,
		ImageOutputTokens:     l.ImageOutputTokens,
		ImageOutputCost:       l.ImageOutputCost,
		ImageSizeSource:       l.ImageSizeSource,
		ImageSizeBreakdown:    l.ImageSizeBreakdown,
		MediaType:             l.MediaType,
		UserAgent:             l.UserAgent,
		IPAddress:             l.IPAddress,
		CacheTTLOverridden:    l.CacheTTLOverridden,
		BillingMode:           l.BillingMode,
		CreatedAt:             l.CreatedAt,
		User:                  UserFromServiceShallow(l.User),
		APIKey:                APIKeyFromService(l.APIKey),
		Subscription:          UserSubscriptionFromService(l.Subscription),
	}
}

// UsageLogFromService converts a service UsageLog to DTO for regular users.
// It excludes admin-only account/upstream internals while keeping user billing and request metadata.
func UsageLogFromService(l *service.UsageLog) *UsageLog {
	if l == nil {
		return nil
	}
	u := usageLogFromServiceUser(l)
	return &u
}

// UsageLogFromServiceAdmin converts a service UsageLog to DTO for admin users.
// It includes minimal Account info (ID, Name only) and IP address.
func UsageLogFromServiceAdmin(l *service.UsageLog) *AdminUsageLog {
	if l == nil {
		return nil
	}
	return &AdminUsageLog{
		UsageLog:                usageLogFromServiceUser(l),
		AccountID:               l.AccountID,
		UpstreamEndpoint:        l.UpstreamEndpoint,
		SessionID:               l.SessionID,
		UpstreamModel:           l.UpstreamModel,
		UpstreamReasoningEffort: adminUpstreamReasoningEffort(l),
		UpstreamResponseModel:   l.UpstreamResponseModel,
		UpstreamModelMismatch:   l.UpstreamModelMismatch,
		UpstreamRequestID:       l.UpstreamRequestID,
		AccountRateMultiplier:   l.AccountRateMultiplier,
		IPAddress:               l.IPAddress,
		Account:                 AccountSummaryFromService(l.Account),
	}
}

func userFacingReasoningEffort(l *service.UsageLog) *string {
	if l == nil {
		return nil
	}
	if requested := strings.TrimSpace(derefString(l.RequestedReasoningEffort)); requested != "" {
		return &requested
	}
	return l.ReasoningEffort
}

func adminUpstreamReasoningEffort(l *service.UsageLog) *string {
	if l == nil {
		return nil
	}
	forwarded := strings.TrimSpace(derefString(l.ReasoningEffort))
	if forwarded == "" {
		return nil
	}
	requested := userFacingReasoningEffort(l)
	if requested != nil && service.NormalizeMaxReasoningEffort(*requested) == service.NormalizeMaxReasoningEffort(forwarded) {
		return nil
	}
	return &forwarded
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func UsageCleanupTaskFromService(task *service.UsageCleanupTask) *UsageCleanupTask {
	if task == nil {
		return nil
	}
	return &UsageCleanupTask{
		ID:     task.ID,
		Status: task.Status,
		Filters: UsageCleanupFilters{
			StartTime:   task.Filters.StartTime,
			EndTime:     task.Filters.EndTime,
			UserID:      task.Filters.UserID,
			APIKeyID:    task.Filters.APIKeyID,
			AccountID:   task.Filters.AccountID,
			Model:       task.Filters.Model,
			RequestType: requestTypeStringPtr(task.Filters.RequestType),
			Stream:      task.Filters.Stream,
			BillingType: task.Filters.BillingType,
		},
		CreatedBy:    task.CreatedBy,
		DeletedRows:  task.DeletedRows,
		ErrorMessage: task.ErrorMsg,
		CanceledBy:   task.CanceledBy,
		CanceledAt:   task.CanceledAt,
		StartedAt:    task.StartedAt,
		FinishedAt:   task.FinishedAt,
		CreatedAt:    task.CreatedAt,
		UpdatedAt:    task.UpdatedAt,
	}
}

func requestTypeStringPtr(requestType *int16) *string {
	if requestType == nil {
		return nil
	}
	value := service.RequestTypeFromInt16(*requestType).String()
	return &value
}

func SettingFromService(s *service.Setting) *Setting {
	if s == nil {
		return nil
	}
	return &Setting{
		ID:        s.ID,
		Key:       s.Key,
		Value:     s.Value,
		UpdatedAt: s.UpdatedAt,
	}
}

func UserSubscriptionFromService(sub *service.UserSubscription) *UserSubscription {
	if sub == nil {
		return nil
	}
	out := userSubscriptionFromServiceBase(sub)
	return &out
}

// UserSubscriptionFromServiceAdmin converts a service UserSubscription to DTO for admin users.
// It includes assignment metadata and notes.
func UserSubscriptionFromServiceAdmin(sub *service.UserSubscription) *AdminUserSubscription {
	if sub == nil {
		return nil
	}
	return &AdminUserSubscription{
		UserSubscription: userSubscriptionFromServiceBase(sub),
		AssignedBy:       sub.AssignedBy,
		AssignedAt:       sub.AssignedAt,
		Notes:            sub.Notes,
		AssignedByUser:   UserFromServiceShallow(sub.AssignedByUser),
	}
}

// SubscriptionPlanRefFromService 套餐引用；nil 安全
func SubscriptionPlanRefFromService(p *service.SubscriptionPlan) *SubscriptionPlanRef {
	if p == nil {
		return nil
	}
	return &SubscriptionPlanRef{ID: p.ID, Name: p.Name}
}

// SubscriptionPlanFromService 订阅上挂的套餐（限额 + 模型集）；nil 安全
func SubscriptionPlanFromService(p *service.SubscriptionPlan) *SubscriptionPlan {
	if p == nil {
		return nil
	}
	out := &SubscriptionPlan{
		SubscriptionPlanRef: SubscriptionPlanRef{ID: p.ID, Name: p.Name},
		DailyLimitUSD:       p.DailyLimitUSD,
		WeeklyLimitUSD:      p.WeeklyLimitUSD,
		MonthlyLimitUSD:     p.MonthlyLimitUSD,
		Models:              make([]SubscriptionPlanModel, 0, len(p.Models)),
	}
	for _, m := range p.Models {
		out.Models = append(out.Models, SubscriptionPlanModel{EntryID: m.EntryID, ModelID: m.ModelID, DisplayName: m.DisplayName})
	}
	return out
}

// MaskAPIKey 脱敏：前 6 后 4，中间 ****；短 key 全遮
func MaskAPIKey(plain string) string {
	if len(plain) <= 10 {
		return "****"
	}
	return plain[:6] + "****" + plain[len(plain)-4:]
}

// SubscriptionAPIKeyRefFromService 订阅 key 引用；nil 安全
func SubscriptionAPIKeyRefFromService(k *service.APIKey) *SubscriptionAPIKeyRef {
	if k == nil {
		return nil
	}
	return &SubscriptionAPIKeyRef{ID: k.ID, Name: k.Name, KeyMasked: MaskAPIKey(k.Key)}
}

func userSubscriptionFromServiceBase(sub *service.UserSubscription) UserSubscription {
	return UserSubscription{
		ID:                 sub.ID,
		UserID:             sub.UserID,
		PlanID:             sub.PlanID,
		StartsAt:           sub.StartsAt,
		ExpiresAt:          sub.ExpiresAt,
		Status:             sub.Status,
		DailyWindowStart:   sub.DailyWindowStart,
		WeeklyWindowStart:  sub.WeeklyWindowStart,
		MonthlyWindowStart: sub.MonthlyWindowStart,
		DailyUsageUSD:      sub.DailyUsageUSD,
		WeeklyUsageUSD:     sub.WeeklyUsageUSD,
		MonthlyUsageUSD:    sub.MonthlyUsageUSD,
		CreatedAt:          sub.CreatedAt,
		UpdatedAt:          sub.UpdatedAt,
		RevokedAt:          sub.DeletedAt,
		User:               UserFromServiceShallow(sub.User),
		Plan:               SubscriptionPlanFromService(sub.Plan),
		APIKey:             SubscriptionAPIKeyRefFromService(sub.APIKey),
	}
}

func BulkAssignResultFromService(r *service.BulkAssignResult) *BulkAssignResult {
	if r == nil {
		return nil
	}
	subs := make([]AdminUserSubscription, 0, len(r.Subscriptions))
	for i := range r.Subscriptions {
		subs = append(subs, *UserSubscriptionFromServiceAdmin(&r.Subscriptions[i]))
	}
	statuses := make(map[string]string, len(r.Statuses))
	for userID, status := range r.Statuses {
		statuses[strconv.FormatInt(userID, 10)] = status
	}
	return &BulkAssignResult{
		SuccessCount:  r.SuccessCount,
		CreatedCount:  r.CreatedCount,
		ReusedCount:   r.ReusedCount,
		FailedCount:   r.FailedCount,
		Subscriptions: subs,
		Errors:        r.Errors,
		Statuses:      statuses,
	}
}
