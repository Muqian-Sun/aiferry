package service

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
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
