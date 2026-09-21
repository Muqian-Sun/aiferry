package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// 周期任务对 coding plan 账号的额度探测行为（runOnce 集成路径）：
//   - kimi coding 账号（含已被阈值停调的）→ 额度探测被调用；
//   - 智谱 coding 账号 → 额度探测被调用（智谱不进 kimi/deepseek 余额循环）；
//   - payg 账号不经过额度探测（走余额路径，本测试不放 payg 账号避免真实网络）；
//   - 非激活账号完全跳过；
//   - 供应商按协议地址识别，平台标签不参与：kimi 标签的中转 key 不探，
//     openai 标签的官方智谱 key 要探。

// fakeCNQuotaProber 需要并发安全：runOnce 以 cnQuotaProbeConcurrency 并发调用 QueryUsage。
type fakeCNQuotaProber struct {
	mu     sync.Mutex
	probed []int64
}

func (f *fakeCNQuotaProber) QueryUsage(ctx context.Context, accountID int64) (*CNProviderQuotaProbeResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.probed = append(f.probed, accountID)
	return &CNProviderQuotaProbeResult{Success: true, Persisted: true}, nil
}

type fakeCNCheckRepo struct {
	AccountRepository
	active []Account
}

// ListActive 只返回 active 的账号，与真实仓储口径一致。
func (r *fakeCNCheckRepo) ListActive(context.Context) ([]Account, error) {
	var out []Account
	for _, account := range r.active {
		if account.IsActive() {
			out = append(out, account)
		}
	}
	return out, nil
}

func TestCNProviderBalanceCheckRunOnceProbesCodingPlanQuota(t *testing.T) {
	kimiActive := Account{ID: 1, Platform: PlatformKimi, Type: AccountTypeAPIKey, Status: StatusActive,
		Credentials: map[string]any{"account_mode": "coding"}, ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.moonshot.cn/anthropic", APIProtocolChatCompletions: "https://api.moonshot.cn/v1"}}
	// 已被阈值停调的 coding 账号也要刷新快照（决定是否续停）。
	kimiPaused := Account{ID: 2, Platform: PlatformKimi, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: false,
		Credentials: map[string]any{"account_mode": "coding"}, ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.moonshot.cn/anthropic", APIProtocolChatCompletions: "https://api.moonshot.cn/v1"}}
	// 非激活账号跳过。
	kimiInactive := Account{ID: 3, Platform: PlatformKimi, Type: AccountTypeAPIKey, Status: StatusDisabled,
		Credentials: map[string]any{"account_mode": "coding"}, ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.moonshot.cn/anthropic", APIProtocolChatCompletions: "https://api.moonshot.cn/v1"}}
	zhipuCoding := Account{ID: 4, Platform: PlatformZhipu, Type: AccountTypeAPIKey, Status: StatusActive,
		Credentials: map[string]any{"account_mode": "coding"}, ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://open.bigmodel.cn/api/anthropic", APIProtocolChatCompletions: "https://open.bigmodel.cn/api/paas/v4"}}
	minimaxCoding := Account{ID: 5, Platform: PlatformMiniMax, Type: AccountTypeAPIKey, Status: StatusActive,
		Credentials: map[string]any{"account_mode": "coding"}, ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.minimaxi.com/anthropic", APIProtocolChatCompletions: "https://api.minimaxi.com/v1", APIProtocolResponses: "https://api.minimaxi.com/v1"}}

	// 标签 kimi 但地址是中转：不是国产供应商，不探。
	kimiLabelledRelay := Account{ID: 6, Platform: PlatformKimi, Type: AccountTypeAPIKey, Status: StatusActive,
		Credentials: map[string]any{"account_mode": "coding"}, ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://relay.example/v1"}}
	// 标签 openai 但地址是官方智谱：按厂商探。
	openaiLabelledZhipu := Account{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive,
		Credentials: map[string]any{"account_mode": "coding"}, ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://open.bigmodel.cn/api/anthropic", APIProtocolChatCompletions: "https://open.bigmodel.cn/api/paas/v4"}}
	// 成品号不参与。
	kimiOAuth := Account{ID: 8, Platform: PlatformKimi, Type: AccountTypeOAuth, Status: StatusActive,
		Credentials: map[string]any{"account_mode": "coding"}}

	repo := &fakeCNCheckRepo{active: []Account{
		kimiActive, kimiPaused, kimiInactive, zhipuCoding, minimaxCoding, kimiLabelledRelay, openaiLabelledZhipu, kimiOAuth,
	}}
	prober := &fakeCNQuotaProber{}
	svc := &CNProviderBalanceCheckService{
		accountRepo:  repo,
		quotaService: prober,
		cfg:          &config.Config{},
	}

	svc.runOnce()

	require.ElementsMatch(t, []int64{1, 2, 4, 5, 7}, prober.probed)
}

// runOnceZhipuQuota 在 quotaService 缺失时安全跳过（Start 门控不启动的老部署路径）。
func TestCNProviderBalanceCheckRunOnceWithoutQuotaService(t *testing.T) {
	repo := &fakeCNCheckRepo{active: []Account{
		{ID: 4, Platform: PlatformZhipu, Type: AccountTypeAPIKey, Status: StatusActive,
			Credentials: map[string]any{"account_mode": "coding"}, ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://open.bigmodel.cn/api/anthropic", APIProtocolChatCompletions: "https://open.bigmodel.cn/api/paas/v4"}},
	}}
	svc := &CNProviderBalanceCheckService{accountRepo: repo, cfg: &config.Config{}}
	require.NotPanics(t, func() { svc.runOnce() })
}

// recordingCNBalanceLoadRepo 记录余额探测服务的 GetByID 调用：payg 账号进入
// payg 检查队列必然触发 loadPayGAccount → GetByID，以此断言"未入队"这一内部
// 状态。GetByID 直接报错，保证不发生真实外呼。
type recordingCNBalanceLoadRepo struct {
	AccountRepository
	getByIDIDs []int64
}

func (r *recordingCNBalanceLoadRepo) GetByID(ctx context.Context, id int64) (*Account, error) {
	r.getByIDIDs = append(r.getByIDIDs, id)
	return nil, errors.New("not found")
}

// 挂在国产平台下、base_url 指向官方 ollama.com 的账号由 Ollama Cloud 用量窗口
// 负责：CN 探测端点由 base_url 衍生，ollama.com 会被出站 URL 白名单拒绝
// （CN_BALANCE_URL_REJECTED），周期任务必须整体跳过——不进额度目标，也不进
// payg 检查队列。
func TestCNProviderBalanceCheckRunOnceSkipsOllamaCloudUsageAccounts(t *testing.T) {
	// 对照组：普通 kimi coding 账号仍进额度探测。
	kimiCoding := Account{ID: 1, Platform: PlatformKimi, Type: AccountTypeAPIKey, Status: StatusActive,
		Credentials: map[string]any{"account_mode": "coding"}, ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.moonshot.cn/anthropic", APIProtocolChatCompletions: "https://api.moonshot.cn/v1"}}
	// ollama.com 挂 kimi：coding 模式不进额度目标。
	ollamaKimiCoding := Account{ID: 2, Platform: PlatformKimi, Type: AccountTypeAPIKey, Status: StatusActive,
		Credentials:       map[string]any{"account_mode": "coding", "base_url": "https://ollama.com"},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://ollama.com"}}
	// ollama.com 挂 kimi：payg 模式不进 payg 检查队列。
	ollamaKimiPayg := Account{ID: 3, Platform: PlatformKimi, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Credentials:       map[string]any{"base_url": "https://ollama.com", "api_key": "sk-ollama"},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://ollama.com"}}

	loadRepo := &recordingCNBalanceLoadRepo{}
	repo := &fakeCNCheckRepo{active: []Account{kimiCoding, ollamaKimiCoding, ollamaKimiPayg}}
	prober := &fakeCNQuotaProber{}
	svc := &CNProviderBalanceCheckService{
		accountRepo:    repo,
		balanceService: NewCNProviderBalanceService(loadRepo, nil, nil, &config.Config{}),
		quotaService:   prober,
		cfg:            &config.Config{},
	}

	svc.runOnce()

	require.Equal(t, []int64{1}, prober.probed)
	require.Empty(t, loadRepo.getByIDIDs, "ollama 账号不得进入 payg 检查队列")
}

// 双币种（deepseek CNY+USD）停调判定：任一币种达标即不停调，全部低于阈值才停；
// 无明细时退回主币种（兼容旧结果）。
func TestAllCNBalancesBelowThreshold(t *testing.T) {
	dualLow := &CNProviderBalanceResult{
		Balance:  1.0,
		Currency: "CNY",
		Balances: []CNProviderBalanceEntry{
			{Currency: "CNY", Balance: 1.0},
			{Currency: "USD", Balance: 0.5},
		},
	}
	require.True(t, allCNBalancesBelowThreshold(dualLow, 5.0))

	dualMixed := &CNProviderBalanceResult{
		Balance:  1.0,
		Currency: "CNY",
		Balances: []CNProviderBalanceEntry{
			{Currency: "CNY", Balance: 1.0},
			{Currency: "USD", Balance: 20.0},
		},
	}
	require.False(t, allCNBalancesBelowThreshold(dualMixed, 5.0))

	// 无明细：按主币种判定（旧行为）。
	singleLow := &CNProviderBalanceResult{Balance: 1.0, Currency: "CNY"}
	require.True(t, allCNBalancesBelowThreshold(singleLow, 5.0))
	singleOK := &CNProviderBalanceResult{Balance: 10.0, Currency: "CNY"}
	require.False(t, allCNBalancesBelowThreshold(singleOK, 5.0))
}
