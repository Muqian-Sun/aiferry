package service

import "github.com/Wei-Shaw/sub2api/internal/config"

// OpenAIUpstreamTransport 表示 OpenAI 上游传输协议。
type OpenAIUpstreamTransport string

const (
	OpenAIUpstreamTransportAny                  OpenAIUpstreamTransport = ""
	OpenAIUpstreamTransportHTTPSSE              OpenAIUpstreamTransport = "http_sse"
	OpenAIUpstreamTransportResponsesWebsocket   OpenAIUpstreamTransport = "responses_websockets"
	OpenAIUpstreamTransportResponsesWebsocketV2 OpenAIUpstreamTransport = "responses_websockets_v2"
	// OpenAIUpstreamTransportResponsesWebsocketV2Ingress 用于 WS ingress 入口选账号：
	// mode_router_v2 开启时允许 ctx_pool/passthrough/http_bridge，拒绝 off。
	OpenAIUpstreamTransportResponsesWebsocketV2Ingress OpenAIUpstreamTransport = "responses_websockets_v2_ingress"
)

// OpenAIWSProtocolDecision 表示协议决策结果。
type OpenAIWSProtocolDecision struct {
	Transport OpenAIUpstreamTransport
	Reason    string
}

// OpenAIWSProtocolResolver 定义 OpenAI 上游协议决策。
type OpenAIWSProtocolResolver interface {
	Resolve(account *Account) OpenAIWSProtocolDecision
}

type defaultOpenAIWSProtocolResolver struct {
	cfg *config.Config
}

// NewOpenAIWSProtocolResolver 创建默认协议决策器。
func NewOpenAIWSProtocolResolver(cfg *config.Config) OpenAIWSProtocolResolver {
	return &defaultOpenAIWSProtocolResolver{cfg: cfg}
}

func (r *defaultOpenAIWSProtocolResolver) Resolve(account *Account) OpenAIWSProtocolDecision {
	if account == nil {
		return openAIWSHTTPDecision("account_missing")
	}
	if !openAIProtocolFeaturesApply(account) {
		return openAIWSHTTPDecision("platform_not_openai")
	}
	// WSv2 是 Responses 协议的传输：第三方 key 本次不以 responses 协议转发（没配地址）时走 HTTP。
	if account.IsThirdPartyKey() && openAIGatewayKeyProtocol(account, APIProtocolResponses) != APIProtocolResponses {
		return openAIWSHTTPDecision("responses_endpoint_missing")
	}
	if account.IsOpenAIWSForceHTTPEnabled() {
		return openAIWSHTTPDecision("account_force_http")
	}
	if r == nil || r.cfg == nil {
		return openAIWSHTTPDecision("config_missing")
	}

	wsCfg := r.cfg.Gateway.OpenAIWS
	if wsCfg.ForceHTTP {
		return openAIWSHTTPDecision("global_force_http")
	}
	if !wsCfg.Enabled {
		return openAIWSHTTPDecision("global_disabled")
	}
	if account.IsOpenAIOAuthLike() {
		if !wsCfg.OAuthEnabled {
			return openAIWSHTTPDecision("oauth_disabled")
		}
	} else if account.IsThirdPartyKey() {
		if !wsCfg.APIKeyEnabled {
			return openAIWSHTTPDecision("apikey_disabled")
		}
	} else {
		return openAIWSHTTPDecision("unknown_auth_type")
	}
	if wsCfg.ModeRouterV2Enabled {
		mode := account.ResolveOpenAIResponsesWebSocketV2Mode(wsCfg.IngressModeDefault)
		switch mode {
		case OpenAIWSIngressModeOff:
			return openAIWSHTTPDecision("account_mode_off")
		case OpenAIWSIngressModeCtxPool, OpenAIWSIngressModePassthrough:
			// continue
		case OpenAIWSIngressModeHTTPBridge:
			return openAIWSHTTPDecision("ws_v2_mode_http_bridge")
		case OpenAIWSIngressModeShared, OpenAIWSIngressModeDedicated:
			// 历史值兼容：按 ctx_pool 处理。
			mode = OpenAIWSIngressModeCtxPool
		default:
			return openAIWSHTTPDecision("account_mode_off")
		}
		if account.Concurrency <= 0 {
			return openAIWSHTTPDecision("account_concurrency_invalid")
		}
		if wsCfg.ResponsesWebsocketsV2 {
			return OpenAIWSProtocolDecision{
				Transport: OpenAIUpstreamTransportResponsesWebsocketV2,
				Reason:    "ws_v2_mode_" + mode,
			}
		}
		if wsCfg.ResponsesWebsockets {
			return OpenAIWSProtocolDecision{
				Transport: OpenAIUpstreamTransportResponsesWebsocket,
				Reason:    "ws_v1_mode_" + mode,
			}
		}
		return openAIWSHTTPDecision("feature_disabled")
	}
	// legacy 路径（mode_router_v2 关）恒走 HTTP：渠道级 WS 开关 2026-09-28 P5 删了。
	// 要开 WS 用 GATEWAY_OPENAI_WS_MODE_ROUTER_V2_ENABLED（走上面的 ingress_mode_default）。
	return openAIWSHTTPDecision("account_disabled")
}

func openAIWSHTTPDecision(reason string) OpenAIWSProtocolDecision {
	return OpenAIWSProtocolDecision{
		Transport: OpenAIUpstreamTransportHTTPSSE,
		Reason:    reason,
	}
}
