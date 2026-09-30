//go:build unit

package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

func requireMappedModel(t *testing.T, account *Account, requested, expected string) {
	t.Helper()
	if actual := account.GetMappedModel(requested); actual != expected {
		t.Fatalf("GetMappedModel(%q) = %q, want %q", requested, actual, expected)
	}
}

// newMappingTestAccount 装配映射测试用的账号：普通渠道的改名放在 CatalogUpstreamModels（承接关系上的上游名）；
// shadow=true 时是 spark 影子号，credentials.model_mapping 是它的模型集合（替换默认表、兼任白名单、支持通配）。
func newMappingTestAccount(platform string, credentials map[string]any, catalogUpstreamModels map[string]string, shadow bool) *Account {
	account := &Account{
		Platform:              platform,
		Credentials:           credentials,
		CatalogUpstreamModels: catalogUpstreamModels,
	}
	if shadow {
		parentID := int64(1)
		account.ParentAccountID = &parentID
	}
	return account
}

func TestMatchWildcard(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		str      string
		expected bool
	}{
		// 精确匹配
		{"exact match", "claude-sonnet-4-5", "claude-sonnet-4-5", true},
		{"exact mismatch", "claude-sonnet-4-5", "claude-opus-4-5", false},

		// 通配符匹配
		{"wildcard prefix match", "claude-*", "claude-sonnet-4-5", true},
		{"wildcard prefix match 2", "claude-*", "claude-opus-4-5-thinking", true},
		{"wildcard prefix mismatch", "claude-*", "gemini-3-flash", false},
		{"wildcard partial match", "gemini-3*", "gemini-3-flash", true},
		{"wildcard partial match 2", "gemini-3*", "gemini-3-pro-image", true},
		{"wildcard partial mismatch", "gemini-3*", "gemini-2.5-flash", false},

		// 边界情况
		{"empty pattern exact", "", "", true},
		{"empty pattern mismatch", "", "claude", false},
		{"single star", "*", "anything", true},
		{"star at end only", "abc*", "abcdef", true},
		{"star at end empty suffix", "abc*", "abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchWildcard(tt.pattern, tt.str)
			if result != tt.expected {
				t.Errorf("matchWildcard(%q, %q) = %v, want %v", tt.pattern, tt.str, result, tt.expected)
			}
		})
	}
}

func TestMatchWildcardMappingResult(t *testing.T) {
	tests := []struct {
		name           string
		mapping        map[string]string
		requestedModel string
		expected       string
		matched        bool
	}{
		// 精确匹配优先于通配符
		{
			name: "exact match takes precedence",
			mapping: map[string]string{
				"claude-sonnet-4-5": "claude-sonnet-4-5-exact",
				"claude-*":          "claude-default",
			},
			requestedModel: "claude-sonnet-4-5",
			expected:       "claude-sonnet-4-5-exact",
			matched:        true,
		},

		// 最长通配符优先
		{
			name: "longer wildcard takes precedence",
			mapping: map[string]string{
				"claude-*":         "claude-default",
				"claude-sonnet-*":  "claude-sonnet-default",
				"claude-sonnet-4*": "claude-sonnet-4-series",
			},
			requestedModel: "claude-sonnet-4-5",
			expected:       "claude-sonnet-4-series",
			matched:        true,
		},

		// 单个通配符
		{
			name: "single wildcard",
			mapping: map[string]string{
				"claude-*": "claude-mapped",
			},
			requestedModel: "claude-opus-4-5",
			expected:       "claude-mapped",
			matched:        true,
		},

		// 无匹配返回原始模型
		{
			name: "no match returns original",
			mapping: map[string]string{
				"claude-*": "claude-mapped",
			},
			requestedModel: "gemini-3-flash",
			expected:       "gemini-3-flash",
			matched:        false,
		},

		// 空映射返回原始模型
		{
			name:           "empty mapping returns original",
			mapping:        map[string]string{},
			requestedModel: "claude-sonnet-4-5",
			expected:       "claude-sonnet-4-5",
			matched:        false,
		},

		// Gemini 模型映射
		{
			name: "gemini wildcard mapping",
			mapping: map[string]string{
				"gemini-3*":   "gemini-3-pro-high",
				"gemini-2.5*": "gemini-2.5-flash",
			},
			requestedModel: "gemini-3-flash-preview",
			expected:       "gemini-3-pro-high",
			matched:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, matched := matchWildcardMappingResult(tt.mapping, tt.requestedModel)
			if result != tt.expected || matched != tt.matched {
				t.Errorf("matchWildcardMappingResult(%v, %q) = (%q, %v), want (%q, %v)", tt.mapping, tt.requestedModel, result, matched, tt.expected, tt.matched)
			}
		})
	}
}

func TestAccountIsModelSupported(t *testing.T) {
	tests := []struct {
		name                  string
		platform              string
		credentials           map[string]any
		catalogUpstreamModels map[string]string
		// shadow 为 true 时按 spark 影子号装配：credentials.model_mapping 是模型集合（白名单、支持通配）。
		shadow         bool
		requestedModel string
		expected       bool
	}{
		// 无映射 = 允许所有
		{
			name:           "no mapping allows all",
			credentials:    nil,
			requestedModel: "any-model",
			expected:       true,
		},
		{
			name:           "empty mapping allows all",
			credentials:    map[string]any{},
			requestedModel: "any-model",
			expected:       true,
		},

		// 承接关系上的上游名只改名、不兼任白名单
		{
			name:                  "catalog upstream rename supported",
			catalogUpstreamModels: map[string]string{"claude-sonnet-4-5": "target-model"},
			requestedModel:        "claude-sonnet-4-5",
			expected:              true,
		},
		{
			name:                  "catalog upstream rename does not restrict other models",
			catalogUpstreamModels: map[string]string{"claude-sonnet-4-5": "target-model"},
			requestedModel:        "claude-opus-4-5",
			expected:              true,
		},

		// spark 影子号的模型列表：精确匹配（白名单）
		{
			name:   "shadow list exact match supported",
			shadow: true,
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"claude-sonnet-4-5": "target-model",
				},
			},
			requestedModel: "claude-sonnet-4-5",
			expected:       true,
		},
		{
			name:   "shadow list exact match not supported",
			shadow: true,
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"claude-sonnet-4-5": "target-model",
				},
			},
			requestedModel: "claude-opus-4-5",
			expected:       false,
		},

		// spark 影子号的模型列表：通配符匹配
		{
			name:   "shadow list wildcard match supported",
			shadow: true,
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"claude-*": "claude-sonnet-4-5",
				},
			},
			requestedModel: "claude-opus-4-5-thinking",
			expected:       true,
		},
		{
			name:     "gemini customtools alias matches normalized shadow list",
			platform: PlatformGemini,
			shadow:   true,
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"gemini-3.1-pro-preview": "gemini-3.1-pro-preview",
				},
			},
			requestedModel: "gemini-3.1-pro-preview-customtools",
			expected:       true,
		},
		{
			name:   "shadow list wildcard match not supported",
			shadow: true,
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"claude-*": "claude-sonnet-4-5",
				},
			},
			requestedModel: "gemini-3-flash",
			expected:       false,
		},

		// DeepSeek 空映射：官方模型白名单（不再是「允许所有」）
		{
			name:           "deepseek empty mapping allows official flash",
			platform:       PlatformDeepseek,
			credentials:    map[string]any{},
			requestedModel: "deepseek-flash",
			expected:       true,
		},
		{
			name:           "deepseek empty mapping normalizes claude code long context suffix",
			platform:       PlatformDeepseek,
			credentials:    map[string]any{},
			requestedModel: "deepseek-flash[1m]",
			expected:       true,
		},
		{
			name:           "deepseek empty mapping matches case-insensitively",
			platform:       PlatformDeepseek,
			credentials:    map[string]any{},
			requestedModel: "deepseek-FLASH",
			expected:       true,
		},
		{
			name:           "deepseek empty mapping allows versioned pro name",
			platform:       PlatformDeepseek,
			credentials:    map[string]any{},
			requestedModel: "deepseek-v4-pro-0813",
			expected:       true,
		},
		{
			name:           "deepseek empty mapping rejects retired chat model",
			platform:       PlatformDeepseek,
			credentials:    map[string]any{},
			requestedModel: "deepseek-chat",
			expected:       false,
		},
		{
			name:           "deepseek empty mapping rejects foreign claude model",
			platform:       PlatformDeepseek,
			credentials:    map[string]any{},
			requestedModel: "claude-sonnet-4-6",
			expected:       false,
		},
		{
			name:           "deepseek empty mapping rejects unknown gpt model",
			platform:       PlatformDeepseek,
			credentials:    map[string]any{},
			requestedModel: "gpt-future-model",
			expected:       false,
		},
		{
			name:           "deepseek empty mapping rejects typo",
			platform:       PlatformDeepseek,
			credentials:    map[string]any{},
			requestedModel: "deepseek-flas",
			expected:       false,
		},
		{
			name:                  "deepseek catalog upstream model wins over whitelist",
			platform:              PlatformDeepseek,
			credentials:           map[string]any{},
			catalogUpstreamModels: map[string]string{"foo-bar": "deepseek-flash"},
			requestedModel:        "foo-bar",
			expected:              true,
		},
		{
			name:           "non deepseek empty mapping still allows all",
			platform:       PlatformAnthropic,
			credentials:    map[string]any{},
			requestedModel: "any-model",
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := newMappingTestAccount(tt.platform, tt.credentials, tt.catalogUpstreamModels, tt.shadow)
			result := account.IsModelSupported(tt.requestedModel)
			if result != tt.expected {
				t.Errorf("IsModelSupported(%q) = %v, want %v", tt.requestedModel, result, tt.expected)
			}
		})
	}
}

func TestAccountGetMappedModel(t *testing.T) {
	tests := []struct {
		name                  string
		platform              string
		credentials           map[string]any
		catalogUpstreamModels map[string]string
		shadow                bool
		requestedModel        string
		expected              string
	}{
		// 无映射 = 返回原始模型
		{
			name:           "no mapping returns original",
			credentials:    nil,
			requestedModel: "claude-sonnet-4-5",
			expected:       "claude-sonnet-4-5",
		},
		{
			name:           "no mapping preserves gemini customtools model",
			platform:       PlatformGemini,
			credentials:    nil,
			requestedModel: "gemini-3.1-pro-preview-customtools",
			expected:       "gemini-3.1-pro-preview-customtools",
		},

		// 精确匹配
		{
			name:                  "exact match",
			catalogUpstreamModels: map[string]string{"claude-sonnet-4-5": "target-model"},
			requestedModel:        "claude-sonnet-4-5",
			expected:              "target-model",
		},

		// 通配符匹配（最长优先）：只有 spark 影子号的模型列表支持通配
		{
			name:   "wildcard longest match",
			shadow: true,
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"claude-*":        "claude-default",
					"claude-sonnet-*": "claude-sonnet-mapped",
				},
			},
			requestedModel: "claude-sonnet-4-5",
			expected:       "claude-sonnet-mapped",
		},

		// 无匹配返回原始模型
		{
			name:                  "gemini customtools alias resolves through normalized mapping",
			platform:              PlatformGemini,
			catalogUpstreamModels: map[string]string{"gemini-3.1-pro-preview": "gemini-3.1-pro-preview"},
			requestedModel:        "gemini-3.1-pro-preview-customtools",
			expected:              "gemini-3.1-pro-preview",
		},
		{
			name:     "gemini customtools exact mapping wins over normalized fallback",
			platform: PlatformGemini,
			catalogUpstreamModels: map[string]string{
				"gemini-3.1-pro-preview":             "gemini-3.1-pro-preview",
				"gemini-3.1-pro-preview-customtools": "gemini-3.1-pro-preview-customtools",
			},
			requestedModel: "gemini-3.1-pro-preview-customtools",
			expected:       "gemini-3.1-pro-preview-customtools",
		},
		{
			name:   "no match returns original",
			shadow: true,
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"gemini-*": "gemini-mapped",
				},
			},
			requestedModel: "claude-sonnet-4-5",
			expected:       "claude-sonnet-4-5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := newMappingTestAccount(tt.platform, tt.credentials, tt.catalogUpstreamModels, tt.shadow)
			result := account.GetMappedModel(tt.requestedModel)
			if result != tt.expected {
				t.Errorf("GetMappedModel(%q) = %q, want %q", tt.requestedModel, result, tt.expected)
			}
		})
	}
}

// 别名补全只作用于整份替换默认表的列表（影子号那条路径）；普通号的上游名按目录标识精确叠加、原样生效。
func TestAccountGetModelMapping_AntigravityNormalizesGemini31ProAliases(t *testing.T) {
	t.Parallel()

	account := newMappingTestAccount(PlatformAntigravity, map[string]any{
		"model_mapping": map[string]any{
			domain.AntigravityGemini31ProAgentModel: domain.AntigravityGemini31ProAgentModel,
			"gemini-3.1-pro-high":                   "gemini-3.1-pro-high",
			"gemini-3.1-pro-preview":                "gemini-3.1-pro-high",
		},
	}, nil, true)

	mapping := account.GetModelMapping()

	if got := mapping["gemini-3.1-pro"]; got != domain.AntigravityGemini31ProAgentModel {
		t.Fatalf("expected gemini-3.1-pro to map to %q, got %q", domain.AntigravityGemini31ProAgentModel, got)
	}
	if got := mapping["gemini-3.1-pro-high"]; got != domain.AntigravityGemini31ProAgentModel {
		t.Fatalf("expected gemini-3.1-pro-high to map to %q, got %q", domain.AntigravityGemini31ProAgentModel, got)
	}
	if got := mapping["gemini-3.1-pro-preview"]; got != domain.AntigravityGemini31ProAgentModel {
		t.Fatalf("expected gemini-3.1-pro-preview to map to %q, got %q", domain.AntigravityGemini31ProAgentModel, got)
	}
}

func TestAccountGetModelMapping_AntigravityPreservesGemini31ProOverrides(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformAntigravity,
		CatalogUpstreamModels: map[string]string{
			domain.AntigravityGemini31ProAgentModel: domain.AntigravityGemini31ProAgentModel,
			"gemini-3.1-pro-high":                   "custom-high",
			"gemini-3.1-pro-preview":                "custom-preview",
		},
	}

	mapping := account.GetModelMapping()

	if got := mapping["gemini-3.1-pro-high"]; got != "custom-high" {
		t.Fatalf("expected gemini-3.1-pro-high override to be preserved, got %q", got)
	}
	if got := mapping["gemini-3.1-pro-preview"]; got != "custom-preview" {
		t.Fatalf("expected gemini-3.1-pro-preview override to be preserved, got %q", got)
	}
	if got := mapping["gemini-3.1-pro"]; got != domain.AntigravityGemini31ProAgentModel {
		t.Fatalf("expected gemini-3.1-pro alias to default to %q, got %q", domain.AntigravityGemini31ProAgentModel, got)
	}
}

// 通配只出现在 spark 影子号的模型列表里（它替换默认表），别名补全要让位给通配。
func TestAccountGetModelMapping_AntigravityGemini31ProAliasesRespectWildcard(t *testing.T) {
	t.Parallel()

	parentID := int64(1)
	account := &Account{
		Platform:        PlatformAntigravity,
		ParentAccountID: &parentID,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				domain.AntigravityGemini31ProAgentModel: domain.AntigravityGemini31ProAgentModel,
				"gemini-3.1-*":                          "custom-wildcard",
			},
		},
	}

	mapping := account.GetModelMapping()

	if got := mapping["gemini-3.1-pro"]; got != "" {
		t.Fatalf("expected gemini-3.1-pro exact alias to stay unset when wildcard exists, got %q", got)
	}
	if got := mapping["gemini-3.1-pro-high"]; got != "" {
		t.Fatalf("expected gemini-3.1-pro-high exact alias to stay unset when wildcard exists, got %q", got)
	}
	if got := mapping["gemini-3.1-pro-preview"]; got != "" {
		t.Fatalf("expected gemini-3.1-pro-preview exact alias to stay unset when wildcard exists, got %q", got)
	}
}

func TestAccountResolveMappedModel(t *testing.T) {
	tests := []struct {
		name                  string
		platform              string
		credentials           map[string]any
		catalogUpstreamModels map[string]string
		shadow                bool
		requestedModel        string
		expectedModel         string
		expectedMatch         bool
	}{
		{
			name:           "no mapping reports unmatched",
			credentials:    nil,
			requestedModel: "gpt-5.4",
			expectedModel:  "gpt-5.4",
			expectedMatch:  false,
		},
		{
			name:                  "exact passthrough mapping still counts as matched",
			catalogUpstreamModels: map[string]string{"gpt-5.4": "gpt-5.4"},
			requestedModel:        "gpt-5.4",
			expectedModel:         "gpt-5.4",
			expectedMatch:         true,
		},
		{
			name:   "wildcard passthrough mapping still counts as matched",
			shadow: true,
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"gpt-*": "gpt-5.4",
				},
			},
			requestedModel: "gpt-5.4",
			expectedModel:  "gpt-5.4",
			expectedMatch:  true,
		},
		{
			name:                  "gemini customtools alias reports normalized match",
			platform:              PlatformGemini,
			catalogUpstreamModels: map[string]string{"gemini-3.1-pro-preview": "gemini-3.1-pro-preview"},
			requestedModel:        "gemini-3.1-pro-preview-customtools",
			expectedModel:         "gemini-3.1-pro-preview",
			expectedMatch:         true,
		},
		{
			name:     "gemini customtools exact mapping reports exact match",
			platform: PlatformGemini,
			catalogUpstreamModels: map[string]string{
				"gemini-3.1-pro-preview":             "gemini-3.1-pro-preview",
				"gemini-3.1-pro-preview-customtools": "gemini-3.1-pro-preview-customtools",
			},
			requestedModel: "gemini-3.1-pro-preview-customtools",
			expectedModel:  "gemini-3.1-pro-preview-customtools",
			expectedMatch:  true,
		},
		{
			name:                  "missing mapping reports unmatched",
			catalogUpstreamModels: map[string]string{"gpt-5.2": "gpt-5.2"},
			requestedModel:        "gpt-5.4",
			expectedModel:         "gpt-5.4",
			expectedMatch:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := newMappingTestAccount(tt.platform, tt.credentials, tt.catalogUpstreamModels, tt.shadow)
			mappedModel, matched := account.ResolveMappedModel(tt.requestedModel)
			if mappedModel != tt.expectedModel || matched != tt.expectedMatch {
				t.Fatalf("ResolveMappedModel(%q) = (%q, %v), want (%q, %v)", tt.requestedModel, mappedModel, matched, tt.expectedModel, tt.expectedMatch)
			}
		})
	}
}

// 透传补全保证替换默认表的模型列表（spark 影子号）里仍有这些 Gemini 默认模型。
func TestAccountGetModelMapping_AntigravityEnsuresGeminiDefaultPassthroughs(t *testing.T) {
	parentID := int64(1)
	account := &Account{
		Platform:        PlatformAntigravity,
		ParentAccountID: &parentID,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"gemini-3-pro-high": "gemini-3.1-pro-high",
			},
		},
	}

	mapping := account.GetModelMapping()
	if mapping["gemini-3-flash"] != "gemini-3-flash" {
		t.Fatalf("expected gemini-3-flash passthrough to be auto-filled, got: %q", mapping["gemini-3-flash"])
	}
	if mapping["gemini-3.1-pro-high"] != "gemini-3.1-pro-high" {
		t.Fatalf("expected gemini-3.1-pro-high passthrough to be auto-filled, got: %q", mapping["gemini-3.1-pro-high"])
	}
	if mapping["gemini-3.1-pro-low"] != "gemini-3.1-pro-low" {
		t.Fatalf("expected gemini-3.1-pro-low passthrough to be auto-filled, got: %q", mapping["gemini-3.1-pro-low"])
	}
}

func TestAccountGetModelMapping_GoogleOneUsesConservativeDefaults(t *testing.T) {
	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"oauth_type": "google_one",
		},
	}

	mapping := account.GetModelMapping()
	for _, model := range []string{"gemini-2.0-flash", "gemini-2.5-flash", "gemini-2.5-pro"} {
		if mapping[model] != model {
			t.Fatalf("expected Google One model %q to map to itself, got %q", model, mapping[model])
		}
	}
	for _, model := range []string{"gemini-2.5-flash-image", "gemini-3.1-flash-image", "gemini-3.5-flash"} {
		if _, ok := mapping[model]; ok {
			t.Fatalf("did not expect unsupported Google One model %q", model)
		}
	}
	if account.IsModelSupported("gemini-3.5-flash") {
		t.Fatal("Google One defaults must not treat unsupported models as eligible")
	}
}

// 承接关系上的上游名叠在 Google One 保守默认表之上：改名生效、默认表里的模型仍在，默认表仍是模型集合。
func TestAccountGetModelMapping_GoogleOneMergesCatalogUpstreamModels(t *testing.T) {
	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"oauth_type": "google_one",
		},
		CatalogUpstreamModels: map[string]string{
			"custom-model": "gemini-2.5-flash",
		},
	}

	mapping := account.GetModelMapping()
	if mapping["custom-model"] != "gemini-2.5-flash" {
		t.Fatalf("expected catalog upstream model to be applied, got %v", mapping)
	}
	if mapping["gemini-2.5-pro"] != "gemini-2.5-pro" {
		t.Fatalf("expected Google One defaults to stay under the catalog upstream models: %v", mapping)
	}
	if account.IsModelSupported("gemini-3.5-flash") {
		t.Fatal("Google One defaults must stay the model set when catalog upstream models are merged on top")
	}
}

func TestAccountGetModelMapping_AntigravityRespectsWildcardOverride(t *testing.T) {
	parentID := int64(1)
	account := &Account{
		Platform:        PlatformAntigravity,
		ParentAccountID: &parentID,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"gemini-3*": "gemini-3.1-pro-high",
			},
		},
	}

	mapping := account.GetModelMapping()
	if _, exists := mapping["gemini-3-flash"]; exists {
		t.Fatalf("did not expect explicit gemini-3-flash passthrough when wildcard already exists")
	}
	if _, exists := mapping["gemini-3.1-pro-high"]; exists {
		t.Fatalf("did not expect explicit gemini-3.1-pro-high passthrough when wildcard already exists")
	}
	if _, exists := mapping["gemini-3.1-pro-low"]; exists {
		t.Fatalf("did not expect explicit gemini-3.1-pro-low passthrough when wildcard already exists")
	}
	if mapped := account.GetMappedModel("gemini-3-flash"); mapped != "gemini-3.1-pro-high" {
		t.Fatalf("expected wildcard mapping to stay effective, got: %q", mapped)
	}
}

// 影子号的模型列表在 credentials 里：换了 credentials 缓存要失效。
func TestAccountGetModelMapping_CacheInvalidatesOnCredentialsReplace(t *testing.T) {
	parentID := int64(1)
	account := &Account{
		ParentAccountID: &parentID,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"claude-3-5-sonnet": "upstream-a",
			},
		},
	}

	first := account.GetModelMapping()
	if first["claude-3-5-sonnet"] != "upstream-a" {
		t.Fatalf("unexpected first mapping: %v", first)
	}

	account.Credentials = map[string]any{
		"model_mapping": map[string]any{
			"claude-3-5-sonnet": "upstream-b",
		},
	}
	second := account.GetModelMapping()
	if second["claude-3-5-sonnet"] != "upstream-b" {
		t.Fatalf("expected cache invalidated after credentials replace, got: %v", second)
	}
}

func TestAccountGetModelMapping_CacheInvalidatesOnCatalogUpstreamModelsReplace(t *testing.T) {
	account := &Account{
		CatalogUpstreamModels: map[string]string{"claude-3-5-sonnet": "upstream-a"},
	}

	first := account.GetModelMapping()
	if first["claude-3-5-sonnet"] != "upstream-a" {
		t.Fatalf("unexpected first mapping: %v", first)
	}

	account.CatalogUpstreamModels = map[string]string{"claude-3-5-sonnet": "upstream-b"}
	second := account.GetModelMapping()
	if second["claude-3-5-sonnet"] != "upstream-b" {
		t.Fatalf("expected cache invalidated after catalog upstream models replace, got: %v", second)
	}
}

func TestAccountGetModelMapping_CacheInvalidatesOnMappingLenChange(t *testing.T) {
	rawMapping := map[string]string{
		"claude-sonnet": "sonnet-a",
	}
	account := &Account{
		CatalogUpstreamModels: rawMapping,
	}

	first := account.GetModelMapping()
	if len(first) != 1 {
		t.Fatalf("unexpected first mapping length: %d", len(first))
	}

	rawMapping["claude-opus"] = "opus-b"
	second := account.GetModelMapping()
	if second["claude-opus"] != "opus-b" {
		t.Fatalf("expected cache invalidated after mapping len change, got: %v", second)
	}
}

func TestAccountGetModelMapping_CacheInvalidatesOnInPlaceValueChange(t *testing.T) {
	rawMapping := map[string]string{
		"claude-sonnet": "sonnet-a",
	}
	account := &Account{
		CatalogUpstreamModels: rawMapping,
	}

	first := account.GetModelMapping()
	if first["claude-sonnet"] != "sonnet-a" {
		t.Fatalf("unexpected first mapping: %v", first)
	}

	rawMapping["claude-sonnet"] = "sonnet-b"
	second := account.GetModelMapping()
	if second["claude-sonnet"] != "sonnet-b" {
		t.Fatalf("expected cache invalidated after in-place value change, got: %v", second)
	}
}

// 模型映射缓存键含厂商：同一对象改了协议地址（管理端改号后复用），厂商默认映射要跟着变，
// 不能沿用上一次按旧厂商解析出的结果。
func TestAccountGetModelMapping_CacheInvalidatesWhenVendorChanges(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformGrok, Type: AccountTypeAPIKey,
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://relay.example/v1"},
		Credentials:       map[string]any{},
	}
	if relayMapping := account.GetModelMapping(); len(relayMapping) != 0 {
		t.Fatalf("a relay key with an empty mapping allows everything, got %v", relayMapping)
	}

	// 同一个对象的厂商变了（改成 Grok 成品号）：按厂商启用 xAI 模型目录默认映射，缓存必须重建。
	// 第三方 key 的厂商只可能是国产厂商或空串（海外四家不再有官方 key），它们都没有默认映射。
	account.Type = AccountTypeOAuth
	account.ProtocolEndpoints = nil
	if account.Vendor() != PlatformGrok {
		t.Fatalf("fixture: expected grok vendor, got %q", account.Vendor())
	}
	if officialMapping := account.GetModelMapping(); len(officialMapping) == 0 {
		t.Fatal("expected the cache to be rebuilt for the new vendor with the xAI default mapping")
	}
}
