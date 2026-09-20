package service

import (
	"context"
	"fmt"
	"slices"
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

// WithCatalogRoute 把目录路由与客户端原始模型名挂到 ctx 上，并把条目的网关族写成本次请求的
// 目标平台：handler 族分发（routes）、选号（resolvePlatform）、错误透传平台都先读这把钥匙，
// 带模型的请求从此按条目路由，不看分组平台。
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

// CatalogRouteResolver 只需要 ResolveRoute；ModelCatalogService 满足它。
type CatalogRouteResolver interface {
	ResolveRoute(ctx context.Context, model string) (CatalogRoute, bool)
}

// ResolveCatalogRouteForCandidates 把一次请求里所有可能被下游绑定到的模型名（重复键、大小写变体、
// session.model）逐一解析：任一解析不到上架条目、或解析到不同条目，都返回 false 与第一个
// 出问题的候选名。HTTP 准入中间件与 Responses WS 逐帧准入共用这条规则。
func ResolveCatalogRouteForCandidates(ctx context.Context, resolver CatalogRouteResolver, candidates []string) (CatalogRoute, string, bool) {
	var route CatalogRoute
	resolved := false
	for _, candidate := range candidates {
		candidateRoute, ok := resolver.ResolveRoute(ctx, candidate)
		if !ok || (resolved && candidateRoute.EntryID != route.EntryID) {
			return CatalogRoute{}, candidate, false
		}
		route, resolved = candidateRoute, true
	}
	if !resolved {
		return CatalogRoute{}, "", false
	}
	return route, "", true
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

// CatalogRouteServes 报告账号在条目网关族上能承接哪些入站协议（选号用同一张矩阵，
// 见 accountServesCatalogRoute）。绑定校验与诊断接口都读它。
func CatalogRouteServes(entry *ModelCatalogEntry, account *Account) map[string]bool {
	rp := CatalogRoutePlatform(entry)
	serves := make(map[string]bool, len(catalogBindingInboundProtocols))
	for _, inbound := range catalogBindingInboundProtocols {
		serves[inbound] = accountServesCatalogRoute(account, rp, inbound)
	}
	return serves
}

// AccountServesCatalogEntry 绑定前检查资源能否承接该条目至少一种入站协议：
// 第三方 key 看它在条目网关族上有没有可用的上游地址；成品号看厂商 × 网关族的矩阵。
func AccountServesCatalogEntry(entry *ModelCatalogEntry, account *Account) error {
	if entry == nil || account == nil {
		return infraerrors.BadRequest("CATALOG_BINDING_UNSERVABLE", "entry and account are required")
	}
	for _, ok := range CatalogRouteServes(entry, account) {
		if ok {
			return nil
		}
	}
	rp := CatalogRoutePlatform(entry)
	if account.IsThirdPartyKey() {
		return infraerrors.BadRequest("CATALOG_BINDING_UNSERVABLE",
			fmt.Sprintf("account %d has no upstream address usable on the %s gateway", account.ID, rp))
	}
	return infraerrors.BadRequest("CATALOG_BINDING_UNSERVABLE",
		fmt.Sprintf("account %d (%s) cannot serve models on the %s gateway", account.ID, account.Vendor(), rp))
}

// CatalogBindingAccountSource 绑定校验时按 ID 取账号；AdminService 满足它。
type CatalogBindingAccountSource interface {
	GetAccount(ctx context.Context, id int64) (*Account, error)
}

// ListBindings 返回条目的资源绑定。
func (s *ModelCatalogService) ListBindings(ctx context.Context, entryID int64) ([]ModelCatalogBinding, error) {
	if s == nil || s.repo == nil {
		return nil, ErrModelCatalogEntryNotFound
	}
	if _, err := s.repo.GetEntryByID(ctx, entryID); err != nil {
		return nil, err
	}
	return s.repo.ListBindingsByEntry(ctx, entryID)
}

// ReplaceBindings 用整份列表覆盖条目的资源绑定：先确认条目存在，再逐个取账号并检查
// 它能承接条目（AccountServesCatalogEntry），全部通过才写库，然后失效快照。
func (s *ModelCatalogService) ReplaceBindings(ctx context.Context, entryID int64, bindings []ModelCatalogBinding, accounts CatalogBindingAccountSource) error {
	if s == nil || s.repo == nil {
		return ErrModelCatalogEntryNotFound
	}
	entry, err := s.repo.GetEntryByID(ctx, entryID)
	if err != nil {
		return err
	}
	seen := make(map[int64]struct{}, len(bindings))
	normalized := make([]ModelCatalogBinding, 0, len(bindings))
	for _, binding := range bindings {
		if _, dup := seen[binding.AccountID]; dup {
			return catalogValidationError(fmt.Sprintf("duplicate account %d in bindings", binding.AccountID))
		}
		seen[binding.AccountID] = struct{}{}
		account, err := accounts.GetAccount(ctx, binding.AccountID)
		if err != nil {
			return err
		}
		if err := AccountServesCatalogEntry(entry, account); err != nil {
			return err
		}
		normalized = append(normalized, ModelCatalogBinding{EntryID: entryID, AccountID: binding.AccountID, Priority: binding.Priority})
	}
	if err := s.repo.ReplaceBindings(ctx, entryID, normalized); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}

// SchedulingScopeID 粘性会话、Responses 会话窗、Gemini 摘要会话的作用域：目录路由下是条目 ID，
// 否则是分组 ID（未分组为 0）。条目 ID 与分组 ID 共用数字空间：撞上时成员判定
// （accountInSchedulingScope）会把不在池里的粘性账号判为未命中，只是多选一次号。
func SchedulingScopeID(ctx context.Context, groupID *int64) int64 {
	if route, ok := CatalogRouteFromContext(ctx); ok {
		return route.EntryID
	}
	return derefGroupID(groupID)
}

// accountInSchedulingScope 账号是否属于本次请求的调度池：目录路由看绑定，否则看分组
// （groupID 为 nil = 未分组账号）。
func accountInSchedulingScope(ctx context.Context, account *Account, groupID *int64) bool {
	if account == nil {
		return false
	}
	if route, ok := CatalogRouteFromContext(ctx); ok {
		return slices.Contains(account.CatalogEntryIDs, route.EntryID)
	}
	if groupID == nil {
		return len(account.AccountGroups) == 0 && len(account.GroupIDs) == 0
	}
	for _, id := range account.GroupIDs {
		if id == *groupID {
			return true
		}
	}
	for _, ag := range account.AccountGroups {
		if ag.GroupID == *groupID {
			return true
		}
	}
	return false
}
