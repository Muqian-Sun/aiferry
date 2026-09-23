//go:build unit

package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// countTokensAccountRepo：目录条目 199 绑定池里全部账号（count_tokens 带模型 → 走目录桶）。
type countTokensAccountRepo struct {
	openAIImagesFailoverAccountRepo
}

func (r countTokensAccountRepo) ListSchedulingCandidatesByCatalogEntry(_ context.Context, entryID int64) ([]service.Account, error) {
	out := make([]service.Account, 0, len(r.accounts))
	for _, account := range r.accounts {
		account.CatalogEntryIDs = []int64{entryID}
		out = append(out, account)
	}
	return out, nil
}

// newOpenAICountTokensHandlerForTest 装一个只带调度器与计费桩的 OpenAI handler：池 = accounts。
func newOpenAICountTokensHandlerForTest(t *testing.T, accounts []service.Account) *OpenAIGatewayHandler {
	t.Helper()
	accountRepo := countTokensAccountRepo{openAIImagesFailoverAccountRepo{accounts: accounts}}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	gatewayService := service.NewOpenAIGatewayService(
		accountRepo, nil, nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		newTestSchedulerOverRepo(cfg, accountRepo, testOpenAIGroup(1)),
	)
	billingService := service.NewBillingCacheService(nil, nil, nil, nil, nil, cfg)
	t.Cleanup(billingService.Stop)
	return NewOpenAIGatewayHandler(
		gatewayService, service.NewConcurrencyService(nil), billingService,
		service.NewAPIKeyService(nil, nil, nil, cfg), nil, nil, nil, nil, cfg, nil,
	)
}

// count_tokens 没有协议转换、只有 /v1/responses/input_tokens 一条桥：池里只有 chat 地址的 key 时，
// 选号就没有候选，回 404 让客户端本地估算（原来选到 chat key 后在取地址时 500，§6-8）。
func TestOpenAIGatewayCountTokens_NoResponsesCapableAccount404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	chatOnlyKey := service.Account{
		ID: 1, Name: "chat-only-key", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, Concurrency: 5, Priority: 1,
		Credentials:       map[string]any{"api_key": "k", "model_mapping": map[string]any{"gpt-5.6": "gpt-5.6"}},
		ProtocolEndpoints: map[string]string{service.APIProtocolChatCompletions: "https://relay.example.com"},
	}
	handler := newOpenAICountTokensHandlerForTest(t, []service.Account{chatOnlyKey})

	body := []byte(`{"model":"gpt-5.6","messages":[{"role":"user","content":"hello"}]}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	route := service.CatalogRoute{EntryID: 199, CanonicalModel: "gpt-5.6", RequestedModel: "gpt-5.6", Entry: &service.ModelCatalogEntry{ID: 199, ModelID: "gpt-5.6", Vendor: "openai"}}
	c.Request = req.WithContext(service.WithInboundProtocol(service.WithCatalogRoute(context.Background(), route), service.APIProtocolAnthropic))
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 99, User: &service.User{ID: 100, Balance: 10, Concurrency: 10}, Status: service.StatusActive})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 100, Concurrency: 10})

	handler.CountTokens(c)

	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "count_tokens endpoint is not supported for this model")
}

// 对照：同一个池换成配了 responses 地址的 key → 选中并进入转发（夹具没有 HTTP 上游，转发失败但不是 404）。
func TestOpenAIGatewayCountTokens_ResponsesCapableAccountIsSelected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	responsesKey := service.Account{
		ID: 2, Name: "responses-key", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, Concurrency: 5, Priority: 1,
		Credentials:       map[string]any{"api_key": "k", "model_mapping": map[string]any{"gpt-5.6": "gpt-5.6"}},
		ProtocolEndpoints: map[string]string{service.APIProtocolResponses: "https://relay.example.com"},
	}
	handler := newOpenAICountTokensHandlerForTest(t, []service.Account{responsesKey})

	body := []byte(`{"model":"gpt-5.6","messages":[{"role":"user","content":"hello"}]}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	route := service.CatalogRoute{EntryID: 199, CanonicalModel: "gpt-5.6", RequestedModel: "gpt-5.6", Entry: &service.ModelCatalogEntry{ID: 199, ModelID: "gpt-5.6", Vendor: "openai"}}
	c.Request = req.WithContext(service.WithInboundProtocol(service.WithCatalogRoute(context.Background(), route), service.APIProtocolAnthropic))
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 99, User: &service.User{ID: 100, Balance: 10, Concurrency: 10}, Status: service.StatusActive})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 100, Concurrency: 10})

	func() {
		defer func() { _ = recover() }() // 夹具无上游：转发阶段 panic / 报错都算「已选中」
		handler.CountTokens(c)
	}()

	require.NotEqual(t, http.StatusNotFound, rec.Code, rec.Body.String())
}
