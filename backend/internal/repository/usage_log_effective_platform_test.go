package repository

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// 分组删除后，用量行的「有效平台」只看承接该请求的账号，不再有分组平台覆写。
func TestUsageLogEffectivePlatformExprUsesAccountPlatform(t *testing.T) {
	expr := strings.ToLower(usageLogEffectivePlatformExpr)

	require.Equal(t, "a.platform", expr)
	require.NotContains(t, expr, "g.platform")
	require.NotContains(t, expr, "composite")
}
