//go:build unit

package handler

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 分组策略已全部失效（7b-1，D4 / D5 / D8 / D10）：key 仍绑着一个把所有门都开着的分组，
// chat / responses（含生图意图）照常到上游；字段与管理接口留到 7b-3 删。
func TestGroupedKey_NoPolicyGates(t *testing.T) {
	const entryID = 199
	newGroup := func(id int64) *service.Group {
		group := keyRouteGroup(id, service.PlatformOpenAI)
		group.ClaudeCodeOnly = true
		group.AllowImageGeneration = false
		group.MaxReasoningEffort = "low"
		group.MaxReasoningEffortOverLimit = service.ReasoningEffortOverLimitDeny
		group.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"some-other-model"}}
		return group
	}

	t.Run("chat completions", func(t *testing.T) {
		group := newGroup(2701)
		key := keyRouteAccount(1701, group.ID, service.PlatformOpenAI,
			map[string]string{service.APIProtocolChatCompletions: "https://relay.example.com"}, "gpt-5.6")
		key.CatalogEntryIDs = []int64{entryID}
		hs := newKeyRouteHarness(t, group, []*service.Account{key})
		hs.openAIUpstream.respBody = openAIChatCompletionOK
		hs.openAIUpstream.contentType = "application/json"

		body := []byte(`{"model":"gpt-5.6","messages":[{"role":"user","content":"hello"}],"reasoning_effort":"high"}`)
		c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/chat/completions", body, group, service.APIProtocolChatCompletions, "")
		openAIRouteEntry(c, entryID, "gpt-5.6")

		hs.handler.ChatCompletions(c)

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		got := hs.openAIUpstream.recorded()
		require.Len(t, got, 1)
		require.True(t, strings.HasSuffix(got[0].url, "/v1/chat/completions"), got[0].url)
		require.Contains(t, string(got[0].body), `"reasoning_effort":"high"`, "推理强度由客户端定，不再被分组上限压低")
	})

	t.Run("responses with image intent", func(t *testing.T) {
		group := newGroup(2702)
		key := responsesKey(1702, group.ID, "https://relay.example.com", 1, entryID)
		hs := newKeyRouteHarness(t, group, []*service.Account{key})

		body := []byte(`{"model":"gpt-5.6","input":"draw a cat","tools":[{"type":"image_generation"}],"stream":false}`)
		c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/responses", body, group, service.APIProtocolResponses, "")
		openAIRouteEntry(c, entryID, "gpt-5.6")

		hs.handler.Responses(c)

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Len(t, hs.openAIUpstream.recorded(), 1, "生图意图不再被分组开关拦下")
	})
}
