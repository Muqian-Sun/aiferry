//go:build unit

package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

func TestGrokBaseURLForMode(t *testing.T) {
	for _, tc := range []struct {
		mode string
		want string
	}{
		{"api", xai.DefaultBaseURL},
		{"us-east-1", xai.DefaultUSEast1BaseURL},
		{"us-west-2", xai.DefaultUSWest2BaseURL},
		{"eu-west-1", xai.DefaultEUWest1BaseURL},
		{"cli", xai.DefaultCLIBaseURL},
		{"invalid", xai.DefaultCLIBaseURL},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			require.Equal(t, tc.want, GrokBaseURLForMode(tc.mode))
		})
	}
}

// 站点默认区域是代码常量 GrokDefaultBaseURLMode（cli）：成品号一律走 CLI 网关。
func TestResolveGrokBaseURLFollowsSiteModeOnly(t *testing.T) {
	account := &Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Credentials: map[string]any{}}
	require.Equal(t, xai.DefaultCLIBaseURL, resolveGrokBaseURL(account))

	// 账号上残留的地址（官方、区域或中转）都不能覆盖站点级模式。
	for _, stored := range []string{xai.DefaultBaseURL, xai.DefaultEUWest1BaseURL, "https://attacker.invalid/v1"} {
		account.Credentials["base_url"] = stored
		require.Equal(t, xai.DefaultCLIBaseURL, resolveGrokBaseURL(account))
	}
}
