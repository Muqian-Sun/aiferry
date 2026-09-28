//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// ---------- resolveBalanceThreshold ----------

func TestResolveBalanceThreshold_Fixed(t *testing.T) {
	// Fixed type always returns the raw threshold regardless of totalRecharged.
	require.Equal(t, 10.0, resolveBalanceThreshold(10, thresholdTypeFixed, 1000))
	require.Equal(t, 10.0, resolveBalanceThreshold(10, thresholdTypeFixed, 0))
	require.Equal(t, 0.0, resolveBalanceThreshold(0, thresholdTypeFixed, 1000))
}

func TestResolveBalanceThreshold_Percentage(t *testing.T) {
	// 10% of 1000 = 100
	require.Equal(t, 100.0, resolveBalanceThreshold(10, thresholdTypePercentage, 1000))
	// 50% of 200 = 100
	require.Equal(t, 100.0, resolveBalanceThreshold(50, thresholdTypePercentage, 200))
}

func TestResolveBalanceThreshold_PercentageZeroRecharged(t *testing.T) {
	// When totalRecharged is 0, percentage falls through to raw threshold
	// (treated as fixed). This is the defensive behavior.
	require.Equal(t, 10.0, resolveBalanceThreshold(10, thresholdTypePercentage, 0))
}

func TestResolveBalanceThreshold_EmptyType(t *testing.T) {
	// Empty type is treated as fixed (not percentage).
	require.Equal(t, 10.0, resolveBalanceThreshold(10, "", 1000))
}

// ---------- quotaDim.resolvedThreshold ----------

// 额度提醒写死「剩余降到限额的 20%（用到 80%）时提醒」（2026-09-28 P5，channel_features.go）。
func TestResolvedThreshold_RemainingPercentFromCode(t *testing.T) {
	d := quotaDim{limit: 1000}
	require.InDelta(t, 800.0, d.resolvedThreshold(), 0.001)
	d = quotaDim{limit: 50}
	require.InDelta(t, 40.0, d.resolvedThreshold(), 0.001)
}

func TestResolvedThreshold_ZeroLimit(t *testing.T) {
	// limit=0 → returns 0 to avoid false alerts on unlimited quotas
	d := quotaDim{limit: 0}
	require.Equal(t, 0.0, d.resolvedThreshold())
}

func TestResolvedThreshold_NegativeLimit(t *testing.T) {
	// Negative limit treated as 0
	d := quotaDim{limit: -10}
	require.Equal(t, 0.0, d.resolvedThreshold())
}

// ---------- sanitizeEmailHeader ----------

func TestSanitizeEmailHeader_CRLF(t *testing.T) {
	require.Equal(t, "Subject injected", sanitizeEmailHeader("Subject\r\n injected"))
}

func TestSanitizeEmailHeader_OnlyCR(t *testing.T) {
	require.Equal(t, "foobar", sanitizeEmailHeader("foo\rbar"))
}

func TestSanitizeEmailHeader_OnlyLF(t *testing.T) {
	require.Equal(t, "foobar", sanitizeEmailHeader("foo\nbar"))
}

func TestSanitizeEmailHeader_Clean(t *testing.T) {
	require.Equal(t, "Sub2API", sanitizeEmailHeader("Sub2API"))
}

func TestSanitizeEmailHeader_Empty(t *testing.T) {
	require.Equal(t, "", sanitizeEmailHeader(""))
}

func TestSanitizeEmailHeader_MultipleNewlines(t *testing.T) {
	require.Equal(t, "abc", sanitizeEmailHeader("a\r\nb\r\nc"))
}

// ---------- buildQuotaDims ----------

func TestBuildQuotaDims_AllDimensionsReturned(t *testing.T) {
	a := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Extra: map[string]any{
			"quota_daily_limit":  500.0,
			"quota_weekly_limit": 2000.0,
			"quota_limit":        10000.0,
			"quota_daily_used":   50.0,
			"quota_weekly_used":  300.0,
			"quota_used":         1000.0,
		},
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"},
	}

	dims := buildQuotaDims(a)
	require.Len(t, dims, 3)

	require.Equal(t, quotaDimDaily, dims[0].name)
	require.Equal(t, 500.0, dims[0].limit)
	require.Equal(t, 50.0, dims[0].currentUsed)

	require.Equal(t, quotaDimWeekly, dims[1].name)
	require.Equal(t, 2000.0, dims[1].limit)
	require.Equal(t, 300.0, dims[1].currentUsed)

	require.Equal(t, quotaDimTotal, dims[2].name)
	require.Equal(t, 10000.0, dims[2].limit)
	require.Equal(t, 1000.0, dims[2].currentUsed)
}

func TestBuildQuotaDims_EmptyExtra(t *testing.T) {
	// Missing fields default to zero (no limit → no alert).
	a := &Account{
		Platform:          PlatformAnthropic,
		Type:              AccountTypeAPIKey,
		Extra:             map[string]any{},
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"},
	}
	dims := buildQuotaDims(a)
	require.Len(t, dims, 3)
	for _, d := range dims {
		require.Equal(t, 0.0, d.limit)
		require.Equal(t, 0.0, d.resolvedThreshold())
	}
}

// ---------- buildQuotaDimsFromState ----------

func TestBuildQuotaDimsFromState_UsesStateValues(t *testing.T) {
	state := &AccountQuotaState{
		DailyUsed:   77.0,
		DailyLimit:  500.0,
		WeeklyUsed:  88.0,
		WeeklyLimit: 2000.0,
		TotalUsed:   99.0,
		TotalLimit:  10000.0,
	}
	dims := buildQuotaDimsFromState(state)
	require.Len(t, dims, 3)
	require.Equal(t, 77.0, dims[0].currentUsed)
	require.Equal(t, 500.0, dims[0].limit)
	require.Equal(t, 88.0, dims[1].currentUsed)
	require.Equal(t, 2000.0, dims[1].limit)
	require.Equal(t, 99.0, dims[2].currentUsed)
	require.Equal(t, 10000.0, dims[2].limit)
}

// ---------- collectBalanceNotifyRecipients ----------

func TestCollectBalanceNotifyRecipients_Empty(t *testing.T) {
	s := &BalanceNotifyService{}
	u := &User{BalanceNotifyExtraEmails: nil}
	require.Empty(t, s.collectBalanceNotifyRecipients(u))
}

func TestCollectBalanceNotifyRecipients_FiltersDisabledAndUnverified(t *testing.T) {
	s := &BalanceNotifyService{}
	u := &User{
		BalanceNotifyExtraEmails: []NotifyEmailEntry{
			{Email: "a@example.com", Verified: true, Disabled: false},
			{Email: "b@example.com", Verified: true, Disabled: true},   // disabled
			{Email: "c@example.com", Verified: false, Disabled: false}, // unverified
			{Email: "d@example.com", Verified: true, Disabled: false},
		},
	}
	got := s.collectBalanceNotifyRecipients(u)
	require.Equal(t, []string{"a@example.com", "d@example.com"}, got)
}

func TestCollectBalanceNotifyRecipients_DeduplicatesCaseInsensitive(t *testing.T) {
	s := &BalanceNotifyService{}
	u := &User{
		BalanceNotifyExtraEmails: []NotifyEmailEntry{
			{Email: "User@Example.com", Verified: true},
			{Email: "user@example.com", Verified: true},
			{Email: "USER@EXAMPLE.COM", Verified: true},
		},
	}
	got := s.collectBalanceNotifyRecipients(u)
	require.Len(t, got, 1)
	// The original casing of the first entry is preserved.
	require.Equal(t, "User@Example.com", got[0])
}

func TestCollectBalanceNotifyRecipients_SkipsEmpty(t *testing.T) {
	s := &BalanceNotifyService{}
	u := &User{
		BalanceNotifyExtraEmails: []NotifyEmailEntry{
			{Email: "  ", Verified: true},
			{Email: "", Verified: true},
			{Email: "valid@example.com", Verified: true},
		},
	}
	got := s.collectBalanceNotifyRecipients(u)
	require.Equal(t, []string{"valid@example.com"}, got)
}

func TestCollectBalanceNotifyRecipients_IncludesAccountEmailFirst(t *testing.T) {
	s := &BalanceNotifyService{}
	u := &User{
		Email: "owner@example.com",
		BalanceNotifyExtraEmails: []NotifyEmailEntry{
			{Email: "Owner@Example.com", Verified: true}, // 与账号邮箱重复
			{Email: "extra@example.com", Verified: true},
			{Email: "pending@example.com", Verified: false},
		},
	}
	got := s.collectBalanceNotifyRecipients(u)
	require.Equal(t, []string{"owner@example.com", "extra@example.com"}, got)
}

func TestCollectBalanceNotifyRecipients_AccountEmailAloneStillNotified(t *testing.T) {
	s := &BalanceNotifyService{}
	u := &User{Email: "owner@example.com"}
	require.Equal(t, []string{"owner@example.com"}, s.collectBalanceNotifyRecipients(u))
}

func TestCollectBalanceNotifyRecipients_SkipsSyntheticAccountEmail(t *testing.T) {
	s := &BalanceNotifyService{}
	u := &User{
		Email: "12345" + WeChatConnectSyntheticEmailDomain,
		BalanceNotifyExtraEmails: []NotifyEmailEntry{
			{Email: "extra@example.com", Verified: true},
		},
	}
	require.Equal(t, []string{"extra@example.com"}, s.collectBalanceNotifyRecipients(u))
}

func TestCollectBalanceNotifyRecipients_TrimsWhitespace(t *testing.T) {
	s := &BalanceNotifyService{}
	u := &User{
		BalanceNotifyExtraEmails: []NotifyEmailEntry{
			{Email: "  trimmed@example.com  ", Verified: true},
		},
	}
	got := s.collectBalanceNotifyRecipients(u)
	require.Equal(t, []string{"trimmed@example.com"}, got)
}
