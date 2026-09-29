//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 上游 503 一律直接换下一个渠道（2026-09-29 muqian 定）：Gemini 三个入口（Claude 兼容 Forward、
// 原生 ForwardNative、Chat Completions 兼容）收到 503 时上游只调用 1 次，返回 503 的 failover 错误交 handler 换号。
// 旧逻辑在原渠道最多重试 5 次、退避 1 / 2 / 4 / 8s。

func newGemini503SwitchTestService(upstream HTTPUpstream) *GeminiMessagesCompatService {
	return &GeminiMessagesCompatService{
		httpUpstream:     upstream,
		cfg:              &config.Config{},
		rateLimitService: NewRateLimitService(&errorPolicyRepoStub{}, nil, &config.Config{}, nil, nil),
	}
}

func gemini503SwitchAccount() *Account {
	return &Account{
		ID:                702,
		Platform:          PlatformGemini,
		Type:              AccountTypeAPIKey,
		Status:            StatusActive,
		ProtocolEndpoints: map[string]string{APIProtocolGemini: "https://generativelanguage.googleapis.com"},
		Credentials:       map[string]any{"api_key": "test-key"},
	}
}

func TestGemini503SwitchesAccountAfterSingleUpstreamCall(t *testing.T) {
	gin.SetMode(gin.TestMode)
	claudeBody := []byte(`{"model":"gemini-2.5-flash","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	nativeBody := []byte(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`)
	chatBody := []byte(`{"model":"gemini-2.5-flash","messages":[{"role":"user","content":"hello"}]}`)
	entries := []struct {
		name    string
		path    string
		body    []byte
		forward func(svc *GeminiMessagesCompatService, c *gin.Context) error
	}{
		{name: "Forward", path: "/v1/messages", body: claudeBody, forward: func(svc *GeminiMessagesCompatService, c *gin.Context) error {
			_, err := svc.Forward(context.Background(), c, gemini503SwitchAccount(), claudeBody)
			return err
		}},
		{name: "ForwardNative", path: "/v1beta/models/gemini-2.5-flash:generateContent", body: nativeBody, forward: func(svc *GeminiMessagesCompatService, c *gin.Context) error {
			_, err := svc.ForwardNative(context.Background(), c, gemini503SwitchAccount(), "gemini-2.5-flash", "generateContent", false, nativeBody)
			return err
		}},
		{name: "ForwardAsChatCompletions", path: "/v1/chat/completions", body: chatBody, forward: func(svc *GeminiMessagesCompatService, c *gin.Context) error {
			_, err := svc.ForwardAsChatCompletions(context.Background(), c, gemini503SwitchAccount(), chatBody)
			return err
		}},
	}
	for _, entry := range entries {
		t.Run(entry.name, func(t *testing.T) {
			upstream := &countingFixedUpstream{status: http.StatusServiceUnavailable, body: `{"error":{"code":503,"message":"The model is overloaded. Please try again later.","status":"UNAVAILABLE"}}`}
			svc := newGemini503SwitchTestService(upstream)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, entry.path, strings.NewReader(string(entry.body)))

			start := time.Now()
			err := entry.forward(svc, c)
			elapsed := time.Since(start)

			var failoverErr *UpstreamFailoverError
			require.True(t, errors.As(err, &failoverErr), "503 应返回 failover 错误交 handler 换号，实际 err=%v", err)
			require.Equal(t, http.StatusServiceUnavailable, failoverErr.StatusCode)
			require.Equal(t, 1, upstream.callCount(), "503 不得在原渠道重试")
			require.Less(t, elapsed, 500*time.Millisecond, "503 不得在原渠道退避")
		})
	}
}
