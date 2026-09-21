//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 协议匹配优先（系统特色之一）：池里有能以入站协议直连的资源就用它，没有才按配置的优先级转换；
// 成品号与 key 平等，同优先级不再成品号优先；粘性已绑定的资源只要还能承接就继续用。

const protocolMatchEntryID = int64(91)

func protocolMatchPool() (responsesKey, anthropicOAuth, geminiOAuth, geminiKey Account) {
	responsesKey = Account{
		ID: 91001, Name: "responses-key", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive,
		Schedulable: true, Concurrency: 5, Priority: 1, CatalogEntryIDs: []int64{protocolMatchEntryID},
		ProtocolEndpoints: map[string]string{APIProtocolResponses: "https://relay.example.com"},
	}
	anthropicOAuth = Account{
		ID: 91002, Name: "anthropic-oauth", Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive,
		Schedulable: true, Concurrency: 5, Priority: 5, CatalogEntryIDs: []int64{protocolMatchEntryID},
	}
	geminiOAuth = Account{
		ID: 91003, Name: "gemini-oauth", Platform: PlatformGemini, Type: AccountTypeOAuth, Status: StatusActive,
		Schedulable: true, Concurrency: 5, Priority: 3, CatalogEntryIDs: []int64{protocolMatchEntryID},
	}
	geminiKey = Account{
		ID: 91004, Name: "gemini-key", Platform: PlatformGemini, Type: AccountTypeAPIKey, Status: StatusActive,
		Schedulable: true, Concurrency: 5, Priority: 3, CatalogEntryIDs: []int64{protocolMatchEntryID},
		ProtocolEndpoints: map[string]string{APIProtocolGemini: "https://relay.example.com"},
	}
	return
}

func newProtocolMatchService(t *testing.T, loadBatch bool, cache *mockGatewayCacheForPlatform, accounts ...Account) *GatewayService {
	t.Helper()
	repo := &mockAccountRepoForPlatform{accounts: accounts, accountsByID: map[int64]*Account{}}
	for i := range repo.accounts {
		repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
	}
	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = loadBatch
	if cache == nil {
		cache = &mockGatewayCacheForPlatform{}
	}
	return &GatewayService{
		accountRepo:        repo,
		groupRepo:          &mockGroupRepoForGateway{groups: map[int64]*Group{}},
		cache:              cache,
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(&mockConcurrencyCache{}),
	}
}

func TestSelectAccountWithLoadAwareness_ProtocolMatchBeatsPriority(t *testing.T) {
	responsesKey, anthropicOAuth, _, _ := protocolMatchPool()
	for _, loadBatch := range []bool{true, false} {
		name := map[bool]string{true: "load aware", false: "legacy"}[loadBatch]
		t.Run(name, func(t *testing.T) {
			svc := newProtocolMatchService(t, loadBatch, nil, responsesKey, anthropicOAuth)

			result, err := svc.SelectAccountWithLoadAwareness(catalogRouteCtx(protocolMatchEntryID, APIProtocolAnthropic), nil, "", "claude-sonnet-4-5", nil)
			require.NoError(t, err)
			require.Equal(t, anthropicOAuth.ID, result.Account.ID, "message 入站：anthropic 直连赢过优先级更高但要转换的 responses key")

			result, err = svc.SelectAccountWithLoadAwareness(catalogRouteCtx(protocolMatchEntryID, APIProtocolResponses), nil, "", "claude-sonnet-4-5", nil)
			require.NoError(t, err)
			require.Equal(t, responsesKey.ID, result.Account.ID, "response 入站：直连的是 responses key")

			result, err = svc.SelectAccountWithLoadAwareness(catalogRouteCtx(protocolMatchEntryID, APIProtocolChatCompletions), nil, "", "claude-sonnet-4-5", nil)
			require.NoError(t, err)
			require.Equal(t, responsesKey.ID, result.Account.ID, "completion 入站两个都要转换：按配置的优先级")
		})
	}
}

// 同优先级、都要转换（message 入站到 gemini 资源）、都从未用过：随机，成品号不再优先于 key
// （原 preferOAuth 会让 gemini 池里的成品号在这种平局下永远赢）。
func TestSelectAccountWithLoadAwareness_SamePriorityNoOAuthPreference(t *testing.T) {
	_, _, geminiOAuth, geminiKey := protocolMatchPool()
	for _, loadBatch := range []bool{true, false} {
		name := map[bool]string{true: "load aware", false: "legacy"}[loadBatch]
		t.Run(name, func(t *testing.T) {
			seen := map[int64]int{}
			for i := 0; i < 40; i++ {
				// 每轮换一下池里的顺序，legacy 路径是逐个比较，顺序不能成为偏好来源。
				pool := []Account{geminiOAuth, geminiKey}
				if i%2 == 1 {
					pool = []Account{geminiKey, geminiOAuth}
				}
				svc := newProtocolMatchService(t, loadBatch, nil, pool...)
				result, err := svc.SelectAccountWithLoadAwareness(catalogRouteCtx(protocolMatchEntryID, APIProtocolAnthropic), nil, "", "gemini-2.5-pro", nil)
				require.NoError(t, err)
				seen[result.Account.ID]++
			}
			require.Positive(t, seen[geminiKey.ID], "key 从未被选中：成品号仍有平局偏好")
			require.Positive(t, seen[geminiOAuth.ID])
		})
	}
}

// 粘性已绑定在要转换的资源上，池里新增了直连资源：绑定的资源只要还能承接就继续用。
func TestSelectAccountWithLoadAwareness_StickyKeepsConvertingAccount(t *testing.T) {
	responsesKey, anthropicOAuth, _, _ := protocolMatchPool()
	cache := &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{"session-1": responsesKey.ID}}
	svc := newProtocolMatchService(t, true, cache, responsesKey, anthropicOAuth)

	result, err := svc.SelectAccountWithLoadAwareness(catalogRouteCtx(protocolMatchEntryID, APIProtocolAnthropic), nil, "session-1", "claude-sonnet-4-5", nil)
	require.NoError(t, err)
	require.Equal(t, responsesKey.ID, result.Account.ID, "粘性绑定的 responses key 仍能承接 message → 继续用，不因 anthropic 直连出现而换")

	// 绑定的资源承接不了本次入站（generate 没有到 responses 的转换）→ 绑定失效，重选能承接的。
	_, _, _, geminiKey := protocolMatchPool()
	svc = newProtocolMatchService(t, true, &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{"session-2": responsesKey.ID}}, responsesKey, anthropicOAuth, geminiKey)
	result, err = svc.SelectAccountWithLoadAwareness(catalogRouteCtx(protocolMatchEntryID, APIProtocolGemini), nil, "session-2", "gemini-2.5-pro", nil)
	require.NoError(t, err)
	require.Equal(t, geminiKey.ID, result.Account.ID)
}

// 会话数限制按属性算：apikey 类型的资源设了 max_sessions 同样生效（原来只算 Anthropic OAuth / setup token）。
func TestSessionLimitAppliesToAnyAccountWithMaxSessions(t *testing.T) {
	responsesKey, _, _, _ := protocolMatchPool()
	responsesKey.Extra = map[string]any{"max_sessions": 1}
	limit := &stubSessionLimitCache{registered: map[int64]map[string]struct{}{}}
	svc := newProtocolMatchService(t, true, nil, responsesKey)
	svc.sessionLimitCache = limit

	ctx := catalogRouteCtx(protocolMatchEntryID, APIProtocolResponses)
	require.True(t, svc.checkAndRegisterSession(ctx, &responsesKey, "session-a"))
	require.False(t, svc.checkAndRegisterSession(ctx, &responsesKey, "session-b"), "第二个会话被 max_sessions=1 拒绝")
	require.True(t, svc.checkAndRegisterSession(ctx, &responsesKey, "session-a"), "已注册的会话可以继续")
}

type stubSessionLimitCache struct {
	SessionLimitCache
	registered map[int64]map[string]struct{}
}

func (c *stubSessionLimitCache) RegisterSession(_ context.Context, accountID int64, sessionID string, maxSessions int, _ time.Duration) (bool, error) {
	sessions := c.registered[accountID]
	if sessions == nil {
		sessions = map[string]struct{}{}
		c.registered[accountID] = sessions
	}
	if _, ok := sessions[sessionID]; ok {
		return true, nil
	}
	if len(sessions) >= maxSessions {
		return false, nil
	}
	sessions[sessionID] = struct{}{}
	return true, nil
}
