//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

// 上游 503 一律直接换下一个渠道（2026-09-29 muqian 定）：Antigravity 三个入口（Claude / Gemini 原生 /
// OpenAI 兼容）收到 503 时上游只调用 1 次、不在原渠道等待或重试，直接返回 503 的 failover 错误交 handler 换号。
// 覆盖普通 503、MODEL_CAPACITY_EXHAUSTED、RATE_LIMIT_EXCEEDED（短 / 长 retryDelay）四种响应体。

// countingFixedUpstream 每次都回同一个状态码与响应体，并计数。
type countingFixedUpstream struct {
	mu     sync.Mutex
	calls  int
	status int
	body   string
}

func (u *countingFixedUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	if req != nil && req.Body != nil {
		_, _ = io.Copy(io.Discard, req.Body)
	}
	u.mu.Lock()
	u.calls++
	u.mu.Unlock()
	return &http.Response{
		StatusCode: u.status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(u.body)),
	}, nil
}

func (u *countingFixedUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func (u *countingFixedUpstream) callCount() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.calls
}

const (
	antigravityPlain503Body = `{"error":{"code":503,"message":"The service is currently unavailable.","status":"UNAVAILABLE"}}`
	// MODEL_CAPACITY_EXHAUSTED：旧逻辑在原渠道每秒重试一次、最多 60 次。
	antigravityModelCapacity503Body = `{"error":{"code":503,"status":"UNAVAILABLE","message":"No capacity available for model claude-sonnet-4-5 on the server","details":[` +
		`{"@type":"type.googleapis.com/google.rpc.ErrorInfo","metadata":{"model":"claude-sonnet-4-5"},"reason":"MODEL_CAPACITY_EXHAUSTED"},` +
		`{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"39s"}]}}`
	// RATE_LIMIT_EXCEEDED + 短 retryDelay：旧逻辑等 1s 后在原渠道智能重试 1 次。
	antigravityRateLimitShort503Body = `{"error":{"code":503,"status":"RESOURCE_EXHAUSTED","details":[` +
		`{"@type":"type.googleapis.com/google.rpc.ErrorInfo","metadata":{"model":"claude-sonnet-4-5"},"reason":"RATE_LIMIT_EXCEEDED"},` +
		`{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"0.5s"}]}}`
	// RATE_LIMIT_EXCEEDED + 长 retryDelay（≥ 7s）：写模型限流 + 换号（现有行为保留，改由上层 handleUpstreamError 写）。
	antigravityRateLimitLong503Body = `{"error":{"code":503,"status":"RESOURCE_EXHAUSTED","details":[` +
		`{"@type":"type.googleapis.com/google.rpc.ErrorInfo","metadata":{"model":"claude-sonnet-4-5"},"reason":"RATE_LIMIT_EXCEEDED"},` +
		`{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"30s"}]}}`
)

type antigravitySwitchEntry struct {
	name    string
	forward func(svc *AntigravityGatewayService, account *Account) error
}

func antigravitySwitchEntries() []antigravitySwitchEntry {
	claudeBody := []byte(`{"model":"claude-sonnet-4-5","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	geminiBody := []byte(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`)
	chatBody := []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"hello"}]}`)
	return []antigravitySwitchEntry{
		{name: "Forward(claude)", forward: func(svc *AntigravityGatewayService, account *Account) error {
			c, _ := newAntigravityCompatContext(http.MethodPost, "/v1/messages", claudeBody)
			_, err := svc.Forward(context.Background(), c, account, claudeBody, false)
			return err
		}},
		{name: "ForwardGemini", forward: func(svc *AntigravityGatewayService, account *Account) error {
			c, _ := newAntigravityCompatContext(http.MethodPost, "/v1beta/models/claude-sonnet-4-5:generateContent", geminiBody)
			_, err := svc.ForwardGemini(context.Background(), c, account, "claude-sonnet-4-5", "generateContent", false, geminiBody, false)
			return err
		}},
		{name: "ForwardAsChatCompletions", forward: func(svc *AntigravityGatewayService, account *Account) error {
			c, _ := newAntigravityCompatContext(http.MethodPost, "/v1/chat/completions", chatBody)
			_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, chatBody, nil)
			return err
		}},
	}
}

func newAntigravitySwitchTestService(repo AccountRepository, upstream HTTPUpstream) *AntigravityGatewayService {
	tokenProvider := NewAntigravityTokenProvider(nil, &antigravityCompatTokenCache{token: "fresh-oauth-token"}, nil)
	return NewAntigravityGatewayService(
		repo, nil, nil, tokenProvider, nil, upstream,
		NewSettingService(&antigravitySettingRepoStub{}, &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}),
		nil,
	)
}

func TestAntigravity503SwitchesAccountAfterSingleUpstreamCall(t *testing.T) {
	bodies := []struct {
		name            string
		body            string
		wantModelLimits bool
	}{
		{name: "plain", body: antigravityPlain503Body},
		{name: "model_capacity_exhausted", body: antigravityModelCapacity503Body},
		{name: "rate_limit_short_delay", body: antigravityRateLimitShort503Body},
		{name: "rate_limit_long_delay", body: antigravityRateLimitLong503Body, wantModelLimits: true},
	}
	for _, entry := range antigravitySwitchEntries() {
		for _, tc := range bodies {
			t.Run(entry.name+"/"+tc.name, func(t *testing.T) {
				repo := &stubAntigravityAccountRepo{}
				upstream := &countingFixedUpstream{status: http.StatusServiceUnavailable, body: tc.body}
				svc := newAntigravitySwitchTestService(repo, upstream)

				start := time.Now()
				err := entry.forward(svc, newAntigravityCompatAccount(AccountTypeOAuth))
				elapsed := time.Since(start)

				var failoverErr *UpstreamFailoverError
				require.True(t, errors.As(err, &failoverErr), "503 应返回 failover 错误交 handler 换号，实际 err=%v", err)
				require.Equal(t, http.StatusServiceUnavailable, failoverErr.StatusCode)
				require.Equal(t, 1, upstream.callCount(), "503 不得在原渠道重试")
				require.Less(t, elapsed, 500*time.Millisecond, "503 不得在原渠道等待")
				if tc.wantModelLimits {
					// 长 retryDelay 的 RATE_LIMIT_EXCEEDED 仍写模型限流（上层 handleUpstreamError），调度会避开该模型。
					require.NotEmpty(t, repo.modelRateLimitCalls, "长 retryDelay 仍应写模型限流")
					require.Equal(t, "claude-sonnet-4-5", repo.modelRateLimitCalls[0].modelKey)
					require.WithinDuration(t, time.Now().Add(30*time.Second), repo.modelRateLimitCalls[0].resetAt, 5*time.Second)
				} else {
					require.Empty(t, repo.modelRateLimitCalls, "不应写模型限流")
				}
				require.Empty(t, repo.rateCalls, "503 不应写账号级限流")
			})
		}
	}
}

// 529 同样不在原渠道重试（这里不挂 RateLimitService，走的是 shouldRetryAntigravityError 分支；
// 生产里 529 先被错误策略拦下同样直接返回）。
func TestAntigravity529SwitchesAccountAfterSingleUpstreamCall(t *testing.T) {
	for _, entry := range antigravitySwitchEntries() {
		t.Run(entry.name, func(t *testing.T) {
			upstream := &countingFixedUpstream{status: 529, body: `{"error":{"code":529,"message":"overloaded"}}`}
			svc := newAntigravitySwitchTestService(&stubAntigravityAccountRepo{}, upstream)

			start := time.Now()
			err := entry.forward(svc, newAntigravityCompatAccount(AccountTypeOAuth))

			var failoverErr *UpstreamFailoverError
			require.True(t, errors.As(err, &failoverErr), "529 应返回 failover 错误，实际 err=%v", err)
			require.Equal(t, 529, failoverErr.StatusCode)
			require.Equal(t, 1, upstream.callCount(), "529 不得在原渠道重试")
			require.Less(t, time.Since(start), 500*time.Millisecond)
		})
	}
}
