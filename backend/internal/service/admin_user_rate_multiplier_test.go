//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type userRepoStubForRateMultiplier struct {
	userRepoStubForListUsers
	created *User
	updated *User
	fields  UserUpdateFields
}

func (r *userRepoStubForRateMultiplier) Create(_ context.Context, user *User) error {
	r.created = user
	return nil
}

func (r *userRepoStubForRateMultiplier) GetByID(_ context.Context, id int64) (*User, error) {
	return &User{ID: id, Role: RoleUser, RateMultiplier: 1}, nil
}

func (r *userRepoStubForRateMultiplier) Update(_ context.Context, user *User, fields UserUpdateFields) error {
	r.updated = user
	r.fields = fields
	return nil
}

func TestAdminService_CreateUser_RateMultiplierDefaultsToOneAndRejectsNegative(t *testing.T) {
	repo := &userRepoStubForRateMultiplier{}
	svc := &adminServiceImpl{userRepo: repo}

	_, err := svc.CreateUser(context.Background(), &CreateUserInput{Email: "a@example.com", Password: "secret123"})
	require.NoError(t, err)
	require.Equal(t, 1.0, repo.created.RateMultiplier)

	half := 0.5
	_, err = svc.CreateUser(context.Background(), &CreateUserInput{Email: "b@example.com", Password: "secret123", RateMultiplier: &half})
	require.NoError(t, err)
	require.Equal(t, 0.5, repo.created.RateMultiplier)

	negative := -1.0
	_, err = svc.CreateUser(context.Background(), &CreateUserInput{Email: "c@example.com", Password: "secret123", RateMultiplier: &negative})
	require.ErrorContains(t, err, "rate_multiplier must be >= 0")
}

func TestAdminService_UpdateUser_RateMultiplierZeroMeansFree(t *testing.T) {
	repo := &userRepoStubForRateMultiplier{}
	svc := &adminServiceImpl{userRepo: repo}

	zero := 0.0
	_, err := svc.UpdateUser(context.Background(), 7, &UpdateUserInput{RateMultiplier: &zero})
	require.NoError(t, err)
	require.True(t, repo.fields.RateMultiplier)
	require.Equal(t, 0.0, repo.updated.RateMultiplier)

	_, err = svc.UpdateUser(context.Background(), 7, &UpdateUserInput{})
	require.NoError(t, err)
	require.False(t, repo.fields.RateMultiplier, "unset input must not touch the column")
}
