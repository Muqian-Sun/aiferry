//go:build unit

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newSchedulerCacheUnit(t *testing.T) *schedulerCache {
	cache, _ := newSchedulerCacheUnitWithRedis(t)
	return cache
}

func newSchedulerCacheUnitWithRedis(t *testing.T) (*schedulerCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache, ok := newSchedulerCacheWithChunkSizes(rdb, defaultSchedulerSnapshotMGetChunkSize, defaultSchedulerSnapshotWriteChunkSize).(*schedulerCache)
	require.True(t, ok)
	return cache, mr
}

func TestSchedulerCacheWriteAccountIDsSkipsUnencodableTimes(t *testing.T) {
	ctx := context.Background()
	cache := newSchedulerCacheUnit(t)
	invalidTime := time.Date(10000, time.January, 1, 0, 0, 0, 0, time.UTC)

	accountIDs, err := cache.writeAccountIDs(ctx, []service.Account{
		{ID: 111, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, ProtocolEndpoints: map[string]string{service.APIProtocolChatCompletions: "https://api.openai.com", service.APIProtocolResponses: "https://api.openai.com"}},
		{ID: 112, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, ExpiresAt: &invalidTime, ProtocolEndpoints: map[string]string{service.APIProtocolChatCompletions: "https://api.openai.com", service.APIProtocolResponses: "https://api.openai.com"}},
	})
	require.NoError(t, err)
	require.Equal(t, []int64{111}, accountIDs)

	cached, err := cache.GetAccount(ctx, 111)
	require.NoError(t, err)
	require.NotNil(t, cached)

	invalid, err := cache.GetAccount(ctx, 112)
	require.NoError(t, err)
	require.Nil(t, invalid)
}

func TestSchedulerCacheSetAccountClearsUnencodablePayload(t *testing.T) {
	ctx := context.Background()
	cache := newSchedulerCacheUnit(t)

	account := service.Account{ID: 113, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, ProtocolEndpoints: map[string]string{service.APIProtocolChatCompletions: "https://api.openai.com", service.APIProtocolResponses: "https://api.openai.com"}}
	require.NoError(t, cache.SetAccount(ctx, &account))

	invalidTime := time.Date(10000, time.January, 1, 0, 0, 0, 0, time.UTC)
	account.ExpiresAt = &invalidTime
	require.NoError(t, cache.SetAccount(ctx, &account))

	cached, err := cache.GetAccount(ctx, account.ID)
	require.NoError(t, err)
	require.Nil(t, cached)
}

func TestSchedulerCacheUpdateLastUsedClearsUnencodablePayload(t *testing.T) {
	ctx := context.Background()
	cache := newSchedulerCacheUnit(t)
	account := service.Account{ID: 114, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, ProtocolEndpoints: map[string]string{service.APIProtocolChatCompletions: "https://api.openai.com", service.APIProtocolResponses: "https://api.openai.com"}}
	require.NoError(t, cache.SetAccount(ctx, &account))

	invalidTime := time.Date(10000, time.January, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, cache.UpdateLastUsed(ctx, map[int64]time.Time{account.ID: invalidTime}))

	cached, err := cache.GetAccount(ctx, account.ID)
	require.NoError(t, err)
	require.Nil(t, cached)
}

func TestMarshalSchedulerCacheAccountKeepsEncodingJSONWireFormat(t *testing.T) {
	cases := []struct {
		name    string
		account service.Account
	}{
		{name: "nil collections", account: service.Account{ID: 801}},
		{name: "empty collections", account: service.Account{
			ID:          802,
			Credentials: map[string]any{},
			Extra:       map[string]any{},
		}},
		{name: "nested maps and escaping", account: service.Account{
			ID:          803,
			Credentials: map[string]any{"model_mapping": map[string]any{"z": "<last>", "a": "&first"}},
			Extra:       map[string]any{"mixed_scheduling": true},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			full, meta, err := marshalSchedulerCacheAccount(tc.account)
			require.NoError(t, err)
			wantFull, err := json.Marshal(tc.account)
			require.NoError(t, err)
			wantMeta, err := json.Marshal(buildSchedulerMetadataAccount(tc.account))
			require.NoError(t, err)
			require.Equal(t, wantFull, full)
			require.Equal(t, wantMeta, meta)
		})
	}
}

func TestBuildSchedulerMetadataAccount_KeepsOpenAIWSFlags(t *testing.T) {
	account := service.Account{
		ID:       42,
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeOAuth,
		Extra: map[string]any{
			"openai_oauth_responses_websockets_v2_enabled": true,
			"openai_oauth_responses_websockets_v2_mode":    service.OpenAIWSIngressModePassthrough,
			"openai_ws_force_http":                         true,
			// 已退役的 Responses 探测标记：没有读取方，不进调度投影。
			"openai_responses_mode":      "force_chat_completions",
			"openai_responses_supported": false,
			"codex_fingerprint_mode":     "session",
			"codex_fingerprint_seed":     "11111111-1111-4111-8111-111111111111",
			"mixed_scheduling":           true,
			"unused_large_field":         "drop-me",
		},
	}

	got := buildSchedulerMetadataAccount(account)

	require.Equal(t, true, got.Extra["openai_oauth_responses_websockets_v2_enabled"])
	require.Equal(t, service.OpenAIWSIngressModePassthrough, got.Extra["openai_oauth_responses_websockets_v2_mode"])
	require.Equal(t, true, got.Extra["openai_ws_force_http"])
	require.NotContains(t, got.Extra, "openai_responses_mode")
	require.NotContains(t, got.Extra, "openai_responses_supported")
	require.Equal(t, "session", got.Extra["codex_fingerprint_mode"])
	require.Equal(t, "11111111-1111-4111-8111-111111111111", got.Extra["codex_fingerprint_seed"])
	require.NotContains(t, got.Extra, "mixed_scheduling", "混合调度标记随分组池下线（7b-2b）")
	require.Nil(t, got.Extra["unused_large_field"])
}

func TestBuildSchedulerMetadataAccount_KeepsGrokMediaEligibility(t *testing.T) {
	t.Run("explicit override", func(t *testing.T) {
		account := service.Account{
			ID:       43,
			Platform: service.PlatformGrok,
			Type:     service.AccountTypeOAuth,
			Extra: map[string]any{
				service.GrokMediaEligibleExtraKey: false,
				"unused_large_field":              "drop-me",
			},
		}

		got := buildSchedulerMetadataAccount(account)

		eligible, reason := got.GrokMediaGenerationEligibility()
		require.False(t, eligible)
		require.Equal(t, "override_disabled", reason)
		require.Equal(t, false, got.Extra[service.GrokMediaEligibleExtraKey])
		require.Nil(t, got.Extra["unused_large_field"])
	})

	t.Run("forbidden billing observation", func(t *testing.T) {
		account := service.Account{
			ID:       44,
			Platform: service.PlatformGrok,
			Type:     service.AccountTypeOAuth,
			Extra: map[string]any{
				"grok_billing_snapshot": map[string]any{
					"status_code":         200,
					"weekly_status_code":  403,
					"monthly_status_code": 200,
				},
			},
		}

		got := buildSchedulerMetadataAccount(account)

		eligible, reason := got.GrokMediaGenerationEligibility()
		require.False(t, eligible)
		require.Equal(t, "billing_forbidden", reason)
		require.NotNil(t, got.Extra["grok_billing_snapshot"])
	})
}

func TestBuildSchedulerMetadataAccount_KeepsQuotaAutoPauseFields(t *testing.T) {
	account := service.Account{
		ID: 88,
		Extra: map[string]any{
			"codex_5h_used_percent":        12.34,
			"codex_7d_used_percent":        56.78,
			"codex_5h_reset_at":            "2026-05-29T10:00:00Z",
			"codex_7d_reset_at":            "2026-06-01T10:00:00Z",
			"codex_5h_reset_after_seconds": 300,
			"codex_7d_reset_after_seconds": 600,
			"codex_usage_updated_at":       "2026-05-29T09:00:00Z",
			"auto_pause_5h_threshold":      0.95,
			"auto_pause_7d_threshold":      0.96,
			"auto_pause_5h_disabled":       true,
			"auto_pause_7d_disabled":       false,
		},
	}

	got := buildSchedulerMetadataAccount(account)

	require.Equal(t, 12.34, got.Extra["codex_5h_used_percent"])
	require.Equal(t, 56.78, got.Extra["codex_7d_used_percent"])
	require.Equal(t, "2026-05-29T10:00:00Z", got.Extra["codex_5h_reset_at"])
	require.Equal(t, "2026-06-01T10:00:00Z", got.Extra["codex_7d_reset_at"])
	require.Equal(t, 300, got.Extra["codex_5h_reset_after_seconds"])
	require.Equal(t, 600, got.Extra["codex_7d_reset_after_seconds"])
	require.Equal(t, "2026-05-29T09:00:00Z", got.Extra["codex_usage_updated_at"])
	require.Equal(t, 0.95, got.Extra["auto_pause_5h_threshold"])
	require.Equal(t, 0.96, got.Extra["auto_pause_7d_threshold"])
	require.Equal(t, true, got.Extra["auto_pause_5h_disabled"])
	require.Equal(t, false, got.Extra["auto_pause_7d_disabled"])
}

// 配额计数不再由调度器评估（状态服务在用量入账时写成 temp_unschedulable），
// 命中缓存的账号只要状态字段没停就可调度；计数键仍进投影供展示 / 诊断。
func TestBuildSchedulerMetadataAccount_QuotaCountersDoNotBlockCachedAccounts(t *testing.T) {
	now := time.Now().UTC()
	activeStart := now.Add(-time.Hour).Format(time.RFC3339)
	expiredDailyStart := now.Add(-25 * time.Hour).Format(time.RFC3339)
	expiredWeeklyStart := now.Add(-8 * 24 * time.Hour).Format(time.RFC3339)

	cases := []struct {
		name          string
		platform      string
		typ           string
		extra         map[string]any
		quotaExceeded bool
	}{
		{
			name: "anthropic api key total quota exhausted", platform: service.PlatformAnthropic, typ: service.AccountTypeAPIKey,
			extra: map[string]any{"quota_limit": 10.0, "quota_used": 10.0}, quotaExceeded: true,
		},
		{
			name: "gemini api key rolling daily quota exhausted", platform: service.PlatformGemini, typ: service.AccountTypeAPIKey,
			extra: map[string]any{
				"quota_daily_limit": 20.0, "quota_daily_used": 20.0,
				"quota_daily_start": activeStart,
			}, quotaExceeded: true,
		},
		{
			name: "gemini api key expired rolling daily window", platform: service.PlatformGemini, typ: service.AccountTypeAPIKey,
			extra: map[string]any{
				"quota_daily_limit": 20.0, "quota_daily_used": 20.0,
				"quota_daily_start": expiredDailyStart,
			},
		},
		{
			name: "bedrock rolling weekly quota exhausted", platform: service.PlatformAnthropic, typ: service.AccountTypeBedrock,
			extra: map[string]any{
				"quota_weekly_limit": 30.0, "quota_weekly_used": 30.0, "quota_weekly_start": activeStart,
			}, quotaExceeded: true,
		},
		{
			name: "bedrock expired rolling weekly window", platform: service.PlatformAnthropic, typ: service.AccountTypeBedrock,
			extra: map[string]any{
				"quota_weekly_limit": 30.0, "quota_weekly_used": 30.0, "quota_weekly_start": expiredWeeklyStart,
			},
		},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			extra := make(map[string]any, len(tc.extra)+1)
			for key, value := range tc.extra {
				extra[key] = value
			}
			extra["unrelated"] = "drop me"
			account := service.Account{
				ID: int64(46690 + i), Platform: tc.platform, Type: tc.typ, Extra: extra,
				Status: service.StatusActive, Schedulable: true,
			}
			cache := newSchedulerCacheUnit(t)
			ctx := context.Background()
			bucket := service.SchedulerBucket{PoolID: int64(46690 + i), Platform: tc.platform, Mode: service.SchedulerModeSingle}
			token, err := cache.CaptureBucketWriteToken(ctx, bucket)
			require.NoError(t, err)
			require.NoError(t, cache.SetSnapshot(ctx, bucket, token, []service.Account{account}))

			snapshot, hit, err := cache.GetSnapshot(ctx, bucket)
			require.NoError(t, err)
			require.True(t, hit)
			require.Len(t, snapshot, 1)
			cached := snapshot[0]
			require.Equal(t, tc.extra, cached.Extra)
			require.NotContains(t, cached.Extra, "unrelated")
			require.True(t, cached.IsSchedulable(), "quota_exceeded=%v 也不在调度器里判", tc.quotaExceeded)
		})
	}
}

func TestBuildSchedulerMetadataAccount_KeepsModelRateLimits(t *testing.T) {
	account := service.Account{
		ID:       90,
		Platform: service.PlatformAntigravity,
		Extra: map[string]any{
			"model_rate_limits": map[string]any{
				"gemini-3-flash": map[string]any{
					"rate_limit_reset_at": "2026-05-30T10:10:00Z",
				},
				"antigravity:gemini": map[string]any{
					"rate_limit_reset_at": "2026-05-30T10:10:00Z",
				},
			},
			"unused_large_field": "drop-me",
		},
	}

	got := buildSchedulerMetadataAccount(account)

	limits, ok := got.Extra["model_rate_limits"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, limits, "gemini-3-flash")
	require.Contains(t, limits, "antigravity:gemini")
	require.Nil(t, got.Extra["unused_large_field"])
}

func TestBuildSchedulerMetadataAccount_KeepsSparkShadowRoutingIdentity(t *testing.T) {
	parentID := int64(100)
	account := service.Account{
		ID:              200,
		Platform:        service.PlatformOpenAI,
		Type:            service.AccountTypeOAuth,
		ParentAccountID: &parentID,
		QuotaDimension:  service.QuotaDimensionSpark,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"gpt-5.3-codex-spark": "gpt-5.3-codex-spark",
			},
			"compact_model_mapping": map[string]any{
				"gpt-5.4": "gpt-5.4-openai-compact",
			},
			"access_token": "drop-me",
		},
	}

	got := buildSchedulerMetadataAccount(account)

	require.NotNil(t, got.ParentAccountID)
	require.Equal(t, parentID, *got.ParentAccountID)
	require.Equal(t, service.QuotaDimensionSpark, got.QuotaDimension)
	require.Equal(t, map[string]any{"gpt-5.3-codex-spark": "gpt-5.3-codex-spark"}, got.Credentials["model_mapping"])
	require.Equal(t, map[string]any{"gpt-5.4": "gpt-5.4-openai-compact"}, got.Credentials["compact_model_mapping"])
	require.Nil(t, got.Credentials["access_token"])
}

var schedulerCachePayloadBenchmarkSink int

func BenchmarkSchedulerCacheAccountPayloadReuse(b *testing.B) {
	for _, size := range []int{1, 100, 10_000} {
		accounts := schedulerCacheBenchmarkAccounts(size)
		b.Run(fmt.Sprintf("pair_baseline_%d_accounts", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				first, err := benchmarkSchedulerLegacySnapshotPayload(accounts)
				if err != nil {
					b.Fatal(err)
				}
				second, err := benchmarkSchedulerLegacySnapshotPayload(accounts)
				if err != nil {
					b.Fatal(err)
				}
				schedulerCachePayloadBenchmarkSink = first + second
			}
		})
		b.Run(fmt.Sprintf("pair_reuse_%d_accounts", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				ids, total, err := benchmarkSchedulerReusableSnapshotPayload(accounts)
				if err != nil {
					b.Fatal(err)
				}
				// 第二个桶仍构造成员，只跳过账号 JSON 与全局账号键。
				total += len(schedulerSnapshotMembers(ids))
				schedulerCachePayloadBenchmarkSink = total
			}
		})
		b.Run(fmt.Sprintf("first_baseline_%d_accounts", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				total, err := benchmarkSchedulerLegacySnapshotPayload(accounts)
				if err != nil {
					b.Fatal(err)
				}
				schedulerCachePayloadBenchmarkSink = total
			}
		})
		b.Run(fmt.Sprintf("first_reuse_%d_accounts", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				ids, total, err := benchmarkSchedulerReusableSnapshotPayload(accounts)
				if err != nil {
					b.Fatal(err)
				}
				total += len(ids)
				schedulerCachePayloadBenchmarkSink = total
			}
		})
	}
}

func benchmarkSchedulerLegacySnapshotPayload(accounts []service.Account) (int, error) {
	cacheable := make([]service.Account, 0, len(accounts))
	total := 0
	for _, account := range accounts {
		full, meta, err := marshalSchedulerCacheAccount(account)
		if err != nil {
			continue
		}
		total += len(full) + len(meta)
		cacheable = append(cacheable, account)
	}
	members := make([]redis.Z, 0, len(cacheable))
	for idx, account := range cacheable {
		members = append(members, redis.Z{Score: float64(idx), Member: strconv.FormatInt(account.ID, 10)})
	}
	return total + len(members), nil
}

func benchmarkSchedulerReusableSnapshotPayload(accounts []service.Account) ([]int64, int, error) {
	accountIDs := make([]int64, 0, len(accounts))
	total := 0
	for _, account := range accounts {
		full, meta, err := marshalSchedulerCacheAccount(account)
		if err != nil {
			continue
		}
		total += len(full) + len(meta)
		accountIDs = append(accountIDs, account.ID)
	}
	total += len(schedulerSnapshotMembers(accountIDs))
	return accountIDs, total, nil
}

func schedulerCacheBenchmarkAccounts(size int) []service.Account {
	largeValue := strings.Repeat("x", 4096)
	credentials := map[string]any{
		"api_key":       "benchmark-key",
		"model_mapping": map[string]any{"z-model": "z-target", "a-model": "a-target"},
		"large_value":   largeValue,
	}
	extra := map[string]any{
		"mixed_scheduling": true,
		"model_rate_limits": map[string]any{
			"z-model": map[string]any{"rate_limit_reset_at": "2026-07-16T00:00:00Z"},
			"a-model": map[string]any{"rate_limit_reset_at": "2026-07-16T00:00:00Z"},
		},
		"large_value": largeValue,
	}
	accounts := make([]service.Account, size)
	for i := range accounts {
		id := int64(i + 1)
		accounts[i] = service.Account{
			ID:          id,
			Name:        "benchmark-account",
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeOAuth,
			Credentials: credentials,
			Extra:       extra,
		}
	}
	return accounts
}

// 调度投影必须保留 OpenAI 透传开关。
//
// 候选过滤走 ListSchedulableAccounts，读的是 buildSchedulerMetadataAccount 产出的精简投影；
// Account.IsModelSupported 又靠 extra 上的透传开关短路 model_mapping 白名单（#4936）。
// 一旦投影把开关裁掉、却保留了白名单，透传账号在选号阶段就会退回白名单判定并被误判成
// model_not_supported，而转发阶段（读完整账号）仍按透传工作 —— 表现为"单独测这个账号能通、
// 走网关却报 no available accounts"。#4936 修的是判定逻辑，这里守的是喂给判定的输入。
func TestBuildSchedulerMetadataAccount_KeepsOpenAIPassthroughForModelGate(t *testing.T) {
	for _, key := range []string{"openai_passthrough", "openai_oauth_passthrough"} {
		t.Run(key, func(t *testing.T) {
			account := service.Account{
				ID:       383,
				Platform: service.PlatformOpenAI,
				Type:     service.AccountTypeOAuth,
				Credentials: map[string]any{
					// 账号从白名单模式切到透传后常见的残留映射，未列出请求的模型。
					"model_mapping": map[string]any{"gpt-5.5": "gpt-5.5"},
					"access_token":  "drop-me",
				},
				Extra: map[string]any{key: true},
			}
			require.True(t, account.IsModelSupported("gpt-5.6-sol"),
				"前置条件：透传账号本应放行白名单外的模型")

			meta := buildSchedulerMetadataAccount(account)

			// 走一遍真实的序列化/反序列化路径（写入 sched:meta 再由 decodeCachedAccount 读回）。
			payload, err := json.Marshal(meta)
			require.NoError(t, err)
			var restored service.Account
			require.NoError(t, json.Unmarshal(payload, &restored))

			require.Equal(t, true, restored.Extra[key])
			require.True(t, restored.IsOpenAIPassthroughEnabled())
			require.True(t, restored.IsModelSupported("gpt-5.6-sol"),
				"投影裁掉透传开关会让透传账号在候选过滤阶段被误判为 model_not_supported")
			// 白名单本身仍需保留：非透传账号依赖它做模型门。
			require.Equal(t, map[string]any{"gpt-5.5": "gpt-5.5"}, restored.Credentials["model_mapping"])
		})
	}
}

// 目录桶的绑定优先级：账号元数据全局共享（装的是账号自身优先级），目录桶用 ZSET score 存
// 绑定生效的优先级，命中缓存时写回 Priority；分组桶不受影响。
func TestSchedulerCacheCatalogBucketKeepsBindingPriority(t *testing.T) {
	ctx := context.Background()
	cache := newSchedulerCacheUnit(t)
	key := func(id int64, priority int) service.Account {
		return service.Account{
			ID: id, Name: fmt.Sprintf("key-%d", id), Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
			Status: service.StatusActive, Schedulable: true, Priority: priority,
			ProtocolEndpoints: map[string]string{service.APIProtocolChatCompletions: "https://api.openai.com"},
		}
	}

	catalog := service.SchedulerBucket{PoolID: 7, Platform: service.PlatformOpenAI, Mode: service.SchedulerModeCatalog}
	token, err := cache.CaptureBucketWriteToken(ctx, catalog)
	require.NoError(t, err)
	// 绑定生效优先级 9 / 1，账号自身都是 50（元数据里就是 50）。
	require.NoError(t, cache.SetSnapshot(ctx, catalog, token, []service.Account{key(901, 9), key(902, 1)}))

	got, hit, err := cache.GetSnapshot(ctx, catalog)
	require.NoError(t, err)
	require.True(t, hit)
	require.Len(t, got, 2)
	require.Equal(t, int64(902), got[0].ID, "lower score first")
	require.Equal(t, 1, got[0].Priority)
	require.Equal(t, int64(901), got[1].ID)
	require.Equal(t, 9, got[1].Priority)

	// 同两个账号写进分组桶，账号自身优先级 50 才是元数据里的值。
	single := service.SchedulerBucket{PoolID: 7, Platform: service.PlatformOpenAI, Mode: service.SchedulerModeSingle}
	token, err = cache.CaptureBucketWriteToken(ctx, single)
	require.NoError(t, err)
	require.NoError(t, cache.SetSnapshot(ctx, single, token, []service.Account{key(901, 50), key(902, 50)}))

	got, hit, err = cache.GetSnapshot(ctx, single)
	require.NoError(t, err)
	require.True(t, hit)
	require.Equal(t, []int64{901, 902}, []int64{got[0].ID, got[1].ID}, "group buckets keep insertion order")
	require.Equal(t, 50, got[0].Priority)
	require.Equal(t, 50, got[1].Priority)

	got, hit, err = cache.GetSnapshot(ctx, catalog)
	require.NoError(t, err)
	require.True(t, hit)
	require.Equal(t, 1, got[0].Priority, "the catalog bucket's priority survives the shared metadata rewrite")
	require.Equal(t, 9, got[1].Priority)
}

// 退役标记已随分组生命周期删除（D19）：Redis 里残留的 sched:retired: 键不再拦截写入。
func TestSchedulerCacheCaptureNoLongerFencedByRetiredKey(t *testing.T) {
	ctx := context.Background()
	cache, mr := newSchedulerCacheUnitWithRedis(t)
	bucket := service.SchedulerBucket{Platform: service.PlatformOpenAI, Mode: service.SchedulerModeSingle}

	require.NoError(t, mr.Set("sched:retired:"+bucket.String(), "3"))

	token, err := cache.CaptureBucketWriteToken(ctx, bucket)
	require.NoError(t, err)
	require.Positive(t, token.Epoch)

	require.NoError(t, cache.SetSnapshot(ctx, bucket, token, []service.Account{
		{ID: 1, Platform: service.PlatformOpenAI, Status: service.StatusActive, Schedulable: true},
	}))
	accounts, hit, err := cache.GetSnapshot(ctx, bucket)
	require.NoError(t, err)
	require.True(t, hit)
	require.Len(t, accounts, 1)
}
