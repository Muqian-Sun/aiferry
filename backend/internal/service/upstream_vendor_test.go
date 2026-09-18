//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOfficialVendorOfURL 固定官方域名的识别口径：预填地址、Grok 各地址模式、国际站
// 都认；只认完整域名，仿冒后缀与中转地址不认。
func TestOfficialVendorOfURL(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://api.anthropic.com", PlatformAnthropic},
		{"https://api.openai.com/v1", PlatformOpenAI},
		{"https://generativelanguage.googleapis.com", PlatformGemini},
		{"https://api.x.ai/v1", PlatformGrok},
		{"https://us-west-2.api.x.ai/v1", PlatformGrok},
		{"https://cli-chat-proxy.grok.com/v1", PlatformGrok},
		{"https://api.moonshot.cn/anthropic", PlatformKimi},
		{"https://api.kimi.com/coding/v1", PlatformKimi},
		{"https://api.moonshot.ai/v1", PlatformKimi},
		{"https://open.bigmodel.cn/api/coding/paas/v4", PlatformZhipu},
		{"https://api.z.ai/api/paas/v4", PlatformZhipu},
		{"https://api.deepseek.com/anthropic", PlatformDeepseek},
		{"https://api.minimaxi.com/v1", PlatformMiniMax},
		{"https://api.minimax.io/anthropic", PlatformMiniMax},
		{"https://opencode.ai/zen/go/v1", PlatformOpenCodeGo},
		{"HTTPS://API.OPENAI.COM:443/v1", PlatformOpenAI},
		{"https://api.openai.com.relay.example.net/v1", ""},
		{"https://openai-proxy.example.com/v1", ""},
		{"https://example.com/api.openai.com", ""},
		{"api.openai.com", ""},
		{"", ""},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, OfficialVendorOfURL(tc.url), tc.url)
	}
}

// TestBuildOfficialVendorHostsRejectsConflicts 一个域名被两个厂商认领时构建失败，
// 而不是后写入的静默覆盖先写入的。
func TestBuildOfficialVendorHostsRejectsConflicts(t *testing.T) {
	hosts, err := buildOfficialVendorHosts()
	require.NoError(t, err)
	require.NotEmpty(t, hosts)

	original := officialVendorExtraHosts
	t.Cleanup(func() { officialVendorExtraHosts = original })
	officialVendorExtraHosts = map[string][]string{PlatformDeepseek: {"api.openai.com"}}

	_, err = buildOfficialVendorHosts()
	require.ErrorContains(t, err, "api.openai.com")
}

// TestAccountVendor 成品号的厂商是平台；第三方 key 的厂商只看协议地址，平台标签不算数。
func TestAccountVendor(t *testing.T) {
	key := func(platform string, endpoints map[string]string) *Account {
		return &Account{Platform: platform, Type: AccountTypeAPIKey, ProtocolEndpoints: endpoints}
	}

	t.Run("成品号按平台", func(t *testing.T) {
		require.Equal(t, PlatformAntigravity, (&Account{Platform: PlatformAntigravity, Type: AccountTypeOAuth}).Vendor())
		require.Equal(t, PlatformGrok, (&Account{Platform: PlatformGrok, Type: AccountTypeOAuth}).Vendor())
	})

	t.Run("官方地址按域名认厂商，与标签无关", func(t *testing.T) {
		account := key(PlatformOpenAI, map[string]string{
			APIProtocolChatCompletions: "https://api.deepseek.com",
			APIProtocolAnthropic:       "https://api.deepseek.com/anthropic",
		})
		require.Equal(t, PlatformDeepseek, account.Vendor())
	})

	t.Run("同一厂商的不同站点仍算该厂商", func(t *testing.T) {
		account := key(PlatformKimi, map[string]string{
			APIProtocolChatCompletions: "https://api.moonshot.cn/v1",
			APIProtocolAnthropic:       "https://api.kimi.com/coding",
		})
		require.Equal(t, PlatformKimi, account.Vendor())
	})

	t.Run("中转地址按通用处理，即使标签是厂商", func(t *testing.T) {
		require.Equal(t, "", key(PlatformDeepseek, map[string]string{
			APIProtocolChatCompletions: "https://relay.example.com/v1",
		}).Vendor())
	})

	t.Run("官方地址混着中转地址按通用处理", func(t *testing.T) {
		require.Equal(t, "", key(PlatformOpenAI, map[string]string{
			APIProtocolResponses:       "https://api.openai.com",
			APIProtocolChatCompletions: "https://relay.example.com/v1",
		}).Vendor())
	})

	t.Run("地址分属不同厂商按通用处理", func(t *testing.T) {
		require.Equal(t, "", key(PlatformOpenAI, map[string]string{
			APIProtocolResponses: "https://api.openai.com",
			APIProtocolAnthropic: "https://api.anthropic.com",
		}).Vendor())
	})

	t.Run("没有地址的 key 没有厂商", func(t *testing.T) {
		require.Equal(t, "", key(PlatformOpenAI, nil).Vendor())
	})
}
