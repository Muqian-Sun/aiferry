//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type resetAccountQuotaRepoStub struct {
	mockAccountRepoForGemini
	account             *Account
	getByIDErr          error
	resetErr            error
	resetCalls          int
	clearRateLimitCalls int
	callOrder           []string
	overloaded          bool
}

func (r *resetAccountQuotaRepoStub) GetByID(context.Context, int64) (*Account, error) {
	return r.account, r.getByIDErr
}

func (r *resetAccountQuotaRepoStub) ResetQuotaUsedAndClearRateLimitCooldown(context.Context, int64) error {
	r.resetCalls++
	r.callOrder = append(r.callOrder, "reset_quota_and_clear_rate_limit_cooldown")
	return r.resetErr
}

func (r *resetAccountQuotaRepoStub) ClearTempUnschedulable(context.Context, int64) error {
	r.callOrder = append(r.callOrder, "clear_temp_unschedulable")
	return nil
}

func (r *resetAccountQuotaRepoStub) ClearRateLimit(context.Context, int64) error {
	r.clearRateLimitCalls++
	r.callOrder = append(r.callOrder, "clear_rate_limit")
	r.overloaded = false
	return nil
}

func TestResetAccountQuota_ClearsSchedulerRateLimitWithoutClearingOverload(t *testing.T) {
	repo := &resetAccountQuotaRepoStub{account: &Account{ID: 42}, overloaded: true}
	svc := &adminServiceImpl{accountRepo: repo}

	err := svc.ResetAccountQuota(context.Background(), 42)

	require.NoError(t, err)
	require.Equal(t, 1, repo.resetCalls)
	require.Zero(t, repo.clearRateLimitCalls)
	require.Equal(t, []string{"reset_quota_and_clear_rate_limit_cooldown"}, repo.callOrder)
	require.True(t, repo.overloaded, "quota reset must preserve an unrelated overload block")
}

func TestResetAccountQuota_PreservesLookupAndSparkShadowShortCircuits(t *testing.T) {
	t.Run("lookup failure", func(t *testing.T) {
		getErr := errors.New("get account failed")
		repo := &resetAccountQuotaRepoStub{getByIDErr: getErr}
		svc := &adminServiceImpl{accountRepo: repo}

		err := svc.ResetAccountQuota(context.Background(), 42)

		require.ErrorIs(t, err, getErr)
		require.Zero(t, repo.resetCalls)
		require.Zero(t, repo.clearRateLimitCalls)
	})

	t.Run("spark shadow", func(t *testing.T) {
		parentID := int64(7)
		repo := &resetAccountQuotaRepoStub{
			account: &Account{ID: 42, ParentAccountID: &parentID},
		}
		svc := &adminServiceImpl{accountRepo: repo}

		err := svc.ResetAccountQuota(context.Background(), 42)

		require.Error(t, err)
		require.Zero(t, repo.resetCalls)
		require.Zero(t, repo.clearRateLimitCalls)
	})
}

func TestResetAccountQuota_PropagatesAtomicRepositoryFailure(t *testing.T) {
	resetErr := errors.New("atomic reset failed")
	repo := &resetAccountQuotaRepoStub{
		account:  &Account{ID: 42},
		resetErr: resetErr,
	}
	svc := &adminServiceImpl{accountRepo: repo}

	err := svc.ResetAccountQuota(context.Background(), 42)

	require.ErrorIs(t, err, resetErr)
	require.Equal(t, 1, repo.resetCalls)
	require.Zero(t, repo.clearRateLimitCalls)
}

// 总额度超限由状态服务写成 temp_unschedulable（停到管理员重置）：重置配额要一并解除；别的原因写的停调不动。
func TestResetAccountQuota_ClearsQuotaCounterPauseOnly(t *testing.T) {
	future := time.Now().Add(time.Hour)
	t.Run("quota counter pause cleared", func(t *testing.T) {
		repo := &resetAccountQuotaRepoStub{account: &Account{ID: 42, TempUnschedulableUntil: &future,
			TempUnschedulableReason: BuildTempUnschedReasonPayload(quotaCounterSource, "total quota 10 used >= 10")}}
		svc := &adminServiceImpl{accountRepo: repo}
		require.NoError(t, svc.ResetAccountQuota(context.Background(), 42))
		require.Equal(t, []string{"reset_quota_and_clear_rate_limit_cooldown", "clear_temp_unschedulable"}, repo.callOrder)
	})
	t.Run("other pause preserved", func(t *testing.T) {
		repo := &resetAccountQuotaRepoStub{account: &Account{ID: 42, TempUnschedulableUntil: &future,
			TempUnschedulableReason: BuildTempUnschedReasonPayload(grokFreeQuotaSource, "xai free tier local usage 480000 tokens >= soft gate 450000")}}
		svc := &adminServiceImpl{accountRepo: repo}
		require.NoError(t, svc.ResetAccountQuota(context.Background(), 42))
		require.Equal(t, []string{"reset_quota_and_clear_rate_limit_cooldown"}, repo.callOrder)
	})
}
