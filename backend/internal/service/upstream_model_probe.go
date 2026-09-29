package service

import (
	"context"
	"fmt"
	"strings"
)

// 建渠道时的「探测模型」（2026-09-29 muqian 定）：填好第三方地址与 key 后向上游要模型名单，
// 对上模型目录里的条目自动勾上承接，目录里没有的可以一键导入（未上架）再勾上。

// ProbedUpstreamModel 探测到的一个上游模型，以及它在模型目录里对上的条目（没对上时 EntryID 为 0）。
type ProbedUpstreamModel struct {
	ID           string `json:"id"`
	EntryID      int64  `json:"entry_id,omitempty"`
	EntryModelID string `json:"entry_model_id,omitempty"`
	Listed       bool   `json:"listed,omitempty"`
}

// ProbeUpstreamModels 向第三方 key 的上游要模型名单：只看协议地址（平台标签不参与），不补模型元数据；
// 上游没有模型列表端点（404 / 405）时报「不支持」，不像「同步上游模型」那样回落到 model_mapping。
func (s *AccountTestService) ProbeUpstreamModels(ctx context.Context, account *Account) ([]string, error) {
	models, _, err := s.fetchUpstreamModelList(ctx, account)
	if err != nil {
		if upstreamModelListEndpointUnsupported(err) {
			return nil, newUpstreamModelSyncUnsupportedError("Upstream does not provide a model list endpoint", err)
		}
		return nil, err
	}
	return dedupeAndSortModelIDs(models), nil
}

// MatchUpstreamModels 把上游模型名对到目录条目（上架与未上架都算）：先比规范化后的模型名与别名
// （NormalizeModelCatalogKey，与计价查表同口径），对不上再去掉厂商前缀（"anthropic/claude-…" 这类聚合平台写法）比一次。
func (s *ModelCatalogService) MatchUpstreamModels(ctx context.Context, upstream []string) ([]ProbedUpstreamModel, error) {
	entries, err := s.ListEntries(ctx)
	if err != nil {
		return nil, err
	}
	byKey := catalogEntriesByKey(entries)
	result := make([]ProbedUpstreamModel, 0, len(upstream))
	for _, id := range upstream {
		probed := ProbedUpstreamModel{ID: id}
		if entry, ok := lookupCatalogEntryForUpstreamModel(byKey, id); ok {
			probed.EntryID = entry.ID
			probed.EntryModelID = entry.ModelID
			probed.Listed = entry.Status == ModelCatalogStatusListed
		}
		result = append(result, probed)
	}
	return result, nil
}

// ImportUpstreamModels 把目录里还没有的上游模型建成条目：未上架；内置价格表查得到就带上价格与厂商，
// 查不到只有模型名（上架前要去模型页定价）。已有的（按模型名或别名对上）不重复建，原样返回。
// 返回的条目与 ids 一一对应（去重后），调用方据此把它们绑到渠道。
func (s *ModelCatalogService) ImportUpstreamModels(ctx context.Context, ids []string) ([]ModelCatalogEntry, error) {
	entries, err := s.ListEntries(ctx)
	if err != nil {
		return nil, err
	}
	byKey := catalogEntriesByKey(entries)
	seen := make(map[string]struct{}, len(ids))
	result := make([]ModelCatalogEntry, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		key := NormalizeModelCatalogKey(id)
		if key == "" {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		if existing, ok := lookupCatalogEntryForUpstreamModel(byKey, id); ok {
			result = append(result, *existing)
			continue
		}
		entry, found := s.LookupPriceFileEntry(id)
		if !found {
			entry = ModelCatalogEntry{}
		}
		entry.ID = 0
		entry.ModelID = id
		entry.Status = ModelCatalogStatusUnlisted
		entry.SeedAliases = nil
		if err := s.CreateEntry(ctx, &entry); err != nil {
			return nil, fmt.Errorf("import upstream model %q: %w", id, err)
		}
		byKey[key] = &entry
		result = append(result, entry)
	}
	return result, nil
}

func catalogEntriesByKey(entries []ModelCatalogEntry) map[string]*ModelCatalogEntry {
	byKey := make(map[string]*ModelCatalogEntry, len(entries)*2)
	for i := range entries {
		entry := &entries[i]
		byKey[NormalizeModelCatalogKey(entry.ModelID)] = entry
		for _, alias := range entry.Aliases {
			if key := NormalizeModelCatalogKey(alias.Alias); key != "" {
				if _, taken := byKey[key]; !taken {
					byKey[key] = entry
				}
			}
		}
	}
	return byKey
}

func lookupCatalogEntryForUpstreamModel(byKey map[string]*ModelCatalogEntry, id string) (*ModelCatalogEntry, bool) {
	if entry, ok := byKey[NormalizeModelCatalogKey(id)]; ok {
		return entry, true
	}
	if i := strings.LastIndex(id, "/"); i >= 0 && i < len(id)-1 {
		if entry, ok := byKey[NormalizeModelCatalogKey(id[i+1:])]; ok {
			return entry, true
		}
	}
	return nil, false
}
