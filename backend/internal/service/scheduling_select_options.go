package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// SelectOptions 是本次请求对资源的额外要求；零值 = 没有要求（/v1/messages、Gemini 入站）。
// 它们都是账号属性判定（能不能承接这种请求），不是额度、不是状态。
type SelectOptions struct {
	// Capability 端点能力：chat_completions / responses / live / grok_media…（SupportsOpenAIEndpointCapability）
	Capability OpenAIEndpointCapability
	// ImageCapability /v1/images 的能力档：native / basic（SupportsOpenAIImageCapability）
	ImageCapability OpenAIImagesCapability
	// RequireCompact /responses/compact 只能派给 compact 档 > 0 的资源（openAICompactSupportTier）
	RequireCompact bool
	// Transport 需要的上游传输（WS v2 / HTTP SSE / any），按 cfg 与账号解析（openAIAccountTransportCompatible）
	Transport OpenAIUpstreamTransport
	// NoSlot 计 token 这类非计费请求：不抢槽、不绑粘性、不等待（粘性命中就用它，否则优先级 + LRU 首个）。
	NoSlot bool
	// OnlyAccountID 只认这一个账号（grok 视频状态轮询只能落回任务归属账号）；它不能承接就是无候选，不逃逸到别的账号。
	OnlyAccountID int64
	// Platform 端点要求的厂商平台（无模型端点用：live / realtime → openai，web_search / tts → grok…）。
	// 池按它装载，且候选必须是该平台的成品号——网关平台桶会放进任何有兼容地址的第三方 key，
	// 而这些端点调的是厂商原生 API（xAI 搜索 / 语音、OpenAI live）。第三方 key 一律按中转，
	// 贴着该平台标签的 key 也承接不了（2026-09-29 定，海外四家不再有官方 key）。
	// 不是 ForcePlatform——不跳过任何门。空 = 由路由 / 分组决定。
	Platform string
}

type selectOptionsCtxKey struct{}

// WithSelectOptions 把要求挂到 ctx：入口设一次，负载感知与 legacy 两条路径的门都从 ctx 读。
func WithSelectOptions(ctx context.Context, opts SelectOptions) context.Context {
	return context.WithValue(ctx, selectOptionsCtxKey{}, opts)
}

func selectOptionsFromContext(ctx context.Context) SelectOptions {
	if ctx == nil {
		return SelectOptions{}
	}
	opts, _ := ctx.Value(selectOptionsCtxKey{}).(SelectOptions)
	return opts
}

// admits 报告账号能否承接这些要求；reason 是第一条不满足的门名。
func (o SelectOptions) admits(cfg *config.Config, resolver OpenAIWSProtocolResolver, account *Account) (bool, string) {
	if o.OnlyAccountID > 0 && account.ID != o.OnlyAccountID {
		return false, "not_owner"
	}
	if o.Platform != "" && (account.Platform != o.Platform || account.IsThirdPartyKey()) {
		return false, "platform_mismatch"
	}
	if !account.SupportsOpenAIEndpointCapability(o.Capability) {
		return false, "capability_mismatch"
	}
	if !account.SupportsOpenAIImageCapability(o.ImageCapability) {
		return false, "image_capability_mismatch"
	}
	if o.RequireCompact && openAICompactSupportTier(account) == 0 {
		return false, "compact_unsupported"
	}
	if o.Transport != "" && !openAIAccountTransportCompatible(cfg, resolver, account, o.Transport) {
		return false, "transport_mismatch"
	}
	return true, ""
}

// openAIAccountTransportCompatible 报告账号能否以 required 传输承接：HTTP / any 恒可；
// WS v2 ingress 按路由模式或账号解析出的传输判定。
func openAIAccountTransportCompatible(cfg *config.Config, resolver OpenAIWSProtocolResolver, account *Account, required OpenAIUpstreamTransport) bool {
	if required == OpenAIUpstreamTransportAny || required == OpenAIUpstreamTransportHTTPSSE {
		return true
	}
	if account == nil || resolver == nil {
		return false
	}
	if required == OpenAIUpstreamTransportResponsesWebsocketV2Ingress {
		if cfg == nil || !cfg.Gateway.OpenAIWS.ModeRouterV2Enabled {
			return resolver.Resolve(account).Transport == OpenAIUpstreamTransportResponsesWebsocketV2
		}
		mode := account.ResolveOpenAIResponsesWebSocketV2Mode(cfg.Gateway.OpenAIWS.IngressModeDefault)
		switch mode {
		case OpenAIWSIngressModeCtxPool, OpenAIWSIngressModePassthrough, OpenAIWSIngressModeHTTPBridge, OpenAIWSIngressModeShared, OpenAIWSIngressModeDedicated:
			return true
		default:
			return false
		}
	}
	return resolver.Resolve(account).Transport == required
}

// wsProtocolResolver 返回传输门用的 WS 协议解析器（按 cfg 懒建一次）。
func (s *GatewayService) wsProtocolResolver() OpenAIWSProtocolResolver {
	s.openaiWSResolverOnce.Do(func() {
		if s.openaiWSResolver == nil {
			s.openaiWSResolver = NewOpenAIWSProtocolResolver(s.cfg)
		}
	})
	return s.openaiWSResolver
}
