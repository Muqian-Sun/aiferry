//go:build unit

package service_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/authidentity"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

type authIdentitySettingRepoStub struct {
	values map[string]string
}

func (s *authIdentitySettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	panic("unexpected Get call")
}

func (s *authIdentitySettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", service.ErrSettingNotFound
}

func (s *authIdentitySettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *authIdentitySettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if v, ok := s.values[key]; ok {
			out[key] = v
		}
	}
	return out, nil
}

func (s *authIdentitySettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *authIdentitySettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *authIdentitySettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func newAuthServiceWithEnt(
	t *testing.T,
	settings map[string]string,
	defaultSubAssigner service.DefaultSubscriptionAssigner,
) (*service.AuthService, service.UserRepository, *dbent.Client) {
	t.Helper()

	db, err := sql.Open("sqlite", "file:auth_service_identity_sync?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	repo := repository.NewUserRepository(client, db)
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:     "test-auth-identity-secret",
			ExpireHour: 1,
		},
	}
	settingSvc := service.NewSettingService(&authIdentitySettingRepoStub{
		values: settings,
	}, cfg)

	svc := service.NewAuthService(client, repo, nil, nil, cfg, settingSvc, nil, nil, nil, defaultSubAssigner, nil)
	return svc, repo, client
}

func TestAuthServiceRegisterDualWritesEmailIdentity(t *testing.T) {
	svc, _, client := newAuthServiceWithEnt(t, map[string]string{}, nil)
	ctx := context.Background()

	token, user, err := svc.Register(ctx, "user@qq.com", "password")
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.NotNil(t, user)

	storedUser, err := client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, "email", storedUser.SignupSource)
	require.NotNil(t, storedUser.LastLoginAt)
	require.NotNil(t, storedUser.LastActiveAt)

	identity, err := client.AuthIdentity.Query().
		Where(
			authidentity.ProviderTypeEQ("email"),
			authidentity.ProviderKeyEQ("email"),
			authidentity.ProviderSubjectEQ("user@qq.com"),
		).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, user.ID, identity.UserID)
	require.NotNil(t, identity.VerifiedAt)
}

func TestAuthServiceLoginDefersLastLoginTouchUntilRecordSuccessfulLogin(t *testing.T) {
	svc, _, client := newAuthServiceWithEnt(t, map[string]string{}, nil)
	ctx := context.Background()

	passwordHash, err := svc.HashPassword("password")
	require.NoError(t, err)
	user, err := client.User.Create().
		SetEmail("login@example.com").
		SetPasswordHash(passwordHash).
		SetRole(service.RoleUser).
		SetStatus(service.StatusActive).
		SetBalance(1).
		SetConcurrency(1).
		Save(ctx)
	require.NoError(t, err)

	old := time.Now().Add(-2 * time.Hour).UTC().Round(time.Second)
	_, err = client.User.UpdateOneID(user.ID).
		SetLastLoginAt(old).
		SetLastActiveAt(old).
		Save(ctx)
	require.NoError(t, err)

	token, gotUser, err := svc.Login(ctx, user.Email, "password")
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.NotNil(t, gotUser)

	storedUser, err := client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, storedUser.LastLoginAt)
	require.NotNil(t, storedUser.LastActiveAt)
	require.True(t, storedUser.LastLoginAt.Equal(old))
	require.True(t, storedUser.LastActiveAt.Equal(old))

	identityCount, err := client.AuthIdentity.Query().
		Where(
			authidentity.ProviderTypeEQ("email"),
			authidentity.ProviderKeyEQ("email"),
			authidentity.ProviderSubjectEQ("login@example.com"),
		).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, identityCount)

	svc.RecordSuccessfulLogin(ctx, user.ID)

	identity, err := client.AuthIdentity.Query().
		Where(
			authidentity.ProviderTypeEQ("email"),
			authidentity.ProviderKeyEQ("email"),
			authidentity.ProviderSubjectEQ("login@example.com"),
		).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, user.ID, identity.UserID)
}

func TestAuthServiceRecordSuccessfulLoginBackfillsEmailIdentity(t *testing.T) {
	svc, repo, client := newAuthServiceWithEnt(t, map[string]string{}, nil)
	ctx := context.Background()

	user := &service.User{
		Email:       "record@example.com",
		Role:        service.RoleUser,
		Status:      service.StatusActive,
		Balance:     1,
		Concurrency: 1,
	}
	require.NoError(t, user.SetPassword("password"))
	require.NoError(t, repo.Create(ctx, user))

	svc.RecordSuccessfulLogin(ctx, user.ID)

	identity, err := client.AuthIdentity.Query().
		Where(
			authidentity.ProviderTypeEQ("email"),
			authidentity.ProviderKeyEQ("email"),
			authidentity.ProviderSubjectEQ("record@example.com"),
		).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, user.ID, identity.UserID)
}

// 老用户（注册时没写邮箱身份）登录成功后补写一条邮箱身份。
func TestAuthServiceLogin_BackfillsLegacyEmailIdentity(t *testing.T) {
	svc, _, client := newAuthServiceWithEnt(t, map[string]string{}, nil)
	ctx := context.Background()

	passwordHash, err := svc.HashPassword("password")
	require.NoError(t, err)
	user, err := client.User.Create().
		SetEmail("legacy@example.com").
		SetUsername("legacy-user").
		SetPasswordHash(passwordHash).
		SetRole(service.RoleUser).
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	token, gotUser, err := svc.Login(ctx, user.Email, "password")
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.NotNil(t, gotUser)
	svc.RecordSuccessfulLogin(ctx, user.ID)

	identityCount, err := client.AuthIdentity.Query().
		Where(
			authidentity.ProviderTypeEQ("email"),
			authidentity.ProviderKeyEQ("email"),
			authidentity.ProviderSubjectEQ("legacy@example.com"),
		).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, identityCount)
}
