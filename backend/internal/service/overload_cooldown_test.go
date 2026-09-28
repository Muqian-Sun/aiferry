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

func TestHandleUpstreamError_529PausesAccountForTenMinutes(t *testing.T) {
	accountRepo := &overloadAccountRepoStub{}
	svc := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)

	account := &Account{ID: 42, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
	before := time.Now()
	shouldDisable := svc.HandleUpstreamError(context.Background(), account, 529, nil, []byte(`{"error":{"message":"overloaded"}}`))

	require.False(t, shouldDisable)
	require.Equal(t, 1, accountRepo.overloadCalls)
	require.Equal(t, int64(42), accountRepo.lastOverloadID)
	require.WithinDuration(t, before.Add(10*time.Minute), accountRepo.lastOverloadEnd, 2*time.Second)
}

func TestHandle529_NilSettingService_ZeroConfig_DefaultsTen(t *testing.T) {
	accountRepo := &overloadAccountRepoStub{}
	svc := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)

	account := &Account{ID: 88, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
	before := time.Now()
	svc.handle529(context.Background(), account)

	require.Equal(t, 1, accountRepo.overloadCalls)
	require.WithinDuration(t, before.Add(10*time.Minute), accountRepo.lastOverloadEnd, 2*time.Second)
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

// 渠道级自定义错误码 2026-09-28 P5 已删：旧行留着的配置不再让 529 停掉渠道，照常走全站过载冷却。
func TestHandleUpstreamError_529LegacyCustomCodesStillUseOverloadCooldown(t *testing.T) {
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
	require.Equal(t, 1, repo.overloadCalls)
}
