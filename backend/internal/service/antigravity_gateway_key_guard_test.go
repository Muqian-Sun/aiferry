//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 第三方 key 不能进入 Antigravity v1internal：即使标签是 antigravity、凭据里凑齐了
// 令牌与项目号，入口也要以非 failover 错误拒绝，且不发出任何上游请求。

func antigravityLabelledKeyForGuardTest() *Account {
	return &Account{
		ID:          9101,
		Name:        "antigravity-labelled-key",
		Platform:    PlatformAntigravity,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":      "relay-key",
			"access_token": "token",
			antigravityProjectIDFallbackCredentialKey: "project",
			"model_mapping": map[string]any{
				"gemini-2.5-flash":  "gemini-2.5-flash",
				"claude-sonnet-4-5": "claude-sonnet-4-5",
			},
		},
		ProtocolEndpoints: map[string]string{
			APIProtocolAnthropic: "https://anthropic-relay.example.com",
			APIProtocolGemini:    "https://gemini-relay.example.com",
		},
	}
}

func newAntigravityGuardTestService(upstream HTTPUpstream) *AntigravityGatewayService {
	return &AntigravityGatewayService{
		settingService: NewSettingService(&antigravitySettingRepoStub{}, &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}),
		tokenProvider:  &AntigravityTokenProvider{},
		httpUpstream:   upstream,
	}
}

func requireAntigravityKeyRejected(t *testing.T, err error, rec *httptest.ResponseRecorder, upstream *queuedHTTPUpstreamStub) {
	t.Helper()
	require.Error(t, err)
	require.Contains(t, err.Error(), "third-party key")
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr), "a misrouted key must not be hidden behind account failover")
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Empty(t, upstream.requestBodies)
}

func TestAntigravityGatewayService_Forward_RejectsThirdPartyKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	upstream := &queuedHTTPUpstreamStub{}

	result, err := newAntigravityGuardTestService(upstream).Forward(context.Background(), c, antigravityLabelledKeyForGuardTest(), body, false)

	require.Nil(t, result)
	requireAntigravityKeyRejected(t, err, rec, upstream)
}

func TestAntigravityGatewayService_ForwardGemini_RejectsThirdPartyKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-flash:generateContent", bytes.NewReader(body))
	upstream := &queuedHTTPUpstreamStub{}

	result, err := newAntigravityGuardTestService(upstream).ForwardGemini(context.Background(), c, antigravityLabelledKeyForGuardTest(), "gemini-2.5-flash", "generateContent", false, body, false)

	require.Nil(t, result)
	requireAntigravityKeyRejected(t, err, rec, upstream)
}

func TestAntigravityThirdPartyKeyError_SubscriptionPasses(t *testing.T) {
	require.NoError(t, antigravityThirdPartyKeyError(&Account{Platform: PlatformAntigravity, Type: AccountTypeOAuth}))
	require.NoError(t, antigravityThirdPartyKeyError(nil))
	require.Error(t, antigravityThirdPartyKeyError(&Account{Platform: PlatformAntigravity, Type: AccountTypeAPIKey}))
}
