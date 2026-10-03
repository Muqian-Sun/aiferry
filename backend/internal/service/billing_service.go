package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

// APIKeyRateLimitCacheData holds rate limit usage data cached in Redis.
type APIKeyRateLimitCacheData struct {
	Usage5h  float64 `json:"usage_5h"`
	Usage1d  float64 `json:"usage_1d"`
	Usage7d  float64 `json:"usage_7d"`
	Window5h int64   `json:"window_5h"` // unix timestamp, 0 = not started
	Window1d int64   `json:"window_1d"`
	Window7d int64   `json:"window_7d"`
}

// BillingCache defines cache operations for billing service
type BillingCache interface {
	// Balance operations
	GetUserBalance(ctx context.Context, userID int64) (float64, error)
	SetUserBalance(ctx context.Context, userID int64, balance float64) error
	DeductUserBalance(ctx context.Context, userID int64, amount float64) error
	InvalidateUserBalance(ctx context.Context, userID int64) error

	// Subscription operations
	GetSubscriptionCache(ctx context.Context, userID, planID int64) (*SubscriptionCacheData, error)
	SetSubscriptionCache(ctx context.Context, userID, planID int64, data *SubscriptionCacheData) error
	UpdateSubscriptionUsage(ctx context.Context, userID, planID int64, cost float64) error
	InvalidateSubscriptionCache(ctx context.Context, userID, planID int64) error

	// API Key rate limit operations
	GetAPIKeyRateLimit(ctx context.Context, keyID int64) (*APIKeyRateLimitCacheData, error)
	SetAPIKeyRateLimit(ctx context.Context, keyID int64, data *APIKeyRateLimitCacheData) error
	UpdateAPIKeyRateLimitUsage(ctx context.Context, keyID int64, cost float64) error
	InvalidateAPIKeyRateLimit(ctx context.Context, keyID int64) error
}

// ModelPricing 模型价格配置（per-token价格，与LiteLLM格式一致）
type ModelPricing struct {
	InputPricePerToken           float64  // 每token输入价格 (USD)
	ImageInputPricePerToken      float64  // 图片输入 token 价格 (USD)，用于多模态 embedding 等图文不同价场景；为 0 时回退到 InputPricePerToken
	ImageCacheReadPricePerToken  float64  // 图片缓存输入价格；无独立价格时沿用缓存读取价
	OutputPricePerToken          float64  // 每token输出价格 (USD)
	CacheCreationPricePerToken   float64  // 缓存创建每token价格 (USD)
	CacheCreationPriceExplicit   bool     // 是否由渠道/区间定价显式设定（为 true 时即使 == 0 也不回退）
	CacheReadPricePerToken       float64  // 缓存读取每token价格 (USD)
	MaxReasoningEffortMultiplier *float64 // max 推理等级的额度/计费倍率；nil 时沿用模型默认行为
	CacheCreation5mPrice         float64  // 5分钟缓存创建每token价格 (USD)
	CacheCreation1hPrice         float64  // 1小时缓存创建每token价格 (USD)
	SupportsCacheBreakdown       bool     // 是否支持详细的缓存分类
	ImageOutputPricePerToken     float64  // 图片输出 token 价格 (USD)
	ImageOutputPriceExplicit     bool     // 是否由渠道定价显式设定（为 true 时即使 == 0 也不回退）
	AudioInputPricePerToken      float64  // 音频输入 token 价格 (USD)；为 0 时回退到文本输入价（已含分段调整）
	AudioOutputPricePerToken     float64  // 音频输出 token 价格 (USD)；为 0 时回退到文本输出价
}

func normalizeBillingServiceTier(serviceTier string) string {
	return strings.ToLower(strings.TrimSpace(serviceTier))
}

// UsageTokens 使用的token数量
type UsageTokens struct {
	InputTokens           int
	ImageInputTokens      int
	ImageCacheReadTokens  int
	OutputTokens          int
	CacheCreationTokens   int
	CacheReadTokens       int
	CacheCreation5mTokens int
	CacheCreation1hTokens int
	ImageOutputTokens     int
	// AudioInputTokens / AudioOutputTokens 是 InputTokens / OutputTokens 里的音频部分（上游把音频
	// 计在总数里：OpenAI *_tokens_details.audio_tokens，Gemini 的 AUDIO 模态）。计费时从文本 token
	// 中剥出来按音频价计；InputTokens 已扣掉缓存读写时，这里也只算未命中缓存的音频输入。
	AudioInputTokens  int
	AudioOutputTokens int
}

// CostBreakdown 费用明细
type CostBreakdown struct {
	InputCost         float64 // 输入费用（含音频输入，不含图片输入，图片输入单独记入 ImageInputCost）
	ImageInputCost    float64 // 图片输入 token 费用（如 gpt-image-2 图片编辑）
	OutputCost        float64 // 输出费用（含音频输出，不含图片输出）
	ImageOutputCost   float64
	CacheCreationCost float64
	CacheReadCost     float64
	TotalCost         float64
	ActualCost        float64 // 应用倍率后的实际费用
	BillingMode       string  // 计费模式（"token"/"per_request"/"image"），由 CalculateCostUnified 填充
	// AudioInputCost / AudioOutputCost 是 InputCost / OutputCost 中的音频部分（已含在内，用量行不单列），
	// 仅供明细与测试核对。
	AudioInputCost  float64
	AudioOutputCost float64
	// WebSearchCount / WebSearchCost 联网搜索的次数与搜索费（官方原价、不乘用户倍率，已含在 TotalCost / ActualCost 里）。
	WebSearchCount int
	WebSearchCost  float64
}

func applyCostBreakdownMultiplier(cost *CostBreakdown, multiplier float64) {
	if cost == nil || multiplier == 1 {
		return
	}
	cost.InputCost *= multiplier
	cost.ImageInputCost *= multiplier
	cost.OutputCost *= multiplier
	cost.ImageOutputCost *= multiplier
	cost.AudioInputCost *= multiplier
	cost.AudioOutputCost *= multiplier
	cost.CacheCreationCost *= multiplier
	cost.CacheReadCost *= multiplier
	cost.TotalCost *= multiplier
	cost.ActualCost *= multiplier
}

const claudeFable51MaxReasoningEffortMultiplier = 3.0

func isClaudeFable51Model(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	for _, marker := range []string{"fable-5-1", "fable-5.1", "fable5.1", "fable51"} {
		if at := strings.Index(model, marker); at >= 0 {
			after := at + len(marker)
			if after == len(model) || model[after] < '0' || model[after] > '9' {
				return true
			}
		}
	}
	return false
}

func defaultMaxReasoningEffortMultiplier(model string) *float64 {
	if !isClaudeFable51Model(model) {
		return nil
	}
	multiplier := claudeFable51MaxReasoningEffortMultiplier
	return &multiplier
}

func maxReasoningEffortBillingMultiplier(model, effort string, pricing *ModelPricing) float64 {
	if NormalizeMaxReasoningEffort(effort) != "max" {
		return 1
	}
	if pricing != nil && pricing.MaxReasoningEffortMultiplier != nil && *pricing.MaxReasoningEffortMultiplier > 0 {
		return *pricing.MaxReasoningEffortMultiplier
	}
	if multiplier := defaultMaxReasoningEffortMultiplier(model); multiplier != nil {
		return *multiplier
	}
	return 1
}

// resolvedTimePricingMultiplier 返回命中价卡的分时倍率。
// 只认目录来源：分组价卡在管理端就被 GROUP_MODEL_TIME_PRICING_UNSUPPORTED 拒掉
// （admin_group.go），groups.model_pricing 是 JSONB，手工写进去的分时配置不该生效。
func resolvedTimePricingMultiplier(resolved *ResolvedPricing, at time.Time) float64 {
	if resolved == nil || resolved.Source != PricingSourceCatalog || resolved.configuredPricing == nil {
		return 1
	}
	return resolved.configuredPricing.TimePricing.MultiplierAt(at)
}

// ErrModelPricingUnavailable indicates that none of the configured pricing
// sources can price the requested model.
var ErrModelPricingUnavailable = errors.New("pricing not found")

// ---- DeepSeek 官方低谷价（$/token）----
// 2026-09-10 官方公告：DeepSeek-V4.1-Flash（新名 deepseek-flash）大幅降价，
// Flash 低谷价降为 $0.15/$0.60/$0.003 per MTok（输入缓存未命中/输出/缓存命中）；
// deepseek-v4-pro 名义价格暂不变，但自北京时间 2026-09-14 12:00（04:00 UTC）起
// 其请求被上游路由到 V4.1-Flash 并按 Flash 价计费（见 deepseekProBilledAsFlash）。
// Source: https://api-docs.deepseek.com/news/news260910
//
//	https://api-docs.deepseek.com/quick_start/pricing
//
// 高峰价 = 2× 低谷价；高峰时段 01:00–04:00 与 06:00–10:00 UTC（仅工作日），
// 北京时间周六/周日全天低谷。时段判定见 deepseekPeakMultiplierAt。
const (
	deepseekFlashOffPeakInputPrice  = 1.5e-7  // $0.15 per MTok (cache miss)
	deepseekFlashOffPeakOutputPrice = 6.0e-7  // $0.60 per MTok
	deepseekFlashOffPeakCacheRead   = 3e-9    // $0.003 per MTok (cache hit)
	deepseekProOffPeakInputPrice    = 6.6e-7  // $0.66 per MTok (cache miss)
	deepseekProOffPeakOutputPrice   = 1.98e-6 // $1.98 per MTok
	deepseekProOffPeakCacheRead     = 2.2e-8  // $0.022 per MTok (cache hit)
)

// isDeepSeekModel 判断模型名是否为 DeepSeek 模型（大小写不敏感）。
// 任意 deepseek- 前缀均视为 DeepSeek 模型：官方模型（v4-flash / v4-pro /
// v4-flash-vision-exp）按各自价卡计价，其余 deepseek-*（含已停服的
// deepseek-chat / deepseek-reasoner 与未知型号）统一按 flash 价兜底，
// 避免计费中断；新名字由 fallback warn 日志（每模型每进程一条）暴露，
// 运营者据此更新价卡。
func isDeepSeekModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "deepseek-")
}

// deepseekPeakMultiplierAt 返回指定时刻的 DeepSeek 官方峰谷定价因子。
// 官方口径（2026-08-23 起生效）：高峰价 = 2× 低谷价；高峰时段为
// 01:00–04:00 与 06:00–10:00 UTC（半开区间），仅工作日；
// 周末（北京时间周六/周日）全天低谷。北京时间用固定 +8 偏移（无夏令时）。
func deepseekPeakMultiplierAt(now time.Time) float64 {
	beijing := now.In(time.FixedZone("Asia/Shanghai", 8*3600))
	switch beijing.Weekday() {
	case time.Saturday, time.Sunday:
		return 1.0
	}
	switch h := now.UTC().Hour(); {
	case h >= 1 && h < 4, h >= 6 && h < 10:
		return 2.0
	}
	return 1.0
}

// deepseekProRoutesToFlashAt：官方公告自北京时间 2026-09-14 12:00（04:00 UTC）起，
// 所有 deepseek-v4-pro 请求被上游路由到 V4.1-Flash 并按 Flash 价计费（直至未来
// V4.1 Pro 上线）。Source: https://api-docs.deepseek.com/news/news260910
var deepseekProRoutesToFlashAt = time.Date(2026, 9, 14, 4, 0, 0, 0, time.UTC)

// deepseekProBilledAsFlash 报告指定计费时点 deepseek-v4-pro 是否已按 Flash 价
// 计费：计费时点到达或晚于切换时点返回 true；零值时点回退当前时刻（与峰谷
// 倍率的取时点方式一致，见 calculateTokenCost）。
func deepseekProBilledAsFlash(pricingAt time.Time) bool {
	if pricingAt.IsZero() {
		pricingAt = timezone.Now()
	}
	return !pricingAt.Before(deepseekProRoutesToFlashAt)
}

// isDeepSeekProModel 判断模型名是否归入 deepseek-v4-pro 档（含版本化名称，
// 如 deepseek-v4-pro-0813）。
func isDeepSeekProModel(model string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(model)), "deepseek-v4-pro")
}

// BillingService 计费服务
type BillingService struct {
	cfg            *config.Config
	pricingService *PricingService
	fallbackPrices map[string]*ModelPricing // 硬编码回退价格

	// fallbackWarnSeen 记录已打过 fallback 警告日志的(已小写化)模型名,
	// 让 "[Billing] Using fallback pricing" 每个模型每进程最多打一条,
	// 避免热路径上每请求刷屏(issue #3394)。零值即可用,无需在构造函数初始化。
	fallbackWarnSeen sync.Map
}

// NewBillingService 创建计费服务实例
func NewBillingService(cfg *config.Config, pricingService *PricingService) *BillingService {
	s := &BillingService{
		cfg:            cfg,
		pricingService: pricingService,
		fallbackPrices: make(map[string]*ModelPricing),
	}

	// 初始化硬编码回退价格（当动态价格不可用时使用）
	s.initFallbackPricing()

	return s
}

// initFallbackPricing 初始化硬编码回退价格（当动态价格不可用时使用）
// 价格单位：USD per token（与LiteLLM格式一致）
func (s *BillingService) initFallbackPricing() {
	// Claude 4.5 Opus
	s.fallbackPrices["claude-opus-4.5"] = &ModelPricing{
		InputPricePerToken:         5e-6,    // $5 per MTok
		OutputPricePerToken:        25e-6,   // $25 per MTok
		CacheCreationPricePerToken: 6.25e-6, // $6.25 per MTok
		CacheReadPricePerToken:     0.5e-6,  // $0.50 per MTok
		SupportsCacheBreakdown:     false,
	}

	// Claude 4 Sonnet
	s.fallbackPrices["claude-sonnet-4"] = &ModelPricing{
		InputPricePerToken:         3e-6,    // $3 per MTok
		OutputPricePerToken:        15e-6,   // $15 per MTok
		CacheCreationPricePerToken: 3.75e-6, // $3.75 per MTok
		CacheReadPricePerToken:     0.3e-6,  // $0.30 per MTok
		SupportsCacheBreakdown:     false,
	}

	// Claude 3.5 Sonnet
	s.fallbackPrices["claude-3-5-sonnet"] = &ModelPricing{
		InputPricePerToken:         3e-6,    // $3 per MTok
		OutputPricePerToken:        15e-6,   // $15 per MTok
		CacheCreationPricePerToken: 3.75e-6, // $3.75 per MTok
		CacheReadPricePerToken:     0.3e-6,  // $0.30 per MTok
		SupportsCacheBreakdown:     false,
	}

	// Claude 3.5 Haiku
	s.fallbackPrices["claude-3-5-haiku"] = &ModelPricing{
		InputPricePerToken:         1e-6,    // $1 per MTok
		OutputPricePerToken:        5e-6,    // $5 per MTok
		CacheCreationPricePerToken: 1.25e-6, // $1.25 per MTok
		CacheReadPricePerToken:     0.1e-6,  // $0.10 per MTok
		SupportsCacheBreakdown:     false,
	}

	// Claude 3 Opus
	s.fallbackPrices["claude-3-opus"] = &ModelPricing{
		InputPricePerToken:         15e-6,    // $15 per MTok
		OutputPricePerToken:        75e-6,    // $75 per MTok
		CacheCreationPricePerToken: 18.75e-6, // $18.75 per MTok
		CacheReadPricePerToken:     1.5e-6,   // $1.50 per MTok
		SupportsCacheBreakdown:     false,
	}

	// Claude 3 Haiku
	s.fallbackPrices["claude-3-haiku"] = &ModelPricing{
		InputPricePerToken:         0.25e-6, // $0.25 per MTok
		OutputPricePerToken:        1.25e-6, // $1.25 per MTok
		CacheCreationPricePerToken: 0.3e-6,  // $0.30 per MTok
		CacheReadPricePerToken:     0.03e-6, // $0.03 per MTok
		SupportsCacheBreakdown:     false,
	}

	// Claude 4.6 Opus (与4.5同价)
	s.fallbackPrices["claude-opus-4.6"] = s.fallbackPrices["claude-opus-4.5"]

	// Claude 4.7 Opus (暂与4.6同价，待官方定价更新)
	s.fallbackPrices["claude-opus-4.7"] = s.fallbackPrices["claude-opus-4.6"]

	// Claude 4.8 Opus / Claude Opus 5（$5/$25 per MTok）。
	// 缺少这两条时 getFallbackPricing 会掉到 claude-3-opus（$15/$75），造成 3 倍超收。
	s.fallbackPrices["claude-opus-4.8"] = s.fallbackPrices["claude-opus-4.7"]
	s.fallbackPrices["claude-opus-5"] = s.fallbackPrices["claude-opus-4.8"]

	// Claude Fable 5.x uses the same input/output and cache-write prices, while
	// Fable 5.1 reduces cache reads from $1 to $0.25 per MTok.
	s.fallbackPrices["claude-fable-5"] = &ModelPricing{
		InputPricePerToken:         10e-6,
		OutputPricePerToken:        50e-6,
		CacheCreationPricePerToken: 12.5e-6,
		CacheCreation5mPrice:       12.5e-6,
		CacheCreation1hPrice:       20e-6,
		CacheReadPricePerToken:     1e-6,
		SupportsCacheBreakdown:     true,
	}
	s.fallbackPrices["claude-fable-5-1"] = &ModelPricing{
		InputPricePerToken:         10e-6,
		OutputPricePerToken:        50e-6,
		CacheCreationPricePerToken: 12.5e-6,
		CacheCreation5mPrice:       12.5e-6,
		CacheCreation1hPrice:       20e-6,
		CacheReadPricePerToken:     0.25e-6,
		SupportsCacheBreakdown:     true,
	}

	// Gemini 3.1 Pro
	s.fallbackPrices["gemini-3.1-pro"] = &ModelPricing{
		InputPricePerToken:         2e-6,   // $2 per MTok
		OutputPricePerToken:        12e-6,  // $12 per MTok
		CacheCreationPricePerToken: 2e-6,   // $2 per MTok
		CacheReadPricePerToken:     0.2e-6, // $0.20 per MTok
		SupportsCacheBreakdown:     false,
	}

	// Gemini 3.6 Flash (Google AI pricing: $1.50 input / $7.50 output /
	// $0.15 cached input per MTok). Antigravity's -high/-low/-medium/-tiered
	// aliases are matched below so unavailable remote pricing never records
	// token-bearing requests at $0.
	s.fallbackPrices["gemini-3.6-flash"] = &ModelPricing{
		InputPricePerToken:     1.5e-6,
		OutputPricePerToken:    7.5e-6,
		CacheReadPricePerToken: 0.15e-6,
		SupportsCacheBreakdown: false,
	}

	// Gemini 3.7 Flash (Google AI pricing: $0.75 input / $3.75 output /
	// $0.075 cached input per MTok, promotional through 2026-12-31; official
	// rates double to $1.50/$7.50/$0.15 from 2027-01-01). Antigravity's
	// -high/-low/-medium/-tiered aliases are matched below so unavailable
	// remote pricing never records token-bearing requests at $0.
	s.fallbackPrices["gemini-3.7-flash"] = &ModelPricing{
		InputPricePerToken:     0.75e-6,
		OutputPricePerToken:    3.75e-6,
		CacheReadPricePerToken: 0.075e-6,
		SupportsCacheBreakdown: false,
	}

	// Gemini 3.8 Flash (Google AI pricing: $0.75 input / $3.75 output /
	// $0.075 cached input per MTok, promotional through 2026-12-31; official
	// rates double to $1.50/$7.50/$0.15 from 2027-01-01). Antigravity's
	// -high/-low/-medium/-tiered aliases are matched below so unavailable
	// remote pricing never records token-bearing requests at $0.
	s.fallbackPrices["gemini-3.8-flash"] = &ModelPricing{
		InputPricePerToken:     0.75e-6,
		OutputPricePerToken:    3.75e-6,
		CacheReadPricePerToken: 0.075e-6,
		SupportsCacheBreakdown: false,
	}

	// OpenAI GPT-5.4（业务指定价格）
	s.fallbackPrices["gpt-5.4"] = &ModelPricing{
		InputPricePerToken:         2.5e-6,  // $2.5 per MTok
		OutputPricePerToken:        15e-6,   // $15 per MTok
		CacheCreationPricePerToken: 2.5e-6,  // $2.5 per MTok
		CacheReadPricePerToken:     0.25e-6, // $0.25 per MTok
		SupportsCacheBreakdown:     false,
	}
	// OpenAI GPT-5.5 官方价格。
	// Source: https://platform.openai.com/docs/pricing
	s.fallbackPrices["gpt-5.5"] = &ModelPricing{
		InputPricePerToken:  5e-6,
		OutputPricePerToken: 30e-6,
		// 官方未列独立 cache-write 价；内部出现 cache creation token 时按输入价兜底。
		CacheCreationPricePerToken: 5e-6,
		CacheReadPricePerToken:     0.5e-6,
		SupportsCacheBreakdown:     false,
	}
	// GPT-5.5 Pro。
	s.fallbackPrices["gpt-5.5-pro"] = &ModelPricing{
		InputPricePerToken:  30e-6,
		OutputPricePerToken: 180e-6,
		// 官方未列独立 cached-input/cache-write 价；内部出现对应 token 时按输入价兜底。
		CacheCreationPricePerToken: 30e-6,
		CacheReadPricePerToken:     30e-6,
		SupportsCacheBreakdown:     false,
	}

	s.fallbackPrices["gpt-6-astra"] = &ModelPricing{
		InputPricePerToken:         10e-6,
		OutputPricePerToken:        50e-6,
		CacheCreationPricePerToken: 12.5e-6,
		CacheReadPricePerToken:     1e-6,
	}

	// OpenAI GPT-5.6 官方价格（USD/token）。缓存写入为输入价的 1.25 倍。
	s.fallbackPrices["gpt-5.6-sol"] = &ModelPricing{
		InputPricePerToken:         5e-6,
		OutputPricePerToken:        30e-6,
		CacheCreationPricePerToken: 6.25e-6,
		CacheReadPricePerToken:     0.5e-6,
	}
	s.fallbackPrices["gpt-5.6-terra"] = &ModelPricing{
		InputPricePerToken:         2e-6,
		OutputPricePerToken:        12e-6,
		CacheCreationPricePerToken: 2.5e-6,
		CacheReadPricePerToken:     0.2e-6,
	}
	s.fallbackPrices["gpt-5.6-luna"] = &ModelPricing{
		InputPricePerToken:         0.2e-6,
		OutputPricePerToken:        1.2e-6,
		CacheCreationPricePerToken: 0.25e-6,
		CacheReadPricePerToken:     0.02e-6,
	}

	s.fallbackPrices["gpt-5.4-mini"] = &ModelPricing{
		InputPricePerToken:     7.5e-7,
		OutputPricePerToken:    4.5e-6,
		CacheReadPricePerToken: 7.5e-8,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["gpt-5.4-nano"] = &ModelPricing{
		InputPricePerToken:     2e-7,
		OutputPricePerToken:    1.25e-6,
		CacheReadPricePerToken: 2e-8,
		SupportsCacheBreakdown: false,
	}
	// OpenAI GPT-5.2（本地兜底）
	s.fallbackPrices["gpt-5.2"] = &ModelPricing{
		InputPricePerToken:         1.75e-6,
		OutputPricePerToken:        14e-6,
		CacheCreationPricePerToken: 1.75e-6,
		CacheReadPricePerToken:     0.175e-6,
		SupportsCacheBreakdown:     false,
	}
	// Codex 族兜底统一按 GPT-5.3 Codex 价格计费
	s.fallbackPrices["gpt-5.3-codex"] = &ModelPricing{
		InputPricePerToken:         1.5e-6, // $1.5 per MTok
		OutputPricePerToken:        12e-6,  // $12 per MTok
		CacheCreationPricePerToken: 1.5e-6, // $1.5 per MTok
		CacheReadPricePerToken:     0.15e-6,
		SupportsCacheBreakdown:     false,
	}

	// ============================================================
	// 国产 LLM 兜底定价（数据源：各家官方定价页/USD 口径）
	// 顺序：DeepSeek → 智谱 GLM → 月之暗面 Kimi → MiniMax
	// 覆盖逻辑见同文件 getFallbackPricing()
	// ============================================================

	// ---- DeepSeek 系列 ----
	// Source: https://api-docs.deepseek.com/quick_start/pricing
	// 官方口径（2026-09-10 公告降价后）：现行模型为 deepseek-flash（=
	// DeepSeek-V4.1-Flash，旧名 deepseek-v4-flash 兼容路由）/ deepseek-v4-pro /
	// deepseek-v4-flash-vision-exp；deepseek-chat / deepseek-reasoner 已停止服务，
	// 其余 deepseek-*（含未知型号）统一按 flash 价兜底（见 getFallbackPricing），
	// 避免计费中断。
	// 以下均为官方低谷价；高峰价 = 2× 低谷价（高峰时段 01:00–04:00
	// 与 06:00–10:00 UTC，仅工作日；北京时间周六/周日全天低谷），见 deepseekPeakMultiplierAt。
	s.fallbackPrices["deepseek-v4-pro"] = &ModelPricing{
		InputPricePerToken:     deepseekProOffPeakInputPrice,  // $0.66 per MTok (cache miss, off-peak)
		OutputPricePerToken:    deepseekProOffPeakOutputPrice, // $1.98 per MTok
		CacheReadPricePerToken: deepseekProOffPeakCacheRead,   // $0.022 per MTok (cache hit)
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["deepseek-v4-flash"] = &ModelPricing{
		InputPricePerToken:     deepseekFlashOffPeakInputPrice,  // $0.15 per MTok (cache miss, off-peak)
		OutputPricePerToken:    deepseekFlashOffPeakOutputPrice, // $0.60 per MTok
		CacheReadPricePerToken: deepseekFlashOffPeakCacheRead,   // $0.003 per MTok (cache hit)
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["deepseek-v4-flash-vision-exp"] = &ModelPricing{
		InputPricePerToken:     deepseekFlashOffPeakInputPrice,
		OutputPricePerToken:    deepseekFlashOffPeakOutputPrice,
		CacheReadPricePerToken: deepseekFlashOffPeakCacheRead,
		SupportsCacheBreakdown: false,
	}

	// ---- 智谱 GLM（Z.AI）----
	// Source: https://docs.z.ai/guides/overview/pricing (USD per 1M tokens)
	// 注意：CacheReadPricePerToken 即"缓存命中"价格，CacheCreationPricePerToken 留空（智谱未公开写入价，按 0 处理）。
	// GLM-4.6 与 GLM-4.5 在 z.ai 国际版上定价一致；GLM-4.5 国内按 ¥0.8/¥2，汇率换算后约 $0.112/$0.28，与国际版 $0.6/$2.2 不同，本分支采用国际版 USD 口径与现有 Claude/GPT 一致。
	// GLM-5.3 / GLM-5.2 与 GLM-5.1 在 z.ai 上同价。
	// GLM-5.3-Flash 列表价 $0.15/$0.50（2026-09-09 前五折促销，此处按列表价，与其它模型口径一致）。
	s.fallbackPrices["glm-5.3-flash"] = &ModelPricing{
		InputPricePerToken:     0.15e-6, // $0.15 per MTok
		OutputPricePerToken:    0.5e-6,  // $0.50 per MTok
		CacheReadPricePerToken: 0.03e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-5.3"] = &ModelPricing{
		InputPricePerToken:     1.4e-6, // $1.40 per MTok
		OutputPricePerToken:    4.4e-6, // $4.40 per MTok
		CacheReadPricePerToken: 0.26e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-5.2"] = &ModelPricing{
		InputPricePerToken:     1.4e-6, // $1.40 per MTok
		OutputPricePerToken:    4.4e-6, // $4.40 per MTok
		CacheReadPricePerToken: 0.26e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-5.1"] = &ModelPricing{
		InputPricePerToken:     1.4e-6, // $1.40 per MTok
		OutputPricePerToken:    4.4e-6, // $4.40 per MTok
		CacheReadPricePerToken: 0.26e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-5"] = &ModelPricing{
		InputPricePerToken:     1e-6, // $1.00 per MTok
		OutputPricePerToken:    3.2e-6,
		CacheReadPricePerToken: 0.2e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-5-turbo"] = &ModelPricing{
		InputPricePerToken:     1.2e-6,
		OutputPricePerToken:    4e-6,
		CacheReadPricePerToken: 0.24e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-4.7"] = &ModelPricing{
		InputPricePerToken:     0.6e-6, // $0.60 per MTok
		OutputPricePerToken:    2.2e-6,
		CacheReadPricePerToken: 0.11e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-4.7-flashx"] = &ModelPricing{
		InputPricePerToken:     0.07e-6, // $0.07 per MTok
		OutputPricePerToken:    0.4e-6,
		CacheReadPricePerToken: 0.01e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-4.6"] = &ModelPricing{
		InputPricePerToken:     0.6e-6, // $0.60 per MTok
		OutputPricePerToken:    2.2e-6,
		CacheReadPricePerToken: 0.11e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-4.5"] = &ModelPricing{
		InputPricePerToken:     0.6e-6, // $0.60 per MTok
		OutputPricePerToken:    2.2e-6,
		CacheReadPricePerToken: 0.11e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-4.5-x"] = &ModelPricing{
		InputPricePerToken:     2.2e-6, // $2.20 per MTok
		OutputPricePerToken:    8.9e-6,
		CacheReadPricePerToken: 0.45e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-4.5-air"] = &ModelPricing{
		InputPricePerToken:     0.2e-6, // $0.20 per MTok
		OutputPricePerToken:    1.1e-6,
		CacheReadPricePerToken: 0.03e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-4.5-airx"] = &ModelPricing{
		InputPricePerToken:     1.1e-6,
		OutputPricePerToken:    4.5e-6,
		CacheReadPricePerToken: 0.22e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-4-32b-0414-128k"] = &ModelPricing{
		InputPricePerToken:     0.1e-6, // $0.10 per MTok
		OutputPricePerToken:    0.1e-6,
		SupportsCacheBreakdown: false,
	}
	// GLM-4.5-Flash / GLM-4.7-Flash 在 z.ai 上为 Free，保留 zero-cost entry 防止未知 alias 误计费。
	s.fallbackPrices["glm-4.5-flash"] = &ModelPricing{
		InputPricePerToken:     0,
		OutputPricePerToken:    0,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["glm-4.7-flash"] = &ModelPricing{
		InputPricePerToken:     0,
		OutputPricePerToken:    0,
		SupportsCacheBreakdown: false,
	}

	// ---- 月之暗面 Kimi（K 系列）----
	// Source: https://platform.moonshot.cn/docs/pricing/overview (元/百万 tokens 口径)
	//       交叉验证：https://www.tmtpost.com/7961404.html (USD 口径)
	// Moonshot V1 (¥2/¥5/¥10 多 tier) 公开页未直接标注 USD 价，本分支不覆盖，避免误计价。
	// K2-0905 / K2-0711 官方页面未保留定价，不覆盖。
	// Kimi K3 国际站 USD 价目：https://platform.kimi.ai/docs/pricing/chat-k3.md
	// Kimi Code bare aliases（k3 / k3-256k）官方无按 token 价目；复用 API Platform
	// kimi-k3 档位作代理计费 fallback（同 kimi-for-coding 对 K2.6 的处理口径）。
	s.fallbackPrices["kimi-k3"] = &ModelPricing{
		InputPricePerToken:     3e-6,    // $3.00 per MTok (cache miss)
		OutputPricePerToken:    15e-6,   // $15.00 per MTok
		CacheReadPricePerToken: 0.30e-6, // $0.30 per MTok (cache hit)
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["kimi-k2.6"] = &ModelPricing{
		InputPricePerToken:     0.95e-6, // $0.95 per MTok (cache miss)
		OutputPricePerToken:    4e-6,    // $4.00 per MTok
		CacheReadPricePerToken: 0.15e-6, // $0.15 per MTok (cache hit, ¥1.10)
		SupportsCacheBreakdown: false,
	}
	// kimi-for-coding 走 Kimi Coding endpoint，按当前 K2.6 coding 档位兜底计费。
	s.fallbackPrices["kimi-for-coding"] = &ModelPricing{
		InputPricePerToken:     0.95e-6,
		OutputPricePerToken:    4e-6,
		CacheReadPricePerToken: 0.15e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["kimi-k2.5"] = &ModelPricing{
		InputPricePerToken:     0.60e-6, // $0.60 per MTok
		OutputPricePerToken:    3e-6,    // $3.00 per MTok
		CacheReadPricePerToken: 0.098e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["kimi-k2-thinking"] = &ModelPricing{
		InputPricePerToken:     0.56e-6, // ¥4/百万 ≈ $0.56
		OutputPricePerToken:    2.24e-6, // ¥16/百万
		CacheReadPricePerToken: 0.14e-6, // ¥1/百万
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["kimi-k2"] = &ModelPricing{
		InputPricePerToken:     0.56e-6, // ¥4/百万
		OutputPricePerToken:    2.24e-6, // ¥16/百万
		CacheReadPricePerToken: 0.14e-6, // ¥1/百万
		SupportsCacheBreakdown: false,
	}

	// ---- MiniMax M 系列 ----
	// Source: https://platform.minimax.io/docs/guides/pricing-paygo
	// 注意：MiniMax M3 在 >512K context 时价格翻倍，本兜底采用 ≤512K 标准 tier（保守口径，对用户有利）；
	// 需要时在模型目录给它配 >512K 的按 token 分段价。
	s.fallbackPrices["minimax-m3"] = &ModelPricing{
		InputPricePerToken:     0.60e-6, // $0.60 per MTok (≤512K standard tier, 含 50% 永久折扣前原价 $1.20)
		OutputPricePerToken:    2.40e-6,
		CacheReadPricePerToken: 0.12e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["minimax-m2.7"] = &ModelPricing{
		InputPricePerToken:     0.30e-6, // $0.30 per MTok
		OutputPricePerToken:    1.20e-6,
		CacheReadPricePerToken: 0.06e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["minimax-m2.7-highspeed"] = &ModelPricing{
		InputPricePerToken:     0.60e-6,
		OutputPricePerToken:    2.40e-6,
		CacheReadPricePerToken: 0.06e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["minimax-m2.5"] = &ModelPricing{
		InputPricePerToken:     0.30e-6,
		OutputPricePerToken:    1.20e-6,
		CacheReadPricePerToken: 0.03e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["minimax-m2.1"] = &ModelPricing{
		InputPricePerToken:     0.30e-6,
		OutputPricePerToken:    1.20e-6,
		CacheReadPricePerToken: 0.03e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["minimax-m2"] = &ModelPricing{
		InputPricePerToken:     0.30e-6,
		OutputPricePerToken:    1.20e-6,
		CacheReadPricePerToken: 0.03e-6,
		SupportsCacheBreakdown: false,
	}

	// ---- 火山方舟 豆包 Embedding（多模态向量化）----
	// doubao-embedding-vision 图文向量化：上游 usage 回传 prompt_tokens_details.{text_tokens,image_tokens}，
	// 按量付费官方价 文本 ¥0.7/MTok、图片 ¥1.8/MTok；汇率口径 ÷7.14（与本表其他国产模型一致，¥1≈$0.14）。
	// embedding 无 output，OutputPricePerToken 置 0。
	s.fallbackPrices["doubao-embedding-vision"] = &ModelPricing{
		InputPricePerToken:      0.098e-6, // ¥0.7/MTok ≈ $0.098（文本输入）
		ImageInputPricePerToken: 0.252e-6, // ¥1.8/MTok ≈ $0.252（图片输入）
		OutputPricePerToken:     0,
		SupportsCacheBreakdown:  false,
	}

	// xAI Grok 4.5: $2 input / $0.30 cached input / $6 output below 200k;
	// long-context rates are $4 / $0.60 / $12 (>=200k prompt tokens).
	s.fallbackPrices["grok-4.5"] = &ModelPricing{
		InputPricePerToken:     2e-6,
		OutputPricePerToken:    6e-6,
		CacheReadPricePerToken: 0.3e-6,
		SupportsCacheBreakdown: false,
	}

	// xAI Grok 4.6: $2 input / $0.50 cached input / $6 output below 200k;
	// long-context rates are $4 / $1 / $12 (>=200k prompt tokens).
	s.fallbackPrices["grok-4.6"] = &ModelPricing{
		InputPricePerToken:     2e-6,
		OutputPricePerToken:    6e-6,
		CacheReadPricePerToken: 0.5e-6,
		SupportsCacheBreakdown: false,
	}

	// xAI Grok 4.3: $1.25 input / $0.20 cached / $2.50 output below 200k;
	// long-context rates are $2.50 / $0.40 / $5.
	s.fallbackPrices["grok-4.3"] = &ModelPricing{
		InputPricePerToken:     1.25e-6,
		OutputPricePerToken:    2.5e-6,
		CacheReadPricePerToken: 0.2e-6,
		SupportsCacheBreakdown: false,
	}
	// Grok 4.20 variants share the official $1.25 / $0.20 / $2.50 card
	// (and $2.50 / $0.40 / $5 long-context rates) with Grok 4.3.
	s.fallbackPrices["grok-4.20"] = &ModelPricing{
		InputPricePerToken:     1.25e-6,
		OutputPricePerToken:    2.5e-6,
		CacheReadPricePerToken: 0.2e-6,
		SupportsCacheBreakdown: false,
	}

	// Keep legacy Grok 3 Mini requests on their own historical xAI price card;
	// otherwise the generic Grok fallback bills them as Grok 4.5.
	s.fallbackPrices["grok-3-mini"] = &ModelPricing{
		InputPricePerToken:     0.30e-6,
		OutputPricePerToken:    0.50e-6,
		CacheReadPricePerToken: 0.075e-6,
		SupportsCacheBreakdown: false,
	}
	s.fallbackPrices["grok-3-mini-fast"] = &ModelPricing{
		InputPricePerToken:     0.60e-6,
		OutputPricePerToken:    4e-6,
		CacheReadPricePerToken: 0.15e-6,
		SupportsCacheBreakdown: false,
	}
	// xAI Grok Build 0.1 (official docs: $1 input / $0.20 cached input /
	// $2 output per MTok). Composer is available only through Grok Build and
	// has no standalone public API rate card, so its aliases use this coding
	// model rate instead of silently billing at zero.
	s.fallbackPrices["grok-build-0.1"] = &ModelPricing{
		InputPricePerToken:     1e-6,
		OutputPricePerToken:    2e-6,
		CacheReadPricePerToken: 0.2e-6,
		SupportsCacheBreakdown: false,
	}
}

// getFallbackPricing 根据模型系列获取回退价格
func (s *BillingService) getFallbackPricing(model string) *ModelPricing {
	modelLower := strings.ToLower(model)

	// 按模型系列匹配
	if isClaudeFable51Model(modelLower) {
		return s.fallbackPrices["claude-fable-5-1"]
	}
	if strings.Contains(modelLower, "fable-5") || strings.Contains(modelLower, "fable5") {
		return s.fallbackPrices["claude-fable-5"]
	}
	if strings.Contains(modelLower, "opus") {
		// "opus-5" 必须先判：不能用裸 "5" 匹配，否则 claude-opus-4-5 会被误判。
		if strings.Contains(modelLower, "opus-5") || strings.Contains(modelLower, "opus5") {
			return s.fallbackPrices["claude-opus-5"]
		}
		if strings.Contains(modelLower, "4.8") || strings.Contains(modelLower, "4-8") {
			return s.fallbackPrices["claude-opus-4.8"]
		}
		if strings.Contains(modelLower, "4.7") || strings.Contains(modelLower, "4-7") {
			return s.fallbackPrices["claude-opus-4.7"]
		}
		if strings.Contains(modelLower, "4.6") || strings.Contains(modelLower, "4-6") {
			return s.fallbackPrices["claude-opus-4.6"]
		}
		if strings.Contains(modelLower, "4.5") || strings.Contains(modelLower, "4-5") {
			return s.fallbackPrices["claude-opus-4.5"]
		}
		return s.fallbackPrices["claude-3-opus"]
	}
	if strings.Contains(modelLower, "sonnet") {
		if strings.Contains(modelLower, "4") && !strings.Contains(modelLower, "3") {
			return s.fallbackPrices["claude-sonnet-4"]
		}
		return s.fallbackPrices["claude-3-5-sonnet"]
	}
	if strings.Contains(modelLower, "haiku") {
		if strings.Contains(modelLower, "3-5") || strings.Contains(modelLower, "3.5") {
			return s.fallbackPrices["claude-3-5-haiku"]
		}
		return s.fallbackPrices["claude-3-haiku"]
	}
	// Claude 未知型号统一回退到 Sonnet，避免计费中断。
	if strings.Contains(modelLower, "claude") {
		return s.fallbackPrices["claude-sonnet-4"]
	}
	if strings.Contains(modelLower, "gemini-3.1-pro") || strings.Contains(modelLower, "gemini-3-1-pro") {
		return s.fallbackPrices["gemini-3.1-pro"]
	}
	if strings.Contains(modelLower, "gemini-3.6-flash") || strings.Contains(modelLower, "gemini-3-6-flash") {
		return s.fallbackPrices["gemini-3.6-flash"]
	}
	if strings.Contains(modelLower, "gemini-3.7-flash") || strings.Contains(modelLower, "gemini-3-7-flash") {
		return s.fallbackPrices["gemini-3.7-flash"]
	}
	if strings.Contains(modelLower, "gemini-3.8-flash") || strings.Contains(modelLower, "gemini-3-8-flash") {
		return s.fallbackPrices["gemini-3.8-flash"]
	}

	// DeepSeek 系列：官方模型 V4 Pro/Flash（含 vision-exp）按各自价卡；
	// 其余 deepseek-*（含已停服的 deepseek-chat / deepseek-reasoner 与未知型号）
	// 统一按 flash 价兜底，避免计费中断。新名字由 fallback warn 日志
	// （每模型每进程一条）暴露，运营者据此更新价卡。
	// "deepseek-v4-flash-vision-exp" 含 "deepseek-v4-flash" 子串，显式分支置于 flash 之前，语义清晰。
	if strings.Contains(modelLower, "deepseek-v4-flash-vision-exp") {
		return s.fallbackPrices["deepseek-v4-flash-vision-exp"]
	}
	if strings.Contains(modelLower, "deepseek-v4-flash") {
		return s.fallbackPrices["deepseek-v4-flash"]
	}
	if strings.Contains(modelLower, "deepseek-v4-pro") {
		return s.fallbackPrices["deepseek-v4-pro"]
	}
	if strings.HasPrefix(modelLower, "deepseek-") {
		return s.fallbackPrices["deepseek-v4-flash"]
	}

	// ---- 国产 LLM 兜底匹配 ----
	// 匹配策略：长 key 优先（具体模型 → 系列 / 厂商），未知型号不回退以避免误计价。
	// 与 DeepSeek 一样采用"白名单"语义：未在本表命中的国产模型 alias 一律不返回兜底价。

	// 智谱 GLM（z.ai 公开 SKU：glm-5.3 / glm-5.3-flash / glm-5.2 / glm-5.1 / glm-5 / glm-5-turbo / glm-4.7 / glm-4.6 / glm-4.5 等）
	// 匹配顺序：先判别最高 tier，再依次降级。
	// 注意：带小数点的型号必须排在裸 "glm-5" 之前，否则会被 strings.Contains 抢走；
	// glm-5.3-flash 必须排在 glm-5.3 之前（前者包含后者子串）。
	if strings.Contains(modelLower, "glm-5.3-flash") || strings.Contains(modelLower, "glm-5.3flash") {
		return s.fallbackPrices["glm-5.3-flash"]
	}
	if strings.Contains(modelLower, "glm-5.3") {
		return s.fallbackPrices["glm-5.3"]
	}
	if strings.Contains(modelLower, "glm-5.2") {
		return s.fallbackPrices["glm-5.2"]
	}
	if strings.Contains(modelLower, "glm-5.1") {
		return s.fallbackPrices["glm-5.1"]
	}
	if strings.Contains(modelLower, "glm-5-turbo") || strings.Contains(modelLower, "glm-5turbo") {
		return s.fallbackPrices["glm-5-turbo"]
	}
	if strings.Contains(modelLower, "glm-5") {
		return s.fallbackPrices["glm-5"]
	}
	if strings.Contains(modelLower, "glm-4.7-flashx") {
		return s.fallbackPrices["glm-4.7-flashx"]
	}
	if strings.Contains(modelLower, "glm-4.7-flash") {
		return s.fallbackPrices["glm-4.7-flash"]
	}
	if strings.Contains(modelLower, "glm-4.7") {
		return s.fallbackPrices["glm-4.7"]
	}
	if strings.Contains(modelLower, "glm-4.6") {
		return s.fallbackPrices["glm-4.6"]
	}
	if strings.Contains(modelLower, "glm-4.5-flash") {
		return s.fallbackPrices["glm-4.5-flash"]
	}
	if strings.Contains(modelLower, "glm-4.5-x") || strings.Contains(modelLower, "glm-4.5x") {
		return s.fallbackPrices["glm-4.5-x"]
	}
	if strings.Contains(modelLower, "glm-4.5-airx") || strings.Contains(modelLower, "glm-4.5airx") {
		return s.fallbackPrices["glm-4.5-airx"]
	}
	if strings.Contains(modelLower, "glm-4.5-air") || strings.Contains(modelLower, "glm-4.5air") {
		return s.fallbackPrices["glm-4.5-air"]
	}
	if strings.Contains(modelLower, "glm-4.5") {
		return s.fallbackPrices["glm-4.5"]
	}
	if strings.Contains(modelLower, "glm-4-32b") {
		return s.fallbackPrices["glm-4-32b-0414-128k"]
	}

	// 月之暗面 Kimi（kimi-k3 / k3 / k3-256k / kimi-k2.6 / kimi-for-coding / kimi-k2.5 / kimi-k2-thinking / kimi-k2）
	// K2-0905 / K2-0711 官方未保留定价，不进入 fallback。
	// K3 规则置于 K2 前：API Platform 仅官方 kimi-k3（及 / 路径后缀）；
	// Code bare aliases 仅精确 k3 / k3-256k 或 /k3|/k3-256k 后缀，避免 kimi-k30 等未知型号误命中。
	// 注意：kimi-k3[1m] 是 Claude Code 上下文选择语法，不是 Kimi API 模型 ID，不进入 fallback。
	if strings.Contains(modelLower, "kimi-for-coding") {
		return s.fallbackPrices["kimi-for-coding"]
	}
	if modelLower == "kimi-k3" || strings.HasSuffix(modelLower, "/kimi-k3") ||
		modelLower == "k3" || modelLower == "k3-256k" ||
		strings.HasSuffix(modelLower, "/k3") || strings.HasSuffix(modelLower, "/k3-256k") {
		return s.fallbackPrices["kimi-k3"]
	}
	if strings.Contains(modelLower, "kimi-k2.6") || strings.Contains(modelLower, "kimi-k2-6") {
		return s.fallbackPrices["kimi-k2.6"]
	}
	if strings.Contains(modelLower, "kimi-k2.5") || strings.Contains(modelLower, "kimi-k2-5") {
		return s.fallbackPrices["kimi-k2.5"]
	}
	if strings.Contains(modelLower, "kimi-k2-thinking") || strings.Contains(modelLower, "kimi-k2-thinking-") {
		return s.fallbackPrices["kimi-k2-thinking"]
	}
	if strings.Contains(modelLower, "kimi-k2") || strings.Contains(modelLower, "kimi/k2") {
		return s.fallbackPrices["kimi-k2"]
	}

	// MiniMax M 系列（M3 / M2.7 / M2.5 / M2.1 / M2；含 highspeed 变体）
	if strings.Contains(modelLower, "minimax-m3") {
		return s.fallbackPrices["minimax-m3"]
	}
	if strings.Contains(modelLower, "minimax-m2.7-highspeed") || strings.Contains(modelLower, "minimax-m2-7-highspeed") {
		return s.fallbackPrices["minimax-m2.7-highspeed"]
	}
	if strings.Contains(modelLower, "minimax-m2.7") || strings.Contains(modelLower, "minimax-m2-7") {
		return s.fallbackPrices["minimax-m2.7"]
	}
	if strings.Contains(modelLower, "minimax-m2.5") || strings.Contains(modelLower, "minimax-m2-5") {
		return s.fallbackPrices["minimax-m2.5"]
	}
	if strings.Contains(modelLower, "minimax-m2.1") || strings.Contains(modelLower, "minimax-m2-1") {
		return s.fallbackPrices["minimax-m2.1"]
	}
	if strings.Contains(modelLower, "minimax-m2") || strings.Contains(modelLower, "minimax-m-2") {
		return s.fallbackPrices["minimax-m2"]
	}

	// 火山方舟 豆包 Embedding（多模态向量化）。
	// most-specific-first：放在未来任何 doubao-embedding / doubao 宽匹配之前。
	// 覆盖带版本后缀的别名（如 doubao-embedding-vision-251215）。
	if strings.Contains(modelLower, "doubao-embedding-vision") {
		return s.fallbackPrices["doubao-embedding-vision"]
	}

	// OpenAI（GPT-5 / Codex 族）：仅匹配已知型号，避免未知 OpenAI 型号误计价。
	if normalized := normalizeKnownOpenAICodexModel(modelLower); normalized != "" {
		switch normalized {
		case "gpt-6-astra":
			return s.fallbackPrices["gpt-6-astra"]
		case "gpt-5.6-sol":
			return s.fallbackPrices["gpt-5.6-sol"]
		case "gpt-5.6-terra":
			return s.fallbackPrices["gpt-5.6-terra"]
		case "gpt-5.6-luna":
			return s.fallbackPrices["gpt-5.6-luna"]
		case "gpt-5.5-pro":
			return s.fallbackPrices["gpt-5.5-pro"]
		case "gpt-5.5":
			return s.fallbackPrices["gpt-5.5"]
		case "gpt-5.4-mini":
			return s.fallbackPrices["gpt-5.4-mini"]
		case "gpt-5.4-nano":
			return s.fallbackPrices["gpt-5.4-nano"]
		case "gpt-5.4":
			return s.fallbackPrices["gpt-5.4"]
		case "gpt-5.2":
			return s.fallbackPrices["gpt-5.2"]
		case "gpt-5.3-codex", "gpt-5.3-codex-spark":
			return s.fallbackPrices["gpt-5.3-codex"]
		}
	}

	switch modelLower {
	case "grok", "grok-latest", "grok-4.6", "grok-4.6-latest":
		return s.fallbackPrices["grok-4.6"]
	case "grok-4.5", "grok-4.5-latest":
		return s.fallbackPrices["grok-4.5"]
	case "grok-3-mini":
		return s.fallbackPrices["grok-3-mini"]
	case "grok-3-mini-fast":
		return s.fallbackPrices["grok-3-mini-fast"]
	case "grok-4.3":
		return s.fallbackPrices["grok-4.3"]
	case "grok-4.20-0309-reasoning",
		"grok-4.20-0309-non-reasoning",
		"grok-4.20-multi-agent-0309",
		"grok-4.20-reasoning",
		"grok-4.20-non-reasoning":
		return s.fallbackPrices["grok-4.20"]
	case "grok-build", "grok-build-latest", "grok-build-0.1", "grok-composer", "grok-composer-2.5-fast", "composer-2.5":
		return s.fallbackPrices["grok-build-0.1"]
	}

	// Unknown Grok text IDs (grok-5, dated snapshots, provider-prefixed) inherit
	// the current default text card so a new model cannot ship unbilled.
	if pricing := s.grokUnknownTextFamilyFallback(modelLower); pricing != nil {
		return pricing
	}

	return nil
}

func (s *BillingService) grokUnknownTextFamilyFallback(model string) *ModelPricing {
	if s == nil || !isGrokUnknownTextFamilyModel(model) {
		return nil
	}
	return s.fallbackPrices["grok-4.6"]
}

func isGrokUnknownTextFamilyModel(model string) bool {
	native := strings.ToLower(strings.TrimSpace(xai.StripGrokProviderPrefix(model)))
	if isGrokMediaFamilyModel(native) {
		return false
	}
	switch {
	case native == "grok", native == "grok-latest":
		return true
	case strings.HasPrefix(native, "grok-build"),
		strings.HasPrefix(native, "grok-composer"),
		strings.HasPrefix(native, "composer-"):
		return true
	case len(native) > 5 && strings.HasPrefix(native, "grok-"):
		rest := native[len("grok-"):]
		return rest[0] >= '0' && rest[0] <= '9'
	default:
		return false
	}
}

// isGrokMediaFamilyModel matches ids that are billed per image/video/audio unit
// rather than per token, so version-numbered media ids (grok-2-image-1212,
// grok-5-video) cannot slip into the unknown-text fallback and pick up a token
// card. "vision" is deliberately absent: multimodal chat models are token billed.
func isGrokMediaFamilyModel(native string) bool {
	for _, marker := range []string{"imagine", "image", "video", "audio", "speech", "tts", "transcribe", "realtime"} {
		if strings.Contains(native, marker) {
			return true
		}
	}
	return false
}

// HasIdentifiedTokenPricing 判断模型能否在价格表中被"确定性识别"出 token 价格。
//
// 与 GetModelPricing 的关键区别：本函数拒绝按子串猜系列的兜底。GetModelPricing 会
// 让任意含 "haiku"/"opus"/"claude" 的名字（哪怕是不存在的型号）落到 getFallbackPricing
// 的系列兜底价上，因此凡是模型名来自外部、且"能查到价"会直接影响计费金额的场景
// （如按上游响应自报模型计费），都必须用本函数而不是 GetModelPricing 做准入判断。
func (s *BillingService) HasIdentifiedTokenPricing(model string) bool {
	if s == nil {
		return false
	}
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return false
	}
	if s.pricingService != nil {
		// 仅有图片价的条目不能用于 token 计费，口径与 GetModelPricing 保持一致。
		if pricing := s.pricingService.GetIdentifiedModelPricing(model); pricing != nil && !pricing.TokenPricingAbsent {
			return true
		}
	}
	pricing, ok := s.fallbackPrices[model]
	return ok && pricing != nil
}

// SnapshotFallbackPricing 返回硬编码兜底价表的浅拷贝（键 → 条目副本），
// 供模型目录播种使用。只含字面键，不含 getFallbackPricing 的系列子串兜底——
// 子串兜底匹配的是不存在的型号名，播进目录只会凭空造出模型。
func (s *BillingService) SnapshotFallbackPricing() map[string]*ModelPricing {
	if s == nil {
		return nil
	}
	out := make(map[string]*ModelPricing, len(s.fallbackPrices))
	for name, pricing := range s.fallbackPrices {
		if pricing == nil {
			continue
		}
		cloned := *pricing
		out[name] = &cloned
	}
	return out
}

// GetModelPricing 获取模型价格配置
func (s *BillingService) GetModelPricing(model string) (*ModelPricing, error) {
	// 无显式计费时点，DeepSeek pro→Flash 切换按当前时刻判定。
	return s.getModelPricingAt(model, timezone.Now())
}

// getModelPricingAt 是 GetModelPricing 的带计费时点内部变体：pricingAt 显式
// 驱动 DeepSeek pro→Flash 切换判定（切换点前 Pro 价、之后 Flash 价），使
// 展示/估算路径可与历史补账同刻复算，测试也能用固定时点钉住断言。
func (s *BillingService) getModelPricingAt(model string, pricingAt time.Time) (*ModelPricing, error) {
	// 标准化模型名称（转小写）
	model = strings.ToLower(model)

	// 1. 优先从动态价格服务获取
	if s.pricingService != nil {
		litellmPricing := s.pricingService.GetModelPricing(model)
		// 仅有图片价、无 token 价的条目（如 LiteLLM 的 imagen 类模型）不能用于
		// token 计费：直接返回会把 token 流量按 $0 计费。跳过后走 fallback，
		// 无 fallback 则 fail-closed（ErrModelPricingUnavailable）。
		// 图片计费路径（getDefaultImagePrice / getImageUnitPrice）直接读
		// PricingService，不受影响。
		if litellmPricing != nil && litellmPricing.TokenPricingAbsent {
			litellmPricing = nil
		}
		if litellmPricing != nil {
			// 启用 5m/1h 分类计费的条件：
			// 1. 存在 1h 价格
			// 2. 1h 价格 > 5m 价格（防止 LiteLLM 数据错误导致少收费）
			price5m := litellmPricing.CacheCreationInputTokenCost
			price1h := litellmPricing.CacheCreationInputTokenCostAbove1hr
			enableBreakdown := price1h > 0 && price1h > price5m
			return s.applyModelSpecificPricingPolicyEx(model, &ModelPricing{
				InputPricePerToken:          litellmPricing.InputCostPerToken,
				OutputPricePerToken:         litellmPricing.OutputCostPerToken,
				CacheCreationPricePerToken:  litellmPricing.CacheCreationInputTokenCost,
				CacheReadPricePerToken:      litellmPricing.CacheReadInputTokenCost,
				CacheCreation5mPrice:        price5m,
				CacheCreation1hPrice:        price1h,
				SupportsCacheBreakdown:      enableBreakdown,
				ImageInputPricePerToken:     litellmPricing.InputCostPerImageToken,
				ImageCacheReadPricePerToken: litellmPricing.CacheReadInputImageTokenCost,
				ImageOutputPricePerToken:    litellmPricing.OutputCostPerImageToken,
				AudioInputPricePerToken:     litellmPricing.InputCostPerAudioToken,
				AudioOutputPricePerToken:    litellmPricing.OutputCostPerAudioToken,
			}, true, pricingAt), nil
		}
	}

	// 2. 使用硬编码回退价格
	fallback := s.getFallbackPricing(model)
	if fallback != nil {
		// 按模型名去重:每个模型每进程最多打一条 warn,避免热路径每请求刷屏（issue #3394）。
		// model 在函数入口已 ToLower,故 GLM-5.2 / glm-5.2 视为同一条目。
		if _, seen := s.fallbackWarnSeen.LoadOrStore(model, struct{}{}); !seen {
			log.Printf("[Billing] Using fallback pricing for model: %s", model)
		}
		return s.applyModelSpecificPricingPolicyEx(model, fallback, true, pricingAt), nil
	}

	return nil, fmt.Errorf("%w for model: %s", ErrModelPricingUnavailable, model)
}

// GetModelPricingWithChannel 获取模型定价，渠道配置的价格覆盖默认值
// 渠道存在时，未配置的图片输出价格归零（不回退到 LiteLLM）
func (s *BillingService) GetModelPricingWithChannel(model string, channelPricing *PricingCard) (*ModelPricing, error) {
	pricing, err := s.GetModelPricing(model)
	if err != nil {
		return nil, err
	}
	if channelPricing == nil {
		return pricing, nil
	}
	// 防止修改 fallbackPrices 中的共享指针
	cloned := *pricing
	pricing = &cloned
	applyChannelTokenPriceOverrides(pricing, channelPricing)
	if channelPricing.MaxReasoningEffortMultiplier != nil {
		pricing.MaxReasoningEffortMultiplier = channelPricing.MaxReasoningEffortMultiplier
	}
	if channelPricing.ImageOutputPrice != nil {
		pricing.ImageOutputPricePerToken = *channelPricing.ImageOutputPrice
	} else {
		pricing.ImageOutputPricePerToken = 0
	}
	pricing.ImageOutputPriceExplicit = true
	applyConfiguredImageInputPrice(channelPricing, pricing)
	return pricing, nil
}

// applyConfiguredImageInputPrice 应用渠道价卡的图片输入价：显式配置则用配置值；
// 未配置时归零，使 computeTokenBreakdown 回退到文本输入价。
func applyConfiguredImageInputPrice(chPricing *PricingCard, pricing *ModelPricing) {
	if chPricing != nil && chPricing.ImageInputPrice != nil {
		pricing.ImageInputPricePerToken = *chPricing.ImageInputPrice
	} else {
		pricing.ImageInputPricePerToken = 0
	}
}

func applyChannelTokenPriceOverrides(pricing *ModelPricing, channelPricing *PricingCard) {
	if pricing == nil || channelPricing == nil {
		return
	}
	if channelPricing.InputPrice != nil {
		pricing.InputPricePerToken = *channelPricing.InputPrice
	}
	if channelPricing.OutputPrice != nil {
		pricing.OutputPricePerToken = *channelPricing.OutputPrice
	}
	if channelPricing.CacheWritePrice != nil {
		pricing.CacheCreationPricePerToken = *channelPricing.CacheWritePrice
		pricing.CacheCreationPriceExplicit = true
		pricing.CacheCreation5mPrice = *channelPricing.CacheWritePrice
		if channelPricing.CacheWrite1hPrice == nil {
			// Preserve the pre-split behavior for existing configurations: a lone
			// cache_write_price continues to override both TTL tiers.
			pricing.CacheCreation1hPrice = *channelPricing.CacheWritePrice
		}
	}
	if channelPricing.CacheWrite1hPrice != nil {
		pricing.CacheCreation1hPrice = *channelPricing.CacheWrite1hPrice
		pricing.SupportsCacheBreakdown = true
	}
	if channelPricing.CacheReadPrice != nil {
		pricing.CacheReadPricePerToken = *channelPricing.CacheReadPrice
	}
}

// --- 统一计费入口 ---

// CostInput 统一计费输入
type CostInput struct {
	Ctx             context.Context
	Model           string
	Tokens          UsageTokens
	RequestCount    int     // 按次计费时使用
	UsageUnits      float64 // 音频等连续计量单位（分钟/小时/百万字符）
	SizeTier        string  // 按次/图片模式的层级标签（"1K","2K","4K","HD" 等）
	RateMultiplier  float64
	PricingAt       time.Time             // 渠道分时定价使用的计费时刻
	ReasoningEffort string                // 最终转发的推理等级；max 可触发模型的最高推理倍率
	Resolver        *ModelPricingResolver // 定价解析器
	Resolved        *ResolvedPricing      // 可选：预解析的定价结果（避免重复 Resolve 调用）
}

// CalculateCostUnified 统一计费入口，支持三种计费模式。
// 使用 ModelPricingResolver 解析定价，然后根据 BillingMode 分发计算。
func (s *BillingService) CalculateCostUnified(input CostInput) (*CostBreakdown, error) {
	if input.Resolver == nil {
		// 无 Resolver，回退到旧路径
		breakdown, err := s.calculateCostInternal(
			input.Model,
			input.Tokens,
			input.RateMultiplier,
			nil,
		)
		if err == nil {
			applyCostBreakdownMultiplier(breakdown, maxReasoningEffortBillingMultiplier(input.Model, input.ReasoningEffort, nil))
		}
		return breakdown, err
	}

	// 优先使用预解析结果，避免重复 Resolve 调用
	resolved := input.Resolved
	if resolved == nil {
		resolved = input.Resolver.Resolve(input.Ctx, PricingInput{Model: input.Model})
	}

	// 保存时强制 > 0；若仍有负数泄漏（缓存/迁移残留），按 0 处理避免按 1x 误扣。
	if input.RateMultiplier < 0 {
		input.RateMultiplier = 0
	}

	var breakdown *CostBreakdown
	var err error
	switch resolved.Mode {
	case BillingModePerRequest, BillingModeImage, BillingModeVideo:
		breakdown, err = s.calculatePerRequestCost(resolved, input)
	default: // BillingModeToken
		breakdown, err = s.calculateTokenCost(resolved, input)
	}
	if err == nil && breakdown != nil {
		breakdown.BillingMode = string(resolved.Mode)
		if breakdown.BillingMode == "" {
			breakdown.BillingMode = string(BillingModeToken)
		}
	}
	return breakdown, err
}

// calculateTokenCost 按 token 区间计费
func (s *BillingService) calculateTokenCost(resolved *ResolvedPricing, input CostInput) (*CostBreakdown, error) {
	totalContext := input.Tokens.InputTokens + input.Tokens.CacheCreationTokens + input.Tokens.CacheReadTokens

	// 按 token 分段：本次请求的输入侧 token 数落在哪一段就整条按那一段的价（目录条目的分段；没有分段按基础价）。
	pricing := input.Resolver.GetIntervalPricing(resolved, totalContext)
	if pricing == nil {
		return nil, fmt.Errorf("no pricing available for model: %s: %w", input.Model, ErrModelPricingUnavailable)
	}

	// 计费时点：优先请求级 PricingAt（历史补账与 DeepSeek pro→Flash 切换判定
	// 同源），零值回退当前时刻。
	pricingAt := input.PricingAt
	if pricingAt.IsZero() {
		pricingAt = timezone.Now()
	}

	// 平台默认价卡应用 DeepSeek 官方价强制覆盖（幂等，GetModelPricing 内部已强制过）；
	// 运营者定价（分组价卡、被管理员改过的目录条目）保留运营者配置，不强制覆盖官方价。
	// 播种出来的目录条目与价格文件同源，仍属平台默认价卡，所以按 operatorPricing 判定
	// 而不是按 Source——否则播种一上线，官方价政策就会整体失效。
	// 厂商政策按 CanonicalModel 判定：请求名可能是目录别名，政策要看它指向的条目。
	pricing = s.applyModelSpecificPricingPolicyEx(resolved.CanonicalModel, pricing, !resolved.operatorPricing, pricingAt)

	// DeepSeek 模型默认价卡按官方峰谷口径调整：高峰时段（01:00–04:00 与
	// 06:00–10:00 UTC，仅工作日；北京时间周末全天低谷）按 2× 低谷价计费。
	// 仅作用于平台默认价卡——运营者定价保持运营者语义，不叠加。
	// 先克隆再乘，避免污染共享 fallbackPrices 指针。
	if !resolved.operatorPricing && isDeepSeekModel(resolved.CanonicalModel) {
		if mult := deepseekPeakMultiplierAt(pricingAt); mult > 1 {
			cloned := *pricing
			cloned.InputPricePerToken *= mult
			cloned.OutputPricePerToken *= mult
			cloned.CacheReadPricePerToken *= mult
			pricing = &cloned
		}
	}

	breakdown := s.computeTokenBreakdown(pricing, input.Tokens, input.RateMultiplier)
	applyCostBreakdownMultiplier(breakdown, resolvedTimePricingMultiplier(resolved, input.PricingAt))
	applyCostBreakdownMultiplier(breakdown, maxReasoningEffortBillingMultiplier(resolved.CanonicalModel, input.ReasoningEffort, pricing))
	return breakdown, nil
}

// computeTokenBreakdown 是 token 计费的核心逻辑，由 calculateTokenCost 和 calculateCostInternal 共用。
// 分段价在调用前已由 GetIntervalPricing 选好，这里只按传入的价卡算。
func (s *BillingService) computeTokenBreakdown(
	pricing *ModelPricing, tokens UsageTokens,
	rateMultiplier float64,
) *CostBreakdown {
	// 保存时强制 > 0；若仍有负数泄漏，按 0 处理避免按 1x 误扣。
	if rateMultiplier < 0 {
		rateMultiplier = 0
	}

	inputPrice := pricing.InputPricePerToken
	outputPrice := pricing.OutputPricePerToken
	cacheReadPrice := pricing.CacheReadPricePerToken
	cacheCreationPrice := pricing.CacheCreationPricePerToken

	bd := &CostBreakdown{}
	// 分离图片输入 token 与文本输入 token（多模态 embedding、图片编辑等图文不同价场景）。
	// InputCost 不含图片输入，图片输入费用单独记入 ImageInputCost，便于对账；总额不变。
	// ImageInputTokens 为 0 时（绝大多数 chat/vision 流量）走原始单价路径，行为不变。
	textInputTokens := tokens.InputTokens
	if tokens.ImageInputTokens > 0 {
		imageInputTokens := tokens.ImageInputTokens
		textInputTokens = tokens.InputTokens - imageInputTokens
		if textInputTokens < 0 {
			textInputTokens = 0
			imageInputTokens = tokens.InputTokens
		}
		imageInputPrice := pricing.ImageInputPricePerToken
		if imageInputPrice == 0 {
			// 未配置图片输入档时回退到文本 input 价（已含分段调整）
			imageInputPrice = inputPrice
		}
		bd.ImageInputCost = float64(imageInputTokens) * imageInputPrice
	}
	// 音频输入 token 计在输入总数里：剥出来按音频价计，费用仍并入 InputCost（用量行不单列），
	// AudioInputCost 只做明细。未配音频价时回退到文本 input 价，与剥离前同价（不会变成 0）。
	audioInputTokens := min(max(tokens.AudioInputTokens, 0), max(textInputTokens, 0))
	textInputTokens -= audioInputTokens
	bd.InputCost = float64(textInputTokens) * inputPrice
	if audioInputTokens > 0 {
		audioInputPrice := pricing.AudioInputPricePerToken
		if audioInputPrice <= 0 {
			audioInputPrice = inputPrice
		}
		bd.AudioInputCost = float64(audioInputTokens) * audioInputPrice
		bd.InputCost += bd.AudioInputCost
	}

	// 分离图片输出 token 与文本输出 token
	textOutputTokens := tokens.OutputTokens - tokens.ImageOutputTokens
	if textOutputTokens < 0 {
		textOutputTokens = 0
	}
	// 音频输出同理：从文本输出里剥出来按音频价计，并入 OutputCost；未配音频价回退文本 output 价。
	audioOutputTokens := min(max(tokens.AudioOutputTokens, 0), textOutputTokens)
	textOutputTokens -= audioOutputTokens
	bd.OutputCost = float64(textOutputTokens) * outputPrice
	if audioOutputTokens > 0 {
		audioOutputPrice := pricing.AudioOutputPricePerToken
		if audioOutputPrice <= 0 {
			audioOutputPrice = outputPrice
		}
		bd.AudioOutputCost = float64(audioOutputTokens) * audioOutputPrice
		bd.OutputCost += bd.AudioOutputCost
	}

	// 图片输出 token 费用（独立费率）
	if tokens.ImageOutputTokens > 0 {
		imgPrice := pricing.ImageOutputPricePerToken
		if imgPrice == 0 && !pricing.ImageOutputPriceExplicit {
			imgPrice = outputPrice
		}
		bd.ImageOutputCost = float64(tokens.ImageOutputTokens) * imgPrice
	}

	// 缓存创建费用
	bd.CacheCreationCost = s.computeCacheCreationCost(pricing, tokens, cacheCreationPrice)

	bd.CacheReadCost = float64(tokens.CacheReadTokens) * cacheReadPrice
	if imageCached := min(max(tokens.ImageCacheReadTokens, 0), max(tokens.CacheReadTokens, 0)); imageCached > 0 && pricing.ImageCacheReadPricePerToken > 0 {
		bd.CacheReadCost = float64(tokens.CacheReadTokens-imageCached)*cacheReadPrice + float64(imageCached)*pricing.ImageCacheReadPricePerToken
	}

	bd.TotalCost = bd.InputCost + bd.ImageInputCost + bd.OutputCost + bd.ImageOutputCost +
		bd.CacheCreationCost + bd.CacheReadCost
	bd.ActualCost = bd.TotalCost * rateMultiplier

	return bd
}

// computeCacheCreationCost 计算缓存创建费用（支持 5m/1h 分类或标准计费）。
func (s *BillingService) computeCacheCreationCost(pricing *ModelPricing, tokens UsageTokens, price float64) float64 {
	if pricing.SupportsCacheBreakdown && (pricing.CacheCreation5mPrice > 0 || pricing.CacheCreation1hPrice > 0) {
		cacheCreation5mTokens, cacheCreation1hTokens := normalizeCacheCreationBreakdown(tokens)
		if cacheCreation5mTokens == 0 && cacheCreation1hTokens == 0 && tokens.CacheCreationTokens > 0 {
			// API 未返回 ephemeral 明细，回退到全部按 5m 单价计费
			return float64(tokens.CacheCreationTokens) * pricing.CacheCreation5mPrice
		}
		return float64(cacheCreation5mTokens)*pricing.CacheCreation5mPrice +
			float64(cacheCreation1hTokens)*pricing.CacheCreation1hPrice
	}
	return float64(tokens.CacheCreationTokens) * price
}

// normalizeCacheCreationBreakdown caps contradictory 5m/1h details at an explicitly
// positive aggregate while retaining their reported ratio as closely as integer tokens allow.
func normalizeCacheCreationBreakdown(tokens UsageTokens) (int, int) {
	cacheCreation5mTokens := tokens.CacheCreation5mTokens
	cacheCreation1hTokens := tokens.CacheCreation1hTokens
	aggregate := tokens.CacheCreationTokens
	if cacheCreation5mTokens < 0 {
		cacheCreation5mTokens = 0
	}
	if cacheCreation1hTokens < 0 {
		cacheCreation1hTokens = 0
	}
	if aggregate <= 0 || (cacheCreation5mTokens <= aggregate && cacheCreation1hTokens <= aggregate-cacheCreation5mTokens) {
		return cacheCreation5mTokens, cacheCreation1hTokens
	}

	detailTotal := float64(cacheCreation5mTokens) + float64(cacheCreation1hTokens)
	normalized5mTokens := math.Round(float64(aggregate) * float64(cacheCreation5mTokens) / detailTotal)
	if normalized5mTokens >= float64(aggregate) {
		cacheCreation5mTokens = aggregate
	} else {
		cacheCreation5mTokens = int(normalized5mTokens)
	}
	return cacheCreation5mTokens, aggregate - cacheCreation5mTokens
}

// calculatePerRequestCost 按次/图片计费
func (s *BillingService) calculatePerRequestCost(resolved *ResolvedPricing, input CostInput) (*CostBreakdown, error) {
	units := input.UsageUnits
	if units <= 0 {
		count := input.RequestCount
		if count <= 0 {
			count = 1
		}
		units = float64(count)
	}

	var unitPrice float64

	if input.SizeTier != "" {
		unitPrice = input.Resolver.GetRequestTierPrice(resolved, input.SizeTier)
	}

	if unitPrice == 0 {
		totalContext := input.Tokens.InputTokens + input.Tokens.CacheCreationTokens + input.Tokens.CacheReadTokens
		unitPrice = input.Resolver.GetRequestTierPriceByContext(resolved, totalContext)
	}

	// 回退到默认按次价格
	if unitPrice == 0 {
		unitPrice = resolved.DefaultPerRequestPrice
	}

	totalCost := unitPrice * units
	actualCost := totalCost * input.RateMultiplier

	return &CostBreakdown{
		TotalCost:  totalCost,
		ActualCost: actualCost,
	}, nil
}

// CalculateCost 计算使用费用
func (s *BillingService) CalculateCost(model string, tokens UsageTokens, rateMultiplier float64) (*CostBreakdown, error) {
	return s.calculateCostInternal(model, tokens, rateMultiplier, nil)
}

// calculateCostInternal 不经目录、直接按价格文件 / 兜底价计费（无解析器的旧路径）。按 token 分段只存在于
// 模型目录，这条路径没有分段，一律按基础价。
func (s *BillingService) calculateCostInternal(model string, tokens UsageTokens, rateMultiplier float64, channelPricing *PricingCard) (*CostBreakdown, error) {
	var pricing *ModelPricing
	var err error
	if channelPricing != nil {
		pricing, err = s.GetModelPricingWithChannel(model, channelPricing)
	} else {
		pricing, err = s.GetModelPricing(model)
	}
	if err != nil {
		return nil, err
	}

	return s.computeTokenBreakdown(pricing, tokens, rateMultiplier), nil
}

// applyModelSpecificPricingPolicy 对目录数据做模型特定修正：DeepSeek 官方价
// 强制覆盖；GPT-5.6 缺 cache_write 价时按官方规则补 1.25 倍输入价。按 token
// 分段不在此处：只由模型目录的分段驱动（价格文件的 above_XXXk 阶梯在播种时换算成
// 分段）。强制 DeepSeek 官方价且无显式计费时点（pro→Flash 切换按当前时刻判定），
// 供无既有时点的策略修正场景与测试使用；计费/展示主路径分别经
// calculateTokenCost 与 getModelPricingAt 显式传时点，分组/渠道自定义定价
// 用 applyModelSpecificPricingPolicyEx 关闭强制，保留运营者配置。
func (s *BillingService) applyModelSpecificPricingPolicy(model string, pricing *ModelPricing) *ModelPricing {
	return s.applyModelSpecificPricingPolicyEx(model, pricing, true, time.Time{})
}

// applyModelSpecificPricingPolicyEx 与 applyModelSpecificPricingPolicy 相同，
// 但由调用方控制是否强制 DeepSeek 官方价（forceDeepSeekRates），并显式传入
// 计费时点 pricingAt（零值表示按当前时刻判定）。
// calculateTokenCost 对分组/渠道自定义定价（Source 非 LiteLLM）传 false：
// 强制覆盖会把运营者配置的售价盖回官方价，违反自定义定价语义。
func (s *BillingService) applyModelSpecificPricingPolicyEx(model string, pricing *ModelPricing, forceDeepSeekRates bool, pricingAt time.Time) *ModelPricing {
	if pricing == nil {
		return nil
	}
	// DeepSeek 模型：无论 JSON/远端价格表给什么价，一律强制官方低谷价
	// （Flash 三档为 2026-09-10 官方降价后口径）。这是覆盖远端旧价的关键——远端
	// 仓库不可改，生产会先拉到旧价，必须在此兜底修正；克隆后再覆盖，避免污染
	// 共享 fallbackPrices 指针。
	// 档位判定：含 "deepseek-v4-pro" 的版本化名称（如 deepseek-v4-pro-0813）归 pro 档，
	// 其余 deepseek-*（含已停服的 chat/reasoner 与未知型号）统一归 flash 档。
	// 2026-09-14 04:00 UTC 起上游把 pro 请求路由到 V4.1-Flash，pro 档改按
	// Flash 三档价计费；历史时点（早于切换时刻）仍按 Pro 价。
	// 高峰时段倍率不在本函数处理，由 calculateTokenCost 按 deepseekPeakMultiplierAt
	// 对默认价卡另行叠加（分组/渠道自定义定价不叠加）。
	if forceDeepSeekRates && isDeepSeekModel(model) {
		cloned := *pricing
		if isDeepSeekProModel(model) && !deepseekProBilledAsFlash(pricingAt) {
			cloned.InputPricePerToken = deepseekProOffPeakInputPrice
			cloned.OutputPricePerToken = deepseekProOffPeakOutputPrice
			cloned.CacheReadPricePerToken = deepseekProOffPeakCacheRead
		} else {
			// deepseek-flash（= V4.1-Flash）、deepseek-v4-flash /
			// deepseek-v4-flash-vision-exp 与其余 deepseek-* 共用 flash 价；
			// 切换时点之后的 pro 请求同样按 flash 价计费。
			cloned.InputPricePerToken = deepseekFlashOffPeakInputPrice
			cloned.OutputPricePerToken = deepseekFlashOffPeakOutputPrice
			cloned.CacheReadPricePerToken = deepseekFlashOffPeakCacheRead
		}
		return &cloned
	}
	normalized := normalizeKnownOpenAICodexModel(model)
	isGPT56 := isOpenAIGPT56Model(normalized)
	needsMaxReasoningEffortMultiplier := isClaudeFable51Model(model) && pricing.MaxReasoningEffortMultiplier == nil
	needsCacheCreationPolicy := isGPT56 && !pricing.CacheCreationPriceExplicit && pricing.CacheCreationPricePerToken <= 0
	if !needsCacheCreationPolicy && !needsMaxReasoningEffortMultiplier {
		return pricing
	}
	cloned := *pricing
	if needsMaxReasoningEffortMultiplier {
		cloned.MaxReasoningEffortMultiplier = defaultMaxReasoningEffortMultiplier(model)
	}
	if needsCacheCreationPolicy {
		cloned.CacheCreationPricePerToken = cloned.InputPricePerToken * 1.25
	}
	return &cloned
}

// ListSupportedModels 列出所有支持的模型（现在总是返回true，因为有模糊匹配）
func (s *BillingService) ListSupportedModels() []string {
	models := make([]string, 0)
	// 返回回退价格支持的模型系列
	for model := range s.fallbackPrices {
		models = append(models, model)
	}
	return models
}

// IsModelSupported 检查模型是否支持（现在总是返回true，因为有模糊匹配回退）
func (s *BillingService) IsModelSupported(model string) bool {
	// 所有Claude模型都有回退价格支持
	modelLower := strings.ToLower(model)
	return strings.Contains(modelLower, "claude") ||
		strings.Contains(modelLower, "opus") ||
		strings.Contains(modelLower, "sonnet") ||
		strings.Contains(modelLower, "haiku")
}

const (
	// Grok Voice 内置单价（realtime 每分钟 / TTS 每百万字符 / STT 每小时）。
	defaultAudioRealtimePricePerMin     = 0.05
	defaultAudioTTSPricePerMillionChars = 15.0
	defaultAudioSTTPricePerHour         = 0.10
)

// CalculateAudioCost supports realtime (per min), tts (per M chars), stt (per hr) at the
// built-in unit prices.
func (s *BillingService) CalculateAudioCost(mode string, durationOrUnits float64, rateMultiplier float64) *CostBreakdown {
	if durationOrUnits <= 0 {
		return &CostBreakdown{}
	}
	var unitPrice float64
	switch strings.ToLower(mode) {
	case "realtime":
		unitPrice = defaultAudioRealtimePricePerMin
	case "tts":
		unitPrice = defaultAudioTTSPricePerMillionChars
	case "stt":
		unitPrice = defaultAudioSTTPricePerHour
	default:
		return &CostBreakdown{}
	}
	if rateMultiplier < 0 {
		rateMultiplier = 0
	}
	total := unitPrice * durationOrUnits
	return &CostBreakdown{
		TotalCost:   total,
		ActualCost:  total * rateMultiplier,
		BillingMode: string(BillingModePerRequest),
	}
}
