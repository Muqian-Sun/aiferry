package admin

import (
	"context"
	"net/url"
	"sort"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// 价格页（管理站「供给 › 价格」）的读写接口：官方价与上游价都在这里改，给模型加一个渠道就是承接。
// 按模型保存一块 = 官方价 + 这个模型的全部承接关系；按渠道保存一块 = 这个渠道承接的全部模型与上游价。

// ModelCatalogAccountSource 目录接口读账号：承接关系、诊断按 ID 取；价格页列全部渠道。生产上是 AdminService。
type ModelCatalogAccountSource interface {
	GetAccount(ctx context.Context, id int64) (*service.Account, error)
	ListAccounts(ctx context.Context, page, pageSize int, platform, accountType, status, search string, privacyMode string, sortBy, sortOrder string) ([]service.Account, int64, error)
}

// ProfitSettingsSource 价格页标毛利用的最低毛利率。生产上是 SettingService。
type ProfitSettingsSource interface {
	GetProfitControlSettings(ctx context.Context) service.ProfitControlSettings
}

// pricingAccountPageSize 价格页列全部渠道时每页取多少（分页读到总数为止）。
const pricingAccountPageSize = 500

// PricingOverviewResponse 价格页的全部数据。
type PricingOverviewResponse struct {
	// DefaultUserRate 默认售价倍率（官方价 × 它 = 售价），毛利按它算。
	DefaultUserRate float64 `json:"default_user_rate"`
	// MinMargin 利润门的最低毛利率，0 = 利润门关闭。
	MinMargin float64                  `json:"min_margin"`
	Entries   []PricingEntryResponse   `json:"entries"`
	Accounts  []PricingAccountResponse `json:"accounts"`
}

// PricingEntryResponse 价格页上的一个模型：官方价、分段与承接关系。
type PricingEntryResponse struct {
	ID                int64                     `json:"id"`
	ModelID           string                    `json:"model_id"`
	DisplayName       string                    `json:"display_name"`
	Vendor            string                    `json:"vendor"`
	Status            string                    `json:"status"`
	InputPrice        *float64                  `json:"input_price"`
	OutputPrice       *float64                  `json:"output_price"`
	CacheWritePrice   *float64                  `json:"cache_write_price"`
	CacheWrite1hPrice *float64                  `json:"cache_write_1h_price"`
	CacheReadPrice    *float64                  `json:"cache_read_price"`
	Intervals         []service.PricingInterval `json:"intervals"`
	Bindings          []PricingBindingResponse  `json:"bindings"`
	// BindableAccountIDs 能承接这个模型的渠道（「加渠道」只列这些里还没承接的）。
	BindableAccountIDs []int64 `json:"bindable_account_ids"`
}

// PricingBindingResponse 一条承接关系的上游价。
type PricingBindingResponse struct {
	EntryID           int64                     `json:"entry_id"`
	AccountID         int64                     `json:"account_id"`
	InputPrice        float64                   `json:"input_price"`
	OutputPrice       float64                   `json:"output_price"`
	CacheWritePrice   *float64                  `json:"cache_write_price"`
	CacheWrite1hPrice *float64                  `json:"cache_write_1h_price"`
	CacheReadPrice    *float64                  `json:"cache_read_price"`
	Intervals         []service.PricingInterval `json:"intervals"`
	// CostRatio 上游成本比（上游价 ÷ 官方价，逐项、逐段取最高），与利润门同一个数；官方价没有可比项时为 null。
	CostRatio *float64 `json:"cost_ratio"`
}

// PricingAccountResponse 价格页上的一个渠道。
type PricingAccountResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Platform    string `json:"platform"`
	Type        string `json:"type"`
	Vendor      string `json:"vendor"`
	Status      string `json:"status"`
	Schedulable bool   `json:"schedulable"`
	Priority    int    `json:"priority"`
	// Protocol 第三方 key 的上游协议；成品号为空。
	Protocol string `json:"protocol"`
	// UpstreamHost 第三方 key 上游地址的主机名（「从同上游的渠道复制价格」按它找）；成品号为空。
	UpstreamHost string `json:"upstream_host"`
}

// PricingPricesRequest 五项 token 价（USD / token）与按 Token 分段。
type PricingPricesRequest struct {
	InputPrice        *float64                  `json:"input_price"`
	OutputPrice       *float64                  `json:"output_price"`
	CacheWritePrice   *float64                  `json:"cache_write_price"`
	CacheWrite1hPrice *float64                  `json:"cache_write_1h_price"`
	CacheReadPrice    *float64                  `json:"cache_read_price"`
	Intervals         []service.PricingInterval `json:"intervals"`
}

// PricingModelBindingRequest 按模型保存时的一条承接关系。
type PricingModelBindingRequest struct {
	AccountID int64 `json:"account_id" binding:"required"`
	PricingPricesRequest
}

// PricingModelSaveRequest 按模型保存一块：官方价 + 这个模型的全部承接关系（整份覆盖）。
type PricingModelSaveRequest struct {
	PricingPricesRequest
	Bindings []PricingModelBindingRequest `json:"bindings"`
}

// PricingChannelBindingRequest 按渠道保存时的一条承接关系。
type PricingChannelBindingRequest struct {
	EntryID int64 `json:"entry_id" binding:"required"`
	PricingPricesRequest
}

// PricingChannelSaveRequest 按渠道保存一块：这个渠道承接的全部模型与上游价（整份覆盖）。
type PricingChannelSaveRequest struct {
	Bindings []PricingChannelBindingRequest `json:"bindings"`
}

// toBinding 上游价的输入 / 输出必填（muqian：「必须填，没填不能承接」）。
func (r *PricingPricesRequest) toBinding(entryID, accountID int64) (service.ModelCatalogBinding, string) {
	if r.InputPrice == nil || r.OutputPrice == nil {
		return service.ModelCatalogBinding{}, "upstream input_price and output_price are required"
	}
	return service.ModelCatalogBinding{
		EntryID:           entryID,
		AccountID:         accountID,
		InputPrice:        *r.InputPrice,
		OutputPrice:       *r.OutputPrice,
		CacheWritePrice:   r.CacheWritePrice,
		CacheWrite1hPrice: r.CacheWrite1hPrice,
		CacheReadPrice:    r.CacheReadPrice,
		Intervals:         r.Intervals,
	}, ""
}

// PricingOverview 价格页的全部数据：按 Token 计费的模型（官方价、承接关系与上游成本比）、全部渠道、毛利口径。
// GET /api/v1/admin/pricing
func (h *ModelCatalogHandler) PricingOverview(c *gin.Context) {
	ctx := c.Request.Context()
	entries, err := h.service.ListPricingEntries(ctx)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	accounts, err := h.listAllAccounts(ctx)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := PricingOverviewResponse{
		DefaultUserRate: service.NewUserRateMultiplier,
		MinMargin:       h.settings.GetProfitControlSettings(ctx).MinMargin,
		Entries:         make([]PricingEntryResponse, 0, len(entries)),
		Accounts:        make([]PricingAccountResponse, 0, len(accounts)),
	}
	for i := range entries {
		out.Entries = append(out.Entries, h.pricingEntryResponse(&entries[i], accounts))
	}
	for i := range accounts {
		out.Accounts = append(out.Accounts, pricingAccountResponse(&accounts[i]))
	}
	response.Success(c, out)
}

// SavePricingModel 按模型保存一块。返回保存后的这个模型。
// PUT /api/v1/admin/pricing/models/:id
func (h *ModelCatalogHandler) SavePricingModel(c *gin.Context) {
	id, ok := parseModelCatalogID(c, "Invalid model catalog entry ID")
	if !ok {
		return
	}
	var req PricingModelSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	bindings := make([]service.ModelCatalogBinding, 0, len(req.Bindings))
	for i := range req.Bindings {
		binding, msg := req.Bindings[i].toBinding(id, req.Bindings[i].AccountID)
		if msg != "" {
			response.BadRequest(c, msg)
			return
		}
		bindings = append(bindings, binding)
	}
	official := service.OfficialPrices{
		InputPrice:        req.InputPrice,
		OutputPrice:       req.OutputPrice,
		CacheWritePrice:   req.CacheWritePrice,
		CacheWrite1hPrice: req.CacheWrite1hPrice,
		CacheReadPrice:    req.CacheReadPrice,
		Intervals:         req.Intervals,
	}
	ctx := c.Request.Context()
	entry, err := h.service.SaveEntryPricing(ctx, id, official, bindings, h.accounts)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	accounts, err := h.listAllAccounts(ctx)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, h.pricingEntryResponse(entry, accounts))
}

// SavePricingChannel 按渠道保存一块。返回保存后这个渠道的承接关系（带上游成本比）。
// PUT /api/v1/admin/pricing/channels/:id
func (h *ModelCatalogHandler) SavePricingChannel(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	var req PricingChannelSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	bindings := make([]service.ModelCatalogBinding, 0, len(req.Bindings))
	for i := range req.Bindings {
		binding, msg := req.Bindings[i].toBinding(req.Bindings[i].EntryID, accountID)
		if msg != "" {
			response.BadRequest(c, msg)
			return
		}
		bindings = append(bindings, binding)
	}
	ctx := c.Request.Context()
	saved, err := h.service.SaveAccountPricing(ctx, accountID, bindings, h.accounts)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]PricingBindingResponse, 0, len(saved))
	for i := range saved {
		entry, err := h.service.GetEntry(ctx, saved[i].EntryID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		out = append(out, pricingBindingResponse(entry, &saved[i]))
	}
	response.Success(c, out)
}

// listAllAccounts 分页读完全部渠道（按优先级、ID 排序）。
func (h *ModelCatalogHandler) listAllAccounts(ctx context.Context) ([]service.Account, error) {
	var all []service.Account
	for page := 1; ; page++ {
		accounts, total, err := h.accounts.ListAccounts(ctx, page, pricingAccountPageSize, "", "", "", "", "", "priority", "asc")
		if err != nil {
			return nil, err
		}
		all = append(all, accounts...)
		if len(accounts) == 0 || int64(len(all)) >= total {
			return all, nil
		}
	}
}

func (h *ModelCatalogHandler) pricingEntryResponse(entry *service.ModelCatalogEntry, accounts []service.Account) PricingEntryResponse {
	out := PricingEntryResponse{
		ID:                 entry.ID,
		ModelID:            entry.ModelID,
		DisplayName:        entry.DisplayName,
		Vendor:             entry.Vendor,
		Status:             entry.Status,
		InputPrice:         entry.InputPrice,
		OutputPrice:        entry.OutputPrice,
		CacheWritePrice:    entry.CacheWritePrice,
		CacheWrite1hPrice:  entry.CacheWrite1hPrice,
		CacheReadPrice:     entry.CacheReadPrice,
		Intervals:          nonNilIntervals(entry.Intervals),
		Bindings:           make([]PricingBindingResponse, 0, len(entry.Bindings)),
		BindableAccountIDs: make([]int64, 0),
	}
	for i := range entry.Bindings {
		out.Bindings = append(out.Bindings, pricingBindingResponse(entry, &entry.Bindings[i]))
	}
	for i := range accounts {
		if h.service.CanBindAccount(entry, &accounts[i]) {
			out.BindableAccountIDs = append(out.BindableAccountIDs, accounts[i].ID)
		}
	}
	return out
}

func pricingBindingResponse(entry *service.ModelCatalogEntry, b *service.ModelCatalogBinding) PricingBindingResponse {
	out := PricingBindingResponse{
		EntryID:           b.EntryID,
		AccountID:         b.AccountID,
		InputPrice:        b.InputPrice,
		OutputPrice:       b.OutputPrice,
		CacheWritePrice:   b.CacheWritePrice,
		CacheWrite1hPrice: b.CacheWrite1hPrice,
		CacheReadPrice:    b.CacheReadPrice,
		Intervals:         nonNilIntervals(b.Intervals),
	}
	if ratio, ok := entry.UpstreamCostRatio(b); ok {
		out.CostRatio = &ratio
	}
	return out
}

func pricingAccountResponse(a *service.Account) PricingAccountResponse {
	out := PricingAccountResponse{
		ID:          a.ID,
		Name:        a.Name,
		Platform:    a.Platform,
		Type:        a.Type,
		Vendor:      a.Vendor(),
		Status:      a.Status,
		Schedulable: a.IsSchedulable(),
		Priority:    a.Priority,
	}
	if a.IsThirdPartyKey() {
		protocols := make([]string, 0, len(a.ProtocolEndpoints))
		for protocol := range a.ProtocolEndpoints {
			protocols = append(protocols, protocol)
		}
		sort.Strings(protocols)
		if len(protocols) > 0 {
			out.Protocol = protocols[0]
			if u, err := url.Parse(a.ProtocolEndpoint(protocols[0])); err == nil {
				out.UpstreamHost = u.Hostname()
			}
		}
	}
	return out
}

func nonNilIntervals(intervals []service.PricingInterval) []service.PricingInterval {
	if intervals == nil {
		return []service.PricingInterval{}
	}
	return intervals
}
