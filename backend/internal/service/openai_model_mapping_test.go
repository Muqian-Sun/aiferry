package service

import "testing"

func TestResolveOpenAIForwardModel(t *testing.T) {
	tests := []struct {
		name           string
		account        *Account
		requestedModel string
		expectedModel  string
	}{
		{
			name:           "nil account keeps requested model",
			requestedModel: "claude-fable-5",
			expectedModel:  "claude-fable-5",
		},
		{
			name: "unknown gpt model without mapping keeps requested model",
			account: &Account{
				Credentials: map[string]any{},
			},
			requestedModel: "gpt6",
			expectedModel:  "gpt6",
		},
		{
			name: "account exact mapping applies",
			account: &Account{
				CatalogUpstreamModels: map[string]string{
					"claude-fable-5": "gpt-5.5",
				},
			},
			requestedModel: "claude-fable-5",
			expectedModel:  "gpt-5.5",
		},
		{
			name: "account passthrough mapping keeps the requested model",
			account: &Account{
				CatalogUpstreamModels: map[string]string{
					"claude-fable-5": "claude-fable-5",
				},
			},
			requestedModel: "claude-fable-5",
			expectedModel:  "claude-fable-5",
		},
		{
			name: "ordinary codex spark request keeps requested model",
			account: &Account{
				Credentials: map[string]any{},
			},
			requestedModel: "gpt-5.3-codex-spark",
			expectedModel:  "gpt-5.3-codex-spark",
		},
		{
			name: "ordinary gpt-5.5 request keeps requested model",
			account: &Account{
				Credentials: map[string]any{},
			},
			requestedModel: "gpt-5.5",
			expectedModel:  "gpt-5.5",
		},
		{
			name: "ordinary gpt-5.5-pro request keeps requested model",
			account: &Account{
				Credentials: map[string]any{},
			},
			requestedModel: "gpt-5.5-pro",
			expectedModel:  "gpt-5.5-pro",
		},
		{
			name: "ordinary compact-spelled gpt5.5 request keeps requested model",
			account: &Account{
				Credentials: map[string]any{},
			},
			requestedModel: "gpt5.5",
			expectedModel:  "gpt5.5",
		},
		{
			name: "ordinary namespaced gpt-5.5 request keeps requested model",
			account: &Account{
				Credentials: map[string]any{},
			},
			requestedModel: "openai/gpt-5.5",
			expectedModel:  "openai/gpt-5.5",
		},
		{
			name: "ordinary compact gpt-5.5 request keeps requested model",
			account: &Account{
				Credentials: map[string]any{},
			},
			requestedModel: "gpt-5.5-openai-compact",
			expectedModel:  "gpt-5.5-openai-compact",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveOpenAIForwardModel(tt.account, tt.requestedModel); got != tt.expectedModel {
				t.Fatalf("resolveOpenAIForwardModel(...) = %q, want %q", got, tt.expectedModel)
			}
		})
	}
}

// 渠道级 compact 专属映射与 OpenAI 自动透传 2026-09-28 P5 都删了：库里残留的
// compact_model_mapping / openai_passthrough 不再影响 Forward 与调度的模型解析，一律走承接关系上的上游名；
// 计费按用户请求的目录模型。
func TestResolveOpenAIForwardMappedModels_IgnoresLegacyCompactMappingAndPassthrough(t *testing.T) {
	conflictingMappings := map[string]any{
		"compact_model_mapping": map[string]any{"gpt-5.5": "gpt-5.5-openai-compact"},
	}
	upstreamModels := map[string]string{"gpt-5.5": "gpt-5.4"}
	tests := []struct {
		name         string
		account      *Account
		wantBilling  string
		wantUpstream string
	}{
		{
			name: "legacy compact mapping no longer overrides ordinary mapping",
			account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth,
				Credentials: conflictingMappings, CatalogUpstreamModels: upstreamModels},
			wantBilling:  "gpt-5.5",
			wantUpstream: "gpt-5.4",
		},
		{
			name: "legacy passthrough key no longer bypasses ordinary mapping",
			account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth,
				Credentials: conflictingMappings, CatalogUpstreamModels: upstreamModels, Extra: map[string]any{"openai_passthrough": true}},
			wantBilling:  "gpt-5.5",
			wantUpstream: "gpt-5.4",
		},
		{
			name: "raw chat fallback uses ordinary mapping",
			account: &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
				Credentials: conflictingMappings, CatalogUpstreamModels: upstreamModels, ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://relay.example/v1"}},
			wantBilling:  "gpt-5.5",
			wantUpstream: "gpt-5.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			billing, upstream := resolveOpenAIForwardMappedModels(tt.account, "gpt-5.5")
			if billing != tt.wantBilling {
				t.Fatalf("billing model = %q, want %q", billing, tt.wantBilling)
			}
			if upstream != tt.wantUpstream {
				t.Fatalf("upstream model = %q, want %q", upstream, tt.wantUpstream)
			}
			if scheduler := resolveOpenAIAccountUpstreamModelForRequest(tt.account, "gpt-5.5"); scheduler != upstream {
				t.Fatalf("scheduler model %q disagrees with Forward model %q", scheduler, upstream)
			}
		})
	}
}

func TestCanonicalOpenAIAccountSchedulingModelMatchesForwardSemantics(t *testing.T) {
	tests := []struct {
		name    string
		account *Account
		model   string
		want    string
	}{
		{
			name:    "OpenAI OAuth applies Codex alias normalization",
			account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth},
			model:   "gpt-5.6",
			want:    "gpt-5.6-sol",
		},
		{
			name: "legacy OpenAI passthrough key no longer bypasses account mapping",
			account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth,
				CatalogUpstreamModels: map[string]string{"public": "private"},
				Extra:                 map[string]any{"openai_passthrough": true}},
			model: "public",
			want:  "private",
		},
		{
			name:    "Grok OAuth does not inherit OpenAI Codex aliases",
			account: &Account{Platform: PlatformGrok, Type: AccountTypeOAuth},
			model:   "gpt-5.6",
			want:    "gpt-5.6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canonicalOpenAIAccountSchedulingModel(tt.account, tt.model); got != tt.want {
				t.Fatalf("canonical scheduling model = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveOpenAIErrorSchedulingModelPrefersActualUpstreamModel(t *testing.T) {
	if got := resolveOpenAIErrorSchedulingModel("gpt-5.4", "gpt-5.5-openai-compact"); got != "gpt-5.5-openai-compact" {
		t.Fatalf("error scheduling model = %q, want compact upstream model", got)
	}
	if got := resolveOpenAIErrorSchedulingModel("gpt-5.4", ""); got != "gpt-5.4" {
		t.Fatalf("empty upstream fallback = %q, want billing model", got)
	}
}

func TestNormalizeCodexModel(t *testing.T) {
	cases := map[string]string{
		"gpt-5.3-codex-spark":       "gpt-5.3-codex-spark",
		"gpt-5.3-codex-spark-high":  "gpt-5.3-codex-spark",
		"gpt-5.3-codex-spark-xhigh": "gpt-5.3-codex-spark",
		"gpt-5.3":                   "gpt-5.3-codex",
		"gpt-image-2":               "gpt-image-2",
		"gpt-5.4-nano":              "gpt-5.4-nano",
		"gpt-5.4-nano-high":         "gpt-5.4-nano",
		"gpt6":                      "gpt6",
		"claude-opus-4-6":           "claude-opus-4-6",
	}

	for input, expected := range cases {
		if got := normalizeCodexModel(input); got != expected {
			t.Fatalf("normalizeCodexModel(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestNormalizeOpenAIModelForUpstream(t *testing.T) {
	tests := []struct {
		name    string
		account *Account
		model   string
		want    string
	}{
		{
			name:    "oauth routes bare GPT-5.6 alias to Sol",
			account: &Account{Type: AccountTypeOAuth},
			model:   "gpt-5.6",
			want:    "gpt-5.6-sol",
		},
		{
			name:    "oauth routes provider-prefixed GPT-5.6 alias to Sol",
			account: &Account{Type: AccountTypeOAuth},
			model:   "openai/gpt-5.6",
			want:    "gpt-5.6-sol",
		},
		{
			name:    "oauth preserves unknown non codex model",
			account: &Account{Type: AccountTypeOAuth},
			model:   "gemini-3-flash-preview",
			want:    "gemini-3-flash-preview",
		},
		{
			name:    "oauth preserves invalid gpt model",
			account: &Account{Type: AccountTypeOAuth},
			model:   "gpt6",
			want:    "gpt6",
		},
		{
			name:    "oauth normalizes known codex alias",
			account: &Account{Type: AccountTypeOAuth},
			model:   "gpt-5.4-high",
			want:    "gpt-5.4",
		},
		{
			name:    "oauth preserves GPT-5.5 Pro model",
			account: &Account{Type: AccountTypeOAuth},
			model:   "openai/gpt-5.5-pro",
			want:    "gpt-5.5-pro",
		},
		{
			name:    "oauth preserves codex auto review model",
			account: &Account{Type: AccountTypeOAuth},
			model:   "codex-auto-review",
			want:    "codex-auto-review",
		},
		{
			name:    "apikey preserves official bare GPT-5.6 alias",
			account: &Account{Type: AccountTypeAPIKey},
			model:   "gpt-5.6",
			want:    "gpt-5.6",
		},
		{
			name:    "apikey preserves custom compatible model",
			account: &Account{Type: AccountTypeAPIKey},
			model:   "gemini-3-flash-preview",
			want:    "gemini-3-flash-preview",
		},
		{
			name:    "apikey preserves official non codex model",
			account: &Account{Type: AccountTypeAPIKey},
			model:   "gpt-4.1",
			want:    "gpt-4.1",
		},
		{
			name:    "deepseek strips claude code long context suffix",
			account: &Account{Type: AccountTypeAPIKey, Platform: PlatformDeepseek, ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.deepseek.com/anthropic", APIProtocolChatCompletions: "https://api.deepseek.com", APIProtocolResponses: "https://api.deepseek.com"}},
			model:   "deepseek-flash[1m]",
			want:    "deepseek-flash",
		},
		{
			name:    "deepseek strips duplicated long context suffix",
			account: &Account{Type: AccountTypeAPIKey, Platform: PlatformDeepseek, ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.deepseek.com/anthropic", APIProtocolChatCompletions: "https://api.deepseek.com", APIProtocolResponses: "https://api.deepseek.com"}},
			model:   "deepseek-flash[1M][1m]",
			want:    "deepseek-flash",
		},
		{
			name:    "deepseek preserves plain model",
			account: &Account{Type: AccountTypeAPIKey, Platform: PlatformDeepseek, ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.deepseek.com/anthropic", APIProtocolChatCompletions: "https://api.deepseek.com", APIProtocolResponses: "https://api.deepseek.com"}},
			model:   "deepseek-flash",
			want:    "deepseek-flash",
		},
		{
			name:    "non deepseek preserves long context suffix",
			account: &Account{Type: AccountTypeAPIKey, Platform: PlatformOpenAI, ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"}},
			model:   "deepseek-flash[1m]",
			want:    "deepseek-flash[1m]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeOpenAIModelForUpstream(tt.account, tt.model); got != tt.want {
				t.Fatalf("normalizeOpenAIModelForUpstream(...) = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUsageBillingModelCandidatesPreserveCodexAutoReviewModel(t *testing.T) {
	candidates := usageBillingModelCandidates("codex-auto-review")

	expected := []string{"codex-auto-review"}
	if len(candidates) != len(expected) {
		t.Fatalf("usageBillingModelCandidates(codex-auto-review) = %#v, want %#v", candidates, expected)
	}
	for i := range expected {
		if candidates[i] != expected[i] {
			t.Fatalf("usageBillingModelCandidates(codex-auto-review) = %#v, want %#v", candidates, expected)
		}
	}
}

func TestUsageBillingModelCandidatesPreserveGPT55ProModel(t *testing.T) {
	candidates := usageBillingModelCandidates("openai/gpt-5.5-pro")

	expected := []string{"openai/gpt-5.5-pro", "gpt-5.5-pro"}
	if len(candidates) != len(expected) {
		t.Fatalf("usageBillingModelCandidates(openai/gpt-5.5-pro) = %#v, want %#v", candidates, expected)
	}
	for i := range expected {
		if candidates[i] != expected[i] {
			t.Fatalf("usageBillingModelCandidates(openai/gpt-5.5-pro) = %#v, want %#v", candidates, expected)
		}
	}
}
