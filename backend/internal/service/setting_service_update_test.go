//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type settingUpdateRepoStub struct {
	updates        map[string]string
	setMultipleErr error
}

func (s *settingUpdateRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *settingUpdateRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *settingUpdateRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *settingUpdateRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *settingUpdateRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	s.updates = make(map[string]string, len(settings))
	for k, v := range settings {
		s.updates[k] = v
	}
	return s.setMultipleErr
}

func (s *settingUpdateRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *settingUpdateRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

type settingGetAllRepoStub struct {
	values map[string]string
}

func (s *settingGetAllRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *settingGetAllRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *settingGetAllRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *settingGetAllRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *settingGetAllRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *settingGetAllRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	out := make(map[string]string, len(s.values))
	for key, value := range s.values {
		out[key] = value
	}
	return out, nil
}

func (s *settingGetAllRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

type forwardedIPMigrationRepoStub struct {
	values         map[string]string
	updates        map[string]string
	getMultipleErr error
	setMultipleErr error
}

func (s *forwardedIPMigrationRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *forwardedIPMigrationRepoStub) GetValue(_ context.Context, key string) (string, error) {
	value, ok := s.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (s *forwardedIPMigrationRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *forwardedIPMigrationRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	if s.getMultipleErr != nil {
		return nil, s.getMultipleErr
	}
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func (s *forwardedIPMigrationRepoStub) SetMultiple(_ context.Context, values map[string]string) error {
	if s.setMultipleErr != nil {
		return s.setMultipleErr
	}
	s.updates = make(map[string]string, len(values))
	for key, value := range values {
		s.values[key] = value
		s.updates[key] = value
	}
	return nil
}

func (s *forwardedIPMigrationRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *forwardedIPMigrationRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func TestSettingService_AffiliateAdminRechargeSetting(t *testing.T) {
	t.Run("missing value defaults to disabled", func(t *testing.T) {
		svc := NewSettingService(&settingGetAllRepoStub{values: map[string]string{}}, &config.Config{})

		settings, err := svc.GetAllSettings(context.Background())
		require.NoError(t, err)
		require.False(t, settings.AdminRechargeRebateEnabled)
	})

	t.Run("explicit value is parsed", func(t *testing.T) {
		svc := NewSettingService(&settingGetAllRepoStub{values: map[string]string{
			SettingKeyAffiliateAdminRechargeEnabled: "true",
		}}, &config.Config{})

		settings, err := svc.GetAllSettings(context.Background())
		require.NoError(t, err)
		require.True(t, settings.AdminRechargeRebateEnabled)
	})

	t.Run("value is persisted", func(t *testing.T) {
		repo := &settingUpdateRepoStub{}
		svc := NewSettingService(repo, &config.Config{})

		err := svc.UpdateSettings(context.Background(), &SystemSettings{
			AdminRechargeRebateEnabled: true,
		})
		require.NoError(t, err)
		require.Equal(t, "true", repo.updates[SettingKeyAffiliateAdminRechargeEnabled])
	})
}

// D2：利润门只写最低毛利率一个键；超出 [0, ProfitControlRatioMax] 是客户端错误（400），不是 500。
func TestSettingService_UpdateSettings_ProfitControl(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{ProfitMinMargin: 0.3})
	require.NoError(t, err)
	require.Equal(t, "0.30000000", repo.updates[SettingKeyProfitMinMargin])

	for _, margin := range []float64{-0.1, ProfitControlRatioMax + 0.001} {
		repo = &settingUpdateRepoStub{}
		svc = NewSettingService(repo, &config.Config{})
		err = svc.UpdateSettings(context.Background(), &SystemSettings{ProfitMinMargin: margin})
		require.Error(t, err, "margin=%v", margin)
		require.Equal(t, "INVALID_PROFIT_CONTROL", infraerrors.Reason(err))
		require.Nil(t, repo.updates)
	}
}

func TestSettingService_PasskeyFollowsDeploymentConfig(t *testing.T) {
	cfg := &config.Config{WebAuthn: config.WebAuthnConfig{
		Enabled:   true,
		RPID:      "sub3.nebula-spaces.com",
		RPOrigins: []string{"https://sub3.nebula-spaces.com"},
	}}
	// 库里残留旧开关关着也不影响：配了部署就开
	configured := NewSettingService(&forwardedIPMigrationRepoStub{values: map[string]string{"passkey_enabled": "false"}}, cfg)
	require.True(t, configured.PasskeyEnabled())
	publicSettings, err := configured.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, publicSettings.PasskeyEnabled)

	// 没配部署：残留旧开关开着也不可用
	unconfigured := NewSettingService(&forwardedIPMigrationRepoStub{values: map[string]string{"passkey_enabled": "true"}}, &config.Config{})
	require.False(t, unconfigured.PasskeyEnabled())
}
