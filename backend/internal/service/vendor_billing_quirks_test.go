//go:build unit

package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// 第三方 key 的计费特例按 Vendor（协议地址）判定，平台标签只用于展示。
// 夹具定义在 account_vendor_model_mapping_test.go 与 vendor_error_quirks_test.go。

func TestFilterCNProviderBillingModelCandidates_FollowsVendor(t *testing.T) {
	svc := &OpenAIGatewayService{}
	apiKey := &APIKey{}
	candidates := []string{"claude-sonnet-4-6", "kimi-k2"}

	relay := vendorTestKey(PlatformKimi, vendorTestRelayChat)
	require.Empty(t, relay.Vendor())
	require.Equal(t, candidates, svc.filterCNProviderBillingModelCandidates(context.Background(), relay, apiKey, candidates))

	moonshot := vendorTestKey(PlatformOpenAI, vendorTestMoonshot)
	require.Equal(t, PlatformKimi, moonshot.Vendor())
	require.Equal(t, []string{"kimi-k2"}, svc.filterCNProviderBillingModelCandidates(context.Background(), moonshot, apiKey, candidates))
}

func TestOpenAIRecordUsage_Resets403CounterWithEscalatingPolicyScope(t *testing.T) {
	record := func(t *testing.T, account *Account) []int64 {
		t.Helper()
		usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
		svc := newOpenAIRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
		counter := &openAI403CounterCacheStub{}
		rls := NewRateLimitService(&rateLimitAccountRepoStub{}, nil, &config.Config{}, nil, nil)
		rls.SetOpenAI403CounterCache(counter)
		svc.rateLimitService = rls
		err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
			Result: &OpenAIForwardResult{
				RequestID: "resp_vendor_403_reset_" + strconv.FormatInt(account.ID, 10),
				Usage:     OpenAIUsage{InputTokens: 10, OutputTokens: 5},
				Model:     "gpt-5.4",
				Duration:  time.Second,
			},
			APIKey:  &APIKey{ID: 9500 + account.ID},
			User:    &User{ID: 9600, RateMultiplier: customRate(1.1)},
			Account: account,
		})
		require.NoError(t, err)
		return counter.resetCalls
	}

	relay := vendorTestKey(PlatformAnthropic, vendorTestRelayAnthropic)
	relay.ID = 1
	require.Empty(t, relay.Vendor())
	require.Equal(t, []int64{1}, record(t, relay))

	// 指向 api.anthropic.com 的 key 按中转（2026-09-29 海外四家不再有官方 key），同样清零。
	keyOnOfficialHost := vendorTestKey(PlatformOpenAI, vendorTestAnthropic)
	keyOnOfficialHost.ID = 2
	require.Empty(t, keyOnOfficialHost.Vendor())
	require.Equal(t, []int64{2}, record(t, keyOnOfficialHost))

	// Anthropic 成品号首次 403 即停用、不计数，也就不清零。
	subscription := vendorTestSubscription(PlatformAnthropic, AccountTypeSetupToken, nil)
	subscription.ID = 3
	require.Empty(t, record(t, subscription))
}
