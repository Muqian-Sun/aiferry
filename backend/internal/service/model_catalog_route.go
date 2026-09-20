package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// CatalogRoute 是准入后挂在 request.Context 上的「本次请求命中了哪个目录条目」。
type CatalogRoute struct {
	EntryID        int64
	CanonicalModel string             // entry.ModelID，计费与用量记录用
	RequestedModel string             // 客户端写的名字，调度与上游转发继续用它
	Platform       string             // CatalogRoutePlatform(entry)
	Entry          *ModelCatalogEntry // 快照指针，只读
}

// WithCatalogRoute 把目录路由挂到 ctx 上。同时写 ResolvedTargetPlatform：请求链下游
// （resolvePlatform、错误透传平台、handler 族分发）都先读这把钥匙。
func WithCatalogRoute(ctx context.Context, route CatalogRoute) context.Context {
	ctx = context.WithValue(ctx, ctxkey.CatalogRoute, route)
	ctx = WithResolvedTargetPlatform(ctx, route.Platform)
	return context.WithValue(ctx, ctxkey.RequestedPublicModel, route.RequestedModel)
}

// CatalogRouteFromContext 取出准入挂上的目录路由。
func CatalogRouteFromContext(ctx context.Context) (CatalogRoute, bool) {
	if ctx == nil {
		return CatalogRoute{}, false
	}
	route, ok := ctx.Value(ctxkey.CatalogRoute).(CatalogRoute)
	return route, ok
}

// catalogVendorPlatforms 把目录 vendor（LiteLLM 的 provider 串）映射到网关族。
// 表里的键来自当前播种结果（openai / anthropic / gemini / xai / deepseek / moonshot /
// zhipu / minimax / bedrock / text-completion-openai），vertex_ai-* 与 azure* 按前缀归族。
var catalogVendorPlatforms = map[string]string{
	"anthropic":              PlatformAnthropic,
	"bedrock":                PlatformAnthropic,
	"openai":                 PlatformOpenAI,
	"text-completion-openai": PlatformOpenAI,
	"gemini":                 PlatformGemini,
	"google":                 PlatformGemini,
	"xai":                    PlatformGrok,
	"moonshot":               PlatformKimi,
	"kimi":                   PlatformKimi,
	"zhipu":                  PlatformZhipu,
	"zai":                    PlatformZhipu,
	"deepseek":               PlatformDeepseek,
	"minimax":                PlatformMiniMax,
	"opencode":               PlatformOpenCodeGo,
	"opencode_go":            PlatformOpenCodeGo,
}

// CatalogRoutePlatform 返回条目走哪条网关族：显式 RoutePlatform > Vendor 映射 >
// DetectModelPlatform(ModelID) > Protocols（含 anthropic 归 anthropic，只含 gemini 归
// gemini，其余归 openai）。返回值绝不为空。
func CatalogRoutePlatform(entry *ModelCatalogEntry) string {
	if entry == nil {
		return PlatformOpenAI
	}
	if rp := strings.ToLower(strings.TrimSpace(entry.RoutePlatform)); rp != "" {
		return rp
	}
	vendor := strings.ToLower(strings.TrimSpace(entry.Vendor))
	if platform, ok := catalogVendorPlatforms[vendor]; ok {
		return platform
	}
	switch {
	case strings.HasPrefix(vendor, "vertex_ai"):
		return PlatformGemini
	case strings.HasPrefix(vendor, "azure"):
		return PlatformOpenAI
	}
	if platform, ok := DetectModelPlatform(entry.ModelID); ok {
		return platform
	}
	hasGemini := false
	for _, protocol := range entry.Protocols {
		switch protocol {
		case ModelCatalogProtocolAnthropic:
			return PlatformAnthropic
		case ModelCatalogProtocolGemini:
			hasGemini = true
		}
	}
	if hasGemini && len(entry.Protocols) == 1 {
		return PlatformGemini
	}
	return PlatformOpenAI
}

// ResolveRoute 准入用：模型名（精确 / 别名 / 通配别名 / Codex 归一化）命中 listed 条目
// 才返回 true。
func (s *ModelCatalogService) ResolveRoute(ctx context.Context, model string) (CatalogRoute, bool) {
	entry := s.LookupPricingEntry(ctx, model)
	if entry == nil || entry.Status != ModelCatalogStatusListed {
		return CatalogRoute{}, false
	}
	return CatalogRoute{
		EntryID:        entry.ID,
		CanonicalModel: entry.ModelID,
		RequestedModel: strings.TrimSpace(model),
		Platform:       CatalogRoutePlatform(entry),
		Entry:          entry,
	}, true
}

// ListListedEntries 返回用户可见 / 可调用的条目（快照副本，按模型标识排序）。
func (s *ModelCatalogService) ListListedEntries(ctx context.Context) []ModelCatalogEntry {
	if s == nil {
		return nil
	}
	snapshot := s.loadSnapshot(ctx)
	if snapshot == nil {
		return nil
	}
	listed := make([]ModelCatalogEntry, 0, len(snapshot.entries))
	for i := range snapshot.entries {
		if snapshot.entries[i].Status == ModelCatalogStatusListed {
			listed = append(listed, *snapshot.entries[i].Clone())
		}
	}
	sort.SliceStable(listed, func(i, j int) bool { return listed[i].ModelID < listed[j].ModelID })
	return listed
}

// CatalogListingSource 是用户可见模型列表的来源；ModelCatalogService 满足它。
type CatalogListingSource interface {
	ListListedEntries(ctx context.Context) []ModelCatalogEntry
	ResolveRoute(ctx context.Context, model string) (CatalogRoute, bool)
}

// FilterListedModelIDs 保序保留能解析到 listed 条目的 ID（含别名）。
func FilterListedModelIDs(ctx context.Context, src CatalogListingSource, ids []string) []string {
	kept := make([]string, 0, len(ids))
	for _, id := range ids {
		if IsListedModel(ctx, src, id) {
			kept = append(kept, id)
		}
	}
	return kept
}

// IsListedModel 报告模型名能否解析到 listed 条目；Gemini 的 "models/xxx" 由调用方先去前缀。
func IsListedModel(ctx context.Context, src CatalogListingSource, model string) bool {
	_, ok := src.ResolveRoute(ctx, model)
	return ok
}

// catalogBindingInboundProtocols 是绑定校验时逐个试的入站协议。
var catalogBindingInboundProtocols = []string{
	APIProtocolAnthropic, APIProtocolChatCompletions, APIProtocolResponses, APIProtocolGemini,
}

// AccountServesCatalogEntry 绑定前检查资源能否承接该条目至少一种入站协议：
// 第三方 key 看它在条目网关族上有没有可用的上游地址；成品号看厂商与网关族的关系。
func AccountServesCatalogEntry(entry *ModelCatalogEntry, account *Account) error {
	if entry == nil || account == nil {
		return infraerrors.BadRequest("CATALOG_BINDING_UNSERVABLE", "entry and account are required")
	}
	rp := CatalogRoutePlatform(entry)
	if account.IsThirdPartyKey() {
		for _, inbound := range catalogBindingInboundProtocols {
			if account.KeyUpstreamProtocolFor(rp, inbound) != "" {
				return nil
			}
		}
		return infraerrors.BadRequest("CATALOG_BINDING_UNSERVABLE",
			fmt.Sprintf("account %d has no upstream address usable on the %s gateway", account.ID, rp))
	}
	if subscriptionServesRoutePlatform(account.Vendor(), rp) {
		return nil
	}
	return infraerrors.BadRequest("CATALOG_BINDING_UNSERVABLE",
		fmt.Sprintf("account %d (%s) cannot serve models on the %s gateway", account.ID, account.Vendor(), rp))
}

// subscriptionServesRoutePlatform 报告成品号厂商能否在该网关族上承接请求：
// anthropic 成品号只走 Anthropic 族；antigravity 走 Anthropic 与 Gemini 族；gemini 成品号
// 只走 Gemini 族；OpenAI 族成品号（openai / grok / 国产）要求族内平台精确一致。
func subscriptionServesRoutePlatform(vendor, routePlatform string) bool {
	switch vendor {
	case PlatformAnthropic:
		return routePlatform == PlatformAnthropic
	case PlatformAntigravity:
		return routePlatform == PlatformAnthropic || routePlatform == PlatformGemini
	case PlatformGemini:
		return routePlatform == PlatformGemini
	default:
		return IsOpenAIGatewayPlatform(routePlatform) && NormalizeOpenAICompatiblePlatform(routePlatform) == vendor
	}
}
