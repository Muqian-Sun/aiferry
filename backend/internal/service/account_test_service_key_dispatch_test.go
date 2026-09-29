//go:build unit

package service

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// 第三方 key 的连接测试按协议地址与 Vendor 分派，平台标签不参与。每个用例都刻意
// 让标签与地址不一致：任何一处回到按标签分派，探针就会打到另一个地址或另一种
// 线格式而变红。

func keyDispatchAccount(id int64, platform string, endpoints map[string]string) *Account {
	return &Account{
		ID:                id,
		Name:              "key-dispatch-test",
		Platform:          platform,
		Type:              AccountTypeAPIKey,
		Status:            StatusActive,
		Concurrency:       1,
		Credentials:       map[string]any{"api_key": "sk-dispatch"},
		ProtocolEndpoints: endpoints,
	}
}

// 标签 anthropic、只配 chat_completions 地址：打 {cc}/v1/chat/completions，
// 不再落到通用 Claude 探针的 /v1/messages。
func TestAccountTestService_KeyAnthropicLabelChatEndpointProbesChatCompletions(t *testing.T) {
	account := keyDispatchAccount(401, PlatformAnthropic, map[string]string{
		APIProtocolChatCompletions: "http://chat.example/v1",
	})
	svc, upstream := adaptiveCNAccountTestService(account, adaptiveCNChatTestResponse())
	c, recorder := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "", "hi", AccountTestModeDefault)

	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "http://chat.example/v1/chat/completions", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer sk-dispatch", upstream.requests[0].Header.Get("Authorization"))
	require.Contains(t, recorder.Body.String(), `"type":"test_complete"`)
}

// 标签 openai、只配 anthropic 地址：打 {anthropic}/v1/messages?beta=true（与真实转发同址），用 x-api-key。
func TestAccountTestService_KeyOpenAILabelAnthropicEndpointProbesMessages(t *testing.T) {
	account := keyDispatchAccount(402, PlatformOpenAI, map[string]string{
		APIProtocolAnthropic: "http://anthropic.example",
	})
	svc, upstream := adaptiveCNAccountTestService(account, adaptiveCNAnthropicTestResponse())
	c, recorder := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "", "", AccountTestModeDefault)

	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "http://anthropic.example/v1/messages?beta=true", upstream.requests[0].URL.String())
	require.Equal(t, "sk-dispatch", upstream.requests[0].Header.Get("x-api-key"))
	require.Contains(t, recorder.Body.String(), `"type":"test_complete"`)
}

// 指向 api.anthropic.com 的 key 也按中转测，与真实转发同址同形：带 ?beta=true（真实转发对所有 key 都拼），
// 不主动加 anthropic-beta（真实转发只透传客户端带来的）。
func TestAccountTestService_KeyOnOfficialAnthropicHostProbesLikeRelay(t *testing.T) {
	account := keyDispatchAccount(403, PlatformKimi, map[string]string{
		APIProtocolAnthropic: "https://api.anthropic.com",
	})
	require.Empty(t, account.Vendor(), "海外四家不再有官方 key：指向 api.anthropic.com 的 key 按中转")
	svc, upstream := adaptiveCNAccountTestService(account, adaptiveCNAnthropicTestResponse())
	c, _ := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "", "", AccountTestModeDefault)

	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://api.anthropic.com/v1/messages?beta=true", upstream.requests[0].URL.String(), "与真实转发同址")
	require.Empty(t, upstream.requests[0].Header.Get("anthropic-beta"))
}

// 地址带 /v1 的 anthropic 端点与真实转发一样拼成 {base}/messages，不再拼出
// /v1/v1/messages，也不被误判成 OpenAI 兼容端点。
func TestAccountTestService_KeyAnthropicEndpointWithVersionSuffixJoinsOnce(t *testing.T) {
	account := keyDispatchAccount(404, PlatformOpenAI, map[string]string{
		APIProtocolAnthropic: "http://anthropic.example/v1",
	})
	svc, upstream := adaptiveCNAccountTestService(account, adaptiveCNAnthropicTestResponse())
	c, _ := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "", "", AccountTestModeDefault)

	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "http://anthropic.example/v1/messages?beta=true", upstream.requests[0].URL.String())
}

// 只配 responses 地址的 key 打 {responses}/v1/responses。
func TestAccountTestService_KeyResponsesOnlyEndpointProbesResponses(t *testing.T) {
	account := keyDispatchAccount(405, PlatformAnthropic, map[string]string{
		APIProtocolResponses: "http://responses.example",
	})
	svc, upstream := adaptiveCNAccountTestService(account, adaptiveCNResponsesTestResponse())
	c, _ := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "", "", AccountTestModeDefault)

	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "http://responses.example/v1/responses", upstream.requests[0].URL.String())
}

// 只配 gemini 地址的 key 打 Gemini 原生端点，标签写的是 anthropic 也一样。
func TestAccountTestService_KeyGeminiEndpointProbesGeminiEndpoint(t *testing.T) {
	account := keyDispatchAccount(406, PlatformAnthropic, map[string]string{
		APIProtocolGemini: "http://gemini.example",
	})
	svc, upstream := adaptiveCNAccountTestService(account, newJSONResponse(http.StatusUnauthorized, `{"error":"nope"}`))
	c, _ := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "", "", AccountTestModeDefault)

	require.Error(t, err)
	require.Len(t, upstream.requests, 1)
	require.True(t, strings.HasPrefix(upstream.requests[0].URL.String(), "http://gemini.example/v1beta/models/"),
		"unexpected URL: %s", upstream.requests[0].URL.String())
	require.Equal(t, "sk-dispatch", upstream.requests[0].Header.Get("x-goog-api-key"))
}

// 同时配了 anthropic 与 gemini 地址时，管理员选的 gemini-* 模型决定测 Gemini 端点。
func TestAccountTestService_KeyGeminiModelPrefersGeminiEndpoint(t *testing.T) {
	account := keyDispatchAccount(407, PlatformAnthropic, map[string]string{
		APIProtocolAnthropic: "http://anthropic.example",
		APIProtocolGemini:    "http://gemini.example",
	})
	svc, upstream := adaptiveCNAccountTestService(account, newJSONResponse(http.StatusUnauthorized, `{"error":"nope"}`))
	c, _ := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "gemini-2.5-pro", "", AccountTestModeDefault)

	require.Error(t, err)
	require.Len(t, upstream.requests, 1)
	require.Contains(t, upstream.requests[0].URL.String(), "http://gemini.example/v1beta/models/gemini-2.5-pro")
}

// 贴 grok 标签但地址是中转的 key 走通用协议探针，不走 xAI 的 Responses 端点。
func TestAccountTestService_KeyGrokLabelRelayUsesProtocolProbe(t *testing.T) {
	account := keyDispatchAccount(408, PlatformGrok, map[string]string{
		APIProtocolChatCompletions: "http://chat.example/v1",
	})
	require.Empty(t, account.Vendor())
	svc, upstream := adaptiveCNAccountTestService(account, adaptiveCNChatTestResponse())
	c, _ := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "", "", AccountTestModeDefault)

	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "http://chat.example/v1/chat/completions", upstream.requests[0].URL.String())
}

// 地址是 xAI 官方站的 key 也按中转：走通用协议探针，不走 Grok 专用测连（2026-09-29 海外四家不再有官方 key）。
func TestAccountTestService_KeyOnOfficialXAIHostUsesProtocolProbe(t *testing.T) {
	account := keyDispatchAccount(409, PlatformGrok, map[string]string{
		APIProtocolChatCompletions: "https://api.x.ai/v1",
	})
	require.Empty(t, account.Vendor())
	svc, upstream := adaptiveCNAccountTestService(account, adaptiveCNChatTestResponse())
	c, _ := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "grok-4", "", AccountTestModeDefault)

	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://api.x.ai/v1/chat/completions", upstream.requests[0].URL.String())
}

// 一个协议地址都没配：报 MISSING_PROTOCOL_ENDPOINT，一次上游请求都不发。
func TestAccountTestService_KeyWithoutProtocolEndpointFailsClosed(t *testing.T) {
	account := keyDispatchAccount(410, PlatformAnthropic, nil)
	svc, upstream := adaptiveCNAccountTestService(account, adaptiveCNChatTestResponse())
	c, recorder := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "", "", AccountTestModeDefault)

	require.Error(t, err)
	require.Empty(t, upstream.requests)
	require.Contains(t, recorder.Body.String(), "has no anthropic / chat_completions / responses / gemini upstream address configured")
}
