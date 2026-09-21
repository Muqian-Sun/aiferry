//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// 成品号只走厂商官方地址：Gemini OAuth（AI Studio 模式）测试连接不再读 credentials.base_url，
// 残留的地址字段不能把探测请求打到别处。
func TestBuildGeminiOAuthRequest_IgnoresAccountLevelBaseURL(t *testing.T) {
	svc := &AccountTestService{cfg: &config.Config{}, geminiTokenProvider: &GeminiTokenProvider{}}
	account := &Account{
		ID: 7, Platform: PlatformGemini, Type: AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "ya29.test",
			"expires_at":   time.Now().Add(2 * time.Hour).Format(time.RFC3339),
			"base_url":     "https://relay.example.test",
		},
	}

	req, err := svc.buildGeminiOAuthRequest(context.Background(), account, "gemini-2.5-pro", []byte(`{}`))

	require.NoError(t, err)
	require.Equal(t, "generativelanguage.googleapis.com", req.URL.Host)
	require.Equal(t, "Bearer ya29.test", req.Header.Get("Authorization"))
}
