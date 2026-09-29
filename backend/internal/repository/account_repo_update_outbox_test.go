package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbaccount "github.com/Wei-Shaw/sub2api/ent/account"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func TestLockAndMergeAccountProbeExtraProtectsOllamaManagedFields(t *testing.T) {
	for _, identityUnchanged := range []bool{true, false} {
		t.Run(map[bool]string{true: "same identity keeps snapshot", false: "changed identity clears snapshot"}[identityUnchanged], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			t.Cleanup(func() { _ = client.Close() })

			mock.ExpectQuery(`(?s)`+regexp.QuoteMeta("SELECT")+`.*`+regexp.QuoteMeta("FOR NO KEY UPDATE")).
				WithArgs(int64(29), service.AccountTypeAPIKey, `{"api_key":"key","base_url":"https://ollama.com"}`, nil, `{"anthropic":"https://ollama.com"}`).
				WillReturnRows(sqlmock.NewRows([]string{"ollama_group_unchanged", "ollama_proxy_unchanged", "ollama_session", "ollama_snapshot"}).
					AddRow(identityUnchanged, true, []byte(`"local-ciphertext"`), []byte(`{"status":"ok"}`)))

			account := &service.Account{
				ID: 29, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey,
				Credentials: map[string]any{"api_key": "key", "base_url": "https://ollama.com"},
				// 第三方 key 的上游地址来自协议映射，Ollama Cloud 判定读的是它。
				ProtocolEndpoints: map[string]string{
					service.APIProtocolAnthropic: "https://ollama.com",
				},
				Extra: map[string]any{
					service.OllamaCloudUsageSessionExtraKey:  "forged-ciphertext",
					service.OllamaCloudUsageSnapshotExtraKey: map[string]any{"status": "forged"},
				},
			}
			got, err := lockAndMergeAccountProbeExtra(context.Background(), client, account)
			require.NoError(t, err)
			if identityUnchanged {
				require.Equal(t, "local-ciphertext", got[service.OllamaCloudUsageSessionExtraKey])
				require.Equal(t, map[string]any{"status": "ok"}, got[service.OllamaCloudUsageSnapshotExtraKey])
			} else {
				require.NotContains(t, got, service.OllamaCloudUsageSessionExtraKey)
				require.NotContains(t, got, service.OllamaCloudUsageSnapshotExtraKey)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUpdateRollsBackWhenOutboxFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)`+regexp.QuoteMeta("SELECT")+`.*`+regexp.QuoteMeta("FOR NO KEY UPDATE")).
		WithArgs(int64(27), service.AccountTypeAPIKey, `{"api_key":"sk-test"}`, nil, `{"chat_completions":"https://api.openai.com","responses":"https://api.openai.com"}`).
		WillReturnRows(sqlmock.NewRows([]string{"ollama_group_unchanged", "ollama_proxy_unchanged", "ollama_session", "ollama_snapshot"}).
			AddRow(false, true, nil, nil))
	mock.ExpectExec(`(?s)UPDATE .*accounts.*SET.*WHERE .*id.*`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?s)SELECT .* FROM "accounts" WHERE "id" = \$1`).
		WithArgs(int64(27)).
		WillReturnRows(updatedAccountRows(27, `{}`))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO scheduler_outbox")).WillReturnError(errors.New("outbox failed"))
	mock.ExpectRollback()

	repo := newAccountRepositoryWithSQL(client, db, nil)
	account := &service.Account{
		ID:                27,
		Name:              "test",
		Platform:          service.PlatformOpenAI,
		Type:              service.AccountTypeAPIKey,
		Credentials:       map[string]any{"api_key": "sk-test"},
		Concurrency:       1,
		Priority:          1,
		Status:            service.StatusActive,
		Schedulable:       true,
		ProtocolEndpoints: map[string]string{service.APIProtocolChatCompletions: "https://api.openai.com", service.APIProtocolResponses: "https://api.openai.com"},
	}

	err = repo.Update(context.Background(), account)

	require.EqualError(t, err, "outbox failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateExtraRollsBackWhenOutboxFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts SET extra = COALESCE(extra, '{}'::jsonb) || $1::jsonb")).
		WithArgs(`{"base_rpm":10}`, int64(27)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO scheduler_outbox")).WillReturnError(errors.New("outbox failed"))
	mock.ExpectRollback()

	repo := newAccountRepositoryWithSQL(client, db, nil)
	err = repo.UpdateExtra(context.Background(), 27, map[string]any{"base_rpm": 10})

	require.EqualError(t, err, "outbox failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateCredentialsRollsBackWhenOutboxFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	mock.ExpectBegin()
	mock.ExpectExec(`(?s)UPDATE accounts.*credentials IS DISTINCT FROM \$1::jsonb`).
		WithArgs(`{"api_key":"sk-new"}`, int64(27)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO scheduler_outbox")).WillReturnError(errors.New("outbox failed"))
	mock.ExpectRollback()

	repo := newAccountRepositoryWithSQL(client, db, nil)
	err = repo.UpdateCredentials(context.Background(), 27, map[string]any{"api_key": "sk-new"})

	require.EqualError(t, err, "outbox failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBulkUpdateRollsBackWhenOutboxFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	name := "renamed"
	mock.ExpectBegin()
	mock.ExpectExec(`(?s)UPDATE accounts SET name = \$1.*WHERE id = ANY\(\$2\)`).
		WithArgs(name, `{27,28}`).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO scheduler_outbox")).WillReturnError(errors.New("outbox failed"))
	mock.ExpectRollback()

	repo := newAccountRepositoryWithSQL(client, db, nil)
	rows, err := repo.BulkUpdate(context.Background(), []int64{27, 28}, service.AccountBulkUpdate{Name: &name})

	require.EqualError(t, err, "outbox failed")
	require.Zero(t, rows)
	require.NoError(t, mock.ExpectationsWereMet())
}

func updatedAccountRows(id int64, extra string) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(dbaccount.Columns).AddRow(
		id, now, now, nil, "test", nil, service.PlatformOpenAI, service.AccountTypeAPIKey,
		[]byte(`{"api_key":"sk-test"}`), []byte(extra),
		// protocol_endpoints：ent 列序里紧跟 extra 之后
		[]byte(`{}`),
		nil, nil, 1, nil, 1, 1.0,
		service.StatusActive, nil, nil, nil, false, true, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, service.QuotaDimensionGlobal,
	)
}
