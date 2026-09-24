package service

import (
	"net/url"
	"strings"
)

// OpenCode Go 是 OpenCode Zen 的订阅网关：额度窗口为 rolling(5h) / weekly / monthly。
// 上游协议与其他第三方 key 一样由账号配的那一个协议地址决定（一个资源只承接一个上游
// 协议，见 NormalizeProtocolEndpoints），要多协议就按协议各配一个 key。

const (
	openCodeGoUsagePath = "/usage"
	// DefaultOpenCodeGoTestModel is the admin connection-test fallback when
	// the UI does not pick a model. glm-5.3 is a Chat Completions catalog ID.
	DefaultOpenCodeGoTestModel = "glm-5.3"
)

// DefaultOpenCodeGoModelIDs 是官方文档当前公开的模型 ID 目录，
// 供 /v1/models 在尚未同步上游列表时回退，以及账号白名单预填。
func DefaultOpenCodeGoModelIDs() []string {
	return []string{
		"grok-4.6",
		"gpt-5.6-luna",
		"glm-5.3-flash",
		"glm-5.3",
		"glm-5.2",
		"glm-5.1",
		"kimi-k3",
		"kimi-k2.7-code",
		"kimi-k2.6",
		"longcat-2.0",
		"deepseek-v4-pro",
		"deepseek-v4-flash",
		"deepseek-v4-flash-vision-exp",
		"mimo-v2.5",
		"mimo-v2.5-pro",
		"minimax-m3",
		"minimax-m2.7",
		"minimax-m2.5",
		"muse-spark-1.3-contributor",
		"muse-spark-1.2-contributor",
		"qwen3.8-max",
		"qwen3.8-flash",
		"qwen3.7-max",
		"qwen3.7-plus",
		"qwen3.6-plus",
		"hy4-preview",
		"hy3",
		"omen-alpha",
	}
}

func (a *Account) IsOpenCodeGo() bool {
	return a != nil && a.Platform == PlatformOpenCodeGo
}

// openCodeEndpointMode 按协议地址区分 OpenCode 的 Go 套餐与 Zen 按量：Go 的官方地址在
// /zen/go 下。地址是比平台标签下的 account_mode 更直接的依据。
func (a *Account) openCodeEndpointMode() string {
	for _, endpoint := range a.ProtocolEndpoints {
		parsed, err := url.Parse(strings.TrimSpace(endpoint))
		if err != nil {
			continue
		}
		path := strings.TrimRight(parsed.Path, "/")
		if path == "/zen/go" || strings.HasPrefix(path, "/zen/go/") {
			return AccountModeGo
		}
	}
	return AccountModeZen
}

func openCodeGoQuotaURL(baseURL string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/") + openCodeGoUsagePath
}
