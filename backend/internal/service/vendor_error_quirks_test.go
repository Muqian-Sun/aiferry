//go:build unit

package service

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

// 第三方 key 的错误语义按 Vendor（协议地址）判定，平台标签只用于展示。
// 夹具工厂 vendorTestKey 与 vendorTestRelayURL 定义在 account_vendor_model_mapping_test.go；
// 每个用例先断言夹具的 Vendor，保证「中转 / 官方」前提真的成立。

var (
	vendorTestRelayChat      = map[string]string{APIProtocolChatCompletions: vendorTestRelayURL}
	vendorTestRelayAnthropic = map[string]string{APIProtocolAnthropic: "https://relay.example.com"}
	vendorTestRelayGemini    = map[string]string{APIProtocolGemini: "https://relay.example.com"}
	vendorTestMoonshot       = map[string]string{APIProtocolChatCompletions: "https://api.moonshot.cn/v1"}
	vendorTestAnthropic      = map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"}
	vendorTestOpenAI         = map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"}
	vendorTestXAI            = map[string]string{APIProtocolChatCompletions: "https://api.x.ai/v1", APIProtocolResponses: "https://api.x.ai/v1"}
	vendorTestGemini         = map[string]string{APIProtocolGemini: "https://generativelanguage.googleapis.com"}
)

func TestHandleUpstreamError_402RecoverablePauseFollowsVendor(t *testing.T) {
	body := []byte(`{"error":{"message":"insufficient balance"}}`)

	t.Run("kimi label on relay is permanently disabled", func(t *testing.T) {
		repo := &rateLimitAccountRepoStub{}
		svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		account := vendorTestKey(PlatformKimi, vendorTestRelayChat)
		require.Empty(t, account.Vendor())

		require.True(t, svc.HandleUpstreamError(context.Background(), account, http.StatusPaymentRequired, http.Header{}, body))
		require.Equal(t, 1, repo.setErrorCalls)
		require.Zero(t, repo.tempCalls)
	})

	t.Run("openai label on api.moonshot.cn gets the recoverable pause", func(t *testing.T) {
		repo := &rateLimitAccountRepoStub{}
		svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		account := vendorTestKey(PlatformOpenAI, vendorTestMoonshot)
		require.Equal(t, PlatformKimi, account.Vendor())

		require.True(t, svc.HandleUpstreamError(context.Background(), account, http.StatusPaymentRequired, http.Header{}, body))
		require.Zero(t, repo.setErrorCalls)
		require.Equal(t, 1, repo.tempCalls)
		require.True(t, strings.HasPrefix(repo.lastTempReason, cnBalanceLowReasonPrefix), repo.lastTempReason)
		// balance_low 标记的键前缀按厂商写：余额探测按同一前缀清除，按标签写会永远清不掉。
		require.Equal(t, map[string]any{cnExtraKey(PlatformKimi, cnBalanceExtraSuffixLow): true}, repo.lastExtraUpdates)
		require.NotContains(t, repo.lastExtraUpdates, cnExtraKey(PlatformOpenAI, cnBalanceExtraSuffixLow))
	})

	// OpenCode：Zen 按量有余额概念（可恢复暂停），Go 订阅没有（永久停用）；两者按地址区分，
	// 标签与 credentials.account_mode 都不参与。
	t.Run("openai label on opencode zen address gets the recoverable pause", func(t *testing.T) {
		repo := &rateLimitAccountRepoStub{}
		svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		account := vendorTestKey(PlatformOpenAI, map[string]string{APIProtocolChatCompletions: DefaultOpenCodeZenBaseURL})
		require.Equal(t, PlatformOpenCodeGo, account.Vendor())

		require.True(t, svc.HandleUpstreamError(context.Background(), account, http.StatusPaymentRequired, http.Header{}, body))
		require.Zero(t, repo.setErrorCalls)
		require.Equal(t, 1, repo.tempCalls)
		require.True(t, strings.HasPrefix(repo.lastTempReason, cnBalanceLowReasonPrefix), repo.lastTempReason)
	})

	t.Run("opencode label with zen account_mode on go address is permanently disabled", func(t *testing.T) {
		repo := &rateLimitAccountRepoStub{}
		svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		account := vendorTestKey(PlatformOpenCodeGo, map[string]string{APIProtocolChatCompletions: DefaultOpenCodeGoBaseURL, APIProtocolAnthropic: DefaultOpenCodeGoAnthropicBaseURL})
		account.Credentials = map[string]any{"account_mode": AccountModeZen}
		require.Equal(t, PlatformOpenCodeGo, account.Vendor())

		require.True(t, svc.HandleUpstreamError(context.Background(), account, http.StatusPaymentRequired, http.Header{}, body))
		require.Equal(t, 1, repo.setErrorCalls)
		require.Zero(t, repo.tempCalls)
	})
}

func TestHandleUpstreamError_CreditBalance400FollowsVendor(t *testing.T) {
	body := []byte(`{"type":"error","error":{"type":"invalid_request_error","message":"Your credit balance is too low to access the Anthropic API."}}`)

	t.Run("anthropic label on relay is not disabled", func(t *testing.T) {
		repo := &rateLimitAccountRepoStub{}
		svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		account := vendorTestKey(PlatformAnthropic, vendorTestRelayAnthropic)
		require.Empty(t, account.Vendor())

		require.False(t, svc.HandleUpstreamError(context.Background(), account, http.StatusBadRequest, http.Header{}, body))
		require.Zero(t, repo.setErrorCalls)
	})

	t.Run("openai label on api.anthropic.com is disabled", func(t *testing.T) {
		repo := &rateLimitAccountRepoStub{}
		svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		account := vendorTestKey(PlatformOpenAI, vendorTestAnthropic)
		require.Equal(t, PlatformAnthropic, account.Vendor())

		require.True(t, svc.HandleUpstreamError(context.Background(), account, http.StatusBadRequest, http.Header{}, body))
		require.Equal(t, 1, repo.setErrorCalls)
	})
}

// requireShortFallbackReset 断言限流只走了秒级通用兜底，没有被厂商窗口拉长。
func requireShortFallbackReset(t *testing.T, before, reset time.Time) {
	t.Helper()
	require.True(t, reset.After(before), "reset %v must be after %v", reset, before)
	require.True(t, reset.Before(before.Add(time.Minute)), "reset %v must stay a short fallback", reset)
}

func TestHandleUpstreamError_Anthropic429WindowHeadersFollowVendor(t *testing.T) {
	const rateLimitBody = `{"type":"error","error":{"type":"rate_limit_error","message":"rate limited"}}`

	t.Run("5h window exhausted", func(t *testing.T) {
		resetAt := time.Now().Add(3 * time.Hour).Truncate(time.Second)
		headers := http.Header{}
		headers.Set("anthropic-ratelimit-unified-5h-utilization", "1.02")
		headers.Set("anthropic-ratelimit-unified-5h-reset", strconv.FormatInt(resetAt.Unix(), 10))

		relayRepo := &anthropicWindowLimitRepo{}
		relay := vendorTestKey(PlatformAnthropic, vendorTestRelayAnthropic)
		require.Empty(t, relay.Vendor())
		before := time.Now()
		NewRateLimitService(relayRepo, nil, nil, nil, nil).HandleUpstreamError(context.Background(), relay, http.StatusTooManyRequests, headers, []byte(rateLimitBody))
		require.Equal(t, 1, relayRepo.rateLimitCalls)
		requireShortFallbackReset(t, before, relayRepo.lastRateLimitReset)
		require.Zero(t, relayRepo.sessionWindowCalls)

		officialRepo := &anthropicWindowLimitRepo{}
		official := vendorTestKey(PlatformOpenAI, vendorTestAnthropic)
		require.Equal(t, PlatformAnthropic, official.Vendor())
		NewRateLimitService(officialRepo, nil, nil, nil, nil).HandleUpstreamError(context.Background(), official, http.StatusTooManyRequests, headers, []byte(rateLimitBody))
		require.Equal(t, 1, officialRepo.rateLimitCalls)
		require.Equal(t, resetAt, officialRepo.lastRateLimitReset)
	})

	t.Run("fable 7d_oi window exhausted", func(t *testing.T) {
		now := time.Now()
		headers := fable429Headers(now.Add(2*time.Hour).Truncate(time.Second), now.Add(96*time.Hour).Truncate(time.Second))

		relayRepo := &anthropicWindowLimitRepo{}
		relay := vendorTestKey(PlatformAnthropic, vendorTestRelayAnthropic)
		require.Empty(t, relay.Vendor())
		before := time.Now()
		NewRateLimitService(relayRepo, nil, nil, nil, nil).HandleUpstreamError(context.Background(), relay, http.StatusTooManyRequests, headers, []byte(rateLimitBody))
		require.Zero(t, relayRepo.modelRateLimitCalls)
		require.Equal(t, 1, relayRepo.rateLimitCalls)
		requireShortFallbackReset(t, before, relayRepo.lastRateLimitReset)

		officialRepo := &anthropicWindowLimitRepo{}
		official := vendorTestKey(PlatformOpenAI, vendorTestAnthropic)
		NewRateLimitService(officialRepo, nil, nil, nil, nil).HandleUpstreamError(context.Background(), official, http.StatusTooManyRequests, headers, []byte(rateLimitBody))
		require.Equal(t, 1, officialRepo.modelRateLimitCalls)
		require.Equal(t, anthropicFableRateLimitKey, officialRepo.lastModelRateLimitScope)
		require.Zero(t, officialRepo.rateLimitCalls)
	})

	t.Run("aggregated unified reset only", func(t *testing.T) {
		resetAt := time.Now().Add(2 * time.Hour).Truncate(time.Second)
		headers := http.Header{}
		headers.Set("anthropic-ratelimit-unified-reset", strconv.FormatInt(resetAt.Unix(), 10))

		relayRepo := &anthropicWindowLimitRepo{}
		relay := vendorTestKey(PlatformAnthropic, vendorTestRelayAnthropic)
		before := time.Now()
		NewRateLimitService(relayRepo, nil, nil, nil, nil).HandleUpstreamError(context.Background(), relay, http.StatusTooManyRequests, headers, []byte(rateLimitBody))
		require.Equal(t, 1, relayRepo.rateLimitCalls)
		requireShortFallbackReset(t, before, relayRepo.lastRateLimitReset)

		officialRepo := &anthropicWindowLimitRepo{}
		official := vendorTestKey(PlatformOpenAI, vendorTestAnthropic)
		NewRateLimitService(officialRepo, nil, nil, nil, nil).HandleUpstreamError(context.Background(), official, http.StatusTooManyRequests, headers, []byte(rateLimitBody))
		require.Equal(t, 1, officialRepo.rateLimitCalls)
		require.Equal(t, resetAt, officialRepo.lastRateLimitReset)
	})
}

func TestHandle429_OpenAICodexSignalsFollowVendor(t *testing.T) {
	t.Run("x-codex headers", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("x-codex-primary-used-percent", "100")
		headers.Set("x-codex-primary-reset-after-seconds", "7200")
		headers.Set("x-codex-primary-window-minutes", "10080")
		body := []byte(`{"error":{"type":"rate_limit_exceeded","message":"rate limited"}}`)

		relayRepo := &anthropicWindowLimitRepo{}
		relay := vendorTestKey(PlatformOpenAI, vendorTestRelayChat)
		require.Empty(t, relay.Vendor())
		before := time.Now()
		NewRateLimitService(relayRepo, nil, nil, nil, nil).handle429(context.Background(), relay, headers, body)
		require.Equal(t, 1, relayRepo.rateLimitCalls)
		requireShortFallbackReset(t, before, relayRepo.lastRateLimitReset)
		require.Nil(t, relayRepo.lastExtraUpdates, "relay keys must not persist codex usage snapshots")

		officialRepo := &anthropicWindowLimitRepo{}
		official := vendorTestKey(PlatformKimi, vendorTestOpenAI)
		require.Equal(t, PlatformOpenAI, official.Vendor())
		before = time.Now()
		NewRateLimitService(officialRepo, nil, nil, nil, nil).handle429(context.Background(), official, headers, body)
		require.Equal(t, 1, officialRepo.rateLimitCalls)
		require.WithinDuration(t, before.Add(7200*time.Second), officialRepo.lastRateLimitReset, 5*time.Second)
		require.Contains(t, officialRepo.lastExtraUpdates, "codex_usage_updated_at")
	})

	t.Run("usage_limit_reached body", func(t *testing.T) {
		resetAt := time.Now().Add(90 * time.Minute).Unix()
		body := []byte(`{"error":{"type":"usage_limit_reached","message":"The usage limit has been reached","resets_at":` + strconv.FormatInt(resetAt, 10) + `}}`)

		relayRepo := &anthropicWindowLimitRepo{}
		before := time.Now()
		NewRateLimitService(relayRepo, nil, nil, nil, nil).handle429(context.Background(), vendorTestKey(PlatformOpenAI, vendorTestRelayChat), http.Header{}, body)
		require.Equal(t, 1, relayRepo.rateLimitCalls)
		requireShortFallbackReset(t, before, relayRepo.lastRateLimitReset)

		officialRepo := &anthropicWindowLimitRepo{}
		NewRateLimitService(officialRepo, nil, nil, nil, nil).handle429(context.Background(), vendorTestKey(PlatformKimi, vendorTestOpenAI), http.Header{}, body)
		require.Equal(t, 1, officialRepo.rateLimitCalls)
		require.Equal(t, time.Unix(resetAt, 0), officialRepo.lastRateLimitReset)
	})
}

func TestHandle429_CNProviderReactivePathFollowsVendor(t *testing.T) {
	body := []byte(`{"error":{"message":"insufficient balance, please recharge"}}`)

	relayRepo := &anthropicWindowLimitRepo{}
	relay := vendorTestKey(PlatformKimi, vendorTestRelayChat)
	require.Empty(t, relay.Vendor())
	NewRateLimitService(relayRepo, nil, nil, nil, nil).handle429(context.Background(), relay, http.Header{}, body)
	require.Zero(t, relayRepo.tempUnschedCalls)
	require.Equal(t, 1, relayRepo.rateLimitCalls)

	officialRepo := &anthropicWindowLimitRepo{}
	official := vendorTestKey(PlatformOpenAI, vendorTestMoonshot)
	require.Equal(t, PlatformKimi, official.Vendor())
	NewRateLimitService(officialRepo, nil, nil, nil, nil).handle429(context.Background(), official, http.Header{}, body)
	require.Equal(t, 1, officialRepo.tempUnschedCalls)
	require.Zero(t, officialRepo.rateLimitCalls)
}

func TestHandleGeminiUpstreamError_PSTMidnightCooldownFollowsVendor(t *testing.T) {
	// 没有 quotaResetDelay、也不含 "per day"，解析不出重置时间。
	body := []byte(`{"error":{"code":429,"message":"Resource has been exhausted (e.g. check quota).","status":"RESOURCE_EXHAUSTED"}}`)
	require.Nil(t, ParseGeminiRateLimitResetTime(body))

	newSvc := func(repo *rateLimit429AccountRepoStub) *GeminiMessagesCompatService {
		return &GeminiMessagesCompatService{accountRepo: repo, rateLimitService: NewRateLimitService(repo, nil, &config.Config{}, nil, nil)}
	}

	relayRepo := &rateLimit429AccountRepoStub{}
	relay := vendorTestKey(PlatformGemini, vendorTestRelayGemini)
	require.Empty(t, relay.Vendor())
	before := time.Now()
	newSvc(relayRepo).handleGeminiUpstreamError(context.Background(), relay, http.StatusTooManyRequests, http.Header{}, body)
	require.Equal(t, 1, relayRepo.rateLimitCalls)
	requireShortFallbackReset(t, before, relayRepo.lastRateLimitReset)

	officialRepo := &rateLimit429AccountRepoStub{}
	official := vendorTestKey(PlatformOpenAI, vendorTestGemini)
	require.Equal(t, PlatformGemini, official.Vendor())
	newSvc(officialRepo).handleGeminiUpstreamError(context.Background(), official, http.StatusTooManyRequests, http.Header{}, body)
	require.Equal(t, 1, officialRepo.rateLimitCalls)
	require.WithinDuration(t, geminiDailyResetTime(time.Now()), officialRepo.lastRateLimitReset, 2*time.Second)
}

func TestGeminiQuotaForAccount_FollowsVendor(t *testing.T) {
	quotaSvc := NewGeminiQuotaService(&config.Config{}, nil)

	relay := vendorTestKey(PlatformGemini, vendorTestRelayGemini)
	require.Empty(t, relay.Vendor())
	_, ok := quotaSvc.QuotaForAccount(context.Background(), relay)
	require.False(t, ok)
	require.Empty(t, geminiQuotaTierKeyForAccount(relay))

	official := vendorTestKey(PlatformOpenAI, vendorTestGemini)
	require.Equal(t, PlatformGemini, official.Vendor())
	_, ok = quotaSvc.QuotaForAccount(context.Background(), official)
	require.True(t, ok)
	require.Equal(t, GeminiTierAIStudioFree, geminiQuotaTierKeyForAccount(official))
}

type geminiPrecheckUsageRepoStub struct {
	usageBatchLogRepoStub
	stats []usagestats.ModelStat
}

func (r *geminiPrecheckUsageRepoStub) GetModelStatsWithFilters(context.Context, time.Time, time.Time, int64, int64, int64, int64, *int16, *bool, *int8) ([]usagestats.ModelStat, error) {
	return r.stats, nil
}

func TestHandle403_EscalatingPolicyFollowsVendor(t *testing.T) {
	const structured403 = `{"error":{"type":"permission_error","message":"forbidden"}}`

	for _, label := range []string{PlatformAnthropic, PlatformAntigravity} {
		t.Run(label+" label on relay uses the escalating policy", func(t *testing.T) {
			h := newOpenAI403TestHarness(t, 9201, 1)
			h.account = vendorTestKey(label, vendorTestRelayAnthropic)
			require.Empty(t, h.account.Vendor())

			require.True(t, h.handle(structured403))
			require.Zero(t, h.repo.setErrorCalls)
			require.Equal(t, 1, h.repo.tempCalls)
			require.Equal(t, 1, h.counter.increments)
		})
	}

	t.Run("official anthropic key keeps the immediate disable", func(t *testing.T) {
		h := newOpenAI403TestHarness(t, 9202, 1)
		h.account = vendorTestKey(PlatformOpenAI, vendorTestAnthropic)
		require.Equal(t, PlatformAnthropic, h.account.Vendor())

		require.True(t, h.handle(structured403))
		require.Equal(t, 1, h.repo.setErrorCalls)
		require.Zero(t, h.repo.tempCalls)
		require.Zero(t, h.counter.increments)
	})

	t.Run("kimi concurrency message is recognised by vendor", func(t *testing.T) {
		concurrency := `{"error":{"message":"` + kimiConcurrentRequestLimitMessage + `"}}`

		official := newOpenAI403TestHarness(t, 9203, 1)
		official.account = vendorTestKey(PlatformOpenAI, vendorTestMoonshot)
		require.Equal(t, PlatformKimi, official.account.Vendor())
		require.True(t, official.handle(concurrency))
		require.True(t, strings.HasPrefix(official.repo.lastTempReason, cnConcurrencyLimitReasonPrefix), official.repo.lastTempReason)
		require.Zero(t, official.counter.increments)

		relay := newOpenAI403TestHarness(t, 9204, 1)
		relay.account = vendorTestKey(PlatformKimi, vendorTestRelayChat)
		require.Empty(t, relay.account.Vendor())
		require.True(t, relay.handle(concurrency))
		require.False(t, strings.HasPrefix(relay.repo.lastTempReason, cnConcurrencyLimitReasonPrefix), relay.repo.lastTempReason)
		require.Equal(t, 1, relay.counter.increments)
	})
}

func TestTryTempUnschedulable_401EscalationAppliesToAntigravityLabelledKeys(t *testing.T) {
	credentials := map[string]any{
		"temp_unschedulable_enabled": true,
		"temp_unschedulable_rules": []any{
			map[string]any{"error_code": float64(http.StatusUnauthorized), "keywords": []any{"unauthorized"}, "duration_minutes": float64(10)},
		},
	}
	previous401 := `{"status_code":401}`
	body := []byte(`{"error":{"message":"unauthorized"}}`)

	subscription := &Account{ID: 9301, Platform: PlatformAntigravity, Type: AccountTypeOAuth, Credentials: credentials, TempUnschedulableReason: previous401}
	require.True(t, NewRateLimitService(&rateLimitAccountRepoStub{}, nil, &config.Config{}, nil, nil).tryTempUnschedulable(context.Background(), subscription, http.StatusUnauthorized, body),
		"fixture: antigravity subscriptions skip the 401 escalation and re-apply the rule")

	key := vendorTestKey(PlatformAntigravity, vendorTestRelayAnthropic)
	key.Credentials = credentials
	key.TempUnschedulableReason = previous401
	require.Empty(t, key.Vendor())
	require.False(t, NewRateLimitService(&rateLimitAccountRepoStub{}, nil, &config.Config{}, nil, nil).tryTempUnschedulable(context.Background(), key, http.StatusUnauthorized, body))
}

func TestHandleUpstreamModelNotFound_AntigravityLabelledKeyUsesPlainMappedModel(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxkey.ThinkingEnabled, true)
	repo := &modelNotFoundAccountRepoStub{}
	svc := &RateLimitService{accountRepo: repo}
	key := vendorTestKey(PlatformAntigravity, vendorTestRelayAnthropic)
	key.Credentials = map[string]any{"model_mapping": map[string]any{"claude-sonnet-4-5": "claude-sonnet-4-5"}}
	require.Empty(t, key.Vendor())

	require.True(t, svc.HandleUpstreamModelNotFound(ctx, key, "claude-sonnet-4-5", http.StatusNotFound, []byte(`{"error":{"code":"model_not_found","message":"model not found"}}`)))
	require.Len(t, repo.modelRateLimitCalls, 1)
	require.Equal(t, "claude-sonnet-4-5", repo.modelRateLimitCalls[0].scope)
}

func TestHandleOpenAIImageErrors_FollowOpenAIOrRelayVendor(t *testing.T) {
	rateLimitBody := []byte(`{"error":{"message":"Rate limit reached for gpt-image-2 in organization org-x on images per min."}}`)
	capabilityBody := []byte(`{"error":{"message":"Tool choice 'image_generation' not found in 'tools' parameter."}}`)

	relayRepo := &modelNotFoundAccountRepoStub{}
	relaySvc := &RateLimitService{accountRepo: relayRepo}
	relay := vendorTestKey(PlatformKimi, vendorTestRelayChat)
	require.Empty(t, relay.Vendor())
	require.True(t, relaySvc.HandleOpenAIImageRateLimit(context.Background(), relay, http.StatusTooManyRequests, http.Header{}, rateLimitBody))
	require.True(t, relaySvc.HandleOpenAIImageCapabilityLoss(context.Background(), relay, http.StatusBadRequest, capabilityBody))
	require.Len(t, relayRepo.modelRateLimitCalls, 2)

	officialRepo := &modelNotFoundAccountRepoStub{}
	officialSvc := &RateLimitService{accountRepo: officialRepo}
	moonshot := vendorTestKey(PlatformOpenAI, vendorTestMoonshot)
	require.Equal(t, PlatformKimi, moonshot.Vendor())
	require.False(t, officialSvc.HandleOpenAIImageRateLimit(context.Background(), moonshot, http.StatusTooManyRequests, http.Header{}, rateLimitBody))
	require.False(t, officialSvc.HandleOpenAIImageCapabilityLoss(context.Background(), moonshot, http.StatusBadRequest, capabilityBody))
	require.Empty(t, officialRepo.modelRateLimitCalls)
}

func newVendorFastpathGateway() (*OpenAIGatewayService, *errorPolicyRepoStub) {
	repo := &errorPolicyRepoStub{}
	return &OpenAIGatewayService{rateLimitService: NewRateLimitService(repo, nil, &config.Config{}, nil, nil)}, repo
}

func TestOpenAIAccessStateBlock_OnlyForOfficialOpenAIVendor(t *testing.T) {
	body := []byte(`{"error":{"code":"account_deactivated","message":"This account has been deactivated."}}`)

	gateway, repo := newVendorFastpathGateway()
	relay := vendorTestKey(PlatformOpenAI, vendorTestRelayChat)
	require.Empty(t, relay.Vendor())
	require.False(t, gateway.handleOpenAIAccountUpstreamError(context.Background(), relay, http.StatusBadRequest, http.Header{}, body, "gpt-5.4"))
	require.Zero(t, repo.setErrCalls)
	require.False(t, gateway.isOpenAIAccountRuntimeBlocked(relay))

	gateway, repo = newVendorFastpathGateway()
	official := vendorTestKey(PlatformKimi, vendorTestOpenAI)
	require.Equal(t, PlatformOpenAI, official.Vendor())
	require.True(t, gateway.handleOpenAIAccountUpstreamError(context.Background(), official, http.StatusBadRequest, http.Header{}, body, "gpt-5.4"))
	require.Equal(t, 1, repo.setErrCalls)
	require.True(t, gateway.isOpenAIAccountRuntimeBlocked(official))
}

func TestOpenAITransientCooldown_OnlyForOpenAIOrRelayVendorKeys(t *testing.T) {
	body := []byte(`{"error":{"message":"upstream timeout"}}`)
	hit := func(gateway *OpenAIGatewayService, account *Account) {
		for i := 0; i < 2; i++ {
			require.False(t, gateway.handleOpenAIAccountUpstreamError(context.Background(), account, http.StatusGatewayTimeout, http.Header{}, body, "gpt-5.4"))
		}
	}

	gateway, _ := newVendorFastpathGateway()
	relay := vendorTestKey(PlatformKimi, vendorTestRelayChat)
	require.Empty(t, relay.Vendor())
	hit(gateway, relay)
	require.True(t, gateway.isOpenAIAccountRequestRuntimeBlocked(relay, "gpt-5.4"))

	gateway, _ = newVendorFastpathGateway()
	moonshot := vendorTestKey(PlatformOpenAI, vendorTestMoonshot)
	require.Equal(t, PlatformKimi, moonshot.Vendor())
	hit(gateway, moonshot)
	require.False(t, gateway.isOpenAIAccountRequestRuntimeBlocked(moonshot, "gpt-5.4"))
}

// 过载与上下文超长是请求级错误：对通用中转同样只豁免、不计入账号×模型冷却。
// 两个响应体都会被瞬时冷却识别（5xx /「processing your request」），能区分豁免是否生效。
func TestOpenAIRequestScopedErrorExemptions_ApplyToRelayVendor(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
	}{
		"capacity shed":  {http.StatusInternalServerError, `{"error":{"message":"The server is overloaded. Please try again later."}}`},
		"context window": {http.StatusBadRequest, `{"error":{"code":"context_length_exceeded","message":"An error occurred while processing your request: context_length_exceeded"}}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			require.True(t, shouldCooldownOpenAITransientUpstreamError(tc.status, []byte(tc.body)), "fixture must be a transient-cooldown candidate")
			gateway, _ := newVendorFastpathGateway()
			relay := vendorTestKey(PlatformKimi, vendorTestRelayChat)
			require.Empty(t, relay.Vendor())
			for i := 0; i < 2; i++ {
				require.False(t, gateway.handleOpenAIAccountUpstreamError(context.Background(), relay, tc.status, http.Header{}, []byte(tc.body), "gpt-5.4"))
			}
			require.False(t, gateway.isOpenAIAccountRequestRuntimeBlocked(relay, "gpt-5.4"))
		})
	}
}

func TestGrokContentPolicyExemption_FollowsVendor(t *testing.T) {
	body := []byte(`{"code":"permission-denied","error":"Content violates usage guidelines. "}`)
	require.True(t, isGrokContentPolicyRejection(http.StatusForbidden, body))

	gateway, repo := newVendorFastpathGateway()
	official := vendorTestKey(PlatformOpenAI, vendorTestXAI)
	require.Equal(t, PlatformGrok, official.Vendor())
	require.False(t, gateway.handleOpenAIAccountUpstreamError(context.Background(), official, http.StatusForbidden, http.Header{}, body))
	require.Zero(t, repo.setErrCalls)

	gateway, repo = newVendorFastpathGateway()
	relay := vendorTestKey(PlatformGrok, vendorTestRelayChat)
	require.Empty(t, relay.Vendor())
	require.True(t, gateway.handleOpenAIAccountUpstreamError(context.Background(), relay, http.StatusForbidden, http.Header{}, body))
	require.Equal(t, 1, repo.setErrCalls)
}

func TestOpenAIRuntimeBlock_AppliesToKeysOfAnyLabel(t *testing.T) {
	gateway := &OpenAIGatewayService{}
	key := vendorTestKey(PlatformAnthropic, vendorTestRelayAnthropic)

	gateway.BlockAccountScheduling(key, time.Time{}, "upstream_disable")

	require.True(t, gateway.isOpenAIAccountRuntimeBlocked(key))
}

func TestCanonicalOpenAIAccountSchedulingModel_KeysUseForwardModelResolution(t *testing.T) {
	// 标签为 kimi、地址是 DeepSeek 官方：转发会剥掉 [1m]，调度模型键必须一致。
	key := vendorTestKey(PlatformKimi, map[string]string{APIProtocolChatCompletions: "https://api.deepseek.com"})
	require.Equal(t, PlatformDeepseek, key.Vendor())
	require.Equal(t, "deepseek-flash", canonicalOpenAIAccountSchedulingModel(key, "deepseek-flash[1m]"))
}
