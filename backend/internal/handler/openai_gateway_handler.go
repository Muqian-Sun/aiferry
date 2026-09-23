package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmodel"
	"github.com/Wei-Shaw/sub2api/internal/securityaudit"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// OpenAIGatewayHandler handles OpenAI API gateway requests
type OpenAIGatewayHandler struct {
	gatewayService             *service.OpenAIGatewayService
	billingCacheService        *service.BillingCacheService
	apiKeyService              *service.APIKeyService
	usageRecordWorkerPool      *service.UsageRecordWorkerPool
	errorPassthroughService    *service.ErrorPassthroughService
	contentModerationService   *service.ContentModerationService
	securityAuditCoordinator   *securityaudit.Coordinator
	grokMediaEligibilityProber grokMediaEligibilityProber
	opsService                 *service.OpsService
	concurrencyHelper          *ConcurrencyHelper
	imageLimiter               *ImageConcurrencyLimiter
	maxAccountSwitches         int
	cfg                        *config.Config
	// modelCatalog 用户可见模型列表的来源：Codex 清单只露出目录上架的 slug。
	modelCatalog service.CatalogListingSource
}

// openAIWSTurnModelSnapshot 记住上一 turn 的请求模型：换模型时要求连接账号也承接新模型。
type openAIWSTurnModelSnapshot struct {
	turn  int
	model string
}

func advanceOpenAIWSCyberBlockState(blocked, pending, marked bool, turnErr error) (bool, bool) {
	var failoverErr *service.UpstreamFailoverError
	isFailover := errors.As(turnErr, &failoverErr)
	if marked {
		if isFailover {
			return false, true
		}
		return true, false
	}
	if pending && !isFailover {
		return true, false
	}
	return blocked, pending
}

var errOpenAIWSUnsupportedModelSwitch = errors.New("selected account does not support websocket model switch")

func newOpenAIWSUnsupportedModelSwitchError(model string) error {
	cause := fmt.Errorf("%w: model %q", errOpenAIWSUnsupportedModelSwitch, strings.TrimSpace(model))
	return service.NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "model switch requires reconnect", cause)
}

func shouldReportOpenAIWSProxyAccountFailure(err error) bool {
	return err != nil && !errors.Is(err, errOpenAIWSUnsupportedModelSwitch) && !service.IsOpenAIWSSessionPreemptedError(err)
}

// openAIWSIngressEndedByClient reports whether a finished ingress WebSocket turn
// ended the way a healthy client ends one, rather than through an upstream or
// account fault.
//
// Three error shapes describe that same benign outcome and only the first was
// recognised:
//
//   - *service.OpenAIWSClientCloseError carrying 1000 — the gateway closing the
//     socket on its own terms, e.g. the inter-turn idle timeout.
//   - a bare coderws.CloseError{Code: 1000} — what coder/websocket returns when
//     the client closes cleanly. ReadOpenAIWSClientMessage hands conn.Read's
//     error back verbatim, so nothing ever wraps it into the type above and an
//     errors.As against that type cannot see it.
//   - context.Canceled — the client went away mid-turn. That path closes with
//     StatusGoingAway (1001) and carries the cancellation as its cause, so a
//     check for 1000 alone never matched it either.
//
// The last two fell through to shouldReportOpenAIWSProxyAccountFailure, which
// filters only model-switch and session-preemption errors. Everything else
// reaches ObserveOpenAIAPIKeyHealthFailure and scheduler.ReportResult(false), so
// a client that merely disconnected counted against the upstream account's
// health and could trip it out of scheduling.
//
// failoverClientGone already states the rule this restores for the HTTP failover
// path — a cancelled client context "被误报成账号耗尽" is a bug, not a signal —
// and summarizeWSCloseErrorForLog already reads the close code the correct way,
// which is why the resulting WARN printed close_status=1000(StatusNormalClosure)
// for an error that was, in the same breath, being charged to the account.
//
// Deliberately narrow. StatusGoingAway is not matched on its own: the gateway
// emits 1001 when it tears a session down for its own reasons too, and the
// client-cancellation case is already covered by context.Canceled.
// context.DeadlineExceeded is left out as well — the idle-timeout path wraps it
// in a 1000 close error and stays benign through the first check, while any
// other deadline is a genuine stall worth reporting.
func openAIWSIngressEndedByClient(err error) bool {
	if err == nil {
		return true
	}
	var closeErr *service.OpenAIWSClientCloseError
	if errors.As(err, &closeErr) && closeErr.StatusCode() == coderws.StatusNormalClosure {
		return true
	}
	if coderws.CloseStatus(err) == coderws.StatusNormalClosure {
		return true
	}
	return errors.Is(err, context.Canceled)
}

func openAIWSTurnBillingModel(result *service.OpenAIForwardResult, requestedModel, upstreamModel string) string {
	billingModel := ""
	if result != nil {
		billingModel = strings.TrimSpace(result.BillingModel)
	}
	if billingModel == "" {
		billingModel = strings.TrimSpace(upstreamModel)
	}
	if billingModel == "" {
		billingModel = strings.TrimSpace(requestedModel)
	}
	return billingModel
}

type grokMediaEligibilityProber interface {
	ProbeMediaEligibility(ctx context.Context, accountID int64) (bool, string, error)
}

const maxOpenAIFirstOutputTimeoutSwitches = 1

func openAIForwardSucceededForScheduling(result *service.OpenAIForwardResult) bool {
	return result.SucceededForScheduling()
}

func openAIAccountScheduleModel(c *gin.Context, account *service.Account, forwardModel string, requireCompact bool, result *service.OpenAIForwardResult) string {
	if result != nil {
		if actual := strings.TrimSpace(result.UpstreamModel); actual != "" {
			return actual
		}
	}
	if c != nil {
		if value, ok := c.Get(service.OpsUpstreamModelKey); ok {
			if actual, ok := value.(string); ok && strings.TrimSpace(actual) != "" {
				return strings.TrimSpace(actual)
			}
		}
	}
	return service.ResolveOpenAIAccountUpstreamModelForRequest(account, forwardModel, requireCompact)
}

func usageRecordContext(parent context.Context, base context.Context) context.Context {
	if base == nil {
		base = context.Background()
	}
	if parent == nil {
		return base
	}
	if clientRequestID, _ := parent.Value(ctxkey.ClientRequestID).(string); strings.TrimSpace(clientRequestID) != "" {
		base = context.WithValue(base, ctxkey.ClientRequestID, strings.TrimSpace(clientRequestID))
	}
	if requestID, _ := parent.Value(ctxkey.RequestID).(string); strings.TrimSpace(requestID) != "" {
		base = context.WithValue(base, ctxkey.RequestID, strings.TrimSpace(requestID))
	}
	return base
}

func wrapUsageRecordTaskContext(parent context.Context, task service.UsageRecordTask) service.UsageRecordTask {
	if task == nil {
		return nil
	}
	return func(ctx context.Context) {
		task(usageRecordContext(parent, ctx))
	}
}

func openAIResponsesRequiredCapability(imageIntent bool, platform string) service.OpenAIEndpointCapability {
	if imageIntent && platform == service.PlatformOpenAI {
		return service.OpenAIEndpointCapabilityResponses
	}
	return service.OpenAIEndpointCapabilityChatCompletions
}

// openAIResponsesRequiredCapabilityForRequest returns the endpoint capability
// required by an image or Responses request. needsResponses includes both the
// legacy /responses/compact endpoint and native remote compaction v2.
func openAIResponsesRequiredCapabilityForRequest(imageIntent bool, needsResponses bool, platform string) service.OpenAIEndpointCapability {
	if needsResponses && platform == service.PlatformOpenAI {
		return service.OpenAIEndpointCapabilityResponses
	}
	return openAIResponsesRequiredCapability(imageIntent, platform)
}

// NewOpenAIGatewayHandler creates a new OpenAIGatewayHandler
func NewOpenAIGatewayHandler(
	gatewayService *service.OpenAIGatewayService,
	concurrencyService *service.ConcurrencyService,
	billingCacheService *service.BillingCacheService,
	apiKeyService *service.APIKeyService,
	usageRecordWorkerPool *service.UsageRecordWorkerPool,
	errorPassthroughService *service.ErrorPassthroughService,
	contentModerationService *service.ContentModerationService,
	opsService *service.OpsService,
	cfg *config.Config,
	modelCatalog service.CatalogListingSource,
) *OpenAIGatewayHandler {
	pingInterval := time.Duration(0)
	maxAccountSwitches := 3
	if cfg != nil {
		pingInterval = time.Duration(cfg.Concurrency.PingInterval) * time.Second
		if cfg.Gateway.MaxAccountSwitches > 0 {
			maxAccountSwitches = cfg.Gateway.MaxAccountSwitches
		}
	}
	return &OpenAIGatewayHandler{
		gatewayService:           gatewayService,
		billingCacheService:      billingCacheService,
		apiKeyService:            apiKeyService,
		usageRecordWorkerPool:    usageRecordWorkerPool,
		errorPassthroughService:  errorPassthroughService,
		contentModerationService: contentModerationService,
		opsService:               opsService,
		concurrencyHelper:        NewConcurrencyHelper(concurrencyService, SSEPingFormatComment, pingInterval),
		imageLimiter:             &ImageConcurrencyLimiter{},
		maxAccountSwitches:       maxAccountSwitches,
		cfg:                      cfg,
		modelCatalog:             modelCatalog,
	}
}

func isOpenAILegacyCompactPath(c *gin.Context) bool {
	return service.IsOpenAIResponsesCompactPath(c)
}

// isBareOpenAIResponsesPath 仅匹配裸 /responses 端点（无 /compact 等子路径），
// body-signal 提升只允许发生在这里，避免误伤 /responses/{id}/... 形态的请求。
func isBareOpenAIResponsesPath(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return false
	}
	normalizedPath := strings.TrimRight(strings.TrimSpace(c.Request.URL.Path), "/")
	switch normalizedPath {
	case EndpointResponses, "/openai/v1/responses", "/responses", "/backend-api/codex/responses":
		return true
	default:
		return false
	}
}

func isOpenAIRemoteCompactionV2Request(body []byte) bool {
	stream, valid := parseOpenAICompatibleStream(body)
	return valid && stream && service.HasCompactionTriggerInInput(body)
}

func (h *OpenAIGatewayHandler) normalizeOpenAIResponsesCompactRequest(c *gin.Context, reqLog *zap.Logger, body []byte) ([]byte, bool) {
	return normalizeOpenAIResponsesCompactRequest(c, reqLog, body)
}

// anthropicErrorResponse writes an error in Anthropic Messages API format.
func (h *OpenAIGatewayHandler) anthropicErrorResponse(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}

func normalizeCodexDelegationBootstrap(body []byte) ([]byte, bool) {
	// 已有任务通过 send_message_to_thread 唤醒时会携带 previous_response_id；
	// 完整历史回放还会带有已配对的调用项。delegation 仍是客户端注入的用户输入，
	// 不属于这些历史调用的结果，因此允许它与可明确配对的历史上下文共存。
	return normalizeCodexCallOutputBootstrap(body, isCodexDelegationCandidate, true)
}

func normalizeCodexAutomationBootstrap(body []byte) ([]byte, bool) {
	return normalizeCodexCallOutputBootstrap(body, isCodexAutomationCandidate, false)
}

func normalizeCodexCallOutputBootstrap(body []byte, isCandidate func(map[string]any) bool, allowHistoricalContext bool) ([]byte, bool) {
	if !hasUniqueJSONMembers(body) {
		return body, false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var request map[string]any
	if err := decoder.Decode(&request); err != nil {
		return body, false
	}
	if previousResponseID, exists := request["previous_response_id"]; exists {
		value, ok := previousResponseID.(string)
		if !ok || (!allowHistoricalContext && strings.TrimSpace(value) != "") {
			return body, false
		}
	}
	input, ok := request["input"].([]any)
	if !ok {
		return body, false
	}

	// Responses built-ins follow the *_call / *_call_output naming convention,
	// so classify by the wire type shape instead of maintaining an incomplete
	// allowlist. Delegation may coexist with historical anchors only when their
	// IDs make them unambiguous; automation retains the bootstrap-only boundary.
	for _, raw := range input {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		typ := stringField(item, "type")
		if isCandidate(item) {
			callIDValue, exists := item["call_id"]
			callID, isString := callIDValue.(string)
			if exists && (!isString || strings.TrimSpace(callID) != "") {
				return body, false
			}
			continue
		}
		if typ == "item_reference" {
			if allowHistoricalContext && strings.TrimSpace(stringField(item, "id")) != "" {
				continue
			}
			return body, false
		}
		if strings.HasSuffix(typ, "_call") || isResponsesCallOutputType(typ) {
			if allowHistoricalContext && strings.TrimSpace(stringField(item, "call_id")) != "" {
				continue
			}
			return body, false
		}
	}

	changed := false
	for i, raw := range input {
		item, ok := raw.(map[string]any)
		if !ok || !isCandidate(item) {
			continue
		}
		output, ok := item["output"].(string)
		if !ok {
			continue
		}
		input[i] = map[string]any{
			"type": "message",
			"role": "user",
			"content": []any{map[string]any{
				"type": "input_text",
				"text": output,
			}},
		}
		changed = true
	}
	if !changed {
		return body, false
	}
	normalized, err := json.Marshal(request)
	if err != nil {
		return body, false
	}
	return normalized, true
}

func hasUniqueJSONMembers(body []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(body))
	if !consumeUniqueJSONValue(decoder) {
		return false
	}
	_, err := decoder.Token()
	return err == io.EOF
}

func consumeUniqueJSONValue(decoder *json.Decoder) bool {
	token, err := decoder.Token()
	if err != nil {
		return false
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return true
	}

	switch delim {
	case '{':
		members := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return false
			}
			key, ok := keyToken.(string)
			if !ok {
				return false
			}
			if _, duplicate := members[key]; duplicate {
				return false
			}
			members[key] = struct{}{}
			if !consumeUniqueJSONValue(decoder) {
				return false
			}
		}
		end, err := decoder.Token()
		return err == nil && end == json.Delim('}')
	case '[':
		for decoder.More() {
			if !consumeUniqueJSONValue(decoder) {
				return false
			}
		}
		end, err := decoder.Token()
		return err == nil && end == json.Delim(']')
	default:
		return false
	}
}

func isResponsesCallOutputType(typ string) bool {
	return strings.HasSuffix(typ, "_call_output") || typ == "tool_search_output"
}

func isCodexDelegationCandidate(item map[string]any) bool {
	if stringField(item, "type") != "function_call_output" ||
		!isCodexDelegationTool(stringField(item, "namespace"), stringField(item, "name")) {
		return false
	}
	output, ok := item["output"].(string)
	return ok && validCodexDelegationEnvelope(output)
}

func isCodexAutomationCandidate(item map[string]any) bool {
	if stringField(item, "type") != "function_call_output" ||
		stringField(item, "namespace") != "codex_app" ||
		stringField(item, "name") != "automation_update" {
		return false
	}
	output, ok := item["output"].(string)
	return ok && (validCodexAutomationBootstrap(output) || validCodexAutomationHeartbeat(output))
}

func stringField(item map[string]any, key string) string {
	value, _ := item[key].(string)
	return value
}

func isCodexDelegationTool(namespace, name string) bool {
	return (namespace == "codex_app" || namespace == "codex_tui") &&
		(name == "create_thread" || name == "send_message_to_thread")
}

func validCodexAutomationBootstrap(value string) bool {
	normalized := strings.ReplaceAll(value, "\r\n", "\n")
	if strings.ContainsRune(normalized, '\r') {
		return false
	}
	lines := strings.Split(normalized, "\n")
	if len(lines) < 6 {
		return false
	}
	if _, ok := codexAutomationHeaderValue(lines[0], "Automation: "); !ok {
		return false
	}
	automationID, ok := codexAutomationHeaderValue(lines[1], "Automation ID: ")
	if !ok || !validCodexAutomationID(automationID) {
		return false
	}
	expectedMemory := "Automation memory: $CODEX_HOME/automations/" + automationID + "/memory.md"
	if lines[2] != expectedMemory {
		return false
	}
	lastRun, ok := codexAutomationHeaderValue(lines[3], "Last run: ")
	if !ok || !validCodexAutomationLastRun(lastRun) || lines[4] != "" {
		return false
	}
	return strings.TrimSpace(strings.Join(lines[5:], "\n")) != ""
}

func codexAutomationHeaderValue(line, prefix string) (string, bool) {
	if !strings.HasPrefix(line, prefix) {
		return "", false
	}
	value := strings.TrimPrefix(line, prefix)
	return value, value != "" && strings.TrimSpace(value) == value
}

func validCodexAutomationID(value string) bool {
	if len(value) == 0 || len(value) > 128 || value == "." || value == ".." {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' {
			continue
		}
		return false
	}
	return true
}

func validCodexAutomationLastRun(value string) bool {
	if value == "never" {
		return true
	}
	separator := strings.LastIndex(value, " (")
	if separator <= 0 || !strings.HasSuffix(value, ")") {
		return false
	}
	runAt, err := time.Parse(time.RFC3339Nano, value[:separator])
	if err != nil {
		return false
	}
	epochMillis, err := strconv.ParseInt(value[separator+2:len(value)-1], 10, 64)
	return err == nil && runAt.UnixMilli() == epochMillis
}

func validCodexAutomationHeartbeat(value string) bool {
	decoder := xml.NewDecoder(strings.NewReader(value))
	var rootSeen, automationIDSeen bool
	var childName string
	var childText bytes.Buffer
	fields := make(map[string]string)
	depth := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			id := fields["automation_id"]
			timestamp, hasTime := fields["current_time_iso"]
			instructions, hasInstructions := fields["instructions"]
			if hasTime != hasInstructions {
				return false
			}
			if hasTime {
				if _, err := time.Parse(time.RFC3339Nano, timestamp); err != nil || strings.TrimSpace(instructions) == "" {
					return false
				}
			}
			return rootSeen && automationIDSeen && depth == 0 &&
				strings.TrimSpace(id) == id && validCodexAutomationID(id)
		}
		if err != nil {
			return false
		}
		switch current := token.(type) {
		case xml.StartElement:
			depth++
			if current.Name.Space != "" || len(current.Attr) != 0 || depth > 2 {
				return false
			}
			if depth == 1 {
				if rootSeen || current.Name.Local != "heartbeat" {
					return false
				}
				rootSeen = true
			} else {
				childName = current.Name.Local
				switch childName {
				case "automation_id", "current_time_iso", "instructions":
				default:
					return false
				}
				if _, duplicate := fields[childName]; duplicate {
					return false
				}
				childText.Reset()
			}
		case xml.EndElement:
			if current.Name.Space != "" {
				return false
			}
			if depth == 2 {
				fields[childName] = childText.String()
				automationIDSeen = automationIDSeen || childName == "automation_id"
			}
			depth--
			if depth < 0 {
				return false
			}
		case xml.CharData:
			if depth == 2 {
				_, _ = childText.Write(current)
			} else if len(bytes.TrimSpace(current)) != 0 {
				return false
			}
		case xml.Comment, xml.ProcInst, xml.Directive:
			return false
		}
	}
}

func validCodexDelegationEnvelope(value string) bool {
	decoder := xml.NewDecoder(strings.NewReader(value))
	var rootSeen, sourceSeen, inputSeen bool
	var childName string
	var childText bytes.Buffer
	depth := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return rootSeen && depth == 0 && sourceSeen && inputSeen
		}
		if err != nil {
			return false
		}
		switch current := token.(type) {
		case xml.StartElement:
			depth++
			if current.Name.Space != "" || len(current.Attr) != 0 || (depth == 1 && current.Name.Local != "codex_delegation") || depth > 2 {
				return false
			}
			if depth == 1 {
				if rootSeen {
					return false
				}
				rootSeen = true
				continue
			}
			if current.Name.Local != "source_thread_id" && current.Name.Local != "input" {
				return false
			}
			childName = current.Name.Local
			childText.Reset()
		case xml.EndElement:
			if current.Name.Space != "" {
				return false
			}
			if depth == 2 {
				if current.Name.Local != childName || strings.TrimSpace(childText.String()) == "" {
					return false
				}
				if childName == "source_thread_id" {
					if sourceSeen {
						return false
					}
					sourceSeen = true
				} else {
					if inputSeen {
						return false
					}
					inputSeen = true
				}
				childName = ""
			}
			depth--
			if depth < 0 {
				return false
			}
		case xml.CharData:
			if depth == 2 {
				_, _ = childText.Write(current)
			} else if len(bytes.TrimSpace(current)) != 0 {
				return false
			}
		case xml.Comment:
			return false
		case xml.ProcInst, xml.Directive:
			return false
		}
	}
}

func (h *OpenAIGatewayHandler) acquireResponsesUserSlot(
	c *gin.Context,
	userID int64,
	userConcurrency int,
	reqStream bool,
	streamStarted *bool,
	reqLog *zap.Logger,
) (func(), bool) {
	ctx := c.Request.Context()
	userReleaseFunc, err := h.concurrencyHelper.AcquireUserSlotWithWait(c, userID, userConcurrency, reqStream, streamStarted)
	if err != nil {
		reqLog.Warn("openai.user_slot_acquire_failed", zap.Error(err))
		h.handleConcurrencyError(c, err, "user", *streamStarted)
		return nil, false
	}
	return wrapReleaseOnDone(ctx, userReleaseFunc), true
}

// openAISlotAcquireResult 是账号槽位获取的三态结果。
type openAISlotAcquireResult int

const (
	openAISlotAcquireOK openAISlotAcquireResult = iota
	// openAISlotAcquireFailed：错误响应已写出，调用方直接 return。
	openAISlotAcquireFailed
	// openAISlotAcquireProfitVetoed：槽位获取成功后利润终检否决。槽位已释放、
	// 未写任何响应；调用方应经 recordOpenAIProfitVeto 把该账号加入本请求排除集
	// 后重新选号，全池耗尽由下一轮选号返回标准 no available accounts。
	openAISlotAcquireProfitVetoed
)

// openAIWSTurnPricing 持有 WebSocket 连接内「当前 turn」的计费定价时刻。
// 由 BeforeTurn 在每个 turn 开始时冻结，AfterTurn 的用量提交读取它；turn 在
// 连接内串行推进，互斥锁只为跨用量提交 goroutine 的读取安全。
//
// 零值语义（重要）：首轮准入由握手路径完成，不调用 BeforeTurn，因此首轮保持
// 零值并回退到 TurnStarted 记录的首轮开始时刻。后续 turn 在 response.create
// 写入上游前调用 BeforeTurn，按当时的利润门复核并冻结定价。绝不能用建连时刻
// 初始化，否则会把长连接的所有 turn 钉死在建连时的峰谷因子。
type openAIWSTurnPricing struct {
	mu sync.Mutex
	at time.Time
}

func (p *openAIWSTurnPricing) freeze(at time.Time) {
	p.mu.Lock()
	p.at = at
	p.mu.Unlock()
}

func (p *openAIWSTurnPricing) currentOr(fallback time.Time) time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.at.IsZero() {
		return p.at
	}
	return fallback
}

// recordOpenAIProfitVeto 记录 OpenAI 侧选号循环的一次利润门终检否决：把账号
// 加入本请求排除集并递增否决计数。返回 false 表示否决次数已达
// maxProfitVetoAttempts，调用方必须停止重选并按「无可用账号」终止。
//
// OpenAI 路径用的是各自的 failedAccountIDs map + for 循环（不是 FailoverState），
// 这里用一个独立计数器复用同一上限语义。上限是必需的：WaitPlan 分支先阻塞
// 排队（sticky 45s / fallback 30s）拿到槽位才终检，无上限重选会把单次请求的
// 延迟放大到 N × WaitPlan.Timeout。
func recordOpenAIProfitVeto(failedAccountIDs map[int64]struct{}, accountID int64, vetoCount *int) bool {
	failedAccountIDs[accountID] = struct{}{}
	*vetoCount++
	return *vetoCount < maxProfitVetoAttempts
}

// handleOpenAIProfitVetoExhausted 在利润否决预算耗尽时写出错误响应。
// 与 acquireResponsesAccountSlot 内部的 no-available-accounts 失败分支同形，
// 保证同一调用方在两条路径上拿到一致的响应格式。
func (h *OpenAIGatewayHandler) handleOpenAIProfitVetoExhausted(
	c *gin.Context,
	streamStarted bool,
	reqLog *zap.Logger,
	vetoCount int,
) {
	reqLog.Warn("openai.profit_veto_attempts_exhausted", zap.Int("profit_veto_count", vetoCount))
	markOpsRoutingCapacityLimited(c)
	h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", profitVetoExhaustedMessage, streamStarted)
}

func (h *OpenAIGatewayHandler) acquireResponsesAccountSlot(
	c *gin.Context,
	sessionHash string,
	selection *service.AccountSelectionResult,
	reqStream bool,
	streamStarted *bool,
	reqLog *zap.Logger,
) (func(), openAISlotAcquireResult) {
	return h.acquireOpenAIAccountSlot(c, sessionHash, selection, reqStream, streamStarted, reqLog, nil)
}

type openAISlotErrorWriter func(status int, errType, code, message string)

// acquireOpenAIAccountSlot centralizes scheduler selection admission. The
// optional error writer lets non-Responses endpoints retain their wire format
// while sharing the same WaitPlan, cancellation, and release semantics.
func (h *OpenAIGatewayHandler) acquireOpenAIAccountSlot(
	c *gin.Context,
	sessionHash string,
	selection *service.AccountSelectionResult,
	reqStream bool,
	streamStarted *bool,
	reqLog *zap.Logger,
	writeError openAISlotErrorWriter,
) (func(), openAISlotAcquireResult) {
	if writeError == nil {
		writeError = func(status int, errType, code, message string) {
			h.handleStreamingAwareErrorWithCode(c, status, errType, code, message, *streamStarted, false)
		}
	}
	if selection == nil || selection.Account == nil {
		markOpsRoutingCapacityLimited(c)
		writeError(http.StatusServiceUnavailable, "api_error", "", "No available accounts")
		return nil, openAISlotAcquireFailed
	}

	// 终检与准入后绑定使用选号结果携带的门：composite 等跨分组调度解析出的
	// 门只存在于调度栈的局部 ctx，必须经选号结果重放到本函数的 ctx 上。
	ctx := service.ContextWithSelectionProfitGate(c.Request.Context(), selection)
	account := selection.Account
	if selection.Acquired {
		latest, vetoed, reason := h.gatewayService.Scheduler().GatewayProfitControlVetoLatest(ctx, account)
		if vetoed {
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			reqLog.Debug("openai.account_slot_profit_vetoed", zap.Int64("account_id", account.ID), zap.String("reason", reason))
			return nil, openAISlotAcquireProfitVetoed
		}
		account = latest
		selection.Account = latest
		// 调度器已抢槽路径无门时由选号内部完成 eager 绑定；门下选号内部
		// 推迟绑定，这里在终检通过后补准入后绑定。
		if selection.ProfitGateActive() {
			if err := h.gatewayService.Scheduler().BindStickySessionAfterProfitAdmission(ctx, sessionHash, account.ID); err != nil {
				reqLog.Warn("openai.bind_sticky_session_after_profit_admission_failed", zap.Int64("account_id", account.ID), zap.Error(err))
			}
		}
		return wrapReleaseOnDone(ctx, selection.ReleaseFunc), openAISlotAcquireOK
	}
	if selection.WaitPlan == nil {
		markOpsRoutingCapacityLimited(c)
		writeError(http.StatusServiceUnavailable, "api_error", "", "No available accounts")
		return nil, openAISlotAcquireFailed
	}

	fastReleaseFunc, fastAcquired, err := h.concurrencyHelper.TryAcquireAccountSlot(
		ctx,
		account.ID,
		selection.WaitPlan.MaxConcurrency,
	)
	if err != nil {
		reqLog.Warn("openai.account_slot_quick_acquire_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		status, errType, code, message := concurrencyErrorResponse(err, "account")
		writeError(status, errType, code, message)
		return nil, openAISlotAcquireFailed
	}
	if fastAcquired {
		// 分组利润控制：快速抢槽成功后终检。选号与抢槽之间账号
		// 倍率可能刷新，越线则释放槽位交由调用方排除重选，不绑定粘连。
		latest, vetoed, reason := h.gatewayService.Scheduler().GatewayProfitControlVetoLatest(ctx, account)
		if vetoed {
			if fastReleaseFunc != nil {
				fastReleaseFunc()
			}
			reqLog.Debug("openai.account_slot_profit_vetoed", zap.Int64("account_id", account.ID), zap.String("reason", reason))
			return nil, openAISlotAcquireProfitVetoed
		}
		account = latest
		selection.Account = latest
		if err := h.gatewayService.Scheduler().BindStickySessionAfterProfitAdmission(ctx, sessionHash, account.ID); err != nil {
			reqLog.Warn("openai.bind_sticky_session_after_profit_admission_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		}
		return wrapReleaseOnDone(ctx, fastReleaseFunc), openAISlotAcquireOK
	}

	canWait, waitErr := h.concurrencyHelper.IncrementAccountWaitCount(ctx, account.ID, selection.WaitPlan.MaxWaiting)
	if waitErr != nil {
		reqLog.Warn("openai.account_wait_counter_increment_failed", zap.Int64("account_id", account.ID), zap.Error(waitErr))
	} else if !canWait {
		reqLog.Info("openai.account_wait_queue_full",
			zap.Int64("account_id", account.ID),
			zap.Int("max_waiting", selection.WaitPlan.MaxWaiting),
		)
		writeError(http.StatusTooManyRequests, "rate_limit_error", gatewayQueueFullCode, "Too many pending requests, please retry later")
		return nil, openAISlotAcquireFailed
	}

	accountWaitCounted := waitErr == nil && canWait
	releaseWait := func() {
		if accountWaitCounted {
			h.concurrencyHelper.DecrementAccountWaitCount(ctx, account.ID)
			accountWaitCounted = false
		}
	}
	defer releaseWait()

	accountReleaseFunc, err := h.concurrencyHelper.AcquireAccountSlotWithWaitTimeout(
		c,
		account.ID,
		selection.WaitPlan.MaxConcurrency,
		selection.WaitPlan.Timeout,
		reqStream,
		streamStarted,
	)
	if err != nil {
		reqLog.Warn("openai.account_slot_acquire_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		status, errType, code, message := concurrencyErrorResponse(err, "account")
		writeError(status, errType, code, message)
		return nil, openAISlotAcquireFailed
	}

	// Slot acquired: no longer waiting in queue.
	releaseWait()
	// 分组利润控制：WaitPlan 排队成功后终检。排队期间账号倍率
	// 可能上调，越线则释放槽位交由调用方排除重选，不绑定粘连。
	latest, vetoed, reason := h.gatewayService.Scheduler().GatewayProfitControlVetoLatest(ctx, account)
	if vetoed {
		if accountReleaseFunc != nil {
			accountReleaseFunc()
		}
		reqLog.Debug("openai.account_slot_profit_vetoed", zap.Int64("account_id", account.ID), zap.String("reason", reason))
		return nil, openAISlotAcquireProfitVetoed
	}
	account = latest
	selection.Account = latest
	if err := h.gatewayService.Scheduler().BindStickySessionAfterProfitAdmission(ctx, sessionHash, account.ID); err != nil {
		reqLog.Warn("openai.bind_sticky_session_after_profit_admission_failed", zap.Int64("account_id", account.ID), zap.Error(err))
	}
	return wrapReleaseOnDone(ctx, accountReleaseFunc), openAISlotAcquireOK
}

// ResponsesWebSocket handles OpenAI Responses API WebSocket ingress endpoint
// GET /openai/v1/responses (Upgrade: websocket)
func (h *OpenAIGatewayHandler) ResponsesWebSocket(c *gin.Context) {
	if !isOpenAIWSUpgradeRequest(c.Request) {
		h.errorResponse(c, http.StatusUpgradeRequired, "invalid_request_error", "WebSocket upgrade required (Upgrade: websocket)")
		return
	}
	setOpenAIClientTransportWS(c)

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}

	reqLog := requestLogger(
		c,
		"handler.openai_gateway.responses_ws",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
		zap.Bool("openai_ws_mode", true),
	)
	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}
	reqLog.Info("openai.websocket_ingress_started")
	clientIP := ip.GetClientIP(c)
	userAgent := strings.TrimSpace(c.GetHeader("User-Agent"))
	clientLifecycleCtx := c.Request.Context()
	ctx := clientLifecycleCtx
	maxIngressConnections := 0
	if h.cfg != nil {
		maxIngressConnections = h.cfg.Gateway.OpenAIWS.MaxIngressConnectionsPerAPIKey
	}
	ingressLease, ingressLeaseAcquired, ingressLeaseErr := h.concurrencyHelper.AcquireOpenAIWSIngressLease(ctx, apiKey.ID, maxIngressConnections)
	if ingressLeaseErr != nil {
		reqLog.Error("openai.websocket_ingress_lease_acquire_failed", zap.Error(ingressLeaseErr))
		h.errorResponse(c, http.StatusServiceUnavailable, "service_unavailable", "WebSocket ingress capacity is temporarily unavailable")
		return
	}
	if !ingressLeaseAcquired {
		reqLog.Info("openai.websocket_ingress_capacity_rejected", zap.Int("max_ingress_connections_per_api_key", maxIngressConnections))
		c.Header("Retry-After", "5")
		h.errorResponse(c, http.StatusTooManyRequests, "rate_limit_error", "Too many open WebSocket connections, please retry later")
		return
	}
	if ingressLease != nil {
		defer ingressLease.Release()
		ctx = ingressLease.Context()
		c.Request = c.Request.WithContext(ctx)
	}

	wsConn, err := coderws.Accept(c.Writer, c.Request, &coderws.AcceptOptions{
		CompressionMode: coderws.CompressionContextTakeover,
	})
	if err != nil {
		reqLog.Warn("openai.websocket_accept_failed",
			zap.Error(err),
			zap.String("client_ip", clientIP),
			zap.String("request_user_agent", userAgent),
			zap.String("upgrade_header", strings.TrimSpace(c.GetHeader("Upgrade"))),
			zap.String("connection_header", strings.TrimSpace(c.GetHeader("Connection"))),
			zap.String("sec_websocket_version", strings.TrimSpace(c.GetHeader("Sec-WebSocket-Version"))),
			zap.Bool("has_sec_websocket_key", strings.TrimSpace(c.GetHeader("Sec-WebSocket-Key")) != ""),
		)
		return
	}
	defer func() {
		_ = wsConn.CloseNow()
	}()
	wsConn.SetReadLimit(service.ResolveOpenAIWSClientReadLimitBytes(h.cfg))

	firstMessageTimeout := service.ResolveOpenAIWSClientFirstMessageTimeout(h.cfg)
	msgType, firstMessage, err := service.ReadOpenAIWSClientMessage(
		ctx,
		wsConn,
		firstMessageTimeout,
		coderws.StatusPolicyViolation,
		"missing first response.create message",
	)
	if err != nil {
		if errors.Is(context.Cause(ctx), service.ErrOpenAIWSIngressLeaseLost) {
			reqLog.Warn("openai.websocket_ingress_lease_lost_before_first_message", zap.Error(err))
			closeOpenAIClientWS(wsConn, coderws.StatusTryAgainLater, "websocket ingress capacity lease lost; please reconnect")
			return
		}
		closeStatus, closeReason := summarizeWSCloseErrorForLog(err)
		reqLog.Warn("openai.websocket_read_first_message_failed",
			zap.Error(err),
			zap.String("client_ip", clientIP),
			zap.String("close_status", closeStatus),
			zap.String("close_reason", closeReason),
			zap.Duration("read_timeout", firstMessageTimeout),
		)
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, "missing first response.create message")
		return
	}
	firstTurnStartedAt := time.Now()
	if msgType != coderws.MessageText && msgType != coderws.MessageBinary {
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, "unsupported websocket message type")
		return
	}
	if !gjson.ValidBytes(firstMessage) {
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, "invalid JSON payload")
		return
	}
	reqModel := strings.TrimSpace(gjson.GetBytes(firstMessage, "model").String())
	if reqModel == "" {
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, "model is required in first response.create payload")
		return
	}
	// 目录准入：首帧的模型必须解析到上架条目，命中后把条目路由挂到 ctx
	// （WS 入口没有经过 HTTP 准入中间件）。与 HTTP 准入一致：帧内重复 model 键 / 大小写
	// 变体可能被上游按末值绑定，全部候选值逐一校验，任一未命中即拒绝。
	firstCandidates := requestmodel.FromBodyCandidates("", "application/json", firstMessage)
	route, blockedCandidate, routed := service.ResolveCatalogRouteForCandidates(c.Request.Context(), h.modelCatalog, firstCandidates)
	if !routed {
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalModelConfiguration)
		middleware2.MarkIngressRejected(c, middleware2.IngressRejectModelNotListed)
		if blockedCandidate == "" {
			blockedCandidate = reqModel
		}
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, fmt.Sprintf("Model %q is not available", blockedCandidate))
		return
	}
	// 订阅模型集：订阅 key 只能调套餐里的条目（HTTP 由 SubscriptionModelAdmission 中间件判，WS 在这里判同一条规则）。
	wsSubscription, _ := middleware2.GetSubscriptionFromContext(c)
	if !service.SubscriptionCoversRoute(wsSubscription, route) {
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalModelConfiguration)
		middleware2.MarkIngressRejected(c, middleware2.IngressRejectModelNotInPlan)
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, fmt.Sprintf("Model %q is not included in your subscription plan", route.RequestedModel))
		return
	}
	c.Request = c.Request.WithContext(service.WithCatalogRoute(c.Request.Context(), route))
	ctx = c.Request.Context()
	previousResponseID := strings.TrimSpace(gjson.GetBytes(firstMessage, "previous_response_id").String())
	previousResponseIDKind := service.ClassifyOpenAIPreviousResponseIDKind(previousResponseID)
	if previousResponseID != "" && previousResponseIDKind == service.OpenAIPreviousResponseIDKindMessageID {
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, "previous_response_id must be a response.id (resp_*), not a message id")
		return
	}
	firstMessageToolCoverage := service.AnalyzeToolCallOutputContextCoverageBytes(firstMessage)
	previousResponseCanMove := !firstMessageToolCoverage.HasFunctionCallOutput || firstMessageToolCoverage.ContextCoversAllCallIDs
	reqLog = reqLog.With(
		zap.Bool("ws_ingress", true),
		zap.String("session_initial_model", reqModel),
		zap.Bool("has_previous_response_id", previousResponseID != ""),
		zap.String("previous_response_id_kind", previousResponseIDKind),
	)
	setOpsRequestContext(c, reqModel, true)
	setOpsEndpointContext(c, "", int16(service.RequestTypeWSV2))

	if decision := h.checkSecurityAuditStage(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIResponses, reqModel, firstMessage, "first_turn"); decision != nil && !decision.AllowNextStage {
		writeSecurityAuditWSError(ctx, wsConn, decision)
		closeOpenAIClientWS(wsConn, securityAuditWSCloseStatus(decision), securityAuditWSCloseReason(decision))
		return
	}

	// The first response.create frame is available here, so explicit IDs are
	// checked directly and body-derived sessions use the coarse scope gate.
	if cyberBlockKey := findBlockedCyberSessionKey(c.Request.Context(), h.gatewayService, apiKey.ID, c, firstMessage); cyberBlockKey != "" {
		writeCyberSessionBlockedWSError(c.Request.Context(), wsConn)
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, "session blocked by cyber-security policy")
		enqueueCyberSessionBlockedOpsEntry(c, h.opsService, apiKey, reqModel, cyberBlockKey)
		return
	}
	cyberBlockedThisConn := false
	cyberBlockPendingAfterFailover := false
	var cyberTurnBodiesMu sync.Mutex
	cyberTurnBodies := map[int][]byte{1: append([]byte(nil), firstMessage...)}
	setCyberTurnBody := func(turn int, payload []byte) {
		cyberTurnBodiesMu.Lock()
		cyberTurnBodies[turn] = append([]byte(nil), payload...)
		cyberTurnBodiesMu.Unlock()
	}
	takeCyberTurnBody := func(turn int) []byte {
		cyberTurnBodiesMu.Lock()
		body := cyberTurnBodies[turn]
		delete(cyberTurnBodies, turn)
		cyberTurnBodiesMu.Unlock()
		return body
	}

	wsForwardModel := reqModel

	var currentUserRelease func()
	var currentAccountRelease func()
	releaseAccountSlot := func() {
		if currentAccountRelease != nil {
			currentAccountRelease()
			currentAccountRelease = nil
		}
	}
	releaseTurnSlots := func() {
		releaseAccountSlot()
		if currentUserRelease != nil {
			currentUserRelease()
			currentUserRelease = nil
		}
	}
	// 必须尽早注册，确保任何 early return 都能释放已获取的并发槽位。
	defer releaseTurnSlots()

	userReleaseFunc, userAcquired, err := h.concurrencyHelper.TryAcquireUserSlotForAPIKey(ctx, subject.UserID, subject.Concurrency, apiKey.ID)
	if err != nil {
		reqLog.Warn("openai.websocket_user_slot_acquire_failed", zap.Error(err))
		closeOpenAIClientWS(wsConn, coderws.StatusInternalError, "failed to acquire user concurrency slot")
		return
	}
	if !userAcquired {
		closeOpenAIClientWS(wsConn, coderws.StatusTryAgainLater, "too many concurrent requests, please retry later")
		return
	}
	currentUserRelease = wrapReleaseOnDone(ctx, userReleaseFunc)
	ensureUserSlotHeld := func() bool {
		if currentUserRelease != nil {
			return true
		}
		userReleaseFunc, userAcquired, err := h.concurrencyHelper.TryAcquireUserSlotForAPIKey(ctx, subject.UserID, subject.Concurrency, apiKey.ID)
		if err != nil {
			reqLog.Warn("openai.websocket_user_slot_reacquire_failed", zap.Error(err))
			closeOpenAIClientWS(wsConn, coderws.StatusInternalError, "failed to acquire user concurrency slot")
			return false
		}
		if !userAcquired {
			closeOpenAIClientWS(wsConn, coderws.StatusTryAgainLater, "too many concurrent requests, please retry later")
			return false
		}
		currentUserRelease = wrapReleaseOnDone(ctx, userReleaseFunc)
		return true
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	requestPlatform := service.OpenAICompatibleRequestPlatform(ctx)
	requiredTransport := service.OpenAIUpstreamTransportResponsesWebsocketV2Ingress
	if requestPlatform == service.PlatformGrok {
		requiredTransport = service.OpenAIUpstreamTransportHTTPSSE
	}
	if err := h.billingCacheService.CheckBillingEligibility(ctx, apiKey.User, apiKey, subscription); err != nil {
		reqLog.Info("openai.websocket_billing_eligibility_check_failed", zap.Error(err))
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, "billing check failed")
		return
	}

	sessionHash := h.gatewayService.GenerateSessionHashWithFallback(
		c,
		firstMessage,
		openAIWSIngressFallbackSessionSeed(subject.UserID, apiKey.ID),
	)
	ctx = service.WithOpenAIGuardianParentAffinity(ctx, c, firstMessage, reqModel)
	maxAccountSwitches := h.maxAccountSwitches
	switchCount := 0
	profitVetoCount := 0
	failedAccountIDs := make(map[int64]struct{})
	sameAccountRetryCount := make(map[int64]int)
	var lastFailoverErr *service.UpstreamFailoverError
	var oauth429FailoverState service.OpenAIOAuth429FailoverState
	wsAttemptMessage := append([]byte(nil), firstMessage...)
	waitForWSSameAccountRetry := func(account *service.Account, failoverErr *service.UpstreamFailoverError) bool {
		if account == nil || failoverErr == nil || failoverErr.StatusCode != http.StatusTooManyRequests || failoverErr.SameAccountRetryDeadline.IsZero() {
			return false
		}
		retryLimit := effectiveSameAccountRetryLimit(failoverErr, account)
		if !sameAccountRetryAllowed(failoverErr, sameAccountRetryCount[account.ID], retryLimit) {
			return false
		}
		sameAccountRetryCount[account.ID]++
		retryDelay := sameAccountRetryDelayFor(failoverErr, sameAccountRetryCount[account.ID])
		reqLog.Warn("openai.websocket.same_account_retry",
			zap.Int64("account_id", account.ID),
			zap.Int("upstream_status", failoverErr.StatusCode),
			zap.Int("retry_count", sameAccountRetryCount[account.ID]),
			zap.Duration("retry_delay", retryDelay),
		)
		select {
		case <-ctx.Done():
			return false
		case <-time.After(retryDelay):
			return true
		}
	}
	handleWSFailover := func(account *service.Account, failoverErr *service.UpstreamFailoverError) bool {
		if ctx.Err() != nil {
			return false
		}
		if failoverErr.ShouldReportAccountScheduleFailure() {
			h.gatewayService.ObserveOpenAIAccountResult(account, openAIAccountScheduleModel(c, account, wsForwardModel, false, nil), false, failoverErr)
		}
		releaseAccountSlot()
		if !failoverErr.ShouldRetryNextAccount() {
			closeOpenAIWSFailoverExhausted(c, wsConn, failoverErr)
			return false
		}
		if ctx.Err() != nil {
			return false
		}
		failedAccountIDs[account.ID] = struct{}{}
		lastFailoverErr = failoverErr
		if switchCount >= maxAccountSwitches {
			closeOpenAIWSFailoverExhausted(c, wsConn, failoverErr)
			return false
		}
		switchCount++
		if h.gatewayService.ShouldStopOpenAIOAuth429Failover(account, failoverErr.StatusCode, switchCount, &oauth429FailoverState) {
			closeOpenAIWSFailoverExhausted(c, wsConn, failoverErr)
			return false
		}
		reqLog.Warn("openai.websocket_upstream_failover_switching",
			zap.Int64("account_id", account.ID),
			zap.Int("upstream_status", failoverErr.StatusCode),
			zap.Int("switch_count", switchCount),
			zap.Int("max_switches", maxAccountSwitches),
		)
		if ctx.Err() != nil {
			return false
		}
		return ensureUserSlotHeld()
	}

	// 与 HTTP Responses 路径保持一致：生图意图请求要求账号支持 Responses API（#4417）。
	// WSv2 传输本身已隐含 Responses 支持，此处为防御性对齐。
	// 使用 IsExplicitImageGenerationIntent 排除被动 namespace 声明（#4476）。
	requiredCapability := service.OpenAIEndpointCapabilityChatCompletions
	if service.IsExplicitImageGenerationIntent("/v1/responses", reqModel, firstMessage) && requestPlatform == service.PlatformOpenAI {
		requiredCapability = service.OpenAIEndpointCapabilityResponses
	}

	// 分组利润控制：WS 桥按连接装配定价上下文并装门（选号与抢槽共用该
	// ctx）。连接内不重选号，但每个 turn 开始经 BeforeTurn 重新冻结 pricingAt
	// 并按最新门复核当前账号（准入与计费同源），峰前建连保活不能让后续 turn
	// 继续按建连时刻的谷价计费。生图意图只影响能力路由与图片计费，不关门。
	// 建连时刻只用于选号/准入，不作为任何 turn 的计费定价时刻。
	wsPricingCtx, _ := h.gatewayService.WithOpenAIRequestPricingContext(ctx)
	ctx = wsPricingCtx

	// 续链与守护父线程亲和都是「已绑定的资源」：做成预取粘性（同 HTTP），选号时优先于缓存里的会话绑定。
	stickyID := int64(0)
	if previousResponseID != "" {
		stickyID = h.gatewayService.ResolveAccountIDByPreviousResponseIDForScheduler(ctx, previousResponseID, wsForwardModel, nil, requiredCapability, false)
	}
	if stickyID == 0 {
		stickyID = h.gatewayService.ResolveOpenAIGuardianParentAccountID(ctx)
	}
	if stickyID > 0 {
		ctx = service.WithPrefetchedStickySession(ctx, stickyID, service.SchedulingScopeID(ctx), false)
	}
	sched := h.gatewayService.Scheduler()

	for {
		if ctx.Err() != nil {
			return
		}
		reqLog.Debug("openai.websocket_account_selecting", zap.Int("excluded_account_count", len(failedAccountIDs)))
		selection, err := sched.SelectAccountWithOptions(ctx, sessionHash, wsForwardModel, failedAccountIDs,
			service.SelectOptions{Capability: requiredCapability, Transport: requiredTransport})
		if err != nil {
			reqLog.Warn("openai.websocket_account_select_failed",
				zap.Error(openAICompatibleSelectionErrorForLog(err, requestPlatform)),
				zap.Int("excluded_account_count", len(failedAccountIDs)),
			)
			if lastFailoverErr != nil {
				closeOpenAIWSFailoverExhausted(c, wsConn, lastFailoverErr)
			} else {
				closeOpenAIClientWS(wsConn, coderws.StatusTryAgainLater, "no available account")
			}
			return
		}
		if selection == nil || selection.Account == nil {
			if lastFailoverErr != nil {
				closeOpenAIWSFailoverExhausted(c, wsConn, lastFailoverErr)
			} else {
				closeOpenAIClientWS(wsConn, coderws.StatusTryAgainLater, "no available account")
			}
			return
		}

		account := selection.Account
		// 预取的续链 / 父线程账号被选中才算命中；未命中时首包的 previous_response_id 要剥掉（见下）
		stickyPreviousHit := stickyID > 0 && account.ID == stickyID
		accountMaxConcurrency := account.Concurrency
		if selection.WaitPlan != nil && selection.WaitPlan.MaxConcurrency > 0 {
			accountMaxConcurrency = selection.WaitPlan.MaxConcurrency
		}
		// 终检、准入后绑定与后续 turn 级复核都使用选号结果携带的门（composite
		// 等跨分组调度的门只存在于调度栈局部 ctx）；准入成功后并入连接 ctx。
		admissionCtx := service.ContextWithSelectionProfitGate(ctx, selection)
		accountReleaseFunc := selection.ReleaseFunc
		if selection.Acquired {
			// 调度器已抢槽路径同样终检：选号与抢槽之间账号倍率可能刷新。
			latest, vetoed, reason := sched.GatewayProfitControlVetoLatest(admissionCtx, account)
			if vetoed {
				if accountReleaseFunc != nil {
					accountReleaseFunc()
				}
				reqLog.Debug("openai.websocket_account_slot_profit_vetoed", zap.Int64("account_id", account.ID), zap.String("reason", reason))
				if !recordOpenAIProfitVeto(failedAccountIDs, account.ID, &profitVetoCount) {
					reqLog.Warn("openai.websocket_profit_veto_attempts_exhausted", zap.Int("profit_veto_count", profitVetoCount))
					closeOpenAIClientWS(wsConn, coderws.StatusTryAgainLater, "no available account")
					return
				}
				continue
			}
			account = latest
			selection.Account = latest
		}
		if !selection.Acquired {
			if selection.WaitPlan == nil {
				closeOpenAIClientWS(wsConn, coderws.StatusTryAgainLater, "account is busy, please retry later")
				return
			}
			fastReleaseFunc, fastAcquired, err := h.concurrencyHelper.TryAcquireAccountSlot(
				ctx,
				account.ID,
				selection.WaitPlan.MaxConcurrency,
			)
			if err != nil {
				reqLog.Warn("openai.websocket_account_slot_acquire_failed", zap.Int64("account_id", account.ID), zap.Error(err))
				closeOpenAIClientWS(wsConn, coderws.StatusInternalError, "failed to acquire account concurrency slot")
				return
			}
			if !fastAcquired {
				closeOpenAIClientWS(wsConn, coderws.StatusTryAgainLater, "account is busy, please retry later")
				return
			}
			// 分组利润控制：WS 快速抢槽成功后终检，越线则释放
			// 槽位、排除该账号重新选号，全池耗尽由下一轮选号关闭连接。
			latest, vetoed, reason := sched.GatewayProfitControlVetoLatest(admissionCtx, account)
			if vetoed {
				if fastReleaseFunc != nil {
					fastReleaseFunc()
				}
				reqLog.Debug("openai.websocket_account_slot_profit_vetoed", zap.Int64("account_id", account.ID), zap.String("reason", reason))
				if !recordOpenAIProfitVeto(failedAccountIDs, account.ID, &profitVetoCount) {
					reqLog.Warn("openai.websocket_profit_veto_attempts_exhausted", zap.Int("profit_veto_count", profitVetoCount))
					closeOpenAIClientWS(wsConn, coderws.StatusTryAgainLater, "no available account")
					return
				}
				continue
			}
			account = latest
			selection.Account = latest
			accountReleaseFunc = fastReleaseFunc
		}
		// 准入完成：门并入连接 ctx，turn 级复核与 failover 重选共用。
		ctx = admissionCtx
		// Account selection starts a fresh upstream attempt. Clear any model
		// captured by the previous failover account before credential lookup.
		setOpsSelectedAccount(c, account.ID, account.Platform)
		currentAccountRelease = wrapReleaseOnDone(ctx, accountReleaseFunc)
		if err := sched.BindStickySessionAfterProfitAdmission(ctx, sessionHash, account.ID); err != nil {
			reqLog.Warn("openai.websocket_bind_sticky_session_after_profit_admission_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		}

		token, _, err := h.gatewayService.GetRequestCredential(ctx, c, account)
		if err != nil {
			reqLog.Warn("openai.websocket_get_access_token_failed", zap.Int64("account_id", account.ID), zap.Error(err))
			if ctx.Err() != nil {
				return
			}
			var failoverErr *service.UpstreamFailoverError
			if errors.As(err, &failoverErr) {
				if handleWSFailover(account, failoverErr) {
					continue
				}
				return
			}
			closeOpenAIClientWS(wsConn, coderws.StatusInternalError, "failed to get access token")
			return
		}

		reqLog.Debug("openai.websocket_account_selected",
			zap.Int64("account_id", account.ID),
			zap.String("account_name", account.Name),
			zap.Bool("sticky_hit", stickyPreviousHit),
		)

		var requestPayloadHash string
		var turnStartsMu sync.Mutex
		turnStarts := make(map[int]time.Time, 4)
		recordTurnStart := func(turn int, startedAt time.Time) {
			if turn <= 0 || startedAt.IsZero() {
				return
			}
			turnStartsMu.Lock()
			turnStarts[turn] = startedAt
			turnStartsMu.Unlock()
		}
		getTurnStart := func(turn int) time.Time {
			turnStartsMu.Lock()
			startedAt := turnStarts[turn]
			delete(turnStarts, turn)
			turnStartsMu.Unlock()
			return startedAt
		}
		// Passthrough rejects overlapping response.create frames, so one immutable
		// turn-tagged slot preserves the exact mapping used for the in-flight request.
		var turnModel atomic.Pointer[openAIWSTurnModelSnapshot]
		turnModel.Store(&openAIWSTurnModelSnapshot{turn: 1, model: reqModel})
		// turn 级定价：首轮回退到 TurnStarted 的所属 turn 时刻；后续 turn 由
		// BeforeTurn 重新冻结 pricingAt 并按最新门复核当前账号。
		var turnPricing openAIWSTurnPricing
		hooks := &service.OpenAIWSIngressHooks{
			ClientLifecycleContext: clientLifecycleCtx,
			InitialRequestModel:    reqModel,
			InitialTurnStartedAt:   firstTurnStartedAt,
			TurnStarted:            recordTurnStart,
			BeforeRequest: func(turn int, payload []byte, originalModel string) error {
				c.Set(securityAuditWSTurnContextKey, turn)
				service.BeginOpsStreamTurn(c, turn)
				setCyberTurnBody(turn, payload)
				// 连接级 cyber session gate 也在 BeforeRequest 先执行，使 native 与
				// passthrough ingress 都能在 BeforeTurn 及上游写入前无副作用地拒绝。
				// BeforeTurn 中保留同一检查作为防御式兜底。
				if cyberBlockedThisConn {
					return service.NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, cyberSessionBlockedClientMsg, nil)
				}
				if turn == 1 {
					return nil
				}
				if !gjson.ValidBytes(payload) {
					return service.NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "invalid websocket request payload", errors.New("invalid json"))
				}
				model := strings.TrimSpace(originalModel)
				if model == "" {
					model = strings.TrimSpace(gjson.GetBytes(payload, "model").String())
				}
				if model == "" {
					model = reqModel
				}
				// 帧内重复 model 键 / 大小写变体 / 嵌套 session.model 逐一过目录准入（实际生效模型始终参与）。
				candidates := append([]string{model}, requestmodel.FromBodyCandidates("", "application/json", payload)...)
				// 目录准入：后续 turn 的模型也必须是上架条目；换到别的条目时连接绑死的账号
				// 必须也绑定了那个条目（连接不会中途换号）。
				turnRoute, blockedCandidate, ok := service.ResolveCatalogRouteForCandidates(ctx, h.modelCatalog, candidates)
				if !ok {
					service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalModelConfiguration)
					middleware2.MarkIngressRejected(c, middleware2.IngressRejectModelNotListed)
					return service.NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, fmt.Sprintf("Model %q is not available", blockedCandidate), nil)
				}
				// 换模型也要在套餐模型集里
				if !service.SubscriptionCoversRoute(wsSubscription, turnRoute) {
					service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalModelConfiguration)
					middleware2.MarkIngressRejected(c, middleware2.IngressRejectModelNotInPlan)
					return service.NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, fmt.Sprintf("Model %q is not included in your subscription plan", turnRoute.RequestedModel), nil)
				}
				if turnRoute.EntryID != route.EntryID && !slices.Contains(account.CatalogEntryIDs, turnRoute.EntryID) {
					return newOpenAIWSUnsupportedModelSwitchError(model)
				}
				if decision := h.checkSecurityAuditStage(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIResponses, model, payload, "subsequent_turn"); decision != nil && !decision.AllowNextStage {
					writeSecurityAuditWSError(ctx, wsConn, decision)
					return service.NewOpenAIWSClientCloseError(securityAuditWSCloseStatus(decision), securityAuditWSCloseReason(decision), nil)
				}
				return nil
			},
			MapRequestModel: func(turn int, originalModel string) (string, error) {
				model := strings.TrimSpace(originalModel)
				if model == "" {
					model = reqModel
				}
				setOpsRequestContext(c, model, true)
				modelUnchanged := false
				if previous := turnModel.Load(); previous != nil && previous.turn < turn {
					modelUnchanged = previous.model == model
				}
				if turn > 1 && !modelUnchanged && !account.IsModelSupported(model) {
					return "", newOpenAIWSUnsupportedModelSwitchError(model)
				}
				turnModel.Store(&openAIWSTurnModelSnapshot{turn: turn, model: model})
				return model, nil
			},
			BeforeTurn: func(turn int) error {
				// turn==1 的会话屏蔽已由握手层检查覆盖；连接内 flag 只拦截后续 turn。
				if cyberBlockedThisConn {
					return service.NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, cyberSessionBlockedClientMsg, nil)
				}
				// 长连接跨峰谷/倍率刷新防护：每个 turn 按当前时刻重装门并复核
				// 当前账号，越线即要求客户端重连重选（连接绑定单一上游账号，
				// 无法中途换号）。本 turn 的准入与计费共用同一 pricingAt。
				turnCtx, turnAt := h.gatewayService.WithOpenAITurnPricingContext(ctx)
				if _, vetoed, reason := sched.GatewayProfitControlVetoLatest(turnCtx, account); vetoed {
					reqLog.Info("openai.websocket_turn_profit_vetoed",
						zap.Int("turn", turn),
						zap.Int64("account_id", account.ID),
						zap.String("reason", reason))
					return service.NewOpenAIWSClientCloseError(coderws.StatusTryAgainLater, "account is no longer eligible for this connection, please reconnect", nil)
				}
				turnPricing.freeze(turnAt)
				if turn == 1 {
					return nil
				}
				// 防御式清理：避免异常路径下旧槽位覆盖导致泄漏。
				releaseTurnSlots()
				// 非首轮 turn 需要重新抢占并发槽位，避免长连接空闲占槽。
				userReleaseFunc, userAcquired, err := h.concurrencyHelper.TryAcquireUserSlotForAPIKey(ctx, subject.UserID, subject.Concurrency, apiKey.ID)
				if err != nil {
					return service.NewOpenAIWSClientCloseError(coderws.StatusInternalError, "failed to acquire user concurrency slot", err)
				}
				if !userAcquired {
					return service.NewOpenAIWSClientCloseError(coderws.StatusTryAgainLater, "too many concurrent requests, please retry later", nil)
				}
				accountReleaseFunc, accountAcquired, err := h.concurrencyHelper.TryAcquireAccountSlot(ctx, account.ID, accountMaxConcurrency)
				if err != nil {
					if userReleaseFunc != nil {
						userReleaseFunc()
					}
					return service.NewOpenAIWSClientCloseError(coderws.StatusInternalError, "failed to acquire account concurrency slot", err)
				}
				if !accountAcquired {
					if userReleaseFunc != nil {
						userReleaseFunc()
					}
					return service.NewOpenAIWSClientCloseError(coderws.StatusTryAgainLater, "account is busy, please retry later", nil)
				}
				currentUserRelease = wrapReleaseOnDone(ctx, userReleaseFunc)
				currentAccountRelease = wrapReleaseOnDone(ctx, accountReleaseFunc)
				return nil
			},
			AfterTurn: func(turn int, result *service.OpenAIForwardResult, turnErr error) {
				turnStart := getTurnStart(turn)
				cyberBlockBody := takeCyberTurnBody(turn)
				// 每次 attempt 都清 cyber mark；failover 链结束前保留 recorded guard，
				// 避免同一逻辑 turn 换号后重复落风控。CyberBlocked 必须在 submit 前
				// 同步预捕获（task 闭包由 worker 池异步执行，届时 mark 已清除）。
				defer func() {
					clearCyberPolicyAttemptState(c, !cyberBlockPendingAfterFailover)
				}()
				releaseTurnSlots()
				turnRequestedModel := reqModel
				turnUpstreamModel := ""
				if result != nil && turn > 1 {
					if model := strings.TrimSpace(result.Model); model != "" {
						turnRequestedModel = model
					}
				}
				if result != nil {
					turnUpstreamModel = strings.TrimSpace(result.UpstreamModel)
				}
				if turnUpstreamModel == "" {
					turnUpstreamModel = turnRequestedModel
				}
				cyberMarked := service.GetOpsCyberPolicy(c) != nil
				h.recordCyberPolicyIfMarked(c, apiKey, account, subscription, turnRequestedModel, turnErr != nil, cyberBlockBody, turnRequestedModel, requestPayloadHash)
				cyberBlockedThisConn, cyberBlockPendingAfterFailover = advanceOpenAIWSCyberBlockState(
					cyberBlockedThisConn,
					cyberBlockPendingAfterFailover,
					cyberMarked,
					turnErr,
				)
				if turnErr != nil {
					if result == nil || result.ImageCount <= 0 {
						return
					}
					// cyber 命中时该 turn 的用量已由 recordCyberPolicyIfMarked(forwardErrored=true)
					// 按真实 token 记录，这里不再走下方 RecordUsage，避免对同一 turn 双写/双扣费。
					if service.GetOpsCyberPolicy(c) != nil {
						return
					}
					reqLog.Warn("openai.websocket_partial_error_with_image_result",
						zap.Int64("account_id", account.ID),
						zap.Int("image_count", result.ImageCount),
						zap.Error(turnErr),
					)
				}
				if result == nil {
					return
				}
				result.BillingModel = openAIWSTurnBillingModel(result, turnRequestedModel, turnUpstreamModel)
				reqLog.Debug("openai.websocket_turn_billing",
					zap.Int("turn", turn),
					zap.String("turn_requested_model", turnRequestedModel),
					zap.String("turn_upstream_model", turnUpstreamModel),
					zap.String("billing_model", result.BillingModel),
				)
				// 排除 spark 影子:其 codex_* 仅由 QueryUsage(/wham/usage bengalfox)更新(外审第7轮 P1)。
				if account.Type == service.AccountTypeOAuth && !account.IsShadow() {
					h.gatewayService.UpdateCodexUsageSnapshotFromHeaders(ctx, account.ID, result.ResponseHeaders)
				}
				scheduleModel := turnUpstreamModel
				if scheduleModel == "" {
					scheduleModel = turnRequestedModel
				}
				h.gatewayService.ObserveOpenAIAccountResult(account, scheduleModel, openAIForwardSucceededForScheduling(result))
				inboundEndpoint := GetInboundEndpoint(c)
				upstreamEndpoint := resolveOpenAIUpstreamEndpoint(c, account, result)
				sessionID := service.ExtractClientSessionID(c)
				turnRecordPricingAt := turnPricing.currentOr(turnStart)
				cyberBlocked := service.GetOpsCyberPolicy(c) != nil
				h.submitOpenAIUsageRecordTask(ctx, result, func(taskCtx context.Context) {
					if err := h.gatewayService.RecordUsage(taskCtx, &service.OpenAIRecordUsageInput{
						Result:             result,
						APIKey:             apiKey,
						User:               apiKey.User,
						Account:            account,
						Subscription:       subscription,
						InboundEndpoint:    inboundEndpoint,
						UpstreamEndpoint:   upstreamEndpoint,
						UserAgent:          userAgent,
						IPAddress:          clientIP,
						RequestPayloadHash: requestPayloadHash,
						APIKeyService:      h.apiKeyService,
						SessionID:          sessionID,
						RequestedModel:     turnRequestedModel,
						PricingAt:          turnRecordPricingAt,
						CyberBlocked:       cyberBlocked,
					}); err != nil {
						reqLog.Error("openai.websocket_record_usage_failed",
							zap.Int64("account_id", account.ID),
							zap.String("request_id", result.RequestID),
							zap.Error(err),
						)
					}
				})
			},
		}

		wsFirstMessage := wsAttemptMessage
		// 切组/会话失配防护：previous_response_id 未在当前分组命中粘连账号（StickyPreviousHit=false），
		// 说明该会话链不属于本次调度到的账号，原样转发会触发上游会话链鉴权失败（“鉴权失败，请检查 API Key”）。
		// 故剥离首包里的 previous_response_id，改用首包内 input 重建上下文；带 function_call_output 的
		// 工具续链无法重建，保持原样。仅作用于首轮首包，后续 turn 的续链由 WS 转发层既有逻辑处理。
		if previousResponseID != "" && !stickyPreviousHit && previousResponseCanMove {
			wsFirstMessage = service.RemovePreviousResponseIDFromBody(wsFirstMessage)
			reqLog.Debug("openai.websocket_previous_response_id_stripped_cross_group",
				zap.Int64("account_id", account.ID),
			)
		}

		// WebSocket 首包可能很大，hash 必须在 hooks 外算成字符串，避免 AfterTurn 闭包保活请求体。
		requestPayloadHash = service.HashUsageRequestPayload(wsFirstMessage)
		if preemptCtx, cleanupPreempt, armed := h.gatewayService.BeginOpenAIWSIngressSessionPreemptionWithClient(ctx, c, account, wsFirstMessage, wsConn); armed {
			ctx = preemptCtx
			defer cleanupPreempt()
		}

		for {
			err := h.gatewayService.ProxyResponsesWebSocketFromClient(ctx, c, wsConn, account, token, wsFirstMessage, hooks)
			if err == nil {
				reqLog.Info("openai.websocket_ingress_closed", zap.Int64("account_id", account.ID))
				return
			}
			if service.IsOpenAIWSSessionPreemptedError(err) {
				// 关闭帧已由抢占登记在取消前发给本连接，这里只记录并释放。
				reqLog.Info("openai.websocket_ingress_preempted", zap.Int64("account_id", account.ID))
				return
			}
			var failoverErr *service.UpstreamFailoverError
			if errors.As(err, &failoverErr) {
				retryPayload, retryCurrentTurn := service.OpenAIWSCurrentTurnRetryPayload(err)
				nextAttemptMessage, retrySafe := openAIWSNextAttemptMessage(wsAttemptMessage, retryPayload, retryCurrentTurn)
				if !retrySafe {
					closeOpenAIWSFailoverExhausted(c, wsConn, failoverErr)
					return
				}
				wsAttemptMessage = nextAttemptMessage
				if retryCurrentTurn {
					previousResponseID = ""
					reqLog.Warn("openai.websocket_current_turn_failover_retry",
						zap.Int64("account_id", account.ID),
						zap.Int("upstream_status", failoverErr.StatusCode),
						zap.Int("retry_payload_bytes", len(retryPayload)),
					)
				}
				if waitForWSSameAccountRetry(account, failoverErr) {
					if failoverErr.ShouldReportAccountScheduleFailure() {
						h.gatewayService.ObserveOpenAIAccountResult(account, openAIAccountScheduleModel(c, account, wsForwardModel, false, nil), false, err)
					}
					if !ensureUserSlotHeld() {
						return
					}
					if currentAccountRelease == nil {
						accountRelease, acquired, acquireErr := h.concurrencyHelper.TryAcquireAccountSlot(ctx, account.ID, accountMaxConcurrency)
						if acquireErr != nil || !acquired {
							reqLog.Warn("openai.websocket_same_account_retry_slot_unavailable",
								zap.Int64("account_id", account.ID),
								zap.Error(acquireErr),
							)
							closeOpenAIClientWS(wsConn, coderws.StatusTryAgainLater, "account is busy, please retry later")
							return
						}
						currentAccountRelease = wrapReleaseOnDone(ctx, accountRelease)
					}
					wsFirstMessage = wsAttemptMessage
					continue
				}
				if handleWSFailover(account, failoverErr) {
					break
				}
				return
			}

			if errors.Is(context.Cause(ctx), service.ErrOpenAIWSIngressLeaseLost) {
				reqLog.Warn("openai.websocket_ingress_lease_lost",
					zap.Int64("account_id", account.ID),
					zap.Error(err),
				)
				closeOpenAIClientWS(wsConn, coderws.StatusTryAgainLater, "websocket ingress capacity lease lost; please reconnect")
				return
			}

			var closeErr *service.OpenAIWSClientCloseError
			hasClientCloseErr := errors.As(err, &closeErr)
			if openAIWSIngressEndedByClient(err) {
				closedFields := []zap.Field{zap.Int64("account_id", account.ID)}
				if hasClientCloseErr {
					closedFields = append(closedFields, zap.String("reason", closeErr.Reason()))
				} else {
					closedFields = append(closedFields, zap.Error(err))
				}
				reqLog.Info("openai.websocket_ingress_closed_normally", closedFields...)
				// A bare coderws.CloseError or a plain cancellation carries no
				// gateway-chosen close frame; mirror the client's clean 1000
				// rather than the 1011 the proxy-failure tail would have sent.
				if hasClientCloseErr {
					closeOpenAIClientWS(wsConn, closeErr.StatusCode(), closeErr.Reason())
				} else {
					closeOpenAIClientWS(wsConn, coderws.StatusNormalClosure, "")
				}
				return
			}

			if shouldReportOpenAIWSProxyAccountFailure(err) {
				h.gatewayService.ObserveOpenAIAccountResult(account, openAIAccountScheduleModel(c, account, wsForwardModel, false, nil), false, err)
			}
			closeStatus, closeReason := summarizeWSCloseErrorForLog(err)
			proxyFailedFields := []zap.Field{
				zap.Int64("account_id", account.ID),
				zap.Error(err),
				zap.String("close_status", closeStatus),
				zap.String("close_reason", closeReason),
			}
			if account.Proxy != nil {
				proxyFailedFields = append(proxyFailedFields,
					zap.Int64("proxy_id", account.Proxy.ID),
					zap.String("proxy_name", account.Proxy.Name),
					zap.String("proxy_host", account.Proxy.Host),
					zap.Int("proxy_port", account.Proxy.Port),
				)
			} else if account.ProxyID != nil {
				proxyFailedFields = append(proxyFailedFields, zap.Int64p("proxy_id", account.ProxyID))
			}
			reqLog.Warn("openai.websocket_proxy_failed", proxyFailedFields...)
			if hasClientCloseErr {
				closeOpenAIClientWS(wsConn, closeErr.StatusCode(), closeErr.Reason())
				return
			}
			closeOpenAIClientWS(wsConn, coderws.StatusInternalError, "upstream websocket proxy failed")
			return
		}
	}

}

func (h *OpenAIGatewayHandler) recoverResponsesPanic(c *gin.Context, streamStarted *bool) {
	recoverForwardPanic(c, streamStarted, recover(), h.ensureForwardErrorResponse, "handler.openai_gateway.responses", "openai.responses_panic_recovered")
}

func (h *OpenAIGatewayHandler) ensureResponsesDependencies(c *gin.Context, reqLog *zap.Logger) bool {
	missing := h.missingResponsesDependencies()
	if len(missing) == 0 {
		return true
	}

	if reqLog == nil {
		reqLog = requestLogger(c, "handler.openai_gateway.responses")
	}
	reqLog.Error("openai.handler_dependencies_missing", zap.Strings("missing_dependencies", missing))

	if c != nil && c.Writer != nil && !c.Writer.Written() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{
				"type":    "api_error",
				"message": "Service temporarily unavailable",
			},
		})
	}
	return false
}

func (h *OpenAIGatewayHandler) missingResponsesDependencies() []string {
	missing := make([]string, 0, 5)
	if h == nil {
		return append(missing, "handler")
	}
	if h.gatewayService == nil {
		missing = append(missing, "gatewayService")
	}
	if h.billingCacheService == nil {
		missing = append(missing, "billingCacheService")
	}
	if h.apiKeyService == nil {
		missing = append(missing, "apiKeyService")
	}
	if h.concurrencyHelper == nil || h.concurrencyHelper.concurrencyService == nil {
		missing = append(missing, "concurrencyHelper")
	}
	return missing
}

func getContextInt64(c *gin.Context, key string) (int64, bool) {
	if c == nil || key == "" {
		return 0, false
	}
	v, ok := c.Get(key)
	if !ok {
		return 0, false
	}
	switch t := v.(type) {
	case int64:
		return t, true
	case int:
		return int64(t), true
	case int32:
		return int64(t), true
	case float64:
		return int64(t), true
	default:
		return 0, false
	}
}

func (h *OpenAIGatewayHandler) submitUsageRecordTask(parent context.Context, task service.UsageRecordTask) {
	if task == nil {
		return
	}
	task = wrapUsageRecordTaskContext(parent, task)
	if h.usageRecordWorkerPool != nil {
		if mode := h.usageRecordWorkerPool.Submit(task); mode != service.UsageRecordSubmitModeDroppedStopped {
			return
		}
		// 池已停止（进程关停窗口）：计费任务不能静默丢失，降级为内联同步执行。
		// 显式配置的 drop/sample 溢出丢弃仍按配置语义保留。
		logger.L().With(
			zap.String("component", "handler.openai_gateway.responses"),
		).Warn("openai.usage_record_task_stopped_sync_fallback")
	}
	// 回退路径：worker 池未注入或已停止时同步执行，避免退回到无界 goroutine 模式。
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.L().With(
				zap.String("component", "handler.openai_gateway.responses"),
				zap.Any("panic", recovered),
			).Error("openai.usage_record_task_panic_recovered")
		}
	}()
	task(ctx)
}

func (h *OpenAIGatewayHandler) submitOpenAIUsageRecordTask(parent context.Context, result *service.OpenAIForwardResult, task service.UsageRecordTask) {
	// Money-critical bills never drop on pool overflow: media, search surcharge, voice.
	if result != nil && (result.ImageCount > 0 || result.VideoCount > 0 ||
		result.SearchCount > 0 || result.WebSearchCalls > 0 || result.AudioUsage != nil) {
		h.submitMandatoryUsageRecordTask(parent, task)
		return
	}
	h.submitUsageRecordTask(parent, task)
}

func (h *OpenAIGatewayHandler) submitMandatoryUsageRecordTask(parent context.Context, task service.UsageRecordTask) {
	if task == nil {
		return
	}
	task = wrapUsageRecordTaskContext(parent, task)
	if h.usageRecordWorkerPool != nil {
		if mode := h.usageRecordWorkerPool.Submit(task); !mode.Dropped() {
			return
		}
		logger.L().With(
			zap.String("component", "handler.openai_gateway.usage"),
		).Warn("openai.usage_record_task_mandatory_sync_fallback")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.L().With(
				zap.String("component", "handler.openai_gateway.usage"),
				zap.Any("panic", recovered),
			).Error("openai.usage_record_task_panic_recovered")
		}
	}()
	task(ctx)
}

func (h *OpenAIGatewayHandler) acquireImageGenerationSlot(c *gin.Context, streamStarted bool) (func(), bool) {
	if h == nil {
		return nil, true
	}
	return acquireImageGenerationSlot(c, h.cfg, h.imageLimiter, streamStarted)
}

// handleConcurrencyError handles concurrency-related acquire errors.
func (h *OpenAIGatewayHandler) handleConcurrencyError(c *gin.Context, err error, slotType string, streamStarted bool) {
	status, errType, code, message := concurrencyErrorResponse(err, slotType)
	h.handleStreamingAwareErrorWithCode(c, status, errType, code, message, streamStarted, false)
}

func (h *OpenAIGatewayHandler) handleFailoverExhausted(c *gin.Context, failoverErr *service.UpstreamFailoverError, streamStarted bool) {
	if failoverErr == nil {
		h.handleFailoverExhaustedSimple(c, http.StatusBadGateway, streamStarted)
		return
	}
	if failoverErr.IsOpenAIRequestBodyTooLarge() {
		service.SetOpsUpstreamError(c, http.StatusRequestEntityTooLarge, service.OpenAIRequestBodyTooLargeClientMessage, "")
		h.handleStreamingAwareError(
			c,
			http.StatusRequestEntityTooLarge,
			"invalid_request_error",
			service.OpenAIRequestBodyTooLargeClientMessage,
			streamStarted,
		)
		return
	}
	if failoverErr.Reason == service.OpenAIHTTPContinuationUnsupportedReason {
		message := strings.TrimSpace(failoverErr.ClientMessage)
		if message == "" {
			message = "previous_response_id requires an OpenAI API-key account for HTTP requests"
		}
		h.handleStreamingAwareError(c, http.StatusBadRequest, "invalid_request_error", message, streamStarted)
		return
	}
	copyFailoverRetryAfter(c, failoverErr.ResponseHeaders)
	if failoverErr.IsCredentialFailure() {
		status, message := credentialFailoverClientResponse(failoverErr)
		h.handleStreamingAwareError(c, status, "upstream_error", message, streamStarted)
		return
	}
	if failoverErr.IsOpenAICapacityShed() && strings.TrimSpace(failoverErr.ClientMessage) != "" {
		status := failoverErr.ClientStatusCode
		if status <= 0 {
			status = http.StatusServiceUnavailable
		}
		h.handleStreamingAwareError(c, status, "server_error", failoverErr.ClientMessage, streamStarted)
		return
	}
	statusCode := failoverErr.StatusCode
	responseBody := failoverErr.ResponseBody
	if statusCode == http.StatusBadRequest && service.IsOpenAICompatibleModelNotFound400(responseBody) && !streamStarted {
		upstreamMsg := service.SanitizeUpstreamErrorMessage(service.ExtractUpstreamErrorMessage(responseBody))
		service.SetOpsUpstreamError(c, statusCode, upstreamMsg, "")
		service.WriteOpenAIUpstreamClientError(c, statusCode, responseBody, upstreamMsg)
		return
	}
	if service.IsOpenAISilentRefusalErrorBody(responseBody) {
		service.SetOpsUpstreamError(c, statusCode, service.OpenAISilentRefusalClientMessage(), "")
		h.handleStreamingAwareError(c, http.StatusBadGateway, "upstream_error", service.OpenAISilentRefusalClientMessage(), streamStarted)
		return
	}

	// 先检查透传规则
	if h.errorPassthroughService != nil && len(responseBody) > 0 {
		if rule := h.errorPassthroughService.MatchRule("openai", statusCode, responseBody); rule != nil {
			// 确定响应状态码
			respCode := statusCode
			if !rule.PassthroughCode && rule.ResponseCode != nil {
				respCode = *rule.ResponseCode
			}

			// 确定响应消息
			msg := service.ExtractUpstreamErrorMessage(responseBody)
			if !rule.PassthroughBody && rule.CustomMessage != nil {
				msg = *rule.CustomMessage
			}

			if rule.SkipMonitoring {
				c.Set(service.OpsSkipPassthroughKey, true)
			}

			h.handleStreamingAwareError(c, respCode, "upstream_error", msg, streamStarted)
			return
		}
	}

	// 记录原始上游状态码，以便 ops 错误日志捕获真实的上游错误
	upstreamMsg := service.ExtractUpstreamErrorMessage(responseBody)
	service.SetOpsUpstreamError(c, statusCode, upstreamMsg, "")

	// 使用默认的错误映射
	status, errType, errMsg := h.mapUpstreamError(statusCode)
	h.handleStreamingAwareError(c, status, errType, errMsg, streamStarted)
}

func credentialFailoverClientResponse(failoverErr *service.UpstreamFailoverError) (int, string) {
	if failoverErr != nil && failoverErr.Reason == service.OpenAIUpstreamAccessStateReason && strings.TrimSpace(failoverErr.ClientMessage) != "" {
		status := failoverErr.ClientStatusCode
		if status <= 0 {
			status = http.StatusServiceUnavailable
		}
		return status, failoverErr.ClientMessage
	}
	if failoverErr != nil && failoverErr.Reason == service.AntigravityCredentialRejectedReason {
		return http.StatusBadGateway, service.AntigravityCredentialRejectedClientMessage
	}
	return http.StatusServiceUnavailable, service.GrokCredentialUnavailableClientMessage
}

func copyFailoverRetryAfter(c *gin.Context, headers http.Header) {
	if c == nil || headers == nil {
		return
	}
	retryAfter := strings.TrimSpace(headers.Get("Retry-After"))
	if retryAfter == "" || len(retryAfter) > 128 || strings.ContainsAny(retryAfter, "\r\n") || !isSafeRetryAfter(retryAfter) {
		return
	}
	c.Header("Retry-After", retryAfter)
}

func isSafeRetryAfter(value string) bool {
	digitsOnly := true
	for _, char := range value {
		if char < '0' || char > '9' {
			digitsOnly = false
			break
		}
	}
	if digitsOnly {
		seconds, err := strconv.ParseUint(value, 10, 32)
		return err == nil && seconds <= uint64((7*24*time.Hour)/time.Second)
	}
	retryAt, err := http.ParseTime(value)
	if err != nil {
		return false
	}
	return !retryAt.After(time.Now().Add(7 * 24 * time.Hour))
}

// handleFailoverExhaustedSimple 简化版本，用于没有响应体的情况
func (h *OpenAIGatewayHandler) handleFailoverExhaustedSimple(c *gin.Context, statusCode int, streamStarted bool) {
	status, errType, errMsg := h.mapUpstreamError(statusCode)
	service.SetOpsUpstreamError(c, statusCode, errMsg, "")
	h.handleStreamingAwareError(c, status, errType, errMsg, streamStarted)
}

func (h *OpenAIGatewayHandler) mapUpstreamError(statusCode int) (int, string, string) {
	switch statusCode {
	case 401:
		return http.StatusBadGateway, "upstream_error", "Upstream authentication failed, please contact administrator"
	case 403:
		return http.StatusBadGateway, "upstream_error", "Upstream access forbidden, please contact administrator"
	case 429:
		return http.StatusTooManyRequests, "rate_limit_error", "Upstream rate limit exceeded, please retry later"
	case 529:
		return http.StatusServiceUnavailable, "upstream_error", "Upstream service overloaded, please retry later"
	case 500, 502, 503, 504:
		return http.StatusBadGateway, "upstream_error", "Upstream service temporarily unavailable"
	default:
		return http.StatusBadGateway, "upstream_error", "Upstream request failed"
	}
}

// handleStreamingAwareError handles errors that may occur after streaming has started
func (h *OpenAIGatewayHandler) handleStreamingAwareError(c *gin.Context, status int, errType, message string, streamStarted bool) {
	h.handleStreamingAwareErrorWithCode(c, status, errType, "", message, streamStarted, false)
}

func (h *OpenAIGatewayHandler) handleStreamingAwareErrorWithCode(c *gin.Context, status int, errType string, code string, message string, streamStarted bool, countTowardsSLA bool) {
	openAIStreamingAwareError(c, status, errType, code, message, streamStarted, countTowardsSLA)
}

// ensureForwardErrorResponse 在 Forward 返回错误但尚未写响应时补写统一错误响应。
func (h *OpenAIGatewayHandler) ensureForwardErrorResponse(c *gin.Context, streamStarted bool) bool {
	if c == nil || c.Writer == nil {
		return false
	}
	if c.Request != nil && errors.Is(c.Request.Context().Err(), context.Canceled) {
		failoverClientGone(c)
		return false
	}
	// 先停 compact 心跳再读 Writer 状态，避免与心跳 goroutine 竞争。
	compactKeepaliveCommitted := service.StopOpenAICompactSSEKeepaliveCommitted(c)
	if compactKeepaliveCommitted {
		streamStarted = true
	}
	imageKeepalivePresent := service.OpenAIImagesJSONKeepalivePresent(c)
	service.StopOpenAIImagesJSONKeepaliveCommitted(c)
	imageKeepalivePaddingOnly := false
	imageKeepaliveResponseWritten := false
	if imageKeepalivePresent {
		adjustedSize := service.OpenAIImagesJSONKeepaliveAdjustedWrittenSize(c)
		imageKeepalivePaddingOnly = adjustedSize < 0
		imageKeepaliveResponseWritten = adjustedSize >= 0
	}
	compactKeepaliveHasMeaningfulOutput := compactKeepaliveCommitted && service.OpenAICompactKeepaliveAdjustedWrittenSize(c) > 0
	// Compact keepalive may have committed 200 headers without writing a
	// semantic SSE event. In that case the Responses stream still needs its
	// protocol-correct terminal response.failed event.
	if (service.IsResponseCommitted(c) && (!compactKeepaliveCommitted || compactKeepaliveHasMeaningfulOutput)) || (!compactKeepaliveCommitted && imageKeepaliveResponseWritten) {
		return false
	}
	if c.Writer.Written() && !imageKeepalivePaddingOnly {
		streamStarted = true
	}
	h.handleStreamingAwareError(c, http.StatusBadGateway, "upstream_error", "Upstream request failed", streamStarted)
	return true
}

func shouldLogOpenAIForwardFailureAsWarn(c *gin.Context, wroteFallback bool) bool {
	if wroteFallback {
		return false
	}
	if c == nil || c.Writer == nil {
		return false
	}
	return c.Writer.Written()
}

// openAIForwardErrorAlreadyCommunicated reports whether Forward returned an
// error after it had already written the upstream terminal error response to
// the client.
//
// This matters for Responses streams: upstream may return HTTP 200 with a
// non-retryable `response.failed` event (for example a policy/safety rejection).
// The service layer forwards that terminal event verbatim, then returns an
// error so the caller can log/account for the failed upstream response. The
// handler must not append its generic fallback `response.failed`, otherwise
// strict clients may see the useful upstream message replaced by "Upstream
// request failed" or receive duplicate terminal events.
func openAIForwardErrorAlreadyCommunicated(c *gin.Context, writerSizeBeforeForward int, err error) bool {
	if err == nil || c == nil || c.Writer == nil {
		return false
	}
	// 与快照同口径：排除 compact 心跳字节，避免"仅心跳写出"被误判为
	// 响应已写出（#3887）。
	if service.OpenAICompactKeepaliveAdjustedWrittenSize(c) == writerSizeBeforeForward ||
		service.OpenAIImagesJSONKeepaliveAdjustedWrittenSize(c) == writerSizeBeforeForward {
		return false
	}

	// cyber_policy 命中时上游原始错误体已透传给客户端（非流式 c.Data 写出 400 body，
	// 流式写出 response.failed 事件），不能再让 ensureForwardErrorResponse 追加
	// fallback —— 否则在已写出的完整响应尾部追加 SSE（responses 端点尾随
	// response.failed、chat 端点尾随 event:error），污染响应体。Size 已变化证明响应确已写出。
	if service.GetOpsCyberPolicy(c) != nil {
		return true
	}

	msg := strings.TrimSpace(err.Error())
	for _, prefix := range []string{
		"upstream response failed:",
		"non-streaming openai protocol error:",
	} {
		if strings.HasPrefix(msg, prefix) {
			return true
		}
	}
	return false
}

func openAIForwardMayFailover(c *gin.Context, writerSizeBeforeForward int, failoverErr *service.UpstreamFailoverError) bool {
	if c == nil || c.Writer == nil {
		return false
	}
	if service.OpenAICompactKeepaliveAdjustedWrittenSize(c) == writerSizeBeforeForward {
		return true
	}
	return failoverErr != nil && failoverErr.SafeToFailoverAfterWrite
}

func openAIFirstOutputFailoverExhausted(failoverErr *service.UpstreamFailoverError, switchCount *int) bool {
	if failoverErr == nil || !failoverErr.SafeToFailoverAfterWrite || switchCount == nil {
		return false
	}
	if *switchCount >= maxOpenAIFirstOutputTimeoutSwitches {
		return true
	}
	*switchCount = *switchCount + 1
	return false
}

// errorResponse returns OpenAI API format error response
func (h *OpenAIGatewayHandler) errorResponse(c *gin.Context, status int, errType, message string) {
	// body-signal compact 心跳可能已把响应头提交为 200：JSON 错误体会与已
	// 提交的 SSE 流交错，必须降级为 response.failed 终止事件（#3887）。
	if service.StopOpenAICompactSSEKeepaliveCommitted(c) {
		service.MarkOpsStreamError(c, errType, message, status)
		if writeResponsesFailedSSE(c, errType, "", message) {
			return
		}
	}
	c.JSON(status, gin.H{
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}

func setOpenAIClientTransportHTTP(c *gin.Context) {
	service.SetOpenAIClientTransport(c, service.OpenAIClientTransportHTTP)
}

func setOpenAIClientTransportWS(c *gin.Context) {
	service.SetOpenAIClientTransport(c, service.OpenAIClientTransportWS)
}

func ensureOpenAIPoolModeSessionHash(sessionHash string, account *service.Account) string {
	if sessionHash != "" || account == nil || !account.IsPoolMode() {
		return sessionHash
	}
	// 为当前请求生成一次性粘性会话键，确保同账号重试不会重新负载均衡到其他账号。
	return "openai-pool-retry-" + uuid.NewString()
}

func openAIWSIngressFallbackSessionSeed(userID, apiKeyID int64) string {
	return fmt.Sprintf("openai_ws_ingress:%d:%d", userID, apiKeyID)
}

func isOpenAIWSUpgradeRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket") {
		return false
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(r.Header.Get("Connection"))), "upgrade")
}

// blockedModelAllowlistCandidate 对全部候选模型逐一校验分组白名单，返回第一个
// 未命中的值（全部命中或白名单未开启返回空串）。WS 帧与 HTTP 请求体共用该
// 规则：重复 model 键/大小写变体可能被上游按末值绑定，任一未命中即拒绝。
func closeOpenAIClientWS(conn *coderws.Conn, status coderws.StatusCode, reason string) {
	if conn == nil {
		return
	}
	reason = strings.TrimSpace(reason)
	if len(reason) > 120 {
		reason = reason[:120]
	}
	_ = conn.Close(status, reason)
	_ = conn.CloseNow()
}

func openAIWSNextAttemptMessage(current, retryPayload []byte, retryCurrentTurn bool) ([]byte, bool) {
	if !retryCurrentTurn {
		return append([]byte(nil), current...), true
	}
	if len(retryPayload) == 0 {
		return nil, false
	}
	return append([]byte(nil), retryPayload...), true
}

func closeOpenAIWSFailoverExhausted(c *gin.Context, conn *coderws.Conn, failoverErr *service.UpstreamFailoverError) {
	intendedStatus := http.StatusBadGateway
	errorType := "upstream_error"
	errorCode := "upstream_ws_failover_exhausted"
	message := "upstream websocket proxy failed"
	closeStatus := coderws.StatusInternalError

	if failoverErr != nil {
		if reason := strings.TrimSpace(string(failoverErr.Reason)); reason != "" {
			errorCode = reason
		}
		if failoverErr.Stage == service.GatewayFailureStageAccountAuth {
			intendedStatus = http.StatusServiceUnavailable
			errorType = "api_error"
			message = service.GrokCredentialUnavailableClientMessage
			closeStatus = coderws.StatusTryAgainLater
		} else {
			switch failoverErr.StatusCode {
			case http.StatusTooManyRequests:
				intendedStatus = http.StatusTooManyRequests
				errorType = "rate_limit_error"
				message = "upstream rate limit exceeded, please retry later"
				closeStatus = coderws.StatusTryAgainLater
			case 529, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
				intendedStatus = failoverErr.StatusCode
				message = "upstream service temporarily unavailable"
				closeStatus = coderws.StatusTryAgainLater
			case http.StatusUnauthorized, http.StatusForbidden:
				intendedStatus = failoverErr.StatusCode
				errorType = "authentication_error"
				message = "upstream websocket authentication failed"
				closeStatus = coderws.StatusPolicyViolation
			}
		}
	}

	service.MarkOpsStreamFailure(c, errorType, errorCode, message, intendedStatus)
	closeOpenAIClientWS(conn, closeStatus, message)
}

func writeContentModerationWSError(ctx context.Context, conn *coderws.Conn, decision *service.ContentModerationDecision) {
	if conn == nil || decision == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	message := strings.TrimSpace(decision.Message)
	if message == "" {
		message = "content moderation blocked this request"
	}
	payload, err := json.Marshal(gin.H{
		"event_id": "evt_content_moderation_blocked",
		"type":     "error",
		"error": gin.H{
			"type":    "invalid_request_error",
			"code":    contentModerationErrorCode(decision),
			"message": message,
		},
	})
	if err != nil {
		payload = []byte(`{"event_id":"evt_content_moderation_blocked","type":"error","error":{"type":"invalid_request_error","code":"content_policy_violation","message":"content moderation blocked this request"}}`)
	}
	writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = conn.Write(writeCtx, coderws.MessageText, payload)
}

// writeCyberSessionBlockedWSError sends an error frame telling the client this
// session is blocked by the cyber session block (F5a) before closing.
func writeCyberSessionBlockedWSError(ctx context.Context, conn *coderws.Conn) {
	if conn == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	payload, err := json.Marshal(gin.H{
		"event_id": "evt_cyber_session_blocked",
		"type":     "error",
		"error": gin.H{
			"type":    "permission_error",
			"code":    "session_blocked_by_cyber_policy",
			"message": cyberSessionBlockedClientMsg,
		},
	})
	if err != nil {
		payload = []byte(`{"event_id":"evt_cyber_session_blocked","type":"error","error":{"type":"permission_error","code":"session_blocked_by_cyber_policy","message":"This session is blocked by cyber-security policy, please start a new session"}}`)
	}
	writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = conn.Write(writeCtx, coderws.MessageText, payload)
}

// cyberPolicyRecordedKey guards against double-firing recordCyberPolicyIfMarked
// within one request (e.g. in a retry/failover loop).
const cyberPolicyRecordedKey = "ops_cyber_recorded"

// cyberPolicyOpsErrorMeta carries request-scoped fields captured outside the
// async goroutine for building the cyber ops_error_logs entry.
type cyberPolicyOpsErrorMeta struct {
	RequestID       string
	ClientRequestID string
	Platform        string
	Model           string
	RequestPath     string
	Stream          bool
	InboundEndpoint string
	UserAgent       string
	APIKeyPrefix    string
	UserID          int64
	APIKeyID        int64
	AccountID       int64
	GroupID         *int64
	ClientIP        string
	CreatedAt       time.Time
	SessionBlockKey string
}

// buildCyberPolicyOpsErrorEntry builds the ops_error_logs entry for an upstream
// cyber_policy hit. StatusCode mirrors what the codex client actually received
// (400 non-stream / 200 stream), per F6.
func buildCyberPolicyOpsErrorEntry(meta cyberPolicyOpsErrorMeta, mark *service.CyberPolicyMark) *service.OpsInsertErrorLogInput {
	rt := int16(service.RequestTypeCyberBlocked)
	entry := &service.OpsInsertErrorLogInput{
		RequestID:         meta.RequestID,
		ClientRequestID:   meta.ClientRequestID,
		Platform:          meta.Platform,
		Model:             meta.Model,
		RequestPath:       meta.RequestPath,
		Stream:            meta.Stream,
		InboundEndpoint:   meta.InboundEndpoint,
		RequestType:       &rt,
		UserAgent:         meta.UserAgent,
		APIKeyPrefix:      meta.APIKeyPrefix,
		ErrorPhase:        "request",
		ErrorType:         "cyber_policy",
		Severity:          "P3",
		StatusCode:        mark.UpstreamStatus,
		IsBusinessLimited: true,
		ErrorMessage:      "cyber_policy: " + mark.Message,
		// 原始 body 直接入队；ops service 落库前统一走 sanitizeErrorBodyForStorage 脱敏与截断。
		ErrorBody:   mark.Body,
		ErrorSource: "upstream_http",
		ErrorOwner:  "provider",
		CreatedAt:   meta.CreatedAt,
	}
	if meta.UserID > 0 {
		entry.UserID = &meta.UserID
	}
	if meta.APIKeyID > 0 {
		entry.APIKeyID = &meta.APIKeyID
	}
	if meta.AccountID > 0 {
		entry.AccountID = &meta.AccountID
	}
	if meta.ClientIP != "" {
		entry.ClientIP = &meta.ClientIP
	}
	return entry
}

// 双语单串：网关客户端面向中英用户，且本错误无 i18n 协商通道。
const cyberSessionBlockedClientMsg = "该会话已被网络安全策略屏蔽，请开启新会话 / This session is blocked by cyber-security policy, please start a new session"

// buildCyberSessionBlockedOpsEntry builds the ops_error_logs entry for a request
// rejected locally by the cyber session block (F5a). Distinct error_type from
// upstream `cyber_policy`; never feeds moderation logs / violation counting
// (the request never reached upstream — see spec).
func buildCyberSessionBlockedOpsEntry(meta cyberPolicyOpsErrorMeta) *service.OpsInsertErrorLogInput {
	rt := int16(service.RequestTypeCyberBlocked)
	entry := &service.OpsInsertErrorLogInput{
		RequestID:         meta.RequestID,
		ClientRequestID:   meta.ClientRequestID,
		Platform:          meta.Platform,
		Model:             meta.Model,
		RequestPath:       meta.RequestPath,
		Stream:            meta.Stream,
		InboundEndpoint:   meta.InboundEndpoint,
		RequestType:       &rt,
		UserAgent:         meta.UserAgent,
		APIKeyPrefix:      meta.APIKeyPrefix,
		ErrorPhase:        "request",
		ErrorType:         "cyber_policy_session_blocked",
		Severity:          "P3",
		StatusCode:        http.StatusForbidden,
		IsBusinessLimited: true,
		ErrorMessage:      "cyber_policy_session_blocked: request rejected locally by session block",
		ErrorSource:       "gateway_local",
		ErrorOwner:        "platform",
		CreatedAt:         meta.CreatedAt,
		// AccountID 有意不设：请求在账号选择前即被拒绝。
	}
	if meta.SessionBlockKey != "" {
		entry.ErrorBody = "session_block_key=" + meta.SessionBlockKey
	}
	if meta.UserID > 0 {
		entry.UserID = &meta.UserID
	}
	if meta.APIKeyID > 0 {
		entry.APIKeyID = &meta.APIKeyID
	}
	if meta.ClientIP != "" {
		entry.ClientIP = &meta.ClientIP
	}
	return entry
}

// cyberSessionBlockFormat selects the per-endpoint error envelope for a locally
// blocked session (用户决策：兼容路径各自格式).
type cyberSessionBlockFormat int

const (
	cyberBlockFormatResponses cyberSessionBlockFormat = iota
	cyberBlockFormatChat
	cyberBlockFormatAnthropic
)

type cyberSessionBlockWritePlan struct {
	scopeKey string
	keys     []string
}

func buildCyberSessionBlockWritePlan(apiKeyID int64, c *gin.Context, body []byte) cyberSessionBlockWritePlan {
	plan := cyberSessionBlockWritePlan{}
	if key := service.CyberSessionExplicitBlockKey(apiKeyID, c, body); key != "" {
		plan.keys = append(plan.keys, key)
	}
	transcriptKeys := service.CyberSessionTranscriptBlockKeys(apiKeyID, body)
	for _, key := range transcriptKeys {
		if len(plan.keys) == 0 || key != plan.keys[0] {
			plan.keys = append(plan.keys, key)
		}
	}
	if len(transcriptKeys) > 0 {
		plan.scopeKey = cyberSessionScopeKey(apiKeyID, c)
	}
	return plan
}

func findBlockedCyberSessionKey(ctx context.Context, gatewayService *service.OpenAIGatewayService, apiKeyID int64, c *gin.Context, body []byte) string {
	if gatewayService == nil {
		return ""
	}
	clientIP, userAgent := "", ""
	if c != nil {
		clientIP = strings.TrimSpace(ip.GetClientIP(c))
		userAgent = c.GetHeader("User-Agent")
	}
	return gatewayService.FindCyberSessionBlockedForRequest(ctx, apiKeyID, c, body, clientIP, userAgent)
}

func cyberSessionScopeKey(apiKeyID int64, c *gin.Context) string {
	if c == nil {
		return ""
	}
	return service.CyberSessionScopeKey(apiKeyID, strings.TrimSpace(ip.GetClientIP(c)), c.GetHeader("User-Agent"))
}

func (h *OpenAIGatewayHandler) cyberPolicyDeps() cyberPolicyDeps {
	return cyberPolicyDeps{contentModeration: h.contentModerationService, openAIGateway: h.gatewayService, ops: h.opsService, apiKeys: h.apiKeyService}
}

func (h *OpenAIGatewayHandler) rejectIfCyberSessionBlocked(c *gin.Context, apiKey *service.APIKey, body []byte, model string, format cyberSessionBlockFormat) bool {
	if h == nil {
		return false
	}
	return rejectIfCyberSessionBlocked(c, h.cyberPolicyDeps(), apiKey, body, model, format)
}

func (h *OpenAIGatewayHandler) recordCyberPolicyIfMarked(c *gin.Context, apiKey *service.APIKey, account *service.Account, subscription *service.UserSubscription, model string, forwardErrored bool, cyberBlockBody []byte, requestedModel, requestPayloadHash string) {
	recordCyberPolicyIfMarked(c, h.cyberPolicyDeps(), apiKey, account, subscription, model, forwardErrored, cyberBlockBody, requestedModel, requestPayloadHash)
}

// clearCyberPolicyTurnState resets the cyber mark and recorded guard after a
// logical WS turn has finished.
func clearCyberPolicyTurnState(c *gin.Context) {
	clearCyberPolicyAttemptState(c, true)
}

func clearCyberPolicyAttemptState(c *gin.Context, resetRecorded bool) {
	if c == nil {
		return
	}
	service.ClearOpsCyberPolicy(c)
	if resetRecorded {
		c.Set(cyberPolicyRecordedKey, false)
	}
}

func summarizeWSCloseErrorForLog(err error) (string, string) {
	if err == nil {
		return "-", "-"
	}
	statusCode := coderws.CloseStatus(err)
	if statusCode == -1 {
		return "-", "-"
	}
	closeStatus := fmt.Sprintf("%d(%s)", int(statusCode), statusCode.String())
	closeReason := "-"
	var closeErr coderws.CloseError
	if errors.As(err, &closeErr) {
		reason := strings.TrimSpace(closeErr.Reason)
		if reason != "" {
			closeReason = reason
		}
	}
	return closeStatus, closeReason
}
