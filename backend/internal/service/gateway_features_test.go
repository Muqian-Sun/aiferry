//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 这一组用例断言 gateway_features.go 里写死的值在调用链上真的生效（别处没有用例守着的几项）。

// 没配映射的 Grok 渠道不把 Claude / GPT / Codex 模型名映射成 Grok 模型（不换模型），"grok" 别名指向默认文本模型。
func TestGrokDefaultMappingNeverSubstitutesOtherVendors(t *testing.T) {
	account := &Account{Platform: PlatformGrok, Credentials: map[string]any{}}
	for _, model := range []string{"claude-sonnet-4-5", "gpt-5.6", "codex-mini-latest", "o3"} {
		requireMappedModel(t, account, model, model)
	}
	requireMappedModel(t, account, "grok", xai.DefaultTextModel)
}

// Beta 策略：fast-mode 对任何模型、任何渠道类型都从 anthropic-beta 里去掉，且不拦请求。
func TestBetaPolicy_FastModeFilteredForEveryModel(t *testing.T) {
	svc := &GatewayService{}
	for _, account := range []*Account{
		{Platform: PlatformAnthropic, Type: AccountTypeOAuth},
		{Platform: PlatformAnthropic, Type: AccountTypeAPIKey},
	} {
		for _, model := range []string{"claude-sonnet-5", "claude-opus-4-6", "claude-haiku-4-5"} {
			res := svc.evaluateBetaPolicy(context.Background(), claude.BetaFastMode, account, model)
			require.Nil(t, res.blockErr, "%s/%s", account.Type, model)
			require.Contains(t, res.filterSet, claude.BetaFastMode, "%s/%s", account.Type, model)
		}
	}
}

// metadata 不透传：Claude 成品号出站时按渠道重写 metadata.user_id。
func TestBuildUpstreamRequest_OAuthRewritesMetadataUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	originalUserID := FormatMetadataUserID(
		"d61f76d0730d2b920763648949bad5c79742155c27037fc77ac3f9805cb90169",
		"",
		"7578cf37-aaca-46e4-a45c-71285d9dbb83",
		"2.1.78",
	)
	body := []byte(`{"model":"claude-haiku-4-5","metadata":{"user_id":` + strconvQuote(originalUserID) + `},"messages":[{"role":"user","content":"hi"}]}`)

	svc := &GatewayService{cfg: &config.Config{}}
	svc.identityService = NewIdentityService(&stubIdentityCache{fingerprint: &Fingerprint{
		UserAgent: "claude-cli/2.1.78 (external, cli)", ClientID: "client-xyz", UpdatedAt: time.Now().Unix(),
	}})
	account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Extra: map[string]any{"account_uuid": "acc-uuid"}}

	req, wireBody, err := svc.buildUpstreamRequest(context.Background(), c, account, body, "test-token", "oauth", "claude-haiku-4-5", false, false)
	require.NoError(t, err)
	defer func() { require.NoError(t, req.Body.Close()) }()

	userID := gjson.GetBytes(wireBody, "metadata.user_id").String()
	require.NotEqual(t, originalUserID, userID)
	require.Contains(t, userID, "acc-uuid")
}

// 流式中途超时要处理渠道：每次超时按 10 分钟窗口计数；窗口内第 2 次不动渠道，
// 第 3 次把渠道暂停调度到 now+5 分钟（临时不可调度）并清零计数。
func TestHandleStreamTimeout_PausesAccountOnThirdTimeoutInWindow(t *testing.T) {
	ctx := context.Background()
	account := &Account{ID: 7, Platform: PlatformAnthropic}
	counter := &streamTimeoutCounterSpy{next: 2}
	repo := &transportTempUnschedRepoStub{}
	svc := &RateLimitService{accountRepo: repo, timeoutCounterCache: counter}

	require.False(t, svc.HandleStreamTimeout(ctx, account, "claude-sonnet-4-5"))
	require.Equal(t, 1, counter.increments)
	require.Equal(t, 10, counter.lastWindowMinutes)
	require.Zero(t, repo.calls, "窗口内第 2 次超时不动渠道")

	counter.next = 3
	before := time.Now()
	require.True(t, svc.HandleStreamTimeout(ctx, account, "claude-sonnet-4-5"))
	after := time.Now()
	require.Equal(t, 2, counter.increments)
	require.Equal(t, 10, counter.lastWindowMinutes)
	require.Equal(t, 1, repo.calls, "窗口内第 3 次超时暂停渠道")
	require.Equal(t, account.ID, repo.lastID)
	require.False(t, repo.lastUntil.Before(before.Add(5*time.Minute)), "暂停到 now+5 分钟：until=%s before=%s", repo.lastUntil, before)
	require.False(t, repo.lastUntil.After(after.Add(5*time.Minute)), "暂停到 now+5 分钟：until=%s after=%s", repo.lastUntil, after)
	require.Contains(t, repo.lastReason, "stream_timeout")
	require.Equal(t, 1, counter.resets, "暂停后清零计数")
}

// streamTimeoutCounterSpy 记录计数调用；IncrementTimeoutCount 返回 next（窗口内累计次数）。
type streamTimeoutCounterSpy struct {
	next              int64
	increments        int
	lastWindowMinutes int
	resets            int
}

func (s *streamTimeoutCounterSpy) IncrementTimeoutCount(_ context.Context, _ int64, windowMinutes int) (int64, error) {
	s.increments++
	s.lastWindowMinutes = windowMinutes
	return s.next, nil
}
func (s *streamTimeoutCounterSpy) GetTimeoutCount(context.Context, int64) (int64, error) {
	return 0, nil
}
func (s *streamTimeoutCounterSpy) ResetTimeoutCount(context.Context, int64) error {
	s.resets++
	return nil
}
func (s *streamTimeoutCounterSpy) GetTimeoutCountTTL(context.Context, int64) (time.Duration, error) {
	return 0, nil
}

// 请求整流：成品号和 API Key 渠道遇到 thinking 签名错误都要整流重试；budget 整流开。
func TestRectifierPolicy_CodeDefaults(t *testing.T) {
	svc := &GatewayService{}
	body := []byte(`{"type":"error","error":{"type":"invalid_request_error","message":"Invalid ` + "`signature`" + ` in ` + "`thinking`" + ` block"}}`)
	ctx := context.Background()
	require.True(t, svc.shouldRectifySignatureError(ctx, &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}, body, "claude-sonnet-4-5"))
	require.True(t, svc.shouldRectifySignatureError(ctx, &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey}, body, "claude-sonnet-4-5"))
	require.True(t, budgetRectifierEnabled())
}

// API Key 渠道端到端：上游回 thinking 签名 400 后，去掉 thinking 块重发一次，重发成功就把结果回给客户端。
func TestGatewayForward_APIKeyChannelRetriesAfterThinkingSignatureError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	upstream := &queuedHTTPUpstream{responses: []*http.Response{
		newJSONResponse(http.StatusBadRequest, `{"type":"error","error":{"type":"invalid_request_error","message":"messages.1.content.0: Invalid `+"`signature`"+` in `+"`thinking`"+` block"}}`),
		newJSONResponse(http.StatusOK, `{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":3,"output_tokens":1}}`),
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
		ID:                21,
		Name:              "anthropic-key",
		Platform:          PlatformAnthropic,
		Type:              AccountTypeAPIKey,
		Concurrency:       1,
		Credentials:       map[string]any{"api_key": "upstream-key", "base_url": "https://api.anthropic.com"},
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"},
		Status:            StatusActive,
		Schedulable:       true,
	}
	body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":64,"thinking":{"type":"enabled","budget_tokens":1024},"messages":[` +
		`{"role":"user","content":"hi"},` +
		`{"role":"assistant","content":[{"type":"thinking","thinking":"reasoning","signature":"forged-signature"},{"type":"text","text":"hello"}]},` +
		`{"role":"user","content":"again"}]}`)
	parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
	require.NoError(t, err)

	result, err := svc.Forward(context.Background(), c, account, parsed)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.requests, 2, "签名 400 之后整流重发一次")
	firstBody, err := io.ReadAll(upstream.requests[0].Body)
	require.NoError(t, err)
	require.Contains(t, string(firstBody), "forged-signature")
	retryBody, err := io.ReadAll(upstream.requests[1].Body)
	require.NoError(t, err)
	require.NotContains(t, string(retryBody), "forged-signature", "重发的请求去掉了带签名的 thinking 块")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"text":"ok"`)
}

// Claude 请求转 Gemini（Antigravity）时注入身份补丁，用内置模板。
func TestAntigravityClaudeTransformOptions_IdentityPatchOn(t *testing.T) {
	opts := (&AntigravityGatewayService{}).getClaudeTransformOptions(context.Background())
	require.True(t, opts.EnableIdentityPatch)
	require.Empty(t, opts.IdentityPatch)
}
