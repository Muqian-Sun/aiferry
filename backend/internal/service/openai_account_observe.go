package service

import "context"

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
