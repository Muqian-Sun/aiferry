//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

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
}

var (
	keyProtocolChatIngress = keyProtocolIngress{
		name: "chat_completions",
		path: "/v1/chat/completions",
		body: []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`),
		forward: func(svc *OpenAIGatewayService, c *gin.Context, account *Account, body []byte) error {
			_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
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
			_, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "")
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
	err := ingress.forward(svc, adaptiveProtocolTestContext(ingress.path, ingress.body), account, ingress.body)
	require.Error(t, err)
	require.Len(t, upstream.requests, 1, "exactly one upstream request")
	return upstream
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

	t.Run("anthropic upstream receives an anthropic body", func(t *testing.T) {
		upstream := captureKeyProtocolRequest(t, keyProtocolTestAccount(PlatformZhipu, map[string]string{
			APIProtocolAnthropic: "http://anthropic.example",
		}), ingress)
		require.Equal(t, "http://anthropic.example/v1/messages", upstream.lastReq.URL.String())
		require.True(t, gjson.GetBytes(upstream.lastBody, "messages").IsArray())
		require.False(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
	})
}

func TestOpenAIGatewayKeyProtocol_AnthropicOnlyKey(t *testing.T) {
	endpoints := map[string]string{APIProtocolAnthropic: "http://anthropic.example"}
	for _, ingress := range []keyProtocolIngress{keyProtocolChatIngress, keyProtocolResponsesIngress, keyProtocolMessagesIngress} {
		t.Run(ingress.name, func(t *testing.T) {
			upstream := captureKeyProtocolRequest(t, keyProtocolTestAccount(PlatformOpenAI, endpoints), ingress)
			require.Equal(t, "http://anthropic.example/v1/messages", upstream.lastReq.URL.String())
		})
	}
}

func TestOpenAIGatewayKeyProtocol_MessagesPreferAnthropicEndpoint(t *testing.T) {
	account := keyProtocolTestAccount(PlatformZhipu, map[string]string{
		APIProtocolChatCompletions: "http://chat.example",
		APIProtocolAnthropic:       "http://anthropic.example",
	})
	ingress := keyProtocolMessagesIngress
	ingress.body = []byte(`{"model":"glm-4.7","max_tokens":32,"messages":[{"role":"user","content":"hello"}],"stream":false}`)
	upstream := captureKeyProtocolRequest(t, account, ingress)
	require.Equal(t, "http://anthropic.example/v1/messages", upstream.lastReq.URL.String())
	require.Equal(t, "glm-4.7", gjson.GetBytes(upstream.lastBody, "model").String())
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

func TestOpenAIGatewayKeyProtocol_OpenCodeModelRuleChoosesAmongConfiguredProtocols(t *testing.T) {
	goEndpoints := func() map[string]string {
		return map[string]string{
			APIProtocolChatCompletions: DefaultOpenCodeGoBaseURL,
			APIProtocolAnthropic:       DefaultOpenCodeGoAnthropicBaseURL,
			APIProtocolResponses:       DefaultOpenCodeGoBaseURL,
		}
	}
	responsesBody := func(model string) keyProtocolIngress {
		ingress := keyProtocolResponsesIngress
		ingress.body = []byte(`{"model":"` + model + `","input":"hello","stream":false}`)
		return ingress
	}

	t.Run("rule protocol is used on the official host whatever the label", func(t *testing.T) {
		upstream := captureKeyProtocolRequest(t, keyProtocolTestAccount(PlatformOpenAI, goEndpoints()), responsesBody("minimax-m3"))
		require.Equal(t, "https://opencode.ai/zen/go/v1/messages", upstream.lastReq.URL.String())
	})

	t.Run("unmatched model goes to chat completions", func(t *testing.T) {
		upstream := captureKeyProtocolRequest(t, keyProtocolTestAccount(PlatformOpenAI, goEndpoints()), responsesBody("glm-5.3"))
		require.Equal(t, "https://opencode.ai/zen/go/v1/chat/completions", upstream.lastReq.URL.String())
	})

	t.Run("rule protocol without an address falls back to the generic choice", func(t *testing.T) {
		endpoints := goEndpoints()
		delete(endpoints, APIProtocolAnthropic)
		upstream := captureKeyProtocolRequest(t, keyProtocolTestAccount(PlatformOpenAI, endpoints), responsesBody("minimax-m3"))
		require.Equal(t, "https://opencode.ai/zen/go/v1/responses", upstream.lastReq.URL.String())
	})

	t.Run("opencodego label on a relay does not apply model rules", func(t *testing.T) {
		upstream := captureKeyProtocolRequest(t, keyProtocolTestAccount(PlatformOpenCodeGo, map[string]string{
			APIProtocolChatCompletions: "http://relay.example/v1",
			APIProtocolAnthropic:       "http://relay.example",
			APIProtocolResponses:       "http://relay.example/v1",
		}), responsesBody("minimax-m3"))
		require.Equal(t, "http://relay.example/v1/responses", upstream.lastReq.URL.String())
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
