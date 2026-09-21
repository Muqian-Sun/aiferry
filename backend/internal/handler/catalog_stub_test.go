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

// listAllCatalogStub 把任何模型名都当作同一条上架条目（ID 1），网关族按模型名推断，推断不出按 openai。
type listAllCatalogStub struct{}

func (listAllCatalogStub) ListListedEntries(context.Context) []service.ModelCatalogEntry { return nil }

func (listAllCatalogStub) ResolveRoute(_ context.Context, model string) (service.CatalogRoute, bool) {
	model = strings.TrimSpace(model)
	platform, ok := service.DetectModelPlatform(model)
	if !ok {
		platform = service.PlatformOpenAI
	}
	return service.CatalogRoute{EntryID: listAllCatalogEntryID, CanonicalModel: model, RequestedModel: model, Platform: platform}, true
}

// listAllCatalogEntryID 是 listAllCatalogStub 给所有模型的条目 ID。
const listAllCatalogEntryID int64 = 1

// boundToCatalogEntry 返回把账号绑到条目上的副本（目录路由下池成员判定读 CatalogEntryIDs）。
func boundToCatalogEntry(account service.Account, entryID int64) service.Account {
	for _, id := range account.CatalogEntryIDs {
		if id == entryID {
			return account
		}
	}
	account.CatalogEntryIDs = append(append([]int64(nil), account.CatalogEntryIDs...), entryID)
	return account
}

// allBoundToCatalogEntry 把一组账号都绑到条目上（stub 仓储实现 ListSchedulingCandidatesByCatalogEntry 用）。
func allBoundToCatalogEntry(accounts []service.Account, entryID int64) []service.Account {
	out := make([]service.Account, 0, len(accounts))
	for _, account := range accounts {
		out = append(out, boundToCatalogEntry(account, entryID))
	}
	return out
}
