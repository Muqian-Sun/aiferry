//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
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

type settingAntigravityUARepoStub struct {
	values map[string]string
}

func (s *settingAntigravityUARepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *settingAntigravityUARepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (s *settingAntigravityUARepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *settingAntigravityUARepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *settingAntigravityUARepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *settingAntigravityUARepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *settingAntigravityUARepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

type defaultSubPlanReaderStub struct {
	byID  map[int64]*SubscriptionPlan
	errBy map[int64]error
	calls []int64
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

func (s *defaultSubPlanReaderStub) GetByID(ctx context.Context, id int64) (*SubscriptionPlan, error) {
	s.calls = append(s.calls, id)
	if err, ok := s.errBy[id]; ok {
		return nil, err
	}
	if g, ok := s.byID[id]; ok {
		return g, nil
	}
	return nil, ErrPlanNotFound
}

func TestSettingService_UpdateSettings_DefaultSubscriptions_ValidPlan(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	planReader := &defaultSubPlanReaderStub{
		byID: map[int64]*SubscriptionPlan{
			11: {ID: 11, Name: "Pro"},
		},
	}
	svc := NewSettingService(repo, &config.Config{})
	svc.SetDefaultSubscriptionPlanReader(planReader)

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		DefaultSubscriptions: []DefaultSubscriptionSetting{
			{PlanID: 11, ValidityDays: 30},
		},
	})
	require.NoError(t, err)
	require.Equal(t, []int64{11}, planReader.calls)

	raw, ok := repo.updates[SettingKeyDefaultSubscriptions]
	require.True(t, ok)

	var got []DefaultSubscriptionSetting
	require.NoError(t, json.Unmarshal([]byte(raw), &got))
	require.Equal(t, []DefaultSubscriptionSetting{
		{PlanID: 11, ValidityDays: 30},
	}, got)
}

func TestSettingService_UpdateSettings_DefaultSubscriptions_RejectsNotFoundPlan(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	planReader := &defaultSubPlanReaderStub{
		errBy: map[int64]error{
			13: ErrPlanNotFound,
		},
	}
	svc := NewSettingService(repo, &config.Config{})
	svc.SetDefaultSubscriptionPlanReader(planReader)

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		DefaultSubscriptions: []DefaultSubscriptionSetting{
			{PlanID: 13, ValidityDays: 7},
		},
	})
	require.Error(t, err)
	require.Equal(t, "DEFAULT_SUBSCRIPTION_PLAN_INVALID", infraerrors.Reason(err))
	require.Equal(t, "13", infraerrors.FromError(err).Metadata["plan_id"])
	require.Nil(t, repo.updates)
}

// 同一时间只允许一条有效订阅：默认订阅列表最多一项，多了直接拒（不看套餐读者）。
func TestSettingService_UpdateSettings_DefaultSubscriptions_RejectsMoreThanOne(t *testing.T) {
	for _, withReader := range []bool{true, false} {
		repo := &settingUpdateRepoStub{}
		svc := NewSettingService(repo, &config.Config{})
		planReader := &defaultSubPlanReaderStub{byID: map[int64]*SubscriptionPlan{11: {ID: 11}, 12: {ID: 12}}}
		if withReader {
			svc.SetDefaultSubscriptionPlanReader(planReader)
		}

		err := svc.UpdateSettings(context.Background(), &SystemSettings{
			DefaultSubscriptions: []DefaultSubscriptionSetting{
				{PlanID: 11, ValidityDays: 30},
				{PlanID: 12, ValidityDays: 60},
			},
		})
		require.Error(t, err)
		require.Equal(t, "DEFAULT_SUB_TOO_MANY", infraerrors.Reason(err))
		require.Nil(t, repo.updates)
		require.Empty(t, planReader.calls, "数量校验在读套餐之前")
	}
}

// D2：利润门三键写入；margin + buffer 超过 ProfitControlRatioMax 是客户端错误（400），不是 500。
func TestSettingService_UpdateSettings_ProfitControl(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		ProfitControlEnabled: true, ProfitMinMargin: 0.3, ProfitSafetyBuffer: 0.05,
	})
	require.NoError(t, err)
	require.Equal(t, "true", repo.updates[SettingKeyProfitControlEnabled])
	require.Equal(t, "0.30000000", repo.updates[SettingKeyProfitMinMargin])
	require.Equal(t, "0.05000000", repo.updates[SettingKeyProfitSafetyBuffer])

	repo = &settingUpdateRepoStub{}
	svc = NewSettingService(repo, &config.Config{})
	err = svc.UpdateSettings(context.Background(), &SystemSettings{
		ProfitControlEnabled: true, ProfitMinMargin: 0.6, ProfitSafetyBuffer: 0.5,
	})
	require.Error(t, err)
	require.Equal(t, "INVALID_PROFIT_CONTROL", infraerrors.Reason(err))
	require.Nil(t, repo.updates)
}

func TestParseDefaultSubscriptions_NormalizesValues(t *testing.T) {
	got := parseDefaultSubscriptions(`[{"plan_id":11,"validity_days":30},{"plan_id":11,"validity_days":60},{"plan_id":0,"validity_days":10},{"plan_id":12,"validity_days":99999}]`)
	require.Equal(t, []DefaultSubscriptionSetting{
		{PlanID: 11, ValidityDays: 30},
		{PlanID: 11, ValidityDays: 60},
		{PlanID: 12, ValidityDays: MaxValidityDays},
	}, got)
}

func TestSettingService_UpdateSettings_PaymentVisibleMethods(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		PaymentVisibleMethodAlipaySource:  "alipay",
		PaymentVisibleMethodWxpaySource:   "easypay",
		PaymentVisibleMethodAlipayEnabled: true,
		PaymentVisibleMethodWxpayEnabled:  false,
	})
	require.NoError(t, err)
	require.Equal(t, VisibleMethodSourceOfficialAlipay, repo.updates[SettingPaymentVisibleMethodAlipaySource])
	require.Equal(t, VisibleMethodSourceEasyPayWechat, repo.updates[SettingPaymentVisibleMethodWxpaySource])
	require.Equal(t, "true", repo.updates[SettingPaymentVisibleMethodAlipayEnabled])
	require.Equal(t, "false", repo.updates[SettingPaymentVisibleMethodWxpayEnabled])
}

func TestSettingService_UpdateSettings_AntigravityUserAgentVersion(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		AntigravityUserAgentVersion: "1.23.2",
	})
	require.NoError(t, err)
	require.Equal(t, "1.23.2", repo.updates[SettingKeyAntigravityUserAgentVersion])
}

func TestSettingService_GetAntigravityUserAgentVersion_Precedence(t *testing.T) {
	t.Run("后台设置优先", func(t *testing.T) {
		svc := NewSettingService(&settingAntigravityUARepoStub{values: map[string]string{
			SettingKeyAntigravityUserAgentVersion: "1.24.0",
		}}, &config.Config{})

		require.Equal(t, "1.24.0", svc.GetAntigravityUserAgentVersion(context.Background()))
	})

	t.Run("空值回退配置默认值", func(t *testing.T) {
		svc := NewSettingService(&settingAntigravityUARepoStub{values: map[string]string{
			SettingKeyAntigravityUserAgentVersion: "",
		}}, &config.Config{})

		require.Equal(t, antigravity.GetDefaultUserAgentVersion(), svc.GetAntigravityUserAgentVersion(context.Background()))
	})

	t.Run("缺失回退配置默认值", func(t *testing.T) {
		svc := NewSettingService(&settingAntigravityUARepoStub{values: map[string]string{}}, &config.Config{})

		require.Equal(t, antigravity.GetDefaultUserAgentVersion(), svc.GetAntigravityUserAgentVersion(context.Background()))
	})
}

func TestSettingService_UpdateSettings_RejectsInvalidPaymentVisibleMethodSource(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		PaymentVisibleMethodAlipaySource: "not-a-provider",
	})
	require.Error(t, err)
	require.Equal(t, "INVALID_PAYMENT_VISIBLE_METHOD_SOURCE", infraerrors.Reason(err))
	require.Nil(t, repo.updates)
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
