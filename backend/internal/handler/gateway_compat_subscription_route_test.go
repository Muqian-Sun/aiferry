//go:build unit

package handler

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// /v1/responses 与 /v1/chat/completions 的普通文本请求按协议转换注册表选资源：Antigravity 成品号
// 能从这两种入站协议转换（ForwardAsResponses / ForwardAsChatCompletions），不能再被
// 「chat_completions 端点能力」挡掉（2026-10-08 生产：gemini-3.8-flash 只有 Antigravity 成品号承接，
// /responses 一直 503 No upstream）。
func TestGatewayHandlerCompatEntries_AntigravitySubscriptionServesGeminiModel(t *testing.T) {
	const entryID = 31
	withGeminiRoute := func(c *gin.Context) {
		entry := &service.ModelCatalogEntry{ID: entryID, ModelID: "gemini-3.8-flash", Vendor: "gemini", Status: service.ModelCatalogStatusListed}
		route := service.CatalogRoute{EntryID: entryID, CanonicalModel: "gemini-3.8-flash", RequestedModel: "gemini-3.8-flash", Entry: entry}
		c.Request = c.Request.WithContext(service.WithCatalogRoute(c.Request.Context(), route))
	}
	newHarness := func(t *testing.T) *keyRouteHarness {
		subscription := &service.Account{
			ID:          1501,
			Name:        "ag-oauth",
			Platform:    service.PlatformAntigravity,
			Type:        service.AccountTypeOAuth,
			Credentials: map[string]any{"access_token": "tok", "project_id": "proj-1"},
			// 承接上填的上游名（Antigravity 上没有不带后缀的 gemini-3.8-flash）
			CatalogUpstreamModels: map[string]string{"gemini-3.8-flash": "gemini-3.8-flash-high"},
			Concurrency:           1,
			Priority:              1,
			Status:                service.StatusActive,
			Schedulable:           true,
			CatalogEntryIDs:       []int64{entryID},
		}
		hs := newKeyRouteHarness(t, []*service.Account{subscription})
		// Antigravity 上游恒定流式（streamGenerateContent?alt=sse），正文是 {"response":{…}} 包着的 Gemini 分片
		hs.antigravityUpsteam.contentType = "text/event-stream"
		hs.antigravityUpsteam.respBody = `data: {"response":{"responseId":"resp_1","candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":1}}}` + "\n\n"
		hs.handler.antigravityGatewayService = service.NewAntigravityGatewayService(
			nil, nil, nil, service.NewAntigravityTokenProvider(nil, &fakeAntigravityTokenCache{token: "fresh"}, nil), nil, hs.antigravityUpsteam,
			service.NewSettingService(nil, &config.Config{}), nil)
		return hs
	}

	t.Run("responses", func(t *testing.T) {
		hs := newHarness(t)
		body := []byte(`{"model":"gemini-3.8-flash","input":"hello","stream":false}`)
		c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/responses", body, service.APIProtocolResponses, "")
		withGeminiRoute(c)

		hs.handler.Responses(c)

		require.NotContains(t, rec.Body.String(), "No upstream is currently available", rec.Body.String())
		got := hs.antigravityUpsteam.recorded()
		require.Len(t, got, 1, rec.Body.String())
		require.True(t, strings.Contains(got[0].url, "/v1internal:"), got[0].url)
		require.Contains(t, string(got[0].body), "gemini-3.8-flash-high", "按承接的上游名转发")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})

	t.Run("chat completions", func(t *testing.T) {
		hs := newHarness(t)
		body := []byte(`{"model":"gemini-3.8-flash","messages":[{"role":"user","content":"hello"}],"stream":false}`)
		c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/chat/completions", body, service.APIProtocolChatCompletions, "")
		withGeminiRoute(c)

		hs.handler.ChatCompletions(c)

		require.NotContains(t, rec.Body.String(), "No upstream is currently available", rec.Body.String())
		got := hs.antigravityUpsteam.recorded()
		require.Len(t, got, 1, rec.Body.String())
		require.Contains(t, string(got[0].body), "gemini-3.8-flash-high")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})
}
