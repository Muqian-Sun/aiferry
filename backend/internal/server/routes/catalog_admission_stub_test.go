package routes

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// admitAllCatalog 让任何模型名通过目录准入，网关族按模型名推断（推断不出按 openai）。
type admitAllCatalog struct{}

func (admitAllCatalog) ResolveRoute(_ context.Context, model string) (service.CatalogRoute, bool) {
	model = strings.TrimSpace(model)
	platform, ok := service.DetectModelPlatform(model)
	if !ok {
		platform = service.PlatformOpenAI
	}
	return service.CatalogRoute{EntryID: 1, CanonicalModel: model, RequestedModel: model, Platform: platform}, true
}
