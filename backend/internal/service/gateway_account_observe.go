package service

import (
	"errors"
	"time"
)

// ObserveRelayKeyResult 中转 key 一次转发结束后的「渠道 × 模型」连续失败观测。中转不分协议用同一套规则
// （2026-09-29 muqian 定）：上游 5xx / 52x 记一次，同一模型连续第 2 次避开 10 秒、第 3 次起 45 秒，成功清零，
// 529 不计；调度经 candidateAdmits → ModelTransientBlocked 读取。成品号不参与，按各自厂商的规则处理。
//
// 由 handler 在 Messages 协议（GatewayService.Forward / ForwardAsChatCompletions / ForwardAsResponses）与
// Gemini 协议（GeminiMessagesCompatService.Forward / ForwardAsChatCompletions / ForwardNative）的转发之后调用；
// OpenAI 协议的中转在 OpenAIGatewayService 的上游错误处理与 ObserveOpenAIAccountResult 里按同一规则记。
// requestedModel 必须是本次选号用的模型名，否则记下的键和调度查的键对不上。
func (s *GatewayService) ObserveRelayKeyResult(account *Account, requestedModel string, err error) {
	if s == nil || s.rateLimitService == nil || !account.IsThirdPartyKey() {
		return
	}
	model := canonicalOpenAIAccountSchedulingModel(account, requestedModel)
	if err == nil {
		s.rateLimitService.ClearModelTransient(account.ID, model)
		return
	}
	var failoverErr *UpstreamFailoverError
	if !errors.As(err, &failoverErr) || !isTransientUpstreamServerStatus(failoverErr.StatusCode) {
		return
	}
	// 池模式的可重试错误由请求内的同渠道重试预算兜着，这里再记会挡掉预算内的下一次重试（同 OpenAI）。
	if account.IsPoolMode() && account.IsPoolModeRetryableStatus(failoverErr.StatusCode) {
		return
	}
	s.rateLimitService.RecordModelTransientFailure(account, model, time.Now())
}
