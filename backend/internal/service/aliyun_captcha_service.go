package service

import (
	"context"
	"fmt"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

var (
	ErrAliyunCaptchaVerificationFailed = infraerrors.BadRequest("ALIYUN_CAPTCHA_VERIFICATION_FAILED", "aliyun captcha verification failed")
	ErrAliyunCaptchaNotConfigured      = infraerrors.ServiceUnavailable("ALIYUN_CAPTCHA_NOT_CONFIGURED", "aliyun captcha not configured")
)

// AliyunCaptchaCredentials 阿里云验证码 2.0 服务端校验所需的完整凭证
type AliyunCaptchaCredentials struct {
	AccessKeyID     string
	AccessKeySecret string
	SceneID         string
	Endpoint        string
}

// AliyunCaptchaVerifyResult VerifyIntelligentCaptcha 的归一化结果
type AliyunCaptchaVerifyResult struct {
	VerifyResult bool
	VerifyCode   string // 阿里云细分结果码，仅用于日志
}

// AliyunCaptchaAPIError 阿里云 OpenAPI 业务错误。
// repository 层负责把 SDK 错误归一化为该类型，service 层不依赖 SDK 包。
type AliyunCaptchaAPIError struct {
	Code    string
	Message string
}

func (e *AliyunCaptchaAPIError) Error() string {
	return fmt.Sprintf("aliyun captcha api error: %s: %s", e.Code, e.Message)
}

// AliyunCaptchaVerifier 调用阿里云验证码 2.0 服务端校验的端口
type AliyunCaptchaVerifier interface {
	VerifyCaptcha(ctx context.Context, cred AliyunCaptchaCredentials, captchaVerifyParam string) (*AliyunCaptchaVerifyResult, error)
}

const (
	// AliyunCaptchaRegionCN 中国内地；AliyunCaptchaRegionSGP 新加坡。
	// 该值同时下发给前端 AliyunCaptchaConfig.region，两端必须一致。
	AliyunCaptchaRegionCN  = "cn"
	AliyunCaptchaRegionSGP = "sgp"

	aliyunCaptchaEndpointCN  = "captcha.cn-shanghai.aliyuncs.com"
	aliyunCaptchaEndpointSGP = "captcha.ap-southeast-1.aliyuncs.com"
)

// aliyunCaptchaEndpoint 按后台配置的地域返回服务端接入点，未知值回退中国内地
func aliyunCaptchaEndpoint(region string) string {
	if region == AliyunCaptchaRegionSGP {
		return aliyunCaptchaEndpointSGP
	}
	return aliyunCaptchaEndpointCN
}

// normalizeAliyunCaptchaRegion 非法值一律视为中国内地
func normalizeAliyunCaptchaRegion(value string) string {
	if value == AliyunCaptchaRegionSGP {
		return AliyunCaptchaRegionSGP
	}
	return AliyunCaptchaRegionCN
}

// AliyunCaptchaService 阿里云验证码 2.0 服务端校验
type AliyunCaptchaService struct {
	settingService *SettingService
	verifier       AliyunCaptchaVerifier
}

func NewAliyunCaptchaService(settingService *SettingService, verifier AliyunCaptchaVerifier) *AliyunCaptchaService {
	return &AliyunCaptchaService{settingService: settingService, verifier: verifier}
}

func aliyunCaptchaCredentials(config AliyunCaptchaConfig) (AliyunCaptchaCredentials, bool) {
	cred := AliyunCaptchaCredentials{
		AccessKeyID:     strings.TrimSpace(config.AccessKeyID),
		AccessKeySecret: strings.TrimSpace(config.AccessKeySecret),
		SceneID:         strings.TrimSpace(config.SceneID),
		Endpoint:        aliyunCaptchaEndpoint(config.Region),
	}
	if cred.AccessKeyID == "" || cred.AccessKeySecret == "" || cred.SceneID == "" {
		return AliyunCaptchaCredentials{}, false
	}
	return cred, true
}

// VerifyParamWithConfig 校验阿里云验证码 2.0 的 captchaVerifyParam。
// 调用异常时返回错误（fail-closed），与 Turnstile 网络错误行为对称。
func (s *AliyunCaptchaService) VerifyParamWithConfig(ctx context.Context, config AliyunCaptchaConfig, captchaVerifyParam string) error {
	if s == nil || s.verifier == nil {
		return ErrAliyunCaptchaNotConfigured
	}
	cred, ok := aliyunCaptchaCredentials(config)
	if !ok {
		logger.LegacyPrintf("service.aliyun_captcha", "%s", "[AliyunCaptcha] credentials not configured")
		return ErrAliyunCaptchaNotConfigured
	}

	if strings.TrimSpace(captchaVerifyParam) == "" {
		logger.LegacyPrintf("service.aliyun_captcha", "%s", "[AliyunCaptcha] captchaVerifyParam is empty")
		return ErrAliyunCaptchaVerificationFailed
	}

	result, err := s.verifier.VerifyCaptcha(ctx, cred, captchaVerifyParam)
	if err != nil {
		logger.LegacyPrintf("service.aliyun_captcha", "[AliyunCaptcha] verify request failed: %v", err)
		return fmt.Errorf("%w: verifier request failed", ErrAliyunCaptchaVerificationFailed)
	}

	if result == nil || !result.VerifyResult {
		if result != nil {
			logger.LegacyPrintf("service.aliyun_captcha", "[AliyunCaptcha] rejected, verify code: %s", result.VerifyCode)
		}
		return ErrAliyunCaptchaVerificationFailed
	}
	return nil
}
