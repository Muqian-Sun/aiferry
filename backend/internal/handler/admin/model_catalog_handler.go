package admin

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ModelCatalogHandler 处理模型目录的管理端请求。
type ModelCatalogHandler struct {
	service  *service.ModelCatalogService
	accounts ModelCatalogAccountSource
	settings ProfitSettingsSource
}

// NewModelCatalogHandler 创建模型目录处理器（含价格页接口）。
func NewModelCatalogHandler(svc *service.ModelCatalogService, accounts ModelCatalogAccountSource, settings ProfitSettingsSource) *ModelCatalogHandler {
	return &ModelCatalogHandler{service: svc, accounts: accounts, settings: settings}
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
	// 音频 token 价；空 = 音频 token 按文本输入 / 输出价计。
	AudioInputPrice  *float64 `json:"audio_input_price"`
	AudioOutputPrice *float64 `json:"audio_output_price"`

	PerRequestPrice    *float64 `json:"per_request_price"`
	SearchPricePerCall *float64 `json:"search_price_per_call"`
	// X 搜索按取回条目收的官方价（每条帖子、每个主页）；价格页填，这里原样写回。
	XPostPrice *float64 `json:"x_post_price"`
	XUserPrice *float64 `json:"x_user_price"`

	MaxReasoningEffortMultiplier *float64 `json:"max_reasoning_effort_multiplier"`

	Notes *string `json:"notes"`

	Intervals   []service.PricingInterval `json:"intervals"`
	TimePricing *service.TimePricing      `json:"time_pricing"`
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
		AudioInputPrice:     r.AudioInputPrice,
		AudioOutputPrice:    r.AudioOutputPrice,

		PerRequestPrice:    r.PerRequestPrice,
		SearchPricePerCall: r.SearchPricePerCall,
		XPostPrice:         r.XPostPrice,
		XUserPrice:         r.XUserPrice,

		MaxReasoningEffortMultiplier: r.MaxReasoningEffortMultiplier,

		Notes:       r.Notes,
		Intervals:   r.Intervals,
		TimePricing: r.TimePricing,
	}
}

// ModelCatalogBindingResponse 承接关系（渠道给这个模型的上游价，USD / token）及其账号摘要。
type ModelCatalogBindingResponse struct {
	EntryID           int64                              `json:"entry_id"`
	AccountID         int64                              `json:"account_id"`
	InputPrice        float64                            `json:"input_price"`
	OutputPrice       float64                            `json:"output_price"`
	CacheWritePrice   *float64                           `json:"cache_write_price"`
	CacheWrite1hPrice *float64                           `json:"cache_write_1h_price"`
	CacheReadPrice    *float64                           `json:"cache_read_price"`
	Intervals         []service.PricingInterval          `json:"intervals"`
	CreatedAt         time.Time                          `json:"created_at"`
	UpdatedAt         time.Time                          `json:"updated_at"`
	Account           *ModelCatalogBindingAccountSummary `json:"account,omitempty"`
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
	// Schedulable 账号此刻能否进入调度；BlockedReason 不能时的第一个原因
	// （disabled / unschedulable / expired / overloaded / rate_limited / temp_unschedulable / quota_exceeded）。
	Schedulable   bool   `json:"schedulable"`
	BlockedReason string `json:"blocked_reason,omitempty"`
	// Serves 入站协议 → 能否承接（anthropic / chat_completions / responses / gemini）。
	Serves map[string]bool `json:"serves"`
}

// ModelCatalogDiagnosisResponse 条目的资源诊断。
type ModelCatalogDiagnosisResponse struct {
	EntryID  int64                       `json:"entry_id"`
	Accounts []ModelCatalogDiagnosisItem `json:"accounts"`
}

// ModelCatalogEntryView 是列表里的条目，附带厂商族：渠道表单按厂商族（与渠道平台同一套标识）
// 分组展示目录模型；厂商 → 平台的对照只在后端维护（CatalogVendorPlatform），前端不留副本。
type ModelCatalogEntryView struct {
	service.ModelCatalogEntry
	// VendorPlatform 厂商族，认不出的厂商（含空厂商）为空串。
	VendorPlatform string `json:"vendor_platform"`
	// ExtensionEndpoints 经扩展端点（生图 / 视频 / 向量）承接：渠道表单的默认勾选不含这类模型。
	ExtensionEndpoints bool `json:"extension_endpoints"`
	// SchedulableChannels 能派到请求的承接渠道数（按新用户默认倍率过利润门）；为 0 时 UnschedulableReason 给原因：
	// no_bindings / channels_disabled / profit_gate。列表标「没有能调度的渠道」、上架前提示用（D6）。
	SchedulableChannels int    `json:"schedulable_channels"`
	UnschedulableReason string `json:"unschedulable_reason,omitempty"`
}

// ListEntries 返回全部目录条目（含别名、分档、分时）。
// GET /api/v1/admin/model-catalog/entries
func (h *ModelCatalogHandler) ListEntries(c *gin.Context) {
	ctx := c.Request.Context()
	entries, err := h.service.ListEntries(ctx)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	accounts, err := h.listAllAccounts(ctx)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	byID := make(map[int64]*service.Account, len(accounts))
	for i := range accounts {
		byID[accounts[i].ID] = &accounts[i]
	}
	profit := h.settings.GetProfitControlSettings(ctx)
	views := make([]ModelCatalogEntryView, len(entries))
	for i := range entries {
		schedulable, reason := service.SchedulableBindings(&entries[i], byID, profit)
		views[i] = ModelCatalogEntryView{
			ModelCatalogEntry:   entries[i],
			VendorPlatform:      service.CatalogVendorPlatform(&entries[i]),
			ExtensionEndpoints:  h.service.EntryServedByExtensionEndpoints(&entries[i]),
			SchedulableChannels: schedulable,
			UnschedulableReason: reason,
		}
	}
	response.Success(c, views)
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

// ListBindings 返回条目的承接关系（上游价）及账号摘要。
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
		intervals := binding.Intervals
		if intervals == nil {
			intervals = []service.PricingInterval{}
		}
		item := ModelCatalogBindingResponse{
			EntryID:           binding.EntryID,
			AccountID:         binding.AccountID,
			InputPrice:        binding.InputPrice,
			OutputPrice:       binding.OutputPrice,
			CacheWritePrice:   binding.CacheWritePrice,
			CacheWrite1hPrice: binding.CacheWrite1hPrice,
			CacheReadPrice:    binding.CacheReadPrice,
			Intervals:         intervals,
			CreatedAt:         binding.CreatedAt,
			UpdatedAt:         binding.UpdatedAt,
		}
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

// PriceLookup 按模型 ID 带出建议条目（厂商、计费方式、价格），给「添加模型」自动填：先查内置价格资料，
// 查不到再查联网的官方模型名单。
// GET /api/v1/admin/model-catalog/price-lookup?model_id=
func (h *ModelCatalogHandler) PriceLookup(c *gin.Context) {
	modelID := strings.TrimSpace(c.Query("model_id"))
	if modelID == "" {
		response.BadRequest(c, "model_id is required")
		return
	}
	entry, ok := h.service.LookupPriceEntry(c.Request.Context(), modelID)
	if !ok {
		response.Success(c, gin.H{"found": false})
		return
	}
	response.Success(c, gin.H{"found": true, "entry": entry})
}

// officialLookupMaxModels 一次最多查多少个模型 ID（拍的：上游名单常见几十到一两百个）。
const officialLookupMaxModels = 500

// OfficialLookupRequest 批量判断上游模型 ID 是不是官方 ID。
type OfficialLookupRequest struct {
	ModelIDs []string `json:"model_ids" binding:"required"`
}

// OfficialLookup 逐个判断是不是官方模型 ID（联网的 LiteLLM 公开价格表，只认官方厂商、精确 ID），是的带官方价。
// 建渠道「承接模型」那一步对目录里没有的上游模型用：官方的加进目录，不是的在承接关系上做别名映射。
// POST /api/v1/admin/model-catalog/official-lookup
func (h *ModelCatalogHandler) OfficialLookup(c *gin.Context) {
	var req OfficialLookupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if len(req.ModelIDs) > officialLookupMaxModels {
		response.BadRequest(c, fmt.Sprintf("model_ids accepts at most %d items", officialLookupMaxModels))
		return
	}
	response.Success(c, h.service.LookupOfficialModels(c.Request.Context(), req.ModelIDs))
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
			Schedulable:   reason == "",
			BlockedReason: reason,
			Serves:        service.CatalogBindingServes(account),
		})
	}
	response.Success(c, ModelCatalogDiagnosisResponse{
		EntryID:  entry.ID,
		Accounts: items,
	})
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

// UpstreamModelIDsRequest 一批上游模型名（「探测模型」拿到的名单）。
type UpstreamModelIDsRequest struct {
	ModelIDs []string `json:"model_ids" binding:"required"`
}

// MatchUpstreamModels 把上游模型名对到目录条目（按规范化模型名与别名，再试去掉厂商前缀），没对上的 entry_id 为空。
// POST /api/v1/admin/model-catalog/entries/match
func (h *ModelCatalogHandler) MatchUpstreamModels(c *gin.Context) {
	var req UpstreamModelIDsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	models, err := h.service.MatchUpstreamModels(c.Request.Context(), req.ModelIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"models": models})
}

// ImportUpstreamModels 把目录里还没有的上游模型建成未上架条目（内置价格表查得到就带价），已有的原样返回。
// POST /api/v1/admin/model-catalog/entries/import
func (h *ModelCatalogHandler) ImportUpstreamModels(c *gin.Context) {
	var req UpstreamModelIDsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	entries, err := h.service.ImportUpstreamModels(c.Request.Context(), req.ModelIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"entries": entries})
}
