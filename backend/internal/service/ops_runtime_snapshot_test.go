package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

var (
	benchmarkOpsMonitoringEnabled bool
	benchmarkOpsAdvancedSettings  OpsAdvancedSettings
)

type opsRuntimeRefreshRepo struct {
	SettingRepository
	mu     sync.RWMutex
	values map[string]string
	fail   atomic.Bool
	calls  atomic.Int64
}

func (r *opsRuntimeRefreshRepo) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	r.calls.Add(1)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if r.fail.Load() {
		return nil, errors.New("settings unavailable")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (r *opsRuntimeRefreshRepo) set(key, value string) {
	r.mu.Lock()
	r.values[key] = value
	r.mu.Unlock()
}

func waitForOpsRefresh(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition not satisfied before timeout")
}

func TestOpsRuntimeSettingsSnapshotLoadsOnceAndServesHotPath(t *testing.T) {
	repo := newRuntimeSettingRepoStub()
	repo.values["ops_monitoring_enabled"] = "false" // 旧软开关：运维监控只认 OPS_ENABLED，不读它
	repo.values[SettingKeyOpsAdvancedSettings] = `{"ignore_context_canceled":false,"auto_refresh_interval_seconds":45}`

	svc := &OpsService{settingRepo: repo}
	svc.initRuntimeSettings(context.Background())
	if repo.getMultipleCalls != 1 {
		t.Fatalf("startup GetMultiple calls = %d, want 1", repo.getMultipleCalls)
	}

	for range 1000 {
		if !svc.IsMonitoringEnabled(context.Background()) {
			t.Fatal("monitoring disabled by a database row, want it to follow OPS_ENABLED only")
		}
		cfg, err := svc.GetOpsAdvancedSettings(context.Background())
		if err != nil {
			t.Fatalf("GetOpsAdvancedSettings() error = %v", err)
		}
		if cfg.AutoRefreshIntervalSec != 45 {
			t.Fatalf("AutoRefreshIntervalSec = %d, want 45", cfg.AutoRefreshIntervalSec)
		}
	}
	if repo.getValueCalls != 0 || repo.getMultipleCalls != 1 {
		t.Fatalf("hot path touched repository: get=%d get_multiple=%d", repo.getValueCalls, repo.getMultipleCalls)
	}
}

func TestOpsRuntimeSettingsAdministrativeUpdatesAreImmediatelyVisible(t *testing.T) {
	svc := &OpsService{}
	svc.initRuntimeSettings(context.Background())

	cfg := defaultOpsAdvancedSettings()
	cfg.IgnoreNoAvailableAccounts = true
	svc.storeAdvancedSettingsSnapshot(cfg)
	got, err := svc.GetOpsAdvancedSettings(context.Background())
	if err != nil {
		t.Fatalf("GetOpsAdvancedSettings() error = %v", err)
	}
	if !got.IgnoreNoAvailableAccounts {
		t.Fatal("advanced settings update was not visible")
	}
}

// 运维监控只认部署配置 OPS_ENABLED：关了就关，库里旧的软开关开着也没用。
func TestOpsMonitoringFollowsDeploymentConfigOnly(t *testing.T) {
	repo := newRuntimeSettingRepoStub()
	repo.values["ops_monitoring_enabled"] = "true"

	disabled := &OpsService{settingRepo: repo, cfg: &config.Config{Ops: config.OpsConfig{Enabled: false}}}
	disabled.initRuntimeSettings(context.Background())
	if disabled.IsMonitoringEnabled(context.Background()) {
		t.Fatal("OPS_ENABLED=false must disable monitoring")
	}

	enabled := &OpsService{settingRepo: newRuntimeSettingRepoStub(), cfg: &config.Config{Ops: config.OpsConfig{Enabled: true}}}
	enabled.initRuntimeSettings(context.Background())
	if !enabled.IsMonitoringEnabled(context.Background()) {
		t.Fatal("OPS_ENABLED=true must enable monitoring")
	}
}

func TestOpsRuntimeSettingsBackgroundRefreshConverges(t *testing.T) {
	repo := &opsRuntimeRefreshRepo{values: map[string]string{
		SettingKeyOpsRuntimeLogConfig: `{"persist_access_logs":false}`,
	}}
	sink := &OpsSystemLogSink{}
	svc := &OpsService{settingRepo: repo, systemLogSink: sink}
	svc.initRuntimeSettings(context.Background())
	if sink.persistAccessLogs.Load() {
		t.Fatal("initial access-log persistence = true, want false")
	}

	repo.set(SettingKeyOpsRuntimeLogConfig, `{"persist_access_logs":true}`)
	svc.startRuntimeSettingsRefresh(context.Background(), 5*time.Millisecond, 0, 50*time.Millisecond)
	t.Cleanup(svc.StopRuntimeSettingsRefresh)
	waitForOpsRefresh(t, time.Second, func() bool {
		return sink.persistAccessLogs.Load() &&
			svc.RuntimeSettingsRefreshHealth().SuccessTotal > 0
	})
}

func TestOpsRuntimeSettingsRefreshUsesSafeAccessLogDefault(t *testing.T) {
	repo := &opsRuntimeRefreshRepo{values: map[string]string{
		SettingKeyOpsRuntimeLogConfig: `{"persist_access_logs":true}`,
	}}
	sink := &OpsSystemLogSink{}
	svc := &OpsService{settingRepo: repo, systemLogSink: sink}
	svc.initRuntimeSettings(context.Background())
	if !sink.persistAccessLogs.Load() {
		t.Fatal("valid runtime config did not enable access-log persistence")
	}

	repo.set(SettingKeyOpsRuntimeLogConfig, `{invalid-json}`)
	if err := svc.RefreshRuntimeSettings(context.Background()); err != nil {
		t.Fatalf("RefreshRuntimeSettings() error = %v", err)
	}
	if sink.persistAccessLogs.Load() {
		t.Fatal("invalid runtime config should fail closed for access-log persistence")
	}
}

func TestOpsRuntimeSettingsRefreshFailuresKeepLastKnownGoodSnapshot(t *testing.T) {
	repo := &opsRuntimeRefreshRepo{values: map[string]string{
		SettingKeyOpsAdvancedSettings: `{"ignore_no_available_accounts":true}`,
		SettingKeyOpsRuntimeLogConfig: `{"persist_access_logs":true}`,
	}}
	sink := &OpsSystemLogSink{}
	svc := &OpsService{settingRepo: repo, systemLogSink: sink}
	svc.initRuntimeSettings(context.Background())
	if !sink.persistAccessLogs.Load() {
		t.Fatal("initial access-log setting was not applied")
	}
	repo.fail.Store(true)
	svc.startRuntimeSettingsRefresh(context.Background(), 5*time.Millisecond, 0, 50*time.Millisecond)
	t.Cleanup(svc.StopRuntimeSettingsRefresh)
	waitForOpsRefresh(t, time.Second, func() bool {
		return svc.RuntimeSettingsRefreshHealth().FailureTotal >= 3
	})

	if !svc.OpsAdvancedSettingsSnapshot().IgnoreNoAvailableAccounts {
		t.Fatal("failed refresh overwrote last known advanced settings")
	}
	if !sink.persistAccessLogs.Load() {
		t.Fatal("failed refresh overwrote last known access-log setting")
	}
}

func TestOpsRuntimeSettingsRefreshStopEndsLifecycle(t *testing.T) {
	repo := &opsRuntimeRefreshRepo{values: map[string]string{}}
	svc := &OpsService{settingRepo: repo}
	svc.initRuntimeSettings(context.Background())
	svc.startRuntimeSettingsRefresh(context.Background(), 5*time.Millisecond, 0, 50*time.Millisecond)
	waitForOpsRefresh(t, time.Second, func() bool {
		return svc.RuntimeSettingsRefreshHealth().SuccessTotal > 0
	})

	svc.StopRuntimeSettingsRefresh()
	callsAfterStop := repo.calls.Load()
	time.Sleep(20 * time.Millisecond)
	if got := repo.calls.Load(); got != callsAfterStop {
		t.Fatalf("refresh continued after Stop: before=%d after=%d", callsAfterStop, got)
	}
	if svc.RuntimeSettingsRefreshHealth().Running {
		t.Fatal("refresh health still reports running after Stop")
	}
	// Idempotence is part of the cleanup contract.
	svc.StopRuntimeSettingsRefresh()
}

func BenchmarkOpsRuntimeSettingsSnapshotRead(b *testing.B) {
	svc := &OpsService{}
	svc.initRuntimeSettings(context.Background())
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		benchmarkOpsMonitoringEnabled = svc.IsMonitoringEnabled(ctx)
		benchmarkOpsAdvancedSettings = svc.OpsAdvancedSettingsSnapshot()
	}
}
