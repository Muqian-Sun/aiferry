package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Claude Code 配第三方模型时的联网搜索（方案页 X8eQjqzjAEx3hF4xzCzKZr 第三版；muqian 2026-10-03 定）：
// Claude Code 的 WebSearch 是个客户端工具，模型调用它之后，Claude Code 另发一次只带 Anthropic 云端搜索工具
// （web_search_20250305）的 /v1/messages 请求去搜，用的是主模型。第三方模型执行不了这个工具（或搜了也拿不到
// 结果块），所以这一次请求交给 claude-haiku-4-5 执行，拿 Anthropic 真实的搜索结果：
//   - 只认 Claude Code（User-Agent）、tools 只有一个 web_search、请求的模型不是 Anthropic 厂商；
//   - 「联网搜索」计费项就是这条目录条目的官方价，承接它的渠道就是执行渠道（上游价、利润门照旧）；
//     它不对用户上架，用户站只显示客户端请求的模型和「联网搜索」字样；
//   - 没有可用渠道就直接报错，不换别的路子。

// WebSearchDelegateModel 代执行搜索的目录模型。
const WebSearchDelegateModel = "claude-haiku-4-5"

// webSearchDelegateToolType Haiku 4.5 只支持基础版 web_search（不支持动态过滤的新版本）。
const webSearchDelegateToolType = "web_search_20250305"

// IsWebSearchOnlyRequest 请求的 tools 只有一个 web_search 工具。
func IsWebSearchOnlyRequest(body []byte) bool {
	tools := gjson.GetBytes(body, "tools")
	if !tools.IsArray() {
		return false
	}
	list := tools.Array()
	return len(list) == 1 && strings.HasPrefix(list[0].Get("type").String(), "web_search")
}

// NeedsWebSearchDelegate 请求的模型不是 Anthropic 厂商：执行不了 Anthropic 的云端搜索工具。
func NeedsWebSearchDelegate(route CatalogRoute) bool {
	return CatalogVendorPlatform(route.Entry) != PlatformAnthropic
}

// BuildWebSearchDelegateBody 把请求改成交给 Haiku 执行：模型换成 WebSearchDelegateModel，工具换成基础版
// web_search，保留次数上限、域名白 / 黑名单与地理位置。
func BuildWebSearchDelegateBody(body []byte) ([]byte, error) {
	original := gjson.GetBytes(body, "tools.0")
	tool := map[string]any{"type": webSearchDelegateToolType, "name": "web_search"}
	for _, key := range []string{"max_uses", "allowed_domains", "blocked_domains", "user_location"} {
		if value := original.Get(key); value.Exists() {
			tool[key] = json.RawMessage(value.Raw)
		}
	}
	out, err := sjson.SetBytes(body, "model", WebSearchDelegateModel)
	if err != nil {
		return nil, err
	}
	return sjson.SetBytes(out, "tools", []any{tool})
}

// WebSearchDelegateRoute 代执行搜索的目录路由：不看上架状态（Haiku 只内部用）；目录里没有这条时返回 false。
func (s *GatewayService) WebSearchDelegateRoute(ctx context.Context) (CatalogRoute, bool) {
	if s == nil || s.resolver == nil {
		return CatalogRoute{}, false
	}
	entry := s.resolver.lookupCatalogEntry(ctx, WebSearchDelegateModel)
	if entry == nil {
		return CatalogRoute{}, false
	}
	return CatalogRoute{
		EntryID:        entry.ID,
		CanonicalModel: entry.ModelID,
		RequestedModel: entry.ModelID,
		Entry:          entry,
	}, true
}

// WithWebSearchDelegate 把目录路由换成代执行的模型（调度、转发、计费都按它）；用量记录的「请求模型」仍是
// 客户端写的模型（用户站不出现 Haiku），并标上这次是代执行的搜索。
func WithWebSearchDelegate(ctx context.Context, route CatalogRoute, requestedModel string) context.Context {
	ctx = WithCatalogRoute(ctx, route)
	ctx = context.WithValue(ctx, ctxkey.RequestedPublicModel, requestedModel)
	return context.WithValue(ctx, ctxkey.WebSearchDelegated, true)
}

// IsWebSearchDelegated 这次请求是不是交给 Haiku 代执行的搜索。
func IsWebSearchDelegated(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	delegated, _ := ctx.Value(ctxkey.WebSearchDelegated).(bool)
	return delegated
}
