//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

const selectOptionsEntryID int64 = 8101

func selectOptionsCtx(inbound string) context.Context {
	return catalogRouteCtx(selectOptionsEntryID, inbound)
}

// openAIOAuthSub 官方 OpenAI 成品号（上游 responses）。
func openAIOAuthSub(id int64, priority int) Account {
	return Account{
		ID: id, Name: "openai-oauth", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive,
		Schedulable: true, Concurrency: 5, Priority: priority, CatalogEntryIDs: []int64{selectOptionsEntryID},
		Credentials: map[string]any{"access_token": "tok"},
	}
}

// openAIKey 配 responses + chat 地址的第三方 key（通用中转）。
func openAIKey(id int64, priority int, extra map[string]any) Account {
	return Account{
		ID: id, Name: "openai-key", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive,
		Schedulable: true, Concurrency: 5, Priority: priority, CatalogEntryIDs: []int64{selectOptionsEntryID},
		Credentials:       map[string]any{"api_key": "k"},
		ProtocolEndpoints: map[string]string{APIProtocolResponses: "https://relay.example.com", APIProtocolChatCompletions: "https://relay.example.com"},
		Extra:             extra,
	}
}

type fixedWSResolver struct{ transport OpenAIUpstreamTransport }

func (r fixedWSResolver) Resolve(*Account) OpenAIWSProtocolDecision {
	return OpenAIWSProtocolDecision{Transport: r.transport, Reason: "test"}
}

func TestSelectAccountWithOptions_CapabilityGate(t *testing.T) {
	oauth := openAIOAuthSub(81001, 1)
	key := openAIKey(81002, 5, nil)
	for _, loadBatch := range []bool{true, false} {
		name := map[bool]string{true: "load aware", false: "legacy"}[loadBatch]
		t.Run(name, func(t *testing.T) {
			svc := newProtocolMatchService(t, loadBatch, nil, oauth, key)
			ctx := selectOptionsCtx(APIProtocolChatCompletions)

			result, err := svc.SelectAccountWithOptions(ctx, "", "gpt-5.6", nil, SelectOptions{})
			require.NoError(t, err)
			require.Equal(t, key.ID, result.Account.ID, "零要求：chat 直连的 key 赢过要转换的成品号")

			result, err = svc.SelectAccountWithOptions(ctx, "", "gpt-5.6", nil, SelectOptions{Capability: OpenAIEndpointCapabilityLive})
			require.NoError(t, err)
			require.Equal(t, oauth.ID, result.Account.ID, "live 只有 ChatGPT OAuth 成品号有")

			result, err = svc.SelectAccountWithOptions(ctx, "", "gpt-5.6", nil, SelectOptions{Capability: OpenAIEndpointCapabilityEmbeddings})
			require.NoError(t, err)
			require.Equal(t, key.ID, result.Account.ID, "embeddings 只有 API key 有")
		})
	}
}

func TestSelectAccountWithOptions_ImageCapability(t *testing.T) {
	anthropicKey := Account{
		ID: 81011, Name: "anthropic-key", Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Status: StatusActive,
		Schedulable: true, Concurrency: 5, Priority: 1, CatalogEntryIDs: []int64{selectOptionsEntryID},
		Credentials:       map[string]any{"api_key": "k"},
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://relay.example.com"},
	}
	key := openAIKey(81012, 5, nil)
	svc := newProtocolMatchService(t, true, nil, anthropicKey, key)
	ctx := selectOptionsCtx(APIProtocolAnthropic)

	result, err := svc.SelectAccountWithOptions(ctx, "", "gpt-5.6", nil, SelectOptions{})
	require.NoError(t, err)
	require.Equal(t, anthropicKey.ID, result.Account.ID, "零要求：message 入站 anthropic 直连 + 优先级更高")

	result, err = svc.SelectAccountWithOptions(ctx, "", "gpt-5.6", nil, SelectOptions{ImageCapability: OpenAIImagesCapabilityNative})
	require.NoError(t, err)
	require.Equal(t, key.ID, result.Account.ID, "/v1/images 只有 OpenAI 协议的资源承接")
}

func TestSelectAccountWithOptions_RequireCompactPrefersKnownSupport(t *testing.T) {
	tier0 := openAIKey(81021, 1, map[string]any{"openai_compact_supported": false})
	tier1 := openAIKey(81022, 1, nil)
	tier2 := openAIKey(81023, 1, map[string]any{"openai_compact_supported": true})
	ctx := selectOptionsCtx(APIProtocolResponses)

	svc := newProtocolMatchService(t, true, nil, tier0, tier1, tier2)
	result, err := svc.SelectAccountWithOptions(ctx, "", "gpt-5.6", nil, SelectOptions{RequireCompact: true})
	require.NoError(t, err)
	require.Equal(t, tier2.ID, result.Account.ID, "同优先级：明确支持 compact 的先于未探测的")

	svc = newProtocolMatchService(t, true, nil, tier0)
	_, err = svc.SelectAccountWithOptions(ctx, "", "gpt-5.6", nil, SelectOptions{RequireCompact: true})
	require.ErrorIs(t, err, ErrNoAvailableCompactAccounts)

	svc = newProtocolMatchService(t, true, nil, tier0)
	result, err = svc.SelectAccountWithOptions(ctx, "", "gpt-5.6", nil, SelectOptions{})
	require.NoError(t, err)
	require.Equal(t, tier0.ID, result.Account.ID, "不要求 compact 时 tier 0 照常可用")
}

// Compact 模式 2026-09-28 P5 写死 auto（A3-13）：库里残留的 openai_compact_mode 不再覆盖探测结果。
// 改之前 force_on 让探测为不支持的账号照样接 compact，force_off 把探测为支持的账号挡在外面。
func TestSelectAccountWithOptions_RequireCompactIgnoresLegacyCompactMode(t *testing.T) {
	ctx := selectOptionsCtx(APIProtocolResponses)
	for _, loadBatch := range []bool{true, false} {
		name := map[bool]string{true: "load aware", false: "legacy"}[loadBatch]
		t.Run(name, func(t *testing.T) {
			forcedOnUnsupported := openAIKey(81031, 1, map[string]any{"openai_compact_mode": "force_on", "openai_compact_supported": false})
			svc := newProtocolMatchService(t, loadBatch, nil, forcedOnUnsupported)
			_, err := svc.SelectAccountWithOptions(ctx, "", "gpt-5.6", nil, SelectOptions{RequireCompact: true})
			require.ErrorIs(t, err, ErrNoAvailableCompactAccounts, "残留 force_on 不得让探测为不支持的账号接 compact")

			forcedOffSupported := openAIKey(81032, 1, map[string]any{"openai_compact_mode": "force_off", "openai_compact_supported": true})
			svc = newProtocolMatchService(t, loadBatch, nil, forcedOffSupported)
			result, err := svc.SelectAccountWithOptions(ctx, "", "gpt-5.6", nil, SelectOptions{RequireCompact: true})
			require.NoError(t, err, "残留 force_off 不得挡住探测为支持的账号")
			require.Equal(t, forcedOffSupported.ID, result.Account.ID)
		})
	}
}

// 渠道级 WS mode 2026-09-28 P5 删了（A3-10）：默认部署（openai_ws 开、mode_router_v2 关，取值同
// config.go setDefaults）下，库里残留「WS 开」键的 OpenAI 成品号 / key 也只走 HTTP，WS 入站选不到它们；
// 改之前这些键会让账号承接 WS v2。HTTP 入站照常。
func TestSelectAccountWithOptions_LegacyWSKeysStayHTTPByDefault(t *testing.T) {
	legacyOAuth := openAIOAuthSub(81041, 1)
	legacyOAuth.Extra = map[string]any{
		"openai_oauth_responses_websockets_v2_enabled": true,
		"openai_oauth_responses_websockets_v2_mode":    OpenAIWSIngressModeCtxPool,
		"responses_websockets_v2_enabled":              true,
		"openai_ws_enabled":                            true,
	}
	legacyKey := openAIKey(81042, 1, map[string]any{
		"openai_apikey_responses_websockets_v2_enabled": true,
		"openai_apikey_responses_websockets_v2_mode":    OpenAIWSIngressModePassthrough,
	})
	ctx := selectOptionsCtx(APIProtocolResponses)
	for _, loadBatch := range []bool{true, false} {
		name := map[bool]string{true: "load aware", false: "legacy"}[loadBatch]
		t.Run(name, func(t *testing.T) {
			svc := newProtocolMatchService(t, loadBatch, nil, legacyOAuth, legacyKey)
			svc.cfg.Gateway.OpenAIWS.Enabled = true
			svc.cfg.Gateway.OpenAIWS.OAuthEnabled = true
			svc.cfg.Gateway.OpenAIWS.APIKeyEnabled = true
			svc.cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
			svc.cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = false

			for _, account := range []Account{legacyOAuth, legacyKey} {
				decision := svc.wsProtocolResolver().Resolve(&account)
				require.Equal(t, OpenAIUpstreamTransportHTTPSSE, decision.Transport, account.Name)
				require.Equal(t, "account_disabled", decision.Reason, account.Name)
			}

			_, err := svc.SelectAccountWithOptions(ctx, "", "gpt-5.6", nil, SelectOptions{Transport: OpenAIUpstreamTransportResponsesWebsocketV2Ingress})
			require.True(t, errors.Is(err, ErrNoAvailableAccounts), "残留 WS 键不得让账号承接 WS 入站：%v", err)

			_, err = svc.SelectAccountWithOptions(ctx, "", "gpt-5.6", nil, SelectOptions{Transport: OpenAIUpstreamTransportHTTPSSE})
			require.NoError(t, err)
		})
	}
}

func TestSelectAccountWithOptions_Transport(t *testing.T) {
	oauth := openAIOAuthSub(81031, 1)
	svc := newProtocolMatchService(t, true, nil, oauth)
	svc.openaiWSResolver = fixedWSResolver{transport: OpenAIUpstreamTransportHTTPSSE}
	ctx := selectOptionsCtx(APIProtocolResponses)

	_, err := svc.SelectAccountWithOptions(ctx, "", "gpt-5.6", nil, SelectOptions{Transport: OpenAIUpstreamTransportResponsesWebsocketV2})
	require.True(t, errors.Is(err, ErrNoAvailableAccounts), "解析出的传输是 HTTP，承接不了 WS v2")

	result, err := svc.SelectAccountWithOptions(ctx, "", "gpt-5.6", nil, SelectOptions{Transport: OpenAIUpstreamTransportAny})
	require.NoError(t, err)
	require.Equal(t, oauth.ID, result.Account.ID)

	svc.openaiWSResolver = fixedWSResolver{transport: OpenAIUpstreamTransportResponsesWebsocketV2}
	result, err = svc.SelectAccountWithOptions(ctx, "", "gpt-5.6", nil, SelectOptions{Transport: OpenAIUpstreamTransportResponsesWebsocketV2})
	require.NoError(t, err)
	require.Equal(t, oauth.ID, result.Account.ID)
}

// 无模型端点：没有目录路由、没有分组，池按 SelectOptions.Platform 装载（成品号按平台过滤；
// 第三方 key 一律不承接厂商原生端点，见 TestSelectAccountWithOptions_PlatformRejectsKeysOfSameLabel）。
func TestSelectAccountWithOptions_PlatformFiltersPool(t *testing.T) {
	grokOAuth := Account{
		ID: 81041, Name: "grok-oauth", Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive,
		Schedulable: true, Concurrency: 5, Priority: 5, Credentials: map[string]any{"access_token": "tok"},
	}
	openAIOAuth := openAIOAuthSub(81042, 1)
	openAIOAuth.CatalogEntryIDs = nil
	svc := newProtocolMatchService(t, true, nil, grokOAuth, openAIOAuth)
	ctx := context.Background()

	result, err := svc.SelectAccountWithOptions(ctx, "", "", nil, SelectOptions{Platform: PlatformGrok})
	require.NoError(t, err)
	require.Equal(t, grokOAuth.ID, result.Account.ID, "端点声明 grok：优先级更高的 openai 成品号不在池里")

	result, err = svc.SelectAccountWithOptions(ctx, "", "", nil, SelectOptions{Platform: PlatformOpenAI})
	require.NoError(t, err)
	require.Equal(t, openAIOAuth.ID, result.Account.ID)
}

// 第三方 key 会进任何有兼容地址的网关平台桶，但厂商原生端点（web_search / tts / live）只能由该厂商的账号承接：
// Platform 要求候选的账号平台相等，别家的 key 再高优先级也不选；池里只有别家 key 时无候选。
func TestSelectAccountWithOptions_PlatformRejectsOtherVendorKeys(t *testing.T) {
	grokOAuth := Account{
		ID: 81051, Name: "grok-oauth", Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive,
		Schedulable: true, Concurrency: 5, Priority: 50, Credentials: map[string]any{"access_token": "tok"},
	}
	openAIKey := Account{
		ID: 81052, Name: "openai-key", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive,
		Schedulable: true, Concurrency: 5, Priority: 1, Credentials: map[string]any{"api_key": "sk"},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com/v1", APIProtocolResponses: "https://api.openai.com/v1"},
	}
	ctx := context.Background()

	svc := newProtocolMatchService(t, true, nil, grokOAuth, openAIKey)
	result, err := svc.SelectAccountWithOptions(ctx, "", "", nil, SelectOptions{Platform: PlatformGrok})
	require.NoError(t, err)
	require.Equal(t, grokOAuth.ID, result.Account.ID, "优先级 1 的 openai key 有兼容地址也不能承接 grok 原生端点")

	onlyKey := newProtocolMatchService(t, true, nil, openAIKey)
	_, err = onlyKey.SelectAccountWithOptions(ctx, "", "", nil, SelectOptions{Platform: PlatformGrok})
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
}

// 厂商原生端点（xAI 搜索 / 语音、OpenAI live）只由该厂商的成品号承接：贴着同一平台标签、地址还是厂商官方
// 域名的第三方 key 也按中转，不进候选（2026-09-29 海外四家不再有官方 key）。
func TestSelectAccountWithOptions_PlatformRejectsKeysOfSameLabel(t *testing.T) {
	grokOAuth := Account{
		ID: 81061, Name: "grok-oauth", Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive,
		Schedulable: true, Concurrency: 5, Priority: 50, Credentials: map[string]any{"access_token": "tok"},
	}
	grokKey := Account{
		ID: 81062, Name: "grok-key", Platform: PlatformGrok, Type: AccountTypeAPIKey, Status: StatusActive,
		Schedulable: true, Concurrency: 5, Priority: 1, Credentials: map[string]any{"api_key": "xai"},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.x.ai/v1"},
	}
	ctx := context.Background()

	svc := newProtocolMatchService(t, true, nil, grokOAuth, grokKey)
	result, err := svc.SelectAccountWithOptions(ctx, "", "", nil, SelectOptions{Platform: PlatformGrok})
	require.NoError(t, err)
	require.Equal(t, grokOAuth.ID, result.Account.ID, "优先级 1 的 grok 标签 key（地址是 api.x.ai）也不能承接 grok 原生端点")

	onlyKey := newProtocolMatchService(t, true, nil, grokKey)
	_, err = onlyKey.SelectAccountWithOptions(ctx, "", "", nil, SelectOptions{Platform: PlatformGrok})
	require.ErrorIs(t, err, ErrNoAvailableAccounts)

	ok, reason := SelectOptions{Platform: PlatformGrok}.admits(nil, nil, &grokKey)
	require.False(t, ok)
	require.Equal(t, "platform_mismatch", reason)
	ok, _ = SelectOptions{Platform: PlatformGrok}.admits(nil, nil, &grokOAuth)
	require.True(t, ok)
}

// NoSlot（计 token）：不抢槽、不绑粘性、不等待；粘性命中且在候选里就用它，否则优先级 + LRU 首个。
func TestSelectAccountWithOptions_NoSlotDoesNotAcquire(t *testing.T) {
	high := openAIKey(81021, 1, nil)
	low := openAIKey(81022, 5, nil)
	for _, loadBatch := range []bool{true, false} {
		name := map[bool]string{true: "load aware", false: "legacy order"}[loadBatch]
		t.Run(name, func(t *testing.T) {
			concurrency := &mockConcurrencyCache{}
			cache := &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{"sticky-count": low.ID}}
			svc := newProtocolMatchService(t, loadBatch, cache, high, low)
			svc.concurrencyService = NewConcurrencyService(concurrency)
			ctx := selectOptionsCtx(APIProtocolResponses)
			opts := SelectOptions{Capability: OpenAIEndpointCapabilityResponses, NoSlot: true}

			result, err := svc.SelectAccountWithOptions(ctx, "", "gpt-5.6", nil, opts)
			require.NoError(t, err)
			require.Equal(t, high.ID, result.Account.ID, "无粘性：优先级最小的候选")
			require.False(t, result.Acquired)
			require.Nil(t, result.WaitPlan)
			require.Nil(t, result.ReleaseFunc)

			result, err = svc.SelectAccountWithOptions(ctx, "sticky-count", "gpt-5.6", nil, opts)
			require.NoError(t, err)
			require.Equal(t, low.ID, result.Account.ID, "粘性命中且在候选里：用它")
			require.False(t, result.Acquired)

			result, err = svc.SelectAccountWithOptions(ctx, "fresh-count", "gpt-5.6", nil, opts)
			require.NoError(t, err)
			require.Equal(t, high.ID, result.Account.ID)
			require.Zero(t, concurrency.acquireAccountCalls, "NoSlot 不得抢槽")
			require.NotContains(t, cache.sessionBindings, "fresh-count", "NoSlot 不得写粘性绑定")
		})
	}
}
