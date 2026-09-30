package service

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// 价卡：模型目录条目、分组价卡共用的定价结构与校验。
// 原先住在 channel.go / channel_service.go（渠道实体的一部分），PR-5a 搬出来，
// 渠道实体本身在 PR-5b 删除。

// BillingMode 计费模式
type BillingMode string

const (
	BillingModeToken      BillingMode = "token"       // 按 token 区间计费
	BillingModePerRequest BillingMode = "per_request" // 按次计费（支持上下文窗口分层）
	BillingModeImage      BillingMode = "image"       // 图片计费（当前按次，预留 token 计费）
	BillingModeVideo      BillingMode = "video"       // 视频生成计费（按视频生成次数）
)

// IsValid 检查 BillingMode 是否为合法值
func (m BillingMode) IsValid() bool {
	switch m {
	case BillingModeToken, BillingModePerRequest, BillingModeImage, BillingModeVideo, "":
		return true
	}
	return false
}

// IsValidUsageFilter 检查 BillingMode 是否可用于使用记录筛选。
func (m BillingMode) IsValidUsageFilter() bool {
	switch m {
	case BillingModeToken, BillingModePerRequest, BillingModeImage, BillingModeVideo, "":
		return true
	}
	return false
}

// PricingCard 一张价卡：一组模型（Models）的计费模式与各项单价。目录条目经 PricingCard()
// 投影成它，分组价卡直接存它。
type PricingCard struct {
	ID                           int64             `json:"id,omitempty"`
	Models                       []string          `json:"models"`
	BillingMode                  BillingMode       `json:"billing_mode"`
	InputPrice                   *float64          `json:"input_price"`
	OutputPrice                  *float64          `json:"output_price"`
	CacheWritePrice              *float64          `json:"cache_write_price"`
	CacheWrite1hPrice            *float64          `json:"cache_write_1h_price"`
	CacheReadPrice               *float64          `json:"cache_read_price"`
	MaxReasoningEffortMultiplier *float64          `json:"max_reasoning_effort_multiplier"`
	ImageInputPrice              *float64          `json:"image_input_price"`
	ImageOutputPrice             *float64          `json:"image_output_price"`
	PerRequestPrice              *float64          `json:"per_request_price"`
	SearchPricePerCall           *float64          `json:"search_price_per_call,omitempty"`
	Intervals                    []PricingInterval `json:"intervals"`
	TimePricing                  *TimePricing      `json:"time_pricing,omitempty"`
	CreatedAt                    time.Time         `json:"created_at,omitempty"`
	UpdatedAt                    time.Time         `json:"updated_at,omitempty"`
}

// TimePricing 价卡的分时倍率配置。
type TimePricing struct {
	Timezone     string              `json:"timezone"`
	WeekdaysOnly bool                `json:"weekdays_only,omitempty"`
	Periods      []TimePricingPeriod `json:"periods"`
}

// TimePricingPeriod 是秒级的左闭右开分时倍率区间，并兼容历史 HH:mm 数据。
type TimePricingPeriod struct {
	StartTime  string  `json:"start_time"`
	EndTime    string  `json:"end_time"`
	Multiplier float64 `json:"multiplier"`
}

// PricingInterval 定价区间（token 区间 / 按次分层 / 图片分辨率分层）
type PricingInterval struct {
	ID                   int64     `json:"id,omitempty"`
	PricingID            int64     `json:"pricing_id,omitempty"`
	MinTokens            int       `json:"min_tokens"`
	MaxTokens            *int      `json:"max_tokens"`
	TierLabel            string    `json:"tier_label"`
	InputPrice           *float64  `json:"input_price"`
	OutputPrice          *float64  `json:"output_price"`
	CacheWritePrice      *float64  `json:"cache_write_price"`
	CacheWrite1hPrice    *float64  `json:"cache_write_1h_price"`
	CacheReadPrice       *float64  `json:"cache_read_price"`
	InputMultiplier      *float64  `json:"input_multiplier"`
	OutputMultiplier     *float64  `json:"output_multiplier"`
	CacheWriteMultiplier *float64  `json:"cache_write_multiplier"`
	CacheReadMultiplier  *float64  `json:"cache_read_multiplier"`
	PerRequestPrice      *float64  `json:"per_request_price"`
	SortOrder            int       `json:"sort_order"`
	CreatedAt            time.Time `json:"created_at,omitempty"`
	UpdatedAt            time.Time `json:"updated_at,omitempty"`
}

// FindMatchingInterval 在区间列表中查找匹配 totalTokens 的区间。
// 区间为左开右闭 (min, max]：min 不含，max 包含。
// 第一个区间 min=0 时，0 token 不匹配任何区间（回退到默认价格）。
func FindMatchingInterval(intervals []PricingInterval, totalTokens int) *PricingInterval {
	for i := range intervals {
		iv := &intervals[i]
		if totalTokens > iv.MinTokens && (iv.MaxTokens == nil || totalTokens <= *iv.MaxTokens) {
			return iv
		}
	}
	return nil
}

// GetIntervalForContext 根据总 context token 数查找匹配的区间。
func (p *PricingCard) GetIntervalForContext(totalTokens int) *PricingInterval {
	return FindMatchingInterval(p.Intervals, totalTokens)
}

// GetTierByLabel 根据标签查找层级（用于 per_request / image 模式）
func (p *PricingCard) GetTierByLabel(label string) *PricingInterval {
	labelLower := strings.ToLower(label)
	for i := range p.Intervals {
		if strings.ToLower(p.Intervals[i].TierLabel) == labelLower {
			return &p.Intervals[i]
		}
	}
	return nil
}

// Clone 返回 PricingCard 的拷贝（切片独立，指针字段共享，调用方只读安全）
func (p PricingCard) Clone() PricingCard {
	cp := p
	if p.Models != nil {
		cp.Models = make([]string, len(p.Models))
		copy(cp.Models, p.Models)
	}
	if p.Intervals != nil {
		cp.Intervals = make([]PricingInterval, len(p.Intervals))
		copy(cp.Intervals, p.Intervals)
	}
	if p.TimePricing != nil {
		cp.TimePricing = &TimePricing{
			Timezone:     p.TimePricing.Timezone,
			WeekdaysOnly: p.TimePricing.WeekdaysOnly,
		}
		if p.TimePricing.Periods != nil {
			cp.TimePricing.Periods = append([]TimePricingPeriod(nil), p.TimePricing.Periods...)
		}
	}
	return cp
}

// ValidateIntervals 校验区间列表的合法性。
//
// mode 决定区间语义：
//   - BillingModeToken（含空值）：区间是上下文 token 数分段 (min, max]，
//     按 MinTokens 排序后无重叠，无界区间（MaxTokens=nil）必须是最后一个。
//   - BillingModeImage / BillingModeVideo：区间是按 tier_label 分档（图片按输出尺寸
//     1K/2K/4K，视频按分辨率 480p/720p/1080p），每档必须带 tier_label 与 per_request_price，
//     tier_label 同条目内唯一且只接受上述取值（计费查档区分大小写）。
//   - BillingModePerRequest：按 tier_label 分档，标签自由；跳过区间重叠与 last-unlimited 校验。
//
// 通用规则：MinTokens >= 0；MaxTokens 若非 nil 则 > 0 且 > MinTokens；
// 所有价格字段 >= 0。
func ValidateIntervals(intervals []PricingInterval, mode BillingMode) error {
	if len(intervals) == 0 {
		return nil
	}
	sorted := make([]PricingInterval, len(intervals))
	copy(sorted, intervals)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].MinTokens < sorted[j].MinTokens
	})

	for i := range sorted {
		if err := validateSingleInterval(&sorted[i], i); err != nil {
			return err
		}
	}

	switch mode {
	case BillingModeImage:
		return validateMediaTiers(intervals, mediaTierLabels(ImageBillingSize1K, ImageBillingSize2K, ImageBillingSize4K), "image")
	case BillingModeVideo:
		return validateMediaTiers(intervals, mediaTierLabels(VideoBillingResolution480P, VideoBillingResolution720P, VideoBillingResolution1080P), "video")
	case BillingModePerRequest:
		// 按 tier_label 匹配，不做 token 区间重叠校验
		return nil
	}
	return validateIntervalOverlap(sorted)
}

func mediaTierLabels(labels ...string) map[string]bool {
	out := make(map[string]bool, len(labels))
	for _, label := range labels {
		out[label] = true
	}
	return out
}

// validateMediaTiers 校验图片 / 视频分档：每档带合法 tier_label 与 per_request_price，标签不重复。
func validateMediaTiers(intervals []PricingInterval, allowed map[string]bool, mode string) error {
	seen := make(map[string]bool, len(intervals))
	for i := range intervals {
		label := intervals[i].TierLabel
		if label == "" {
			return fmt.Errorf("interval #%d: %s tier requires a tier_label", i+1, mode)
		}
		if !allowed[label] {
			return fmt.Errorf("interval #%d: unknown %s tier_label %q", i+1, mode, label)
		}
		if seen[label] {
			return fmt.Errorf("interval #%d: duplicate %s tier_label %q", i+1, mode, label)
		}
		seen[label] = true
		if intervals[i].PerRequestPrice == nil {
			return fmt.Errorf("interval #%d: %s tier %q requires a per_request_price", i+1, mode, label)
		}
	}
	return nil
}

// validateSingleInterval 校验单个区间的字段合法性
func validateSingleInterval(iv *PricingInterval, idx int) error {
	if iv.MinTokens < 0 {
		return fmt.Errorf("interval #%d: min_tokens (%d) must be >= 0", idx+1, iv.MinTokens)
	}
	if iv.MaxTokens != nil {
		if *iv.MaxTokens <= 0 {
			return fmt.Errorf("interval #%d: max_tokens (%d) must be > 0", idx+1, *iv.MaxTokens)
		}
		if *iv.MaxTokens <= iv.MinTokens {
			return fmt.Errorf("interval #%d: max_tokens (%d) must be > min_tokens (%d)",
				idx+1, *iv.MaxTokens, iv.MinTokens)
		}
	}
	return validateIntervalPrices(iv, idx)
}

// validateIntervalPrices 校验区间价格 >= 0、倍率 > 0。
func validateIntervalPrices(iv *PricingInterval, idx int) error {
	prices := []struct {
		name string
		val  *float64
	}{
		{"input_price", iv.InputPrice},
		{"output_price", iv.OutputPrice},
		{"cache_write_price", iv.CacheWritePrice},
		{"cache_write_1h_price", iv.CacheWrite1hPrice},
		{"cache_read_price", iv.CacheReadPrice},
		{"per_request_price", iv.PerRequestPrice},
	}
	for _, p := range prices {
		if p.val != nil && *p.val < 0 {
			return fmt.Errorf("interval #%d: %s must be >= 0", idx+1, p.name)
		}
	}
	multipliers := []struct {
		name string
		val  *float64
	}{
		{"input_multiplier", iv.InputMultiplier},
		{"output_multiplier", iv.OutputMultiplier},
		{"cache_write_multiplier", iv.CacheWriteMultiplier},
		{"cache_read_multiplier", iv.CacheReadMultiplier},
	}
	for _, multiplier := range multipliers {
		if multiplier.val != nil && *multiplier.val <= 0 {
			return fmt.Errorf("interval #%d: %s must be > 0", idx+1, multiplier.name)
		}
	}
	return nil
}

// validateIntervalOverlap 校验排序后的区间列表无重叠，且无界区间在最后
func validateIntervalOverlap(sorted []PricingInterval) error {
	for i, iv := range sorted {
		// 无界区间必须是最后一个
		if iv.MaxTokens == nil && i < len(sorted)-1 {
			return fmt.Errorf("interval #%d: unbounded interval (max_tokens=null) must be the last one",
				i+1)
		}
		if i == 0 {
			continue
		}
		prev := sorted[i-1]
		// 检查重叠：前一个区间的上界 > 当前区间的下界则重叠
		// (min, max] 语义：prev 覆盖 (prev.Min, prev.Max]，cur 覆盖 (cur.Min, cur.Max]
		if prev.MaxTokens == nil || *prev.MaxTokens > iv.MinTokens {
			return fmt.Errorf("interval #%d and #%d overlap: prev max=%s > cur min=%d",
				i, i+1, formatMaxTokensLabel(prev.MaxTokens), iv.MinTokens)
		}
	}
	return nil
}

func formatMaxTokensLabel(max *int) string {
	if max == nil {
		return "∞"
	}
	return fmt.Sprintf("%d", *max)
}

// wildcardSuffix 是模型模式中的通配符后缀标记（仅支持尾部匹配）。
const wildcardSuffix = "*"

// splitWildcardSuffix 将模型模式拆分为 (prefix, isWildcard)。
//
//	"claude-opus-*"  → ("claude-opus-", true)
//	"claude-opus-4"  → ("claude-opus-4", false)
//	"*"              → ("", true)
//
// 注意：返回的 prefix 保持原始大小写，由调用方按需 ToLower。
func splitWildcardSuffix(pattern string) (prefix string, isWildcard bool) {
	if strings.HasSuffix(pattern, wildcardSuffix) {
		return strings.TrimSuffix(pattern, wildcardSuffix), true
	}
	return pattern, false
}

// normalizePricingModelName makes Anthropic's dot and hyphen spelling
// differences equivalent in pricing lookups (channel cache keys and conflict detection).
func normalizePricingModelName(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	if strings.HasPrefix(model, "claude-") {
		model = strings.ReplaceAll(model, ".", "-")
	}
	return model
}
