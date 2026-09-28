package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/websearch"
	"golang.org/x/sync/singleflight"
)

// WebSearchEmulationConfig holds the global web search emulation configuration.
// 后台只配服务商与 Key（2026-09-28 上线收口）：配了带 Key 的服务商就开，没有全局开关；
// 服务商不限次数、不走服务商级代理（搜索走渠道自己的代理，没有就直连）。
type WebSearchEmulationConfig struct {
	Providers []WebSearchProviderConfig `json:"providers"`
}

// WebSearchProviderConfig describes a single search provider (Brave or Tavily).
type WebSearchProviderConfig struct {
	Type             string `json:"type"`                 // websearch.ProviderTypeBrave | Tavily
	APIKey           string `json:"api_key,omitempty"`    // secret
	APIKeyConfigured bool   `json:"api_key_configured"`   // read-only mask
	ExpiresAt        *int64 `json:"expires_at,omitempty"` // optional expiration timestamp
}

// webSearchEmulationActive Web Search 模拟是否生效：有配了 Key 的服务商。
func webSearchEmulationActive(cfg *WebSearchEmulationConfig) bool {
	if cfg == nil {
		return false
	}
	for _, p := range cfg.Providers {
		if p.APIKey != "" {
			return true
		}
	}
	return false
}

// WebSearchEmulationAdminView 管理端读写接口的返回。
// enabled 只读 = 有配了 Key 的服务商（渠道表单据它决定显不显示渠道级「Web Search 模拟」开关）。
type WebSearchEmulationAdminView struct {
	Enabled   bool                      `json:"enabled"`
	Providers []WebSearchProviderConfig `json:"providers"`
}

// --- Validation ---

const maxWebSearchProviders = 10

var validProviderTypes = map[string]bool{
	websearch.ProviderTypeBrave:  true,
	websearch.ProviderTypeTavily: true,
}

func validateWebSearchConfig(cfg *WebSearchEmulationConfig) error {
	if cfg == nil {
		return nil
	}
	if len(cfg.Providers) > maxWebSearchProviders {
		return fmt.Errorf("too many providers (max %d)", maxWebSearchProviders)
	}
	seen := make(map[string]bool, len(cfg.Providers))
	for i, p := range cfg.Providers {
		if !validProviderTypes[p.Type] {
			return fmt.Errorf("provider[%d]: invalid type %q", i, p.Type)
		}
		if seen[p.Type] {
			return fmt.Errorf("provider[%d]: duplicate type %q", i, p.Type)
		}
		seen[p.Type] = true
	}
	return nil
}

// --- In-process cache (same pattern as gateway forwarding settings) ---

const sfKeyWebSearchConfig = "web_search_emulation_config"

type cachedWebSearchEmulationConfig struct {
	config    *WebSearchEmulationConfig
	expiresAt int64 // unix nano
}

var webSearchEmulationCache atomic.Value // *cachedWebSearchEmulationConfig
var webSearchEmulationSF singleflight.Group

const (
	webSearchEmulationCacheTTL  = 60 * time.Second
	webSearchEmulationErrorTTL  = 5 * time.Second
	webSearchEmulationDBTimeout = 5 * time.Second
)

// GetWebSearchEmulationConfig returns the configuration with in-process cache + singleflight.
func (s *SettingService) GetWebSearchEmulationConfig(ctx context.Context) (*WebSearchEmulationConfig, error) {
	if cached := webSearchEmulationCache.Load(); cached != nil {
		if c, ok := cached.(*cachedWebSearchEmulationConfig); ok && time.Now().UnixNano() < c.expiresAt {
			return c.config, nil
		}
	}
	result, err, _ := webSearchEmulationSF.Do(sfKeyWebSearchConfig, func() (any, error) {
		return s.loadWebSearchConfigFromDB()
	})
	if err != nil {
		return &WebSearchEmulationConfig{}, err
	}
	if cfg, ok := result.(*WebSearchEmulationConfig); ok {
		return cfg, nil
	}
	return &WebSearchEmulationConfig{}, nil
}

func (s *SettingService) loadWebSearchConfigFromDB() (*WebSearchEmulationConfig, error) {
	dbCtx, cancel := context.WithTimeout(context.Background(), webSearchEmulationDBTimeout)
	defer cancel()

	raw, err := s.settingRepo.GetValue(dbCtx, SettingKeyWebSearchEmulationConfig)
	if err != nil {
		// Missing key is the normal first-boot state: return empty config (no providers = off).
		if errors.Is(err, ErrSettingNotFound) {
			cfg := &WebSearchEmulationConfig{}
			webSearchEmulationCache.Store(&cachedWebSearchEmulationConfig{
				config:    cfg,
				expiresAt: time.Now().Add(webSearchEmulationCacheTTL).UnixNano(),
			})
			return cfg, nil
		}
		webSearchEmulationCache.Store(&cachedWebSearchEmulationConfig{
			config:    &WebSearchEmulationConfig{},
			expiresAt: time.Now().Add(webSearchEmulationErrorTTL).UnixNano(),
		})
		return &WebSearchEmulationConfig{}, err
	}
	cfg := parseWebSearchConfigJSON(raw)
	webSearchEmulationCache.Store(&cachedWebSearchEmulationConfig{
		config:    cfg,
		expiresAt: time.Now().Add(webSearchEmulationCacheTTL).UnixNano(),
	})
	return cfg, nil
}

func parseWebSearchConfigJSON(raw string) *WebSearchEmulationConfig {
	cfg := &WebSearchEmulationConfig{}
	if raw == "" {
		return cfg
	}
	if err := json.Unmarshal([]byte(raw), cfg); err != nil {
		slog.Warn("websearch: failed to parse config JSON", "error", err)
		return &WebSearchEmulationConfig{}
	}
	return cfg
}

// SaveWebSearchEmulationConfig validates and persists the configuration.
// Empty API keys in the input are preserved from the existing config.
func (s *SettingService) SaveWebSearchEmulationConfig(ctx context.Context, cfg *WebSearchEmulationConfig) error {
	if err := validateWebSearchConfig(cfg); err != nil {
		return infraerrors.BadRequest("INVALID_WEB_SEARCH_CONFIG", err.Error())
	}
	s.mergeExistingAPIKeys(ctx, cfg)

	// 没有全局开关，列出来的服务商就是要用的：合并后每个都得有 Key
	for _, p := range cfg.Providers {
		if p.APIKey == "" {
			return infraerrors.BadRequest("MISSING_API_KEY",
				fmt.Sprintf("provider %s has no API key configured", p.Type))
		}
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("websearch: marshal config: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyWebSearchEmulationConfig, string(data)); err != nil {
		return fmt.Errorf("websearch: save config: %w", err)
	}
	// Invalidate: forget singleflight first, then store new value
	webSearchEmulationSF.Forget(sfKeyWebSearchConfig)
	webSearchEmulationCache.Store(&cachedWebSearchEmulationConfig{
		config:    cfg,
		expiresAt: time.Now().Add(webSearchEmulationCacheTTL).UnixNano(),
	})

	// Hot-reload: rebuild the global Manager with new config
	s.rebuildWebSearchManager(ctx)
	return nil
}

// mergeExistingAPIKeys preserves API keys from the current config when incoming value is empty.
func (s *SettingService) mergeExistingAPIKeys(ctx context.Context, cfg *WebSearchEmulationConfig) {
	existing, _ := s.getWebSearchEmulationConfigRaw(ctx)
	if existing == nil || cfg == nil {
		return
	}
	existingByType := make(map[string]string, len(existing.Providers))
	for _, p := range existing.Providers {
		if p.APIKey != "" {
			existingByType[p.Type] = p.APIKey
		}
	}
	for i := range cfg.Providers {
		if cfg.Providers[i].APIKey == "" {
			if key, ok := existingByType[cfg.Providers[i].Type]; ok {
				cfg.Providers[i].APIKey = key
			}
		}
	}
}

func (s *SettingService) getWebSearchEmulationConfigRaw(ctx context.Context) (*WebSearchEmulationConfig, error) {
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyWebSearchEmulationConfig)
	if err != nil {
		return nil, err
	}
	return parseWebSearchConfigJSON(raw), nil
}

// IsWebSearchEmulationEnabled Web Search 模拟是否生效：有配了 Key 的服务商。
func (s *SettingService) IsWebSearchEmulationEnabled(ctx context.Context) bool {
	cfg, err := s.GetWebSearchEmulationConfig(ctx)
	if err != nil {
		return false
	}
	return webSearchEmulationActive(cfg)
}

// SetWebSearchManagerBuilder injects a callback that creates and wires a websearch.Manager.
// The infra layer (main/wire) provides this builder, keeping redis out of the service layer.
// Triggers initial build.
func (s *SettingService) SetWebSearchManagerBuilder(ctx context.Context, builder WebSearchManagerBuilder) {
	s.webSearchManagerBuilder = builder
	s.rebuildWebSearchManager(ctx)
}

// rebuildWebSearchManager reads the current config and invokes the builder.
func (s *SettingService) rebuildWebSearchManager(ctx context.Context) {
	if s.webSearchManagerBuilder == nil {
		return
	}
	cfg, err := s.GetWebSearchEmulationConfig(ctx)
	if err != nil {
		SetWebSearchManager(nil)
		return
	}
	s.webSearchManagerBuilder(cfg)
}

// WebSearchTestResult holds the result of a search test.
type WebSearchTestResult struct {
	Provider string                   `json:"provider"`
	Results  []websearch.SearchResult `json:"results"`
	Query    string                   `json:"query"`
}

// TestWebSearch executes a test search using the currently configured Manager (Manager.TestSearch).
const testSearchTimeout = 15 * time.Second

func TestWebSearch(ctx context.Context, query string) (*WebSearchTestResult, error) {
	mgr := getWebSearchManager()
	if mgr == nil {
		return nil, fmt.Errorf("web search: manager not initialized, save config first")
	}
	testCtx, cancel := context.WithTimeout(ctx, testSearchTimeout)
	defer cancel()
	resp, providerName, err := mgr.TestSearch(testCtx, websearch.SearchRequest{
		Query:      query,
		MaxResults: webSearchDefaultMaxResults,
	})
	if err != nil {
		return nil, err
	}
	return &WebSearchTestResult{
		Provider: providerName,
		Results:  resp.Results,
		Query:    resp.Query,
	}, nil
}

// WebSearchEmulationAdminViewOf 管理端返回：api_key 原样返回（管理端可以显示 / 复制），
// 标出每个服务商配没配 Key，并算出只读的 enabled。
func WebSearchEmulationAdminViewOf(cfg *WebSearchEmulationConfig) *WebSearchEmulationAdminView {
	providers := make([]WebSearchProviderConfig, len(cfg.Providers))
	for i, p := range cfg.Providers {
		providers[i] = p
		providers[i].APIKeyConfigured = p.APIKey != ""
	}
	return &WebSearchEmulationAdminView{Enabled: webSearchEmulationActive(cfg), Providers: providers}
}
