//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"

	"github.com/stretchr/testify/require"
)

// newBalanceNotifyServiceForTest 未配 SMTP 的余额提醒服务：提醒一律关着。
func newBalanceNotifyServiceForTest() (*BalanceNotifyService, *mockSettingRepo) {
	return newBalanceNotifyServiceWith(nil, nil)
}

// newBalanceNotifyServiceWith cfg 决定配没配 SMTP、用户站地址；admins 给渠道额度提醒找收件人。
// 用例要避开真正会发信的越线场景（发信走 goroutine，会去连 cfg 里的 SMTP）。
func newBalanceNotifyServiceWith(cfg *config.Config, admins AdminEmailReader) (*BalanceNotifyService, *mockSettingRepo) {
	repo := newMockSettingRepo()
	email := NewEmailService(repo, nil, cfg)
	return NewBalanceNotifyService(email, repo, nil, admins, cfg), repo
}

type firstAdminStub struct {
	user *User
	err  error
}

func (s firstAdminStub) GetFirstAdmin(context.Context) (*User, error) { return s.user, s.err }

// ---------- guard clauses ----------

func TestCheckBalanceAfterDeduction_NilUser(t *testing.T) {
	s, _ := newBalanceNotifyServiceForTest()
	// Should not panic.
	s.CheckBalanceAfterDeduction(context.Background(), nil, 100, 50)
}

func TestCheckBalanceAfterDeduction_UserNotifyDisabled(t *testing.T) {
	s, _ := newBalanceNotifyServiceWith(smtpConfiguredForTest(), nil)
	u := &User{ID: 1, BalanceNotifyEnabled: false}
	// Even with a crossing, disabled flag short-circuits.
	s.CheckBalanceAfterDeduction(context.Background(), u, 20, 0.5)
}

func TestCheckBalanceAfterDeduction_SMTPNotConfigured(t *testing.T) {
	s, repo := newBalanceNotifyServiceForTest()
	repo.data["balance_low_notify_enabled"] = "true" // 旧后台开关留下的行，不再生效
	u := &User{ID: 1, BalanceNotifyEnabled: true}
	s.CheckBalanceAfterDeduction(context.Background(), u, 20, 19.5)
}

func TestCheckBalanceAfterDeduction_UserThresholdZero(t *testing.T) {
	s, _ := newBalanceNotifyServiceWith(smtpConfiguredForTest(), nil)
	zero := 0.0
	u := &User{ID: 1, BalanceNotifyEnabled: true, BalanceNotifyThreshold: &zero}
	s.CheckBalanceAfterDeduction(context.Background(), u, 20, 19.5)
}

func TestCheckBalanceAfterDeduction_UserThresholdOverride(t *testing.T) {
	s, _ := newBalanceNotifyServiceWith(smtpConfiguredForTest(), nil)
	customThreshold := 5.0
	u := &User{
		ID:                     1,
		BalanceNotifyEnabled:   true,
		BalanceNotifyThreshold: &customThreshold,
	}
	// 用户自己的 5.0 覆盖默认阈值；20 -> 15 没跨过 5，不发（以不 panic 为准）。
	s.CheckBalanceAfterDeduction(context.Background(), u, 20, 5)
}

func TestCheckBalanceAfterDeduction_NoCrossingNotFired(t *testing.T) {
	s, _ := newBalanceNotifyServiceWith(smtpConfiguredForTest(), nil)
	u := &User{ID: 1, BalanceNotifyEnabled: true}

	// 100 -> 95，都在默认阈值之上，没越线。
	s.CheckBalanceAfterDeduction(context.Background(), u, 100, 5)
	// 0.5 -> 0.3，已经在阈值之下，不算越线（只在从上往下第一次越过时发）。
	s.CheckBalanceAfterDeduction(context.Background(), u, 0.5, 0.2)
}

// ---------- nil-service guards on CheckAccountQuotaAfterIncrement ----------

func TestCheckAccountQuotaAfterIncrement_NilAccount(t *testing.T) {
	s, _ := newBalanceNotifyServiceForTest()
	// Should not panic.
	s.CheckAccountQuotaAfterIncrement(context.Background(), nil, 10, nil)
}

func TestCheckAccountQuotaAfterIncrement_ZeroCost(t *testing.T) {
	s, _ := newBalanceNotifyServiceForTest()
	a := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"}}
	s.CheckAccountQuotaAfterIncrement(context.Background(), a, 0, nil)
}

func TestCheckAccountQuotaAfterIncrement_NegativeCost(t *testing.T) {
	s, _ := newBalanceNotifyServiceForTest()
	a := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"}}
	s.CheckAccountQuotaAfterIncrement(context.Background(), a, -5, nil)
}

func TestCheckAccountQuotaAfterIncrement_SMTPNotConfigured(t *testing.T) {
	s, repo := newBalanceNotifyServiceForTest()
	repo.data["account_quota_notify_enabled"] = "true" // 旧后台开关留下的行，不再生效
	a := &Account{
		ID:       1,
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Extra: map[string]any{
			"quota_daily_limit": 1000.0,
			"quota_daily_used":  850.0,
		},
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"},
	}
	// 没配 SMTP → 即使越线（750 → 850 跨过 800）也不处理。
	s.CheckAccountQuotaAfterIncrement(context.Background(), a, 100, nil)
}

// ---------- 通知由代码决定：跟着 SMTP 走，阈值与充值页写死 ----------

func TestGetBalanceNotifyConfig_FollowsSMTPAndCode(t *testing.T) {
	cfg := smtpConfiguredForTest()
	cfg.Server.FrontendURL = "https://user.example/"
	s, repo := newBalanceNotifyServiceWith(cfg, nil)
	// 旧后台设置留下的行都不再生效
	repo.data["balance_low_notify_enabled"] = "false"
	repo.data["balance_low_notify_threshold"] = "12.5"
	repo.data["balance_low_notify_recharge_url"] = "https://admin.example/pay"

	enabled, threshold, url := s.getBalanceNotifyConfig(context.Background())
	require.True(t, enabled)
	require.Equal(t, BalanceLowNotifyThreshold, threshold)
	require.Equal(t, "https://user.example/billing/recharge", url)
}

func TestGetBalanceNotifyConfig_DisabledWithoutSMTP(t *testing.T) {
	s, repo := newBalanceNotifyServiceForTest()
	repo.data["balance_low_notify_enabled"] = "true"

	enabled, _, _ := s.getBalanceNotifyConfig(context.Background())
	require.False(t, enabled)
}

func TestGetBalanceNotifyConfig_NoFrontendURLNoRechargeLink(t *testing.T) {
	s, _ := newBalanceNotifyServiceWith(smtpConfiguredForTest(), nil)

	enabled, _, url := s.getBalanceNotifyConfig(context.Background())
	require.True(t, enabled)
	require.Empty(t, url)
}

func TestIsAccountQuotaNotifyEnabled_FollowsSMTP(t *testing.T) {
	off, repo := newBalanceNotifyServiceForTest()
	repo.data["account_quota_notify_enabled"] = "true"
	require.False(t, off.isAccountQuotaNotifyEnabled(context.Background()))

	on, _ := newBalanceNotifyServiceWith(smtpConfiguredForTest(), nil)
	require.True(t, on.isAccountQuotaNotifyEnabled(context.Background()))
}

func TestGetAccountQuotaNotifyEmails_FirstAdmin(t *testing.T) {
	s, repo := newBalanceNotifyServiceWith(smtpConfiguredForTest(), firstAdminStub{user: &User{Email: "admin@example.com"}})
	repo.data["account_quota_notify_emails"] = `[{"email":"stale@example.com","verified":true}]`
	require.Equal(t, []string{"admin@example.com"}, s.getAccountQuotaNotifyEmails(context.Background()))

	none, _ := newBalanceNotifyServiceWith(smtpConfiguredForTest(), firstAdminStub{err: ErrUserNotFound})
	require.Empty(t, none.getAccountQuotaNotifyEmails(context.Background()))

	synthetic, _ := newBalanceNotifyServiceWith(smtpConfiguredForTest(), firstAdminStub{user: &User{Email: "7" + WeChatConnectSyntheticEmailDomain}})
	require.Empty(t, synthetic.getAccountQuotaNotifyEmails(context.Background()))
}

// ---------- crossedDownward ----------

func TestCrossedDownward_CrossesBelow(t *testing.T) {
	// oldBalance > threshold, newBalance < threshold → true
	require.True(t, crossedDownward(100, 5, 10))
}

func TestCrossedDownward_ExactlyAtThreshold(t *testing.T) {
	// oldBalance > threshold, newBalance == threshold → false (not below)
	require.False(t, crossedDownward(100, 10, 10))
}

func TestCrossedDownward_OldExactlyAtThreshold_NewBelow(t *testing.T) {
	// oldBalance == threshold, newBalance < threshold → true
	// (at-or-above → below counts as a crossing)
	require.True(t, crossedDownward(10, 5, 10))
}

func TestCrossedDownward_AlreadyBelow(t *testing.T) {
	// oldBalance < threshold → false (already below, no new crossing)
	require.False(t, crossedDownward(5, 3, 10))
}

func TestCrossedDownward_BothAbove(t *testing.T) {
	// oldBalance > threshold, newBalance > threshold → false (no crossing)
	require.False(t, crossedDownward(100, 50, 10))
}

func TestCrossedDownward_ZeroThreshold(t *testing.T) {
	// threshold == 0 → oldV >= 0 is always true, but newV < 0 only for negatives
	// Typical case: positive balances should not fire when threshold is 0.
	require.False(t, crossedDownward(10, 5, 0))
	require.False(t, crossedDownward(0, 0, 0))
}

func TestCrossedDownward_ZeroThreshold_NegativeNew(t *testing.T) {
	// Edge case: newBalance goes negative with threshold=0.
	require.True(t, crossedDownward(5, -1, 0))
}

func TestCrossedDownward_NegativeValues(t *testing.T) {
	// Both already negative, threshold is positive → no crossing (already below).
	require.False(t, crossedDownward(-5, -10, 10))
}

func TestCrossedDownward_LargeDecrement(t *testing.T) {
	// A single large deduction crosses the threshold.
	require.True(t, crossedDownward(1000, 0.5, 100))
}

func TestCrossedDownward_SmallDecrement_NoCrossing(t *testing.T) {
	// A tiny deduction stays above threshold.
	require.False(t, crossedDownward(100, 99.99, 10))
}

// ---------- checkQuotaDimCrossings ----------

func TestCheckQuotaDimCrossings_NoDimensions(t *testing.T) {
	s, _ := newBalanceNotifyServiceForTest()
	account := &Account{ID: 1, Name: "test", Platform: PlatformAnthropic}
	// Empty dims → no crossing, no panic.
	s.checkQuotaDimCrossings(account, nil, 10, []string{"admin@example.com"}, "TestSite")
	s.checkQuotaDimCrossings(account, []quotaDim{}, 10, []string{"admin@example.com"}, "TestSite")
}

func TestCheckQuotaDimCrossings_NoCrossing_BothBelowThreshold(t *testing.T) {
	s, _ := newBalanceNotifyServiceForTest()
	account := &Account{ID: 1, Name: "test", Platform: PlatformAnthropic}
	// limit=1000 → 提醒点 = 用到 800
	// currentUsed=300 (after), oldUsed=300-50=250 (before). Both < 800, no crossing.
	dims := []quotaDim{{name: quotaDimDaily, currentUsed: 300, limit: 1000}}
	s.checkQuotaDimCrossings(account, dims, 50, []string{"admin@example.com"}, "TestSite")
}

func TestCheckQuotaDimCrossings_NoCrossing_BothAboveThreshold(t *testing.T) {
	s, _ := newBalanceNotifyServiceForTest()
	account := &Account{ID: 1, Name: "test", Platform: PlatformAnthropic}
	// currentUsed=900 (after), oldUsed=900-50=850 (before). Both >= 800, no crossing.
	dims := []quotaDim{{name: quotaDimDaily, currentUsed: 900, limit: 1000}}
	s.checkQuotaDimCrossings(account, dims, 50, []string{"admin@example.com"}, "TestSite")
}

func TestCheckQuotaDimCrossings_ZeroLimit_Skipped(t *testing.T) {
	s, _ := newBalanceNotifyServiceForTest()
	account := &Account{ID: 1, Name: "test", Platform: PlatformAnthropic}
	// limit=0 → resolvedThreshold returns 0 → skipped.
	dims := []quotaDim{{name: quotaDimTotal, currentUsed: 50, limit: 0}}
	s.checkQuotaDimCrossings(account, dims, 50, []string{"admin@example.com"}, "TestSite")
}

func TestCheckQuotaDimCrossings_MultipleDims_NoneCrossing(t *testing.T) {
	s, _ := newBalanceNotifyServiceForTest()
	account := &Account{ID: 1, Name: "test", Platform: PlatformAnthropic}
	dims := []quotaDim{
		{name: quotaDimDaily, currentUsed: 300, limit: 1000},  // 250 → 300，都在 800 以下
		{name: quotaDimWeekly, currentUsed: 950, limit: 1000}, // 900 → 950，都在 800 以上
		{name: quotaDimTotal, currentUsed: 500, limit: 0},     // 没设限额
	}
	// None should trigger. No panic expected.
	s.checkQuotaDimCrossings(account, dims, 50, []string{"admin@example.com"}, "TestSite")
}
