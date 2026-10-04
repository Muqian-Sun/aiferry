//go:build unit

package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 给客户端的「没有可用上游」说法改了以后，运维的错误归类（api_error → routing）仍要认得出（2026-10-04 D5）
func TestNoUpstreamMessagesStillClassifiedAsRouting(t *testing.T) {
	for _, msg := range []string{
		noUpstreamMessage("gpt-5.5"),
		noUpstreamMessage(""),
		profitVetoExhaustedMessage,
		upstreamBusyMessage,
		compactUnsupportedMessage,
	} {
		require.True(t, isOpsNoAvailableAccountMessage(msg), msg)
		require.NotContains(t, msg, "account", "不露内部概念")
	}
	require.Equal(t, "No upstream is currently available for model gpt-5.5. Please try again later.", noUpstreamMessage("gpt-5.5"))
}
