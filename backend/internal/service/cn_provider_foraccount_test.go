package service

// ForAccount 直传入口（P2-6）的校验回归测试：
// QueryUsageForAccount / QueryBalanceForAccount 接受已加载的 *Account，
// 但必须复用与 ID 入口相同的加载后校验——直传不能绕过平台/模式检查，
// 且校验在 singleflight 之前完成（无效账号不得发起任何上游请求）。

import (
	"context"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func officialCNEndpoints(platform string) map[string]string {
	switch platform {
	case PlatformKimi:
		return map[string]string{APIProtocolChatCompletions: DefaultKimiPayGBaseURL}
	case PlatformZhipu:
		return map[string]string{APIProtocolChatCompletions: DefaultZhipuPayGBaseURL}
	case PlatformDeepseek:
		return map[string]string{APIProtocolChatCompletions: DefaultDeepseekBaseURL}
	case PlatformMiniMax:
		return map[string]string{APIProtocolChatCompletions: DefaultMiniMaxBaseURL}
	default:
		return nil
	}
}

func codingAccount(platform string) *Account {
	return &Account{
		ID: 1, Platform: platform, Type: AccountTypeAPIKey, Status: StatusActive,
		Credentials:       map[string]any{"account_mode": AccountModeCoding, "api_key": "sk-test"},
		ProtocolEndpoints: officialCNEndpoints(platform),
	}
}

func paygAccount(platform string) *Account {
	return &Account{
		ID: 2, Platform: platform, Type: AccountTypeAPIKey, Status: StatusActive,
		Credentials:       map[string]any{"account_mode": AccountModePayG, "api_key": "sk-test"},
		ProtocolEndpoints: officialCNEndpoints(platform),
	}
}

func requireReason(t *testing.T, err error, reason string) {
	t.Helper()
	require.Error(t, err)
	var appErr *infraerrors.ApplicationError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, reason, appErr.Reason)
}

func TestValidateCodingPlanAccount_Matrix(t *testing.T) {
	cases := []struct {
		name       string
		account    *Account
		wantReason string
	}{
		{name: "nil", account: nil, wantReason: "CN_QUOTA_ACCOUNT_NOT_FOUND"},
		{name: "non cn provider", account: &Account{ID: 3, Platform: PlatformAnthropic}, wantReason: "CN_QUOTA_INVALID_PLATFORM"},
		{name: "payg has no quota endpoint", account: paygAccount(PlatformKimi), wantReason: "CN_QUOTA_NOT_CODING_PLAN"},
		{name: "kimi coding ok", account: codingAccount(PlatformKimi)},
		{name: "zhipu coding ok", account: codingAccount(PlatformZhipu)},
		{name: "minimax coding ok", account: codingAccount(PlatformMiniMax)},
		{name: "openai label on kimi coding still ok", account: &Account{
			ID: 6, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive,
			Credentials:       map[string]any{"account_mode": AccountModeCoding, "api_key": "sk-test"},
			ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: DefaultKimiCodingBaseURL},
		}},
		{name: "kimi label on relay coding rejected", account: &Account{
			ID: 7, Platform: PlatformKimi, Type: AccountTypeAPIKey, Status: StatusActive,
			Credentials:       map[string]any{"account_mode": AccountModeCoding, "api_key": "sk-test"},
			ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://relay.example.com/v1"},
		}, wantReason: "CN_QUOTA_INVALID_PLATFORM"},
		{name: "opencode go by address not label", account: &Account{
			ID: 8, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive,
			ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: DefaultOpenCodeGoBaseURL},
		}},
		{name: "opencode zen by address rejected", account: &Account{
			ID: 9, Platform: PlatformOpenCodeGo, Type: AccountTypeAPIKey, Status: StatusActive,
			Credentials:       map[string]any{"account_mode": AccountModeGo},
			ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: DefaultOpenCodeZenBaseURL},
		}, wantReason: "CN_QUOTA_NOT_CODING_PLAN"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCodingPlanAccount(tc.account)
			if tc.wantReason == "" {
				require.NoError(t, err)
				return
			}
			requireReason(t, err, tc.wantReason)
		})
	}
}

func TestValidatePayGAccount_Matrix(t *testing.T) {
	cases := []struct {
		name       string
		account    *Account
		wantReason string
	}{
		{name: "nil", account: nil, wantReason: "CN_BALANCE_ACCOUNT_NOT_FOUND"},
		{name: "non cn provider", account: &Account{ID: 3, Platform: PlatformAnthropic}, wantReason: "CN_BALANCE_INVALID_PLATFORM"},
		{name: "coding has no balance endpoint", account: codingAccount(PlatformKimi), wantReason: "CN_BALANCE_CODING_PLAN"},
		{name: "kimi payg ok", account: paygAccount(PlatformKimi)},
		{name: "deepseek payg ok", account: paygAccount(PlatformDeepseek)},
		{name: "anthropic label on moonshot still payg", account: &Account{
			ID: 4, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Status: StatusActive,
			Credentials:       map[string]any{"account_mode": AccountModePayG, "api_key": "sk-test"},
			ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: DefaultKimiPayGBaseURL},
		}},
		{name: "kimi label on relay rejected", account: &Account{
			ID: 5, Platform: PlatformKimi, Type: AccountTypeAPIKey, Status: StatusActive,
			Credentials:       map[string]any{"account_mode": AccountModePayG, "api_key": "sk-test"},
			ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://relay.example.com/v1"},
		}, wantReason: "CN_BALANCE_INVALID_PLATFORM"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePayGAccount(tc.account)
			if tc.wantReason == "" {
				require.NoError(t, err)
				return
			}
			requireReason(t, err, tc.wantReason)
		})
	}
}

// 直传入口的校验在 singleflight/上游请求之前：无效账号必须零出站请求。
func TestCNProviderQuotaService_QueryUsageForAccount_RejectsInvalidAccount(t *testing.T) {
	repo := &fakeCNProbeAccountRepo{}
	upstream := &recordingHTTPUpstream{}
	svc := NewCNProviderQuotaService(repo, nil, upstream, nil)

	_, err := svc.QueryUsageForAccount(context.Background(), paygAccount(PlatformKimi))
	requireReason(t, err, "CN_QUOTA_NOT_CODING_PLAN")
	require.Zero(t, upstream.calls)

	_, err = svc.QueryUsageForAccount(context.Background(), nil)
	requireReason(t, err, "CN_QUOTA_ACCOUNT_NOT_FOUND")
	require.Zero(t, upstream.calls)
}

func TestCNProviderBalanceService_QueryBalanceForAccount_RejectsInvalidAccount(t *testing.T) {
	repo := &fakeCNProbeAccountRepo{}
	upstream := &recordingHTTPUpstream{}
	svc := NewCNProviderBalanceService(repo, nil, upstream, nil)

	_, err := svc.QueryBalanceForAccount(context.Background(), codingAccount(PlatformKimi))
	requireReason(t, err, "CN_BALANCE_CODING_PLAN")
	require.Zero(t, upstream.calls)

	_, err = svc.QueryBalanceForAccount(context.Background(), &Account{ID: 9, Platform: PlatformAnthropic})
	requireReason(t, err, "CN_BALANCE_INVALID_PLATFORM")
	require.Zero(t, upstream.calls)
}

// ID 入口与 ForAccount 入口对同一账号的行为一致（loadCodingPlanAccount 的
// 加载后校验 = validateCodingPlanAccount；余额侧对称）。
func TestCNProviderServices_IDEntryAppliesSameValidation(t *testing.T) {
	repo := &fakeCNProbeAccountRepo{account: paygAccount(PlatformKimi)}
	upstream := &recordingHTTPUpstream{}
	svc := NewCNProviderQuotaService(repo, nil, upstream, nil)

	_, err := svc.QueryUsage(context.Background(), 2)
	requireReason(t, err, "CN_QUOTA_NOT_CODING_PLAN")
	require.Zero(t, upstream.calls)
}
