package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Responses WS 没有经过 HTTP 准入中间件，首帧与后续 turn 在 handler 里做目录准入。

// 首帧模型未上架：连接被 1008 关闭，原因是目录（不是分组白名单）。
func TestResponsesWebSocket_RejectsUnlistedFirstFrame(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModePassthrough, service.OpenAIWSIngressModeDedicated} {
		t.Run(mode, func(t *testing.T) {
			runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
				firstPayload:            `{"type":"response.create","model":"gpt-4.1","stream":false}`,
				catalog:                 listedCatalogStub{ids: []string{"gpt-5.4"}},
				ingressMode:             mode,
				firstFrameCloseExpected: true,
				closeReasonContains:     `Model "gpt-4.1" is not available`,
			})
		})
	}
}

// 首帧模型上架但不是 OpenAI 族（claude 条目走 Anthropic 族）：Responses WS 承接不了，关闭。
func TestResponsesWebSocket_RejectsNonOpenAIFamilyRoute(t *testing.T) {
	runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
		firstPayload:            `{"type":"response.create","model":"claude-sonnet-4","stream":false}`,
		catalog:                 listedCatalogStub{ids: []string{"claude-sonnet-4"}},
		firstFrameCloseExpected: true,
		closeReasonContains:     `Model "claude-sonnet-4" is not available`,
	})
}

// 首帧上架且是 OpenAI 族：正常建立连接。
func TestResponsesWebSocket_ListedOpenAIFamilyProceeds(t *testing.T) {
	got := runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
		firstPayload: `{"type":"response.create","model":"gpt-5.4","stream":false}`,
		catalog:      listedCatalogStub{ids: []string{"gpt-5.4"}},
	})
	if len(got.clientEvents) != 1 {
		t.Fatalf("expected one completed event, got %d", len(got.clientEvents))
	}
}

// 后续 turn 换到别的上架条目：连接绑死的账号没绑那个条目 → 要求重连；未上架 → 目录拒绝。
func TestResponsesWebSocket_SubsequentTurnSwitchRequiresBinding(t *testing.T) {
	t.Run("switch to an entry the account is not bound to", func(t *testing.T) {
		runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
			firstPayload:            `{"type":"response.create","model":"gpt-5.4","stream":false}`,
			secondPayload:           `{"type":"response.create","model":"gpt-4.1","stream":false}`,
			catalog:                 listedCatalogStub{ids: []string{"gpt-5.4", "gpt-4.1"}},
			secondTurnCloseExpected: true,
			closeReasonContains:     "model switch requires reconnect",
		})
	})
	t.Run("switch to an unlisted model", func(t *testing.T) {
		runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
			firstPayload:            `{"type":"response.create","model":"gpt-5.4","stream":false}`,
			secondPayload:           `{"type":"response.create","model":"gpt-4.1","stream":false}`,
			catalog:                 listedCatalogStub{ids: []string{"gpt-5.4"}},
			secondTurnCloseExpected: true,
			closeReasonContains:     `Model "gpt-4.1" is not available`,
		})
	})
}

// 目录路由下 /v1/messages 走 OpenAI 网关不受分组 allow_messages_dispatch 开关与 dispatch 映射约束。
func TestAllowOpenAICompatibleMessagesDispatch_CatalogRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apiKey := &service.APIKey{Group: &service.Group{ID: 9, Platform: service.PlatformOpenAI, AllowMessagesDispatch: false,
		MessagesDispatchModelConfig: service.OpenAIMessagesDispatchModelConfig{ExactModelMappings: map[string]string{"gpt-5.6": "gpt-5.4"}}}}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	require.False(t, allowOpenAICompatibleMessagesDispatch(c, apiKey), "group policy still applies without a route")
	require.Equal(t, "gpt-5.4", resolveOpenAIMessagesDispatchMappedModel(c, apiKey, "gpt-5.6"))

	routed := service.WithCatalogRoute(c.Request.Context(), service.CatalogRoute{EntryID: 1, Platform: service.PlatformOpenAI})
	c.Request = c.Request.WithContext(routed)
	require.True(t, allowOpenAICompatibleMessagesDispatch(c, apiKey))
	require.Equal(t, "", resolveOpenAIMessagesDispatchMappedModel(c, apiKey, "gpt-5.6"), "no group-level dispatch rewrite under a catalog route")
}
