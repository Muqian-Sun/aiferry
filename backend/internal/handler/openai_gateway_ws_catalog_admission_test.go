package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
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

// 首帧模型上架、条目厂商是 anthropic：准入只看上架，不看厂商（谁能承接由调度按协议定）。
func TestResponsesWebSocket_ListedAnthropicVendorRouteProceeds(t *testing.T) {
	got := runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
		firstPayload: `{"type":"response.create","model":"claude-sonnet-4","stream":false}`,
		catalog:      listedCatalogStub{ids: []string{"claude-sonnet-4"}},
	})
	if len(got.clientEvents) != 1 {
		t.Fatalf("expected one completed event, got %d", len(got.clientEvents))
	}
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

// wsPlanCovering 只含给定条目的套餐订阅（listedCatalogStub 的条目 ID = 下标 + 1）。
func wsPlanCovering(entryIDs ...int64) *service.UserSubscription {
	models := make([]service.SubscriptionPlanModel, 0, len(entryIDs))
	for _, id := range entryIDs {
		models = append(models, service.SubscriptionPlanModel{EntryID: id})
	}
	return &service.UserSubscription{ID: 9, UserID: 1701, PlanID: 3, Plan: &service.SubscriptionPlan{ID: 3, Models: models}}
}

// 订阅 key 首帧模型上架但不在套餐模型集：1008 关闭，原因是套餐（不是目录、不是分组白名单）。
func TestResponsesWebSocket_RejectsFirstFrameModelNotInPlan(t *testing.T) {
	runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
		firstPayload:            `{"type":"response.create","model":"gpt-5.4","stream":false}`,
		catalog:                 listedCatalogStub{ids: []string{"gpt-5.4", "gpt-4.1"}},
		subscription:            wsPlanCovering(2), // 只含 gpt-4.1
		firstFrameCloseExpected: true,
		closeReasonContains:     `Model "gpt-5.4" is not included in your subscription plan`,
	})
}

// 首帧在套餐内，第二个 turn 换到套餐外的上架条目：关闭，原因是套餐。
func TestResponsesWebSocket_RejectsTurnSwitchToModelNotInPlan(t *testing.T) {
	runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
		firstPayload:            `{"type":"response.create","model":"gpt-5.4","stream":false}`,
		secondPayload:           `{"type":"response.create","model":"gpt-4.1","stream":false}`,
		catalog:                 listedCatalogStub{ids: []string{"gpt-5.4", "gpt-4.1"}},
		subscription:            wsPlanCovering(1), // 只含 gpt-5.4
		secondTurnCloseExpected: true,
		closeReasonContains:     `Model "gpt-4.1" is not included in your subscription plan`,
	})
}

// 订阅 key 首帧在套餐内：正常建立连接。
func TestResponsesWebSocket_AllowsCoveredModelForSubscriptionKey(t *testing.T) {
	got := runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
		firstPayload: `{"type":"response.create","model":"gpt-5.4","stream":false}`,
		catalog:      listedCatalogStub{ids: []string{"gpt-5.4"}},
		subscription: wsPlanCovering(1),
	})
	if len(got.clientEvents) != 1 {
		t.Fatalf("expected one completed event, got %d", len(got.clientEvents))
	}
}
