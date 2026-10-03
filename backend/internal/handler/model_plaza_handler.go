package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ModelPlazaHandler 处理「模型广场」查询：列出目录上架的模型与基准价。
//
// 广场路由挂 OptionalJWT 中间件：匿名可访问（除非 require_auth 开启）。
type ModelPlazaHandler struct {
	plazaService   *service.ModelPlazaService
	settingService *service.SettingService
}

// NewModelPlazaHandler 创建模型广场 handler。
func NewModelPlazaHandler(
	plazaService *service.ModelPlazaService,
	settingService *service.SettingService,
) *ModelPlazaHandler {
	return &ModelPlazaHandler{
		plazaService:   plazaService,
		settingService: settingService,
	}
}

// modelPlazaTimePricingPeriod 分时倍率时段（配置时区当天 [start, end)）。
type modelPlazaTimePricingPeriod struct {
	StartTime  string  `json:"start_time"`
	EndTime    string  `json:"end_time"`
	Multiplier float64 `json:"multiplier"`
}

// modelPlazaTimePricing 分时倍率配置。
// WeekdaysOnly 为 true 时时段仅周一至周五生效，周末整天按标准价计费。
type modelPlazaTimePricing struct {
	Timezone     string                        `json:"timezone"`
	WeekdaysOnly bool                          `json:"weekdays_only,omitempty"`
	Periods      []modelPlazaTimePricingPeriod `json:"periods"`
}

// modelPlazaModel 广场模型条目：目录基准价（白名单形态）。
type modelPlazaModel struct {
	ModelID     string                     `json:"model_id"`
	DisplayName string                     `json:"display_name"`
	Vendor      string                     `json:"vendor"`
	BillingMode string                     `json:"billing_mode"`
	Pricing     *userSupportedModelPricing `json:"pricing"`
	// TimePricing 分时倍率时段，落在时段内的请求整单乘倍率；无分时省略。
	TimePricing *modelPlazaTimePricing `json:"time_pricing,omitempty"`
	Aliases     []string               `json:"aliases"`
}

// modelPlazaResponse 广场页响应：平铺的上架模型列表。
// 价格一律是目录官方价（USD）；展示价 = 官方价 × 访问者的用户倍率，未登录按 DefaultRateMultiplier（新用户默认倍率）。
type modelPlazaResponse struct {
	Description           string            `json:"description"`
	DefaultRateMultiplier float64           `json:"default_rate_multiplier"`
	Models                []modelPlazaModel `json:"models"`
	// ClaudeCodeWebSearch 用 Claude Code 配非 Anthropic 模型时，那次搜索请求的计费项（官方价；token × 用户倍率，
	// 每次搜索按原价）；目录里没有代执行模型时省略。
	ClaudeCodeWebSearch *plazaWebSearchBillingDTO `json:"claude_code_web_search,omitempty"`
}

type plazaWebSearchBillingDTO struct {
	InputPrice         *float64 `json:"input_price"`
	OutputPrice        *float64 `json:"output_price"`
	CacheReadPrice     *float64 `json:"cache_read_price"`
	CacheWritePrice    *float64 `json:"cache_write_price"`
	SearchPricePerCall float64  `json:"search_price_per_call"`
}

// Get 返回模型广场数据。
// GET /api/v1/model-plaza
func (h *ModelPlazaHandler) Get(c *gin.Context) {
	// 模型广场对所有人开放（含未登录）：目录只列上架模型与标价，没有开关。
	description := h.settingService.GetModelPlazaDescription(c.Request.Context())
	models := h.plazaService.ListModels(c.Request.Context())
	out := make([]modelPlazaModel, 0, len(models))
	for i := range models {
		out = append(out, toModelPlazaModelDTO(&models[i]))
	}
	resp := modelPlazaResponse{
		Description:           description,
		DefaultRateMultiplier: service.NewUserRateMultiplier,
		Models:                out,
	}
	if billing := h.plazaService.ClaudeCodeWebSearchBilling(c.Request.Context()); billing != nil {
		resp.ClaudeCodeWebSearch = &plazaWebSearchBillingDTO{
			InputPrice:         billing.InputPrice,
			OutputPrice:        billing.OutputPrice,
			CacheReadPrice:     billing.CacheReadPrice,
			CacheWritePrice:    billing.CacheWritePrice,
			SearchPricePerCall: billing.SearchPricePerCall,
		}
	}
	response.Success(c, resp)
}

// toModelPlazaModelDTO 将 service 层广场模型映射为白名单 DTO。
func toModelPlazaModelDTO(m *service.PlazaCatalogModel) modelPlazaModel {
	aliases := m.Aliases
	if aliases == nil {
		aliases = []string{}
	}
	return modelPlazaModel{
		ModelID:     m.ModelID,
		DisplayName: m.DisplayName,
		Vendor:      m.Vendor,
		BillingMode: string(m.BillingMode),
		Pricing:     toUserPricing(m.Pricing, m.TokenExtras),
		TimePricing: toModelPlazaTimePricing(m.TimePricing),
		Aliases:     aliases,
	}
}

// toModelPlazaTimePricing 转换分时倍率配置；nil 或无时段透传 nil（JSON 省略）。
func toModelPlazaTimePricing(p *service.TimePricing) *modelPlazaTimePricing {
	if p == nil || len(p.Periods) == 0 {
		return nil
	}
	periods := make([]modelPlazaTimePricingPeriod, 0, len(p.Periods))
	for _, period := range p.Periods {
		periods = append(periods, modelPlazaTimePricingPeriod{
			StartTime:  period.StartTime,
			EndTime:    period.EndTime,
			Multiplier: period.Multiplier,
		})
	}
	return &modelPlazaTimePricing{Timezone: p.Timezone, WeekdaysOnly: p.WeekdaysOnly, Periods: periods}
}

// userSupportedModelPricing 用户可见的定价字段白名单。
type userSupportedModelPricing struct {
	BillingMode                  string   `json:"billing_mode"`
	InputPrice                   *float64 `json:"input_price"`
	OutputPrice                  *float64 `json:"output_price"`
	CacheWritePrice              *float64 `json:"cache_write_price"`
	CacheWrite1hPrice            *float64 `json:"cache_write_1h_price"`
	CacheReadPrice               *float64 `json:"cache_read_price"`
	MaxReasoningEffortMultiplier *float64 `json:"max_reasoning_effort_multiplier,omitempty"`
	ImageInputPrice              *float64 `json:"image_input_price"`
	ImageOutputPrice             *float64 `json:"image_output_price"`
	PerRequestPrice              *float64 `json:"per_request_price"`
	// 联网搜索的官方单价（不乘用户倍率）：每次 web 搜索；xAI 模型另有每条 X 帖子、每个 X 主页。
	SearchPricePerCall  *float64                 `json:"search_price_per_call,omitempty"`
	XPostPrice          *float64                 `json:"x_post_price,omitempty"`
	XUserPrice          *float64                 `json:"x_user_price,omitempty"`
	ImageCacheReadPrice *float64                 `json:"image_cache_read_price"`
	AudioInputPrice     *float64                 `json:"audio_input_price"`
	AudioOutputPrice    *float64                 `json:"audio_output_price"`
	Intervals           []userPricingIntervalDTO `json:"intervals"`
}

// userPricingIntervalDTO 定价区间白名单（去掉内部 ID、SortOrder 等前端不渲染的字段）。
type userPricingIntervalDTO struct {
	MinTokens            int      `json:"min_tokens"`
	MaxTokens            *int     `json:"max_tokens"`
	TierLabel            string   `json:"tier_label,omitempty"`
	InputPrice           *float64 `json:"input_price"`
	OutputPrice          *float64 `json:"output_price"`
	CacheWritePrice      *float64 `json:"cache_write_price"`
	CacheWrite1hPrice    *float64 `json:"cache_write_1h_price"`
	CacheReadPrice       *float64 `json:"cache_read_price"`
	InputMultiplier      *float64 `json:"input_multiplier"`
	OutputMultiplier     *float64 `json:"output_multiplier"`
	CacheWriteMultiplier *float64 `json:"cache_write_multiplier"`
	CacheReadMultiplier  *float64 `json:"cache_read_multiplier"`
	PerRequestPrice      *float64 `json:"per_request_price"`
}

// toUserPricingIntervals 将定价区间转换为用户 DTO 白名单形态；nil 入参返回 nil（JSON omitempty 可省略）。
func toUserPricingIntervals(src []service.PricingInterval) []userPricingIntervalDTO {
	if src == nil {
		return nil
	}
	intervals := make([]userPricingIntervalDTO, 0, len(src))
	for _, iv := range src {
		intervals = append(intervals, userPricingIntervalDTO{
			MinTokens:            iv.MinTokens,
			MaxTokens:            iv.MaxTokens,
			TierLabel:            iv.TierLabel,
			InputPrice:           iv.InputPrice,
			OutputPrice:          iv.OutputPrice,
			CacheWritePrice:      iv.CacheWritePrice,
			CacheWrite1hPrice:    iv.CacheWrite1hPrice,
			CacheReadPrice:       iv.CacheReadPrice,
			InputMultiplier:      iv.InputMultiplier,
			OutputMultiplier:     iv.OutputMultiplier,
			CacheWriteMultiplier: iv.CacheWriteMultiplier,
			CacheReadMultiplier:  iv.CacheReadMultiplier,
			PerRequestPrice:      iv.PerRequestPrice,
		})
	}
	return intervals
}

// toUserPricing 将 service 层定价转换为用户 DTO；价卡为 nil 时返回 nil。价格一律是目录官方价。
func toUserPricing(p *service.PricingCard, extras service.PlazaTokenExtras) *userSupportedModelPricing {
	if p == nil {
		return nil
	}
	intervals := toUserPricingIntervals(p.Intervals)
	if intervals == nil {
		// 用户侧定价的 intervals 固定输出数组（空配置为 []），保持既有契约。
		intervals = []userPricingIntervalDTO{}
	}
	billingMode := string(p.BillingMode)
	if billingMode == "" {
		billingMode = string(service.BillingModeToken)
	}
	return &userSupportedModelPricing{
		BillingMode:                  billingMode,
		InputPrice:                   p.InputPrice,
		OutputPrice:                  p.OutputPrice,
		CacheWritePrice:              p.CacheWritePrice,
		CacheWrite1hPrice:            p.CacheWrite1hPrice,
		CacheReadPrice:               p.CacheReadPrice,
		MaxReasoningEffortMultiplier: p.MaxReasoningEffortMultiplier,
		ImageInputPrice:              p.ImageInputPrice,
		ImageOutputPrice:             p.ImageOutputPrice,
		PerRequestPrice:              p.PerRequestPrice,
		SearchPricePerCall:           extras.WebSearchPricePerCall,
		XPostPrice:                   extras.XPostPrice,
		XUserPrice:                   extras.XUserPrice,
		ImageCacheReadPrice:          extras.ImageCacheReadPrice,
		AudioInputPrice:              extras.AudioInputPrice,
		AudioOutputPrice:             extras.AudioOutputPrice,
		Intervals:                    intervals,
	}
}
