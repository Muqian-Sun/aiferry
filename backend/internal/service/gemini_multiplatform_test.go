//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

// mockAccountRepoForGemini Gemini 测试用的 mock
type mockAccountRepoForGemini struct {
	accounts           []Account
	accountsByID       map[int64]*Account
	listByGroupFunc    func(ctx context.Context, groupID int64, platforms []string) ([]Account, error)
	listByPlatformFunc func(ctx context.Context, platforms []string) ([]Account, error)
}

func (m *mockAccountRepoForGemini) GetByID(ctx context.Context, id int64) (*Account, error) {
	if acc, ok := m.accountsByID[id]; ok {
		return acc, nil
	}
	return nil, errors.New("account not found")
}

func (m *mockAccountRepoForGemini) GetByIDs(ctx context.Context, ids []int64) ([]*Account, error) {
	var result []*Account
	for _, id := range ids {
		if acc, ok := m.accountsByID[id]; ok {
			result = append(result, acc)
		}
	}
	return result, nil
}

func (m *mockAccountRepoForGemini) ExistsByID(ctx context.Context, id int64) (bool, error) {
	if m.accountsByID == nil {
		return false, nil
	}
	_, ok := m.accountsByID[id]
	return ok, nil
}

func (m *mockAccountRepoForGemini) ListSchedulableByPlatform(ctx context.Context, platform string) ([]Account, error) {
	var result []Account
	for _, acc := range m.accounts {
		if acc.Platform == platform && acc.IsSchedulable() {
			result = append(result, acc)
		}
	}
	return result, nil
}

func (m *mockAccountRepoForGemini) Create(ctx context.Context, account *Account) error { return nil }
func (m *mockAccountRepoForGemini) FindByExtraField(ctx context.Context, key string, value any) ([]Account, error) {
	return nil, nil
}

func (m *mockAccountRepoForGemini) Update(ctx context.Context, account *Account) error { return nil }
func (m *mockAccountRepoForGemini) Delete(ctx context.Context, id int64) error         { return nil }
func (m *mockAccountRepoForGemini) List(ctx context.Context, params pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (m *mockAccountRepoForGemini) ListWithFilters(ctx context.Context, params pagination.PaginationParams, platform, accountType, status, search string, privacyMode string) ([]Account, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (m *mockAccountRepoForGemini) ListAllWithFilters(ctx context.Context, platform, accountType, status, search string, privacyMode string) ([]Account, error) {
	return nil, nil
}
func (m *mockAccountRepoForGemini) ListActive(ctx context.Context) ([]Account, error) {
	return nil, nil
}
func (m *mockAccountRepoForGemini) ListByPlatform(ctx context.Context, platform string) ([]Account, error) {
	return nil, nil
}
func (m *mockAccountRepoForGemini) UpdateLastUsed(ctx context.Context, id int64) error { return nil }
func (m *mockAccountRepoForGemini) BatchUpdateLastUsed(ctx context.Context, updates map[int64]time.Time) error {
	return nil
}
func (m *mockAccountRepoForGemini) SetError(ctx context.Context, id int64, errorMsg string) error {
	return nil
}
func (m *mockAccountRepoForGemini) ClearError(ctx context.Context, id int64) error {
	return nil
}
func (m *mockAccountRepoForGemini) SetSchedulable(ctx context.Context, id int64, schedulable bool) error {
	return nil
}
func (m *mockAccountRepoForGemini) AutoPauseExpiredAccounts(ctx context.Context, now time.Time) (int64, error) {
	return 0, nil
}
func (m *mockAccountRepoForGemini) ListSchedulable(ctx context.Context) ([]Account, error) {
	return nil, nil
}
func (m *mockAccountRepoForGemini) ListSchedulingCandidates(ctx context.Context, platforms []string) ([]Account, error) {
	if m.listByPlatformFunc != nil {
		return m.listByPlatformFunc(ctx, platforms)
	}
	var result []Account
	platformSet := make(map[string]bool)
	for _, p := range platforms {
		platformSet[p] = true
	}
	for _, acc := range m.accounts {
		if (platformSet[acc.Platform] || acc.IsThirdPartyKey()) && acc.IsSchedulable() {
			result = append(result, acc)
		}
	}
	return result, nil
}
func (m *mockAccountRepoForGemini) ListSchedulingCandidatesByCatalogEntry(context.Context, int64) ([]Account, error) {
	return nil, nil
}

func (m *mockAccountRepoForGemini) ListModelAvailabilityCandidates(ctx context.Context, platforms []string) ([]Account, error) {
	return m.ListSchedulingCandidates(ctx, platforms)
}
func (m *mockAccountRepoForGemini) SetRateLimited(ctx context.Context, id int64, resetAt time.Time) error {
	return nil
}
func (m *mockAccountRepoForGemini) SetModelRateLimit(ctx context.Context, id int64, scope string, resetAt time.Time, reason ...string) error {
	return nil
}
func (m *mockAccountRepoForGemini) SetOverloaded(ctx context.Context, id int64, until time.Time) error {
	return nil
}
func (m *mockAccountRepoForGemini) SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error {
	return nil
}
func (m *mockAccountRepoForGemini) ClearTempUnschedulable(ctx context.Context, id int64) error {
	return nil
}
func (m *mockAccountRepoForGemini) ClearRateLimit(ctx context.Context, id int64) error { return nil }
func (m *mockAccountRepoForGemini) ClearAntigravityQuotaScopes(ctx context.Context, id int64) error {
	return nil
}
func (m *mockAccountRepoForGemini) ClearModelRateLimits(ctx context.Context, id int64) error {
	return nil
}
func (m *mockAccountRepoForGemini) UpdateSessionWindow(ctx context.Context, id int64, start, end *time.Time, status string) error {
	return nil
}
func (m *mockAccountRepoForGemini) UpdateSessionWindowEnd(ctx context.Context, id int64, end time.Time) error {
	return nil
}
func (m *mockAccountRepoForGemini) UpdateExtra(ctx context.Context, id int64, updates map[string]any) error {
	return nil
}
func (m *mockAccountRepoForGemini) BulkUpdate(ctx context.Context, ids []int64, updates AccountBulkUpdate) (int64, error) {
	return 0, nil
}

func (m *mockAccountRepoForGemini) IncrementQuotaUsed(ctx context.Context, id int64, amount float64) error {
	return nil
}

func (m *mockAccountRepoForGemini) ResetQuotaUsedAndClearRateLimitCooldown(ctx context.Context, id int64) error {
	return nil
}

func (m *mockAccountRepoForGemini) RevertProxyFallback(ctx context.Context, accountID int64) error {
	return nil
}

func (m *mockAccountRepoForGemini) ListShadowsByParent(ctx context.Context, parentID int64) ([]*Account, error) {
	return nil, nil
}

// Verify interface implementation
var _ AccountRepository = (*mockAccountRepoForGemini)(nil)

// mockGatewayCacheForGemini Gemini 测试用的 cache mock
type mockGatewayCacheForGemini struct {
	sessionBindings map[string]int64
	deletedSessions map[string]int
}

func (m *mockGatewayCacheForGemini) GetSessionAccountID(ctx context.Context, groupID int64, sessionHash string) (int64, error) {
	if id, ok := m.sessionBindings[sessionHash]; ok {
		return id, nil
	}
	return 0, errors.New("not found")
}

func (m *mockGatewayCacheForGemini) SetSessionAccountID(ctx context.Context, groupID int64, sessionHash string, accountID int64, ttl time.Duration) error {
	if m.sessionBindings == nil {
		m.sessionBindings = make(map[string]int64)
	}
	m.sessionBindings[sessionHash] = accountID
	return nil
}

func (m *mockGatewayCacheForGemini) RefreshSessionTTL(ctx context.Context, groupID int64, sessionHash string, ttl time.Duration) error {
	return nil
}

func (m *mockGatewayCacheForGemini) DeleteSessionAccountID(ctx context.Context, groupID int64, sessionHash string) error {
	if m.sessionBindings == nil {
		return nil
	}
	if m.deletedSessions == nil {
		m.deletedSessions = make(map[string]int)
	}
	m.deletedSessions[sessionHash]++
	delete(m.sessionBindings, sessionHash)
	return nil
}

func (m *mockGatewayCacheForGemini) SetGrokVideoPendingBilling(_ context.Context, _ string, _ []byte, _ time.Duration) error {
	return nil
}
func (m *mockGatewayCacheForGemini) GetGrokVideoPendingBilling(_ context.Context, _ string) ([]byte, error) {
	return nil, nil
}
func (m *mockGatewayCacheForGemini) ClaimGrokVideoBilled(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return true, nil
}

func (m *mockGatewayCacheForGemini) ReleaseGrokVideoBilled(_ context.Context, _ string) error {
	return nil
}

func (m *mockGatewayCacheForGemini) SetReasoningContent(_ context.Context, _ string, _ string, _ time.Duration) error {
	return nil
}
func (m *mockGatewayCacheForGemini) GetReasoningContent(_ context.Context, _ string) (string, error) {
	return "", ErrReasoningContentNotFound
}

// TestGeminiMessagesCompatService_SelectAccountForModelWithExclusions_GeminiPlatform 测试 Gemini 单平台选择
// TestGeminiPlatformRouting_DocumentRouteDecision 测试平台路由决策逻辑
func TestGeminiPlatformRouting_DocumentRouteDecision(t *testing.T) {
	tests := []struct {
		name            string
		platform        string
		expectedService string // "gemini" 表示 ForwardNative, "antigravity" 表示 ForwardGemini
	}{
		{
			name:            "Gemini平台走ForwardNative",
			platform:        PlatformGemini,
			expectedService: "gemini",
		},
		{
			name:            "Antigravity平台走ForwardGemini",
			platform:        PlatformAntigravity,
			expectedService: "antigravity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{Platform: tt.platform}

			// 模拟 Handler 层的路由逻辑
			var serviceName string
			if account.Platform == PlatformAntigravity {
				serviceName = "antigravity"
			} else {
				serviceName = "gemini"
			}

			require.Equal(t, tt.expectedService, serviceName,
				"平台 %s 应该路由到 %s 服务", tt.platform, tt.expectedService)
		})
	}
}
