//go:build unit

package service

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

func TestGrokMediaGenerationEligibility(t *testing.T) {
	weeklyUsagePercent := 12.5
	forbiddenBilling := &xai.BillingSummary{
		StatusCode:        http.StatusForbidden,
		WeeklyStatusCode:  http.StatusForbidden,
		MonthlyStatusCode: http.StatusForbidden,
	}
	weeklyAllowance := &xai.BillingSummary{
		PeriodType:       "weekly",
		UsagePercent:     &weeklyUsagePercent,
		StatusCode:       http.StatusOK,
		WeeklyStatusCode: http.StatusOK,
	}
	weeklyForbidden := &xai.BillingSummary{
		StatusCode:        http.StatusOK,
		WeeklyStatusCode:  http.StatusForbidden,
		MonthlyStatusCode: http.StatusOK,
	}
	monthlyForbidden := &xai.BillingSummary{
		StatusCode:        http.StatusOK,
		WeeklyStatusCode:  http.StatusOK,
		MonthlyStatusCode: http.StatusForbidden,
	}

	tests := []struct {
		name       string
		account    *Account
		want       bool
		wantReason string
	}{
		{name: "nil account", account: nil, want: false, wantReason: "not_grok"},
		{name: "non grok account", account: &Account{Platform: PlatformOpenAI}, want: false, wantReason: "not_grok"},
		{name: "non oauth grok account stays eligible", account: &Account{Platform: PlatformGrok, Type: AccountTypeAPIKey, ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.x.ai/v1", APIProtocolResponses: "https://api.x.ai/v1"}}, want: true, wantReason: "non_oauth"},
		// 第三方 key 按协议地址判厂商，与调度侧一致：openai 标签 + 官方 xAI 地址是 grok；grok 标签 + 中转不是。
		{name: "openai-labelled key on official xAI host is grok", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.x.ai/v1", APIProtocolResponses: "https://api.x.ai/v1"}}, want: true, wantReason: "non_oauth"},
		{name: "grok-labelled key on a relay is not grok", account: &Account{Platform: PlatformGrok, Type: AccountTypeAPIKey, ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://relay.example.test/v1"}}, want: false, wantReason: "not_grok"},
		{name: "unobserved oauth fails closed", account: &Account{Platform: PlatformGrok, Type: AccountTypeOAuth}, want: false, wantReason: "billing_unobserved"},
		{name: "inconclusive successful billing remains eligible", account: &Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Extra: map[string]any{grokBillingExtraKey: &xai.BillingSummary{StatusCode: http.StatusOK, Partial: true}}}, want: true, wantReason: "billing_inconclusive"},
		{name: "weekly paid usage is eligible without inferring from period type", account: &Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Extra: map[string]any{grokBillingExtraKey: weeklyAllowance}}, want: true, wantReason: "eligible"},
		{name: "billing forbidden is rejected", account: &Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Extra: map[string]any{grokBillingExtraKey: forbiddenBilling}}, want: false, wantReason: "billing_forbidden"},
		{name: "weekly billing forbidden is rejected after partial success", account: &Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Extra: map[string]any{grokBillingExtraKey: weeklyForbidden}}, want: false, wantReason: "billing_forbidden"},
		{name: "monthly billing forbidden is rejected after partial success", account: &Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Extra: map[string]any{grokBillingExtraKey: monthlyForbidden}}, want: false, wantReason: "billing_forbidden"},
		{name: "malformed billing observation fails closed", account: &Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Extra: map[string]any{grokBillingExtraKey: make(chan int)}}, want: false, wantReason: "billing_unobserved"},
		// 渠道级手动覆盖 2026-09-28 P5 删了：库里残留的 grok_media_eligible 不再生效，只按探测结果判断。
		{name: "legacy disable override is ignored", account: &Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Extra: map[string]any{"grok_media_eligible": false, grokBillingExtraKey: weeklyAllowance}}, want: true, wantReason: "eligible"},
		{name: "legacy enable override no longer beats forbidden probe", account: &Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Extra: map[string]any{"grok_media_eligible": true, grokBillingExtraKey: forbiddenBilling}}, want: false, wantReason: "billing_forbidden"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := tt.account.GrokMediaGenerationEligibility()
			require.Equal(t, tt.want, got)
			require.Equal(t, tt.wantReason, reason)
		})
	}
}

func TestGrokMediaCapabilityKeepsUnobservedAndInconclusiveOAuthAsCandidates(t *testing.T) {
	unobserved := &Account{Platform: PlatformGrok, Type: AccountTypeOAuth}
	eligible, reason := unobserved.GrokMediaGenerationEligibility()
	require.False(t, eligible)
	require.Equal(t, "billing_unobserved", reason)
	require.True(t, unobserved.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityGrokMediaGeneration))

	inconclusive := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{grokBillingExtraKey: &xai.BillingSummary{
			StatusCode: http.StatusOK,
			Partial:    true,
		}},
	}
	eligible, reason = inconclusive.GrokMediaGenerationEligibility()
	require.True(t, eligible)
	require.Equal(t, "billing_inconclusive", reason)
	require.True(t, inconclusive.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityGrokMediaGeneration))
}

func TestGrokMediaCapabilityFiltersOnlyGeneration(t *testing.T) {
	account := &Account{
		ID:          1,
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Extra: map[string]any{grokBillingExtraKey: &xai.BillingSummary{
			StatusCode:        http.StatusForbidden,
			WeeklyStatusCode:  http.StatusForbidden,
			MonthlyStatusCode: http.StatusForbidden,
		}},
	}

	require.True(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions))
	require.False(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityGrokMediaGeneration))
	ok, reason := SelectOptions{Capability: OpenAIEndpointCapabilityGrokMediaGeneration}.admits(nil, nil, account)
	require.False(t, ok)
	require.Equal(t, "capability_mismatch", reason)
}
