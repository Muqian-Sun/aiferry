package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type anthropicHTTPUpstreamRecorder struct {
	lastReq  *http.Request
	lastBody []byte
	resp     *http.Response
	err      error
}

func (u *anthropicHTTPUpstreamRecorder) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	u.lastReq = req
	if req != nil && req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		u.lastBody = b
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(b))
	}
	if u.err != nil {
		return nil, u.err
	}
	return u.resp, nil
}

func (u *anthropicHTTPUpstreamRecorder) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

type streamReadCloser struct {
	payload []byte
	sent    bool
	err     error
}

func (r *streamReadCloser) Read(p []byte) (int, error) {
	if !r.sent {
		r.sent = true
		n := copy(p, r.payload)
		return n, nil
	}
	if r.err != nil {
		return 0, r.err
	}
	return 0, io.EOF
}

func (r *streamReadCloser) Close() error { return nil }

func TestGatewayService_AnthropicOAuth_IgnoresStoredPassthroughKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	svc := &GatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize},
		},
	}
	account := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"anthropic_passthrough": true,
		},
	}

	req, _, err := svc.buildUpstreamRequest(context.Background(), c, account, []byte(`{"model":"claude-3-7-sonnet-20250219"}`), "oauth-token", "oauth", "claude-3-7-sonnet-20250219", true, false)
	require.NoError(t, err)
	require.Equal(t, "Bearer oauth-token", getHeaderRaw(req.Header, "authorization"))
	require.Contains(t, getHeaderRaw(req.Header, "anthropic-beta"), claude.BetaOAuth, "OAuth 链路仍应按原逻辑补齐 oauth beta")
}

func TestGatewayService_AnthropicOAuthMimic_RewritesSystemWithBillingBlock(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name                       string
		body                       string
		wantModel                  string
		wantOriginalSystem         string
		wantOriginalSystemCacheTTL string
		wantMetadataUserID         string
	}{
		{
			name:               "sonnet system array",
			body:               `{"model":"claude-3-5-sonnet-latest","system":[{"type":"text","text":"x-anthropic-billing-header keep"}],"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`,
			wantModel:          "claude-3-5-sonnet-latest",
			wantOriginalSystem: "x-anthropic-billing-header keep",
		},
		{
			name:               "sonnet system string",
			body:               `{"model":"claude-3-5-sonnet-latest","system":"x-anthropic-billing-header keep","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`,
			wantModel:          "claude-3-5-sonnet-latest",
			wantOriginalSystem: "x-anthropic-billing-header keep",
		},
		{
			name:                       "haiku full mimicry",
			body:                       `{"model":"claude-haiku-4-5","metadata":{"user_id":"pi-session-metadata"},"system":[{"type":"text","text":"Pi project instructions","cache_control":{"type":"ephemeral","ttl":"1h"}}],"thinking":{"type":"enabled","budget_tokens":1024},"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`,
			wantModel:                  "claude-haiku-4-5-20251001",
			wantOriginalSystem:         "Pi project instructions",
			wantOriginalSystemCacheTTL: "1h",
			wantMetadataUserID:         "pi-session-metadata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			c.Request.Header.Set("User-Agent", "pi/0.51.0")
			c.Request.Header.Set("Anthropic-Beta", "client-only-beta")

			parsed, err := ParseGatewayRequest(NewRequestBodyRef([]byte(tt.body)), PlatformAnthropic)
			require.NoError(t, err)

			upstream := &anthropicHTTPUpstreamRecorder{
				resp: &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"Content-Type": []string{"application/json"},
						"x-request-id": []string{"rid-oauth-mimic"},
					},
					Body: io.NopCloser(strings.NewReader(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-haiku-4-5-20251001","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":12,"output_tokens":7}}`)),
				},
			}

			cfg := &config.Config{
				Gateway: config.GatewayConfig{
					MaxLineSize: defaultMaxLineSize,
				},
			}
			svc := &GatewayService{
				cfg:                  cfg,
				responseHeaderFilter: compileResponseHeaderFilter(cfg),
				httpUpstream:         upstream,
				rateLimitService:     &RateLimitService{},
				deferredService:      &DeferredService{},
			}

			account := &Account{
				ID:          301,
				Name:        "anthropic-oauth-mimic",
				Platform:    PlatformAnthropic,
				Type:        AccountTypeOAuth,
				Concurrency: 1,
				Credentials: map[string]any{
					"access_token": "oauth-token",
				},
				Status:      StatusActive,
				Schedulable: true,
			}

			result, err := svc.Forward(context.Background(), c, account, parsed)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, upstream.lastReq)
			require.Equal(t, "Bearer oauth-token", getHeaderRaw(upstream.lastReq.Header, "authorization"))
			finalBeta := getHeaderRaw(upstream.lastReq.Header, "anthropic-beta")
			for _, beta := range claude.FullClaudeCodeMimicryBetas() {
				require.Truef(t, anthropicBetaTokensContains(finalBeta, beta), "missing mimic beta %s", beta)
			}
			require.False(t, anthropicBetaTokensContains(finalBeta, "client-only-beta"))
			for key, value := range claude.DefaultHeaders {
				require.Equal(t, value, getHeaderRaw(upstream.lastReq.Header, key), "mimic fingerprint header %s", key)
			}
			require.NotEmpty(t, getHeaderRaw(upstream.lastReq.Header, "x-client-request-id"))

			require.Equal(t, tt.wantModel, gjson.GetBytes(upstream.lastBody, "model").String())
			system := gjson.GetBytes(upstream.lastBody, "system")
			require.True(t, system.Exists())
			require.True(t, system.IsArray(), "system should be an array")
			arr := system.Array()
			require.Len(t, arr, 3, "system array should have billing block + cc prompt block + expansion block")

			billingText := arr[0].Get("text").String()
			require.Contains(t, billingText, "x-anthropic-billing-header:")
			require.Contains(t, billingText, "cc_version="+claude.CLICurrentVersion+".")
			require.Contains(t, billingText, "cc_entrypoint=cli;")

			require.Equal(t, claudeCodeSystemPrompt, arr[1].Get("text").String())
			require.False(t, arr[1].Get("cache_control").Exists(), "身份前缀 block 不应带 cache_control")

			require.Equal(t, claudeCodeSystemPromptExpansion, arr[2].Get("text").String())
			require.Equal(t, "ephemeral", arr[2].Get("cache_control.type").String())

			// 原始 system prompt 应迁移至 messages 中。
			messages := gjson.GetBytes(upstream.lastBody, "messages")
			require.True(t, messages.IsArray())
			firstMsg := messages.Array()[0]
			require.Equal(t, "user", firstMsg.Get("role").String())
			require.Contains(t, firstMsg.Get("content.0.text").String(), tt.wantOriginalSystem)
			if tt.wantOriginalSystemCacheTTL != "" {
				require.Equal(t, "ephemeral", firstMsg.Get("content.0.cache_control.type").String())
				require.Equal(t, tt.wantOriginalSystemCacheTTL, firstMsg.Get("content.0.cache_control.ttl").String())
			} else {
				require.False(t, firstMsg.Get("content.0.cache_control").Exists())
			}

			if tt.wantMetadataUserID != "" {
				require.Equal(t, tt.wantMetadataUserID, gjson.GetBytes(upstream.lastBody, "metadata.user_id").String())
				require.True(t, gjson.GetBytes(upstream.lastBody, "context_management").Exists())
			}
		})
	}
}

func TestGatewayService_AnthropicOAuthRealClaudeCodeHaiku_PreservesClientHeadersAndBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	metadataUserID := FormatMetadataUserID(
		"a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		"550e8400-e29b-41d4-a716-446655440000",
		"123e4567-e89b-42d3-a456-426614174000",
		claude.CLICurrentVersion,
	)
	body := []byte(`{"model":"claude-haiku-4-5-20251001","metadata":{"user_id":` + strconvQuote(metadataUserID) + `},"system":[{"type":"text","text":"Client-owned Claude Code system","cache_control":{"type":"ephemeral"}}],"context_management":{"edits":[{"type":"clear_thinking_20251015","keep":"all"}]},"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)
	parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("User-Agent", "claude-cli/"+claude.CLICurrentVersion+" (external, cli)")
	c.Request.Header.Set("X-Stainless-Package-Version", "real-client-package")
	clientBeta := strings.Join([]string{
		claude.BetaClaudeCode,
		claude.BetaOAuth,
		claude.BetaInterleavedThinking,
		claude.BetaContextManagement,
	}, ",")
	c.Request.Header.Set("Anthropic-Beta", clientBeta)

	upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"msg_real_cc","type":"message","role":"assistant","model":"claude-haiku-4-5-20251001","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":12,"output_tokens":7}}`)),
	}}
	cfg := &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}
	svc := &GatewayService{
		cfg:                  cfg,
		responseHeaderFilter: compileResponseHeaderFilter(cfg),
		httpUpstream:         upstream,
		rateLimitService:     &RateLimitService{},
		deferredService:      &DeferredService{},
	}
	account := &Account{
		ID: 302, Name: "anthropic-real-cc", Platform: PlatformAnthropic, Type: AccountTypeOAuth, Concurrency: 1,
		Credentials: map[string]any{"access_token": "oauth-token"}, Status: StatusActive, Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, parsed)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, c.Request.Header.Get("User-Agent"), getHeaderRaw(upstream.lastReq.Header, "User-Agent"))
	require.Equal(t, "real-client-package", getHeaderRaw(upstream.lastReq.Header, "X-Stainless-Package-Version"))
	require.Equal(t, clientBeta, getHeaderRaw(upstream.lastReq.Header, "anthropic-beta"))
	require.Empty(t, getHeaderRaw(upstream.lastReq.Header, "x-client-request-id"), "真实 CC 不应被强制写入 mimic request id")
	require.Equal(t, gjson.GetBytes(body, "system").Raw, gjson.GetBytes(upstream.lastBody, "system").Raw)
	require.Equal(t, gjson.GetBytes(body, "messages").Raw, gjson.GetBytes(upstream.lastBody, "messages").Raw)
	require.Equal(t, metadataUserID, gjson.GetBytes(upstream.lastBody, "metadata.user_id").String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "context_management").Exists())
	require.NotContains(t, string(upstream.lastBody), "x-anthropic-billing-header:")
}

func TestGatewayService_ParseSSEUsagePassthrough_MessageStartFallbacks(t *testing.T) {
	usage := &ClaudeUsage{}
	data := `{"type":"message_start","message":{"usage":{"input_tokens":12,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"cached_tokens":9,"cache_creation":{"ephemeral_5m_input_tokens":3,"ephemeral_1h_input_tokens":4}}}}`

	parseSSEUsagePassthrough(data, usage)

	require.Equal(t, 12, usage.InputTokens)
	require.Equal(t, 9, usage.CacheReadInputTokens, "应兼容 cached_tokens 字段")
	require.Equal(t, 7, usage.CacheCreationInputTokens, "聚合字段为空时应从 5m/1h 明细回填")
	require.Equal(t, 3, usage.CacheCreation5mTokens)
	require.Equal(t, 4, usage.CacheCreation1hTokens)
}

func TestGatewayService_ParseSSEUsagePassthrough_MessageDeltaSelectiveOverwrite(t *testing.T) {
	usage := &ClaudeUsage{}
	start := `{"type":"message_start","message":{"usage":{"input_tokens":10,"cache_creation_input_tokens":463184,"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":463184}}}}`
	parseSSEUsagePassthrough(start, usage)

	data := `{"type":"message_delta","usage":{"input_tokens":0,"output_tokens":5,"cache_creation_input_tokens":463184,"cache_read_input_tokens":0,"cached_tokens":11,"cache_creation":{"ephemeral_5m_input_tokens":463184,"ephemeral_1h_input_tokens":0}}}`

	parseSSEUsagePassthrough(data, usage)

	require.Equal(t, 10, usage.InputTokens, "message_delta 中 0 值不应覆盖已有 input_tokens")
	require.Equal(t, 5, usage.OutputTokens)
	require.Equal(t, 463184, usage.CacheCreationInputTokens)
	require.Equal(t, 11, usage.CacheReadInputTokens, "cache_read_input_tokens 为空时应回退到 cached_tokens")
	require.Equal(t, 463184, usage.CacheCreation5mTokens)
	require.Equal(t, 0, usage.CacheCreation1hTokens)
}

func TestGatewayService_ParseSSEUsagePassthrough_NoopCases(t *testing.T) {

	usage := &ClaudeUsage{InputTokens: 3}
	parseSSEUsagePassthrough("", usage)
	require.Equal(t, 3, usage.InputTokens)

	parseSSEUsagePassthrough("[DONE]", usage)
	require.Equal(t, 3, usage.InputTokens)

	parseSSEUsagePassthrough("not-json", usage)
	require.Equal(t, 3, usage.InputTokens)

	// nil usage 不应 panic
	parseSSEUsagePassthrough(`{"type":"message_start"}`, nil)
}

func TestGatewayService_ParseSSEUsagePassthrough_FallbackFromUsageNode(t *testing.T) {
	usage := &ClaudeUsage{}
	data := `{"type":"content_block_delta","usage":{"cached_tokens":6,"cache_creation":{"ephemeral_5m_input_tokens":2,"ephemeral_1h_input_tokens":1}}}`

	parseSSEUsagePassthrough(data, usage)

	require.Equal(t, 6, usage.CacheReadInputTokens)
	require.Equal(t, 3, usage.CacheCreationInputTokens)
}

func TestParseClaudeUsageFromResponseBody(t *testing.T) {
	t.Run("empty or missing usage", func(t *testing.T) {
		got := parseClaudeUsageFromResponseBody(nil)
		require.NotNil(t, got)
		require.Equal(t, 0, got.InputTokens)

		got = parseClaudeUsageFromResponseBody([]byte(`{"id":"x"}`))
		require.NotNil(t, got)
		require.Equal(t, 0, got.OutputTokens)
	})

	t.Run("parse all usage fields and fallback", func(t *testing.T) {
		body := []byte(`{"usage":{"input_tokens":21,"output_tokens":34,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"cached_tokens":13,"cache_creation":{"ephemeral_5m_input_tokens":5,"ephemeral_1h_input_tokens":8}}}`)
		got := parseClaudeUsageFromResponseBody(body)
		require.Equal(t, 21, got.InputTokens)
		require.Equal(t, 34, got.OutputTokens)
		require.Equal(t, 13, got.CacheReadInputTokens, "cache_read_input_tokens 为空时应回退 cached_tokens")
		require.Equal(t, 13, got.CacheCreationInputTokens, "聚合字段为空时应由 5m/1h 回填")
		require.Equal(t, 5, got.CacheCreation5mTokens)
		require.Equal(t, 8, got.CacheCreation1hTokens)
	})

	t.Run("keep explicit aggregate values", func(t *testing.T) {
		body := []byte(`{"usage":{"input_tokens":1,"output_tokens":2,"cache_creation_input_tokens":9,"cache_read_input_tokens":7,"cached_tokens":99,"cache_creation":{"ephemeral_5m_input_tokens":4,"ephemeral_1h_input_tokens":5}}}`)
		got := parseClaudeUsageFromResponseBody(body)
		require.Equal(t, 9, got.CacheCreationInputTokens, "已显式提供聚合字段时不应被明细覆盖")
		require.Equal(t, 7, got.CacheReadInputTokens, "已显式提供 cache_read_input_tokens 时不应回退 cached_tokens")
	})
}
