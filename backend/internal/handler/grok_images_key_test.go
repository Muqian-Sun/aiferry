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
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// /v1/images/* 的 xai 条目由 Grok 成品号与配了 chat_completions 地址的第三方 key 共同承接：
// 选中成品号走 xAI 原生媒体端点，选中 key 走 {chat_completions 根}/v1/images/*；两类账号之间能互相 failover。

const (
	grokImageOAuthID int64 = 1
	grokImageKeyID   int64 = 2
	grokImageModel         = "grok-imagine-image"
)

type grokImageCatalogStub struct{ entry *service.ModelCatalogEntry }

func (s grokImageCatalogStub) LookupPricingEntry(_ context.Context, model string) *service.ModelCatalogEntry {
	if model == s.entry.ModelID {
		return s.entry
	}
	return nil
}

func grokImageCatalogEntry() *service.ModelCatalogEntry {
	price := func(v float64) *float64 { return &v }
	return &service.ModelCatalogEntry{
		ID: listAllCatalogEntryID, ModelID: grokImageModel, Vendor: "xai", BillingMode: service.BillingModeImage,
		Status: service.ModelCatalogStatusListed, ManagedBy: service.ModelCatalogManagedBySeed, PerRequestPrice: price(0.02),
		Bindings: []service.ModelCatalogBinding{
			{EntryID: listAllCatalogEntryID, AccountID: grokImageOAuthID, PerRequestPrice: price(0)},
			{EntryID: listAllCatalogEntryID, AccountID: grokImageKeyID, PerRequestPrice: price(0.01)},
		},
	}
}

type grokImageUpstream struct {
	service.HTTPUpstream
	mu       sync.Mutex
	calls    []int64
	urls     []string
	bodies   [][]byte
	response func(accountID int64) *http.Response
}

func (u *grokImageUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	u.mu.Lock()
	u.calls = append(u.calls, accountID)
	u.urls = append(u.urls, req.URL.String())
	u.bodies = append(u.bodies, body)
	u.mu.Unlock()
	return u.response(accountID), nil
}

func grokImageJSON(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}

const grokImageTwoImages = `{"data":[{"b64_json":"aW1hZ2UtYQ=="},{"b64_json":"aW1hZ2UtYg=="}]}`

func newGrokImageMixedHandler(t *testing.T, keyFirst bool, response func(accountID int64) *http.Response) (*OpenAIGatewayHandler, *grokImageUpstream, *handlerUsageLogRepoStub) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	oauthPriority, keyPriority := 0, 1
	if keyFirst {
		oauthPriority, keyPriority = 1, 0
	}
	accounts := []service.Account{
		{ID: grokImageOAuthID, Name: "grok-oauth", Platform: service.PlatformGrok, Type: service.AccountTypeOAuth,
			Status: service.StatusActive, Schedulable: true, Concurrency: 5, Priority: oauthPriority,
			Credentials: map[string]any{
				"access_token":  "grok-token",
				"refresh_token": "grok-refresh",
				"expires_at":    time.Now().Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339),
			}},
		{ID: grokImageKeyID, Name: "relay-key", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
			Status: service.StatusActive, Schedulable: true, Concurrency: 5, Priority: keyPriority,
			Credentials:           map[string]any{"api_key": "relay-key"},
			CatalogUpstreamModels: map[string]string{grokImageModel: "grok-imagine-image-pro"},
			ProtocolEndpoints:     map[string]string{service.APIProtocolChatCompletions: "https://relay.example.com/v1"}},
	}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	repo := openAIImagesFailoverAccountRepo{accounts: accounts}
	slots := &grokMediaSlotsCache{accounts: map[string]int64{}, users: map[string]int64{}}
	concurrency := service.NewConcurrencyService(slots)
	upstream := &grokImageUpstream{response: response}
	usageLogs := &handlerUsageLogRepoStub{}
	// 利润门全站一档，最低毛利率 0 = 不能亏本（线上默认）
	service.InvalidateProfitControlSettingsCache()
	t.Cleanup(service.InvalidateProfitControlSettingsCache)
	settings := service.NewSettingService(&oauthCaptchaSettingRepo{values: map[string]string{service.SettingKeyProfitMinMargin: "0"}}, &config.Config{})
	scheduler := service.NewGatewayService(repo, nil, nil, nil, nil, &grokMediaSlotBindings{}, cfg, nil, concurrency,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, settings, nil, nil, nil)
	gateway := service.NewOpenAIGatewayService(repo, usageLogs, nil, handlerUserRepoStub{}, handlerSubRepoStub{}, &grokMediaSlotBindings{}, cfg, nil,
		concurrency, service.NewBillingService(), nil, nil, upstream, &service.DeferredService{}, nil, service.NewGrokTokenProvider(repo, nil),
		service.NewModelPricingResolver(grokImageCatalogStub{entry: grokImageCatalogEntry()}), nil, nil, scheduler)
	billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, cfg)
	t.Cleanup(billing.Stop)
	h := NewOpenAIGatewayHandler(gateway, concurrency, billing, service.NewAPIKeyService(nil, nil, nil, cfg), nil, nil, nil, nil, cfg, listAllCatalogStub{})
	h.grokMediaEligibilityProber = grokMediaSlotProber(func(context.Context, int64) (bool, string, error) { return true, "eligible", nil })
	return h, upstream, usageLogs
}

func newGrokImageContext(path string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	return newGrokImageContextFor(path, body, grokImageCatalogEntry())
}

// newGrokImageContextFor 目录路由带上 entry（官方价与承接的上游价，利润门按它比）；用户倍率同认证中间件。
func newGrokImageContextFor(path string, body string, entry *service.ModelCatalogEntry) (*gin.Context, *httptest.ResponseRecorder) {
	user := &service.User{ID: 10, Balance: 100}
	ctx := service.WithUserRateMultiplier(context.Background(), user)
	ctx = service.WithCatalogRoute(ctx, service.CatalogRoute{EntryID: entry.ID, CanonicalModel: entry.ModelID, RequestedModel: entry.ModelID, Entry: entry})
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 20, UserID: 10, User: user})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 10, Concurrency: 5})
	return c, rec
}

// 利润门：key 的上游按张价高于按张售价（没定售价 = 官方价 0.02），排在前面也不派，由成品号接。
func TestGrokImagesProfitGateSkipsLossMakingKey(t *testing.T) {
	h, upstream, logs := newGrokImageMixedHandler(t, true, func(int64) *http.Response {
		return grokImageJSON(http.StatusOK, grokImageTwoImages)
	})
	entry := grokImageCatalogEntry()
	entry.Bindings[1].PerRequestPrice = func(v float64) *float64 { return &v }(0.021)
	c, rec := newGrokImageContextFor("/v1/images/generations", grokImageRequest, entry)

	h.GrokImages(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, []int64{grokImageOAuthID}, upstream.calls)
	requireGrokImageUsage(t, logs, grokImageOAuthID, 0)
}

const grokImageRequest = `{"model":"grok-imagine-image","prompt":"a cat","n":2,"size":"1024x1024","response_format":"b64_json"}`

func (u *grokImageUpstream) keyRequests() (urls []string, bodies [][]byte) {
	u.mu.Lock()
	defer u.mu.Unlock()
	for i, id := range u.calls {
		if id == grokImageKeyID {
			urls = append(urls, u.urls[i])
			bodies = append(bodies, u.bodies[i])
		}
	}
	return urls, bodies
}

func requireGrokImageUsage(t *testing.T, logs *handlerUsageLogRepoStub, accountID int64, accountCost float64) {
	t.Helper()
	logs.mu.Lock()
	defer logs.mu.Unlock()
	require.Len(t, logs.logs, 1)
	log := logs.logs[0]
	require.Equal(t, accountID, log.AccountID)
	require.Equal(t, 2, log.ImageCount)
	require.Equal(t, string(service.BillingModeImage), *log.BillingMode)
	require.InDelta(t, 2*0.02, log.TotalCost, 1e-12, "xai entry per-request price × images")
	require.InDelta(t, 2*0.02, log.ActualCost, 1e-12, "no sale price, no discount: the official price")
	require.InDelta(t, accountCost, log.AccountCost, 1e-12, "binding per-request upstream price × images")
}

func TestGrokImagesFailsOverFromOAuthToKey(t *testing.T) {
	h, upstream, logs := newGrokImageMixedHandler(t, false, func(accountID int64) *http.Response {
		if accountID == grokImageOAuthID {
			// xAI 回 200 但没有图：成品号路径按 failover 处理。
			return grokImageJSON(http.StatusOK, `{"data":[]}`)
		}
		return grokImageJSON(http.StatusOK, grokImageTwoImages)
	})
	c, rec := newGrokImageContext("/v1/images/generations", grokImageRequest)

	h.GrokImages(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, []int64{grokImageOAuthID, grokImageKeyID}, upstream.calls)
	urls, bodies := upstream.keyRequests()
	require.Equal(t, []string{"https://relay.example.com/v1/images/generations"}, urls)
	require.Equal(t, "grok-imagine-image-pro", gjson.GetBytes(bodies[0], "model").String())
	require.Equal(t, 2, int(gjson.Get(rec.Body.String(), "data.#").Int()))
	requireGrokImageUsage(t, logs, grokImageKeyID, 2*0.01)
}

func TestGrokImagesFailsOverFromKeyToOAuth(t *testing.T) {
	h, upstream, logs := newGrokImageMixedHandler(t, true, func(accountID int64) *http.Response {
		if accountID == grokImageKeyID {
			return grokImageJSON(http.StatusServiceUnavailable, `{"error":{"message":"relay overloaded"}}`)
		}
		return grokImageJSON(http.StatusOK, grokImageTwoImages)
	})
	c, rec := newGrokImageContext("/v1/images/generations", grokImageRequest)

	h.GrokImages(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, []int64{grokImageKeyID, grokImageOAuthID}, upstream.calls)
	require.NotContains(t, upstream.urls[1], "relay.example.com", "the Grok OAuth account goes to the xAI media endpoint")
	requireGrokImageUsage(t, logs, grokImageOAuthID, 0)
}

// 异步生图复用 GrokImages：key 路径同样能出图并入账。
func TestAsyncGrokImagesServedByKey(t *testing.T) {
	h, upstream, logs := newGrokImageMixedHandler(t, true, func(int64) *http.Response {
		return grokImageJSON(http.StatusOK, grokImageTwoImages)
	})
	c, _ := newGrokImageContext("/v1/images/generations/async", grokImageRequest)
	taskCtx, recorder, cancel := newAsyncImageContext(c, []byte(grokImageRequest), time.Minute)
	defer cancel()

	(&AsyncImageHandler{openAI: h}).executeWithGateway(service.PlatformGrok, taskCtx)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.True(t, bytes.Contains(recorder.Body.Bytes(), []byte("aW1hZ2UtYQ==")))
	urls, _ := upstream.keyRequests()
	require.Equal(t, []string{"https://relay.example.com/v1/images/generations"}, urls)
	requireGrokImageUsage(t, logs, grokImageKeyID, 2*0.01)
}

// 视频创建仍只认成品号：成品号资格被拒后池里只剩 key，key 不被放行，回「没有可用的 Grok 媒体账号」。
func TestGrokVideoGenerationStillRejectsKeys(t *testing.T) {
	h, upstream, _ := newGrokImageMixedHandler(t, true, func(int64) *http.Response {
		return grokImageJSON(http.StatusOK, `{"request_id":"task"}`)
	})
	c, rec := newGrokImageContext("/v1/videos/generations", `{"model":"grok-imagine-video","prompt":"a cat","duration":6}`)
	h.grokMediaEligibilityProber = grokMediaSlotProber(func(context.Context, int64) (bool, string, error) {
		return false, "billing_forbidden", nil
	})

	h.GrokVideoGeneration(c)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	require.Equal(t, "grok_media_no_eligible_account", gjson.Get(rec.Body.String(), "error.type").String())
	require.Empty(t, upstream.calls)
}
