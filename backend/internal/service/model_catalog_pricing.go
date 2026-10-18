package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// 价格页（管理站「供给 › 价格」）：官方价与上游价都在这里改；给模型加一个渠道就是这个渠道承接这个模型
// （muqian 2026-09-30，方案页 X8eQjqzjAEx3hF4xzCzKZr）。按模型保存一块 = 官方价 + 这个模型的全部承接关系；
// 按渠道保存一块 = 这个渠道承接的全部模型与上游价。两种保存都是整份覆盖、一个事务。

// OfficialPrices 价格页上改的官方价：五项 token 价（USD / token）、按 Token 分段，
// 联网搜索价（USD / 次、/ 条；nil = 用厂商公开价）。按次计价的条目（生图 / 视频 / 按次）只用 PerRequestPrice 与
// Intervals（这时是按 tier_label 的分档，每档一个按次价）。
type OfficialPrices struct {
	InputPrice         *float64
	OutputPrice        *float64
	CacheWritePrice    *float64
	CacheWrite1hPrice  *float64
	CacheReadPrice     *float64
	Intervals          []PricingInterval
	SearchPricePerCall *float64
	XPostPrice         *float64
	XUserPrice         *float64
	// TimePricing 官方忙闲时（目录条目的分时，如 DeepSeek 工作日高峰 × 2）：向用户收钱时整单乘倍数；nil = 不分忙闲时。
	TimePricing *TimePricing
	// MaxReasoningEffortMultiplier 官方最高推理倍率（如 claude-fable-5-1 的 effort = max 整单 × 3）；nil = 不加价。
	MaxReasoningEffortMultiplier *float64
	// PerRequestPrice 按次计价条目的官方按次价（USD / 张、/ 秒、/ 次），这类条目必填。
	PerRequestPrice *float64
}

// UpstreamCostRatio 这条承接关系的上游成本比（上游价 ÷ 官方价，逐项、逐段取最高）；价格页的毛利
// 与利润门用同一个数。官方价没有可比的项时 ok=false。
func (e *ModelCatalogEntry) UpstreamCostRatio(b *ModelCatalogBinding) (ratio float64, ok bool) {
	return bindingCostRatio(e, b)
}

// UpstreamPeakCostRatio 一周里最差的上游成本比（上游忙时涨、我们没涨的时段）；不比平时差时 ok=false。
func (e *ModelCatalogEntry) UpstreamPeakCostRatio(b *ModelCatalogBinding) (ratio float64, ok bool) {
	return bindingPeakCostRatio(e, b)
}

// UpstreamMaxReasoningCostRatio 最高推理档最差的上游成本比（上游在 max 档比售价涨得多）；不比平时差时 ok=false。
func (e *ModelCatalogEntry) UpstreamMaxReasoningCostRatio(b *ModelCatalogBinding) (ratio float64, ok bool) {
	return bindingMaxReasoningCostRatio(e, b)
}

// ValidateAgainst 校验承接关系上的上游价：
//   - 按次计价的模型（生图 / 视频 / 按次）只填上游按次价（必填、>= 0），见 validatePerRequestAgainst；
//   - 各项价 >= 0；官方价有的缓存项（缓存写 5 分钟 / 1 小时、缓存读）上游价也必须填（muqian：「必须填，没填不能承接」）；
//   - 联网搜索价可不填：没填 = 上游不收搜索费（muqian 2026-10-06：「上游没填费用就是免费」）；
//   - 最高推理倍率可不填（跟官方），填了须 > 0；
//   - 分段与官方价同一套规则（ValidateIntervals），只用绝对价，每段至少一项价；
//   - 上游模型名是一个具体的名字：不带通配、不含空白，最长 255 个字符。
func (b *ModelCatalogBinding) ValidateAgainst(entry *ModelCatalogEntry) error {
	if b == nil || entry == nil {
		return catalogValidationError("binding and entry are required")
	}
	if err := validateBindingUpstreamModel(b.UpstreamModel); err != nil {
		return err
	}
	if entry.IsPerRequestBilled() {
		return b.validatePerRequestAgainst(entry)
	}
	prices := map[string]*float64{
		"input_price":           &b.InputPrice,
		"output_price":          &b.OutputPrice,
		"cache_write_price":     b.CacheWritePrice,
		"cache_write_1h_price":  b.CacheWrite1hPrice,
		"cache_read_price":      b.CacheReadPrice,
		"search_price_per_call": b.SearchPricePerCall,
		"x_post_price":          b.XPostPrice,
		"x_user_price":          b.XUserPrice,
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
	if m := b.MaxReasoningEffortMultiplier; m != nil && *m <= 0 {
		return catalogValidationError("upstream max_reasoning_effort_multiplier must be > 0")
	}
	if err := validateTimePricing(b.TimePricing); err != nil {
		return catalogValidationError(fmt.Sprintf("upstream time_pricing: %s", err.Error()))
	}
	return validatePriceSegments("upstream", b.Intervals)
}

// validatePerRequestAgainst 按次计价模型的承接：上游按次价必填、>= 0（0 = 成品号这类不按次付钱的上游）；
// token 价、分段、搜索价、忙闲时、最高推理倍率都不参与媒体计费，填了就拒，免得看着填了却不生效。
func (b *ModelCatalogBinding) validatePerRequestAgainst(entry *ModelCatalogEntry) error {
	if b.PerRequestPrice == nil {
		return catalogValidationError(fmt.Sprintf("upstream per_request_price is required: %s is billed per request", entry.ModelID))
	}
	if *b.PerRequestPrice < 0 {
		return catalogValidationError("upstream per_request_price must be >= 0")
	}
	if b.InputPrice != 0 || b.OutputPrice != 0 || b.CacheWritePrice != nil || b.CacheWrite1hPrice != nil || b.CacheReadPrice != nil ||
		len(b.Intervals) > 0 || b.SearchPricePerCall != nil || b.XPostPrice != nil || b.XUserPrice != nil ||
		b.MaxReasoningEffortMultiplier != nil || b.TimePricing != nil {
		return catalogValidationError(fmt.Sprintf("%s is billed per request: upstream token prices, segments, search prices, peak hours and max reasoning do not apply", entry.ModelID))
	}
	return nil
}

// validateBindingUpstreamModel 上游模型名是一个具体的名字：不带通配、不含空白，最长 255 个字符；空 = 与目录标识同名。
func validateBindingUpstreamModel(name string) error {
	if name == "" {
		return nil
	}
	if strings.ContainsAny(name, "* \t\r\n") {
		return catalogValidationError("upstream_model must be a single model name without wildcards or spaces")
	}
	if utf8.RuneCountInString(name) > maxBindingUpstreamModelLength {
		return catalogValidationError(fmt.Sprintf("upstream_model must be at most %d characters", maxBindingUpstreamModelLength))
	}
	return nil
}

// roundPricePerMillion 售价、成本价每百万 Token 最多保留 4 位小数（muqian 2026-10-07）；单价存的是每 Token 的美元价，
// 所以按每 Token 10 位小数取整。
func roundPricePerMillion(price float64) float64 {
	return math.Round(price*1e10) / 1e10
}

func roundPricePtr(price *float64) *float64 {
	if price == nil {
		return nil
	}
	v := roundPricePerMillion(*price)
	return &v
}

// roundBindingPrices 成本价（五项 token 价与各段）按每百万 Token 4 位小数取整；搜索价按次计，不在此列。
func roundBindingPrices(b *ModelCatalogBinding) {
	b.InputPrice = roundPricePerMillion(b.InputPrice)
	b.OutputPrice = roundPricePerMillion(b.OutputPrice)
	b.CacheWritePrice = roundPricePtr(b.CacheWritePrice)
	b.CacheWrite1hPrice = roundPricePtr(b.CacheWrite1hPrice)
	b.CacheReadPrice = roundPricePtr(b.CacheReadPrice)
	for i := range b.Intervals {
		iv := &b.Intervals[i]
		iv.InputPrice = roundPricePtr(iv.InputPrice)
		iv.OutputPrice = roundPricePtr(iv.OutputPrice)
		iv.CacheWritePrice = roundPricePtr(iv.CacheWritePrice)
		iv.CacheWrite1hPrice = roundPricePtr(iv.CacheWrite1hPrice)
		iv.CacheReadPrice = roundPricePtr(iv.CacheReadPrice)
	}
}

// normalizeTimePricing 没有时段的忙闲时等于不分忙闲时，存成 nil。
func normalizeTimePricing(tp *TimePricing) *TimePricing {
	if tp == nil || len(tp.Periods) == 0 {
		return nil
	}
	cp := *tp
	cp.Timezone = strings.TrimSpace(cp.Timezone)
	cp.Periods = append([]TimePricingPeriod(nil), tp.Periods...)
	cp.ExcludeDates = normalizeExcludeDates(tp.ExcludeDates)
	return &cp
}

// normalizeExcludeDates 去空白、去重、按日期排好；一个都没有存 nil。
func normalizeExcludeDates(dates []string) []string {
	seen := make(map[string]struct{}, len(dates))
	out := make([]string, 0, len(dates))
	for _, raw := range dates {
		date := strings.TrimSpace(raw)
		if date == "" {
			continue
		}
		if _, dup := seen[date]; dup {
			continue
		}
		seen[date] = struct{}{}
		out = append(out, date)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}

// maxBindingUpstreamModelLength 与 262 号迁移的 VARCHAR(255) 一致。
const maxBindingUpstreamModelLength = 255

// normalizeBindingUpstreamModel 去掉首尾空白；与目录标识相同的名字存成空串（= 同名），
// 这样改了目录标识以外的东西不会让同名的行变成「改过名」。
func normalizeBindingUpstreamModel(entry *ModelCatalogEntry, name string) string {
	name = strings.TrimSpace(name)
	if entry != nil && name == entry.ModelID {
		return ""
	}
	return name
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

// sameOfficialPrices 两份条目的五项 token 价、联网搜索价、最高推理倍率、按次价、忙闲时与分段 / 分档是否一致（按起点比，忽略 ID 与排序号）。
func sameOfficialPrices(a, b *ModelCatalogEntry) bool {
	pairs := [][2]*float64{
		{a.InputPrice, b.InputPrice},
		{a.OutputPrice, b.OutputPrice},
		{a.CacheWritePrice, b.CacheWritePrice},
		{a.CacheWrite1hPrice, b.CacheWrite1hPrice},
		{a.CacheReadPrice, b.CacheReadPrice},
		{a.SearchPricePerCall, b.SearchPricePerCall},
		{a.XPostPrice, b.XPostPrice},
		{a.XUserPrice, b.XUserPrice},
		{a.MaxReasoningEffortMultiplier, b.MaxReasoningEffortMultiplier},
		{a.PerRequestPrice, b.PerRequestPrice},
	}
	for _, p := range pairs {
		if !samePricePtr(p[0], p[1]) {
			return false
		}
	}
	if !sameTimePricing(a.TimePricing, b.TimePricing) {
		return false
	}
	as, bs := normalizePriceSegments(a.Intervals), normalizePriceSegments(b.Intervals)
	if len(as) != len(bs) {
		return false
	}
	for i := range as {
		x, y := as[i], bs[i]
		if x.MinTokens != y.MinTokens || !sameIntPtr(x.MaxTokens, y.MaxTokens) || x.TierLabel != y.TierLabel ||
			!samePricePtr(x.InputPrice, y.InputPrice) || !samePricePtr(x.OutputPrice, y.OutputPrice) ||
			!samePricePtr(x.CacheWritePrice, y.CacheWritePrice) || !samePricePtr(x.CacheWrite1hPrice, y.CacheWrite1hPrice) ||
			!samePricePtr(x.CacheReadPrice, y.CacheReadPrice) || !samePricePtr(x.PerRequestPrice, y.PerRequestPrice) ||
			!samePricePtr(x.InputMultiplier, y.InputMultiplier) || !samePricePtr(x.OutputMultiplier, y.OutputMultiplier) ||
			!samePricePtr(x.CacheWriteMultiplier, y.CacheWriteMultiplier) || !samePricePtr(x.CacheReadMultiplier, y.CacheReadMultiplier) {
			return false
		}
	}
	return true
}

func samePricePtr(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func sameIntPtr(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
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

// ListPricingEntries 价格页的数据：全部条目（按 Token 计费的带五项价与分段，生图 / 视频 / 按次的带按次价与分档，
// 都带承接关系），直接读库，不走热路径快照。
func (s *ModelCatalogService) ListPricingEntries(ctx context.Context) ([]ModelCatalogEntry, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	return s.repo.ListEntries(ctx)
}

// SaveEntryPricing 价格页按模型保存一块：官方价（输入 / 输出必填；按次计价的条目是按次价必填）+ 这个模型的全部承接关系（整份覆盖），
// 同一事务；全部校验通过才写库，然后失效目录快照。返回保存后的条目（带承接关系）。
func (s *ModelCatalogService) SaveEntryPricing(ctx context.Context, entryID int64, official OfficialPrices, sale CatalogSalePrices, bindings []ModelCatalogBinding, accounts CatalogBindingAccountSource) (*ModelCatalogEntry, error) {
	if s == nil || s.repo == nil {
		return nil, ErrModelCatalogEntryNotFound
	}
	current, err := s.repo.GetEntryByID(ctx, entryID)
	if err != nil {
		return nil, err
	}
	entry := current.Clone()
	if entry.IsPerRequestBilled() {
		if err := applyOfficialPerRequestPrices(entry, official); err != nil {
			return nil, err
		}
	} else if err := applyOfficialTokenPrices(entry, official); err != nil {
		return nil, err
	}
	// 售价整份覆盖（同一块一起保存）；改售价不改条目归属：官方价照旧跟着价格文件刷新，售价播种不碰。
	entry.SalePrices = normalizeSalePrices(sale)
	// 官方价（含忙闲时）真改了才算运营者定价：种子不再刷新它；只加 / 改承接渠道、改售价时保持原来的归属。
	if !sameOfficialPrices(current, entry) {
		entry.ManagedBy = ModelCatalogManagedByAdmin
	}
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
		normalizePricingBinding(entry, &binding)
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
		normalizePricingBinding(entry, &binding)
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

// applyOfficialTokenPrices 按 Token 计费的条目：五项价（输入 / 输出必填）、按 Token 分段、搜索价、忙闲时与最高推理倍率整份覆盖。
func applyOfficialTokenPrices(entry *ModelCatalogEntry, official OfficialPrices) error {
	if official.InputPrice == nil || official.OutputPrice == nil {
		return catalogValidationError("official input_price and output_price are required")
	}
	if err := validatePriceSegments("official", official.Intervals); err != nil {
		return err
	}
	entry.InputPrice = clonePricePtr(official.InputPrice)
	entry.OutputPrice = clonePricePtr(official.OutputPrice)
	entry.CacheWritePrice = clonePricePtr(official.CacheWritePrice)
	entry.CacheWrite1hPrice = clonePricePtr(official.CacheWrite1hPrice)
	entry.CacheReadPrice = clonePricePtr(official.CacheReadPrice)
	entry.Intervals = normalizePriceSegments(official.Intervals)
	entry.SearchPricePerCall = clonePricePtr(official.SearchPricePerCall)
	entry.XPostPrice = clonePricePtr(official.XPostPrice)
	entry.XUserPrice = clonePricePtr(official.XUserPrice)
	entry.TimePricing = normalizeTimePricing(official.TimePricing)
	entry.MaxReasoningEffortMultiplier = clonePricePtr(official.MaxReasoningEffortMultiplier)
	return nil
}

// applyOfficialPerRequestPrices 按次计价的条目：价格页只改按次价（必填）与分档（整份覆盖，档位名由 entry.Validate 按模式校验）。
// 条目上照抄的 token 价（同一模型经对话入口、没有出图时按 token 算）、搜索价、忙闲时、最高推理倍率不在价格页上，保持原样。
func applyOfficialPerRequestPrices(entry *ModelCatalogEntry, official OfficialPrices) error {
	if official.PerRequestPrice == nil {
		return catalogValidationError(fmt.Sprintf("official per_request_price is required: %s is billed per request", entry.ModelID))
	}
	tiers := make([]PricingInterval, len(official.Intervals))
	for i, iv := range official.Intervals {
		if iv.PerRequestPrice == nil {
			return catalogValidationError(fmt.Sprintf("official tier %d has no per_request_price", i+1))
		}
		if iv.InputPrice != nil || iv.OutputPrice != nil || iv.CacheWritePrice != nil || iv.CacheWrite1hPrice != nil || iv.CacheReadPrice != nil ||
			iv.InputMultiplier != nil || iv.OutputMultiplier != nil || iv.CacheWriteMultiplier != nil || iv.CacheReadMultiplier != nil {
			return catalogValidationError(fmt.Sprintf("official tier %d must only have a per_request_price", i+1))
		}
		tiers[i] = PricingInterval{
			MinTokens: iv.MinTokens, MaxTokens: iv.MaxTokens, TierLabel: strings.TrimSpace(iv.TierLabel),
			PerRequestPrice: clonePricePtr(iv.PerRequestPrice), SortOrder: i,
		}
	}
	entry.PerRequestPrice = clonePricePtr(official.PerRequestPrice)
	entry.Intervals = tiers
	return nil
}

// normalizePricingBinding 保存前把承接规整好：上游模型名、分段排序、成本价取整、忙闲时；
// 按 Token 计费的模型不用上游按次价，请求里带了也丢掉。
func normalizePricingBinding(entry *ModelCatalogEntry, b *ModelCatalogBinding) {
	b.UpstreamModel = normalizeBindingUpstreamModel(entry, b.UpstreamModel)
	b.Intervals = normalizePriceSegments(b.Intervals)
	roundBindingPrices(b)
	b.TimePricing = normalizeTimePricing(b.TimePricing)
	if !entry.IsPerRequestBilled() {
		b.PerRequestPrice = nil
	}
}

// sameTimePricing 两份忙闲时是否一样（nil 与没有时段的一样，都是不分忙闲时）。
func sameTimePricing(a, b *TimePricing) bool {
	a, b = normalizeTimePricing(a), normalizeTimePricing(b)
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if a.Timezone != b.Timezone || a.WeekdaysOnly != b.WeekdaysOnly || len(a.Periods) != len(b.Periods) ||
		strings.Join(a.ExcludeDates, ",") != strings.Join(b.ExcludeDates, ",") {
		return false
	}
	for i := range a.Periods {
		if a.Periods[i] != b.Periods[i] {
			return false
		}
	}
	return true
}
