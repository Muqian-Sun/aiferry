package websearch

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyutil"
)

// ProviderConfig holds the configuration for a single search provider.
// 服务商不限次数、不走服务商级代理（2026-09-28 上线收口：后台只留服务商与 Key）。
type ProviderConfig struct {
	Type      string `json:"type"`                 // ProviderTypeBrave | ProviderTypeTavily
	APIKey    string `json:"api_key"`              // secret
	ExpiresAt *int64 `json:"expires_at,omitempty"` // optional expiration (unix seconds)
}

// Manager 按配置顺序挑可用的服务商（有 Key、没过期）执行搜索，失败换下一个。
type Manager struct {
	configs []ProviderConfig

	clientMu    sync.Mutex
	clientCache map[string]*http.Client
}

// Timeout constants for proxy and search operations.
const (
	proxyDialTimeout     = 3 * time.Second  // proxy TCP connection timeout
	proxyTLSTimeout      = 3 * time.Second  // TLS handshake timeout
	searchDataTimeout    = 60 * time.Second // response data transfer timeout
	searchRequestTimeout = searchDataTimeout + proxyDialTimeout
	maxCachedClients     = 100
)

// ErrProxyUnavailable indicates the search failed due to a proxy connectivity issue.
// Callers may use this to trigger account switching instead of direct fallback.
var ErrProxyUnavailable = errors.New("websearch: proxy unavailable")

// NewManager creates a Manager with the given provider configs; provider order is preserved.
func NewManager(configs []ProviderConfig) *Manager {
	copied := make([]ProviderConfig, len(configs))
	copy(copied, configs)
	return &Manager{
		configs:     copied,
		clientCache: make(map[string]*http.Client),
	}
}

// SearchWithBestProvider 按配置顺序试可用的服务商，返回第一个成功的结果。
// 请求带了渠道代理（req.ProxyURL）且因代理 / 网络错误失败时不再换服务商：
// 同一个代理换服务商也没用，返回 ErrProxyUnavailable 让上层换渠道。
func (m *Manager) SearchWithBestProvider(ctx context.Context, req SearchRequest) (*SearchResponse, string, error) {
	if strings.TrimSpace(req.Query) == "" {
		return nil, "", fmt.Errorf("websearch: empty search query")
	}

	tried := false
	for _, cfg := range m.configs {
		if !m.isProviderAvailable(cfg) {
			continue
		}
		tried = true
		resp, err := m.executeSearch(ctx, cfg, req)
		if err != nil {
			if req.ProxyURL != "" && isProxyError(err) {
				slog.Warn("websearch: account proxy error, aborting failover",
					"provider", cfg.Type, "error", err)
				return nil, "", fmt.Errorf("%w: %s", ErrProxyUnavailable, err.Error())
			}
			slog.Warn("websearch: provider search failed",
				"provider", cfg.Type, "error", err)
			continue
		}
		return resp, cfg.Type, nil
	}
	if !tried {
		return nil, "", fmt.Errorf("websearch: no available provider (all expired or missing API key)")
	}
	return nil, "", fmt.Errorf("websearch: no available provider (all failed)")
}

func (m *Manager) isProviderAvailable(cfg ProviderConfig) bool {
	if cfg.APIKey == "" {
		return false
	}
	if cfg.ExpiresAt != nil && time.Now().Unix() > *cfg.ExpiresAt {
		slog.Info("websearch: provider expired, skipping",
			"provider", cfg.Type, "expires_at", *cfg.ExpiresAt)
		return false
	}
	return true
}

// isProxyError checks whether the error is likely caused by proxy or network connectivity
// (as opposed to an API-level error from the search provider).
func isProxyError(err error) bool {
	if err == nil {
		return false
	}
	// Network-level errors (timeout, connection refused, DNS failure)
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	// TLS handshake failures (often caused by proxy intercepting/blocking)
	var tlsErr *tls.RecordHeaderError
	if errors.As(err, &tlsErr) {
		return true
	}
	// String-based detection for wrapped errors
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "proxy") ||
		strings.Contains(msg, "socks") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "tls handshake") ||
		strings.Contains(msg, "certificate")
}

// --- Search execution ---

// TestSearch executes a search using the first available provider.
// Intended for admin test functionality only.
func (m *Manager) TestSearch(ctx context.Context, req SearchRequest) (*SearchResponse, string, error) {
	if strings.TrimSpace(req.Query) == "" {
		return nil, "", fmt.Errorf("websearch: empty search query")
	}
	for _, cfg := range m.configs {
		if !m.isProviderAvailable(cfg) {
			continue
		}
		resp, err := m.executeSearch(ctx, cfg, req)
		if err != nil {
			continue
		}
		return resp, cfg.Type, nil
	}
	return nil, "", fmt.Errorf("websearch: no available provider")
}

// executeSearch 用请求带的渠道代理（没有就直连）调服务商。
func (m *Manager) executeSearch(ctx context.Context, cfg ProviderConfig, req SearchRequest) (*SearchResponse, error) {
	client, err := m.getOrCreateHTTPClient(req.ProxyURL)
	if err != nil {
		return nil, fmt.Errorf("websearch: %w", err)
	}
	provider := m.buildProvider(cfg, client)
	return provider.Search(ctx, req)
}

// --- HTTP client cache ---

func (m *Manager) getOrCreateHTTPClient(proxyURL string) (*http.Client, error) {
	m.clientMu.Lock()
	defer m.clientMu.Unlock()

	if c, ok := m.clientCache[proxyURL]; ok {
		return c, nil
	}
	if len(m.clientCache) >= maxCachedClients {
		m.clientCache = make(map[string]*http.Client)
	}
	c, err := newHTTPClient(proxyURL)
	if err != nil {
		return nil, err
	}
	m.clientCache[proxyURL] = c
	return c, nil
}

// newHTTPClient creates an HTTP client with proper timeout settings.
// Uses proxyutil.ConfigureTransportProxy for unified proxy protocol support
// (HTTP/HTTPS/SOCKS5/SOCKS5H).
// Returns error if proxyURL is invalid — never falls back to direct connection.
func newHTTPClient(proxyURL string) (*http.Client, error) {
	transport := &http.Transport{
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		DialContext:           (&net.Dialer{Timeout: proxyDialTimeout}).DialContext,
		TLSHandshakeTimeout:   proxyTLSTimeout,
		ResponseHeaderTimeout: searchDataTimeout,
	}
	if proxyURL != "" {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL %q: %w", proxyURL, err)
		}
		if err := proxyutil.ConfigureTransportProxy(transport, parsed); err != nil {
			return nil, fmt.Errorf("configure proxy: %w", err)
		}
	}
	return &http.Client{Transport: transport, Timeout: searchRequestTimeout}, nil
}

// --- Provider factory ---

func (m *Manager) buildProvider(cfg ProviderConfig, client *http.Client) Provider {
	switch cfg.Type {
	case braveProviderName:
		return NewBraveProvider(cfg.APIKey, client)
	case tavilyProviderName:
		return NewTavilyProvider(cfg.APIKey, client)
	default:
		slog.Warn("websearch: unknown provider type, falling back to brave",
			"type", cfg.Type)
		return NewBraveProvider(cfg.APIKey, client)
	}
}
