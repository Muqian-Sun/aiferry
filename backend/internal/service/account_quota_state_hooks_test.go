//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

// 快照写入点是状态服务的钩子：写完 Extra 必须立刻评估要不要停调。每个写入点一条。

func TestApplyAccountQuotaStateAfterExtraUpdate_MergesBeforeEvaluating(t *testing.T) {
	rl, repo := quotaStateTestService(t)
	account := openAIQuotaStateAccount(2001, map[string]any{"auto_pause_5h_threshold": 0.9})

	// 内存对象还是写入前的样子（没有用量），更新里带着超限的快照。
	paused := rl.ApplyAccountQuotaStateAfterExtraUpdate(context.Background(), account, map[string]any{
		"codex_5h_used_percent": 95.0,
		"codex_5h_reset_at":     time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	})

	require.True(t, paused)
	require.Equal(t, 1, repo.tempCalls)
	require.Equal(t, 95.0, account.Extra["codex_5h_used_percent"], "更新合并进了内存对象")
}

func TestApplyAccountQuotaStateByID_ReloadsAccount(t *testing.T) {
	rl, repo := quotaStateTestService(t)
	repo.accountsByID = map[int64]*Account{
		2002: openAIQuotaStateAccount(2002, map[string]any{
			"codex_5h_used_percent": 95.0, "auto_pause_5h_threshold": 0.9,
			"codex_5h_reset_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		}),
	}

	require.True(t, rl.ApplyAccountQuotaStateByID(context.Background(), 2002))
	require.Equal(t, 1, repo.tempCalls)
	require.Equal(t, int64(2002), repo.lastTempID)
}

// Codex 响应头快照（persistOpenAICodexSnapshot）落库后评估。
func TestPersistOpenAICodexSnapshot_AppliesQuotaState(t *testing.T) {
	rl, repo := quotaStateTestService(t)
	account := openAIQuotaStateAccount(2003, map[string]any{"auto_pause_5h_threshold": 0.9})
	headers := http.Header{}
	headers.Set("x-codex-secondary-used-percent", "96")
	headers.Set("x-codex-secondary-reset-after-seconds", "3600")
	headers.Set("x-codex-secondary-window-minutes", "300")

	rl.persistOpenAICodexSnapshot(context.Background(), account, headers)

	require.Equal(t, 1, repo.updateExtraCalls)
	require.Equal(t, 1, repo.tempCalls, "快照落库后账号立刻停调")
	require.NotNil(t, account.TempUnschedulableUntil)
	require.WithinDuration(t, time.Now().Add(time.Hour), *account.TempUnschedulableUntil, 5*time.Second)
}

// Anthropic 被动用量采样（UpdateSessionWindow → samplePassiveUsageFromHeaders）落库后按阈值评估。
func TestUpdateSessionWindow_PassiveUsageAppliesQuotaState(t *testing.T) {
	setGatewayPolicyForTest(t, &accountSchedulingThresholds, map[string]int{PlatformOpenAI: 100, PlatformAnthropic: 80, PlatformGrok: 100})
	repo := &rateLimitAccountRepoStub{}
	rl := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)

	account := &Account{ID: 2004, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true}
	reset := time.Now().Add(2 * 24 * time.Hour).Unix()
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-5h-status", "allowed")
	headers.Set("anthropic-ratelimit-unified-7d-utilization", "0.9")
	headers.Set("anthropic-ratelimit-unified-7d-reset", strconv.FormatInt(reset, 10))

	rl.UpdateSessionWindow(context.Background(), account, headers)

	require.GreaterOrEqual(t, repo.updateExtraCalls, 1)
	require.Equal(t, 1, repo.tempCalls, "被动采样落库后按 anthropic 阈值停调")
	require.True(t, IsAccountSchedulingThresholdReason(repo.lastTempReason))
}

// xAI 用量快照（updateGrokUsageSnapshot）落库后评估：窗口用尽但没装限流的场景（池模式 key 不装）由状态服务兜住。
func TestUpdateGrokUsageSnapshot_AppliesQuotaState(t *testing.T) {
	repo := &grokQuotaAccountRepo{}
	svc := &OpenAIGatewayService{accountRepo: repo, rateLimitService: &RateLimitService{accountRepo: repo}}
	account := &Account{ID: 2005, Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true}
	retryAfter := 60

	svc.updateGrokUsageSnapshotWithRateLimit(context.Background(), account, &xai.QuotaSnapshot{
		StatusCode:        http.StatusTooManyRequests,
		RetryAfterSeconds: &retryAfter,
		UpdatedAt:         time.Now().UTC().Format(time.RFC3339),
	}, false)

	require.Equal(t, 1, repo.updateCalls)
	require.Equal(t, 1, repo.tempUnschedCalls, "快照落库后按 retry_after 停调")
	require.WithinDuration(t, time.Now().Add(time.Minute), repo.lastTempUnschedUntil, 5*time.Second)
}

// xAI 配额主动探测（GrokQuotaService.probeUsage）落库后评估：窗口用尽装了限流的账号不再重复写 temp_unschedulable，
// 限流装不上的场景（没有重置点的 retry_after 之外的用尽窗口）由状态服务兜住。
func TestGrokQuotaServiceProbeUsage_AppliesQuotaState(t *testing.T) {
	account := healthyGrokQuotaOAuthAccount(2006)
	repo := &grokQuotaAccountRepo{mockAccountRepoForPlatform: &mockAccountRepoForPlatform{accountsByID: map[int64]*Account{account.ID: account}}}
	headers := http.Header{}
	headers.Set("Retry-After", "90")
	upstream := &fixedResponseUpstream{status: http.StatusTooManyRequests, header: headers, body: `{"error":{"message":"rate limited"}}`}
	svc := NewGrokQuotaService(repo, nil, NewGrokTokenProvider(repo, nil), upstream, nil)
	svc.SetRateLimitService(&RateLimitService{accountRepo: repo})

	_, err := svc.probeUsage(context.Background(), account.ID)

	require.NoError(t, err)
	require.Equal(t, 1, repo.updateCalls)
	require.Equal(t, 1, repo.rateLimitedCalls, "retry_after 装成限流状态")
	require.NotNil(t, account.RateLimitResetAt, "内存对象同步了限流")
	require.Zero(t, repo.tempUnschedCalls, "已限流的账号不再重复写 temp_unschedulable")
}

// 国产供应商 Coding Plan 额度探测（CNProviderQuotaService.queryUsageForAccount）落库后按阈值评估。
func TestCNProviderQuotaQueryUsage_AppliesQuotaState(t *testing.T) {
	setGatewayPolicyForTest(t, &accountSchedulingThresholds, map[string]int{PlatformOpenAI: 100, PlatformAnthropic: 100, PlatformGrok: 100, PlatformKimi: 80})
	account := codingAccount(PlatformKimi)
	account.Schedulable = true
	account.ProtocolEndpoints = map[string]string{APIProtocolChatCompletions: "https://api.kimi.com"}
	repo := &rateLimitAccountRepoStub{}
	rl := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)

	reset := time.Now().Add(3 * time.Hour).UTC().Format(time.RFC3339)
	upstream := &fixedResponseUpstream{status: http.StatusOK, body: `{"limits":[{"detail":{"limit":100,"remaining":5,"resetTime":"` + reset + `"}}]}`}
	svc := NewCNProviderQuotaService(repo, nil, upstream, cnProbeAllowlistConfig("api.kimi.com", "api.moonshot.cn"))
	svc.SetRateLimitService(rl)

	result, err := svc.QueryUsageForAccount(context.Background(), account)

	require.NoError(t, err)
	require.True(t, result.Persisted, "探测结果落库: %+v", result)
	require.Equal(t, 1, repo.tempCalls, "kimi 5h 用量 95% ≥ 阈值 80% → 停调")
}

type fixedResponseUpstream struct {
	status int
	header http.Header
	body   string
	calls  int
}

func (u *fixedResponseUpstream) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	u.calls++
	header := u.header
	if header == nil {
		header = http.Header{}
	}
	return &http.Response{StatusCode: u.status, Header: header, Body: io.NopCloser(strings.NewReader(u.body))}, nil
}

func (u *fixedResponseUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}
