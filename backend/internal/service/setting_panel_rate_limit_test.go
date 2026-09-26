package service

import (
	"context"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type panelRateLimitSettingRepo struct {
	mu            sync.Mutex
	values        map[string]string
	getValueErr   error
	getValueCalls int
}

func (r *panelRateLimitSettingRepo) Get(_ context.Context, key string) (*Setting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.values[key]
	if !ok {
		return nil, ErrSettingNotFound
	}
	return &Setting{Key: key, Value: value}, nil
}

func (r *panelRateLimitSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.getValueCalls++
	if r.getValueErr != nil {
		return "", r.getValueErr
	}
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *panelRateLimitSettingRepo) Set(_ context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.values == nil {
		r.values = make(map[string]string)
	}
	r.values[key] = value
	return nil
}

func (r *panelRateLimitSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (r *panelRateLimitSettingRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.values == nil {
		r.values = make(map[string]string)
	}
	for key, value := range settings {
		r.values[key] = value
	}
	return nil
}

func (r *panelRateLimitSettingRepo) GetAll(_ context.Context) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]string, len(r.values))
	for key, value := range r.values {
		out[key] = value
	}
	return out, nil
}

func (r *panelRateLimitSettingRepo) Delete(_ context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.values, key)
	return nil
}

func newPanelRateLimitTestService(repo SettingRepository) *SettingService {
	return NewSettingService(repo, &config.Config{})
}

// 面板限流由代码决定（DefaultPanelRateLimitSettings）：库里旧配置不生效，也不读库。
func TestGetPanelRateLimitSettingsCachedIgnoresStoredSettings(t *testing.T) {
	repo := &panelRateLimitSettingRepo{values: map[string]string{
		"panel_rate_limit_settings": `{"enabled":false,"user_rpm":1,"heavy_rpm":1,"exempt_admin":false,"public_ip_rpm":1}`,
	}}
	svc := NewSettingService(repo, &config.Config{})

	got := svc.GetPanelRateLimitSettingsCached(context.Background())
	require.Equal(t, *DefaultPanelRateLimitSettings(), got)
	require.True(t, got.Enabled)
	require.Equal(t, 240, got.UserRPM)
	require.Equal(t, 60, got.HeavyRPM)
	require.True(t, got.ExemptAdmin)
	require.Equal(t, 300, got.PublicIPRPM)
	require.Zero(t, repo.getValueCalls)
}
