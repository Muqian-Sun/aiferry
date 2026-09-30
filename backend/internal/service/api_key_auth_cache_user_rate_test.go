package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// 缓存命中的请求也按用户倍率计费：快照往返必须把 users.rate_multiplier 带回来，漏了就按 0 免费。
func TestAPIKeyAuthSnapshot_KeepsUserRateMultiplier(t *testing.T) {
	apiKey := &APIKey{
		ID: 83, UserID: 41, Key: "sk-user-rate-roundtrip", Status: StatusActive,
		User: &User{ID: 41, Status: StatusActive, RateMultiplier: customRate(0.5)},
	}
	svc := &APIKeyService{}

	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: svc.snapshotFromAPIKey(context.Background(), apiKey)})
	require.NoError(t, err)
	var cached APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &cached))

	materialized, used, err := svc.applyAuthCacheEntry(apiKey.Key, &cached)
	require.NoError(t, err)
	require.True(t, used)
	require.NotNil(t, materialized.User)
	require.Equal(t, 0.5, *materialized.User.RateMultiplier)
}
