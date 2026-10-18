package service

// 请求级调度轨迹：请求进入时在 context 里放一个记录器（WithSchedulingTrace），调度器每选一次渠道
// 往里写一次「挑选」（候选各自的判定、粘性会话结果、选中谁、走哪一层）；请求结束时由网关中间件
// 合上每次上游尝试与最终结果，交给 SchedulingTraceWriter 异步落库。不记请求内容。

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// 一个候选在一次挑选里的判定。
const (
	SchedulingTraceExcluded    = "excluded"     // 本次请求里已经失败过，被排除
	SchedulingTraceSkipped     = "skipped"      // 准入不通过，Reason 写原因
	SchedulingTraceEligible    = "eligible"     // 准入通过，但先挑中了别的
	SchedulingTraceFull        = "full"         // 准入通过但并发已满，只能排队
	SchedulingTraceSlotBusy    = "slot_busy"    // 轮到它时没抢到并发槽
	SchedulingTraceSessionFull = "session_full" // 会话数已满，不接新会话
	SchedulingTracePicked      = "picked"       // 选中，直接转发
	SchedulingTraceQueued      = "queued"       // 选中，但要排队等并发槽
)

// 粘性会话层的结果。
const (
	SchedulingTraceStickyHit  = "hit"  // 回到绑定的渠道
	SchedulingTraceStickyWait = "wait" // 回到绑定的渠道，排队等槽
	SchedulingTraceStickyMiss = "miss" // 绑定的渠道这次不能用，Reason 写原因
)

// 一次挑选最终走的层。
const (
	SchedulingTraceLayerSticky = "sticky"
	SchedulingTraceLayerLoad   = "load"
	SchedulingTraceLayerLegacy = "legacy"
	SchedulingTraceLayerQueue  = "queue"
)

// 一次请求的结果。
const (
	SchedulingTraceOutcomeSuccess   = "success"
	SchedulingTraceOutcomeFailed    = "failed"
	SchedulingTraceOutcomeNoChannel = "no_channel"
	SchedulingTraceOutcomeCanceled  = "canceled"
)

// SchedulingTraceSticky 粘性会话层在这次挑选里的结果。
type SchedulingTraceSticky struct {
	AccountID int64  `json:"account_id"`
	Outcome   string `json:"outcome"`
	Reason    string `json:"reason,omitempty"`
}

// SchedulingTraceCandidate 一个候选渠道在这次挑选里的判定。
type SchedulingTraceCandidate struct {
	AccountID int64  `json:"account_id"`
	Decision  string `json:"decision"`
	Reason    string `json:"reason,omitempty"`
	LoadRate  *int   `json:"load_rate,omitempty"`
}

// SchedulingTracePick 调度器的一次选渠道。AtMs 是距请求开始的毫秒数。
type SchedulingTracePick struct {
	AtMs       int64                      `json:"at_ms"`
	Layer      string                     `json:"layer,omitempty"`
	AccountID  int64                      `json:"account_id,omitempty"`
	Error      string                     `json:"error,omitempty"`
	Sticky     *SchedulingTraceSticky     `json:"sticky,omitempty"`
	Candidates []SchedulingTraceCandidate `json:"candidates"`
}

// SchedulingTraceAttempt 一次失败的上游尝试（取自运维的上游错误记录）。
type SchedulingTraceAttempt struct {
	AtMs       int64  `json:"at_ms"`
	AccountID  int64  `json:"account_id"`
	StatusCode int    `json:"status_code,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Message    string `json:"message,omitempty"`
}

// SchedulingTrace 一个请求的调度轨迹（scheduling_traces 一行）。
type SchedulingTrace struct {
	ID              int64                    `json:"id"`
	CreatedAt       time.Time                `json:"created_at"`
	RequestID       string                   `json:"request_id"`
	UserID          *int64                   `json:"user_id,omitempty"`
	APIKeyID        *int64                   `json:"api_key_id,omitempty"`
	Model           string                   `json:"model"`
	CatalogEntryID  *int64                   `json:"catalog_entry_id,omitempty"`
	InboundProtocol string                   `json:"inbound_protocol"`
	Outcome         string                   `json:"outcome"`
	StatusCode      int                      `json:"status_code"`
	FinalAccountID  *int64                   `json:"final_account_id,omitempty"`
	SwitchCount     int                      `json:"switch_count"`
	StickyHit       bool                     `json:"sticky_hit"`
	DurationMs      int64                    `json:"duration_ms"`
	Picks           []SchedulingTracePick    `json:"picks"`
	Attempts        []SchedulingTraceAttempt `json:"attempts"`
}

// SchedulingTraceRecorder 收集一个请求里调度器的每次挑选。
type SchedulingTraceRecorder struct {
	start time.Time
	mu    sync.Mutex
	picks []SchedulingTracePick
}

type schedulingTraceContextKey struct{}

// WithSchedulingTrace 给请求挂上调度轨迹记录器。
func WithSchedulingTrace(ctx context.Context, start time.Time) (context.Context, *SchedulingTraceRecorder) {
	recorder := &SchedulingTraceRecorder{start: start}
	return context.WithValue(ctx, schedulingTraceContextKey{}, recorder), recorder
}

// Start 请求开始的时刻（AtMs 的零点）。
func (r *SchedulingTraceRecorder) Start() time.Time { return r.start }

// Picks 已记下的挑选（副本）。
func (r *SchedulingTraceRecorder) Picks() []SchedulingTracePick {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]SchedulingTracePick(nil), r.picks...)
}

func (r *SchedulingTraceRecorder) add(pick SchedulingTracePick) {
	r.mu.Lock()
	r.picks = append(r.picks, pick)
	r.mu.Unlock()
}

// schedulingTracePick 调度器内部写一次挑选用；没有记录器时为 nil，所有方法都是空操作。
type schedulingTracePick struct {
	recorder *SchedulingTraceRecorder
	pick     SchedulingTracePick
	index    map[int64]int
	legacy   bool
}

func beginSchedulingTracePick(ctx context.Context) *schedulingTracePick {
	recorder, _ := ctx.Value(schedulingTraceContextKey{}).(*SchedulingTraceRecorder)
	if recorder == nil {
		return nil
	}
	return &schedulingTracePick{
		recorder: recorder,
		pick:     SchedulingTracePick{AtMs: time.Since(recorder.start).Milliseconds(), Candidates: []SchedulingTraceCandidate{}},
		index:    map[int64]int{},
	}
}

func (p *schedulingTracePick) sticky(accountID int64, outcome, reason string) {
	if p == nil {
		return
	}
	p.pick.Sticky = &SchedulingTraceSticky{AccountID: accountID, Outcome: outcome, Reason: reason}
}

func (p *schedulingTracePick) candidate(accountID int64, decision, reason string) {
	if p == nil {
		return
	}
	if i, ok := p.index[accountID]; ok {
		p.pick.Candidates[i].Decision, p.pick.Candidates[i].Reason = decision, reason
		return
	}
	p.index[accountID] = len(p.pick.Candidates)
	p.pick.Candidates = append(p.pick.Candidates, SchedulingTraceCandidate{AccountID: accountID, Decision: decision, Reason: reason})
}

func (p *schedulingTracePick) loadRate(accountID int64, rate int) {
	if p == nil {
		return
	}
	if i, ok := p.index[accountID]; ok {
		p.pick.Candidates[i].LoadRate = &rate
	}
}

func (p *schedulingTracePick) markLegacy() {
	if p != nil {
		p.legacy = true
	}
}

// finish 根据选号结果定层并落到记录器。
func (p *schedulingTracePick) finish(result *AccountSelectionResult, err error) {
	if p == nil {
		return
	}
	switch {
	case err != nil || result == nil || result.Account == nil:
		p.pick.Error = schedulingTraceErrorCode(err)
	default:
		p.pick.AccountID = result.Account.ID
		stickyUsed := p.pick.Sticky != nil && p.pick.Sticky.AccountID == result.Account.ID &&
			(p.pick.Sticky.Outcome == SchedulingTraceStickyHit || p.pick.Sticky.Outcome == SchedulingTraceStickyWait)
		switch {
		case stickyUsed:
			p.pick.Layer = SchedulingTraceLayerSticky
		case result.WaitPlan != nil:
			p.pick.Layer = SchedulingTraceLayerQueue
			p.candidate(result.Account.ID, SchedulingTraceQueued, "")
		case p.legacy:
			p.pick.Layer = SchedulingTraceLayerLegacy
			p.candidate(result.Account.ID, SchedulingTracePicked, "")
		default:
			p.pick.Layer = SchedulingTraceLayerLoad
			p.candidate(result.Account.ID, SchedulingTracePicked, "")
		}
	}
	p.recorder.add(p.pick)
}

// schedulingTraceMessageLimit 尝试里错误信息保留的字节数（只为看个大概，完整的在运维错误日志里）。
const schedulingTraceMessageLimit = 300

// SchedulingTraceInput 请求结束时由网关中间件交来的信息。
type SchedulingTraceInput struct {
	RequestID      string
	UserID         *int64
	APIKeyID       *int64
	StatusCode     int
	Canceled       bool
	StreamFailed   bool
	End            time.Time
	UpstreamErrors []*OpsUpstreamErrorEvent
}

// Build 合成这个请求的轨迹；调度器一次都没选过渠道（没走到调度）时返回 false。
// 模型、目录条目、入站协议取自 ctx（准入与端点中间件挂上的）。
func (r *SchedulingTraceRecorder) Build(ctx context.Context, in SchedulingTraceInput) (SchedulingTrace, bool) {
	picks := r.Picks()
	if len(picks) == 0 {
		return SchedulingTrace{}, false
	}
	trace := SchedulingTrace{
		CreatedAt:       r.start,
		RequestID:       in.RequestID,
		UserID:          in.UserID,
		APIKeyID:        in.APIKeyID,
		InboundProtocol: InboundProtocolFromContext(ctx),
		StatusCode:      in.StatusCode,
		DurationMs:      in.End.Sub(r.start).Milliseconds(),
		Picks:           picks,
		Attempts:        []SchedulingTraceAttempt{},
		StickyHit:       picks[0].Layer == SchedulingTraceLayerSticky,
	}
	if route, ok := CatalogRouteFromContext(ctx); ok {
		entryID := route.EntryID
		trace.CatalogEntryID = &entryID
		trace.Model = route.RequestedModel
	}
	picked := 0
	for _, pick := range picks {
		if pick.AccountID > 0 {
			picked++
			accountID := pick.AccountID
			trace.FinalAccountID = &accountID
		}
	}
	trace.SwitchCount = max(picked-1, 0)
	for _, event := range in.UpstreamErrors {
		if event == nil {
			continue
		}
		trace.Attempts = append(trace.Attempts, SchedulingTraceAttempt{
			AtMs:       event.AtUnixMs - r.start.UnixMilli(),
			AccountID:  event.AccountID,
			StatusCode: event.UpstreamStatusCode,
			Kind:       event.Kind,
			Message:    truncateString(event.Message, schedulingTraceMessageLimit),
		})
	}
	switch {
	case in.Canceled || in.StatusCode == 499:
		trace.Outcome = SchedulingTraceOutcomeCanceled
	case picked == 0:
		trace.Outcome = SchedulingTraceOutcomeNoChannel
	case in.StatusCode >= 400 || in.StreamFailed:
		trace.Outcome = SchedulingTraceOutcomeFailed
	default:
		trace.Outcome = SchedulingTraceOutcomeSuccess
	}
	return trace, true
}

// OpsUpstreamErrorEventsFromContext 本次请求已记下的上游错误（每次失败的尝试一条，按发生顺序）。
func OpsUpstreamErrorEventsFromContext(c *gin.Context) []*OpsUpstreamErrorEvent {
	if c == nil {
		return nil
	}
	value, ok := c.Get(OpsUpstreamErrorsKey)
	if !ok {
		return nil
	}
	events, _ := value.([]*OpsUpstreamErrorEvent)
	return events
}

func schedulingTraceErrorCode(err error) string {
	switch {
	case err == nil:
		return "no_account"
	case errors.Is(err, ErrNoAvailableCompactAccounts):
		return "no_compact_accounts"
	case errors.Is(err, ErrNoAvailableAccounts):
		return "no_available_accounts"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "canceled"
	default:
		return "error"
	}
}
