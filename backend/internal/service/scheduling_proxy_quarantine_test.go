//go:build unit

package service

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 全池只剩被隔离代理后面的账号时，第二遍无视隔离再选一次；没有隔离中的代理就不重试。
func TestSelectAccountWithOptions_ProxyQuarantineFailOpen(t *testing.T) {
	proxyID := int64(8301)
	key := openAIKey(83001, 1, nil)
	key.ProxyID = &proxyID
	svc := newProtocolMatchService(t, true, nil, key)
	svc.rateLimitService = &RateLimitService{cfg: svc.cfg}
	svc.rateLimitService.proxyStream = newOpenAIProxyStreamCircuit(openAIProxyStreamCircuitSettings{
		failureThreshold: 1, failureWindow: time.Minute, quarantineTTL: 10 * time.Minute, maxEntries: 16,
	})
	ctx := selectOptionsCtx(APIProtocolResponses)
	svc.rateLimitService.RecordProxyStreamDisconnect(&key, errors.New("stream ended before terminal event"), "rid")
	require.Equal(t, 1, svc.rateLimitService.ActiveProxyQuarantines(time.Now()))

	result, err := svc.SelectAccountWithOptions(ctx, "", "gpt-5.6", nil, SelectOptions{})
	require.NoError(t, err, "隔离降级成偏好：宁可用坏代理也不回无可用账号")
	require.Equal(t, key.ID, result.Account.ID)
	require.Equal(t, 1, svc.rateLimitService.ActiveProxyQuarantines(time.Now()), "二次放行不清隔离")

	// 没有隔离中的代理：无可用账号直接返回，不重试（仓储只被列举一次）
	other := openAIKey(83002, 1, map[string]any{"openai_compact_supported": false})
	svc = newProtocolMatchService(t, true, nil, other)
	svc.rateLimitService = &RateLimitService{cfg: svc.cfg}
	repo := svc.accountRepo.(*mockAccountRepoForPlatform)
	before := repo.listCatalogCalls
	_, err = svc.SelectAccountWithOptions(ctx, "", "gpt-5.6", nil, SelectOptions{RequireCompact: true})
	require.ErrorIs(t, err, ErrNoAvailableCompactAccounts)
	require.Equal(t, before+1, repo.listCatalogCalls, "没有隔离中的代理时只选一遍")
}
