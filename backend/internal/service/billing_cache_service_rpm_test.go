//go:build unit

package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// userRPMCacheStub 记录每种计数器被调用的次数，并可注入返回值与错误。
type userRPMCacheStub struct {
	userGroupCalls int32
	userCalls      int32

	userCounts []int // 依次返回的计数值
	userErr    error
}

func (s *userRPMCacheStub) IncrementUserGroupRPM(_ context.Context, _, _ int64) (int, error) {
	atomic.AddInt32(&s.userGroupCalls, 1)
	return 1, nil
}

func (s *userRPMCacheStub) IncrementUserRPM(_ context.Context, _ int64) (int, error) {
	idx := int(atomic.AddInt32(&s.userCalls, 1)) - 1
	if s.userErr != nil {
		return 0, s.userErr
	}
	if idx < len(s.userCounts) {
		return s.userCounts[idx], nil
	}
	return 1, nil
}

func (s *userRPMCacheStub) GetUserGroupRPM(_ context.Context, _, _ int64) (int, error) {
	return 0, nil
}

func (s *userRPMCacheStub) GetUserRPM(_ context.Context, _ int64) (int, error) {
	return 0, nil
}

func newBillingServiceForRPM(t *testing.T, cache UserRPMCache) *BillingCacheService {
	t.Helper()
	// 用 nil BillingCache 走 "无缓存" 分支，避免 CheckBillingEligibility 副作用；只直接测 checkRPM。
	svc := NewBillingCacheService(nil, nil, nil, nil, cache, &config.Config{})
	t.Cleanup(svc.Stop)
	return svc
}

// RPM 只有用户级：users.rpm_limit 是唯一的一道门。
func TestBillingCacheService_CheckRPM_UserLimitOnly(t *testing.T) {
	cache := &userRPMCacheStub{userCounts: []int{1, 2, 3}}
	svc := newBillingServiceForRPM(t, cache)

	user := &User{ID: 1, RPMLimit: 2}
	require.NoError(t, svc.checkRPM(context.Background(), user))
	require.NoError(t, svc.checkRPM(context.Background(), user))
	require.ErrorIs(t, svc.checkRPM(context.Background(), user), ErrUserRPMExceeded)
	require.EqualValues(t, 3, atomic.LoadInt32(&cache.userCalls))
	require.EqualValues(t, 0, atomic.LoadInt32(&cache.userGroupCalls), "分组级计数器不再被碰")
}

// 分组 rpm_limit 随分组下线：key 所属分组设了限额也不生效（CheckBillingEligibility 传进来的 group 不进 RPM）。
func TestBillingCacheService_CheckRPM_GroupLimitIgnored(t *testing.T) {
	cache := &userRPMCacheStub{userCounts: []int{1, 2, 3, 4, 5}}
	svc := newBillingServiceForRPM(t, cache)

	user := &User{ID: 1, RPMLimit: 0}
	for i := 0; i < 5; i++ {
		require.NoError(t, svc.checkRPM(context.Background(), user))
	}
	require.EqualValues(t, 0, atomic.LoadInt32(&cache.userCalls), "用户级 0 = 不限，连计数都不做")
	require.EqualValues(t, 0, atomic.LoadInt32(&cache.userGroupCalls))
}

func TestBillingCacheService_CheckRPM_RedisErrorFailOpen(t *testing.T) {
	cache := &userRPMCacheStub{userErr: errors.New("redis unavailable")}
	svc := newBillingServiceForRPM(t, cache)

	user := &User{ID: 1, RPMLimit: 5}
	require.NoError(t, svc.checkRPM(context.Background(), user))
	require.EqualValues(t, 1, atomic.LoadInt32(&cache.userCalls))
}

func TestBillingCacheService_CheckRPM_NilUserIsNoop(t *testing.T) {
	cache := &userRPMCacheStub{}
	svc := newBillingServiceForRPM(t, cache)

	require.NoError(t, svc.checkRPM(context.Background(), nil))
	require.EqualValues(t, 0, atomic.LoadInt32(&cache.userCalls))
}
