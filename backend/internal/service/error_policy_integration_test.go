//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Mocks (scoped to this file by naming convention)
// ---------------------------------------------------------------------------

// epFixedUpstream returns a fixed response for every request.
type epFixedUpstream struct {
	statusCode int
	body       string
	calls      int
}

func (u *epFixedUpstream) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	u.calls++
	return &http.Response{
		StatusCode: u.statusCode,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(u.body)),
	}, nil
}

func (u *epFixedUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

// epAccountRepo records SetTempUnschedulable / SetError calls.
type epAccountRepo struct {
	mockAccountRepoForGemini
	tempCalls   int
	setErrCalls int
}

func (r *epAccountRepo) SetTempUnschedulable(_ context.Context, _ int64, _ time.Time, _ string) error {
	r.tempCalls++
	return nil
}

func (r *epAccountRepo) SetError(_ context.Context, _ int64, _ string) error {
	r.setErrCalls++
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func saveAndSetBaseURLs(t *testing.T) {
	t.Helper()
	oldBaseURLs := append([]string(nil), antigravity.BaseURLs...)
	oldAvail := antigravity.DefaultURLAvailability
	antigravity.BaseURLs = []string{"https://ep-test.example"}
	antigravity.DefaultURLAvailability = antigravity.NewURLAvailability(time.Minute)
	t.Cleanup(func() {
		antigravity.BaseURLs = oldBaseURLs
		antigravity.DefaultURLAvailability = oldAvail
	})
}

func newRetryParams(account *Account, upstream HTTPUpstream, handleError func(context.Context, string, *Account, int, http.Header, []byte, string, int64, string, bool) *handleModelRateLimitResult) antigravityRetryLoopParams {
	return antigravityRetryLoopParams{
		ctx:            context.Background(),
		prefix:         "[ep-test]",
		account:        account,
		accessToken:    "token",
		action:         "generateContent",
		body:           []byte(`{"input":"test"}`),
		httpUpstream:   upstream,
		requestedModel: "claude-sonnet-4-5",
		handleError:    handleError,
	}
}

// ---------------------------------------------------------------------------
// TestRetryLoop_ErrorPolicy_NilRateLimitService
// ---------------------------------------------------------------------------

func TestRetryLoop_ErrorPolicy_NilRateLimitService(t *testing.T) {
	saveAndSetBaseURLs(t)

	upstream := &epFixedUpstream{statusCode: 429, body: `{"error":"rate limited"}`}
	// rateLimitService is nil — must not panic
	svc := &AntigravityGatewayService{rateLimitService: nil}

	account := &Account{
		ID:          300,
		Type:        AccountTypeOAuth,
		Platform:    PlatformAntigravity,
		Schedulable: true,
		Status:      StatusActive,
		Concurrency: 1,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	p := newRetryParams(account, upstream, func(_ context.Context, _ string, _ *Account, _ int, _ http.Header, _ []byte, _ string, _ int64, _ string, _ bool) *handleModelRateLimitResult {
		return nil
	})
	p.ctx = ctx

	// Should not panic; enters the default retry path (eventually times out)
	result, err := svc.antigravityRetryLoop(p)

	require.Nil(t, result)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.GreaterOrEqual(t, upstream.calls, 1)
}

// ---------------------------------------------------------------------------
// TestRetryLoop_ErrorPolicy_NoPolicy_OriginalBehavior
// ---------------------------------------------------------------------------

func TestRetryLoop_ErrorPolicy_NoPolicy_OriginalBehavior(t *testing.T) {
	saveAndSetBaseURLs(t)

	upstream := &epFixedUpstream{statusCode: 429, body: `{"error":"rate limited"}`}
	repo := &epAccountRepo{}
	rlSvc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc := &AntigravityGatewayService{rateLimitService: rlSvc}

	// Plain OAuth account with no error policy configured
	account := &Account{
		ID:          400,
		Type:        AccountTypeOAuth,
		Platform:    PlatformAntigravity,
		Schedulable: true,
		Status:      StatusActive,
		Concurrency: 1,
	}

	var handleErrorCount int
	p := newRetryParams(account, upstream, func(_ context.Context, _ string, _ *Account, _ int, _ http.Header, _ []byte, _ string, _ int64, _ string, _ bool) *handleModelRateLimitResult {
		handleErrorCount++
		return nil
	})

	result, err := svc.antigravityRetryLoop(p)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.resp)
	defer func() { _ = result.resp.Body.Close() }()

	require.Equal(t, http.StatusTooManyRequests, result.resp.StatusCode)
	require.Equal(t, antigravityMaxRetries, upstream.calls, "should exhaust all retries")
	require.Equal(t, 1, handleErrorCount, "handleError should be called once after retries exhausted")
}

// ---------------------------------------------------------------------------
// TestRetryLoop_LegacyCustomErrorCodesIgnored — 渠道级自定义错误码 2026-09-28 P5 已删：
// 库里旧行留着的 [599] 不再把上游 429 改写成 500、也不再跳过重试与账号处理，行为同无策略账号。
// ---------------------------------------------------------------------------

func TestRetryLoop_LegacyCustomErrorCodesIgnored(t *testing.T) {
	saveAndSetBaseURLs(t)

	upstream := &epFixedUpstream{statusCode: 429, body: `{"error":"rate limited"}`}
	repo := &epAccountRepo{}
	rlSvc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc := &AntigravityGatewayService{rateLimitService: rlSvc}

	account := &Account{
		ID:          500,
		Type:        AccountTypeAPIKey,
		Platform:    PlatformAntigravity,
		Schedulable: true,
		Status:      StatusActive,
		Concurrency: 1,
		Credentials: map[string]any{
			"custom_error_codes_enabled": true,
			"custom_error_codes":         []any{float64(599)},
		},
	}

	var handleErrorCount int
	p := newRetryParams(account, upstream, func(_ context.Context, _ string, _ *Account, _ int, _ http.Header, _ []byte, _ string, _ int64, _ string, _ bool) *handleModelRateLimitResult {
		handleErrorCount++
		return nil
	})

	result, err := svc.antigravityRetryLoop(p)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.resp)
	defer func() { _ = result.resp.Body.Close() }()

	require.Equal(t, http.StatusTooManyRequests, result.resp.StatusCode, "不再改写成 500")
	require.Equal(t, antigravityMaxRetries, upstream.calls, "should exhaust all retries")
	require.Equal(t, 1, handleErrorCount, "handleError should be called once after retries exhausted")
}
