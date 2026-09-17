//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// require_privacy_set 分组的 privacy 判定按上游厂商（Vendor），不看第三方 key 的平台标签。
func TestAccountIsPrivacySet_ByVendorNotLabel(t *testing.T) {
	trainingOff := map[string]any{"privacy_mode": PrivacyModeTrainingOff}
	officialOpenAI := map[string]string{APIProtocolResponses: "https://api.openai.com"}

	tests := []struct {
		name    string
		account Account
		want    bool
	}{
		// 成品号：按平台，行为不变。
		{"openai oauth without privacy", Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}, false},
		{"openai oauth training off", Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: trainingOff}, true},
		{"antigravity oauth without privacy", Account{Platform: PlatformAntigravity, Type: AccountTypeOAuth}, false},
		{"antigravity oauth privacy set", Account{Platform: PlatformAntigravity, Type: AccountTypeOAuth, Extra: map[string]any{"privacy_mode": AntigravityPrivacySet}}, true},
		{"anthropic oauth has no privacy concept", Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}, true},
		// 第三方 key：标签不参与。
		{"official openai key with anthropic label", Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey, ProtocolEndpoints: officialOpenAI}, false},
		{"official openai key training off", Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey, ProtocolEndpoints: officialOpenAI, Extra: trainingOff}, true},
		{"relay key with openai label", Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, ProtocolEndpoints: map[string]string{APIProtocolResponses: "https://relay.example.com/v1"}}, true},
		{"relay key with antigravity label", Account{Platform: PlatformAntigravity, Type: AccountTypeAPIKey, ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://relay.example.com"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.account.IsPrivacySet())
		})
	}
}
