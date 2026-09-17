//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetWebSearchEmulationMode_Enabled(t *testing.T) {
	a := &Account{
		Platform:          PlatformAnthropic,
		Type:              AccountTypeAPIKey,
		Extra:             map[string]any{featureKeyWebSearchEmulation: "enabled"},
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"},
	}
	require.Equal(t, WebSearchModeEnabled, a.GetWebSearchEmulationMode())
}

func TestGetWebSearchEmulationMode_Disabled(t *testing.T) {
	a := &Account{
		Platform:          PlatformAnthropic,
		Type:              AccountTypeAPIKey,
		Extra:             map[string]any{featureKeyWebSearchEmulation: "disabled"},
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"},
	}
	require.Equal(t, WebSearchModeDisabled, a.GetWebSearchEmulationMode())
}

func TestGetWebSearchEmulationMode_Default(t *testing.T) {
	a := &Account{
		Platform:          PlatformAnthropic,
		Type:              AccountTypeAPIKey,
		Extra:             map[string]any{featureKeyWebSearchEmulation: "default"},
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"},
	}
	require.Equal(t, WebSearchModeDefault, a.GetWebSearchEmulationMode())
}

func TestGetWebSearchEmulationMode_UnknownString(t *testing.T) {
	a := &Account{
		Platform:          PlatformAnthropic,
		Type:              AccountTypeAPIKey,
		Extra:             map[string]any{featureKeyWebSearchEmulation: "unknown"},
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"},
	}
	require.Equal(t, WebSearchModeDefault, a.GetWebSearchEmulationMode())
}

func TestGetWebSearchEmulationMode_OldBoolTrue(t *testing.T) {
	a := &Account{
		Platform:          PlatformAnthropic,
		Type:              AccountTypeAPIKey,
		Extra:             map[string]any{featureKeyWebSearchEmulation: true},
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"},
	}
	// bool true → tolerant fallback → enabled (not default)
	require.Equal(t, WebSearchModeEnabled, a.GetWebSearchEmulationMode())
}

func TestGetWebSearchEmulationMode_OldBoolFalse(t *testing.T) {
	a := &Account{
		Platform:          PlatformAnthropic,
		Type:              AccountTypeAPIKey,
		Extra:             map[string]any{featureKeyWebSearchEmulation: false},
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"},
	}
	require.Equal(t, WebSearchModeDefault, a.GetWebSearchEmulationMode())
}

func TestGetWebSearchEmulationMode_NilAccount(t *testing.T) {
	var a *Account
	require.Equal(t, WebSearchModeDefault, a.GetWebSearchEmulationMode())
}

func TestGetWebSearchEmulationMode_NilExtra(t *testing.T) {
	a := &Account{
		Platform:          PlatformAnthropic,
		Type:              AccountTypeAPIKey,
		Extra:             nil,
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"},
	}
	require.Equal(t, WebSearchModeDefault, a.GetWebSearchEmulationMode())
}

func TestGetWebSearchEmulationMode_MissingField(t *testing.T) {
	a := &Account{
		Platform:          PlatformAnthropic,
		Type:              AccountTypeAPIKey,
		Extra:             map[string]any{},
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"},
	}
	require.Equal(t, WebSearchModeDefault, a.GetWebSearchEmulationMode())
}

// 账号级模式对任何展示标签的第三方 key 生效（模拟只在 Anthropic 协议转发路径上判定）。
func TestGetWebSearchEmulationMode_NonAnthropicLabelKey(t *testing.T) {
	a := &Account{
		Platform:          PlatformOpenAI,
		Type:              AccountTypeAPIKey,
		Extra:             map[string]any{featureKeyWebSearchEmulation: "enabled"},
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://anthropic-relay.example.com"},
	}
	require.Equal(t, WebSearchModeEnabled, a.GetWebSearchEmulationMode())
}

func TestGetWebSearchEmulationMode_NonAPIKeyType(t *testing.T) {
	a := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Extra:    map[string]any{featureKeyWebSearchEmulation: "enabled"},
	}
	require.Equal(t, WebSearchModeDefault, a.GetWebSearchEmulationMode())
}
