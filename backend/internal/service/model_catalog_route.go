package service

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// CatalogRoute 是准入后挂在 request.Context 上的「本次请求命中了哪个目录条目」。
type CatalogRoute struct {
	EntryID        int64
	CanonicalModel string             // entry.ModelID，计费与用量记录用
	RequestedModel string             // 客户端写的名字，调度与上游转发继续用它
	Entry          *ModelCatalogEntry // 快照指针，只读
}

// WithCatalogRoute 把目录路由与客户端原始模型名挂到 ctx 上。条目没有「网关族」：调度按协议
// （accountServesCatalogRoute），扩展端点分发与厂商特有处理读 RequestVendorPlatform。
func WithCatalogRoute(ctx context.Context, route CatalogRoute) context.Context {
	ctx = context.WithValue(ctx, ctxkey.CatalogRoute, route)
	return context.WithValue(ctx, ctxkey.RequestedPublicModel, route.RequestedModel)
}

// RequestedPublicModelFromContext 客户端原始请求的公开模型名（准入时随目录路由挂上，可能是条目别名）。
func RequestedPublicModelFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	model, ok := ctx.Value(ctxkey.RequestedPublicModel).(string)
	model = strings.TrimSpace(model)
	if !ok || model == "" {
		return "", false
	}
	return model, true
}

// CatalogRouteFromContext 取出准入挂上的目录路由。
func CatalogRouteFromContext(ctx context.Context) (CatalogRoute, bool) {
	if ctx == nil {
		return CatalogRoute{}, false
	}
	route, ok := ctx.Value(ctxkey.CatalogRoute).(CatalogRoute)
	return route, ok
}

// catalogVendorPlatforms 把目录 vendor（LiteLLM 的 provider 串）映射到平台常量。
// 表里的键来自当前播种结果（openai / anthropic / gemini / xai / deepseek / moonshot /
// zhipu / minimax / bedrock / text-completion-openai），vertex_ai-* 与 azure* 按前缀归类。
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

// CatalogVendorPlatform 返回条目厂商对应的平台常量：catalogVendorPlatforms 精确表 >
// vertex_ai* 归 gemini、azure* 归 openai；其余（含空厂商）返回空串——目录不猜模型名，
// 没有厂商就没有厂商特有处理。
func CatalogVendorPlatform(entry *ModelCatalogEntry) string {
	if entry == nil {
		return ""
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
	return ""
}

// RequestVendorPlatform 本次请求的厂商平台：目录路由按条目厂商（CatalogVendorPlatform）；
// 无路由（无模型端点）没有厂商。只给扩展端点分发（routes）与厂商特有处理（grok 传输 / 缓存身份、
// 错误透传规则平台、运维日志平台）读；调度与准入不看它。
func RequestVendorPlatform(ctx context.Context) (string, bool) {
	route, ok := CatalogRouteFromContext(ctx)
	if !ok {
		return "", false
	}
	if platform := CatalogVendorPlatform(route.Entry); platform != "" {
		return platform, true
	}
	return "", false
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

// IsVisibleModel 报告模型对这把 key 可见：上架，且（订阅 key 时）在套餐模型集里。
// 余额 key 传 nil 订阅，退化成 IsListedModel。
func IsVisibleModel(ctx context.Context, src CatalogListingSource, sub *UserSubscription, model string) bool {
	route, ok := src.ResolveRoute(ctx, model)
	if !ok {
		return false
	}
	return SubscriptionCoversRoute(sub, route)
}

// catalogBindingInboundProtocols 是绑定校验时逐个试的入站协议。
var catalogBindingInboundProtocols = []string{
	APIProtocolAnthropic, APIProtocolChatCompletions, APIProtocolResponses, APIProtocolGemini,
}

// CatalogBindingServes 报告资源在四个入站协议上能否承接：只看协议转换注册表
// （ServesInbound），key 与成品号同一条规则。绑定校验与诊断接口都读它。
func CatalogBindingServes(account *Account) map[string]bool {
	serves := make(map[string]bool, len(catalogBindingInboundProtocols))
	for _, inbound := range catalogBindingInboundProtocols {
		serves[inbound] = account.ServesInbound(inbound)
	}
	return serves
}

// AccountServesCatalogEntry 绑定前检查资源能否承接该条目至少一种入站协议。
func AccountServesCatalogEntry(entry *ModelCatalogEntry, account *Account) error {
	if entry == nil || account == nil {
		return infraerrors.BadRequest("CATALOG_BINDING_UNSERVABLE", "entry and account are required")
	}
	for _, ok := range CatalogBindingServes(account) {
		if ok {
			return nil
		}
	}
	return infraerrors.BadRequest("CATALOG_BINDING_UNSERVABLE",
		fmt.Sprintf("account %d has no upstream protocol that can serve %s", account.ID, entry.ModelID))
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

// catalogExtensionEndpointPriceFileModes 是价格文件里「经扩展端点调用」的模型 mode：
// 生图走 /v1/images、向量走 /v1/embeddings。
var catalogExtensionEndpointPriceFileModes = map[string]bool{
	"image_generation": true,
	"embedding":        true,
}

// CatalogEntryServedByExtensionEndpoints 报告条目是否经扩展端点（/v1/images、/v1/videos、
// /v1/embeddings，入站协议为空）承接。扩展端点只按 openai / grok 厂商分流（routes/gateway.go），
// 其余厂商的条目恒为 false——Gemini 出图模型走对话入口。判据是图片 / 视频计费，或价格文件把
// 模型标成生图 / 向量模型（gpt-image-* 在目录里按 token 计价，只能靠这一条认出来）。
func CatalogEntryServedByExtensionEndpoints(entry *ModelCatalogEntry, priceFileMode string) bool {
	switch CatalogVendorPlatform(entry) {
	case PlatformOpenAI, PlatformGrok:
	default:
		return false
	}
	switch entry.EffectiveBillingMode() {
	case BillingModeImage, BillingModeVideo:
		return true
	}
	return catalogExtensionEndpointPriceFileModes[strings.ToLower(strings.TrimSpace(priceFileMode))]
}

// AccountServesCatalogExtensionEndpoints 绑定前检查资源能否承接扩展端点：与调度同一条规则
// （ServesInbound("")）——第三方 key 必须配 chat_completions 地址，成品号看厂商。
func AccountServesCatalogExtensionEndpoints(entry *ModelCatalogEntry, account *Account) error {
	if account.ServesInbound("") {
		return nil
	}
	return infraerrors.BadRequest("CATALOG_BINDING_UNSERVABLE",
		fmt.Sprintf("account %d cannot serve %s: image / video / embedding models are called through the extension endpoints, "+
			"and a third-party key serves those only when it has a chat_completions endpoint", account.ID, entry.ModelID))
}

// priceFileMode 返回价格文件里该模型的 mode（确定性识别，不按子串猜）；没有价格服务或识别不到时为空。
func (s *ModelCatalogService) priceFileMode(modelID string) string {
	if pricing := s.seedInput.PricingService.GetIdentifiedModelPricing(modelID); pricing != nil {
		return pricing.Mode
	}
	return ""
}

// ReplaceBindings 用整份列表覆盖条目的资源绑定：先确认条目存在，再逐个取账号并检查
// 它能承接条目（AccountServesCatalogEntry；经扩展端点调用的条目还要能承接扩展端点），
// 全部通过才写库，然后失效快照。
func (s *ModelCatalogService) ReplaceBindings(ctx context.Context, entryID int64, bindings []ModelCatalogBinding, accounts CatalogBindingAccountSource) error {
	if s == nil || s.repo == nil {
		return ErrModelCatalogEntryNotFound
	}
	entry, err := s.repo.GetEntryByID(ctx, entryID)
	if err != nil {
		return err
	}
	extensionEndpoints := CatalogEntryServedByExtensionEndpoints(entry, s.priceFileMode(entry.ModelID))
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
		if extensionEndpoints {
			if err := AccountServesCatalogExtensionEndpoints(entry, account); err != nil {
				return err
			}
		}
		normalized = append(normalized, ModelCatalogBinding{EntryID: entryID, AccountID: binding.AccountID, Priority: binding.Priority})
	}
	if err := s.repo.ReplaceBindings(ctx, entryID, normalized); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}

// ListAccountEntryIDs 返回账号被哪些目录条目绑定（渠道表单里「承接的模型」）。
func (s *ModelCatalogService) ListAccountEntryIDs(ctx context.Context, accountID int64, accounts CatalogBindingAccountSource) ([]int64, error) {
	if s == nil || s.repo == nil {
		return nil, ErrModelCatalogEntryNotFound
	}
	if _, err := accounts.GetAccount(ctx, accountID); err != nil {
		return nil, err
	}
	return s.repo.ListEntryIDsByAccount(ctx, accountID)
}

// ReplaceAccountBindings 用整份条目列表覆盖账号承接的目录模型：逐个确认条目存在、账号能承接
// （与 ReplaceBindings 同一套校验：AccountServesCatalogEntry，经扩展端点调用的条目还要能承接扩展端点），
// 全部通过才写库，然后失效快照。
func (s *ModelCatalogService) ReplaceAccountBindings(ctx context.Context, accountID int64, entryIDs []int64, accounts CatalogBindingAccountSource) error {
	if s == nil || s.repo == nil {
		return ErrModelCatalogEntryNotFound
	}
	account, err := accounts.GetAccount(ctx, accountID)
	if err != nil {
		return err
	}
	seen := make(map[int64]struct{}, len(entryIDs))
	normalized := make([]int64, 0, len(entryIDs))
	for _, entryID := range entryIDs {
		if _, dup := seen[entryID]; dup {
			return catalogValidationError(fmt.Sprintf("duplicate entry %d in bindings", entryID))
		}
		seen[entryID] = struct{}{}
		entry, err := s.repo.GetEntryByID(ctx, entryID)
		if err != nil {
			return err
		}
		if err := AccountServesCatalogEntry(entry, account); err != nil {
			return err
		}
		if CatalogEntryServedByExtensionEndpoints(entry, s.priceFileMode(entry.ModelID)) {
			if err := AccountServesCatalogExtensionEndpoints(entry, account); err != nil {
				return err
			}
		}
		normalized = append(normalized, entryID)
	}
	if err := s.repo.ReplaceAccountBindings(ctx, accountID, normalized); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}

// SchedulingScopeID 粘性会话、Responses 会话窗、Gemini 摘要会话的作用域：目录路由下是条目 ID，
// 无路由（无模型端点）为 0。
func SchedulingScopeID(ctx context.Context) int64 {
	if route, ok := CatalogRouteFromContext(ctx); ok {
		return route.EntryID
	}
	return 0
}

// accountInSchedulingScope 账号是否属于本次请求的调度池：目录路由看绑定；无路由的池是全部资源，恒真。
func accountInSchedulingScope(ctx context.Context, account *Account) bool {
	if account == nil {
		return false
	}
	if route, ok := CatalogRouteFromContext(ctx); ok {
		return slices.Contains(account.CatalogEntryIDs, route.EntryID)
	}
	return true
}

// SchedulingBlockedReason 返回账号此刻不可调度的类别（SchedulingState.Reason），可调度时返回空串。
// 诊断接口用它解释「绑了却选不到」。
func SchedulingBlockedReason(account *Account) string {
	return account.SchedulingState(time.Now()).Reason
}
