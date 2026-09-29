//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 请求头覆写对任何标签的第三方 key 生效，也包括走 Gemini 协议地址的转发路径。

type geminiKeyHeaderRecorder struct {
	requests []*http.Request
	body     string
}

func (u *geminiKeyHeaderRecorder) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.requests = append(u.requests, req)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(u.body)),
	}, nil
}

func (u *geminiKeyHeaderRecorder) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func antigravityLabelledGeminiKeyWithOverrides() *Account {
	return &Account{
		ID:          9401,
		Name:        "antigravity-labelled-gemini-key",
		Platform:    PlatformAntigravity,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":              "relay-key",
			"model_mapping":        map[string]any{"gemini-2.5-flash": "gemini-2.5-flash"},
			credKeyHeaderOverrides: map[string]any{"x-relay-tenant": "tenant-1"},
		},
		ProtocolEndpoints: map[string]string{APIProtocolGemini: "https://gemini-relay.example.com"},
	}
}

const geminiKeyHeaderOverrideResponse = `{"candidates":[{"content":{"role":"model","parts":[{"text":"hi"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":1,"totalTokenCount":4}}`

func TestGeminiCompatForwarding_KeyHeaderOverridesApplyOnEveryAPIKeyPath(t *testing.T) {
	paths := []struct {
		name string
		call func(svc *GeminiMessagesCompatService, c *gin.Context, account *Account) error
	}{
		{"messages", func(svc *GeminiMessagesCompatService, c *gin.Context, account *Account) error {
			_, err := svc.Forward(context.Background(), c, account, []byte(`{"model":"gemini-2.5-flash","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`))
			return err
		}},
		{"native", func(svc *GeminiMessagesCompatService, c *gin.Context, account *Account) error {
			_, err := svc.ForwardNative(context.Background(), c, account, "gemini-2.5-flash", "generateContent", false, []byte(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`))
			return err
		}},
		{"chat_completions", func(svc *GeminiMessagesCompatService, c *gin.Context, account *Account) error {
			_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, []byte(`{"model":"gemini-2.5-flash","messages":[{"role":"user","content":"hello"}]}`))
			return err
		}},
	}
	for _, path := range paths {
		t.Run(path.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
			upstream := &geminiKeyHeaderRecorder{body: geminiKeyHeaderOverrideResponse}
			svc := NewGeminiMessagesCompatService(nil, nil, nil, nil, nil, upstream, nil, &config.Config{})

			require.NoError(t, path.call(svc, c, antigravityLabelledGeminiKeyWithOverrides()))

			require.Len(t, upstream.requests, 1)
			require.True(t, strings.HasPrefix(upstream.requests[0].URL.String(), "https://gemini-relay.example.com/v1beta/models"))
			require.Equal(t, "relay-key", upstream.requests[0].Header.Get("x-goog-api-key"))
			require.Equal(t, "tenant-1", getHeaderRaw(upstream.requests[0].Header, "x-relay-tenant"))
		})
	}
}
