//go:build unit

package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 中转 key 不分协议用同一套「渠道 × 模型」连续失败规则（2026-09-29 muqian 定，与 OpenAI 协议的中转相同）：
// 上游 5xx 同一模型连续第 2 次后避开一段时间，调度不再选它；成功一次清零；529 不计。这里覆盖 Messages 与
// Gemini 协议的中转，走真实 handler → 调度 → 转发；同一个 handler 连发多次请求，避让状态留在它的 RateLimitService 里。

const anthropicMessagesSSEOK = "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"model\":\"claude-sonnet-4-5\",\"stop_reason\":null,\"stop_sequence\":null,\"usage\":{\"input_tokens\":5,\"output_tokens\":1}}}\n\n" +
	"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
	"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"ok\"}}\n\n" +
	"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
	"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\",\"stop_sequence\":null},\"usage\":{\"output_tokens\":2}}\n\n" +
	"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"

const (
	relayGeminiEntryID = 9
	relayGeminiModel   = "gemini-2.5-flash"
	gemini503Body      = `{"error":{"code":503,"message":"The model is overloaded. Please try again later.","status":"UNAVAILABLE"}}`
)

// messagesRelayPair 两个 Messages 协议的中转 key：bad 优先级高（先被选），good 兜底。
func messagesRelayPair(badID, goodID int64) []*service.Account {
	endpoints := map[string]string{service.APIProtocolAnthropic: "https://anthropic-relay.example.com"}
	return []*service.Account{
		failoverE2EKey(badID, 1, endpoints, "claude-sonnet-4-5"),
		failoverE2EKey(goodID, 2, endpoints, "claude-sonnet-4-5"),
	}
}

// geminiRelayPair 两个 Gemini 协议的中转 key，挂在同一个目录条目下。
func geminiRelayPair(badID, goodID int64) []*service.Account {
	endpoints := map[string]string{service.APIProtocolGemini: "https://gemini-relay.example.com"}
	bad := failoverE2EKey(badID, 1, endpoints, relayGeminiModel)
	good := failoverE2EKey(goodID, 2, endpoints, relayGeminiModel)
	bad.CatalogEntryIDs = []int64{relayGeminiEntryID}
	good.CatalogEntryIDs = []int64{relayGeminiEntryID}
	return []*service.Account{bad, good}
}

func withRelayGeminiRoute(c *gin.Context) {
	entry := &service.ModelCatalogEntry{ID: relayGeminiEntryID, ModelID: relayGeminiModel, Vendor: "gemini", Status: service.ModelCatalogStatusListed}
	c.Request = c.Request.WithContext(service.WithCatalogRoute(c.Request.Context(),
		service.CatalogRoute{EntryID: relayGeminiEntryID, CanonicalModel: relayGeminiModel, RequestedModel: relayGeminiModel, Entry: entry}))
}

// sendRelayRequest 从指定入口发一次请求（model 为请求模型；gemini 为 true 时挂 Gemini 目录路由），
// 返回本次请求依次打到的账号。每次内容不同，免得命中粘性会话。
func sendRelayRequest(t *testing.T, h *GatewayHandler, upstream *failoverStatusUpstream, entry, model string, gemini bool, n int) []int64 {
	t.Helper()
	before := len(upstream.calls())
	var c *gin.Context
	var rec *httptest.ResponseRecorder
	var run func()
	switch entry {
	case "/v1/messages":
		body := []byte(fmt.Sprintf(`{"model":%q,"max_tokens":16,"messages":[{"role":"user","content":"hello %d"}]}`, model, n))
		c, rec = newKeyRouteContext(t, http.MethodPost, entry, body, service.APIProtocolAnthropic, "")
		run = func() { h.Messages(c) }
	case "/v1/chat/completions":
		body := []byte(fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hello %d"}]}`, model, n))
		c, rec = newKeyRouteContext(t, http.MethodPost, entry, body, service.APIProtocolChatCompletions, "")
		run = func() { h.ChatCompletions(c) }
	case "/v1/responses":
		body := []byte(fmt.Sprintf(`{"model":%q,"input":"hello %d"}`, model, n))
		c, rec = newKeyRouteContext(t, http.MethodPost, entry, body, service.APIProtocolResponses, "")
		run = func() { h.Responses(c) }
	case "/v1beta":
		body := []byte(fmt.Sprintf(`{"contents":[{"role":"user","parts":[{"text":"hello %d"}]}]}`, n))
		c, rec = newKeyRouteContext(t, http.MethodPost, "/v1beta/models/"+model+":generateContent", body, service.APIProtocolGemini, "")
		c.Params = gin.Params{{Key: "modelAction", Value: "/" + model + ":generateContent"}}
		run = func() { h.GeminiV1BetaModels(c) }
	default:
		t.Fatalf("unknown entry %s", entry)
	}
	if gemini {
		withRelayGeminiRoute(c)
	}
	run()
	require.Equal(t, http.StatusOK, rec.Code, "%s 第 %d 次请求应成功：%s", entry, n, rec.Body.String())
	return upstream.calls()[before:]
}

// Messages 协议的中转：同一模型连续第 2 次 5xx 后避开，第 3 次请求不再打坏渠道；5xx 的范围与 OpenAI 协议的中转相同。
func TestRelayKeyTransient_MessagesProtocol_Consecutive5xxAvoided(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			upstream := &failoverStatusUpstream{fail: map[int64]bool{71: true}, failStatus: status, failBody: anthropicOverloaded, okBody: anthropicMessagesOK}
			h := newFailoverE2EHandler(t, messagesRelayPair(71, 72), upstream, 3)

			require.Equal(t, []int64{71, 72}, sendRelayRequest(t, h, upstream, "/v1/messages", "claude-sonnet-4-5", false, 1))
			require.Equal(t, []int64{71, 72}, sendRelayRequest(t, h, upstream, "/v1/messages", "claude-sonnet-4-5", false, 2))
			require.Equal(t, []int64{72}, sendRelayRequest(t, h, upstream, "/v1/messages", "claude-sonnet-4-5", false, 3), "连续 2 次 %d 后应避开坏渠道", status)
		})
	}
}

// 529 不计入连续失败：中转的 529 只换号，不避让（过载冷却只对 Claude 成品号）。
func TestRelayKeyTransient_529NotCounted(t *testing.T) {
	upstream := &failoverStatusUpstream{fail: map[int64]bool{73: true}, failStatus: 529, failBody: anthropicOverloaded, okBody: anthropicMessagesOK}
	h := newFailoverE2EHandler(t, messagesRelayPair(73, 74), upstream, 3)

	for n := 1; n <= 3; n++ {
		require.Equal(t, []int64{73, 74}, sendRelayRequest(t, h, upstream, "/v1/messages", "claude-sonnet-4-5", false, n), "第 %d 次：529 渠道每次都应再被选到", n)
	}
}

// 成功一次清零：失败 → 成功 → 失败，只算连续 1 次，不避让；再失败一次才避开。
func TestRelayKeyTransient_SuccessResetsStreak(t *testing.T) {
	upstream := &failoverStatusUpstream{fail: map[int64]bool{75: true}, failStatus: http.StatusServiceUnavailable, failBody: anthropicOverloaded, okBody: anthropicMessagesOK}
	h := newFailoverE2EHandler(t, messagesRelayPair(75, 76), upstream, 3)

	require.Equal(t, []int64{75, 76}, sendRelayRequest(t, h, upstream, "/v1/messages", "claude-sonnet-4-5", false, 1))
	upstream.fail[75] = false
	require.Equal(t, []int64{75}, sendRelayRequest(t, h, upstream, "/v1/messages", "claude-sonnet-4-5", false, 2))
	upstream.fail[75] = true
	require.Equal(t, []int64{75, 76}, sendRelayRequest(t, h, upstream, "/v1/messages", "claude-sonnet-4-5", false, 3))
	require.Equal(t, []int64{75, 76}, sendRelayRequest(t, h, upstream, "/v1/messages", "claude-sonnet-4-5", false, 4), "中间成功过一次，连续失败应从 1 重新数")
	require.Equal(t, []int64{76}, sendRelayRequest(t, h, upstream, "/v1/messages", "claude-sonnet-4-5", false, 5))
}

// chat / responses 入口转发到 Messages 协议的中转，同样记连续失败。
func TestRelayKeyTransient_MessagesProtocolViaChatAndResponses(t *testing.T) {
	for _, entry := range []string{"/v1/chat/completions", "/v1/responses"} {
		t.Run(entry, func(t *testing.T) {
			upstream := &failoverStatusUpstream{fail: map[int64]bool{77: true}, failStatus: http.StatusServiceUnavailable, failBody: anthropicOverloaded,
				okBody: anthropicMessagesSSEOK, okContentType: "text/event-stream"}
			h := newFailoverE2EHandler(t, messagesRelayPair(77, 78), upstream, 3)

			require.Equal(t, []int64{77, 78}, sendRelayRequest(t, h, upstream, entry, "claude-sonnet-4-5", false, 1))
			require.Equal(t, []int64{77, 78}, sendRelayRequest(t, h, upstream, entry, "claude-sonnet-4-5", false, 2))
			require.Equal(t, []int64{78}, sendRelayRequest(t, h, upstream, entry, "claude-sonnet-4-5", false, 3), "连续 2 次 503 后应避开坏渠道")
		})
	}
}

// Gemini 协议的中转：Claude 兼容、chat 兼容与 /v1beta 原生三条路径都记连续失败。
func TestRelayKeyTransient_GeminiProtocol(t *testing.T) {
	for _, entry := range []string{"/v1/messages", "/v1/chat/completions", "/v1beta"} {
		t.Run(entry, func(t *testing.T) {
			upstream := &failoverStatusUpstream{fail: map[int64]bool{81: true}, failStatus: http.StatusServiceUnavailable, failBody: gemini503Body, okBody: geminiGenerateContentOK}
			h := newFailoverE2EHandler(t, geminiRelayPair(81, 82), upstream, 3)

			require.Equal(t, []int64{81, 82}, sendRelayRequest(t, h, upstream, entry, relayGeminiModel, true, 1))
			require.Equal(t, []int64{81, 82}, sendRelayRequest(t, h, upstream, entry, relayGeminiModel, true, 2))
			require.Equal(t, []int64{82}, sendRelayRequest(t, h, upstream, entry, relayGeminiModel, true, 3), "连续 2 次 503 后应避开坏渠道")
		})
	}
}
