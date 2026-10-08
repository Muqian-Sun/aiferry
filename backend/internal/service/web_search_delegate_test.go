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
	require.JSONEq(t, `{"type":"disabled"}`, gjson.GetBytes(out, "thinking").Raw, "代搜不思考")
}

// 客户端是为主模型写的思考预算 / 采样参数 / effort：Haiku 5.5 收到手动 budget_tokens 或非默认采样参数回 400，
// effort 为 xhigh / max 时又不能关思考，代执行时一律去掉。
func TestBuildWebSearchDelegateBodyDropsMainModelParams(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"gpt-5.5","max_tokens":2000,"temperature":0.2,"top_p":0.9,"top_k":40,` +
		`"thinking":{"type":"enabled","budget_tokens":4096},"output_config":{"effort":"xhigh"},` +
		`"messages":[{"role":"user","content":"q"}],"tools":[{"type":"web_search_20250305","name":"web_search"}]}`)
	out, err := BuildWebSearchDelegateBody(body)
	require.NoError(t, err)
	for _, key := range []string{"temperature", "top_p", "top_k", "output_config"} {
		require.False(t, gjson.GetBytes(out, key).Exists(), key)
	}
	require.JSONEq(t, `{"type":"disabled"}`, gjson.GetBytes(out, "thinking").Raw)
	require.Equal(t, `[{"role":"user","content":"q"}]`, gjson.GetBytes(out, "messages").Raw)
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
