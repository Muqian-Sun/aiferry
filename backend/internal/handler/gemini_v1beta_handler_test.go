//go:build unit

package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// TestGeminiV1BetaHandler_PlatformRoutingInvariant 验证 GeminiV1BetaModels 的分流判定：
// 只有 Antigravity 成品号走 ForwardGemini（v1internal），第三方 key 不论标签都走 ForwardNative。
func TestGeminiV1BetaHandler_PlatformRoutingInvariant(t *testing.T) {
	tests := []struct {
		name            string
		account         *service.Account
		expectedService string
	}{
		{
			name:            "Gemini成品号使用ForwardNative",
			account:         &service.Account{Platform: service.PlatformGemini, Type: service.AccountTypeOAuth},
			expectedService: "GeminiMessagesCompatService.ForwardNative",
		},
		{
			name:            "Antigravity成品号使用ForwardGemini",
			account:         &service.Account{Platform: service.PlatformAntigravity, Type: service.AccountTypeOAuth},
			expectedService: "AntigravityGatewayService.ForwardGemini",
		},
		{
			name:            "标签为antigravity的第三方key使用ForwardNative",
			account:         &service.Account{Platform: service.PlatformAntigravity, Type: service.AccountTypeAPIKey},
			expectedService: "GeminiMessagesCompatService.ForwardNative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routedService := "GeminiMessagesCompatService.ForwardNative"
			if usesAntigravityV1Internal(tt.account) {
				routedService = "AntigravityGatewayService.ForwardGemini"
			}
			require.Equal(t, tt.expectedService, routedService)
		})
	}
}
