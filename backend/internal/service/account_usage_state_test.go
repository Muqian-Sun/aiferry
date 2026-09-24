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

type windowCostCacheStub struct {
	SessionLimitCache
	cost    map[int64]float64
	setCost map[int64]float64
}

func (c *windowCostCacheStub) GetWindowCost(_ context.Context, accountID int64) (float64, bool, error) {
	cost, ok := c.cost[accountID]
	return cost, ok, nil
}

func (c *windowCostCacheStub) SetWindowCost(_ context.Context, accountID int64, cost float64) error {
	if c.setCost == nil {
		c.setCost = map[int64]float64{}
	}
	c.setCost[accountID] = cost
	if c.cost == nil {
		c.cost = map[int64]float64{}
	}
	c.cost[accountID] = cost
	return nil
}

type windowStatsRepoStub struct {
	usageBatchLogRepoStub
	stats *usagestats.AccountStats
	calls int
}

func (r *windowStatsRepoStub) GetAccountWindowStats(context.Context, int64, time.Time) (*usagestats.AccountStats, error) {
	r.calls++
	return r.stats, nil
}

func windowCostAccount(id int64, accountType string, limit float64, windowEnd time.Time) *Account {
	return &Account{
		ID: id, Platform: PlatformAnthropic, Type: accountType, Status: StatusActive, Schedulable: true,
		SessionWindowStart: ptrTime(windowEnd.Add(-5 * time.Hour)),
		SessionWindowEnd:   ptrTime(windowEnd),
		Extra:              map[string]any{"window_cost_limit": limit},
	}
}

func TestApplyAccountUsageState_WindowCostPausesUntilWindowEnd(t *testing.T) {
	rl, repo := quotaStateTestService(t)
	cache := &windowCostCacheStub{cost: map[int64]float64{3001: 0.6}}
	rl.SetSessionLimitCache(cache)
	windowEnd := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	account := windowCostAccount(3001, AccountTypeOAuth, 1.0, windowEnd)

	rl.ApplyAccountUsageState(context.Background(), account, "claude-sonnet-4-5", 0.5)

	require.Equal(t, 1.1, cache.setCost[3001], "本次费用累进缓存")
	require.Equal(t, 1, repo.tempCalls)
	require.NotNil(t, account.TempUnschedulableUntil)
	require.True(t, account.TempUnschedulableUntil.Equal(windowEnd), "停到窗口结束")
	payload, ok := parseTempUnschedReasonPayload(repo.lastTempReason)
	require.True(t, ok)
	require.Equal(t, windowCostSource, payload.Source)
}

func TestApplyAccountUsageState_WindowCostAtLimitPauses(t *testing.T) {
	rl, repo := quotaStateTestService(t)
	rl.SetSessionLimitCache(&windowCostCacheStub{cost: map[int64]float64{3010: 0.5}})
	account := windowCostAccount(3010, AccountTypeOAuth, 1.0, time.Now().Add(time.Hour))

	rl.ApplyAccountUsageState(context.Background(), account, "claude-sonnet-4-5", 0.5)

	require.Equal(t, 1, repo.tempCalls, "费用 == 阈值即停（与原 CheckWindowCostSchedulability 一致）")
}

func TestApplyAccountUsageState_WindowCostBelowLimitAllows(t *testing.T) {
	rl, repo := quotaStateTestService(t)
	rl.SetSessionLimitCache(&windowCostCacheStub{cost: map[int64]float64{3002: 0.3}})
	account := windowCostAccount(3002, AccountTypeOAuth, 1.0, time.Now().Add(time.Hour))

	rl.ApplyAccountUsageState(context.Background(), account, "claude-sonnet-4-5", 0.5)

	require.Zero(t, repo.tempCalls)
}

// 原则 4：任何设了 window_cost_limit 的资源都算，不问类型（原来只算 Anthropic OAuth / setup token）。
func TestApplyAccountUsageState_WindowCostAppliesToAnyAccountWithLimit(t *testing.T) {
	rl, repo := quotaStateTestService(t)
	rl.SetSessionLimitCache(&windowCostCacheStub{cost: map[int64]float64{3003: 2.0}})
	account := windowCostAccount(3003, AccountTypeAPIKey, 1.0, time.Now().Add(time.Hour))
	account.ProtocolEndpoints = map[string]string{APIProtocolAnthropic: "https://relay.example.com"}

	rl.ApplyAccountUsageState(context.Background(), account, "claude-sonnet-4-5", 0.1)

	require.Equal(t, 1, repo.tempCalls)
}

func TestApplyAccountUsageState_WindowCostFallsBackToUsageLogsOnCacheMiss(t *testing.T) {
	rl, repo := quotaStateTestService(t)
	usage := &windowStatsRepoStub{stats: &usagestats.AccountStats{StandardCost: 1.5}}
	rl.usageRepo = usage
	cache := &windowCostCacheStub{}
	rl.SetSessionLimitCache(cache)
	account := windowCostAccount(3004, AccountTypeOAuth, 1.0, time.Now().Add(time.Hour))

	rl.ApplyAccountUsageState(context.Background(), account, "claude-sonnet-4-5", 0.2)

	require.Equal(t, 1, usage.calls, "缓存未命中从用量日志聚合")
	require.Equal(t, 1.5, cache.setCost[3004], "聚合结果回填缓存")
	require.Equal(t, 1, repo.tempCalls)
}

func TestApplyAccountUsageState_GrokFreeQuotaPauses(t *testing.T) {
	rl, repo := quotaStateTestService(t)
	rl.cfg = grokFreeQuotaTestConfig()
	rl.usageRepo = &windowStatsRepoStub{stats: &usagestats.AccountStats{Tokens: 480_000}}
	account := &Account{ID: 3005, Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
		Credentials: map[string]any{"subscription_tier": "free"}}

	rl.ApplyAccountUsageState(context.Background(), account, "grok-4.5", 0)

	require.Equal(t, 1, repo.tempCalls)
	payload, ok := parseTempUnschedReasonPayload(repo.lastTempReason)
	require.True(t, ok)
	require.Equal(t, grokFreeQuotaSource, payload.Source)
	require.WithinDuration(t, time.Now().Add(grokFreeQuotaPauseMin), *account.TempUnschedulableUntil, 5*time.Second)

	// 非 free 档不受影响。
	paid := &Account{ID: 3006, Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
		Credentials: map[string]any{"subscription_tier": "supergrok"}}
	rl.ApplyAccountUsageState(context.Background(), paid, "grok-4.5", 0)
	require.Equal(t, 1, repo.tempCalls)
}

// Google 官方档位的本地 RPD 用满后按模型档写模型级限流：官方地址的 key 被挡，中转 key 不受影响。
func TestApplyAccountUsageState_GeminiLocalRPDSetsModelRateLimit(t *testing.T) {
	usage := &geminiPrecheckUsageRepoStub{stats: []usagestats.ModelStat{{Model: "gemini-2.5-pro", Requests: 1000}}}
	quotaSvc := NewGeminiQuotaService(&config.Config{}, nil)
	repo := &geminiLocalQuotaRepoStub{}
	rl := NewRateLimitService(repo, usage, &config.Config{}, quotaSvc, nil)

	official := vendorTestKey(PlatformOpenAI, vendorTestGemini)
	official.ID = 3007
	official.Status, official.Schedulable = StatusActive, true
	require.Equal(t, PlatformGemini, official.Vendor())

	rl.ApplyAccountUsageState(context.Background(), official, "gemini-2.5-pro", 0)

	require.Equal(t, []string{geminiLocalQuotaScope(geminiModelPro)}, repo.scopes, "只挡 pro 档")
	require.WithinDuration(t, geminiDailyResetTime(time.Now()), repo.lastResetAt, 2*time.Second)
	require.False(t, official.SchedulingAllows(context.Background(), "gemini-2.5-pro", time.Now()))
	require.True(t, official.SchedulingAllows(context.Background(), "gemini-2.5-flash", time.Now()), "flash 档不受影响")

	relay := vendorTestKey(PlatformGemini, vendorTestRelayGemini)
	relay.ID = 3008
	relay.Status, relay.Schedulable = StatusActive, true
	require.Empty(t, relay.Vendor())
	rl.ApplyAccountUsageState(context.Background(), relay, "gemini-2.5-pro", 0)
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

	rl.ApplyAccountUsageState(context.Background(), stale, "claude-sonnet-4-5", 0.5)

	require.Equal(t, 1, repo.tempCalls, "按重读后的计数停调")
	payload, ok := parseTempUnschedReasonPayload(repo.lastTempReason)
	require.True(t, ok)
	require.Equal(t, quotaCounterSource, payload.Source)
}

// 两个网关的用量入账末尾都是状态写入点：入账后账号的窗口费用超限即停调。
func TestRecordUsage_AppliesAccountUsageState(t *testing.T) {
	newRateLimit := func() (*RateLimitService, *rateLimitAccountRepoStub) {
		rl, repo := quotaStateTestService(t)
		rl.SetSessionLimitCache(&windowCostCacheStub{cost: map[int64]float64{3: 1.5}})
		return rl, repo
	}
	account := func() *Account {
		return windowCostAccount(3, AccountTypeOAuth, 1.0, time.Now().Add(time.Hour))
	}

	t.Run("anthropic gateway", func(t *testing.T) {
		rl, repo := newRateLimit()
		svc := newGatewayRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{inserted: true}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
		svc.rateLimitService = rl
		err := svc.RecordUsage(context.Background(), &RecordUsageInput{
			Result:  &ForwardResult{RequestID: "usage-state-gw", Model: "claude-sonnet-4-5", Usage: ClaudeUsage{InputTokens: 100, OutputTokens: 50}, Duration: time.Second},
			APIKey:  &APIKey{ID: 1},
			User:    &User{ID: 2, RateMultiplier: 1},
			Account: account(),
		})
		require.NoError(t, err)
		require.Equal(t, 1, repo.tempCalls, "入账后窗口费用超限 → 停调")
	})

	t.Run("openai gateway", func(t *testing.T) {
		rl, repo := newRateLimit()
		svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{inserted: true}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
		svc.rateLimitService = rl
		err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
			Result:  &OpenAIForwardResult{RequestID: "usage-state-oa", Model: "gpt-5.1", Usage: OpenAIUsage{InputTokens: 100, OutputTokens: 50}, Duration: time.Second},
			APIKey:  &APIKey{ID: 1},
			User:    &User{ID: 2, RateMultiplier: 1},
			Account: account(),
		})
		require.NoError(t, err)
		require.Equal(t, 1, repo.tempCalls, "入账后窗口费用超限 → 停调")
	})
}
