package service

import (
	"context"
	"log/slog"
)

// withGatewayProfitControlGate installs the gate only for explicitly marked
// token requests. This keeps media, metadata, and models-list paths outside
// the profit-control surface by construction. 门是全站一档（全局设置），同一
// 请求的 failover 重入复用 ctx 里已装的门（阈值稳定）。
func (s *GatewayService) withGatewayProfitControlGate(ctx context.Context) context.Context {
	if _, ok := gatewayTokenRequestPricingAtFromContext(ctx); !ok {
		return ctx
	}
	if existing, ok := ctx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate); ok && existing != nil {
		return ctx
	}
	if s == nil || s.settingService == nil {
		return ctx
	}
	settings := s.settingService.GetProfitControlSettings(ctx)
	if !settings.Enabled {
		return ctx
	}

	pricingAt, _ := gatewayTokenRequestPricingAtFromContext(ctx)

	// D = 用户倍率（用户价 = 目录价 × 它），与 RecordUsage 同源。
	downstream := UserRateMultiplierFromContext(ctx)
	threshold := clampProfitControlThreshold(downstream * (1 - settings.MinMargin - settings.SafetyBuffer))

	gate := &openAIProfitControlGate{
		threshold: threshold,
		pricingAt: pricingAt,
	}
	openAIProfitControlObserverInstance.recordInstall(gate.threshold)
	return context.WithValue(ctx, openAIProfitControlGateCtxKey{}, gate)
}

// GatewayProfitControlVetoLatest performs the terminal post-slot check against
// the latest scheduler snapshot. Snapshot read failures are deliberately
// fail-open to preserve availability, but are observable.
func (s *GatewayService) GatewayProfitControlVetoLatest(ctx context.Context, selected *Account) (*Account, bool, string) {
	return profitControlVetoLatest(ctx, selected, s.schedulerSnapshot)
}

func profitControlVetoLatest(ctx context.Context, selected *Account, snapshot *SchedulerSnapshotService) (*Account, bool, string) {
	gate, _ := ctx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate)
	if gate == nil || selected == nil {
		return selected, false, ""
	}
	latest := selected
	if snapshot != nil {
		refreshed, err := snapshot.GetAccount(ctx, selected.ID)
		if err != nil || refreshed == nil {
			slog.Warn("profit_control_account_refresh_failed", "account_id", selected.ID, "error", err)
			openAIProfitControlObserverInstance.recordRefreshFailure(gate.threshold)
		} else if !refreshed.UpdatedAt.Before(selected.UpdatedAt) {
			// 选号路径可能已做过 DB recheck，selected 比缓存快照更新鲜；只有
			// 快照不落后时才替换，避免终检把新鲜账号换回较旧的缓存对象。
			latest = refreshed
		}
	}
	vetoed, reason := openAIProfitControlVetoReason(ctx, latest)
	return latest, vetoed, reason
}

func (s *GatewayService) isGatewayAccountProfitEligible(ctx context.Context, account *Account) bool {
	vetoed, _ := openAIProfitControlVetoReason(ctx, account)
	return !vetoed
}

func gatewayProfitControlGateActive(ctx context.Context) bool {
	gate, _ := ctx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate)
	return gate != nil
}
