//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// 「只留管理站」模式由代码决定（BackendModeEnabled，site_features.go）：库里残留旧开关开着也不生效。
func TestIsBackendModeEnabled_IgnoresStaleSettingRow(t *testing.T) {
	repo := newMockSettingRepo()
	repo.data["backend_mode_enabled"] = "true"
	svc := NewSettingService(repo, &config.Config{})

	require.False(t, svc.IsBackendModeEnabled(context.Background()))
	public, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.False(t, public.BackendModeEnabled)
}

// bmUpdateRepoStub 记下 SetMultiple 写了什么；其他用例（如 setting_service_user_error_persist_test.go）复用。
type bmUpdateRepoStub struct {
	updates    map[string]string
	getValueFn func(ctx context.Context, key string) (string, error)
}

func (s *bmUpdateRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *bmUpdateRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if s.getValueFn == nil {
		panic("unexpected GetValue call")
	}
	return s.getValueFn(ctx, key)
}

func (s *bmUpdateRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *bmUpdateRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *bmUpdateRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	s.updates = make(map[string]string, len(settings))
	for k, v := range settings {
		s.updates[k] = v
	}
	return nil
}

func (s *bmUpdateRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *bmUpdateRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}
