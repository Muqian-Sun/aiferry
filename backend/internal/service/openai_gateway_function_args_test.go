package service

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestForwardResponsesChatCompletionsFallbackKeepsFunctionArgumentsSingle(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"gpt-5.4","input":"run a command","stream":true}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(string(body)))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		passthroughSSEData(chatToolCallChunkJSON(true, "")),
		"",
		passthroughSSEData(chatToolCallChunkJSON(false, `{"cmd":"echo hi"}`)),
		"",
		`data: {"id":"chatcmpl_tool","object":"chat.completion.chunk","model":"gpt-5.4","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_fallback_tool_args"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	account := passthroughArgsFallbackAccount()
	// 只留 chat_completions 地址：Responses 入站转成 Chat Completions。
	delete(account.ProtocolEndpoints, APIProtocolResponses)
	svc := &OpenAIGatewayService{
		cfg:          passthroughArgsTestConfig(),
		httpUpstream: upstream,
	}

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)

	const wantArgs = `{"cmd":"echo hi"}`
	events := collectSSEDataPayloads(t, rec.Body.String())
	require.Equal(t, wantArgs, accumulateFunctionArgumentDeltas(events, "chatcmpl-tool-a"))
	require.Equal(t, wantArgs, gjson.Get(findSSEEvent(t, events, "response.function_call_arguments.done", "chatcmpl-tool-a"), "arguments").String())
	require.Equal(t, wantArgs, gjson.Get(findSSEEvent(t, events, "response.output_item.done", "chatcmpl-tool-a"), "item.arguments").String())
}

func passthroughSSEData(payload string) string {
	return "data: " + payload + "\n\n"
}

func chatToolCallChunkJSON(includeIdentity bool, arguments string) string {
	identity := ""
	functionFields := make([]string, 0, 2)
	if includeIdentity {
		identity = `"id":"chatcmpl-tool-a","type":"function",`
		functionFields = append(functionFields, `"name":"exec_command"`)
	}
	if includeIdentity || arguments != "" {
		functionFields = append(functionFields, `"arguments":`+strconv.Quote(arguments))
	}
	return fmt.Sprintf(
		`{"id":"chatcmpl_tool","object":"chat.completion.chunk","model":"gpt-5.4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,%s"function":{%s}}]},"finish_reason":null}]}`,
		identity,
		strings.Join(functionFields, ","),
	)
}

func passthroughArgsTestConfig() *config.Config {
	return &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{
				Enabled:           false,
				AllowInsecureHTTP: true,
			},
		},
	}
}

func passthroughArgsFallbackAccount() *Account {
	return &Account{
		ID:          102,
		Name:        "passthrough-args-openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "http://upstream.example",
		},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "http://upstream.example", APIProtocolResponses: "http://upstream.example"},
	}
}

func collectSSEDataPayloads(t *testing.T, body string) []string {
	t.Helper()
	scanner := bufio.NewScanner(strings.NewReader(body))
	var events []string
	for scanner.Scan() {
		data, ok := extractOpenAISSEDataLine(scanner.Text())
		if !ok {
			continue
		}
		if strings.TrimSpace(data) == "[DONE]" {
			continue
		}
		require.True(t, gjson.Valid(data), "invalid SSE data payload: %s", data)
		events = append(events, data)
	}
	require.NoError(t, scanner.Err())
	return events
}

func findSSEEvent(t *testing.T, events []string, eventType, callID string) string {
	t.Helper()
	for _, event := range events {
		if gjson.Get(event, "type").String() != eventType {
			continue
		}
		if callID == "" ||
			gjson.Get(event, "call_id").String() == callID ||
			gjson.Get(event, "item.call_id").String() == callID {
			return event
		}
	}
	t.Fatalf("missing event type=%s call_id=%s in %d events", eventType, callID, len(events))
	return ""
}

func accumulateFunctionArgumentDeltas(events []string, callID string) string {
	var b strings.Builder
	for _, event := range events {
		if gjson.Get(event, "type").String() != "response.function_call_arguments.delta" {
			continue
		}
		if gjson.Get(event, "call_id").String() != callID {
			continue
		}
		_, _ = b.WriteString(gjson.Get(event, "delta").String())
	}
	return b.String()
}
