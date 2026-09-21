package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
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
type modelPlazaResponse struct {
	Description string            `json:"description"`
	Models      []modelPlazaModel `json:"models"`
}

// Get 返回模型广场数据。
// GET /api/v1/model-plaza
func (h *ModelPlazaHandler) Get(c *gin.Context) {
	if h.settingService == nil {
		response.NotFound(c, "Model plaza is not enabled")
		return
	}
	rt := h.settingService.GetModelPlazaRuntime(c.Request.Context())
	if !rt.Enabled {
		response.NotFound(c, "Model plaza is not enabled")
		return
	}

	_, authed := middleware.GetAuthSubjectFromContext(c)
	if rt.RequireAuth && !authed {
		response.Unauthorized(c, "Authentication required")
		return
	}

	models := h.plazaService.ListModels(c.Request.Context())
	out := make([]modelPlazaModel, 0, len(models))
	for i := range models {
		out = append(out, toModelPlazaModelDTO(&models[i]))
	}
	response.Success(c, modelPlazaResponse{
		Description: rt.Description,
		Models:      out,
	})
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
		Pricing:     toUserPricing(m.Pricing),
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
