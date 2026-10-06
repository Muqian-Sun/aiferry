package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ModelPlazaHandler 处理「模型广场」查询：列出目录上架的模型与售价。
//
// 广场路由挂 OptionalJWT 中间件：匿名可访问（除非 require_auth 开启）。
type ModelPlazaHandler struct {
	plazaService   *service.ModelPlazaService
	settingService *service.SettingService
	users          PlazaUserSource
}

// PlazaUserSource 取登录访问者的用户记录，算他的生效倍率；生产上是 *service.UserService。
type PlazaUserSource interface {
	GetByID(ctx context.Context, id int64) (*service.User, error)
}

// NewModelPlazaHandler 创建模型广场 handler。
func NewModelPlazaHandler(
	plazaService *service.ModelPlazaService,
	settingService *service.SettingService,
	users PlazaUserSource,
) *ModelPlazaHandler {
	return &ModelPlazaHandler{
		plazaService:   plazaService,
		settingService: settingService,
		users:          users,
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
}

// modelPlazaResponse 广场页响应：平铺的上架模型列表。
// 价格是访问者的售价（USD）= 目录官方价 × 访问者的用户倍率，未登录按新用户默认倍率；
// 官方价与倍率都不出接口（2026-10-04 D1）。联网搜索按次价按原价收，原样给。
type modelPlazaResponse struct {
	Description string            `json:"description"`
	Models      []modelPlazaModel `json:"models"`
	// ClaudeCodeWebSearch 用 Claude Code 配非 Anthropic 模型时，那次搜索请求的计费项（token 已是售价，
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
	multiplier, err := h.viewerMultiplier(c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	description := h.settingService.GetModelPlazaDescription(c.Request.Context())
	models := h.plazaService.ListModels(c.Request.Context())
	out := make([]modelPlazaModel, 0, len(models))
	for i := range models {
		out = append(out, toModelPlazaModelDTO(&models[i], multiplier))
	}
	resp := modelPlazaResponse{
		Description: description,
		Models:      out,
	}
	if billing := h.plazaService.ClaudeCodeWebSearchBilling(c.Request.Context()); billing != nil {
		resp.ClaudeCodeWebSearch = &plazaWebSearchBillingDTO{
			InputPrice:         scalePlazaPrice(billing.InputPrice, multiplier),
			OutputPrice:        scalePlazaPrice(billing.OutputPrice, multiplier),
			CacheReadPrice:     scalePlazaPrice(billing.CacheReadPrice, multiplier),
			CacheWritePrice:    scalePlazaPrice(billing.CacheWritePrice, multiplier),
			SearchPricePerCall: billing.SearchPricePerCall,
		}
	}
	response.Success(c, resp)
}

// viewerMultiplier 访问者的生效倍率：登录用户按他的倍率（单独设的或全站默认），未登录按新用户默认倍率。
func (h *ModelPlazaHandler) viewerMultiplier(c *gin.Context) (float64, error) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		return service.NewUserRateMultiplier, nil
	}
	user, err := h.users.GetByID(c.Request.Context(), subject.UserID)
	if err != nil {
		return 0, err
	}
	return service.UserRateMultiplier(user), nil
}

// scalePlazaPrice 官方价 × 倍率 = 售价；没定价（nil）保持 nil。
func scalePlazaPrice(p *float64, multiplier float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p * multiplier
	return &v
}

// toModelPlazaModelDTO 将 service 层广场模型映射为白名单 DTO，价格乘访问者倍率成售价。
func toModelPlazaModelDTO(m *service.PlazaCatalogModel, multiplier float64) modelPlazaModel {
	return modelPlazaModel{
		ModelID:     m.ModelID,
		DisplayName: m.DisplayName,
		Vendor:      m.Vendor,
		BillingMode: string(m.BillingMode),
		Pricing:     toUserPricing(m.Pricing, m.TokenExtras, multiplier),
		TimePricing: toModelPlazaTimePricing(m.TimePricing),
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
	// 联网搜索按次价（按原价收、不乘用户倍率）：每次 web 搜索；xAI 模型另有每条 X 帖子、每个 X 主页。
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

// toUserPricingIntervals 将定价区间转换为用户 DTO 白名单形态，各段单价乘访问者倍率；nil 入参返回 nil（JSON omitempty 可省略）。
// 区间上的 *_multiplier 是相对基础价的比例，不乘。
func toUserPricingIntervals(src []service.PricingInterval, multiplier float64) []userPricingIntervalDTO {
	if src == nil {
		return nil
	}
	intervals := make([]userPricingIntervalDTO, 0, len(src))
	for _, iv := range src {
		intervals = append(intervals, userPricingIntervalDTO{
			MinTokens:            iv.MinTokens,
			MaxTokens:            iv.MaxTokens,
			TierLabel:            iv.TierLabel,
			InputPrice:           scalePlazaPrice(iv.InputPrice, multiplier),
			OutputPrice:          scalePlazaPrice(iv.OutputPrice, multiplier),
			CacheWritePrice:      scalePlazaPrice(iv.CacheWritePrice, multiplier),
			CacheWrite1hPrice:    scalePlazaPrice(iv.CacheWrite1hPrice, multiplier),
			CacheReadPrice:       scalePlazaPrice(iv.CacheReadPrice, multiplier),
			InputMultiplier:      iv.InputMultiplier,
			OutputMultiplier:     iv.OutputMultiplier,
			CacheWriteMultiplier: iv.CacheWriteMultiplier,
			CacheReadMultiplier:  iv.CacheReadMultiplier,
			PerRequestPrice:      scalePlazaPrice(iv.PerRequestPrice, multiplier),
		})
	}
	return intervals
}

// toUserPricing 将 service 层定价转换为用户 DTO；价卡为 nil 时返回 nil。
// 按 token / 次 / 张 / 秒的单价乘访问者倍率成售价；联网搜索按次价（web 搜索、X 帖子 / 主页）按原价收、不乘，
// 最高推理档倍率是比例、不乘。与计费口径一致（billing_service）。
func toUserPricing(p *service.PricingCard, extras service.PlazaTokenExtras, multiplier float64) *userSupportedModelPricing {
	if p == nil {
		return nil
	}
	intervals := toUserPricingIntervals(p.Intervals, multiplier)
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
		InputPrice:                   scalePlazaPrice(p.InputPrice, multiplier),
		OutputPrice:                  scalePlazaPrice(p.OutputPrice, multiplier),
		CacheWritePrice:              scalePlazaPrice(p.CacheWritePrice, multiplier),
		CacheWrite1hPrice:            scalePlazaPrice(p.CacheWrite1hPrice, multiplier),
		CacheReadPrice:               scalePlazaPrice(p.CacheReadPrice, multiplier),
		MaxReasoningEffortMultiplier: p.MaxReasoningEffortMultiplier,
		ImageInputPrice:              scalePlazaPrice(p.ImageInputPrice, multiplier),
		ImageOutputPrice:             scalePlazaPrice(p.ImageOutputPrice, multiplier),
		PerRequestPrice:              scalePlazaPrice(p.PerRequestPrice, multiplier),
		SearchPricePerCall:           extras.WebSearchPricePerCall,
		XPostPrice:                   extras.XPostPrice,
		XUserPrice:                   extras.XUserPrice,
		ImageCacheReadPrice:          scalePlazaPrice(extras.ImageCacheReadPrice, multiplier),
		AudioInputPrice:              scalePlazaPrice(extras.AudioInputPrice, multiplier),
		AudioOutputPrice:             scalePlazaPrice(extras.AudioOutputPrice, multiplier),
		Intervals:                    intervals,
	}
}
