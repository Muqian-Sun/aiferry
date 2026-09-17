package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 成品号只走厂商官方地址：存量数据里残留的中继配置不得改变上游地址。
// 要走中转的一律按第三方 key 建号（协议映射）。

func TestAnthropicOAuthIgnoresStoredRelayAddress(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`)

	for _, accountType := range []string{AccountTypeOAuth, AccountTypeSetupToken} {
		t.Run(accountType, func(t *testing.T) {
			account := &Account{
				ID:          7001,
				Platform:    PlatformAnthropic,
				Type:        accountType,
				Concurrency: 1,
				Credentials: map[string]any{"access_token": "oauth-token"},
				Extra: map[string]any{
					"custom_base_url_enabled": true,
					"custom_base_url":         "https://relay.example.com",
				},
				Status:      StatusActive,
				Schedulable: true,
			}
			svc := &GatewayService{cfg: &config.Config{}}
			newCtx := func() *gin.Context {
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
				return c
			}

			req, _, err := svc.buildUpstreamRequest(context.Background(), newCtx(), account, body, "oauth-token", "oauth", "claude-sonnet-4-5", false, false)
			require.NoError(t, err)
			require.Equal(t, claudeAPIURL, req.URL.String())

			countReq, _, err := svc.buildCountTokensRequest(context.Background(), newCtx(), account, body, "oauth-token", "oauth", "claude-sonnet-4-5", false)
			require.NoError(t, err)
			require.Equal(t, claudeAPICountTokensURL, countReq.URL.String())
		})
	}
}

func TestGrokOAuthIgnoresStoredBaseURL(t *testing.T) {
	for _, stored := range []string{"https://relay.example.com/v1", "https://us-east-1.api.x.ai/v1", xai.DefaultCLIBaseURL} {
		t.Run(stored, func(t *testing.T) {
			account := &Account{
				ID:          7002,
				Platform:    PlatformGrok,
				Type:        AccountTypeOAuth,
				Credentials: map[string]any{"access_token": "grok-token", "base_url": stored},
			}

			// 未注入站点设置时取 CLI 官方网关；注入时完全由站点级模式决定，账号不能覆盖。
			require.Equal(t, xai.DefaultCLIBaseURL, account.GetGrokBaseURL())
			require.Equal(t, xai.DefaultBaseURL, account.GetGrokBaseURLOr(xai.DefaultBaseURL))
			require.Equal(t, xai.DefaultEUWest1BaseURL, account.GetGrokBaseURLOr(GrokBaseURLForMode(GrokDefaultBaseURLModeEUWest1)))
		})
	}
}

func TestGeminiSubscriptionIgnoresStoredBaseURL(t *testing.T) {
	for _, accountType := range []string{AccountTypeOAuth, AccountTypeServiceAccount} {
		t.Run(accountType, func(t *testing.T) {
			account := &Account{
				ID:          7003,
				Platform:    PlatformGemini,
				Type:        accountType,
				Credentials: map[string]any{"base_url": "https://relay.example.com"},
			}

			require.Equal(t, "https://generativelanguage.googleapis.com", account.GetGeminiBaseURL("https://generativelanguage.googleapis.com"))
			require.Empty(t, account.PrimaryUpstreamBaseURL(), "成品号没有账号级上游地址")
		})
	}
}
