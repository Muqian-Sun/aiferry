//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 池模式同渠道重试状态码写死 401 / 403 / 429（2026-09-28 P5，channel_features.go），
// 渠道上的 pool_mode_retry_status_codes 不再读。
func TestIsPoolModeRetryableStatus_Account(t *testing.T) {
	legacyList := map[string]any{"pool_mode": true, "pool_mode_retry_status_codes": []any{float64(502), float64(503)}}
	legacyEmpty := map[string]any{"pool_mode": true, "pool_mode_retry_status_codes": []any{}}
	tests := []struct {
		name       string
		account    *Account
		statusCode int
		expected   bool
	}{
		{name: "nil_account_401", account: nil, statusCode: 401, expected: true},
		{name: "nil_account_500", account: nil, statusCode: 500, expected: false},
		{name: "unconfigured_403", account: &Account{Credentials: map[string]any{"pool_mode": true}}, statusCode: 403, expected: true},
		{name: "unconfigured_429", account: &Account{Credentials: map[string]any{"pool_mode": true}}, statusCode: 429, expected: true},
		{name: "unconfigured_502", account: &Account{Credentials: map[string]any{"pool_mode": true}}, statusCode: 502, expected: false},
		{name: "legacy_list_ignored_401", account: &Account{Credentials: legacyList}, statusCode: 401, expected: true},
		{name: "legacy_list_ignored_502", account: &Account{Credentials: legacyList}, statusCode: 502, expected: false},
		{name: "legacy_empty_list_ignored_429", account: &Account{Credentials: legacyEmpty}, statusCode: 429, expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.account.IsPoolModeRetryableStatus(tt.statusCode))
		})
	}
}
