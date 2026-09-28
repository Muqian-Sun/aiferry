//go:build unit

package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// A2-27 渠道级用户消息限速已删（2026-09-28 P5）：只看部署配置 gateway.user_message_queue.mode
// （GATEWAY_USER_MESSAGE_QUEUE_MODE），渠道上存的 user_msg_queue_mode / user_msg_queue_enabled 不再生效。
func TestGetUserMsgQueueMode_OnlyDeploymentConfig(t *testing.T) {
	parsed, err := service.ParseGatewayRequest(service.NewRequestBodyRef([]byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"hello"}]}`)), service.PlatformAnthropic)
	require.NoError(t, err)
	require.True(t, service.IsRealUserMessage(parsed))
	account := &service.Account{Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth, Extra: map[string]any{
		"user_msg_queue_mode":    config.UMQModeSerialize,
		"user_msg_queue_enabled": true,
	}}
	handlerWith := func(mode string) *GatewayHandler {
		cfg := &config.Config{}
		cfg.Gateway.UserMessageQueue.Mode = mode
		return &GatewayHandler{cfg: cfg, userMsgQueueHelper: &UserMsgQueueHelper{}}
	}

	require.Equal(t, "", handlerWith("").getUserMsgQueueMode(account, parsed), "部署配置没开：渠道存的 serialize 不生效")
	require.Equal(t, config.UMQModeThrottle, handlerWith(config.UMQModeThrottle).getUserMsgQueueMode(account, parsed), "部署配置 throttle：不被渠道存的 serialize 覆盖")
}
