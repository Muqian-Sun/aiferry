package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
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

// maxReasoningEffortBillingMultiplier max 推理等级的计费倍率：只认目录条目上的最高推理倍率，没设 = 1。
func maxReasoningEffortBillingMultiplier(effort string, pricing *ModelPricing) float64 {
	if NormalizeMaxReasoningEffort(effort) != "max" {
		return 1
	}
	if pricing != nil && pricing.MaxReasoningEffortMultiplier != nil && *pricing.MaxReasoningEffortMultiplier > 0 {
		return *pricing.MaxReasoningEffortMultiplier
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

// isDeepSeekModel 判断模型名是否为 DeepSeek 模型（大小写不敏感）。
// 任意 deepseek- 前缀均视为 DeepSeek 模型：官方模型（v4-flash / v4-pro /
// v4-flash-vision-exp）按各自价卡计价，其余 deepseek-*（含已停服的
// deepseek-chat / deepseek-reasoner 与未知型号）统一按 flash 价兜底，
// 避免计费中断；新名字由 fallback warn 日志（每模型每进程一条）暴露，
// 运营者据此更新价卡。
func isDeepSeekModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "deepseek-")
}

// BillingService 计费服务
type BillingService struct {
	fallbackPrices map[string]*ModelPricing // 内置价表：只用来播种模型目录（价格文件没有的模型），不参与计费
}

// NewBillingService 创建计费服务实例
// 计费只认模型目录（muqian 2026-10-06），不读价格文件与配置。
func NewBillingService() *BillingService {
	s := &BillingService{
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
		// max 推理等级按 3 倍计（播种时写进目录条目，计费只认目录）
		MaxReasoningEffortMultiplier: func() *float64 { v := 3.0; return &v }(),
	}

	// Gemini 3.1 Pro
	s.fallbackPrices["gemini-3.1-pro"] = &ModelPricing{
		InputPricePerToken:         2e-6,   // $2 per MTok
		OutputPricePerToken:        12e-6,  // $12 per MTok
		CacheCreationPricePerToken: 2e-6,   // $2 per MTok
		CacheReadPricePerToken:     0.2e-6, // $0.20 per MTok
		SupportsCacheBreakdown:     false,
	}

	// Gemini 3.6 Flash (Google AI pricing: $0.75 input / $3.75 output /
	// $0.075 cached input per MTok, promotional through 2026-12-31; official
	// rates double to $1.50/$7.50/$0.15 from 2027-01-01 — muqian 2026-10-06:
	// store the current official price). Antigravity's -high/-low/-medium/-tiered
	// aliases are matched below so unavailable remote pricing never records
	// token-bearing requests at $0.
	s.fallbackPrices["gemini-3.6-flash"] = &ModelPricing{
		InputPricePerToken:     0.75e-6,
		OutputPricePerToken:    3.75e-6,
		CacheReadPricePerToken: 0.075e-6,
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
	// gpt-5.6-sol 是官网优惠价（2026-08-21 起，「至少到 2026-11-21」，之后价格官网未公布；原价 $5 / $30）；
	// muqian 2026-10-06 定存官网现价，到期要回来改。
	s.fallbackPrices["gpt-5.6-sol"] = &ModelPricing{
		InputPricePerToken:         4e-6,
		OutputPricePerToken:        20e-6,
		CacheCreationPricePerToken: 5e-6,
		CacheReadPricePerToken:     0.4e-6,
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
	// 2026-10-06 按官网核对：现价是「永久五折」后的 $0.30 / $1.20 / 缓存 $0.06（划线原价 $0.60 / $2.40）。
	s.fallbackPrices["minimax-m3"] = &ModelPricing{
		InputPricePerToken:     0.30e-6, // $0.30 per MTok (≤512K, Permanent 50% off)
		OutputPricePerToken:    1.20e-6,
		CacheReadPricePerToken: 0.06e-6,
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
	// 按量付费官方价 文本 ¥0.7/MTok、图片 ¥1.8/MTok；只有人民币价的按 1 美元 = 6.8 元换算（muqian 2026-10-06）。
	// embedding 无 output，OutputPricePerToken 置 0。
	// 官方 ID 带版本号（火山方舟模型列表 2026-09-28：doubao-embedding-vision-251215；裸名只是定价页的产品名）
	s.fallbackPrices["doubao-embedding-vision-251215"] = &ModelPricing{
		InputPricePerToken:      0.7 / 6.8 * 1e-6, // ¥0.7/MTok（文本输入）
		ImageInputPricePerToken: 1.8 / 6.8 * 1e-6, // ¥1.8/MTok（图片输入）
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

// applyConfiguredImageInputPrice 应用渠道价卡的图片输入价：显式配置则用配置值；
// 未配置时归零，使 computeTokenBreakdown 回退到文本输入价。
func applyConfiguredImageInputPrice(chPricing *PricingCard, pricing *ModelPricing) {
	if chPricing != nil && chPricing.ImageInputPrice != nil {
		pricing.ImageInputPricePerToken = *chPricing.ImageInputPrice
	} else {
		pricing.ImageInputPricePerToken = 0
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
	// 计费只认模型目录（muqian 2026-10-06）：没有解析器就没有价。
	if input.Resolver == nil && input.Resolved == nil {
		return nil, fmt.Errorf("no pricing resolver for model %s: %w", input.Model, ErrModelPricingUnavailable)
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

	breakdown := s.computeTokenBreakdown(pricing, input.Tokens, input.RateMultiplier)
	// 单独定了售价的项：实付按售价换算成的官方口径算（× 计费倍率 = 售价 × 折扣）；TotalCost 仍是官方价合计。
	segment := FindMatchingInterval(resolved.Intervals, totalContext)
	if sale := saleEquivalentPricing(resolved.sale, segment, resolved.BasePricing, pricing); sale != nil {
		breakdown.ActualCost = s.computeTokenBreakdown(sale, input.Tokens, input.RateMultiplier).ActualCost
	}
	// 忙闲时（如 DeepSeek 工作日高峰 × 2）是目录条目的分时：官方价合计与实付整单一起乘。
	applyCostBreakdownMultiplier(breakdown, resolvedTimePricingMultiplier(resolved, input.PricingAt))
	applyCostBreakdownMultiplier(breakdown, maxReasoningEffortBillingMultiplier(input.ReasoningEffort, pricing))
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
