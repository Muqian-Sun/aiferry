//go:build unit

package service

import (
	"context"
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

// 进程一启动（不经过读后台设置）就用代码里的 Grok 映射选项：
// 没配映射的 Grok 渠道不把 Claude / GPT 模型名映射成 Grok 模型（不换模型），"grok" 别名指向默认文本模型。
func TestGrokDefaultMappingOptionsPublishedAtStartup(t *testing.T) {
	require.Equal(t, xai.ModelMappingOptions{
		DefaultText:          GrokDefaultTextModel,
		EnableCrossClientMap: GrokCrossClientModelMapEnabled,
	}, xai.RuntimeModelMappingOptions())

	account := &Account{Platform: PlatformGrok, Credentials: map[string]any{}}
	requireMappedModel(t, account, "claude-sonnet-4-5", "claude-sonnet-4-5")
	requireMappedModel(t, account, "gpt-5.6", "gpt-5.6")
	requireMappedModel(t, account, "grok", GrokDefaultTextModel)
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

// 流式中途超时的处理：关。不计数，也不动渠道状态。
func TestHandleStreamTimeout_OffByDefault(t *testing.T) {
	counter := &streamTimeoutCounterSpy{}
	svc := &RateLimitService{timeoutCounterCache: counter}
	require.False(t, svc.HandleStreamTimeout(context.Background(), &Account{ID: 1, Platform: PlatformAnthropic}, "claude-sonnet-4-5"))
	require.Zero(t, counter.increments)
}

type streamTimeoutCounterSpy struct{ increments int }

func (s *streamTimeoutCounterSpy) IncrementTimeoutCount(context.Context, int64, int) (int64, error) {
	s.increments++
	return 1, nil
}
func (s *streamTimeoutCounterSpy) GetTimeoutCount(context.Context, int64) (int64, error) {
	return 0, nil
}
func (s *streamTimeoutCounterSpy) ResetTimeoutCount(context.Context, int64) error { return nil }
func (s *streamTimeoutCounterSpy) GetTimeoutCountTTL(context.Context, int64) (time.Duration, error) {
	return 0, nil
}

// 请求整流：成品号遇到 thinking 签名错误要整流重试，API Key 渠道的签名整流关着；budget 整流开。
func TestRectifierPolicy_CodeDefaults(t *testing.T) {
	svc := &GatewayService{}
	body := []byte(`{"type":"error","error":{"type":"invalid_request_error","message":"Invalid ` + "`signature`" + ` in ` + "`thinking`" + ` block"}}`)
	ctx := context.Background()
	require.True(t, svc.shouldRectifySignatureError(ctx, &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}, body, "claude-sonnet-4-5"))
	require.False(t, svc.shouldRectifySignatureError(ctx, &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey}, body, "claude-sonnet-4-5"))
	require.True(t, budgetRectifierEnabled())
}

// Claude 请求转 Gemini（Antigravity）时注入身份补丁，用内置模板。
func TestAntigravityClaudeTransformOptions_IdentityPatchOn(t *testing.T) {
	opts := (&AntigravityGatewayService{}).getClaudeTransformOptions(context.Background())
	require.True(t, opts.EnableIdentityPatch)
	require.Empty(t, opts.IdentityPatch)
}
