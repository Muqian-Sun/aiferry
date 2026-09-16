//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIsAntigravity 固定 IsAntigravity 的判定口径：只看 Platform，不看 Type。
// 该方法用于替换散落在各处的 `account.Platform == PlatformAntigravity` 裸比较，
// 因此必须与裸比较逐字段等价。
func TestIsAntigravity(t *testing.T) {
	tests := []struct {
		name     string
		account  Account
		expected bool
	}{
		{
			name:     "antigravity oauth",
			account:  Account{Platform: PlatformAntigravity, Type: AccountTypeOAuth},
			expected: true,
		},
		{
			name:     "antigravity apikey",
			account:  Account{Platform: PlatformAntigravity, Type: AccountTypeAPIKey},
			expected: true,
		},
		{
			name:     "anthropic is not antigravity",
			account:  Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth},
			expected: false,
		},
		{
			name:     "gemini is not antigravity",
			account:  Account{Platform: PlatformGemini, Type: AccountTypeOAuth},
			expected: false,
		},
		{
			name:     "empty platform is not antigravity",
			account:  Account{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := tt.account
			require.Equal(t, tt.expected, account.IsAntigravity())
		})
	}
}
