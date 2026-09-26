//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminService_CreateUser_Success(t *testing.T) {
	repo := &userRepoStub{nextID: 10}
	svc := &adminServiceImpl{userRepo: repo}
	balance := 12.5

	input := &CreateUserInput{
		Email:       "user@test.com",
		Password:    "strong-pass",
		Username:    "tester",
		Notes:       "note",
		Balance:     &balance,
		Concurrency: ptrInt(7),
	}

	user, err := svc.CreateUser(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, user)
	require.Equal(t, int64(10), user.ID)
	require.Equal(t, input.Email, user.Email)
	require.Equal(t, input.Username, user.Username)
	require.Equal(t, input.Notes, user.Notes)
	require.Equal(t, balance, user.Balance)
	require.Equal(t, 7, user.Concurrency)
	require.Equal(t, RoleUser, user.Role)
	require.Equal(t, StatusActive, user.Status)
	require.True(t, user.CheckPassword(input.Password))
	require.Len(t, repo.created, 1)
	require.Equal(t, user, repo.created[0])
}

// 管理员新建用户不传余额 / 并发 / RPM 时，和自助注册一样取「新用户默认值」（site_features.go）。
func TestAdminService_CreateUser_UsesNewUserDefaultsWhenOmitted(t *testing.T) {
	repo := &userRepoStub{nextID: 11}
	svc := &adminServiceImpl{userRepo: repo}

	user, err := svc.CreateUser(context.Background(), &CreateUserInput{
		Email:    "default-limits@test.com",
		Password: "strong-pass",
	})

	require.NoError(t, err)
	require.Equal(t, NewUserBalance, user.Balance)
	require.Equal(t, NewUserConcurrency, user.Concurrency)
	require.Equal(t, NewUserRPMLimit, user.RPMLimit)
	require.Len(t, repo.created, 1)
	require.Equal(t, NewUserConcurrency, repo.created[0].Concurrency)
}

func TestAdminService_CreateUser_ExplicitValuesOverrideNewUserDefaults(t *testing.T) {
	repo := &userRepoStub{nextID: 12}
	svc := &adminServiceImpl{userRepo: repo}
	balance := 1.5

	user, err := svc.CreateUser(context.Background(), &CreateUserInput{
		Email:       "explicit-limits@test.com",
		Password:    "strong-pass",
		Balance:     &balance,
		Concurrency: ptrInt(0),
		RPMLimit:    ptrInt(30),
	})

	require.NoError(t, err)
	require.Equal(t, 1.5, user.Balance)
	require.Equal(t, 0, user.Concurrency)
	require.Equal(t, 30, user.RPMLimit)
}

func TestAdminService_CreateUser_EmailExists(t *testing.T) {
	repo := &userRepoStub{createErr: ErrEmailExists}
	svc := &adminServiceImpl{userRepo: repo}

	_, err := svc.CreateUser(context.Background(), &CreateUserInput{
		Email:    "dup@test.com",
		Password: "password",
	})
	require.ErrorIs(t, err, ErrEmailExists)
	require.Empty(t, repo.created)
}

func TestAdminService_CreateUser_CreateError(t *testing.T) {
	createErr := errors.New("db down")
	repo := &userRepoStub{createErr: createErr}
	svc := &adminServiceImpl{userRepo: repo}

	_, err := svc.CreateUser(context.Background(), &CreateUserInput{
		Email:    "user@test.com",
		Password: "password",
	})
	require.ErrorIs(t, err, createErr)
	require.Empty(t, repo.created)
}

func TestAdminService_CreateUser_AssignsNewUserDefaultSubscriptions(t *testing.T) {
	withNewUserDefaultSubscriptions(t, []DefaultSubscriptionSetting{{PlanID: 5, ValidityDays: 30}})
	repo := &userRepoStub{nextID: 21}
	assigner := &defaultSubscriptionAssignerStub{}
	svc := &adminServiceImpl{userRepo: repo, defaultSubAssigner: assigner}

	_, err := svc.CreateUser(context.Background(), &CreateUserInput{
		Email:    "new-user@test.com",
		Password: "password",
	})
	require.NoError(t, err)
	require.Len(t, assigner.calls, 1)
	require.Equal(t, int64(21), assigner.calls[0].UserID)
	require.Equal(t, int64(5), assigner.calls[0].PlanID)
	require.Equal(t, 30, assigner.calls[0].ValidityDays)
}

// 默认赠送套餐写死为空：新建用户不发任何订阅。
func TestAdminService_CreateUser_NoDefaultSubscriptionsByDefault(t *testing.T) {
	repo := &userRepoStub{nextID: 22}
	assigner := &defaultSubscriptionAssignerStub{}
	svc := &adminServiceImpl{userRepo: repo, defaultSubAssigner: assigner}

	_, err := svc.CreateUser(context.Background(), &CreateUserInput{
		Email:    "no-subs@test.com",
		Password: "password",
	})
	require.NoError(t, err)
	require.Empty(t, assigner.calls)
}

// withNewUserDefaultSubscriptions 临时改「新用户默认赠送套餐」（代码里写死为空），测完恢复。
func withNewUserDefaultSubscriptions(t *testing.T, items []DefaultSubscriptionSetting) {
	t.Helper()
	previous := newUserDefaultSubscriptions
	newUserDefaultSubscriptions = items
	t.Cleanup(func() { newUserDefaultSubscriptions = previous })
}
