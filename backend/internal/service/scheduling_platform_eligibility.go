package service

import "context"

// 本文件是调度候选「准入」的唯一定义处。Gateway 与 OpenAI 两套调度器、
// 调度快照装桶、候选装载与模型可用性诊断都走这里，规则不会在各路径间分叉。
//
// 目录路由（有 CatalogRoute）：资格 = 协议转换注册表（accountServesCatalogRoute），不分成品号 / key。
//
// 平台池（无模型端点）：账号平台必须相等——key 与成品号同一条规则，与选号的 SelectOptions.Platform
// 门一致；第三方 key 另外要求按平台与本次入站协议能选出一个已配地址的上游协议（KeyUpstreamProtocolFor）。

// schedulingBucketAdmits 报告账号是否属于 platform 平台池的调度桶。
//
// 调度桶按 pool:platform:mode 缓存，与入站协议无关：协议在选号时由
// accountServesSchedulingPlatform 过滤。
func schedulingBucketAdmits(account *Account, platform string) bool {
	return account != nil && account.Platform == platform
}

// accountServesSchedulingPlatform 报告账号能否为入站协议为 inboundProtocol 的请求，
// 在 platform 平台池上被调度。inboundProtocol 为空表示 OpenAI 扩展端点（图片、向量等）；
// 没有入站请求的模型列表用 AccountServesPlatformForAnyInbound。
func accountServesSchedulingPlatform(account *Account, platform, inboundProtocol string) bool {
	if account == nil || account.Platform != platform {
		return false
	}
	if account.IsThirdPartyKey() {
		return account.KeyUpstreamProtocolFor(platform, inboundProtocol) != ""
	}
	return true
}

// accountServesCatalogRoute 目录路由下资源能否承接本次入站协议：拥有的上游协议里有一个存在从
// inboundProtocol 出发的转换实现（protocol_conversion.go 的注册表），key 与成品号同一条规则。
// platform 只剩 /antigravity 强制路由这一个用途：强制 antigravity 时只放行 antigravity 成品号
// （该入口的语义就是「走 antigravity」）。绑定校验走同一张注册表（CatalogBindingServes）。
func accountServesCatalogRoute(account *Account, platform, inboundProtocol string) bool {
	if account == nil {
		return false
	}
	if platform == PlatformAntigravity && account.Vendor() != PlatformAntigravity {
		return false
	}
	return account.ServesInbound(inboundProtocol)
}

// AccountServesPlatformForAnyInbound 报告账号能否为 platform 形态的请求承接至少一种入站协议，
// 供没有入站请求的模型列表使用（管理端模型候选、未映射模型补齐）。
//
// 这是「能不能承接」而不是「在不在哪个池」：第三方 key 的标签不参与，只要能为其中一种入站协议
// 选出已配地址的上游协议就计入（平台池的成员判定见 schedulingBucketAdmits）。
// OpenAI 扩展端点（入站协议为空）只用 chat_completions 地址，已被 chat_completions 入站覆盖。
func AccountServesPlatformForAnyInbound(account *Account, platform string) bool {
	if account == nil {
		return false
	}
	if !account.IsThirdPartyKey() {
		return account.Platform == platform
	}
	for _, inboundProtocol := range UpstreamProtocols() {
		if account.KeyUpstreamProtocolFor(platform, inboundProtocol) != "" {
			return true
		}
	}
	return false
}

// requestPlatformPredicate 返回本次请求的平台准入判定：目录路由下按协议转换注册表，否则按平台池规则。
// 入站协议取自请求 context。
func requestPlatformPredicate(ctx context.Context, platform string) func(*Account) bool {
	inboundProtocol := InboundProtocolFromContext(ctx)
	if _, routed := CatalogRouteFromContext(ctx); routed {
		return func(account *Account) bool {
			return accountServesCatalogRoute(account, platform, inboundProtocol)
		}
	}
	return func(account *Account) bool {
		return accountServesSchedulingPlatform(account, platform, inboundProtocol)
	}
}

// isAccountSchedulableOnPlatform 是选号路径的平台准入判定。
func isAccountSchedulableOnPlatform(ctx context.Context, account *Account, platform string) bool {
	return requestPlatformPredicate(ctx, platform)(account)
}

// filterAccountsSchedulableOnPlatform 按 isAccountSchedulableOnPlatform 过滤候选列表，保持原有顺序。
// 全部通过时原样返回，不复制切片。
func filterAccountsSchedulableOnPlatform(ctx context.Context, accounts []Account, platform string) []Account {
	serves := requestPlatformPredicate(ctx, platform)
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
func filterSchedulingBucketAccounts(accounts []Account, platform string) []Account {
	filtered := make([]Account, 0, len(accounts))
	for i := range accounts {
		if schedulingBucketAdmits(&accounts[i], platform) {
			filtered = append(filtered, accounts[i])
		}
	}
	return filtered
}
