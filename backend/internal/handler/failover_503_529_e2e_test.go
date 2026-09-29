//go:build unit

package handler

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 上游 503 / 529 的换号规则（2026-09-29 muqian 定）端到端：走真实 handler → 调度 → service Forward，
// 假上游按账号回固定状态码并记下每次调用。坏渠道只能被打 1 次，请求立即落到下一个渠道，
// 候选用完也不退避回头重打。

// failoverStatusUpstream 按账号回状态码：fail 表里的账号回 failStatus + failBody，其余回 200 + okBody。
type failoverStatusUpstream struct {
	mu            sync.Mutex
	accountIDs    []int64
	fail          map[int64]bool
	failStatus    int
	failBody      string
	okBody        string
	okContentType string
}

func (u *failoverStatusUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	if req.Body != nil {
		_, _ = io.Copy(io.Discard, req.Body)
	}
	u.mu.Lock()
	u.accountIDs = append(u.accountIDs, accountID)
	u.mu.Unlock()
	if u.fail[accountID] {
		return &http.Response{
			StatusCode: u.failStatus,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(u.failBody)),
		}, nil
	}
	contentType := u.okContentType
	if contentType == "" {
		contentType = "application/json"
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(u.okBody)),
	}, nil
}

func (u *failoverStatusUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func (u *failoverStatusUpstream) calls() []int64 {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]int64(nil), u.accountIDs...)
}

// overloadRecordingAccountRepo 只记 529 过载冷却（SetOverloaded）的调用。
type overloadRecordingAccountRepo struct {
	service.AccountRepository
	mu         sync.Mutex
	overloaded map[int64]time.Time
}

func (r *overloadRecordingAccountRepo) SetOverloaded(_ context.Context, id int64, until time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.overloaded == nil {
		r.overloaded = map[int64]time.Time{}
	}
	r.overloaded[id] = until
	return nil
}

func (r *overloadRecordingAccountRepo) overloadedIDs() map[int64]time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[int64]time.Time, len(r.overloaded))
	for id, until := range r.overloaded {
		out[id] = until
	}
	return out
}

// newFailoverE2EHandler 装一个 GatewayHandler：调度走快照（池 = accounts），Anthropic / OpenAI / Gemini
// 三个转发服务共用同一个假上游，换号上限 maxSwitches；Anthropic 转发的错误副作用走真实 RateLimitService。
func newFailoverE2EHandler(t *testing.T, accounts []*service.Account, upstream service.HTTPUpstream, maxSwitches int) *GatewayHandler {
	h, _ := newFailoverE2EHandlerWithRepo(t, accounts, upstream, maxSwitches)
	return h
}

func newFailoverE2EHandlerWithRepo(t *testing.T, accounts []*service.Account, upstream service.HTTPUpstream, maxSwitches int) (*GatewayHandler, *overloadRecordingAccountRepo) {
	t.Helper()
	cfg := &config.Config{RunMode: config.RunModeSimple}
	repo := &overloadRecordingAccountRepo{}
	schedulerSnapshot := service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: accounts}, nil, nil, nil)
	gwSvc := service.NewGatewayService(
		nil, nil, nil, nil, nil, nil, cfg,
		schedulerSnapshot,
		nil, nil,
		service.NewRateLimitService(repo, nil, cfg, nil, nil),
		nil, nil,
		upstream,
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	billingCacheSvc := service.NewBillingCacheService(nil, nil, nil, nil, nil, cfg)
	t.Cleanup(billingCacheSvc.Stop)
	openAISvc := service.NewOpenAIGatewayService(
		nil, &handlerUsageLogRepoStub{}, nil, handlerUserRepoStub{}, handlerSubRepoStub{}, nil, cfg, nil, nil,
		service.NewBillingService(cfg, nil), nil, &service.BillingCacheService{}, upstream,
		&service.DeferredService{}, nil, nil, nil, nil, nil,
		nil,
	)
	return &GatewayHandler{
		gatewayService:       gwSvc,
		openAIGatewayService: openAISvc,
		geminiCompatService:  service.NewGeminiMessagesCompatService(nil, nil, nil, nil, nil, upstream, nil, cfg),
		billingCacheService:  billingCacheSvc,
		apiKeyService:        service.NewAPIKeyService(nil, nil, nil, cfg),
		concurrencyHelper:    NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0),
		modelCatalog:         listAllCatalogStub{},
		maxAccountSwitches:   maxSwitches,
	}, repo
}

func failoverE2EKey(id int64, priority int, endpoints map[string]string, model string) *service.Account {
	account := keyRouteAccount(id, service.PlatformOpenAI, endpoints, model)
	account.Priority = priority
	return account
}

const (
	anthropicMessagesOK  = `{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":2}}`
	anthropicOverloaded  = `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`
	openAIServerOverload = `{"error":{"type":"server_error","message":"The server is overloaded, please try again later."}}`
)

// /v1/messages，两个 Anthropic 协议的 key：第一个恒 503 / 529，第二个正常。坏渠道只打 1 次就落到第二个。
func TestFailoverE2E_Messages_AnthropicKeyBadChannelHitOnce(t *testing.T) {
	anthropic := map[string]string{service.APIProtocolAnthropic: "https://anthropic-relay.example.com"}
	for _, status := range []int{http.StatusServiceUnavailable, 529} {
		upstream := &failoverStatusUpstream{fail: map[int64]bool{11: true}, failStatus: status, failBody: anthropicOverloaded, okBody: anthropicMessagesOK}
		h := newFailoverE2EHandler(t, []*service.Account{
			failoverE2EKey(11, 1, anthropic, "claude-sonnet-4-5"),
			failoverE2EKey(12, 2, anthropic, "claude-sonnet-4-5"),
		}, upstream, 3)

		body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
		c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/messages", body, service.APIProtocolAnthropic, "")
		start := time.Now()
		h.Messages(c)
		elapsed := time.Since(start)

		require.Equal(t, http.StatusOK, rec.Code, "status=%d body=%s", status, rec.Body.String())
		require.Equal(t, []int64{11, 12}, upstream.calls(), "status=%d 坏渠道只能打 1 次", status)
		require.Less(t, elapsed, time.Second, "status=%d 不得等待", status)
	}
}

// /v1/messages，两个渠道都恒 503：各打 1 次即按耗尽返回，不再「等 2s、清空排除列表、回头重打」。
func TestFailoverE2E_Messages_AllChannels503NoBackoffRetry(t *testing.T) {
	anthropic := map[string]string{service.APIProtocolAnthropic: "https://anthropic-relay.example.com"}
	upstream := &failoverStatusUpstream{fail: map[int64]bool{21: true, 22: true}, failStatus: http.StatusServiceUnavailable, failBody: anthropicOverloaded}
	h := newFailoverE2EHandler(t, []*service.Account{
		failoverE2EKey(21, 1, anthropic, "claude-sonnet-4-5"),
		failoverE2EKey(22, 2, anthropic, "claude-sonnet-4-5"),
	}, upstream, 3)

	body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/messages", body, service.APIProtocolAnthropic, "")
	start := time.Now()
	h.Messages(c)
	elapsed := time.Since(start)

	require.GreaterOrEqual(t, rec.Code, 500, rec.Body.String())
	require.Equal(t, []int64{21, 22}, upstream.calls(), "每个坏渠道只能打 1 次")
	require.Less(t, elapsed, time.Second, "候选用完不得退避 2s 再重试")
}

// /v1/messages 经 OpenAI 服务转发的 responses 协议 key：上游 503 / 529「server is overloaded」
// 在旧规则下会标成请求级容量、在原渠道同号重试；现在只打 1 次就换号。
func TestFailoverE2E_Messages_ResponsesKeyOverloadNoSameAccountRetry(t *testing.T) {
	const entryID = 199
	responses := map[string]string{service.APIProtocolResponses: "https://relay.example.com"}
	for _, status := range []int{http.StatusServiceUnavailable, 529} {
		upstream := &failoverStatusUpstream{fail: map[int64]bool{31: true}, failStatus: status, failBody: openAIServerOverload, okBody: openAIResponsesSSEOK, okContentType: "text/event-stream"}
		bad := failoverE2EKey(31, 1, responses, "gpt-5.6")
		good := failoverE2EKey(32, 2, responses, "gpt-5.6")
		bad.CatalogEntryIDs = []int64{entryID}
		good.CatalogEntryIDs = []int64{entryID}
		h := newFailoverE2EHandler(t, []*service.Account{bad, good}, upstream, 3)

		body := []byte(`{"model":"gpt-5.6","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
		c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/messages", body, service.APIProtocolAnthropic, "")
		openAIRouteEntry(c, entryID, "gpt-5.6")
		start := time.Now()
		h.Messages(c)
		elapsed := time.Since(start)

		require.Equal(t, http.StatusOK, rec.Code, "status=%d body=%s", status, rec.Body.String())
		require.Equal(t, []int64{31, 32}, upstream.calls(), "status=%d 不得在原渠道同号重试", status)
		require.Less(t, elapsed, time.Second, "status=%d 不得等待", status)
	}
}

// /v1/chat/completions（OpenAI 入口）：同上，503 / 529「server is overloaded」不同号重试。
func TestFailoverE2E_ChatCompletions_OverloadNoSameAccountRetry(t *testing.T) {
	const entryID = 199
	responses := map[string]string{service.APIProtocolResponses: "https://relay.example.com"}
	for _, status := range []int{http.StatusServiceUnavailable, 529} {
		upstream := &failoverStatusUpstream{fail: map[int64]bool{41: true}, failStatus: status, failBody: openAIServerOverload, okBody: openAIResponsesSSEOK, okContentType: "text/event-stream"}
		bad := failoverE2EKey(41, 1, responses, "gpt-5.6")
		good := failoverE2EKey(42, 2, responses, "gpt-5.6")
		bad.CatalogEntryIDs = []int64{entryID}
		good.CatalogEntryIDs = []int64{entryID}
		h := newFailoverE2EHandler(t, []*service.Account{bad, good}, upstream, 3)

		body := []byte(`{"model":"gpt-5.6","messages":[{"role":"user","content":"hello"}]}`)
		c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/chat/completions", body, service.APIProtocolChatCompletions, "")
		openAIRouteEntry(c, entryID, "gpt-5.6")
		start := time.Now()
		h.ChatCompletions(c)
		elapsed := time.Since(start)

		require.Equal(t, http.StatusOK, rec.Code, "status=%d body=%s", status, rec.Body.String())
		require.Equal(t, []int64{41, 42}, upstream.calls(), "status=%d 不得在原渠道同号重试", status)
		require.Less(t, elapsed, time.Second, "status=%d 不得等待", status)
	}
}
