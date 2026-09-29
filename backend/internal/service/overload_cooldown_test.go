//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// overloadAccountRepoStub: records SetOverloaded calls
// ---------------------------------------------------------------------------

type overloadAccountRepoStub struct {
	mockAccountRepoForGemini
	overloadCalls   int
	errorCalls      int
	lastOverloadID  int64
	lastOverloadEnd time.Time
}

func (r *overloadAccountRepoStub) SetError(_ context.Context, _ int64, _ string) error {
	r.errorCalls++
	return nil
}

func (r *overloadAccountRepoStub) SetOverloaded(_ context.Context, id int64, until time.Time) error {
	r.overloadCalls++
	r.lastOverloadID = id
	r.lastOverloadEnd = until
	return nil
}

// ===========================================================================
// RateLimitService: handle529 behaviour
// ===========================================================================

// 529 过载冷却只对 Claude 成品号（OAuth / setup-token），恰为 1 分钟（2026-09-29 muqian 定）。
func TestHandleUpstreamError_529PausesClaudeSubscriptionForOneMinute(t *testing.T) {
	for _, accountType := range []string{AccountTypeOAuth, AccountTypeSetupToken} {
		t.Run(accountType, func(t *testing.T) {
			accountRepo := &overloadAccountRepoStub{}
			svc := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)

			account := &Account{ID: 42, Platform: PlatformAnthropic, Type: accountType}
			before := time.Now()
			shouldDisable := svc.HandleUpstreamError(context.Background(), account, 529, nil, []byte(`{"error":{"message":"overloaded"}}`))
			after := time.Now()

			require.False(t, shouldDisable)
			require.Equal(t, 1, accountRepo.overloadCalls)
			require.Equal(t, int64(42), accountRepo.lastOverloadID)
			require.False(t, accountRepo.lastOverloadEnd.Before(before.Add(time.Minute)), "冷却不得短于 1 分钟")
			require.False(t, accountRepo.lastOverloadEnd.After(after.Add(time.Minute)), "冷却不得长于 1 分钟")
		})
	}
}

func TestHandle529_PausesForOneMinute(t *testing.T) {
	accountRepo := &overloadAccountRepoStub{}
	svc := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)

	account := &Account{ID: 88, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
	before := time.Now()
	svc.handle529(context.Background(), account)
	after := time.Now()

	require.Equal(t, 1, accountRepo.overloadCalls)
	require.False(t, accountRepo.lastOverloadEnd.Before(before.Add(time.Minute)))
	require.False(t, accountRepo.lastOverloadEnd.After(after.Add(time.Minute)))
}

// 除 Claude 成品号外，所有渠道收到 529 都不冷却（本次请求由 handler 换号）。
func TestHandleUpstreamError_529DoesNotCoolNonClaudeSubscriptions(t *testing.T) {
	tests := []struct {
		name    string
		account *Account
	}{
		{name: "anthropic official api key", account: &Account{ID: 201, Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
			Credentials: map[string]any{"api_key": "sk-ant"}, ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"}}},
		{name: "relay api key", account: &Account{ID: 202, Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
			Credentials: map[string]any{"api_key": "relay"}, ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://relay.example.com"}}},
		{name: "bedrock", account: &Account{ID: 203, Platform: PlatformAnthropic, Type: AccountTypeBedrock}},
		{name: "openai oauth", account: &Account{ID: 204, Platform: PlatformOpenAI, Type: AccountTypeOAuth}},
		{name: "openai api key", account: &Account{ID: 205, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
			Credentials: map[string]any{"api_key": "sk"}, ProtocolEndpoints: map[string]string{APIProtocolResponses: "https://api.openai.com"}}},
		{name: "gemini oauth", account: &Account{ID: 206, Platform: PlatformGemini, Type: AccountTypeOAuth}},
		{name: "gemini api key", account: &Account{ID: 207, Platform: PlatformGemini, Type: AccountTypeAPIKey,
			Credentials: map[string]any{"api_key": "g"}, ProtocolEndpoints: map[string]string{APIProtocolGemini: "https://generativelanguage.googleapis.com"}}},
		{name: "antigravity oauth", account: &Account{ID: 208, Platform: PlatformAntigravity, Type: AccountTypeOAuth}},
		{name: "grok oauth", account: &Account{ID: 209, Platform: PlatformGrok, Type: AccountTypeOAuth}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &overloadAccountRepoStub{}
			svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)

			shouldDisable := svc.HandleUpstreamError(context.Background(), tt.account, 529, nil, []byte(`{"error":{"message":"overloaded"}}`))

			require.False(t, shouldDisable)
			require.Zero(t, repo.overloadCalls, "非 Claude 成品号 529 不冷却")
			require.Zero(t, repo.errorCalls)
		})
	}
}

func TestHandleUpstreamError_529RespectsAccountPolicies(t *testing.T) {
	tests := []struct {
		name        string
		credentials map[string]any
	}{
		{
			name:        "pool mode",
			credentials: map[string]any{"pool_mode": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &overloadAccountRepoStub{}
			svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
			account := &Account{
				ID:          101,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeAPIKey,
				Credentials: tt.credentials,
			}

			shouldDisable := svc.HandleUpstreamError(context.Background(), account, 529, nil, []byte(`{"error":{"message":"overloaded"}}`))

			require.False(t, shouldDisable)
			require.Zero(t, repo.overloadCalls)
			require.Zero(t, repo.errorCalls)
		})
	}
}

// 渠道级自定义错误码 2026-09-28 P5 已删：旧行留着的配置不再让 529 停掉渠道；
// 非 Claude 成品号的 529 也不做过载冷却（2026-09-29 定）。
func TestHandleUpstreamError_529LegacyCustomCodesDoNotCooldown(t *testing.T) {
	repo := &overloadAccountRepoStub{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account := &Account{
		ID:       102,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"custom_error_codes_enabled": true,
			"custom_error_codes":         []any{float64(529)},
		},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
	}

	shouldDisable := svc.HandleUpstreamError(context.Background(), account, 529, nil, []byte(`{"error":{"message":"overloaded"}}`))

	require.False(t, shouldDisable)
	require.Zero(t, repo.errorCalls)
	require.Zero(t, repo.overloadCalls)
}
