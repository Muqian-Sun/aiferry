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

func TestGroupBillsOpenAIFastAtStandard_FollowsOpenAIOrRelayVendor(t *testing.T) {
	apiKey := &APIKey{Group: &Group{ID: 1, Platform: PlatformOpenAI, FreeOpenAIFast: true}}

	relay := vendorTestKey(PlatformKimi, vendorTestRelayChat)
	require.Empty(t, relay.Vendor())
	require.True(t, groupBillsOpenAIFastAtStandard(apiKey, relay, "priority"))

	moonshot := vendorTestKey(PlatformOpenAI, vendorTestMoonshot)
	require.Equal(t, PlatformKimi, moonshot.Vendor())
	require.False(t, groupBillsOpenAIFastAtStandard(apiKey, moonshot, "priority"))
}

func TestOpenAILongContextBillingGate_KeysFollowStoredFlagNotLabel(t *testing.T) {
	t.Run("key of any label with the flag stored is gated by it", func(t *testing.T) {
		key := vendorTestKey(PlatformKimi, vendorTestRelayChat)
		key.Extra = map[string]any{openAILongContextBillingEnabledKey: true}
		gate := openAILongContextBillingGate(key)
		require.NotNil(t, gate)
		require.True(t, *gate)
	})

	t.Run("key without the flag has no per-account gate even on api.openai.com", func(t *testing.T) {
		key := vendorTestKey(PlatformOpenAI, vendorTestOpenAI)
		require.Equal(t, PlatformOpenAI, key.Vendor())
		require.Nil(t, openAILongContextBillingGate(key))
	})

	t.Run("openai subscription keeps the default-off gate", func(t *testing.T) {
		gate := openAILongContextBillingGate(&Account{ID: 9401, Platform: PlatformOpenAI, Type: AccountTypeOAuth})
		require.NotNil(t, gate)
		require.False(t, *gate)
	})
}

func TestFilterCNProviderBillingModelCandidates_FollowsVendor(t *testing.T) {
	svc := &OpenAIGatewayService{}
	apiKey := &APIKey{Group: &Group{ID: 1, Platform: PlatformKimi}}
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
		svc := newOpenAIRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
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
			APIKey:  openAIRecordUsageAPIKeyWithGroup(svc, 9500+account.ID, false),
			User:    &User{ID: 9600},
			Account: account,
		})
		require.NoError(t, err)
		return counter.resetCalls
	}

	relay := vendorTestKey(PlatformAnthropic, vendorTestRelayAnthropic)
	relay.ID = 1
	require.Empty(t, relay.Vendor())
	require.Equal(t, []int64{1}, record(t, relay))

	official := vendorTestKey(PlatformOpenAI, vendorTestAnthropic)
	official.ID = 2
	require.Equal(t, PlatformAnthropic, official.Vendor())
	require.Empty(t, record(t, official))
}
