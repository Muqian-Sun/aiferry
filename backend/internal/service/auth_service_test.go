package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsReservedEmail_WeChatSyntheticDomain(t *testing.T) {
	require.True(t, isReservedEmail("wechat-123@wechat-connect.invalid"))
	require.True(t, isReservedEmail("WECHAT-456@WECHAT-CONNECT.INVALID")) // case-insensitive
	require.False(t, isReservedEmail("real@wechat.com"))
}
