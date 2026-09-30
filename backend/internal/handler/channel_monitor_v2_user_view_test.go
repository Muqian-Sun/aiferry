package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// monitorV2FixtureRepo 按真实仓储的形状返回带平台、上游状态码、其他用户身份的数据。
type monitorV2FixtureRepo struct{}

func (monitorV2FixtureRepo) GetConfig(context.Context) (*service.ChannelMonitorV2Config, error) {
	return &service.ChannelMonitorV2Config{
		Version: 3, Enabled: true, RefreshIntervalSeconds: 60,
		Platforms: []service.ChannelMonitorV2PlatformConfig{
			{Platform: "fixtureplatformalpha", Enabled: true, Models: []string{"gpt-5"}},
			{Platform: "fixtureplatformbeta", Enabled: true},
		},
		HealthThresholds:       service.DefaultChannelMonitorV2HealthThresholds(),
		IgnoredErrorCategories: []string{"content_policy"},
	}, nil
}

func (monitorV2FixtureRepo) UpdateConfig(context.Context, service.ChannelMonitorV2Config, int) (*service.ChannelMonitorV2Config, error) {
	return nil, nil
}

func monitorV2FixtureMetric() service.ChannelMonitorV2Metric {
	p50, p90, p95 := int64(800), int64(1500), int64(2100)
	avg := 900.0
	affected, attempts := int64(4), int64(9)
	return service.ChannelMonitorV2Metric{
		SuccessRequests: 95, ErrorRequests: 5, RequestCount: 100,
		InputTokens: 1000, OutputTokens: 500, CacheCreationTokens: 10, CacheReadTokens: 300, TokenCount: 1810,
		RPM: 12.5, TPM: 3000, ErrorRate: 0.03, SuccessRate: 0.95, CacheRate: 0.23,
		CacheRateNumerator: 300, CacheRateDenominator: 1310,
		TTFT:                     service.ChannelMonitorV2Latency{SampleCount: 100, P50Ms: &p50, P90Ms: &p90, P95Ms: &p95, AvgMs: &avg},
		Duration:                 service.ChannelMonitorV2Latency{SampleCount: 100, P50Ms: &p90, AvgMs: &avg},
		UpstreamAffectedRequests: &affected,
		UpstreamAttemptCount:     &attempts,
	}
}

func monitorV2FixtureCoverage() service.ChannelMonitorV2Coverage {
	end := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	return service.ChannelMonitorV2Coverage{
		RequestedStart: end.Add(-90 * time.Minute), RequestedEnd: end,
		CoverageStart: end.Add(-90 * time.Minute), DataThrough: end.Add(-time.Minute), ComputedAt: end,
		CoverageComplete: true, BucketSeconds: 300,
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

func (monitorV2FixtureRepo) GetDimensions(context.Context, service.ChannelMonitorV2Filter, service.ChannelMonitorV2Config) (*service.ChannelMonitorV2Dimensions, error) {
	// 真实仓储的模型维度值是「平台 \x00 模型」，同名模型在两个平台上各一条。
	return &service.ChannelMonitorV2Dimensions{
		Platforms: []service.ChannelMonitorV2Dimension{
			{Value: "fixtureplatformalpha", Label: "fixtureplatformalpha", RequestCount: 70},
			{Value: "fixtureplatformbeta", Label: "fixtureplatformbeta", RequestCount: 30},
		},
		Models: []service.ChannelMonitorV2Dimension{
			{Value: "fixtureplatformalpha\x00gpt-5", Label: "fixtureplatformalpha\x00gpt-5", Platform: "fixtureplatformalpha", RequestCount: 60},
			{Value: "fixtureplatformbeta\x00gpt-5", Label: "fixtureplatformbeta\x00gpt-5", Platform: "fixtureplatformbeta", RequestCount: 30},
			{Value: "fixtureplatformalpha\x00__other__", Label: "fixtureplatformalpha\x00__other__", Platform: "fixtureplatformalpha", RequestCount: 10},
		},
	}, nil
}

func (r monitorV2FixtureRepo) GetSnapshot(ctx context.Context, _ service.ChannelMonitorV2Filter, _ service.ChannelMonitorV2Config, _ bool) (*service.ChannelMonitorV2Snapshot, error) {
	cfg, _ := r.GetConfig(ctx)
	m := monitorV2FixtureMetric()
	return &service.ChannelMonitorV2Snapshot{
		Config: *cfg, Coverage: monitorV2FixtureCoverage(), Metrics: m,
		Health: service.ChannelMonitorV2HealthFor(m),
		Trend:  []service.ChannelMonitorV2TrendPoint{monitorV2FixturePoint()},
	}, nil
}

func (monitorV2FixtureRepo) GetModels(context.Context, service.ChannelMonitorV2Filter, service.ChannelMonitorV2Config, bool) (*service.ChannelMonitorV2List[service.ChannelMonitorV2ModelRow], error) {
	m := monitorV2FixtureMetric()
	return &service.ChannelMonitorV2List[service.ChannelMonitorV2ModelRow]{
		Coverage: monitorV2FixtureCoverage(),
		Items: []service.ChannelMonitorV2ModelRow{
			{Platform: "fixtureplatformalpha", Model: "gpt-5", Metrics: m, Health: service.ChannelMonitorV2HealthFor(m)},
		},
	}, nil
}

func (monitorV2FixtureRepo) GetMatrix(_ context.Context, _ service.ChannelMonitorV2Filter, _ service.ChannelMonitorV2Config, groupBy service.ChannelMonitorV2GroupBy, _ bool) (*service.ChannelMonitorV2Matrix, error) {
	m := monitorV2FixtureMetric()
	return &service.ChannelMonitorV2Matrix{
		GroupBy: groupBy, Coverage: monitorV2FixtureCoverage(),
		Items: []service.ChannelMonitorV2MatrixRow{
			{Platform: "fixtureplatformalpha", Model: "gpt-5", Metrics: m, Health: service.ChannelMonitorV2HealthFor(m), Buckets: []service.ChannelMonitorV2TrendPoint{monitorV2FixturePoint()}},
		},
	}, nil
}

func (monitorV2FixtureRepo) GetErrors(context.Context, service.ChannelMonitorV2Filter, service.ChannelMonitorV2Config, bool) (*service.ChannelMonitorV2List[service.ChannelMonitorV2ErrorRow], error) {
	return &service.ChannelMonitorV2List[service.ChannelMonitorV2ErrorRow]{
		Coverage: monitorV2FixtureCoverage(),
		Items: []service.ChannelMonitorV2ErrorRow{{
			Category: "rate_or_capacity", Count: 5, Rate: 0.05,
			Details: []service.ChannelMonitorV2ErrorDetail{{
				Platform: "fixtureplatformalpha", Model: "gpt-5", ErrorType: "rate_limit",
				StatusCode: 429, UpstreamStatusCode: 529, Message: "fixture upstream overloaded", Count: 5,
			}},
		}},
	}, nil
}

func (monitorV2FixtureRepo) GetUsers(context.Context, service.ChannelMonitorV2Filter, service.ChannelMonitorV2Config, bool) (*service.ChannelMonitorV2List[service.ChannelMonitorV2UserRow], error) {
	otherID, selfID := int64(9), int64(7)
	m := monitorV2FixtureMetric()
	return &service.ChannelMonitorV2List[service.ChannelMonitorV2UserRow]{
		Coverage: monitorV2FixtureCoverage(),
		Items: []service.ChannelMonitorV2UserRow{
			{UserID: &otherID, Email: "fixture-other@example.com", Username: "fixtureotheruser", DisplayLabel: "fixtureotheruser", Metrics: m},
			{UserID: &selfID, Email: "fixture-self@example.com", Username: "fixtureselfuser", DisplayLabel: "fixtureselfuser", Metrics: m},
		},
	}, nil
}

func (monitorV2FixtureRepo) GetAggregationWatermark(context.Context) (*service.ChannelMonitorV2AggregationWatermark, error) {
	return &service.ChannelMonitorV2AggregationWatermark{}, nil
}

func (monitorV2FixtureRepo) RecomputeRange(context.Context, time.Time, time.Time) error { return nil }

// monitorV2RuntimeFixture 模拟站长没开「隐藏吞吐 / 隐藏排行」——用户侧最容易漏数据的一种配置。
type monitorV2RuntimeFixture struct{}

func (monitorV2RuntimeFixture) GetChannelMonitorRuntime(context.Context) service.ChannelMonitorRuntime {
	return service.ChannelMonitorRuntime{Enabled: true, Mode: service.ChannelMonitorModeV2}
}

func newMonitorV2FixtureHandler() *ChannelMonitorV2Handler {
	svc := service.NewChannelMonitorV2Service(monitorV2FixtureRepo{}, listedCatalogStub{ids: []string{"gpt-5"}})
	svc.SetRuntimeReader(monitorV2RuntimeFixture{})
	return NewChannelMonitorV2Handler(svc)
}

// serveMonitorV2 以普通用户（userID 7）或管理员身份调一个 handler，返回状态码与响应体。
func serveMonitorV2(t *testing.T, handle gin.HandlerFunc, target string, admin bool) (int, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, target, nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
	role := service.RoleUser
	if admin {
		role = service.RoleAdmin
	}
	c.Set(string(middleware.ContextKeyUserRole), role)
	handle(c)
	return recorder.Code, recorder.Body.String()
}

// 普通用户的服务状态接口里不能出现的键：平台 / 渠道、上游状态码与错误原文、
// 请求量与 Token 量、RPM / TPM、缓存率、健康分阈值、其他用户的身份与名次。
var forbiddenServiceStatusKeys = []string{
	"platform", "platforms", "group_by", "channel_id", "channel_name", "account_id",
	"status_code", "upstream_status_code", "upstream_affected_requests", "upstream_attempt_count",
	"details", "error_type",
	"rpm", "tpm", "request_count", "success_requests", "error_requests", "token_count",
	"input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens",
	"cache_rate", "cache_rate_numerator", "cache_rate_denominator", "cache", "cache_score",
	"sample_count", "count", "config", "thresholds", "score",
	"user_id", "email", "username", "display_label", "rank", "is_self", "can_drilldown",
}

// 夹具里写进去的平台名、其他用户身份、上游错误原文：响应体里一个字都不能有。
var forbiddenServiceStatusValues = []string{
	"fixtureplatform", "fixture-other@example.com", "fixtureotheruser", "Other user",
	"fixture-self@example.com", "fixtureselfuser", "fixture upstream overloaded",
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

func TestServiceStatusUserPayloadsCarryNoChannelOrOtherUserData(t *testing.T) {
	h := newMonitorV2FixtureHandler()
	cases := []struct {
		name   string
		handle gin.HandlerFunc
		// check 断言白名单字段确实在（防止「什么都没返回」也算通过）。
		check func(t *testing.T, data map[string]any)
	}{
		{"dimensions", h.Dimensions, func(t *testing.T, data map[string]any) {
			require.Equal(t, []any{"gpt-5", "__other__"}, data["models"])
		}},
		{"snapshot", h.Snapshot, func(t *testing.T, data map[string]any) {
			require.EqualValues(t, 60, data["refresh_interval_seconds"])
			metrics := jsonObject(t, data["metrics"])
			require.InDelta(t, 0.97, metrics["availability"], 1e-9)
			require.EqualValues(t, 800, metrics["ttft_p50_ms"])
			require.Equal(t, "healthy", jsonObject(t, data["health"])["overall"])
			require.Len(t, data["trend"], 1)
		}},
		{"models", h.Models, func(t *testing.T, data map[string]any) {
			items := jsonArray(t, data["items"])
			require.Len(t, items, 1)
			require.Equal(t, "gpt-5", jsonObject(t, items[0])["model"])
		}},
		{"matrix", h.Matrix, func(t *testing.T, data map[string]any) {
			items := jsonArray(t, data["items"])
			require.Len(t, items, 1)
			row := jsonObject(t, items[0])
			require.Equal(t, "gpt-5", row["model"])
			require.Len(t, row["buckets"], 1)
		}},
		{"errors", h.Errors, func(t *testing.T, data map[string]any) {
			items := jsonArray(t, data["items"])
			require.Len(t, items, 1)
			require.Equal(t, "rate_or_capacity", jsonObject(t, items[0])["category"])
		}},
		{"users", h.Users, func(t *testing.T, data map[string]any) {
			// 排行里只剩当前用户自己那一行
			items := jsonArray(t, data["items"])
			require.Len(t, items, 1)
			require.InDelta(t, 0.97, jsonObject(t, jsonObject(t, items[0])["metrics"])["availability"], 1e-9)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, body := serveMonitorV2(t, tc.handle, "/channel-monitor-v2/"+tc.name+"?platform=fixtureplatformalpha", false)
			require.Equal(t, http.StatusOK, code, body)
			data := decodeMonitorV2Data(t, body)

			keys := map[string]string{}
			collectJSONKeys(data, "data", keys)
			for _, key := range forbiddenServiceStatusKeys {
				_, found := keys[key]
				require.False(t, found, "user %s payload has forbidden key %q at %s", tc.name, key, keys[key])
			}
			for _, value := range forbiddenServiceStatusValues {
				require.NotContains(t, body, value, "user %s payload leaks %q", tc.name, value)
			}
			tc.check(t, jsonObject(t, data))
		})
	}
}

// 管理员路由不走白名单：平台、RPM、上游状态码照旧返回。
func TestServiceStatusWhitelistLeavesAdminPayloadsUntouched(t *testing.T) {
	h := newMonitorV2FixtureHandler()
	for name, tc := range map[string]struct {
		handle gin.HandlerFunc
		want   []string
	}{
		"dimensions": {h.Dimensions, []string{`"platforms"`, `fixtureplatformbeta`}},
		"snapshot":   {h.AdminSnapshot, []string{`"rpm":12.5`, `"cache_rate"`, `"platforms"`}},
		"models":     {h.AdminModels, []string{`"platform":"fixtureplatformalpha"`, `"request_count":100`}},
		"matrix":     {h.AdminMatrix, []string{`"platform":"fixtureplatformalpha"`, `"group_by"`}},
		"errors":     {h.Errors, []string{`"upstream_status_code":529`, `"count":5`}},
		"users":      {h.AdminUsers, []string{`fixture-other@example.com`, `"rank":1`}},
	} {
		code, body := serveMonitorV2(t, tc.handle, "/admin/channel-monitor-v2/"+name, true)
		require.Equal(t, http.StatusOK, code, body)
		for _, want := range tc.want {
			require.Contains(t, body, want, "admin %s payload", name)
		}
	}
}
