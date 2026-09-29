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
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 第三方 key 在 Anthropic / Gemini / Antigravity 网关上按账号类别与协议分流，不看展示标签。

type recordedUpstreamRequest struct {
	url    string
	header http.Header
	body   []byte
}

type recordingHTTPUpstream struct {
	mu       sync.Mutex
	requests []recordedUpstreamRequest
	respBody string
	// contentType 为空时按 application/json 回；Responses 上游要回 text/event-stream。
	contentType string
	// status 为 0 时回 200。
	status int
}

func (u *recordingHTTPUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}
	u.mu.Lock()
	u.requests = append(u.requests, recordedUpstreamRequest{url: req.URL.String(), header: req.Header.Clone(), body: body})
	u.mu.Unlock()
	contentType := u.contentType
	if contentType == "" {
		contentType = "application/json"
	}
	status := u.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(u.respBody)),
	}, nil
}

func (u *recordingHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func (u *recordingHTTPUpstream) recorded() []recordedUpstreamRequest {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]recordedUpstreamRequest(nil), u.requests...)
}

const geminiGenerateContentOK = `{"candidates":[{"content":{"role":"model","parts":[{"text":"hi"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":1,"totalTokenCount":4}}`

// openAIResponsesSSEOK 是 Responses 上游的最小 SSE 正文（ForwardAsAnthropic 对上游恒定流式）。
const openAIResponsesSSEOK = "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"model\":\"gpt-5.6\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"id\":\"msg_1\",\"role\":\"assistant\",\"status\":\"completed\",\"content\":[{\"type\":\"output_text\",\"text\":\"ok\"}]}],\"usage\":{\"input_tokens\":5,\"output_tokens\":2,\"total_tokens\":7}}}\n\ndata: [DONE]\n\n"

const openAIChatCompletionOK = `{"id":"chatcmpl_1","object":"chat.completion","model":"gpt-5.6","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`

// handlerUsageLogRepoStub 只记 Create，供 handler 级用例断言入账走到了哪个网关服务。
type handlerUsageLogRepoStub struct {
	service.UsageLogRepository
	mu   sync.Mutex
	logs []*service.UsageLog
}

func (s *handlerUsageLogRepoStub) Create(_ context.Context, log *service.UsageLog) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = append(s.logs, log)
	return true, nil
}

func (s *handlerUsageLogRepoStub) recorded() []*service.UsageLog {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*service.UsageLog(nil), s.logs...)
}

type handlerUserRepoStub struct{ service.UserRepository }

func (handlerUserRepoStub) DeductBalance(context.Context, int64, float64) error { return nil }

type handlerSubRepoStub struct {
	service.UserSubscriptionRepository
}

func (handlerSubRepoStub) IncrementUsage(context.Context, int64, float64) error { return nil }

// fakeAntigravityTokenCache 直接给出固定 token，让 Antigravity 成品号在测试里不联网就能走到上游调用。
type fakeAntigravityTokenCache struct{ token string }

func (f *fakeAntigravityTokenCache) GetAccessToken(context.Context, string) (string, error) {
	return f.token, nil
}
func (f *fakeAntigravityTokenCache) SetAccessToken(context.Context, string, string, time.Duration) error {
	return nil
}
func (f *fakeAntigravityTokenCache) DeleteAccessToken(context.Context, string) error { return nil }
func (f *fakeAntigravityTokenCache) AcquireRefreshLock(context.Context, string, time.Duration) (bool, error) {
	return true, nil
}
func (f *fakeAntigravityTokenCache) ReleaseRefreshLock(context.Context, string) error { return nil }

type keyRouteHarness struct {
	handler            *GatewayHandler
	geminiUpstream     *recordingHTTPUpstream
	antigravityUpsteam *recordingHTTPUpstream
	openAIUpstream     *recordingHTTPUpstream
	usageLogs          *handlerUsageLogRepoStub
}

func newKeyRouteHarness(t *testing.T, accounts []*service.Account) *keyRouteHarness {
	t.Helper()
	return newKeyRouteHarnessWithConfig(t, accounts, &config.Config{RunMode: config.RunModeSimple})
}

// newKeyRouteHarnessWithConfig 同 newKeyRouteHarness，但各网关服务用调用方给的配置。
func newKeyRouteHarnessWithConfig(t *testing.T, accounts []*service.Account, cfg *config.Config) *keyRouteHarness {
	t.Helper()
	h, cleanup := newTestGatewayHandler(t, accounts)
	t.Cleanup(cleanup)
	geminiUpstream := &recordingHTTPUpstream{respBody: geminiGenerateContentOK}
	antigravityUpstream := &recordingHTTPUpstream{respBody: geminiGenerateContentOK}
	openAIUpstream := &recordingHTTPUpstream{respBody: openAIResponsesSSEOK, contentType: "text/event-stream"}
	usageLogs := &handlerUsageLogRepoStub{}
	h.geminiCompatService = service.NewGeminiMessagesCompatService(nil, nil, nil, nil, nil, geminiUpstream, nil, cfg)
	h.antigravityGatewayService = service.NewAntigravityGatewayService(nil, nil, nil, nil, nil, antigravityUpstream, nil, nil)
	h.openAIGatewayService = service.NewOpenAIGatewayService(
		nil, usageLogs, nil, handlerUserRepoStub{}, handlerSubRepoStub{}, nil, cfg, nil, nil,
		service.NewBillingService(cfg, nil), nil, &service.BillingCacheService{}, openAIUpstream,
		&service.DeferredService{}, nil, nil, nil, nil, nil,
		nil,
	)
	return &keyRouteHarness{handler: h, geminiUpstream: geminiUpstream, antigravityUpsteam: antigravityUpstream, openAIUpstream: openAIUpstream, usageLogs: usageLogs}
}

func keyRouteAccount(id int64, label string, endpoints map[string]string, model string) *service.Account {
	return &service.Account{
		ID:       id,
		Name:     "key-" + label,
		Platform: label,
		Type:     service.AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":       "relay-key",
			"model_mapping": map[string]any{model: model},
		},
		ProtocolEndpoints: endpoints,
		Concurrency:       1,
		Priority:          1,
		Status:            service.StatusActive,
		Schedulable:       true,
	}
}

func newKeyRouteContext(t *testing.T, method, path string, body []byte, inboundProtocol, forcePlatform string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// 目录路由的条目厂商（池由 fakeSchedulerCache 给全部账号）。
	ctx := withTestCatalogRoute(req.Context(), 1, service.PlatformAnthropic, "m")
	ctx = service.WithInboundProtocol(ctx, inboundProtocol)
	if forcePlatform != "" {
		ctx = context.WithValue(ctx, ctxkey.ForcePlatform, forcePlatform)
		c.Set(string(middleware.ContextKeyForcePlatform), forcePlatform)
	}
	c.Request = req.WithContext(ctx)

	apiKey := &service.APIKey{
		ID:     3101,
		UserID: 4101,
		Status: service.StatusActive,
		User:   &service.User{ID: 4101, Concurrency: 10, Balance: 100},
	}
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 10})
	return c, rec
}

func TestGatewayHandlerMessages_CatalogRouteDispatchesByAccount(t *testing.T) {
	const entryID = 7
	withRoute := func(c *gin.Context) {
		entry := &service.ModelCatalogEntry{ID: entryID, ModelID: "gemini-2.5-flash", Vendor: "gemini", Status: service.ModelCatalogStatusListed}
		route := service.CatalogRoute{EntryID: entryID, CanonicalModel: "gemini-2.5-flash", RequestedModel: "gemini-2.5-flash", Entry: entry}
		c.Request = c.Request.WithContext(service.WithCatalogRoute(c.Request.Context(), route))
	}
	body := []byte(`{"model":"gemini-2.5-flash","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)

	t.Run("gemini-address key goes through gemini compat", func(t *testing.T) {
		key := keyRouteAccount(1103, service.PlatformOpenAI,
			map[string]string{service.APIProtocolGemini: "https://gemini-relay.example.com"}, "gemini-2.5-flash")
		key.CatalogEntryIDs = []int64{entryID}
		hs := newKeyRouteHarness(t, []*service.Account{key})

		c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/messages", body, service.APIProtocolAnthropic, "")
		withRoute(c)

		hs.handler.Messages(c)

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		got := hs.geminiUpstream.recorded()
		require.Len(t, got, 1)
		require.Equal(t, "https://gemini-relay.example.com/v1beta/models/gemini-2.5-flash:generateContent", got[0].url)
		require.Empty(t, hs.antigravityUpsteam.recorded())
	})

	t.Run("antigravity subscription goes through claude-shaped v1internal", func(t *testing.T) {
		subscription := &service.Account{
			ID:              1104,
			Name:            "ag-oauth",
			Platform:        service.PlatformAntigravity,
			Type:            service.AccountTypeOAuth,
			Credentials:     map[string]any{"access_token": "tok", "model_mapping": map[string]any{"gemini-2.5-flash": "gemini-2.5-flash"}},
			Concurrency:     1,
			Priority:        1,
			Status:          service.StatusActive,
			Schedulable:     true,
			CatalogEntryIDs: []int64{entryID},
		}
		hs := newKeyRouteHarness(t, []*service.Account{subscription})

		c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/messages", body, service.APIProtocolAnthropic, "")
		withRoute(c)

		hs.handler.Messages(c)

		require.Contains(t, rec.Body.String(), "Antigravity token provider not configured")
		require.Contains(t, rec.Body.String(), `"type":"error"`)
		require.Empty(t, hs.geminiUpstream.recorded())
	})
}

// /v1beta 不按分组 / 条目厂商拦：anthropic 厂商的条目在 anthropic 分组的 key 上也进选号，
// 池里没有能承接 gemini 入站的资源时是 503（不是主线的 404 / 分组平台 400）。
func TestGeminiV1BetaModels_AnthropicVendorEntryIsSchedulingsCall(t *testing.T) {
	const entryID = 8
	key := keyRouteAccount(1106, service.PlatformAnthropic,
		map[string]string{service.APIProtocolAnthropic: "https://relay.example.com"}, "claude-sonnet-4")
	key.CatalogEntryIDs = []int64{entryID}
	hs := newKeyRouteHarness(t, []*service.Account{key})

	body := []byte(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1beta/models/claude-sonnet-4:generateContent", body, service.APIProtocolGemini, "")
	c.Params = gin.Params{{Key: "modelAction", Value: "/claude-sonnet-4:generateContent"}}
	entry := &service.ModelCatalogEntry{ID: entryID, ModelID: "claude-sonnet-4", Vendor: "anthropic", Status: service.ModelCatalogStatusListed}
	c.Request = c.Request.WithContext(service.WithCatalogRoute(c.Request.Context(),
		service.CatalogRoute{EntryID: entryID, CanonicalModel: "claude-sonnet-4", RequestedModel: "claude-sonnet-4", Entry: entry}))

	hs.handler.GeminiV1BetaModels(c)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "platform is not gemini")
	require.Contains(t, rec.Body.String(), "No available Gemini accounts")
	require.Empty(t, hs.geminiUpstream.recorded())
	require.Empty(t, hs.antigravityUpsteam.recorded())
}

// key 没有 Anthropic 地址 = 上游不支持 count_tokens：本地估算回 200，不转发（2026-09-29 定：上游不支持就本地算）。
func TestGatewayHandlerCountTokens_KeyWithoutAnthropicProtocolEstimatesLocally(t *testing.T) {
	key := keyRouteAccount(1105, service.PlatformAntigravity,
		map[string]string{
			service.APIProtocolGemini: "https://gemini-relay.example.com",
		}, "gemini-2.5-flash")
	hs := newKeyRouteHarness(t, []*service.Account{key})

	body := []byte(`{"model":"gemini-2.5-flash","messages":[{"role":"user","content":"hello"}]}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/messages/count_tokens", body, service.APIProtocolAnthropic, "")

	func() {
		// 测试装配里 GatewayService 没有 HTTP 上游：若请求被转发，会在发往上游时 panic。
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("count_tokens was forwarded upstream: %v", r)
			}
		}()
		hs.handler.CountTokens(c)
	}()

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Positive(t, gjson.Get(rec.Body.String(), "input_tokens").Int())
}

func TestUsesAntigravityV1Internal(t *testing.T) {
	tests := []struct {
		name    string
		account *service.Account
		want    bool
	}{
		{"antigravity oauth subscription", &service.Account{Platform: service.PlatformAntigravity, Type: service.AccountTypeOAuth}, true},
		{"key labelled antigravity", &service.Account{Platform: service.PlatformAntigravity, Type: service.AccountTypeAPIKey}, false},
		{"gemini oauth subscription", &service.Account{Platform: service.PlatformGemini, Type: service.AccountTypeOAuth}, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, usesAntigravityV1Internal(tt.account))
		})
	}
}

func TestCompatForwardTargets_FollowUpstreamProtocol(t *testing.T) {
	geminiOnly := map[string]string{service.APIProtocolGemini: "https://gemini-relay.example.com"}
	anthropicOnly := map[string]string{service.APIProtocolAnthropic: "https://anthropic-relay.example.com"}
	responsesOnly := map[string]string{service.APIProtocolResponses: "https://relay.example.com"}
	chatOnly := map[string]string{service.APIProtocolChatCompletions: "https://relay.example.com"}
	both := map[string]string{
		service.APIProtocolGemini:    "https://gemini-relay.example.com",
		service.APIProtocolAnthropic: "https://anthropic-relay.example.com",
	}
	key := func(label string, endpoints map[string]string) *service.Account {
		return &service.Account{Platform: label, Type: service.AccountTypeAPIKey, ProtocolEndpoints: endpoints}
	}
	subscription := func(vendor string) *service.Account {
		return &service.Account{Platform: vendor, Type: service.AccountTypeOAuth}
	}

	// 三个入站共用一份规则：只看资源承接该入站实际用的上游协议（协议转换注册表），
	// 不看资源种类、标签或分组 / 条目的「族」。responses / chat_completions 上游经 OpenAI 服务转发。
	tests := []struct {
		name          string
		account       *service.Account
		wantMessages  compatForwardTarget
		wantCC        compatForwardTarget
		wantResponses compatForwardTarget
	}{
		{"anthropic-labelled key with gemini endpoint", key(service.PlatformAnthropic, geminiOnly), compatForwardGemini, compatForwardGemini, compatForwardSkip},
		{"antigravity-labelled key with both endpoints prefers anthropic direct", key(service.PlatformAntigravity, both), compatForwardAnthropic, compatForwardAnthropic, compatForwardAnthropic},
		{"gemini-labelled key with anthropic endpoint", key(service.PlatformGemini, anthropicOnly), compatForwardAnthropic, compatForwardAnthropic, compatForwardAnthropic},
		{"openai-labelled key with anthropic endpoint is not label-gated", key(service.PlatformOpenAI, anthropicOnly), compatForwardAnthropic, compatForwardAnthropic, compatForwardAnthropic},
		{"responses-only key goes through the OpenAI service", key(service.PlatformOpenAI, responsesOnly), compatForwardOpenAI, compatForwardOpenAI, compatForwardOpenAI},
		{"chat-only key goes through the OpenAI service", key(service.PlatformAnthropic, chatOnly), compatForwardOpenAI, compatForwardOpenAI, compatForwardOpenAI},
		{"key without any endpoint is skipped", key(service.PlatformOpenAI, nil), compatForwardSkip, compatForwardSkip, compatForwardSkip},
		{"gemini subscription has no responses conversion", subscription(service.PlatformGemini), compatForwardGemini, compatForwardGemini, compatForwardSkip},
		{"antigravity subscription", subscription(service.PlatformAntigravity), compatForwardAntigravity, compatForwardAntigravity, compatForwardAntigravity},
		{"anthropic subscription", subscription(service.PlatformAnthropic), compatForwardAnthropic, compatForwardAnthropic, compatForwardAnthropic},
		{"openai subscription goes through the OpenAI service", subscription(service.PlatformOpenAI), compatForwardOpenAI, compatForwardOpenAI, compatForwardOpenAI},
		{"grok subscription goes through the OpenAI service", subscription(service.PlatformGrok), compatForwardOpenAI, compatForwardOpenAI, compatForwardOpenAI},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.wantMessages, messagesForwardTarget(tt.account), "messages")
			require.Equal(t, tt.wantCC, chatCompletionsForwardTarget(tt.account), "chat completions")
			require.Equal(t, tt.wantResponses, responsesForwardTarget(tt.account), "responses")
		})
	}
}

func TestKeyServesAnthropicCountTokens(t *testing.T) {
	both := map[string]string{
		service.APIProtocolGemini:    "https://gemini-relay.example.com",
		service.APIProtocolAnthropic: "https://anthropic-relay.example.com",
	}
	key := &service.Account{Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey, ProtocolEndpoints: both}
	require.True(t, keyServesAnthropicCountTokens(service.PlatformAnthropic, key))
	require.True(t, keyServesAnthropicCountTokens(service.PlatformAntigravity, key))
	require.True(t, keyServesAnthropicCountTokens("", key))
	require.True(t, keyServesAnthropicCountTokens(service.PlatformGemini, key), "有 anthropic 地址就能直连，不看网关平台")

	geminiOnly := &service.Account{Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey, ProtocolEndpoints: map[string]string{service.APIProtocolGemini: "https://gemini-relay.example.com"}}
	require.False(t, keyServesAnthropicCountTokens(service.PlatformAnthropic, geminiOnly))
}

// 没有 Responses → Gemini 的转换：只配 gemini 地址的 key 在 /v1/responses 上被跳过，不会被当成别的协议发出去。
func TestGatewayHandlerResponses_GeminiOnlyKeyIsSkippedInsteadOfSentAsAnthropic(t *testing.T) {
	key := keyRouteAccount(1106, service.PlatformAntigravity,
		map[string]string{
			service.APIProtocolGemini: "https://gemini-relay.example.com",
		}, "gemini-2.5-flash")
	hs := newKeyRouteHarness(t, []*service.Account{key})

	body := []byte(`{"model":"gemini-2.5-flash","input":"hello"}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/responses", body, service.APIProtocolResponses, "")

	func() {
		// 测试装配里 GatewayService 没有 HTTP 上游：若 key 被当成 Anthropic 转发，会在发往上游时 panic。
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("responses request was forwarded over the anthropic protocol: %v", r)
			}
		}()
		hs.handler.Responses(c)
	}()

	require.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Empty(t, hs.geminiUpstream.recorded())
	require.Empty(t, hs.antigravityUpsteam.recorded())
}

// 网关平台只有 request.Context 一个来源：/antigravity 路由的强制平台被清空后，
// handler 侧的判定要跟着回到目录路由的条目厂商，不能再从 gin store 读到旧值。
func TestMessagesGatewayPlatform_FollowsRequestContextNotGinStore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request = c.Request.WithContext(service.WithCatalogRoute(c.Request.Context(), service.CatalogRoute{
		EntryID: 1, CanonicalModel: "gemini-2.5-pro", RequestedModel: "gemini-2.5-pro",
		Entry: &service.ModelCatalogEntry{ID: 1, ModelID: "gemini-2.5-pro", Vendor: "gemini", Status: service.ModelCatalogStatusListed},
	}))

	c.Set(string(middleware.ContextKeyForcePlatform), service.PlatformAntigravity)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.ForcePlatform, service.PlatformAntigravity))
	require.Equal(t, service.PlatformAntigravity, messagesGatewayPlatform(c))

	// 强制平台清空：只清 request.Context，gin store 里的旧值不得再被读到。
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.ForcePlatform, ""))
	require.Equal(t, service.PlatformGemini, messagesGatewayPlatform(c))
}

// 3b-3：/v1/messages 承接 responses / chat_completions 上游资源，经 OpenAI 网关服务转换。

func openAIRouteEntry(c *gin.Context, entryID int64, model string) {
	entry := &service.ModelCatalogEntry{ID: entryID, ModelID: model, Vendor: "openai", Status: service.ModelCatalogStatusListed}
	route := service.CatalogRoute{EntryID: entryID, CanonicalModel: model, RequestedModel: model, Entry: entry}
	c.Request = c.Request.WithContext(service.WithCatalogRoute(c.Request.Context(), route))
}

func TestGatewayHandlerMessages_ResponsesKeyForwardsViaOpenAIService(t *testing.T) {
	const entryID = 199
	key := keyRouteAccount(1201, service.PlatformOpenAI,
		map[string]string{service.APIProtocolResponses: "https://relay.example.com"}, "gpt-5.6")
	key.CatalogEntryIDs = []int64{entryID}
	hs := newKeyRouteHarness(t, []*service.Account{key})

	body := []byte(`{"model":"gpt-5.6","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/messages", body, service.APIProtocolAnthropic, "")
	openAIRouteEntry(c, entryID, "gpt-5.6")

	hs.handler.Messages(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"type":"message"`)
	require.Contains(t, rec.Body.String(), `"ok"`)
	got := hs.openAIUpstream.recorded()
	require.Len(t, got, 1)
	require.True(t, strings.HasSuffix(got[0].url, "/v1/responses"), got[0].url)
	require.Empty(t, hs.geminiUpstream.recorded())
	require.Empty(t, hs.antigravityUpsteam.recorded())
}

func TestGatewayHandlerMessages_ChatKeyForwardsViaOpenAIService(t *testing.T) {
	const entryID = 199
	key := keyRouteAccount(1202, service.PlatformOpenAI,
		map[string]string{service.APIProtocolChatCompletions: "https://relay.example.com"}, "gpt-5.6")
	key.CatalogEntryIDs = []int64{entryID}
	hs := newKeyRouteHarness(t, []*service.Account{key})
	hs.openAIUpstream.respBody = openAIChatCompletionOK
	hs.openAIUpstream.contentType = "application/json"

	body := []byte(`{"model":"gpt-5.6","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/messages", body, service.APIProtocolAnthropic, "")
	openAIRouteEntry(c, entryID, "gpt-5.6")

	hs.handler.Messages(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"type":"message"`)
	got := hs.openAIUpstream.recorded()
	require.Len(t, got, 1)
	require.True(t, strings.HasSuffix(got[0].url, "/v1/chat/completions"), got[0].url)
}

// OpenAI 目标的入账走 OpenAIGatewayService.RecordUsage（usageRecordWorkerPool 未注入时内联执行）。
func TestGatewayHandlerMessages_OpenAITargetRecordsOpenAIUsage(t *testing.T) {
	const entryID = 199
	key := keyRouteAccount(1203, service.PlatformOpenAI,
		map[string]string{service.APIProtocolResponses: "https://relay.example.com"}, "gpt-5.6")
	key.CatalogEntryIDs = []int64{entryID}
	hs := newKeyRouteHarness(t, []*service.Account{key})

	body := []byte(`{"model":"gpt-5.6","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/messages", body, service.APIProtocolAnthropic, "")
	openAIRouteEntry(c, entryID, "gpt-5.6")

	hs.handler.Messages(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	logs := hs.usageLogs.recorded()
	require.Len(t, logs, 1)
	require.Equal(t, "gpt-5.6", logs[0].Model)
	require.Equal(t, key.ID, logs[0].AccountID)
	require.Equal(t, 5, logs[0].InputTokens)
	require.Equal(t, 2, logs[0].OutputTokens)
}

// PromptTooLong 直接写出上游错误，只转发一次。
func TestGatewayHandlerMessages_PromptTooLongNoFallback(t *testing.T) {
	subscription := &service.Account{
		ID:          1204,
		Name:        "ag-oauth",
		Platform:    service.PlatformAntigravity,
		Type:        service.AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "tok", "project_id": "proj-1", "model_mapping": map[string]any{"claude-sonnet-4-5": "claude-sonnet-4-5"}},
		Extra:       map[string]any{"mixed_scheduling": true},
		Concurrency: 1,
		Priority:    1,
		Status:      service.StatusActive,
		Schedulable: true,
	}
	hs := newKeyRouteHarness(t, []*service.Account{subscription})
	hs.antigravityUpsteam.status = http.StatusBadRequest
	hs.antigravityUpsteam.respBody = `{"error":{"message":"prompt is too long"}}`
	hs.handler.antigravityGatewayService = service.NewAntigravityGatewayService(
		nil, nil, nil, service.NewAntigravityTokenProvider(nil, &fakeAntigravityTokenCache{token: "fresh"}, nil), nil, hs.antigravityUpsteam, nil, nil)

	body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/messages", body, service.APIProtocolAnthropic, "")

	hs.handler.Messages(c)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"type":"error"`)
	require.Len(t, hs.antigravityUpsteam.recorded(), 1, "prompt too long must not retry on a fallback group")
}

// 凭据失败（Stage=account_auth）按凭据失败映射，不透传上游原文。
func TestGatewayHandlerMessages_CredentialFailureMapsClientResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	(&GatewayHandler{}).handleFailoverExhausted(c, &service.UpstreamFailoverError{
		StatusCode:   http.StatusUnauthorized,
		Stage:        service.GatewayFailureStageAccountAuth,
		Reason:       service.AntigravityCredentialRejectedReason,
		ResponseBody: []byte(`{"error":{"message":"token revoked","token":"must-not-leak"}}`),
	}, service.PlatformAntigravity, false)

	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Contains(t, rec.Body.String(), service.AntigravityCredentialRejectedClientMessage)
	require.NotContains(t, rec.Body.String(), "must-not-leak")
}

// OAuth 429 风暴刹车接在 Gateway 的 failover 循环上：两个 grok OAuth 成品号连续 429 后停止，不扫第三个。
func TestGatewayHandlerMessages_OAuth429StormStopsAfterSwitches(t *testing.T) {
	const entryID = 199
	grokOAuth := func(id int64, priority int) *service.Account {
		return &service.Account{
			ID: id, Name: "grok-oauth", Platform: service.PlatformGrok, Type: service.AccountTypeOAuth,
			Status: service.StatusActive, Schedulable: true, Concurrency: 1, Priority: priority,
			Credentials: map[string]any{
				"access_token": "healthy-access", "refresh_token": "healthy-refresh",
				"expires_at": time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339),
			},
			CatalogEntryIDs: []int64{entryID},
		}
	}
	accounts := []*service.Account{grokOAuth(801, 1), grokOAuth(802, 2), grokOAuth(803, 3)}
	hs := newKeyRouteHarness(t, accounts)

	repo := &grokCredentialHandlerRepo{missingOnGet: map[int64]bool{}}
	for _, account := range accounts {
		repo.accounts = append(repo.accounts, *account)
	}
	tokenCache := &grokCredentialHandlerTokenCache{}
	provider := service.NewGrokTokenProvider(repo, tokenCache)
	provider.SetRefreshAPI(service.NewOAuthRefreshAPI(repo, tokenCache), &grokCredentialHandlerRefresher{mode: "all_429", started: make(chan struct{})})
	upstream := &grokCredentialHandlerUpstream{rateLimitIDs: map[int64]bool{801: true, 802: true}}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	hs.handler.openAIGatewayService = service.NewOpenAIGatewayService(
		repo, nil, nil, nil, nil, nil, cfg, nil, nil, service.NewBillingService(cfg, nil), nil,
		&service.BillingCacheService{}, upstream, &service.DeferredService{}, nil, provider, nil, nil, nil,
		nil,
	)
	hs.handler.maxAccountSwitches = 10

	body := []byte(`{"model":"grok-4.5","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/messages", body, service.APIProtocolAnthropic, "")
	openAIRouteEntry(c, entryID, "grok-4.5")

	hs.handler.Messages(c)

	require.Equal(t, http.StatusTooManyRequests, rec.Code, rec.Body.String())
	require.Equal(t, []int64{801, 802}, upstream.accountHits(), "the third account must not be swept")
	require.NotContains(t, rec.Body.String(), "rate limited")
}
