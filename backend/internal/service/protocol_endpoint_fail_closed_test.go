//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestThirdPartyKeyWithoutEndpointFailsClosed 固定「第三方 key 缺协议地址即报错」。
//
// 每一行对应一个取址点。历史上这些点在地址为空时会静默回落官方端点，把第三方 key
// 发给官方域——请求不报错，只是打错了上游。任何一行回落，这里就会拿到 nil 错误或
// 其他错误码而变红。
func TestThirdPartyKeyWithoutEndpointFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	body := []byte(`{"model":"m","input":"hi","messages":[{"role":"user","content":"hi"}]}`)

	apiKey := func(platform string) *Account {
		return &Account{
			ID:          4242,
			Platform:    platform,
			Type:        AccountTypeAPIKey,
			Concurrency: 1,
			Credentials: map[string]any{"api_key": "sk-third-party"},
		}
	}
	ginCtx := func() *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		return c
	}
	cfg := &config.Config{}
	gateway := &GatewayService{cfg: cfg}
	openai := &OpenAIGatewayService{cfg: cfg}
	accountTest := &AccountTestService{cfg: cfg}

	cases := []struct {
		name string
		call func() error
	}{
		{"anthropic messages", func() error {
			_, _, err := gateway.buildUpstreamRequest(ctx, ginCtx(), apiKey(PlatformAnthropic), body, "tok", "apikey", "m", false, false)
			return err
		}},
		{"anthropic count_tokens", func() error {
			_, _, err := gateway.buildCountTokensRequest(ctx, ginCtx(), apiKey(PlatformAnthropic), body, "tok", "apikey", "m", false)
			return err
		}},
		{"anthropic passthrough messages", func() error {
			_, _, err := gateway.buildUpstreamRequestAnthropicAPIKeyPassthrough(ctx, ginCtx(), apiKey(PlatformAnthropic), body, "tok")
			return err
		}},
		{"anthropic passthrough count_tokens", func() error {
			_, err := gateway.buildCountTokensRequestAnthropicAPIKeyPassthrough(ctx, ginCtx(), apiKey(PlatformAnthropic), body, "tok")
			return err
		}},
		{"openai responses", func() error {
			_, err := openai.buildUpstreamRequest(ctx, ginCtx(), apiKey(PlatformOpenAI), body, "tok", false, "", false)
			return err
		}},
		{"openai responses websocket", func() error {
			_, err := openai.buildOpenAIResponsesWSURL(apiKey(PlatformOpenAI))
			return err
		}},
		{"openai passthrough", func() error {
			_, err := openai.buildUpstreamRequestOpenAIPassthrough(ctx, ginCtx(), apiKey(PlatformOpenAI), body, "tok")
			return err
		}},
		{"openai input_tokens", func() error {
			_, err := openai.buildInputTokensUpstreamRequest(ctx, ginCtx(), apiKey(PlatformOpenAI), body, "tok")
			return err
		}},
		{"openai alpha search", func() error {
			_, err := openai.openAIAlphaSearchURL(apiKey(PlatformOpenAI))
			return err
		}},
		{"openai images", func() error {
			_, err := openai.buildOpenAIImagesRequest(ctx, ginCtx(), apiKey(PlatformOpenAI), body, "application/json", "tok", openAIImagesGenerationsEndpoint)
			return err
		}},
		{"gemini aistudio GET", func() error {
			_, err := (&GeminiMessagesCompatService{cfg: cfg}).ForwardAIStudioGET(ctx, apiKey(PlatformGemini), "/v1beta/models")
			return err
		}},
		{"gemini chat completions compat", func() error {
			buildReq, _ := (&GeminiMessagesCompatService{cfg: cfg}).buildGeminiChatCompletionsUpstreamRequestFunc(apiKey(PlatformGemini), "gemini-2.5-pro", body, false, false)
			_, _, err := buildReq(ctx)
			return err
		}},
		{"gemini test connection", func() error {
			_, err := accountTest.buildGeminiAPIKeyRequest(ctx, apiKey(PlatformGemini), "gemini-2.5-pro", body)
			return err
		}},
		{"anthropic model sync", func() error {
			_, err := accountTest.buildAnthropicUpstreamModelsRequest(ctx, apiKey(PlatformAnthropic))
			return err
		}},
		{"openai model sync", func() error {
			_, err := buildOpenAIAPIKeyModelsRequest(ctx, apiKey(PlatformOpenAI), accountTest.validateUpstreamBaseURL)
			return err
		}},
		{"gemini model sync", func() error {
			_, err := accountTest.buildGeminiUpstreamModelsRequest(ctx, apiKey(PlatformGemini))
			return err
		}},
		{"grok model sync", func() error {
			_, err := accountTest.buildGrokUpstreamModelsRequest(ctx, apiKey(PlatformGrok))
			return err
		}},
		{"grok responses", func() error {
			_, err := buildGrokResponsesURL(apiKey(PlatformGrok), cfg)
			return err
		}},
		{"grok chat completions", func() error {
			_, err := buildGrokChatCompletionsURL(apiKey(PlatformGrok), cfg)
			return err
		}},
		{"grok media", func() error {
			_, err := buildGrokMediaURL(apiKey(PlatformGrok), cfg, GrokMediaEndpointImagesGenerations, "")
			return err
		}},
		{"grok voice", func() error {
			_, err := buildGrokVoiceURL(apiKey(PlatformGrok), cfg, "tts")
			return err
		}},
		{"cn coding plan quota", func() error {
			_, err := (&CNProviderQuotaService{cfg: cfg}).queryUsageForAccount(ctx, apiKey(PlatformOpenCodeGo))
			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			require.Error(t, err)
			require.Equal(t, "MISSING_PROTOCOL_ENDPOINT", infraerrors.Reason(err), "got: %v", err)
		})
	}
}

// TestGeminiForwardWithoutEndpointFailsClosed 覆盖 Forward / ForwardNative 内联的 buildReq。
// 这两处的构建错误会被 writeClaudeError 转成 502 文本，原因码不透传，只能按消息断言；
// 关键断言是上游一次都没被调用。
func TestGeminiForwardWithoutEndpointFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newAccount := func() *Account {
		return &Account{ID: 4444, Platform: PlatformGemini, Type: AccountTypeAPIKey, Concurrency: 1,
			Credentials: map[string]any{"api_key": "sk-third-party"}}
	}
	newService := func() (*GeminiMessagesCompatService, *httpUpstreamRecorder) {
		upstream := &httpUpstreamRecorder{err: errors.New("upstream must not be called")}
		return &GeminiMessagesCompatService{cfg: &config.Config{}, httpUpstream: upstream}, upstream
	}

	t.Run("messages compat", func(t *testing.T) {
		svc, upstream := newService()
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
		_, err := svc.Forward(context.Background(), c, newAccount(),
			[]byte(`{"model":"gemini-2.5-pro","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`))
		require.ErrorContains(t, err, "has no gemini upstream address configured")
		require.Nil(t, upstream.lastReq)
	})

	t.Run("native", func(t *testing.T) {
		svc, upstream := newService()
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-pro:generateContent", nil)
		_, err := svc.ForwardNative(context.Background(), c, newAccount(), "gemini-2.5-pro", "generateContent", false,
			[]byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
		require.ErrorContains(t, err, "has no gemini upstream address configured")
		require.Nil(t, upstream.lastReq)
	})
}

// TestUpstreamBillingProbeWithoutEndpointRecordsMissingEndpoint 计费探测不向官方域兜底：
// 缺地址记为独立失败原因，且不发出任何请求。
func TestUpstreamBillingProbeWithoutEndpointRecordsMissingEndpoint(t *testing.T) {
	account := &Account{
		ID:          4343,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Credentials: map[string]any{"api_key": "sk-third-party"},
	}
	repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}
	upstream := &httpUpstreamRecorder{}
	svc := newUpstreamBillingProbeTestService(repo, upstream, &upstreamBillingProbeSettingRepo{})

	snapshot, err := svc.ProbeAccount(context.Background(), account.ID)

	require.NoError(t, err)
	require.Equal(t, "missing_protocol_endpoint", snapshot.LastError)
	require.Nil(t, upstream.lastReq)
}
