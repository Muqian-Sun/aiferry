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

			result, err := svc.SelectAccountWithOptions(ctx, nil, "", "gpt-5.6", nil, SelectOptions{})
			require.NoError(t, err)
			require.Equal(t, key.ID, result.Account.ID, "零要求：chat 直连的 key 赢过要转换的成品号")

			result, err = svc.SelectAccountWithOptions(ctx, nil, "", "gpt-5.6", nil, SelectOptions{Capability: OpenAIEndpointCapabilityLive})
			require.NoError(t, err)
			require.Equal(t, oauth.ID, result.Account.ID, "live 只有 ChatGPT OAuth 成品号有")

			result, err = svc.SelectAccountWithOptions(ctx, nil, "", "gpt-5.6", nil, SelectOptions{Capability: OpenAIEndpointCapabilityEmbeddings})
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

	result, err := svc.SelectAccountWithOptions(ctx, nil, "", "gpt-5.6", nil, SelectOptions{})
	require.NoError(t, err)
	require.Equal(t, anthropicKey.ID, result.Account.ID, "零要求：message 入站 anthropic 直连 + 优先级更高")

	result, err = svc.SelectAccountWithOptions(ctx, nil, "", "gpt-5.6", nil, SelectOptions{ImageCapability: OpenAIImagesCapabilityNative})
	require.NoError(t, err)
	require.Equal(t, key.ID, result.Account.ID, "/v1/images 只有 OpenAI 协议的资源承接")
}

func TestSelectAccountWithOptions_RequireCompactPrefersKnownSupport(t *testing.T) {
	tier0 := openAIKey(81021, 1, map[string]any{"openai_compact_supported": false})
	tier1 := openAIKey(81022, 1, nil)
	tier2 := openAIKey(81023, 1, map[string]any{"openai_compact_supported": true})
	ctx := selectOptionsCtx(APIProtocolResponses)

	svc := newProtocolMatchService(t, true, nil, tier0, tier1, tier2)
	result, err := svc.SelectAccountWithOptions(ctx, nil, "", "gpt-5.6", nil, SelectOptions{RequireCompact: true})
	require.NoError(t, err)
	require.Equal(t, tier2.ID, result.Account.ID, "同优先级：明确支持 compact 的先于未探测的")

	svc = newProtocolMatchService(t, true, nil, tier0)
	_, err = svc.SelectAccountWithOptions(ctx, nil, "", "gpt-5.6", nil, SelectOptions{RequireCompact: true})
	require.ErrorIs(t, err, ErrNoAvailableCompactAccounts)

	svc = newProtocolMatchService(t, true, nil, tier0)
	result, err = svc.SelectAccountWithOptions(ctx, nil, "", "gpt-5.6", nil, SelectOptions{})
	require.NoError(t, err)
	require.Equal(t, tier0.ID, result.Account.ID, "不要求 compact 时 tier 0 照常可用")
}

func TestSelectAccountWithOptions_Transport(t *testing.T) {
	oauth := openAIOAuthSub(81031, 1)
	svc := newProtocolMatchService(t, true, nil, oauth)
	svc.openaiWSResolver = fixedWSResolver{transport: OpenAIUpstreamTransportHTTPSSE}
	ctx := selectOptionsCtx(APIProtocolResponses)

	_, err := svc.SelectAccountWithOptions(ctx, nil, "", "gpt-5.6", nil, SelectOptions{Transport: OpenAIUpstreamTransportResponsesWebsocketV2})
	require.True(t, errors.Is(err, ErrNoAvailableAccounts), "解析出的传输是 HTTP，承接不了 WS v2")

	result, err := svc.SelectAccountWithOptions(ctx, nil, "", "gpt-5.6", nil, SelectOptions{Transport: OpenAIUpstreamTransportAny})
	require.NoError(t, err)
	require.Equal(t, oauth.ID, result.Account.ID)

	svc.openaiWSResolver = fixedWSResolver{transport: OpenAIUpstreamTransportResponsesWebsocketV2}
	result, err = svc.SelectAccountWithOptions(ctx, nil, "", "gpt-5.6", nil, SelectOptions{Transport: OpenAIUpstreamTransportResponsesWebsocketV2})
	require.NoError(t, err)
	require.Equal(t, oauth.ID, result.Account.ID)
}

// 无模型端点：没有目录路由、没有分组，池按 SelectOptions.Platform 装载（成品号按平台过滤；
// 第三方 key 任意平台标签都进池，由能力门决定）。
func TestSelectAccountWithOptions_PlatformFiltersPool(t *testing.T) {
	grokOAuth := Account{
		ID: 81041, Name: "grok-oauth", Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive,
		Schedulable: true, Concurrency: 5, Priority: 5, Credentials: map[string]any{"access_token": "tok"},
	}
	openAIOAuth := openAIOAuthSub(81042, 1)
	openAIOAuth.CatalogEntryIDs = nil
	svc := newProtocolMatchService(t, true, nil, grokOAuth, openAIOAuth)
	ctx := context.Background()

	result, err := svc.SelectAccountWithOptions(ctx, nil, "", "", nil, SelectOptions{Platform: PlatformGrok})
	require.NoError(t, err)
	require.Equal(t, grokOAuth.ID, result.Account.ID, "端点声明 grok：优先级更高的 openai 成品号不在池里")

	result, err = svc.SelectAccountWithOptions(ctx, nil, "", "", nil, SelectOptions{Platform: PlatformOpenAI})
	require.NoError(t, err)
	require.Equal(t, openAIOAuth.ID, result.Account.ID)
}
