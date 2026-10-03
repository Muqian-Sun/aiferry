//go:build unit

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
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 联网搜索次数要从每条转发链路带到计费结果里（不看渠道平台）：上游回一次搜索 + 一次打开网页，
// 结果里应是 1 次搜索。哪条链路漏接，对应的用例就会变红。

const webSearchWiringSSE = "data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"web_search_call\",\"id\":\"ws_1\",\"status\":\"completed\",\"action\":{\"type\":\"search\",\"query\":\"q\"}}}\n\n" +
	"data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"web_search_call\",\"id\":\"ws_2\",\"status\":\"completed\",\"action\":{\"type\":\"open_page\",\"url\":\"https://a\"}}}\n\n" +
	"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_ws\",\"object\":\"response\",\"model\":\"gpt-5.5\",\"status\":\"completed\",\"output\":[" +
	"{\"type\":\"web_search_call\",\"id\":\"ws_1\",\"status\":\"completed\",\"action\":{\"type\":\"search\",\"query\":\"q\"}}," +
	"{\"type\":\"web_search_call\",\"id\":\"ws_2\",\"status\":\"completed\",\"action\":{\"type\":\"open_page\",\"url\":\"https://a\"}}," +
	"{\"type\":\"message\",\"id\":\"msg_1\",\"role\":\"assistant\",\"status\":\"completed\",\"content\":[{\"type\":\"output_text\",\"text\":\"ok\"}]}]," +
	"\"usage\":{\"input_tokens\":5,\"output_tokens\":2,\"total_tokens\":7}}}\n\n" +
	"data: [DONE]\n\n"

const webSearchWiringJSON = `{"id":"resp_ws","object":"response","model":"gpt-5.5","status":"completed","output":[
	{"type":"web_search_call","id":"ws_1","status":"completed","action":{"type":"search","query":"q"}},
	{"type":"web_search_call","id":"ws_2","status":"completed","action":{"type":"open_page","url":"https://a"}},
	{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],
	"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}`

func webSearchWiringService(contentType, body string) *OpenAIGatewayService {
	return &OpenAIGatewayService{
		httpUpstream: &httpUpstreamRecorder{resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{contentType}, "x-request-id": []string{"rid_ws"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}},
		cfg: &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}
}

func webSearchWiringAccount() *Account {
	return &Account{
		ID: 1, Name: "openai-oauth", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1,
		Status: StatusActive, Schedulable: true,
		Credentials: map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
	}
}

func webSearchWiringContext(path string, body []byte) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func TestWebSearchWiring_ResponsesForward(t *testing.T) {
	t.Run("stream", func(t *testing.T) {
		body := []byte(`{"model":"gpt-5.5","stream":true,"input":"hi","tools":[{"type":"web_search"}]}`)
		svc := webSearchWiringService("text/event-stream", webSearchWiringSSE)
		result, err := svc.Forward(context.Background(), webSearchWiringContext("/v1/responses", body), webSearchWiringAccount(), body)
		require.NoError(t, err)
		require.Equal(t, WebSearchUsage{WebSearchCalls: 1}, result.WebSearch)
	})
	t.Run("json", func(t *testing.T) {
		body := []byte(`{"model":"gpt-5.5","stream":false,"input":"hi","tools":[{"type":"web_search"}]}`)
		svc := webSearchWiringService("application/json", webSearchWiringJSON)
		result, err := svc.Forward(context.Background(), webSearchWiringContext("/v1/responses", body), webSearchWiringAccount(), body)
		require.NoError(t, err)
		require.Equal(t, WebSearchUsage{WebSearchCalls: 1}, result.WebSearch)
	})
}

// Claude Code 配 GPT：Messages 请求转 Responses 上游。
func TestWebSearchWiring_MessagesBridge(t *testing.T) {
	for _, stream := range []bool{false, true} {
		name := "buffered"
		if stream {
			name = "stream"
		}
		t.Run(name, func(t *testing.T) {
			streamJSON := "false"
			if stream {
				streamJSON = "true"
			}
			body := []byte(`{"model":"gpt-5.5","max_tokens":16,"stream":` + streamJSON + `,"messages":[{"role":"user","content":"hi"}],"tools":[{"type":"web_search_20250305","name":"web_search"}]}`)
			svc := webSearchWiringService("text/event-stream", webSearchWiringSSE)
			result, err := svc.ForwardAsAnthropic(context.Background(), webSearchWiringContext("/v1/messages", body), webSearchWiringAccount(), body, "")
			require.NoError(t, err)
			require.Equal(t, WebSearchUsage{WebSearchCalls: 1}, result.WebSearch)
		})
	}
}

// Chat Completions 请求转 Responses 上游。
func TestWebSearchWiring_ChatCompletionsBridge(t *testing.T) {
	for _, stream := range []bool{false, true} {
		name := "buffered"
		if stream {
			name = "stream"
		}
		t.Run(name, func(t *testing.T) {
			streamJSON := "false"
			if stream {
				streamJSON = "true"
			}
			body := []byte(`{"model":"gpt-5.5","stream":` + streamJSON + `,"messages":[{"role":"user","content":"hi"}]}`)
			svc := webSearchWiringService("text/event-stream", webSearchWiringSSE)
			result, err := svc.ForwardAsChatCompletions(context.Background(), webSearchWiringContext("/v1/chat/completions", body), webSearchWiringAccount(), body, "")
			require.NoError(t, err)
			require.Equal(t, WebSearchUsage{WebSearchCalls: 1}, result.WebSearch)
		})
	}
}
