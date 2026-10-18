package service

// 调度总览（管理站「调度」页）：每个模型的每条入站协议线上，渠道此刻的调度顺序或不被选的原因，
// 叠加并发 / 排队、粘性会话数和时间窗内实际接了多少请求。顺序与原因来自 PreviewCatalogScheduling，
// 请求数来自渠道状态的分钟汇总（与渠道状态页同一口径）。

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

// 总览里拿不到、但不影响其余内容的部分，前端据此提示。
const (
	SchedulingWarningLoadUnavailable   = "load_unavailable"
	SchedulingWarningStickyUnavailable = "sticky_unavailable"
)

// SchedulingTrafficRow 一个时间窗内某模型（原始模型名，可能是别名）在某渠道上的请求数；AccountID 0 = 没选到渠道。
type SchedulingTrafficRow struct {
	Model     string
	AccountID int64
	Success   int64
	Errors    int64
}

// SchedulingTrafficSource 读时间窗内的请求汇总；dataThrough 是汇总已覆盖到的时刻（还没汇总过为 nil）。
type SchedulingTrafficSource interface {
	SchedulingTraffic(ctx context.Context, since time.Time) (rows []SchedulingTrafficRow, dataThrough *time.Time, err error)
}

// StickySessionCounter 数当前的粘性会话绑定：调度 scope（目录条目 ID）→ 渠道 → 会话数。
type StickySessionCounter interface {
	CountStickySessions(ctx context.Context) (map[int64]map[int64]int, error)
}

// SchedulingOverview 管理站调度页的数据。
type SchedulingOverview struct {
	GeneratedAt time.Time           `json:"generated_at"`
	Window      string              `json:"window"`
	DataThrough *time.Time          `json:"data_through,omitempty"`
	Warnings    []string            `json:"warnings"`
	Channels    []SchedulingChannel `json:"channels"`
	Models      []SchedulingModel   `json:"models"`
}

// SchedulingChannel 渠道此刻的整体状态与负载；请求数是时间窗内全部模型的合计。
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

// SchedulingModel 一个目录条目：每条入站协议一条调度线，加上时间窗内按渠道分的请求数。
type SchedulingModel struct {
	EntryID     int64                    `json:"entry_id"`
	ModelID     string                   `json:"model_id"`
	DisplayName string                   `json:"display_name"`
	Vendor      string                   `json:"vendor"`
	Status      string                   `json:"status"`
	Requests    int64                    `json:"requests"`
	Failures    int64                    `json:"failures"`
	Unrouted    int64                    `json:"unrouted"`
	Lanes       []SchedulingLane         `json:"lanes"`
	Channels    []SchedulingModelChannel `json:"channels"`
}

// SchedulingLane 一条入站协议线上渠道的调度顺序。
type SchedulingLane struct {
	Protocol string                  `json:"protocol"`
	Items    []SchedulingPreviewItem `json:"items"`
}

// SchedulingModelChannel 时间窗内这个模型落到某渠道的请求数，以及此刻绑在它上面的粘性会话数。
type SchedulingModelChannel struct {
	AccountID      int64 `json:"account_id"`
	Requests       int64 `json:"requests"`
	Failures       int64 `json:"failures"`
	StickySessions int   `json:"sticky_sessions"`
}

// SchedulingOverviewService 组装调度总览。
type SchedulingOverviewService struct {
	catalog  *ModelCatalogService
	accounts AccountRepository
	gateway  *GatewayService
	traffic  SchedulingTrafficSource
	sticky   StickySessionCounter
}

// NewSchedulingOverviewService 创建调度总览服务。
func NewSchedulingOverviewService(catalog *ModelCatalogService, accounts AccountRepository, gateway *GatewayService, traffic SchedulingTrafficSource, sticky StickySessionCounter) *SchedulingOverviewService {
	return &SchedulingOverviewService{catalog: catalog, accounts: accounts, gateway: gateway, traffic: traffic, sticky: sticky}
}

type schedulingCounts struct{ requests, failures int64 }

// Overview 组装 window（15m / 1h / 24h）的调度总览。列出上架条目里有绑定、或时间窗内有请求的
// （没有绑定却有请求 = 全是没选到渠道的失败）。请求按模型标识逐字归到条目，与准入同一规则。
func (s *SchedulingOverviewService) Overview(ctx context.Context, window string) (*SchedulingOverview, error) {
	span, ok := schedulingOverviewWindows[window]
	if !ok {
		return nil, ErrSchedulingWindowInvalid
	}
	now := time.Now()
	entries, err := s.catalog.ListEntries(ctx)
	if err != nil {
		return nil, err
	}
	rows, dataThrough, err := s.traffic.SchedulingTraffic(ctx, now.Add(-span))
	if err != nil {
		return nil, err
	}

	byEntry := map[int64]map[int64]*schedulingCounts{}
	byAccount := map[int64]*schedulingCounts{}
	resolved := map[string]int64{}
	for _, row := range rows {
		add := func(m map[int64]*schedulingCounts, id int64) {
			c := m[id]
			if c == nil {
				c = &schedulingCounts{}
				m[id] = c
			}
			c.requests += row.Success + row.Errors
			c.failures += row.Errors
		}
		if row.AccountID > 0 {
			add(byAccount, row.AccountID)
		}
		entryID, seen := resolved[row.Model]
		if !seen {
			if route, ok := s.catalog.ResolveRoute(ctx, row.Model); ok {
				entryID = route.EntryID
			}
			resolved[row.Model] = entryID
		}
		if entryID == 0 {
			continue
		}
		if byEntry[entryID] == nil {
			byEntry[entryID] = map[int64]*schedulingCounts{}
		}
		add(byEntry[entryID], row.AccountID)
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

	overview := &SchedulingOverview{GeneratedAt: now, Window: window, DataThrough: dataThrough, Warnings: []string{}}
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
		channel := SchedulingChannel{
			ID: acc.ID, Name: acc.Name, Platform: acc.Platform, Type: acc.Type, Status: acc.Status,
			Priority: acc.Priority, Concurrency: acc.Concurrency,
			BlockedReason: state.Reason, BlockedUntil: state.BlockedUntil,
		}
		if info := loads[acc.ID]; info != nil {
			inUse, waiting := info.CurrentConcurrency, info.WaitingCount
			channel.InUse, channel.Waiting = &inUse, &waiting
		}
		if c := byAccount[acc.ID]; c != nil {
			channel.Requests, channel.Failures = c.requests, c.failures
		}
		overview.Channels = append(overview.Channels, channel)
	}
	sort.Slice(overview.Channels, func(i, j int) bool { return overview.Channels[i].ID < overview.Channels[j].ID })

	overview.Models = make([]SchedulingModel, 0, len(selected))
	for _, entry := range selected {
		model := SchedulingModel{EntryID: entry.ID, ModelID: entry.ModelID, DisplayName: entry.DisplayName, Vendor: entry.Vendor, Status: entry.Status, Lanes: []SchedulingLane{}}
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
		model.Channels = make([]SchedulingModelChannel, 0, len(bound))
		for _, acc := range bound {
			channel := SchedulingModelChannel{AccountID: acc.ID, StickySessions: sticky[entry.ID][acc.ID]}
			if c := counts[acc.ID]; c != nil {
				channel.Requests, channel.Failures = c.requests, c.failures
			}
			model.Channels = append(model.Channels, channel)
		}
		for accountID, c := range counts {
			model.Requests += c.requests
			model.Failures += c.failures
			if accountID == 0 {
				model.Unrouted += c.requests
			}
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

func anyAccountServesInbound(accounts []*Account, inbound string) bool {
	for _, acc := range accounts {
		if acc.ServesInbound(inbound) {
			return true
		}
	}
	return false
}
