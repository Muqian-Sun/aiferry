//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDeriveAccountSourceKind 固定「账号类型 → 来源维度」的推导口径。
// migrations/239 的回填、仓储层新建与更新账号都走这一条规则，三者必须同源：
// 一旦分叉，就会出现类型是 apikey、来源却是 subscription 的错配，
// 而这种错配只在数据里看得出来，没有任何报错。
func TestDeriveAccountSourceKind(t *testing.T) {
	tests := []struct {
		accountType string
		expected    string
	}{
		{AccountTypeOAuth, AccountSourceSubscription},
		{AccountTypeSetupToken, AccountSourceSubscription},
		{AccountTypeBedrock, AccountSourceSubscription},
		{AccountTypeServiceAccount, AccountSourceSubscription},
		{AccountTypeAPIKey, AccountSourceAPIKey},
		{AccountTypeUpstream, AccountSourceAPIKey},
		{"", AccountSourceSubscription},
		{"future-unknown-type", AccountSourceSubscription},
	}

	for _, tt := range tests {
		t.Run(tt.accountType, func(t *testing.T) {
			require.Equal(t, tt.expected, DeriveAccountSourceKind(tt.accountType))
		})
	}
}
