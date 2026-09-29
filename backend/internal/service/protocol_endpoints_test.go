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

	t.Run("四种具体协议都接受（各自单独配）", func(t *testing.T) {
		for _, protocol := range UpstreamProtocols() {
			got, err := NormalizeProtocolEndpoints(map[string]string{protocol: "https://relay.example.com"})
			require.NoError(t, err, protocol)
			require.Equal(t, map[string]string{protocol: "https://relay.example.com"}, got)
		}
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
}

// TestPlatformProtocolDefaultsOnlyDomesticProviders 官方预填只有国产厂商与 OpenCode：海外四家只认成品号，
// 不再预填官方地址（2026-09-29）。
func TestPlatformProtocolDefaultsOnlyDomesticProviders(t *testing.T) {
	require.ElementsMatch(t, []string{PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax, PlatformOpenCodeGo}, PlatformsWithProtocolDefaults())
	for _, platform := range []string{PlatformAnthropic, PlatformOpenAI, PlatformGemini, PlatformGrok} {
		for _, mode := range []string{"", AccountModeCoding, AccountModeGo} {
			require.Emptyf(t, PlatformProtocolDefaults(platform, mode), "%s/%s", platform, mode)
		}
	}
}

// TestPlatformProtocolDefaultsCoverRoutedProtocols 固定官方预填与转发路径的一致性：
// 预填缺了某个协议，按预填建出来的 key 走到该协议时就会报缺地址。
func TestPlatformProtocolDefaultsCoverRoutedProtocols(t *testing.T) {
	modes := []string{"", AccountModeCoding, AccountModeGo}
	for _, platform := range PlatformsWithProtocolDefaults() {
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
