package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// 插件系统已下线（255）：两张插件表都要删，且先删引用方 bindings，再删被引用的 installations。
func TestDropPluginsMigrationDropsBothTablesInDependencyOrder(t *testing.T) {
	content, err := FS.ReadFile("255_drop_plugins.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	bindings := strings.Index(sql, "DROP TABLE IF EXISTS sub2api_plugin_bindings;")
	installations := strings.Index(sql, "DROP TABLE IF EXISTS sub2api_plugin_installations;")
	require.GreaterOrEqual(t, bindings, 0, "missing drop of sub2api_plugin_bindings")
	require.GreaterOrEqual(t, installations, 0, "missing drop of sub2api_plugin_installations")
	require.Less(t, bindings, installations, "bindings references installations and must be dropped first")
	require.NotContains(t, strings.ToUpper(sql), "CASCADE")
}
