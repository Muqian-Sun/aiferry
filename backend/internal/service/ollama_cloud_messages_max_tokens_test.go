//go:build unit

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// Anthropic Messages 出站（三个 builder）的 Ollama Cloud DeepSeek max_tokens clamp
// 接线验证。实测背景：POST ollama.com/v1/messages（Bearer）max_tokens=256000 被上游
// 以 "max_tokens (256000) exceeds model's maximum output tokens (65536)" 400 拒绝。
// 用例直接调用各 builder 实际生成 *http.Request，同时检查出站 URL 与 wire body。

func messagesClampTestConfig() *config.Config {
	cfg := rawChatCompletionsTestConfig()
	cfg.Gateway = config.GatewayConfig{MaxLineSize: defaultMaxLineSize}
	return cfg
}

func newMessagesClampTestContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	return c
}

// messagesClampOllamaAccount 构造挂在实际 ollama.com 上游的 APIKey 账号。
// builder A 的 Messages base 取 GetBaseURL()，builder C 的 Anthropic 协议 base
// 取 anthropic 协议地址，二者同源。
func messagesClampOllamaAccount(id int64, platform string) *Account {
	return &Account{
		ID:       id,
		Name:     "ollama-cloud-messages",
		Platform: platform,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://ollama.com",
		},
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://ollama.com"},
		Extra:             map[string]any{},
	}
}

func messagesClampBody(model string, maxTokens int) []byte {
	return []byte(`{"model":"` + model + `","max_tokens":` + strconv.Itoa(maxTokens) +
		`,"stream":false,"messages":[{"role":"user","content":"hi"}]}`)
}

// TestBuildUpstreamRequest_ClampsOllamaCloudDeepSeekMaxTokens 覆盖 GatewayService
// .buildUpstreamRequest（Anthropic 平台原生 Messages + 共享重试路径）。
func TestBuildUpstreamRequest_ClampsOllamaCloudDeepSeekMaxTokens(t *testing.T) {
	svc := &GatewayService{cfg: messagesClampTestConfig()}
	c := newMessagesClampTestContext(t)

	body := messagesClampBody("deepseek-v4-flash", 256000)
	account := messagesClampOllamaAccount(401, PlatformAnthropic)

	req, wireBody, err := svc.buildUpstreamRequest(
		context.Background(), c, account, body, "sk-test", "api_key",
		"deepseek-v4-flash", false, false,
	)
	require.NoError(t, err)
	require.Equal(t, "https://ollama.com/v1/messages?beta=true", req.URL.String())
	require.Equal(t, "deepseek-v4-flash", gjson.GetBytes(wireBody, "model").String())
	require.Equal(t, int64(65535), gjson.GetBytes(wireBody, "max_tokens").Int())

	t.Run("official anthropic base is untouched", func(t *testing.T) {
		official := messagesClampOllamaAccount(402, PlatformAnthropic)
		official.Credentials["base_url"] = "https://api.anthropic.com"
		official.ProtocolEndpoints = map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"}
		_, wire, err := svc.buildUpstreamRequest(
			context.Background(), c, official, body, "sk-test", "api_key",
			"deepseek-v4-flash", false, false,
		)
		require.NoError(t, err)
		require.Equal(t, int64(256000), gjson.GetBytes(wire, "max_tokens").Int())
	})

	t.Run("non deepseek model untouched", func(t *testing.T) {
		nonDeepSeek := messagesClampBody("k3-256k", 256000)
		_, wire, err := svc.buildUpstreamRequest(
			context.Background(), c, account, nonDeepSeek, "sk-test", "api_key",
			"k3-256k", false, false,
		)
		require.NoError(t, err)
		require.Equal(t, int64(256000), gjson.GetBytes(wire, "max_tokens").Int())
	})

	t.Run("at cap and extra cap zero untouched", func(t *testing.T) {
		atCap := messagesClampBody("deepseek-v4-flash", 65535)
		_, wire, err := svc.buildUpstreamRequest(
			context.Background(), c, account, atCap, "sk-test", "api_key",
			"deepseek-v4-flash", false, false,
		)
		require.NoError(t, err)
		require.Equal(t, int64(65535), gjson.GetBytes(wire, "max_tokens").Int())

		disabled := messagesClampOllamaAccount(403, PlatformAnthropic)
		disabled.Extra[OllamaCloudMaxTokensCapExtraKey] = 0
		_, wire, err = svc.buildUpstreamRequest(
			context.Background(), c, disabled, body, "sk-test", "api_key",
			"deepseek-v4-flash", false, false,
		)
		require.NoError(t, err)
		require.Equal(t, int64(256000), gjson.GetBytes(wire, "max_tokens").Int())
	})
}

func TestBuildUpstreamRequest_ClampsTrailingSlashOllamaBase(t *testing.T) {
	svc := &GatewayService{cfg: messagesClampTestConfig()}
	c := newMessagesClampTestContext(t)
	body := messagesClampBody("deepseek-v4-flash", 256000)

	newAccount := func(id int64) *Account {
		account := messagesClampOllamaAccount(id, PlatformAnthropic)
		account.Credentials["base_url"] = "https://ollama.com/"
		account.ProtocolEndpoints = map[string]string{APIProtocolAnthropic: "https://ollama.com/"}
		return account
	}

	t.Run("builder A trailing slash base is clamped", func(t *testing.T) {
		req, wireBody, err := svc.buildUpstreamRequest(
			context.Background(), c, newAccount(441), body, "sk-test", "api_key",
			"deepseek-v4-flash", false, false,
		)
		require.NoError(t, err)
		require.Equal(t, "https://ollama.com/v1/messages?beta=true", req.URL.String())
		require.Equal(t, int64(65535), gjson.GetBytes(wireBody, "max_tokens").Int())
	})

	t.Run("evil suffix and custom path never match", func(t *testing.T) {
		for _, base := range []string{"https://ollama.com.evil.com/", "https://ollama.com/anthropic/"} {
			account := newAccount(443)
			account.ProtocolEndpoints = map[string]string{APIProtocolAnthropic: base}
			_, wireBody, err := svc.buildUpstreamRequest(
				context.Background(), c, account, body, "sk-test", "api_key",
				"deepseek-v4-flash", false, false,
			)
			require.NoError(t, err)
			require.Equal(t, int64(256000), gjson.GetBytes(wireBody, "max_tokens").Int(),
				"base %q 不得被识别为 Ollama Cloud", base)
		}
	})
}
