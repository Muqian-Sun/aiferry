package service

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 目录条目的上架状态：listed 的条目用户可见且可调用（准入只放行 listed），
// unlisted 的条目对用户不存在。播种出来的条目默认 unlisted，由管理员绑定资源后上架。
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
	// ErrModelCatalogBindingAccountNotFound 绑定引用的账号不存在。
	ErrModelCatalogBindingAccountNotFound = infraerrors.NotFound("MODEL_CATALOG_BINDING_ACCOUNT_NOT_FOUND", "binding account not found")
)

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
	// AudioInputPrice / AudioOutputPrice 是音频 token 价；nil 时音频 token 按文本输入 / 输出价计。
	AudioInputPrice  *float64 `json:"audio_input_price"`
	AudioOutputPrice *float64 `json:"audio_output_price"`

	PerRequestPrice *float64 `json:"per_request_price"`
	// SearchPricePerCall 官方每次 web 搜索价（联网搜索费，见 web_search_usage.go）；nil 表示用厂商公开价。
	SearchPricePerCall *float64 `json:"search_price_per_call"`
	// XPostPrice / XUserPrice xAI X 搜索按取回条目收的官方价（每条帖子、每个主页）；nil 表示用 xAI 公开价。
	XPostPrice *float64 `json:"x_post_price"`
	XUserPrice *float64 `json:"x_user_price"`

	MaxReasoningEffortMultiplier *float64 `json:"max_reasoning_effort_multiplier"`

	// SalePrices 我们自己定的售价（基础价五项 + 各段，USD / token）；没填的项按官方价 × DefaultSalePriceRatio 收。
	// 只有价格页保存售价时写（SaveEntryPricing），播种刷新、编辑模型都不碰它。
	SalePrices CatalogSalePrices `json:"sale_prices"`

	Notes *string `json:"notes,omitempty"`

	Intervals   []PricingInterval     `json:"intervals"`
	TimePricing *TimePricing          `json:"time_pricing,omitempty"`
	Bindings    []ModelCatalogBinding `json:"bindings"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ModelCatalogBinding 是承接关系：一个渠道（账号）承接一个目录条目，带这个渠道给这个模型的上游价
// （USD / token）。渠道成本 = 用量 × 上游价，算法与官方价相同（分段、缓存 5 分钟 / 1 小时、最高推理倍率）。
// 输入 / 输出必填；官方价有的缓存项上游价也必须填（ValidateAgainst）；Intervals 是按 Token 分段的上游价，
// 只用绝对价。UpstreamModel 是这个渠道给这个模型用的上游模型名，空 = 与目录标识同名：转发时
// 目录标识 → 上游名只转换这一次（D4，muqian 2026-10-01）。
type ModelCatalogBinding struct {
	EntryID           int64             `json:"entry_id"`
	AccountID         int64             `json:"account_id"`
	UpstreamModel     string            `json:"upstream_model"`
	InputPrice        float64           `json:"input_price"`
	OutputPrice       float64           `json:"output_price"`
	CacheWritePrice   *float64          `json:"cache_write_price"`
	CacheWrite1hPrice *float64          `json:"cache_write_1h_price"`
	CacheReadPrice    *float64          `json:"cache_read_price"`
	Intervals         []PricingInterval `json:"intervals"`
	// 联网搜索的上游价（USD / 次、/ 条）：官方价显式设了的项必须填；没填的按官方搜索价记成本。
	SearchPricePerCall *float64  `json:"search_price_per_call"`
	XPostPrice         *float64  `json:"x_post_price"`
	XUserPrice         *float64  `json:"x_user_price"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// NormalizeModelCatalogKey 返回查表用的规范化模型名。
// 与渠道定价查表同口径（小写 + claude-* 的 "." → "-"），保证从渠道价卡迁到
// 目录之后 "claude-opus-4.5" 与 "claude-opus-4-5" 仍然命中同一条。
func NormalizeModelCatalogKey(model string) string {
	return normalizePricingModelName(model)
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
	if e.SalePrices.Segments != nil {
		cp.SalePrices.Segments = append([]CatalogSaleSegment(nil), e.SalePrices.Segments...)
	}
	if e.Bindings != nil {
		cp.Bindings = append([]ModelCatalogBinding(nil), e.Bindings...)
	}
	if e.TimePricing != nil {
		tp := TimePricing{
			Timezone:     e.TimePricing.Timezone,
			WeekdaysOnly: e.TimePricing.WeekdaysOnly,
		}
		if e.TimePricing.Periods != nil {
			tp.Periods = append([]TimePricingPeriod(nil), e.TimePricing.Periods...)
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
// 倍率复用同一套代码。只投影 PricingCard 已有的字段；目录独有的字段
// （图片缓存读价、音频价等）由 ApplyToModelPricing 直接写进 ModelPricing。
func (e *ModelCatalogEntry) PricingCard() *PricingCard {
	if e == nil {
		return nil
	}
	card := &PricingCard{
		Models:                       []string{e.ModelID},
		BillingMode:                  e.EffectiveBillingMode(),
		InputPrice:                   e.InputPrice,
		OutputPrice:                  e.OutputPrice,
		CacheWritePrice:              e.CacheWritePrice,
		CacheWrite1hPrice:            e.CacheWrite1hPrice,
		CacheReadPrice:               e.CacheReadPrice,
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

// HasPrice 报告条目能否独立定出价：token 模式要有任一 token 价；按次 / 图片 / 视频
// 模式要有按次价或有效分档。上架条目必须满足它——上架即可调用，无价的条目调用
// 后计费会走 ErrModelPricingUnavailable，用户看到的是能选却用不了的模型。
func (e *ModelCatalogEntry) HasPrice() bool {
	if e == nil {
		return false
	}
	switch e.EffectiveBillingMode() {
	case BillingModePerRequest, BillingModeImage, BillingModeVideo:
		return e.PerRequestPrice != nil || len(filterValidIntervals(e.Intervals)) > 0
	default:
		return e.HasAnyTokenPrice()
	}
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
		e.AudioInputPrice, e.AudioOutputPrice,
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

	if e.InputPrice != nil {
		pricing.InputPricePerToken = *e.InputPrice
	}
	if e.OutputPrice != nil {
		pricing.OutputPricePerToken = *e.OutputPrice
	}
	if e.CacheReadPrice != nil {
		pricing.CacheReadPricePerToken = *e.CacheReadPrice
	}

	if e.CacheWritePrice != nil {
		pricing.CacheCreationPricePerToken = *e.CacheWritePrice
		pricing.CacheCreationPriceExplicit = true
		pricing.CacheCreation5mPrice = *e.CacheWritePrice
		if e.CacheWrite1hPrice == nil {
			pricing.CacheCreation1hPrice = *e.CacheWritePrice
		}
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
	if e.AudioInputPrice != nil {
		pricing.AudioInputPricePerToken = *e.AudioInputPrice
	}
	if e.AudioOutputPrice != nil {
		pricing.AudioOutputPricePerToken = *e.AudioOutputPrice
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
		e.Status = ModelCatalogStatusUnlisted
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
	if e.Status == ModelCatalogStatusListed && !e.HasPrice() {
		return catalogValidationError("a listed entry must have a price")
	}
	// 图片 / 视频条目上架必须有默认按次价：计费路径没有「分档不命中就退默认价、默认价也没有」
	// 的兜底，缺了会把媒体用量按 token 价静默算。
	if e.Status == ModelCatalogStatusListed && e.PerRequestPrice == nil {
		switch e.EffectiveBillingMode() {
		case BillingModeImage, BillingModeVideo:
			return catalogValidationError("a listed image/video entry must have a per_request_price")
		}
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
		"input_price":            e.InputPrice,
		"output_price":           e.OutputPrice,
		"cache_write_price":      e.CacheWritePrice,
		"cache_write_1h_price":   e.CacheWrite1hPrice,
		"cache_read_price":       e.CacheReadPrice,
		"image_input_price":      e.ImageInputPrice,
		"image_output_price":     e.ImageOutputPrice,
		"image_cache_read_price": e.ImageCacheReadPrice,
		"audio_input_price":      e.AudioInputPrice,
		"audio_output_price":     e.AudioOutputPrice,
		"per_request_price":      e.PerRequestPrice,
		"search_price_per_call":  e.SearchPricePerCall,
		"x_post_price":           e.XPostPrice,
		"x_user_price":           e.XUserPrice,
	}
	for _, name := range sortedPriceFieldNames(prices) {
		if value := prices[name]; value != nil && *value < 0 {
			return catalogValidationError(fmt.Sprintf("%s must be >= 0", name))
		}
	}
	multipliers := map[string]*float64{
		"max_reasoning_effort_multiplier": e.MaxReasoningEffortMultiplier,
	}
	for _, name := range sortedPriceFieldNames(multipliers) {
		if value := multipliers[name]; value != nil && *value <= 0 {
			return catalogValidationError(fmt.Sprintf("%s must be > 0", name))
		}
	}
	if err := ValidateIntervals(e.Intervals, e.EffectiveBillingMode()); err != nil {
		return catalogValidationError(err.Error())
	}
	if err := validateSalePrices(e); err != nil {
		return err
	}
	// 分时倍率复用渠道那一套校验（时区、HH:mm(:ss) 解析、倍率精度、时段不重叠），
	// 保证目录与渠道的分时语义一致。
	if err := validateTimePricing(e.TimePricing); err != nil {
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

func validateCatalogLength(name, value string, max int) error {
	if utf8.RuneCountInString(value) > max {
		return catalogValidationError(fmt.Sprintf("%s must be at most %d characters", name, max))
	}
	return nil
}
