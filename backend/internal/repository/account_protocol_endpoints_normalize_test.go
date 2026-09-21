//go:build unit

package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNormalizeProtocolEndpoints 保证写库的值永远不是 nil：
// 列上是 NOT NULL DEFAULT '{}'，写 nil 会在插入时报错而不是回落默认值。
func TestNormalizeProtocolEndpoints(t *testing.T) {
	require.Equal(t, map[string]string{}, normalizeProtocolEndpoints(nil))

	given := map[string]string{"anthropic_messages": "https://relay.example.com"}
	require.Equal(t, given, normalizeProtocolEndpoints(given))
}
