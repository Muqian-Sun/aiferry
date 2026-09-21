//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type visibilityUserRepo struct {
	UserRepository
	user *User
	err  error
}

func (r *visibilityUserRepo) GetByID(context.Context, int64) (*User, error) { return r.user, r.err }

type visibilityGroupRepo struct {
	GroupRepository
	groups []Group
}

func (r *visibilityGroupRepo) ListActive(context.Context) ([]Group, error) { return r.groups, nil }

// 分组可见性只看 user_allowed_groups；订阅已脱离分组，不再授予任何分组的可见性 / 绑定资格。
func TestGetUserGroupVisibilityOnlyAllowedGroups(t *testing.T) {
	for _, restricted := range []bool{false, true} {
		t.Run(map[bool]string{false: "unrestricted", true: "restricted"}[restricted], func(t *testing.T) {
			svc := &APIKeyService{
				userRepo:  &visibilityUserRepo{user: &User{ID: 1, AllowedGroups: []int64{7}, RestrictPublicGroups: restricted}},
				groupRepo: &visibilityGroupRepo{groups: []Group{{ID: 42, IsExclusive: true}, {ID: 7, IsExclusive: true}}},
			}
			available, err := svc.GetAvailableGroups(context.Background(), 1)
			require.NoError(t, err)
			require.Len(t, available, 1)
			require.Equal(t, int64(7), available[0].ID, "专属分组只有明确授权的可绑")
			visible, restrict, err := svc.GetUserGroupVisibility(context.Background(), 1)
			require.NoError(t, err)
			require.Equal(t, restricted, restrict)
			require.Equal(t, map[int64]struct{}{7: {}}, visible)
		})
	}
}

func TestGetUserGroupVisibilityEmptyAndErrors(t *testing.T) {
	failure := errors.New("repository unavailable")
	for _, tc := range []struct {
		name    string
		userErr error
	}{
		{name: "empty"}, {name: "user failure", userErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &APIKeyService{userRepo: &visibilityUserRepo{user: &User{ID: 1}, err: tc.userErr}}
			got, _, err := svc.GetUserGroupVisibility(context.Background(), 1)
			if tc.userErr != nil {
				require.ErrorIs(t, err, failure)
				require.Nil(t, got, "repository failures must not become anonymous visibility")
			} else {
				require.NoError(t, err)
				require.NotNil(t, got, "an empty logged-in user must not become anonymous")
				require.Empty(t, got)
			}
		})
	}
}
