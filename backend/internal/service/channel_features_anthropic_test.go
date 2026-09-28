//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/pkg/websearch"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 2026-09-28 P5：Anthropic 成品号 / Bedrock / Vertex / Anthropic 协议 key 的渠道级开关删掉、取值写死
// （channel_features_anthropic.go）。这里的用例走真实转发 / 判定路径，断言写死后的行为，
// 并且库里存着旧值（包括原来会改变行为的非默认值）时也不再生效。

// tlsProfileRecorder 记下每次出站用的 TLS 指纹模板。
type tlsProfileRecorder struct {
	anthropicHTTPUpstreamRecorder
	profiles []*tlsfingerprint.Profile
}

func (u *tlsProfileRecorder) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	u.profiles = append(u.profiles, profile)
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func newChannelFeatureForwardFixture(respBody string, clientBeta string) (*gin.Context, *GatewayService, *tlsProfileRecorder) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	if clientBeta != "" {
		c.Request.Header.Set("Anthropic-Beta", clientBeta)
	}
	upstream := &tlsProfileRecorder{anthropicHTTPUpstreamRecorder: anthropicHTTPUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(respBody)),
	}}}
	cfg := &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}
	svc := &GatewayService{
		cfg:                  cfg,
		responseHeaderFilter: compileResponseHeaderFilter(cfg),
		httpUpstream:         upstream,
		rateLimitService:     &RateLimitService{},
		deferredService:      &DeferredService{},
	}
	return c, svc, upstream
}

func channelFeatureAnthropicKey(endpoint string, extra map[string]any) *Account {
	return &Account{
		ID: 9401, Name: "anthropic-protocol-key", Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials:       map[string]any{"api_key": "relay-key"},
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: endpoint},
		Extra:             extra, Status: StatusActive, Schedulable: true,
	}
}

func channelFeatureAnthropicOAuth(extra map[string]any) *Account {
	return &Account{
		ID: 9402, Name: "anthropic-oauth", Platform: PlatformAnthropic, Type: AccountTypeOAuth, Concurrency: 1,
		Credentials: map[string]any{"access_token": "oauth-token"},
		Extra:       extra, Status: StatusActive, Schedulable: true,
	}
}

const channelFeatureMessageResponse = `{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":3,"output_tokens":1}}`

// A2-16 自动透传写死关：库里存着 anthropic_passthrough=true 的 key 也走兼容链路。
// 兼容链路按 Beta 策略过滤 fast-mode（透传分支原样转发客户端的 anthropic-beta），并把过滤集写进上下文。
func TestAnthropicKey_StoredPassthroughStillUsesCompatChain(t *testing.T) {
	c, svc, upstream := newChannelFeatureForwardFixture(channelFeatureMessageResponse, "fast-mode-2026-02-01,interleaved-thinking-2025-05-14")
	body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	parsed := &ParsedRequest{Body: NewRequestBodyRef(body), Model: "claude-sonnet-4-5"}
	account := channelFeatureAnthropicKey("https://anthropic-relay.example.com", map[string]any{"anthropic_passthrough": true})

	result, err := svc.Forward(context.Background(), c, account, parsed)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)
	_, marked := c.Get("anthropic_passthrough")
	require.False(t, marked, "不再有透传分支")
	_, evaluated := c.Get(betaPolicyFilterSetKey)
	require.True(t, evaluated, "兼容链路评估了 Beta 策略")
	beta := getHeaderRaw(upstream.lastReq.Header, "anthropic-beta")
	require.False(t, anthropicBetaTokensContains(beta, "fast-mode-2026-02-01"), "fast-mode 被 Beta 策略过滤，got %q", beta)
	require.True(t, anthropicBetaTokensContains(beta, "interleaved-thinking-2025-05-14"), "其余客户端 beta 照常转发，got %q", beta)
}

// A2-29 / A2-30 TLS 指纹写死开、只用内置模板：Anthropic 成品号不看库里的开关与模板 ID；其它渠道不模拟。
func TestAnthropicSubscription_TLSFingerprintAlwaysBuiltin(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	cases := []struct {
		name    string
		account *Account
		want    string // 空 = 不模拟（profile 为 nil）
	}{
		{"oauth without stored switch", channelFeatureAnthropicOAuth(nil), builtinTLSFingerprintProfileName},
		{"oauth with stored off and profile id", channelFeatureAnthropicOAuth(map[string]any{"enable_tls_fingerprint": false, "tls_fingerprint_profile_id": 7}), builtinTLSFingerprintProfileName},
		{"oauth with stored random profile", channelFeatureAnthropicOAuth(map[string]any{"enable_tls_fingerprint": true, "tls_fingerprint_profile_id": -1}), builtinTLSFingerprintProfileName},
		{"third-party key with stored on", channelFeatureAnthropicKey("https://api.anthropic.com", map[string]any{"enable_tls_fingerprint": true}), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, svc, upstream := newChannelFeatureForwardFixture(channelFeatureMessageResponse, "")
			parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
			require.NoError(t, err)

			_, err = svc.Forward(context.Background(), c, tc.account, parsed)

			require.NoError(t, err)
			require.Len(t, upstream.profiles, 1)
			if tc.want == "" {
				require.Nil(t, upstream.profiles[0])
				return
			}
			require.NotNil(t, upstream.profiles[0])
			require.Equal(t, tc.want, upstream.profiles[0].Name)
			require.Empty(t, upstream.profiles[0].CipherSuites, "内置模板：参数留空由 dialer 用代码默认值")
		})
	}
}

// A2-32 / A2-33 缓存 TTL 强制替换写死关：库里存着「强制归 5m」时，上游报的 1h 缓存写入照原样计。
func TestAnthropicSubscription_StoredCacheTTLOverrideIgnored(t *testing.T) {
	resp := `{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":3,"output_tokens":1,"cache_creation_input_tokens":100,"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":100}}}`
	c, svc, _ := newChannelFeatureForwardFixture(resp, "")
	body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
	require.NoError(t, err)
	account := channelFeatureAnthropicOAuth(map[string]any{"cache_ttl_override_enabled": true, "cache_ttl_override_target": "5m"})

	result, err := svc.Forward(context.Background(), c, account, parsed)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 100, result.Usage.CacheCreation1hTokens)
	require.Equal(t, 0, result.Usage.CacheCreation5mTokens)
}

// A2-31 会话 ID 伪装写死关：库里存着开关、缓存里也有伪装 ID 时，metadata.user_id 里不会换成它。
func TestAnthropicSubscription_StoredSessionIDMaskingIgnored(t *testing.T) {
	masked := "11111111-2222-4333-8444-555555555555"
	svc := NewIdentityService(&identityCacheStub{maskedSessionID: masked})
	userID := FormatMetadataUserID("d61f76d0730d2b920763648949bad5c79742155c27037fc77ac3f9805cb90169", "", "7578cf37-aaca-46e4-a45c-71285d9dbb83", "2.1.78")
	body := []byte(`{"messages":[],"metadata":{"user_id":` + strconvQuote(userID) + `}}`)
	account := channelFeatureAnthropicOAuth(map[string]any{"session_id_masking_enabled": true})

	out, err := svc.RewriteUserIDWithMasking(context.Background(), body, account, "acc-uuid", "client-xyz", "claude-cli/2.1.78 (external, cli)")

	require.NoError(t, err)
	rewritten := gjson.GetBytes(out, "metadata.user_id").String()
	require.NotEmpty(t, rewritten)
	require.NotContains(t, rewritten, masked)
}

// sessionIdleRecorder 记下注册会话时用的空闲超时。
type sessionIdleRecorder struct {
	SessionLimitCache
	idleTimeouts []time.Duration
}

func (c *sessionIdleRecorder) RegisterSession(_ context.Context, _ int64, _ string, _ int, idleTimeout time.Duration) (bool, error) {
	c.idleTimeouts = append(c.idleTimeouts, idleTimeout)
	return true, nil
}

// A2-23 空闲超时写死 5 分钟：库里存着 11 分钟也按 5 分钟数活跃会话。
func TestAnthropicSubscription_SessionIdleTimeoutFixed(t *testing.T) {
	cache := &sessionIdleRecorder{}
	svc := &GatewayService{sessionLimitCache: cache}
	account := channelFeatureAnthropicOAuth(map[string]any{"max_sessions": 2, "session_idle_timeout_minutes": 11})

	require.True(t, svc.checkAndRegisterSession(context.Background(), account, "session-hash"))
	require.Equal(t, []time.Duration{5 * time.Minute}, cache.idleTimeouts)
}

// A2-25 / A2-26 RPM 策略写死 tiered、粘性缓冲写死自动：库里存着「粘性豁免」和手填缓冲 50 也按三区 + 自动缓冲判。
// base_rpm=10、并发 0、无会话上限 → 自动缓冲 = 10/5 = 2：10–11 只放粘性，12 起谁都不放。
func TestAnthropicSubscription_RPMTieredWithAutoBuffer(t *testing.T) {
	svc := &GatewayService{}
	account := channelFeatureAnthropicOAuth(map[string]any{"base_rpm": 10, "rpm_strategy": "sticky_exempt", "rpm_sticky_buffer": 50})
	at := func(rpm int) context.Context {
		return context.WithValue(context.Background(), rpmPrefetchContextKey, map[int64]int{account.ID: rpm})
	}

	require.True(t, svc.isAccountSchedulableForRPM(at(9), account, false), "绿区")
	require.False(t, svc.isAccountSchedulableForRPM(at(11), account, false), "黄区不放新会话")
	require.True(t, svc.isAccountSchedulableForRPM(at(11), account, true), "黄区放粘性")
	require.False(t, svc.isAccountSchedulableForRPM(at(12), account, true), "红区连粘性也不放（不按存的 50 算缓冲，也不豁免粘性）")
}

// A2-20 / A2-21 窗口费用阈值已删：库里存着 1 美元阈值、窗口里已用 5 美元，用量入账后也不停调。
func TestAnthropicSubscription_StoredWindowCostLimitIgnored(t *testing.T) {
	rl, repo := quotaStateTestService(t)
	usage := &windowStatsRepoStub{stats: &usagestats.AccountStats{StandardCost: 5}}
	rl.usageRepo = usage
	account := channelFeatureAnthropicOAuth(map[string]any{"window_cost_limit": 1.0, "window_cost_sticky_reserve": 0.5})

	rl.ApplyAccountUsageState(context.Background(), account, "claude-sonnet-4-5")

	require.Zero(t, repo.tempCalls)
	require.Zero(t, usage.calls, "不再为窗口费用聚合用量")
}

// A2-18 web_search 模拟：全站生效（配了带 Key 的服务商）时，第三方 key 且不是 Anthropic 官方地址就模拟，
// 不看渠道上存的开关；官方地址的 key、成品号、Bedrock 不模拟。
func TestWebSearchEmulation_AppliesToNonOfficialKeysRegardlessOfStoredSwitch(t *testing.T) {
	SetWebSearchManager(websearch.NewManager([]websearch.ProviderConfig{{Type: "brave", APIKey: "k"}}))
	defer SetWebSearchManager(nil)
	setGlobalWebSearchConfig(&WebSearchEmulationConfig{Providers: []WebSearchProviderConfig{{Type: "brave", APIKey: "k"}}})
	defer clearGlobalWebSearchConfig()
	svc := &GatewayService{settingService: newSettingServiceForWebSearchTest(true)}

	cases := []struct {
		name    string
		account *Account
		want    bool
	}{
		{"relay key without stored switch", channelFeatureAnthropicKey("https://anthropic-relay.example.com", nil), true},
		{"relay key with stored off", channelFeatureAnthropicKey("https://anthropic-relay.example.com", map[string]any{"web_search_emulation": false}), true},
		{"vendor key on its anthropic endpoint", channelFeatureAnthropicKey("https://api.moonshot.cn/anthropic", map[string]any{"web_search_emulation": false}), true},
		{"official anthropic key with stored on", channelFeatureAnthropicKey("https://api.anthropic.com", map[string]any{"web_search_emulation": true}), false},
		{"subscription with stored on", channelFeatureAnthropicOAuth(map[string]any{"web_search_emulation": true}), false},
		{"bedrock with stored on", &Account{ID: 9403, Platform: PlatformAnthropic, Type: AccountTypeBedrock, Extra: map[string]any{"web_search_emulation": true}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, svc.shouldEmulateWebSearch(context.Background(), tc.account, webSearchToolBody))
		})
	}

	// 没有配了 Key 的服务商 = 全站不生效，中转 key 也不模拟。
	setGlobalWebSearchConfig(&WebSearchEmulationConfig{Providers: []WebSearchProviderConfig{{Type: "brave"}}})
	noKey := &GatewayService{settingService: newSettingServiceForWebSearchTest(false)}
	require.False(t, noKey.shouldEmulateWebSearch(context.Background(), channelFeatureAnthropicKey("https://anthropic-relay.example.com", nil), webSearchToolBody))
}

// 同上，走 Forward：中转 key 的纯 web_search 请求被截下来做搜索（这里让渠道代理连不上，
// 搜索返回「代理不可用」→ 换渠道错误），上游没收到请求；官方地址的 key 照常发给上游。
func TestWebSearchEmulation_ForwardInterceptsRelayKeyOnly(t *testing.T) {
	SetWebSearchManager(websearch.NewManager([]websearch.ProviderConfig{{Type: "brave", APIKey: "k"}}))
	defer SetWebSearchManager(nil)
	setGlobalWebSearchConfig(&WebSearchEmulationConfig{Providers: []WebSearchProviderConfig{{Type: "brave", APIKey: "k"}}})
	defer clearGlobalWebSearchConfig()

	closed, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	deadAddr, ok := closed.Addr().(*net.TCPAddr)
	require.True(t, ok)
	deadPort := deadAddr.Port
	require.NoError(t, closed.Close())
	body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":16,"tools":[{"type":"web_search_20250305","name":"web_search"}],"messages":[{"role":"user","content":"latest news"}]}`)

	t.Run("relay key with stored off is intercepted", func(t *testing.T) {
		c, svc, upstream := newChannelFeatureForwardFixture(channelFeatureMessageResponse, "")
		svc.settingService = newSettingServiceForWebSearchTest(true)
		proxyID := int64(1)
		account := channelFeatureAnthropicKey("https://anthropic-relay.example.com", map[string]any{"web_search_emulation": false})
		account.ProxyID = &proxyID
		account.Proxy = &Proxy{ID: proxyID, Protocol: "http", Host: "127.0.0.1", Port: deadPort}

		_, err := svc.Forward(context.Background(), c, account, &ParsedRequest{Body: NewRequestBodyRef(body), Model: "claude-sonnet-4-5"})

		var failover *UpstreamFailoverError
		require.True(t, errors.As(err, &failover), "搜索走渠道代理失败 → 换渠道，got %v", err)
		require.Contains(t, string(failover.ResponseBody), "proxy unavailable")
		require.Nil(t, upstream.lastReq, "请求被模拟截下，没发给上游")
	})

	t.Run("official anthropic key goes upstream", func(t *testing.T) {
		c, svc, upstream := newChannelFeatureForwardFixture(channelFeatureMessageResponse, "")
		svc.settingService = newSettingServiceForWebSearchTest(true)
		account := channelFeatureAnthropicKey("https://api.anthropic.com", map[string]any{"web_search_emulation": true})

		_, err := svc.Forward(context.Background(), c, account, &ParsedRequest{Body: NewRequestBodyRef(body), Model: "claude-sonnet-4-5"})

		require.NoError(t, err)
		require.NotNil(t, upstream.lastReq)
		require.Equal(t, "https://api.anthropic.com/v1/messages?beta=true", upstream.lastReq.URL.String())
	})
}

// A2-3 / A2-4 Vertex Project ID 只从 Service Account JSON 取：库里另存的 project_id 副本不看。
func TestVertexServiceAccount_ProjectIDFromServiceAccountJSONOnly(t *testing.T) {
	c := newVertexBetaTestContext(t, "")
	account := newVertexServiceAccount(9404)
	account.Credentials["project_id"] = "stale-proj"
	account.Credentials["client_email"] = "stale@stale-proj.iam.gserviceaccount.com"
	body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)

	req, _, err := (&GatewayService{}).buildUpstreamRequest(context.Background(), c, account, body, "vertex-token", "service_account", "claude-sonnet-4-5@20250929", false, false)

	require.NoError(t, err)
	require.Contains(t, req.URL.String(), "/projects/vertex-proj/")
	require.NotContains(t, req.URL.String(), "stale-proj")
	require.Equal(t, "vertex-proj", account.VertexProjectID())
}

// A2-9 AWS Session Token 已删：库里存着也不带 X-Amz-Security-Token 签名。
func TestBedrockSigner_StoredSessionTokenIgnored(t *testing.T) {
	account := &Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock, Credentials: map[string]any{
		"aws_access_key_id": "AKIDEXAMPLE", "aws_secret_access_key": "secret", "aws_region": "us-east-1",
		"aws_session_token": "stale-session-token",
	}}
	signer, err := NewBedrockSignerFromAccount(account)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "https://bedrock-runtime.us-east-1.amazonaws.com/model/x/invoke", strings.NewReader(`{}`))

	require.NoError(t, signer.SignRequest(context.Background(), req, []byte(`{}`)))

	require.NotEmpty(t, req.Header.Get("Authorization"))
	require.Empty(t, req.Header.Get("X-Amz-Security-Token"))
}

// A2-13~15 Bedrock 池模式已删：库里存着 pool_mode=true 也不是池模式（不在同一渠道重试）。
func TestBedrock_StoredPoolModeIgnored(t *testing.T) {
	bedrock := &Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock, Credentials: map[string]any{
		"pool_mode": true, "pool_mode_retry_count": 5, "pool_mode_retry_status_codes": []any{float64(500)},
	}}
	require.False(t, bedrock.IsPoolMode())
	key := &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Credentials: map[string]any{"pool_mode": true}}
	require.True(t, key.IsPoolMode(), "第三方 key 的池模式保留")
}
