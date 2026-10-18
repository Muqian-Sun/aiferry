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

const (
	videoGrokID   int64 = 1
	videoRelayAID int64 = 2
	videoRelayBID int64 = 3
	videoKeyID    int64 = 20
	videoUserID   int64 = 10
	videoEntryID  int64 = 1
)

// videoTaskRepoStub 内存版任务表：读写都拷贝，终态不再改写，计费抢占在锁内原子完成（同 SQL 的条件更新）。
type videoTaskRepoStub struct {
	mu     sync.Mutex
	tasks  map[string]*service.VideoTask
	claims int
}

func (r *videoTaskRepoStub) CreateVideoTask(_ context.Context, task *service.VideoTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	clone := *task
	r.tasks[task.ID] = &clone
	return nil
}

func (r *videoTaskRepoStub) GetVideoTask(_ context.Context, id string, apiKeyID int64) (*service.VideoTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[id]
	if !ok || task.APIKeyID != apiKeyID {
		return nil, service.ErrVideoTaskNotFound
	}
	clone := *task
	return &clone, nil
}

func (r *videoTaskRepoStub) UpdateVideoTaskResult(_ context.Context, task *service.VideoTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	stored := r.tasks[task.ID]
	if stored == nil || stored.State.IsTerminal() {
		return nil
	}
	stored.State, stored.Progress, stored.VideoURL = task.State, task.Progress, task.VideoURL
	stored.ErrorCode, stored.ErrorMessage = task.ErrorCode, task.ErrorMessage
	stored.DurationSeconds, stored.Resolution = task.DurationSeconds, task.Resolution
	return nil
}

func (r *videoTaskRepoStub) ClaimVideoTaskBilling(_ context.Context, id string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	task := r.tasks[id]
	if task == nil || task.BilledAt != nil {
		return false, nil
	}
	now := time.Now()
	task.BilledAt = &now
	r.claims++
	return true, nil
}

func (r *videoTaskRepoStub) ReleaseVideoTaskBilling(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if task := r.tasks[id]; task != nil {
		task.BilledAt = nil
	}
	return nil
}

func (r *videoTaskRepoStub) DeleteVideoTasksBefore(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (r *videoTaskRepoStub) only(t *testing.T) *service.VideoTask {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	require.Len(t, r.tasks, 1)
	for _, task := range r.tasks {
		clone := *task
		return &clone
	}
	return nil
}

// videoUpstream 把上游请求交给 http.Handler（httptest 录制）。
type videoUpstream struct {
	service.HTTPUpstream
	mu      sync.Mutex
	handler http.Handler
	calls   []videoUpstreamCall
}

type videoUpstreamCall struct {
	Method, URL, Auth string
	Body              []byte
	AccountID         int64
}

func (u *videoUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}
	u.mu.Lock()
	u.calls = append(u.calls, videoUpstreamCall{Method: req.Method, URL: req.URL.String(), Auth: req.Header.Get("Authorization"), Body: body, AccountID: accountID})
	u.mu.Unlock()
	replay := req.Clone(req.Context())
	replay.Body = io.NopCloser(bytes.NewReader(body))
	rec := httptest.NewRecorder()
	u.handler.ServeHTTP(rec, replay)
	return rec.Result(), nil
}

func (u *videoUpstream) recorded() []videoUpstreamCall {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]videoUpstreamCall(nil), u.calls...)
}

func (u *videoUpstream) count(method, urlPrefix string) int {
	n := 0
	for _, call := range u.recorded() {
		if call.Method == method && strings.HasPrefix(call.URL, urlPrefix) {
			n++
		}
	}
	return n
}

func videoPrice(v float64) *float64 { return &v }

// videoCatalogEntry 视频条目：每秒 480p 0.08 / 720p 0.14 / 1080p 0.25；承接的上游按次价 0.04（基础档），各档同比例。
func videoCatalogEntry(model, vendor string) *service.ModelCatalogEntry {
	return &service.ModelCatalogEntry{
		ID: videoEntryID, ModelID: model, Vendor: vendor, BillingMode: service.BillingModeVideo,
		Status: service.ModelCatalogStatusListed, ManagedBy: service.ModelCatalogManagedBySeed, PerRequestPrice: videoPrice(0.08),
		Intervals: []service.PricingInterval{
			{TierLabel: service.VideoBillingResolution480P, PerRequestPrice: videoPrice(0.08), SortOrder: 0},
			{TierLabel: service.VideoBillingResolution720P, PerRequestPrice: videoPrice(0.14), SortOrder: 1},
			{TierLabel: service.VideoBillingResolution1080P, PerRequestPrice: videoPrice(0.25), SortOrder: 2},
		},
		Bindings: []service.ModelCatalogBinding{
			{EntryID: videoEntryID, AccountID: videoGrokID, PerRequestPrice: videoPrice(0.02)},
			{EntryID: videoEntryID, AccountID: videoRelayAID, PerRequestPrice: videoPrice(0.04)},
			{EntryID: videoEntryID, AccountID: videoRelayBID, PerRequestPrice: videoPrice(0.04)},
		},
	}
}

type videoCatalogStub struct {
	entries map[string]*service.ModelCatalogEntry
}

func (s videoCatalogStub) LookupPricingEntry(_ context.Context, model string) *service.ModelCatalogEntry {
	return s.entries[model]
}

type videoHarness struct {
	engine    *gin.Engine
	handler   *OpenAIGatewayHandler
	gateway   *service.OpenAIGatewayService
	repo      *videoTaskRepoStub
	accounts  []service.Account
	upstream  *videoUpstream
	mux       *http.ServeMux
	usageLogs *handlerUsageLogRepoStub
	traces    *schedulingTraceRepoStub
	writer    *service.SchedulingTraceWriter
	entries   map[string]*service.ModelCatalogEntry
}

// newVideoHarness 三个渠道：Grok 成品号与两个中转 key（relay-a / relay-b，都配了 chat_completions 地址），
// 都绑在同一个视频条目上；priority 决定选号先后。
func newVideoHarness(t *testing.T, priorities map[int64]int) *videoHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)
	priority := func(id int64) int { return priorities[id] }
	accounts := []service.Account{
		{ID: videoGrokID, Name: "grok-oauth", Platform: service.PlatformGrok, Type: service.AccountTypeOAuth,
			Status: service.StatusActive, Schedulable: true, Concurrency: 5, Priority: priority(videoGrokID),
			Credentials: map[string]any{"access_token": "grok-token", "refresh_token": "grok-refresh",
				"expires_at": time.Now().Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339)}},
		{ID: videoRelayAID, Name: "relay-a", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
			Status: service.StatusActive, Schedulable: true, Concurrency: 5, Priority: priority(videoRelayAID),
			Credentials:           map[string]any{"api_key": "sk-relay-a"},
			CatalogUpstreamModels: map[string]string{"sora-2": "sora-2-relay", "grok-imagine-video": "grok-video-relay"},
			ProtocolEndpoints:     map[string]string{service.APIProtocolChatCompletions: "https://relay-a.example.com/v1"}},
		{ID: videoRelayBID, Name: "relay-b", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
			Status: service.StatusActive, Schedulable: true, Concurrency: 5, Priority: priority(videoRelayBID),
			Credentials:       map[string]any{"api_key": "sk-relay-b"},
			ProtocolEndpoints: map[string]string{service.APIProtocolChatCompletions: "https://relay-b.example.com/v1"}},
	}
	h := &videoHarness{
		accounts: accounts, repo: &videoTaskRepoStub{tasks: map[string]*service.VideoTask{}},
		mux: http.NewServeMux(), usageLogs: &handlerUsageLogRepoStub{}, traces: &schedulingTraceRepoStub{},
		entries: map[string]*service.ModelCatalogEntry{
			"sora-2":             videoCatalogEntry("sora-2", "openai"),
			"grok-imagine-video": videoCatalogEntry("grok-imagine-video", "xai"),
		},
	}
	h.upstream = &videoUpstream{handler: h.mux}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	repo := openAIImagesFailoverAccountRepo{accounts: h.accounts}
	slots := &grokMediaSlotsCache{accounts: map[string]int64{}, users: map[string]int64{}}
	concurrency := service.NewConcurrencyService(slots)
	bindings := &grokMediaSlotBindings{}
	scheduler := service.NewGatewayService(repo, nil, nil, nil, nil, bindings, cfg, nil, concurrency,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	h.gateway = service.NewOpenAIGatewayService(repo, h.usageLogs, nil, handlerUserRepoStub{}, handlerSubRepoStub{}, bindings, cfg, nil,
		concurrency, service.NewBillingService(), nil, nil, h.upstream, &service.DeferredService{}, nil, service.NewGrokTokenProvider(repo, nil),
		service.NewModelPricingResolver(videoCatalogStub{entries: h.entries}), nil, nil, scheduler)
	billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, cfg)
	t.Cleanup(billing.Stop)
	h.handler = NewOpenAIGatewayHandler(h.gateway, concurrency, billing, service.NewAPIKeyService(nil, nil, nil, cfg), nil, nil, nil, nil, cfg, listAllCatalogStub{})
	h.handler.grokMediaEligibilityProber = grokMediaSlotProber(func(context.Context, int64) (bool, string, error) { return true, "eligible", nil })
	h.handler.videoTasks = service.NewVideoTaskService(h.repo, h.gateway)
	t.Cleanup(h.handler.videoTasks.Stop)
	h.writer = service.NewSchedulingTraceWriter(h.traces)
	t.Cleanup(h.writer.Stop)

	prepare := func(c *gin.Context) {
		keyID := videoKeyID
		if c.GetHeader("X-Test-Other-Key") != "" {
			keyID = videoKeyID + 1
		}
		apiKey := &service.APIKey{ID: keyID, UserID: videoUserID, Status: service.StatusActive, User: &service.User{ID: videoUserID, Balance: 100, Concurrency: 10}}
		c.Set(string(middleware2.ContextKeyAPIKey), apiKey)
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: videoUserID, Concurrency: 10})
		if c.Request.Method == http.MethodPost {
			body, _ := io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
			model := gjson.GetBytes(body, "model").String()
			if entry := h.entries[model]; entry != nil {
				c.Request = c.Request.WithContext(service.WithCatalogRoute(c.Request.Context(), service.CatalogRoute{
					EntryID: entry.ID, CanonicalModel: entry.ModelID, RequestedModel: model, Entry: entry}))
			}
		}
		c.Next()
	}
	engine := gin.New()
	engine.Use(prepare, InboundEndpointMiddleware(), SchedulingTraceMiddleware(h.writer))
	engine.POST("/v1/videos", h.handler.VideoTaskCreate(service.VideoShapeOpenAI))
	engine.POST("/v1/videos/generations", h.handler.VideoTaskCreate(service.VideoShapeXAI))
	engine.POST("/v1/video/generations", h.handler.VideoTaskCreate(service.VideoShapeNewAPI))
	engine.GET("/v1/videos/:request_id", h.handler.VideoTaskStatus(service.VideoShapeOpenAI))
	engine.GET("/v1/videos/:request_id/content", h.handler.VideoTaskContent)
	engine.GET("/v1/videos/generations/:request_id", h.handler.VideoTaskStatus(service.VideoShapeXAI))
	engine.GET("/v1/video/generations/:task_id", h.handler.VideoTaskStatus(service.VideoShapeNewAPI))
	h.engine = engine
	return h
}

func (h *videoHarness) do(t *testing.T, method, path, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.engine.ServeHTTP(rec, req)
	return rec
}

func writeJSONBody(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

// requireVideoUsage 只有一条用量：一条视频，计费档 × 秒数按目录价，渠道成本按承接的上游按次价。
func requireVideoUsage(t *testing.T, logs *handlerUsageLogRepoStub, taskID string, accountID int64, resolution string, seconds int, userPerSecond, upstreamPerSecond float64, inbound, upstreamEndpoint string) {
	t.Helper()
	recorded := logs.recorded()
	require.Len(t, recorded, 1)
	log := recorded[0]
	require.Equal(t, taskID, log.RequestID)
	require.Equal(t, accountID, log.AccountID)
	require.Equal(t, 1, log.VideoCount)
	require.Equal(t, resolution, *log.VideoResolution)
	require.Equal(t, seconds, *log.VideoDurationSeconds)
	require.Equal(t, string(service.BillingModeVideo), *log.BillingMode)
	require.InDelta(t, userPerSecond*float64(seconds), log.TotalCost, 1e-9, "目录价：分辨率档单价 × 秒")
	require.InDelta(t, upstreamPerSecond*float64(seconds), log.AccountCost, 1e-9, "渠道成本：承接的上游按次价 × 秒")
	require.Equal(t, inbound, *log.InboundEndpoint)
	require.Equal(t, upstreamEndpoint, *log.UpstreamEndpoint)
}

// OpenAI Videos 形态创建 → new-api 中转 key 承接 → 轮询到成功（计费一次）→ 取成片（代理上游地址）。
func TestVideoTaskOpenAIShapeEndToEndViaNewAPIRelay(t *testing.T) {
	h := newVideoHarness(t, map[int64]int{videoRelayAID: 0, videoRelayBID: 1, videoGrokID: 2})
	status := `{"code":"success","data":{"task_id":"na-1","status":"IN_PROGRESS","progress":"30%"}}`
	h.mux.HandleFunc("POST relay-a.example.com/v1/video/generations", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONBody(w, 200, `{"task_id":"na-1","status":"queued"}`)
	})
	h.mux.HandleFunc("GET relay-a.example.com/v1/video/generations/na-1", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONBody(w, 200, status)
	})
	h.mux.HandleFunc("GET cdn.example.com/out.mp4", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = io.WriteString(w, "MP4DATA")
	})

	rec := h.do(t, http.MethodPost, "/v1/videos", `{"model":"sora-2","prompt":"waves","seconds":"8","size":"1280x720"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	id := gjson.Get(rec.Body.String(), "id").String()
	require.True(t, service.IsVideoTaskID(id), rec.Body.String())
	require.Equal(t, "video", gjson.Get(rec.Body.String(), "object").String())
	require.Equal(t, "queued", gjson.Get(rec.Body.String(), "status").String())
	require.Equal(t, "1280x720", gjson.Get(rec.Body.String(), "size").String())
	require.Equal(t, "8", gjson.Get(rec.Body.String(), "seconds").String())

	create := h.upstream.recorded()[0]
	require.Equal(t, "https://relay-a.example.com/v1/video/generations", create.URL)
	require.Equal(t, "Bearer sk-relay-a", create.Auth)
	require.Equal(t, "sora-2-relay", gjson.GetBytes(create.Body, "model").String())
	require.Equal(t, int64(8), gjson.GetBytes(create.Body, "duration").Int())
	require.Equal(t, "1280x720", gjson.GetBytes(create.Body, "size").String())
	task := h.repo.only(t)
	require.Equal(t, videoRelayAID, task.AccountID)
	require.Equal(t, service.VideoUpstreamNewAPIRelay, task.UpstreamKind)
	require.Equal(t, "na-1", task.UpstreamTaskID)
	require.Empty(t, h.usageLogs.recorded(), "创建不计费")

	rec = h.do(t, http.MethodGet, "/v1/videos/"+id, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "in_progress", gjson.Get(rec.Body.String(), "status").String())
	require.Equal(t, int64(30), gjson.Get(rec.Body.String(), "progress").Int())
	require.Empty(t, h.usageLogs.recorded())

	status = `{"code":"success","data":{"task_id":"na-1","status":"SUCCESS","progress":"100%","result_url":"https://cdn.example.com/out.mp4"}}`
	rec = h.do(t, http.MethodGet, "/v1/videos/"+id, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "completed", gjson.Get(rec.Body.String(), "status").String())
	requireVideoUsage(t, h.usageLogs, id, videoRelayAID, service.VideoBillingResolution720P, 8, 0.14, 0.07, "/v1/videos", "/v1/video/generations")

	statusPolls := h.upstream.count(http.MethodGet, "https://relay-a.example.com/v1/video/generations/na-1")
	rec = h.do(t, http.MethodGet, "/v1/video/generations/"+id, "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "completed", gjson.Get(rec.Body.String(), "status").String(), "按 new-api 路径的形态回包")
	require.Equal(t, "/v1/videos/"+id+"/content", gjson.Get(rec.Body.String(), "url").String())
	require.Equal(t, "SUCCESS", gjson.Get(rec.Body.String(), "data.status").String())
	rec = h.do(t, http.MethodGet, "/v1/videos/generations/"+id, "")
	require.Equal(t, "done", gjson.Get(rec.Body.String(), "status").String(), "按 xAI 路径的形态回包")
	require.Equal(t, statusPolls, h.upstream.count(http.MethodGet, "https://relay-a.example.com/v1/video/generations/na-1"), "终态不再查上游")

	rec = h.do(t, http.MethodGet, "/v1/videos/"+id+"/content", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "MP4DATA", rec.Body.String())
	require.Equal(t, "video/mp4", rec.Header().Get("Content-Type"))
	calls := h.upstream.recorded()
	require.Equal(t, "https://cdn.example.com/out.mp4", calls[len(calls)-1].URL)
	require.Empty(t, calls[len(calls)-1].Auth, "上游给的第三方地址不带 key")
	require.Len(t, h.usageLogs.recorded(), 1, "取成片不再计费")
}

// xAI 形态创建 → xAI 形态的中转 key 承接；状态按 xAI 形态回包，成片走中转自己的代理地址（带 key）。
func TestVideoTaskXAIShapeViaXAIRelay(t *testing.T) {
	h := newVideoHarness(t, map[int64]int{videoRelayAID: 0, videoGrokID: 1, videoRelayBID: 2})
	h.mux.HandleFunc("POST relay-a.example.com/v1/videos/generations", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONBody(w, 200, `{"request_id":"xr-1"}`)
	})
	h.mux.HandleFunc("GET relay-a.example.com/v1/videos/xr-1", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONBody(w, 200, `{"status":"done","model":"grok-video-relay","video":{"url":"/v1/videos/xr-1/content","duration":6}}`)
	})
	h.mux.HandleFunc("GET relay-a.example.com/v1/videos/xr-1/content", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "XAIMP4")
	})

	rec := h.do(t, http.MethodPost, "/v1/videos/generations",
		`{"model":"grok-imagine-video","prompt":"animate","image":{"url":"https://example.com/f.png"},"duration":6,"resolution":"720p","aspect_ratio":"16:9"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	id := gjson.Get(rec.Body.String(), "request_id").String()
	require.True(t, service.IsVideoTaskID(id), rec.Body.String())
	create := h.upstream.recorded()[0]
	require.Equal(t, "https://relay-a.example.com/v1/videos/generations", create.URL)
	require.JSONEq(t, `{"model":"grok-video-relay","prompt":"animate","image":{"url":"https://example.com/f.png"},"duration":6,"aspect_ratio":"16:9","resolution":"720p"}`, string(create.Body))
	require.Equal(t, service.VideoUpstreamXAIRelay, h.repo.only(t).UpstreamKind)

	rec = h.do(t, http.MethodGet, "/v1/videos/"+id, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "done", gjson.Get(rec.Body.String(), "status").String(), "创建于 xAI 形态，/videos/{id} 按 xAI 回包")
	require.Equal(t, "/v1/videos/"+id+"/content", gjson.Get(rec.Body.String(), "video.url").String())
	require.Equal(t, int64(6), gjson.Get(rec.Body.String(), "video.duration").Int())
	requireVideoUsage(t, h.usageLogs, id, videoRelayAID, service.VideoBillingResolution720P, 6, 0.14, 0.07, "/v1/videos/generations", "/v1/videos/generations")

	rec = h.do(t, http.MethodGet, "/v1/videos/"+id+"/content", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "XAIMP4", rec.Body.String())
	calls := h.upstream.recorded()
	require.Equal(t, "https://relay-a.example.com/v1/videos/xr-1/content", calls[len(calls)-1].URL)
	require.Equal(t, "Bearer sk-relay-a", calls[len(calls)-1].Auth, "中转自己的代理下载要带 key")
}

// Grok 成品号 + xAI 形态经同一框架：走 xAI 原生端点，按成品号凭据。
func TestVideoTaskGrokOAuthViaFramework(t *testing.T) {
	h := newVideoHarness(t, map[int64]int{videoGrokID: 0, videoRelayAID: 1, videoRelayBID: 2})
	h.mux.HandleFunc("POST api.x.ai/v1/videos/generations", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONBody(w, 200, `{"request_id":"g-1"}`)
	})
	h.mux.HandleFunc("GET api.x.ai/v1/videos/g-1", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONBody(w, 200, `{"status":"done","video":{"url":"https://vidgen.x.ai/g-1.mp4","duration":10}}`)
	})
	h.mux.HandleFunc("GET vidgen.x.ai/g-1.mp4", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "GROKMP4") })

	rec := h.do(t, http.MethodPost, "/v1/videos/generations", `{"model":"grok-imagine-video","prompt":"waves","duration":10,"resolution":"480p"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	id := gjson.Get(rec.Body.String(), "request_id").String()
	create := h.upstream.recorded()[0]
	require.Equal(t, "https://api.x.ai/v1/videos/generations", create.URL)
	require.Equal(t, "Bearer grok-token", create.Auth)
	require.Equal(t, service.VideoUpstreamGrokOAuth, h.repo.only(t).UpstreamKind)

	rec = h.do(t, http.MethodGet, "/v1/videos/"+id+"/content", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "GROKMP4", rec.Body.String())
	requireVideoUsage(t, h.usageLogs, id, videoGrokID, service.VideoBillingResolution480P, 10, 0.08, 0.02, "/v1/videos/generations", "/v1/videos/generations")
}

// 创建失败按上游状态 failover：第一个渠道 5xx，第二个接下任务；上游错误进运维事件（调度轨迹的尝试也读它）。
func TestVideoTaskCreateFailsOverOnUpstream5xx(t *testing.T) {
	h := newVideoHarness(t, map[int64]int{videoRelayAID: 0, videoRelayBID: 1, videoGrokID: 2})
	h.mux.HandleFunc("POST relay-a.example.com/v1/video/generations", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONBody(w, http.StatusServiceUnavailable, `{"error":{"message":"overloaded"}}`)
	})
	h.mux.HandleFunc("POST relay-b.example.com/v1/video/generations", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONBody(w, 200, `{"task_id":"nb-1"}`)
	})

	rec := h.do(t, http.MethodPost, "/v1/video/generations", `{"model":"sora-2","prompt":"waves","duration":5}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "queued", gjson.Get(rec.Body.String(), "status").String())
	require.True(t, service.IsVideoTaskID(gjson.Get(rec.Body.String(), "task_id").String()))
	calls := h.upstream.recorded()
	require.Len(t, calls, 2)
	require.Equal(t, []int64{videoRelayAID, videoRelayBID}, []int64{calls[0].AccountID, calls[1].AccountID})
	task := h.repo.only(t)
	require.Equal(t, videoRelayBID, task.AccountID)
	require.Equal(t, "nb-1", task.UpstreamTaskID)

	h.writer.Stop()
	h.traces.mu.Lock()
	defer h.traces.mu.Unlock()
	require.Len(t, h.traces.inserted, 1, "创建记一条轨迹")
	trace := h.traces.inserted[0]
	require.Equal(t, videoRelayBID, *trace.FinalAccountID)
	require.Equal(t, 1, trace.SwitchCount)
	require.Len(t, trace.Attempts, 1)
	require.Equal(t, videoRelayAID, trace.Attempts[0].AccountID)
	require.Equal(t, "failover", trace.Attempts[0].Kind)
}

// 上游明确拒绝请求（4xx）不换号，原样回给客户端。
func TestVideoTaskCreateReturnsUpstreamClientErrorWithoutFailover(t *testing.T) {
	h := newVideoHarness(t, map[int64]int{videoRelayAID: 0, videoRelayBID: 1, videoGrokID: 2})
	h.mux.HandleFunc("POST relay-a.example.com/v1/video/generations", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONBody(w, http.StatusBadRequest, `{"error":{"message":"prompt too long"}}`)
	})
	rec := h.do(t, http.MethodPost, "/v1/videos", `{"model":"sora-2","prompt":"waves"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Equal(t, "prompt too long", gjson.Get(rec.Body.String(), "error.message").String())
	require.Len(t, h.upstream.recorded(), 1)
	require.Empty(t, h.repo.tasks)
}

// createSucceededTask 建一个任务，上游状态恒为成功；statusGate 非空时每次状态查询先过它（并发测试用来让多个轮询同时在途）。
func (h *videoHarness) createSucceededTask(t *testing.T, statusGate ...func()) string {
	t.Helper()
	h.mux.HandleFunc("POST relay-a.example.com/v1/video/generations", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONBody(w, 200, `{"task_id":"na-1"}`)
	})
	h.mux.HandleFunc("GET relay-a.example.com/v1/video/generations/na-1", func(w http.ResponseWriter, _ *http.Request) {
		for _, gate := range statusGate {
			gate()
		}
		writeJSONBody(w, 200, `{"task_id":"na-1","status":"completed","url":"https://cdn.example.com/out.mp4","metadata":{"duration":4}}`)
	})
	h.mux.HandleFunc("GET cdn.example.com/out.mp4", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "MP4") })
	rec := h.do(t, http.MethodPost, "/v1/videos", `{"model":"sora-2","prompt":"waves","seconds":"8","size":"720x1280"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return gjson.Get(rec.Body.String(), "id").String()
}

// 别的 key 查不到这个任务（404，不碰上游）；不认识的本网关 ID 同样 404。
func TestVideoTaskLookupIsScopedToTheCreatingKey(t *testing.T) {
	h := newVideoHarness(t, map[int64]int{videoRelayAID: 0, videoRelayBID: 1, videoGrokID: 2})
	id := h.createSucceededTask(t)
	before := len(h.upstream.recorded())
	for _, path := range []string{"/v1/videos/" + id, "/v1/videos/" + id + "/content", "/v1/video/generations/" + id, "/v1/videos/generations/" + id} {
		rec := h.do(t, http.MethodGet, path, "", "X-Test-Other-Key", "1")
		require.Equal(t, http.StatusNotFound, rec.Code, path)
		require.Equal(t, "Video task not found", gjson.Get(rec.Body.String(), "error.message").String(), path)
	}
	rec := h.do(t, http.MethodGet, "/v1/videos/vtask_doesnotexist", "")
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Len(t, h.upstream.recorded(), before, "不属于这把 key 的任务不查上游")
	require.Empty(t, h.usageLogs.recorded())
}

// 不是 vtask_ 前缀的 ID 是旧 Grok 路径发的 request_id：交给旧的 Grok 查询（它按 Redis 归属键找账号）；
// new-api 路径没有旧任务，直接 404。
func TestVideoTaskLegacyGrokRequestIDFallsBack(t *testing.T) {
	h := newVideoHarness(t, map[int64]int{videoGrokID: 0})
	rec := h.do(t, http.MethodGet, "/v1/videos/legacy-123", "")
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "Video request not found", gjson.Get(rec.Body.String(), "error.message").String(), "旧 Grok 查询的错误")

	h.mux.HandleFunc("GET api.x.ai/v1/videos/legacy-ok", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONBody(w, 200, `{"status":"pending"}`)
	})
	require.NoError(t, h.gateway.BindGrokMediaVideoRequestAccount(context.Background(), "legacy-ok", videoUserID, videoKeyID, videoGrokID))
	rec = h.do(t, http.MethodGet, "/v1/videos/legacy-ok", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "pending", gjson.Get(rec.Body.String(), "status").String())
	require.Equal(t, 1, h.upstream.count(http.MethodGet, "https://api.x.ai/v1/videos/legacy-ok"))

	rec = h.do(t, http.MethodGet, "/v1/video/generations/legacy-ok", "")
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "Video task not found", gjson.Get(rec.Body.String(), "error.message").String())
}

// 并发轮询同时看到成功：只有一个抢到计费，只有一条用量。
func TestVideoTaskConcurrentPollsBillOnce(t *testing.T) {
	h := newVideoHarness(t, map[int64]int{videoRelayAID: 0, videoRelayBID: 1, videoGrokID: 2})
	// 状态查询在上游处会合：凑齐 inFlight 个才一起放行，这些请求都是在任务还没计费时读到的任务。
	const inFlight = 6
	var arrivals sync.Mutex
	count := 0
	released := make(chan struct{})
	gate := func() {
		arrivals.Lock()
		count++
		if count == inFlight {
			close(released)
		}
		arrivals.Unlock()
		select {
		case <-released:
		case <-time.After(5 * time.Second):
		}
	}
	id := h.createSucceededTask(t, gate)
	var wg sync.WaitGroup
	paths := []string{"/v1/videos/" + id, "/v1/videos/" + id + "/content", "/v1/video/generations/" + id}
	for i := range 30 {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			rec := h.do(t, http.MethodGet, path, "")
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		}(paths[i%len(paths)])
	}
	wg.Wait()
	// 720x1280 → 720p；上游报了 4 秒，按实际时长计。
	requireVideoUsage(t, h.usageLogs, id, videoRelayAID, service.VideoBillingResolution720P, 4, 0.14, 0.07, "/v1/videos", "/v1/video/generations")
	h.repo.mu.Lock()
	defer h.repo.mu.Unlock()
	require.Equal(t, 1, h.repo.claims)
}

// 写用量失败释放抢占，下一次轮询重新入账（仍只有一条）。
func TestVideoTaskBillingClaimReleasedWhenUsageFails(t *testing.T) {
	h := newVideoHarness(t, map[int64]int{videoRelayAID: 0, videoRelayBID: 1, videoGrokID: 2})
	id := h.createSucceededTask(t)
	missingParent := int64(999)
	h.accounts[1].ParentAccountID = &missingParent // relay-a 变成母账号找不到的影子号：RecordUsage 解析凭据失败

	rec := h.do(t, http.MethodGet, "/v1/videos/"+id, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "completed", gjson.Get(rec.Body.String(), "status").String())
	require.Empty(t, h.usageLogs.recorded())
	require.Nil(t, h.repo.only(t).BilledAt, "入账失败释放抢占")

	h.accounts[1].ParentAccountID = nil
	rec = h.do(t, http.MethodGet, "/v1/videos/"+id, "")
	require.Equal(t, http.StatusOK, rec.Code)
	requireVideoUsage(t, h.usageLogs, id, videoRelayAID, service.VideoBillingResolution720P, 4, 0.14, 0.07, "/v1/videos", "/v1/video/generations")
	require.NotNil(t, h.repo.only(t).BilledAt)
}

// 只有创建记调度轨迹：新框架的状态 / 内容轮询不经调度器，旧 Grok 查询（经调度器选回归属账号）也不记。
func TestVideoTaskPollsAreNotTraced(t *testing.T) {
	h := newVideoHarness(t, map[int64]int{videoRelayAID: 0, videoRelayBID: 1, videoGrokID: 2})
	id := h.createSucceededTask(t)
	for _, path := range []string{"/v1/videos/" + id, "/v1/videos/" + id + "/content", "/v1/video/generations/" + id, "/v1/videos/generations/" + id} {
		rec := h.do(t, http.MethodGet, path, "")
		require.Equal(t, http.StatusOK, rec.Code, path)
	}
	h.mux.HandleFunc("GET api.x.ai/v1/videos/legacy-ok", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONBody(w, 200, `{"status":"pending"}`)
	})
	require.NoError(t, h.gateway.BindGrokMediaVideoRequestAccount(context.Background(), "legacy-ok", videoUserID, videoKeyID, videoGrokID))
	rec := h.do(t, http.MethodGet, "/v1/videos/legacy-ok", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, 1, h.upstream.count(http.MethodGet, "https://api.x.ai/v1/videos/legacy-ok"), "旧查询确实经调度器选号并转发了")

	h.writer.Stop()
	h.traces.mu.Lock()
	defer h.traces.mu.Unlock()
	require.Len(t, h.traces.inserted, 1, "只有创建那一条")
	require.Equal(t, videoRelayAID, *h.traces.inserted[0].FinalAccountID)
}

// token 计价的条目不能走视频生成（算不出视频价）。
func TestVideoTaskRejectsTokenPricedEntries(t *testing.T) {
	h := newVideoHarness(t, map[int64]int{videoRelayAID: 0})
	h.entries["gpt-5"] = &service.ModelCatalogEntry{ID: 9, ModelID: "gpt-5", Vendor: "openai", Status: service.ModelCatalogStatusListed}
	rec := h.do(t, http.MethodPost, "/v1/videos", `{"model":"gpt-5","prompt":"waves"}`)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "does not support video generation")
	require.Empty(t, h.upstream.recorded())
}
