//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type windowStatsRepoStub struct {
	usageBatchLogRepoStub
	stats *usagestats.AccountStats
	calls int
}

func (r *windowStatsRepoStub) GetAccountWindowStats(context.Context, int64, time.Time) (*usagestats.AccountStats, error) {
	r.calls++
	return r.stats, nil
}

func TestApplyAccountUsageState_GrokFreeQuotaPauses(t *testing.T) {
	rl, repo := quotaStateTestService(t)
	rl.cfg = grokFreeQuotaTestConfig()
	rl.usageRepo = &windowStatsRepoStub{stats: &usagestats.AccountStats{Tokens: 480_000}}
	account := &Account{ID: 3005, Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
		Credentials: map[string]any{"subscription_tier": "free"}}

	rl.ApplyAccountUsageState(context.Background(), account, "grok-4.5")

	require.Equal(t, 1, repo.tempCalls)
	payload, ok := parseTempUnschedReasonPayload(repo.lastTempReason)
	require.True(t, ok)
	require.Equal(t, grokFreeQuotaSource, payload.Source)
	require.WithinDuration(t, time.Now().Add(grokFreeQuotaPauseMin), *account.TempUnschedulableUntil, 5*time.Second)

	// 非 free 档不受影响。
	paid := &Account{ID: 3006, Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
		Credentials: map[string]any{"subscription_tier": "supergrok"}}
	rl.ApplyAccountUsageState(context.Background(), paid, "grok-4.5")
	require.Equal(t, 1, repo.tempCalls)
}

// Google 官方档位的本地 RPD 用满后按模型档写模型级限流：Gemini 成品号被挡，第三方 key（一律按中转）不受影响。
func TestApplyAccountUsageState_GeminiLocalRPDSetsModelRateLimit(t *testing.T) {
	usage := &geminiPrecheckUsageRepoStub{stats: []usagestats.ModelStat{{Model: "gemini-2.5-pro", Requests: 1000}}}
	quotaSvc := NewGeminiQuotaService(&config.Config{}, nil)
	repo := &geminiLocalQuotaRepoStub{}
	rl := NewRateLimitService(repo, usage, &config.Config{}, quotaSvc, nil)

	// 本地按天限流只对免费档成立，这里用 AI Studio 授权的免费档成品号
	official := &Account{ID: 3007, Platform: PlatformGemini, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
		Credentials: map[string]any{"oauth_type": "ai_studio", "tier_id": GeminiTierAIStudioFree}}
	require.Equal(t, PlatformGemini, official.Vendor())

	rl.ApplyAccountUsageState(context.Background(), official, "gemini-2.5-pro")

	require.Equal(t, []string{geminiLocalQuotaScope(geminiModelPro)}, repo.scopes, "只挡 pro 档")
	require.WithinDuration(t, geminiDailyResetTime(time.Now()), repo.lastResetAt, 2*time.Second)
	require.False(t, official.SchedulingAllows(context.Background(), "gemini-2.5-pro", time.Now()))
	require.True(t, official.SchedulingAllows(context.Background(), "gemini-2.5-flash", time.Now()), "flash 档不受影响")

	relay := vendorTestKey(PlatformGemini, vendorTestRelayGemini)
	relay.ID = 3008
	relay.Status, relay.Schedulable = StatusActive, true
	require.Empty(t, relay.Vendor())
	rl.ApplyAccountUsageState(context.Background(), relay, "gemini-2.5-pro")
	require.Len(t, repo.scopes, 1, "中转 key 没有官方档位配额")
}

type geminiLocalQuotaRepoStub struct {
	rateLimitAccountRepoStub
	scopes      []string
	lastResetAt time.Time
}

func (r *geminiLocalQuotaRepoStub) SetModelRateLimit(_ context.Context, _ int64, scope string, resetAt time.Time, _ ...string) error {
	r.scopes = append(r.scopes, scope)
	r.lastResetAt = resetAt
	return nil
}

func TestApplyAccountUsageState_QuotaCounterReloadsAndPauses(t *testing.T) {
	rl, repo := quotaStateTestService(t)
	stale := &Account{ID: 3009, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
		Extra: map[string]any{"quota_limit": 1.0, "quota_used": 0.5}}
	fresh := *stale
	fresh.Extra = map[string]any{"quota_limit": 1.0, "quota_used": 1.0}
	repo.accountsByID = map[int64]*Account{3009: &fresh}

	rl.ApplyAccountUsageState(context.Background(), stale, "claude-sonnet-4-5")

	require.Equal(t, 1, repo.tempCalls, "按重读后的计数停调")
	payload, ok := parseTempUnschedReasonPayload(repo.lastTempReason)
	require.True(t, ok)
	require.Equal(t, quotaCounterSource, payload.Source)
}

// 两个网关的用量入账末尾都是状态写入点：入账后账号的配额计数（重读后）用满即停调。
func TestRecordUsage_AppliesAccountUsageState(t *testing.T) {
	account := func() *Account {
		return &Account{ID: 3, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
			Extra: map[string]any{"quota_limit": 1.0, "quota_used": 0.5}}
	}
	newRateLimit := func() (*RateLimitService, *rateLimitAccountRepoStub) {
		rl, repo := quotaStateTestService(t)
		fresh := account()
		fresh.Extra = map[string]any{"quota_limit": 1.0, "quota_used": 1.0}
		repo.accountsByID = map[int64]*Account{3: fresh}
		return rl, repo
	}

	t.Run("anthropic gateway", func(t *testing.T) {
		rl, repo := newRateLimit()
		svc := newGatewayRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{inserted: true}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
		svc.rateLimitService = rl
		err := svc.RecordUsage(context.Background(), &RecordUsageInput{
			Result:  &ForwardResult{RequestID: "usage-state-gw", Model: "claude-sonnet-4-5", Usage: ClaudeUsage{InputTokens: 100, OutputTokens: 50}, Duration: time.Second},
			APIKey:  &APIKey{ID: 1},
			User:    &User{ID: 2, RateMultiplier: customRate(1)},
			Account: account(),
		})
		require.NoError(t, err)
		require.Equal(t, 1, repo.tempCalls, "入账后配额用满 → 停调")
	})

	t.Run("openai gateway", func(t *testing.T) {
		rl, repo := newRateLimit()
		svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{inserted: true}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
		svc.rateLimitService = rl
		err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
			Result:  &OpenAIForwardResult{RequestID: "usage-state-oa", Model: "gpt-5.1", Usage: OpenAIUsage{InputTokens: 100, OutputTokens: 50}, Duration: time.Second},
			APIKey:  &APIKey{ID: 1},
			User:    &User{ID: 2, RateMultiplier: customRate(1)},
			Account: account(),
		})
		require.NoError(t, err)
		require.Equal(t, 1, repo.tempCalls, "入账后配额用满 → 停调")
	})
}
