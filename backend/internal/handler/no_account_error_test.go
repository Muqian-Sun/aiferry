//go:build unit

package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type fakeDiagnoser struct {
	calls []fakeDiagnoseCall
	resp  service.ModelAvailabilityDiagnosis
}

type fakeDiagnoseCall struct {
	Model    string
	Platform string
}

func (f *fakeDiagnoser) DiagnoseModelAvailabilityForPlatform(
	_ context.Context,
	model, platform string,
) service.ModelAvailabilityDiagnosis {
	f.calls = append(f.calls, fakeDiagnoseCall{
		Model:    model,
		Platform: platform,
	})
	return f.resp
}

func ptrInt64(v int64) *int64 { return &v }

// newTestGinContextWithRequest wraps the bare newTestGinContext helper
// (defined in openai_gateway_cyber_test.go) by additionally attaching a stub
// *http.Request so the classifier can extract c.Request.Context().
func newTestGinContextWithRequest() *gin.Context {
	c := newTestGinContext()
	c.Request = httptest.NewRequest(http.MethodPost, "/test", nil)
	return c
}

func TestClassifyNoAccountError_NilDiagnoser_Falls503(t *testing.T) {
	c := newTestGinContextWithRequest()

	cls := classifyNoAccountErrorFromGin(c, nil, "gpt-5", "gpt-5", service.PlatformOpenAI)

	require.Equal(t, http.StatusServiceUnavailable, cls.Status)
	require.Equal(t, "api_error", cls.ErrType)
	require.False(t, cls.ModelNotFound)
}

func TestClassifySelectionFailureError_RateLimitedPool(t *testing.T) {
	fallback := noAccountErrorClassification{Status: http.StatusServiceUnavailable, ErrType: "api_error", Message: "Service temporarily unavailable"}

	got := classifySelectionFailureError(
		fmt.Errorf("no available accounts supporting model: gpt-5.6-sol (total=3 eligible=0 model_rate_limited=3)"),
		fallback,
	)

	require.Equal(t, http.StatusTooManyRequests, got.Status)
	require.Equal(t, "rate_limit_error", got.ErrType)
	require.Contains(t, got.Message, "rate-limited")
	require.Equal(t, fallback, classifySelectionFailureError(fmt.Errorf("model_rate_limited=0"), fallback))
	require.Equal(t, fallback, classifySelectionFailureError(fmt.Errorf("no available accounts"), fallback))
}

// 分组没了：诊断只看模型与平台（目录路由下按条目绑定），无分组 key 同样能得到 404 model_not_found。
func TestClassifyNoAccountError_UngroupedKeyGets404(t *testing.T) {
	c := newTestGinContextWithRequest()
	fd := &fakeDiagnoser{resp: service.ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: false}}

	cls := classifyNoAccountErrorFromGin(c, fd, "gpt-5", "gpt-5", service.PlatformOpenAI)

	require.Equal(t, http.StatusNotFound, cls.Status)
	require.True(t, cls.ModelNotFound)
	require.Len(t, fd.calls, 1)
}

func TestClassifyNoAccountError_EmptyModel_Falls503(t *testing.T) {
	c := newTestGinContextWithRequest()
	fd := &fakeDiagnoser{resp: service.ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: false}}

	cls := classifyNoAccountErrorFromGin(c, fd, "   ", "", service.PlatformOpenAI)

	require.Equal(t, http.StatusServiceUnavailable, cls.Status)
	require.False(t, cls.ModelNotFound)
	require.Empty(t, fd.calls)
}

func TestClassifyNoAccountError_ModelNotSupported_Returns404(t *testing.T) {
	c := newTestGinContextWithRequest()
	fd := &fakeDiagnoser{resp: service.ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: false}}

	cls := classifyNoAccountErrorFromGin(c, fd, "gpt-5.1-codex-mini", "gpt-5.1-codex-mini", service.PlatformOpenAI)

	require.Equal(t, http.StatusNotFound, cls.Status)
	require.Equal(t, "model_not_found", cls.ErrType)
	require.True(t, cls.ModelNotFound)
	require.Contains(t, cls.Message, "gpt-5.1-codex-mini", "message must surface the requested model")

	require.Len(t, fd.calls, 1)
	require.Equal(t, "gpt-5.1-codex-mini", fd.calls[0].Model)
	require.Equal(t, service.PlatformOpenAI, fd.calls[0].Platform)
	require.True(t, service.HasOpsClientBusinessLimited(c))
	require.Equal(t, service.OpsClientBusinessLimitedReasonLocalModelConfiguration, service.OpsClientBusinessLimitedReason(c))
}

func TestClassifyOpenAICompatibleNoAccountError_GrokUsesGrokPlatform(t *testing.T) {
	c := newTestGinContextWithRequest()
	fd := &fakeDiagnoser{resp: service.ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: false}}
	c.Request = c.Request.WithContext(service.WithCatalogRoute(c.Request.Context(), service.CatalogRoute{
		EntryID: 1, CanonicalModel: "grok-4.5", RequestedModel: "grok-4.5",
		Entry: &service.ModelCatalogEntry{ID: 1, ModelID: "grok-4.5", Vendor: "xai", Status: service.ModelCatalogStatusListed},
	}))

	cls := classifyOpenAICompatibleNoAccountErrorFromGin(c, fd, "grok-4.5", "grok-4.5")

	require.Equal(t, http.StatusNotFound, cls.Status)
	require.Equal(t, "model_not_found", cls.ErrType)
	require.True(t, cls.ModelNotFound)
	require.Len(t, fd.calls, 1)
	require.Equal(t, service.PlatformGrok, fd.calls[0].Platform)
	require.True(t, service.HasOpsClientBusinessLimited(c))
	require.Equal(t, service.OpsClientBusinessLimitedReasonLocalModelConfiguration, service.OpsClientBusinessLimitedReason(c))

	logErr := openAICompatibleSelectionErrorForLog(
		fmt.Errorf("no available OpenAI accounts supporting model: grok-4.5"),
		service.PlatformGrok,
	)
	require.EqualError(t, logErr, "no available Grok accounts supporting model: grok-4.5")
}

func TestClassifyNoAccountError_PureClassifierDoesNotMarkGinContext(t *testing.T) {
	c := newTestGinContextWithRequest()
	fd := &fakeDiagnoser{resp: service.ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: false}}

	cls := classifyNoAccountError(c.Request.Context(), fd, "gpt-5", "gpt-5", service.PlatformOpenAI)

	require.True(t, cls.ModelNotFound)
	require.False(t, service.HasOpsClientBusinessLimited(c))
	require.Empty(t, service.OpsClientBusinessLimitedReason(c))
}

func TestClassifyNoAccountError_HasModelSupport_KeepsRoutingMessageGenerationToCaller(t *testing.T) {
	c := newTestGinContextWithRequest()
	fd := &fakeDiagnoser{resp: service.ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: true}}

	cls := classifyNoAccountErrorFromGin(c, fd, "gpt-5", "gpt-5", service.PlatformOpenAI)

	require.Equal(t, http.StatusServiceUnavailable, cls.Status, "model exists somewhere — caller stays on 503")
	require.Equal(t, "api_error", cls.ErrType)
	require.False(t, cls.ModelNotFound)
}

func TestClassifyNoAccountError_ModelSupportedOnlyByRateLimitedAccount_Returns503(t *testing.T) {
	c := newTestGinContextWithRequest()
	// The diagnoser's configured-state lookup still sees the model-supporting
	// account even though normal scheduling has excluded it during cooldown.
	fd := &fakeDiagnoser{resp: service.ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: true}}

	cls := classifyNoAccountErrorFromGin(c, fd, "claude-opus-4-8", "claude-opus-4-8", service.PlatformAnthropic)

	require.Equal(t, http.StatusServiceUnavailable, cls.Status)
	require.Equal(t, "api_error", cls.ErrType)
	require.False(t, cls.ModelNotFound, "temporary account cooldown must remain retryable")
}

func TestClassifyNoAccountError_NoAccountsInPool_Stays503(t *testing.T) {
	c := newTestGinContextWithRequest()
	fd := &fakeDiagnoser{resp: service.ModelAvailabilityDiagnosis{HasAccountsInPool: false, HasModelSupport: false}}

	cls := classifyNoAccountErrorFromGin(c, fd, "gpt-5", "gpt-5", service.PlatformOpenAI)

	require.Equal(t, http.StatusServiceUnavailable, cls.Status, "empty pool is a service-availability issue, not a model issue")
	require.False(t, cls.ModelNotFound)
}

func TestClassifyNoAccountError_DisplayModelOverridesRoutingForMessage(t *testing.T) {
	c := newTestGinContextWithRequest()
	fd := &fakeDiagnoser{resp: service.ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: false}}

	cls := classifyNoAccountErrorFromGin(c, fd, "gpt-5", "claude-3-fancy", service.PlatformOpenAI)

	require.True(t, cls.ModelNotFound)
	require.Contains(t, cls.Message, "claude-3-fancy", "user-facing message must reference the model the user asked for, not the post-mapping routing model")
	require.Len(t, fd.calls, 1)
	require.Equal(t, "gpt-5", fd.calls[0].Model, "diagnosis must run against the routing model (post group dispatch mapping)")
}

func TestClassifyNoAccountError_FromGin_NilContextStillSafe(t *testing.T) {
	fd := &fakeDiagnoser{resp: service.ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: false}}

	cls := classifyNoAccountErrorFromGin(nil, fd, "gpt-5", "gpt-5", service.PlatformOpenAI)

	require.Equal(t, http.StatusNotFound, cls.Status, "even with a nil gin context the classifier must still run and yield a coherent response")
	require.True(t, cls.ModelNotFound)
	require.False(t, service.HasOpsClientBusinessLimited(nil))
	require.Empty(t, service.OpsClientBusinessLimitedReason(nil))
}

// 权威的 404 model_not_found 不能被"账号被限流"的 429 盖掉。
//
// 选号失败的错误串同时携带多种过滤原因，例如
// "pool=9, filtered: model_not_supported=8 model_rate_limited=1"：8 个账号根本不支持该模型，
// 剩下 1 个恰好处于模型级冷却。此时 classifyNoAccountError 已通过持久化判据确认整个分组
// 没有账号能服务该模型（ModelNotFound=true），改判成 429 "All available accounts are
// currently rate-limited" 是错误诊断——重试永远不会成功，而把 429 当限流的客户端会反复
// 重试并吞掉 body（Codex 只显示 "exceeded retry limit"），恰好丢掉唯一说明真实原因的信息。
func TestClassifySelectionFailureError_ModelNotFoundIsNotOverriddenByRateLimited(t *testing.T) {
	modelNotFound := noAccountErrorClassification{
		Status:        http.StatusNotFound,
		ErrType:       "model_not_found",
		Message:       `Model "gpt-5.3-codex" is not supported by any configured account in this group`,
		ModelNotFound: true,
	}

	got := classifySelectionFailureError(
		fmt.Errorf("no available OpenAI accounts supporting model: gpt-5.3-codex "+
			"(pool=9, filtered: model_not_supported=8 model_rate_limited=1)"),
		modelNotFound,
	)

	require.Equal(t, modelNotFound, got,
		"分组里没有任何账号能服务该模型时，模型级冷却不该把 404 改判成 429")
}

// 真实调用点的顺序：先 classifyNoAccountErrorFromGin，再 classifySelectionFailureError。
// 覆盖这条链路是为了同时锁住 ops 归因——调用点用 ModelNotFound 决定是否标记
// routing capacity limited，一旦 404 被改判成 429，同一个请求会既被标成
// local model configuration 又被标成容量问题，自相矛盾。
func TestClassifySelectionFailureError_CallSiteChainKeepsModelNotFoundAttribution(t *testing.T) {
	c := newTestGinContextWithRequest()
	fd := &fakeDiagnoser{resp: service.ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: false}}

	cls := classifyNoAccountErrorFromGin(c, fd, "gpt-5.3-codex", "gpt-5.3-codex", service.PlatformOpenAI)
	cls = classifySelectionFailureError(
		fmt.Errorf("no available OpenAI accounts supporting model: gpt-5.3-codex "+
			"(pool=9, filtered: model_not_supported=8 model_rate_limited=1)"),
		cls,
	)

	require.Equal(t, http.StatusNotFound, cls.Status)
	require.Equal(t, "model_not_found", cls.ErrType)
	require.True(t, cls.ModelNotFound)
	require.Contains(t, cls.Message, "gpt-5.3-codex")
	require.True(t, service.HasOpsClientBusinessLimited(c))
	require.Equal(t, service.OpsClientBusinessLimitedReasonLocalModelConfiguration, service.OpsClientBusinessLimitedReason(c))
}

// 池子里确实存在能服务该模型、只是全部在冷却的账号时，429 改判仍需保留：
// 这种情况 fallback 是 503（HasModelSupport=true），重试是有意义的。
func TestClassifySelectionFailureError_StillUpgradesNonModelNotFoundFallback(t *testing.T) {
	fallback := noAccountErrorClassification{
		Status:  http.StatusServiceUnavailable,
		ErrType: "api_error",
		Message: "Service temporarily unavailable",
	}

	got := classifySelectionFailureError(
		fmt.Errorf("no available accounts supporting model: gpt-5.6-sol (total=3 eligible=0 model_rate_limited=3)"),
		fallback,
	)

	require.Equal(t, http.StatusTooManyRequests, got.Status)
	require.Equal(t, "rate_limit_error", got.ErrType)
	require.False(t, got.ModelNotFound)
}
