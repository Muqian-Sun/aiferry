//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type batchLimitsUserRepoStub struct {
	*userRepoStub
	calls          int
	userIDs        []int64
	concurrency    *int
	rpmLimit       *int
	rateMultiplier *float64
	useDefaultRate bool
	affected       int
	err            error
}

func (s *batchLimitsUserRepoStub) BatchUpdateLimits(_ context.Context, userIDs []int64, concurrency, rpmLimit *int, rateMultiplier *float64, useDefaultRate bool) (int, error) {
	s.calls++
	s.userIDs = append([]int64(nil), userIDs...)
	s.concurrency = cloneBatchLimitValue(concurrency)
	s.rpmLimit = cloneBatchLimitValue(rpmLimit)
	if rateMultiplier != nil {
		v := *rateMultiplier
		s.rateMultiplier = &v
	}
	s.useDefaultRate = useDefaultRate
	if s.affected == 0 && s.err == nil {
		s.affected = len(s.userIDs)
	}
	return s.affected, s.err
}

func cloneBatchLimitValue(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func TestAdminServiceBatchUpdateLimitsPassesOnlyProvidedFields(t *testing.T) {
	concurrency := 0
	repo := &batchLimitsUserRepoStub{
		userRepoStub: &userRepoStub{},
		affected:     2,
	}
	invalidator := &authCacheInvalidatorStub{}
	service := &adminServiceImpl{userRepo: repo, authCacheInvalidator: invalidator}

	affected, err := service.BatchUpdateLimits(
		context.Background(),
		[]int64{3, 0, 3, 7, -1},
		&concurrency,
		nil,
		nil,
		false,
	)

	require.NoError(t, err)
	require.Equal(t, 2, affected)
	require.Equal(t, []int64{3, 7}, repo.userIDs)
	require.Equal(t, pointerToInt(0), repo.concurrency)
	require.Nil(t, repo.rpmLimit)
	require.Equal(t, []int64{3, 7}, invalidator.userIDs)
}

func TestAdminServiceBatchUpdateLimitsDoesNotInvalidateCacheOnRepositoryError(t *testing.T) {
	rpmLimit := 60
	repo := &batchLimitsUserRepoStub{
		userRepoStub: &userRepoStub{},
		err:          errors.New("database unavailable"),
	}
	invalidator := &authCacheInvalidatorStub{}
	service := &adminServiceImpl{userRepo: repo, authCacheInvalidator: invalidator}

	affected, err := service.BatchUpdateLimits(context.Background(), []int64{1, 2}, nil, &rpmLimit, nil, false)

	require.EqualError(t, err, "database unavailable")
	require.Zero(t, affected)
	require.Empty(t, invalidator.userIDs)
}

func TestAdminServiceBatchUpdateLimitsRequiresAField(t *testing.T) {
	repo := &batchLimitsUserRepoStub{userRepoStub: &userRepoStub{}}
	service := &adminServiceImpl{userRepo: repo, authCacheInvalidator: &authCacheInvalidatorStub{}}

	affected, err := service.BatchUpdateLimits(context.Background(), []int64{1}, nil, nil, nil, false)

	require.Error(t, err)
	require.Zero(t, affected)
	require.Zero(t, repo.calls)
}

func pointerToInt(value int) *int {
	return &value
}

func TestAdminServiceBatchUpdateLimitsRateMultiplier(t *testing.T) {
	repo := &batchLimitsUserRepoStub{userRepoStub: &userRepoStub{}}
	service := &adminServiceImpl{userRepo: repo, authCacheInvalidator: &authCacheInvalidatorStub{}}

	half := 0.5
	affected, err := service.BatchUpdateLimits(context.Background(), []int64{1}, nil, nil, &half, false)
	require.NoError(t, err)
	require.Equal(t, 1, affected)
	require.NotNil(t, repo.rateMultiplier)
	require.Equal(t, 0.5, *repo.rateMultiplier)

	negative := -1.0
	_, err = service.BatchUpdateLimits(context.Background(), []int64{1}, nil, nil, &negative, false)
	require.ErrorContains(t, err, "rate_multiplier must be >= 0")
}

// 批量把倍率改回全站默认（清掉单独设的值）：只认「改回默认」，与指定倍率互斥
func TestAdminServiceBatchUpdateLimitsUseDefaultRate(t *testing.T) {
	repo := &batchLimitsUserRepoStub{userRepoStub: &userRepoStub{}}
	service := &adminServiceImpl{userRepo: repo}

	affected, err := service.BatchUpdateLimits(context.Background(), []int64{1, 2}, nil, nil, nil, true)
	require.NoError(t, err)
	require.Equal(t, 2, affected)
	require.True(t, repo.useDefaultRate)
	require.Nil(t, repo.rateMultiplier)

	half := 0.5
	_, err = service.BatchUpdateLimits(context.Background(), []int64{1}, nil, nil, &half, true)
	require.Error(t, err)
	require.Equal(t, 1, repo.calls)
}
