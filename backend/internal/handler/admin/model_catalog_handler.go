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
}

// NewModelCatalogHandler 创建模型目录处理器。
func NewModelCatalogHandler(svc *service.ModelCatalogService) *ModelCatalogHandler {
	return &ModelCatalogHandler{service: svc}
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
		ModelID:     r.ModelID,
		DisplayName: r.DisplayName,
		Vendor:      r.Vendor,
		Protocols:   r.Protocols,
		BillingMode: service.BillingMode(r.BillingMode),
		Status:      r.Status,

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
