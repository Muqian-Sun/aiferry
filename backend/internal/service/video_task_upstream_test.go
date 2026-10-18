//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// videoFakeUpstream 把上游请求交给 http.Handler（httptest 录制），按主机注入传输错误。
type videoFakeUpstream struct {
	HTTPUpstream
	mu           sync.Mutex
	handler      http.Handler
	calls        []videoFakeCall
	transportErr map[string]error
}

type videoFakeCall struct {
	Method, URL, Auth string
	Body              []byte
	AccountID         int64
}

func (u *videoFakeUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}
	u.mu.Lock()
	u.calls = append(u.calls, videoFakeCall{Method: req.Method, URL: req.URL.String(), Auth: req.Header.Get("Authorization"), Body: body, AccountID: accountID})
	err := u.transportErr[req.URL.Host]
	u.mu.Unlock()
	if err != nil {
		return nil, err
	}
	replay := req.Clone(req.Context())
	replay.Body = io.NopCloser(bytes.NewReader(body))
	rec := httptest.NewRecorder()
	u.handler.ServeHTTP(rec, replay)
	return rec.Result(), nil
}

func (u *videoFakeUpstream) recorded() []videoFakeCall {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]videoFakeCall(nil), u.calls...)
}

func videoTestRelayKey(id int64, root string) *Account {
	return &Account{ID: id, Name: "relay", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Concurrency: 5, Credentials: map[string]any{"api_key": "sk-relay"},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: root}}
}

func videoTestGrokOAuth(id int64) *Account {
	return &Account{ID: id, Name: "grok", Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
		Concurrency: 5, Credentials: map[string]any{
			"access_token": "grok-token", "refresh_token": "grok-refresh",
			"expires_at": time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
		}}
}

func videoTestGinContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/vtask_x/content", nil)
	return c
}

func writeVideoJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func TestNewAPIRelayVideoUpstream(t *testing.T) {
	statusBody := ""
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/video/generations", func(w http.ResponseWriter, _ *http.Request) {
		writeVideoJSON(w, 200, `{"task_id":"up-1","status":"queued"}`)
	})
	mux.HandleFunc("GET /v1/video/generations/up-1", func(w http.ResponseWriter, _ *http.Request) {
		writeVideoJSON(w, 200, statusBody)
	})
	mux.HandleFunc("GET /v1/videos/up-1/content", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = io.WriteString(w, "relay-mp4")
	})
	upstream := &videoFakeUpstream{handler: mux}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	adapter := newAPIRelayVideoUpstream{s: svc}
	account := videoTestRelayKey(2, "https://relay.example.com/v1")
	seed := int64(7)
	req := VideoTaskRequest{Model: "sora-2", Prompt: "waves", NegativePrompt: "blur", DurationSeconds: 8,
		Resolution: VideoResolution720P, AspectRatio: "16:9", Seed: &seed,
		Images: []VideoInputImage{{URL: "https://example.com/a.png", Role: VideoInputImageFirstFrame}, {URL: "https://example.com/z.png", Role: VideoInputImageLastFrame}},
		Extra:  map[string]any{"style": "anime", "aspect_ratio": "ignored"}}

	id, err := adapter.Create(context.Background(), videoTestGinContext(), account, req, "upstream-sora")
	require.NoError(t, err)
	require.Equal(t, "up-1", id)
	call := upstream.recorded()[0]
	require.Equal(t, "https://relay.example.com/v1/video/generations", call.URL)
	require.Equal(t, "Bearer sk-relay", call.Auth)
	require.JSONEq(t, `{"model":"upstream-sora","prompt":"waves","image":"https://example.com/a.png","duration":8,"seconds":"8",
		"width":1280,"height":720,"size":"1280x720","resolution":"720p","seed":7,
		"metadata":{"negative_prompt":"blur","aspect_ratio":"16:9","resolution":"720p","last_frame_image":"https://example.com/z.png","style":"anime"}}`, string(call.Body))

	for _, tc := range []struct {
		name, body string
		state      VideoTaskState
		progress   int
		url        string
		duration   float64
		message    string
	}{
		{"source in progress", `{"code":"success","data":{"task_id":"up-1","status":"IN_PROGRESS","progress":"30%"}}`, VideoTaskRunning, 30, "", 0, ""},
		{"source queued", `{"code":"success","data":{"status":"NOT_START","progress":"0%"}}`, VideoTaskQueued, 0, "", 0, ""},
		{"source success", `{"code":"success","data":{"status":"SUCCESS","progress":"100%","result_url":"https://relay.example.com/v1/videos/up-1/content"}}`, VideoTaskSucceeded, 100, "https://relay.example.com/v1/videos/up-1/content", 0, ""},
		{"documented completed", `{"task_id":"up-1","status":"completed","url":"https://cdn.example.com/v.mp4","metadata":{"duration":5,"width":1280,"height":720}}`, VideoTaskSucceeded, -1, "https://cdn.example.com/v.mp4", 5, ""},
		{"gemini realtime", `{"code":"success","data":{"status":"succeeded","url":"https://cdn.example.com/g.mp4"}}`, VideoTaskSucceeded, -1, "https://cdn.example.com/g.mp4", 0, ""},
		{"success without url keeps polling", `{"code":"success","data":{"status":"SUCCESS"}}`, VideoTaskRunning, -1, "", 0, ""},
		{"source failure", `{"code":"success","data":{"status":"FAILURE","fail_reason":"content rejected"}}`, VideoTaskFailed, -1, "", 0, "content rejected"},
		{"documented failed", `{"status":"failed","error":{"code":0,"message":"quota"}}`, VideoTaskFailed, -1, "", 0, "quota"},
		{"unknown status", `{"status":"weird"}`, VideoTaskRunning, -1, "", 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			statusBody = tc.body
			result, err := adapter.Query(context.Background(), videoTestGinContext(), account, "up-1")
			require.NoError(t, err)
			require.Equal(t, tc.state, result.State)
			if tc.progress >= 0 {
				require.NotNil(t, result.Progress)
				require.Equal(t, tc.progress, *result.Progress)
			}
			require.Equal(t, tc.url, result.VideoURL)
			require.InDelta(t, tc.duration, result.DurationSeconds, 1e-9)
			require.Equal(t, tc.message, result.ErrorMessage)
		})
	}

	// new-api 自己的代理下载地址（同主机）带 key；厂商公开地址不带；私网地址拒绝。
	resp, err := adapter.OpenContent(context.Background(), videoTestGinContext(), account, "up-1",
		VideoTaskResult{State: VideoTaskSucceeded, VideoURL: "https://relay.example.com/v1/videos/up-1/content"})
	require.NoError(t, err)
	content, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, "relay-mp4", string(content))
	calls := upstream.recorded()
	require.Equal(t, "Bearer sk-relay", calls[len(calls)-1].Auth)

	mux.HandleFunc("GET cdn.example.com/v.mp4", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "cdn-mp4") })
	resp, err = adapter.OpenContent(context.Background(), videoTestGinContext(), account, "up-1",
		VideoTaskResult{State: VideoTaskSucceeded, VideoURL: "https://cdn.example.com/v.mp4"})
	require.NoError(t, err)
	_ = resp.Body.Close()
	calls = upstream.recorded()
	require.Equal(t, "https://cdn.example.com/v.mp4", calls[len(calls)-1].URL)
	require.Empty(t, calls[len(calls)-1].Auth, "第三方地址不带 key")

	before := len(upstream.recorded())
	_, err = adapter.OpenContent(context.Background(), videoTestGinContext(), account, "up-1",
		VideoTaskResult{State: VideoTaskSucceeded, VideoURL: "https://127.0.0.1/v.mp4"})
	require.Error(t, err)
	require.Len(t, upstream.recorded(), before, "私网地址不发请求")
}

func TestXAIRelayVideoUpstream(t *testing.T) {
	statusBody := ""
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/videos/generations", func(w http.ResponseWriter, _ *http.Request) {
		writeVideoJSON(w, 200, `{"request_id":"x-1"}`)
	})
	mux.HandleFunc("GET /v1/videos/x-1", func(w http.ResponseWriter, _ *http.Request) { writeVideoJSON(w, 200, statusBody) })
	mux.HandleFunc("GET /v1/videos/x-1/content", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "mp4") })
	upstream := &videoFakeUpstream{handler: mux}
	adapter := xaiRelayVideoUpstream{s: &OpenAIGatewayService{httpUpstream: upstream}}
	account := videoTestRelayKey(3, "https://relay.example.com")

	id, err := adapter.Create(context.Background(), videoTestGinContext(), account, VideoTaskRequest{
		Model: "grok-imagine-video", Prompt: "animate", DurationSeconds: 6, Resolution: VideoResolution720P, AspectRatio: "16:9",
		Images: []VideoInputImage{{URL: "https://example.com/f.png", Role: VideoInputImageFirstFrame}, {URL: "https://example.com/r.png", Role: VideoInputImageReference}},
		Extra:  map[string]any{"camera_motion": "pan"},
	}, "grok-imagine-video-relay")
	require.NoError(t, err)
	require.Equal(t, "x-1", id)
	call := upstream.recorded()[0]
	require.Equal(t, "https://relay.example.com/v1/videos/generations", call.URL)
	require.Equal(t, "Bearer sk-relay", call.Auth)
	require.JSONEq(t, `{"model":"grok-imagine-video-relay","prompt":"animate","image":{"url":"https://example.com/f.png"},
		"reference_images":[{"url":"https://example.com/r.png"}],"duration":6,"aspect_ratio":"16:9","resolution":"720p","camera_motion":"pan"}`, string(call.Body))

	_, err = adapter.Create(context.Background(), videoTestGinContext(), account, VideoTaskRequest{Model: "m", Prompt: "x",
		Images: []VideoInputImage{{URL: "https://example.com/l.png", Role: VideoInputImageLastFrame}}}, "m")
	var clientErr *VideoTaskClientError
	require.ErrorAs(t, err, &clientErr, "xAI 没有尾帧")

	for _, tc := range []struct {
		name, body string
		state      VideoTaskState
		url        string
		duration   float64
		code       string
	}{
		{"pending", `{"status":"pending"}`, VideoTaskQueued, "", 0, ""},
		{"pending with progress", `{"status":"pending","progress":40}`, VideoTaskRunning, "", 0, ""},
		{"done", `{"status":"done","model":"m","video":{"url":"/v1/videos/x-1/content","duration":6}}`, VideoTaskSucceeded, "/v1/videos/x-1/content", 6, ""},
		{"done without url", `{"status":"done","video":{}}`, VideoTaskRunning, "", 0, ""},
		{"failed", `{"status":"failed","error":{"code":"moderated","message":"blocked"}}`, VideoTaskFailed, "", 0, "moderated"},
		{"expired", `{"status":"expired"}`, VideoTaskFailed, "", 0, "expired"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			statusBody = tc.body
			result, err := adapter.Query(context.Background(), videoTestGinContext(), account, "x-1")
			require.NoError(t, err)
			require.Equal(t, tc.state, result.State)
			require.Equal(t, tc.url, result.VideoURL)
			require.InDelta(t, tc.duration, result.DurationSeconds, 1e-9)
			require.Equal(t, tc.code, result.ErrorCode)
		})
	}
	require.Equal(t, "https://relay.example.com/v1/videos/x-1", upstream.recorded()[2].URL)

	resp, err := adapter.OpenContent(context.Background(), videoTestGinContext(), account, "x-1",
		VideoTaskResult{State: VideoTaskSucceeded, VideoURL: "/v1/videos/x-1/content"})
	require.NoError(t, err)
	_ = resp.Body.Close()
	calls := upstream.recorded()
	require.Equal(t, "https://relay.example.com/v1/videos/x-1/content", calls[len(calls)-1].URL, "相对地址按根地址补全")
	require.Equal(t, "Bearer sk-relay", calls[len(calls)-1].Auth)
}

func TestGrokOAuthVideoUpstream(t *testing.T) {
	statusBody := ""
	mux := http.NewServeMux()
	mux.HandleFunc("POST api.x.ai/v1/videos/generations", func(w http.ResponseWriter, _ *http.Request) {
		writeVideoJSON(w, 200, `{"request_id":"g-1"}`)
	})
	mux.HandleFunc("GET api.x.ai/v1/videos/g-1", func(w http.ResponseWriter, _ *http.Request) { writeVideoJSON(w, 200, statusBody) })
	mux.HandleFunc("GET api.x.ai/v1/videos/g-1/content", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "xai-mp4") })
	mux.HandleFunc("GET vidgen.x.ai/signed.mp4", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "signed-mp4") })
	upstream := &videoFakeUpstream{handler: mux}
	adapter := grokOAuthVideoUpstream{s: &OpenAIGatewayService{httpUpstream: upstream, grokTokenProvider: NewGrokTokenProvider(nil, nil)}}
	account := videoTestGrokOAuth(1)

	id, err := adapter.Create(context.Background(), videoTestGinContext(), account, VideoTaskRequest{
		Model: "grok-imagine-video", Prompt: "animate", DurationSeconds: 10, Resolution: VideoResolution720P,
		Images: []VideoInputImage{{URL: "https://example.com/f.png", Role: VideoInputImageFirstFrame}},
	}, "grok-imagine-video-1.5")
	require.NoError(t, err)
	require.Equal(t, "g-1", id)
	call := upstream.recorded()[0]
	require.Equal(t, "https://api.x.ai/v1/videos/generations", call.URL)
	require.Equal(t, "Bearer grok-token", call.Auth)
	require.JSONEq(t, `{"model":"grok-imagine-video-1.5","prompt":"animate","image":{"url":"https://example.com/f.png"},"duration":10,"resolution":"720p"}`, string(call.Body))

	statusBody = `{"status":"done","video":{"url":"https://vidgen.x.ai/signed.mp4","duration":10}}`
	result, err := adapter.Query(context.Background(), videoTestGinContext(), account, "g-1")
	require.NoError(t, err)
	require.Equal(t, VideoTaskSucceeded, result.State)
	require.Equal(t, "https://api.x.ai/v1/videos/g-1", upstream.recorded()[1].URL)

	resp, err := adapter.OpenContent(context.Background(), videoTestGinContext(), account, "g-1", result)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, "signed-mp4", string(body))
	calls := upstream.recorded()
	require.Empty(t, calls[len(calls)-1].Auth, "签名地址不带凭据")

	resp, err = adapter.OpenContent(context.Background(), videoTestGinContext(), account, "g-1", VideoTaskResult{State: VideoTaskSucceeded})
	require.NoError(t, err)
	_ = resp.Body.Close()
	calls = upstream.recorded()
	require.Equal(t, "https://api.x.ai/v1/videos/g-1/content", calls[len(calls)-1].URL)
	require.Equal(t, "Bearer grok-token", calls[len(calls)-1].Auth)

	_, err = adapter.OpenContent(context.Background(), videoTestGinContext(), account, "g-1",
		VideoTaskResult{State: VideoTaskSucceeded, VideoURL: "https://evil.example.com/v.mp4"})
	require.Error(t, err, "只认 xAI 的签名下载域名")
}

func TestVideoTaskSubmitErrorClassification(t *testing.T) {
	status, body := 0, ""
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/video/generations", func(w http.ResponseWriter, _ *http.Request) { writeVideoJSON(w, status, body) })
	upstream := &videoFakeUpstream{handler: mux, transportErr: map[string]error{}}
	gateway := &OpenAIGatewayService{httpUpstream: upstream}
	svc := NewVideoTaskService(nil, gateway)
	t.Cleanup(svc.Stop)
	account := videoTestRelayKey(2, "https://relay.example.com/v1")
	req := VideoTaskRequest{Model: "sora-2", Prompt: "waves"}

	opsKinds := func(c *gin.Context) []string {
		var kinds []string
		for _, event := range OpsUpstreamErrorEventsFromContext(c) {
			kinds = append(kinds, event.Kind)
		}
		return kinds
	}

	status, body = http.StatusServiceUnavailable, `{"error":{"message":"overloaded"}}`
	c := videoTestGinContext()
	_, err := svc.Submit(context.Background(), c, account, "openai", req)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusServiceUnavailable, failoverErr.StatusCode)
	require.Equal(t, []string{"failover"}, opsKinds(c))

	status, body = http.StatusBadRequest, `{"error":{"message":"prompt too long"}}`
	c = videoTestGinContext()
	_, err = svc.Submit(context.Background(), c, account, "openai", req)
	var clientErr *VideoTaskClientError
	require.ErrorAs(t, err, &clientErr)
	require.Equal(t, http.StatusBadRequest, clientErr.StatusCode)
	require.Equal(t, "prompt too long", clientErr.Message)
	require.Equal(t, []string{"http_error"}, opsKinds(c))

	status, body = http.StatusOK, `{"status":"queued"}`
	_, err = svc.Submit(context.Background(), videoTestGinContext(), account, "openai", req)
	require.ErrorAs(t, err, &failoverErr, "没回任务 ID 当上游故障换号")

	upstream.transportErr["relay.example.com"] = errors.New("dial tcp: connection refused")
	c = videoTestGinContext()
	_, err = svc.Submit(context.Background(), c, account, "openai", req)
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.Equal(t, []string{"request_error"}, opsKinds(c))
	delete(upstream.transportErr, "relay.example.com")

	status, body = http.StatusOK, `{"task_id":"up-9"}`
	account.CatalogUpstreamModels = map[string]string{"sora-2": "sora-2-relay"}
	submission, err := svc.Submit(context.Background(), videoTestGinContext(), account, "openai", req)
	require.NoError(t, err)
	require.Equal(t, VideoTaskSubmission{UpstreamKind: VideoUpstreamNewAPIRelay, UpstreamTaskID: "up-9", UpstreamModel: "sora-2-relay"}, submission)
	calls := upstream.recorded()
	require.Equal(t, "sora-2-relay", gjson.GetBytes(calls[len(calls)-1].Body, "model").String(), "发给上游的是承接上的上游模型名")
}

func TestVideoTaskUpstreamRegistry(t *testing.T) {
	relay := videoTestRelayKey(2, "https://relay.example.com/v1")
	grok := videoTestGrokOAuth(1)
	vendorKey := videoTestRelayKey(4, "https://api.minimaxi.com/v1")
	openAIOAuth := &Account{ID: 5, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	for _, tc := range []struct {
		vendor  string
		account *Account
		want    VideoUpstreamKind
		ok      bool
	}{
		{PlatformGrok, grok, VideoUpstreamGrokOAuth, true},
		{PlatformGrok, relay, VideoUpstreamXAIRelay, true},
		{PlatformOpenAI, relay, VideoUpstreamNewAPIRelay, true},
		{"volcengine", relay, VideoUpstreamNewAPIRelay, true},
		{"", relay, VideoUpstreamNewAPIRelay, true},
		{PlatformOpenAI, grok, "", false},
		{PlatformMiniMax, vendorKey, "", false},
		{PlatformOpenAI, openAIOAuth, "", false},
	} {
		kind, ok := VideoTaskUpstreamKindFor(tc.vendor, tc.account)
		require.Equal(t, tc.ok, ok, "%s %d", tc.vendor, tc.account.ID)
		require.Equal(t, tc.want, kind, "%s %d", tc.vendor, tc.account.ID)
	}
	require.Equal(t, PlatformGrok, VideoTaskVendorFor(&ModelCatalogEntry{Vendor: "xai"}))
	require.Equal(t, "volcengine", VideoTaskVendorFor(&ModelCatalogEntry{Vendor: "VolcEngine"}))

	admits := func(vendor string, account *Account) bool {
		ok, _ := SelectOptions{Capability: OpenAIEndpointCapabilityVideoGeneration, VideoVendor: vendor}.admits(nil, nil, account)
		return ok
	}
	require.True(t, admits(PlatformGrok, grok))
	require.True(t, admits(PlatformGrok, relay))
	require.True(t, admits(PlatformOpenAI, relay))
	require.False(t, admits(PlatformOpenAI, grok), "成品号没有非 xAI 厂商的视频适配器")
	require.False(t, admits(PlatformMiniMax, vendorKey), "官方厂商 key 等 PR-D 的适配器")
	noRoot := videoTestRelayKey(6, "")
	noRoot.ProtocolEndpoints = map[string]string{APIProtocolResponses: "https://relay.example.com/v1"}
	require.False(t, admits(PlatformOpenAI, noRoot), "中转 key 要有扩展端点根地址")
	forbidden := videoTestGrokOAuth(7)
	forbidden.Extra = map[string]any{grokBillingExtraKey: &xai.BillingSummary{StatusCode: http.StatusForbidden}}
	require.False(t, admits(PlatformGrok, forbidden), "成品号沿用 Grok 媒体的付费资格")
	require.False(t, grok.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityGrokMediaGeneration) && relay.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityGrokMediaGeneration),
		"旧的 grok_media_generation 仍只认成品号")
}

type videoTaskRepoMemory struct {
	mu      sync.Mutex
	tasks   map[string]*VideoTask
	cutoffs []time.Time
}

func (r *videoTaskRepoMemory) CreateVideoTask(_ context.Context, task *VideoTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	clone := *task
	r.tasks[task.ID] = &clone
	return nil
}

func (r *videoTaskRepoMemory) GetVideoTask(_ context.Context, id string, apiKeyID int64) (*VideoTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[id]
	if !ok || task.APIKeyID != apiKeyID {
		return nil, ErrVideoTaskNotFound
	}
	clone := *task
	return &clone, nil
}

func (r *videoTaskRepoMemory) UpdateVideoTaskResult(_ context.Context, task *VideoTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	stored := r.tasks[task.ID]
	if stored == nil || stored.State.IsTerminal() {
		return nil
	}
	stored.State, stored.VideoURL, stored.ErrorCode, stored.ErrorMessage = task.State, task.VideoURL, task.ErrorCode, task.ErrorMessage
	return nil
}

func (r *videoTaskRepoMemory) ClaimVideoTaskBilling(context.Context, string) (bool, error) {
	return false, nil
}
func (r *videoTaskRepoMemory) ReleaseVideoTaskBilling(context.Context, string) error { return nil }

func (r *videoTaskRepoMemory) DeleteVideoTasksBefore(_ context.Context, cutoff time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cutoffs = append(r.cutoffs, cutoff)
	return 0, nil
}

func TestVideoTaskServicePurgesOnStartAndStops(t *testing.T) {
	repo := &videoTaskRepoMemory{tasks: map[string]*VideoTask{}}
	svc := NewVideoTaskService(repo, &OpenAIGatewayService{})
	require.Eventually(t, func() bool {
		repo.mu.Lock()
		defer repo.mu.Unlock()
		return len(repo.cutoffs) == 1
	}, time.Second, 5*time.Millisecond)
	svc.Stop()
	svc.Stop()
	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.WithinDuration(t, time.Now().Add(-7*24*time.Hour), repo.cutoffs[0], 5*time.Second, "保留 7 天")
}

type videoTaskAccountRepo struct {
	AccountRepository
	accounts map[int64]*Account
}

func (r videoTaskAccountRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	if account, ok := r.accounts[id]; ok {
		return account, nil
	}
	return nil, errors.New("not found")
}

func TestVideoTaskPollMarksGoneTasksFailedAndKeepsStateOnTransientErrors(t *testing.T) {
	status, body := http.StatusOK, `{"status":"pending"}`
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/videos/x-1", func(w http.ResponseWriter, _ *http.Request) { writeVideoJSON(w, status, body) })
	upstream := &videoFakeUpstream{handler: mux}
	account := videoTestRelayKey(3, "https://relay.example.com/v1")
	gateway := &OpenAIGatewayService{httpUpstream: upstream, accountRepo: videoTaskAccountRepo{accounts: map[int64]*Account{3: account}}}
	repo := &videoTaskRepoMemory{tasks: map[string]*VideoTask{}}
	svc := NewVideoTaskService(repo, gateway)
	t.Cleanup(svc.Stop)
	task := &VideoTask{ID: "vtask_1", APIKeyID: 9, AccountID: 3, UpstreamKind: VideoUpstreamXAIRelay, UpstreamTaskID: "x-1", State: VideoTaskQueued}
	require.NoError(t, repo.CreateVideoTask(context.Background(), task))

	status, body = http.StatusInternalServerError, `{"error":{"message":"boom"}}`
	_, _, err := svc.Poll(context.Background(), videoTestGinContext(), task)
	require.Error(t, err)
	require.Equal(t, VideoTaskQueued, task.State, "临时错误不改任务状态")

	status, body = http.StatusNotFound, `{"error":{"message":"no such task"}}`
	result, _, err := svc.Poll(context.Background(), videoTestGinContext(), task)
	require.NoError(t, err)
	require.Equal(t, VideoTaskFailed, result.State)
	stored, err := repo.GetVideoTask(context.Background(), "vtask_1", 9)
	require.NoError(t, err)
	require.Equal(t, VideoTaskFailed, stored.State)
	require.Equal(t, "upstream_task_not_found", stored.ErrorCode)

	status, body = http.StatusOK, `{"status":"done","video":{"url":"https://cdn.example.com/v.mp4"}}`
	_, _, err = svc.Poll(context.Background(), videoTestGinContext(), stored)
	require.NoError(t, err)
	stored, _ = repo.GetVideoTask(context.Background(), "vtask_1", 9)
	require.Equal(t, VideoTaskFailed, stored.State, "终态不再改写")
}

func TestVideoTaskRequestRoundTripsThroughStorage(t *testing.T) {
	seed := int64(9007199254740993)
	audio := true
	req := VideoTaskRequest{Model: "m", Prompt: "p", Seed: &seed, GenerateAudio: &audio, Extra: map[string]any{"n": json.Number("3")}}
	raw, err := MarshalVideoTaskRequest(req)
	require.NoError(t, err)
	back, err := UnmarshalVideoTaskRequest(raw)
	require.NoError(t, err)
	require.Equal(t, seed, *back.Seed)
	require.True(t, *back.GenerateAudio)
	require.Equal(t, json.Number("3"), back.Extra["n"])
	require.True(t, strings.HasPrefix(func() string { id, _ := NewVideoTaskID(); return id }(), VideoTaskIDPrefix))
}
