//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func newOpenAIOAuthAccountForModelTest() *Account {
	return &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
	}
}

func TestIsModelSupported_OpenAIOAuthEmptyMapping_ServableModels(t *testing.T) {
	account := newOpenAIOAuthAccountForModelTest()

	servable := []string{
		"", // 空模型交由上层必填校验
		"gpt-5.4",
		"gpt-5.4-high", // 推理后缀变体
		"gpt-5.3-codex",
		"gpt-5.1-codex-mini",
		"gpt-5",
		"codex-mini-latest",
		"gpt5.3codexspark",  // 别名拼写
		"gpt-image-1",       // 图像生成模型
		"claude-sonnet-4-6", // /v1/messages 调度默认映射兜底
		"claude-3-opus-20240229",
		"gpt-4o",          // 保守 fail-open：非黑名单模型保持允许
		"my-custom-alias", // 自定义别名可能由渠道级映射在转发前改写，保持允许
	}
	for _, model := range servable {
		require.True(t, account.IsModelSupported(model), "expected %q to be servable by empty-mapping OpenAI OAuth account", model)
	}
}

func TestIsModelSupported_OpenAIOAuthEmptyMapping_RejectsForeignModels(t *testing.T) {
	account := newOpenAIOAuthAccountForModelTest()

	// Codex 上游必然以不可重试的 400 拒绝这些厂商家族；调度阶段就应跳过
	// 该账号，让显式声明支持的 API Key 账号接手（#3662）。
	foreign := []string{
		"deepseek-v4",
		"deepseek-chat",
		"glm-4.7",
		"kimi-k2",
		"k3",          // Kimi Code bare ID（无厂商前缀，需精确拒绝）
		"k3-256k",     // Kimi Code bare ID
		"provider/k3", // vendor/model 取 last segment 后仍为 k3
		"moonshot-v1-128k",
		"gemini-3.0-pro",
		"grok-4",
		"qwen3-max",
		"minimax-m2.5",
		"llama-3.3-70b",
		"provider/deepseek-v4", // vendor/model 形式取最后一段判定
	}
	for _, model := range foreign {
		require.False(t, account.IsModelSupported(model), "expected %q to be rejected by empty-mapping OpenAI OAuth account", model)
	}
}

func TestIsModelSupported_OpenAIOAuthExplicitMappingUnchanged(t *testing.T) {
	account := newOpenAIOAuthAccountForModelTest()
	account.CatalogUpstreamModels = map[string]string{
		"deepseek-v4": "gpt-5.4",
		"k3":          "gpt-5.4", // 承接关系上的上游名优先：bare k3 改名后仍可支持
	}

	// 命中承接关系上的上游名即支持；上游名不兼任白名单，未命中的按空映射规则判定（其他厂商家族仍排除）。
	require.True(t, account.IsModelSupported("deepseek-v4"))
	require.True(t, account.IsModelSupported("k3"))
	require.False(t, account.IsModelSupported("glm-4.7"))
	require.True(t, account.IsModelSupported("gpt-5.6-sol"))
}

// OpenAI 自动透传 2026-09-28 P5 写死关：残留 openai_passthrough=true 的 OAuth 账号与普通账号一样
// 按空映射规则排除其他厂商模型（改之前一律放行）；有承接关系上的上游名时，未命中的同样按空映射规则判定。
func TestIsModelSupported_OpenAIOAuthLegacyPassthroughKeyIgnored(t *testing.T) {
	account := newOpenAIOAuthAccountForModelTest()
	account.Extra = map[string]any{"openai_passthrough": true}
	require.False(t, account.IsModelSupported("deepseek-v4"), "空映射仍排除其他厂商家族")

	account.CatalogUpstreamModels = map[string]string{"gpt-5.4": "gpt-5.4-mini"}
	require.False(t, account.IsModelSupported("deepseek-v4"), "有上游名时未命中的仍排除其他厂商家族")
	require.True(t, account.IsModelSupported("gpt-5.6-sol"), "承接关系上的上游名不兼任白名单")
	require.True(t, account.IsModelSupported("gpt-5.4"))
}

func TestIsModelSupported_OpenAIAPIKeyEmptyMappingAllowsAll(t *testing.T) {
	account := &Account{
		ID:                2,
		Platform:          PlatformOpenAI,
		Type:              AccountTypeAPIKey,
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
	}

	// API Key 账号（第三方 OpenAI 兼容上游）可服务任意别名，语义不变。
	require.True(t, account.IsModelSupported("deepseek-v4"))
	require.True(t, account.IsModelSupported("gpt-5.4"))
}

func TestIsModelSupported_NonOpenAIPlatformsUnchanged(t *testing.T) {
	anthropic := &Account{ID: 3, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
	require.True(t, anthropic.IsModelSupported("claude-sonnet-4-6"))
	require.True(t, anthropic.IsModelSupported("deepseek-v4"))
}

func TestIsOpenAIOAuthServableModel(t *testing.T) {
	require.True(t, isOpenAIOAuthServableModel("gpt-5.4-high"))
	require.True(t, isOpenAIOAuthServableModel("  gpt-5.3-codex  "))
	require.True(t, isOpenAIOAuthServableModel("claude-3-5-haiku-20241022"))
	require.True(t, isOpenAIOAuthServableModel("DeepThink-x"))  // 非黑名单前缀，保持允许
	require.False(t, isOpenAIOAuthServableModel("DeepSeek-V4")) // 大小写不敏感
	require.False(t, isOpenAIOAuthServableModel("qwen3-235b-thinking"))
	require.True(t, isOpenAIOAuthServableModel("deepseekcoder")) // 无连字符 → 非黑名单前缀，保持允许
	require.False(t, isOpenAIOAuthServableModel("k3"))
	require.False(t, isOpenAIOAuthServableModel("k3-256k"))
	require.False(t, isOpenAIOAuthServableModel("provider/k3"))
	require.True(t, isOpenAIOAuthServableModel("my-k3-alias")) // 非精确 bare ID，自定义别名 fail-open
}
