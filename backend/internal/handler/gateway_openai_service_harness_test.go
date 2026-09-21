package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// gatewayHarnessGroupRepo 只回答分组查询：Gateway 调度器按 apiKey.GroupID 取分组平台。
type gatewayHarnessGroupRepo struct {
	service.GroupRepository
	group *service.Group
}

func (r gatewayHarnessGroupRepo) GetByID(context.Context, int64) (*service.Group, error) {
	return r.group, nil
}

func (r gatewayHarnessGroupRepo) GetByIDLite(context.Context, int64) (*service.Group, error) {
	return r.group, nil
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
	gwSvc := service.NewGatewayService(
		accountRepo, gatewayHarnessGroupRepo{group: group}, nil, nil, nil, nil, nil, cfg,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
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
