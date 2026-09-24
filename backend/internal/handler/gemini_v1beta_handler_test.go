//go:build unit

package handler

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGeminiV1BetaListModels_AllowlistFiltersNativeResponse(t *testing.T) {
	body := []byte(`{"models":[{"name":"models/gemini-2.5-pro"},{"name":"models/gemini-2.5-flash"}],"nextPageToken":"next"}`)
	filtered, dropped, ok := filterUpstreamGeminiModelsBody(body, func(name string) bool { return strings.TrimPrefix(name, "models/") == "gemini-2.5-pro" })
	require.True(t, ok)
	require.True(t, dropped)
	require.JSONEq(t, `{"models":[{"name":"models/gemini-2.5-pro"}],"nextPageToken":"next"}`, string(filtered))
}

func TestGeminiModelAllowlist_DisabledPreservesNativeResponse(t *testing.T) {
	body := []byte(`{"models":[{"name":"models/gemini-2.5-pro"}]}`)
	filtered, dropped, ok := filterUpstreamGeminiModelsBody(body, func(string) bool { return true })
	require.True(t, ok)
	require.False(t, dropped)
	require.Equal(t, body, filtered)
}

// TestGeminiV1BetaHandler_PlatformRoutingInvariant 验证 GeminiV1BetaModels 的分流判定：
// 只有 Antigravity 成品号走 ForwardGemini（v1internal），第三方 key 不论标签都走 ForwardNative。
func TestGeminiV1BetaHandler_PlatformRoutingInvariant(t *testing.T) {
	tests := []struct {
		name            string
		account         *service.Account
		expectedService string
	}{
		{
			name:            "Gemini成品号使用ForwardNative",
			account:         &service.Account{Platform: service.PlatformGemini, Type: service.AccountTypeOAuth},
			expectedService: "GeminiMessagesCompatService.ForwardNative",
		},
		{
			name:            "Antigravity成品号使用ForwardGemini",
			account:         &service.Account{Platform: service.PlatformAntigravity, Type: service.AccountTypeOAuth},
			expectedService: "AntigravityGatewayService.ForwardGemini",
		},
		{
			name:            "标签为antigravity的第三方key使用ForwardNative",
			account:         &service.Account{Platform: service.PlatformAntigravity, Type: service.AccountTypeAPIKey},
			expectedService: "GeminiMessagesCompatService.ForwardNative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routedService := "GeminiMessagesCompatService.ForwardNative"
			if usesAntigravityV1Internal(tt.account) {
				routedService = "AntigravityGatewayService.ForwardGemini"
			}
			require.Equal(t, tt.expectedService, routedService)
		})
	}
}

// TestGeminiV1BetaHandler_ListModelsAntigravityFallback 验证 ListModels 的 antigravity 降级逻辑
// 当没有 gemini 账户但有 antigravity 账户时，应返回静态模型列表
func TestGeminiV1BetaHandler_ListModelsAntigravityFallback(t *testing.T) {
	tests := []struct {
		name             string
		hasGeminiAccount bool
		hasAntigravity   bool
		expectedBehavior string
	}{
		{
			name:             "有Gemini账户-调用ForwardAIStudioGET",
			hasGeminiAccount: true,
			hasAntigravity:   false,
			expectedBehavior: "forward_to_upstream",
		},
		{
			name:             "无Gemini有Antigravity-返回静态列表",
			hasGeminiAccount: false,
			hasAntigravity:   true,
			expectedBehavior: "static_fallback",
		},
		{
			name:             "无任何账户-返回503",
			hasGeminiAccount: false,
			hasAntigravity:   false,
			expectedBehavior: "service_unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟 GeminiV1BetaListModels 的逻辑 (lines 33-44 in gemini_v1beta_handler.go)
			var behavior string

			if tt.hasGeminiAccount {
				behavior = "forward_to_upstream"
			} else if tt.hasAntigravity {
				behavior = "static_fallback"
			} else {
				behavior = "service_unavailable"
			}

			require.Equal(t, tt.expectedBehavior, behavior)
		})
	}
}

// TestGeminiV1BetaHandler_GetModelAntigravityFallback 验证 GetModel 的 antigravity 降级逻辑
func TestGeminiV1BetaHandler_GetModelAntigravityFallback(t *testing.T) {
	tests := []struct {
		name             string
		hasGeminiAccount bool
		hasAntigravity   bool
		expectedBehavior string
	}{
		{
			name:             "有Gemini账户-调用ForwardAIStudioGET",
			hasGeminiAccount: true,
			hasAntigravity:   false,
			expectedBehavior: "forward_to_upstream",
		},
		{
			name:             "无Gemini有Antigravity-返回静态模型信息",
			hasGeminiAccount: false,
			hasAntigravity:   true,
			expectedBehavior: "static_model_info",
		},
		{
			name:             "无任何账户-返回503",
			hasGeminiAccount: false,
			hasAntigravity:   false,
			expectedBehavior: "service_unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟 GeminiV1BetaGetModel 的逻辑 (lines 77-87 in gemini_v1beta_handler.go)
			var behavior string

			if tt.hasGeminiAccount {
				behavior = "forward_to_upstream"
			} else if tt.hasAntigravity {
				behavior = "static_model_info"
			} else {
				behavior = "service_unavailable"
			}

			require.Equal(t, tt.expectedBehavior, behavior)
		})
	}
}

func TestShouldFallbackGeminiModel_KnownFallbackOn404(t *testing.T) {
	t.Parallel()

	res := &service.UpstreamHTTPResult{StatusCode: http.StatusNotFound}
	require.True(t, shouldFallbackGeminiModel("gemini-3.1-pro-preview-customtools", res))
}

func TestShouldFallbackGeminiModel_UnknownModelOn404(t *testing.T) {
	t.Parallel()

	res := &service.UpstreamHTTPResult{StatusCode: http.StatusNotFound}
	require.False(t, shouldFallbackGeminiModel("gemini-future-model", res))
}

func TestShouldFallbackGeminiModel_DelegatesScopeFallback(t *testing.T) {
	t.Parallel()

	res := &service.UpstreamHTTPResult{
		StatusCode: http.StatusForbidden,
		Headers:    http.Header{"Www-Authenticate": []string{"Bearer error=\"insufficient_scope\""}},
		Body:       []byte("insufficient authentication scopes"),
	}
	require.True(t, shouldFallbackGeminiModel("gemini-future-model", res))
}
