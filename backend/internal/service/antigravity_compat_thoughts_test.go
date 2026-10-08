//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 客户端要推理（Codex reasoning.effort）时，转给 Gemini 的请求要让它流出思考摘要：Antigravity 思考完之前
// 连响应头都不发，不开这个首字要等整段思考（10-08 实测 -high 一道小题 23 秒）。
func TestAntigravityForwardAsResponses_RequestsThoughtSummaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	thoughtThenAnswer := `data: {"response":{"candidates":[{"content":{"role":"model","parts":[{"text":"Counting primes","thought":true}]}}]}}` + "\n\n" +
		`data: {"response":{"candidates":[{"content":{"role":"model","parts":[{"text":"16"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":30,"candidatesTokenCount":2,"thoughtsTokenCount":487}}}` + "\n\n"

	for _, tt := range []struct {
		name         string
		reasoning    string
		wantThoughts bool
	}{
		{name: "high effort", reasoning: `,"reasoning":{"effort":"high","summary":"auto"}`, wantThoughts: true},
		{name: "low effort", reasoning: `,"reasoning":{"effort":"low"}`, wantThoughts: false},
		{name: "no reasoning", reasoning: "", wantThoughts: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       io.NopCloser(strings.NewReader(thoughtThenAnswer)),
			}}}
			svc := newAntigravityCompatService(config.GatewayConfig{MaxLineSize: defaultMaxLineSize}, upstream)
			body := []byte(`{"model":"gemini-3.8-flash","stream":true,"input":"how many primes"` + tt.reasoning + `}`)
			c, recorder := newAntigravityCompatContext(http.MethodPost, "/v1/responses", body)
			account := newAntigravityCompatAccount(AccountTypeOAuth)
			account.CatalogUpstreamModels = map[string]string{"gemini-3.8-flash": "gemini-3.8-flash-high"}

			result, err := svc.ForwardAsResponses(context.Background(), c, account, body, nil)
			require.NoError(t, err)
			require.Len(t, upstream.requestBodies, 1)

			include := gjson.GetBytes(upstream.requestBodies[0], "request.generationConfig.thinkingConfig.includeThoughts")
			require.Equal(t, tt.wantThoughts, include.Bool(), string(upstream.requestBodies[0]))
			if !tt.wantThoughts {
				require.False(t, include.Exists())
				return
			}
			// 思考摘要先于正文到达客户端，用量里思考 token 照计
			out := recorder.Body.String()
			summary := strings.Index(out, "response.reasoning_summary_text.delta")
			text := strings.Index(out, "response.output_text.delta")
			require.True(t, summary >= 0 && text > summary, out)
			require.Contains(t, out, "Counting primes")
			require.Equal(t, 489, result.Usage.OutputTokens)
		})
	}
}
