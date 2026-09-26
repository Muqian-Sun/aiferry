//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// newAuthServiceForCaptchaTest cfg 里带着部署配置的人机验证凭证（人机验证只认部署配置）。
func newAuthServiceForCaptchaTest(cfg *config.Config, required bool, turnstileVerifier TurnstileVerifier, tencentVerifier TencentCaptchaVerifier) *AuthService {
	cfg.Server.Mode = "release"
	cfg.Turnstile.Required = required
	settingService := NewSettingService(&settingRepoStub{values: map[string]string{}}, cfg)
	var turnstileService *TurnstileService
	if turnstileVerifier != nil {
		turnstileService = NewTurnstileService(settingService, turnstileVerifier)
	}
	svc := NewAuthService(nil, &userRepoStub{}, nil, nil, cfg, settingService, nil, turnstileService, nil, nil, nil)
	if tencentVerifier != nil {
		svc.SetTencentCaptchaService(NewTencentCaptchaService(settingService, tencentVerifier))
	}
	return svc
}

func TestVerifyCaptchaUsesTencentWhenEnabled(t *testing.T) {
	verifier := &tencentCaptchaVerifierStub{response: &TencentCaptchaVerifyResponse{CaptchaCode: 1}}
	svc := newAuthServiceForCaptchaTest(tencentCaptchaTestConfig(""), false, nil, verifier)

	err := svc.VerifyCaptcha(context.Background(), CaptchaProof{
		TencentTicket:  "ticket",
		TencentRandstr: "@rand",
	}, "203.0.113.10")

	require.NoError(t, err)
	require.Equal(t, 1, verifier.calls)
}

func TestVerifyCaptchaRequiredModeAcceptsCompleteTencentProvider(t *testing.T) {
	verifier := &tencentCaptchaVerifierStub{response: &TencentCaptchaVerifyResponse{CaptchaCode: 1}}
	svc := newAuthServiceForCaptchaTest(tencentCaptchaTestConfig(""), true, nil, verifier)

	err := svc.VerifyCaptcha(context.Background(), CaptchaProof{
		TencentTicket:  "ticket",
		TencentRandstr: "@rand",
	}, "203.0.113.10")

	require.NoError(t, err)
}

func TestVerifyCaptchaForRegisterSkipsDuplicateTencentTicketAfterEmailCode(t *testing.T) {
	verifier := &tencentCaptchaVerifierStub{response: &TencentCaptchaVerifyResponse{CaptchaCode: 1}}
	svc := newAuthServiceForCaptchaTest(tencentCaptchaTestConfig(""), true, nil, verifier)
	svc.cfg.SMTP = testSMTPConfigured // 配了 SMTP：注册要验证邮箱

	err := svc.VerifyCaptchaForRegister(context.Background(), CaptchaProof{}, "203.0.113.10", "123456")

	require.NoError(t, err)
	require.Zero(t, verifier.calls)
}

func TestVerifyActionCaptchaIfEnabledVerifiesTencentProof(t *testing.T) {
	verifier := &tencentCaptchaVerifierStub{response: &TencentCaptchaVerifyResponse{CaptchaCode: 1}}
	svc := newAuthServiceForCaptchaTest(tencentCaptchaTestConfig(""), false, nil, verifier)

	err := svc.VerifyActionCaptchaIfEnabled(context.Background(), CaptchaProof{
		TencentTicket:  "ticket",
		TencentRandstr: "@rand",
	}, "203.0.113.10")

	require.NoError(t, err)
	require.Equal(t, 1, verifier.calls)
	require.Equal(t, TencentCaptchaProof{Ticket: "ticket", Randstr: "@rand"}, verifier.proof)
}

func TestVerifyActionCaptchaIfEnabledDoesNotExpandTurnstileCoverage(t *testing.T) {
	turnstileVerifier := &turnstileVerifierSpy{}
	svc := newAuthServiceForCaptchaTest(&config.Config{
		Turnstile: config.TurnstileConfig{SiteKey: "site-key", SecretKey: "turnstile-secret"},
	}, false, turnstileVerifier, nil)

	err := svc.VerifyActionCaptchaIfEnabled(context.Background(), CaptchaProof{}, "203.0.113.10")

	require.NoError(t, err)
	require.Zero(t, turnstileVerifier.called)
}
