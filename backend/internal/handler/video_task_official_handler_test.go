//go:build unit

package handler

import (
	"io"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const (
	videoSeedanceKeyID int64 = 30
	videoKlingKeyID    int64 = 31
	videoMiniMaxKeyID  int64 = 32
)

// officialVideoKey 官方视频 key：只配 video 地址（厂商官方根地址）。
func officialVideoKey(id int64, platform, base string, mapping map[string]string) service.Account {
	return service.Account{ID: id, Name: platform + "-video", Platform: platform, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, Concurrency: 5,
		Credentials:           map[string]any{"api_key": "sk-" + platform},
		CatalogUpstreamModels: mapping,
		ProtocolEndpoints:     map[string]string{service.APIProtocolVideo: base}}
}

// officialVideoEntry 视频条目，分档与价同播种表；承接价给基础档（最低档）的上游价。
func officialVideoEntry(id int64, model, vendor string, perSecond map[string]float64, bindings map[int64]float64) *service.ModelCatalogEntry {
	entry := &service.ModelCatalogEntry{ID: id, ModelID: model, Vendor: vendor, BillingMode: service.BillingModeVideo,
		Status: service.ModelCatalogStatusListed, ManagedBy: service.ModelCatalogManagedBySeed}
	for _, resolution := range service.VideoBillingResolutions() {
		price, ok := perSecond[resolution]
		if !ok {
			continue
		}
		if entry.PerRequestPrice == nil {
			entry.PerRequestPrice = videoPrice(price)
		}
		entry.Intervals = append(entry.Intervals, service.PricingInterval{TierLabel: resolution, PerRequestPrice: videoPrice(price), SortOrder: len(entry.Intervals)})
	}
	for accountID, price := range bindings {
		entry.Bindings = append(entry.Bindings, service.ModelCatalogBinding{EntryID: id, AccountID: accountID, PerRequestPrice: videoPrice(price)})
	}
	return entry
}

// 官方 key 排第一，中转在后：中转同样能承接这些条目（new-api 形态），选号先到官方 key。
func newOfficialVideoHarness(t *testing.T) *videoHarness {
	t.Helper()
	h := newVideoHarness(t,
		map[int64]int{videoSeedanceKeyID: 0, videoKlingKeyID: 0, videoMiniMaxKeyID: 0, videoRelayAID: 5, videoRelayBID: 6, videoGrokID: 7},
		officialVideoKey(videoSeedanceKeyID, service.PlatformSeedance, service.DefaultSeedanceCNBaseURL,
			map[string]string{"seedance-2.0": "doubao-seedance-2-0-260128"}),
		officialVideoKey(videoKlingKeyID, service.PlatformKling, service.DefaultKlingIntlBaseURL, nil),
		officialVideoKey(videoMiniMaxKeyID, service.PlatformMiniMax, service.DefaultMiniMaxVideoCNBaseURL, nil),
	)
	h.entries["seedance-2.0"] = officialVideoEntry(11, "seedance-2.0", "seedance",
		map[string]float64{"480p": 0.0649, "720p": 0.1461, "1080p": 0.3645, "4k": 0.7433},
		map[int64]float64{videoSeedanceKeyID: 0.03, videoRelayAID: 0.06})
	h.entries["kling-3.0"] = officialVideoEntry(12, "kling-3.0", "kling",
		map[string]float64{"720p": 0.084, "1080p": 0.112, "4k": 0.42}, map[int64]float64{videoKlingKeyID: 0.05})
	h.entries["MiniMax-H3"] = officialVideoEntry(13, "MiniMax-H3", "minimax",
		map[string]float64{"768p": 0.08, "2k": 0.13}, map[int64]float64{videoMiniMaxKeyID: 0.06})
	return h
}

// OpenAI Videos 形态创建 seedance-2.0 → Seedance 官方 key（上游模型名按承接映射）→ 轮询到成功 → 按 1080p 档 × 秒计费，
// 渠道成本按承接的上游价（基础档价按官方同比例折到 1080p）；成片下载不带 key。
func TestVideoTaskSeedanceOfficialEndToEnd(t *testing.T) {
	h := newOfficialVideoHarness(t)
	status := `{"id":"cgt-1","status":"running"}`
	h.mux.HandleFunc("POST ark.cn-beijing.volces.com/api/v3/contents/generations/tasks", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONBody(w, 200, `{"id":"cgt-1"}`)
	})
	h.mux.HandleFunc("GET ark.cn-beijing.volces.com/api/v3/contents/generations/tasks/cgt-1", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONBody(w, 200, status)
	})
	h.mux.HandleFunc("GET ark-content.example.com/cgt-1.mp4", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = io.WriteString(w, "SEEDANCEMP4")
	})

	rec := h.do(t, http.MethodPost, "/v1/videos", `{"model":"seedance-2.0","prompt":"waves","seconds":"8","size":"1920x1080"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	id := gjson.Get(rec.Body.String(), "id").String()
	require.True(t, service.IsVideoTaskID(id))
	create := h.upstream.recorded()[0]
	require.Equal(t, "https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks", create.URL)
	require.Equal(t, "Bearer sk-seedance", create.Auth)
	require.Equal(t, videoSeedanceKeyID, create.AccountID)
	require.JSONEq(t, `{"model":"doubao-seedance-2-0-260128","content":[{"type":"text","text":"waves"}],"resolution":"1080p","ratio":"16:9","duration":8}`, string(create.Body))
	task := h.repo.only(t)
	require.Equal(t, service.VideoUpstreamSeedanceOfficial, task.UpstreamKind)
	require.Equal(t, "doubao-seedance-2-0-260128", task.UpstreamModel)

	rec = h.do(t, http.MethodGet, "/v1/videos/"+id, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "in_progress", gjson.Get(rec.Body.String(), "status").String())
	require.Empty(t, h.usageLogs.recorded())

	status = `{"id":"cgt-1","status":"succeeded","content":{"video_url":"https://ark-content.example.com/cgt-1.mp4"},"duration":8,"resolution":"1080p","framespersecond":24}`
	rec = h.do(t, http.MethodGet, "/v1/videos/"+id, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "completed", gjson.Get(rec.Body.String(), "status").String())
	requireVideoUsage(t, h.usageLogs, id, videoSeedanceKeyID, service.VideoBillingResolution1080P, 8, 0.3645, 0.03*0.3645/0.0649,
		"/v1/videos", "/contents/generations/tasks")

	rec = h.do(t, http.MethodGet, "/v1/videos/"+id+"/content", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "SEEDANCEMP4", rec.Body.String())
	calls := h.upstream.recorded()
	require.Equal(t, "https://ark-content.example.com/cgt-1.mp4", calls[len(calls)-1].URL)
	require.Empty(t, calls[len(calls)-1].Auth, "签名地址下载不带 key")
	require.Len(t, h.usageLogs.recorded(), 1)
	for _, call := range calls {
		require.NotContains(t, call.URL, "relay-", "中转没被调用")
	}
}

// xAI 形态创建 kling-3.0 4k → 可灵官方 key（文生视频端点）→ 成功后按 4k 档 × 上游报的秒数计费。
func TestVideoTaskKlingOfficialEndToEnd(t *testing.T) {
	h := newOfficialVideoHarness(t)
	h.mux.HandleFunc("POST api-singapore.klingai.com/text-to-video/kling-3.0", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONBody(w, 200, `{"code":0,"request_id":"req-1","data":{"id":"kt-9","status":"submitted"}}`)
	})
	h.mux.HandleFunc("GET api-singapore.klingai.com/tasks", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "kt-9", r.URL.Query().Get("task_ids"))
		writeJSONBody(w, 200, `{"code":0,"data":[{"id":"kt-9","status":"succeeded","outputs":[{"type":"video","url":"https://kling-cdn.example.com/kt-9.mp4","duration":"10"}]}]}`)
	})
	h.mux.HandleFunc("GET kling-cdn.example.com/kt-9.mp4", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "KLINGMP4") })

	rec := h.do(t, http.MethodPost, "/v1/videos/generations", `{"model":"kling-3.0","prompt":"a train","duration":10,"resolution":"4k","aspect_ratio":"16:9"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	id := gjson.Get(rec.Body.String(), "request_id").String()
	create := h.upstream.recorded()[0]
	require.Equal(t, "https://api-singapore.klingai.com/text-to-video/kling-3.0", create.URL)
	require.Equal(t, "Bearer sk-kling", create.Auth)
	require.JSONEq(t, `{"prompt":"a train","settings":{"resolution":"4k","aspect_ratio":"16:9","duration":10},"options":{"watermark_info":{"enabled":false}}}`, string(create.Body))
	require.Equal(t, "kt-9", h.repo.only(t).UpstreamTaskID)

	rec = h.do(t, http.MethodGet, "/v1/videos/generations/"+id, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "done", gjson.Get(rec.Body.String(), "status").String())
	requireVideoUsage(t, h.usageLogs, id, videoKlingKeyID, service.VideoBillingResolution4K, 10, 0.42, 0.05*0.42/0.084,
		"/v1/videos/generations", "/{text|image}-to-video")

	rec = h.do(t, http.MethodGet, "/v1/videos/"+id+"/content", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "KLINGMP4", rec.Body.String())
}

// new-api 形态创建 MiniMax-H3（metadata.resolution 2K，没给宽高比）→ MiniMax 官方 key → 成功后按上游报的 2K 档 × 6 秒计费。
func TestVideoTaskMiniMaxOfficialEndToEnd(t *testing.T) {
	h := newOfficialVideoHarness(t)
	h.mux.HandleFunc("POST api.minimax.cn/v2/video_generation", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONBody(w, 200, `{"task_id":"mm-1"}`)
	})
	h.mux.HandleFunc("GET api.minimax.cn/v2/query/video_generation/mm-1", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONBody(w, 200, `{"task":{"id":"mm-1","status":"succeeded","content":{"url":"https://hailuo-cdn.example.com/mm-1.mp4"},"resolution":"2K","duration":6}}`)
	})

	rec := h.do(t, http.MethodPost, "/v1/video/generations", `{"model":"MiniMax-H3","prompt":"space opera","duration":6,"metadata":{"resolution":"2K"}}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	id := gjson.Get(rec.Body.String(), "task_id").String()
	create := h.upstream.recorded()[0]
	require.Equal(t, "https://api.minimax.cn/v2/video_generation", create.URL)
	require.Equal(t, "Bearer sk-minimax", create.Auth)
	require.JSONEq(t, `{"model":"MiniMax-H3","content":[{"type":"text","text":"space opera"}],"resolution":"2K","duration":6,"ratio":"16:9"}`, string(create.Body))
	require.Equal(t, "16:9", h.repo.only(t).Request.AspectRatio, "落库的是适配器补齐后的请求（文生视频补 16:9）")

	rec = h.do(t, http.MethodGet, "/v1/video/generations/"+id, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "completed", gjson.Get(rec.Body.String(), "status").String())
	requireVideoUsage(t, h.usageLogs, id, videoMiniMaxKeyID, service.VideoBillingResolution2K, 6, 0.13, 0.06*0.13/0.08,
		"/v1/video/generations", "/v2/video_generation")
}

// 条目没有请求的分辨率档：400，不发上游、不建任务（不拿别的档顶着计费）；超出型号范围的时长同样 400，不收敛。
func TestVideoTaskOfficialRejectsMissingTierAndOutOfRange(t *testing.T) {
	h := newOfficialVideoHarness(t)
	entry := h.entries["seedance-2.0"]
	entry.Intervals = entry.Intervals[:3] // 去掉 4k 档

	rec := h.do(t, http.MethodPost, "/v1/videos/generations", `{"model":"seedance-2.0","prompt":"x","resolution":"4k","duration":5}`)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Contains(t, gjson.Get(rec.Body.String(), "error.message").String(), "resolution 4k is not available for model seedance-2.0")

	rec = h.do(t, http.MethodPost, "/v1/videos/generations", `{"model":"kling-3.0","prompt":"x","duration":20}`)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Contains(t, gjson.Get(rec.Body.String(), "error.message").String(), "between 3 and 15 seconds")

	require.Empty(t, h.upstream.recorded())
	require.Empty(t, h.repo.tasks)
}
