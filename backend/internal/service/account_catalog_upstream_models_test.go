//go:build unit

package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

// 渠道上的「模型改名」（credentials.model_mapping / model_mapping_rename_only）已删（2026-10-01）：
// 改名只认承接关系上的上游模型名（Account.CatalogUpstreamModels），只改名、不兼任白名单，叠在厂商默认表之上；
// 渠道承接哪些模型由目录绑定决定。spark 影子号的模型列表（credentials.model_mapping）仍是模型集合。

func relayKeyWithCatalogUpstreamModels(upstreamModels map[string]string) *Account {
	return &Account{
		Platform:              PlatformOpenAI,
		Type:                  AccountTypeAPIKey,
		Credentials:           map[string]any{"api_key": "sk-test"},
		ProtocolEndpoints:     map[string]string{APIProtocolChatCompletions: "https://relay.example.com/v1"},
		CatalogUpstreamModels: upstreamModels,
	}
}

func TestCatalogUpstreamModels_DoesNotRestrictOtherModels(t *testing.T) {
	account := relayKeyWithCatalogUpstreamModels(map[string]string{"claude-sonnet-4-5": "anthropic/claude-sonnet-4.5"})

	require.True(t, account.IsModelSupported("claude-sonnet-4-5"))
	require.Equal(t, "anthropic/claude-sonnet-4.5", account.GetMappedModel("claude-sonnet-4-5"))
	require.True(t, account.IsModelSupported("gpt-5.4"), "上游名没命中也可用：承接哪些模型由目录绑定决定")
	require.Equal(t, "gpt-5.4", account.GetMappedModel("gpt-5.4"))
}

// 库里残留的 credentials.model_mapping / model_mapping_rename_only 对普通渠道不再生效：既不改名，也不限制模型。
func TestCatalogUpstreamModels_IgnoresLeftoverCredentialsMapping(t *testing.T) {
	for name, renameOnly := range map[string]bool{"without rename_only": false, "with rename_only": true} {
		t.Run(name, func(t *testing.T) {
			account := relayKeyWithCatalogUpstreamModels(nil)
			account.Credentials["model_mapping"] = map[string]any{"claude-sonnet-4-5": "anthropic/claude-sonnet-4.5"}
			account.Credentials["model_mapping_rename_only"] = renameOnly

			require.Empty(t, account.GetModelMapping())
			require.Equal(t, "claude-sonnet-4-5", account.GetMappedModel("claude-sonnet-4-5"))
			require.True(t, account.IsModelSupported("gpt-5.4"))
		})
	}
}

func TestCatalogUpstreamModels_OpenAIOAuthStillRejectsForeignFamilies(t *testing.T) {
	account := &Account{
		Platform:              PlatformOpenAI,
		Type:                  AccountTypeOAuth,
		Credentials:           map[string]any{},
		CatalogUpstreamModels: map[string]string{"my-codex": "gpt-5.3-codex"},
	}
	require.True(t, account.IsModelSupported("my-codex"))
	require.Equal(t, "gpt-5.3-codex", account.GetMappedModel("my-codex"))
	require.True(t, account.IsModelSupported("gpt-5.4"))
	require.False(t, account.IsModelSupported("deepseek-chat"), "Codex 上游对其他厂商模型必然 400，兜底规则照旧")
}

func TestCatalogUpstreamModels_VendorDefaultsStayTheModelSet(t *testing.T) {
	defaults := xai.DefaultModelMapping()
	require.NotEmpty(t, defaults)
	var defaultModel string
	for model := range defaults {
		defaultModel = model
		break
	}

	account := &Account{
		Platform:              PlatformGrok,
		Type:                  AccountTypeOAuth,
		Credentials:           map[string]any{},
		CatalogUpstreamModels: map[string]string{"my-grok": "grok-4.5"},
	}
	require.True(t, account.IsModelSupported("my-grok"), "上游名叠在默认表之上")
	require.Equal(t, "grok-4.5", account.GetMappedModel("my-grok"))
	require.True(t, account.IsModelSupported(defaultModel), "只改一个模型的名不应让默认表里的模型不可用")
	require.False(t, account.IsModelSupported("definitely-not-a-grok-model"), "默认表仍是这个上游能接的模型集合")
}

// spark 影子号的模型列表是系统维护的模型集合：兼任白名单、替换厂商默认表。
func TestSparkShadowModelList_StaysTheModelSet(t *testing.T) {
	parentID := int64(1)

	t.Run("spark shadow list is a whitelist", func(t *testing.T) {
		shadow := &Account{
			Platform:        PlatformOpenAI,
			Type:            AccountTypeOAuth,
			ParentAccountID: &parentID,
			QuotaDimension:  QuotaDimensionSpark,
			Credentials:     map[string]any{"model_mapping": defaultSparkShadowModelMapping()},
		}
		require.True(t, shadow.IsModelSupported("gpt-5.3-codex-spark"))
		require.False(t, shadow.IsModelSupported("gpt-5.4"), "影子号的模型列表兼任白名单（普通 OpenAI OAuth 号会接 gpt-5.4）")
	})

	t.Run("shadow list replaces vendor defaults", func(t *testing.T) {
		var defaultModel string
		for model := range xai.DefaultModelMapping() {
			defaultModel = model
			break
		}
		shadow := &Account{
			Platform:        PlatformGrok,
			Type:            AccountTypeOAuth,
			ParentAccountID: &parentID,
			Credentials:     map[string]any{"model_mapping": map[string]any{"my-grok": "grok-4.5"}},
		}
		require.True(t, shadow.IsModelSupported("my-grok"))
		require.Equal(t, "grok-4.5", shadow.GetMappedModel("my-grok"))
		require.False(t, shadow.IsModelSupported(defaultModel), "影子号的模型列表替换默认表")
	})
}

// 叠加时只按目录标识精确覆盖：Antigravity 普通号有一条无关的上游名，默认表里其余条目（如
// gemini-3.1-pro-high → gemini-pro-agent）原样生效，不被「整份替换默认表」时才用的补全改掉。
func TestCatalogUpstreamModels_AntigravityKeepsDefaultTargets(t *testing.T) {
	account := &Account{
		Platform:              PlatformAntigravity,
		Type:                  AccountTypeOAuth,
		Credentials:           map[string]any{"access_token": "token"},
		CatalogUpstreamModels: map[string]string{"claude-sonnet-4-5": "claude-sonnet-4-6"},
	}
	require.Equal(t, "claude-sonnet-4-6", account.GetMappedModel("claude-sonnet-4-5"))
	for model, target := range map[string]string{
		"gemini-3.1-pro-high": domain.DefaultAntigravityModelMapping["gemini-3.1-pro-high"],
		"claude-haiku-4-5":    domain.DefaultAntigravityModelMapping["claude-haiku-4-5"],
	} {
		require.NotEmpty(t, target, model)
		require.Equal(t, target, account.GetMappedModel(model), model)
	}
	require.Equal(t, "gemini-pro-agent", account.GetMappedModel("gemini-3.1-pro-high"), "vendor table target unchanged")
}
