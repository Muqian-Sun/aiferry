package routes

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// admitAllCatalog 让任何模型名通过目录准入；条目厂商按模型名推断成目录 vendor 串
// （推断不出按 openai），让 routePlatform 的按厂商分发在路由测试里可用。
type admitAllCatalog struct{}

func (admitAllCatalog) ResolveRoute(_ context.Context, model string) (service.CatalogRoute, bool) {
	model = strings.TrimSpace(model)
	entry := &service.ModelCatalogEntry{ID: 1, ModelID: model, Vendor: testVendorForModel(model), Status: service.ModelCatalogStatusListed}
	return service.CatalogRoute{EntryID: 1, CanonicalModel: model, RequestedModel: model, Entry: entry}, true
}

func testVendorForModel(model string) string {
	platform, ok := service.DetectModelPlatform(model)
	if !ok {
		return "openai"
	}
	switch platform {
	case service.PlatformGrok:
		return "xai"
	case service.PlatformKimi:
		return "moonshot"
	case service.PlatformOpenCodeGo:
		return "opencode"
	default:
		return platform
	}
}

// ListListedEntries 让 admitAllCatalog 也能充当 handler 的模型列表来源（空目录，
// 路由测试里的列表内容来自固定账号清单或生成清单，逐条按 ResolveRoute 放行）。
func (admitAllCatalog) ListListedEntries(context.Context) []service.ModelCatalogEntry { return nil }
