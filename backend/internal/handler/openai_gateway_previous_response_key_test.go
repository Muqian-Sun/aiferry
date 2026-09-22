//go:build unit

package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type previousResponseKeyUpstream struct {
	service.HTTPUpstream
	mu   sync.Mutex
	hits int
}

func (u *previousResponseKeyUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	if req.Body != nil {
		_, _ = io.ReadAll(req.Body)
	}
	u.mu.Lock()
	u.hits++
	u.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusInternalServerError,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"upstream failure"}}`)),
	}, nil
}

func (u *previousResponseKeyUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

// HTTP 请求携带 previous_response_id 时，第三方 key 只有以 responses 协议转发到官方 OpenAI
// 或通用中转才承接续链；按协议 / 厂商排除，不看平台标签。
func TestGatewayResponses_HTTPContinuationExcludesKeysByProtocolAndVendor(t *testing.T) {
	cases := map[string]map[string]string{
		// openai 标签，但只有 chat_completions 地址：续链状态会在转换里丢失。
		"converted to chat completions": {service.APIProtocolChatCompletions: "https://relay.example.com/v1"},
		// openai 标签，但地址指向 DeepSeek 官方：其他已知厂商不承接。请求模型取 DeepSeek
		// 白名单内的，保证这个 key 是在续链这一关被排除，而不是先在模型支持那一关。
		"other known vendor": {service.APIProtocolResponses: service.DefaultDeepseekBaseURL},
	}
	for name, endpoints := range cases {
		t.Run(name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			groupID := int64(31)
			account := service.Account{
				ID: 3101, Name: "key", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
				Status: service.StatusActive, Schedulable: true, Concurrency: 5,
				Credentials:       map[string]any{"api_key": "sk-test"},
				ProtocolEndpoints: endpoints,
				GroupIDs:          []int64{groupID},
				AccountGroups:     []service.AccountGroup{{AccountID: 3101, GroupID: groupID}},
			}
			repo := openAIImagesFailoverAccountRepo{accounts: []service.Account{account}}
			upstream := &previousResponseKeyUpstream{}
			cfg := &config.Config{RunMode: config.RunModeSimple}
			cfg.Gateway.MaxAccountSwitches = 3
			billingCache := service.NewBillingCacheService(nil, nil, nil, nil, nil, cfg)
			t.Cleanup(billingCache.Stop)
			gateway := service.NewOpenAIGatewayService(
				repo, nil, nil, nil, nil, nil, cfg, nil, nil,
				service.NewBillingService(cfg, nil), nil, billingCache, upstream,
				&service.DeferredService{}, nil, nil, nil, nil, nil,
				nil,
			)
			cache := &concurrencyCacheMock{
				acquireUserSlotFn:    func(context.Context, int64, int, string) (bool, error) { return true, nil },
				acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil },
			}
			apiKey := &service.APIKey{
				ID: 3201, UserID: 3301, GroupID: &groupID,
				User:  &service.User{ID: 3301, Status: service.StatusActive},
				Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI, Status: service.StatusActive},
			}
			h := newGatewayHandlerOverOpenAIService(cfg, repo, apiKey.Group, gateway, billingCache, service.NewConcurrencyService(cache))
			require.NoError(t, gateway.BindOpenAIHTTPResponseOwner(context.Background(), groupID, "resp_key_continuation", apiKey.UserID, apiKey.ID))

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(
				`{"model":"deepseek-flash","stream":false,"previous_response_id":"resp_key_continuation","input":"hello"}`,
			))
			c.Request.Header.Set("Content-Type", "application/json")
			// 入站协议由 InboundEndpointMiddleware 写入请求 context，这里直接构造 handler，手动补上。
			c.Request = c.Request.WithContext(service.WithInboundProtocol(c.Request.Context(), service.APIProtocolResponses))
			c.Set(string(middleware.ContextKeyAPIKey), apiKey)
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 1})

			h.Responses(c)

			upstream.mu.Lock()
			hits := upstream.hits
			upstream.mu.Unlock()
			require.Zero(t, hits, "an excluded key must never reach the upstream")
			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			require.Contains(t, w.Body.String(), "previous_response_id requires")
		})
	}
}
