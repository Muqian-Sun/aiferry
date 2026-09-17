package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

func grokBaseURLValidator(account *Account, cfg *config.Config) (xai.BaseURLValidator, error) {
	if account == nil {
		return nil, fmt.Errorf("grok account is required")
	}
	// 第三方 key 的地址来自管理员填写的协议映射，按出站 URL 安全策略校验，与标签无关。
	if account.IsThirdPartyKey() {
		return redactedGrokBaseURLValidator(grokOperatorPolicyValidator(cfg)), nil
	}
	if !account.IsGrok() {
		return nil, fmt.Errorf("grok account is required")
	}
	switch account.Type {
	case AccountTypeOAuth:
		// Official gateway hosts are always trusted and always usable, even when
		// the operator enables a restrictive URL allowlist. A custom forwarding
		// host is vetted by the same operator policy as API-key accounts.
		//
		// The official-vs-custom decision is made on the host, not via
		// ValidateTrustedBaseURL: that validator relaxes to accept-any under the
		// XAI_ALLOW_UNSAFE_URL_OVERRIDES debug switch, which must never let an
		// OAuth bearer token reach an arbitrary custom host.
		policyValidator := grokOperatorPolicyValidator(cfg)
		return redactedGrokBaseURLValidator(func(raw string) (string, error) {
			if xai.IsOfficialBaseURL(raw) {
				return xai.ValidateTrustedBaseURL(raw)
			}
			return policyValidator(raw)
		}), nil
	default:
		return nil, fmt.Errorf("unsupported grok account type: %s", account.Type)
	}
}

// grokOperatorPolicyValidator 按全局出站 URL 安全策略校验自定义 base_url：
// 白名单开启时强制 UpstreamHosts；关闭时仅做格式校验（HTTP 允许与否跟随配置）。
func grokOperatorPolicyValidator(cfg *config.Config) xai.BaseURLValidator {
	if cfg == nil {
		return xai.ValidateBaseURL
	}
	if !cfg.Security.URLAllowlist.Enabled {
		return func(raw string) (string, error) {
			return urlvalidator.ValidateURLFormat(raw, cfg.Security.URLAllowlist.AllowInsecureHTTP)
		}
	}
	return func(raw string) (string, error) {
		return urlvalidator.ValidateHTTPSURL(raw, urlvalidator.ValidationOptions{
			AllowedHosts:     cfg.Security.URLAllowlist.UpstreamHosts,
			RequireAllowlist: true,
			AllowPrivate:     cfg.Security.URLAllowlist.AllowPrivateHosts,
		})
	}
}

func redactedGrokBaseURLValidator(validator xai.BaseURLValidator) xai.BaseURLValidator {
	return func(raw string) (string, error) {
		validated, err := validator(raw)
		if err != nil {
			return "", errors.New("base URL rejected by URL security policy")
		}
		return validated, nil
	}
}

func buildGrokResponsesURL(account *Account, cfg *config.Config, settings ...*SettingService) (string, error) {
	validator, err := grokBaseURLValidator(account, cfg)
	if err != nil {
		return "", err
	}
	baseURL, err := grokProtocolBaseURL(account, APIProtocolResponses, settings...)
	if err != nil {
		return "", err
	}
	return xai.BuildResponsesURLWithValidator(baseURL, validator)
}

// grokProtocolBaseURL 取 Grok 文本流量在指定协议下的上游地址。
//
// 第三方 key 只认该协议的映射，缺了直接报错；成品号沿用原有口径：账号地址优先，
// 其次站点配置的默认区域。
func grokProtocolBaseURL(account *Account, protocol string, settings ...*SettingService) (string, error) {
	if account.IsThirdPartyKey() {
		baseURL := account.ProtocolEndpoint(protocol)
		if baseURL == "" {
			return "", MissingProtocolEndpointError(account, protocol)
		}
		return baseURL, nil
	}
	if len(settings) > 0 && settings[0] != nil {
		return settings[0].ResolveGrokBaseURL(context.Background(), account), nil
	}
	return account.GetGrokBaseURL(), nil
}

func buildGrokChatCompletionsURL(account *Account, cfg *config.Config, settings ...*SettingService) (string, error) {
	validator, err := grokBaseURLValidator(account, cfg)
	if err != nil {
		return "", err
	}
	baseURL, err := grokProtocolBaseURL(account, APIProtocolChatCompletions, settings...)
	if err != nil {
		return "", err
	}
	return xai.BuildChatCompletionsURLWithValidator(baseURL, validator)
}

// buildGrokBillingURL 解析 billing 探测端点：跟随账号的转发 base_url，
// 未定制的账号仍指向官方 CLI 网关。
func buildGrokBillingURL(account *Account, cfg *config.Config, weekly bool) (string, error) {
	validator, err := grokBaseURLValidator(account, cfg)
	if err != nil {
		return "", err
	}
	baseURL := account.GetGrokBaseURL()
	// Official public/regional API hosts do not expose Grok Build billing.
	// Keep custom relays on their configured host because they may proxy the CLI
	// billing path alongside inference.
	if xai.IsOfficialBaseURL(baseURL) && !isGrokCLIProxyBaseURL(baseURL) {
		baseURL = xai.DefaultCLIBaseURL
	}
	return xai.BuildBillingURLWithValidator(baseURL, weekly, validator)
}

func buildGrokMediaURL(account *Account, cfg *config.Config, endpoint GrokMediaEndpoint, requestID string) (string, error) {
	validator, err := grokBaseURLValidator(account, cfg)
	if err != nil {
		return "", err
	}
	baseURL := account.GetGrokMediaBaseURL()
	if baseURL == "" {
		return "", MissingProtocolEndpointError(account, APIProtocolChatCompletions)
	}
	switch endpoint {
	case GrokMediaEndpointImagesGenerations:
		return xai.BuildImagesGenerationsURLWithValidator(baseURL, validator)
	case GrokMediaEndpointImagesEdits:
		return xai.BuildImagesEditsURLWithValidator(baseURL, validator)
	case GrokMediaEndpointVideosGenerations:
		return xai.BuildVideosGenerationsURLWithValidator(baseURL, validator)
	case GrokMediaEndpointVideosEdits:
		return xai.BuildVideosEditsURLWithValidator(baseURL, validator)
	case GrokMediaEndpointVideosExtensions:
		return xai.BuildVideosExtensionsURLWithValidator(baseURL, validator)
	case GrokMediaEndpointVideoStatus:
		return xai.BuildVideoURLWithValidator(baseURL, requestID, validator)
	case GrokMediaEndpointVideoContent:
		videoURL, err := xai.BuildVideoURLWithValidator(baseURL, requestID, validator)
		if err != nil {
			return "", err
		}
		return videoURL + "/content", nil
	default:
		return "", fmt.Errorf("unsupported grok media endpoint: %s", endpoint)
	}
}

// buildGrokVoiceURL returns the official xAI Voice API endpoint.
// Voice HTTP (/tts, /stt, /custom-voices) and WS (/realtime) are only exposed
// by api.x.ai — the CLI chat proxy does not implement them. When the account
// base_url points at the CLI proxy (or is empty), fall back to DefaultBaseURL.
func buildGrokVoiceURL(account *Account, cfg *config.Config, endpoint string) (string, error) {
	validator, err := grokBaseURLValidator(account, cfg)
	if err != nil {
		return "", err
	}
	base := ""
	if account != nil {
		base = account.GetGrokMediaBaseURL()
		if base == "" && account.IsThirdPartyKey() {
			return "", MissingProtocolEndpointError(account, APIProtocolChatCompletions)
		}
	}
	if strings.TrimSpace(base) == "" || isGrokCLIProxyBaseURL(base) {
		base = xai.DefaultBaseURL
	}
	validated, err := validator(base)
	if err != nil {
		return "", err
	}
	ep := strings.Trim(strings.TrimSpace(endpoint), "/")
	if ep == "" {
		return "", fmt.Errorf("voice endpoint is required")
	}
	parts := strings.Split(ep, "/")
	encoded := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" || part == "." || part == ".." {
			return "", fmt.Errorf("invalid voice endpoint path")
		}
		encoded = append(encoded, url.PathEscape(part))
	}
	return strings.TrimRight(validated, "/") + "/" + strings.Join(encoded, "/"), nil
}

func isGrokCLIProxyBaseURL(raw string) bool {
	return isGrokCLIProxyTarget(raw)
}
