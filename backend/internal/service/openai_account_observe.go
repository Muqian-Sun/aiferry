package service

import (
	"context"
	"time"
)

// ObserveOpenAIAccountResult 一次转发结束后的账号观测：key 健康熔断（成功 / 失败），成功时清 OAuth 429
// 同账号重试窗口与账号×模型瞬时熔断。返回失败观测是否触发了健康熔断。调度统计已随 OpenAI 调度器删除。
func (s *OpenAIGatewayService) ObserveOpenAIAccountResult(account *Account, model string, success bool, observedErr ...error) bool {
	if account == nil {
		return false
	}
	healthTripped := false
	if s != nil && s.rateLimitService != nil {
		if success {
			s.rateLimitService.ObserveOpenAIAPIKeyHealthSuccess(context.Background(), account)
		} else if len(observedErr) > 0 && observedErr[0] != nil {
			healthTripped = s.rateLimitService.ObserveOpenAIAPIKeyHealthFailure(context.Background(), account, observedErr[0])
		}
	}
	if success && s != nil {
		s.openaiOAuth429RetryStartedAt.Delete(account.ID)
		s.clearOpenAIAccountModelTransientState(account.ID, normalizeOpenAIAccountModelTransientModel(model))
	}
	return healthTripped
}

// ObserveOpenAIAccountHealthFailure records failures that cannot reach the
// per-request observe path, for example after semantic response bytes were sent.
func (s *OpenAIGatewayService) ObserveOpenAIAccountHealthFailure(ctx context.Context, account *Account, observedErr error) bool {
	if s == nil || s.rateLimitService == nil || account == nil || observedErr == nil {
		return false
	}
	return s.rateLimitService.ObserveOpenAIAPIKeyHealthFailure(ctx, account, observedErr)
}

// openAIWSSessionStickyTTL WS 会话状态（连接 / turn 状态 / 无效加密内容标记）的 TTL；配置优先，否则与粘性会话 TTL 相同。
func (s *OpenAIGatewayService) openAIWSSessionStickyTTL() time.Duration {
	if s != nil && s.cfg != nil && s.cfg.Gateway.OpenAIWS.StickySessionTTLSeconds > 0 {
		return time.Duration(s.cfg.Gateway.OpenAIWS.StickySessionTTLSeconds) * time.Second
	}
	return openaiStickySessionTTL
}

func clamp01(value float64) float64 {
	switch {
	case value < 0:
		return 0
	case value > 1:
		return 1
	default:
		return value
	}
}
