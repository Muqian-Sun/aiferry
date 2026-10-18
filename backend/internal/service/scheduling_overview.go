package service

// 调度总览（管理站「调度 · 实时」）：每个模型的每条入站协议线上，渠道此刻的调度顺序或不被选的原因，
// 叠加并发 / 排队、粘性会话数和时间窗内实际接了多少请求。顺序与原因来自 PreviewCatalogScheduling，
// 请求数来自请求级调度轨迹（按入站协议分开）。

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// ErrSchedulingWindowInvalid 时间窗只接受 schedulingOverviewWindows 里的值。
var ErrSchedulingWindowInvalid = infraerrors.BadRequest("SCHEDULING_WINDOW_INVALID", "window must be one of 15m, 1h, 24h")

var schedulingOverviewWindows = map[string]time.Duration{
	"15m": 15 * time.Minute,
	"1h":  time.Hour,
	"24h": 24 * time.Hour,
}

// SchedulingWindowDuration 时间窗的长度；不认识的时间窗返回 ErrSchedulingWindowInvalid。
func SchedulingWindowDuration(window string) (time.Duration, error) {
	span, ok := schedulingOverviewWindows[window]
	if !ok {
		return 0, ErrSchedulingWindowInvalid
	}
	return span, nil
}

// 总览里拿不到、但不影响其余内容的部分，前端据此提示。
const (
	SchedulingWarningLoadUnavailable   = "load_unavailable"
	SchedulingWarningStickyUnavailable = "sticky_unavailable"
)

// StickySessionCounter 数当前的粘性会话绑定：调度 scope（目录条目 ID）→ 渠道 → 会话数。
type StickySessionCounter interface {
	CountStickySessions(ctx context.Context) (map[int64]map[int64]int, error)
}

// SchedulingCounts 时间窗内的请求数；Unrouted 是没选到渠道的（只出现在模型上）。
type SchedulingCounts struct {
	Requests int64 `json:"requests"`
	Failures int64 `json:"failures"`
	Unrouted int64 `json:"unrouted"`
}

func (c *SchedulingCounts) add(row SchedulingTraceCount) {
	c.Requests += row.Requests
	if row.Outcome == SchedulingTraceOutcomeFailed {
		c.Failures += row.Requests
	}
	if row.AccountID == 0 {
		c.Unrouted += row.Requests
	}
}

// SchedulingOverview 管理站调度页的数据。
type SchedulingOverview struct {
	GeneratedAt time.Time           `json:"generated_at"`
	Window      string              `json:"window"`
	Warnings    []string            `json:"warnings"`
	Channels    []SchedulingChannel `json:"channels"`
	Models      []SchedulingModel   `json:"models"`
}

// SchedulingChannel 渠道此刻的整体状态与负载；请求数是时间窗内全部模型、全部协议的合计。
type SchedulingChannel struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	Platform      string     `json:"platform"`
	Type          string     `json:"type"`
	Status        string     `json:"status"`
	Priority      int        `json:"priority"`
	Concurrency   int        `json:"concurrency"`
	InUse         *int       `json:"in_use"`
	Waiting       *int       `json:"waiting"`
	BlockedReason string     `json:"blocked_reason,omitempty"`
	BlockedUntil  *time.Time `json:"blocked_until,omitempty"`
	Requests      int64      `json:"requests"`
	Failures      int64      `json:"failures"`
}

// SchedulingModel 一个目录条目：每条入站协议一条调度线，加上时间窗内的请求数（合计与按协议）。
type SchedulingModel struct {
	EntryID     int64                       `json:"entry_id"`
	ModelID     string                      `json:"model_id"`
	DisplayName string                      `json:"display_name"`
	Vendor      string                      `json:"vendor"`
	Status      string                      `json:"status"`
	Requests    int64                       `json:"requests"`
	Failures    int64                       `json:"failures"`
	Unrouted    int64                       `json:"unrouted"`
	ByProtocol  map[string]SchedulingCounts `json:"by_protocol"`
	Lanes       []SchedulingLane            `json:"lanes"`
	Channels    []SchedulingModelChannel    `json:"channels"`
}

// SchedulingLane 一条入站协议线上渠道的调度顺序。
type SchedulingLane struct {
	Protocol string                  `json:"protocol"`
	Items    []SchedulingPreviewItem `json:"items"`
}

// SchedulingModelChannel 时间窗内这个模型落到某渠道的请求数（合计与按协议），以及此刻绑在它上面的粘性会话数。
type SchedulingModelChannel struct {
	AccountID      int64                       `json:"account_id"`
	Requests       int64                       `json:"requests"`
	Failures       int64                       `json:"failures"`
	ByProtocol     map[string]SchedulingCounts `json:"by_protocol"`
	StickySessions int                         `json:"sticky_sessions"`
}

// SchedulingOverviewService 组装调度总览，并提供请求轨迹的查询。
type SchedulingOverviewService struct {
	catalog  *ModelCatalogService
	accounts AccountRepository
	gateway  *GatewayService
	traces   SchedulingTraceRepository
	sticky   StickySessionCounter
}

// NewSchedulingOverviewService 创建调度总览服务。
func NewSchedulingOverviewService(catalog *ModelCatalogService, accounts AccountRepository, gateway *GatewayService, traces SchedulingTraceRepository, sticky StickySessionCounter) *SchedulingOverviewService {
	return &SchedulingOverviewService{catalog: catalog, accounts: accounts, gateway: gateway, traces: traces, sticky: sticky}
}

// schedulingEntryCounts 一个条目的请求数：按渠道（0 = 没选到渠道）× 入站协议。
type schedulingEntryCounts map[int64]map[string]*SchedulingCounts

func (m schedulingEntryCounts) add(row SchedulingTraceCount) {
	if m[row.AccountID] == nil {
		m[row.AccountID] = map[string]*SchedulingCounts{}
	}
	if m[row.AccountID][row.InboundProtocol] == nil {
		m[row.AccountID][row.InboundProtocol] = &SchedulingCounts{}
	}
	m[row.AccountID][row.InboundProtocol].add(row)
}

// Overview 组装 window（15m / 1h / 24h）的调度总览。列出上架条目里有绑定、或时间窗内有请求的
// （没有绑定却有请求 = 全是没选到渠道的失败）。
func (s *SchedulingOverviewService) Overview(ctx context.Context, window string) (*SchedulingOverview, error) {
	span, err := SchedulingWindowDuration(window)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	entries, err := s.catalog.ListEntries(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.traces.CountSchedulingTraces(ctx, now.Add(-span))
	if err != nil {
		return nil, err
	}

	byEntry := map[int64]schedulingEntryCounts{}
	byAccount := map[int64]*SchedulingCounts{}
	for _, row := range rows {
		if row.AccountID > 0 {
			if byAccount[row.AccountID] == nil {
				byAccount[row.AccountID] = &SchedulingCounts{}
			}
			byAccount[row.AccountID].add(row)
		}
		if row.CatalogEntryID == 0 {
			continue
		}
		if byEntry[row.CatalogEntryID] == nil {
			byEntry[row.CatalogEntryID] = schedulingEntryCounts{}
		}
		byEntry[row.CatalogEntryID].add(row)
	}

	var selected []*ModelCatalogEntry
	accountIDs := map[int64]struct{}{}
	for i := range entries {
		entry := &entries[i]
		if entry.Status != ModelCatalogStatusListed || (len(entry.Bindings) == 0 && byEntry[entry.ID] == nil) {
			continue
		}
		selected = append(selected, entry)
		for _, binding := range entry.Bindings {
			accountIDs[binding.AccountID] = struct{}{}
		}
	}
	ids := make([]int64, 0, len(accountIDs))
	for id := range accountIDs {
		ids = append(ids, id)
	}
	var accounts []*Account
	if len(ids) > 0 {
		if accounts, err = s.accounts.GetByIDs(ctx, ids); err != nil {
			return nil, err
		}
	}
	accountByID := make(map[int64]*Account, len(accounts))
	for _, acc := range accounts {
		accountByID[acc.ID] = acc
	}

	overview := &SchedulingOverview{GeneratedAt: now, Window: window, Warnings: []string{}}
	loads, err := s.gateway.SchedulingLoads(ctx, accounts)
	if err != nil {
		loads = nil
		overview.Warnings = append(overview.Warnings, SchedulingWarningLoadUnavailable)
		if !errors.Is(err, errLoadBatchDisabled) {
			slog.Warn("scheduling_overview.load_unavailable", "error", err)
		}
	}
	sticky, err := s.sticky.CountStickySessions(ctx)
	if err != nil {
		sticky = nil
		overview.Warnings = append(overview.Warnings, SchedulingWarningStickyUnavailable)
		slog.Warn("scheduling_overview.sticky_unavailable", "error", err)
	}

	overview.Channels = make([]SchedulingChannel, 0, len(accounts))
	for _, acc := range accounts {
		state := acc.SchedulingState(now)
		reason, _ := displaySchedulingState(acc, state)
		channel := SchedulingChannel{
			ID: acc.ID, Name: acc.Name, Platform: acc.Platform, Type: acc.Type, Status: acc.Status,
			Priority: acc.Priority, Concurrency: acc.Concurrency,
			BlockedReason: reason, BlockedUntil: state.BlockedUntil,
		}
		if info := loads[acc.ID]; info != nil {
			inUse, waiting := info.CurrentConcurrency, info.WaitingCount
			channel.InUse, channel.Waiting = &inUse, &waiting
		}
		if c := byAccount[acc.ID]; c != nil {
			channel.Requests, channel.Failures = c.Requests, c.Failures
		}
		overview.Channels = append(overview.Channels, channel)
	}
	sort.Slice(overview.Channels, func(i, j int) bool { return overview.Channels[i].ID < overview.Channels[j].ID })

	overview.Models = make([]SchedulingModel, 0, len(selected))
	for _, entry := range selected {
		model := SchedulingModel{
			EntryID: entry.ID, ModelID: entry.ModelID, DisplayName: entry.DisplayName, Vendor: entry.Vendor, Status: entry.Status,
			ByProtocol: map[string]SchedulingCounts{}, Lanes: []SchedulingLane{},
		}
		bound := make([]*Account, 0, len(entry.Bindings))
		for _, binding := range entry.Bindings {
			if acc := accountByID[binding.AccountID]; acc != nil {
				bound = append(bound, acc)
			}
		}
		for _, protocol := range UpstreamProtocols() {
			if !anyAccountServesInbound(bound, protocol) {
				continue
			}
			model.Lanes = append(model.Lanes, SchedulingLane{
				Protocol: protocol,
				Items:    s.gateway.PreviewCatalogScheduling(ctx, entry, bound, protocol, loads),
			})
		}
		counts := byEntry[entry.ID]
		for _, perProtocol := range counts {
			for protocol, c := range perProtocol {
				total := model.ByProtocol[protocol]
				total.Requests += c.Requests
				total.Failures += c.Failures
				total.Unrouted += c.Unrouted
				model.ByProtocol[protocol] = total
				model.Requests += c.Requests
				model.Failures += c.Failures
				model.Unrouted += c.Unrouted
			}
		}
		model.Channels = make([]SchedulingModelChannel, 0, len(bound))
		for _, acc := range bound {
			channel := SchedulingModelChannel{AccountID: acc.ID, StickySessions: sticky[entry.ID][acc.ID], ByProtocol: map[string]SchedulingCounts{}}
			for protocol, c := range counts[acc.ID] {
				channel.ByProtocol[protocol] = SchedulingCounts{Requests: c.Requests, Failures: c.Failures}
				channel.Requests += c.Requests
				channel.Failures += c.Failures
			}
			model.Channels = append(model.Channels, channel)
		}
		overview.Models = append(overview.Models, model)
	}
	sort.SliceStable(overview.Models, func(i, j int) bool {
		a, b := overview.Models[i], overview.Models[j]
		if a.Requests != b.Requests {
			return a.Requests > b.Requests
		}
		return a.ModelID < b.ModelID
	})
	return overview, nil
}

// SchedulingTracePage 请求轨迹的一页。
type SchedulingTracePage struct {
	Items    []SchedulingTrace `json:"items"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

// 请求轨迹列表的结果筛选值；空 = 全部。
var schedulingTraceOutcomeFilters = map[string]bool{
	"": true, "switched": true,
	SchedulingTraceOutcomeSuccess: true, SchedulingTraceOutcomeFailed: true,
	SchedulingTraceOutcomeNoChannel: true, SchedulingTraceOutcomeCanceled: true,
}

// ErrSchedulingTraceFilterInvalid 筛选值不认识。
var ErrSchedulingTraceFilterInvalid = infraerrors.BadRequest("SCHEDULING_TRACE_FILTER_INVALID", "invalid outcome or protocol filter")

// schedulingTraceMaxPageSize 每页最多条数。
const schedulingTraceMaxPageSize = 100

// ListTraces 查 window 内的请求轨迹（新的在前）。
func (s *SchedulingOverviewService) ListTraces(ctx context.Context, window string, filter SchedulingTraceFilter, page, pageSize int) (*SchedulingTracePage, error) {
	span, err := SchedulingWindowDuration(window)
	if err != nil {
		return nil, err
	}
	if !schedulingTraceOutcomeFilters[filter.Outcome] {
		return nil, ErrSchedulingTraceFilterInvalid
	}
	if filter.InboundProtocol != "" && !IsUpstreamProtocol(filter.InboundProtocol) {
		return nil, ErrSchedulingTraceFilterInvalid
	}
	page = max(page, 1)
	pageSize = min(max(pageSize, 1), schedulingTraceMaxPageSize)
	filter.Since = time.Now().Add(-span)
	filter.Limit, filter.Offset = pageSize, (page-1)*pageSize
	items, total, err := s.traces.ListSchedulingTraces(ctx, filter)
	if err != nil {
		return nil, err
	}
	return &SchedulingTracePage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func anyAccountServesInbound(accounts []*Account, inbound string) bool {
	for _, acc := range accounts {
		if acc.ServesInbound(inbound) {
			return true
		}
	}
	return false
}
