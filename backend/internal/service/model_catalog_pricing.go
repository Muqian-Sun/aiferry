package service

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// 价格页（管理站「供给 › 价格」）：官方价与上游价都在这里改；给模型加一个渠道就是这个渠道承接这个模型
// （muqian 2026-09-30，方案页 X8eQjqzjAEx3hF4xzCzKZr）。按模型保存一块 = 官方价 + 这个模型的全部承接关系；
// 按渠道保存一块 = 这个渠道承接的全部模型与上游价。两种保存都是整份覆盖、一个事务。

// OfficialPrices 价格页上改的官方价：五项 token 价（USD / token）与按 Token 分段。
type OfficialPrices struct {
	InputPrice        *float64
	OutputPrice       *float64
	CacheWritePrice   *float64
	CacheWrite1hPrice *float64
	CacheReadPrice    *float64
	Intervals         []PricingInterval
}

// UpstreamCostRatio 这条承接关系的上游成本比（上游价 ÷ 官方价，逐项、逐段取最高）；价格页的毛利
// 与利润门用同一个数。官方价没有可比的项时 ok=false。
func (e *ModelCatalogEntry) UpstreamCostRatio(b *ModelCatalogBinding) (ratio float64, ok bool) {
	return bindingCostRatio(e, b)
}

// ValidateAgainst 校验承接关系上的上游价：
//   - 只有按 Token 计费的模型能设承接（现阶段只做大语言模型）；
//   - 各项价 >= 0；官方价有的缓存项（缓存写 5 分钟 / 1 小时、缓存读）上游价也必须填（muqian：「必须填，没填不能承接」）；
//   - 分段与官方价同一套规则（ValidateIntervals），只用绝对价，每段至少一项价。
func (b *ModelCatalogBinding) ValidateAgainst(entry *ModelCatalogEntry) error {
	if b == nil || entry == nil {
		return catalogValidationError("binding and entry are required")
	}
	if entry.EffectiveBillingMode() != BillingModeToken {
		return catalogValidationError(fmt.Sprintf("%s is not billed by token; only token models can be bound on the pricing page", entry.ModelID))
	}
	prices := map[string]*float64{
		"input_price":          &b.InputPrice,
		"output_price":         &b.OutputPrice,
		"cache_write_price":    b.CacheWritePrice,
		"cache_write_1h_price": b.CacheWrite1hPrice,
		"cache_read_price":     b.CacheReadPrice,
	}
	for _, name := range sortedPriceFieldNames(prices) {
		if value := prices[name]; value != nil && *value < 0 {
			return catalogValidationError(fmt.Sprintf("upstream %s must be >= 0", name))
		}
	}
	required := []struct {
		name     string
		official *float64
		upstream *float64
	}{
		{"cache_write_price", entry.CacheWritePrice, b.CacheWritePrice},
		{"cache_write_1h_price", entry.CacheWrite1hPrice, b.CacheWrite1hPrice},
		{"cache_read_price", entry.CacheReadPrice, b.CacheReadPrice},
	}
	for _, item := range required {
		if item.official != nil && item.upstream == nil {
			return catalogValidationError(fmt.Sprintf("upstream %s is required because the official price has it", item.name))
		}
	}
	return validatePriceSegments("upstream", b.Intervals)
}

// validatePriceSegments 价格页上的分段（官方价与上游价同一套）：只用绝对价、按 Token 分段、每段至少一项价，
// 再走 ValidateIntervals 查区间合法与不重叠。
func validatePriceSegments(label string, intervals []PricingInterval) error {
	for i, iv := range intervals {
		if iv.InputMultiplier != nil || iv.OutputMultiplier != nil || iv.CacheWriteMultiplier != nil || iv.CacheReadMultiplier != nil {
			return catalogValidationError(fmt.Sprintf("%s segment %d must use absolute prices", label, i+1))
		}
		if iv.TierLabel != "" || iv.PerRequestPrice != nil {
			return catalogValidationError(fmt.Sprintf("%s segment %d must be a token segment", label, i+1))
		}
		if iv.InputPrice == nil && iv.OutputPrice == nil && iv.CacheWritePrice == nil && iv.CacheWrite1hPrice == nil && iv.CacheReadPrice == nil {
			return catalogValidationError(fmt.Sprintf("%s segment %d has no price", label, i+1))
		}
	}
	if err := ValidateIntervals(intervals, BillingModeToken); err != nil {
		return catalogValidationError(label + " " + err.Error())
	}
	return nil
}

// normalizePriceSegments 按起点排序并重排 SortOrder；丢掉请求里带来的 ID 与时间戳（整份覆盖时重建）。
func normalizePriceSegments(intervals []PricingInterval) []PricingInterval {
	if len(intervals) == 0 {
		return nil
	}
	out := make([]PricingInterval, len(intervals))
	copy(out, intervals)
	sort.SliceStable(out, func(i, j int) bool { return out[i].MinTokens < out[j].MinTokens })
	for i := range out {
		out[i].ID = 0
		out[i].PricingID = 0
		out[i].SortOrder = i
		out[i].CreatedAt = time.Time{}
		out[i].UpdatedAt = time.Time{}
	}
	return out
}

// validateBindingAccount 渠道能承接这个模型：至少有一个入站协议能承接；经扩展端点调用的条目还要能承接扩展端点。
func (s *ModelCatalogService) validateBindingAccount(entry *ModelCatalogEntry, account *Account) error {
	if err := AccountServesCatalogEntry(entry, account); err != nil {
		return err
	}
	if CatalogEntryServedByExtensionEndpoints(entry, s.priceFileMode(entry.ModelID)) {
		return AccountServesCatalogExtensionEndpoints(entry, account)
	}
	return nil
}

// CanBindAccount 这个渠道能不能承接这个模型（价格页「加渠道 / 加模型」只列能承接的）。
func (s *ModelCatalogService) CanBindAccount(entry *ModelCatalogEntry, account *Account) bool {
	return s.validateBindingAccount(entry, account) == nil
}

// ListPricingEntries 价格页的数据：按 Token 计费的条目（带官方价、分段与承接关系），直接读库，不走热路径快照。
// 现阶段只做大语言模型，生图 / 视频 / 按次的条目不在价格页。
func (s *ModelCatalogService) ListPricingEntries(ctx context.Context) ([]ModelCatalogEntry, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	entries, err := s.repo.ListEntries(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ModelCatalogEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.EffectiveBillingMode() == BillingModeToken {
			out = append(out, entry)
		}
	}
	return out, nil
}

// SaveEntryPricing 价格页按模型保存一块：官方价（输入 / 输出必填）+ 这个模型的全部承接关系（整份覆盖），
// 同一事务；全部校验通过才写库，然后失效目录快照。返回保存后的条目（带承接关系）。
func (s *ModelCatalogService) SaveEntryPricing(ctx context.Context, entryID int64, official OfficialPrices, bindings []ModelCatalogBinding, accounts CatalogBindingAccountSource) (*ModelCatalogEntry, error) {
	if s == nil || s.repo == nil {
		return nil, ErrModelCatalogEntryNotFound
	}
	current, err := s.repo.GetEntryByID(ctx, entryID)
	if err != nil {
		return nil, err
	}
	entry := current.Clone()
	if entry.EffectiveBillingMode() != BillingModeToken {
		return nil, catalogValidationError(fmt.Sprintf("%s is not billed by token; only token models are priced on the pricing page", entry.ModelID))
	}
	if official.InputPrice == nil || official.OutputPrice == nil {
		return nil, catalogValidationError("official input_price and output_price are required")
	}
	if err := validatePriceSegments("official", official.Intervals); err != nil {
		return nil, err
	}
	entry.InputPrice = clonePricePtr(official.InputPrice)
	entry.OutputPrice = clonePricePtr(official.OutputPrice)
	entry.CacheWritePrice = clonePricePtr(official.CacheWritePrice)
	entry.CacheWrite1hPrice = clonePricePtr(official.CacheWrite1hPrice)
	entry.CacheReadPrice = clonePricePtr(official.CacheReadPrice)
	entry.Intervals = normalizePriceSegments(official.Intervals)
	entry.ManagedBy = ModelCatalogManagedByAdmin
	entry.Normalize()
	if err := entry.Validate(); err != nil {
		return nil, err
	}

	seen := make(map[int64]struct{}, len(bindings))
	normalized := make([]ModelCatalogBinding, 0, len(bindings))
	for _, binding := range bindings {
		if _, dup := seen[binding.AccountID]; dup {
			return nil, catalogValidationError(fmt.Sprintf("duplicate account %d in bindings", binding.AccountID))
		}
		seen[binding.AccountID] = struct{}{}
		account, err := accounts.GetAccount(ctx, binding.AccountID)
		if err != nil {
			return nil, err
		}
		if err := s.validateBindingAccount(entry, account); err != nil {
			return nil, err
		}
		binding.EntryID = entryID
		binding.Intervals = normalizePriceSegments(binding.Intervals)
		if err := binding.ValidateAgainst(entry); err != nil {
			return nil, err
		}
		normalized = append(normalized, binding)
	}
	if err := s.repo.SaveEntryPricing(ctx, entry, normalized); err != nil {
		return nil, err
	}
	s.invalidate(ctx)
	return s.repo.GetEntryByID(ctx, entryID)
}

// SaveAccountPricing 价格页按渠道保存一块：这个渠道承接的全部模型与上游价（整份覆盖：删掉不在列表里的、
// 改价、新增），同一事务；全部校验通过才写库，然后失效目录快照。返回保存后的承接关系。
func (s *ModelCatalogService) SaveAccountPricing(ctx context.Context, accountID int64, bindings []ModelCatalogBinding, accounts CatalogBindingAccountSource) ([]ModelCatalogBinding, error) {
	if s == nil || s.repo == nil {
		return nil, ErrModelCatalogEntryNotFound
	}
	account, err := accounts.GetAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]struct{}, len(bindings))
	normalized := make([]ModelCatalogBinding, 0, len(bindings))
	for _, binding := range bindings {
		if _, dup := seen[binding.EntryID]; dup {
			return nil, catalogValidationError(fmt.Sprintf("duplicate entry %d in bindings", binding.EntryID))
		}
		seen[binding.EntryID] = struct{}{}
		entry, err := s.repo.GetEntryByID(ctx, binding.EntryID)
		if err != nil {
			return nil, err
		}
		if err := s.validateBindingAccount(entry, account); err != nil {
			return nil, err
		}
		binding.AccountID = accountID
		binding.Intervals = normalizePriceSegments(binding.Intervals)
		if err := binding.ValidateAgainst(entry); err != nil {
			return nil, err
		}
		normalized = append(normalized, binding)
	}
	if err := s.repo.ReplaceAccountBindings(ctx, accountID, normalized); err != nil {
		return nil, err
	}
	s.invalidate(ctx)
	return normalized, nil
}
