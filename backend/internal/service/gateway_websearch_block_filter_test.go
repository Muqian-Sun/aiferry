//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// relayWebSearchBody 是 fenno 这类中转回的真实搜索块：ID 恰好也是 srvtoolu_ws_ 前缀（原来的本地搜索模拟用的
// 同一个前缀），以前会被当成模拟伪造的块剥掉。
const relayWebSearchBody = `{"model":"claude-sonnet-4-6","max_tokens":1024,"messages":[` +
	`{"role":"user","content":[{"type":"text","text":"search the weather"}]},` +
	`{"role":"assistant","content":[` +
	`{"type":"server_tool_use","id":"srvtoolu_ws_0123456789abcdef","name":"web_search","input":{"query":"weather"}},` +
	`{"type":"web_search_tool_result","tool_use_id":"srvtoolu_ws_0123456789abcdef","content":[{"type":"web_search_result","url":"https://example.com","title":"Weather"}]},` +
	`{"type":"text","text":"Here are the search results for \"weather\":"}]},` +
	`{"role":"user","content":[{"type":"text","text":"thanks, continue"}]}]}`

// genuineWebSearchBody carries real Anthropic web-search blocks.
const genuineWebSearchBody = `{"model":"claude-sonnet-4-6","max_tokens":1024,"messages":[` +
	`{"role":"user","content":[{"type":"text","text":"search"}]},` +
	`{"role":"assistant","content":[` +
	`{"type":"server_tool_use","id":"srvtoolu_01ABCDEF","name":"web_search","input":{"query":"weather"}},` +
	`{"type":"web_search_tool_result","tool_use_id":"srvtoolu_01ABCDEF","content":[{"type":"web_search_result","url":"https://example.com","title":"Weather"}]},` +
	`{"type":"text","text":"summary with citations"}]}]}`

func collectContentTypes(t *testing.T, body []byte) []string {
	t.Helper()
	var types []string
	for _, msg := range gjson.GetBytes(body, "messages").Array() {
		for _, block := range msg.Get("content").Array() {
			types = append(types, block.Get("type").String())
		}
	}
	return types
}

func TestFilterWebSearchHistoryBlocks_KeepsRelayBlocks(t *testing.T) {
	for _, model := range []string{"claude-sonnet-4-6", "totally-unknown-model"} {
		body := []byte(relayWebSearchBody)
		out := FilterWebSearchHistoryBlocks(body, model)

		require.Equal(t, string(body), string(out), model)
	}
}

func TestFilterWebSearchHistoryBlocks_KeepsGenuineBlocksForAnthropicStrict(t *testing.T) {
	body := []byte(genuineWebSearchBody)
	out := FilterWebSearchHistoryBlocks(body, "claude-sonnet-4-6")

	require.Equal(t, string(body), string(out))
}

func TestFilterWebSearchHistoryBlocks_StripsAllBlocksForPassbackRequired(t *testing.T) {
	// GLM only accepts text/thinking/image/tool_use/tool_result and rejects
	// server_tool_use with 400, so genuine blocks must be stripped as well.
	out := FilterWebSearchHistoryBlocks([]byte(genuineWebSearchBody), "glm-4.7")

	require.Equal(t, []string{"text", "text"}, collectContentTypes(t, out))
	require.NotContains(t, string(out), "server_tool_use")
	require.NotContains(t, string(out), "web_search_tool_result")
	require.Contains(t, string(out), "summary with citations")
}

func TestFilterWebSearchHistoryBlocks_KeepsGenuineBlocksForUnknownModel(t *testing.T) {
	body := []byte(genuineWebSearchBody)
	out := FilterWebSearchHistoryBlocks(body, "totally-unknown-model")

	require.Equal(t, string(body), string(out))
}

func TestFilterWebSearchHistoryBlocks_NoWebSearchBlocksFastPath(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`)
	out := FilterWebSearchHistoryBlocks(body, "claude-sonnet-4-6")

	require.Equal(t, string(body), string(out))
}

func TestFilterWebSearchHistoryBlocks_EmptiedMessageGetsPlaceholder(t *testing.T) {
	body := []byte(`{"model":"glm-4.7","messages":[` +
		`{"role":"user","content":[{"type":"text","text":"search"}]},` +
		`{"role":"assistant","content":[` +
		`{"type":"server_tool_use","id":"srvtoolu_01X","name":"web_search","input":{"query":"q"}},` +
		`{"type":"web_search_tool_result","tool_use_id":"srvtoolu_01X","content":[]}]}]}`)

	out := FilterWebSearchHistoryBlocks(body, "glm-4.7")

	msgs := gjson.GetBytes(out, "messages").Array()
	require.Len(t, msgs, 2)
	assistant := msgs[1]
	require.Equal(t, "assistant", assistant.Get("role").String())
	content := assistant.Get("content").Array()
	require.Len(t, content, 1)
	require.Equal(t, "text", content[0].Get("type").String())
	require.Equal(t, "(assistant content removed)", content[0].Get("text").String())
}

func TestFilterWebSearchHistoryBlocks_StringContentUntouched(t *testing.T) {
	// A string mentioning the pattern inside a text value must not trigger a rewrite.
	body := []byte(`{"model":"claude-sonnet-4-6","messages":[` +
		`{"role":"user","content":"please explain \"server_tool_use\" blocks"}]}`)

	out := FilterWebSearchHistoryBlocks(body, "claude-sonnet-4-6")

	require.Equal(t, string(body), string(out))
}

func TestFilterWebSearchHistoryBlocks_InvalidMessagesUnchanged(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-6","messages":"server_tool_use"}`)
	out := FilterWebSearchHistoryBlocks(body, "claude-sonnet-4-6")

	require.Equal(t, string(body), string(out))
}

func TestFilterWebSearchHistoryBlocks_PreservesOtherToolBlocks(t *testing.T) {
	body := []byte(`{"model":"glm-4.7","messages":[` +
		`{"role":"assistant","content":[` +
		`{"type":"tool_use","id":"toolu_01A","name":"get_weather","input":{}},` +
		`{"type":"server_tool_use","id":"srvtoolu_ws_abc","name":"web_search","input":{"query":"q"}},` +
		`{"type":"text","text":"result"}]},` +
		`{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_01A","content":"sunny"}]}]}`)

	out := FilterWebSearchHistoryBlocks(body, "glm-4.7")

	require.Equal(t, []string{"tool_use", "text", "tool_result"}, collectContentTypes(t, out))
}
