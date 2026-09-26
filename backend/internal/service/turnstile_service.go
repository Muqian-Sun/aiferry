package service

import (
	"context"
	"fmt"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

var (
	ErrTurnstileVerificationFailed = infraerrors.BadRequest("TURNSTILE_VERIFICATION_FAILED", "turnstile verification failed")
	ErrTurnstileNotConfigured      = infraerrors.ServiceUnavailable("TURNSTILE_NOT_CONFIGURED", "turnstile not configured")
)

// TurnstileVerifier 验证 Turnstile token 的接口
type TurnstileVerifier interface {
	VerifyToken(ctx context.Context, secretKey, token, remoteIP string) (*TurnstileVerifyResponse, error)
}

// TurnstileService Turnstile 验证服务
type TurnstileService struct {
	settingService *SettingService
	verifier       TurnstileVerifier
}

// TurnstileVerifyResponse Cloudflare Turnstile 验证响应
type TurnstileVerifyResponse struct {
	Success     bool     `json:"success"`
	ChallengeTS string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
	ErrorCodes  []string `json:"error-codes"`
	Action      string   `json:"action"`
	CData       string   `json:"cdata"`
}

// NewTurnstileService 创建 Turnstile 服务实例
func NewTurnstileService(settingService *SettingService, verifier TurnstileVerifier) *TurnstileService {
	return &TurnstileService{
		settingService: settingService,
		verifier:       verifier,
	}
}

// VerifyToken 验证 Turnstile token
func (s *TurnstileService) VerifyToken(ctx context.Context, token string, remoteIP string) error {
	providerConfig := s.settingService.CaptchaProviderConfig()
	if !providerConfig.TurnstileEnabled {
		logger.LegacyPrintf("service.turnstile", "%s", "[Turnstile] Disabled, skipping verification")
		return nil
	}
	return s.VerifyTokenWithSecret(ctx, providerConfig.TurnstileSecretKey, token, remoteIP)
}

func (s *TurnstileService) VerifyTokenWithSecret(ctx context.Context, secretKey, token, remoteIP string) error {
	if secretKey == "" {
		logger.LegacyPrintf("service.turnstile", "%s", "[Turnstile] Secret key not configured")
		return ErrTurnstileNotConfigured
	}

	// 如果 token 为空，返回错误
	if token == "" {
		logger.LegacyPrintf("service.turnstile", "%s", "[Turnstile] Token is empty")
		return ErrTurnstileVerificationFailed
	}

	logger.LegacyPrintf("service.turnstile", "[Turnstile] Verifying token for IP: %s", remoteIP)
	if s == nil || s.verifier == nil {
		return ErrTurnstileNotConfigured
	}
	result, err := s.verifier.VerifyToken(ctx, secretKey, token, remoteIP)
	if err != nil {
		logger.LegacyPrintf("service.turnstile", "[Turnstile] Request failed: %v", err)
		return fmt.Errorf("send request: %w", err)
	}

	if !result.Success {
		logger.LegacyPrintf("service.turnstile", "[Turnstile] Verification failed, error codes: %v", result.ErrorCodes)
		return ErrTurnstileVerificationFailed
	}

	logger.LegacyPrintf("service.turnstile", "%s", "[Turnstile] Verification successful")
	return nil
}

// IsEnabled 检查 Turnstile 是否启用
func (s *TurnstileService) IsEnabled(ctx context.Context) bool {
	return s.settingService.CaptchaProviderConfig().TurnstileEnabled
}
