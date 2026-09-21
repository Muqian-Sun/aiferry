package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

// 进程内的调度状态：账号×模型瞬时熔断（第三方 key 连续 5xx）与代理流式隔离。
// 它们不落 DB、SchedulingState 看不到，由状态服务持有；调度器经 ModelTransientBlocked /
// ProxyStreamQuarantined 读，OpenAI 转发路径经 Record* / Clear* 写。

type runtimeSchedulingState struct {
	modelTransientOnce       sync.Once
	modelTransient           *openAIAccountModelTransientState
	proxyStreamOnce          sync.Once
	proxyStream              *openAIProxyStreamCircuit
	proxyStreamFailOpenLogAt atomic.Int64
}

func (s *RateLimitService) getModelTransientState() *openAIAccountModelTransientState {
	if s == nil {
		return nil
	}
	s.modelTransientOnce.Do(func() {
		if s.modelTransient == nil {
			s.modelTransient = newOpenAIAccountModelTransientState(openAIModelTransientDefaultMax)
		}
	})
	return s.modelTransient
}

// RecordModelTransientFailure 记一次账号×模型的瞬时失败；返回本次连击与冷却。
func (s *RateLimitService) RecordModelTransientFailure(account *Account, canonicalModel string, now time.Time) openAIAccountModelTransientDecision {
	if s == nil || account == nil {
		return openAIAccountModelTransientDecision{}
	}
	state := s.getModelTransientState()
	if state == nil {
		return openAIAccountModelTransientDecision{}
	}
	return state.recordFailure(account.ID, openAIAccountModelTransientModel(canonicalModel), now)
}

// ClearModelTransient 账号×模型成功一次：清连击。
func (s *RateLimitService) ClearModelTransient(accountID int64, model string) {
	state := s.getModelTransientState()
	if state == nil {
		return
	}
	state.recordSuccess(accountID, model)
}

// ModelTransientBlocked 报告账号对本次请求模型是否处在瞬时冷却里。
func (s *RateLimitService) ModelTransientBlocked(account *Account, requestedModel string, now time.Time) bool {
	if s == nil || account == nil {
		return false
	}
	state := s.getModelTransientState()
	if state == nil {
		return false
	}
	canonicalModel := canonicalOpenAIAccountSchedulingModel(account, requestedModel)
	return state.isBlocked(account.ID, openAIAccountModelTransientModel(canonicalModel), now)
}

func (s *RateLimitService) getProxyStreamCircuit() *openAIProxyStreamCircuit {
	if s == nil {
		return nil
	}
	s.proxyStreamOnce.Do(func() {
		if s.proxyStream == nil {
			s.proxyStream = newOpenAIProxyStreamCircuit(resolveOpenAIProxyStreamCircuitSettings(s.cfg))
		}
	})
	return s.proxyStream
}

// ActiveProxyQuarantines 当前被隔离的代理数（调度入口据此决定要不要二次放行）。
func (s *RateLimitService) ActiveProxyQuarantines(now time.Time) int {
	if s == nil {
		return 0
	}
	return s.getProxyStreamCircuit().activeBlockCount(now)
}

// RecordProxyStreamDisconnect 记一次代理上的流中断；达到阈值则隔离该代理。
func (s *RateLimitService) RecordProxyStreamDisconnect(account *Account, streamErr error, upstreamRequestID string) {
	if s == nil {
		return
	}
	proxyID, ok := openAIProxyStreamCircuitProxyID(account)
	if !ok || streamErr == nil || errors.Is(streamErr, context.Canceled) || errors.Is(streamErr, context.DeadlineExceeded) {
		return
	}
	circuit := s.getProxyStreamCircuit()
	tripped, until := circuit.recordFailure(proxyID, time.Now())
	if !tripped {
		return
	}
	logger.L().With(zap.String("component", "service.ratelimit")).Warn(
		"openai.proxy_quarantined_stream_disconnect",
		zap.Int64("proxy_id", proxyID),
		zap.Int64("account_id", account.ID),
		zap.Time("until", until),
		zap.String("upstream_request_id", upstreamRequestID),
		zap.String("error", sanitizeUpstreamErrorMessage(streamErr.Error())),
	)
}

// ClearProxyStreamDisconnect 流正常终止：清该代理的失败计数。
func (s *RateLimitService) ClearProxyStreamDisconnect(account *Account) {
	if s == nil {
		return
	}
	proxyID, ok := openAIProxyStreamCircuitProxyID(account)
	if !ok {
		return
	}
	if circuit := s.getProxyStreamCircuit(); circuit != nil {
		circuit.recordSuccess(proxyID)
	}
}

// ProxyStreamQuarantined 报告账号的代理是否被隔离；bypass ctx（二次放行）下恒 false。
func (s *RateLimitService) ProxyStreamQuarantined(ctx context.Context, account *Account) bool {
	if s == nil {
		return false
	}
	proxyID, ok := openAIProxyStreamCircuitProxyID(account)
	if !ok {
		return false
	}
	if openAIProxyStreamQuarantineBypassed(ctx) {
		return false
	}
	circuit := s.getProxyStreamCircuit()
	return circuit != nil && circuit.isBlocked(proxyID, time.Now())
}

// logOpenAIProxyStreamQuarantineFailOpen emits a rate-limited warning when a
// selection pass had to re-admit quarantined proxies to serve at all.
func (s *RateLimitService) logProxyStreamQuarantineFailOpen(requestedModel string, blockedProxies int) {
	if s == nil {
		return
	}
	now := time.Now().UnixNano()
	last := s.proxyStreamFailOpenLogAt.Load()
	if now-last < int64(openAIProxyStreamFailOpenLogInterval) ||
		!s.proxyStreamFailOpenLogAt.CompareAndSwap(last, now) {
		return
	}
	logger.L().With(zap.String("component", "service.ratelimit")).Warn(
		"openai.proxy_stream_quarantine_fail_open",
		zap.Int("blocked_proxies", blockedProxies),
		zap.String("model", requestedModel),
	)
}
