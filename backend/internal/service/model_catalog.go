package service

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 目录条目的上架状态。本阶段只记录，不参与准入与列表过滤。
const (
	ModelCatalogStatusListed   = "listed"
	ModelCatalogStatusUnlisted = "unlisted"
)

// 目录条目 / 别名的维护方。
//   - seed：由播种器从价格文件 + 硬编码兜底价生成，重复播种会刷新；
//   - admin：管理员改过，播种器永不覆盖。
const (
	ModelCatalogManagedBySeed  = "seed"
	ModelCatalogManagedByAdmin = "admin"
)

// 别名来源。
const (
	ModelCatalogAliasSourceManual = "manual"
	ModelCatalogAliasSourceSeed   = "seed"
)

// 目录条目支持的上游协议。
const (
	ModelCatalogProtocolAnthropic       = "anthropic"
	ModelCatalogProtocolChatCompletions = "chat_completions"
	ModelCatalogProtocolResponses       = "responses"
	ModelCatalogProtocolGemini          = "gemini"
)

var (
	// ErrModelCatalogEntryNotFound 目录条目不存在。
	ErrModelCatalogEntryNotFound = infraerrors.NotFound("MODEL_CATALOG_ENTRY_NOT_FOUND", "model catalog entry not found")
	// ErrModelCatalogEntryExists 模型标识已存在（大小写不敏感）。
	ErrModelCatalogEntryExists = infraerrors.Conflict("MODEL_CATALOG_ENTRY_EXISTS", "model catalog entry already exists")
	// ErrModelCatalogAliasNotFound 别名不存在。
	ErrModelCatalogAliasNotFound = infraerrors.NotFound("MODEL_CATALOG_ALIAS_NOT_FOUND", "model catalog alias not found")
	// ErrModelCatalogAliasExists 别名已被占用（大小写不敏感，全局唯一）。
	ErrModelCatalogAliasExists = infraerrors.Conflict("MODEL_CATALOG_ALIAS_EXISTS", "model catalog alias already exists")
)

// ModelCatalogAlias 是指向某个目录条目的别名。
// Alias 以 "*" 结尾表示前缀模式，例如 "claude-3-5-sonnet-*"。
type ModelCatalogAlias struct {
	ID        int64     `json:"id"`
	EntryID   int64     `json:"entry_id"`
	Alias     string    `json:"alias"`
	Source    string    `json:"source"`
	Notes     *string   `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ModelCatalogEntry 是模型目录里的一个模型。
//
// 价格字段全部可空：nil 表示「本条目不配置这一项」，计费时沿用价格文件 /
// 硬编码兜底价里的同名项。这与渠道价卡的指针语义一致，但目录是**基准价**，
// 不做渠道那种「未配置即归零」的破坏性覆盖——否则没配图片价的条目会把图片
// token 静默算成免费。
type ModelCatalogEntry struct {
	ID          int64       `json:"id"`
	ModelID     string      `json:"model_id"`
	DisplayName string      `json:"display_name"`
	Vendor      string      `json:"vendor"`
	Protocols   []string    `json:"protocols"`
	BillingMode BillingMode `json:"billing_mode"`
	Status      string      `json:"status"`
	ManagedBy   string      `json:"managed_by"`

	InputPrice          *float64 `json:"input_price"`
	OutputPrice         *float64 `json:"output_price"`
	CacheWritePrice     *float64 `json:"cache_write_price"`
	CacheWrite1hPrice   *float64 `json:"cache_write_1h_price"`
	CacheReadPrice      *float64 `json:"cache_read_price"`
	ImageInputPrice     *float64 `json:"image_input_price"`
	ImageOutputPrice    *float64 `json:"image_output_price"`
	ImageCacheReadPrice *float64 `json:"image_cache_read_price"`

	InputPricePriority      *float64 `json:"input_price_priority"`
	OutputPricePriority     *float64 `json:"output_price_priority"`
	CacheWritePricePriority *float64 `json:"cache_write_price_priority"`
	CacheReadPricePriority  *float64 `json:"cache_read_price_priority"`

	PerRequestPrice *float64 `json:"per_request_price"`

	LongContextInputThreshold     *int     `json:"long_context_input_threshold"`
	LongContextThresholdInclusive bool     `json:"long_context_threshold_inclusive"`
	LongContextInputMultiplier    *float64 `json:"long_context_input_multiplier"`
	LongContextOutputMultiplier   *float64 `json:"long_context_output_multiplier"`

	FastMultiplier               *float64 `json:"fast_multiplier"`
	FlexMultiplier               *float64 `json:"flex_multiplier"`
	MaxReasoningEffortMultiplier *float64 `json:"max_reasoning_effort_multiplier"`

	Notes *string `json:"notes,omitempty"`

	Intervals   []PricingInterval   `json:"intervals"`
	TimePricing *ChannelTimePricing `json:"time_pricing,omitempty"`
	Aliases     []ModelCatalogAlias `json:"aliases"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NormalizeModelCatalogKey 返回查表用的规范化模型名。
// 与渠道定价查表同口径（小写 + claude-* 的 "." → "-"），保证从渠道价卡迁到
// 目录之后 "claude-opus-4.5" 与 "claude-opus-4-5" 仍然命中同一条。
func NormalizeModelCatalogKey(model string) string {
	return normalizeChannelPricingModelName(model)
}

// Clone 返回深拷贝，调用方改动不会污染服务内的缓存快照。
func (e *ModelCatalogEntry) Clone() *ModelCatalogEntry {
	if e == nil {
		return nil
	}
	cp := *e
	if e.Protocols != nil {
		cp.Protocols = append([]string(nil), e.Protocols...)
	}
	if e.Intervals != nil {
		cp.Intervals = append([]PricingInterval(nil), e.Intervals...)
	}
	if e.Aliases != nil {
		cp.Aliases = append([]ModelCatalogAlias(nil), e.Aliases...)
	}
	if e.TimePricing != nil {
		tp := ChannelTimePricing{
			Timezone:     e.TimePricing.Timezone,
			WeekdaysOnly: e.TimePricing.WeekdaysOnly,
		}
		if e.TimePricing.Periods != nil {
			tp.Periods = append([]ChannelTimePricingPeriod(nil), e.TimePricing.Periods...)
		}
		cp.TimePricing = &tp
	}
	return &cp
}

// EffectiveBillingMode 返回归一化后的计费模式（空值按 token 处理）。
func (e *ModelCatalogEntry) EffectiveBillingMode() BillingMode {
	if e == nil || e.BillingMode == "" {
		return BillingModeToken
	}
	return e.BillingMode
}

// IsOperatorAuthored 报告该条目的价格是不是管理员写的。
// 播种出来的条目是平台默认价卡，和价格文件同源，不算运营者定价。
func (e *ModelCatalogEntry) IsOperatorAuthored() bool {
	return e != nil && e.ManagedBy == ModelCatalogManagedByAdmin
}

// PricingCard 把目录条目投影成共享的价卡结构，供区间匹配、显式字段判定与分时
// 倍率复用同一套代码。只投影 ChannelModelPricing 已有的字段；目录独有的字段
// （priority 价、长上下文、图片缓存读价）由 ApplyToModelPricing 直接写进 ModelPricing。
func (e *ModelCatalogEntry) PricingCard() *ChannelModelPricing {
	if e == nil {
		return nil
	}
	card := &ChannelModelPricing{
		Models:                       []string{e.ModelID},
		BillingMode:                  e.EffectiveBillingMode(),
		InputPrice:                   e.InputPrice,
		OutputPrice:                  e.OutputPrice,
		CacheWritePrice:              e.CacheWritePrice,
		CacheWrite1hPrice:            e.CacheWrite1hPrice,
		CacheReadPrice:               e.CacheReadPrice,
		FastMultiplier:               e.FastMultiplier,
		FlexMultiplier:               e.FlexMultiplier,
		MaxReasoningEffortMultiplier: e.MaxReasoningEffortMultiplier,
		ImageInputPrice:              e.ImageInputPrice,
		ImageOutputPrice:             e.ImageOutputPrice,
		PerRequestPrice:              e.PerRequestPrice,
		TimePricing:                  e.TimePricing,
	}
	if e.Intervals != nil {
		card.Intervals = append([]PricingInterval(nil), e.Intervals...)
	}
	return card
}

// HasAnyTokenPrice 报告条目是否配置了任何一项 token 价格（含区间）。
// 全都没有时目录不构成价格来源，解析结果保持「无价」，由计费链路走既有的
// ErrModelPricingUnavailable 路径——不能悄悄变成一份全 0 的价卡。
func (e *ModelCatalogEntry) HasAnyTokenPrice() bool {
	if e == nil {
		return false
	}
	for _, p := range []*float64{
		e.InputPrice, e.OutputPrice, e.CacheWritePrice, e.CacheWrite1hPrice, e.CacheReadPrice,
		e.ImageInputPrice, e.ImageOutputPrice, e.ImageCacheReadPrice,
		e.InputPricePriority, e.OutputPricePriority, e.CacheWritePricePriority, e.CacheReadPricePriority,
	} {
		if p != nil {
			return true
		}
	}
	return len(filterValidIntervals(e.Intervals)) > 0
}

// ApplyToModelPricing 把目录条目叠加到基准价卡上。
//
// 只写非 nil 的项：目录是基准价，没配的项应当沿用价格文件里的值，而不是像渠道
// 价卡那样「未配置即归零」。
func (e *ModelCatalogEntry) ApplyToModelPricing(pricing *ModelPricing) {
	if e == nil || pricing == nil {
		return
	}

	setStandardAndPriority := func(standard, priority *float64, std, prio *float64) {
		// standard/priority 是目标字段地址，std/prio 是目录里的值。
		if std != nil {
			// 目录没写 priority 价时，沿用「基准价比例」推出来的档位价，
			// 与渠道覆盖同口径（channelTierOverridePrice）。
			derived := channelTierOverridePrice(*standard, *priority, *std)
			*standard = *std
			*priority = derived
		}
		if prio != nil {
			*priority = *prio
		}
	}

	setStandardAndPriority(&pricing.InputPricePerToken, &pricing.InputPricePerTokenPriority, e.InputPrice, e.InputPricePriority)
	setStandardAndPriority(&pricing.OutputPricePerToken, &pricing.OutputPricePerTokenPriority, e.OutputPrice, e.OutputPricePriority)
	setStandardAndPriority(&pricing.CacheReadPricePerToken, &pricing.CacheReadPricePerTokenPriority, e.CacheReadPrice, e.CacheReadPricePriority)

	if e.CacheWritePrice != nil {
		derived := channelTierOverridePrice(pricing.CacheCreationPricePerToken, pricing.CacheCreationPricePerTokenPriority, *e.CacheWritePrice)
		pricing.CacheCreationPricePerToken = *e.CacheWritePrice
		pricing.CacheCreationPricePerTokenPriority = derived
		pricing.CacheCreationPriceExplicit = true
		pricing.CacheCreation5mPrice = *e.CacheWritePrice
		if e.CacheWrite1hPrice == nil {
			pricing.CacheCreation1hPrice = *e.CacheWritePrice
		}
	}
	if e.CacheWritePricePriority != nil {
		pricing.CacheCreationPricePerTokenPriority = *e.CacheWritePricePriority
	}
	if e.CacheWrite1hPrice != nil {
		pricing.CacheCreation1hPrice = *e.CacheWrite1hPrice
	}
	// 5m/1h 分档的判定口径与价格文件一致：1h 价存在且严格高于 5m 价才分档。
	// 数据写反时按不分档处理，避免把 1h 缓存按更低的价算。
	if e.CacheWrite1hPrice != nil || e.CacheWritePrice != nil {
		pricing.SupportsCacheBreakdown = pricing.CacheCreation1hPrice > 0 &&
			pricing.CacheCreation1hPrice > pricing.CacheCreation5mPrice
	}

	if e.ImageInputPrice != nil {
		pricing.ImageInputPricePerToken = *e.ImageInputPrice
	}
	if e.ImageOutputPrice != nil {
		pricing.ImageOutputPricePerToken = *e.ImageOutputPrice
		pricing.ImageOutputPriceExplicit = true
	}
	if e.ImageCacheReadPrice != nil {
		pricing.ImageCacheReadPricePerToken = *e.ImageCacheReadPrice
	}

	if e.LongContextInputThreshold != nil {
		pricing.LongContextInputThreshold = *e.LongContextInputThreshold
		pricing.LongContextThresholdInclusive = e.LongContextThresholdInclusive
	}
	if e.LongContextInputMultiplier != nil {
		pricing.LongContextInputMultiplier = *e.LongContextInputMultiplier
	}
	if e.LongContextOutputMultiplier != nil {
		pricing.LongContextOutputMultiplier = *e.LongContextOutputMultiplier
	}

	if e.FastMultiplier != nil {
		pricing.FastMultiplier = e.FastMultiplier
	}
	if e.FlexMultiplier != nil {
		pricing.FlexMultiplier = e.FlexMultiplier
	}
	if e.MaxReasoningEffortMultiplier != nil {
		pricing.MaxReasoningEffortMultiplier = e.MaxReasoningEffortMultiplier
	}
}

// Normalize 把条目归一化到可持久化的形状（去空白、补默认值、排序区间）。
func (e *ModelCatalogEntry) Normalize() {
	if e == nil {
		return
	}
	e.ModelID = strings.TrimSpace(e.ModelID)
	e.DisplayName = strings.TrimSpace(e.DisplayName)
	e.Vendor = strings.ToLower(strings.TrimSpace(e.Vendor))
	if e.BillingMode == "" {
		e.BillingMode = BillingModeToken
	}
	if e.Status == "" {
		e.Status = ModelCatalogStatusListed
	}
	if e.ManagedBy == "" {
		e.ManagedBy = ModelCatalogManagedBySeed
	}
	e.Protocols = normalizeModelCatalogProtocols(e.Protocols)
	sort.SliceStable(e.Intervals, func(i, j int) bool {
		if e.Intervals[i].SortOrder != e.Intervals[j].SortOrder {
			return e.Intervals[i].SortOrder < e.Intervals[j].SortOrder
		}
		return e.Intervals[i].MinTokens < e.Intervals[j].MinTokens
	})
	if e.TimePricing != nil && len(e.TimePricing.Periods) == 0 {
		e.TimePricing = nil
	}
}

// Validate 校验条目字段，返回第一处错误。
func (e *ModelCatalogEntry) Validate() error {
	if e == nil {
		return catalogValidationError("nil model catalog entry")
	}
	if e.ModelID == "" {
		return catalogValidationError("model_id is required")
	}
	// 长度上限与 243 号迁移里的 VARCHAR(n) 一致，按字符数计（与 PostgreSQL 口径相同）。
	for _, column := range []struct {
		name  string
		value string
		max   int
	}{
		{"model_id", e.ModelID, 200},
		{"display_name", e.DisplayName, 200},
		{"vendor", e.Vendor, 50},
	} {
		if err := validateCatalogLength(column.name, column.value, column.max); err != nil {
			return err
		}
	}
	for i := range e.Intervals {
		if err := validateCatalogLength(fmt.Sprintf("intervals[%d].tier_label", i), e.Intervals[i].TierLabel, 50); err != nil {
			return err
		}
	}
	if e.TimePricing != nil {
		if err := validateCatalogLength("time_pricing.timezone", e.TimePricing.Timezone, 64); err != nil {
			return err
		}
	}
	if !e.BillingMode.IsValid() {
		return catalogValidationError(fmt.Sprintf("invalid billing_mode: %s", e.BillingMode))
	}
	switch e.Status {
	case ModelCatalogStatusListed, ModelCatalogStatusUnlisted:
	default:
		return catalogValidationError(fmt.Sprintf("invalid status: %s", e.Status))
	}
	switch e.ManagedBy {
	case ModelCatalogManagedBySeed, ModelCatalogManagedByAdmin:
	default:
		return catalogValidationError(fmt.Sprintf("invalid managed_by: %s", e.ManagedBy))
	}
	for _, protocol := range e.Protocols {
		switch protocol {
		case ModelCatalogProtocolAnthropic, ModelCatalogProtocolChatCompletions,
			ModelCatalogProtocolResponses, ModelCatalogProtocolGemini:
		default:
			return catalogValidationError(fmt.Sprintf("invalid protocol: %s", protocol))
		}
	}
	prices := map[string]*float64{
		"input_price":                e.InputPrice,
		"output_price":               e.OutputPrice,
		"cache_write_price":          e.CacheWritePrice,
		"cache_write_1h_price":       e.CacheWrite1hPrice,
		"cache_read_price":           e.CacheReadPrice,
		"image_input_price":          e.ImageInputPrice,
		"image_output_price":         e.ImageOutputPrice,
		"image_cache_read_price":     e.ImageCacheReadPrice,
		"input_price_priority":       e.InputPricePriority,
		"output_price_priority":      e.OutputPricePriority,
		"cache_write_price_priority": e.CacheWritePricePriority,
		"cache_read_price_priority":  e.CacheReadPricePriority,
		"per_request_price":          e.PerRequestPrice,
	}
	for _, name := range sortedPriceFieldNames(prices) {
		if value := prices[name]; value != nil && *value < 0 {
			return catalogValidationError(fmt.Sprintf("%s must be >= 0", name))
		}
	}
	multipliers := map[string]*float64{
		"long_context_input_multiplier":   e.LongContextInputMultiplier,
		"long_context_output_multiplier":  e.LongContextOutputMultiplier,
		"fast_multiplier":                 e.FastMultiplier,
		"flex_multiplier":                 e.FlexMultiplier,
		"max_reasoning_effort_multiplier": e.MaxReasoningEffortMultiplier,
	}
	for _, name := range sortedPriceFieldNames(multipliers) {
		if value := multipliers[name]; value != nil && *value <= 0 {
			return catalogValidationError(fmt.Sprintf("%s must be > 0", name))
		}
	}
	if e.LongContextInputThreshold != nil && *e.LongContextInputThreshold < 0 {
		return catalogValidationError("long_context_input_threshold must be >= 0")
	}
	if err := ValidateIntervals(e.Intervals, e.EffectiveBillingMode()); err != nil {
		return catalogValidationError(err.Error())
	}
	// 分时倍率复用渠道那一套校验（时区、HH:mm(:ss) 解析、倍率精度、时段不重叠），
	// 保证目录与渠道的分时语义一致。
	if err := validateChannelTimePricing(e.TimePricing); err != nil {
		return catalogValidationError(fmt.Sprintf("time_pricing: %s", err.Error()))
	}
	return nil
}

func catalogValidationError(message string) error {
	return infraerrors.BadRequest("MODEL_CATALOG_INVALID", message)
}

func sortedPriceFieldNames(m map[string]*float64) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func normalizeModelCatalogProtocols(protocols []string) []string {
	if len(protocols) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(protocols))
	out := make([]string, 0, len(protocols))
	for _, protocol := range protocols {
		protocol = strings.ToLower(strings.TrimSpace(protocol))
		if protocol == "" {
			continue
		}
		if _, ok := seen[protocol]; ok {
			continue
		}
		seen[protocol] = struct{}{}
		out = append(out, protocol)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// NormalizeModelCatalogAlias 归一化别名（去空白）。别名大小写不敏感唯一，
// 但保留管理员输入的原始大小写用于展示。
func NormalizeModelCatalogAlias(alias string) string {
	return strings.TrimSpace(alias)
}

func validateCatalogLength(name, value string, max int) error {
	if utf8.RuneCountInString(value) > max {
		return catalogValidationError(fmt.Sprintf("%s must be at most %d characters", name, max))
	}
	return nil
}

// ValidateModelCatalogAlias 校验别名。
func ValidateModelCatalogAlias(alias, source string) error {
	alias = NormalizeModelCatalogAlias(alias)
	if alias == "" {
		return catalogValidationError("alias is required")
	}
	if err := validateCatalogLength("alias", alias, 200); err != nil {
		return err
	}
	// "*" 只允许出现在末尾，且不能是单独一个 "*"：全量通配会让任意模型名都拿到
	// 同一份价卡，等于关掉「查不到价」这个信号。
	if star := strings.Index(alias, "*"); star >= 0 {
		if star != len(alias)-1 {
			return catalogValidationError("alias wildcard '*' is only allowed as the last character")
		}
		if star == 0 {
			return catalogValidationError("alias must not be a bare wildcard")
		}
	}
	switch source {
	case ModelCatalogAliasSourceManual, ModelCatalogAliasSourceSeed, "":
		return nil
	default:
		return catalogValidationError(fmt.Sprintf("invalid alias source: %s", source))
	}
}
