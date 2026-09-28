package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// OpenAI Fast 策略写在代码里（service.openAIFastPolicy，默认不设规则），本包换不了它。
// 这里断言两件事：鉴权中间件把用户 ID 以 ctxkey.UserID（int64，按用户生效的 Fast 规则就读它）
// 放进请求 context 并随 Forward 传下去；默认策略下客户端的 priority 档位原样到上游。
// 按用户生效的规则本身由 service 包的 *_UserScopedRuleOverridesGlobalRule 用例覆盖。
func TestAPIKeyAuthForwardsUserIDAndDefaultOpenAIFastPolicyToUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstreamBodies := make(chan []byte, 2)
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read request body", http.StatusInternalServerError)
			return
		}
		upstreamBodies <- body
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_test","object":"response","model":"gpt-5","status":"completed","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`))
	}))
	defer upstreamServer.Close()

	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true

	settingService := service.NewSettingService(&openAIFastPolicyForwardingSettingRepo{}, cfg)
	gatewayService := service.NewOpenAIGatewayService(
		nil, nil, nil, nil, nil, nil, cfg,
		nil, nil, nil, nil, nil, &openAIFastPolicyForwardingHTTPUpstream{client: upstreamServer.Client()},
		nil, nil, nil, nil, nil, settingService,
		nil,
	)

	apiKeys := map[string]*service.APIKey{
		"key-user-42": newOpenAIFastPolicyForwardingAPIKey(1, "key-user-42", 42),
		"key-user-43": newOpenAIFastPolicyForwardingAPIKey(2, "key-user-43", 43),
	}
	apiKeyService := service.NewAPIKeyService(&openAIFastPolicyForwardingAPIKeyRepo{apiKeys: apiKeys}, nil, nil, cfg)
	account := &service.Account{
		ProtocolEndpoints: map[string]string{
			service.APIProtocolChatCompletions: upstreamServer.URL,
			service.APIProtocolResponses:       upstreamServer.URL,
		},
		ID:          900,
		Name:        "openai-upstream",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": upstreamServer.URL,
		},
		Extra: map[string]any{"use_responses_api": true},
	}

	observedUserIDs := make(chan int64, 2)
	router := gin.New()
	router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(apiKeyService, nil, cfg)))
	router.POST("/v1/responses", func(c *gin.Context) {
		userID, _ := c.Request.Context().Value(ctxkey.UserID).(int64)
		observedUserIDs <- userID
		body, readErr := io.ReadAll(c.Request.Body)
		if readErr != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		service.SetOpenAIClientTransport(c, service.OpenAIClientTransportHTTP)
		if _, forwardErr := gatewayService.Forward(c.Request.Context(), c, account, body); forwardErr != nil {
			c.Status(http.StatusBadGateway)
			return
		}
		c.Status(http.StatusOK)
	})

	send := func(apiKey string) {
		request := httptest.NewRequest(
			http.MethodPost,
			"/v1/responses",
			bytes.NewBufferString(`{"model":"gpt-5","stream":false,"service_tier":"priority","input":"hi"}`),
		)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("x-api-key", apiKey)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code)
	}

	send("key-user-42")
	send("key-user-43")

	require.Equal(t, int64(42), <-observedUserIDs)
	require.Equal(t, int64(43), <-observedUserIDs)
	firstUserBody := <-upstreamBodies
	secondUserBody := <-upstreamBodies
	require.Equal(t, service.OpenAIFastTierPriority, gjson.GetBytes(firstUserBody, "service_tier").String())
	require.Equal(t, service.OpenAIFastTierPriority, gjson.GetBytes(secondUserBody, "service_tier").String())
}

func newOpenAIFastPolicyForwardingAPIKey(id int64, key string, userID int64) *service.APIKey {
	return &service.APIKey{
		ID:     id,
		UserID: userID,
		Key:    key,
		Status: service.StatusActive,
		User: &service.User{
			ID:          userID,
			Role:        service.RoleUser,
			Status:      service.StatusActive,
			Balance:     10,
			Concurrency: 1,
		},
	}
}

type openAIFastPolicyForwardingAPIKeyRepo struct {
	service.APIKeyRepository
	apiKeys map[string]*service.APIKey
}

func (r *openAIFastPolicyForwardingAPIKeyRepo) GetByKeyForAuth(_ context.Context, key string) (*service.APIKey, error) {
	apiKey, ok := r.apiKeys[key]
	if !ok {
		return nil, service.ErrAPIKeyNotFound
	}
	clone := *apiKey
	return &clone, nil
}

func (r *openAIFastPolicyForwardingAPIKeyRepo) UpdateLastUsed(context.Context, int64, time.Time) error {
	return nil
}

// openAIFastPolicyForwardingSettingRepo 空库：什么设置都没有。
type openAIFastPolicyForwardingSettingRepo struct {
	service.SettingRepository
}

func (r *openAIFastPolicyForwardingSettingRepo) GetValue(context.Context, string) (string, error) {
	return "", service.ErrSettingNotFound
}

type openAIFastPolicyForwardingHTTPUpstream struct {
	client *http.Client
}

func (u *openAIFastPolicyForwardingHTTPUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return u.client.Do(req)
}

func (u *openAIFastPolicyForwardingHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}
