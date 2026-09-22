package service

import (
	"context"
	"strings"
)

// ModelAvailabilityDiagnosis describes whether the requested model can be
// served by any persistently eligible account in the request's pool (active
// with its schedulable setting enabled), ignoring transient state such as
// rate limits, overload, temporary unschedulability, and runtime blocks.
// Handlers use this on the "no available accounts" error path to distinguish
// 404 model_not_found from 503 service_unavailable.
type ModelAvailabilityDiagnosis struct {
	// HasAccountsInPool is true if the pool has at least one persistently
	// eligible account that can serve the request (catalog route: an account
	// bound to the entry; otherwise an account on the queried platform).
	HasAccountsInPool bool
	// HasModelSupport is true if at least one account's model mapping admits
	// the requested model.
	HasModelSupport bool
}

// ModelAvailabilityDiagnoser is implemented by gateway services that can
// report whether the requested model is configured to be served by any
// account. Both *GatewayService and *OpenAIGatewayService implement this so
// handlers in either package can share a single classifier.
type ModelAvailabilityDiagnoser interface {
	DiagnoseModelAvailabilityForPlatform(
		ctx context.Context,
		requestedModel string,
		platform string,
	) ModelAvailabilityDiagnosis
}

// modelAvailabilityCandidatePlatforms 是模型可用性诊断查询的账号平台：第三方 key 的平台只是
// 展示标签，任何标签的 key 都可能承接某个网关平台的请求，所以查全部平台，再按调度的平台准入
// 规则（isAccountSchedulableOnPlatform）过滤。只在「无可用账号」的错误路径上运行。
func modelAvailabilityCandidatePlatforms() []string {
	platforms := schedulerSnapshotPlatforms()
	return platforms[:]
}

// modelAvailabilityCandidates 诊断用的候选：目录路由 = 条目绑定的账号（持久可调度），
// 否则 = 全部平台的持久可调度账号。绕过调度快照、忽略瞬时状态。
func modelAvailabilityCandidates(ctx context.Context, repo AccountRepository) ([]Account, error) {
	if route, ok := CatalogRouteFromContext(ctx); ok {
		return repo.ListSchedulingCandidatesByCatalogEntry(ctx, route.EntryID)
	}
	return repo.ListModelAvailabilityCandidates(ctx, modelAvailabilityCandidatePlatforms())
}

// DiagnoseModelAvailabilityForPlatform inspects accounts enabled for scheduling
// by persistent configuration and returns whether the requested model is
// configured to be served by any of them. The dedicated repository query
// bypasses scheduler snapshots and deliberately ignores transient rate-limit,
// overload, temporary-unschedulable, expiry, quota, and runtime-block state.
//
// Safe to call on the error path: returns {true,true} on any internal failure
// or when the inputs preclude meaningful diagnosis (empty model, etc.), so
// callers stay on the 503 fallback branch.
func (s *GatewayService) DiagnoseModelAvailabilityForPlatform(
	ctx context.Context,
	requestedModel string,
	platform string,
) ModelAvailabilityDiagnosis {
	if s == nil {
		return ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: true}
	}
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" {
		// No model specified — cannot decide model_not_found. Caller falls back to 503.
		return ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: true}
	}
	if strings.TrimSpace(platform) == "" {
		// Without a platform we cannot scope the lookup; bail out to the
		// 503 branch rather than make an unscoped scan.
		return ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: true}
	}

	if s.accountRepo == nil {
		return ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: true}
	}

	accounts, err := modelAvailabilityCandidates(ctx, s.accountRepo)
	if err != nil {
		// Conservative fallback: pretend everything is fine so the caller
		// returns 503 (we don't want to flip to 404 just because a lookup
		// hiccup'd).
		return ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: true}
	}

	diag := ModelAvailabilityDiagnosis{}
	for i := range accounts {
		if !isAccountSchedulableOnPlatform(ctx, &accounts[i], platform, false) {
			continue
		}
		diag.HasAccountsInPool = true
		if s.isModelSupportedByAccountWithContext(ctx, &accounts[i], requestedModel) {
			diag.HasModelSupport = true
			return diag
		}
	}
	return diag
}
