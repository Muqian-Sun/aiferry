//go:build unit

package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 调度快照的精简账号必须保留类型与协议地址：缓存命中路径上，第三方 key 能否承接请求
// （KeyUpstreamProtocolFor）与厂商识别（Vendor）都只读这份投影。
func TestBuildSchedulerMetadataAccount_KeepsKeyTypeAndProtocolEndpoints(t *testing.T) {
	account := service.Account{
		ID:       51,
		Platform: service.PlatformAnthropic,
		Type:     service.AccountTypeAPIKey,
		ProtocolEndpoints: map[string]string{
			service.APIProtocolChatCompletions: "https://relay.example.com/v1",
		},
	}

	got := buildSchedulerMetadataAccount(account)

	require.True(t, got.IsThirdPartyKey())
	require.Equal(t, account.ProtocolEndpoints, got.ProtocolEndpoints)
	require.Equal(t, service.APIProtocolChatCompletions, got.KeyUpstreamProtocolFor(service.PlatformOpenAI, service.APIProtocolChatCompletions))
}
