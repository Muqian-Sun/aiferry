//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 渠道用量查询不按标签分流第三方 key：贴 gemini / grok 标签的 key 与其它中转一样走通用逻辑（key 不支持用量查询），
// Gemini / Grok 的专用用量只对成品号（2026-09-29 定）。
func TestGetUsageForAccount_KeyIgnoresPlatformLabel(t *testing.T) {
	svc := &AccountUsageService{}
	ctx := context.Background()
	for _, label := range []string{PlatformGemini, PlatformGrok, PlatformOpenAI} {
		key := &Account{ID: 9401, Platform: label, Type: AccountTypeAPIKey, Status: StatusActive,
			ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://relay.example/v1"}}
		usage, err := svc.GetUsageForAccount(ctx, key)
		require.Nil(t, usage, "label=%s", label)
		require.ErrorContains(t, err, "does not support usage query", "label=%s: key 按中转，不进 %s 专用用量", label, label)
	}

	for _, platform := range []string{PlatformGemini, PlatformGrok} {
		usage, err := svc.GetUsageForAccount(ctx, &Account{ID: 9402, Platform: platform, Type: AccountTypeOAuth, Status: StatusActive})
		require.NoError(t, err, "platform=%s 成品号仍走专用用量", platform)
		require.NotNil(t, usage, "platform=%s", platform)
	}
}
