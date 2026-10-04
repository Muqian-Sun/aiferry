//go:build unit

package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 网关鉴权错误按入口协议返回（2026-10-04 D5）：Anthropic 入口 / OpenAI 兼容入口 / 本站自己的接口各一种格式，
// 协议格式的 error 里都带本站 code。
func TestAbortGatewayErrorFormatsByInboundProtocol(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		path, anthropicVersion string
		want                   string // anthropic / openai / plain
		wantType               string
	}{
		{path: "/v1/messages", want: "anthropic", wantType: "authentication_error"},
		{path: "/v1/messages/count_tokens", want: "anthropic", wantType: "authentication_error"},
		{path: "/antigravity/v1/messages", want: "anthropic", wantType: "authentication_error"},
		{path: "/v1/models", anthropicVersion: "2023-06-01", want: "anthropic", wantType: "authentication_error"},
		{path: "/v1/models", want: "openai", wantType: "invalid_request_error"},
		{path: "/v1/chat/completions", want: "openai", wantType: "invalid_request_error"},
		{path: "/v1/responses", want: "openai", wantType: "invalid_request_error"},
		{path: "/backend-api/codex/responses", want: "openai", wantType: "invalid_request_error"},
		{path: "/v1/usage", want: "plain"},
		{path: "/v1/sub2api/billing", want: "plain"},
		{path: "/antigravity/models", want: "plain"},
	}
	for _, tc := range cases {
		t.Run(tc.path+tc.anthropicVersion, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, tc.path, nil)
			if tc.anthropicVersion != "" {
				c.Request.Header.Set("anthropic-version", tc.anthropicVersion)
			}
			abortGatewayError(c, http.StatusUnauthorized, "INVALID_API_KEY", "Invalid API key. Check the key and try again.")

			require.Equal(t, http.StatusUnauthorized, rec.Code)
			require.True(t, c.IsAborted())
			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			switch tc.want {
			case "anthropic":
				require.Equal(t, "error", body["type"])
				errObj := body["error"].(map[string]any)
				require.Equal(t, tc.wantType, errObj["type"])
				require.Equal(t, "INVALID_API_KEY", errObj["code"])
				require.Equal(t, "Invalid API key. Check the key and try again.", errObj["message"])
			case "openai":
				require.NotContains(t, body, "type")
				errObj := body["error"].(map[string]any)
				require.Equal(t, tc.wantType, errObj["type"])
				require.Equal(t, "INVALID_API_KEY", errObj["code"])
				require.Contains(t, errObj, "param")
				require.Equal(t, "Invalid API key. Check the key and try again.", errObj["message"])
			default:
				require.Equal(t, "INVALID_API_KEY", body["code"])
				require.Equal(t, "Invalid API key. Check the key and try again.", body["message"])
			}
		})
	}
}

// OpenAI 兼容入口：没钱 / 额度用完是 insufficient_quota（客户端据此不再重试），其余 4xx 是 invalid_request_error
func TestOpenAIGatewayErrorType(t *testing.T) {
	require.Equal(t, "insufficient_quota", openAIGatewayErrorType(http.StatusForbidden, "INSUFFICIENT_BALANCE"))
	require.Equal(t, "insufficient_quota", openAIGatewayErrorType(http.StatusTooManyRequests, "USAGE_LIMIT_EXCEEDED"))
	require.Equal(t, "rate_limit_error", openAIGatewayErrorType(http.StatusTooManyRequests, "INVALID_AUTH_RATE_LIMITED"))
	require.Equal(t, "invalid_request_error", openAIGatewayErrorType(http.StatusForbidden, "ACCESS_DENIED"))
	require.Equal(t, "api_error", openAIGatewayErrorType(http.StatusServiceUnavailable, "API_KEY_AUTH_OVERLOADED"))
}
