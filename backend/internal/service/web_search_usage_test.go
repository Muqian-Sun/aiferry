//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// xaiWebSearchResponse 是 qiniu · Responses 转发 grok-4.5 的真实返回（2026-10-03 抓），截掉了推理、正文和多余来源。
const xaiWebSearchResponse = `{
	"id":"resp_IngvIOiomVcvIVMs","object":"response","model":"grok-4.5","status":"completed",
	"output":[
		{"type":"reasoning","id":"rs_1","summary":[]},
		{"id":"ws_3304d60d-7002-98c9-81cf-07d9dd2ebb45_call-cca16d70-3a30-4863-a711-b9a6ed330a46-0","action":{"query":"xAI recent news OR post","type":"search","queries":null,"sources":[{"type":"url","url":"https://telanganatoday.com/tag/xai"}]},"status":"completed","type":"web_search_call"},
		{"id":"ws_3304d60d-7002-98c9-81cf-07d9dd2ebb45_call-cca16d70-3a30-4863-a711-b9a6ed330a46-1","action":{"query":"xAI site:x.com OR site:twitter.com recent","type":"search","queries":null,"sources":[{"type":"url","url":"https://x.com/spacexai"}]},"status":"completed","type":"web_search_call"},
		{"type":"message","role":"assistant","content":[{"type":"output_text","text":"..."}]}
	],
	"usage":{"input_tokens":18349,"input_tokens_details":{"audio_tokens":null,"cached_tokens":7424,"text_tokens":null},
		"output_tokens":1135,"output_tokens_details":{"reasoning_tokens":1048,"text_tokens":null},"total_tokens":19484,
		"cost":null,"num_sources_used":0,"num_server_side_tools_used":2,"cost_in_usd_ticks":205016480,
		"server_side_tool_usage_details":{"web_search_calls":2,"x_search_calls":0,"x_posts_fetched":0,"x_users_fetched":0,
			"code_interpreter_calls":0,"file_search_calls":0,"mcp_calls":0,"document_search_calls":0,"image_generation_calls":0}}
}`

func TestWebSearchUsageFromResponsesBody(t *testing.T) {
	t.Parallel()

	require.Equal(t, WebSearchUsage{WebSearchCalls: 2}, webSearchUsageFromResponsesBody([]byte(xaiWebSearchResponse)))

	// xAI 明细为准：X 搜索按取回条目计，output 里的条目不再另数
	xSearch := `{"output":[
		{"type":"web_search_call","id":"a","action":{"type":"search"}},
		{"type":"web_search_call","id":"b","action":{"type":"search"}}],
		"usage":{"server_side_tool_usage_details":{"web_search_calls":1,"x_search_calls":2,"x_posts_fetched":12,"x_users_fetched":3}}}`
	require.Equal(t, WebSearchUsage{WebSearchCalls: 1, XSearchCalls: 2, XPostsFetched: 12, XUsersFetched: 3},
		webSearchUsageFromResponsesBody([]byte(xSearch)))

	// OpenAI：只数动作是 search 的 web_search_call；打开网页、页内查找、同名的客户端函数调用都不算
	openai := `{"output":[
		{"type":"web_search_call","id":"1","action":{"type":"search","query":"q"}},
		{"type":"web_search_call","id":"2","action":{"type":"open_page","url":"https://a"}},
		{"type":"web_search_call","id":"3","action":{"type":"find_in_page","pattern":"x"}},
		{"type":"web_search_call","id":"4","action":{"type":"search","query":"q2"}},
		{"type":"function_call","name":"web_search","call_id":"c"}]}`
	require.Equal(t, WebSearchUsage{WebSearchCalls: 2}, webSearchUsageFromResponsesBody([]byte(openai)))

	// 顶层和 response.* 各带一份时只看 response 那份
	nested := `{"output":[{"type":"web_search_call","action":{"type":"search"}}],
		"response":{"output":[{"type":"web_search_call","action":{"type":"search"}},{"type":"web_search_call","action":{"type":"search"}}]}}`
	require.Equal(t, 2, webSearchUsageFromResponsesBody([]byte(nested)).WebSearchCalls)
	nestedNull := `{"output":[{"type":"web_search_call","action":{"type":"search"}}],"response":{"output":null}}`
	require.Equal(t, 1, webSearchUsageFromResponsesBody([]byte(nestedNull)).WebSearchCalls)

	require.True(t, webSearchUsageFromResponsesBody(nil).IsZero())
	require.True(t, webSearchUsageFromResponsesBody([]byte(`not json`)).IsZero())
}

func TestResponsesWebSearchCounter(t *testing.T) {
	t.Parallel()

	itemDone := func(id, action string) []byte {
		return []byte(`{"type":"response.output_item.done","item":{"type":"web_search_call","id":"` + id + `","action":{"type":"` + action + `"}}}`)
	}

	t.Run("item.done 与终止事件是同一次搜索，只算一次", func(t *testing.T) {
		var c responsesWebSearchCounter
		c.Observe(itemDone("ws1", "search"))
		c.Observe([]byte(`{"type":"response.output_text.delta","delta":"web_search_call"}`))
		c.Observe([]byte(`{"type":"response.completed","response":{"output":[{"type":"web_search_call","id":"ws1","action":{"type":"search"}}],"usage":{"input_tokens":3}}}`))
		require.Equal(t, WebSearchUsage{WebSearchCalls: 1}, c.Usage())
	})

	t.Run("中途断开没收到终止事件，按已完成的条目算", func(t *testing.T) {
		var c responsesWebSearchCounter
		c.Observe(itemDone("ws1", "search"))
		c.Observe(itemDone("ws2", "search"))
		c.Observe(itemDone("ws3", "open_page"))
		require.Equal(t, WebSearchUsage{WebSearchCalls: 2}, c.Usage())
	})

	t.Run("终止事件不带 output 时取已完成条目数", func(t *testing.T) {
		var c responsesWebSearchCounter
		c.Observe(itemDone("ws1", "search"))
		c.Observe(itemDone("ws2", "search"))
		c.Observe([]byte(`{"type":"response.completed","response":{"output":[],"usage":{"input_tokens":3}}}`))
		require.Equal(t, WebSearchUsage{WebSearchCalls: 2}, c.Usage())
	})

	t.Run("xAI 明细以终止事件为准", func(t *testing.T) {
		var c responsesWebSearchCounter
		c.Observe(itemDone("ws1", "search"))
		c.Observe(itemDone("ws2", "search"))
		c.Observe(itemDone("ws3", "search"))
		c.Observe([]byte(`{"type":"response.completed","response":` + xaiWebSearchResponse + `}`))
		require.Equal(t, WebSearchUsage{WebSearchCalls: 2}, c.Usage())

		var x responsesWebSearchCounter
		x.Observe([]byte(`{"type":"response.completed","response":{"output":[],"usage":{"server_side_tool_usage_details":{"x_search_calls":1,"x_posts_fetched":20}}}}`))
		require.Equal(t, WebSearchUsage{XSearchCalls: 1, XPostsFetched: 20}, x.Usage())
	})

	t.Run("整段 SSE 文本", func(t *testing.T) {
		sse := "event: response.output_item.done\ndata: " + string(itemDone("ws1", "search")) + "\n\n" +
			"data: {\"type\":\"response.completed\",\"response\":{\"output\":[{\"type\":\"web_search_call\",\"id\":\"ws1\",\"action\":{\"type\":\"search\"}}]}}\n\n" +
			"data: [DONE]\n\n"
		require.Equal(t, WebSearchUsage{WebSearchCalls: 1}, webSearchUsageFromSSEBody(sse))
	})
}

func TestOfficialWebSearchPrices(t *testing.T) {
	t.Parallel()

	configured := 0.02
	require.Equal(t, webSearchPrices{PerCall: 0.01, PerXPost: 0.005, PerXUser: 0.01}, officialWebSearchPrices(nil))
	require.Equal(t, 0.01, officialWebSearchPrices(&ModelCatalogEntry{Vendor: "openai"}).PerCall)
	require.Equal(t, 0.01, officialWebSearchPrices(&ModelCatalogEntry{Vendor: "anthropic"}).PerCall)
	require.Equal(t, 0.005, officialWebSearchPrices(&ModelCatalogEntry{Vendor: "xai"}).PerCall)
	require.Equal(t, 0.02, officialWebSearchPrices(&ModelCatalogEntry{Vendor: "xai", SearchPricePerCall: &configured}).PerCall)

	// xAI：2 次 web 搜索 + 10 条帖子 + 3 个主页 = 0.01 + 0.05 + 0.03
	xai := officialWebSearchPrices(&ModelCatalogEntry{Vendor: "xai"})
	require.InDelta(t, 0.09, xai.cost(WebSearchUsage{WebSearchCalls: 2, XSearchCalls: 4, XPostsFetched: 10, XUsersFetched: 3}), 1e-12)
}

func TestAddWebSearchCharge(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	resolver := newResolverWithSeededEntries(newTestBillingService(), ModelCatalogEntry{
		ModelID: "grok-4.5", Vendor: "xai", BillingMode: BillingModeToken, Status: ModelCatalogStatusListed,
		InputPrice: testPtrFloat64(2e-6), OutputPrice: testPtrFloat64(6e-6), ManagedBy: ModelCatalogManagedByAdmin,
	})

	// 搜索费按官方原价、不乘用户倍率，计入总价和实付
	tokenCost := &CostBreakdown{TotalCost: 1, ActualCost: 0.5}
	got := addWebSearchCharge(ctx, resolver, "grok-4.5", WebSearchUsage{WebSearchCalls: 2, XPostsFetched: 4}, tokenCost)
	require.Same(t, tokenCost, got)
	require.Equal(t, 2, got.WebSearchCount)
	require.InDelta(t, 0.03, got.WebSearchCost, 1e-12) // 2 × 0.005 + 4 × 0.005
	require.InDelta(t, 1.03, got.TotalCost, 1e-12)
	require.InDelta(t, 0.53, got.ActualCost, 1e-12)

	// 没有搜索：原样返回
	plain := &CostBreakdown{TotalCost: 1, ActualCost: 0.5}
	require.Equal(t, &CostBreakdown{TotalCost: 1, ActualCost: 0.5}, addWebSearchCharge(ctx, resolver, "grok-4.5", WebSearchUsage{}, plain))

	// token 价算不出来：只记搜索费
	only := addWebSearchCharge(ctx, resolver, "not-in-catalog", WebSearchUsage{WebSearchCalls: 1}, nil)
	require.InDelta(t, 0.01, only.ActualCost, 1e-12)
	require.InDelta(t, 0.01, only.TotalCost, 1e-12)
}

func TestPlazaWebSearchPrices(t *testing.T) {
	t.Parallel()

	configured := 0.02
	web, post, user := plazaWebSearchPrices(&ModelCatalogEntry{Vendor: "openai", SearchPricePerCall: &configured})
	require.Equal(t, 0.02, *web)
	require.Nil(t, post)
	require.Nil(t, user)

	web, post, user = plazaWebSearchPrices(&ModelCatalogEntry{Vendor: "anthropic"})
	require.Equal(t, 0.01, *web)
	require.Nil(t, post)
	require.Nil(t, user)

	web, post, user = plazaWebSearchPrices(&ModelCatalogEntry{Vendor: "xai"})
	require.Equal(t, 0.005, *web)
	require.Equal(t, 0.005, *post)
	require.Equal(t, 0.01, *user)

	// 没有官方搜索工具的厂商不列
	web, post, user = plazaWebSearchPrices(&ModelCatalogEntry{Vendor: "deepseek"})
	require.Nil(t, web)
	require.Nil(t, post)
	require.Nil(t, user)
}

// fennoMessagesSearchEvents 是 fenno · Messages（背后是 OpenAI）回的真实流（2026-10-03 抓，截掉了文本增量）：
// 有一次搜索的 server_tool_use + web_search_tool_result 块，usage 却没给 web_search_requests。
var fennoMessagesSearchEvents = []string{
	`{"type":"message_start","message":{"id":"resp_066abeddcbe85109016ac10c9471d487d19871414d9a9eeaaf","type":"message","role":"assistant","content":[],"model":"gpt-5.5","stop_reason":null,"usage":{"input_tokens":0,"output_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}`,
	`{"type":"content_block_start","index":0,"content_block":{"type":"server_tool_use","id":"srvtoolu_ws_066abeddcbe85109016ac10c9e9aec87d1888bc447951162ad","name":"web_search","input":{"query":"Tokyo population 2026 official estimate"}}}`,
	`{"type":"content_block_start","index":1,"content_block":{"type":"web_search_tool_result","tool_use_id":"srvtoolu_ws_066abeddcbe85109016ac10c9e9aec87d1888bc447951162ad","content":[]}}`,
	`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":5244,"output_tokens":119,"cache_creation_input_tokens":0,"cache_read_input_tokens":1664}}`,
}

// 出错的搜索：Anthropic 不计费，结果块的 content 是错误对象
const anthropicSearchErrorBlock = `{"type":"content_block_start","index":3,"content_block":{"type":"web_search_tool_result","tool_use_id":"srvtoolu_2","content":{"type":"web_search_tool_result_error","error_code":"max_uses_exceeded"}}}`

// fennoOpenPageEvents 是 fenno · Messages 的真实流（2026-10-03 抓）：先搜一次，再「打开网页」——
// 打开网页被转成一个 query 为空的 server_tool_use，不算一次搜索。
var fennoOpenPageEvents = []string{
	`{"type":"content_block_start","index":2,"content_block":{"type":"server_tool_use","id":"srvtoolu_ws_037315615a976515016ac1179c73e887d2b29dd8a0b3a06f4a","name":"web_search","input":{"query":"Tokyo Metropolitan Government population press release August 2026"}}}`,
	`{"type":"content_block_start","index":3,"content_block":{"type":"web_search_tool_result","tool_use_id":"srvtoolu_ws_037315615a976515016ac1179c73e887d2b29dd8a0b3a06f4a","content":[]}}`,
	`{"type":"content_block_start","index":5,"content_block":{"type":"server_tool_use","id":"srvtoolu_ws_037315615a976515016ac117a8469087d2ab62dce354a4937d","name":"web_search","input":{"query":""}}}`,
	`{"type":"content_block_start","index":6,"content_block":{"type":"web_search_tool_result","tool_use_id":"srvtoolu_ws_037315615a976515016ac117a8469087d2ab62dce354a4937d","content":[]}}`,
	`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":5189,"output_tokens":397,"cache_creation_input_tokens":0,"cache_read_input_tokens":3712}}`,
}

func TestClaudeUsageWebSearchCalls(t *testing.T) {
	t.Parallel()

	reported := func(n int) *int { return &n }
	require.Equal(t, 0, ClaudeUsage{}.webSearchCalls())
	// 上游没报次数：数成功的结果块
	require.Equal(t, 2, ClaudeUsage{WebSearchResults: 2}.webSearchCalls())
	// 报了就以它为准（包括报 0、比结果块少——出错的搜索不计费）
	require.Equal(t, 1, ClaudeUsage{WebSearchRequests: reported(1), WebSearchResults: 2}.webSearchCalls())
	require.Equal(t, 0, ClaudeUsage{WebSearchRequests: reported(0), WebSearchResults: 1}.webSearchCalls())
}

func TestAnthropicUsageParsesWebSearch(t *testing.T) {
	t.Parallel()

	svc := &GatewayService{}
	streamed := func(events ...string) ClaudeUsage {
		usage := &ClaudeUsage{}
		for _, e := range events {
			svc.parseSSEUsage(e, usage)
		}
		return *usage
	}
	bedrockStreamed := func(events ...string) ClaudeUsage {
		usage := &ClaudeUsage{}
		for _, e := range events {
			parseSSEUsagePassthrough(e, usage)
		}
		return *usage
	}

	t.Run("fenno 这类中转没报次数：数结果块", func(t *testing.T) {
		got := streamed(fennoMessagesSearchEvents...)
		require.Nil(t, got.WebSearchRequests)
		require.Equal(t, 1, got.webSearchCalls())
		require.Equal(t, 119, got.OutputTokens)
		require.Equal(t, 1, bedrockStreamed(fennoMessagesSearchEvents...).webSearchCalls())

		// 出错的结果块不算
		withError := append(append([]string{}, fennoMessagesSearchEvents[:3]...), anthropicSearchErrorBlock, fennoMessagesSearchEvents[3])
		require.Equal(t, 1, streamed(withError...).webSearchCalls())
		require.Equal(t, 1, bedrockStreamed(withError...).webSearchCalls())
	})

	t.Run("打开网页（query 为空的调用）不算一次搜索", func(t *testing.T) {
		require.Equal(t, 1, streamed(fennoOpenPageEvents...).webSearchCalls())
		require.Equal(t, 1, bedrockStreamed(fennoOpenPageEvents...).webSearchCalls())

		bridge := &ClaudeUsage{}
		for _, raw := range fennoOpenPageEvents {
			var event apicompat.AnthropicStreamEvent
			require.NoError(t, json.Unmarshal([]byte(raw), &event))
			observeAnthropicWebSearch(bridge, &event)
		}
		require.Equal(t, 1, bridge.webSearchCalls())

		body := []byte(`{"content":[
			{"type":"server_tool_use","id":"s1","name":"web_search","input":{"query":"q"}},
			{"type":"web_search_tool_result","tool_use_id":"s1","content":[]},
			{"type":"server_tool_use","id":"s2","name":"web_search","input":{"query":""}},
			{"type":"web_search_tool_result","tool_use_id":"s2","content":[]},
			{"type":"server_tool_use","id":"s3","name":"web_search","input":{}},
			{"type":"web_search_tool_result","tool_use_id":"s3","content":[]}],
			"usage":{"input_tokens":1,"output_tokens":2}}`)
		// s2 是打开网页不算；s3 没带 query（input 走增量）照常算
		require.Equal(t, 2, parseClaudeUsageFromResponseBody(body).webSearchCalls())
	})

	t.Run("Anthropic 报了次数：以 message_delta 的累计值为准", func(t *testing.T) {
		events := []string{
			`{"type":"message_start","message":{"usage":{"input_tokens":10,"output_tokens":1,"server_tool_use":{"web_search_requests":0}}}}`,
			fennoMessagesSearchEvents[2],
			fennoMessagesSearchEvents[2],
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":10682,"output_tokens":510,"server_tool_use":{"web_search_requests":3}}}`,
		}
		require.Equal(t, 3, streamed(events...).webSearchCalls())
		require.Equal(t, 3, bedrockStreamed(events...).webSearchCalls())
	})

	t.Run("message_start 里的 0 不算报了次数", func(t *testing.T) {
		events := []string{
			`{"type":"message_start","message":{"usage":{"input_tokens":10,"output_tokens":1,"server_tool_use":{"web_search_requests":0}}}}`,
			fennoMessagesSearchEvents[2],
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`,
		}
		require.Equal(t, 1, streamed(events...).webSearchCalls())
		require.Equal(t, 1, bedrockStreamed(events...).webSearchCalls())
	})

	t.Run("Bedrock 非流式", func(t *testing.T) {
		body := []byte(`{"content":[{"type":"server_tool_use","id":"s1","name":"web_search"},{"type":"web_search_tool_result","tool_use_id":"s1","content":[{"type":"web_search_result","url":"https://a"}]},{"type":"text","text":"ok"}],"usage":{"input_tokens":1,"output_tokens":2}}`)
		require.Equal(t, 1, parseClaudeUsageFromResponseBody(body).webSearchCalls())
		reported := []byte(`{"content":[],"usage":{"input_tokens":1,"output_tokens":2,"server_tool_use":{"web_search_requests":2}}}`)
		require.Equal(t, 2, parseClaudeUsageFromResponseBody(reported).webSearchCalls())
	})

	t.Run("协议转换桥（Responses / Chat 客户端走 Anthropic 上游）", func(t *testing.T) {
		usage := &ClaudeUsage{}
		for _, raw := range fennoMessagesSearchEvents {
			var event apicompat.AnthropicStreamEvent
			require.NoError(t, json.Unmarshal([]byte(raw), &event))
			observeAnthropicWebSearch(usage, &event)
		}
		require.Equal(t, 1, usage.webSearchCalls())

		var delta apicompat.AnthropicStreamEvent
		require.NoError(t, json.Unmarshal([]byte(`{"type":"message_delta","usage":{"output_tokens":7,"server_tool_use":{"web_search_requests":4}}}`), &delta))
		observeAnthropicWebSearch(usage, &delta)
		require.Equal(t, 4, usage.webSearchCalls())
		require.Equal(t, WebSearchUsage{WebSearchCalls: 4}, (&ForwardResult{Usage: *usage}).webSearchUsage())
	})
}

func TestGatewayRecordUsageCostAddsWebSearchWithoutRate(t *testing.T) {
	t.Parallel()

	svc := &GatewayService{billingService: newTestBillingService()}
	result := &ForwardResult{Model: "claude-sonnet-4", Usage: ClaudeUsage{InputTokens: 1000, OutputTokens: 500, WebSearchResults: 3}}
	// claude-sonnet-4 兜底价 $3 / $15：token 0.0105，× 倍率 2 = 0.021；搜索 3 × 0.01 不乘倍率
	cost, tokenPath := svc.calculateRecordUsageCost(context.Background(), result, &APIKey{}, "claude-sonnet-4", 2, time.Time{})
	require.True(t, tokenPath)
	require.Equal(t, 3, cost.WebSearchCount)
	require.InDelta(t, 0.03, cost.WebSearchCost, 1e-12)
	require.InDelta(t, 0.0105+0.03, cost.TotalCost, 1e-9)
	require.InDelta(t, 0.021+0.03, cost.ActualCost, 1e-9)
}

// 渠道成本里的搜索部分：承接关系上还没有搜索上游价，按官方搜索价记。
func TestRecordUsageAccountCostIncludesWebSearch(t *testing.T) {
	t.Parallel()

	bs := newTestBillingService()
	resolver := newUpstreamCostTestResolver(t, bs, ModelCatalogEntry{
		ID: 1, ModelID: "gpt-5.5", Vendor: "openai", BillingMode: BillingModeToken, Status: ModelCatalogStatusListed,
		InputPrice: testPtrFloat64(5e-6), OutputPrice: testPtrFloat64(30e-6), ManagedBy: ModelCatalogManagedByAdmin,
		SearchPricePerCall: testPtrFloat64(0.02),
	}, ModelCatalogBinding{AccountID: 7, InputPrice: 1e-6, OutputPrice: 2e-6})
	tokens := UsageTokens{InputTokens: 1000, OutputTokens: 1000}
	// token 成本 1000 × 1e-6 + 1000 × 2e-6 = 0.003；搜索 2 × 0.02 = 0.04
	got := recordUsageAccountCost(context.Background(), bs, resolver, 7, []string{"gpt-5.5"}, tokens, WebSearchUsage{WebSearchCalls: 2}, time.Time{}, "")
	require.InDelta(t, 0.043, got, 1e-12)
	require.InDelta(t, 0.003, recordUsageAccountCost(context.Background(), bs, resolver, 7, []string{"gpt-5.5"}, tokens, WebSearchUsage{}, time.Time{}, ""), 1e-12)
}

// Anthropic 主链路非流式：搜索次数从响应体的 usage.server_tool_use 读出来。
func TestHandleNonStreamingResponseParsesWebSearchRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	body := []byte(`{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],
		"usage":{"input_tokens":10682,"output_tokens":510,"server_tool_use":{"web_search_requests":2}}}`)
	// 没报次数的中转：数成功的结果块（出错的不算）
	unreported := []byte(`{"id":"msg_2","type":"message","role":"assistant","content":[
		{"type":"server_tool_use","id":"s1","name":"web_search","input":{"query":"q"}},
		{"type":"web_search_tool_result","tool_use_id":"s1","content":[]},
		{"type":"web_search_tool_result","tool_use_id":"s2","content":{"type":"web_search_tool_result_error","error_code":"unavailable"}},
		{"type":"text","text":"ok"}],
		"usage":{"input_tokens":5244,"output_tokens":119}}`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
	svc := &GatewayService{cfg: &config.Config{}, rateLimitService: &RateLimitService{}}

	usage, err := svc.handleNonStreamingResponse(context.Background(), resp, c, &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}, "claude-sonnet-4-6", "claude-sonnet-4-6")
	require.NoError(t, err)
	require.Equal(t, 2, usage.webSearchCalls())
	require.Equal(t, 510, usage.OutputTokens)

	resp = &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(unreported)),
	}
	c, _ = gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	usage, err = svc.handleNonStreamingResponse(context.Background(), resp, c, &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}, "gpt-5.5", "gpt-5.5")
	require.NoError(t, err)
	require.Nil(t, usage.WebSearchRequests)
	require.Equal(t, 1, usage.webSearchCalls())
}
