package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"
)

// IsRegistrationEnabled 是否开放注册：由代码决定（site_features.go），不再有后台开关。
func (s *SettingService) IsRegistrationEnabled(ctx context.Context) bool {
	return RegistrationOpen
}

// IsEmailVerifyEnabled 注册要不要验证邮箱：配了 SMTP 就要（能发信才能验证），不再有后台开关。
func (s *SettingService) IsEmailVerifyEnabled(ctx context.Context) bool {
	return s.smtpConfigured()
}

// smtpConfigured 部署时配了 SMTP（主机与发件人）。
func (s *SettingService) smtpConfigured() bool {
	return s != nil && s.cfg != nil && s.cfg.SMTP.Configured()
}

// IsRegistrationEmailDomainQuotaEnabled 白名单之外的域名限量注册：由代码决定（site_features.go）。
func (s *SettingService) IsRegistrationEmailDomainQuotaEnabled(ctx context.Context) bool {
	return RegistrationEmailDomainQuotaEnabled
}

// GetRegistrationEmailSuffixWhitelist 注册邮箱域名白名单：由代码决定（site_features.go）。
func (s *SettingService) GetRegistrationEmailSuffixWhitelist(ctx context.Context) []string {
	return RegistrationEmailSuffixWhitelist()
}

// IsInvitationCodeEnabled 注册是否要邀请码：由代码决定（site_features.go），不再有后台开关。
func (s *SettingService) IsInvitationCodeEnabled(ctx context.Context) bool {
	return InvitationCodeRequired
}

// GetCustomMenuItemsRaw 自定义菜单：前端不再展示（方案定：删），恒为空；读菜单的功能代码保留。
func (s *SettingService) GetCustomMenuItemsRaw(ctx context.Context) string {
	return "[]"
}

// IsAffiliateEnabled 检查是否启用邀请返利功能（总开关）
func (s *SettingService) IsAffiliateEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyAffiliateEnabled)
	if err != nil {
		return false // 默认关闭
	}
	return value == "true"
}

// IsAffiliateAdminRechargeEnabled reports whether admin balance
// deposits should participate in the affiliate rebate program.
func (s *SettingService) IsAffiliateAdminRechargeEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyAffiliateAdminRechargeEnabled)
	if err != nil {
		return AdminRechargeRebateEnabledDefault
	}
	return value == "true"
}

// GetAffiliateRebateRatePercent 读取并 clamp 全局返利比例。
// 解析失败、缺失或越界都回退到 AffiliateRebateRateDefault — 该比例从不抛错，
// 调用方只关心一个可用的数值。
func (s *SettingService) GetAffiliateRebateRatePercent(ctx context.Context) float64 {
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyAffiliateRebateRate)
	if err != nil {
		return AffiliateRebateRateDefault
	}
	rate, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return AffiliateRebateRateDefault
	}
	return clampAffiliateRebateRate(rate)
}

// GetAffiliateRebateFreezeHours 返回返利冻结期（小时）。
// 返回 0 表示不冻结（向后兼容）。
func (s *SettingService) GetAffiliateRebateFreezeHours(ctx context.Context) int {
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyAffiliateRebateFreezeHours)
	if err != nil {
		return AffiliateRebateFreezeHoursDefault
	}
	hours, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || hours < 0 {
		return AffiliateRebateFreezeHoursDefault
	}
	if hours > AffiliateRebateFreezeHoursMax {
		return AffiliateRebateFreezeHoursMax
	}
	return hours
}

// GetAffiliateRebateDurationDays 返回返利有效期（天）。
// 返回 0 表示永久有效。
func (s *SettingService) GetAffiliateRebateDurationDays(ctx context.Context) int {
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyAffiliateRebateDurationDays)
	if err != nil {
		return AffiliateRebateDurationDaysDefault
	}
	days, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || days < 0 {
		return AffiliateRebateDurationDaysDefault
	}
	if days > AffiliateRebateDurationDaysMax {
		return AffiliateRebateDurationDaysMax
	}
	return days
}

// GetAffiliateRebatePerInviteeCap 返回单人返利上限。
// 返回 0 表示无上限。
func (s *SettingService) GetAffiliateRebatePerInviteeCap(ctx context.Context) float64 {
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyAffiliateRebatePerInviteeCap)
	if err != nil {
		return AffiliateRebatePerInviteeCapDefault
	}
	cap, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || cap < 0 || math.IsNaN(cap) || math.IsInf(cap, 0) {
		return AffiliateRebatePerInviteeCapDefault
	}
	return cap
}

// IsPasswordResetEnabled 找回密码能不能用：要能发信（邮箱验证开着），还要配了站点访问地址——重置链接指向它，
// 没配时后端发不出链接。公开设置的 password_reset_enabled 也用它，前端没配好时整个不显示找回密码（2026-10-04 D2）。
func (s *SettingService) IsPasswordResetEnabled(ctx context.Context) bool {
	return s.IsEmailVerifyEnabled(ctx) && s.GetFrontendURL(ctx) != ""
}

// IsTotpEnabled 双因素认证是否可用：配了 TOTP_ENCRYPTION_KEY 就开，不再有后台开关。
// 以前是设置表里的开关，关掉后已绑 TOTP 的账号（含管理员）登录就不再要验证码。
func (s *SettingService) IsTotpEnabled() bool {
	return s.IsTotpEncryptionKeyConfigured()
}

// PasskeyEnabled Passkey 登录跟着部署配置走（webauthn.enabled + RP ID / origins），不再有后台开关。
func (s *SettingService) PasskeyEnabled() bool {
	return s.passkeyConfigured()
}

func (s *SettingService) passkeyConfigured() bool {
	return s != nil && s.cfg != nil && s.cfg.WebAuthn.Enabled
}

// IsTotpEncryptionKeyConfigured 检查 TOTP 加密密钥是否已手动配置
// 只有手动配置了密钥才允许在管理后台启用 TOTP 功能
func (s *SettingService) IsTotpEncryptionKeyConfigured() bool {
	return s.cfg.Totp.EncryptionKeyConfigured
}

// IsSessionBindingEnabled 会话 IP/UA 绑定是否启用：由代码决定（site_features.go），不再有后台开关。
func (s *SettingService) IsSessionBindingEnabled(ctx context.Context) bool {
	return SessionBindingEnabled
}

// IsStepUpEnabled 敏感操作（账号/代理导出、备份、S3 配置、提升管理员等）是否要求 TOTP 二次验证：
// 由代码决定（site_features.go），不再有后台开关。
func (s *SettingService) IsStepUpEnabled(ctx context.Context) bool {
	return StepUpEnabled
}

// TencentCaptchaConfig contains the credentials required by Tencent Cloud's
// ticket verification API. It must never be returned by a public handler.
type TencentCaptchaConfig struct {
	Enabled        bool
	AppID          string
	AppSecretKey   string
	CloudSecretID  string
	CloudSecretKey string
	Region         string
}

// AliyunCaptchaConfig contains the credentials required by Aliyun Captcha 2.0's
// server-side verification API. It must never be returned by a public handler.
type AliyunCaptchaConfig struct {
	Enabled         bool
	AccessKeyID     string
	AccessKeySecret string
	SceneID         string
	Prefix          string
	Region          string
}

type CaptchaProviderConfig struct {
	TurnstileEnabled   bool
	TurnstileSiteKey   string
	TurnstileSecretKey string
	Tencent            TencentCaptchaConfig
	Aliyun             AliyunCaptchaConfig
}

// CaptchaProviderConfig 人机验证只认部署配置：配齐了哪家就开哪家（启动时已校验同一时间最多一家）。
func (s *SettingService) CaptchaProviderConfig() CaptchaProviderConfig {
	if s == nil || s.cfg == nil {
		return CaptchaProviderConfig{}
	}
	turnstile, tencent, aliyun := s.cfg.Turnstile, s.cfg.TencentCaptcha, s.cfg.AliyunCaptcha
	return CaptchaProviderConfig{
		TurnstileEnabled:   turnstile.Configured(),
		TurnstileSiteKey:   strings.TrimSpace(turnstile.SiteKey),
		TurnstileSecretKey: strings.TrimSpace(turnstile.SecretKey),
		Tencent: TencentCaptchaConfig{
			Enabled:        tencent.Configured(),
			AppID:          strings.TrimSpace(tencent.AppID),
			AppSecretKey:   strings.TrimSpace(tencent.AppSecretKey),
			CloudSecretID:  strings.TrimSpace(tencent.CloudSecretID),
			CloudSecretKey: strings.TrimSpace(tencent.CloudSecretKey),
			Region:         normalizeTencentCaptchaRegion(strings.TrimSpace(tencent.Region)),
		},
		Aliyun: AliyunCaptchaConfig{
			Enabled:         aliyun.Configured(),
			AccessKeyID:     strings.TrimSpace(aliyun.AccessKeyID),
			AccessKeySecret: strings.TrimSpace(aliyun.AccessKeySecret),
			SceneID:         strings.TrimSpace(aliyun.SceneID),
			Prefix:          strings.TrimSpace(aliyun.Prefix),
			Region:          normalizeAliyunCaptchaRegion(strings.TrimSpace(aliyun.Region)),
		},
	}
}

// GenerateAdminAPIKey 生成新的管理员 API Key
func (s *SettingService) GenerateAdminAPIKey(ctx context.Context) (string, error) {
	// 生成 32 字节随机数 = 64 位十六进制字符
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}

	key := AdminAPIKeyPrefix + hex.EncodeToString(bytes)

	// 存储到 settings 表
	if err := s.settingRepo.Set(ctx, SettingKeyAdminAPIKey, key); err != nil {
		return "", fmt.Errorf("save admin api key: %w", err)
	}

	return key, nil
}

// GetAdminAPIKeyStatus 获取管理员 API Key 状态
// 返回脱敏的 key、是否存在、错误
func (s *SettingService) GetAdminAPIKeyStatus(ctx context.Context) (maskedKey string, exists bool, err error) {
	key, err := s.settingRepo.GetValue(ctx, SettingKeyAdminAPIKey)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	if key == "" {
		return "", false, nil
	}

	// 脱敏：显示前 10 位和后 4 位
	if len(key) > 14 {
		maskedKey = key[:10] + "..." + key[len(key)-4:]
	} else {
		maskedKey = key
	}

	return maskedKey, true, nil
}

// GetAdminAPIKey 获取完整的管理员 API Key（仅供内部验证使用）
// 如果未配置返回空字符串和 nil 错误，只有数据库错误时才返回 error
func (s *SettingService) GetAdminAPIKey(ctx context.Context) (string, error) {
	key, err := s.settingRepo.GetValue(ctx, SettingKeyAdminAPIKey)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return "", nil // 未配置，返回空字符串
		}
		return "", err // 数据库错误
	}
	return key, nil
}

// DeleteAdminAPIKey 删除管理员 API Key
func (s *SettingService) DeleteAdminAPIKey(ctx context.Context) error {
	return s.settingRepo.Delete(ctx, SettingKeyAdminAPIKey)
}

// GetProfitControlSettings 返回利润门设置（只有最低毛利率）；读不到 / 解析失败一律按 0 = 关
// （fail-open：可用性优先，同原分组门）。
func (s *SettingService) GetProfitControlSettings(ctx context.Context) ProfitControlSettings {
	if s == nil || s.settingRepo == nil {
		return ProfitControlSettings{}
	}
	if cached, ok := profitControlSettingsCache.Load().(*cachedProfitControlSettings); ok && cached != nil && time.Now().UnixNano() < cached.expiresAt {
		return cached.settings
	}
	result, _, _ := profitControlSettingsSF.Do(profitControlSettingsRefreshKey, func() (any, error) {
		if cached, ok := profitControlSettingsCache.Load().(*cachedProfitControlSettings); ok && cached != nil && time.Now().UnixNano() < cached.expiresAt {
			return cached.settings, nil
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), profitControlSettingsDBTimeout)
		defer cancel()
		settings := ProfitControlSettings{}
		ttl := profitControlSettingsCacheTTL
		raw, err := s.settingRepo.GetValue(dbCtx, SettingKeyProfitMinMargin)
		if err != nil && !errors.Is(err, ErrSettingNotFound) {
			slog.Warn("profit_control_settings_load_failed", "error", err)
			ttl = profitControlSettingsErrorTTL
		} else if err == nil {
			settings.MinMargin = parseProfitControlRatio(raw)
		}
		profitControlSettingsCache.Store(&cachedProfitControlSettings{settings: settings, expiresAt: time.Now().Add(ttl).UnixNano()})
		return settings, nil
	})
	if settings, ok := result.(ProfitControlSettings); ok {
		return settings
	}
	return ProfitControlSettings{}
}
