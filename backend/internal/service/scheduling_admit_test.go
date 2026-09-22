//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 影子账号只在母账号（OpenAI OAuth）凭据可用时放行；母账号解析走快照 / 仓储。
func TestCandidateAdmits_ShadowParentUnhealthy(t *testing.T) {
	parentID := int64(82011)
	parent := openAIOAuthSub(parentID, 1)
	shadow := openAIOAuthSub(82012, 1)
	shadow.ParentAccountID = &parentID
	svc := newProtocolMatchService(t, true, nil, parent, shadow)
	ctx := selectOptionsCtx(APIProtocolResponses)

	ok, _ := svc.candidateAdmits(ctx, nil, &shadow, "gpt-5.6")
	require.True(t, ok, "母账号健康")

	until := time.Now().Add(time.Hour)
	parent.TempUnschedulableUntil = &until
	svc = newProtocolMatchService(t, true, nil, parent, shadow)
	ok, reason := svc.candidateAdmits(ctx, nil, &shadow, "gpt-5.6")
	require.False(t, ok)
	require.Equal(t, "shadow_parent_unhealthy", reason)

	svc = newProtocolMatchService(t, true, nil, shadow)
	ok, reason = svc.candidateAdmits(ctx, nil, &shadow, "gpt-5.6")
	require.False(t, ok, "母账号不存在")
	require.Equal(t, "shadow_parent_unhealthy", reason)
}

// 账号×模型瞬时熔断由状态服务持有：打到冷却就拒，成功一次清掉。
func TestCandidateAdmits_ModelTransientBlocked(t *testing.T) {
	key := openAIKey(82021, 1, nil)
	svc := newProtocolMatchService(t, true, nil, key)
	svc.rateLimitService = &RateLimitService{cfg: svc.cfg}
	ctx := selectOptionsCtx(APIProtocolResponses)
	now := time.Now()

	ok, _ := svc.candidateAdmits(ctx, nil, &key, "gpt-5.6")
	require.True(t, ok)

	svc.rateLimitService.RecordModelTransientFailure(&key, "gpt-5.6", now)
	decision := svc.rateLimitService.RecordModelTransientFailure(&key, "gpt-5.6", now.Add(time.Millisecond))
	require.Positive(t, decision.Cooldown, "第二次失败进入冷却")
	ok, reason := svc.candidateAdmits(ctx, nil, &key, "gpt-5.6")
	require.False(t, ok)
	require.Equal(t, "model_transient_blocked", reason)
	ok, _ = svc.candidateAdmits(ctx, nil, &key, "gpt-5.7")
	require.True(t, ok, "冷却按账号×模型，别的模型不受影响")

	svc.rateLimitService.ClearModelTransient(key.ID, "gpt-5.6")
	ok, _ = svc.candidateAdmits(ctx, nil, &key, "gpt-5.6")
	require.True(t, ok)

	_, err := svc.SelectAccountWithOptions(ctx, nil, "", "gpt-5.6", nil, SelectOptions{})
	require.NoError(t, err)
}

// 代理流式隔离由状态服务持有：带该代理的账号拒；bypass ctx 下放行。
func TestCandidateAdmits_ProxyQuarantined(t *testing.T) {
	proxyID := int64(8203)
	key := openAIKey(82031, 1, nil)
	key.ProxyID = &proxyID
	svc := newProtocolMatchService(t, true, nil, key)
	svc.rateLimitService = &RateLimitService{cfg: svc.cfg}
	svc.rateLimitService.proxyStream = newOpenAIProxyStreamCircuit(openAIProxyStreamCircuitSettings{
		failureThreshold: 1, failureWindow: time.Minute, quarantineTTL: 10 * time.Minute, maxEntries: 16,
	})
	ctx := selectOptionsCtx(APIProtocolResponses)

	ok, _ := svc.candidateAdmits(ctx, nil, &key, "gpt-5.6")
	require.True(t, ok)

	svc.rateLimitService.RecordProxyStreamDisconnect(&key, context.DeadlineExceeded, "rid")
	ok, _ = svc.candidateAdmits(ctx, nil, &key, "gpt-5.6")
	require.True(t, ok, "超时 / 取消不算流中断")

	svc.rateLimitService.RecordProxyStreamDisconnect(&key, errors.New("stream ended before terminal event"), "rid")
	ok, reason := svc.candidateAdmits(ctx, nil, &key, "gpt-5.6")
	require.False(t, ok)
	require.Equal(t, "proxy_quarantined", reason)
	ok, _ = svc.candidateAdmits(withOpenAIProxyStreamQuarantineBypass(ctx), nil, &key, "gpt-5.6")
	require.True(t, ok, "二次放行的 ctx 下不看隔离")

	svc.rateLimitService.ClearProxyStreamDisconnect(&key)
	ok, _ = svc.candidateAdmits(ctx, nil, &key, "gpt-5.6")
	require.True(t, ok)
}

// grok 的两个进程内模型级状态（账号×模型免费额度耗尽、team×模型限流冷却）由唯一调度器的候选门读。
func TestCandidateAdmits_GrokModelRuntimeBlocked(t *testing.T) {
	quota := Account{ID: 82041, Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1,
		Credentials: map[string]any{"access_token": "tok", "model_mapping": map[string]any{"grok-4.5": "grok-4.5", "grok-4.3": "grok-4.3"}}}
	team := Account{ID: 82042, Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 2,
		Credentials: map[string]any{"access_token": "tok", "team_id": "team-82042", "model_mapping": map[string]any{"grok-4.5": "grok-4.5", "grok-4.3": "grok-4.3"}}}
	svc := newProtocolMatchService(t, true, nil, quota, team)
	ctx := selectOptionsCtx(APIProtocolResponses)
	now := time.Now()

	ok, _ := svc.candidateAdmits(ctx, nil, &quota, "grok-4.5")
	require.True(t, ok)
	ok, _ = svc.candidateAdmits(ctx, nil, &team, "grok-4.5")
	require.True(t, ok)

	markGrokModelQuotaBlock(quota.ID, "grok-4.5", now.Add(time.Hour))
	ok, reason := svc.candidateAdmits(ctx, nil, &quota, "grok-4.5")
	require.False(t, ok)
	require.Equal(t, "grok_model_blocked", reason)
	ok, _ = svc.candidateAdmits(ctx, nil, &quota, "grok-4.3")
	require.True(t, ok, "免费额度耗尽按账号×模型，别的模型不受影响")

	markGrokTeamModelRateLimit(&team, "grok-4.5", now.Add(time.Hour))
	ok, reason = svc.candidateAdmits(ctx, nil, &team, "grok-4.5")
	require.False(t, ok)
	require.Equal(t, "grok_model_blocked", reason)
}
