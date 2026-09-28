//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

// --- validateWebSearchConfig ---

func TestValidateWebSearchConfig_Nil(t *testing.T) {
	require.NoError(t, validateWebSearchConfig(nil))
}

func TestValidateWebSearchConfig_Valid(t *testing.T) {
	cfg := &WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{
			{Type: "brave"},
			{Type: "tavily"},
		},
	}
	require.NoError(t, validateWebSearchConfig(cfg))
}

func TestValidateWebSearchConfig_TooManyProviders(t *testing.T) {
	cfg := &WebSearchEmulationConfig{Providers: make([]WebSearchProviderConfig, 11)}
	for i := range cfg.Providers {
		cfg.Providers[i] = WebSearchProviderConfig{Type: "brave"}
	}
	err := validateWebSearchConfig(cfg)
	require.ErrorContains(t, err, "too many providers")
}

func TestValidateWebSearchConfig_InvalidType(t *testing.T) {
	cfg := &WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{{Type: "bing"}},
	}
	require.ErrorContains(t, validateWebSearchConfig(cfg), "invalid type")
}

func TestValidateWebSearchConfig_DuplicateType(t *testing.T) {
	cfg := &WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{
			{Type: "brave"},
			{Type: "brave"},
		},
	}
	require.ErrorContains(t, validateWebSearchConfig(cfg), "duplicate type")
}

// --- parseWebSearchConfigJSON ---

func TestParseWebSearchConfigJSON_ValidJSON(t *testing.T) {
	raw := `{"providers":[{"type":"brave","api_key":"sk-xxx"}]}`
	cfg := parseWebSearchConfigJSON(raw)
	require.Len(t, cfg.Providers, 1)
	require.Equal(t, "brave", cfg.Providers[0].Type)
	require.Equal(t, "sk-xxx", cfg.Providers[0].APIKey)
}

func TestParseWebSearchConfigJSON_EmptyString(t *testing.T) {
	cfg := parseWebSearchConfigJSON("")
	require.Empty(t, cfg.Providers)
}

func TestParseWebSearchConfigJSON_InvalidJSON(t *testing.T) {
	cfg := parseWebSearchConfigJSON("not{json")
	require.Empty(t, cfg.Providers)
}

// 库里存的旧配置还带着全局开关、配额、订阅时间、代理：解析照常，这些字段一律不看；
// 旧的 enabled=false 也不再关掉模拟——有带 Key 的服务商就开。
func TestParseWebSearchConfigJSON_BackwardCompatibility(t *testing.T) {
	raw := `{"enabled":false,"providers":[{"type":"brave","api_key":"k","priority":1,"quota_refresh_interval":"monthly","quota_limit":1000,"subscribed_at":1700000000,"quota_used":7,"proxy_id":3}]}`
	cfg := parseWebSearchConfigJSON(raw)
	require.Equal(t, &WebSearchEmulationConfig{Providers: []WebSearchProviderConfig{{Type: "brave", APIKey: "k"}}}, cfg)
	require.True(t, webSearchEmulationActive(cfg))
}

// --- 生效判定：有配了 Key 的服务商 ---

func TestWebSearchEmulationActive(t *testing.T) {
	require.False(t, webSearchEmulationActive(&WebSearchEmulationConfig{}), "没有服务商")
	require.False(t, webSearchEmulationActive(&WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{{Type: "brave"}},
	}), "服务商没配 Key")
	require.True(t, webSearchEmulationActive(&WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{{Type: "brave"}, {Type: "tavily", APIKey: "k"}},
	}), "有一个配了 Key 就开")
}

// 系统设置里的 web_search_emulation_enabled 与管理端配置接口的 enabled 同一口径。
func TestParseSettings_WebSearchEmulationEnabledMeansKeyedProvider(t *testing.T) {
	svc := NewSettingService(newMockSettingRepo(), &config.Config{})
	require.False(t, svc.parseSettings(map[string]string{}).WebSearchEmulationEnabled)
	require.False(t, svc.parseSettings(map[string]string{
		SettingKeyWebSearchEmulationConfig: `{"enabled":true,"providers":[{"type":"brave"}]}`,
	}).WebSearchEmulationEnabled, "旧的全局开关开着、但没有 Key：不生效")
	require.True(t, svc.parseSettings(map[string]string{
		SettingKeyWebSearchEmulationConfig: `{"enabled":false,"providers":[{"type":"brave","api_key":"k"}]}`,
	}).WebSearchEmulationEnabled, "旧的全局开关关着、但有 Key：生效")
}

// --- WebSearchEmulationAdminViewOf ---

func TestWebSearchEmulationAdminViewOf_APIKeyConfigured(t *testing.T) {
	cfg := &WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{
			{Type: "brave", APIKey: "sk-key"},
			{Type: "tavily", APIKey: ""},
		},
	}
	out := WebSearchEmulationAdminViewOf(cfg)
	require.True(t, out.Providers[0].APIKeyConfigured)
	require.Equal(t, "sk-key", out.Providers[0].APIKey, "管理端要能显示 / 复制 Key")
	require.False(t, out.Providers[1].APIKeyConfigured)
}

func TestWebSearchEmulationAdminViewOf_EnabledMeansKeyedProvider(t *testing.T) {
	require.False(t, WebSearchEmulationAdminViewOf(&WebSearchEmulationConfig{}).Enabled)
	require.False(t, WebSearchEmulationAdminViewOf(&WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{{Type: "brave"}},
	}).Enabled)
	require.True(t, WebSearchEmulationAdminViewOf(&WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{{Type: "brave", APIKey: "k"}},
	}).Enabled)
}

func TestWebSearchEmulationAdminViewOf_DoesNotMutateOriginal(t *testing.T) {
	cfg := &WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{{Type: "brave", APIKey: "secret"}},
	}
	_ = WebSearchEmulationAdminViewOf(cfg)
	require.Equal(t, "secret", cfg.Providers[0].APIKey)
	require.False(t, cfg.Providers[0].APIKeyConfigured)
}

// --- SaveWebSearchEmulationConfig ---

// 没有全局开关，列出来的服务商都得有 Key（留空沿用库里同类型服务商的 Key）；存下来的配置里没有开关、配额、代理。
func TestSaveWebSearchEmulationConfig_EveryProviderNeedsAPIKey(t *testing.T) {
	t.Cleanup(clearGlobalWebSearchConfig)
	repo := newMockSettingRepo()
	svc := NewSettingService(repo, &config.Config{})

	err := svc.SaveWebSearchEmulationConfig(context.Background(), &WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{{Type: "brave"}},
	})
	require.Error(t, err)
	require.Equal(t, "MISSING_API_KEY", infraerrors.Reason(err))
	require.NotContains(t, repo.data, SettingKeyWebSearchEmulationConfig)

	require.NoError(t, svc.SaveWebSearchEmulationConfig(context.Background(), &WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{{Type: "brave", APIKey: "k"}},
	}))
	require.JSONEq(t, `{"providers":[{"type":"brave","api_key":"k","api_key_configured":false}]}`, repo.data[SettingKeyWebSearchEmulationConfig])

	// 再次保存时 Key 留空：沿用库里的 Key
	require.NoError(t, svc.SaveWebSearchEmulationConfig(context.Background(), &WebSearchEmulationConfig{
		Providers: []WebSearchProviderConfig{{Type: "brave"}},
	}))
	require.JSONEq(t, `{"providers":[{"type":"brave","api_key":"k","api_key_configured":false}]}`, repo.data[SettingKeyWebSearchEmulationConfig])
}
