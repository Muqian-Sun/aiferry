//go:build unit

package service

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

// captureSlog 把默认 slog 输出接到 buffer，测试结束恢复。
func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// 账号 extra 是开关的唯一来源：bool 原样；历史字符串只有 "enabled" 算开，且要打 warn；
// 其它类型 fail-closed 且打 warn。
func TestWebSearchEmulationEnabled_ExtraValues(t *testing.T) {
	tests := []struct {
		name     string
		extra    map[string]any
		want     bool
		wantWarn string // 期望日志里出现的片段；空 = 不该有日志
	}{
		{name: "bool true", extra: map[string]any{featureKeyWebSearchEmulation: true}, want: true},
		{name: "bool false", extra: map[string]any{featureKeyWebSearchEmulation: false}, want: false},
		{name: "legacy enabled string", extra: map[string]any{featureKeyWebSearchEmulation: "enabled"}, want: true, wantWarn: "legacy string value"},
		{name: "legacy default string", extra: map[string]any{featureKeyWebSearchEmulation: "default"}, want: false, wantWarn: "legacy string value"},
		{name: "legacy disabled string", extra: map[string]any{featureKeyWebSearchEmulation: "disabled"}, want: false, wantWarn: "legacy string value"},
		{name: "unknown string", extra: map[string]any{featureKeyWebSearchEmulation: "unknown"}, want: false, wantWarn: "legacy string value"},
		{name: "number", extra: map[string]any{featureKeyWebSearchEmulation: 1}, want: false, wantWarn: "non-bool value treated as off"},
		{name: "nil value", extra: map[string]any{featureKeyWebSearchEmulation: nil}, want: false},
		{name: "missing key", extra: map[string]any{}, want: false},
		{name: "nil extra", extra: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := captureSlog(t)
			a := &Account{
				ID:                7,
				Platform:          PlatformAnthropic,
				Type:              AccountTypeAPIKey,
				Extra:             tt.extra,
				ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"},
			}
			require.Equal(t, tt.want, a.WebSearchEmulationEnabled())
			if tt.wantWarn == "" {
				require.Empty(t, logs.String())
			} else {
				require.Contains(t, logs.String(), "level=WARN")
				require.Contains(t, logs.String(), tt.wantWarn)
				require.Contains(t, logs.String(), "account_id=7")
			}
		})
	}
}

func TestWebSearchEmulationEnabled_NilAccount(t *testing.T) {
	var a *Account
	require.False(t, a.WebSearchEmulationEnabled())
}

// 开关对任何展示标签的第三方 key 生效（模拟只在 Anthropic 协议转发路径上判定）。
func TestWebSearchEmulationEnabled_NonAnthropicLabelKey(t *testing.T) {
	a := &Account{
		Platform:          PlatformOpenAI,
		Type:              AccountTypeAPIKey,
		Extra:             map[string]any{featureKeyWebSearchEmulation: true},
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://anthropic-relay.example.com"},
	}
	require.True(t, a.WebSearchEmulationEnabled())
}

// 成品号恒关，即使 extra 里有值。
func TestWebSearchEmulationEnabled_NonAPIKeyType(t *testing.T) {
	a := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Extra:    map[string]any{featureKeyWebSearchEmulation: true},
	}
	require.False(t, a.WebSearchEmulationEnabled())
}
