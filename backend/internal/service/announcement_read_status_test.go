//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type readStatusAnnouncementRepo struct {
	AnnouncementRepository
	ann *Announcement
}

func (r *readStatusAnnouncementRepo) GetByID(context.Context, int64) (*Announcement, error) {
	return r.ann, nil
}

type readStatusReadRepo struct {
	AnnouncementReadRepository
}

func (readStatusReadRepo) GetReadMapByUsers(context.Context, int64, []int64) (map[int64]time.Time, error) {
	return map[int64]time.Time{}, nil
}

// readStatusUserRepo 按 filters.Role 过滤，和真实仓储一样：不传角色就把管理员也列出来
type readStatusUserRepo struct {
	UserRepository
	users []User
}

func (r *readStatusUserRepo) ListWithFilters(_ context.Context, _ pagination.PaginationParams, filters UserListFilters) ([]User, *pagination.PaginationResult, error) {
	out := make([]User, 0, len(r.users))
	for _, u := range r.users {
		if filters.Role != "" && u.Role != filters.Role {
			continue
		}
		out = append(out, u)
	}
	return out, &pagination.PaginationResult{Total: int64(len(out)), Page: 1, PageSize: 20}, nil
}

type readStatusSubRepo struct {
	UserSubscriptionRepository
}

func (readStatusSubRepo) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	return nil, nil
}

// 公告已读情况只列普通用户：管理员登不了用户站，看不到公告，算进读者会把已读比例拉低（2026-10-04 走查 A28）
func TestListUserReadStatusExcludesAdmins(t *testing.T) {
	svc := NewAnnouncementService(
		&readStatusAnnouncementRepo{ann: &Announcement{ID: 1, Title: "t"}},
		readStatusReadRepo{},
		&readStatusUserRepo{users: []User{
			{ID: 1, Email: "root@example.com", Role: RoleAdmin},
			{ID: 2, Email: "alice@example.com", Role: RoleUser},
		}},
		readStatusSubRepo{},
	)

	rows, page, err := svc.ListUserReadStatus(context.Background(), 1, pagination.PaginationParams{Page: 1, PageSize: 20}, "")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(2), rows[0].UserID)
	require.Equal(t, int64(1), page.Total)
}
