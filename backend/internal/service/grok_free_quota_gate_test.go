//go:build unit

package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type grokFreeQuotaUsageRepoStub struct {
	UsageLogRepository

	mu      sync.Mutex
	stats   map[int64]*usagestats.AccountStats
	err     error
	calls   int
	lastIDs []int64
	start   time.Time
}

type grokFreeQuotaAccountRepoStub struct {
	AccountRepository
	accounts []Account
}

func (r *grokFreeQuotaAccountRepoStub) ListSchedulingCandidates(context.Context, []string) ([]Account, error) {
	return append([]Account(nil), r.accounts...), nil
}

func (r *grokFreeQuotaUsageRepoStub) GetAccountWindowStatsBatch(_ context.Context, accountIDs []int64, start time.Time) (map[int64]*usagestats.AccountStats, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	r.lastIDs = append([]int64(nil), accountIDs...)
	r.start = start
	if r.err != nil {
		return nil, r.err
	}
	result := make(map[int64]*usagestats.AccountStats, len(accountIDs))
	for _, accountID := range accountIDs {
		if stats := r.stats[accountID]; stats != nil {
			copyStats := *stats
			result[accountID] = &copyStats
		}
	}
	return result, nil
}

func grokFreeQuotaTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Gateway.Grok.FreeQuotaSoftGateEnabled = true
	cfg.Gateway.Grok.FreeQuotaTokenLimit = 500_000
	cfg.Gateway.Grok.FreeQuotaSoftGatePercent = 95
	cfg.Gateway.Grok.FreeQuotaWindowHours = 24
	cfg.Gateway.Grok.FreeQuotaStatsCacheSeconds = 60
	return cfg
}

func TestResolveGrokFreeQuotaGateSettingsDefaultsToNinetyFivePercent(t *testing.T) {
	settings, ok := resolveGrokFreeQuotaGateSettings(grokFreeQuotaTestConfig())
	require.True(t, ok)
	require.Equal(t, int64(500_000), settings.limitTokens)
	require.Equal(t, int64(475_000), settings.gateTokens) // 95% of 500k
	require.Equal(t, 24*time.Hour, settings.window)
}

func TestIsExplicitGrokFreeOAuthAccount_OnlyExactFree(t *testing.T) {
	t.Parallel()
	require.False(t, isExplicitGrokFreeOAuthAccount(nil))
	require.False(t, isExplicitGrokFreeOAuthAccount(&Account{Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"subscription_tier": "free"}, ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.x.ai/v1", APIProtocolResponses: "https://api.x.ai/v1"}}))
	require.True(t, isExplicitGrokFreeOAuthAccount(&Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Credentials: map[string]any{"subscription_tier": "FREE"}}))
	require.True(t, isExplicitGrokFreeOAuthAccount(&Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Credentials: map[string]any{"plan_type": "free"}}))
	// basic / inferred free are NOT soft-gated (only an explicit "free" tier is).
	require.False(t, isExplicitGrokFreeOAuthAccount(&Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Credentials: map[string]any{"subscription_tier": "basic"}}))
	require.False(t, isExplicitGrokFreeOAuthAccount(&Account{Platform: PlatformGrok, Type: AccountTypeOAuth}))
}

func accountIDs(accounts []Account) []int64 {
	ids := make([]int64, 0, len(accounts))
	for i := range accounts {
		ids = append(ids, accounts[i].ID)
	}
	return ids
}
