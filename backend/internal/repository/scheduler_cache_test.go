package repository

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestFilterSchedulerCredentialsKeepsSubscriptionPlanType(t *testing.T) {
	filtered := filterSchedulerCredentials(map[string]any{
		"plan_type":     "plus",
		"access_token":  "secret-access-token",
		"refresh_token": "secret-refresh-token",
	})

	require.Equal(t, "plus", filtered["plan_type"])
	require.NotContains(t, filtered, "access_token")
	require.NotContains(t, filtered, "refresh_token")
}

func TestSchedulerMetadataAccountKeepsOpenAISubscriptionIdentity(t *testing.T) {
	account := service.Account{
		ID:       24,
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeOAuth,
		Credentials: map[string]any{
			"plan_type":    "plus",
			"access_token": "secret-access-token",
		},
	}

	metadata := buildSchedulerMetadataAccount(account)

	require.True(t, metadata.IsOpenAIChatGPTSubscription())
	require.Empty(t, metadata.GetCredential("access_token"))
}

// 命中缓存的账号也要能做目录池的成员判定（accountInSchedulingScope 读 CatalogEntryIDs）。
func TestSchedulerMetadataAccountKeepsCatalogEntryIDs(t *testing.T) {
	account := service.Account{ID: 25, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, CatalogEntryIDs: []int64{7, 9}}

	metadata := buildSchedulerMetadataAccount(account)

	require.Equal(t, []int64{7, 9}, metadata.CatalogEntryIDs)
}

// 调度器只读 SchedulingState：快照投影必须让命中缓存的账号还原出与 DB 账号相同的状态
// （整体停调字段 + 模型级限流 + overages 放行开关）。
func TestSchedulerMetadataAccountReproducesSchedulingState(t *testing.T) {
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	until := now.Add(30 * time.Minute)
	account := service.Account{
		ID:                      31,
		Platform:                service.PlatformAntigravity,
		Type:                    service.AccountTypeOAuth,
		Status:                  service.StatusActive,
		Schedulable:             true,
		TempUnschedulableUntil:  &until,
		TempUnschedulableReason: "window_cost_limit",
		Extra: map[string]any{
			"allow_overages": true,
			"model_rate_limits": map[string]any{
				"claude-sonnet-4-5": map[string]any{"rate_limit_reset_at": now.Add(time.Hour).Format(time.RFC3339)},
			},
		},
	}

	metadata := buildSchedulerMetadataAccount(account)

	require.Equal(t, account.SchedulingState(now), metadata.SchedulingState(now))
	require.True(t, metadata.SchedulingState(now).ModelBlocksWaived)
}
