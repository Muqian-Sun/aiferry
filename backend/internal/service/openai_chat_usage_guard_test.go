//go:build unit

package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Grok 缺用量守卫只看实际发往上游的模型与上游回报的模型：目录模型名以 grok- 开头、承接关系把它改名成
// 别家模型时，上游不是 Grok，缺用量不应被拦成 502（计费名是目录模型，不代表上游）。
func TestChatBufferedResponseGrokGuardIgnoresCatalogName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstreamBody := strings.Join([]string{
		`data: {"type":"response.output_text.delta","sequence_number":0,"delta":"ok"}`,
		"",
		`data: {"type":"response.completed","sequence_number":1,"response":{"id":"resp_renamed","object":"response","model":"gpt-5.4","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}]}}`,
		"",
	}, "\n")
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}
	account := &Account{ID: 5, Name: "relay", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		CatalogUpstreamModels: map[string]string{"grok-alias": "gpt-5.4"}}

	result, err := (&OpenAIGatewayService{}).handleChatBufferedStreamingResponse(resp, c, account, "grok-alias", "grok-alias", "gpt-5.4", time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "grok-alias", result.BillingModel)
	require.Equal(t, "gpt-5.4", result.UpstreamModel)
}
