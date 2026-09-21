//go:build unit

package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 3b-4b：/v1/responses 由 Gateway handler 承接全部资源。

func responsesKey(id, groupID int64, endpoint string, priority int, entryID int64) *service.Account {
	key := keyRouteAccount(id, groupID, service.PlatformOpenAI, map[string]string{service.APIProtocolResponses: endpoint}, "gpt-5.6")
	key.Priority = priority
	key.CatalogEntryIDs = []int64{entryID}
	return key
}

func TestGatewayHandlerResponses_ResponsesKeyForwardsViaOpenAIService(t *testing.T) {
	const entryID = 199
	group := keyRouteGroup(2401, service.PlatformAnthropic)
	key := responsesKey(1401, group.ID, "https://relay.example.com", 1, entryID)
	hs := newKeyRouteHarness(t, group, []*service.Account{key})

	body := []byte(`{"model":"gpt-5.6","input":"hello","stream":false}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/responses", body, group, service.APIProtocolResponses, "")
	openAIRouteEntry(c, entryID, "gpt-5.6")

	hs.handler.Responses(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"response"`)
	got := hs.openAIUpstream.recorded()
	require.Len(t, got, 1)
	require.True(t, strings.HasSuffix(got[0].url, "/v1/responses"), got[0].url)
	require.Empty(t, hs.antigravityUpsteam.recorded())
	logs := hs.usageLogs.recorded()
	require.Len(t, logs, 1)
	require.Equal(t, key.ID, logs[0].AccountID)
}

// previous_response_id 命中的绑定账号做成预取粘性：优先级更低的 B 赢过 A。
func TestGatewayHandlerResponses_PreviousResponseIDPrefetchesSticky(t *testing.T) {
	const entryID = 199
	group := keyRouteGroup(2402, service.PlatformAnthropic)
	keyA := responsesKey(1402, group.ID, "https://a.example.com", 1, entryID)
	keyB := responsesKey(1403, group.ID, "https://b.example.com", 5, entryID)
	hs := newKeyRouteHarness(t, group, []*service.Account{keyA, keyB})
	// OpenAI 服务要能按 ID 取到绑定账号（续链解析走它自己的仓储）
	repo := &grokCredentialHandlerRepo{accounts: []service.Account{*keyA, *keyB}, missingOnGet: map[int64]bool{}}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	hs.handler.openAIGatewayService = service.NewOpenAIGatewayService(
		repo, hs.usageLogs, nil, handlerUserRepoStub{}, handlerSubRepoStub{}, nil, cfg, nil, nil,
		service.NewBillingService(cfg, nil), nil, &service.BillingCacheService{}, hs.openAIUpstream,
		&service.DeferredService{}, nil, nil, nil, nil, nil, nil,
	)
	ctx := context.Background()
	require.True(t, hs.handler.openAIGatewayService.BindOpenAIHTTPResponseAccount(ctx, entryID, "resp_prev_1", keyB.ID))
	require.NoError(t, hs.handler.openAIGatewayService.BindOpenAIHTTPResponseOwner(ctx, entryID, "resp_prev_1", 4101, 3101))

	body := []byte(`{"model":"gpt-5.6","input":"hello","previous_response_id":"resp_prev_1","stream":false}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/responses", body, group, service.APIProtocolResponses, "")
	openAIRouteEntry(c, entryID, "gpt-5.6")

	hs.handler.Responses(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := hs.openAIUpstream.recorded()
	require.Len(t, got, 1)
	require.True(t, strings.HasPrefix(got[0].url, "https://b.example.com/"), got[0].url)

	// 不是本用户的续链 → 400
	c2, rec2 := newKeyRouteContext(t, http.MethodPost, "/v1/responses", []byte(`{"model":"gpt-5.6","input":"hello","previous_response_id":"resp_other","stream":false}`), group, service.APIProtocolResponses, "")
	openAIRouteEntry(c2, entryID, "gpt-5.6")
	hs.handler.Responses(c2)
	require.Equal(t, http.StatusBadRequest, rec2.Code, rec2.Body.String())
	require.Contains(t, rec2.Body.String(), "not available for this user")
}

// HTTP 续链只有以 responses 协议直连的 key 承接：OAuth 成品号被跳过，落到 key。
func TestGatewayHandlerResponses_PreviousResponseIDSkipsOAuth(t *testing.T) {
	const entryID = 199
	group := keyRouteGroup(2403, service.PlatformAnthropic)
	oauth := &service.Account{
		ID: 1404, Name: "openai-oauth", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusActive,
		Schedulable: true, Concurrency: 5, Priority: 1, CatalogEntryIDs: []int64{entryID},
		Credentials: map[string]any{"access_token": "tok"},
	}
	key := responsesKey(1405, group.ID, "https://relay.example.com", 5, entryID)
	hs := newKeyRouteHarness(t, group, []*service.Account{oauth, key})
	require.NoError(t, hs.handler.openAIGatewayService.BindOpenAIHTTPResponseOwner(context.Background(), entryID, "resp_prev_2", 4101, 3101))

	body := []byte(`{"model":"gpt-5.6","input":"hello","previous_response_id":"resp_prev_2","stream":false}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/responses", body, group, service.APIProtocolResponses, "")
	openAIRouteEntry(c, entryID, "gpt-5.6")

	hs.handler.Responses(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := hs.openAIUpstream.recorded()
	require.Len(t, got, 1)
	require.True(t, strings.HasPrefix(got[0].url, "https://relay.example.com/"), got[0].url)
}

// /responses/compact 只能派给 compact 档 > 0 的资源；全被拒时 503 compact_not_supported。
func TestGatewayHandlerResponses_CompactRequiresCompactAccount(t *testing.T) {
	const entryID = 199
	group := keyRouteGroup(2404, service.PlatformAnthropic)
	key := responsesKey(1406, group.ID, "https://relay.example.com", 1, entryID)
	key.Extra = map[string]any{"openai_compact_supported": false}
	hs := newKeyRouteHarness(t, group, []*service.Account{key})

	body := []byte(`{"model":"gpt-5.6","input":"hello","stream":false}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/responses/compact", body, group, service.APIProtocolResponses, "")
	openAIRouteEntry(c, entryID, "gpt-5.6")

	hs.handler.Responses(c)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "compact_not_supported")
	require.Empty(t, hs.openAIUpstream.recorded())
}

// 生图意图必须调度到确实提供 Responses 的资源：只有 chat 地址的 key 承接不了。
func TestGatewayHandlerResponses_ImageIntentRequiresResponsesCapability(t *testing.T) {
	const entryID = 199
	group := keyRouteGroup(2405, service.PlatformOpenAI)
	key := keyRouteAccount(1407, group.ID, service.PlatformOpenAI, map[string]string{service.APIProtocolChatCompletions: "https://relay.example.com"}, "gpt-5.6")
	key.CatalogEntryIDs = []int64{entryID}
	group.AllowImageGeneration = true
	hs := newKeyRouteHarness(t, group, []*service.Account{key})

	body := []byte(`{"model":"gpt-5.6","input":"draw a cat","tools":[{"type":"image_generation"}],"stream":false}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/responses", body, group, service.APIProtocolResponses, "")
	openAIRouteEntry(c, entryID, "gpt-5.6")

	hs.handler.Responses(c)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	require.Empty(t, hs.openAIUpstream.recorded())
}

func TestGatewayHandlerResponses_ServiceTierRejected(t *testing.T) {
	const entryID = 199
	group := keyRouteGroup(2406, service.PlatformAnthropic)
	key := responsesKey(1408, group.ID, "https://relay.example.com", 1, entryID)
	hs := newKeyRouteHarness(t, group, []*service.Account{key})

	body := []byte(`{"model":"gpt-5.6","input":"hello","service_tier":123}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/responses", body, group, service.APIProtocolResponses, "")
	openAIRouteEntry(c, entryID, "gpt-5.6")

	hs.handler.Responses(c)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "service_tier")
}

func TestGatewayHandlerResponses_FunctionCallOutputWithoutContextRejected(t *testing.T) {
	const entryID = 199
	group := keyRouteGroup(2407, service.PlatformAnthropic)
	key := responsesKey(1409, group.ID, "https://relay.example.com", 1, entryID)
	hs := newKeyRouteHarness(t, group, []*service.Account{key})

	body := []byte(`{"model":"gpt-5.6","input":[{"type":"function_call_output","call_id":"call_1","output":"42"}],"stream":false}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/responses", body, group, service.APIProtocolResponses, "")
	openAIRouteEntry(c, entryID, "gpt-5.6")

	hs.handler.Responses(c)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "item_reference")
	// HTTP 入口的提示只指向 WS v2，不建议复用 previous_response_id
	require.Contains(t, rec.Body.String(), "Responses WebSocket v2")
	require.NotContains(t, rec.Body.String(), "reuse previous_response_id")
	require.Empty(t, hs.openAIUpstream.recorded())
}
