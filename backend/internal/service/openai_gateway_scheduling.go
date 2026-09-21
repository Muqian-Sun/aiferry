package service

// 本文件由 openai_gateway_service.go 纯移动拆分而来：粘性会话哈希、账号选择与
// 负载感知调度、配额自动暂停判定、并发槽位获取。仅做代码搬迁，无任何行为变更。

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	openCodeSessionAffinityHeader = "X-Session-Affinity"
	openCodeSessionIDHeader       = "X-Session-Id"
	openCodeNativeSessionHeader   = "X-OpenCode-Session"
	codeBuddyConversationHeader   = "X-Conversation-ID"
)

var explicitOpenAIHeaderSessionNames = []string{
	"session-id",
	"session_id",
	"conversation_id",
	openCodeSessionAffinityHeader,
	openCodeSessionIDHeader,
	openCodeNativeSessionHeader,
	codeBuddyConversationHeader,
}

// explicitOpenAIHeaderSessionID resolves stable conversation identifiers sent
// by OpenAI-compatible clients. Keep this list limited to session-scoped
// fields: request/message IDs rotate every turn and would defeat sticky routing
// and upstream prompt caching.
func explicitOpenAIHeaderSessionID(c *gin.Context) string {
	if c == nil {
		return ""
	}

	for _, header := range explicitOpenAIHeaderSessionNames {
		if sessionID := strings.TrimSpace(c.GetHeader(header)); sessionID != "" {
			return sessionID
		}
	}
	return ""
}

// ExtractSessionID extracts the raw session ID from headers or body without hashing.
// Used by ForwardAsAnthropic to pass as prompt_cache_key for upstream cache.
func (s *OpenAIGatewayService) ExtractSessionID(c *gin.Context, body []byte) string {
	return explicitOpenAIRequestSessionID(c, body)
}

func explicitOpenAISessionID(c *gin.Context, body []byte) string {
	if c == nil {
		return ""
	}

	sessionID := explicitOpenAIHeaderSessionID(c)
	if sessionID == "" && len(body) > 0 {
		sessionID = strings.TrimSpace(openAIRequestPayloadView(body).Get("prompt_cache_key").String())
	}
	return sessionID
}

// openAIRequestPayloadView unwraps Responses WebSocket event envelopes while
// leaving ordinary HTTP objects untouched even when they contain a response
// field for another purpose.
func openAIRequestPayloadView(body []byte) gjson.Result {
	root := parseRawJSONView(body)
	eventType := strings.ToLower(strings.TrimSpace(root.Get("type").String()))
	if strings.HasPrefix(eventType, "response.") {
		if response := root.Get("response"); response.Exists() && response.IsObject() {
			return response
		}
	}
	return root
}

// explicitOpenAIRequestSessionID extends the common OpenAI session signals
// with Grok's native conversation header only for requests authenticated to a
// Grok group. This keeps an unrelated x-grok-conv-id header from changing
// scheduling or upstream session behavior for non-Grok groups.
//
// For Grok groups only, previous_response_id is a last-resort sticky seed so
// multi-turn Responses chains stay on the same OAuth account when no explicit
// session/conversation/prompt_cache_key is present. Non-Grok groups omit this
// so HTTP OpenAI paths that delete previous_response_id before upstream are
// unchanged.
func explicitOpenAIRequestSessionID(c *gin.Context, body []byte) string {
	if c == nil {
		return ""
	}

	sessionID := explicitOpenAIHeaderSessionID(c)
	if sessionID == "" && isGrokRequestContext(c) {
		sessionID = strings.TrimSpace(c.GetHeader(grokConversationIDHeader))
	}
	if sessionID == "" && len(body) > 0 {
		sessionID = strings.TrimSpace(openAIRequestPayloadView(body).Get("prompt_cache_key").String())
	}
	if sessionID == "" && isGrokRequestContext(c) && len(body) > 0 {
		sessionID = grokPreviousResponseSessionSeed(body)
	}
	return sessionID
}

// grokPreviousResponseSessionSeed returns a stable sticky seed from a Responses
// previous_response_id. Only resp_* response ids are accepted; message ids and
// unknown shapes must not pin sticky routing or prompt-cache identity.
func grokPreviousResponseSessionSeed(body []byte) string {
	id := strings.TrimSpace(gjson.GetBytes(body, "previous_response_id").String())
	if id == "" {
		return ""
	}
	if ClassifyOpenAIPreviousResponseIDKind(id) != OpenAIPreviousResponseIDKindResponseID {
		return ""
	}
	// Namespace so content-derived seeds never collide with response ids.
	return "grok-prev-resp:" + id
}

// GenerateExplicitSessionHash generates a sticky-session hash only from explicit
// client session signals. It intentionally skips content-derived fallback and is
// used by stateless endpoints such as /v1/images.
func (s *OpenAIGatewayService) GenerateExplicitSessionHash(c *gin.Context, body []byte) string {
	sessionID := explicitOpenAIRequestSessionID(c, body)
	if sessionID == "" {
		return ""
	}

	currentHash, legacyHash := deriveOpenAISessionHashes(sessionID)
	attachOpenAILegacySessionHashToGin(c, legacyHash)
	return currentHash
}

// GenerateSessionHash generates a sticky-session hash for OpenAI requests.
//
// Priority:
//  1. Header: session-id / session_id
//  2. Header: conversation_id
//  3. Header: x-session-affinity / x-session-id / x-opencode-session (OpenCode)
//  4. Header: x-conversation-id (CodeBuddy)
//  5. Header: x-grok-conv-id (Grok groups only)
//  6. Body:   prompt_cache_key
//  7. Body:   content-based fallback (model + system + tools + first user message)
//
// Grok sticky affinity is intentionally separate from the upstream
// prompt_cache_key identity (resolveGrokCacheIdentity): sticky pins an OAuth
// account for multi-turn routing, while the cache identity is tenant+model
// isolated for xAI server-side prompt cache. For Grok groups we scope the
// sticky seed with the client-requested model so switching models does not
// inherit a stale account binding (grok2api affinityKey pattern).
func (s *OpenAIGatewayService) GenerateSessionHash(c *gin.Context, body []byte) string {
	if c == nil {
		return ""
	}

	sessionID := explicitOpenAIRequestSessionID(c, body)
	if sessionID == "" && len(body) > 0 {
		sessionID = deriveOpenAIContentSessionSeed(body)
	}
	if sessionID == "" {
		return ""
	}

	if isGrokRequestContext(c) {
		sessionID = grokStickyAffinitySeed(sessionID, body)
	}

	currentHash, legacyHash := deriveOpenAISessionHashes(sessionID)
	attachOpenAILegacySessionHashToGin(c, legacyHash)
	return currentHash
}

// grokStickyAffinitySeed scopes sticky routing by model without changing the
// upstream prompt_cache_key written by applyGrokResponsesCacheIdentity.
func grokStickyAffinitySeed(sessionID string, body []byte) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ""
	}
	model := ""
	if len(body) > 0 {
		model = strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "model").String()))
	}
	if model == "" {
		return "grok-affinity:v1:" + sessionID
	}
	return "grok-affinity:v1:" + model + ":" + sessionID
}

// GenerateSessionHashWithFallback 先按常规信号生成会话哈希；
// 当未携带 session_id/conversation_id/prompt_cache_key 时，使用 fallbackSeed 生成稳定哈希。
// 该方法用于 WS ingress，避免会话信号缺失时发生跨账号漂移。
func (s *OpenAIGatewayService) GenerateSessionHashWithFallback(c *gin.Context, body []byte, fallbackSeed string) string {
	sessionHash := s.GenerateSessionHash(c, body)
	if sessionHash != "" {
		return sessionHash
	}

	seed := strings.TrimSpace(fallbackSeed)
	if seed == "" {
		return ""
	}

	currentHash, legacyHash := deriveOpenAISessionHashes(seed)
	attachOpenAILegacySessionHashToGin(c, legacyHash)
	return currentHash
}

func resolveOpenAIUpstreamOriginator(c *gin.Context, isOfficialClient bool) string {
	if c != nil {
		if originator := strings.TrimSpace(c.GetHeader("originator")); originator != "" {
			return originator
		}
	}
	if isOfficialClient {
		return openai.CodexDefaultOriginator
	}
	return "opencode"
}

// NormalizeOpenAICompatiblePlatform 保留 grok 与国产 OpenAI 兼容供应商（kimi/zhipu/
// deepseek）的原值，其他值一律归一为 openai。调度器据此对账号与请求做精确平台匹配：
// kimi 分组请求只命中 kimi 账号，语义与 openai/grok 一致。
// （upstream 曾将本函数改为未导出 normalizeOpenAICompatiblePlatform，本分支的
// handler 调度入口仍需导出，保持导出名。）
func NormalizeOpenAICompatiblePlatform(platform string) string {
	switch platform {
	case PlatformGrok, PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax, PlatformOpenCodeGo:
		return platform
	default:
		return PlatformOpenAI
	}
}

// openAICompactSupportTier classifies an OpenAI-compatible account by compact capability.
// 0 = explicitly unsupported, 1 = unknown / not yet probed, 2 = explicitly supported.
func openAICompactSupportTier(account *Account) int {
	if account == nil {
		return 0
	}
	if account.IsThirdPartyKey() {
		// /responses/compact 是 Responses 协议的端点，不能转换成别的协议：没有 responses 地址的 key 不支持。
		if account.ProtocolEndpoint(APIProtocolResponses) == "" {
			return 0
		}
		vendor := account.Vendor()
		if vendor == PlatformGrok {
			// xAI 官方原生支持（与 Grok 成品号一致）。
			return 2
		}
		// 官方 OpenAI 与通用中转：按手动开关 / 探测结果分级，未知时保留为候选；其余厂商不支持。
		if !openAIProtocolFeaturesApplyToVendor(vendor, true) {
			return 0
		}
	} else {
		if account.IsGrok() {
			return 2
		}
		if !account.IsOpenAI() {
			return 0
		}
	}
	supported, known := account.OpenAICompactSupportKnown()
	if !known {
		return 1
	}
	if supported {
		return 2
	}
	return 0
}

func resolveAccountExtraNumber(extra map[string]any, keys ...string) (float64, bool) {
	if len(extra) == 0 {
		return 0, false
	}
	for _, key := range keys {
		value, ok := extra[key]
		if !ok || value == nil {
			continue
		}
		switch v := value.(type) {
		case float64:
			return v, true
		case float32:
			return float64(v), true
		case int:
			return float64(v), true
		case int64:
			return float64(v), true
		case json.Number:
			parsed, err := v.Float64()
			if err == nil {
				return parsed, true
			}
		case string:
			parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

// resolveOpenAIQuotaUtilization returns the current utilization ratio (0..1) for the
// given Codex usage window. ok=false means there is no usable signal to pause on:
// either no snapshot exists, or the window has already rolled over so the cached
// percentage is stale. The stale guard matters because a paused account stops
// receiving requests, so its snapshot is never refreshed from upstream headers —
// without this check an old used_percent would keep the account paused forever even
// after the real window reset.
func resolveOpenAIQuotaUtilization(extra map[string]any, window string, now time.Time) (float64, bool) {
	usedPercent := readOpenAIQuotaUsedPercent(extra, window)
	if usedPercent <= 0 {
		return 0, false
	}
	if openAIQuotaWindowReset(extra, window, now) {
		return 0, false
	}
	// 快照过于陈旧（账号长期未收到流量刷新）时，不再据此暂停。放行后下一次响应头
	// 会刷新快照实现自愈，避免账号在错误/过期的 used% 上被永久跳过（issue #2994）。
	if openAICodexSnapshotStaleForPause(extra, now) {
		return 0, false
	}
	return usedPercent / 100, true
}

// openAICodexSnapshotStaleForPause reports whether the Codex usage snapshot is stale
// enough that it should no longer keep an account auto-paused. It anchors on
// codex_usage_updated_at (always written by buildCodexUsageExtraUpdates). A missing or
// unparseable timestamp returns false (treated as fresh, so the account stays paused) —
// this is deliberate: it prevents any snapshot without a write time from silently escaping
// auto-pause, and a genuinely-exhausted account that is actively served refreshes the
// timestamp on every response so it never crosses the staleness bound.
func openAICodexSnapshotStaleForPause(extra map[string]any, now time.Time) bool {
	if len(extra) == 0 {
		return false
	}
	updatedRaw, ok := extra["codex_usage_updated_at"]
	if !ok {
		return false
	}
	updatedAt, err := parseTime(fmt.Sprint(updatedRaw))
	if err != nil {
		return false
	}
	return now.Sub(updatedAt) >= openAICodexAutoPauseStaleAfter
}

// openAIQuotaWindowReset reports whether the Codex usage window's reset time has
// already passed relative to now. It prefers the absolute codex_<window>_reset_at
// timestamp and falls back to codex_<window>_reset_after_seconds anchored at
// codex_usage_updated_at, mirroring AccountUsageService's window-progress logic.
func openAIQuotaWindowReset(extra map[string]any, window string, now time.Time) bool {
	resetAt, ok := openAICodexWindowResetAt(extra, window)
	return ok && !now.Before(resetAt)
}

// 绝对时间优先；相对倒计时必须锚定快照采样时间，不能随每次评分向后滑动。
func openAICodexWindowResetAt(extra map[string]any, window string) (time.Time, bool) {
	if len(extra) == 0 {
		return time.Time{}, false
	}
	if resetAtRaw, ok := extra["codex_"+window+"_reset_at"]; ok {
		if resetAt, err := parseTime(fmt.Sprint(resetAtRaw)); err == nil {
			return resetAt, true
		}
	}
	resetAfter := parseExtraInt(extra["codex_"+window+"_reset_after_seconds"])
	if resetAfter <= 0 {
		return time.Time{}, false
	}
	updatedAt, err := parseTime(fmt.Sprint(extra["codex_usage_updated_at"]))
	if err != nil {
		return time.Time{}, false
	}
	return updatedAt.Add(time.Duration(resetAfter) * time.Second), true
}

func readOpenAIQuotaUsedPercent(extra map[string]any, window string) float64 {
	if len(extra) == 0 {
		return 0
	}
	if value, ok := resolveAccountExtraNumber(extra, "codex_"+window+"_used_percent"); ok {
		return value
	}
	return 0
}

// resolveOpenAIAccountUpstreamModelForRequest resolves the upstream model that
// would be sent for a given request, honoring the legacy compact-only mapping
// when the caller is on the /responses/compact path.
func resolveOpenAIAccountUpstreamModelForRequest(account *Account, requestedModel string, requireCompact bool) string {
	// Forward checks the raw Chat Completions fallback before passthrough.
	// These API-key accounts therefore apply normal account model_mapping and
	// upstream normalization, but never compact_model_mapping.
	if shouldForwardOpenAIResponsesViaRawChatCompletions(account) {
		upstreamModel := resolveOpenAIForwardModel(account, requestedModel, "")
		return normalizeOpenAIModelForUpstream(account, upstreamModel)
	}

	// Passthrough accounts only replace authentication. Their Forward path
	// keeps the channel-mapped model in the request body and does not apply the
	// account's normal model_mapping. Legacy /responses/compact is the one
	// exception: forwardOpenAIPassthrough applies compact_model_mapping
	// directly to that channel-mapped model.
	if account != nil && account.IsOpenAIPassthroughEnabled() {
		upstreamModel := strings.TrimSpace(requestedModel)
		if upstreamModel == "" {
			return ""
		}
		if requireCompact {
			return resolveOpenAICompactForwardModel(account, upstreamModel)
		}
		return upstreamModel
	}

	// Compact mappings are keyed by the client-visible model. Prefer an exact
	// compact rule before ordinary account mapping; otherwise a normal alias can
	// hide the compact-specific rule and make scheduling disagree with Forward.
	if requireCompact && account != nil {
		if compactModel, matched := account.ResolveCompactMappedModel(strings.TrimSpace(requestedModel)); matched {
			if compactModel = strings.TrimSpace(compactModel); compactModel != "" {
				return compactModel
			}
		}
	}

	upstreamModel := resolveOpenAIForwardModel(account, requestedModel, "")
	if upstreamModel == "" {
		return ""
	}
	if requireCompact {
		compactModel := resolveOpenAICompactForwardModel(account, upstreamModel)
		if compactModel != upstreamModel {
			return compactModel
		}
	}
	return normalizeOpenAIModelForUpstream(account, upstreamModel)
}

// ResolveOpenAIAccountUpstreamModelForRequest exposes the scheduler's exact
// account mapping chain to handler-side outcome reporting.
func ResolveOpenAIAccountUpstreamModelForRequest(account *Account, requestedModel string, requireCompact bool) string {
	return resolveOpenAIAccountUpstreamModelForRequest(account, requestedModel, requireCompact)
}

// resolveOpenAIForwardMappedModels is the shared account mapping chain for
// Forward callers. billingModel retains the ordinary mapping used for usage
// accounting, while upstreamModel is the model the scheduler has admitted.
func resolveOpenAIForwardMappedModels(account *Account, requestedModel string, requireCompact bool) (billingModel, upstreamModel string) {
	requestedModel = strings.TrimSpace(requestedModel)
	if account != nil && account.IsOpenAIPassthroughEnabled() {
		billingModel = requestedModel
	} else if account != nil {
		billingModel = strings.TrimSpace(account.GetMappedModel(requestedModel))
	}
	if billingModel == "" {
		billingModel = requestedModel
	}
	upstreamModel = resolveOpenAIAccountUpstreamModelForRequest(account, requestedModel, requireCompact)
	if strings.TrimSpace(upstreamModel) == "" {
		upstreamModel = billingModel
	}
	return billingModel, upstreamModel
}

func resolveOpenAIErrorSchedulingModel(billingModel, upstreamModel string) string {
	if upstreamModel = strings.TrimSpace(upstreamModel); upstreamModel != "" {
		return upstreamModel
	}
	return strings.TrimSpace(billingModel)
}

// parentAccountLookup 返回供 parentHealthyForShadow 使用的母账号解析闭包:经 accountRepo
// 按 ID 取当前 Account(repo 为空时 fail-closed 返回 nil)。统一调度/粘连各路径的母账号解析,
// 取代各调用点重复内联的同一闭包(历史上 recheck 等路径还漏写过 accountRepo==nil 守卫)。
// L2 候选循环改用带 per-pass 缓存的 parentLookupL2,不走此方法。
func (s *OpenAIGatewayService) parentAccountLookup(ctx context.Context) func(int64) *Account {
	return func(id int64) *Account {
		if s.accountRepo == nil {
			return nil
		}
		a, _ := s.accountRepo.GetByID(ctx, id)
		return a
	}
}

func (s *OpenAIGatewayService) getSchedulableAccount(ctx context.Context, accountID int64) (*Account, error) {
	var (
		account *Account
		err     error
	)
	if s.schedulerSnapshot != nil {
		account, err = s.schedulerSnapshot.GetAccount(ctx, accountID)
	} else {
		account, err = s.accountRepo.GetByID(ctx, accountID)
	}
	if err != nil || account == nil {
		return account, err
	}
	return account, nil
}
