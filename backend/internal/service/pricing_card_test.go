//go:build unit

package service

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

// 以下价卡校验原本给分组价卡（normalizeGroupModelPricing）用，分组删掉后生产路径已不调用，
// 只剩本文件（unit 标签）的用例在测；放在无标签的 pricing_card.go 里会被默认标签下的 lint 判成未使用。

func validatePricingTimePricing(pricing []PricingCard) error {
	for i := range pricing {
		config := pricing[i].TimePricing
		if config == nil {
			continue
		}
		if len(config.Periods) == 0 {
			pricing[i].TimePricing = nil
			continue
		}
		mode := pricing[i].BillingMode
		if mode != "" && mode != BillingModeToken {
			return infraerrors.BadRequest("TIME_PRICING_UNSUPPORTED_MODE", "time pricing only supports token billing mode")
		}
		if err := validateTimePricing(config); err != nil {
			return infraerrors.BadRequest("INVALID_TIME_PRICING", fmt.Sprintf(
				"invalid time pricing for models %v: %v", pricing[i].Models, err))
		}
	}
	return nil
}

// validatePricingBillingMode 校验计费模式配置：按次/图片模式必须配价格或区间，所有价格字段不能为负，区间至少有一个价格字段。
func validatePricingBillingMode(pricing []PricingCard) error {
	for _, p := range pricing {
		if err := checkBillingModeRequirements(p); err != nil {
			return err
		}
		if err := checkPricesNotNegative(p); err != nil {
			return err
		}
		if err := checkIntervalsHavePrices(p); err != nil {
			return err
		}
	}
	return nil
}

func checkBillingModeRequirements(p PricingCard) error {
	if p.BillingMode == BillingModePerRequest || p.BillingMode == BillingModeImage || p.BillingMode == BillingModeVideo {
		if p.PerRequestPrice == nil && len(p.Intervals) == 0 {
			return infraerrors.BadRequest(
				"BILLING_MODE_MISSING_PRICE",
				"per-request price or intervals required for per_request/image billing mode",
			)
		}
	}
	return nil
}

func checkPricesNotNegative(p PricingCard) error {
	checks := []struct {
		field string
		val   *float64
	}{
		{"input_price", p.InputPrice},
		{"output_price", p.OutputPrice},
		{"cache_write_price", p.CacheWritePrice},
		{"cache_write_1h_price", p.CacheWrite1hPrice},
		{"cache_read_price", p.CacheReadPrice},
		{"image_input_price", p.ImageInputPrice},
		{"image_output_price", p.ImageOutputPrice},
		{"per_request_price", p.PerRequestPrice},
	}
	for _, c := range checks {
		if c.val != nil && *c.val < 0 {
			return infraerrors.BadRequest("NEGATIVE_PRICE", fmt.Sprintf("%s must be >= 0", c.field))
		}
	}
	return nil
}

func checkIntervalsHavePrices(p PricingCard) error {
	for _, iv := range p.Intervals {
		if iv.InputPrice == nil && iv.OutputPrice == nil &&
			iv.CacheWritePrice == nil && iv.CacheWrite1hPrice == nil && iv.CacheReadPrice == nil &&
			iv.PerRequestPrice == nil && iv.InputMultiplier == nil &&
			iv.OutputMultiplier == nil && iv.CacheWriteMultiplier == nil &&
			iv.CacheReadMultiplier == nil {
			return infraerrors.BadRequest(
				"INTERVAL_MISSING_PRICE",
				fmt.Sprintf("interval [%d, %s] has no price fields set for model %v",
					iv.MinTokens, formatMaxTokens(iv.MaxTokens), p.Models),
			)
		}
	}
	return nil
}

func formatMaxTokens(max *int) string {
	if max == nil {
		return "∞"
	}
	return fmt.Sprintf("%d", *max)
}

// modelEntry 表示一个模型模式条目（用于冲突检测）
type modelEntry struct {
	pattern  string // 原始模式（如 "claude-*" 或 "claude-opus-4"）
	prefix   string // lowercase 前缀（通配符去掉 *，精确名保持原样）
	wildcard bool
}

// conflictsBetween 检查两个模型模式是否冲突
func conflictsBetween(a, b modelEntry) bool {
	switch {
	case !a.wildcard && !b.wildcard:
		return a.prefix == b.prefix
	case a.wildcard && !b.wildcard:
		return strings.HasPrefix(b.prefix, a.prefix)
	case !a.wildcard && b.wildcard:
		return strings.HasPrefix(a.prefix, b.prefix)
	default:
		return strings.HasPrefix(a.prefix, b.prefix) ||
			strings.HasPrefix(b.prefix, a.prefix)
	}
}

// toPricingModelEntry 将模型名转换为 modelEntry（用于模型定价的冲突检测）。
//
// 与 toModelEntry 的区别：定价缓存的键走 normalizePricingModelName
// （额外做 TrimSpace，并把 claude-* 的 "." 换成 "-"），冲突检测必须用同一套归一化，
// 否则两个校验时看着不同、写进缓存后键相同的定价会互相静默覆盖。
func toPricingModelEntry(pattern string) modelEntry {
	// 先剥通配符再归一化，与 expandPricingToCache 的处理顺序保持一致
	prefix, isWild := splitWildcardSuffix(pattern)
	return modelEntry{
		pattern:  pattern,
		prefix:   normalizePricingModelName(prefix),
		wildcard: isWild,
	}
}

// validateNoConflictingModels 检查定价列表中是否有冲突模型模式。
// 冲突包括：精确重复、通配符之间的前缀包含、通配符与精确名的前缀匹配。
func validateNoConflictingModels(pricingList []PricingCard) error {
	entries := make([]modelEntry, 0)
	for _, p := range pricingList {
		for _, model := range p.Models {
			entries = append(entries, toPricingModelEntry(model))
		}
	}
	return detectConflicts(entries, "MODEL_PATTERN_CONFLICT", "model patterns")
}

// detectConflicts 在一组 modelEntry 中检测冲突，返回带有 errCode 和 label 的错误
func detectConflicts(entries []modelEntry, errCode, label string) error {
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if conflictsBetween(entries[i], entries[j]) {
				return infraerrors.BadRequest(errCode,
					fmt.Sprintf("%s '%s' and '%s' conflict: overlapping match range "+
						"(model names are matched case-insensitively, so an existing entry already covers all case variants)",
						label, entries[i].pattern, entries[j].pattern))
			}
		}
	}
	return nil
}

func TestGetIntervalForContext(t *testing.T) {
	p := &PricingCard{
		Intervals: []PricingInterval{
			{MinTokens: 0, MaxTokens: testPtrInt(128000), InputPrice: testPtrFloat64(1e-6)},
			{MinTokens: 128000, MaxTokens: nil, InputPrice: testPtrFloat64(2e-6)},
		},
	}

	tests := []struct {
		name      string
		tokens    int
		wantPrice *float64
		wantNil   bool
	}{
		{"first interval", 50000, testPtrFloat64(1e-6), false},
		// (min, max] — 128000 在第一个区间的 max，包含，所以匹配第一个
		{"boundary: max of first (inclusive)", 128000, testPtrFloat64(1e-6), false},
		// 128001 > 128000，匹配第二个区间
		{"boundary: just above first max", 128001, testPtrFloat64(2e-6), false},
		{"unbounded interval", 500000, testPtrFloat64(2e-6), false},
		// (0, max] — 0 不匹配任何区间（左开）
		{"zero tokens: no match", 0, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.GetIntervalForContext(tt.tokens)
			if tt.wantNil {
				require.Nil(t, result)
				return
			}
			require.NotNil(t, result)
			require.InDelta(t, *tt.wantPrice, *result.InputPrice, 1e-12)
		})
	}
}

func TestGetIntervalForContext_NoMatch(t *testing.T) {
	p := &PricingCard{
		Intervals: []PricingInterval{
			{MinTokens: 10000, MaxTokens: testPtrInt(50000)},
		},
	}
	require.Nil(t, p.GetIntervalForContext(5000))     // 5000 <= 10000, not > min
	require.Nil(t, p.GetIntervalForContext(10000))    // 10000 not > 10000 (left-open)
	require.NotNil(t, p.GetIntervalForContext(50000)) // 50000 <= 50000 (right-closed)
	require.Nil(t, p.GetIntervalForContext(50001))    // 50001 > 50000
}

func TestGetIntervalForContext_Empty(t *testing.T) {
	p := &PricingCard{Intervals: nil}
	require.Nil(t, p.GetIntervalForContext(1000))
}

func TestGetTierByLabel(t *testing.T) {
	p := &PricingCard{
		Intervals: []PricingInterval{
			{TierLabel: "1K", PerRequestPrice: testPtrFloat64(0.04)},
			{TierLabel: "2K", PerRequestPrice: testPtrFloat64(0.08)},
			{TierLabel: "HD", PerRequestPrice: testPtrFloat64(0.12)},
		},
	}

	tests := []struct {
		name    string
		label   string
		wantNil bool
		want    float64
	}{
		{"exact match", "1K", false, 0.04},
		{"case insensitive", "hd", false, 0.12},
		{"not found", "4K", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.GetTierByLabel(tt.label)
			if tt.wantNil {
				require.Nil(t, result)
				return
			}
			require.NotNil(t, result)
			require.InDelta(t, tt.want, *result.PerRequestPrice, 1e-12)
		})
	}
}

func TestGetTierByLabel_Empty(t *testing.T) {
	p := &PricingCard{Intervals: nil}
	require.Nil(t, p.GetTierByLabel("1K"))
}

func TestPricingCardClone(t *testing.T) {
	original := PricingCard{
		Models: []string{"a", "b"},
		Intervals: []PricingInterval{
			{MinTokens: 0, TierLabel: "tier1"},
		},
		TimePricing: &TimePricing{
			Timezone:     "Asia/Shanghai",
			WeekdaysOnly: true,
			Periods: []TimePricingPeriod{{
				StartTime:  "09:00",
				EndTime:    "12:00",
				Multiplier: 2,
			}},
		},
	}

	cloned := original.Clone()

	// Modify clone slices — original unchanged
	cloned.Models[0] = "hacked"
	require.Equal(t, "a", original.Models[0])

	cloned.Intervals[0].TierLabel = "hacked"
	require.Equal(t, "tier1", original.Intervals[0].TierLabel)

	cloned.TimePricing.Timezone = "America/New_York"
	cloned.TimePricing.WeekdaysOnly = false
	cloned.TimePricing.Periods[0].StartTime = "10:00"
	cloned.TimePricing.Periods[0].Multiplier = 3
	require.Equal(t, "Asia/Shanghai", original.TimePricing.Timezone)
	require.True(t, original.TimePricing.WeekdaysOnly)
	require.Equal(t, "09:00", original.TimePricing.Periods[0].StartTime)
	require.Equal(t, 2.0, original.TimePricing.Periods[0].Multiplier)
}

func TestBillingModeIsValid(t *testing.T) {
	tests := []struct {
		name string
		mode BillingMode
		want bool
	}{
		{"token", BillingModeToken, true},
		{"per_request", BillingModePerRequest, true},
		{"image", BillingModeImage, true},
		{"empty", BillingMode(""), true},
		{"unknown", BillingMode("unknown"), false},
		{"random", BillingMode("xyz"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.mode.IsValid())
		})
	}
}

func TestPricingCardClone_EdgeCases(t *testing.T) {
	t.Run("nil models", func(t *testing.T) {
		original := PricingCard{Models: nil}
		cloned := original.Clone()
		require.Nil(t, cloned.Models)
	})

	t.Run("nil intervals", func(t *testing.T) {
		original := PricingCard{Intervals: nil}
		cloned := original.Clone()
		require.Nil(t, cloned.Intervals)
	})

	t.Run("empty models", func(t *testing.T) {
		original := PricingCard{Models: []string{}}
		cloned := original.Clone()
		require.NotNil(t, cloned.Models)
		require.Empty(t, cloned.Models)
	})
}

func TestValidateIntervals_Empty(t *testing.T) {
	require.NoError(t, ValidateIntervals(nil, BillingModeToken))
	require.NoError(t, ValidateIntervals([]PricingInterval{}, BillingModeToken))
}

func TestValidateIntervals_ValidIntervals(t *testing.T) {
	tests := []struct {
		name      string
		intervals []PricingInterval
	}{
		{
			name: "single bounded interval",
			intervals: []PricingInterval{
				{MinTokens: 0, MaxTokens: testPtrInt(128000), InputPrice: testPtrFloat64(1e-6)},
			},
		},
		{
			name: "two intervals with gap",
			intervals: []PricingInterval{
				{MinTokens: 0, MaxTokens: testPtrInt(100000), InputPrice: testPtrFloat64(1e-6)},
				{MinTokens: 128000, MaxTokens: nil, InputPrice: testPtrFloat64(2e-6)},
			},
		},
		{
			name: "two contiguous intervals",
			intervals: []PricingInterval{
				{MinTokens: 0, MaxTokens: testPtrInt(128000), InputPrice: testPtrFloat64(1e-6)},
				{MinTokens: 128000, MaxTokens: nil, InputPrice: testPtrFloat64(2e-6)},
			},
		},
		{
			name: "unsorted input (auto-sorted by validator)",
			intervals: []PricingInterval{
				{MinTokens: 128000, MaxTokens: nil, InputPrice: testPtrFloat64(2e-6)},
				{MinTokens: 0, MaxTokens: testPtrInt(128000), InputPrice: testPtrFloat64(1e-6)},
			},
		},
		{
			name: "single unbounded interval",
			intervals: []PricingInterval{
				{MinTokens: 0, MaxTokens: nil, InputPrice: testPtrFloat64(1e-6)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, ValidateIntervals(tt.intervals, BillingModeToken))
		})
	}
}

func TestValidateIntervals_NegativeMinTokens(t *testing.T) {
	intervals := []PricingInterval{
		{MinTokens: -1, MaxTokens: testPtrInt(100), InputPrice: testPtrFloat64(1e-6)},
	}
	err := ValidateIntervals(intervals, BillingModeToken)
	require.Error(t, err)
	require.Contains(t, err.Error(), "min_tokens")
	require.Contains(t, err.Error(), ">= 0")
}

func TestValidateIntervals_MaxTokensZero(t *testing.T) {
	intervals := []PricingInterval{
		{MinTokens: 0, MaxTokens: testPtrInt(0), InputPrice: testPtrFloat64(1e-6)},
	}
	err := ValidateIntervals(intervals, BillingModeToken)
	require.Error(t, err)
	require.Contains(t, err.Error(), "max_tokens")
	require.Contains(t, err.Error(), "> 0")
}

func TestValidateIntervals_MaxLessThanMin(t *testing.T) {
	intervals := []PricingInterval{
		{MinTokens: 100, MaxTokens: testPtrInt(50), InputPrice: testPtrFloat64(1e-6)},
	}
	err := ValidateIntervals(intervals, BillingModeToken)
	require.Error(t, err)
	require.Contains(t, err.Error(), "max_tokens")
	require.Contains(t, err.Error(), "> min_tokens")
}

func TestValidateIntervals_MaxEqualsMin(t *testing.T) {
	intervals := []PricingInterval{
		{MinTokens: 100, MaxTokens: testPtrInt(100), InputPrice: testPtrFloat64(1e-6)},
	}
	err := ValidateIntervals(intervals, BillingModeToken)
	require.Error(t, err)
	require.Contains(t, err.Error(), "max_tokens")
	require.Contains(t, err.Error(), "> min_tokens")
}

func TestValidateIntervals_NegativePrice(t *testing.T) {
	negPrice := -0.01
	intervals := []PricingInterval{
		{MinTokens: 0, MaxTokens: testPtrInt(100), InputPrice: &negPrice},
	}
	err := ValidateIntervals(intervals, BillingModeToken)
	require.Error(t, err)
	require.Contains(t, err.Error(), "input_price")
	require.Contains(t, err.Error(), ">= 0")
}

func TestValidateIntervals_OverlappingIntervals(t *testing.T) {
	intervals := []PricingInterval{
		{MinTokens: 0, MaxTokens: testPtrInt(200), InputPrice: testPtrFloat64(1e-6)},
		{MinTokens: 100, MaxTokens: testPtrInt(300), InputPrice: testPtrFloat64(2e-6)},
	}
	err := ValidateIntervals(intervals, BillingModeToken)
	require.Error(t, err)
	require.Contains(t, err.Error(), "overlap")
}

func TestValidateIntervals_UnboundedNotLast(t *testing.T) {
	intervals := []PricingInterval{
		{MinTokens: 0, MaxTokens: nil, InputPrice: testPtrFloat64(1e-6)},
		{MinTokens: 128000, MaxTokens: testPtrInt(256000), InputPrice: testPtrFloat64(2e-6)},
	}
	err := ValidateIntervals(intervals, BillingModeToken)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unbounded")
	require.Contains(t, err.Error(), "last")
}

func TestValidateIntervals_ImageModeAllowsMultipleUnboundedTiers(t *testing.T) {
	// image / per_request 按 tier_label 匹配，多条 min=0/max=nil 是合法形态。
	intervals := []PricingInterval{
		{MinTokens: 0, MaxTokens: nil, TierLabel: "1K", PerRequestPrice: testPtrFloat64(0.04)},
		{MinTokens: 0, MaxTokens: nil, TierLabel: "2K", PerRequestPrice: testPtrFloat64(0.06)},
		{MinTokens: 0, MaxTokens: nil, TierLabel: "4K", PerRequestPrice: testPtrFloat64(0.08)},
	}
	require.NoError(t, ValidateIntervals(intervals, BillingModeImage))
	require.NoError(t, ValidateIntervals(intervals, BillingModePerRequest))
}

func TestValidateIntervals_ImageModeStillRejectsNegativePrice(t *testing.T) {
	// image 模式只跳过区间重叠校验，单条字段自洽（价格非负）仍要校验。
	intervals := []PricingInterval{
		{MinTokens: 0, MaxTokens: nil, TierLabel: "1K", PerRequestPrice: testPtrFloat64(-1)},
	}
	err := ValidateIntervals(intervals, BillingModeImage)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be >= 0")
}

func TestValidateIntervals_ImageModeStillRejectsBadMaxTokens(t *testing.T) {
	// image 模式仍校验 max <= min 这种单条不合法。
	intervals := []PricingInterval{
		{MinTokens: 100, MaxTokens: testPtrInt(50), TierLabel: "1K", PerRequestPrice: testPtrFloat64(0.04)},
	}
	err := ValidateIntervals(intervals, BillingModeImage)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be > min_tokens")
}

func TestValidateNoConflictingModels(t *testing.T) {
	tests := []struct {
		name        string
		pricingList []PricingCard
		wantErr     bool
		errContains string
	}{
		{
			name: "no duplicates",
			pricingList: []PricingCard{
				{Models: []string{"claude-sonnet-4", "claude-opus-4"}},
				{Models: []string{"gpt-5.1"}},
			},
			wantErr: false,
		},
		{
			name: "duplicate across cards",
			pricingList: []PricingCard{
				{Models: []string{"claude-sonnet-4"}},
				{Models: []string{"claude-sonnet-4"}},
			},
			wantErr:     true,
			errContains: "claude-sonnet-4",
		},
		{
			// 价卡不再按平台分桶：同名模型出现在两张卡上就是冲突
			name: "same model on two cards conflicts",
			pricingList: []PricingCard{
				{Models: []string{"model-a"}},
				{Models: []string{"model-a"}},
			},
			wantErr:     true,
			errContains: "model-a",
		},
		{
			name: "case insensitive",
			pricingList: []PricingCard{
				{Models: []string{"Claude"}},
				{Models: []string{"claude"}},
			},
			wantErr: true,
		},
		{
			name:        "empty list (nil)",
			pricingList: nil,
			wantErr:     false,
		},
		{
			name: "wildcard_vs_wildcard_conflict",
			pricingList: []PricingCard{
				{Models: []string{"claude-*"}},
				{Models: []string{"claude-opus-*"}},
			},
			wantErr:     true,
			errContains: "conflict",
		},
		{
			name: "wildcard_vs_exact_conflict",
			pricingList: []PricingCard{
				{Models: []string{"claude-*"}},
				{Models: []string{"claude-opus-4-6"}},
			},
			wantErr:     true,
			errContains: "conflict",
		},
		{
			name: "wildcard_prefix_conflict_across_cards",
			pricingList: []PricingCard{
				{Models: []string{"claude-opus-*"}},
				{Models: []string{"claude-*"}},
			},
			wantErr:     true,
			errContains: "conflict",
		},
		{
			name: "no_conflict_same_platform_different_prefix",
			pricingList: []PricingCard{
				{Models: []string{"claude-opus-*"}},
				{Models: []string{"gpt-*"}},
			},
			wantErr: false,
		},
		{
			name: "catch_all_wildcard_conflicts_with_everything",
			pricingList: []PricingCard{
				{Models: []string{"*"}},
				{Models: []string{"gpt-5"}},
			},
			wantErr:     true,
			errContains: "conflict",
		},
		// 以下三例：冲突检测必须与 normalizePricingModelName 用同一套归一化，
		// 否则校验放行、写进缓存后键相同，后写的定价会静默覆盖前一条。
		{
			name: "claude_dot_and_hyphen_spelling_conflict",
			pricingList: []PricingCard{
				{Models: []string{"claude-sonnet-4.5"}},
				{Models: []string{"claude-sonnet-4-5"}},
			},
			wantErr:     true,
			errContains: "conflict",
		},
		{
			name: "claude_dot_and_hyphen_spelling_conflict_wildcard",
			pricingList: []PricingCard{
				{Models: []string{"claude-sonnet-4.5*"}},
				{Models: []string{"claude-sonnet-4-5-x"}},
			},
			wantErr:     true,
			errContains: "conflict",
		},
		{
			name: "surrounding_whitespace_conflict",
			pricingList: []PricingCard{
				{Models: []string{"gpt-5.6"}},
				{Models: []string{" gpt-5.6 "}},
			},
			wantErr:     true,
			errContains: "conflict",
		},
		{
			// 只有 claude-* 前缀才做 "." → "-"，别把其它平台也一起归一化了
			name: "non_claude_dot_spelling_is_not_normalized",
			pricingList: []PricingCard{
				{Models: []string{"gpt-5.6"}},
				{Models: []string{"gpt-5-6"}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNoConflictingModels(tt.pricingList)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}

	// Additional sub-case: explicit empty slice
	t.Run("empty list (empty slice)", func(t *testing.T) {
		err := validateNoConflictingModels([]PricingCard{})
		require.NoError(t, err)
	})
}

func TestConflictsBetween(t *testing.T) {
	tests := []struct {
		name string
		a, b modelEntry
		want bool
	}{
		{
			name: "exact same",
			a:    modelEntry{prefix: "claude-opus-4", wildcard: false},
			b:    modelEntry{prefix: "claude-opus-4", wildcard: false},
			want: true,
		},
		{
			name: "exact different",
			a:    modelEntry{prefix: "claude-opus-4", wildcard: false},
			b:    modelEntry{prefix: "gpt-4o", wildcard: false},
			want: false,
		},
		{
			name: "wildcard matches exact",
			a:    modelEntry{prefix: "claude-", wildcard: true},
			b:    modelEntry{prefix: "claude-opus-4", wildcard: false},
			want: true,
		},
		{
			name: "exact does not match unrelated wildcard",
			a:    modelEntry{prefix: "gpt-4o", wildcard: false},
			b:    modelEntry{prefix: "claude-", wildcard: true},
			want: false,
		},
		{
			name: "wildcard prefix overlap",
			a:    modelEntry{prefix: "claude-", wildcard: true},
			b:    modelEntry{prefix: "claude-opus-", wildcard: true},
			want: true,
		},
		{
			name: "wildcards no overlap",
			a:    modelEntry{prefix: "claude-", wildcard: true},
			b:    modelEntry{prefix: "gpt-", wildcard: true},
			want: false,
		},
		{
			name: "catch-all wildcard vs any",
			a:    modelEntry{prefix: "", wildcard: true},
			b:    modelEntry{prefix: "anything", wildcard: false},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, conflictsBetween(tt.a, tt.b))
		})
	}
}

func TestValidatePricingBillingMode(t *testing.T) {
	tests := []struct {
		name    string
		pricing []PricingCard
		wantErr bool
		errMsg  string
	}{
		{
			name:    "token mode - valid",
			pricing: []PricingCard{{BillingMode: BillingModeToken}},
		},
		{
			name: "per_request with price - valid",
			pricing: []PricingCard{{
				BillingMode:     BillingModePerRequest,
				PerRequestPrice: testPtrFloat64(0.5),
			}},
		},
		{
			name: "per_request with intervals - valid",
			pricing: []PricingCard{{
				BillingMode: BillingModePerRequest,
				Intervals:   []PricingInterval{{MinTokens: 0, MaxTokens: testPtrInt(1000), PerRequestPrice: testPtrFloat64(0.1)}},
			}},
		},
		{
			name:    "per_request no price no intervals - invalid",
			pricing: []PricingCard{{BillingMode: BillingModePerRequest}},
			wantErr: true,
			errMsg:  "per-request price or intervals required",
		},
		{
			name:    "image no price no intervals - invalid",
			pricing: []PricingCard{{BillingMode: BillingModeImage}},
			wantErr: true,
			errMsg:  "per-request price or intervals required",
		},
		{
			name:    "empty list - valid",
			pricing: []PricingCard{},
		},
		{
			name: "negative input_price - invalid",
			pricing: []PricingCard{{
				BillingMode: BillingModeToken,
				InputPrice:  testPtrFloat64(-0.01),
			}},
			wantErr: true,
			errMsg:  "input_price must be >= 0",
		},
		{
			name: "interval with no price fields - invalid",
			pricing: []PricingCard{{
				BillingMode:     BillingModePerRequest,
				PerRequestPrice: testPtrFloat64(0.5),
				Intervals:       []PricingInterval{{MinTokens: 0, MaxTokens: testPtrInt(1000)}},
			}},
			wantErr: true,
			errMsg:  "has no price fields set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePricingBillingMode(tt.pricing)
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func validTimePricingForTest() *TimePricing {
	return &TimePricing{Timezone: "Asia/Shanghai", Periods: []TimePricingPeriod{
		{StartTime: "09:00", EndTime: "12:00", Multiplier: 2},
	}}
}

func TestValidatePricingTimePricing(t *testing.T) {
	token := []PricingCard{{BillingMode: BillingModeToken, TimePricing: validTimePricingForTest()}}
	require.NoError(t, validatePricingTimePricing(token))

	implicitToken := []PricingCard{{TimePricing: validTimePricingForTest()}}
	require.NoError(t, validatePricingTimePricing(implicitToken))

	image := []PricingCard{{BillingMode: BillingModeImage, TimePricing: validTimePricingForTest()}}
	modeErr := infraerrors.FromError(validatePricingTimePricing(image))
	require.Equal(t, int32(http.StatusBadRequest), modeErr.Code)
	require.Equal(t, "TIME_PRICING_UNSUPPORTED_MODE", modeErr.Reason)

	invalid := []PricingCard{{
		Models:      []string{"gpt-5"},
		BillingMode: BillingModeToken,
		TimePricing: &TimePricing{Timezone: "UTC+8", Periods: validTimePricingForTest().Periods},
	}}
	invalidErr := infraerrors.FromError(validatePricingTimePricing(invalid))
	require.Equal(t, int32(http.StatusBadRequest), invalidErr.Code)
	require.Equal(t, "INVALID_TIME_PRICING", invalidErr.Reason)
	require.NotContains(t, invalidErr.Message, "platform")
	require.Contains(t, invalidErr.Message, "models [gpt-5]")

	invalidMultiplier := []PricingCard{{
		Models:      []string{"gpt-5"},
		BillingMode: BillingModeToken,
		TimePricing: &TimePricing{Timezone: "Asia/Shanghai", Periods: []TimePricingPeriod{{
			StartTime: "09:00", EndTime: "12:00", Multiplier: 1e-12,
		}}},
	}}
	invalidMultiplierRawErr := validatePricingTimePricing(invalidMultiplier)
	require.Error(t, invalidMultiplierRawErr)
	invalidMultiplierErr := infraerrors.FromError(invalidMultiplierRawErr)
	require.Equal(t, int32(http.StatusBadRequest), invalidMultiplierErr.Code)
	require.Equal(t, "INVALID_TIME_PRICING", invalidMultiplierErr.Reason)

	empty := []PricingCard{{BillingMode: BillingModeToken, TimePricing: &TimePricing{Timezone: "Asia/Shanghai"}}}
	require.NoError(t, validatePricingTimePricing(empty))
	require.Nil(t, empty[0].TimePricing)
}
