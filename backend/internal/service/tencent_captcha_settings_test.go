//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// tencentCaptchaTestConfig 部署配置里配齐腾讯天御（人机验证只认部署配置）。
func tencentCaptchaTestConfig(region string) *config.Config {
	return &config.Config{TencentCaptcha: config.TencentCaptchaConfig{
		AppID:          "123456789",
		AppSecretKey:   "app-secret",
		CloudSecretID:  "cloud-secret-id",
		CloudSecretKey: "cloud-secret-key",
		Region:         region,
	}}
}

func TestSettingService_GetPublicSettingsExposesOnlyTencentCaptchaAppID(t *testing.T) {
	svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, tencentCaptchaTestConfig(TencentCaptchaRegionINTL))

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.TencentCaptchaEnabled)
	require.Equal(t, "123456789", settings.TencentCaptchaAppID)
	// 站点必须原样公开下发：前端据此决定加载哪个站点的 SDK 脚本与构造函数形态。
	require.Equal(t, TencentCaptchaRegionINTL, settings.TencentCaptchaRegion)

	raw, err := json.Marshal(settings)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "app-secret")
	require.NotContains(t, string(raw), "cloud-secret-id")
	require.NotContains(t, string(raw), "cloud-secret-key")
}

func TestSettingService_CaptchaProviderConfigReadsTencentFromDeploymentConfig(t *testing.T) {
	svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, tencentCaptchaTestConfig(""))

	got := svc.CaptchaProviderConfig()

	require.False(t, got.TurnstileEnabled)
	require.False(t, got.Aliyun.Enabled)
	require.Equal(t, TencentCaptchaConfig{
		Enabled:        true,
		AppID:          "123456789",
		AppSecretKey:   "app-secret",
		CloudSecretID:  "cloud-secret-id",
		CloudSecretKey: "cloud-secret-key",
		// 未配置站点时回落中国站
		Region: TencentCaptchaRegionCN,
	}, got.Tencent)
}

// 数据库里旧的人机验证设置行（后台曾经能改）一律不读，只认部署配置。
func TestSettingService_CaptchaProviderConfigIgnoresDatabaseRows(t *testing.T) {
	svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{
		"tencent_captcha_enabled":          "true",
		"tencent_captcha_app_id":           "987654321",
		"tencent_captcha_app_secret_key":   "db-app-secret",
		"tencent_captcha_cloud_secret_id":  "db-cloud-secret-id",
		"tencent_captcha_cloud_secret_key": "db-cloud-secret-key",
		"turnstile_enabled":                "true",
		"turnstile_site_key":               "db-site-key",
		"turnstile_secret_key":             "db-secret-key",
	}}, &config.Config{})

	got := svc.CaptchaProviderConfig()
	require.False(t, got.TurnstileEnabled)
	require.False(t, got.Tencent.Enabled)
	require.False(t, got.Aliyun.Enabled)

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.False(t, settings.TurnstileEnabled)
	require.Empty(t, settings.TurnstileSiteKey)
	require.False(t, settings.TencentCaptchaEnabled)
	require.Empty(t, settings.TencentCaptchaAppID)
}
