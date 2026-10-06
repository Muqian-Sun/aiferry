package service

import (
	"bytes"
	"context"
	"strings"

	"github.com/tidwall/gjson"
)

// 联网搜索按次计费（方案页 X8eQjqzjAEx3hF4xzCzKZr 第三版「联网搜索」；muqian 2026-09-30 定）：
// 官方云端搜索工具的搜索费一律按上游返回内容里的次数、以官方原价收，不乘用户倍率，不看渠道平台。
//   - Anthropic（上游是 Messages）：usage.server_tool_use.web_search_requests；没报次数时数成功的
//     web_search_tool_result 块（见 ClaudeUsage.webSearchCalls）；
//   - xAI：usage.server_side_tool_usage_details——web 搜索按次；X 搜索按取回的帖子数、主页数收（不按次）；
//   - OpenAI：输出里 type=web_search_call 且 action.type=search 的条目（打开网页 open_page、页内查找 find_in_page 不算）。
//
// 上游没给次数就不收这一笔。

// WebSearchUsage 一次请求里官方云端搜索工具的用量。
type WebSearchUsage struct {
	WebSearchCalls int
	// XSearchCalls 只用于显示搜索次数：xAI 的 X 搜索按取回条目收费，不按次。
	XSearchCalls  int
	XPostsFetched int
	XUsersFetched int
}

func (u WebSearchUsage) IsZero() bool {
	return u == WebSearchUsage{}
}

// Count 用量行上的「搜索次数」：web 搜索与 X 搜索的调用次数之和。
func (u WebSearchUsage) Count() int {
	return max(u.WebSearchCalls, 0) + max(u.XSearchCalls, 0)
}

// webSearchUsageFromResponse 从一个 Responses 响应对象（含 output / usage）里取搜索用量：
// 带 xAI 工具明细时以明细为准（fromDetails=true），否则数 output 里的 web_search_call。
func webSearchUsageFromResponse(resp gjson.Result) (usage WebSearchUsage, fromDetails bool) {
	if details := resp.Get("usage.server_side_tool_usage_details"); details.IsObject() {
		return WebSearchUsage{
			WebSearchCalls: int(details.Get("web_search_calls").Int()),
			XSearchCalls:   int(details.Get("x_search_calls").Int()),
			XPostsFetched:  int(details.Get("x_posts_fetched").Int()),
			XUsersFetched:  int(details.Get("x_users_fetched").Int()),
		}, true
	}
	calls := 0
	resp.Get("output").ForEach(func(_, item gjson.Result) bool {
		if isBillableWebSearchCall(item) {
			calls++
		}
		return true
	})
	return WebSearchUsage{WebSearchCalls: calls}, false
}

// webSearchUsageFromResponsesBody 非流式 Responses 响应体（或终止事件）。兼容层可能顶层和
// response.* 各带一份，带了 response 那份就只看它，避免同一次搜索算两遍。
func webSearchUsageFromResponsesBody(body []byte) WebSearchUsage {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return WebSearchUsage{}
	}
	root := gjson.ParseBytes(body)
	if nested := root.Get("response"); nested.Get("output").IsArray() || nested.Get("usage").IsObject() {
		root = nested
	}
	usage, _ := webSearchUsageFromResponse(root)
	return usage
}

// webSearchCalls 上游是 Messages 时按 Claude 的方式计次（muqian 2026-10-03）：usage 报了
// web_search_requests 就以它为准；没报（fenno 这类背后是别家模型的中转只回搜索块、不给次数）
// 就数成功的结果块。
func (u ClaudeUsage) webSearchCalls() int {
	if u.WebSearchRequests != nil {
		return max(*u.WebSearchRequests, 0)
	}
	return u.WebSearchResults
}

// isSuccessfulWebSearchResult 成功的 web_search_tool_result 块：content 是结果数组（可以为空，
// 背后是 OpenAI 的中转不给单条结果）；出错时 content 是 {"type":"web_search_tool_result_error"} 对象。
func isSuccessfulWebSearchResult(block gjson.Result) bool {
	return block.Get("type").String() == "web_search_tool_result" && block.Get("content").IsArray()
}

// isEmptyQueryWebSearchUse 显式带空 query 的 web_search 调用块：fenno 这类背后是 OpenAI 的中转把「打开网页」
// （open_page）也转成一个 server_tool_use，query 为空（2026-10-03 实测）。打开网页不算一次搜索（同 Responses
// 链路），它的结果块不计次。input 里没有 query（Anthropic 流式的 input 走增量下发）不算空。
func isEmptyQueryWebSearchUse(block gjson.Result) bool {
	if block.Get("type").String() != "server_tool_use" || block.Get("name").String() != "web_search" {
		return false
	}
	query := block.Get("input.query")
	return query.Exists() && strings.TrimSpace(query.String()) == ""
}

// countSuccessfulWebSearchResults 数 Anthropic 响应 content 里成功的搜索结果块（空 query 调用对应的不算）。
func countSuccessfulWebSearchResults(content gjson.Result) int {
	var usage ClaudeUsage
	content.ForEach(func(_, block gjson.Result) bool {
		usage.observeWebSearchBlock(block)
		return true
	})
	return usage.WebSearchResults
}

// observeWebSearchBlock 按块计次：成功的结果块 +1；空 query 调用对应的结果块不算。结果块总跟在它的调用块后面。
func (u *ClaudeUsage) observeWebSearchBlock(block gjson.Result) {
	switch {
	case isEmptyQueryWebSearchUse(block):
		u.webSearchSkipID = block.Get("id").String()
	case isSuccessfulWebSearchResult(block):
		if id := block.Get("tool_use_id").String(); id != "" && id == u.webSearchSkipID {
			return
		}
		u.WebSearchResults++
	}
}

// isBillableWebSearchCall 一次真正的搜索：web_search_call 且动作是 search。
func isBillableWebSearchCall(item gjson.Result) bool {
	return item.Get("type").String() == "web_search_call" && item.Get("action.type").String() == "search"
}

// responsesWebSearchCounter 流式 Responses（SSE 与 WebSocket 同一套事件）的搜索计数：以终止事件
// （response.completed / done / incomplete）里的整份响应为准；没收到终止事件（中途断开）时用已完成的
// output_item.done 计数。部分兼容上游的终止事件不带 output，所以两者取较大值——同一次搜索不会算两遍。
type responsesWebSearchCounter struct {
	itemsDone   int
	terminal    WebSearchUsage
	fromDetails bool
}

func (c *responsesWebSearchCounter) Observe(data []byte) {
	// 绝大多数事件是文本增量，先按字节筛掉
	if c == nil || (!bytes.Contains(data, []byte("web_search_call")) && !bytes.Contains(data, []byte("server_side_tool_usage_details"))) {
		return
	}
	if !gjson.ValidBytes(data) {
		return
	}
	event := gjson.ParseBytes(data)
	switch event.Get("type").String() {
	case "response.output_item.done":
		if isBillableWebSearchCall(event.Get("item")) {
			c.itemsDone++
		}
	case "response.completed", "response.done", "response.incomplete":
		c.terminal, c.fromDetails = webSearchUsageFromResponse(event.Get("response"))
	}
}

func (c *responsesWebSearchCounter) Usage() WebSearchUsage {
	if c == nil {
		return WebSearchUsage{}
	}
	if c.fromDetails {
		return c.terminal
	}
	usage := c.terminal
	usage.WebSearchCalls = max(usage.WebSearchCalls, c.itemsDone)
	return usage
}

// webSearchUsageFromSSEBody 整段 SSE 文本（透传链路读完再解析时用）。
func webSearchUsageFromSSEBody(body string) WebSearchUsage {
	var counter responsesWebSearchCounter
	forEachOpenAISSEDataPayload(body, counter.Observe)
	return counter.Usage()
}

// 厂商公开的搜索单价（USD）：播种时写进目录条目（计费只认目录），价格页当参考。来源是厂商公开价（2026-10-03 核对）：
// Anthropic web search $10/千次；OpenAI web_search $10/千次（老 search-preview 模型更贵，价格文件带出的价已写在条目上）；
// xAI web_search $5/千次、X 搜索每千条帖子 $5、每千个主页 $10（docs.x.ai/docs/pricing）。
const (
	defaultWebSearchPricePerCall = 0.01
	xaiWebSearchPricePerCall     = 0.005
	xaiXPostPrice                = 0.005
	xaiXUserPrice                = 0.01
)

// webSearchPrices 搜索费单价（USD / 次、/ 条）。
type webSearchPrices struct {
	PerCall  float64
	PerXPost float64
	PerXUser float64
}

// officialWebSearchPrices 官方搜索价：只认目录条目上的价（muqian 2026-10-06：计费从模型目录出发），
// 没设的项不收。
func officialWebSearchPrices(entry *ModelCatalogEntry) webSearchPrices {
	if entry == nil {
		return webSearchPrices{}
	}
	return webSearchPrices{}.override(entry.SearchPricePerCall, entry.XPostPrice, entry.XUserPrice)
}

// upstreamWebSearchPrices 渠道成本用的搜索上游价：只认承接关系上填的项，没填的不收（muqian 2026-10-06：
// 「上游没填费用就是免费」）。
func upstreamWebSearchPrices(binding *ModelCatalogBinding) webSearchPrices {
	if binding == nil {
		return webSearchPrices{}
	}
	return webSearchPrices{}.override(binding.SearchPricePerCall, binding.XPostPrice, binding.XUserPrice)
}

// override 用设了的项（非 nil、不为负）覆盖。
func (p webSearchPrices) override(perCall, perXPost, perXUser *float64) webSearchPrices {
	for _, item := range []struct {
		dst *float64
		src *float64
	}{{&p.PerCall, perCall}, {&p.PerXPost, perXPost}, {&p.PerXUser, perXUser}} {
		if item.src != nil && *item.src >= 0 {
			*item.dst = *item.src
		}
	}
	return p
}

func (p webSearchPrices) cost(usage WebSearchUsage) float64 {
	return float64(max(usage.WebSearchCalls, 0))*p.PerCall +
		float64(max(usage.XPostsFetched, 0))*p.PerXPost +
		float64(max(usage.XUsersFetched, 0))*p.PerXUser
}

// addWebSearchCharge 把搜索费记进这次的费用：官方原价、不乘用户倍率，计入 TotalCost 与 ActualCost。
// cost 为 nil（token 价算不出来）时只记搜索费。
func addWebSearchCharge(ctx context.Context, resolver *ModelPricingResolver, model string, usage WebSearchUsage, cost *CostBreakdown) *CostBreakdown {
	if usage.IsZero() {
		return cost
	}
	if cost == nil {
		cost = &CostBreakdown{BillingMode: string(BillingModeToken)}
	}
	charge := officialWebSearchPrices(resolver.lookupCatalogEntry(ctx, model)).cost(usage)
	cost.WebSearchCount = usage.Count()
	cost.WebSearchCost = charge
	cost.TotalCost += charge
	cost.ActualCost += charge
	return cost
}

// webSearchToolPlatform 有官方云端搜索工具的厂商（Anthropic / OpenAI / xAI）返回平台，其余返回空串。
func webSearchToolPlatform(entry *ModelCatalogEntry) string {
	switch platform := CatalogVendorPlatform(entry); platform {
	case PlatformAnthropic, PlatformOpenAI, PlatformGrok:
		return platform
	}
	return ""
}

// WebSearchDefaultPrices 厂商公开的联网搜索价（USD / 次、/ 条）：播种时写进目录、价格页当参考；X 帖子 / 主页价只有 xAI 有。
type WebSearchDefaultPrices struct {
	PerCall  float64
	PerXPost *float64
	PerXUser *float64
}

// WebSearchDefaults 这个模型厂商的公开搜索价；没有官方搜索工具的厂商返回 nil（价格页不给它填搜索价）。
func WebSearchDefaults(entry *ModelCatalogEntry) *WebSearchDefaultPrices {
	platform := webSearchToolPlatform(entry)
	if platform == "" {
		return nil
	}
	if platform != PlatformGrok {
		return &WebSearchDefaultPrices{PerCall: defaultWebSearchPricePerCall}
	}
	perXPost, perXUser := xaiXPostPrice, xaiXUserPrice
	return &WebSearchDefaultPrices{PerCall: xaiWebSearchPricePerCall, PerXPost: &perXPost, PerXUser: &perXUser}
}

// plazaWebSearchPrices 模型广场列的搜索费（官方价，不乘用户倍率）：目录条目上设了的项，与计费同源。
func plazaWebSearchPrices(entry *ModelCatalogEntry) (perCall, perXPost, perXUser *float64) {
	if entry == nil {
		return nil, nil, nil
	}
	return clonePricePtr(entry.SearchPricePerCall), clonePricePtr(entry.XPostPrice), clonePricePtr(entry.XUserPrice)
}
