package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// monitorV2FixtureRepo 按真实仓储的形状返回带请求量、Token 量、RPM、缓存率、阈值的数据，
// 看 DTO 白名单有没有把它们挡住。
type monitorV2FixtureRepo struct{}

func monitorV2FixtureMetric() service.ChannelMonitorV2Metric {
	p50, p90, p95 := int64(800), int64(1500), int64(2100)
	avg := 900.0
	return service.ChannelMonitorV2Metric{
		SuccessRequests: 95, ErrorRequests: 5, RequestCount: 100,
		InputTokens: 1000, OutputTokens: 500, CacheCreationTokens: 10, CacheReadTokens: 300, TokenCount: 1810,
		RPM: 12.5, TPM: 3000, ErrorRate: 0.03, SuccessRate: 0.95, CacheRate: 0.9,
		CacheRateNumerator: 1179, CacheRateDenominator: 1310,
		TTFT:     service.ChannelMonitorV2Latency{SampleCount: 100, P50Ms: &p50, P90Ms: &p90, P95Ms: &p95, AvgMs: &avg},
		Duration: service.ChannelMonitorV2Latency{SampleCount: 100, P50Ms: &p90, AvgMs: &avg},
	}
}

func monitorV2FixtureCoverage() service.ChannelMonitorV2Coverage {
	end := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	return service.ChannelMonitorV2Coverage{
		RequestedStart: end.Add(-90 * time.Minute), RequestedEnd: end,
		CoverageStart: end.Add(-90 * time.Minute), DataThrough: end.Add(-time.Minute),
		BucketSeconds: 300,
	}
}

func monitorV2FixturePoint() service.ChannelMonitorV2TrendPoint {
	m := monitorV2FixtureMetric()
	return service.ChannelMonitorV2TrendPoint{
		BucketStart: time.Date(2026, 9, 26, 11, 0, 0, 0, time.UTC),
		Metrics:     m,
		Health:      service.ChannelMonitorV2HealthFor(m),
	}
}

func (monitorV2FixtureRepo) GetSnapshot(context.Context, service.ChannelMonitorV2Filter, service.ChannelMonitorV2Config) (*service.ChannelMonitorV2Snapshot, error) {
	m := monitorV2FixtureMetric()
	return &service.ChannelMonitorV2Snapshot{
		Coverage: monitorV2FixtureCoverage(), Metrics: m,
		Health: service.ChannelMonitorV2HealthFor(m),
		Trend:  []service.ChannelMonitorV2TrendPoint{monitorV2FixturePoint()},
	}, nil
}

func (monitorV2FixtureRepo) GetMatrix(context.Context, service.ChannelMonitorV2Filter, service.ChannelMonitorV2Config) (*service.ChannelMonitorV2Matrix, error) {
	m := monitorV2FixtureMetric()
	return &service.ChannelMonitorV2Matrix{
		Coverage: monitorV2FixtureCoverage(),
		Items: []service.ChannelMonitorV2MatrixRow{
			{Model: "gpt-5", Metrics: m, Health: service.ChannelMonitorV2HealthFor(m), Buckets: []service.ChannelMonitorV2TrendPoint{monitorV2FixturePoint()}},
		},
	}, nil
}

func (monitorV2FixtureRepo) GetChannels(context.Context, service.ChannelMonitorV2Filter, service.ChannelMonitorV2Config, string) (*service.ChannelMonitorV2Channels, error) {
	m := monitorV2FixtureMetric()
	return &service.ChannelMonitorV2Channels{
		ChannelMonitorV2Snapshot: service.ChannelMonitorV2Snapshot{Coverage: monitorV2FixtureCoverage(), Metrics: m, Health: service.ChannelMonitorV2HealthFor(m)},
		Items: []service.ChannelMonitorV2ChannelRow{
			{AccountID: 3, Name: "fenno · Chat", Platform: "openai", Type: "apikey", Status: "active", Metrics: m, Health: service.ChannelMonitorV2HealthFor(m), Buckets: []service.ChannelMonitorV2TrendPoint{monitorV2FixturePoint()}},
			{AccountID: 0, Metrics: m, Health: service.ChannelMonitorV2HealthFor(m)},
		},
	}, nil
}

func (monitorV2FixtureRepo) GetAggregationWatermark(context.Context) (*service.ChannelMonitorV2AggregationWatermark, error) {
	return &service.ChannelMonitorV2AggregationWatermark{}, nil
}

func (monitorV2FixtureRepo) RecomputeRange(context.Context, time.Time, time.Time) error { return nil }

func newMonitorV2FixtureHandler() *ChannelMonitorV2Handler {
	return NewChannelMonitorV2Handler(service.NewChannelMonitorV2Service(monitorV2FixtureRepo{}, listedCatalogStub{ids: []string{"gpt-5"}}))
}

// serveMonitorV2 以未登录访客调一个 handler，返回状态码与响应体。
func serveMonitorV2(t *testing.T, handle gin.HandlerFunc, target string) (int, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, target, nil)
	handle(c)
	return recorder.Code, recorder.Body.String()
}

// 服务状态接口里不能出现的键：平台 / 渠道、上游状态码与错误原文、
// 请求量与 Token 量、RPM / TPM、缓存率、健康分阈值、用户的身份与名次。
var forbiddenServiceStatusKeys = []string{
	"platform", "platforms", "group_by", "channel_id", "channel_name", "account_id",
	"status_code", "upstream_status_code", "upstream_affected_requests", "upstream_attempt_count",
	"details", "error_type",
	"rpm", "tpm", "request_count", "success_requests", "error_requests", "token_count",
	"input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens",
	"cache_rate_numerator", "cache_rate_denominator", "cache_score",
	"sample_count", "count", "config", "thresholds", "score",
	"user_id", "email", "username", "display_label", "rank", "is_self", "can_drilldown",
}

func collectJSONKeys(value any, path string, out map[string]string) {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			out[key] = path + "." + key
			collectJSONKeys(child, path+"."+key, out)
		}
	case []any:
		for i, child := range v {
			collectJSONKeys(child, fmt.Sprintf("%s[%d]", path, i), out)
		}
	}
}

func jsonObject(t *testing.T, value any) map[string]any {
	t.Helper()
	object, ok := value.(map[string]any)
	require.True(t, ok, "want JSON object, got %T", value)
	return object
}

func jsonArray(t *testing.T, value any) []any {
	t.Helper()
	array, ok := value.([]any)
	require.True(t, ok, "want JSON array, got %T", value)
	return array
}

func decodeMonitorV2Data(t *testing.T, body string) any {
	t.Helper()
	var envelope struct {
		Code int `json:"code"`
		Data any `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &envelope), body)
	require.Equal(t, 0, envelope.Code, body)
	require.NotNil(t, envelope.Data, body)
	return envelope.Data
}

func TestServiceStatusPayloadsCarryOnlyWhitelistedFields(t *testing.T) {
	h := newMonitorV2FixtureHandler()
	cases := []struct {
		name   string
		handle gin.HandlerFunc
		// check 断言白名单字段确实在（防止「什么都没返回」也算通过）。
		check func(t *testing.T, data map[string]any)
	}{
		{"snapshot", h.Snapshot, func(t *testing.T, data map[string]any) {
			require.EqualValues(t, service.ChannelMonitorV2RefreshIntervalSeconds, data["refresh_interval_seconds"])
			metrics := jsonObject(t, data["metrics"])
			require.InDelta(t, 0.97, metrics["availability"], 1e-9)
			require.EqualValues(t, 800, metrics["ttft_p50_ms"])
			require.InDelta(t, 0.9, metrics["cache_hit_rate"], 1e-9)
			require.Equal(t, "healthy", jsonObject(t, data["health"])["cache"])
			require.Equal(t, "healthy", jsonObject(t, data["health"])["overall"])
			require.Len(t, data["trend"], 1)
		}},
		{"matrix", h.Matrix, func(t *testing.T, data map[string]any) {
			items := jsonArray(t, data["items"])
			require.Len(t, items, 1)
			row := jsonObject(t, items[0])
			require.Equal(t, "gpt-5", row["model"])
			require.Len(t, row["buckets"], 1)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, body := serveMonitorV2(t, tc.handle, "/channel-monitor-v2/"+tc.name+"?range=90m")
			require.Equal(t, http.StatusOK, code, body)
			data := decodeMonitorV2Data(t, body)

			keys := map[string]string{}
			collectJSONKeys(data, "data", keys)
			for _, key := range forbiddenServiceStatusKeys {
				_, found := keys[key]
				require.False(t, found, "%s payload has forbidden key %q at %s", tc.name, key, keys[key])
			}
			tc.check(t, jsonObject(t, data))
		})
	}
}

func TestServiceStatusRejectsUnknownRange(t *testing.T) {
	h := newMonitorV2FixtureHandler()
	for _, handle := range []gin.HandlerFunc{h.Snapshot, h.Matrix} {
		code, body := serveMonitorV2(t, handle, "/channel-monitor-v2/snapshot?range=15d")
		require.Equal(t, http.StatusBadRequest, code, body)
	}
}

// 管理站渠道状态：管理员能看请求数与渠道身份；上架模型给筛选；没上架的模型名直接 400。
func TestAdminChannelStatusPayload(t *testing.T) {
	h := newMonitorV2FixtureHandler()
	code, body := serveMonitorV2(t, h.AdminChannels, "/admin/channel-status?range=24h")
	require.Equal(t, http.StatusOK, code, body)
	data := jsonObject(t, decodeMonitorV2Data(t, body))
	require.EqualValues(t, 100, jsonObject(t, data["metrics"])["request_count"])
	require.Equal(t, []any{"gpt-5"}, data["models"])
	items := jsonArray(t, data["items"])
	require.Len(t, items, 2)
	channel := jsonObject(t, items[0])
	require.EqualValues(t, 3, channel["account_id"])
	require.Equal(t, "fenno · Chat", channel["name"])
	require.Equal(t, "openai", channel["platform"])
	require.InDelta(t, 0.9, jsonObject(t, channel["metrics"])["cache_hit_rate"], 1e-9)
	require.Len(t, channel["buckets"], 1)

	code, body = serveMonitorV2(t, h.AdminChannels, "/admin/channel-status?range=24h&model=not-listed")
	require.Equal(t, http.StatusBadRequest, code, body)
}
