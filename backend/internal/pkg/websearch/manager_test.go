package websearch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewManager_PreservesOrder(t *testing.T) {
	configs := []ProviderConfig{
		{Type: "brave", APIKey: "k3"},
		{Type: "tavily", APIKey: "k1"},
	}
	m := NewManager(configs)
	require.Equal(t, "brave", m.configs[0].Type)
	require.Equal(t, "tavily", m.configs[1].Type)
}

func TestManager_SearchWithBestProvider_EmptyQuery(t *testing.T) {
	m := NewManager([]ProviderConfig{{Type: "brave", APIKey: "k"}})
	_, _, err := m.SearchWithBestProvider(context.Background(), SearchRequest{Query: ""})
	require.ErrorContains(t, err, "empty search query")

	_, _, err = m.SearchWithBestProvider(context.Background(), SearchRequest{Query: "   "})
	require.ErrorContains(t, err, "empty search query")
}

func TestManager_SearchWithBestProvider_SkipEmptyAPIKey(t *testing.T) {
	m := NewManager([]ProviderConfig{{Type: "brave", APIKey: ""}})
	_, _, err := m.SearchWithBestProvider(context.Background(), SearchRequest{Query: "test"})
	require.ErrorContains(t, err, "no available provider")
}

func TestManager_SearchWithBestProvider_SkipExpired(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour).Unix()
	m := NewManager([]ProviderConfig{
		{Type: "brave", APIKey: "k", ExpiresAt: &past},
	})
	_, _, err := m.SearchWithBestProvider(context.Background(), SearchRequest{Query: "test"})
	require.ErrorContains(t, err, "no available provider")
}

func TestManager_SearchWithBestProvider_UsesFirstAvailable(t *testing.T) {
	srvBrave := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := braveResponse{}
		resp.Web.Results = []braveResult{{URL: "https://brave.com", Title: "Brave", Description: "from brave"}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srvBrave.Close()

	origURL := *braveSearchURL
	u, _ := http.NewRequest("GET", srvBrave.URL, nil)
	*braveSearchURL = *u.URL
	defer func() { *braveSearchURL = origURL }()

	m := NewManager([]ProviderConfig{
		{Type: "brave", APIKey: "k1"},
		{Type: "tavily", APIKey: "k2"},
	})
	m.clientCache[srvBrave.URL] = srvBrave.Client()
	m.clientCache[""] = srvBrave.Client()

	resp, providerName, err := m.SearchWithBestProvider(context.Background(), SearchRequest{Query: "test"})
	require.NoError(t, err)
	require.Equal(t, "brave", providerName)
	require.Len(t, resp.Results, 1)
	require.Equal(t, "from brave", resp.Results[0].Snippet)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func stubHTTPResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
}

// 排在前面的服务商失败就按配置顺序换下一个。
func TestManager_SearchWithBestProvider_FailsOverToNextProvider(t *testing.T) {
	m := NewManager([]ProviderConfig{
		{Type: "brave", APIKey: "k1"},
		{Type: "tavily", APIKey: "k2"},
	})
	var hosts []string
	m.clientCache[""] = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		hosts = append(hosts, r.URL.Host)
		if r.URL.Host == "api.tavily.com" {
			return stubHTTPResponse(http.StatusOK, `{"results":[{"url":"https://t.example","title":"T","content":"from tavily"}]}`), nil
		}
		return stubHTTPResponse(http.StatusInternalServerError, `{}`), nil
	})}

	resp, providerName, err := m.SearchWithBestProvider(context.Background(), SearchRequest{Query: "test"})
	require.NoError(t, err)
	require.Equal(t, "tavily", providerName)
	require.Equal(t, "from tavily", resp.Results[0].Snippet)
	require.Equal(t, []string{"api.search.brave.com", "api.tavily.com"}, hosts)
}

// 带渠道代理的请求因代理 / 网络错误失败时不换服务商，返回 ErrProxyUnavailable 让上层换渠道。
func TestManager_SearchWithBestProvider_AccountProxyErrorAbortsFailover(t *testing.T) {
	const accountProxy = "http://proxy.example:8080"
	m := NewManager([]ProviderConfig{
		{Type: "brave", APIKey: "k1"},
		{Type: "tavily", APIKey: "k2"},
	})
	calls := 0
	m.clientCache[accountProxy] = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("proxyconnect tcp: dial tcp: connection refused")
	})}

	_, _, err := m.SearchWithBestProvider(context.Background(), SearchRequest{Query: "test", ProxyURL: accountProxy})
	require.ErrorIs(t, err, ErrProxyUnavailable)
	require.Equal(t, 1, calls)
}

// --- isProviderAvailable ---

func TestIsProviderAvailable_EmptyAPIKey(t *testing.T) {
	m := NewManager(nil)
	require.False(t, m.isProviderAvailable(ProviderConfig{APIKey: ""}))
}

func TestIsProviderAvailable_Expired(t *testing.T) {
	m := NewManager(nil)
	past := time.Now().Add(-1 * time.Hour).Unix()
	require.False(t, m.isProviderAvailable(ProviderConfig{APIKey: "k", ExpiresAt: &past}))
}

func TestIsProviderAvailable_Valid(t *testing.T) {
	m := NewManager(nil)
	future := time.Now().Add(1 * time.Hour).Unix()
	require.True(t, m.isProviderAvailable(ProviderConfig{APIKey: "k", ExpiresAt: &future}))
	require.True(t, m.isProviderAvailable(ProviderConfig{APIKey: "k"})) // no expiry
}

// --- isProxyError ---

func TestIsProxyError_Nil(t *testing.T) {
	require.False(t, isProxyError(nil))
}

func TestIsProxyError_ConnectionRefused(t *testing.T) {
	require.True(t, isProxyError(fmt.Errorf("dial tcp: connection refused")))
}

func TestIsProxyError_Timeout(t *testing.T) {
	require.True(t, isProxyError(fmt.Errorf("i/o timeout while connecting to proxy")))
}

func TestIsProxyError_SOCKS(t *testing.T) {
	require.True(t, isProxyError(fmt.Errorf("socks connect failed")))
}

func TestIsProxyError_TLSHandshake(t *testing.T) {
	require.True(t, isProxyError(fmt.Errorf("tls handshake timeout")))
}

func TestIsProxyError_APIError_NotProxy(t *testing.T) {
	require.False(t, isProxyError(fmt.Errorf("API rate limit exceeded")))
}

// --- newHTTPClient ---

func TestNewHTTPClient_NoProxy(t *testing.T) {
	c, err := newHTTPClient("")
	require.NoError(t, err)
	require.NotNil(t, c)
}

func TestNewHTTPClient_InvalidProxy(t *testing.T) {
	_, err := newHTTPClient("://bad-url")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid proxy URL")
}

func TestNewHTTPClient_ValidHTTPProxy(t *testing.T) {
	c, err := newHTTPClient("http://proxy.example.com:8080")
	require.NoError(t, err)
	require.NotNil(t, c)
}

func TestNewHTTPClient_ValidSOCKS5Proxy(t *testing.T) {
	c, err := newHTTPClient("socks5://proxy.example.com:1080")
	require.NoError(t, err)
	require.NotNil(t, c)
}
