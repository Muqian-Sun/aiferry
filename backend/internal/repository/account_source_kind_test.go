//go:build unit

package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// TestAccountSourceKind 覆盖写入账号时来源维度的取值优先级：
// 调用方显式指定时以其为准，否则按类型推导。
func TestAccountSourceKind(t *testing.T) {
	tests := []struct {
		name     string
		account  service.Account
		expected string
	}{
		{
			name:     "显式指定优先",
			account:  service.Account{Type: service.AccountTypeOAuth, SourceKind: service.AccountSourceAPIKey},
			expected: service.AccountSourceAPIKey,
		},
		{
			name:     "空白视为未指定，按类型推导",
			account:  service.Account{Type: service.AccountTypeAPIKey, SourceKind: "   "},
			expected: service.AccountSourceAPIKey,
		},
		{
			name:     "未指定时 oauth 推导为成品号",
			account:  service.Account{Type: service.AccountTypeOAuth},
			expected: service.AccountSourceSubscription,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := tt.account
			require.Equal(t, tt.expected, accountSourceKind(&account))
		})
	}
}

// TestNormalizeProtocolEndpoints 保证写库的值永远不是 nil：
// 列上是 NOT NULL DEFAULT '{}'，写 nil 会在插入时报错而不是回落默认值。
func TestNormalizeProtocolEndpoints(t *testing.T) {
	require.Equal(t, map[string]string{}, normalizeProtocolEndpoints(nil))

	given := map[string]string{"anthropic_messages": "https://relay.example.com"}
	require.Equal(t, given, normalizeProtocolEndpoints(given))
}
