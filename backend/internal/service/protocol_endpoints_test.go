//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNormalizeProtocolEndpoints 固定「协议 → 上游地址」映射的校验与规范化口径。
// 校验必须 fail-closed：未知协议或空地址直接报错，而不是静默丢弃——静默丢弃会让
// 管理员以为配置已保存，直到转发时才表现为地址缺失。
func TestNormalizeProtocolEndpoints(t *testing.T) {
	t.Run("空输入返回空映射而非 nil", func(t *testing.T) {
		got, err := NormalizeProtocolEndpoints(nil)
		require.NoError(t, err)
		require.Equal(t, map[string]string{}, got)
	})

	t.Run("四种对话协议都接受（各自单独配）", func(t *testing.T) {
		for _, protocol := range ConversationProtocols() {
			got, err := NormalizeProtocolEndpoints(map[string]string{protocol: "https://relay.example.com"})
			require.NoError(t, err, protocol)
			require.Equal(t, map[string]string{protocol: "https://relay.example.com"}, got)
		}
	})

	// video 地址只有官方视频适配器用：认不出视频厂商的地址存下来也永远调度不到。
	t.Run("video 地址必须是官方视频厂商的地址", func(t *testing.T) {
		for _, good := range []string{DefaultSeedanceCNBaseURL, DefaultSeedanceIntlBaseURL, DefaultKlingCNBaseURL, DefaultKlingIntlBaseURL,
			DefaultMiniMaxVideoCNBaseURL, DefaultMiniMaxVideoIntlBaseURL + "/"} {
			got, err := NormalizeProtocolEndpoints(map[string]string{APIProtocolVideo: good})
			require.NoError(t, err, good)
			require.Len(t, got, 1)
		}
		for _, bad := range []string{"https://relay.example.com/v1", "https://api.minimaxi.com/v1", "https://api.moonshot.cn/v1"} {
			_, err := NormalizeProtocolEndpoints(map[string]string{APIProtocolVideo: bad})
			require.ErrorContains(t, err, "official address of a video vendor", bad)
		}
		_, err := NormalizeProtocolEndpoints(map[string]string{APIProtocolVideo: DefaultKlingCNBaseURL, APIProtocolChatCompletions: "https://relay.example.com/v1"})
		require.ErrorContains(t, err, "exactly one upstream protocol", "video key 同样只有一个协议")
	})

	// 一个资源只承接一个上游协议：一个 key 多个模型、一个协议；同一渠道要多协议就配多个 key。
	// 配两个地址时选号侧与转发侧会按不同依据各挑一个，口径不一致，必须在配置入口挡住。
	t.Run("配多个协议地址报错", func(t *testing.T) {
		_, err := NormalizeProtocolEndpoints(map[string]string{
			APIProtocolAnthropic: "https://relay.example.com",
			APIProtocolResponses: "https://relay.example.com/v1",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "exactly one upstream protocol")
		require.Contains(t, err.Error(), APIProtocolAnthropic)
		require.Contains(t, err.Error(), APIProtocolResponses)
	})

	t.Run("去掉首尾空白与末尾斜杠", func(t *testing.T) {
		got, err := NormalizeProtocolEndpoints(map[string]string{
			"  " + APIProtocolAnthropic + " ": "  https://relay.example.com/  ",
		})
		require.NoError(t, err)
		require.Equal(t, map[string]string{APIProtocolAnthropic: "https://relay.example.com"}, got)
	})

	t.Run("不补任何路径后缀", func(t *testing.T) {
		got, err := NormalizeProtocolEndpoints(map[string]string{
			APIProtocolAnthropic: "https://relay.example.com/claude",
		})
		require.NoError(t, err)
		require.Equal(t, "https://relay.example.com/claude", got[APIProtocolAnthropic])
	})

	t.Run("未知协议报错", func(t *testing.T) {
		_, err := NormalizeProtocolEndpoints(map[string]string{"grpc": "https://relay.example.com"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "grpc")
	})

	t.Run("adaptive 不是具体协议，不能作为键", func(t *testing.T) {
		_, err := NormalizeProtocolEndpoints(map[string]string{"adaptive": "https://relay.example.com"})
		require.Error(t, err)
	})

	t.Run("空地址报错", func(t *testing.T) {
		_, err := NormalizeProtocolEndpoints(map[string]string{APIProtocolAnthropic: "   "})
		require.Error(t, err)
	})

	// 2026-10-04 UI E2E：「not a url」原来也能存，渠道显示正常、可调度，到转发时才失败。
	t.Run("地址必须是带主机名的 http(s) 地址", func(t *testing.T) {
		for _, bad := range []string{"not a url", "relay.example.com/v1", "ftp://relay.example.com", "https://", "http:///v1"} {
			_, err := NormalizeProtocolEndpoints(map[string]string{APIProtocolChatCompletions: bad})
			require.Error(t, err, bad)
			require.Contains(t, err.Error(), "must be an http(s) URL with a host", bad)
		}
		for _, good := range []string{"http://127.0.0.1:9/v1", "https://api.moonshot.cn/anthropic"} {
			_, err := NormalizeProtocolEndpoints(map[string]string{APIProtocolChatCompletions: good})
			require.NoError(t, err, good)
		}
	})
}

// TestPlatformProtocolDefaultsOnlyDomesticProviders 官方预填只有国产厂商、OpenCode 与官方视频厂商：海外四家只认成品号，
// 不再预填官方地址（2026-09-29）。
func TestPlatformProtocolDefaultsOnlyDomesticProviders(t *testing.T) {
	require.ElementsMatch(t, []string{PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax, PlatformOpenCodeGo, PlatformSeedance, PlatformKling},
		PlatformsWithProtocolDefaults())
	for _, platform := range []string{PlatformAnthropic, PlatformOpenAI, PlatformGemini, PlatformGrok} {
		for _, mode := range ProtocolDefaultsModes() {
			require.Emptyf(t, PlatformProtocolDefaults(platform, mode), "%s/%s", platform, mode)
		}
	}
}

// 官方视频地址按站点（cn / intl）各一个，只有 video 协议；MiniMax 的默认 / 套餐模式仍是对话地址。
func TestPlatformProtocolDefaultsVideoSites(t *testing.T) {
	for _, tc := range []struct{ platform, cn, intl string }{
		{PlatformSeedance, "https://ark.cn-beijing.volces.com/api/v3", "https://ark.ap-southeast.bytepluses.com/api/v3"},
		{PlatformKling, "https://api-beijing.klingai.com", "https://api-singapore.klingai.com"},
		{PlatformMiniMax, "https://api.minimax.cn", "https://api.minimax.io"},
	} {
		require.Equal(t, map[string]string{APIProtocolVideo: tc.cn}, PlatformProtocolDefaults(tc.platform, AccountModeCN), tc.platform)
		require.Equal(t, map[string]string{APIProtocolVideo: tc.intl}, PlatformProtocolDefaults(tc.platform, AccountModeIntl), tc.platform)
	}
	for _, platform := range []string{PlatformSeedance, PlatformKling} {
		for _, mode := range []string{"", AccountModeCoding, AccountModeZen, AccountModeGo} {
			require.Emptyf(t, PlatformProtocolDefaults(platform, mode), "%s/%s 只有视频站点", platform, mode)
		}
	}
	require.NotContains(t, PlatformProtocolDefaults(PlatformMiniMax, ""), APIProtocolVideo)
	require.Contains(t, PlatformProtocolDefaults(PlatformMiniMax, ""), APIProtocolChatCompletions)
	for _, platform := range []string{PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformOpenCodeGo} {
		require.Empty(t, PlatformProtocolDefaults(platform, AccountModeCN), platform)
	}
	require.Equal(t, []string{PlatformKling, PlatformMiniMax, PlatformSeedance}, OfficialVideoVendors())
}

// TestPlatformProtocolDefaultsCoverRoutedProtocols 固定官方预填与转发路径的一致性：
// 预填缺了某个协议，按预填建出来的 key 走到该协议时就会报缺地址。
func TestPlatformProtocolDefaultsCoverRoutedProtocols(t *testing.T) {
	modes := []string{"", AccountModeCoding, AccountModeGo}
	for _, platform := range PlatformsWithProtocolDefaults() {
		if platform == PlatformSeedance || platform == PlatformKling {
			continue
		}
		for _, mode := range modes {
			defaults := PlatformProtocolDefaults(platform, mode)
			require.NotEmptyf(t, defaults, "%s/%s 应有官方预填", platform, mode)

			// 官方提供 Responses 端点的厂商必须预填 responses 地址，否则 Responses 入站只能转换。
			if hasStatelessVendorResponses(platform) {
				require.Containsf(t, defaults, APIProtocolResponses, "%s/%s 支持原生 Responses 却未预填", platform, mode)
			}
		}
	}
}
