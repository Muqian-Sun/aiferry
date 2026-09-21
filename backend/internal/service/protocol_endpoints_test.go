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

	t.Run("四种具体协议都接受", func(t *testing.T) {
		in := map[string]string{
			APIProtocolAnthropic:       "https://relay.example.com",
			APIProtocolChatCompletions: "https://relay.example.com/v1",
			APIProtocolResponses:       "https://relay.example.com",
			APIProtocolGemini:          "https://relay.example.com/gemini",
		}
		got, err := NormalizeProtocolEndpoints(in)
		require.NoError(t, err)
		require.Len(t, got, 4)
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
