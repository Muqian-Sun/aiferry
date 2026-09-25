//go:build unit

package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

// 渠道表单去掉「模型白名单」之后（2026-09-25），新表单写的映射带 model_mapping_rename_only：
// 映射只改名，渠道承接哪些模型由目录绑定决定。不带标记的映射（spark 影子、图片模型显式开通、老数据）仍是模型集合。

func relayKeyWithMapping(mapping map[string]any, renameOnly bool) *Account {
	creds := map[string]any{"api_key": "sk-test", "model_mapping": mapping}
	if renameOnly {
		creds["model_mapping_rename_only"] = true
	}
	return &Account{
		Platform:          PlatformOpenAI,
		Type:              AccountTypeAPIKey,
		Credentials:       creds,
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://relay.example.com/v1"},
	}
}

func TestRenameOnlyMapping_DoesNotRestrictOtherModels(t *testing.T) {
	account := relayKeyWithMapping(map[string]any{"claude-sonnet-4-5": "anthropic/claude-sonnet-4.5"}, true)

	require.True(t, account.IsModelSupported("claude-sonnet-4-5"))
	require.Equal(t, "anthropic/claude-sonnet-4.5", account.GetMappedModel("claude-sonnet-4-5"))
	require.True(t, account.IsModelSupported("gpt-5.4"), "只改名的映射没命中也可用：承接哪些模型由目录绑定决定")
	require.Equal(t, "gpt-5.4", account.GetMappedModel("gpt-5.4"))
}

func TestMappingWithoutRenameOnlyFlagStillRestricts(t *testing.T) {
	account := relayKeyWithMapping(map[string]any{"claude-sonnet-4-5": "anthropic/claude-sonnet-4.5"}, false)

	require.True(t, account.IsModelSupported("claude-sonnet-4-5"))
	require.False(t, account.IsModelSupported("gpt-5.4"), "不带标记的映射仍是模型集合（spark 影子等依赖它）")
}

func TestRenameOnlyMapping_OpenAIOAuthStillRejectsForeignFamilies(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"model_mapping":             map[string]any{"my-codex": "gpt-5.3-codex"},
			"model_mapping_rename_only": true,
		},
	}
	require.True(t, account.IsModelSupported("my-codex"))
	require.True(t, account.IsModelSupported("gpt-5.4"))
	require.False(t, account.IsModelSupported("deepseek-chat"), "Codex 上游对其他厂商模型必然 400，兜底规则照旧")
}

func TestRenameOnlyMapping_VendorDefaultsStayTheModelSet(t *testing.T) {
	defaults := xai.DefaultModelMapping()
	require.NotEmpty(t, defaults)
	var defaultModel string
	for model := range defaults {
		defaultModel = model
		break
	}

	account := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"model_mapping":             map[string]any{"my-grok": "grok-4.5"},
			"model_mapping_rename_only": true,
		},
	}
	require.True(t, account.IsModelSupported("my-grok"), "改名叠在默认表之上")
	require.Equal(t, "grok-4.5", account.GetMappedModel("my-grok"))
	require.True(t, account.IsModelSupported(defaultModel), "只配一条改名不应让默认表里的模型不可用")
	require.False(t, account.IsModelSupported("definitely-not-a-grok-model"), "默认表仍是这个上游能接的模型集合")

	withoutFlag := &Account{
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"model_mapping": map[string]any{"my-grok": "grok-4.5"}},
	}
	require.False(t, withoutFlag.IsModelSupported(defaultModel), "不带标记：自定义映射仍替换默认表")
}

// 标记会改变有默认表厂商的解析结果（叠加默认表），映射解析缓存必须把标记算进缓存键。
func TestRenameOnlyMapping_FlagChangeInvalidatesCache(t *testing.T) {
	var defaultModel string
	for model := range xai.DefaultModelMapping() {
		defaultModel = model
		break
	}
	account := &Account{
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"model_mapping": map[string]any{"my-grok": "grok-4.5"}},
	}
	require.False(t, account.IsModelSupported(defaultModel))

	account.Credentials["model_mapping_rename_only"] = true
	require.True(t, account.IsModelSupported(defaultModel), "原地改标记后不能沿用旧的解析缓存")
}
