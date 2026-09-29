//go:build unit

package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
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
// 三个转发服务共用同一个假上游，换号上限 maxSwitches；三者的错误副作用走同一个真实 RateLimitService。
func newFailoverE2EHandler(t *testing.T, accounts []*service.Account, upstream service.HTTPUpstream, maxSwitches int) *GatewayHandler {
	h, _ := newFailoverE2EHandlerWithRepo(t, accounts, upstream, maxSwitches)
	return h
}

func newFailoverE2EHandlerWithRepo(t *testing.T, accounts []*service.Account, upstream service.HTTPUpstream, maxSwitches int) (*GatewayHandler, *overloadRecordingAccountRepo) {
	t.Helper()
	cfg := &config.Config{RunMode: config.RunModeSimple}
	repo := &overloadRecordingAccountRepo{}
	rateLimitSvc := service.NewRateLimitService(repo, nil, cfg, nil, nil)
	schedulerSnapshot := service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: accounts}, nil, nil, nil)
	gwSvc := service.NewGatewayService(
		nil, nil, nil, nil, nil, nil, cfg,
		schedulerSnapshot,
		nil, nil,
		rateLimitSvc,
		nil, nil,
		upstream,
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	billingCacheSvc := service.NewBillingCacheService(nil, nil, nil, nil, nil, cfg)
	t.Cleanup(billingCacheSvc.Stop)
	openAISvc := service.NewOpenAIGatewayService(
		nil, &handlerUsageLogRepoStub{}, nil, handlerUserRepoStub{}, handlerSubRepoStub{}, nil, cfg, nil, nil,
		service.NewBillingService(cfg, nil), rateLimitSvc, &service.BillingCacheService{}, upstream,
		&service.DeferredService{}, nil, nil, nil, nil, nil,
		nil,
	)
	return &GatewayHandler{
		gatewayService:       gwSvc,
		openAIGatewayService: openAISvc,
		geminiCompatService:  service.NewGeminiMessagesCompatService(nil, nil, nil, nil, rateLimitSvc, upstream, nil, cfg),
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
		h, repo := newFailoverE2EHandlerWithRepo(t, []*service.Account{
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
		require.Empty(t, repo.overloadedIDs(), "status=%d 中转 key 不做过载冷却", status)
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

// /v1/messages，两个 Antigravity 成品号：第一个恒 503（MODEL_CAPACITY_EXHAUSTED），第二个正常。
// 旧逻辑在原渠道每秒重试、最多 60 次；现在坏渠道只打 1 次就落到第二个。
func TestFailoverE2E_Messages_AntigravityModelCapacity503HitOnce(t *testing.T) {
	antigravityAccount := func(id int64, priority int) *service.Account {
		return &service.Account{
			ID:          id,
			Name:        "ag-oauth",
			Platform:    service.PlatformAntigravity,
			Type:        service.AccountTypeOAuth,
			Credentials: map[string]any{"access_token": "tok", "project_id": "proj-1", "model_mapping": map[string]any{"claude-sonnet-4-5": "claude-sonnet-4-5"}},
			Concurrency: 1,
			Priority:    priority,
			Status:      service.StatusActive,
			Schedulable: true,
		}
	}
	modelCapacity503 := `{"error":{"code":503,"status":"UNAVAILABLE","message":"No capacity available","details":[` +
		`{"@type":"type.googleapis.com/google.rpc.ErrorInfo","metadata":{"model":"claude-sonnet-4-5"},"reason":"MODEL_CAPACITY_EXHAUSTED"},` +
		`{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"39s"}]}}`
	okSSE := `data: {"response":{"responseId":"resp_1","candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":1}}}` + "\n\n"
	upstream := &failoverStatusUpstream{fail: map[int64]bool{51: true}, failStatus: http.StatusServiceUnavailable, failBody: modelCapacity503, okBody: okSSE, okContentType: "text/event-stream"}
	h := newFailoverE2EHandler(t, []*service.Account{antigravityAccount(51, 1), antigravityAccount(52, 2)}, upstream, 3)
	h.antigravityGatewayService = service.NewAntigravityGatewayService(
		nil, nil, nil, service.NewAntigravityTokenProvider(nil, &fakeAntigravityTokenCache{token: "fresh"}, nil), nil, upstream,
		service.NewSettingService(nil, &config.Config{}), nil)

	body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/messages", body, service.APIProtocolAnthropic, "")
	start := time.Now()
	h.Messages(c)
	elapsed := time.Since(start)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, []int64{51, 52}, upstream.calls(), "坏渠道只能打 1 次")
	require.Less(t, elapsed, time.Second, "不得在原渠道等待")
}

// Gemini 协议的 key：第一个恒 503，第二个正常。旧逻辑在原渠道重试 5 次、退避 1+2+4+8s；
// 现在 /v1/messages（Claude 兼容）与 /v1beta（原生）都只打坏渠道 1 次就落到第二个。
func TestFailoverE2E_GeminiKey503HitOnce(t *testing.T) {
	const entryID = 9
	gemini := map[string]string{service.APIProtocolGemini: "https://gemini-relay.example.com"}
	gemini503 := `{"error":{"code":503,"message":"The model is overloaded. Please try again later.","status":"UNAVAILABLE"}}`
	newAccounts := func() []*service.Account {
		bad := failoverE2EKey(61, 1, gemini, "gemini-2.5-flash")
		good := failoverE2EKey(62, 2, gemini, "gemini-2.5-flash")
		bad.CatalogEntryIDs = []int64{entryID}
		good.CatalogEntryIDs = []int64{entryID}
		return []*service.Account{bad, good}
	}
	withGeminiRoute := func(c *gin.Context) {
		entry := &service.ModelCatalogEntry{ID: entryID, ModelID: "gemini-2.5-flash", Vendor: "gemini", Status: service.ModelCatalogStatusListed}
		c.Request = c.Request.WithContext(service.WithCatalogRoute(c.Request.Context(),
			service.CatalogRoute{EntryID: entryID, CanonicalModel: "gemini-2.5-flash", RequestedModel: "gemini-2.5-flash", Entry: entry}))
	}

	t.Run("/v1/messages", func(t *testing.T) {
		upstream := &failoverStatusUpstream{fail: map[int64]bool{61: true}, failStatus: http.StatusServiceUnavailable, failBody: gemini503, okBody: geminiGenerateContentOK}
		h := newFailoverE2EHandler(t, newAccounts(), upstream, 3)
		body := []byte(`{"model":"gemini-2.5-flash","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
		c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/messages", body, service.APIProtocolAnthropic, "")
		withGeminiRoute(c)

		start := time.Now()
		h.Messages(c)

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Equal(t, []int64{61, 62}, upstream.calls(), "坏渠道只能打 1 次")
		require.Less(t, time.Since(start), time.Second, "不得在原渠道退避")
	})

	t.Run("/v1beta", func(t *testing.T) {
		upstream := &failoverStatusUpstream{fail: map[int64]bool{61: true}, failStatus: http.StatusServiceUnavailable, failBody: gemini503, okBody: geminiGenerateContentOK}
		h := newFailoverE2EHandler(t, newAccounts(), upstream, 3)
		body := []byte(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`)
		c, rec := newKeyRouteContext(t, http.MethodPost, "/v1beta/models/gemini-2.5-flash:generateContent", body, service.APIProtocolGemini, "")
		c.Params = gin.Params{{Key: "modelAction", Value: "/gemini-2.5-flash:generateContent"}}
		withGeminiRoute(c)

		start := time.Now()
		h.GeminiV1BetaModels(c)

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Equal(t, []int64{61, 62}, upstream.calls(), "坏渠道只能打 1 次")
		require.Less(t, time.Since(start), time.Second, "不得在原渠道退避")
	})
}

// Claude 成品号（OAuth / setup-token）回 529：暂停该渠道恰 1 分钟，本次请求立即换到下一个渠道、不在原渠道重试。
func TestFailoverE2E_Messages_ClaudeSubscription529CoolsOneMinuteAndSwitches(t *testing.T) {
	anthropic := map[string]string{service.APIProtocolAnthropic: "https://anthropic-relay.example.com"}
	for _, accountType := range []string{service.AccountTypeOAuth, service.AccountTypeSetupToken} {
		t.Run(accountType, func(t *testing.T) {
			subscription := &service.Account{
				ID:          71,
				Name:        "claude-subscription",
				Platform:    service.PlatformAnthropic,
				Type:        accountType,
				Credentials: map[string]any{"access_token": "sk-ant-oat-test"},
				Concurrency: 1,
				Priority:    1,
				Status:      service.StatusActive,
				Schedulable: true,
			}
			relay := failoverE2EKey(72, 2, anthropic, "claude-sonnet-4-5")
			upstream := &failoverStatusUpstream{fail: map[int64]bool{71: true}, failStatus: 529, failBody: anthropicOverloaded, okBody: anthropicMessagesOK}
			h, repo := newFailoverE2EHandlerWithRepo(t, []*service.Account{subscription, relay}, upstream, 3)

			body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
			c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/messages", body, service.APIProtocolAnthropic, "")
			before := time.Now()
			h.Messages(c)
			after := time.Now()

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Equal(t, []int64{71, 72}, upstream.calls(), "成品号只打 1 次就换号")
			overloaded := repo.overloadedIDs()
			require.Len(t, overloaded, 1, "只有成品号被暂停")
			until, ok := overloaded[71]
			require.True(t, ok)
			require.False(t, until.Before(before.Add(time.Minute)), "冷却不得短于 1 分钟")
			require.False(t, until.After(after.Add(time.Minute)), "冷却不得长于 1 分钟")
			require.Less(t, after.Sub(before), time.Second, "不得等待")
		})
	}
}

// Gemini 协议的 key 回 529：错误策略直接换号、不在原渠道重试，也不做过载冷却。
func TestFailoverE2E_GeminiKey529NoCooldown(t *testing.T) {
	const entryID = 9
	gemini := map[string]string{service.APIProtocolGemini: "https://gemini-relay.example.com"}
	bad := failoverE2EKey(81, 1, gemini, "gemini-2.5-flash")
	good := failoverE2EKey(82, 2, gemini, "gemini-2.5-flash")
	bad.CatalogEntryIDs = []int64{entryID}
	good.CatalogEntryIDs = []int64{entryID}
	upstream := &failoverStatusUpstream{fail: map[int64]bool{81: true}, failStatus: 529, failBody: `{"error":{"code":529,"message":"overloaded"}}`, okBody: geminiGenerateContentOK}
	h, repo := newFailoverE2EHandlerWithRepo(t, []*service.Account{bad, good}, upstream, 3)

	body := []byte(`{"model":"gemini-2.5-flash","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/messages", body, service.APIProtocolAnthropic, "")
	entry := &service.ModelCatalogEntry{ID: entryID, ModelID: "gemini-2.5-flash", Vendor: "gemini", Status: service.ModelCatalogStatusListed}
	c.Request = c.Request.WithContext(service.WithCatalogRoute(c.Request.Context(),
		service.CatalogRoute{EntryID: entryID, CanonicalModel: "gemini-2.5-flash", RequestedModel: "gemini-2.5-flash", Entry: entry}))

	h.Messages(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, []int64{81, 82}, upstream.calls(), "坏渠道只能打 1 次")
	require.Empty(t, repo.overloadedIDs(), "Gemini key 不做过载冷却")
}

// OpenAI 协议的 key 回 529（非「server is overloaded」这类请求级容量文案，会走账号错误处理）：
// 不做过载冷却，本次只打 1 次就换号。
func TestFailoverE2E_ChatCompletions_OpenAIKey529NoCooldown(t *testing.T) {
	const entryID = 199
	responses := map[string]string{service.APIProtocolResponses: "https://relay.example.com"}
	bad := failoverE2EKey(91, 1, responses, "gpt-5.6")
	good := failoverE2EKey(92, 2, responses, "gpt-5.6")
	bad.CatalogEntryIDs = []int64{entryID}
	good.CatalogEntryIDs = []int64{entryID}
	upstream := &failoverStatusUpstream{fail: map[int64]bool{91: true}, failStatus: 529, failBody: anthropicOverloaded, okBody: openAIResponsesSSEOK, okContentType: "text/event-stream"}
	h, repo := newFailoverE2EHandlerWithRepo(t, []*service.Account{bad, good}, upstream, 3)

	body := []byte(`{"model":"gpt-5.6","messages":[{"role":"user","content":"hello"}]}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/chat/completions", body, service.APIProtocolChatCompletions, "")
	openAIRouteEntry(c, entryID, "gpt-5.6")
	h.ChatCompletions(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, []int64{91, 92}, upstream.calls(), "坏渠道只能打 1 次")
	require.Empty(t, repo.overloadedIDs(), "OpenAI key 不做过载冷却")
}

// Grok 成品号回 529（普通响应体 / 模型容量文案）：不冷却账号、不按模型封锁，只打 1 次就换号；
// 下一个请求照常能选回这个账号。旧逻辑分别临时停调 2 分钟 / 按模型封锁 1 分钟。
func TestFailoverE2E_Grok529NoCooldown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "plain", body: `{"error":{"message":"upstream unavailable"}}`},
		{name: "model_capacity", body: `{"error":{"message":"The model is currently at capacity due to high demand"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// first_429 夹具：801 / 802 两个 token 有效的 Grok 成品号，801 优先；这里把 801 改成恒 529。
			_, repo, upstream, router, cleanup := newGrokCredentialFailoverGatewayHandler(t, "first_429")
			defer cleanup()
			upstream.rateLimitIDs = nil
			upstream.failureStatus = map[int64]int{801: 529}
			upstream.failureBody = map[int64]string{801: tc.body}

			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":"grok","input":"hello","stream":false}`))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Equal(t, []int64{801, 802}, upstream.accountHits(), "529 只打 1 次就换号")
			require.Empty(t, repo.setTempIDs, "529 不临时停调")
			require.Empty(t, repo.rateLimitedAccountIDs(), "529 不装限流")

			// 801 恢复后，下一个请求应能直接选回它（没有进程内的账号 / 模型封锁）。
			upstream.failureStatus = nil
			second := httptest.NewRecorder()
			secondReq := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":"grok","input":"again","stream":false}`))
			secondReq.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(second, secondReq)

			require.Equal(t, http.StatusOK, second.Code, second.Body.String())
			require.Equal(t, []int64{801, 802, 801}, upstream.accountHits(), "529 后不得封锁该账号或该模型")
		})
	}
}
