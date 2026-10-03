//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestForwardGrokResponses_PropagatesWebSearchFromJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"grok","input":"search something","tools":[{"type":"web_search"}],"stream":false}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))

	account := healthyGrokOAuthGatewayTestAccount(9901, "access-token")
	repo := &mockAccountRepoForPlatform{accountsByID: map[int64]*Account{account.ID: account}}
	upstreamBody := `{
		"id":"resp_search_bill",
		"object":"response",
		"model":"grok-4.5",
		"status":"completed",
		"output":[
			{"type":"web_search_call","id":"ws1","status":"completed","action":{"type":"search","query":"q"}},
			{"type":"web_search_call","id":"ws2","status":"completed","action":{"type":"open_page","url":"https://a"}},
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}
		],
		"usage":{"input_tokens":10,"output_tokens":5,
			"server_side_tool_usage_details":{"web_search_calls":1,"x_search_calls":1,"x_posts_fetched":8,"x_users_fetched":2}}
	}`
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader([]byte(upstreamBody))),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream:      upstream,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		accountRepo:       repo,
	}

	result, err := svc.forwardGrokResponses(context.Background(), c, account, body, "grok", false, time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, WebSearchUsage{WebSearchCalls: 1, XSearchCalls: 1, XPostsFetched: 8, XUsersFetched: 2}, result.WebSearch,
		"xAI 的工具明细要带到计费")
	require.Equal(t, 10, result.Usage.InputTokens)
	require.Equal(t, 5, result.Usage.OutputTokens)
}

func TestForwardGrokResponses_PropagatesWebSearchFromSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"grok","input":"search","tools":[{"type":"web_search"}],"stream":true}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))

	account := healthyGrokOAuthGatewayTestAccount(9902, "access-token")
	repo := &mockAccountRepoForPlatform{accountsByID: map[int64]*Account{account.ID: account}}
	// item.done + response.completed for same call_id must count once after wire-up.
	sse := "data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"web_search_call\",\"id\":\"ws1\",\"action\":{\"type\":\"search\"}}}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_s\",\"status\":\"completed\",\"output\":[{\"type\":\"web_search_call\",\"id\":\"ws1\",\"action\":{\"type\":\"search\"}}],\"usage\":{\"input_tokens\":3,\"output_tokens\":1}}}\n\n"
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(bytes.NewReader([]byte(sse))),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream:      upstream,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		accountRepo:       repo,
	}

	result, err := svc.forwardGrokResponses(context.Background(), c, account, body, "grok", true, time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, WebSearchUsage{WebSearchCalls: 1}, result.WebSearch, "流式的搜索次数要带到结果里，且同一次搜索只算一次")
}
