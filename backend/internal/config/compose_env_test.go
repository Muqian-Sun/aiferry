//go:build unit

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// deploy/docker-compose*.yml 把站点地址 / 发信 / 人机验证原样透传成 KEY=${KEY:-}：
// .env 里没填的项会以空字符串进到容器。
var composeSiteEnvKeys = []string{
	"SERVER_FRONTEND_URL",
	"SMTP_HOST", "SMTP_PORT", "SMTP_USERNAME", "SMTP_PASSWORD", "SMTP_FROM", "SMTP_USE_TLS",
	"TURNSTILE_SITE_KEY", "TURNSTILE_SECRET_KEY",
}

func setComposeSiteEnvEmpty(t *testing.T) {
	t.Helper()
	for _, key := range composeSiteEnvKeys {
		t.Setenv(key, "")
	}
}

// 空字符串不算配了：没有 config.yaml 时落回内置默认（587 端口、不开隐式 TLS），也不会因为空端口启动失败。
func TestComposeEmptySiteEnvKeepsDefaults(t *testing.T) {
	resetViperWithJWTSecret(t)
	setComposeSiteEnvEmpty(t)

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "", cfg.Server.FrontendURL)
	require.Equal(t, SMTPConfig{Port: 587}, cfg.SMTP)
	require.False(t, cfg.Turnstile.Configured())
}

// 空字符串不能冲掉 config.yaml 里配好的值（升级前已经用 config.yaml 配了发信的部署）。
func TestComposeEmptySiteEnvKeepsConfigFileValues(t *testing.T) {
	resetViperWithJWTSecret(t)
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(`server:
  frontend_url: https://yaml.example.com
smtp:
  host: smtp.yaml.example.com
  port: 465
  username: yaml-user
  password: yaml-pass
  from: noreply@yaml.example.com
  use_tls: true
turnstile:
  site_key: yaml-site
  secret_key: yaml-secret
`), 0o600))
	t.Setenv("CONFIG_FILE", configFile)
	setComposeSiteEnvEmpty(t)

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "https://yaml.example.com", cfg.Server.FrontendURL)
	require.Equal(t, SMTPConfig{
		Host:     "smtp.yaml.example.com",
		Port:     465,
		Username: "yaml-user",
		Password: "yaml-pass",
		From:     "noreply@yaml.example.com",
		UseTLS:   true,
	}, cfg.SMTP)
	require.Equal(t, "yaml-site", cfg.Turnstile.SiteKey)
	require.Equal(t, "yaml-secret", cfg.Turnstile.SecretKey)
}

// 填了值就以 .env 为准，盖过 config.yaml。
func TestComposeSMTPEnvOverridesConfigFile(t *testing.T) {
	resetViperWithJWTSecret(t)
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte("smtp:\n  host: smtp.yaml.example.com\n  port: 587\n  from: yaml@example.com\n"), 0o600))
	t.Setenv("CONFIG_FILE", configFile)
	t.Setenv("SMTP_HOST", "smtp.qq.com")
	t.Setenv("SMTP_PORT", "465")
	t.Setenv("SMTP_USERNAME", "10000@qq.com")
	t.Setenv("SMTP_PASSWORD", "auth-code")
	t.Setenv("SMTP_FROM", "10000@qq.com")
	t.Setenv("SMTP_USE_TLS", "true")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, SMTPConfig{
		Host:     "smtp.qq.com",
		Port:     465,
		Username: "10000@qq.com",
		Password: "auth-code",
		From:     "10000@qq.com",
		UseTLS:   true,
	}, cfg.SMTP)
}
