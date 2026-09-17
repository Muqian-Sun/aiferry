package service

import "context"

// 本文件是调度候选「平台准入」的唯一定义处。Gateway 与 OpenAI 两套调度器、
// 调度快照装桶、候选装载与模型可用性诊断都走这里，规则不会在各路径间分叉。
//
// 成品号是厂商绑定的：按平台精确匹配；anthropic / gemini 分组的混合调度另外放行
// 启用了 mixed_scheduling 的 antigravity 成品号。
//
// 第三方 key 选的平台只是展示标签：它进入所属分组每个网关平台的调度桶，能否被选中
// 只看协议地址——按网关平台与本次入站协议能选出一个已配地址的上游协议
// （KeyUpstreamProtocolFor）才可调度。

// schedulingBucketAdmits 报告账号是否属于 platform 网关平台的调度桶。
//
// 调度桶按 group:platform:mode 缓存，与入站协议无关，所以第三方 key 一律进桶，
// 协议在选号时由 accountServesSchedulingPlatform 过滤。
func schedulingBucketAdmits(account *Account, platform string, useMixed bool) bool {
	if account == nil {
		return false
	}
	if account.IsThirdPartyKey() {
		return true
	}
	return subscriptionServesSchedulingPlatform(account, platform, useMixed)
}

// accountServesSchedulingPlatform 报告账号能否为入站协议为 inboundProtocol 的请求，
// 在 platform 网关平台上被调度。inboundProtocol 为空表示没有具体协议：OpenAI 扩展端点，
// 或没有入站请求的场景（管理端模型列表等）。
func accountServesSchedulingPlatform(account *Account, platform, inboundProtocol string, useMixed bool) bool {
	if account == nil {
		return false
	}
	if account.IsThirdPartyKey() {
		return account.KeyUpstreamProtocolFor(platform, inboundProtocol) != ""
	}
	return subscriptionServesSchedulingPlatform(account, platform, useMixed)
}

func subscriptionServesSchedulingPlatform(account *Account, platform string, useMixed bool) bool {
	if account.Platform == platform {
		return true
	}
	return useMixed && account.IsAntigravity() && account.IsMixedSchedulingEnabled()
}

// isAccountSchedulableOnPlatform 是选号路径的平台准入判定，入站协议取自请求 context。
func isAccountSchedulableOnPlatform(ctx context.Context, account *Account, platform string, useMixed bool) bool {
	return accountServesSchedulingPlatform(account, platform, InboundProtocolFromContext(ctx), useMixed)
}

// filterAccountsSchedulableOnPlatform 按 isAccountSchedulableOnPlatform 过滤候选列表，保持原有顺序。
// 全部通过时原样返回，不复制切片。
func filterAccountsSchedulableOnPlatform(ctx context.Context, accounts []Account, platform string, useMixed bool) []Account {
	inboundProtocol := InboundProtocolFromContext(ctx)
	for i := range accounts {
		if accountServesSchedulingPlatform(&accounts[i], platform, inboundProtocol, useMixed) {
			continue
		}
		filtered := make([]Account, 0, len(accounts)-1)
		filtered = append(filtered, accounts[:i]...)
		for j := i + 1; j < len(accounts); j++ {
			if accountServesSchedulingPlatform(&accounts[j], platform, inboundProtocol, useMixed) {
				filtered = append(filtered, accounts[j])
			}
		}
		return filtered
	}
	return accounts
}

// filterSchedulingBucketAccounts 按 schedulingBucketAdmits 过滤装桶结果，保持原有顺序。
func filterSchedulingBucketAccounts(accounts []Account, platform string, useMixed bool) []Account {
	filtered := make([]Account, 0, len(accounts))
	for i := range accounts {
		if schedulingBucketAdmits(&accounts[i], platform, useMixed) {
			filtered = append(filtered, accounts[i])
		}
	}
	return filtered
}

// schedulingCandidatePlatforms 返回装载某平台调度候选时成品号要匹配的平台；
// 第三方 key 不受这个列表限制（见 AccountRepository.ListSchedulingCandidates*）。
func schedulingCandidatePlatforms(platform string, useMixed bool) []string {
	if useMixed {
		return []string{platform, PlatformAntigravity}
	}
	return []string{platform}
}

// AccountKeepsHTTPPreviousResponseID 报告账号能否在 HTTP Responses 请求里承接
// previous_response_id（续链状态）。groupPlatform 是请求所在网关平台。
//
// 成品号（OAuth / SetupToken）的续链状态挂在 WSv2 会话上，HTTP 请求一律不承接。
// 第三方 key 按 Responses 协议特性规则：本次请求确实以 responses 协议转发（没被转换成
// 别的协议，否则续链状态会被静默丢弃），且厂商是官方 OpenAI 或通用中转。
func AccountKeepsHTTPPreviousResponseID(account *Account, groupPlatform string) bool {
	if account == nil || !account.IsThirdPartyKey() {
		return false
	}
	if account.KeyUpstreamProtocolFor(groupPlatform, APIProtocolResponses) != APIProtocolResponses {
		return false
	}
	return keyFollowsStandardOpenAIResponses(account)
}

// keyFollowsStandardOpenAIResponses 是 OpenAI Responses 协议特性对第三方 key 的厂商门槛：
// 官方 OpenAI，或通用中转（推定按标准协议实现）；其他已知厂商不启用。
func keyFollowsStandardOpenAIResponses(account *Account) bool {
	switch account.Vendor() {
	case PlatformOpenAI, "":
		return true
	default:
		return false
	}
}
