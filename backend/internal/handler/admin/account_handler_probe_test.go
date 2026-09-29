package admin

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 建渠道「探测模型」：用表单里的地址与 key 向上游要模型名单（2026-09-29）。

func setupProbeRouter(adminSvc service.AdminService, upstream service.HTTPUpstream) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	accountTestSvc := service.NewAccountTestService(nil, nil, nil, nil, nil, upstream,
		&config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}}, nil)
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, accountTestSvc, nil, nil, nil, nil)
	router.POST("/api/v1/admin/accounts/models/probe", handler.ProbeUpstreamModels)
	return router
}

func postProbe(t *testing.T, router *gin.Engine, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/models/probe", bytes.NewReader(raw)))
	return rec
}

func modelsListResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func TestProbeUpstreamModels_ReturnsUpstreamList(t *testing.T) {
	upstream := &syncUpstreamHTTPUpstream{resp: modelsListResponse(http.StatusOK, `{"object":"list","data":[{"id":"gpt-5.4"},{"id":"claude-sonnet-4-6"},{"id":"gpt-5.4"}]}`)}
	router := setupProbeRouter(newStubAdminService(), upstream)

	rec := postProbe(t, router, map[string]any{
		"api_key":            "sk-relay",
		"protocol_endpoints": map[string]string{"chat_completions": "https://relay.example.com/v1"},
	})

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var got struct {
		Data struct {
			Models []string `json:"models"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, []string{"claude-sonnet-4-6", "gpt-5.4"}, got.Data.Models, "去重排序")
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://relay.example.com/v1/models", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer sk-relay", upstream.requests[0].Header.Get("Authorization"))
}

// 编辑已有渠道时传 account_id，用存着的 key；地址仍以表单为准。
func TestProbeUpstreamModels_UsesSavedKeyForAccount(t *testing.T) {
	adminSvc := &availableModelsAdminService{stubAdminService: newStubAdminService(), account: service.Account{
		ID: 77, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive,
		Credentials:       map[string]any{"api_key": "sk-saved"},
		ProtocolEndpoints: map[string]string{"chat_completions": "https://old.example.com/v1"},
	}}
	upstream := &syncUpstreamHTTPUpstream{resp: modelsListResponse(http.StatusOK, `{"data":[{"id":"gpt-5.4"}]}`)}
	router := setupProbeRouter(adminSvc, upstream)

	rec := postProbe(t, router, map[string]any{
		"account_id":         77,
		"protocol_endpoints": map[string]string{"chat_completions": "https://new.example.com/v1"},
	})

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://new.example.com/v1/models", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer sk-saved", upstream.requests[0].Header.Get("Authorization"))
}

// 上游没有模型列表端点（404 / 405）：回 400 说明「不支持」，不回落到配置里的模型。
func TestProbeUpstreamModels_UpstreamWithoutListEndpointIsUnsupported(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusMethodNotAllowed} {
		upstream := &syncUpstreamHTTPUpstream{resp: modelsListResponse(status, `{"error":"not found"}`)}
		router := setupProbeRouter(newStubAdminService(), upstream)

		rec := postProbe(t, router, map[string]any{
			"api_key":            "sk-relay",
			"protocol_endpoints": map[string]string{"anthropic": "https://relay.example.com"},
		})

		require.Equal(t, http.StatusBadRequest, rec.Code, "status=%d body=%s", status, rec.Body.String())
		require.Contains(t, rec.Body.String(), "does not provide a model list endpoint", "status=%d", status)
	}
}

func TestProbeUpstreamModels_RequiresKeyAndEndpoint(t *testing.T) {
	router := setupProbeRouter(newStubAdminService(), &syncUpstreamHTTPUpstream{})

	rec := postProbe(t, router, map[string]any{"protocol_endpoints": map[string]string{"chat_completions": "https://relay.example.com/v1"}})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	rec = postProbe(t, router, map[string]any{"api_key": "sk-relay", "protocol_endpoints": map[string]string{}})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

// 「探测协议」：对表单里的一个地址逐个试四个上游协议（2026-09-29）。

// protocolProbeUpstream 四个协议并发探测：按路径回响应，记录加锁。
type protocolProbeUpstream struct {
	mu       sync.Mutex
	requests []*http.Request
	respond  func(req *http.Request) *http.Response
}

func (u *protocolProbeUpstream) Do(req *http.Request, proxyURL string, accountID int64, concurrency int) (*http.Response, error) {
	return u.DoWithTLS(req, proxyURL, accountID, concurrency, nil)
}

func (u *protocolProbeUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.mu.Lock()
	u.requests = append(u.requests, req)
	u.mu.Unlock()
	return u.respond(req), nil
}

func postProtocolProbe(t *testing.T, adminSvc service.AdminService, upstream service.HTTPUpstream, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	accountTestSvc := service.NewAccountTestService(nil, nil, nil, nil, nil, upstream,
		&config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}}, nil)
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, accountTestSvc, nil, nil, nil, nil)
	router.POST("/api/v1/admin/accounts/protocols/probe", handler.ProbeUpstreamProtocols)
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/protocols/probe", bytes.NewReader(raw)))
	return rec
}

// 编辑已有渠道时传 account_id，用存着的 key；地址以表单为准。
func TestProbeUpstreamProtocols_ReturnsPerProtocolResultsWithSavedKey(t *testing.T) {
	adminSvc := &availableModelsAdminService{stubAdminService: newStubAdminService(), account: service.Account{
		ID: 77, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive,
		Credentials:       map[string]any{"api_key": "sk-saved"},
		ProtocolEndpoints: map[string]string{"chat_completions": "https://old.example.com/v1"},
	}}
	upstream := &protocolProbeUpstream{respond: func(req *http.Request) *http.Response {
		switch {
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/v1/models"):
			return modelsListResponse(http.StatusOK, `{"data":[{"id":"gpt-5.5"}]}`)
		case strings.HasSuffix(req.URL.Path, "/v1/chat/completions"):
			// 空请求被参数校验拦下（400），带模型的真实请求才成功
			if req.Body != nil {
				raw, _ := io.ReadAll(req.Body)
				if strings.Contains(string(raw), `"model"`) {
					return modelsListResponse(http.StatusOK, `{"choices":[]}`)
				}
			}
			return modelsListResponse(http.StatusBadRequest, `{"error":{"message":"model is required"}}`)
		}
		return modelsListResponse(http.StatusNotFound, `{"error":"not found"}`)
	}}

	rec := postProtocolProbe(t, adminSvc, upstream, map[string]any{"account_id": 77, "base_url": "https://new.example.com/v1"})

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var got struct {
		Data struct {
			Protocols []service.ProbedUpstreamProtocol `json:"protocols"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Data.Protocols, 4)
	byProtocol := map[string]service.ProbedUpstreamProtocol{}
	for _, p := range got.Data.Protocols {
		byProtocol[p.Protocol] = p
	}
	require.Equal(t, service.ProtocolProbeSupported, byProtocol["chat_completions"].Status)
	require.Equal(t, "https://new.example.com/v1", byProtocol["chat_completions"].BaseURL)
	require.Equal(t, service.ProtocolProbeUnsupported, byProtocol["anthropic"].Status)
	require.Equal(t, "https://new.example.com", byProtocol["gemini"].BaseURL)

	require.Len(t, upstream.requests, 6, "4 个协议的空请求 + 模型列表 + Chat 的真实确认")
	for _, req := range upstream.requests {
		require.Equal(t, "new.example.com", req.URL.Host, "地址以表单为准")
		require.Contains(t, req.Header.Get("Authorization")+req.Header.Get("x-goog-api-key"), "sk-saved", "用存着的 key")
	}
}

func TestProbeUpstreamProtocols_RequiresAddressAndKey(t *testing.T) {
	upstream := &protocolProbeUpstream{respond: func(req *http.Request) *http.Response {
		t.Errorf("不该发请求：%s", req.URL)
		return modelsListResponse(http.StatusTeapot, `{}`)
	}}

	rec := postProtocolProbe(t, newStubAdminService(), upstream, map[string]any{"api_key": "sk-relay"})
	require.Equal(t, http.StatusBadRequest, rec.Code, "缺地址：%s", rec.Body.String())

	rec = postProtocolProbe(t, newStubAdminService(), upstream, map[string]any{"base_url": "https://relay.example.com"})
	require.Equal(t, http.StatusBadRequest, rec.Code, "缺 key：%s", rec.Body.String())

	rec = postProtocolProbe(t, newStubAdminService(), upstream, map[string]any{"api_key": "sk-relay", "base_url": "not a url"})
	require.Equal(t, http.StatusBadRequest, rec.Code, "地址不合法：%s", rec.Body.String())
}
