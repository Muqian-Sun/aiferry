//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// TestCheckErrorPolicy — 池模式与 529 过载（渠道级自定义错误码、临时不可调度规则 2026-09-28 P5 已删）
// ---------------------------------------------------------------------------

// legacyErrorPolicyCredentials 库里旧行可能留着的渠道级自定义错误码与临时不可调度规则：读取处已删，不再生效。
func legacyErrorPolicyCredentials(extra map[string]any) map[string]any {
	credentials := map[string]any{
		"custom_error_codes_enabled": true,
		"custom_error_codes":         []any{float64(429), float64(500), float64(503), float64(529)},
		"temp_unschedulable_enabled": true,
		"temp_unschedulable_rules": []any{
			map[string]any{
				"error_code":       float64(503),
				"keywords":         []any{"overloaded"},
				"duration_minutes": float64(10),
			},
		},
	}
	for key, value := range extra {
		credentials[key] = value
	}
	return credentials
}

func TestCheckErrorPolicy(t *testing.T) {
	openAIEndpoints := map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"}
	tests := []struct {
		name       string
		account    *Account
		statusCode int
		expected   ErrorPolicyResult
	}{
		{
			name:       "no_policy_oauth_returns_none",
			account:    &Account{ID: 1, Type: AccountTypeOAuth, Platform: PlatformAntigravity},
			statusCode: 500,
			expected:   ErrorPolicyNone,
		},
		{
			name: "pool_mode_skips_global_529_cooldown",
			account: &Account{
				ID: 34, Type: AccountTypeAPIKey, Platform: PlatformOpenAI,
				Credentials:       map[string]any{"pool_mode": true},
				ProtocolEndpoints: openAIEndpoints,
			},
			statusCode: 529,
			expected:   ErrorPolicySkipped,
		},
		{
			name: "ordinary_account_uses_global_529_cooldown",
			account: &Account{
				ID: 35, Type: AccountTypeAPIKey, Platform: PlatformOpenAI,
				ProtocolEndpoints: openAIEndpoints,
			},
			statusCode: 529,
			expected:   ErrorPolicyMatched,
		},
		{
			name: "pool_mode_returns_skipped",
			account: &Account{
				ID: 8, Type: AccountTypeAPIKey, Platform: PlatformOpenAI,
				Credentials:       map[string]any{"pool_mode": true},
				ProtocolEndpoints: openAIEndpoints,
			},
			statusCode: 401,
			expected:   ErrorPolicySkipped,
		},
		{
			name: "legacy_custom_codes_and_temp_rules_ignored_500",
			account: &Account{
				ID: 2, Type: AccountTypeAPIKey, Platform: PlatformOpenAI,
				Credentials:       legacyErrorPolicyCredentials(nil),
				ProtocolEndpoints: openAIEndpoints,
			},
			statusCode: 500,
			expected:   ErrorPolicyNone,
		},
		{
			name: "legacy_custom_codes_and_temp_rules_ignored_503",
			account: &Account{
				ID: 3, Type: AccountTypeAPIKey, Platform: PlatformOpenAI,
				Credentials:       legacyErrorPolicyCredentials(nil),
				ProtocolEndpoints: openAIEndpoints,
			},
			statusCode: 503,
			expected:   ErrorPolicyNone,
		},
		{
			name: "legacy_custom_codes_do_not_change_529",
			account: &Account{
				ID: 4, Type: AccountTypeAPIKey, Platform: PlatformOpenAI,
				Credentials:       legacyErrorPolicyCredentials(nil),
				ProtocolEndpoints: openAIEndpoints,
			},
			statusCode: 529,
			expected:   ErrorPolicyMatched,
		},
		{
			name: "legacy_custom_codes_do_not_override_pool_mode",
			account: &Account{
				ID: 7, Type: AccountTypeAPIKey, Platform: PlatformOpenAI,
				Credentials:       legacyErrorPolicyCredentials(map[string]any{"pool_mode": true}),
				ProtocolEndpoints: openAIEndpoints,
			},
			statusCode: 401,
			expected:   ErrorPolicySkipped,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &errorPolicyRepoStub{}
			svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)

			result := svc.CheckErrorPolicy(tt.account, tt.statusCode)
			require.Equal(t, tt.expected, result, "unexpected ErrorPolicyResult")
			require.Zero(t, repo.tempCalls, "策略检查本身不写账号状态")
		})
	}
}

func TestHandleUpstreamError_PoolModePolicies(t *testing.T) {
	openAIEndpoints := map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"}

	t.Run("pool_mode_skips_local_error_policy", func(t *testing.T) {
		repo := &errorPolicyRepoStub{}
		svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		account := &Account{
			ID: 30, Type: AccountTypeAPIKey, Platform: PlatformOpenAI,
			Credentials:       map[string]any{"pool_mode": true},
			ProtocolEndpoints: openAIEndpoints,
		}

		shouldDisable := svc.HandleUpstreamError(context.Background(), account, 401, http.Header{}, []byte("unauthorized"))

		require.False(t, shouldDisable)
		require.Equal(t, 0, repo.setErrCalls)
		require.Equal(t, 0, repo.tempCalls)
	})

	t.Run("pool_mode_legacy_custom_codes_ignored", func(t *testing.T) {
		repo := &errorPolicyRepoStub{}
		svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		account := &Account{
			ID: 31, Type: AccountTypeAPIKey, Platform: PlatformOpenAI,
			Credentials:       legacyErrorPolicyCredentials(map[string]any{"pool_mode": true, "custom_error_codes": []any{float64(401)}}),
			ProtocolEndpoints: openAIEndpoints,
		}

		shouldDisable := svc.HandleUpstreamError(context.Background(), account, 401, http.Header{}, []byte("unauthorized"))

		require.False(t, shouldDisable)
		require.Equal(t, 0, repo.setErrCalls)
		require.Equal(t, 0, repo.tempCalls)
	})

	t.Run("pool_mode_legacy_temp_rule_ignored", func(t *testing.T) {
		repo := &errorPolicyRepoStub{}
		svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		account := &Account{
			ID: 32, Type: AccountTypeAPIKey, Platform: PlatformOpenAI,
			Credentials:       legacyErrorPolicyCredentials(map[string]any{"pool_mode": true}),
			ProtocolEndpoints: openAIEndpoints,
		}

		shouldDisable := svc.HandleUpstreamError(context.Background(), account, http.StatusServiceUnavailable, http.Header{}, []byte("service overloaded"))

		require.False(t, shouldDisable)
		require.Equal(t, 0, repo.setErrCalls)
		require.Equal(t, 0, repo.tempCalls)
		require.Empty(t, repo.modelRateLimitCalls)
	})
}

// ---------------------------------------------------------------------------
// TestApplyErrorPolicy — table-driven cases for the wrapper method
// ---------------------------------------------------------------------------

func TestApplyErrorPolicy(t *testing.T) {
	tests := []struct {
		name             string
		account          *Account
		statusCode       int
		body             []byte
		expectedHandled  bool
		expectedStatus   int // expected outStatus
		handleErrorCalls int
	}{
		{
			name: "none_not_handled",
			account: &Account{
				ID:       10,
				Type:     AccountTypeOAuth,
				Platform: PlatformAntigravity,
			},
			statusCode:       500,
			body:             []byte(`"error"`),
			expectedHandled:  false,
			expectedStatus:   500, // passthrough
			handleErrorCalls: 0,
		},
		{
			name: "pool_mode_skipped_handled_no_handleError",
			account: &Account{
				ID:          11,
				Type:        AccountTypeAPIKey,
				Platform:    PlatformAntigravity,
				Credentials: map[string]any{"pool_mode": true},
			},
			statusCode:       500,
			body:             []byte(`"error"`),
			expectedHandled:  true,
			expectedStatus:   http.StatusInternalServerError, // skipped → 500
			handleErrorCalls: 0,
		},
		{
			name: "overload_matched_handled_calls_handleError",
			account: &Account{
				ID:       12,
				Type:     AccountTypeAPIKey,
				Platform: PlatformAntigravity,
			},
			statusCode:       529,
			body:             []byte(`"overloaded"`),
			expectedHandled:  true,
			expectedStatus:   529, // matched → original status
			handleErrorCalls: 1,
		},
		{
			name: "legacy_temp_rule_ignored",
			account: &Account{
				ID:       13,
				Type:     AccountTypeOAuth,
				Platform: PlatformAntigravity,
				Credentials: legacyErrorPolicyCredentials(map[string]any{
					"model_mapping": map[string]any{"claude-sonnet-4-5": "claude-sonnet-4-5"},
				}),
			},
			statusCode:       503,
			body:             []byte(`overloaded`),
			expectedHandled:  false,
			expectedStatus:   503,
			handleErrorCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &errorPolicyRepoStub{}
			rlSvc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
			svc := &AntigravityGatewayService{
				rateLimitService: rlSvc,
			}

			var handleErrorCount int
			p := antigravityRetryLoopParams{
				ctx:            context.Background(),
				prefix:         "[test]",
				account:        tt.account,
				requestedModel: "claude-sonnet-4-5",
				handleError: func(ctx context.Context, prefix string, account *Account, statusCode int, headers http.Header, body []byte, requestedModel string, groupID int64, sessionHash string, isStickySession bool) *handleModelRateLimitResult {
					handleErrorCount++
					return nil
				},
				isStickySession: true,
			}

			handled, outStatus, retErr := svc.applyErrorPolicy(p, tt.statusCode, http.Header{}, tt.body)

			require.Equal(t, tt.expectedHandled, handled, "handled mismatch")
			require.Equal(t, tt.expectedStatus, outStatus, "outStatus mismatch")
			require.Equal(t, tt.handleErrorCalls, handleErrorCount, "handleError call count mismatch")
			require.NoError(t, retErr)
			require.Zero(t, repo.tempCalls)
			require.Empty(t, repo.modelRateLimitCalls)
		})
	}
}

// 池模式（ErrorPolicySkipped）下，Gemini 模型级限流仍先于跳过分支处理。
func TestApplyErrorPolicy_GeminiRateLimitBypassesPoolModeSkip(t *testing.T) {
	repo := &stubAntigravityAccountRepo{}
	cache := &stubSmartRetryCache{}
	rlSvc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc := &AntigravityGatewayService{
		rateLimitService: rlSvc,
		accountRepo:      repo,
		cache:            cache,
	}

	account := &Account{
		ID:          31,
		Type:        AccountTypeAPIKey,
		Platform:    PlatformAntigravity,
		Credentials: map[string]any{"pool_mode": true},
	}
	body := []byte(`{
		"error": {
			"status": "RESOURCE_EXHAUSTED",
			"details": [
				{"@type": "type.googleapis.com/google.rpc.ErrorInfo", "metadata": {"model": "gemini-3-flash"}, "reason": "RATE_LIMIT_EXCEEDED"},
				{"@type": "type.googleapis.com/google.rpc.RetryInfo", "retryDelay": "15s"}
			]
		}
	}`)
	p := antigravityRetryLoopParams{
		ctx:         context.Background(),
		prefix:      "[test]",
		account:     account,
		accountRepo: repo,
		scopeID:     42,
		sessionHash: "gemini:sticky",
		handleError: func(context.Context, string, *Account, int, http.Header, []byte, string, int64, string, bool) *handleModelRateLimitResult {
			t.Fatal("model rate limit should be handled before the pool-mode skip")
			return nil
		},
	}

	handled, outStatus, retErr := svc.applyErrorPolicy(p, http.StatusTooManyRequests, http.Header{}, body)

	require.True(t, handled)
	require.Equal(t, http.StatusTooManyRequests, outStatus)
	require.NoError(t, retErr)
	require.Len(t, repo.modelRateLimitCalls, 2)
	require.Equal(t, "gemini-3-flash", repo.modelRateLimitCalls[0].modelKey)
	require.Equal(t, antigravityGeminiModelRateLimitKey, repo.modelRateLimitCalls[1].modelKey)
	require.Len(t, cache.deleteCalls, 1)
	require.Equal(t, int64(42), cache.deleteCalls[0].scopeID)
	require.Equal(t, "gemini:sticky", cache.deleteCalls[0].sessionHash)
}

// ---------------------------------------------------------------------------
// errorPolicyRepoStub — minimal AccountRepository stub for error policy tests
// ---------------------------------------------------------------------------

type errorPolicyRepoStub struct {
	mockAccountRepoForGemini
	tempCalls           int
	setErrCalls         int
	lastErrorMsg        string
	modelRateLimitCalls []modelNotFoundRateLimitCall
}

func (r *errorPolicyRepoStub) SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error {
	r.tempCalls++
	return nil
}

func (r *errorPolicyRepoStub) SetError(ctx context.Context, id int64, errorMsg string) error {
	r.setErrCalls++
	r.lastErrorMsg = errorMsg
	return nil
}

func (r *errorPolicyRepoStub) SetModelRateLimit(_ context.Context, id int64, scope string, resetAt time.Time, reason ...string) error {
	call := modelNotFoundRateLimitCall{accountID: id, scope: scope, resetAt: resetAt}
	if len(reason) > 0 {
		call.reason = reason[0]
	}
	r.modelRateLimitCalls = append(r.modelRateLimitCalls, call)
	return nil
}
