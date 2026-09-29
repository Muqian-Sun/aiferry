//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type oauth429RateLimitRepo struct {
	mockAccountRepoForGemini
	setRateLimitedCalls       int
	lastRateLimitedUntil      time.Time
	setModelRateLimitCalls    int
	lastModelRateLimitKey     string
	lastModelRateLimitedUntil time.Time
}

func (r *oauth429RateLimitRepo) SetRateLimited(_ context.Context, _ int64, until time.Time) error {
	r.setRateLimitedCalls++
	r.lastRateLimitedUntil = until
	return nil
}

func (r *oauth429RateLimitRepo) SetModelRateLimit(_ context.Context, _ int64, scope string, until time.Time, _ ...string) error {
	r.setModelRateLimitCalls++
	r.lastModelRateLimitKey = scope
	r.lastModelRateLimitedUntil = until
	return nil
}

func TestOpenAI429FastPath_KeepsOAuthAccountSchedulableDuringRetryWindow(t *testing.T) {
	repo := &oauth429RateLimitRepo{}
	rateLimits := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc := &OpenAIGatewayService{rateLimitService: rateLimits}
	rateLimits.SetAccountRuntimeBlocker(svc)
	account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	apiKeyAccount := &Account{ID: 43, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"}}
	setupTokenAccount := &Account{ID: 44, Platform: PlatformOpenAI, Type: AccountTypeSetupToken}
	grokOAuthAccount := &Account{ID: 45, Platform: PlatformGrok, Type: AccountTypeOAuth}

	shouldDisable := svc.handleOpenAIAccountUpstreamError(context.Background(), account, http.StatusTooManyRequests, http.Header{}, nil)
	apiKeyShouldDisable := svc.handleOpenAIAccountUpstreamError(context.Background(), apiKeyAccount, http.StatusTooManyRequests, http.Header{}, nil)

	require.False(t, shouldDisable)
	require.False(t, apiKeyShouldDisable)
	require.False(t, svc.isOpenAIAccountRuntimeBlocked(account))
	require.True(t, svc.isOpenAIAccountRuntimeBlocked(apiKeyAccount), "API-key 429 keeps the existing scheduler cooldown behavior")
	require.Equal(t, 1, repo.setRateLimitedCalls, "only the API-key 429 should persist a scheduler block")
	require.True(t, svc.shouldRetryOpenAIOAuth429OnSameAccount(account, http.StatusTooManyRequests, false))
	require.True(t, svc.shouldRetryOpenAIOAuth429OnSameAccount(setupTokenAccount, http.StatusTooManyRequests, false))
	require.False(t, svc.shouldRetryOpenAIOAuth429OnSameAccount(apiKeyAccount, http.StatusTooManyRequests, false))
	require.False(t, svc.shouldRetryOpenAIOAuth429OnSameAccount(grokOAuthAccount, http.StatusTooManyRequests, false))
	require.WithinDuration(t, time.Now().Add(openAIOAuth429RetryWindow), svc.openAIOAuth429RetryDeadline(account), time.Second)
	require.WithinDuration(t, time.Now().Add(openAIOAuth429RetryWindow), svc.openAIOAuth429RetryDeadline(setupTokenAccount), time.Second)
}

func TestOpenAI429FastPath_BlocksOAuthOnlyAfterRetryWindow(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 420, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	svc.openaiOAuth429RetryStartedAt.Store(account.ID, time.Now().Add(-openAIOAuth429RetryWindow-time.Second))

	svc.markOpenAIOAuth429RateLimited(context.Background(), account, http.Header{}, nil)

	require.True(t, svc.isOpenAIAccountRuntimeBlocked(account))
	require.False(t, svc.shouldRetryOpenAIOAuth429OnSameAccount(account, http.StatusTooManyRequests, false))
}

func TestOpenAI429FastPath_QuotaWindowNotExhaustedUsesFallbackCooldown(t *testing.T) {
	repo := &oauth429RateLimitRepo{}
	rateLimitService := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc := &OpenAIGatewayService{rateLimitService: rateLimitService}
	rateLimitService.SetAccountRuntimeBlocker(svc)
	account := &Account{ID: 426, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	svc.openaiOAuth429RetryStartedAt.Store(account.ID, time.Now().Add(-openAIOAuth429RetryWindow-time.Second))
	headers := http.Header{}
	headers.Set("x-codex-primary-used-percent", "37")
	headers.Set("x-codex-primary-reset-after-seconds", "604800")
	headers.Set("x-codex-primary-window-minutes", "10080")
	headers.Set("x-codex-secondary-used-percent", "20")
	headers.Set("x-codex-secondary-reset-after-seconds", "3600")
	headers.Set("x-codex-secondary-window-minutes", "300")

	before := time.Now()
	svc.handleOpenAIAccountUpstreamError(context.Background(), account, http.StatusTooManyRequests, headers, []byte(`{"detail":"Rate limit exceeded"}`))
	after := time.Now()

	// 配额没用完时 reset-after 只是窗口信息：按 429 默认回避 5 秒，不能停到 7 天窗口重置。
	require.True(t, svc.isOpenAIAccountRuntimeBlocked(account), "non-exhausted quota headers must use the 429 fallback cooldown")
	value, ok := svc.openaiAccountRuntimeBlockUntil.Load(account.ID)
	require.True(t, ok)
	blockedUntil, ok := value.(time.Time)
	require.True(t, ok)
	require.False(t, blockedUntil.Before(before.Add(5*time.Second)))
	require.False(t, blockedUntil.After(after.Add(5*time.Second)))
	require.Equal(t, 1, repo.setRateLimitedCalls, "the 429 fallback persists a scheduler cooldown")
	require.False(t, repo.lastRateLimitedUntil.Before(before.Add(5*time.Second)))
	require.False(t, repo.lastRateLimitedUntil.After(after.Add(5*time.Second)))
}

func TestOpenAI429FastPath_BlocksOAuthImmediatelyWhenSevenDayQuotaIsExhausted(t *testing.T) {
	repo := &oauth429RateLimitRepo{}
	rateLimits := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc := &OpenAIGatewayService{rateLimitService: rateLimits}
	rateLimits.SetAccountRuntimeBlocker(svc)
	account := &Account{ID: 423, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	headers := http.Header{}
	headers.Set("x-codex-primary-used-percent", "100")
	headers.Set("x-codex-primary-reset-after-seconds", "604800")
	headers.Set("x-codex-primary-window-minutes", "10080")
	headers.Set("x-codex-secondary-used-percent", "20")
	headers.Set("x-codex-secondary-reset-after-seconds", "3600")
	headers.Set("x-codex-secondary-window-minutes", "300")

	shouldDisable := svc.handleOpenAIAccountUpstreamError(context.Background(), account, http.StatusTooManyRequests, headers, []byte(`{"error":{"type":"rate_limit_error","code":"rate_limit_exceeded"}}`))

	require.False(t, shouldDisable)
	require.True(t, svc.isOpenAIAccountRuntimeBlocked(account))
	require.Equal(t, 1, repo.setRateLimitedCalls)
	require.Greater(t, time.Until(repo.lastRateLimitedUntil), 6*24*time.Hour)
	require.False(t, svc.ShouldRetryOpenAIOAuth429(account, headers, nil))
}

func TestOpenAI429FastPath_SparkQuotaOnlyBlocksSparkModel(t *testing.T) {
	repo := &oauth429RateLimitRepo{}
	rateLimits := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc := &OpenAIGatewayService{rateLimitService: rateLimits}
	rateLimits.SetAccountRuntimeBlocker(svc)
	account := &Account{ID: 425, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	headers := http.Header{}
	headers.Set("x-codex-primary-used-percent", "100")
	headers.Set("x-codex-primary-reset-after-seconds", "604800")
	headers.Set("x-codex-primary-window-minutes", "10080")
	headers.Set("x-codex-secondary-used-percent", "20")
	headers.Set("x-codex-secondary-reset-after-seconds", "3600")
	headers.Set("x-codex-secondary-window-minutes", "300")

	shouldDisable := svc.handleOpenAIAccountUpstreamError(
		context.Background(), account, http.StatusTooManyRequests, headers,
		[]byte(`{"error":{"type":"rate_limit_error","code":"rate_limit_exceeded"}}`),
		"gpt-5.3-codex-spark",
	)

	require.False(t, shouldDisable)
	require.False(t, svc.isOpenAIAccountRuntimeBlocked(account), "Spark quota must not create an account runtime block")
	require.Equal(t, 0, repo.setRateLimitedCalls, "Spark quota must not persist account-level rate limit")
	require.Equal(t, 1, repo.setModelRateLimitCalls)
	require.Equal(t, "gpt-5.3-codex-spark", repo.lastModelRateLimitKey)
	require.Greater(t, time.Until(repo.lastModelRateLimitedUntil), 6*24*time.Hour)
}

func TestOpenAI429FastPath_SparkTransient429UsesShortFallback(t *testing.T) {
	repo := &oauth429RateLimitRepo{}
	rateLimits := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc := &OpenAIGatewayService{rateLimitService: rateLimits}
	rateLimits.SetAccountRuntimeBlocker(svc)
	account := &Account{ID: 428, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	headers := http.Header{}
	headers.Set("x-codex-primary-used-percent", "37")
	headers.Set("x-codex-primary-reset-after-seconds", "604800")
	headers.Set("x-codex-primary-window-minutes", "10080")
	headers.Set("x-codex-secondary-used-percent", "20")
	headers.Set("x-codex-secondary-reset-after-seconds", "3600")
	headers.Set("x-codex-secondary-window-minutes", "300")

	shouldDisable := svc.handleOpenAIAccountUpstreamError(
		context.Background(), account, http.StatusTooManyRequests, headers,
		[]byte(`{"error":{"type":"rate_limit_error","code":"rate_limit_exceeded"}}`),
		"gpt-5.3-codex-spark",
	)

	require.False(t, shouldDisable)
	require.Equal(t, 1, repo.setModelRateLimitCalls)
	require.Less(t, time.Until(repo.lastModelRateLimitedUntil), time.Minute)
	require.Greater(t, time.Until(repo.lastModelRateLimitedUntil), time.Second)
}

func TestOpenAIStream429_SparkQuotaUsesQuotaHeaders(t *testing.T) {
	repo := &oauth429RateLimitRepo{}
	rateLimits := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc := &OpenAIGatewayService{rateLimitService: rateLimits}
	rateLimits.SetAccountRuntimeBlocker(svc)
	account := &Account{ID: 429, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	headers := http.Header{}
	headers.Set("x-codex-primary-used-percent", "100")
	headers.Set("x-codex-primary-reset-after-seconds", "604800")
	headers.Set("x-codex-primary-window-minutes", "10080")
	headers.Set("x-codex-secondary-used-percent", "20")
	headers.Set("x-codex-secondary-reset-after-seconds", "3600")
	headers.Set("x-codex-secondary-window-minutes", "300")
	payload := []byte(`{"type":"error","error":{"type":"rate_limit_error","code":"rate_limit_exceeded"}}`)

	status, shouldDisable := svc.handleOpenAIStreamTerminalAccountSideEffects(
		nil, account, payload, "quota exhausted", headers, "gpt-5.3-codex-spark",
	)

	require.Equal(t, http.StatusTooManyRequests, status)
	require.False(t, shouldDisable)
	require.Equal(t, 1, repo.setModelRateLimitCalls)
	require.Equal(t, "gpt-5.3-codex-spark", repo.lastModelRateLimitKey)
	require.Greater(t, time.Until(repo.lastModelRateLimitedUntil), 6*24*time.Hour)
	require.False(t, svc.isOpenAIAccountRuntimeBlocked(account))
}

func TestOpenAIStreamFailover_Spark429KeepsModelScope(t *testing.T) {
	repo := &oauth429RateLimitRepo{}
	rateLimits := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc := &OpenAIGatewayService{rateLimitService: rateLimits}
	rateLimits.SetAccountRuntimeBlocker(svc)
	account := &Account{ID: 432, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	headers := http.Header{}
	headers.Set("x-codex-primary-used-percent", "100")
	headers.Set("x-codex-primary-reset-after-seconds", "604800")
	headers.Set("x-codex-primary-window-minutes", "10080")
	headers.Set("x-codex-secondary-used-percent", "20")
	headers.Set("x-codex-secondary-reset-after-seconds", "3600")
	headers.Set("x-codex-secondary-window-minutes", "300")
	payload := []byte(`{"type":"error","error":{"type":"rate_limit_error","code":"rate_limit_exceeded"}}`)

	failoverErr := svc.newOpenAIStreamFailoverErrorWithModel(
		nil, account, false, "", payload, "quota exhausted", "gpt-5.3-codex-spark", headers,
	)

	require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
	require.Equal(t, 0, repo.setRateLimitedCalls)
	require.Equal(t, 1, repo.setModelRateLimitCalls)
	require.Equal(t, "gpt-5.3-codex-spark", repo.lastModelRateLimitKey)
	require.False(t, svc.isOpenAIAccountRuntimeBlocked(account))
}

func TestOpenAIWSErrorEvent_OrdinaryModelIgnoresHandshakeQuotaHeaders(t *testing.T) {
	repo := &oauth429RateLimitRepo{}
	rateLimits := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc := &OpenAIGatewayService{rateLimitService: rateLimits}
	rateLimits.SetAccountRuntimeBlocker(svc)
	account := &Account{ID: 430, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	headers := http.Header{}
	headers.Set("x-codex-primary-used-percent", "100")
	headers.Set("x-codex-primary-reset-after-seconds", "604800")
	headers.Set("x-codex-primary-window-minutes", "10080")
	payload := []byte(`{"type":"error","error":{"type":"rate_limit_error","code":"rate_limit_exceeded"}}`)

	svc.persistOpenAIWSRateLimitSignal(context.Background(), account, headers, payload, "rate_limit_exceeded", "rate_limit_error", "quota exhausted", "gpt-5.3-codex")

	require.False(t, svc.isOpenAIAccountRuntimeBlocked(account))
	require.Zero(t, repo.setRateLimitedCalls)
}

func TestOpenAIWSErrorEvent_SparkQuotaUsesHandshakeQuotaHeaders(t *testing.T) {
	repo := &oauth429RateLimitRepo{}
	rateLimits := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc := &OpenAIGatewayService{rateLimitService: rateLimits}
	rateLimits.SetAccountRuntimeBlocker(svc)
	account := &Account{ID: 431, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	headers := http.Header{}
	headers.Set("x-codex-primary-used-percent", "100")
	headers.Set("x-codex-primary-reset-after-seconds", "604800")
	headers.Set("x-codex-primary-window-minutes", "10080")
	payload := []byte(`{"type":"error","error":{"type":"rate_limit_error","code":"rate_limit_exceeded"}}`)

	svc.persistOpenAIWSRateLimitSignal(context.Background(), account, headers, payload, "rate_limit_exceeded", "rate_limit_error", "quota exhausted", "gpt-5.3-codex-spark")

	require.Equal(t, 1, repo.setModelRateLimitCalls)
	require.Equal(t, "gpt-5.3-codex-spark", repo.lastModelRateLimitKey)
	require.Greater(t, time.Until(repo.lastModelRateLimitedUntil), 6*24*time.Hour)
	require.False(t, svc.isOpenAIAccountRuntimeBlocked(account))
}

func TestOpenAI429FastPath_SparkShadowQuotaStaysModelScoped(t *testing.T) {
	repo := &oauth429RateLimitRepo{}
	rateLimits := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc := &OpenAIGatewayService{rateLimitService: rateLimits}
	rateLimits.SetAccountRuntimeBlocker(svc)
	parentID := int64(426)
	shadow := &Account{ID: 427, Platform: PlatformOpenAI, Type: AccountTypeOAuth, ParentAccountID: &parentID, QuotaDimension: QuotaDimensionSpark}
	headers := http.Header{}
	headers.Set("x-codex-primary-used-percent", "100")
	headers.Set("x-codex-primary-reset-after-seconds", "604800")
	headers.Set("x-codex-primary-window-minutes", "10080")

	shouldDisable := svc.handleOpenAIAccountUpstreamError(
		context.Background(), shadow, http.StatusTooManyRequests, headers,
		[]byte(`{"error":{"type":"rate_limit_error","code":"rate_limit_exceeded"}}`),
		"gpt-5.3-codex-spark",
	)

	require.False(t, shouldDisable)
	require.False(t, svc.isOpenAIAccountRuntimeBlocked(shadow))
	require.Equal(t, 0, repo.setRateLimitedCalls)
	require.Equal(t, 1, repo.setModelRateLimitCalls)
	require.Equal(t, "gpt-5.3-codex-spark", repo.lastModelRateLimitKey)
}

func TestOpenAI429FastPath_RetriesOAuthWhenNoQuotaSignalExists(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 424, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	headers := http.Header{"Retry-After": []string{"1"}}

	require.True(t, svc.ShouldRetryOpenAIOAuth429(account, headers, []byte(`{"error":{"type":"rate_limit_error","message":"try again"}}`)))
	require.False(t, svc.isOpenAIAccountRuntimeBlocked(account))
}

func TestOpenAIStream429IgnoresSuccessfulQuotaSnapshotHeaders(t *testing.T) {
	repo := &oauth429RateLimitRepo{}
	rateLimits := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc := &OpenAIGatewayService{rateLimitService: rateLimits}
	rateLimits.SetAccountRuntimeBlocker(svc)
	account := &Account{ID: 421, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	svc.openaiOAuth429RetryStartedAt.Store(account.ID, time.Now().Add(-openAIOAuth429RetryWindow-time.Second))
	headers := http.Header{}
	headers.Set("x-codex-primary-used-percent", "37")
	headers.Set("x-codex-primary-reset-after-seconds", "604800")
	headers.Set("x-codex-primary-window-minutes", "10080")
	payload := []byte(`{"type":"error","error":{"type":"rate_limit_error","code":"rate_limit_exceeded","message":"slow down"}}`)

	status, disabled := svc.handleOpenAIStreamTerminalAccountSideEffects(nil, account, payload, "slow down", headers)

	require.Equal(t, http.StatusTooManyRequests, status)
	require.False(t, disabled)
	require.True(t, svc.isOpenAIAccountRuntimeBlocked(account))
	value, ok := svc.openaiAccountRuntimeBlockUntil.Load(account.ID)
	require.True(t, ok)
	blockedUntil, ok := value.(time.Time)
	require.True(t, ok)
	require.Less(t, time.Until(blockedUntil), time.Minute, "stream 429 must not inherit the normal seven-day quota snapshot")
	if !repo.lastRateLimitedUntil.IsZero() {
		require.Less(t, time.Until(repo.lastRateLimitedUntil), time.Minute)
	}
}

func TestOpenAIHTTP429StillUsesQuotaResetHeaders(t *testing.T) {
	svc := &OpenAIGatewayService{rateLimitService: &RateLimitService{}}
	account := &Account{ID: 422, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	svc.openaiOAuth429RetryStartedAt.Store(account.ID, time.Now().Add(-openAIOAuth429RetryWindow-time.Second))
	headers := http.Header{}
	headers.Set("x-codex-primary-used-percent", "100")
	headers.Set("x-codex-primary-reset-after-seconds", "604800")
	headers.Set("x-codex-primary-window-minutes", "10080")

	svc.markOpenAIOAuth429RateLimited(context.Background(), account, headers, nil)

	value, ok := svc.openaiAccountRuntimeBlockUntil.Load(account.ID)
	require.True(t, ok)
	blockedUntil, ok := value.(time.Time)
	require.True(t, ok)
	require.Greater(t, time.Until(blockedUntil), 6*24*time.Hour, "real HTTP 429 must retain the upstream quota reset")
}

func TestOpenAI429RetryDelayHonorsBoundedRetryAfter(t *testing.T) {
	deadline := time.Now().Add(openAIOAuth429RetryWindow)
	require.Equal(t, openAIOAuth429RetryDelay, openAIOAuth429SameAccountRetryDelay(nil, deadline))
	require.Equal(t, openAIOAuth429MaxRetryDelay, openAIOAuth429SameAccountRetryDelay(http.Header{"Retry-After": []string{"90"}}, deadline))
}

func TestOpenAI429FastPath_OpenCodeGoUsageLimitUsesMessageResetDuration(t *testing.T) {
	repo := &rateLimit429AccountRepoStub{}
	rateLimitService := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc := &OpenAIGatewayService{rateLimitService: rateLimitService}
	rateLimitService.SetAccountRuntimeBlocker(svc)
	// OpenCode Go 的额度文案按 OpenCode 官方地址识别（海外四家不再有官方 key，api.openai.com 上的 key 按中转）。
	account := &Account{ID: 44, Platform: PlatformOpenCodeGo, Type: AccountTypeAPIKey, ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: DefaultOpenCodeGoBaseURL}}
	require.Equal(t, PlatformOpenCodeGo, account.Vendor())
	body := []byte(`{"type":"error","error":{"type":"GoUsageLimitError","message":"5-hour usage limit reached. Resets in 4hr 59min. To continue using this model now, enable usage from your available balance: https://opencode.ai/workspace/wrk_test/go"},"metadata":{"workspace":"wrk_test","limitName":"5 hour"}}`)

	before := time.Now()
	shouldDisable := svc.handleOpenAIAccountUpstreamError(
		context.Background(),
		account,
		http.StatusTooManyRequests,
		http.Header{},
		body,
	)
	after := time.Now()

	require.False(t, shouldDisable)
	require.Equal(t, 1, repo.rateLimitCalls)
	require.Equal(t, account.ID, repo.lastRateLimitID)
	expectedResetAfter := 4*time.Hour + 59*time.Minute
	require.False(t, repo.lastRateLimitReset.Before(before.Add(expectedResetAfter-time.Second)))
	require.False(t, repo.lastRateLimitReset.After(after.Add(expectedResetAfter)))
	require.True(t, svc.isOpenAIAccountRuntimeBlocked(account))
}

// TestOpenAI429FastPath_SkipsSparkShadow 外审第8轮 P1:spark 影子被选中后若 /responses 返回 429,
// 不得按 global x-codex-* 信号写内存运行时熔断(否则 spark 被冷却到 global reset、单影子场景无可用账号)。
func TestOpenAI429FastPath_SkipsSparkShadow(t *testing.T) {
	svc := &OpenAIGatewayService{}
	parentID := int64(800)
	shadow := &Account{
		ID:              801,
		Platform:        PlatformOpenAI,
		Type:            AccountTypeOAuth,
		ParentAccountID: &parentID,
		QuotaDimension:  QuotaDimensionSpark,
	}
	normal := &Account{ID: 802, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	headers := http.Header{}
	headers.Set("x-codex-primary-used-percent", "100")
	headers.Set("x-codex-primary-reset-after-seconds", "18000")
	headers.Set("x-codex-primary-window-minutes", "300")

	svc.markOpenAIOAuth429RateLimited(context.Background(), shadow, headers, nil)
	svc.markOpenAIOAuth429RateLimited(context.Background(), normal, headers, nil)

	require.False(t, svc.isOpenAIAccountRuntimeBlocked(shadow), "spark shadow must not be runtime-blocked by /responses global 429")
	require.True(t, svc.isOpenAIAccountRuntimeBlocked(normal), "normal OpenAI OAuth account with an exhausted 5h window must be paused")
}

func TestOpenAIRuntimeBlock_AppliesToOpenAIAPIKeyWhenRateLimitServiceStopsScheduling(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 44, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"}}

	svc.BlockAccountScheduling(account, time.Time{}, "custom_error_code")

	require.True(t, svc.isOpenAIAccountRuntimeBlocked(account))
}

func TestOpenAIRuntimeBlock_DoesNotApplyToOtherPlatforms(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 45, Platform: PlatformGemini, Type: AccountTypeOAuth}

	svc.BlockAccountScheduling(account, time.Time{}, "custom_error_code")

	require.False(t, svc.isOpenAIAccountRuntimeBlocked(account))
}

func TestOpenAIRuntimeBlocker_IgnoresNonOpenAIFromRateLimitService(t *testing.T) {
	gateway := &OpenAIGatewayService{}
	repo := &rateLimitAccountRepoStub{}
	rateLimitService := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	rateLimitService.SetAccountRuntimeBlocker(gateway)
	account := &Account{ID: 45, Platform: PlatformGemini, Type: AccountTypeOAuth}

	shouldDisable := rateLimitService.HandleUpstreamError(context.Background(), account, http.StatusForbidden, http.Header{}, []byte("forbidden"))

	require.True(t, shouldDisable)
	require.False(t, gateway.isOpenAIAccountRuntimeBlocked(account))
}

// 池模式重试状态码写死在代码里（channel_features.go）；这里临时把 524 放进表，验证可重试的 5xx
// 不再额外记账号×模型瞬时冷却（改表时这条守卫要跟着生效）。
func TestOpenAIPoolModeRetryable5xx_DoesNotCreateModelTransientBlock(t *testing.T) {
	setGatewayPolicyForTest(t, &poolModeRetryStatusCodes, []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests, 524})
	repo := &errorPolicyRepoStub{}
	rateLimitService := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	gateway := &OpenAIGatewayService{rateLimitService: rateLimitService}
	account := &Account{
		ID:       47,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"pool_mode": true,
		},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
	}

	for i := 0; i < 2; i++ {
		shouldDisable := gateway.handleOpenAIAccountUpstreamError(
			context.Background(),
			account,
			524,
			http.Header{},
			[]byte(`{"error":{"message":"upstream timeout"}}`),
			"gpt-5.4",
		)
		require.False(t, shouldDisable)
	}

	require.False(t, gateway.isOpenAIAccountRequestRuntimeBlocked(account, "gpt-5.4"))
}

func TestOpenAIPoolModeNonRetryable5xx_StillCreatesModelTransientBlock(t *testing.T) {
	repo := &errorPolicyRepoStub{}
	rateLimitService := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	gateway := &OpenAIGatewayService{rateLimitService: rateLimitService}
	account := &Account{
		ID:       48,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"pool_mode": true,
		},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
	}

	for i := 0; i < 2; i++ {
		shouldDisable := gateway.handleOpenAIAccountUpstreamError(
			context.Background(),
			account,
			http.StatusServiceUnavailable,
			http.Header{},
			[]byte(`{"error":{"message":"upstream unavailable"}}`),
			"gpt-5.4",
		)
		require.False(t, shouldDisable)
	}

	require.True(t, gateway.isOpenAIAccountRequestRuntimeBlocked(account, "gpt-5.4"))
}

func TestOpenAINonPoolAPIKey5xx_StillCreatesModelTransientBlock(t *testing.T) {
	repo := &errorPolicyRepoStub{}
	rateLimitService := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	gateway := &OpenAIGatewayService{rateLimitService: rateLimitService}
	account := &Account{
		ID:                49,
		Platform:          PlatformOpenAI,
		Type:              AccountTypeAPIKey,
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
	}

	for i := 0; i < 2; i++ {
		shouldDisable := gateway.handleOpenAIAccountUpstreamError(
			context.Background(),
			account,
			http.StatusGatewayTimeout,
			http.Header{},
			[]byte(`{"error":{"message":"upstream timeout"}}`),
			"gpt-5.4",
		)
		require.False(t, shouldDisable)
	}

	require.True(t, gateway.isOpenAIAccountRequestRuntimeBlocked(account, "gpt-5.4"))
}

func TestOpenAIModelNotFound_DoesNotRuntimeBlockWholeAccount(t *testing.T) {
	repo := &modelNotFoundAccountRepoStub{}
	svc := &OpenAIGatewayService{
		rateLimitService: &RateLimitService{accountRepo: repo},
	}
	account := openAIModelNotFoundAccount()

	shouldDisable := svc.handleOpenAIAccountUpstreamError(
		context.Background(),
		account,
		http.StatusNotFound,
		http.Header{},
		[]byte(`{"error":{"code":"model_not_found","message":"model not found"}}`),
		"gpt-5.4",
	)

	require.True(t, shouldDisable)
	require.False(t, svc.isOpenAIAccountRuntimeBlocked(account))
	require.Zero(t, repo.tempCalls)
	require.Len(t, repo.modelRateLimitCalls, 1)
}

// 渠道级临时不可调度规则 2026-09-28 P5 已删：库里旧行留着、能命中的 429 规则也不再把号按模型停掉，
// OAuth 429 仍走默认的同号重试。
func TestOpenAIOAuth429_LegacyTempRuleIgnoredKeepsSameAccountRetry(t *testing.T) {
	repo := &modelNotFoundAccountRepoStub{}
	svc := &OpenAIGatewayService{
		rateLimitService: &RateLimitService{accountRepo: repo},
	}
	account := openAIModelNotFoundAccount()
	account.Type = AccountTypeOAuth
	account.Credentials["temp_unschedulable_enabled"] = true
	account.Credentials["temp_unschedulable_rules"] = []any{
		map[string]any{
			"error_code":       float64(http.StatusTooManyRequests),
			"keywords":         []any{"global rate limit"},
			"duration_minutes": float64(10),
		},
	}

	shouldDisable := svc.handleOpenAIAccountUpstreamError(
		context.Background(),
		account,
		http.StatusTooManyRequests,
		http.Header{},
		[]byte(`{"error":{"message":"global rate limit"}}`),
		"gpt-5.4",
	)

	require.False(t, shouldDisable)
	require.False(t, svc.isOpenAIAccountRuntimeBlocked(account))
	require.True(t, svc.shouldRetryOpenAIOAuth429OnSameAccount(account, http.StatusTooManyRequests, false))
	require.Empty(t, repo.modelRateLimitCalls)
}

func TestOpenAIRuntimeBlock_DoesNotShortenExistingBlock(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 46, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	longUntil := time.Now().Add(10 * time.Minute)

	svc.BlockAccountScheduling(account, longUntil, "oauth_401")
	svc.BlockAccountScheduling(account, time.Time{}, "upstream_disable")

	value, ok := svc.openaiAccountRuntimeBlockUntil.Load(account.ID)
	require.True(t, ok)
	actualUntil, ok := value.(time.Time)
	require.True(t, ok)
	require.WithinDuration(t, longUntil, actualUntil, time.Second)
}

func TestOpenAIRuntimeBlock_ClearAccountSchedulingBlock(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 47, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	svc.BlockAccountScheduling(account, time.Now().Add(time.Minute), "429")
	require.True(t, svc.isOpenAIAccountRuntimeBlocked(account))

	svc.ClearAccountSchedulingBlock(account.ID)
	require.False(t, svc.isOpenAIAccountRuntimeBlocked(account))
}

func TestRuntimeBlockHonorsClearedPersistedCooldown(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 92, Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true}
	svc.BlockAccountScheduling(account, time.Now().Add(30*time.Minute), "grok payment required")
	require.False(t, svc.isOpenAIAccountRequestRuntimeBlocked(account, "grok-3"))
	require.False(t, svc.isOpenAIAccountRuntimeBlocked(account))
}

func TestRuntimeBlockConditionalClearSkipsNewerGeneration(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 94, Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true}
	firstUntil := time.Now().Add(10 * time.Minute)
	svc.BlockAccountScheduling(account, firstUntil, "stale")
	snapshot := svc.peekOpenAIAccountRuntimeBlock(account)
	require.True(t, snapshot.blocked)
	newerUntil := time.Now().Add(30 * time.Minute)
	svc.BlockAccountScheduling(account, newerUntil, "fresh")
	svc.clearOpenAIAccountRuntimeBlockIfUnchanged(account.ID, snapshot)
	require.True(t, svc.isOpenAIAccountRuntimeBlocked(account))
	require.False(t, svc.isOpenAIAccountRequestRuntimeBlocked(account, "grok-3"))
	require.False(t, svc.isOpenAIAccountRuntimeBlocked(account))
}

func TestRuntimeBlockKeepsActivePersistedCooldown(t *testing.T) {
	svc := &OpenAIGatewayService{}
	until := time.Now().Add(30 * time.Minute)
	account := &Account{
		ID:                     93,
		Platform:               PlatformGrok,
		Type:                   AccountTypeOAuth,
		Status:                 StatusActive,
		Schedulable:            true,
		TempUnschedulableUntil: &until,
	}
	svc.BlockAccountScheduling(account, until, "grok payment required")
	require.True(t, svc.isOpenAIAccountRequestRuntimeBlocked(account, "grok-3"))
	require.True(t, svc.isOpenAIAccountRuntimeBlocked(account))
}

func TestShouldStopOpenAIOAuth429Failover_AfterBoundedFullWindows(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	apiKeyAccount := &Account{ID: 43, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"}}
	var state OpenAIOAuth429FailoverState

	require.False(t, svc.ShouldStopOpenAIOAuth429Failover(account, http.StatusTooManyRequests, 1, &state))

	require.False(t, svc.ShouldStopOpenAIOAuth429Failover(account, http.StatusTooManyRequests, 1, &state))
	require.False(t, svc.ShouldStopOpenAIOAuth429Failover(account, http.StatusTooManyRequests, 2, &state))
	require.True(t, svc.ShouldStopOpenAIOAuth429Failover(account, http.StatusTooManyRequests, 3, &state))
	require.False(t, svc.ShouldStopOpenAIOAuth429Failover(apiKeyAccount, http.StatusTooManyRequests, 1, &state))
	require.False(t, svc.ShouldStopOpenAIOAuth429Failover(account, http.StatusInternalServerError, 1, &state))
	require.False(t, svc.ShouldStopOpenAIOAuth429Failover(account, http.StatusTooManyRequests, 0, &state))
}

func TestShouldStopOpenAIOAuth429Failover_TracksOneGrokFollowupAttempt(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 44, Platform: PlatformGrok, Type: AccountTypeOAuth}
	apiKeyAccount := &Account{ID: 45, Platform: PlatformGrok, Type: AccountTypeAPIKey, ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.x.ai/v1", APIProtocolResponses: "https://api.x.ai/v1"}}

	t.Run("429 then 500 stops after one followup", func(t *testing.T) {
		var state OpenAIOAuth429FailoverState
		require.False(t, svc.ShouldStopOpenAIOAuth429Failover(account, http.StatusTooManyRequests, 1, &state))
		require.True(t, svc.ShouldStopOpenAIOAuth429Failover(account, http.StatusInternalServerError, 2, &state))
	})

	t.Run("500 then 429 still allows one followup", func(t *testing.T) {
		var state OpenAIOAuth429FailoverState
		require.False(t, svc.ShouldStopOpenAIOAuth429Failover(account, http.StatusInternalServerError, 1, &state))
		require.False(t, svc.ShouldStopOpenAIOAuth429Failover(account, http.StatusTooManyRequests, 2, &state))
		require.True(t, svc.ShouldStopOpenAIOAuth429Failover(account, http.StatusBadGateway, 3, &state))
	})

	t.Run("OAuth 429 then API-key failure consumes the same followup", func(t *testing.T) {
		var state OpenAIOAuth429FailoverState
		require.False(t, svc.ShouldStopOpenAIOAuth429Failover(account, http.StatusTooManyRequests, 1, &state))
		require.True(t, svc.ShouldStopOpenAIOAuth429Failover(apiKeyAccount, http.StatusInternalServerError, 2, &state))
	})

	var state OpenAIOAuth429FailoverState
	require.False(t, svc.ShouldStopOpenAIOAuth429Failover(account, http.StatusTooManyRequests, 0, &state))
	require.False(t, svc.ShouldStopOpenAIOAuth429Failover(apiKeyAccount, http.StatusTooManyRequests, 2, &state))
}
