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

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
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
}

func (u *recordingHTTPUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}
	u.mu.Lock()
	u.requests = append(u.requests, recordedUpstreamRequest{url: req.URL.String(), header: req.Header.Clone(), body: body})
	u.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
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

type keyRouteHarness struct {
	handler            *GatewayHandler
	geminiUpstream     *recordingHTTPUpstream
	antigravityUpsteam *recordingHTTPUpstream
}

func newKeyRouteHarness(t *testing.T, group *service.Group, accounts []*service.Account) *keyRouteHarness {
	t.Helper()
	h, cleanup := newTestGatewayHandler(t, group, accounts)
	t.Cleanup(cleanup)
	cfg := &config.Config{}
	geminiUpstream := &recordingHTTPUpstream{respBody: geminiGenerateContentOK}
	antigravityUpstream := &recordingHTTPUpstream{respBody: geminiGenerateContentOK}
	h.geminiCompatService = service.NewGeminiMessagesCompatService(nil, nil, nil, nil, nil, nil, geminiUpstream, nil, cfg)
	h.antigravityGatewayService = service.NewAntigravityGatewayService(nil, nil, nil, nil, nil, antigravityUpstream, nil, nil)
	return &keyRouteHarness{handler: h, geminiUpstream: geminiUpstream, antigravityUpsteam: antigravityUpstream}
}

func keyRouteGroup(id int64, platform string) *service.Group {
	return &service.Group{ID: id, Hydrated: true, Platform: platform, Status: service.StatusActive}
}

// keyRouteAccount 构造一个第三方 key：选号与转发都按协议地址放行，标签只是展示。
func keyRouteAccount(id, groupID int64, label string, endpoints map[string]string, model string) *service.Account {
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
		AccountGroups:     []service.AccountGroup{{AccountID: id, GroupID: groupID}},
	}
}

func newKeyRouteContext(t *testing.T, method, path string, body []byte, group *service.Group, inboundProtocol, forcePlatform string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), ctxkey.Group, group)
	ctx = service.WithInboundProtocol(ctx, inboundProtocol)
	if forcePlatform != "" {
		ctx = context.WithValue(ctx, ctxkey.ForcePlatform, forcePlatform)
		c.Set(string(middleware.ContextKeyForcePlatform), forcePlatform)
	}
	c.Request = req.WithContext(ctx)

	groupID := group.ID
	apiKey := &service.APIKey{
		ID:      3101,
		UserID:  4101,
		GroupID: &groupID,
		Status:  service.StatusActive,
		User:    &service.User{ID: 4101, Concurrency: 10, Balance: 100},
		Group:   group,
	}
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 10})
	return c, rec
}

func TestGatewayHandlerMessages_GeminiGroupKeyLabelledAntigravityUsesGeminiEndpoint(t *testing.T) {
	group := keyRouteGroup(2101, service.PlatformGemini)
	key := keyRouteAccount(1101, group.ID, service.PlatformAntigravity,
		map[string]string{service.APIProtocolGemini: "https://gemini-relay.example.com"}, "gemini-2.5-flash")
	hs := newKeyRouteHarness(t, group, []*service.Account{key})

	body := []byte(`{"model":"gemini-2.5-flash","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/messages", body, group, service.APIProtocolAnthropic, "")

	hs.handler.Messages(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := hs.geminiUpstream.recorded()
	require.Len(t, got, 1)
	require.Equal(t, "https://gemini-relay.example.com/v1beta/models/gemini-2.5-flash:generateContent", got[0].url)
	require.Equal(t, "relay-key", got[0].header.Get("x-goog-api-key"))
	require.Empty(t, hs.antigravityUpsteam.recorded(), "key must never reach the Antigravity v1internal upstream")
}

func TestGatewayHandlerMessages_GeminiGroupAntigravitySubscriptionStillUsesV1Internal(t *testing.T) {
	group := keyRouteGroup(2102, service.PlatformGemini)
	subscription := &service.Account{
		ID:            1102,
		Name:          "ag-oauth",
		Platform:      service.PlatformAntigravity,
		Type:          service.AccountTypeOAuth,
		Credentials:   map[string]any{"access_token": "tok", "model_mapping": map[string]any{"gemini-2.5-flash": "gemini-2.5-flash"}},
		Extra:         map[string]any{"mixed_scheduling": true},
		Concurrency:   1,
		Priority:      1,
		Status:        service.StatusActive,
		Schedulable:   true,
		AccountGroups: []service.AccountGroup{{AccountID: 1102, GroupID: group.ID}},
	}
	hs := newKeyRouteHarness(t, group, []*service.Account{subscription})

	body := []byte(`{"model":"gemini-2.5-flash","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/messages", body, group, service.APIProtocolAnthropic, "")

	hs.handler.Messages(c)

	// 测试装配里 Antigravity 服务没有令牌提供者：走到 v1internal 入口即报这条错误，
	// 以此确认成品号仍交给 AntigravityGatewayService，而不是 Gemini 兼容转发。
	require.Contains(t, rec.Body.String(), "Antigravity token provider not configured")
	require.Empty(t, hs.geminiUpstream.recorded())
}

func TestGeminiV1BetaModels_AntigravityRouteKeyLabelledAntigravityForwardsNatively(t *testing.T) {
	group := keyRouteGroup(2103, service.PlatformAntigravity)
	key := keyRouteAccount(1103, group.ID, service.PlatformAntigravity,
		map[string]string{service.APIProtocolGemini: "https://gemini-relay.example.com"}, "gemini-2.5-flash")
	hs := newKeyRouteHarness(t, group, []*service.Account{key})

	body := []byte(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/antigravity/v1beta/models/gemini-2.5-flash:generateContent", body, group, service.APIProtocolGemini, service.PlatformAntigravity)
	c.Params = gin.Params{{Key: "modelAction", Value: "/gemini-2.5-flash:generateContent"}}

	hs.handler.GeminiV1BetaModels(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := hs.geminiUpstream.recorded()
	require.Len(t, got, 1)
	require.Equal(t, "https://gemini-relay.example.com/v1beta/models/gemini-2.5-flash:generateContent", got[0].url)
	require.Equal(t, "relay-key", got[0].header.Get("x-goog-api-key"))
	require.Empty(t, hs.antigravityUpsteam.recorded(), "key must never reach the Antigravity v1internal upstream")
}

func TestGatewayHandlerChatCompletions_GeminiGroupCrossLabelKeyUsesGeminiCompat(t *testing.T) {
	group := keyRouteGroup(2104, service.PlatformGemini)
	key := keyRouteAccount(1104, group.ID, service.PlatformAntigravity,
		map[string]string{service.APIProtocolGemini: "https://gemini-relay.example.com"}, "gemini-2.5-flash")
	hs := newKeyRouteHarness(t, group, []*service.Account{key})

	body := []byte(`{"model":"gemini-2.5-flash","messages":[{"role":"user","content":"hello"}]}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/chat/completions", body, group, service.APIProtocolChatCompletions, "")

	hs.handler.ChatCompletions(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := hs.geminiUpstream.recorded()
	require.Len(t, got, 1)
	require.Equal(t, "https://gemini-relay.example.com/v1beta/models/gemini-2.5-flash:generateContent", got[0].url)
	require.Empty(t, hs.antigravityUpsteam.recorded())
}

func TestGatewayHandlerCountTokens_KeyWithoutAnthropicProtocolOnGatewayGets404(t *testing.T) {
	group := keyRouteGroup(2105, service.PlatformGemini)
	key := keyRouteAccount(1105, group.ID, service.PlatformAntigravity,
		map[string]string{
			service.APIProtocolGemini:    "https://gemini-relay.example.com",
			service.APIProtocolAnthropic: "https://anthropic-relay.example.com",
		}, "gemini-2.5-flash")
	hs := newKeyRouteHarness(t, group, []*service.Account{key})

	body := []byte(`{"model":"gemini-2.5-flash","messages":[{"role":"user","content":"hello"}]}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/messages/count_tokens", body, group, service.APIProtocolAnthropic, "")

	func() {
		// 测试装配里 GatewayService 没有 HTTP 上游：若请求被转发，会在发往上游时 panic。
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("count_tokens was forwarded upstream: %v", r)
			}
		}()
		hs.handler.CountTokens(c)
	}()

	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "count_tokens endpoint is not supported for this platform")
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

func TestCompatForwardTargets_KeysFollowGatewayProtocolNotLabel(t *testing.T) {
	geminiOnly := map[string]string{service.APIProtocolGemini: "https://gemini-relay.example.com"}
	anthropicOnly := map[string]string{service.APIProtocolAnthropic: "https://anthropic-relay.example.com"}
	both := map[string]string{
		service.APIProtocolGemini:    "https://gemini-relay.example.com",
		service.APIProtocolAnthropic: "https://anthropic-relay.example.com",
	}
	key := func(label string, endpoints map[string]string) *service.Account {
		return &service.Account{Platform: label, Type: service.AccountTypeAPIKey, ProtocolEndpoints: endpoints}
	}

	tests := []struct {
		name          string
		group         string
		account       *service.Account
		wantCC        compatForwardTarget
		wantResponses compatForwardTarget
	}{
		{"anthropic-labelled key with gemini endpoint in gemini group", service.PlatformGemini, key(service.PlatformAnthropic, geminiOnly), compatForwardGemini, compatForwardSkip},
		{"antigravity-labelled key with both endpoints in gemini group", service.PlatformGemini, key(service.PlatformAntigravity, both), compatForwardGemini, compatForwardSkip},
		{"gemini-labelled key with anthropic endpoint in anthropic group", service.PlatformAnthropic, key(service.PlatformGemini, anthropicOnly), compatForwardAnthropic, compatForwardAnthropic},
		{"gemini-labelled key with both endpoints in anthropic group", service.PlatformAnthropic, key(service.PlatformGemini, both), compatForwardAnthropic, compatForwardAnthropic},
		{"antigravity-labelled key in antigravity group", service.PlatformAntigravity, key(service.PlatformAntigravity, both), compatForwardAnthropic, compatForwardAnthropic},
		{"openai-labelled key without group protocol", service.PlatformGemini, key(service.PlatformOpenAI, anthropicOnly), compatForwardSkip, compatForwardSkip},
		{"ungrouped key uses anthropic gateway", "", key(service.PlatformGemini, anthropicOnly), compatForwardAnthropic, compatForwardAnthropic},
		{"gemini subscription in gemini group", service.PlatformGemini, &service.Account{Platform: service.PlatformGemini, Type: service.AccountTypeOAuth}, compatForwardGemini, compatForwardAnthropic},
		{"antigravity subscription in gemini group", service.PlatformGemini, &service.Account{Platform: service.PlatformAntigravity, Type: service.AccountTypeOAuth}, compatForwardSkip, compatForwardAntigravity},
		{"antigravity subscription in anthropic group", service.PlatformAnthropic, &service.Account{Platform: service.PlatformAntigravity, Type: service.AccountTypeOAuth}, compatForwardAntigravity, compatForwardAntigravity},
		{"anthropic subscription in anthropic group", service.PlatformAnthropic, &service.Account{Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth}, compatForwardAnthropic, compatForwardAnthropic},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.wantCC, chatCompletionsForwardTarget(tt.group, tt.account), "chat completions")
			require.Equal(t, tt.wantResponses, responsesForwardTarget(tt.group, tt.account), "responses")
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
	require.False(t, keyServesAnthropicCountTokens(service.PlatformGemini, key))

	geminiOnly := &service.Account{Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey, ProtocolEndpoints: map[string]string{service.APIProtocolGemini: "https://gemini-relay.example.com"}}
	require.False(t, keyServesAnthropicCountTokens(service.PlatformAnthropic, geminiOnly))
}

func TestGatewayHandlerResponses_GeminiGroupKeyIsSkippedInsteadOfSentAsAnthropic(t *testing.T) {
	group := keyRouteGroup(2106, service.PlatformGemini)
	key := keyRouteAccount(1106, group.ID, service.PlatformAntigravity,
		map[string]string{
			service.APIProtocolGemini:    "https://gemini-relay.example.com",
			service.APIProtocolAnthropic: "https://anthropic-relay.example.com",
		}, "gemini-2.5-flash")
	hs := newKeyRouteHarness(t, group, []*service.Account{key})

	body := []byte(`{"model":"gemini-2.5-flash","input":"hello"}`)
	c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/responses", body, group, service.APIProtocolResponses, "")

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

// 网关平台只有 request.Context 一个来源：/antigravity 路由的强制平台在兜底分组重试时被清空后，
// handler 侧的判定要跟着回到分组平台，不能再从 gin store 读到旧值。
func TestMessagesGatewayPlatform_FollowsRequestContextNotGinStore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	apiKey := &service.APIKey{Group: &service.Group{Platform: service.PlatformGemini}}

	c.Set(string(middleware.ContextKeyForcePlatform), service.PlatformAntigravity)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.ForcePlatform, service.PlatformAntigravity))
	require.Equal(t, service.PlatformAntigravity, messagesGatewayPlatform(c, apiKey))

	// 兜底：只清 request.Context，gin store 里的旧值不得再被读到。
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.ForcePlatform, ""))
	require.Equal(t, service.PlatformGemini, messagesGatewayPlatform(c, apiKey))
}
