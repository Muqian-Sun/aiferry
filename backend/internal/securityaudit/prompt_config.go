package securityaudit

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 提示词审计的运行参数写死在代码里（2026-10-05，瘦身方案 B2）：页面只留开关、同步阻止和风险分类，
// 守卫节点在部署配置里（prompt_audit.guard_endpoints / PROMPT_AUDIT_GUARD_ENDPOINTS）。
const (
	WorkerCount       = 4
	QueueCapacity     = 32768
	GuardTimeoutMS    = 3000
	GuardInputLimit   = 4000
	DefaultPayloadTTL = 30 * time.Minute
	// BlockingLatestTurnOnly 同步阻止时只审最新输入和上一轮输出，不把整段历史每轮重审一遍
	// （对话越长越慢；历史里有一句被判违规后每轮都会被拦）。2026-10-05 muqian 定写死开。
	BlockingLatestTurnOnly = true
	// StorePassEvents 判定为安全的请求不存事件，事件列表只留需要复核的
	StorePassEvents = false
)

type SecretEncryptor interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

// ConfigStore is the injectable boundary between hot-path prompt auditing and
// the concrete settings/PostgreSQL/Redis-backed configuration manager.
type ConfigStore interface {
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error
	Active() (ActiveConfig, bool)
	EffectiveMode() Mode
	// BlockingActivationDegraded is true when storage intent requires blocking
	// but no usable blocking snapshot is active (cold start or failed reload).
	// It must stay false when blocking is not intended, even if config is
	// untrusted—otherwise default-off deployments fail closed for all traffic.
	BlockingActivationDegraded() bool
	Public() (PublicConfig, error)
	Save(ctx context.Context, req UpdateConfigRequest, actorID int64) (PublicConfig, error)
	RuntimeState() (expected int64, active int64, loadedAt *time.Time, loadError string)
	Encrypt(value string) (string, error)
	Decrypt(value string) (string, error)
	// GuardEndpoints 部署配置里的守卫节点（与审计开关无关，未启用时管理页也要能看到、能测）
	GuardEndpoints() []ActiveEndpoint
}

// storageConfig 存在 prompt_audit_config 里、页面上能改的部分
type storageConfig struct {
	Enabled         bool      `json:"enabled"`
	BlockingEnabled bool      `json:"blocking_enabled"`
	Scanners        []string  `json:"scanners"`
	ConfigVersion   int64     `json:"config_version"`
	UpdatedAt       time.Time `json:"updated_at"`
	UpdatedBy       int64     `json:"updated_by"`
	ChangeSummary   string    `json:"change_summary"`
}

// ActiveEndpoint 一个守卫节点（OpenAI 兼容的 Qwen3Guard 服务），来自部署配置，按配置顺序优先、出错切下一个
type ActiveEndpoint struct {
	ID      string
	Name    string
	BaseURL string
	Model   string
	Token   string
}

type ActiveConfig struct {
	RiskControlEnabled bool
	Enabled            bool
	BlockingEnabled    bool
	Scanners           []string
	Endpoints          []ActiveEndpoint
	ConfigVersion      int64
	UpdatedAt          time.Time
	UpdatedBy          int64
	ChangeSummary      string
}

// PublicEndpoint 给管理页展示的守卫节点（只读；改节点要改部署配置），不含密钥
type PublicEndpoint struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	BaseURL  string `json:"base_url"`
	Model    string `json:"model"`
	HasToken bool   `json:"has_token"`
}

type PublicConfig struct {
	Enabled         bool             `json:"enabled"`
	BlockingEnabled bool             `json:"blocking_enabled"`
	EffectiveMode   Mode             `json:"effective_mode"`
	Scanners        []string         `json:"scanners"`
	Endpoints       []PublicEndpoint `json:"endpoints"`
	ConfigVersion   int64            `json:"config_version"`
	UpdatedAt       time.Time        `json:"updated_at"`
	UpdatedBy       int64            `json:"updated_by"`
	ChangeSummary   string           `json:"change_summary"`
}

type UpdateConfigRequest struct {
	ExpectedConfigVersion int64    `json:"expected_config_version" binding:"required"`
	Enabled               bool     `json:"enabled"`
	BlockingEnabled       bool     `json:"blocking_enabled"`
	Scanners              []string `json:"scanners"`
}

func DefaultStorageConfig() storageConfig {
	return storageConfig{
		Enabled:         false,
		BlockingEnabled: false,
		Scanners:        append([]string(nil), AllScannerIDs...),
		ConfigVersion:   1,
	}
}

func ParseStorageConfig(raw string) (storageConfig, error) {
	cfg := DefaultStorageConfig()
	if strings.TrimSpace(raw) == "" {
		return cfg, nil
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return storageConfig{}, fmt.Errorf("decode prompt audit config: %w", err)
	}
	normalizeStorageConfig(&cfg)
	if err := validateStorageConfig(cfg); err != nil {
		return storageConfig{}, err
	}
	return cfg, nil
}

func normalizeStorageConfig(cfg *storageConfig) {
	if cfg == nil {
		return
	}
	if cfg.ConfigVersion < 1 {
		cfg.ConfigVersion = 1
	}
	if len(cfg.Scanners) == 0 {
		cfg.Scanners = append([]string(nil), AllScannerIDs...)
	}
	cfg.Scanners = canonicalScannerIDs(cfg.Scanners)
}

func validateStorageConfig(cfg storageConfig) error {
	if cfg.BlockingEnabled && !cfg.Enabled {
		return infraerrors.BadRequest(ErrorCodeRequiresEnabled, "开启同步阻止前必须先启用提示词审计")
	}
	if len(cfg.Scanners) == 0 {
		return infraerrors.BadRequest("prompt_audit_scanners_required", "至少需要启用一个风险分类")
	}
	return nil
}

func validateUpdateConfigRequest(req UpdateConfigRequest, endpointCount int) error {
	if len(req.Scanners) == 0 {
		return infraerrors.BadRequest("prompt_audit_scanners_required", "至少需要启用一个风险分类")
	}
	for _, scanner := range req.Scanners {
		if _, ok := ScannerCatalog[NormalizeCategory(scanner)]; !ok {
			return infraerrors.BadRequest("prompt_audit_invalid_scanner", "提示词审计风险分类无效")
		}
	}
	if req.Enabled && endpointCount == 0 {
		return infraerrors.BadRequest("prompt_audit_endpoint_required", "部署配置里还没有守卫节点，不能启用提示词审计")
	}
	return nil
}

func (cfg ActiveConfig) EffectiveMode() Mode {
	if !cfg.RiskControlEnabled || !cfg.Enabled {
		return ModeOff
	}
	if cfg.BlockingEnabled {
		return ModeBlocking
	}
	return ModeAsync
}

func PublicFromStorage(cfg storageConfig, riskControlEnabled bool, endpoints []ActiveEndpoint) PublicConfig {
	public := make([]PublicEndpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		public = append(public, PublicEndpoint{
			ID: ep.ID, Name: ep.Name, BaseURL: ep.BaseURL, Model: ep.Model, HasToken: ep.Token != "",
		})
	}
	active := ActiveConfig{RiskControlEnabled: riskControlEnabled, Enabled: cfg.Enabled, BlockingEnabled: cfg.BlockingEnabled}
	return PublicConfig{
		Enabled: cfg.Enabled, BlockingEnabled: cfg.BlockingEnabled, EffectiveMode: active.EffectiveMode(),
		Scanners: append([]string{}, cfg.Scanners...), Endpoints: public, ConfigVersion: cfg.ConfigVersion,
		UpdatedAt: cfg.UpdatedAt, UpdatedBy: cfg.UpdatedBy, ChangeSummary: cfg.ChangeSummary,
	}
}

func ActiveFromStorage(cfg storageConfig, riskControlEnabled bool, endpoints []ActiveEndpoint) ActiveConfig {
	return ActiveConfig{
		RiskControlEnabled: riskControlEnabled, Enabled: cfg.Enabled, BlockingEnabled: cfg.BlockingEnabled,
		Scanners: append([]string(nil), cfg.Scanners...), ConfigVersion: cfg.ConfigVersion,
		UpdatedAt: cfg.UpdatedAt, UpdatedBy: cfg.UpdatedBy, ChangeSummary: cfg.ChangeSummary,
		Endpoints: append([]ActiveEndpoint(nil), endpoints...),
	}
}

func changeSummary(cfg storageConfig) string {
	summary := struct {
		Enabled         bool `json:"enabled"`
		BlockingEnabled bool `json:"blocking_enabled"`
		ScannerCount    int  `json:"scanner_count"`
	}{cfg.Enabled, cfg.BlockingEnabled, len(cfg.Scanners)}
	raw, _ := json.Marshal(summary)
	return string(raw)
}

func canonicalInt64s(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func canonicalScannerIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		id := NormalizeCategory(value)
		if _, ok := ScannerCatalog[id]; ok {
			seen[id] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for _, id := range AllScannerIDs {
		if _, ok := seen[id]; ok {
			result = append(result, id)
		}
	}
	return result
}
