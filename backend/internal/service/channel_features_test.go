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

// 端到端：渠道额度提醒写死「设了限额 → 剩余降到 20%（用到 80%）时提醒一次」（2026-09-28 P5），
// 渠道上不再配提醒开关与阈值；库里旧行留着的 quota_notify_*_enabled=false 也不再挡住提醒。
func TestChannelFeatures_QuotaAlertFiresAtEightyPercentWithoutChannelSettings(t *testing.T) {
	srv, port := startFakeSMTPServer(t, false, false)
	cfg := &config.Config{SMTP: config.SMTPConfig{Host: "127.0.0.1", Port: port, From: "noreply@example.com"}}
	svc, _ := newBalanceNotifyServiceWith(cfg, firstAdminStub{user: &User{Email: "admin@example.com"}})
	account := &Account{
		ID: 4201, Name: "quota-key", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Extra: map[string]any{"quota_notify_daily_enabled": false},
	}
	ctx := context.Background()

	// 78 → 79.9：还没到 80，不发
	svc.CheckAccountQuotaAfterIncrement(ctx, account, 1.9, &AccountQuotaState{DailyUsed: 79.9, DailyLimit: 100})
	// 没设限额的维度不提醒
	svc.CheckAccountQuotaAfterIncrement(ctx, account, 50, &AccountQuotaState{TotalUsed: 500})
	time.Sleep(200 * time.Millisecond)
	require.Zero(t, srv.conns.Load(), "没越过 80% 或没设限额时不应发信")

	// 79.9 → 80.4：越过 80，发一封给管理员
	svc.CheckAccountQuotaAfterIncrement(ctx, account, 0.5, &AccountQuotaState{DailyUsed: 80.4, DailyLimit: 100})
	require.Eventually(t, func() bool {
		return srv.sawCommand("RCPT TO:<ADMIN@EXAMPLE.COM>")
	}, 5*time.Second, 10*time.Millisecond, "越过 80% 时应给管理员发提醒")
	require.Equal(t, int64(1), srv.conns.Load())

	// 已经在 80 以上继续用：不重复提醒
	svc.CheckAccountQuotaAfterIncrement(ctx, account, 1, &AccountQuotaState{DailyUsed: 81.4, DailyLimit: 100})
	time.Sleep(200 * time.Millisecond)
	require.Equal(t, int64(1), srv.conns.Load(), "只在越线那一次提醒")
}

// 端到端：上游请求标识按固定头名表自动取（2026-09-28 P5），渠道上不再配头名；
// 用量记录里 upstream_request_id 取第一个非空的表内响应头。
func TestChannelFeatures_RecordUsageTakesUpstreamRequestIDFromFixedHeaders(t *testing.T) {
	t.Run("gateway", func(t *testing.T) {
		usageRepo := &openAIRecordUsageLogRepoStub{}
		billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
		svc := newGatewayRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
		headers := http.Header{}
		headers.Set("X-Oneapi-Request-Id", "oneapi-req-1")
		headers.Set("X-Client-Request-ID", "sub2api-client-1")

		err := svc.RecordUsage(context.Background(), &RecordUsageInput{
			Result: &ForwardResult{
				RequestID:       "resp-1",
				Usage:           ClaudeUsage{InputTokens: 10, OutputTokens: 6},
				Model:           "claude-sonnet-4",
				Duration:        time.Second,
				UpstreamHeaders: headers,
			},
			APIKey:  &APIKey{ID: 4301},
			User:    &User{ID: 4401, RateMultiplier: 1.1},
			Account: &Account{ID: 4501, Platform: PlatformAnthropic, Type: AccountTypeAPIKey},
		})

		require.NoError(t, err)
		require.NotNil(t, usageRepo.lastLog)
		require.NotNil(t, usageRepo.lastLog.UpstreamRequestID)
		require.Equal(t, "oneapi-req-1", *usageRepo.lastLog.UpstreamRequestID)
	})

	t.Run("openai", func(t *testing.T) {
		usageRepo := &openAIRecordUsageLogRepoStub{}
		billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
		svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
		headers := http.Header{}
		headers.Set("X-Request-Id", "req_openai_1")

		err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
			Result: &OpenAIForwardResult{
				RequestID:       "resp-2",
				Usage:           OpenAIUsage{InputTokens: 8, OutputTokens: 4},
				Model:           "gpt-5.1",
				Duration:        time.Second,
				UpstreamHeaders: headers,
			},
			APIKey:  &APIKey{ID: 4302},
			User:    &User{ID: 4402, RateMultiplier: 1.1},
			Account: &Account{ID: 4502, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		})

		require.NoError(t, err)
		require.NotNil(t, usageRepo.lastLog)
		require.NotNil(t, usageRepo.lastLog.UpstreamRequestID)
		require.Equal(t, "req_openai_1", *usageRepo.lastLog.UpstreamRequestID)
	})
}
