//go:build unit

package service

import (
	"context"
	"fmt"
	"testing"
)

// Vendor() 每次调用都解析第三方 key 的全部协议地址。下面的基准衡量调度候选循环里读 Vendor 的
// 典型调用组合，用来判断要不要缓存：
//
//	go test -tags unit -p 1 -run '^$' -bench 'Vendor' -benchmem ./internal/service/

func vendorBenchCandidates(n int, thirdPartyKeys bool) []Account {
	endpointSets := []map[string]string{
		// 通用中转，三协议
		{APIProtocolChatCompletions: "https://relay.example.com/v1", APIProtocolResponses: "https://relay.example.com/v1", APIProtocolAnthropic: "https://relay.example.com"},
		// 官方 OpenAI
		{APIProtocolChatCompletions: "https://api.openai.com/v1", APIProtocolResponses: "https://api.openai.com/v1"},
		// 官方 Kimi Coding
		{APIProtocolChatCompletions: DefaultKimiCodingBaseURL, APIProtocolAnthropic: DefaultKimiCodingAnthropicBaseURL, APIProtocolResponses: DefaultKimiCodingBaseURL},
		// 官方 DeepSeek
		{APIProtocolChatCompletions: DefaultDeepseekBaseURL, APIProtocolAnthropic: DefaultDeepseekAnthropicBaseURL, APIProtocolResponses: DefaultDeepseekBaseURL},
	}
	accounts := make([]Account, n)
	for i := range accounts {
		account := Account{
			ID:          int64(i + 1),
			Name:        fmt.Sprintf("bench-%d", i),
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 5,
			Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5.1": "gpt-5.1", "gpt-5.1-*": "gpt-5.1"}},
			Extra:       map[string]any{"privacy_mode": PrivacyModeTrainingOff},
		}
		if thirdPartyKeys {
			account.Type = AccountTypeAPIKey
			account.ProtocolEndpoints = endpointSets[i%len(endpointSets)]
		}
		accounts[i] = account
	}
	return accounts
}

// benchmarkVendorCandidateLoop 一次迭代 = 一个请求对 len(accounts) 个候选做一遍准入：平台/协议准入、
// require_privacy_set、模型支持、模型级限流 key、OpenAI 协议特性。这些调用各自读 Vendor。
func benchmarkVendorCandidateLoop(b *testing.B, accounts []Account) {
	ctx := WithInboundProtocol(context.Background(), APIProtocolResponses)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for i := range accounts {
			account := &accounts[i]
			_ = isAccountSchedulableOnPlatform(ctx, account, PlatformOpenAI)
			_ = account.IsPrivacySet()
			_ = account.IsModelSupported("gpt-5.1")
			_ = account.GetModelRateLimitRemainingTimeWithContext(ctx, "gpt-5.1")
			_ = openAIProtocolFeaturesApply(account)
		}
	}
}

func BenchmarkVendorCandidateLoop200Keys(b *testing.B) {
	benchmarkVendorCandidateLoop(b, vendorBenchCandidates(200, true))
}

// 同一循环跑成品号（Vendor 直接返回平台），作为不解析地址的对照。
func BenchmarkVendorCandidateLoop200Subscriptions(b *testing.B) {
	benchmarkVendorCandidateLoop(b, vendorBenchCandidates(200, false))
}

func BenchmarkVendorSingleKey(b *testing.B) {
	account := vendorBenchCandidates(3, true)[2]
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_ = account.Vendor()
	}
}
