//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// Codex 经 /v1/responses 走 Antigravity 成品号：apply_patch 是 custom 工具，历史里是 custom_tool_call /
// custom_tool_call_output。2026-10-08 生产：这两条在转换时被丢掉，对话以模型那一轮结尾，
// Gemini 回 400「Requests ending with a model turn are not supported」，Codex 显示 Upstream request failed。
const codexCustomToolResponsesBody = `{
	"model":"gemini-3.8-flash",
	"stream":%s,
	"tools":[
		{"type":"function","name":"shell","parameters":{"type":"object","properties":{"command":{"type":"array","items":{"type":"string"}}}}},
		{"type":"custom","name":"apply_patch","description":"Apply a patch"}
	],
	"input":[
		{"type":"message","role":"user","content":[{"type":"input_text","text":"fix the typo"}]},
		{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Patching the file."}]},
		{"type":"custom_tool_call","call_id":"call_patch_1","name":"apply_patch","input":"*** Begin Patch\n*** End Patch"},
		{"type":"custom_tool_call_output","call_id":"call_patch_1","output":"Done!"}
	]
}`

// 上游这一轮又调了 apply_patch（Gemini 侧它是参数为 {input} 的函数）
func antigravityApplyPatchCallResponse() *http.Response {
	body := `data: {"response":{"responseId":"resp_patch","candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"apply_patch","args":{"input":"*** Begin Patch\n*** Update File: a.txt\n*** End Patch"}}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":5}}}` + "\n\n"
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestAntigravityForwardAsResponses_CodexCustomToolHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, stream := range []bool{false, true} {
		name := "non-stream"
		if stream {
			name = "stream"
		}
		t.Run(name, func(t *testing.T) {
			var upstreamBody []byte
			upstream := &queuedHTTPUpstreamStub{
				responses: []*http.Response{antigravityApplyPatchCallResponse()},
				onCall: func(req *http.Request, _ *queuedHTTPUpstreamStub) {
					upstreamBody, _ = io.ReadAll(req.Body)
				},
			}
			svc := newAntigravityCompatService(config.GatewayConfig{MaxLineSize: defaultMaxLineSize}, upstream)
			streamFlag := "false"
			if stream {
				streamFlag = "true"
			}
			body := []byte(strings.Replace(codexCustomToolResponsesBody, "%s", streamFlag, 1))
			c, recorder := newAntigravityCompatContext(http.MethodPost, "/v1/responses", body)
			account := newAntigravityCompatAccount(AccountTypeOAuth)
			account.CatalogUpstreamModels = map[string]string{"gemini-3.8-flash": "gemini-3.8-flash-high"}

			_, err := svc.ForwardAsResponses(context.Background(), c, account, body, nil)
			require.NoError(t, err, recorder.Body.String())

			// 发给 Gemini 的对话：custom 工具的调用与结果都在，最后一轮是用户（functionResponse）
			contents := gjson.GetBytes(upstreamBody, "request.contents").Array()
			require.NotEmpty(t, contents, string(upstreamBody))
			last := contents[len(contents)-1]
			require.Equal(t, "user", last.Get("role").String(), string(upstreamBody))
			require.Equal(t, "apply_patch", last.Get("parts.0.functionResponse.name").String(), string(upstreamBody))
			require.Contains(t, string(upstreamBody), `"functionCall":{"args":{"input":"*** Begin Patch\n*** End Patch"},"name":"apply_patch"`)

			// 回给 Codex 的：模型这一轮的 apply_patch 还原成 custom_tool_call（不是 function_call）
			out := recorder.Body.String()
			require.Equal(t, http.StatusOK, recorder.Code, out)
			if stream {
				require.Contains(t, out, `"type":"custom_tool_call"`)
				require.Contains(t, out, "response.custom_tool_call_input.done")
				require.NotContains(t, out, `"type":"function_call"`)
			} else {
				item := gjson.Get(out, `output.#(type=="custom_tool_call")`)
				require.True(t, item.Exists(), out)
				require.Equal(t, "apply_patch", item.Get("name").String())
				require.Equal(t, "*** Begin Patch\n*** Update File: a.txt\n*** End Patch", item.Get("input").String())
				require.False(t, gjson.Get(out, `output.#(type=="function_call")`).Exists(), out)
			}
		})
	}
}
