//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestIsWebSearchOnlyRequest(t *testing.T) {
	t.Parallel()

	require.True(t, IsWebSearchOnlyRequest([]byte(`{"tools":[{"type":"web_search_20250305","name":"web_search"}]}`)))
	require.True(t, IsWebSearchOnlyRequest([]byte(`{"tools":[{"type":"web_search_20260209","name":"web_search","max_uses":8}]}`)))
	require.False(t, IsWebSearchOnlyRequest([]byte(`{"tools":[{"type":"web_search_20250305"},{"name":"Bash"}]}`)), "正常对话还带别的工具")
	require.False(t, IsWebSearchOnlyRequest([]byte(`{"tools":[{"name":"web_search","input_schema":{}}]}`)), "客户端自定义的同名函数不是云端搜索工具")
	require.False(t, IsWebSearchOnlyRequest([]byte(`{"messages":[]}`)))
}

func TestNeedsWebSearchDelegate(t *testing.T) {
	t.Parallel()

	route := func(vendor string) CatalogRoute { return CatalogRoute{Entry: &ModelCatalogEntry{Vendor: vendor}} }
	require.True(t, NeedsWebSearchDelegate(route("openai")))
	require.True(t, NeedsWebSearchDelegate(route("deepseek")))
	require.True(t, NeedsWebSearchDelegate(route("")), "没有厂商的模型也执行不了 Anthropic 的云端搜索")
	require.False(t, NeedsWebSearchDelegate(route("anthropic")))
	require.False(t, NeedsWebSearchDelegate(route("bedrock")), "Bedrock 上的 Claude 也是 Anthropic 厂商")
}

func TestBuildWebSearchDelegateBody(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"gpt-5.5","max_tokens":2000,"stream":true,"messages":[{"role":"user","content":"q"}],` +
		`"tools":[{"type":"web_search_20260209","name":"web_search","max_uses":8,"blocked_domains":["a.com"],"user_location":{"type":"approximate","city":"Tokyo"},"cache_control":{"type":"ephemeral"}}]}`)
	out, err := BuildWebSearchDelegateBody(body)
	require.NoError(t, err)
	require.Equal(t, WebSearchDelegateModel, gjson.GetBytes(out, "model").String())
	require.JSONEq(t, `[{"type":"web_search_20250305","name":"web_search","max_uses":8,"blocked_domains":["a.com"],"user_location":{"type":"approximate","city":"Tokyo"}}]`,
		gjson.GetBytes(out, "tools").Raw)
	require.True(t, gjson.GetBytes(out, "stream").Bool(), "其余字段原样保留")
	require.Equal(t, int64(2000), gjson.GetBytes(out, "max_tokens").Int())
}

func TestWithWebSearchDelegate(t *testing.T) {
	t.Parallel()

	haiku := CatalogRoute{EntryID: 9, CanonicalModel: WebSearchDelegateModel, RequestedModel: WebSearchDelegateModel}
	ctx := WithWebSearchDelegate(context.Background(), haiku, "gpt-5.5")
	route, ok := CatalogRouteFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, int64(9), route.EntryID, "调度、转发、计费按 Haiku")
	model, ok := RequestedPublicModelFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, "gpt-5.5", model, "用量记录的请求模型仍是客户端写的")
	require.True(t, IsWebSearchDelegated(ctx))
	require.False(t, IsWebSearchDelegated(context.Background()))
}
