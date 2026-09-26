package service

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"sync"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"golang.org/x/sync/singleflight"
)

const (
	GrokDefaultBaseURLModeAPI     = "api"
	GrokDefaultBaseURLModeUSEast1 = "us-east-1"
	GrokDefaultBaseURLModeUSWest2 = "us-west-2"
	GrokDefaultBaseURLModeEUWest1 = "eu-west-1"
	GrokDefaultBaseURLModeCLI     = "cli"
)

// GrokDefaultBaseURLModes 返回全部 Grok 默认地址模式，与上面的常量放在一起维护。
func GrokDefaultBaseURLModes() []string {
	return []string{
		GrokDefaultBaseURLModeCLI,
		GrokDefaultBaseURLModeAPI,
		GrokDefaultBaseURLModeUSEast1,
		GrokDefaultBaseURLModeUSWest2,
		GrokDefaultBaseURLModeEUWest1,
	}
}

func normalizeGrokDefaultBaseURLMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case GrokDefaultBaseURLModeAPI:
		return GrokDefaultBaseURLModeAPI
	case GrokDefaultBaseURLModeUSEast1:
		return GrokDefaultBaseURLModeUSEast1
	case GrokDefaultBaseURLModeUSWest2:
		return GrokDefaultBaseURLModeUSWest2
	case GrokDefaultBaseURLModeEUWest1:
		return GrokDefaultBaseURLModeEUWest1
	case GrokDefaultBaseURLModeCLI:
		return GrokDefaultBaseURLModeCLI
	default:
		return GrokDefaultBaseURLModeCLI
	}
}

func GrokBaseURLForMode(mode string) string {
	switch normalizeGrokDefaultBaseURLMode(mode) {
	case GrokDefaultBaseURLModeAPI:
		return xai.DefaultBaseURL
	case GrokDefaultBaseURLModeUSEast1:
		return xai.DefaultUSEast1BaseURL
	case GrokDefaultBaseURLModeUSWest2:
		return xai.DefaultUSWest2BaseURL
	case GrokDefaultBaseURLModeEUWest1:
		return xai.DefaultEUWest1BaseURL
	default:
		return xai.DefaultCLIBaseURL
	}
}

func (s *SettingService) GetGrokDefaultBaseURLMode(ctx context.Context) string {
	if s == nil || s.settingRepo == nil {
		return GrokDefaultBaseURLModeCLI
	}
	dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), gatewayForwardingDBTimeout)
	defer cancel()
	raw, err := s.settingRepo.GetValue(dbCtx, SettingKeyGrokDefaultBaseURLMode)
	if err != nil {
		return GrokDefaultBaseURLModeCLI
	}
	return normalizeGrokDefaultBaseURLMode(raw)
}

func (s *SettingService) GetGrokDefaultBaseURL(ctx context.Context) string {
	return GrokBaseURLForMode(s.GetGrokDefaultBaseURLMode(ctx))
}

func (s *SettingService) ResolveGrokBaseURL(ctx context.Context, account *Account) string {
	def := xai.DefaultCLIBaseURL
	if s != nil {
		def = s.GetGrokDefaultBaseURL(ctx)
	}
	if account == nil {
		return def
	}
	return account.GetGrokBaseURLOr(def)
}

var (
	ErrRegistrationDisabled  = infraerrors.Forbidden("REGISTRATION_DISABLED", "registration is currently disabled")
	ErrSettingNotFound       = infraerrors.NotFound("SETTING_NOT_FOUND", "setting not found")
	ErrDefaultSubPlanInvalid = infraerrors.BadRequest(
		"DEFAULT_SUBSCRIPTION_PLAN_INVALID",
		"default subscription plan must exist",
	)
	ErrDefaultSubTooMany = infraerrors.BadRequest(
		"DEFAULT_SUB_TOO_MANY",
		"at most one default subscription per list",
	)
)

type SettingRepository interface {
	Get(ctx context.Context, key string) (*Setting, error)
	GetValue(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	GetMultiple(ctx context.Context, keys []string) (map[string]string, error)
	SetMultiple(ctx context.Context, settings map[string]string) error
	GetAll(ctx context.Context) (map[string]string, error)
	Delete(ctx context.Context, key string) error
}

// DefaultSubscriptionPlanReader validates plan references used by default subscriptions.
type DefaultSubscriptionPlanReader interface {
	GetByID(ctx context.Context, id int64) (*SubscriptionPlan, error)
}

// WebSearchManagerBuilder creates a websearch.Manager from config (injected by infra layer).
// proxyURLs maps proxy ID to resolved URL for provider-level proxy support.
type WebSearchManagerBuilder func(cfg *WebSearchEmulationConfig, proxyURLs map[int64]string)

// SettingService 系统设置服务
type SettingService struct {
	settingRepo                 SettingRepository
	defaultSubPlanReader        DefaultSubscriptionPlanReader
	proxyRepo                   ProxyRepository // for resolving websearch provider proxy URLs
	cfg                         *config.Config
	onUpdate                    func() // Callback when settings are updated (for cache invalidation)
	version                     string // Application version
	webSearchManagerBuilder     WebSearchManagerBuilder
	antigravityUAVersionCache   atomic.Value // *cachedAntigravityUserAgentVersion
	antigravityUAVersionSF      singleflight.Group
	openAICodexUACache          atomic.Value // *cachedOpenAICodexUserAgent
	openAICodexUASF             singleflight.Group
	openAICodexVersionCache     atomic.Value // *cachedOpenAICodexClientVersion
	openAICodexVersionSF        singleflight.Group
	codexRestrictionPolicyCache atomic.Value // *cachedCodexRestrictionPolicy
	codexRestrictionPolicySF    singleflight.Group

	cyberSessionBlockRuntimeCache atomic.Value // *cachedCyberSessionBlockRuntime
	cyberSessionBlockRuntimeSF    singleflight.Group

	// openAIQuotaAutoPauseSettingsCache holds the most recently observed quota auto-pause
	// settings. GetOpenAIQuotaAutoPauseSettings reads this atomic.Value on the request hot
	// path without ever blocking on the DB; when the cached entry expires, a background
	// goroutine refreshes it via openAIQuotaAutoPauseSettingsSF (stale-while-revalidate).
	// This per-service field also gives tests natural isolation — each SettingService
	// instance owns its own cache, no shared package-level state.
	openAIQuotaAutoPauseSettingsCache atomic.Value // *cachedOpenAIQuotaAutoPauseSettings
	openAIQuotaAutoPauseSettingsSF    singleflight.Group
	openAIAPIKeyHealthBreakerCache    atomic.Value // *cachedOpenAIAPIKeyHealthBreakerSettings

	channelMonitorRuntimeListenersMu sync.Mutex
	channelMonitorRuntimeListeners   []func()
}

type ProviderDefaultGrantSettings struct {
	Balance          float64
	Concurrency      int
	Subscriptions    []DefaultSubscriptionSetting
	GrantOnSignup    bool
	GrantOnFirstBind bool
}

type AuthSourceDefaultSettings struct {
	Email                        ProviderDefaultGrantSettings
	WeChat                       ProviderDefaultGrantSettings
	GitHub                       ProviderDefaultGrantSettings
	Google                       ProviderDefaultGrantSettings
	ForceEmailOnThirdPartySignup bool
}

type authSourceDefaultKeySet struct {
	balance          string
	concurrency      string
	subscriptions    string
	grantOnSignup    string
	grantOnFirstBind string
}

var (
	emailAuthSourceDefaultKeys = authSourceDefaultKeySet{
		balance:          SettingKeyAuthSourceDefaultEmailBalance,
		concurrency:      SettingKeyAuthSourceDefaultEmailConcurrency,
		subscriptions:    SettingKeyAuthSourceDefaultEmailSubscriptions,
		grantOnSignup:    SettingKeyAuthSourceDefaultEmailGrantOnSignup,
		grantOnFirstBind: SettingKeyAuthSourceDefaultEmailGrantOnFirstBind,
	}
	weChatAuthSourceDefaultKeys = authSourceDefaultKeySet{
		balance:          SettingKeyAuthSourceDefaultWeChatBalance,
		concurrency:      SettingKeyAuthSourceDefaultWeChatConcurrency,
		subscriptions:    SettingKeyAuthSourceDefaultWeChatSubscriptions,
		grantOnSignup:    SettingKeyAuthSourceDefaultWeChatGrantOnSignup,
		grantOnFirstBind: SettingKeyAuthSourceDefaultWeChatGrantOnFirstBind,
	}
	gitHubAuthSourceDefaultKeys = authSourceDefaultKeySet{
		balance:          SettingKeyAuthSourceDefaultGitHubBalance,
		concurrency:      SettingKeyAuthSourceDefaultGitHubConcurrency,
		subscriptions:    SettingKeyAuthSourceDefaultGitHubSubscriptions,
		grantOnSignup:    SettingKeyAuthSourceDefaultGitHubGrantOnSignup,
		grantOnFirstBind: SettingKeyAuthSourceDefaultGitHubGrantOnFirstBind,
	}
	googleAuthSourceDefaultKeys = authSourceDefaultKeySet{
		balance:          SettingKeyAuthSourceDefaultGoogleBalance,
		concurrency:      SettingKeyAuthSourceDefaultGoogleConcurrency,
		subscriptions:    SettingKeyAuthSourceDefaultGoogleSubscriptions,
		grantOnSignup:    SettingKeyAuthSourceDefaultGoogleGrantOnSignup,
		grantOnFirstBind: SettingKeyAuthSourceDefaultGoogleGrantOnFirstBind,
	}
)

const (
	defaultAuthSourceBalance     = 0
	defaultAuthSourceConcurrency = 5
	defaultWeChatConnectMode     = "open"
	defaultWeChatConnectScopes   = "snsapi_login"
	defaultWeChatConnectFrontend = "/auth/wechat/callback"
	defaultGitHubOAuthAuthorize  = "https://github.com/login/oauth/authorize"
	defaultGitHubOAuthToken      = "https://github.com/login/oauth/access_token"
	defaultGitHubOAuthUserInfo   = "https://api.github.com/user"
	defaultGitHubOAuthEmails     = "https://api.github.com/user/emails"
	defaultGitHubOAuthScopes     = "read:user user:email"
	defaultGitHubOAuthFrontend   = "/auth/oauth/callback"
	defaultGoogleOAuthAuthorize  = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultGoogleOAuthToken      = "https://oauth2.googleapis.com/token"
	defaultGoogleOAuthUserInfo   = "https://openidconnect.googleapis.com/v1/userinfo"
	defaultGoogleOAuthScopes     = "openid email profile"
	defaultGoogleOAuthFrontend   = "/auth/oauth/callback"
)

// NewSettingService 创建系统设置服务实例
func NewSettingService(settingRepo SettingRepository, cfg *config.Config) *SettingService {
	return &SettingService{
		settingRepo: settingRepo,
		cfg:         cfg,
	}
}

// SetDefaultSubscriptionPlanReader injects an optional plan reader for default subscription validation.
func (s *SettingService) SetDefaultSubscriptionPlanReader(reader DefaultSubscriptionPlanReader) {
	s.defaultSubPlanReader = reader
}

// SetProxyRepository injects a proxy repo for resolving websearch provider proxy URLs.
func (s *SettingService) SetProxyRepository(repo ProxyRepository) {
	s.proxyRepo = repo
}

// GetAllSettings 获取所有系统设置
func (s *SettingService) GetAllSettings(ctx context.Context) (*SystemSettings, error) {
	settings, err := s.settingRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all settings: %w", err)
	}

	return s.parseSettings(settings), nil
}

// SetOnUpdateCallback sets a callback function to be called when settings are updated
// This is used for cache invalidation (e.g., HTML cache in frontend server)
func (s *SettingService) SetOnUpdateCallback(callback func()) {
	s.onUpdate = callback
}

// SubscribeChannelMonitorRuntime registers a listener that is invoked after
// settings are successfully persisted (and process caches refreshed).
// Used by ChannelMonitorRunner / ChannelMonitorV2Aggregator for immediate
// mode flips without waiting for poll intervals.
func (s *SettingService) SubscribeChannelMonitorRuntime(listener func()) (unsubscribe func()) {
	if s == nil || listener == nil {
		return func() {}
	}
	s.channelMonitorRuntimeListenersMu.Lock()
	s.channelMonitorRuntimeListeners = append(s.channelMonitorRuntimeListeners, listener)
	idx := len(s.channelMonitorRuntimeListeners) - 1
	s.channelMonitorRuntimeListenersMu.Unlock()
	return func() {
		s.channelMonitorRuntimeListenersMu.Lock()
		defer s.channelMonitorRuntimeListenersMu.Unlock()
		if idx < 0 || idx >= len(s.channelMonitorRuntimeListeners) {
			return
		}
		s.channelMonitorRuntimeListeners[idx] = nil
	}
}

func (s *SettingService) notifyChannelMonitorRuntimeListeners() {
	if s == nil {
		return
	}
	s.channelMonitorRuntimeListenersMu.Lock()
	listeners := make([]func(), 0, len(s.channelMonitorRuntimeListeners))
	for _, l := range s.channelMonitorRuntimeListeners {
		if l != nil {
			listeners = append(listeners, l)
		}
	}
	s.channelMonitorRuntimeListenersMu.Unlock()
	for _, l := range listeners {
		func(fn func()) {
			defer func() {
				if recovered := recover(); recovered != nil {
					_ = recovered // keep settings path healthy
				}
			}()
			fn()
		}(l)
	}
}

// SetVersion sets the application version for injection into public settings
func (s *SettingService) SetVersion(version string) {
	s.version = version
}

// getStringOrDefault 获取字符串值或默认值
func (s *SettingService) getStringOrDefault(settings map[string]string, key, defaultValue string) string {
	if value, ok := settings[key]; ok && value != "" {
		return value
	}
	return defaultValue
}
