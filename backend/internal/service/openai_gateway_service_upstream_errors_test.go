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
	"github.com/tidwall/gjson"
)

func TestGetAPIKeyIDFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("context 为 nil", func(t *testing.T) {
		require.Equal(t, int64(0), getAPIKeyIDFromContext(nil))
	})

	t.Run("上下文没有 api_key", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		require.Equal(t, int64(0), getAPIKeyIDFromContext(c))
	})

	t.Run("api_key 类型错误", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Set("api_key", "not-api-key")
		require.Equal(t, int64(0), getAPIKeyIDFromContext(c))
	})

	t.Run("api_key 指针为空", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		var k *APIKey
		c.Set("api_key", k)
		require.Equal(t, int64(0), getAPIKeyIDFromContext(c))
	})

	t.Run("正常读取 api_key_id", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Set("api_key", &APIKey{ID: 12345})
		require.Equal(t, int64(12345), getAPIKeyIDFromContext(c))
	})
}

func TestLogOpenAIInstructionsRequiredDebug_LogsRequestDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logSink, restore := captureStructuredLog(t)
	defer restore()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses?trace=1", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "curl/8.0")
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("OpenAI-Beta", "assistants=v2")

	body := []byte(`{"model":"gpt-5.1-codex","stream":false,"prompt_cache_key":"pc-abc","access_token":"secret-token","input":[{"type":"text","text":"hello"}]}`)
	account := &Account{ID: 1001, Name: "codex max套餐"}

	logOpenAIInstructionsRequiredDebug(
		context.Background(),
		c,
		account,
		http.StatusBadRequest,
		"Instructions are required",
		body,
		[]byte(`{"error":{"message":"Instructions are required","type":"invalid_request_error","param":"instructions","code":"missing_required_parameter"}}`),
	)

	require.True(t, logSink.ContainsMessageAtLevel("OpenAI 上游返回 Instructions are required，已记录请求详情用于排查", "warn"))
	require.True(t, logSink.ContainsFieldValue("request_user_agent", "curl/8.0"))
	require.True(t, logSink.ContainsFieldValue("request_model", "gpt-5.1-codex"))
	require.True(t, logSink.ContainsFieldValue("request_query", "trace=1"))
	require.True(t, logSink.ContainsFieldValue("account_name", "codex max套餐"))
	require.True(t, logSink.ContainsFieldValue("request_headers", "openai-beta"))
	require.True(t, logSink.ContainsField("request_body_size"))
	require.False(t, logSink.ContainsField("request_body_preview"))
}

func TestLogOpenAIInstructionsRequiredDebug_NonTargetErrorSkipped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logSink, restore := captureStructuredLog(t)
	defer restore()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "curl/8.0")
	body := []byte(`{"model":"gpt-5.1-codex","stream":false}`)

	logOpenAIInstructionsRequiredDebug(
		context.Background(),
		c,
		&Account{ID: 1001},
		http.StatusForbidden,
		"forbidden",
		body,
		[]byte(`{"error":{"message":"forbidden"}}`),
	)

	require.False(t, logSink.ContainsMessage("OpenAI 上游返回 Instructions are required，已记录请求详情用于排查"))
}

func TestIsOpenAITransientProcessingError(t *testing.T) {
	require.True(t, isOpenAITransientProcessingError(
		http.StatusBadRequest,
		"An error occurred while processing your request.",
		nil,
	))

	require.True(t, isOpenAITransientProcessingError(
		http.StatusBadRequest,
		"Selected model is at capacity. Please try a different model.",
		[]byte(`{"error":{"message":"Selected model is at capacity. Please try a different model.","type":"invalid_request_error"}}`),
	))

	require.True(t, isOpenAITransientProcessingError(
		http.StatusBadRequest,
		"",
		[]byte(`{"error":{"code":"server_is_overloaded","message":"Please retry later.","type":"invalid_request_error"}}`),
	))

	require.True(t, isOpenAITransientProcessingError(
		http.StatusServiceUnavailable,
		"",
		[]byte(`{"error":{"code":"slow_down","message":"Please retry later."}}`),
	))

	require.True(t, isOpenAITransientProcessingError(
		http.StatusBadRequest,
		"",
		[]byte(`{"error":{"message":"Our servers are currently overloaded. Please try again later."}}`),
	))

	require.True(t, isOpenAITransientProcessingError(
		http.StatusServiceUnavailable,
		"Server is overloaded. Please try again later.",
		nil,
	))

	require.True(t, isOpenAITransientProcessingError(
		http.StatusBadGateway,
		"",
		[]byte(`{"error":{"message":"Our servers are currently overloaded. Please try again later."}}`),
	))

	require.True(t, isOpenAITransientProcessingError(
		http.StatusBadRequest,
		"",
		[]byte(`{"error":{"message":"An error occurred while processing your request. You can retry your request, or contact us through our help center at help.openai.com if the error persists. Please include the request ID req_123 in your message."}}`),
	))

	require.False(t, isOpenAITransientProcessingError(
		http.StatusBadRequest,
		"Missing required parameter: 'instructions'",
		[]byte(`{"error":{"message":"Missing required parameter: 'instructions'"}}`),
	))
}

func TestIsOpenAIContextWindowError(t *testing.T) {
	require.True(t, isOpenAIContextWindowError(
		"",
		[]byte(`{"error":{"message":"Your input exceeds the context window of this model. Please adjust your input and try again.","type":"upstream_error","code":null}}`),
	))
	require.True(t, isOpenAIContextWindowError(
		"maximum context length exceeded",
		nil,
	))
	require.True(t, isOpenAIContextWindowError(
		"",
		[]byte(`maximum context length exceeded`),
	))
	require.False(t, isOpenAIContextWindowError(
		"context canceled",
		nil,
	))
	require.False(t, isOpenAIContextWindowError(
		"upstream unavailable",
		[]byte(`{"error":{"message":"upstream unavailable","code":"upstream_error"},"echo":"context_length_exceeded maximum context length"}`),
	))
}

func TestOpenAITransientAndCapacityClassificationIgnoresEchoedJSON(t *testing.T) {
	body := []byte(`{"error":{"message":"upstream unavailable","code":"upstream_error"},"echo":"server is overloaded; selected model is at capacity"}`)

	require.False(t, isOpenAITransientProcessingError(http.StatusBadRequest, "upstream unavailable", body))
	require.False(t, isOpenAIRequestScopedCapacityShed("upstream unavailable", body))

	plainText := []byte(`server is overloaded; please retry later`)
	require.True(t, isOpenAITransientProcessingError(http.StatusServiceUnavailable, "", plainText))
	require.True(t, isOpenAIRequestScopedCapacityShed("", plainText))
}

func TestShouldFailoverOpenAIUpstreamResponseContextWindow502(t *testing.T) {
	svc := &OpenAIGatewayService{}
	body := []byte(`{"error":{"message":"Your input exceeds the context window of this model. Please adjust your input and try again.","type":"upstream_error","code":null}}`)

	require.False(t, svc.shouldFailoverOpenAIUpstreamResponse(newOpenAIUpstreamErrorTestAccount(), http.StatusBadGateway, "", body))
	require.True(t, svc.shouldFailoverOpenAIUpstreamResponse(newOpenAIUpstreamErrorTestAccount(), http.StatusBadGateway, "temporary upstream outage", []byte(`{"error":{"message":"temporary upstream outage"}}`)))
	require.True(t, svc.shouldFailoverOpenAIUpstreamResponse(newOpenAIUpstreamErrorTestAccount(),
		http.StatusBadGateway,
		"temporary upstream outage",
		[]byte(`{"error":{"message":"temporary upstream outage"},"echo":"context_length_exceeded"}`),
	))
}

func TestOpenAIGatewayService_Forward_LogsInstructionsRequiredDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logSink, restore := captureStructuredLog(t)
	defer restore()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses?trace=1", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("OpenAI-Beta", "assistants=v2")

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusBadRequest,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"x-request-id": []string{"rid-upstream"},
			},
			Body: io.NopCloser(strings.NewReader(`{"error":{"message":"Missing required parameter: 'instructions'","type":"invalid_request_error","param":"instructions","code":"missing_required_parameter"}}`)),
		},
	}
	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{ForceCodexCLI: false},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:                1001,
		Name:              "codex max套餐",
		Platform:          PlatformOpenAI,
		Type:              AccountTypeAPIKey,
		Concurrency:       1,
		Credentials:       map[string]any{"api_key": "sk-test"},
		Status:            StatusActive,
		Schedulable:       true,
		RateMultiplier:    f64p(1),
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
	}
	body := []byte(`{"model":"gpt-5.1-codex","stream":false,"input":[{"type":"text","text":"hello"}],"prompt_cache_key":"pc-forward","access_token":"secret-token"}`)

	_, err := svc.Forward(context.Background(), c, account, body)
	require.Error(t, err)
	// missing_required_parameter 是确定性的请求错误：换账号、重试都不会变。按真实的
	// 400 回写并保留 param/code，客户端才知道该补哪个字段（而不是收到可重试的 502）。
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "invalid_request_error", gjson.Get(rec.Body.String(), "error.type").String())
	require.Equal(t, "missing_required_parameter", gjson.Get(rec.Body.String(), "error.code").String())
	require.Equal(t, "instructions", gjson.Get(rec.Body.String(), "error.param").String())
	require.Contains(t, err.Error(), "upstream error: 400")

	require.True(t, logSink.ContainsMessageAtLevel("OpenAI 上游返回 Instructions are required，已记录请求详情用于排查", "warn"))
	require.True(t, logSink.ContainsFieldValue("request_user_agent", "codex_cli_rs/0.1.0"))
	require.True(t, logSink.ContainsFieldValue("request_model", "gpt-5.1-codex"))
	require.True(t, logSink.ContainsFieldValue("request_headers", "openai-beta"))
	require.True(t, logSink.ContainsField("request_body_size"))
	require.False(t, logSink.ContainsField("request_body_preview"))
}

func TestOpenAIGatewayService_Forward_TransientProcessingErrorTriggersFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusBadRequest,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"x-request-id": []string{"rid-processing-400"},
			},
			Body: io.NopCloser(strings.NewReader(`{"error":{"message":"An error occurred while processing your request. You can retry your request, or contact us through our help center at help.openai.com if the error persists. Please include the request ID req_123 in your message.","type":"invalid_request_error"}}`)),
		},
	}
	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{ForceCodexCLI: false},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:                1001,
		Name:              "codex max套餐",
		Platform:          PlatformOpenAI,
		Type:              AccountTypeAPIKey,
		Concurrency:       1,
		Credentials:       map[string]any{"api_key": "sk-test"},
		Status:            StatusActive,
		Schedulable:       true,
		RateMultiplier:    f64p(1),
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
	}
	body := []byte(`{"model":"gpt-5.1-codex","stream":false,"input":[{"type":"text","text":"hello"}]}`)

	_, err := svc.Forward(context.Background(), c, account, body)
	require.Error(t, err)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadRequest, failoverErr.StatusCode)
	require.Contains(t, string(failoverErr.ResponseBody), "An error occurred while processing your request")
	require.False(t, c.Writer.Written(), "service 层应返回 failover 错误给上层换号，而不是直接向客户端写响应")
}

func TestOpenAIGatewayService_Forward_ModelCapacityErrorTriggersFailoverAndSameAccountRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusBadRequest,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"x-request-id": []string{"rid-capacity-400"},
			},
			Body: io.NopCloser(strings.NewReader(`{"error":{"message":"Selected model is at capacity. Please try a different model.","type":"invalid_request_error"}}`)),
		},
	}
	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{ForceCodexCLI: false},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          1001,
		Name:        "codex max套餐",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":   "sk-test",
			"pool_mode": true,
		},
		Status:            StatusActive,
		Schedulable:       true,
		RateMultiplier:    f64p(1),
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
	}
	body := []byte(`{"model":"gpt-5.4","stream":false,"input":[{"type":"text","text":"hello"}]}`)

	_, err := svc.Forward(context.Background(), c, account, body)
	require.Error(t, err)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadRequest, failoverErr.StatusCode)
	require.True(t, failoverErr.RetryableOnSameAccount)
	require.Contains(t, string(failoverErr.ResponseBody), "Selected model is at capacity")
	require.False(t, c.Writer.Written(), "service 层应返回 failover 错误给上层重试/换号，而不是直接向客户端写响应")
}
