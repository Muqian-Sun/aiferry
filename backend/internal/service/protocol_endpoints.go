package service

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// APIProtocolGemini 是 Gemini v1beta generateContent 协议标识。
const APIProtocolGemini = domain.APIProtocolGemini

// IsUpstreamProtocol 报告是否为可用作 protocol_endpoints 键的具体协议。
var IsUpstreamProtocol = domain.IsUpstreamProtocol

// UpstreamProtocols 返回全部具体上游协议，顺序固定，供校验报错与前端选项使用。
func UpstreamProtocols() []string {
	return []string{
		APIProtocolAnthropic,
		APIProtocolChatCompletions,
		APIProtocolResponses,
		APIProtocolGemini,
	}
}

// NormalizeProtocolEndpoints 校验并规范化「协议 → 上游地址」映射。
//
// 规范化只做两件事：去掉首尾空白、去掉地址末尾的斜杠。**不补任何路径后缀**——
// 地址以管理员填写的为准，拼接端点路径是转发时的事（joinUpstreamEndpointURL）。
//
// 校验一律 fail-closed：未知协议键、空地址、**配了多于一个协议**都直接报错而不是静默
// 丢弃。静默丢弃会让管理员以为配置已保存，却在转发时才表现为地址缺失。
//
// 一个资源只承接一个上游协议（产品约定，2026-09-23 写进代码）：一个 key 可以有多个
// 模型，但协议只有一个；同一渠道要承接多个协议就配多个 key，目录条目分别绑到对应的
// key 上。配多个协议地址时，选号侧按入站协议的偏好序挑一个、转发侧可能按别的依据挑另
// 一个，两边口径不一致——与其在转发层补救，不如在配置入口挡住。
func NormalizeProtocolEndpoints(in map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(in))
	if len(in) == 0 {
		return out, nil
	}

	unknown := make([]string, 0)
	for rawKey, rawValue := range in {
		key := strings.ToLower(strings.TrimSpace(rawKey))
		if !IsUpstreamProtocol(key) {
			unknown = append(unknown, rawKey)
			continue
		}
		value := strings.TrimRight(strings.TrimSpace(rawValue), "/")
		if value == "" {
			return nil, fmt.Errorf("protocol %q has an empty base URL", key)
		}
		// 地址必须是带主机名的 http(s) 地址：原来随便一串（如 "not a url"）也能存，渠道显示正常、可调度，
		// 到转发时才失败（2026-10-04 UI E2E）。
		if parsed, err := url.Parse(value); err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return nil, fmt.Errorf("protocol %q base URL %q must be an http(s) URL with a host", key, value)
		}
		if _, exists := out[key]; exists {
			return nil, fmt.Errorf("protocol %q is configured more than once", key)
		}
		out[key] = value
	}

	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf(
			"unknown upstream protocol(s) %s; supported: %s",
			strings.Join(unknown, ", "),
			strings.Join(UpstreamProtocols(), ", "),
		)
	}
	if len(out) > 1 {
		configured := make([]string, 0, len(out))
		for protocol := range out {
			configured = append(configured, protocol)
		}
		sort.Strings(configured)
		return nil, fmt.Errorf(
			"a resource serves exactly one upstream protocol, got %s; configure one key per protocol",
			strings.Join(configured, ", "),
		)
	}
	return out, nil
}

// ProtocolEndpoint 返回账号为指定协议配置的上游地址，未配置时返回空串。
func (a *Account) ProtocolEndpoint(protocol string) string {
	if a == nil || len(a.ProtocolEndpoints) == 0 {
		return ""
	}
	return strings.TrimSpace(a.ProtocolEndpoints[protocol])
}

// IsThirdPartyKey 报告账号是否为第三方 key（与成品号相对）。
// 来源只由类型决定：apikey 是第三方 key，oauth / setup-token / bedrock / service_account 是成品号。
// 成品号需要厂商特有的令牌刷新、客户端伪装与额度窗口解析；第三方 key 不需要。
func (a *Account) IsThirdPartyKey() bool {
	return a != nil && a.Type == AccountTypeAPIKey
}

// WithInboundProtocol 把本次请求的入站协议放进 context，供调度做协议偏好。
func WithInboundProtocol(ctx context.Context, protocol string) context.Context {
	if ctx == nil || !IsUpstreamProtocol(protocol) {
		return ctx
	}
	return context.WithValue(ctx, ctxkey.InboundProtocol, protocol)
}

// InboundProtocolFromContext 取本次请求的入站协议，未设置时返回空串。
func InboundProtocolFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	protocol, _ := ctx.Value(ctxkey.InboundProtocol).(string)
	return protocol
}

// ValidateProtocolEndpoints 校验账号的协议映射是否满足其来源维度的要求。
//
// 第三方 key 必须至少配置一个协议地址：地址是它唯一的上游坐标，缺了就没有任何
// 可回落的默认值。成品号相反——它的端点由厂商决定，不需要也不应该配。
func (a *Account) ValidateProtocolEndpoints() error {
	if a == nil {
		return nil
	}
	if !a.IsThirdPartyKey() {
		return nil
	}
	if len(a.ProtocolEndpoints) == 0 {
		return fmt.Errorf("third-party key account requires at least one protocol endpoint (%s)", strings.Join(UpstreamProtocols(), " / "))
	}
	return nil
}

// PlatformProtocolDefaults 返回某平台各协议的官方端点地址，供管理端建号时预填。
//
// 预填而不是代码里兜底：管理员看到的是一个可改的具体地址，存库的也是这个地址。
// 代码里的隐式默认会让「忘了配地址」表现成「请求打到官方端点然后 401」。
//
// 只有国产厂商与 OpenCode：它们没有成品号，只能以 key 接入，官方地址上有厂商特化（余额 /
// 额度探测、Coding Plan 等）。Anthropic、OpenAI、Gemini、Grok 不在表里——这四家只认成品号，
// 指向它们官方域名的 key 一律按中转处理（muqian 2026-09-29 定）。
//
// accountMode 用于区分同一平台的按量与 Coding 套餐端点，空串按按量处理。
func PlatformProtocolDefaults(platform string, accountMode string) map[string]string {
	coding := accountMode == AccountModeCoding
	switch platform {
	case PlatformKimi:
		if coding {
			return map[string]string{
				APIProtocolChatCompletions: DefaultKimiCodingBaseURL,
				APIProtocolAnthropic:       DefaultKimiCodingAnthropicBaseURL,
				APIProtocolResponses:       DefaultKimiCodingBaseURL,
			}
		}
		return map[string]string{
			APIProtocolChatCompletions: DefaultKimiPayGBaseURL,
			APIProtocolAnthropic:       DefaultKimiPayGAnthropicBaseURL,
			APIProtocolResponses:       DefaultKimiPayGBaseURL,
		}
	case PlatformZhipu:
		chat := DefaultZhipuPayGBaseURL
		if coding {
			chat = DefaultZhipuCodingBaseURL
		}
		return map[string]string{
			APIProtocolChatCompletions: chat,
			APIProtocolAnthropic:       DefaultZhipuAnthropicBaseURL,
		}
	case PlatformDeepseek:
		return map[string]string{
			APIProtocolChatCompletions: DefaultDeepseekBaseURL,
			APIProtocolAnthropic:       DefaultDeepseekAnthropicBaseURL,
			APIProtocolResponses:       DefaultDeepseekBaseURL,
		}
	case PlatformMiniMax:
		return map[string]string{
			APIProtocolChatCompletions: DefaultMiniMaxBaseURL,
			APIProtocolAnthropic:       DefaultMiniMaxAnthropicBaseURL,
			APIProtocolResponses:       DefaultMiniMaxBaseURL,
		}
	case PlatformOpenCodeGo:
		if accountMode == AccountModeGo {
			return map[string]string{
				APIProtocolChatCompletions: DefaultOpenCodeGoBaseURL,
				APIProtocolAnthropic:       DefaultOpenCodeGoAnthropicBaseURL,
				APIProtocolResponses:       DefaultOpenCodeGoBaseURL,
			}
		}
		return map[string]string{
			APIProtocolChatCompletions: DefaultOpenCodeZenBaseURL,
			APIProtocolAnthropic:       DefaultOpenCodeZenAnthropicBaseURL,
			APIProtocolResponses:       DefaultOpenCodeZenBaseURL,
		}
	default:
		return map[string]string{}
	}
}

// PlatformsWithProtocolDefaults 返回有官方端点可预填的平台列表。
// 与 PlatformProtocolDefaults 放在一起维护，避免平台列表被抄成两份。
func PlatformsWithProtocolDefaults() []string {
	return []string{PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax, PlatformOpenCodeGo}
}

// ResolveUpstreamBaseURL 决定最终使用的上游地址。
//
// 第三方 key 取不到地址就是配置错误，直接报错而不是回落官方端点——回落会把
// 「忘了配地址」变成「请求打到官方端点然后 401」，排查要绕一大圈。成品号相反，
// 它本来就该走厂商官方端点。
func ResolveUpstreamBaseURL(account *Account, resolved string, protocol string, officialDefault string) (string, error) {
	if strings.TrimSpace(resolved) != "" {
		return resolved, nil
	}
	if account != nil && account.IsThirdPartyKey() {
		return "", MissingProtocolEndpointError(account, protocol)
	}
	return officialDefault, nil
}

// HasOpenAIProtocolEndpoint 报告第三方 key 是否配了 OpenAI 协议族（Chat Completions 或
// Responses）的地址：OpenAI 协议层面的账号设置（端点能力等）按它露出，不看标签。
func (a *Account) HasOpenAIProtocolEndpoint() bool {
	return a.ProtocolEndpoint(APIProtocolChatCompletions) != "" || a.ProtocolEndpoint(APIProtocolResponses) != ""
}

// PrimaryUpstreamBaseURL 返回账号的主上游地址。
//
// 用于那些「只需要知道这个账号大致指向哪」的判断：Ollama Cloud 识别、模型同步、
// 计费探测等。第三方 key 按 primaryUpstreamProtocolOrder 取第一个已配置的协议地址，
// 与平台标签无关；成品号只走厂商官方地址、没有账号级地址，返回空串。
//
// 不引入这个入口的话，第三方 key 只配协议映射、不配 base_url 之后，这些判断会
// 静默拿到空串——不报错，只是行为悄悄消失。
func (a *Account) PrimaryUpstreamBaseURL() string {
	return a.ProtocolEndpoint(a.PrimaryUpstreamProtocol())
}

// PrimaryUpstreamProtocol 返回 PrimaryUpstreamBaseURL 选中的协议，未配置任何协议地址
// 时返回空串。需要知道「主地址是哪个协议」的调用方（模型列表同步要按协议选请求形态）
// 用它，避免再抄一份取址顺序。
func (a *Account) PrimaryUpstreamProtocol() string {
	if a == nil || !a.IsThirdPartyKey() {
		return ""
	}
	for _, protocol := range primaryUpstreamProtocolOrder {
		if a.ProtocolEndpoint(protocol) != "" {
			return protocol
		}
	}
	return ""
}

// primaryUpstreamProtocolOrder 是 PrimaryUpstreamBaseURL 取地址的协议顺序。
//
// OpenAI API 根地址（chat_completions，其次 responses）在前：余额查询、模型列表、
// 图片等扩展端点都挂在它下面，与 KeyUpstreamProtocols 对扩展端点只认
// chat_completions 地址一致。anthropic、gemini 地址是各自协议的专用根，排在后面。
var primaryUpstreamProtocolOrder = []string{
	APIProtocolChatCompletions,
	APIProtocolResponses,
	APIProtocolAnthropic,
	APIProtocolGemini,
}

// PrimaryUpstreamProtocolOrder 把 PrimaryUpstreamBaseURL 的取址顺序暴露给需要在 SQL 里
// 镜像同一判断的仓储层（Ollama Cloud 识别），两边共用一个真相源。
func PrimaryUpstreamProtocolOrder() []string {
	return append([]string(nil), primaryUpstreamProtocolOrder...)
}

// MissingProtocolEndpointError 是第三方 key 缺少某协议上游地址时的统一错误。
//
// 所有取址点共用这一个构造：错误码固定，排查时按 MISSING_PROTOCOL_ENDPOINT 一搜
// 就能定位到是哪个账号缺哪个协议，而不是各处各报一种「invalid base url」。
func MissingProtocolEndpointError(account *Account, protocol string) error {
	id := int64(0)
	if account != nil {
		id = account.ID
	}
	return infraerrors.BadRequest(
		"MISSING_PROTOCOL_ENDPOINT",
		fmt.Sprintf("account %d has no %s upstream address configured", id, protocol),
	)
}
