//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type aliyunVerifierSpy struct {
	called    int
	lastCred  AliyunCaptchaCredentials
	lastParam string
	result    *AliyunCaptchaVerifyResult
	err       error
}

func (s *aliyunVerifierSpy) VerifyCaptcha(_ context.Context, cred AliyunCaptchaCredentials, param string) (*AliyunCaptchaVerifyResult, error) {
	s.called++
	s.lastCred = cred
	s.lastParam = param
	if s.err != nil {
		return nil, s.err
	}
	if s.result != nil {
		return s.result, nil
	}
	return &AliyunCaptchaVerifyResult{VerifyResult: true}, nil
}

// withAliyunCaptcha 部署配置里配齐阿里云验证码（人机验证只认部署配置）。
func withAliyunCaptcha(cfg *config.Config) *config.Config {
	cfg.AliyunCaptcha = config.AliyunCaptchaConfig{
		AccessKeyID:     "ak-id",
		AccessKeySecret: "ak-secret",
		SceneID:         "scene-1",
		Prefix:          "prefix-1",
	}
	return cfg
}

func aliyunTestConfig() AliyunCaptchaConfig {
	return AliyunCaptchaConfig{
		Enabled:         true,
		AccessKeyID:     "ak-id",
		AccessKeySecret: "ak-secret",
		SceneID:         "scene-1",
		Region:          AliyunCaptchaRegionCN,
	}
}

func newAliyunAuthServiceForTest(cfg *config.Config, aliyunSpy *aliyunVerifierSpy) *AuthService {
	settingService := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, cfg)
	authService := NewAuthService(
		nil, // entClient
		nil, // userRepo
		nil, // redeemRepo
		nil, // refreshTokenCache
		cfg,
		settingService,
		nil, // emailService
		NewTurnstileService(settingService, &turnstileVerifierSpy{}),
		nil, // emailQueueService
		nil, // defaultSubAssigner
		nil,
	)
	authService.SetAliyunCaptchaService(NewAliyunCaptchaService(settingService, aliyunSpy))
	return authService
}

func TestAliyunCaptchaServiceVerifyParamDispatch(t *testing.T) {
	spy := &aliyunVerifierSpy{}
	svc := NewAliyunCaptchaService(nil, spy)

	err := svc.VerifyParamWithConfig(context.Background(), aliyunTestConfig(), "captcha-verify-param")

	require.NoError(t, err)
	require.Equal(t, 1, spy.called)
	require.Equal(t, "captcha-verify-param", spy.lastParam)
	require.Equal(t, "ak-id", spy.lastCred.AccessKeyID)
	require.Equal(t, "scene-1", spy.lastCred.SceneID)
	require.Equal(t, "captcha.cn-shanghai.aliyuncs.com", spy.lastCred.Endpoint)
}

func TestAliyunCaptchaServiceSgpEndpoint(t *testing.T) {
	spy := &aliyunVerifierSpy{}
	svc := NewAliyunCaptchaService(nil, spy)
	cfg := aliyunTestConfig()
	cfg.Region = AliyunCaptchaRegionSGP

	err := svc.VerifyParamWithConfig(context.Background(), cfg, "captcha-verify-param")

	require.NoError(t, err)
	require.Equal(t, "captcha.ap-southeast-1.aliyuncs.com", spy.lastCred.Endpoint)
}

func TestAliyunCaptchaServiceFailsClosedOnVerifierError(t *testing.T) {
	spy := &aliyunVerifierSpy{err: errors.New("network down")}
	svc := NewAliyunCaptchaService(nil, spy)

	err := svc.VerifyParamWithConfig(context.Background(), aliyunTestConfig(), "captcha-verify-param")

	require.ErrorIs(t, err, ErrAliyunCaptchaVerificationFailed)
}

func TestAliyunCaptchaServiceRejectsVerifyResultFalse(t *testing.T) {
	spy := &aliyunVerifierSpy{result: &AliyunCaptchaVerifyResult{VerifyResult: false, VerifyCode: "F001"}}
	svc := NewAliyunCaptchaService(nil, spy)

	err := svc.VerifyParamWithConfig(context.Background(), aliyunTestConfig(), "captcha-verify-param")

	require.ErrorIs(t, err, ErrAliyunCaptchaVerificationFailed)
}

func TestAliyunCaptchaServiceRejectsIncompleteCredentials(t *testing.T) {
	spy := &aliyunVerifierSpy{}
	svc := NewAliyunCaptchaService(nil, spy)
	cfg := aliyunTestConfig()
	cfg.AccessKeySecret = ""

	err := svc.VerifyParamWithConfig(context.Background(), cfg, "captcha-verify-param")

	require.ErrorIs(t, err, ErrAliyunCaptchaNotConfigured)
	require.Zero(t, spy.called)
}

func TestAliyunCaptchaServiceRejectsEmptyParam(t *testing.T) {
	spy := &aliyunVerifierSpy{}
	svc := NewAliyunCaptchaService(nil, spy)

	err := svc.VerifyParamWithConfig(context.Background(), aliyunTestConfig(), "")

	require.ErrorIs(t, err, ErrAliyunCaptchaVerificationFailed)
	require.Zero(t, spy.called)
}

func TestAuthServiceVerifyCaptchaDispatchesAliyun(t *testing.T) {
	spy := &aliyunVerifierSpy{}
	authService := newAliyunAuthServiceForTest(withAliyunCaptcha(&config.Config{}), spy)

	// 阿里云 captchaVerifyParam 复用 turnstile_token 请求字段
	err := authService.VerifyCaptcha(context.Background(), CaptchaProof{TurnstileToken: "captcha-verify-param"}, "127.0.0.1")

	require.NoError(t, err)
	require.Equal(t, 1, spy.called)
	require.Equal(t, "captcha-verify-param", spy.lastParam)
}

func TestAuthServiceVerifyCaptchaRequiredModeWithAliyun(t *testing.T) {
	cfg := &config.Config{
		Server:    config.ServerConfig{Mode: "release"},
		Turnstile: config.TurnstileConfig{Required: true},
	}
	spy := &aliyunVerifierSpy{}
	authService := newAliyunAuthServiceForTest(withAliyunCaptcha(cfg), spy)

	// required 模式 + 阿里云启用且凭证齐全：不误报 NOT_CONFIGURED，正常走阿里云校验
	err := authService.VerifyCaptcha(context.Background(), CaptchaProof{TurnstileToken: "captcha-verify-param"}, "127.0.0.1")

	require.NoError(t, err)
	require.Equal(t, 1, spy.called)
}

func TestAuthServiceVerifyActionCaptchaIfEnabledDispatchesAliyun(t *testing.T) {
	spy := &aliyunVerifierSpy{}
	authService := newAliyunAuthServiceForTest(withAliyunCaptcha(&config.Config{}), spy)

	err := authService.VerifyActionCaptchaIfEnabled(context.Background(), CaptchaProof{TurnstileToken: "captcha-verify-param"}, "127.0.0.1")

	require.NoError(t, err)
	require.Equal(t, 1, spy.called)
	require.Equal(t, "captcha-verify-param", spy.lastParam)
}

func TestAuthServiceVerifyActionCaptchaIfEnabledSkipsWhenOnlyTurnstile(t *testing.T) {
	spy := &aliyunVerifierSpy{}
	authService := newAliyunAuthServiceForTest(&config.Config{
		Turnstile: config.TurnstileConfig{SiteKey: "site-key", SecretKey: "secret"},
	}, spy)

	// Turnstile 不扩大既有覆盖：扩展入口不拦截
	err := authService.VerifyActionCaptchaIfEnabled(context.Background(), CaptchaProof{}, "127.0.0.1")

	require.NoError(t, err)
	require.Zero(t, spy.called)
}
