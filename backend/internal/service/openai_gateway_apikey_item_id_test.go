//go:build unit

package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIGatewayService_SetupTokenLegacy_SanitizesAndTransforms(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstreamSSE := "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_test\",\"model\":\"gpt-5.6-sol\",\"output\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\ndata: [DONE]\n\n"
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(upstreamSSE)),
	}}
	svc := newOpenAIImageGenerationControlTestService(upstream)
	c, _ := newOpenAIImageGenerationControlTestContext("codex_cli_rs/0.144.1")
	account := newOpenAIImageGenerationControlTestAccount()
	account.Type = AccountTypeSetupToken
	account.Credentials = map[string]any{
		"access_token":       "setup-token",
		"chatgpt_account_id": "chatgpt-account",
	}

	body := []byte(`{
		"model":"gpt-5.6-sol",
		"stream":true,
		"store":true,
		"reasoning":{"mode":"pro"},
		"instructions":"test",
		"input":[
			{"type":"custom_tool_call","id":"fc_wrong_custom","call_id":"call_custom_1","name":"apply_patch","input":"patch"},
			{"type":"function_call_output","call_id":"call_orphan","output":"orphan"}
		]
	}`)

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://chatgpt.com/backend-api/codex/responses", upstream.lastReq.URL.String())
	require.Equal(t, false, gjson.GetBytes(upstream.lastBody, "store").Bool())
	require.Equal(t, "max", gjson.GetBytes(upstream.lastBody, "reasoning.effort").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "reasoning.mode").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "input.0.id").Exists())
	require.Len(t, gjson.GetBytes(upstream.lastBody, "input").Array(), 1)
}

func TestShouldStripOpenAIResponsesInputItemID_Reasoning(t *testing.T) {
	cases := []struct {
		name     string
		itemType string
		id       string
		want     bool
	}{
		{"reasoning item_* id", "reasoning", "item_bad_reasoning", true},
		{"reasoning rs id", "reasoning", "rs_abc123", false},
		{"reasoning empty id", "reasoning", "", true},
		{"message msg id", "message", "msg_abc", false},
		{"message item id", "message", "item_x", true},
		{"function_call fc id", "function_call", "fc_abc", false},
		{"function_call ctc id", "function_call", "ctc_abc", true},
		{"function_call item id", "function_call", "item_x", true},
		{"custom tool ctc id", "custom_tool_call", "ctc_abc", false},
		{"custom tool fc id", "custom_tool_call", "fc_abc", true},
		{"tool search tsc id", "tool_search_call", "tsc_abc", false},
		{"tool search fc id", "tool_search_call", "fc_abc", true},
		{"web search ws id", "web_search_call", "ws_001", false},
		{"web search item id", "web_search_call", "item_001", true},
		{"custom output fc id", "custom_tool_call_output", "fc_001", false},
		{"custom output ctco id", "custom_tool_call_output", "ctco_001", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, shouldStripOpenAIResponsesInputItemID(tc.itemType, tc.id))
		})
	}
}

func TestSanitizeOpenAIResponsesInputItemIDs_AllocationGrowthIsLinear(t *testing.T) {
	makeBody := func(itemCount int) []byte {
		items := make([]string, itemCount)
		for i := range items {
			items[i] = fmt.Sprintf(`{"type":"message","id":"item_%d","role":"user","content":[{"type":"input_text","text":"hello"}]}`, i)
		}
		return []byte(`{"model":"gpt-5.6-sol","input":[` + strings.Join(items, ",") + `]}`)
	}
	allocatedBytes := func(body []byte) uint64 {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		sanitized, changed, err := sanitizeOpenAIResponsesInputItemIDs(body)
		runtime.ReadMemStats(&after)
		require.NoError(t, err)
		require.True(t, changed)
		require.NotEmpty(t, sanitized)
		return after.TotalAlloc - before.TotalAlloc
	}

	smallAllocated := allocatedBytes(makeBody(20))
	largeAllocated := allocatedBytes(makeBody(200))
	require.Less(t, largeAllocated, smallAllocated*30,
		"10x more input items must not cause quadratic whole-body allocation growth")
}

func TestNormalizeOpenAIResponsesWebSocketCompatibilityBodyPreservesOpaqueReferences(t *testing.T) {
	body := []byte(`{"type":"response.create","input":[
		{"type":"custom_tool_call","id":"ctc_call","call_id":"call_custom","name":"apply_patch","input":"patch"},
		{"type":"custom_tool_call_output","id":"ctco_bad","call_id":"call_custom","output":"done"},
		{"type":"item_reference","id":"ctco_bad"},
		{"type":"future_item","id":"item_future","payload":"keep"}
	]}`)

	for _, accountType := range []string{AccountTypeAPIKey, AccountTypeOAuth} {
		t.Run(accountType, func(t *testing.T) {
			normalized, changed, err := normalizeOpenAIResponsesWebSocketCompatibilityBody(body, &Account{
				Platform: PlatformOpenAI,
				Type:     accountType,
			}, false)

			require.NoError(t, err)
			require.True(t, changed)
			require.Len(t, gjson.GetBytes(normalized, "input").Array(), 4)
			require.Equal(t, "ctc_call", gjson.GetBytes(normalized, "input.0.id").String())
			require.Equal(t, "call_custom", gjson.GetBytes(normalized, "input.1.call_id").String())
			require.False(t, gjson.GetBytes(normalized, "input.1.id").Exists())
			require.Equal(t, "ctco_bad", gjson.GetBytes(normalized, "input.2.id").String())
			require.Equal(t, "item_future", gjson.GetBytes(normalized, "input.3.id").String())

			second, changedAgain, err := normalizeOpenAIResponsesWebSocketCompatibilityBody(normalized, &Account{
				Platform: PlatformOpenAI,
				Type:     accountType,
			}, false)
			require.NoError(t, err)
			require.False(t, changedAgain)
			require.JSONEq(t, string(normalized), string(second))
		})
	}
}
