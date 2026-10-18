package service

import (
	"fmt"
	"net/url"
	"strings"
)

// officialVendorExtraHosts 是官方预填地址之外、代码里已按官方站点对待的域名。
// 预填表每个厂商只放一个默认站点，国际站等备用站点记在这里，并注明出处。
var officialVendorExtraHosts = map[string][]string{
	PlatformKimi:    {"api.moonshot.ai"}, // 国际站，见 security.url_allowlist.upstream_hosts 默认值
	PlatformMiniMax: {"api.minimax.io"},  // 国际站，见 minimaxQuotaURL
	PlatformZhipu:   {"api.z.ai"},        // 国际站，见 zhipuQuotaHost
}

var officialVendorHosts, officialVideoVendorHosts = mustBuildOfficialVendorHosts()

func mustBuildOfficialVendorHosts() (map[string]string, map[string]string) {
	hosts, videoHosts, err := buildOfficialVendorHosts()
	if err != nil {
		panic(err)
	}
	return hosts, videoHosts
}

// buildOfficialVendorHosts 汇总「官方域名 → 厂商」对照表，对话协议与 video 协议各一张。
//
// 表由官方预填地址（PlatformProtocolDefaults 的全部模式）生成，不另抄一份域名清单：
// 预填地址改了，厂商识别跟着变，两边不会分叉。对话表里只有国产厂商与 OpenCode——
// Anthropic、OpenAI、Gemini、Grok 的官方域名不在表里，指向它们的 key 按中转处理。
// video 表只有官方视频厂商；分开是因为同一域名上的对话 API 不归视频厂商：
// 火山方舟的对话（豆包）与 Seedance 同在 ark.cn-beijing.volces.com，对话 key 仍按中转。
func buildOfficialVendorHosts() (map[string]string, map[string]string, error) {
	hosts := make(map[string]string)
	videoHosts := make(map[string]string)
	add := func(table map[string]string, vendor, rawURL string) error {
		host := upstreamHostOf(rawURL)
		if host == "" {
			return fmt.Errorf("official %s address %q has no host", vendor, rawURL)
		}
		if existing, ok := table[host]; ok && existing != vendor {
			return fmt.Errorf("official host %s is claimed by both %s and %s", host, existing, vendor)
		}
		table[host] = vendor
		return nil
	}
	for _, platform := range PlatformsWithProtocolDefaults() {
		for _, mode := range ProtocolDefaultsModes() {
			for protocol, address := range PlatformProtocolDefaults(platform, mode) {
				table := hosts
				if protocol == APIProtocolVideo {
					table = videoHosts
				}
				if err := add(table, platform, address); err != nil {
					return nil, nil, err
				}
			}
		}
	}
	for vendor, extras := range officialVendorExtraHosts {
		for _, host := range extras {
			if err := add(hosts, vendor, "https://"+host); err != nil {
				return nil, nil, err
			}
		}
	}
	return hosts, videoHosts, nil
}

func upstreamHostOf(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}

// OfficialVendorHosts 返回对话协议「官方域名 → 厂商」对照表的副本：管理端建第三方 key 时按地址提示厂商，
// 与 OfficialVendorOfURL 同一张表，前端不另抄。
func OfficialVendorHosts() map[string]string {
	return copyHostTable(officialVendorHosts)
}

// OfficialVideoVendorHosts 返回 video 协议「官方域名 → 厂商」对照表的副本（OfficialVideoVendorOfURL 同一张表）。
func OfficialVideoVendorHosts() map[string]string {
	return copyHostTable(officialVideoVendorHosts)
}

func copyHostTable(table map[string]string) map[string]string {
	out := make(map[string]string, len(table))
	for host, vendor := range table {
		out[host] = vendor
	}
	return out
}

// OfficialVendorOfURL 返回对话协议地址所属的官方厂商；不是已知官方域名时返回空串。
// 只认完整域名、不做后缀匹配，api.openai.com.example.net 这类地址不会被当成官方。
func OfficialVendorOfURL(rawURL string) string {
	return officialVendorHosts[upstreamHostOf(rawURL)]
}

// OfficialVideoVendorOfURL 返回 video 协议地址所属的官方视频厂商；认不出返回空串（同样只认完整域名）。
func OfficialVideoVendorOfURL(rawURL string) string {
	return officialVideoVendorHosts[upstreamHostOf(rawURL)]
}

// officialVendorOfEndpoint 按协议选表：video 地址查视频厂商表，其余查对话表。
func officialVendorOfEndpoint(protocol, rawURL string) string {
	if protocol == APIProtocolVideo {
		return OfficialVideoVendorOfURL(rawURL)
	}
	return OfficialVendorOfURL(rawURL)
}

// Vendor 返回账号实际对接的上游厂商（平台标识），决定厂商特化是否启用。
//
// 成品号的厂商就是它的平台——成品号只走官方地址。
//
// 第三方 key 选的平台只是展示标签，厂商看协议地址：全部地址都是同一厂商的官方
// 域名才算该厂商；任一地址不是官方域名、或分属不同厂商，返回空串，按通用中转
// 只走标准协议。宁可漏认也不错认：错认会把厂商私有的请求改写与错误语义套到中转上。
//
// 官方域名表只有国产厂商、OpenCode 与官方视频厂商（officialVendorHosts / officialVideoVendorHosts），
// 所以第三方 key 的厂商只可能是它们或空串：Anthropic、OpenAI、Gemini、Grok 的厂商特化只对成品号生效，
// 指向这四家官方域名的 key 同样按中转处理（muqian 2026-09-29 定：「中转不分协议用同一套
// 规则，成品号按各自厂商特色各用一套」）。
func (a *Account) Vendor() string {
	if a == nil {
		return ""
	}
	if !a.IsThirdPartyKey() {
		return a.Platform
	}
	vendor := ""
	for protocol, endpoint := range a.ProtocolEndpoints {
		endpointVendor := officialVendorOfEndpoint(protocol, endpoint)
		if endpointVendor == "" || (vendor != "" && vendor != endpointVendor) {
			return ""
		}
		vendor = endpointVendor
	}
	return vendor
}

// AccountModelFamily 返回账号默认模型表所属的厂商族：成品号即平台；第三方 key 不看标签，
// 按地址识别出官方厂商（国产厂商 / OpenCode / 官方视频厂商）就是该厂商，其余一律按中转、按主协议归族——
// Anthropic 地址归 anthropic，Gemini 地址归 gemini，其余（Chat Completions / Responses）
// 归 openai 兼容族。管理端「可用模型」列表、测试连接的默认模型都按它选表。
func AccountModelFamily(a *Account) string {
	if a == nil {
		return ""
	}
	if !a.IsThirdPartyKey() {
		return a.Platform
	}
	if vendor := a.Vendor(); vendor != "" {
		return vendor
	}
	switch a.PrimaryUpstreamProtocol() {
	case APIProtocolAnthropic:
		return PlatformAnthropic
	case APIProtocolGemini:
		return PlatformGemini
	default:
		return PlatformOpenAI
	}
}
