//go:build unit

package service

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// 利润门是全局设置；token 请求经目录路由选号，候选是利润门用例条目的承接渠道。
// gatewayProfitTestUserRate 夹具用户倍率（阈值 = 用户倍率 × (1 − 最低毛利率)）。
const gatewayProfitTestUserRate = 0.5

// profitControlTestSettingService 造一份利润门设置（全站一档，只有最低毛利率；0 = 关），并让进程内缓存重载。
func profitControlTestSettingService(t *testing.T, minMargin float64) *SettingService {
	t.Helper()
	repo := newMockSettingRepo()
	require.NoError(t, repo.Set(context.Background(), SettingKeyProfitMinMargin, strconv.FormatFloat(minMargin, 'f', -1, 64)))
	InvalidateProfitControlSettingsCache()
	t.Cleanup(InvalidateProfitControlSettingsCache)
	return NewSettingService(repo, &config.Config{})
}

// gatewayProfitTestContext 模拟认证后的 token 请求上下文：D 取用户倍率；带利润门用例条目的目录路由，
// 各渠道的上游成本比取自 prices（token 请求一律经目录路由，选号从条目的承接渠道里挑）。
func gatewayProfitTestContext(prices profitTestPrices) context.Context {
	ctx := WithUserRateMultiplier(context.Background(), &User{ID: 1, RateMultiplier: officialRate(gatewayProfitTestUserRate)})
	ctx, _ = WithGatewayTokenRequestPricing(ctx)
	// 目录路由下选渠道要看能否承接入站协议；夹具渠道四种协议都配了，这里取 Messages。
	ctx = WithInboundProtocol(ctx, APIProtocolAnthropic)
	return prices.withRoute(ctx)
}

// gatewayProfitTestAccount 承接利润门用例条目的渠道，上游成本比 rate 登记进 prices。
func gatewayProfitTestAccount(prices profitTestPrices, id int64, platform string, rate float64) Account {
	prices[id] = rate
	return Account{
		ID:              id,
		Name:            "account",
		Platform:        platform,
		Type:            AccountTypeAPIKey,
		Status:          StatusActive,
		Schedulable:     true,
		Concurrency:     2,
		Priority:        1,
		CatalogEntryIDs: []int64{profitTestEntryID},
		// 第三方 key 能否在某个网关平台被调度看协议地址；四种协议都配上，
		// 让利润控制用例与账号的平台标签、分组平台无关。
		ProtocolEndpoints: map[string]string{
			APIProtocolAnthropic:       "https://relay.example.com",
			APIProtocolChatCompletions: "https://relay.example.com/v1",
			APIProtocolResponses:       "https://relay.example.com/v1",
			APIProtocolGemini:          "https://relay.example.com",
		},
	}
}

// 门是全站一档：最低毛利率 > 0，任何平台的 token 请求都装门（D2a）；非 token 请求（模型列表 / 媒体）不装；
// 最低毛利率 0 不装；同一请求 ctx 里已有门时复用（failover 阈值稳定）。
func TestGatewayProfitControlInstallsFromGlobalSettings(t *testing.T) {
	prices := profitTestPrices{}
	for _, platform := range []string{PlatformOpenAI, PlatformAnthropic, PlatformGemini, PlatformGrok, PlatformAntigravity, PlatformKimi} {
		t.Run(platform, func(t *testing.T) {
			svc := &GatewayService{settingService: profitControlTestSettingService(t, 0.25)}

			tokenCtx := svc.withGatewayProfitControlGate(gatewayProfitTestContext(prices))
			gate, _ := tokenCtx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate)
			require.NotNil(t, gate)
			require.InDelta(t, 0.5*(1-0.25), gate.threshold, 1e-12, "阈值 = 用户倍率 × (1 − 最低毛利率)")

			reused := svc.withGatewayProfitControlGate(tokenCtx)
			require.Same(t, gate, reused.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate), "同一请求复用已装的门")

			metadataCtx := svc.withGatewayProfitControlGate(context.Background())
			gate, _ = metadataCtx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate)
			require.Nil(t, gate, "未显式标记为 token 请求的入口不得装门")
		})
	}

	// muqian 2026-10-07：最低毛利率 0 = 不能亏本，门照样装，上游成本比不超过用户倍率就放行。
	t.Run("zero margin means no loss", func(t *testing.T) {
		svc := &GatewayService{settingService: profitControlTestSettingService(t, 0)}
		atCost := gatewayProfitTestAccount(prices, 102, PlatformOpenAI, gatewayProfitTestUserRate)
		expensive := gatewayProfitTestAccount(prices, 103, PlatformOpenAI, 0.9)
		ctx := svc.withGatewayProfitControlGate(gatewayProfitTestContext(prices))
		gate, _ := ctx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate)
		require.NotNil(t, gate)
		require.InDelta(t, gatewayProfitTestUserRate, gate.threshold, 1e-12, "阈值 = 用户倍率")
		require.True(t, svc.isGatewayAccountProfitEligible(ctx, &atCost), "成本正好等于售价：不亏，放行")
		require.False(t, svc.isGatewayAccountProfitEligible(ctx, &expensive), "亏本的渠道不派")
	})

	t.Run("no setting service", func(t *testing.T) {
		svc := &GatewayService{}
		ctx := svc.withGatewayProfitControlGate(gatewayProfitTestContext(prices))
		gate, _ := ctx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate)
		require.Nil(t, gate)
	})
}

// 无并发服务（L2 走优先级 + LRU 回退）时利润门同样生效：贵的被挡，只剩贵的就无可用账号。
func TestGatewayProfitControlSelectionWithoutConcurrency(t *testing.T) {
	prices := profitTestPrices{}
	cheap := gatewayProfitTestAccount(prices, 1, PlatformGrok, 0.2)
	expensive := gatewayProfitTestAccount(prices, 2, PlatformGrok, 0.8)
	repo := &mockAccountRepoForPlatform{
		accounts:     []Account{expensive, cheap},
		accountsByID: map[int64]*Account{cheap.ID: &cheap, expensive.ID: &expensive},
	}
	svc := &GatewayService{
		accountRepo:    repo,
		cache:          &mockGatewayCacheForPlatform{},
		cfg:            testConfig(),
		settingService: profitControlTestSettingService(t, 0.2),
	}
	opts := SelectOptions{}

	selected, err := svc.SelectAccountWithOptions(gatewayProfitTestContext(prices), "", "", nil, opts)
	require.NoError(t, err)
	require.Equal(t, cheap.ID, selected.Account.ID)

	_, err = svc.SelectAccountWithOptions(gatewayProfitTestContext(prices), "", "", map[int64]struct{}{cheap.ID: {}}, opts)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
}

// 选号端到端：阈值 = 用户倍率 × (1 − 最低毛利率)。用户倍率 0.5、最低毛利率 0.3 → 阈值 0.35：
// 倍率正好 0.35 的渠道能派，0.4 的被筛掉（不看毛利率时 0.4 < 0.5 本可以派）；最低毛利率 0 不装门，0.4 的照派。
func TestGatewayProfitControlMinMarginFormulaInSelection(t *testing.T) {
	prices := profitTestPrices{}
	atThreshold := gatewayProfitTestAccount(prices, 11, PlatformAnthropic, 0.35)
	overThreshold := gatewayProfitTestAccount(prices, 12, PlatformAnthropic, 0.4)
	newSvc := func(t *testing.T, minMargin float64) *GatewayService {
		repo := &mockAccountRepoForPlatform{
			accounts:     []Account{overThreshold, atThreshold},
			accountsByID: map[int64]*Account{atThreshold.ID: &atThreshold, overThreshold.ID: &overThreshold},
		}
		return &GatewayService{
			accountRepo:    repo,
			cache:          &mockGatewayCacheForPlatform{},
			cfg:            testConfig(),
			settingService: profitControlTestSettingService(t, minMargin),
		}
	}
	opts := SelectOptions{}
	onlyOver := map[int64]struct{}{atThreshold.ID: {}}

	t.Run("margin 0.3 filters by D x (1 - margin)", func(t *testing.T) {
		svc := newSvc(t, 0.3)
		selected, err := svc.SelectAccountWithOptions(gatewayProfitTestContext(prices), "", "", nil, opts)
		require.NoError(t, err)
		require.Equal(t, atThreshold.ID, selected.Account.ID, "倍率等于阈值 0.35 的渠道能派")

		_, err = svc.SelectAccountWithOptions(gatewayProfitTestContext(prices), "", "", onlyOver, opts)
		require.ErrorIs(t, err, ErrNoAvailableAccounts, "倍率 0.4 > 0.5 × (1 − 0.3) 的渠道被筛掉")
	})

	t.Run("margin 0 installs no gate", func(t *testing.T) {
		svc := newSvc(t, 0)
		selected, err := svc.SelectAccountWithOptions(gatewayProfitTestContext(prices), "", "", onlyOver, opts)
		require.NoError(t, err)
		require.Equal(t, overThreshold.ID, selected.Account.ID, "最低毛利率 0 = 关，倍率 0.4 的渠道照派")
	})
}

func TestGatewayProfitControlLoadAwareSelectionAndFailover(t *testing.T) {
	prices := profitTestPrices{}
	cheap := gatewayProfitTestAccount(prices, 1, PlatformGrok, 0.2)
	expensive := gatewayProfitTestAccount(prices, 2, PlatformGrok, 0.8)
	repo := &mockAccountRepoForPlatform{
		accounts:     []Account{expensive, cheap},
		accountsByID: map[int64]*Account{cheap.ID: &cheap, expensive.ID: &expensive},
	}
	cfg := &config.Config{RunMode: config.RunModeStandard}
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	svc := &GatewayService{
		accountRepo:        repo,
		cache:              &mockGatewayCacheForPlatform{},
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{}),
		settingService:     profitControlTestSettingService(t, 0.2),
	}

	opts := SelectOptions{}
	result, err := svc.SelectAccountWithOptions(gatewayProfitTestContext(prices), "", "", nil, opts)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, cheap.ID, result.Account.ID)
	if result.ReleaseFunc != nil {
		result.ReleaseFunc()
	}

	result, err = svc.SelectAccountWithOptions(gatewayProfitTestContext(prices), "", "", map[int64]struct{}{cheap.ID: {}}, opts)
	require.Nil(t, result)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
}

func TestGatewayProfitControlStickyVetoKeepsBindingUntilPriceRecovers(t *testing.T) {
	prices := profitTestPrices{}
	expensive := gatewayProfitTestAccount(prices, 1, PlatformAnthropic, 0.8)
	cheap := gatewayProfitTestAccount(prices, 2, PlatformAnthropic, 0.2)
	repo := &mockAccountRepoForPlatform{
		accounts:     []Account{expensive, cheap},
		accountsByID: map[int64]*Account{expensive.ID: &expensive, cheap.ID: &cheap},
	}
	cache := &mockGatewayCacheForPlatform{
		sessionBindings: map[string]int64{"sticky-profit": expensive.ID},
	}
	svc := &GatewayService{
		accountRepo:    repo,
		cache:          cache,
		cfg:            testConfig(),
		settingService: profitControlTestSettingService(t, 0.2),
	}
	ctx := gatewayProfitTestContext(prices)
	opts := SelectOptions{}

	selected, err := svc.SelectAccountWithOptions(ctx, "sticky-profit", "", nil, opts)
	require.NoError(t, err)
	require.Equal(t, cheap.ID, selected.Account.ID)
	require.Equal(t, expensive.ID, cache.sessionBindings["sticky-profit"], "候选过滤不得覆盖旧粘性绑定")

	require.NoError(t, svc.BindStickySessionAfterProfitAdmission(
		svc.withGatewayProfitControlGate(ctx),
		"sticky-profit",
		cheap.ID,
	))
	require.Equal(t, expensive.ID, cache.sessionBindings["sticky-profit"], "终检通过的 fallback 账号也不得覆盖旧绑定")
	require.Zero(t, cache.deletedSessions["sticky-profit"])

	// 上游价调低（新的目录快照，之后的请求带新路由）后应重新命中原粘性账号。
	prices[expensive.ID] = 0.2
	selected, err = svc.SelectAccountWithOptions(gatewayProfitTestContext(prices), "sticky-profit", "", nil, opts)
	require.NoError(t, err)
	require.Equal(t, expensive.ID, selected.Account.ID, "上游价调低后应重新命中原粘性账号")
	require.Zero(t, cache.deletedSessions["sticky-profit"])
}

type gatewayProfitSnapshotCache struct {
	SchedulerCache
	account *Account
	err     error
}

func (c *gatewayProfitSnapshotCache) GetAccount(context.Context, int64) (*Account, error) {
	return c.account, c.err
}

type gatewayProfitAccountRepo struct {
	AccountRepository
	account *Account
	err     error
}

func (r gatewayProfitAccountRepo) GetByID(context.Context, int64) (*Account, error) {
	return r.account, r.err
}

// 抢槽后终检：渠道对象按缓存 → 数据库的顺序刷新成最新的；上游成本比取自本请求的目录路由（与刷新无关）。
func TestGatewayProfitControlTerminalRefreshUsesReplacementObject(t *testing.T) {
	prices := profitTestPrices{}
	selected := gatewayProfitTestAccount(prices, 141, PlatformGemini, 0.8)
	replacement := selected

	snapshot := NewSchedulerSnapshotService(
		&gatewayProfitSnapshotCache{account: &replacement},
		nil,
		gatewayProfitAccountRepo{},
		nil,
	)
	ctx := context.WithValue(prices.withRoute(context.Background()), openAIProfitControlGateCtxKey{}, &openAIProfitControlGate{
		threshold: 0.5,
	})

	latest, vetoed, reason := profitControlVetoLatest(ctx, &selected, snapshot)
	require.Same(t, &replacement, latest)
	require.True(t, vetoed)
	require.Equal(t, openAIProfitFilterReasonThreshold, reason)
}

func TestGatewayProfitControlTerminalRefreshFallsBackFromCacheToDatabase(t *testing.T) {
	prices := profitTestPrices{}
	selected := gatewayProfitTestAccount(prices, 145, PlatformAnthropic, 0.8)
	replacement := selected

	snapshot := NewSchedulerSnapshotService(
		&gatewayProfitSnapshotCache{err: errors.New("cache unavailable")},
		nil,
		gatewayProfitAccountRepo{account: &replacement},
		nil,
	)
	ctx := context.WithValue(prices.withRoute(context.Background()), openAIProfitControlGateCtxKey{}, &openAIProfitControlGate{
		threshold: 0.5,
	})

	latest, vetoed, reason := profitControlVetoLatest(ctx, &selected, snapshot)
	require.Same(t, &replacement, latest, "缓存读取失败时必须继续从数据库重读，不能直接使用选号旧对象")
	require.True(t, vetoed)
	require.Equal(t, openAIProfitFilterReasonThreshold, reason)
}

func TestGatewayProfitControlTerminalRefreshFailureFallsBackToSelectedObject(t *testing.T) {
	prices := profitTestPrices{}
	selected := gatewayProfitTestAccount(prices, 151, PlatformAntigravity, 0.2)
	snapshot := NewSchedulerSnapshotService(
		&gatewayProfitSnapshotCache{err: errors.New("cache unavailable")},
		nil,
		gatewayProfitAccountRepo{err: errors.New("database unavailable")},
		nil,
	)
	ctx := context.WithValue(prices.withRoute(context.Background()), openAIProfitControlGateCtxKey{}, &openAIProfitControlGate{
		threshold: 0.5,
	})

	latest, vetoed, reason := profitControlVetoLatest(ctx, &selected, snapshot)
	require.Same(t, &selected, latest)
	require.False(t, vetoed)
	require.Empty(t, reason)
}

// 选号结果携带门：门安装在调度栈局部 ctx 上，handler 必须经
// ContextWithSelectionProfitGate 重放后终检与准入后绑定才可见（评审修复回归）。
func TestGatewayProfitControlSelectionCarriesGateToHandlerContext(t *testing.T) {
	prices := profitTestPrices{}
	svc := &GatewayService{settingService: profitControlTestSettingService(t, 0.2)}
	expensive := gatewayProfitTestAccount(prices, 161, PlatformAnthropic, 0.9)

	gateCtx := svc.withGatewayProfitControlGate(gatewayProfitTestContext(prices))
	selection, err := svc.newSelectionResult(gateCtx, &expensive, true, nil, nil)
	require.NoError(t, err)
	require.True(t, selection.ProfitGateActive(), "选号结果必须携带调度栈内生效的门")

	// handler 的请求 ctx 带着准入中间件放进去的目录路由，但不含调度栈里装的门。
	requestCtx := prices.withRoute(context.Background())

	// 修复前的缺陷形态：handler 原始 ctx 不含门，终检退化为空操作。
	_, vetoed, _ := svc.GatewayProfitControlVetoLatest(requestCtx, &expensive)
	require.False(t, vetoed, "对照组：不重放门时终检确实看不到门")

	handlerCtx := ContextWithSelectionProfitGate(requestCtx, selection)
	latest, vetoed, reason := svc.GatewayProfitControlVetoLatest(handlerCtx, &expensive)
	require.True(t, vetoed, "重放门后终检必须真实生效")
	require.Equal(t, openAIProfitFilterReasonThreshold, reason)
	require.NotNil(t, latest)

	// 无门选号不携带门，重放为无操作。
	plain, err := svc.newSelectionResult(context.Background(), &expensive, true, nil, nil)
	require.NoError(t, err)
	require.False(t, plain.ProfitGateActive())
	require.Equal(t, context.Background(), ContextWithSelectionProfitGate(context.Background(), plain))
}

// 生图意图不关门（H1/H2 回归锚点）：/v1/responses 混合请求即使带生图声明，
// token 定价上下文照常装配，共享门照常安装并否决越线账号。
func TestGatewayProfitControlImageIntentDoesNotDisableGate(t *testing.T) {
	prices := profitTestPrices{}
	svc := &GatewayService{settingService: profitControlTestSettingService(t, 0.2)}
	expensive := gatewayProfitTestAccount(prices, 162, PlatformAnthropic, 0.9)

	ctx := gatewayProfitTestContext(prices)
	ctx = WithOpenAIImageGenerationIntent(ctx)
	gateCtx := svc.withGatewayProfitControlGate(ctx)
	require.False(t, svc.isGatewayAccountProfitEligible(gateCtx, &expensive),
		"请求体里的生图声明（含被动 image_gen namespace）不得关闭利润门")
}

// 无门时准入后绑定回退官方 eager 语义；门下读失败保守不写（评审 M-Bind 回归）。
func TestGatewayProfitControlAfterAdmissionBindSemantics(t *testing.T) {
	expensiveID := int64(171)
	cheapID := int64(172)

	t.Run("eager without gate", func(t *testing.T) {
		cache := &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{"s": expensiveID}}
		svc := &GatewayService{cache: cache}
		require.NoError(t, svc.BindStickySessionAfterProfitAdmission(context.Background(), "s", cheapID))
		require.Equal(t, cheapID, cache.sessionBindings["s"], "无门时保持既有 eager 绑定行为")
	})

	t.Run("gated read failure is conservative", func(t *testing.T) {
		// mock 的 miss 返回非 sentinel 错误，等价于 Redis 读失败：门下保守不写。
		cache := &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{}}
		svc := &GatewayService{cache: cache}
		gate := &openAIProfitControlGate{threshold: 0.5}
		gateCtx := context.WithValue(context.Background(), openAIProfitControlGateCtxKey{}, gate)
		require.NoError(t, svc.BindStickySessionAfterProfitAdmission(gateCtx, "absent", cheapID))
		require.NotContains(t, cache.sessionBindings, "absent")
	})

	t.Run("gated sentinel miss binds", func(t *testing.T) {
		cache := &sentinelMissGatewayCache{mockGatewayCacheForPlatform: &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{}}}
		svc := &GatewayService{cache: cache}
		gate := &openAIProfitControlGate{threshold: 0.5}
		gateCtx := context.WithValue(context.Background(), openAIProfitControlGateCtxKey{}, gate)
		require.NoError(t, svc.BindStickySessionAfterProfitAdmission(gateCtx, "fresh", cheapID))
		require.Equal(t, cheapID, cache.sessionBindings["fresh"], "门下无既有绑定（sentinel miss）应建立粘性")
	})
}

// sentinelMissGatewayCache 让 miss 返回与真实仓库一致的 ErrStickySessionNotFound。
type sentinelMissGatewayCache struct {
	*mockGatewayCacheForPlatform
}

func (c *sentinelMissGatewayCache) GetSessionAccountID(ctx context.Context, groupID int64, sessionHash string) (int64, error) {
	if id, ok := c.sessionBindings[sessionHash]; ok {
		return id, nil
	}
	return 0, ErrStickySessionNotFound
}
