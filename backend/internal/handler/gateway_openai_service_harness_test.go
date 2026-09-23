package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func newGatewayHandlerOverOpenAIService(
	cfg *config.Config,
	accountRepo service.AccountRepository,
	openAISvc *service.OpenAIGatewayService,
	billingCache *service.BillingCacheService,
	concurrency *service.ConcurrencyService,
) *GatewayHandler {
	gwSvc := newTestSchedulerOverRepo(cfg, accountRepo)
	return &GatewayHandler{
		gatewayService:       gwSvc,
		openAIGatewayService: openAISvc,
		billingCacheService:  billingCache,
		apiKeyService:        service.NewAPIKeyService(nil, nil, nil, cfg),
		concurrencyHelper:    NewConcurrencyHelper(concurrency, SSEPingFormatClaude, 0),
		cfg:                  cfg,
		modelCatalog:         listAllCatalogStub{},
		maxAccountSwitches:   cfg.Gateway.MaxAccountSwitches,
	}
}

// newTestSchedulerOverRepo 装一个无快照的 GatewayService 当唯一调度器：候选直接列自 accountRepo，
// OpenAI 服务 / handler 的 WS 与扩展端点夹具都用它。
func newTestSchedulerOverRepo(cfg *config.Config, accountRepo service.AccountRepository) *service.GatewayService {
	return service.NewGatewayService(
		accountRepo, nil, nil, nil, nil, nil, cfg,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
}
