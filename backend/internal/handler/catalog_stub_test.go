package handler

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// listedCatalogStub 用一组模型标识充当目录里的上架条目；其余名字一律视为未上架。
// 网关族按模型名推断（推断不出按 openai）。
type listedCatalogStub struct {
	ids []string
}

func (s listedCatalogStub) ListListedEntries(context.Context) []service.ModelCatalogEntry {
	entries := make([]service.ModelCatalogEntry, 0, len(s.ids))
	for i, id := range s.ids {
		entries = append(entries, service.ModelCatalogEntry{ID: int64(i + 1), ModelID: id, Status: service.ModelCatalogStatusListed})
	}
	return entries
}

func (s listedCatalogStub) ResolveRoute(_ context.Context, model string) (service.CatalogRoute, bool) {
	model = strings.TrimSpace(model)
	for i, id := range s.ids {
		if strings.EqualFold(id, model) {
			platform, ok := service.DetectModelPlatform(id)
			if !ok {
				platform = service.PlatformOpenAI
			}
			return service.CatalogRoute{EntryID: int64(i + 1), CanonicalModel: id, RequestedModel: model, Platform: platform}, true
		}
	}
	return service.CatalogRoute{}, false
}

// listAllCatalogStub 把任何模型名都当作上架条目（清单合并测试只关心上游元数据处理）。
type listAllCatalogStub struct{}

func (listAllCatalogStub) ListListedEntries(context.Context) []service.ModelCatalogEntry { return nil }

func (listAllCatalogStub) ResolveRoute(_ context.Context, model string) (service.CatalogRoute, bool) {
	model = strings.TrimSpace(model)
	return service.CatalogRoute{EntryID: 1, CanonicalModel: model, RequestedModel: model, Platform: service.PlatformOpenAI}, true
}
