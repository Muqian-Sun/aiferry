//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOfficialVendorOfURL 固定官方域名的识别口径：只有国产厂商与 OpenCode 的预填地址、国际站
// 算官方；Anthropic、OpenAI、Gemini、Grok 的官方域名不在表里（2026-09-29 海外四家不再有官方
// key，指向它们的 key 按中转处理）。只认完整域名，仿冒后缀与中转地址不认。
func TestOfficialVendorOfURL(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://api.anthropic.com", ""},
		{"https://api.openai.com/v1", ""},
		{"https://generativelanguage.googleapis.com", ""},
		{"https://api.x.ai/v1", ""},
		{"https://us-west-2.api.x.ai/v1", ""},
		{"https://cli-chat-proxy.grok.com/v1", ""},
		{"https://api.moonshot.cn/anthropic", PlatformKimi},
		{"https://api.kimi.com/coding/v1", PlatformKimi},
		{"https://api.moonshot.ai/v1", PlatformKimi},
		{"https://open.bigmodel.cn/api/coding/paas/v4", PlatformZhipu},
		{"https://api.z.ai/api/paas/v4", PlatformZhipu},
		{"https://api.deepseek.com/anthropic", PlatformDeepseek},
		{"https://api.minimaxi.com/v1", PlatformMiniMax},
		{"https://api.minimax.io/anthropic", PlatformMiniMax},
		{"https://opencode.ai/zen/go/v1", PlatformOpenCodeGo},
		{"HTTPS://API.DEEPSEEK.COM:443/v1", PlatformDeepseek},
		{"https://api.deepseek.com.relay.example.net/v1", ""},
		{"https://deepseek-proxy.example.com/v1", ""},
		{"https://example.com/api.deepseek.com", ""},
		{"api.deepseek.com", ""},
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
	officialVendorExtraHosts = map[string][]string{PlatformDeepseek: {"api.moonshot.cn"}}

	_, err = buildOfficialVendorHosts()
	require.ErrorContains(t, err, "api.moonshot.cn")
}

// TestOfficialVendorHostsOnlyDomesticProviders 官方域名表只有国产厂商与 OpenCode：海外四家的
// 官方地址（含 Grok 区域与 CLI 网关）都不在表里，管理端也不会再按这些地址提示厂商。
func TestOfficialVendorHostsOnlyDomesticProviders(t *testing.T) {
	hosts := OfficialVendorHosts()
	require.NotEmpty(t, hosts)
	for host, vendor := range hosts {
		require.Contains(t, []string{PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax, PlatformOpenCodeGo}, vendor, host)
	}
	for _, host := range []string{"api.anthropic.com", "api.openai.com", "generativelanguage.googleapis.com", "api.x.ai", "us-east-1.api.x.ai", "cli-chat-proxy.grok.com"} {
		require.NotContains(t, hosts, host)
	}
}

// TestAccountVendor 成品号的厂商是平台；第三方 key 的厂商只看协议地址，平台标签不算数。
func TestAccountVendor(t *testing.T) {
	key := func(platform string, endpoints map[string]string) *Account {
		return &Account{Platform: platform, Type: AccountTypeAPIKey, ProtocolEndpoints: endpoints}
	}

	t.Run("成品号按平台", func(t *testing.T) {
		require.Equal(t, PlatformAntigravity, (&Account{Platform: PlatformAntigravity, Type: AccountTypeOAuth}).Vendor())
		require.Equal(t, PlatformGrok, (&Account{Platform: PlatformGrok, Type: AccountTypeOAuth}).Vendor())
		require.Equal(t, PlatformAnthropic, (&Account{Platform: PlatformAnthropic, Type: AccountTypeSetupToken}).Vendor())
		require.Equal(t, PlatformAnthropic, (&Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock}).Vendor())
		require.Equal(t, PlatformOpenAI, (&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}).Vendor())
		require.Equal(t, PlatformGemini, (&Account{Platform: PlatformGemini, Type: AccountTypeServiceAccount}).Vendor())
	})

	// 海外四家不再有官方 key：key 指向它们的官方域名一律按中转，标签是哪家都一样。
	t.Run("指向海外四家官方域名的 key 按中转", func(t *testing.T) {
		for _, tc := range []struct {
			platform string
			protocol string
			url      string
		}{
			{PlatformAnthropic, APIProtocolAnthropic, "https://api.anthropic.com"},
			{PlatformOpenAI, APIProtocolResponses, "https://api.openai.com"},
			{PlatformOpenAI, APIProtocolChatCompletions, "https://api.openai.com/v1"},
			{PlatformGemini, APIProtocolGemini, "https://generativelanguage.googleapis.com"},
			{PlatformGrok, APIProtocolResponses, "https://api.x.ai/v1"},
			{PlatformGrok, APIProtocolChatCompletions, "https://us-east-1.api.x.ai/v1"},
			{PlatformGrok, APIProtocolResponses, "https://cli-chat-proxy.grok.com/v1"},
		} {
			require.Equal(t, "", key(tc.platform, map[string]string{tc.protocol: tc.url}).Vendor(), tc.url)
		}
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
		require.Equal(t, "", key(PlatformDeepseek, map[string]string{
			APIProtocolResponses:       "https://api.deepseek.com",
			APIProtocolChatCompletions: "https://relay.example.com/v1",
		}).Vendor())
	})

	t.Run("地址分属不同厂商按通用处理", func(t *testing.T) {
		require.Equal(t, "", key(PlatformKimi, map[string]string{
			APIProtocolResponses: "https://api.deepseek.com",
			APIProtocolAnthropic: "https://api.moonshot.cn/anthropic",
		}).Vendor())
	})

	t.Run("没有地址的 key 没有厂商", func(t *testing.T) {
		require.Equal(t, "", key(PlatformOpenAI, nil).Vendor())
	})
}
