package service

import (
	"bytes"
	"context"
	"errors"
	"log"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type subscriptionExpiryRepoStub struct {
	listCalls int
}

func (r *subscriptionExpiryRepoStub) Create(context.Context, *UserSubscription) error {
	return nil
}

func (r *subscriptionExpiryRepoStub) GetByID(context.Context, int64) (*UserSubscription, error) {
	return nil, ErrSubscriptionNotFound
}

func (r *subscriptionExpiryRepoStub) GetByIDForUpdate(context.Context, int64) (*UserSubscription, error) {
	return nil, ErrSubscriptionNotFound
}

func (r *subscriptionExpiryRepoStub) GetByIDIncludeDeleted(context.Context, int64) (*UserSubscription, error) {
	return nil, ErrSubscriptionNotFound
}

func (r *subscriptionExpiryRepoStub) GetByUserIDAndPlanID(context.Context, int64, int64) (*UserSubscription, error) {
	return nil, ErrSubscriptionNotFound
}

func (r *subscriptionExpiryRepoStub) GetActiveByUserIDAndPlanID(context.Context, int64, int64) (*UserSubscription, error) {
	return nil, ErrSubscriptionNotFound
}

func (r *subscriptionExpiryRepoStub) Update(context.Context, *UserSubscription) error {
	return nil
}

func (r *subscriptionExpiryRepoStub) Delete(context.Context, int64) error {
	return nil
}

func (r *subscriptionExpiryRepoStub) Restore(context.Context, int64, string) (*UserSubscription, error) {
	return nil, ErrSubscriptionNotFound
}

func (r *subscriptionExpiryRepoStub) ListByUserID(context.Context, int64) ([]UserSubscription, error) {
	return nil, nil
}

func (r *subscriptionExpiryRepoStub) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	return nil, nil
}

func (r *subscriptionExpiryRepoStub) List(context.Context, pagination.PaginationParams, *int64, *int64, string, string, string) ([]UserSubscription, *pagination.PaginationResult, error) {
	r.listCalls++
	return nil, &pagination.PaginationResult{Page: 1, Pages: 1}, nil
}

func (r *subscriptionExpiryRepoStub) ExistsByUserIDAndPlanID(context.Context, int64, int64) (bool, error) {
	return false, nil
}

func (r *subscriptionExpiryRepoStub) ExtendExpiry(context.Context, int64, time.Time) error {
	return nil
}

func (r *subscriptionExpiryRepoStub) UpdateStatus(context.Context, int64, string) error {
	return nil
}

func (r *subscriptionExpiryRepoStub) UpdateNotes(context.Context, int64, string) error {
	return nil
}

func (r *subscriptionExpiryRepoStub) ActivateWindows(context.Context, int64, time.Time, time.Time) error {
	return nil
}

func (r *subscriptionExpiryRepoStub) ResetUsageWindows(context.Context, int64, bool, bool, bool, time.Time, time.Time) error {
	return nil
}

func (r *subscriptionExpiryRepoStub) ResetDailyUsage(context.Context, int64, *time.Time, time.Time) error {
	return nil
}

func (r *subscriptionExpiryRepoStub) ResetWeeklyUsage(context.Context, int64, *time.Time, time.Time) error {
	return nil
}

func (r *subscriptionExpiryRepoStub) ResetMonthlyUsage(context.Context, int64, *time.Time, time.Time) error {
	return nil
}

func (r *subscriptionExpiryRepoStub) IncrementUsage(context.Context, int64, float64) error {
	return nil
}

func (r *subscriptionExpiryRepoStub) BatchUpdateExpiredStatus(context.Context) (int64, error) {
	return 0, nil
}

type subscriptionExpirySettingRepoStub struct {
	values   map[string]string
	err      error
	multiErr error
}

func (r *subscriptionExpirySettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (r *subscriptionExpirySettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *subscriptionExpirySettingRepoStub) Set(context.Context, string, string) error {
	return nil
}

func (r *subscriptionExpirySettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	if r.multiErr != nil {
		return nil, r.multiErr
	}
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			values[key] = value
		}
	}
	return values, nil
}

func (r *subscriptionExpirySettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	return nil
}

func (r *subscriptionExpirySettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return nil, nil
}

func (r *subscriptionExpirySettingRepoStub) Delete(context.Context, string) error {
	return nil
}

// 订阅到期提醒跟着 SMTP 走：库里旧开关关着也照常扫描。
func TestSubscriptionExpiryService_StaleSwitchDoesNotDisableReminder(t *testing.T) {
	repo := &subscriptionExpiryRepoStub{}
	settingRepo := &subscriptionExpirySettingRepoStub{values: map[string]string{"subscription_expiry_notify_enabled": "false"}}
	svc := NewSubscriptionExpiryService(repo, time.Minute)
	svc.SetSettingRepository(settingRepo)
	svc.SetNotificationEmailService(NewNotificationEmailService(settingRepo, NewEmailService(settingRepo, nil, smtpConfiguredForTest()), nil))
	svc.SetLeaderLock(&fakeLeaderLockCache{}, nil)

	svc.sendExpiryReminders(context.Background())

	require.Equal(t, 1, repo.listCalls)
}

func TestSubscriptionExpiryService_MissingSMTPSkipsReminderScanAndLogsOncePerInterval(t *testing.T) {
	repo := &subscriptionExpiryRepoStub{}
	settingRepo := &subscriptionExpirySettingRepoStub{values: map[string]string{}}
	emailService := NewEmailService(settingRepo, nil, nil)
	svc := NewSubscriptionExpiryService(repo, time.Minute)
	svc.SetSettingRepository(settingRepo)
	svc.SetNotificationEmailService(NewNotificationEmailService(settingRepo, emailService, nil))

	var logs bytes.Buffer
	previousWriter := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&logs)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
	})

	svc.sendExpiryReminders(context.Background())
	svc.sendExpiryReminders(context.Background())

	require.Zero(t, repo.listCalls)
	require.Equal(t, 1, bytes.Count(logs.Bytes(), []byte("SMTP is not configured")))
}

func TestSubscriptionExpiryService_SMTPConfigReadErrorSkipsReminderScan(t *testing.T) {
	repo := &subscriptionExpiryRepoStub{}
	settingRepo := &subscriptionExpirySettingRepoStub{
		values:   map[string]string{},
		multiErr: errors.New("db down"),
	}
	emailService := NewEmailService(settingRepo, nil, nil)
	svc := NewSubscriptionExpiryService(repo, time.Minute)
	svc.SetSettingRepository(settingRepo)
	svc.SetNotificationEmailService(NewNotificationEmailService(settingRepo, emailService, nil))

	svc.sendExpiryReminders(context.Background())

	require.Zero(t, repo.listCalls)
}

func (*subscriptionExpiryRepoStub) GetActiveByID(context.Context, int64) (*UserSubscription, error) {
	return nil, ErrSubscriptionNotFound
}
