package service

// 调度预览：用调度器同一套规则（layer2RejectReason / pickLayer2Candidate），算出一个目录条目在某个
// 入站协议下，绑定的每个渠道此刻排第几、或为什么不会被选。不抢槽、不绑粘性。
// 粘性会话层不在预览里：已绑定的会话会先回到绑定渠道。

import (
	"context"
	"sort"
	"strings"
	"time"
)

// 预览里渠道的三种状态。
const (
	// SchedulingPreviewReady Layer 2 会按 Rank 的顺序挑它。
	SchedulingPreviewReady = "ready"
	// SchedulingPreviewQueue 准入通过但并发已满，只会在 Layer 3 排队。
	SchedulingPreviewQueue = "queue"
	// SchedulingPreviewBlocked 准入不通过，Reason 写原因。
	SchedulingPreviewBlocked = "blocked"
)

// schedulingPreviewModelRateLimited 是 candidateAdmits 的 not_schedulable 里「整体可用、按模型限流」那一种。
const schedulingPreviewModelRateLimited = "model_rate_limited"

// schedulingPreviewError 是整体状态 disabled 里「出错停用」那一种（渠道页显示为异常）。
const schedulingPreviewError = "error"

// displaySchedulingState 整体阻塞原因的展示版：出错停用单独标出并带上错误信息，其余照 SchedulingState。
func displaySchedulingState(acc *Account, state SchedulingState) (reason, detail string) {
	if state.Reason == "disabled" && acc.Status == StatusError {
		return schedulingPreviewError, strings.TrimSpace(acc.ErrorMessage)
	}
	return state.Reason, state.Detail
}

// SchedulingPreviewItem 一个渠道在某条调度线上的位置。
type SchedulingPreviewItem struct {
	AccountID int64  `json:"account_id"`
	State     string `json:"state"`
	// Rank 从 1 起，只在 ready 时有值：1 就是下一个没有粘性绑定的请求会落的渠道。
	Rank int `json:"rank,omitempty"`
	// Direct 能以入站协议直连（不用转换），调度时排在要转换的前面。
	Direct bool       `json:"direct"`
	Reason string     `json:"reason,omitempty"`
	Until  *time.Time `json:"until,omitempty"`
	Detail string     `json:"detail,omitempty"`
}

// SchedulingLoads 取负载图，口径与选号一致：负载图没开或没有并发服务时返回 errLoadBatchDisabled，
// 此时调度器按优先级 + LRU 回退排序，预览也一样。
func (s *GatewayService) SchedulingLoads(ctx context.Context, accounts []*Account) (map[int64]*AccountLoadInfo, error) {
	if s.concurrencyService == nil || !s.schedulingConfig().LoadBatchEnabled {
		return nil, errLoadBatchDisabled
	}
	loads := make([]AccountWithConcurrency, 0, len(accounts))
	for _, acc := range accounts {
		loads = append(loads, AccountWithConcurrency{ID: acc.ID, MaxConcurrency: acc.EffectiveLoadFactor()})
	}
	return s.concurrencyService.GetAccountsLoadBatch(ctx, loads)
}

// PreviewCatalogScheduling 预览 entry 在入站协议 inbound 下的调度顺序。accounts 是条目绑定的渠道；
// loads 为 nil 表示负载图不可用（按优先级 + LRU 回退排序）。返回顺序：ready 按 Rank，再 queue，再 blocked。
func (s *GatewayService) PreviewCatalogScheduling(ctx context.Context, entry *ModelCatalogEntry, accounts []*Account, inbound string, loads map[int64]*AccountLoadInfo) []SchedulingPreviewItem {
	now := time.Now()
	ctx = WithCatalogRoute(ctx, CatalogRoute{EntryID: entry.ID, CanonicalModel: entry.ModelID, RequestedModel: entry.ModelID, Entry: entry})
	ctx = WithInboundProtocol(ctx, inbound)
	ctx = s.withGatewayProfitControlGate(ctx)
	values := make([]Account, 0, len(accounts))
	for _, acc := range accounts {
		values = append(values, *acc)
	}
	ctx = s.withRPMPrefetch(ctx, values)
	platform, _ := resolvePlatform(ctx)

	items := make(map[int64]*SchedulingPreviewItem, len(accounts))
	admitted := make([]*Account, 0, len(accounts))
	for _, acc := range accounts {
		item := &SchedulingPreviewItem{AccountID: acc.ID, Direct: acc.ProtocolMatches(inbound)}
		items[acc.ID] = item
		if reason := s.layer2RejectReason(ctx, acc, platform, entry.ModelID); reason != "" {
			item.State = SchedulingPreviewBlocked
			item.Reason, item.Until, item.Detail = explainSchedulingReject(ctx, acc, entry.ModelID, reason, now)
			continue
		}
		admitted = append(admitted, acc)
	}

	rank := 0
	if loads != nil {
		available := make([]accountWithLoad, 0, len(admitted))
		for _, acc := range admitted {
			info := loads[acc.ID]
			if info == nil {
				info = &AccountLoadInfo{AccountID: acc.ID}
			}
			if info.LoadRate < 100 {
				available = append(available, accountWithLoad{account: acc, loadInfo: info})
			} else {
				items[acc.ID].State = SchedulingPreviewQueue
			}
		}
		cfg := s.schedulingConfig()
		for len(available) > 0 {
			selected := pickLayer2Candidate(ctx, available, inbound, cfg)
			if selected == nil {
				break
			}
			rank++
			items[selected.account.ID].State, items[selected.account.ID].Rank = SchedulingPreviewReady, rank
			selectedID := selected.account.ID
			next := make([]accountWithLoad, 0, len(available)-1)
			for _, acc := range available {
				if acc.account.ID != selectedID {
					next = append(next, acc)
				}
			}
			available = next
		}
	} else {
		ordered := append([]*Account(nil), admitted...)
		sortAccountsByPriorityAndLastUsed(ordered, inbound)
		for _, acc := range ordered {
			rank++
			items[acc.ID].State, items[acc.ID].Rank = SchedulingPreviewReady, rank
		}
	}

	out := make([]SchedulingPreviewItem, 0, len(items))
	for _, acc := range accounts {
		out = append(out, *items[acc.ID])
	}
	stateOrder := map[string]int{SchedulingPreviewReady: 0, SchedulingPreviewQueue: 1, SchedulingPreviewBlocked: 2}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].State != out[j].State {
			return stateOrder[out[i].State] < stateOrder[out[j].State]
		}
		return out[i].Rank < out[j].Rank
	})
	return out
}

// explainSchedulingReject 把准入原因细化成预览能展示的样子：not_schedulable 拆成整体状态（带解除时间）
// 或按模型限流（带最晚的解除时间与命中的限流 key），其余原样。
func explainSchedulingReject(ctx context.Context, acc *Account, model, reason string, now time.Time) (string, *time.Time, string) {
	if reason != "not_schedulable" {
		return reason, nil, ""
	}
	state := acc.SchedulingState(now)
	if state.Blocked {
		reason, detail := displaySchedulingState(acc, state)
		return reason, state.BlockedUntil, detail
	}
	var until *time.Time
	var scopes []string
	for _, scope := range acc.modelRateLimitKeysForRequest(ctx, model) {
		end, ok := state.ModelBlocks[scope]
		if !ok || !now.Before(end) {
			continue
		}
		scopes = append(scopes, scope)
		if until == nil || end.After(*until) {
			until = &end
		}
	}
	return schedulingPreviewModelRateLimited, until, strings.Join(scopes, ", ")
}
