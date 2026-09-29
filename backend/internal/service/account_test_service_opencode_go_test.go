//go:build unit

package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// openCodeGoTestAccount 造一个只配了单个协议地址的 OpenCode key——一个资源只承接一个
// 上游协议（NormalizeProtocolEndpoints 在配置入口强制），连接测试按该协议选探针。
func openCodeGoTestAccount(id int64, protocol, baseURL string) *Account {
	return &Account{
		ID:          id,
		Name:        "oc",
		Platform:    PlatformOpenCodeGo,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-opencode-go-test",
		},
		ProtocolEndpoints: map[string]string{protocol: baseURL},
	}
}

func TestAccountTestService_OpenCodeGoChatCompletionsKeyUsesChatCompletions(t *testing.T) {
	account := openCodeGoTestAccount(401, APIProtocolChatCompletions, "https://opencode.ai/zen/go/v1")
	svc, upstream := adaptiveCNAccountTestService(account, adaptiveCNChatTestResponse())
	c, recorder := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "deepseek-v4-flash", "hi", AccountTestModeDefault)

	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://opencode.ai/zen/go/v1/chat/completions", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer sk-opencode-go-test", upstream.requests[0].Header.Get("Authorization"))
	require.NotEmpty(t, upstream.requests[0].Header.Get("X-OpenCode-Session"))
	require.Contains(t, recorder.Body.String(), `"type":"test_complete"`)
	require.NotContains(t, upstream.requests[0].URL.Path, "/messages")
}

func TestAccountTestService_OpenCodeGoResponsesKeyUsesResponses(t *testing.T) {
	account := openCodeGoTestAccount(402, APIProtocolResponses, "https://opencode.ai/zen/go/v1")
	svc, upstream := adaptiveCNAccountTestService(account, adaptiveCNResponsesTestResponse())
	c, recorder := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "grok-4.6", "hi", AccountTestModeDefault)

	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://opencode.ai/zen/go/v1/responses", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer sk-opencode-go-test", upstream.requests[0].Header.Get("Authorization"))
	require.NotEmpty(t, upstream.requests[0].Header.Get("X-OpenCode-Session"))
	require.Contains(t, recorder.Body.String(), `"type":"test_complete"`)
}

// TestAccountTestService_OpenCodeGoAnthropicKeyUsesMessages：anthropic 地址走原生 messages，与真实转发同址
// （{base}/v1/messages?beta=true）。原注释说 OpenCode 不接受 ?beta=true，但那次 404 实为 /v1/v1 拼接错误，
// 没有实测依据；真实转发本来就带，测连与之一致才能如实反映（2026-09-29）。
func TestAccountTestService_OpenCodeGoAnthropicKeyUsesMessages(t *testing.T) {
	account := openCodeGoTestAccount(403, APIProtocolAnthropic, "https://opencode.ai/zen/go")
	svc, upstream := adaptiveCNAccountTestService(account, adaptiveCNAnthropicTestResponse())
	c, recorder := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "minimax-m3", "hi", AccountTestModeDefault)

	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	req := upstream.requests[0]
	require.Equal(t, "https://opencode.ai/zen/go/v1/messages?beta=true", req.URL.String())
	require.Equal(t, "sk-opencode-go-test", req.Header.Get("x-api-key"))
	require.NotEmpty(t, req.Header.Get("X-OpenCode-Session"))
	require.Contains(t, recorder.Body.String(), `"type":"test_complete"`)
}

func TestAccountTestService_OpenCodeGoAppliesModelMappingOnResponses(t *testing.T) {
	account := openCodeGoTestAccount(406, APIProtocolResponses, "https://opencode.ai/zen/go/v1")
	account.Credentials["model_mapping"] = map[string]any{
		"opencode/muse-spark-1.3-contributior-free": "muse-spark-1.3-contributior-free",
	}
	svc, upstream := adaptiveCNAccountTestService(account, adaptiveCNResponsesTestResponse())
	c, recorder := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "opencode/muse-spark-1.3-contributior-free", "hi", AccountTestModeDefault)

	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://opencode.ai/zen/go/v1/responses", upstream.requests[0].URL.String())
	require.Equal(t, "muse-spark-1.3-contributior-free", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Contains(t, recorder.Body.String(), `"model":"muse-spark-1.3-contributior-free"`)
	require.NotContains(t, string(upstream.lastBody), "opencode/muse-spark-1.3-contributior-free")
}

func TestAccountTestService_OpenCodeGoEmptyModelDefaultsToChatCatalogID(t *testing.T) {
	account := openCodeGoTestAccount(405, APIProtocolChatCompletions, "https://opencode.ai/zen/go/v1")
	svc, upstream := adaptiveCNAccountTestService(account, adaptiveCNChatTestResponse())
	c, recorder := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "", "hi", AccountTestModeDefault)

	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://opencode.ai/zen/go/v1/chat/completions", upstream.requests[0].URL.String())
	require.True(t, strings.Contains(recorder.Body.String(), DefaultOpenCodeGoTestModel))
}
