package securityaudit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type prefixEncryptor struct{}

func (prefixEncryptor) Encrypt(value string) (string, error) { return "enc:" + value, nil }
func (prefixEncryptor) Decrypt(value string) (string, error) {
	if !strings.HasPrefix(value, "enc:") {
		return "", errors.New("cipher: message authentication failed")
	}
	return value[4:], nil
}

// testGuardConfig 部署配置里有一个守卫节点（guard-1）
func testGuardConfig(token string) *config.Config {
	return &config.Config{PromptAudit: config.PromptAuditConfig{GuardEndpoints: []config.PromptAuditGuardEndpoint{
		{Name: "Guard One", BaseURL: "http://127.0.0.1:18080/v1", APIKey: token},
	}}}
}

func TestDefaultConfigIsOff(t *testing.T) {
	storage, err := ParseStorageConfig("")
	require.NoError(t, err)
	require.False(t, storage.Enabled)
	active := ActiveFromStorage(storage, true, nil)
	require.Equal(t, ModeOff, active.EffectiveMode())
	require.Equal(t, AllScannerIDs, storage.Scanners)
	publicJSON, err := json.Marshal(PublicFromStorage(storage, true, nil))
	require.NoError(t, err)
	require.Contains(t, string(publicJSON), `"endpoints":[]`)
}

func TestConfigRejectsBlockingWithoutAudit(t *testing.T) {
	storage := DefaultStorageConfig()
	storage.BlockingEnabled = true
	require.Error(t, validateStorageConfig(storage))
}

func TestPublicConfigNeverMarshalsToken(t *testing.T) {
	storage := DefaultStorageConfig()
	public := PublicFromStorage(storage, true, []ActiveEndpoint{{ID: "guard-1", Name: "One", BaseURL: "http://127.0.0.1:8080", Model: DefaultGuardModel, Token: "GUARD_TOKEN_CANARY_SECRET"}})
	raw, err := json.Marshal(public)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "GUARD_TOKEN_CANARY_SECRET")
	require.NotContains(t, string(raw), "ciphertext")
	require.True(t, public.Endpoints[0].HasToken)
}

func TestConfigRuntimeLoadErrorIsStableBoundedAndSecretFree(t *testing.T) {
	const canary = "CONFIG_LOAD_CANARY_SECRET"
	manager := &ConfigManager{clock: fixedClock{}}
	manager.recordLoadError(errors.New("decrypt failed for token " + canary + " Authorization: Bearer " + canary))
	_, _, _, message := manager.RuntimeState()
	require.Equal(t, stableErrorMessage("config_load_failed"), message)
	require.NotContains(t, message, canary)
	require.LessOrEqual(t, len([]rune(message)), 160)
}

func TestConfigManagerPublicRequiresSuccessfullyLoadedSnapshot(t *testing.T) {
	t.Run("absent persisted setting is legitimate default", func(t *testing.T) {
		manager := NewConfigManager(nil, staticSettingRepository{values: map[string]string{
			SettingKeyPromptAuditConfig: "",
			SettingKeyRiskControl:       "false",
		}}, nil, prefixEncryptor{}, testGuardConfig(""))
		require.NoError(t, manager.Reload(context.Background()))

		public, err := manager.Public()
		require.NoError(t, err)
		require.Equal(t, int64(1), public.ConfigVersion)
		require.False(t, public.Enabled)
	})

	t.Run("unparseable persisted config is unavailable", func(t *testing.T) {
		const canary = "persisted-token-canary"
		manager := NewConfigManager(nil, staticSettingRepository{values: map[string]string{
			// A wrongly typed field cannot be decoded, so no trustworthy
			// snapshot can be installed from this raw value.
			SettingKeyPromptAuditConfig: `{"enabled":"` + canary + `","config_version":9}`,
			SettingKeyRiskControl:       "true",
		}}, nil, prefixEncryptor{}, testGuardConfig(""))
		require.Error(t, manager.Reload(context.Background()))

		public, err := manager.Public()
		require.Error(t, err)
		require.Empty(t, public)
		require.Equal(t, ErrorCodeConfigUnavailable, infraerrors.Reason(err))
		require.NotContains(t, err.Error(), canary)
	})

	t.Run("reload failure preserves last successfully loaded snapshot", func(t *testing.T) {
		storage := DefaultStorageConfig()
		storage.ConfigVersion = 4
		storage.ChangeSummary = "trusted snapshot"
		raw, err := json.Marshal(storage)
		require.NoError(t, err)
		repository := &switchableSettingRepository{staticSettingRepository: staticSettingRepository{values: map[string]string{
			SettingKeyPromptAuditConfig: string(raw),
			SettingKeyRiskControl:       "false",
		}}}
		manager := NewConfigManager(nil, repository, nil, prefixEncryptor{}, testGuardConfig(""))
		require.NoError(t, manager.Reload(context.Background()))
		repository.loadErr = errors.New("settings unavailable")
		require.Error(t, manager.Reload(context.Background()))

		public, err := manager.Public()
		require.NoError(t, err)
		require.Equal(t, int64(4), public.ConfigVersion)
		require.Equal(t, "trusted snapshot", public.ChangeSummary)
	})
}

func TestEffectiveModeTruthTable(t *testing.T) {
	tests := []struct {
		risk, enabled, blocking bool
		want                    Mode
	}{
		{false, false, false, ModeOff}, {false, true, true, ModeOff}, {true, false, false, ModeOff},
		{true, true, false, ModeAsync}, {true, true, true, ModeBlocking},
	}
	for _, tt := range tests {
		cfg := ActiveConfig{RiskControlEnabled: tt.risk, Enabled: tt.enabled, BlockingEnabled: tt.blocking}
		require.Equal(t, tt.want, cfg.EffectiveMode())
	}
}

func TestConfigManagerColdStartOnlyFailsClosedForExplicitBlockingIntent(t *testing.T) {
	manager := &ConfigManager{}

	manager.observeExpectedState(`{"enabled":true,"blocking_enabled":false,"config_version":42}`, true)
	require.Equal(t, int64(42), manager.expected.Load())
	require.Equal(t, ModeOff, manager.EffectiveMode(), "an async config version must not imply blocking")
	require.False(t, manager.BlockingActivationDegraded())

	manager.observeExpectedState(`{"enabled":true,"blocking_enabled":true,"config_version":43}`, false)
	require.Equal(t, ModeOff, manager.EffectiveMode(), "the global risk-control switch still gates blocking")

	manager.observeExpectedState(`{"enabled":true,"blocking_enabled":true,"config_version":44}`, true)
	require.Equal(t, ModeBlocking, manager.EffectiveMode())
	require.True(t, manager.BlockingActivationDegraded())

	manager.observeExpectedState(`{"enabled":true`, true)
	require.Equal(t, ModeBlocking, manager.EffectiveMode(), "undecodable storage must not erase the last known strict intent")
}

func TestConfigManagerStaleWeakerSnapshotFailsClosedWhenBlockingExpected(t *testing.T) {
	manager := &ConfigManager{}
	async := ActiveConfig{RiskControlEnabled: true, Enabled: true, BlockingEnabled: false, ConfigVersion: 1}
	manager.snapshot.Store(&activeConfigSnapshot{active: async, storage: DefaultStorageConfig(), loadedAt: fixedClock{}.Now()})
	manager.expected.Store(2)
	manager.expectedBlocking.Store(true)

	require.True(t, manager.BlockingActivationDegraded())
	require.Equal(t, ModeBlocking, manager.EffectiveMode())

	service := &PromptService{config: manager, evaluator: NewGuardEvaluator(nil, nil, nil)}
	decision, err := service.Evaluate(context.Background(), Request{Protocol: "openai_chat_completions", Body: []byte(`{"messages":[{"role":"user","content":"hi"}]}`)})
	require.Error(t, err)
	require.Nil(t, decision)
	var guardErr *GuardError
	require.ErrorAs(t, err, &guardErr)
	require.Equal(t, ErrorCodeUnavailable, guardErr.Code)
}

type errorSettingRepository struct{ staticSettingRepository }

func (errorSettingRepository) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, errors.New("settings unavailable")
}

type switchableSettingRepository struct {
	staticSettingRepository
	loadErr error
}

func (r *switchableSettingRepository) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	if r.loadErr != nil {
		return nil, r.loadErr
	}
	return r.staticSettingRepository.GetMultiple(ctx, keys)
}

func TestConfigManagerStartupLoadFailureDoesNotBlockWhenBlockingNotIntended(t *testing.T) {
	// Settings unavailable and no prior blocking intent: stay ModeOff so the
	// gateway remains usable and admins can still disable/configure Prompt Audit.
	manager := NewConfigManager(nil, errorSettingRepository{}, nil, prefixEncryptor{}, testGuardConfig(""))
	err := manager.Start(context.Background())
	require.Error(t, err)
	require.True(t, manager.configUntrusted.Load())
	require.False(t, manager.BlockingActivationDegraded())
	require.Equal(t, ModeOff, manager.EffectiveMode())

	service := &PromptService{config: manager, evaluator: NewGuardEvaluator(nil, nil, nil)}
	decision, evalErr := service.Evaluate(context.Background(), Request{
		Protocol: "openai_chat_completions",
		Body:     []byte(`{"messages":[{"role":"user","content":"hi"}]}`),
	})
	require.NoError(t, evalErr)
	require.NotNil(t, decision)
	require.Equal(t, DecisionAllow, decision.Kind)
	require.NoError(t, manager.Shutdown(context.Background()))
}

func TestConfigManagerStartupLoadFailureFailsClosedWhenBlockingIntended(t *testing.T) {
	manager := NewConfigManager(nil, errorSettingRepository{}, nil, prefixEncryptor{}, testGuardConfig(""))
	// Simulate intent observed before a later load failure (e.g. decrypt error).
	manager.observeExpectedState(`{"enabled":true,"blocking_enabled":true,"config_version":3}`, true)
	manager.markConfigUntrusted()
	require.True(t, manager.BlockingActivationDegraded())
	require.Equal(t, ModeBlocking, manager.EffectiveMode())

	service := &PromptService{config: manager, evaluator: NewGuardEvaluator(nil, nil, nil)}
	decision, err := service.Evaluate(context.Background(), Request{
		Protocol: "openai_chat_completions",
		Body:     []byte(`{"messages":[{"role":"user","content":"hi"}]}`),
	})
	require.Error(t, err)
	require.Nil(t, decision)
	var guardErr *GuardError
	require.ErrorAs(t, err, &guardErr)
	require.Equal(t, ErrorCodeUnavailable, guardErr.Code)
}

func TestConfigManagerUntrustedClearsOnSuccessfulDisable(t *testing.T) {
	// After a degraded fail-closed period, saving disabled config must restore ModeOff.
	manager := &ConfigManager{encryptor: prefixEncryptor{}, clock: fixedClock{}}
	manager.observeExpectedState(`{"enabled":true,"blocking_enabled":true,"config_version":5}`, true)
	manager.markConfigUntrusted()
	require.Equal(t, ModeBlocking, manager.EffectiveMode())

	// Install a trusted disabled snapshot the same way Save does after commit.
	disabled := DefaultStorageConfig()
	disabled.ConfigVersion = 6
	disabled.Enabled = false
	disabled.BlockingEnabled = false
	active := ActiveFromStorage(disabled, true, nil)
	manager.expected.Store(disabled.ConfigVersion)
	manager.expectedBlocking.Store(false)
	manager.snapshot.Store(&activeConfigSnapshot{storage: disabled, active: active, loadedAt: manager.clock.Now()})
	manager.configUntrusted.Store(false)

	require.False(t, manager.BlockingActivationDegraded())
	require.Equal(t, ModeOff, manager.EffectiveMode())

	service := &PromptService{config: manager, evaluator: NewGuardEvaluator(nil, nil, nil)}
	decision, evalErr := service.Evaluate(context.Background(), Request{
		Protocol: "openai_chat_completions",
		Body:     []byte(`{"messages":[{"role":"user","content":"hi"}]}`),
	})
	require.NoError(t, evalErr)
	require.Equal(t, DecisionAllow, decision.Kind)
}

func TestConfigManagerUntrustedWithoutBlockingDoesNotForceBlockingMode(t *testing.T) {
	manager := &ConfigManager{}
	manager.observeExpectedState(`{"enabled":true,"blocking_enabled":false,"config_version":2}`, true)
	manager.markConfigUntrusted()
	require.False(t, manager.expectedBlocking.Load())
	require.False(t, manager.BlockingActivationDegraded())
	require.Equal(t, ModeOff, manager.EffectiveMode(), "async intent + untrusted must not force blocking unavailable")
}

func TestParseLegacyConfigDefaultsMissingFieldsWithoutEnablingBlocking(t *testing.T) {
	storage, err := ParseStorageConfig(`{"enabled":false,"config_version":9}`)
	require.NoError(t, err)
	require.False(t, storage.BlockingEnabled)
	require.Equal(t, AllScannerIDs, storage.Scanners)
}

func TestUpdateConfigValidatesScannersAndEndpoints(t *testing.T) {
	valid := promptAuditUpdateRequest(1, false)
	require.NoError(t, validateUpdateConfigRequest(valid, 1))

	tests := []struct {
		name      string
		mutate    func(*UpdateConfigRequest)
		endpoints int
		reason    string
	}{
		{name: "unknown scanner", mutate: func(req *UpdateConfigRequest) { req.Scanners = []string{"made_up"} }, endpoints: 1, reason: "prompt_audit_invalid_scanner"},
		{name: "no scanner", mutate: func(req *UpdateConfigRequest) { req.Scanners = nil }, endpoints: 1, reason: "prompt_audit_scanners_required"},
		{name: "enable without guard endpoints", mutate: func(req *UpdateConfigRequest) {}, endpoints: 0, reason: "prompt_audit_endpoint_required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := valid
			req.Scanners = append([]string(nil), valid.Scanners...)
			tt.mutate(&req)
			err := validateUpdateConfigRequest(req, tt.endpoints)
			require.Error(t, err)
			require.Equal(t, tt.reason, infraerrors.Reason(err))
		})
	}

	// 没有守卫节点时仍可以保存「关闭」
	disabled := valid
	disabled.Enabled = false
	require.NoError(t, validateUpdateConfigRequest(disabled, 0))
}

// 部署配置里的守卫节点：按顺序编号 guard-N，地址去掉 /v1，没写名字用地址、没写模型用默认守卫模型
func TestGuardEndpointsFromConfig(t *testing.T) {
	endpoints := guardEndpointsFromConfig([]config.PromptAuditGuardEndpoint{
		{Name: " 主节点 ", BaseURL: "http://guard-a:8000/v1/", APIKey: " key-a ", Model: "Qwen3Guard-Gen-8B"},
		{BaseURL: "https://guard-b.example.com"},
	})
	require.Equal(t, []ActiveEndpoint{
		{ID: "guard-1", Name: "主节点", BaseURL: "http://guard-a:8000", Model: "Qwen3Guard-Gen-8B", Token: "key-a"},
		{ID: "guard-2", Name: "https://guard-b.example.com", BaseURL: "https://guard-b.example.com", Model: DefaultGuardModel},
	}, endpoints)
}

// Regression coverage for issue #5732: refreshLoop reloads every 5s, so
// config_loaded must stay a change signal instead of ~17k identical lines a
// day, while still reporting the first load, real config changes and a
// recovery from a failed reload.
func TestConfigLoadedIsLoggedOnlyWhenSomethingChanged(t *testing.T) {
	storage := DefaultStorageConfig()
	storage.ConfigVersion = 4
	raw, err := json.Marshal(storage)
	require.NoError(t, err)
	repository := &switchableSettingRepository{staticSettingRepository: staticSettingRepository{values: map[string]string{
		SettingKeyPromptAuditConfig: string(raw),
		SettingKeyRiskControl:       "false",
	}}}
	manager := NewConfigManager(nil, repository, nil, prefixEncryptor{}, testGuardConfig(""))

	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	loadedCount := func() int { return strings.Count(output.String(), EventConfigLoaded) }

	require.NoError(t, manager.Reload(context.Background()))
	require.Equal(t, 1, loadedCount(), "the first successful load must be logged")

	require.NoError(t, manager.Reload(context.Background()))
	require.NoError(t, manager.Reload(context.Background()))
	require.Equal(t, 1, loadedCount(), "TTL refreshes of an unchanged config must stay silent")

	repository.values[SettingKeyRiskControl] = "true"
	require.NoError(t, manager.Reload(context.Background()))
	require.Equal(t, 2, loadedCount(), "flipping the global risk control gate must be logged")

	storage.ConfigVersion = 5
	raw, err = json.Marshal(storage)
	require.NoError(t, err)
	repository.values[SettingKeyPromptAuditConfig] = string(raw)
	require.NoError(t, manager.Reload(context.Background()))
	require.Equal(t, 3, loadedCount(), "a new config version must be logged")

	repository.loadErr = errors.New("settings unavailable")
	require.Error(t, manager.Reload(context.Background()))
	require.Equal(t, 3, loadedCount(), "a failed reload must not claim a load")

	repository.loadErr = nil
	require.NoError(t, manager.Reload(context.Background()))
	require.Equal(t, 4, loadedCount(), "recovering from a failed reload must be visible")
}
