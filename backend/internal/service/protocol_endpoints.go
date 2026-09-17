package service

import (
	"context"
	"fmt"
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
// 校验一律 fail-closed：未知协议键、空地址都直接报错而不是静默丢弃。静默丢弃
// 会让管理员以为配置已保存，却在转发时才表现为地址缺失。
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
	return out, nil
}

// ProtocolEndpoint 返回账号为指定协议配置的上游地址，未配置时返回空串。
func (a *Account) ProtocolEndpoint(protocol string) string {
	if a == nil || len(a.ProtocolEndpoints) == 0 {
		return ""
	}
	return strings.TrimSpace(a.ProtocolEndpoints[protocol])
}

// UpstreamProtocolsOf 返回账号能直接对话的上游协议集合。
//
// 第三方 key：完全由 protocol_endpoints 的键决定，不做任何平台推导。地址与协议
// 都是管理员显式声明的，推导只会带来「猜错把请求推给不会说该协议的上游」。
//
// 成品号：按厂商推导。成品号本身就是厂商绑定的——OAuth 刷新、客户端伪装、额度
// 窗口解析都依赖厂商，协议同样由厂商决定，没有配置空间。
func (a *Account) UpstreamProtocolsOf() map[string]struct{} {
	out := make(map[string]struct{}, 4)
	if a == nil {
		return out
	}
	if a.IsThirdPartyKey() {
		for key := range a.ProtocolEndpoints {
			if IsUpstreamProtocol(key) {
				out[key] = struct{}{}
			}
		}
		return out
	}

	switch {
	case a.IsAnthropic():
		out[APIProtocolAnthropic] = struct{}{}
	case a.IsGemini():
		out[APIProtocolGemini] = struct{}{}
	case a.IsAntigravity():
		// Antigravity 同时暴露 Claude 与 Gemini 两种入站形态。
		out[APIProtocolAnthropic] = struct{}{}
		out[APIProtocolGemini] = struct{}{}
	case a.IsOpenAI(), a.IsGrok():
		out[APIProtocolResponses] = struct{}{}
		out[APIProtocolChatCompletions] = struct{}{}
	}
	return out
}

// IsThirdPartyKey 报告账号是否为第三方 key（与成品号相对）。
func (a *Account) IsThirdPartyKey() bool {
	if a == nil {
		return false
	}
	if kind := strings.TrimSpace(a.SourceKind); kind != "" {
		return kind == AccountSourceAPIKey
	}
	return DeriveAccountSourceKind(a.Type) == AccountSourceAPIKey
}

// SpeaksUpstreamProtocol 报告账号是否能直接对话该协议（不经协议转换）。
func (a *Account) SpeaksUpstreamProtocol(protocol string) bool {
	if protocol == "" {
		return false
	}
	_, ok := a.UpstreamProtocolsOf()[protocol]
	return ok
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

// DefaultProtocolForPlatform 返回该平台第三方 key 的默认协议标识。
// 用于把「平台 + base_url」这种旧形态的账号数据转换成协议映射。
func DefaultProtocolForPlatform(platform string) string {
	switch platform {
	case PlatformAnthropic, PlatformAntigravity:
		return APIProtocolAnthropic
	case PlatformGemini:
		return APIProtocolGemini
	case PlatformOpenAI, PlatformGrok, PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax, PlatformOpenCodeGo:
		return APIProtocolChatCompletions
	default:
		return ""
	}
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
// accountMode 用于区分同一平台的按量与 Coding 套餐端点，空串按按量处理。
func PlatformProtocolDefaults(platform string, accountMode string) map[string]string {
	coding := accountMode == AccountModeCoding
	switch platform {
	case PlatformAnthropic:
		return map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"}
	case PlatformOpenAI:
		return map[string]string{
			APIProtocolResponses:       "https://api.openai.com",
			APIProtocolChatCompletions: "https://api.openai.com",
		}
	case PlatformGemini:
		return map[string]string{APIProtocolGemini: "https://generativelanguage.googleapis.com"}
	case PlatformGrok:
		return map[string]string{
			APIProtocolResponses:       "https://api.x.ai/v1",
			APIProtocolChatCompletions: "https://api.x.ai/v1",
		}
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
	return []string{
		PlatformAnthropic, PlatformOpenAI, PlatformGemini, PlatformGrok,
		PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax, PlatformOpenCodeGo,
	}
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

// PrimaryUpstreamBaseURL 返回账号的主上游地址。
//
// 用于那些「只需要知道这个账号大致指向哪」的判断：Ollama Cloud 识别、模型同步、
// 计费探测等。第三方 key 取协议映射（优先该平台的默认协议，其次按固定顺序取
// 任意已配置协议），成品号取 credentials.base_url。
//
// 不引入这个入口的话，第三方 key 只配协议映射、不配 base_url 之后，这些判断会
// 静默拿到空串——不报错，只是行为悄悄消失。
func (a *Account) PrimaryUpstreamBaseURL() string {
	if a == nil {
		return ""
	}
	if !a.IsThirdPartyKey() {
		return a.StoredBaseURL()
	}
	if preferred := DefaultProtocolForPlatform(a.Platform); preferred != "" {
		if endpoint := a.ProtocolEndpoint(preferred); endpoint != "" {
			return endpoint
		}
	}
	for _, protocol := range UpstreamProtocols() {
		if endpoint := a.ProtocolEndpoint(protocol); endpoint != "" {
			return endpoint
		}
	}
	return ""
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
