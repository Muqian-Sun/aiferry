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
// 在 platform 网关平台上被调度。inboundProtocol 为空表示 OpenAI 扩展端点（图片、向量等）；
// 没有入站请求的模型列表用 AccountServesPlatformForAnyInbound。
func accountServesSchedulingPlatform(account *Account, platform, inboundProtocol string, useMixed bool) bool {
	if account == nil {
		return false
	}
	if account.IsThirdPartyKey() {
		return account.KeyUpstreamProtocolFor(platform, inboundProtocol) != ""
	}
	return subscriptionServesSchedulingPlatform(account, platform, useMixed)
}

// accountServesCatalogRoute 目录路由下账号能否在 platform 网关族上承接 inboundProtocol 的请求。
// platform 是本次生效的平台：条目网关族，或 /antigravity 路由强制的 antigravity。
// 第三方 key 与分组路由同一规则（按地址选协议）；成品号按厂商 × 网关族 × 入站协议的矩阵。
func accountServesCatalogRoute(account *Account, platform, inboundProtocol string) bool {
	if account == nil {
		return false
	}
	if account.IsThirdPartyKey() {
		return account.KeyUpstreamProtocolFor(platform, inboundProtocol) != ""
	}
	return subscriptionServesCatalogRoute(account, platform, inboundProtocol)
}

// subscriptionServesCatalogRoute 成品号矩阵（inboundProtocol 为空 = OpenAI 扩展端点，按 chat_completions 看）：
//
//	厂商 \ 入站                          anthropic  chat_completions  responses  gemini
//	anthropic（族 anthropic）               ✓          ✓                ✓          ✗
//	antigravity（族 anthropic / gemini / 强制 antigravity）  ✓  ✓        ✓          ✓
//	gemini（族 gemini）                     ✓          ✓                ✗          ✓
//	openai 族（族 == 账号平台）             ✓          ✓                ✓          ✗
//
// 绑定时管理员已经把资源显式挂到条目上，antigravity 成品号不再看 mixed_scheduling 开关。
func subscriptionServesCatalogRoute(account *Account, platform, inboundProtocol string) bool {
	if inboundProtocol == "" {
		inboundProtocol = APIProtocolChatCompletions
	}
	switch account.Vendor() {
	case PlatformAntigravity:
		return platform == PlatformAnthropic || platform == PlatformGemini || platform == PlatformAntigravity
	case PlatformAnthropic:
		return platform == PlatformAnthropic && inboundProtocol != APIProtocolGemini
	case PlatformGemini:
		return platform == PlatformGemini && inboundProtocol != APIProtocolResponses
	default:
		return IsOpenAIGatewayPlatform(platform) && NormalizeOpenAICompatiblePlatform(platform) == account.Platform && inboundProtocol != APIProtocolGemini
	}
}

// AccountServesPlatformForAnyInbound 报告账号能否在 platform 网关平台上承接至少一种入站
// 协议的请求，供没有入站请求的模型列表使用（网关 /v1/models、管理端模型候选）。
//
// 列表按分组+平台缓存，客户端之后可能用该网关上任一入站协议来调：第三方 key 只要能为
// 其中一种入站协议选出已配地址的上游协议就计入。OpenAI 扩展端点（入站协议为空）只用
// chat_completions 地址，已被 chat_completions 入站覆盖。成品号按平台精确匹配，不含混合调度。
func AccountServesPlatformForAnyInbound(account *Account, platform string) bool {
	if account == nil {
		return false
	}
	if !account.IsThirdPartyKey() {
		return subscriptionServesSchedulingPlatform(account, platform, false)
	}
	for _, inboundProtocol := range UpstreamProtocols() {
		if account.KeyUpstreamProtocolFor(platform, inboundProtocol) != "" {
			return true
		}
	}
	return false
}

func subscriptionServesSchedulingPlatform(account *Account, platform string, useMixed bool) bool {
	if account.Platform == platform {
		return true
	}
	return useMixed && account.IsAntigravity() && account.IsMixedSchedulingEnabled()
}

// requestPlatformPredicate 返回本次请求的平台准入判定：目录路由下按条目矩阵，否则按分组规则。
// 入站协议取自请求 context。
func requestPlatformPredicate(ctx context.Context, platform string, useMixed bool) func(*Account) bool {
	inboundProtocol := InboundProtocolFromContext(ctx)
	if _, routed := CatalogRouteFromContext(ctx); routed {
		return func(account *Account) bool {
			return accountServesCatalogRoute(account, platform, inboundProtocol)
		}
	}
	return func(account *Account) bool {
		return accountServesSchedulingPlatform(account, platform, inboundProtocol, useMixed)
	}
}

// isAccountSchedulableOnPlatform 是选号路径的平台准入判定。
func isAccountSchedulableOnPlatform(ctx context.Context, account *Account, platform string, useMixed bool) bool {
	return requestPlatformPredicate(ctx, platform, useMixed)(account)
}

// filterAccountsSchedulableOnPlatform 按 isAccountSchedulableOnPlatform 过滤候选列表，保持原有顺序。
// 全部通过时原样返回，不复制切片。
func filterAccountsSchedulableOnPlatform(ctx context.Context, accounts []Account, platform string, useMixed bool) []Account {
	serves := requestPlatformPredicate(ctx, platform, useMixed)
	for i := range accounts {
		if serves(&accounts[i]) {
			continue
		}
		filtered := make([]Account, 0, len(accounts)-1)
		filtered = append(filtered, accounts[:i]...)
		for j := i + 1; j < len(accounts); j++ {
			if serves(&accounts[j]) {
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
