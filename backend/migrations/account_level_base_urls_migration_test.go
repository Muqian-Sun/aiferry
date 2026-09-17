package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDropAccountLevelBaseURLsMigration 固定 241 号迁移的清理范围：成品号无条件清
// base_url / api_base_urls；第三方 key 只在已配协议映射时清；中继键对所有账号清。
func TestDropAccountLevelBaseURLsMigration(t *testing.T) {
	content, err := FS.ReadFile("241_drop_account_level_base_urls.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "SET credentials = credentials - 'base_url' - 'api_base_urls'")
	require.Contains(t, sql, "type NOT IN ('apikey', 'upstream') OR protocol_endpoints <> '{}'::jsonb")
	require.Contains(t, sql, "SET extra = extra - 'custom_base_url' - 'custom_base_url_enabled'")
}
