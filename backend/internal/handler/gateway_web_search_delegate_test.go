//go:build unit

package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// Claude Code 配第三方模型时，那次单独的搜索请求交给 claude-haiku-4-5 执行（端到端：真实 handler → 调度 → Forward，
// 假上游记下请求）。没有真实的 Haiku 上游，搜索结果块是假的。

const (
	delegateGPTEntryID   = 50
	delegateHaikuEntryID = 60
)

// claudeCodeSearchBody 是 Claude Code 2.1.282 执行 WebSearch 时单独发的那次请求（2026-10-03 实测抓到的形状）。
const claudeCodeSearchBody = `{"model":"gpt-5.5","max_tokens":2000,"messages":[{"role":"user","content":[{"type":"text","text":"Perform a web search for the query: Tokyo population 2026"}]}],"tools":[{"type":"web_search_20260209","name":"web_search","max_uses":8,"allowed_domains":["metro.tokyo.lg.jp"]}]}`

const anthropicSearchResponse = `{"id":"msg_1","type":"message","role":"assistant","model":"claude-haiku-4-5","content":[` +
	`{"type":"server_tool_use","id":"srvtoolu_1","name":"web_search","input":{"query":"Tokyo population 2026"}},` +
	`{"type":"web_search_tool_result","tool_use_id":"srvtoolu_1","content":[{"type":"web_search_result","url":"https://www.metro.tokyo.lg.jp","title":"Tokyo"}]},` +
	`{"type":"text","text":"About 14 million."}],"stop_reason":"end_turn",` +
	`"usage":{"input_tokens":1000,"output_tokens":100,"server_tool_use":{"web_search_requests":1}}}`

// catalogBucketSchedulerCache 按目录桶给池：目录桶（PoolID = 条目 ID）只给承接这个条目的账号，与生产的桶规则一致
// （fakeSchedulerCache 不分桶，每个桶都给全部账号）。
type catalogBucketSchedulerCache struct{ fakeSchedulerCache }

func (f *catalogBucketSchedulerCache) GetSnapshot(_ context.Context, bucket service.SchedulerBucket) ([]*service.Account, bool, error) {
	if bucket.PoolID <= 0 {
		return f.accounts, true, nil
	}
	var out []*service.Account
	for _, account := range f.accounts {
		for _, id := range account.CatalogEntryIDs {
			if id == bucket.PoolID {
				out = append(out, account)
			}
		}
	}
	return out, true, nil
}

type delegateCatalogStub struct {
	entries map[string]*service.ModelCatalogEntry
}

func (s delegateCatalogStub) LookupPricingEntry(_ context.Context, model string) *service.ModelCatalogEntry {
	return s.entries[model]
}

func delegateEntries(withHaiku bool) map[string]*service.ModelCatalogEntry {
	price := func(v float64) *float64 { return &v }
	out := map[string]*service.ModelCatalogEntry{
		"gpt-5.5": {ID: delegateGPTEntryID, ModelID: "gpt-5.5", Vendor: "openai", Status: service.ModelCatalogStatusListed,
			BillingMode: service.BillingModeToken, InputPrice: price(5e-6), OutputPrice: price(30e-6)},
	}
	if withHaiku {
		out[service.WebSearchDelegateModel] = &service.ModelCatalogEntry{ID: delegateHaikuEntryID, ModelID: service.WebSearchDelegateModel,
			Vendor: "anthropic", Status: service.ModelCatalogStatusUnlisted, BillingMode: service.BillingModeToken,
			InputPrice: price(1e-6), OutputPrice: price(5e-6)}
	}
	return out
}

func delegateKey(id int64, entryID int64, model string) *service.Account {
	account := keyRouteAccount(id, service.PlatformAnthropic, map[string]string{service.APIProtocolAnthropic: "https://relay.example.com"}, model)
	account.CatalogEntryIDs = []int64{entryID}
	return account
}

func newDelegateHandler(t *testing.T, accounts []*service.Account, withHaiku bool) (*GatewayHandler, *recordingHTTPUpstream, *handlerUsageLogRepoStub) {
	t.Helper()
	cfg := &config.Config{RunMode: config.RunModeSimple}
	upstream := &recordingHTTPUpstream{respBody: anthropicSearchResponse}
	usageLogs := &handlerUsageLogRepoStub{}
	billing := service.NewBillingService()
	resolver := service.NewModelPricingResolver(delegateCatalogStub{entries: delegateEntries(withHaiku)})
	gwSvc := service.NewGatewayService(
		nil, usageLogs, nil, nil, nil, nil, cfg,
		service.NewSchedulerSnapshotService(&catalogBucketSchedulerCache{fakeSchedulerCache{accounts: accounts}}, nil, nil, nil),
		nil, billing,
		service.NewRateLimitService(&overloadRecordingAccountRepo{}, nil, cfg, nil, nil),
		nil, nil,
		upstream,
		nil, nil, nil, nil, nil, nil, nil,
		resolver,
		nil,
	)
	billingCacheSvc := service.NewBillingCacheService(nil, nil, nil, nil, nil, cfg)
	t.Cleanup(billingCacheSvc.Stop)
	return &GatewayHandler{
		gatewayService:      gwSvc,
		billingCacheService: billingCacheSvc,
		apiKeyService:       service.NewAPIKeyService(nil, nil, nil, cfg),
		concurrencyHelper:   NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0),
		modelCatalog:        listAllCatalogStub{},
		maxAccountSwitches:  1,
	}, upstream, usageLogs
}

func newDelegateContext(t *testing.T, body, userAgent string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	entry := delegateEntries(false)["gpt-5.5"]
	ctx := service.WithCatalogRoute(req.Context(), service.CatalogRoute{EntryID: entry.ID, CanonicalModel: entry.ModelID, RequestedModel: entry.ModelID, Entry: entry})
	ctx = service.WithInboundProtocol(ctx, service.APIProtocolAnthropic)
	c.Request = req.WithContext(ctx)
	apiKey := &service.APIKey{ID: 3201, UserID: 4201, Status: service.StatusActive, User: &service.User{ID: 4201, Concurrency: 10, Balance: 100}}
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 10})
	return c, rec
}

const claudeCodeUA = "claude-cli/2.1.282 (external, sdk-cli)"

func TestGatewayHandlerMessages_DelegatesClaudeCodeWebSearchToHaiku(t *testing.T) {
	accounts := []*service.Account{
		delegateKey(301, delegateGPTEntryID, "gpt-5.5"),
		delegateKey(302, delegateHaikuEntryID, service.WebSearchDelegateModel),
	}

	t.Run("Claude Code 的搜索请求交给 Haiku", func(t *testing.T) {
		h, upstream, usageLogs := newDelegateHandler(t, accounts, true)
		c, rec := newDelegateContext(t, claudeCodeSearchBody, claudeCodeUA)

		h.Messages(c)

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		calls := upstream.recorded()
		require.Len(t, calls, 1)
		sent := calls[0].body
		require.Equal(t, service.WebSearchDelegateModel, gjson.GetBytes(sent, "model").String())
		require.JSONEq(t, `[{"type":"web_search_20250305","name":"web_search","max_uses":8,"allowed_domains":["metro.tokyo.lg.jp"]}]`,
			gjson.GetBytes(sent, "tools").Raw, "Haiku 4.5 只支持基础版工具；次数上限与域名限制保留")
		require.Equal(t, "Perform a web search for the query: Tokyo population 2026", gjson.GetBytes(sent, "messages.0.content.0.text").String())

		logs := usageLogs.recorded()
		require.Len(t, logs, 1)
		require.True(t, logs[0].WebSearchDelegated)
		require.Equal(t, "gpt-5.5", logs[0].RequestedModel, "用户站显示客户端请求的模型")
		require.Equal(t, service.WebSearchDelegateModel, logs[0].Model, "按 Haiku 这条目录（联网搜索计费项）计价")
		require.NotNil(t, logs[0].UpstreamModel)
		require.Equal(t, service.WebSearchDelegateModel, *logs[0].UpstreamModel, "管理站对账看得到发往上游的 Haiku")
		require.Equal(t, int64(302), logs[0].AccountID)
		require.Equal(t, 1, logs[0].WebSearchCount)
	})

	t.Run("不是 Claude Code 就原样转给请求的模型", func(t *testing.T) {
		h, upstream, usageLogs := newDelegateHandler(t, accounts, true)
		c, rec := newDelegateContext(t, claudeCodeSearchBody, "curl/8.7.1")

		h.Messages(c)

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		calls := upstream.recorded()
		require.Len(t, calls, 1)
		require.Equal(t, "gpt-5.5", gjson.GetBytes(calls[0].body, "model").String())
		require.Equal(t, "web_search_20260209", gjson.GetBytes(calls[0].body, "tools.0.type").String())
		logs := usageLogs.recorded()
		require.Len(t, logs, 1)
		require.False(t, logs[0].WebSearchDelegated)
		require.Equal(t, int64(301), logs[0].AccountID)
	})

	t.Run("还带了别的工具（正常对话）不代执行", func(t *testing.T) {
		h, upstream, _ := newDelegateHandler(t, accounts, true)
		body := `{"model":"gpt-5.5","max_tokens":16,"messages":[{"role":"user","content":"hi"}],"tools":[{"type":"web_search_20250305","name":"web_search"},{"name":"Bash","input_schema":{"type":"object"}}]}`
		c, rec := newDelegateContext(t, body, claudeCodeUA)

		h.Messages(c)

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Equal(t, "gpt-5.5", gjson.GetBytes(upstream.recorded()[0].body, "model").String())
	})

	t.Run("目录里没有 Haiku 就直接报错", func(t *testing.T) {
		h, upstream, _ := newDelegateHandler(t, accounts, false)
		c, rec := newDelegateContext(t, claudeCodeSearchBody, claudeCodeUA)

		h.Messages(c)

		require.Equal(t, http.StatusServiceUnavailable, rec.Code)
		require.Contains(t, rec.Body.String(), webSearchDelegateUnavailableMessage)
		require.NotContains(t, rec.Body.String(), "haiku", "不向用户提 Haiku")
		require.Empty(t, upstream.recorded())
	})

	t.Run("没有承接 Haiku 的渠道也直接报错，不回落到请求的模型", func(t *testing.T) {
		h, upstream, _ := newDelegateHandler(t, accounts[:1], true)
		c, rec := newDelegateContext(t, claudeCodeSearchBody, claudeCodeUA)

		h.Messages(c)

		require.Equal(t, http.StatusServiceUnavailable, rec.Code)
		require.Contains(t, rec.Body.String(), webSearchDelegateUnavailableMessage)
		require.NotContains(t, rec.Body.String(), "haiku")
		require.Empty(t, upstream.recorded())
	})
}
