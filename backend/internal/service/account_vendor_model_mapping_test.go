//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

// 第三方 key 的模型支持、默认映射与上游模型归一按 Vendor（协议地址）判定，平台标签只用于展示。
// 每个用例先断言夹具的 Vendor，保证「中转 / 官方」前提真的成立。

const vendorTestRelayURL = "https://relay.example.com/v1"

func vendorTestKey(platform string, endpoints map[string]string) *Account {
	return &Account{
		ID:                9100,
		Platform:          platform,
		Type:              AccountTypeAPIKey,
		Status:            StatusActive,
		Schedulable:       true,
		ProtocolEndpoints: endpoints,
	}
}

func TestIsModelSupported_DeepseekAllowlistFollowsVendor(t *testing.T) {
	t.Run("deepseek label on relay allows any model", func(t *testing.T) {
		account := vendorTestKey(PlatformDeepseek, map[string]string{APIProtocolChatCompletions: vendorTestRelayURL})
		require.Empty(t, account.Vendor())

		require.True(t, account.IsModelSupported("gpt-5.4"))
		require.True(t, account.IsModelSupported("claude-sonnet-4-6"))
	})

	t.Run("openai label on api.deepseek.com uses the deepseek allowlist", func(t *testing.T) {
		account := vendorTestKey(PlatformOpenAI, map[string]string{APIProtocolChatCompletions: "https://api.deepseek.com"})
		require.Equal(t, PlatformDeepseek, account.Vendor())

		require.True(t, account.IsModelSupported("deepseek-flash"))
		require.True(t, account.IsModelSupported("deepseek-flash[1m]"))
		require.False(t, account.IsModelSupported("gpt-5.4"))
	})
}

// firstXAIAlias 取 xAI 默认映射里一条真正改写模型名的别名，避免断言退化成恒等映射。
func firstXAIAlias(t *testing.T) (alias, target string) {
	t.Helper()
	for from, to := range xai.DefaultModelMapping() {
		if from != to {
			return from, to
		}
	}
	t.Fatal("xai default mapping has no rewriting alias")
	return "", ""
}

func TestResolveModelMapping_XAIDefaultMappingFollowsVendor(t *testing.T) {
	alias, target := firstXAIAlias(t)
	relayEndpoints := map[string]string{APIProtocolChatCompletions: vendorTestRelayURL, APIProtocolResponses: vendorTestRelayURL}
	xaiEndpoints := map[string]string{APIProtocolChatCompletions: "https://api.x.ai/v1", APIProtocolResponses: "https://api.x.ai/v1"}

	// 三个分支：Credentials 为 nil、映射为空、映射里没有字符串值。
	credentialVariants := map[string]map[string]any{
		"nil credentials":         nil,
		"no model_mapping":        {"api_key": "sk-test"},
		"non-string mapping only": {"model_mapping": map[string]any{"grok-4": 1}},
	}

	for name, credentials := range credentialVariants {
		t.Run(name+"/grok label on relay gets no xAI mapping", func(t *testing.T) {
			account := vendorTestKey(PlatformGrok, relayEndpoints)
			account.Credentials = credentials
			require.Empty(t, account.Vendor())

			require.Empty(t, account.GetModelMapping())
			require.Equal(t, alias, account.GetMappedModel(alias))
			require.True(t, account.IsModelSupported("not-an-xai-model"))
		})

		t.Run(name+"/openai label on api.x.ai gets the xAI mapping", func(t *testing.T) {
			account := vendorTestKey(PlatformOpenAI, xaiEndpoints)
			account.Credentials = credentials
			require.Equal(t, PlatformGrok, account.Vendor())

			require.Equal(t, xai.DefaultModelMapping(), account.GetModelMapping())
			require.Equal(t, target, account.GetMappedModel(alias))
			require.False(t, account.IsModelSupported("not-an-xai-model"))
		})
	}
}

func TestAntigravityModelMapping_NeverAppliesToKeys(t *testing.T) {
	svc := &GatewayService{}
	thinkingCtx := context.WithValue(context.Background(), ctxkey.ThinkingEnabled, true)
	relayAnthropic := map[string]string{APIProtocolAnthropic: "https://relay.example.com"}

	const outsideDefault = "claude-3-5-sonnet-20241022"
	_, inDefault := domain.DefaultAntigravityModelMapping[outsideDefault]
	require.False(t, inDefault, "fixture model must be outside the antigravity default table")

	// 三个分支：Credentials 为 nil、映射为空、映射里没有字符串值。
	for name, credentials := range map[string]map[string]any{
		"nil credentials":         nil,
		"no model_mapping":        {"api_key": "sk-test"},
		"non-string mapping only": {"model_mapping": map[string]any{"claude-sonnet-4-5": 1}},
	} {
		t.Run(name+"/empty mapping allows models outside the antigravity default table", func(t *testing.T) {
			account := vendorTestKey(PlatformAntigravity, relayAnthropic)
			account.Credentials = credentials
			require.Empty(t, account.Vendor())

			require.Empty(t, account.GetModelMapping())
			// 调度路径（Anthropic 网关与 Gemini 兼容网关）
			require.True(t, svc.isModelSupportedByAccount(account, outsideDefault))
			require.True(t, svc.isModelSupportedByAccountWithContext(thinkingCtx, account, outsideDefault))
			require.True(t, (&GeminiMessagesCompatService{}).isModelSupportedByAccount(account, outsideDefault))
			// 转发路径：GatewayService.Forward 对 key 用 GetMappedModel，渠道限制检查用 resolveAccountUpstreamModel
			require.Equal(t, outsideDefault, account.GetMappedModel(outsideDefault))
			require.Equal(t, outsideDefault, resolveAccountUpstreamModel(account, outsideDefault))
		})
	}

	t.Run("custom mapping gets no thinking suffix, passthroughs or 3.1-pro aliases", func(t *testing.T) {
		mapping := map[string]any{
			"claude-sonnet-4-5":                     "claude-sonnet-4-5",
			domain.AntigravityGemini31ProAgentModel: domain.AntigravityGemini31ProAgentModel,
		}
		key := vendorTestKey(PlatformAntigravity, relayAnthropic)
		key.Credentials = map[string]any{"model_mapping": mapping}
		require.Empty(t, key.Vendor())

		// 对照：同样映射的 Antigravity 成品号会被注入默认透传与 3.1-pro 别名，thinking 后缀生效。
		subscription := &Account{ID: 9101, Platform: PlatformAntigravity, Type: AccountTypeOAuth, Credentials: map[string]any{"model_mapping": mapping}}
		require.Contains(t, subscription.GetModelMapping(), "gemini-3-flash")
		require.Contains(t, subscription.GetModelMapping(), "gemini-3.1-pro")
		require.False(t, svc.isModelSupportedByAccountWithContext(thinkingCtx, subscription, "claude-sonnet-4-5"))

		keyMapping := key.GetModelMapping()
		require.NotContains(t, keyMapping, "gemini-3-flash")
		require.NotContains(t, keyMapping, "gemini-3.1-pro")
		require.True(t, svc.isModelSupportedByAccountWithContext(thinkingCtx, key, "claude-sonnet-4-5"))
		require.Equal(t, "claude-sonnet-4-5", resolveAccountUpstreamModel(key, "claude-sonnet-4-5"))
	})
}

func TestModelLookupCustomtoolsAliasFollowsVendor(t *testing.T) {
	mapping := map[string]any{"gemini-3.1-pro-preview": "gemini-3.1-pro-preview"}

	relay := vendorTestKey(PlatformGemini, map[string]string{APIProtocolGemini: "https://relay.example.com"})
	relay.Credentials = map[string]any{"model_mapping": mapping}
	require.Empty(t, relay.Vendor())
	require.False(t, relay.IsModelSupported("gemini-3.1-pro-preview-customtools"))
	_, matched := relay.ResolveMappedModel("gemini-3.1-pro-preview-customtools")
	require.False(t, matched)

	official := vendorTestKey(PlatformOpenAI, map[string]string{APIProtocolGemini: "https://generativelanguage.googleapis.com"})
	official.Credentials = map[string]any{"model_mapping": mapping}
	require.Equal(t, PlatformGemini, official.Vendor())
	require.True(t, official.IsModelSupported("gemini-3.1-pro-preview-customtools"))
	mapped, matched := official.ResolveMappedModel("gemini-3.1-pro-preview-customtools")
	require.True(t, matched)
	require.Equal(t, "gemini-3.1-pro-preview", mapped)
}

func TestOpenAIPassthroughAllowAll_OnlyForOpenAIOrRelayVendor(t *testing.T) {
	svc := &GatewayService{}
	passthrough := map[string]any{"openai_passthrough": true}

	t.Run("relay keeps allow-all despite leftover mapping", func(t *testing.T) {
		account := vendorTestKey(PlatformOpenAI, map[string]string{APIProtocolChatCompletions: vendorTestRelayURL})
		account.Extra = passthrough
		account.Credentials = map[string]any{"model_mapping": map[string]any{"gpt-5.4": "gpt-5.4"}}
		require.Empty(t, account.Vendor())

		require.True(t, account.IsModelSupported("gpt-9"))
		require.True(t, svc.isModelSupportedByAccount(account, "gpt-9"))
	})

	t.Run("known non-openai vendor does not bypass its allowlist", func(t *testing.T) {
		account := vendorTestKey(PlatformOpenAI, map[string]string{APIProtocolChatCompletions: "https://api.deepseek.com"})
		account.Extra = passthrough
		require.Equal(t, PlatformDeepseek, account.Vendor())

		require.False(t, account.IsModelSupported("gpt-5.4"))
		require.False(t, svc.isModelSupportedByAccount(account, "gpt-5.4"))
	})
}

func TestNormalizeOpenAIModelForUpstream_DeepseekLongContextSuffixFollowsVendor(t *testing.T) {
	relay := vendorTestKey(PlatformDeepseek, map[string]string{APIProtocolChatCompletions: vendorTestRelayURL})
	require.Empty(t, relay.Vendor())
	require.Equal(t, "deepseek-flash[1m]", normalizeOpenAIModelForUpstream(relay, "deepseek-flash[1m]"))

	official := vendorTestKey(PlatformOpenAI, map[string]string{APIProtocolChatCompletions: "https://api.deepseek.com"})
	require.Equal(t, PlatformDeepseek, official.Vendor())
	require.Equal(t, "deepseek-flash", normalizeOpenAIModelForUpstream(official, "deepseek-flash[1m]"))
}

// Anthropic 短名 → 长 ID 的展开只给成品号用；key（即使指向官方地址、标签为 anthropic）
// 的模型名完全由管理员映射决定。
func TestGatewayModelSupport_AnthropicShortIDNormalizationIsSubscriptionOnly(t *testing.T) {
	svc := &GatewayService{}
	const shortID = "claude-sonnet-4-5"
	longID := "claude-sonnet-4-5-20250929"
	mapping := map[string]any{longID: longID}

	subscription := &Account{ID: 9102, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Credentials: map[string]any{"model_mapping": mapping}}
	require.True(t, svc.isModelSupportedByAccount(subscription, shortID), "fixture: short id must expand to the mapped long id")

	key := vendorTestKey(PlatformAnthropic, map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"})
	key.Credentials = map[string]any{"model_mapping": mapping}
	require.Equal(t, PlatformAnthropic, key.Vendor())
	require.False(t, svc.isModelSupportedByAccount(key, shortID))
}

// 模型级限流的读取 key 必须与 RateLimitService 写入时同口径（按 Vendor）。
func TestModelRateLimitKeys_FollowVendor(t *testing.T) {
	now := time.Now()
	limited := func(account *Account, scope string) *Account {
		setAccountModelRateLimitSnapshot(account, scope, now.Add(time.Hour), "test", now)
		return account
	}

	t.Run("antigravity-labelled relay key reads the plain mapped model key", func(t *testing.T) {
		const model = "claude-3-5-sonnet-20241022"
		account := limited(vendorTestKey(PlatformAntigravity, map[string]string{APIProtocolAnthropic: "https://relay.example.com"}), model)
		require.Empty(t, account.Vendor())
		require.False(t, account.IsSchedulableForModel(model))
	})

	t.Run("antigravity-labelled relay key gets no credits overage bypass", func(t *testing.T) {
		const model = "claude-3-5-sonnet-20241022"
		account := limited(vendorTestKey(PlatformAntigravity, map[string]string{APIProtocolAnthropic: "https://relay.example.com"}), model)
		account.Extra["allow_overages"] = true
		require.False(t, account.IsSchedulableForModel(model))
	})

	t.Run("fable scope is read for the official anthropic vendor only", func(t *testing.T) {
		const model = "claude-fable-5[1m]"
		official := limited(vendorTestKey(PlatformOpenAI, map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"}), anthropicFableRateLimitKey)
		require.Equal(t, PlatformAnthropic, official.Vendor())
		require.False(t, official.IsSchedulableForModel(model))

		relay := limited(vendorTestKey(PlatformAnthropic, map[string]string{APIProtocolAnthropic: "https://relay.example.com"}), anthropicFableRateLimitKey)
		require.True(t, relay.IsSchedulableForModel(model))
	})

	t.Run("image generation scope is read for openai or relay vendors", func(t *testing.T) {
		const model = "gpt-image-2"
		relay := limited(vendorTestKey(PlatformKimi, map[string]string{APIProtocolChatCompletions: vendorTestRelayURL}), openAIImageGenerationRateLimitKey)
		require.Empty(t, relay.Vendor())
		require.False(t, relay.IsSchedulableForModel(model))

		moonshot := limited(vendorTestKey(PlatformOpenAI, map[string]string{APIProtocolChatCompletions: "https://api.moonshot.cn/v1"}), openAIImageGenerationRateLimitKey)
		require.Equal(t, PlatformKimi, moonshot.Vendor())
		require.True(t, moonshot.IsSchedulableForModel(model))
	})
}
