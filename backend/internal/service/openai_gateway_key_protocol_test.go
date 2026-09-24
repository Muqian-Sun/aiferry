//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// OpenAI 网关上第三方 key 的协议选择：协议地址决定上游协议，平台标签不参与。
// 这里的账号故意挂和地址不相干的平台标签，证明分流只看地址。

func keyProtocolTestAccount(platform string, endpoints map[string]string) *Account {
	return &Account{
		ID:                701,
		Name:              "key-protocol",
		Platform:          platform,
		Type:              AccountTypeAPIKey,
		Concurrency:       1,
		Credentials:       map[string]any{"api_key": "sk-test"},
		ProtocolEndpoints: endpoints,
	}
}

func adaptiveProtocolTestContext(path string, body []byte) *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

type keyProtocolIngress struct {
	name    string
	path    string
	body    []byte
	forward func(*OpenAIGatewayService, *gin.Context, *Account, []byte) error
	setup   func(*gin.Context)
}

var (
	keyProtocolChatIngress = keyProtocolIngress{
		name: "chat_completions",
		path: "/v1/chat/completions",
		body: []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`),
		forward: func(svc *OpenAIGatewayService, c *gin.Context, account *Account, body []byte) error {
			_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "")
			return err
		},
	}
	keyProtocolResponsesIngress = keyProtocolIngress{
		name: "responses",
		path: "/v1/responses",
		body: []byte(`{"model":"gpt-5.4","input":"hello","stream":false}`),
		forward: func(svc *OpenAIGatewayService, c *gin.Context, account *Account, body []byte) error {
			_, err := svc.Forward(context.Background(), c, account, body)
			return err
		},
	}
	keyProtocolMessagesIngress = keyProtocolIngress{
		name: "messages",
		path: "/v1/messages",
		body: []byte(`{"model":"gpt-5.4","max_tokens":32,"messages":[{"role":"user","content":"hello"}],"stream":false}`),
		forward: func(svc *OpenAIGatewayService, c *gin.Context, account *Account, body []byte) error {
			_, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "")
			return err
		},
	}
)

// captureKeyProtocolRequest 跑一次转发并返回发往上游的全部请求；上游恒返回传输错误，
// 转发停在第一次发送之后。
func captureKeyProtocolRequest(t *testing.T, account *Account, ingress keyProtocolIngress) *httpUpstreamRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{err: errors.New("stop after capture")}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
	c := adaptiveProtocolTestContext(ingress.path, ingress.body)
	if ingress.setup != nil {
		ingress.setup(c)
	}
	err := ingress.forward(svc, c, account, ingress.body)
	require.Error(t, err)
	require.Len(t, upstream.requests, 1, "exactly one upstream request")
	return upstream
}

// requireAnthropicUpstreamRefused 断言 OpenAI 网关不承接 anthropic 上游：解析成
// anthropic 协议时按「路由与转发判定不一致」直接报错，而不是落到 Responses 转换链
// 报一个指错方向的「没配 responses 地址」（handler 侧本该把这类资源交给 Anthropic
// 网关，见 compatForwardTargetFor）。
//
// 只断言错误内容，不断言「没发上游请求」：只配 anthropic 地址的账号无论如何都会在
// 更下游缺地址失败，那条断言去掉守卫也照样过，是空转。
func requireAnthropicUpstreamRefused(t *testing.T, account *Account, ingress keyProtocolIngress) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{err: errors.New("stop after capture")}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
	c := adaptiveProtocolTestContext(ingress.path, ingress.body)
	if ingress.setup != nil {
		ingress.setup(c)
	}
	err := ingress.forward(svc, c, account, ingress.body)
	require.Error(t, err)
	require.Contains(t, err.Error(), "the OpenAI gateway does not serve")
}

func TestOpenAIGatewayKeyProtocol_RelayWithChatAndResponses(t *testing.T) {
	endpoints := map[string]string{
		APIProtocolChatCompletions: "http://relay.example/v1",
		APIProtocolResponses:       "http://relay.example/v1",
	}

	t.Run("chat completions is forwarded raw to chat completions", func(t *testing.T) {
		upstream := captureKeyProtocolRequest(t, keyProtocolTestAccount(PlatformKimi, endpoints), keyProtocolChatIngress)
		require.Equal(t, "http://relay.example/v1/chat/completions", upstream.lastReq.URL.String())
		require.True(t, gjson.GetBytes(upstream.lastBody, "messages").IsArray())
		require.False(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
	})

	t.Run("responses goes to responses", func(t *testing.T) {
		upstream := captureKeyProtocolRequest(t, keyProtocolTestAccount(PlatformKimi, endpoints), keyProtocolResponsesIngress)
		require.Equal(t, "http://relay.example/v1/responses", upstream.lastReq.URL.String())
		require.True(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
	})

	t.Run("messages goes to responses", func(t *testing.T) {
		upstream := captureKeyProtocolRequest(t, keyProtocolTestAccount(PlatformKimi, endpoints), keyProtocolMessagesIngress)
		require.Equal(t, "http://relay.example/v1/responses", upstream.lastReq.URL.String())
		require.True(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
		require.False(t, gjson.GetBytes(upstream.lastBody, "messages").Exists())
	})
}

func TestOpenAIGatewayKeyProtocol_RelayWithOnlyChatCompletions(t *testing.T) {
	endpoints := map[string]string{APIProtocolChatCompletions: "http://relay.example/v1"}

	for _, ingress := range []keyProtocolIngress{keyProtocolResponsesIngress, keyProtocolMessagesIngress} {
		t.Run(ingress.name+" is converted to chat completions", func(t *testing.T) {
			upstream := captureKeyProtocolRequest(t, keyProtocolTestAccount(PlatformOpenAI, endpoints), ingress)
			require.Equal(t, "http://relay.example/v1/chat/completions", upstream.lastReq.URL.String())
			require.True(t, gjson.GetBytes(upstream.lastBody, "messages").IsArray())
			require.False(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
		})
	}
}

func TestOpenAIGatewayKeyProtocol_ResponsesShapedChatFollowsProtocol(t *testing.T) {
	body := []byte(`{"model":"glm-4.7","input":"hello","max_output_tokens":32,"stream":false}`)
	ingress := keyProtocolChatIngress
	ingress.body = body

	t.Run("chat completions upstream receives a converted body", func(t *testing.T) {
		upstream := captureKeyProtocolRequest(t, keyProtocolTestAccount(PlatformZhipu, map[string]string{
			APIProtocolChatCompletions: "http://relay.example",
			APIProtocolResponses:       "http://relay.example",
		}), ingress)
		require.Equal(t, "http://relay.example/v1/chat/completions", upstream.lastReq.URL.String())
		require.True(t, gjson.GetBytes(upstream.lastBody, "messages").IsArray())
		require.False(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
	})

	t.Run("responses upstream keeps the responses body", func(t *testing.T) {
		upstream := captureKeyProtocolRequest(t, keyProtocolTestAccount(PlatformZhipu, map[string]string{
			APIProtocolResponses: "http://relay.example",
		}), ingress)
		require.Equal(t, "http://relay.example/v1/responses", upstream.lastReq.URL.String())
		require.True(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
	})

	t.Run("anthropic upstream is refused", func(t *testing.T) {
		requireAnthropicUpstreamRefused(t, keyProtocolTestAccount(PlatformZhipu, map[string]string{
			APIProtocolAnthropic: "http://anthropic.example",
		}), ingress)
	})
}

// TestOpenAIGatewayKeyProtocol_AnthropicOnlyKey：只配 anthropic 地址的 key 在 OpenAI
// 网关上三种入站全部报错——anthropic 上游由 Anthropic 网关承接，路由不会把这类资源
// 交到这里来（7b-4 删掉了 OpenAI 网关里的 native anthropic 转发）。
func TestOpenAIGatewayKeyProtocol_AnthropicOnlyKey(t *testing.T) {
	endpoints := map[string]string{APIProtocolAnthropic: "http://anthropic.example"}
	for _, ingress := range []keyProtocolIngress{keyProtocolChatIngress, keyProtocolResponsesIngress, keyProtocolMessagesIngress} {
		t.Run(ingress.name, func(t *testing.T) {
			requireAnthropicUpstreamRefused(t, keyProtocolTestAccount(PlatformOpenAI, endpoints), ingress)
		})
	}
}

func TestOpenAIGatewayKeyProtocol_OfficialOpenAIPrefersResponses(t *testing.T) {
	// 标签是 deepseek，地址是官方 OpenAI：按地址识别为 OpenAI，入站 Chat Completions 先转 Responses。
	account := keyProtocolTestAccount(PlatformDeepseek, map[string]string{
		APIProtocolChatCompletions: "https://api.openai.com",
		APIProtocolResponses:       "https://api.openai.com",
	})
	upstream := captureKeyProtocolRequest(t, account, keyProtocolChatIngress)
	require.Equal(t, "https://api.openai.com/v1/responses", upstream.lastReq.URL.String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
}

func TestOpenAIGatewayKeyProtocol_MissingProtocolEndpointFailsWithoutUpstreamRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := keyProtocolTestAccount(PlatformOpenAI, map[string]string{APIProtocolGemini: "http://gemini.example"})
	for _, ingress := range []keyProtocolIngress{keyProtocolChatIngress, keyProtocolResponsesIngress, keyProtocolMessagesIngress} {
		t.Run(ingress.name, func(t *testing.T) {
			upstream := &httpUpstreamRecorder{err: errors.New("must not be called")}
			svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
			err := ingress.forward(svc, adaptiveProtocolTestContext(ingress.path, ingress.body), account, ingress.body)
			require.Error(t, err)
			require.Equal(t, "MISSING_PROTOCOL_ENDPOINT", infraerrors.Reason(err))
			require.Empty(t, upstream.requests)
		})
	}
}

func TestOpenAIGatewayKeyProtocol_DeepSeekQuirksFollowVendorNotLabel(t *testing.T) {
	body := []byte(`{"model":"deepseek-v4","input":"hello","store":true,"max_output_tokens":32,"stream":false}`)
	ingress := keyProtocolResponsesIngress
	ingress.body = body

	t.Run("deepseek label on a relay gets the standard Responses URL and body", func(t *testing.T) {
		upstream := captureKeyProtocolRequest(t, keyProtocolTestAccount(PlatformDeepseek, map[string]string{
			APIProtocolResponses: "http://relay.example",
		}), ingress)
		require.Equal(t, "http://relay.example/v1/responses", upstream.lastReq.URL.String())
		require.True(t, gjson.GetBytes(upstream.lastBody, "store").Bool())
	})

	t.Run("openai label on api.deepseek.com gets the DeepSeek URL shape and stateless body", func(t *testing.T) {
		upstream := captureKeyProtocolRequest(t, keyProtocolTestAccount(PlatformOpenAI, map[string]string{
			APIProtocolChatCompletions: DefaultDeepseekBaseURL,
			APIProtocolResponses:       DefaultDeepseekBaseURL,
		}), ingress)
		require.Equal(t, "https://api.deepseek.com/responses", upstream.lastReq.URL.String())
		require.True(t, gjson.GetBytes(upstream.lastBody, "store").Exists())
		require.False(t, gjson.GetBytes(upstream.lastBody, "store").Bool())
		require.Equal(t, int64(32), gjson.GetBytes(upstream.lastBody, "max_output_tokens").Int())
	})
}

func TestOpenAIGatewayKeyProtocol_StatelessVendorResponses(t *testing.T) {
	ingress := keyProtocolResponsesIngress
	ingress.body = []byte(`{"model":"k3-256k","input":"hello","store":true,"previous_response_id":"resp_old","stream":false}`)
	account := keyProtocolTestAccount(PlatformOpenAI, map[string]string{
		APIProtocolChatCompletions: DefaultKimiCodingBaseURL,
		APIProtocolAnthropic:       DefaultKimiCodingAnthropicBaseURL,
		APIProtocolResponses:       DefaultKimiCodingBaseURL,
	})
	upstream := captureKeyProtocolRequest(t, account, ingress)
	require.Equal(t, "https://api.kimi.com/coding/v1/responses", upstream.lastReq.URL.String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "store").Bool())
	require.False(t, gjson.GetBytes(upstream.lastBody, "previous_response_id").Exists())
}

// TestOpenAIGatewayKeyProtocol_GrokQuirksFollowVendorNotLabel：xAI 的请求改写与 Grok 请求构造
// 只对地址识别为 xAI 的 key 启用。
func TestOpenAIGatewayKeyProtocol_GrokQuirksFollowVendorNotLabel(t *testing.T) {
	relay := func() *Account {
		return keyProtocolTestAccount(PlatformGrok, map[string]string{
			APIProtocolChatCompletions: "http://relay.example/v1",
			APIProtocolResponses:       "http://relay.example/v1",
		})
	}
	official := func() *Account {
		return keyProtocolTestAccount(PlatformOpenAI, map[string]string{
			APIProtocolChatCompletions: xaiOfficialTestBaseURL,
			APIProtocolResponses:       xaiOfficialTestBaseURL,
		})
	}
	chatIngress := keyProtocolChatIngress
	chatIngress.body = []byte(`{"model":"grok-4.5","messages":[{"role":"user","content":"hello"}],"prompt_cache_key":"cache-1","stream":false}`)
	// xAI 缓存身份按网关 API key 隔离，没有 key ID 时不生成。
	chatIngress.setup = func(c *gin.Context) { c.Set("api_key", &APIKey{ID: 9}) }

	t.Run("grok label on a relay uses its chat completions endpoint without xAI body patches", func(t *testing.T) {
		upstream := captureKeyProtocolRequest(t, relay(), chatIngress)
		require.Equal(t, "http://relay.example/v1/chat/completions", upstream.lastReq.URL.String())
		require.Equal(t, "cache-1", gjson.GetBytes(upstream.lastBody, "prompt_cache_key").String())
		require.Empty(t, upstream.lastReq.Header.Get(grokConversationIDHeader))
	})

	t.Run("grok label on a relay with a path prefix joins the versioned chat completions path", func(t *testing.T) {
		// xAI 的 URL 构造在非空路径后直接拼 /chat/completions；key 的 CC 地址按通用规则补版本段。
		account := keyProtocolTestAccount(PlatformGrok, map[string]string{APIProtocolChatCompletions: "http://relay.example/api"})
		upstream := captureKeyProtocolRequest(t, account, chatIngress)
		require.Equal(t, "http://relay.example/api/v1/chat/completions", upstream.lastReq.URL.String())
	})

	t.Run("openai label on api.x.ai gets xAI chat body patches", func(t *testing.T) {
		upstream := captureKeyProtocolRequest(t, official(), chatIngress)
		require.Equal(t, xaiOfficialTestBaseURL+"/chat/completions", upstream.lastReq.URL.String())
		require.False(t, gjson.GetBytes(upstream.lastBody, "prompt_cache_key").Exists())
		require.NotEmpty(t, upstream.lastReq.Header.Get(grokConversationIDHeader))
	})

	for _, ingress := range []keyProtocolIngress{keyProtocolResponsesIngress, keyProtocolMessagesIngress} {
		t.Run(ingress.name+" on a grok-labelled relay uses the standard Responses request", func(t *testing.T) {
			upstream := captureKeyProtocolRequest(t, relay(), ingress)
			require.Equal(t, "http://relay.example/v1/responses", upstream.lastReq.URL.String())
			require.Equal(t, HTTPUpstreamProfileOpenAI, HTTPUpstreamProfileFromContext(upstream.lastReq.Context()))
		})
		t.Run(ingress.name+" on an openai-labelled xAI key uses the Grok Responses request", func(t *testing.T) {
			upstream := captureKeyProtocolRequest(t, official(), ingress)
			require.Equal(t, xaiOfficialTestBaseURL+"/responses", upstream.lastReq.URL.String())
			require.Equal(t, HTTPUpstreamProfileGrok, HTTPUpstreamProfileFromContext(upstream.lastReq.Context()))
		})
	}
}

const xaiOfficialTestBaseURL = "https://api.x.ai/v1"

// TestGrokVendorQuirkPredicatesFollowVendorNotLabel：xAI 专属的失败分类、流空闲重试、计费 ping 过滤、
// WS HTTP bridge 强制与内容策略错误，都按地址识别的厂商决定。每条断言两侧：grok 标签挂中转不启用，
// openai 标签发往 api.x.ai 启用。
func TestGrokVendorQuirkPredicatesFollowVendorNotLabel(t *testing.T) {
	relay := keyProtocolTestAccount(PlatformGrok, map[string]string{APIProtocolChatCompletions: "https://relay.example/v1"})
	official := keyProtocolTestAccount(PlatformOpenAI, map[string]string{APIProtocolChatCompletions: xaiOfficialTestBaseURL})
	require.Equal(t, PlatformGrok, official.Vendor(), "fixture: api.x.ai must be recognised as xAI")
	require.Equal(t, "", relay.Vendor(), "fixture: relay host must not be recognised as a vendor")

	t.Run("stream idle same-account retry", func(t *testing.T) {
		require.False(t, grokStreamIdleFailoverError(relay, time.Second).RetryableOnSameAccount)
		require.True(t, grokStreamIdleFailoverError(official, time.Second).RetryableOnSameAccount)
	})

	t.Run("capacity 429 same-account retry", func(t *testing.T) {
		body := []byte(`{"error":{"message":"The model is currently at capacity due to high demand"}}`)
		require.False(t, grokRetryableOnSameAccount(relay, http.StatusTooManyRequests, body))
		require.True(t, grokRetryableOnSameAccount(official, http.StatusTooManyRequests, body))
	})

	t.Run("billing ping SSE filter", func(t *testing.T) {
		source := io.NopCloser(strings.NewReader(""))
		require.True(t, newGrokResponsesBillingPingFilterBody(source, relay, defaultMaxLineSize) == source)
		filtered := newGrokResponsesBillingPingFilterBody(source, official, defaultMaxLineSize)
		require.False(t, filtered == source)
		_ = filtered.Close()
	})

	t.Run("billable usage requirement", func(t *testing.T) {
		require.False(t, requiresBillableGrokChatUsage(relay, "gpt-5.4"))
		require.True(t, requiresBillableGrokChatUsage(official, "gpt-5.4"))
	})

	t.Run("WS HTTP bridge is forced", func(t *testing.T) {
		svc := &OpenAIGatewayService{}
		require.False(t, svc.shouldBridgeOpenAIWSHTTP(relay, 1, "resp_existing"))
		require.True(t, svc.shouldBridgeOpenAIWSHTTP(official, 1, "resp_existing"))
		require.False(t, svc.shouldBridgeOpenAIWSPassthroughFirstMessage(relay, []byte(`{}`)))
		require.True(t, svc.shouldBridgeOpenAIWSPassthroughFirstMessage(official, []byte(`{}`)))
		require.Equal(t, "OpenAI WS HTTP bridge", openAIWSHTTPBridgeToolUpstreamName(relay))
		require.Equal(t, "Grok WS HTTP bridge", openAIWSHTTPBridgeToolUpstreamName(official))
	})

	t.Run("CC failover classifier", func(t *testing.T) {
		// xAI 的 ModelInput 解码 422 是账号侧兼容问题，xAI 分类器判 failover；通用分类器不判。
		body := []byte(`{"detail":"data did not match any variant of untagged enum ModelInput at input[3]"}`)
		svc := &OpenAIGatewayService{}
		failover := func(account *Account) *UpstreamFailoverError {
			c := adaptiveProtocolTestContext("/v1/chat/completions", nil)
			resp := &http.Response{StatusCode: http.StatusUnprocessableEntity, Header: http.Header{}}
			return svc.failoverOpenAIUpstreamHTTPError(context.Background(), c, account, resp, body, "decode", "grok-4.5")
		}
		require.Nil(t, failover(relay))
		require.NotNil(t, failover(official))
	})

	t.Run("content policy 403 is rewritten as a client error", func(t *testing.T) {
		body := `{"error":{"code":"content_filter","message":"prohibited content"}}`
		svc := &OpenAIGatewayService{}
		status := func(account *Account) int {
			c := adaptiveProtocolTestContext("/v1/responses", nil)
			resp := &http.Response{
				StatusCode: http.StatusForbidden,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(body)),
			}
			_, err := svc.handleErrorResponse(context.Background(), resp, c, account, nil, "grok-4.5")
			require.Error(t, err)
			return c.Writer.Status()
		}
		require.NotEqual(t, http.StatusForbidden, status(relay))
		require.Equal(t, http.StatusForbidden, status(official))
	})
}

// TestOpenAIGatewayKeyProtocol_OpenCodeFollowsTheConfiguredProtocol：OpenCode 官方地址
// 不再按模型分流——一个 key 只配一个协议地址，入站协议按转换注册表落到那个协议上。
func TestOpenAIGatewayKeyProtocol_OpenCodeFollowsTheConfiguredProtocol(t *testing.T) {
	responsesBody := func(model string) keyProtocolIngress {
		ingress := keyProtocolResponsesIngress
		ingress.body = []byte(`{"model":"` + model + `","input":"hello","stream":false}`)
		return ingress
	}

	// 只配 anthropic 地址：OpenAI 网关不承接，与模型无关（minimax 与 glm 同样报错）。
	for _, model := range []string{"minimax-m3", "glm-5.3"} {
		t.Run("anthropic-only key is refused for "+model, func(t *testing.T) {
			requireAnthropicUpstreamRefused(t, keyProtocolTestAccount(PlatformOpenAI, map[string]string{
				APIProtocolAnthropic: DefaultOpenCodeGoAnthropicBaseURL,
			}), responsesBody(model))
		})
	}

	// 只配 chat_completions 地址：曾被模型规则判给 Responses 的 grok 也走 Chat Completions。
	t.Run("chat-completions-only key serves grok via chat completions", func(t *testing.T) {
		upstream := captureKeyProtocolRequest(t, keyProtocolTestAccount(PlatformOpenCodeGo, map[string]string{
			APIProtocolChatCompletions: DefaultOpenCodeGoBaseURL,
		}), responsesBody("grok-4.6"))
		require.Equal(t, "https://opencode.ai/zen/go/v1/chat/completions", upstream.lastReq.URL.String())
	})
}

// TestOpenAIGatewayKeyProtocol_ResponsesOutputLimitFollowsProtocol：发往 Responses 上游的请求
// 按 Responses 协议归一化输出上限，与标签无关（anthropic 标签不再改写成 max_tokens）。
func TestOpenAIGatewayKeyProtocol_ResponsesOutputLimitFollowsProtocol(t *testing.T) {
	endpoints := map[string]string{APIProtocolResponses: "http://relay.example/v1"}
	for _, platform := range []string{PlatformAnthropic, PlatformGemini, PlatformKimi} {
		t.Run(platform+" label keeps max_output_tokens", func(t *testing.T) {
			ingress := keyProtocolResponsesIngress
			ingress.body = []byte(`{"model":"gpt-5.4","input":"hello","max_output_tokens":64,"stream":false}`)
			upstream := captureKeyProtocolRequest(t, keyProtocolTestAccount(platform, endpoints), ingress)
			require.Equal(t, int64(64), gjson.GetBytes(upstream.lastBody, "max_output_tokens").Int())
			require.False(t, gjson.GetBytes(upstream.lastBody, "max_tokens").Exists())
		})
		t.Run(platform+" label rewrites max_tokens", func(t *testing.T) {
			ingress := keyProtocolResponsesIngress
			ingress.body = []byte(`{"model":"gpt-5.4","input":"hello","max_tokens":48,"stream":false}`)
			upstream := captureKeyProtocolRequest(t, keyProtocolTestAccount(platform, endpoints), ingress)
			require.Equal(t, int64(48), gjson.GetBytes(upstream.lastBody, "max_output_tokens").Int())
			require.False(t, gjson.GetBytes(upstream.lastBody, "max_tokens").Exists())
		})
	}
}

func TestGetOpenAIResponsesBaseURLIsStrict(t *testing.T) {
	chatOnly := keyProtocolTestAccount(PlatformOpenAI, map[string]string{APIProtocolChatCompletions: "http://relay.example/v1"})
	require.Empty(t, chatOnly.GetOpenAIResponsesBaseURL())

	both := keyProtocolTestAccount(PlatformOpenAI, map[string]string{
		APIProtocolChatCompletions: "http://chat.example/v1",
		APIProtocolResponses:       "http://responses.example/v1",
	})
	require.Equal(t, "http://responses.example/v1", both.GetOpenAIResponsesBaseURL())
}

func TestOpenAIKeyGettersIgnoreLabel(t *testing.T) {
	for _, platform := range []string{PlatformOpenAI, PlatformGrok, PlatformAnthropic, PlatformGemini, PlatformAntigravity, PlatformKimi} {
		account := keyProtocolTestAccount(platform, map[string]string{APIProtocolChatCompletions: "http://relay.example/v1"})
		require.Equal(t, "http://relay.example/v1", account.GetOpenAIBaseURL(), platform)
		require.Equal(t, "sk-test", account.GetOpenAIProtocolAPIKey(), platform)
	}
	subscription := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"api_key": "sk-test"}}
	require.Empty(t, subscription.GetOpenAIProtocolAPIKey())
}

func TestShouldForwardOpenAIResponsesViaRawChatCompletions(t *testing.T) {
	require.True(t, shouldForwardOpenAIResponsesViaRawChatCompletions(
		keyProtocolTestAccount(PlatformGrok, map[string]string{APIProtocolChatCompletions: "http://relay.example/v1"})))
	require.False(t, shouldForwardOpenAIResponsesViaRawChatCompletions(
		keyProtocolTestAccount(PlatformKimi, map[string]string{
			APIProtocolChatCompletions: "http://relay.example/v1",
			APIProtocolResponses:       "http://relay.example/v1",
		})))
	require.False(t, shouldForwardOpenAIResponsesViaRawChatCompletions(
		keyProtocolTestAccount(PlatformOpenAI, map[string]string{APIProtocolAnthropic: "http://relay.example"})))
	require.False(t, shouldForwardOpenAIResponsesViaRawChatCompletions(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}))
}

func TestPrimaryUpstreamBaseURLOrderIgnoresLabel(t *testing.T) {
	endpoints := map[string]string{
		APIProtocolAnthropic:       "http://anthropic.example",
		APIProtocolChatCompletions: "http://chat.example/v1",
		APIProtocolResponses:       "http://responses.example/v1",
		APIProtocolGemini:          "http://gemini.example",
	}
	for _, platform := range []string{PlatformAnthropic, PlatformGemini, PlatformAntigravity, PlatformOpenAI} {
		require.Equal(t, "http://chat.example/v1", keyProtocolTestAccount(platform, endpoints).PrimaryUpstreamBaseURL(), platform)
	}

	// 顺序表必须覆盖全部协议且不重复，否则只配了漏掉协议的 key 会拿到空地址。
	seen := map[string]int{}
	for _, protocol := range primaryUpstreamProtocolOrder {
		seen[protocol]++
	}
	for _, protocol := range UpstreamProtocols() {
		require.Equal(t, 1, seen[protocol], protocol)
	}
	require.Len(t, primaryUpstreamProtocolOrder, len(UpstreamProtocols()))
}
