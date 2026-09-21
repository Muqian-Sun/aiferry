package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// gatewayHarnessGroupRepo 只回答分组查询：Gateway 调度器按 apiKey.GroupID 取分组平台。
// group 为 nil 时按请求的 ID 合成一个 openai 平台的活跃分组（夹具里分组 ID 各不相同时用）。
type gatewayHarnessGroupRepo struct {
	service.GroupRepository
	group *service.Group
}

func (r gatewayHarnessGroupRepo) GetByID(ctx context.Context, id int64) (*service.Group, error) {
	return r.GetByIDLite(ctx, id)
}

func (r gatewayHarnessGroupRepo) GetByIDLite(_ context.Context, id int64) (*service.Group, error) {
	if r.group != nil {
		return r.group, nil
	}
	return testOpenAIGroup(id), nil
}

// newGatewayHandlerOverOpenAIService 把一个已装配的 OpenAI 服务挂到 Gateway handler 上：
// /v1/responses 与 /v1/chat/completions 恒定由 Gateway handler 承接，选号走 GatewayService
// （无快照 → 直接列同一个账号仓库），转发 / 入账 / 健康观测仍由传入的 OpenAI 服务完成。
func newGatewayHandlerOverOpenAIService(
	cfg *config.Config,
	accountRepo service.AccountRepository,
	group *service.Group,
	openAISvc *service.OpenAIGatewayService,
	billingCache *service.BillingCacheService,
	concurrency *service.ConcurrencyService,
) *GatewayHandler {
	gwSvc := newTestSchedulerOverRepo(cfg, accountRepo, group)
	return &GatewayHandler{
		gatewayService:       gwSvc,
		openAIGatewayService: openAISvc,
		billingCacheService:  billingCache,
		apiKeyService:        service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg),
		concurrencyHelper:    NewConcurrencyHelper(concurrency, SSEPingFormatClaude, 0),
		cfg:                  cfg,
		modelCatalog:         listAllCatalogStub{},
		maxAccountSwitches:   cfg.Gateway.MaxAccountSwitches,
	}
}

// newTestSchedulerOverRepo 装一个无快照的 GatewayService 当唯一调度器：候选直接列自 accountRepo，
// 分组由 group 回答；OpenAI 服务 / handler 的 WS 与扩展端点夹具都用它。
func newTestSchedulerOverRepo(cfg *config.Config, accountRepo service.AccountRepository, group *service.Group) *service.GatewayService {
	return service.NewGatewayService(
		accountRepo, gatewayHarnessGroupRepo{group: group}, nil, nil, nil, nil, nil, cfg,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
}

// testOpenAIGroup 一个 openai 平台的活跃分组（WS / 扩展端点夹具的调度分组）。
func testOpenAIGroup(groupID int64) *service.Group {
	return &service.Group{ID: groupID, Platform: service.PlatformOpenAI, Status: service.StatusActive}
}
