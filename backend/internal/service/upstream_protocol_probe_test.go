//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

// 探测协议（2026-09-29）：空请求按状态码分类，拿不准的用上游模型列表里的模型发一次真实请求。

type probeCall struct {
	method string
	url    string
	body   string
	header http.Header
}

// routedProbeUpstream 按「方法 + 路径」回响应；四个协议并发探测，记录要加锁。
type routedProbeUpstream struct {
	mu     sync.Mutex
	calls  []probeCall
	handle func(call probeCall) *http.Response
}

func (u *routedProbeUpstream) Do(req *http.Request, proxyURL string, accountID int64, concurrency int) (*http.Response, error) {
	return u.DoWithTLS(req, proxyURL, accountID, concurrency, nil)
}

func (u *routedProbeUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	var body string
	if req.Body != nil {
		raw, _ := io.ReadAll(req.Body)
		body = string(raw)
	}
	call := probeCall{method: req.Method, url: req.URL.String(), body: body, header: req.Header.Clone()}
	u.mu.Lock()
	u.calls = append(u.calls, call)
	u.mu.Unlock()
	return u.handle(call), nil
}

func (u *routedProbeUpstream) callsTo(suffix string) []probeCall {
	u.mu.Lock()
	defer u.mu.Unlock()
	var out []probeCall
	for _, c := range u.calls {
		if strings.HasSuffix(strings.SplitN(c.url, "?", 2)[0], suffix) {
			out = append(out, c)
		}
	}
	return out
}

func probeResponse(status int, contentType, body string) *http.Response {
	h := make(http.Header)
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}
	return &http.Response{StatusCode: status, Header: h, Body: io.NopCloser(strings.NewReader(body))}
}

func newProtocolProbeService(upstream HTTPUpstream) *AccountTestService {
	return &AccountTestService{httpUpstream: upstream, cfg: &config.Config{}}
}

func protocolProbeKey() *Account {
	return &Account{Type: AccountTypeAPIKey, Platform: PlatformOpenAI, Credentials: map[string]any{"api_key": "sk-probe"}}
}

func probeResultsByProtocol(t *testing.T, results []ProbedUpstreamProtocol) map[string]ProbedUpstreamProtocol {
	t.Helper()
	require.Len(t, results, 4)
	out := make(map[string]ProbedUpstreamProtocol, len(results))
	for _, r := range results {
		out[r.Protocol] = r
	}
	return out
}

func TestProbeUpstreamProtocols_ClassifiesEmptyRequests(t *testing.T) {
	upstream := &routedProbeUpstream{handle: func(call probeCall) *http.Response {
		switch {
		case call.method == http.MethodGet && strings.HasSuffix(call.url, "/v1/models"):
			return probeResponse(http.StatusNotFound, "application/json", `{"error":"not found"}`)
		case strings.HasSuffix(call.url, "/v1/messages"):
			return probeResponse(http.StatusBadRequest, "application/json", `{"error":{"message":"model is required"}}`)
		case strings.HasSuffix(call.url, "/v1/chat/completions"):
			return probeResponse(http.StatusNotFound, "application/json", `{"error":"not found"}`)
		case strings.HasSuffix(call.url, "/v1/responses"):
			return probeResponse(http.StatusOK, "text/html", `<!doctype html><html></html>`)
		case strings.HasSuffix(call.url, "/v1beta/models"):
			return probeResponse(http.StatusOK, "application/json", `{"models":[]}`)
		}
		t.Errorf("unexpected call %s %s", call.method, call.url)
		return probeResponse(http.StatusTeapot, "", "")
	}}

	results, err := newProtocolProbeService(upstream).ProbeUpstreamProtocols(context.Background(), protocolProbeKey(), "https://relay.example.com/v1/")
	require.NoError(t, err)
	got := probeResultsByProtocol(t, results)

	require.Equal(t, ProbedUpstreamProtocol{Protocol: APIProtocolAnthropic, BaseURL: "https://relay.example.com/v1",
		Status: ProtocolProbeUnknown, Reason: ProtocolProbeReasonValidationError, HTTPStatus: 400}, got[APIProtocolAnthropic],
		"空请求的 400 只说明端点多半在，拿不到模型做真实确认就停在不确定")
	require.Equal(t, ProtocolProbeUnsupported, got[APIProtocolChatCompletions].Status)
	require.Equal(t, ProtocolProbeReasonNotFound, got[APIProtocolChatCompletions].Reason)
	require.Equal(t, ProtocolProbeUnsupported, got[APIProtocolResponses].Status, "2xx 网页是落到了前端页面，不是 API")
	require.Equal(t, ProtocolProbeReasonNotAPI, got[APIProtocolResponses].Reason)
	require.Equal(t, ProbedUpstreamProtocol{Protocol: APIProtocolGemini, BaseURL: "https://relay.example.com",
		Status: ProtocolProbeSupported, Reason: ProtocolProbeReasonAccepted, HTTPStatus: 200}, got[APIProtocolGemini],
		"Gemini 地址去掉末尾版本段，否则拼成 /v1/v1beta")

	// 4 个协议各一次空请求；anthropic 的 400 要确认，去拉了一次模型列表（404，没有模型可试）
	require.Len(t, upstream.calls, 5)
	require.Len(t, upstream.callsTo("/v1/models"), 1)
	for _, c := range upstream.calls {
		require.NotContains(t, c.body, `"model"`)
	}
	// 端点拼接与认证头
	anthropic := upstream.callsTo("/v1/messages")[0]
	require.Equal(t, "https://relay.example.com/v1/messages", anthropic.url)
	require.Equal(t, "sk-probe", anthropic.header.Get("x-api-key"))
	require.NotEmpty(t, anthropic.header.Get("anthropic-version"))
	gemini := upstream.callsTo("/v1beta/models")[0]
	require.Equal(t, http.MethodGet, gemini.method)
	require.Equal(t, "https://relay.example.com/v1beta/models", gemini.url)
	require.Equal(t, "sk-probe", gemini.header.Get("x-goog-api-key"))
	require.Equal(t, "Bearer sk-probe", upstream.callsTo("/v1/chat/completions")[0].header.Get("Authorization"))
}

func TestProbeUpstreamProtocols_ConfirmsUncertainOnesWithRealRequest(t *testing.T) {
	upstream := &routedProbeUpstream{handle: func(call probeCall) *http.Response {
		real := strings.Contains(call.body, `"model"`) || strings.Contains(call.url, ":generateContent")
		switch {
		case call.method == http.MethodGet && strings.HasSuffix(call.url, "/v1/models"):
			return probeResponse(http.StatusOK, "application/json", `{"data":[{"id":"gpt-5.4"},{"id":"claude-sonnet-4-6"},{"id":"gemini-2.5-flash"}]}`)
		case strings.HasSuffix(call.url, "/v1/messages"):
			if real {
				return probeResponse(http.StatusOK, "application/json", `{"type":"message","content":[]}`)
			}
			return probeResponse(http.StatusServiceUnavailable, "application/json", `{"error":"busy"}`)
		case strings.HasSuffix(call.url, "/v1/chat/completions"):
			if real {
				return probeResponse(http.StatusNotFound, "application/json", `{"error":"no route"}`)
			}
			return probeResponse(http.StatusTooManyRequests, "application/json", `{"error":"slow down"}`)
		case strings.HasSuffix(call.url, "/v1/responses"):
			return probeResponse(http.StatusUnprocessableEntity, "application/json", `{"error":"input required"}`)
		case strings.HasSuffix(call.url, "/v1beta/models"):
			return probeResponse(http.StatusBadGateway, "application/json", `{"error":"bad gateway"}`)
		case strings.Contains(call.url, "/v1beta/models/gemini-2.5-flash:generateContent"):
			return probeResponse(http.StatusOK, "application/json", `{"candidates":[]}`)
		}
		t.Errorf("unexpected call %s %s %s", call.method, call.url, call.body)
		return probeResponse(http.StatusTeapot, "", "")
	}}

	results, err := newProtocolProbeService(upstream).ProbeUpstreamProtocols(context.Background(), protocolProbeKey(), "https://relay.example.com")
	require.NoError(t, err)
	got := probeResultsByProtocol(t, results)

	require.Equal(t, ProbedUpstreamProtocol{Protocol: APIProtocolAnthropic, BaseURL: "https://relay.example.com",
		Status: ProtocolProbeSupported, Reason: ProtocolProbeReasonAccepted, HTTPStatus: 200, Model: "claude-sonnet-4-6"}, got[APIProtocolAnthropic],
		"空请求 503 拿不准，真实请求挑 claude 模型确认支持")
	require.Equal(t, ProbedUpstreamProtocol{Protocol: APIProtocolChatCompletions, BaseURL: "https://relay.example.com",
		Status: ProtocolProbeUnsupported, Reason: ProtocolProbeReasonNotFound, HTTPStatus: 404, Model: "gpt-5.4"}, got[APIProtocolChatCompletions])
	require.Equal(t, ProbedUpstreamProtocol{Protocol: APIProtocolResponses, BaseURL: "https://relay.example.com",
		Status: ProtocolProbeUnknown, Reason: ProtocolProbeReasonRealRejected, HTTPStatus: 422, Model: "gpt-5.4"}, got[APIProtocolResponses],
		"空请求 422 要确认；带模型的真实请求还被 422 就是走不通，不报支持")
	require.Equal(t, ProbedUpstreamProtocol{Protocol: APIProtocolGemini, BaseURL: "https://relay.example.com",
		Status: ProtocolProbeSupported, Reason: ProtocolProbeReasonAccepted, HTTPStatus: 200, Model: "gemini-2.5-flash"}, got[APIProtocolGemini])

	require.Len(t, upstream.callsTo("/v1/models"), 1, "模型列表只拉一次")
	require.Contains(t, upstream.callsTo("/v1/messages")[1].body, `"max_tokens":1`, "真实请求只要 1 个 token")
}

// 有的上游对任意路径回 200 JSON（健康检查兜底）：只看状态码会把不存在的端点当成支持。
func TestProbeUpstreamProtocols_CatchAllOKIsNotSupport(t *testing.T) {
	upstream := &routedProbeUpstream{handle: func(call probeCall) *http.Response {
		switch {
		case call.method == http.MethodGet && strings.HasSuffix(call.url, "/v1/models"):
			return probeResponse(http.StatusOK, "application/json", `{"data":[{"id":"gpt-5.4"}]}`)
		case strings.Contains(call.body, `"model"`) || strings.Contains(call.url, ":generateContent"):
			return probeResponse(http.StatusNotFound, "application/json", `{"error":"unknown route"}`)
		}
		return probeResponse(http.StatusOK, "application/json", `{"status":"ok"}`)
	}}

	results, err := newProtocolProbeService(upstream).ProbeUpstreamProtocols(context.Background(), protocolProbeKey(), "https://relay.example.com")
	require.NoError(t, err)
	for _, r := range results {
		require.Equal(t, ProtocolProbeUnsupported, r.Status, "%s：兜底 200 不算支持，真实请求 404 才是结论", r.Protocol)
		require.Equal(t, "gpt-5.4", r.Model, r.Protocol)
	}

	// 真实请求也拿到兜底 200：不确定，原因写明内容不像这个协议
	upstream.handle = func(call probeCall) *http.Response {
		if call.method == http.MethodGet && strings.HasSuffix(call.url, "/v1/models") {
			return probeResponse(http.StatusOK, "application/json", `{"data":[{"id":"gpt-5.4"}]}`)
		}
		return probeResponse(http.StatusOK, "application/json", `{"status":"ok"}`)
	}
	results, err = newProtocolProbeService(upstream).ProbeUpstreamProtocols(context.Background(), protocolProbeKey(), "https://relay.example.com")
	require.NoError(t, err)
	for _, r := range results {
		require.Equal(t, ProtocolProbeUnknown, r.Status, r.Protocol)
		require.Equal(t, ProtocolProbeReasonUnexpectedBody, r.Reason, r.Protocol)
	}
}

// 400 不直接算支持（2026-09-29 真实上游 fenno：Gemini 路由在，但对非 Gemini 分组的 key 一律回 400）：
// 空请求的 400 用真实请求确认；真实请求还被 400 就保持不确定，不报支持。
func TestProbeUpstreamProtocols_BadRequestNeedsRealConfirmation(t *testing.T) {
	upstream := &routedProbeUpstream{handle: func(call probeCall) *http.Response {
		real := strings.Contains(call.body, `"model"`) || strings.Contains(call.url, ":generateContent")
		switch {
		case call.method == http.MethodGet && strings.HasSuffix(call.url, "/v1/models"):
			return probeResponse(http.StatusOK, "application/json", `{"data":[{"id":"gpt-5.5"}]}`)
		case real && strings.HasSuffix(call.url, "/v1/chat/completions"):
			return probeResponse(http.StatusOK, "application/json", `{"choices":[]}`)
		}
		return probeResponse(http.StatusBadRequest, "application/json", `{"error":{"message":"API key group platform is not gemini"}}`)
	}}

	results, err := newProtocolProbeService(upstream).ProbeUpstreamProtocols(context.Background(), protocolProbeKey(), "https://relay.example.com")
	require.NoError(t, err)
	got := probeResultsByProtocol(t, results)

	require.Equal(t, ProbedUpstreamProtocol{Protocol: APIProtocolChatCompletions, BaseURL: "https://relay.example.com",
		Status: ProtocolProbeSupported, Reason: ProtocolProbeReasonAccepted, HTTPStatus: 200, Model: "gpt-5.5"}, got[APIProtocolChatCompletions],
		"空请求的 400 与对照组一样，靠真实请求确认支持")
	for _, p := range []string{APIProtocolAnthropic, APIProtocolResponses, APIProtocolGemini} {
		require.Equal(t, ProtocolProbeUnknown, got[p].Status, "%s：真实请求也和不存在的路径一样回 400，不能报支持", p)
		require.Equal(t, ProtocolProbeReasonRealRejected, got[p].Reason, p)
	}
}

func TestProbeUpstreamProtocols_KeepsUncertainWhenRealRequestCannotDecide(t *testing.T) {
	t.Run("key 被拒：真实请求也是 401，保留原因", func(t *testing.T) {
		upstream := &routedProbeUpstream{handle: func(call probeCall) *http.Response {
			return probeResponse(http.StatusUnauthorized, "application/json", `{"error":"invalid key"}`)
		}}
		results, err := newProtocolProbeService(upstream).ProbeUpstreamProtocols(context.Background(), protocolProbeKey(), "https://relay.example.com")
		require.NoError(t, err)
		for _, r := range results {
			require.Equal(t, ProtocolProbeUnknown, r.Status, r.Protocol)
			require.Equal(t, ProtocolProbeReasonAuthRejected, r.Reason, r.Protocol)
			require.Empty(t, r.Model, "模型列表同样 401，拿不到模型就不发真实请求")
		}
	})

	t.Run("上游出错又拿不到模型名：标成没有模型可试", func(t *testing.T) {
		upstream := &routedProbeUpstream{handle: func(call probeCall) *http.Response {
			if strings.HasSuffix(call.url, "/v1/models") {
				return probeResponse(http.StatusNotFound, "application/json", `{}`)
			}
			return probeResponse(http.StatusBadGateway, "application/json", `{"error":"upstream down"}`)
		}}
		results, err := newProtocolProbeService(upstream).ProbeUpstreamProtocols(context.Background(), protocolProbeKey(), "https://relay.example.com")
		require.NoError(t, err)
		for _, r := range results {
			require.Equal(t, ProtocolProbeUnknown, r.Status, r.Protocol)
			require.Equal(t, ProtocolProbeReasonNoModel, r.Reason, r.Protocol)
		}
	})
}

func TestProbeUpstreamProtocols_RejectsBadInput(t *testing.T) {
	svc := newProtocolProbeService(&routedProbeUpstream{handle: func(probeCall) *http.Response {
		t.Error("不该发请求")
		return probeResponse(http.StatusTeapot, "", "")
	}})

	_, err := svc.ProbeUpstreamProtocols(context.Background(), protocolProbeKey(), "not a url")
	require.Error(t, err)

	noKey := &Account{Type: AccountTypeAPIKey, Credentials: map[string]any{}}
	_, err = svc.ProbeUpstreamProtocols(context.Background(), noKey, "https://relay.example.com")
	require.Error(t, err)
}

func TestProtocolProbeBaseURL(t *testing.T) {
	for _, tc := range []struct{ protocol, base, want string }{
		{APIProtocolAnthropic, "https://relay.example.com/v1/", "https://relay.example.com/v1"},
		{APIProtocolChatCompletions, "https://relay.example.com", "https://relay.example.com"},
		{APIProtocolGemini, "https://relay.example.com/v1", "https://relay.example.com"},
		{APIProtocolGemini, "https://relay.example.com/api/v1beta/", "https://relay.example.com/api"},
		{APIProtocolGemini, "https://relay.example.com/gemini", "https://relay.example.com/gemini"},
	} {
		require.Equal(t, tc.want, protocolProbeBaseURL(tc.protocol, tc.base), tc.protocol+" "+tc.base)
	}
}

func TestPickProtocolProbeModel(t *testing.T) {
	models := []string{"deepseek-chat", "gemini-2.5-pro", "gpt-5.4", "claude-sonnet-4-6"}
	require.Equal(t, "claude-sonnet-4-6", pickProtocolProbeModel(APIProtocolAnthropic, models))
	require.Equal(t, "gpt-5.4", pickProtocolProbeModel(APIProtocolResponses, models))
	require.Equal(t, "gemini-2.5-pro", pickProtocolProbeModel(APIProtocolGemini, models))
	require.Equal(t, "deepseek-chat", pickProtocolProbeModel(APIProtocolAnthropic, []string{"deepseek-chat"}), "没有对口的用第一个")
	require.Empty(t, pickProtocolProbeModel(APIProtocolChatCompletions, nil))
}
