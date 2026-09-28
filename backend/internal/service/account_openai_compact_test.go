package service

import "testing"

func TestAccountOpenAICompactSupportKnown(t *testing.T) {
	tests := []struct {
		name          string
		account       *Account
		wantSupported bool
		wantKnown     bool
	}{
		{
			name:          "nil account is unknown",
			wantSupported: false,
			wantKnown:     false,
		},
		{
			name: "non openai account is unknown",
			account: &Account{
				Platform: PlatformAnthropic,
				Extra:    map[string]any{"openai_compact_supported": true},
			},
			wantSupported: false,
			wantKnown:     false,
		},
		// Compact 模式 2026-09-28 P5 写死 auto：库里残留的 force_on / force_off 不再覆盖探测结果。
		{
			name: "legacy force_on key no longer overrides probe state",
			account: &Account{
				Platform: PlatformOpenAI,
				Extra: map[string]any{
					"openai_compact_mode":      "force_on",
					"openai_compact_supported": false,
				},
			},
			wantSupported: false,
			wantKnown:     true,
		},
		{
			name: "legacy force_off key no longer overrides probe state",
			account: &Account{
				Platform: PlatformOpenAI,
				Extra: map[string]any{
					"openai_compact_mode":      "force_off",
					"openai_compact_supported": true,
				},
			},
			wantSupported: true,
			wantKnown:     true,
		},
		{
			name: "auto true is known supported",
			account: &Account{
				Platform: PlatformOpenAI,
				Extra:    map[string]any{"openai_compact_supported": true},
			},
			wantSupported: true,
			wantKnown:     true,
		},
		{
			name: "auto false is known unsupported",
			account: &Account{
				Platform: PlatformOpenAI,
				Extra:    map[string]any{"openai_compact_supported": false},
			},
			wantSupported: false,
			wantKnown:     true,
		},
		{
			name: "auto without probe state remains unknown",
			account: &Account{
				Platform: PlatformOpenAI,
				Extra:    map[string]any{},
			},
			wantSupported: false,
			wantKnown:     false,
		},
		{
			name: "invalid probe field remains unknown",
			account: &Account{
				Platform: PlatformOpenAI,
				Extra:    map[string]any{"openai_compact_supported": "true"},
			},
			wantSupported: false,
			wantKnown:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSupported, gotKnown := tt.account.OpenAICompactSupportKnown()
			if gotSupported != tt.wantSupported || gotKnown != tt.wantKnown {
				t.Fatalf("OpenAICompactSupportKnown() = (%v, %v), want (%v, %v)", gotSupported, gotKnown, tt.wantSupported, tt.wantKnown)
			}
		})
	}
}

func TestAccountAllowsOpenAICompact(t *testing.T) {
	tests := []struct {
		name    string
		account *Account
		want    bool
	}{
		{
			name: "nil account does not allow compact",
			want: false,
		},
		{
			name: "non openai account does not allow compact",
			account: &Account{
				Platform: PlatformAnthropic,
			},
			want: false,
		},
		{
			name: "unknown openai account remains allowed",
			account: &Account{
				Platform: PlatformOpenAI,
				Extra:    map[string]any{},
			},
			want: true,
		},
		{
			name: "supported openai account is allowed",
			account: &Account{
				Platform: PlatformOpenAI,
				Extra:    map[string]any{"openai_compact_supported": true},
			},
			want: true,
		},
		{
			name: "unsupported openai account is rejected",
			account: &Account{
				Platform: PlatformOpenAI,
				Extra:    map[string]any{"openai_compact_supported": false},
			},
			want: false,
		},
		{
			name: "legacy force_off key without probe state remains allowed",
			account: &Account{
				Platform: PlatformOpenAI,
				Extra:    map[string]any{"openai_compact_mode": "force_off"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.account.AllowsOpenAICompact(); got != tt.want {
				t.Fatalf("AllowsOpenAICompact() = %v, want %v", got, tt.want)
			}
		})
	}
}
