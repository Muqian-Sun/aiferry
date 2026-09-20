package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ModelCatalogHandler 处理模型目录的管理端请求。
type ModelCatalogHandler struct {
	service *service.ModelCatalogService
	// accounts 绑定资源时按 ID 取账号做承接校验与展示；生产上是 AdminService。
	accounts service.CatalogBindingAccountSource
}

// NewModelCatalogHandler 创建模型目录处理器。
func NewModelCatalogHandler(svc *service.ModelCatalogService, accounts service.CatalogBindingAccountSource) *ModelCatalogHandler {
	return &ModelCatalogHandler{service: svc, accounts: accounts}
}

// ModelCatalogEntryRequest 是条目的创建 / 更新请求体。
//
// 更新是整条覆盖（不是部分更新）：价格字段全是可空指针，只补传部分字段会让没传的
// 项被清空——这是刻意的，价格表用部分更新很容易留下上一版的残值。
type ModelCatalogEntryRequest struct {
	ModelID     string   `json:"model_id" binding:"required"`
	DisplayName string   `json:"display_name"`
	Vendor      string   `json:"vendor"`
	Protocols   []string `json:"protocols"`
	BillingMode string   `json:"billing_mode"`
	Status      string   `json:"status"`
	// RoutePlatform 条目走哪条网关族；空表示按 vendor 推导。
	RoutePlatform string `json:"route_platform"`

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

	Notes *string `json:"notes"`

	Intervals   []service.PricingInterval   `json:"intervals"`
	TimePricing *service.ChannelTimePricing `json:"time_pricing"`
}

func (r *ModelCatalogEntryRequest) toEntry() *service.ModelCatalogEntry {
	return &service.ModelCatalogEntry{
		ModelID:       r.ModelID,
		DisplayName:   r.DisplayName,
		Vendor:        r.Vendor,
		Protocols:     r.Protocols,
		BillingMode:   service.BillingMode(r.BillingMode),
		Status:        r.Status,
		RoutePlatform: r.RoutePlatform,

		InputPrice:          r.InputPrice,
		OutputPrice:         r.OutputPrice,
		CacheWritePrice:     r.CacheWritePrice,
		CacheWrite1hPrice:   r.CacheWrite1hPrice,
		CacheReadPrice:      r.CacheReadPrice,
		ImageInputPrice:     r.ImageInputPrice,
		ImageOutputPrice:    r.ImageOutputPrice,
		ImageCacheReadPrice: r.ImageCacheReadPrice,

		InputPricePriority:      r.InputPricePriority,
		OutputPricePriority:     r.OutputPricePriority,
		CacheWritePricePriority: r.CacheWritePricePriority,
		CacheReadPricePriority:  r.CacheReadPricePriority,

		PerRequestPrice: r.PerRequestPrice,

		LongContextInputThreshold:     r.LongContextInputThreshold,
		LongContextThresholdInclusive: r.LongContextThresholdInclusive,
		LongContextInputMultiplier:    r.LongContextInputMultiplier,
		LongContextOutputMultiplier:   r.LongContextOutputMultiplier,

		FastMultiplier:               r.FastMultiplier,
		FlexMultiplier:               r.FlexMultiplier,
		MaxReasoningEffortMultiplier: r.MaxReasoningEffortMultiplier,

		Notes:       r.Notes,
		Intervals:   r.Intervals,
		TimePricing: r.TimePricing,
	}
}

// ModelCatalogBindingRequest 是条目绑定资源的整份覆盖请求体。
type ModelCatalogBindingRequest struct {
	Bindings []ModelCatalogBindingItem `json:"bindings" binding:"dive"`
}

// ModelCatalogBindingItem 一条绑定：账号 ID 与可选的绑定优先级（空 = 跟随账号）。
type ModelCatalogBindingItem struct {
	AccountID int64 `json:"account_id" binding:"required"`
	Priority  *int  `json:"priority"`
}

// ModelCatalogBindingResponse 绑定及其账号摘要。
type ModelCatalogBindingResponse struct {
	EntryID   int64                              `json:"entry_id"`
	AccountID int64                              `json:"account_id"`
	Priority  *int                               `json:"priority"`
	Account   *ModelCatalogBindingAccountSummary `json:"account,omitempty"`
}

// ModelCatalogBindingAccountSummary 绑定列表里展示账号用的摘要。
type ModelCatalogBindingAccountSummary struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Type     string `json:"type"`
	Vendor   string `json:"vendor"`
	Status   string `json:"status"`
}

// ModelCatalogDiagnosisItem 诊断一条绑定：账号此刻能不能被调度、在条目网关族上能承接哪些入站协议。
type ModelCatalogDiagnosisItem struct {
	ModelCatalogBindingAccountSummary
	Priority *int `json:"priority"`
	// Schedulable 账号此刻能否进入调度；BlockedReason 不能时的第一个原因
	// （disabled / unschedulable / expired / overloaded / rate_limited / temp_unschedulable / quota_exceeded）。
	Schedulable   bool   `json:"schedulable"`
	BlockedReason string `json:"blocked_reason,omitempty"`
	// Serves 入站协议 → 能否承接（anthropic / chat_completions / responses / gemini）。
	Serves map[string]bool `json:"serves"`
}

// ModelCatalogDiagnosisResponse 条目的资源诊断。
type ModelCatalogDiagnosisResponse struct {
	EntryID       int64                       `json:"entry_id"`
	RoutePlatform string                      `json:"route_platform"`
	Accounts      []ModelCatalogDiagnosisItem `json:"accounts"`
}

// ModelCatalogAliasRequest 是别名的创建 / 更新请求体。
type ModelCatalogAliasRequest struct {
	Alias   string  `json:"alias" binding:"required"`
	EntryID int64   `json:"entry_id" binding:"required"`
	Notes   *string `json:"notes"`
}

// ListEntries 返回全部目录条目（含别名、分档、分时）。
// GET /api/v1/admin/model-catalog/entries
func (h *ModelCatalogHandler) ListEntries(c *gin.Context) {
	entries, err := h.service.ListEntries(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, entries)
}

// GetEntry 按 ID 取条目。
// GET /api/v1/admin/model-catalog/entries/:id
func (h *ModelCatalogHandler) GetEntry(c *gin.Context) {
	id, ok := parseModelCatalogID(c, "Invalid model catalog entry ID")
	if !ok {
		return
	}
	entry, err := h.service.GetEntry(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, entry)
}

// CreateEntry 新建条目。
// POST /api/v1/admin/model-catalog/entries
func (h *ModelCatalogHandler) CreateEntry(c *gin.Context) {
	var req ModelCatalogEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	entry := req.toEntry()
	if err := h.service.CreateEntry(c.Request.Context(), entry); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, entry)
}

// UpdateEntry 整条覆盖一个条目。
// PUT /api/v1/admin/model-catalog/entries/:id
func (h *ModelCatalogHandler) UpdateEntry(c *gin.Context) {
	id, ok := parseModelCatalogID(c, "Invalid model catalog entry ID")
	if !ok {
		return
	}
	var req ModelCatalogEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	entry := req.toEntry()
	entry.ID = id
	if err := h.service.UpdateEntry(c.Request.Context(), entry); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, entry)
}

// DeleteEntry 删除条目（别名 / 分档 / 分时随外键级联删除）。
// DELETE /api/v1/admin/model-catalog/entries/:id
func (h *ModelCatalogHandler) DeleteEntry(c *gin.Context) {
	id, ok := parseModelCatalogID(c, "Invalid model catalog entry ID")
	if !ok {
		return
	}
	if err := h.service.DeleteEntry(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Model catalog entry deleted successfully"})
}

// ListBindings 返回条目绑定的资源及账号摘要。
// GET /api/v1/admin/model-catalog/entries/:id/bindings
func (h *ModelCatalogHandler) ListBindings(c *gin.Context) {
	id, ok := parseModelCatalogID(c, "Invalid model catalog entry ID")
	if !ok {
		return
	}
	bindings, err := h.service.ListBindings(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]ModelCatalogBindingResponse, 0, len(bindings))
	for _, binding := range bindings {
		item := ModelCatalogBindingResponse{EntryID: binding.EntryID, AccountID: binding.AccountID, Priority: binding.Priority}
		account, err := h.accounts.GetAccount(c.Request.Context(), binding.AccountID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		item.Account = &ModelCatalogBindingAccountSummary{
			ID: account.ID, Name: account.Name, Platform: account.Platform, Type: account.Type,
			Vendor: account.Vendor(), Status: account.Status,
		}
		out = append(out, item)
	}
	response.Success(c, out)
}

// ReplaceBindings 用整份列表覆盖条目绑定的资源。
// PUT /api/v1/admin/model-catalog/entries/:id/bindings
func (h *ModelCatalogHandler) ReplaceBindings(c *gin.Context) {
	id, ok := parseModelCatalogID(c, "Invalid model catalog entry ID")
	if !ok {
		return
	}
	var req ModelCatalogBindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	bindings := make([]service.ModelCatalogBinding, 0, len(req.Bindings))
	for _, item := range req.Bindings {
		bindings = append(bindings, service.ModelCatalogBinding{EntryID: id, AccountID: item.AccountID, Priority: item.Priority})
	}
	if err := h.service.ReplaceBindings(c.Request.Context(), id, bindings, h.accounts); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.ListBindings(c)
}

// Diagnose 逐个说明条目绑定的资源此刻能不能承接请求：可调度与否、原因、能承接哪些入站协议。
// GET /api/v1/admin/model-catalog/entries/:id/diagnosis
func (h *ModelCatalogHandler) Diagnose(c *gin.Context) {
	id, ok := parseModelCatalogID(c, "Invalid model catalog entry ID")
	if !ok {
		return
	}
	entry, err := h.service.GetEntry(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := make([]ModelCatalogDiagnosisItem, 0, len(entry.Bindings))
	for _, binding := range entry.Bindings {
		account, err := h.accounts.GetAccount(c.Request.Context(), binding.AccountID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		reason := service.SchedulingBlockedReason(account)
		items = append(items, ModelCatalogDiagnosisItem{
			ModelCatalogBindingAccountSummary: ModelCatalogBindingAccountSummary{
				ID: account.ID, Name: account.Name, Platform: account.Platform, Type: account.Type,
				Vendor: account.Vendor(), Status: account.Status,
			},
			Priority:      binding.Priority,
			Schedulable:   reason == "",
			BlockedReason: reason,
			Serves:        service.CatalogRouteServes(entry, account),
		})
	}
	response.Success(c, ModelCatalogDiagnosisResponse{
		EntryID:       entry.ID,
		RoutePlatform: service.CatalogRoutePlatform(entry),
		Accounts:      items,
	})
}

// CreateAlias 新增别名。
// POST /api/v1/admin/model-catalog/aliases
func (h *ModelCatalogHandler) CreateAlias(c *gin.Context) {
	var req ModelCatalogAliasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	alias := &service.ModelCatalogAlias{
		Alias:   req.Alias,
		EntryID: req.EntryID,
		Source:  service.ModelCatalogAliasSourceManual,
		Notes:   req.Notes,
	}
	if err := h.service.CreateAlias(c.Request.Context(), alias); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, alias)
}

// UpdateAlias 更新别名。
// PUT /api/v1/admin/model-catalog/aliases/:id
func (h *ModelCatalogHandler) UpdateAlias(c *gin.Context) {
	id, ok := parseModelCatalogID(c, "Invalid model catalog alias ID")
	if !ok {
		return
	}
	var req ModelCatalogAliasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	alias := &service.ModelCatalogAlias{
		ID:      id,
		Alias:   req.Alias,
		EntryID: req.EntryID,
		Source:  service.ModelCatalogAliasSourceManual,
		Notes:   req.Notes,
	}
	if err := h.service.UpdateAlias(c.Request.Context(), alias); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, alias)
}

// DeleteAlias 删除别名。
// DELETE /api/v1/admin/model-catalog/aliases/:id
func (h *ModelCatalogHandler) DeleteAlias(c *gin.Context) {
	id, ok := parseModelCatalogID(c, "Invalid model catalog alias ID")
	if !ok {
		return
	}
	if err := h.service.DeleteAlias(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Model catalog alias deleted successfully"})
}

// Seed 重新播种：按价格文件 + 硬编码兜底价补齐目录。
// managed_by = admin 的条目永不被覆盖。
// POST /api/v1/admin/model-catalog/seed
func (h *ModelCatalogHandler) Seed(c *gin.Context) {
	result, err := h.service.Seed(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func parseModelCatalogID(c *gin.Context, message string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, message)
		return 0, false
	}
	return id, true
}
