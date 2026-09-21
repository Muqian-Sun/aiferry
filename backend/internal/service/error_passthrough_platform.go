package service

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
)

// ErrorPassthroughRulePlatform 返回匹配管理员错误透传规则用的平台。
//
// 规则按平台配置。成品号按账号平台匹配，保持原有行为：成品号的平台就是它所在网关的
// 平台，混合调度（anthropic / gemini 分组里的 antigravity 成品号）仍匹配 antigravity 规则。
// 第三方 key 的平台只是展示标签，不能决定规则集，按请求所在网关平台 gatewayPlatform 匹配。
func ErrorPassthroughRulePlatform(account *Account, gatewayPlatform string) string {
	if account != nil && !account.IsThirdPartyKey() {
		return account.Platform
	}
	return gatewayPlatform
}

// OpenAICompatibleRequestPlatform 返回 OpenAI 系入口上本次请求的厂商平台：目录路由按条目厂商，
// 其次合成分组解析出的目标平台，再次分组平台，未分组为 openai。grok 与国产供应商保留原值，
// 其他归一为 openai。只给厂商特有处理（grok 传输 / 生图能力 / 续链检查 / 错误文案）用，不是调度平台。
func OpenAICompatibleRequestPlatform(ctx context.Context, apiKey *APIKey) string {
	if platform, ok := RequestVendorPlatform(ctx); ok {
		return NormalizeOpenAICompatiblePlatform(platform)
	}
	if apiKey != nil && apiKey.Group != nil {
		return NormalizeOpenAICompatiblePlatform(apiKey.Group.Platform)
	}
	return PlatformOpenAI
}

// AnthropicGatewayRequestPlatform 返回 /v1/messages 系入口上本次请求的厂商平台：
// 强制平台（/antigravity 路由）优先，其次目录路由的条目厂商 / 合成分组解析出的目标平台，
// 再次分组平台；未分组按 anthropic。只给错误透传规则平台与错误文案用，不是调度平台。
func AnthropicGatewayRequestPlatform(ctx context.Context, apiKey *APIKey) string {
	if ctx != nil {
		if forcePlatform, ok := ctx.Value(ctxkey.ForcePlatform).(string); ok && strings.TrimSpace(forcePlatform) != "" {
			return strings.TrimSpace(forcePlatform)
		}
		if platform, ok := RequestVendorPlatform(ctx); ok {
			return platform
		}
	}
	if apiKey != nil && apiKey.Group != nil && apiKey.Group.Platform != "" {
		return apiKey.Group.Platform
	}
	return PlatformAnthropic
}

// openAIGatewayErrorPassthroughPlatform 是 OpenAI 网关转发路径上匹配错误透传规则的平台。
func openAIGatewayErrorPassthroughPlatform(c *gin.Context, account *Account) string {
	ctx, apiKey := errorPassthroughRequestScope(c)
	return ErrorPassthroughRulePlatform(account, OpenAICompatibleRequestPlatform(ctx, apiKey))
}

// anthropicGatewayErrorPassthroughPlatform 是 Anthropic 网关转发路径（GatewayService）上
// 匹配错误透传规则的平台。
func anthropicGatewayErrorPassthroughPlatform(c *gin.Context, account *Account) string {
	ctx, apiKey := errorPassthroughRequestScope(c)
	return ErrorPassthroughRulePlatform(account, AnthropicGatewayRequestPlatform(ctx, apiKey))
}

func errorPassthroughRequestScope(c *gin.Context) (context.Context, *APIKey) {
	if c == nil {
		return nil, nil
	}
	var ctx context.Context
	if c.Request != nil {
		ctx = c.Request.Context()
	}
	return ctx, getAPIKeyFromContext(c)
}
