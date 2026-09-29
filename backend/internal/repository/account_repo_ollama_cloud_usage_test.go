package repository

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func newOllamaCloudUsageRepositoryTestClient(t *testing.T) (*dbent.Client, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	return client, mock
}

func ollamaCloudUsageRepositoryAccount() *service.Account {
	return &service.Account{
		ID: 17, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "key", "base_url": "https://ollama.com"},
		// 第三方 key 的上游地址来自协议映射，Ollama Cloud 判定读的是它。
		ProtocolEndpoints: map[string]string{
			service.APIProtocolChatCompletions: "https://ollama.com",
		},
		Extra: map[string]any{
			service.OllamaCloudUsageSessionExtraKey: "cipher:wos-session=secret",
		},
	}
}

func TestUpdateOllamaCloudUsageSnapshotRowsAffectedZeroIsIdentityConflict(t *testing.T) {
	client, mock := newOllamaCloudUsageRepositoryTestClient(t)
	mock.ExpectBegin()
	expectOllamaCloudUsageGroupLock(mock, ollamaCloudUsageRepositoryAccount(), true,
		`"cipher:wos-session=secret"`, `null`)
	mock.ExpectExec(`(?s)`+regexp.QuoteMeta("UPDATE accounts")).
		WithArgs(sqlmock.AnyArg(), "key", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	repo := newAccountRepositoryWithSQL(client, nil, nil)

	err := repo.UpdateOllamaCloudUsageSnapshot(context.Background(), ollamaCloudUsageRepositoryAccount(), &service.OllamaCloudUsageSnapshot{
		Status:        service.OllamaCloudUsageStatusOK,
		LastAttemptAt: time.Now(),
		NextRefreshAt: time.Now().Add(time.Hour),
	})

	require.ErrorIs(t, err, service.ErrOllamaCloudUsageIdentityChanged)
	require.NoError(t, mock.ExpectationsWereMet())
}

func expectOllamaCloudUsageGroupLock(
	mock sqlmock.Sqlmock,
	account *service.Account,
	anchorMatches bool,
	sessionJSON, snapshotJSON string,
) {
	apiKey, _ := account.Credentials["api_key"].(string)
	credentials, _ := json.Marshal(normalizeJSONMap(account.Credentials))
	var proxyID any
	if account.ProxyID != nil {
		proxyID = *account.ProxyID
	}
	mock.ExpectQuery(`(?s)`+regexp.QuoteMeta("SELECT")+`.*`+regexp.QuoteMeta("FOR NO KEY UPDATE")).
		WithArgs(apiKey, account.ID, account.Platform, account.Type, string(credentials), proxyID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "anchor_matches", "session", "snapshot"}).
			AddRow(account.ID, anchorMatches, sessionJSON, snapshotJSON))
}

func TestOllamaCloudUsageManagedWriteRejectsChangedProxyIdentity(t *testing.T) {
	client, mock := newOllamaCloudUsageRepositoryTestClient(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)` + regexp.QuoteMeta("SELECT protocol, host, port") + `.*` + regexp.QuoteMeta("FOR SHARE")).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"protocol", "host", "port", "username", "password", "status"}).
			AddRow("http", "new.example", 3128, "user", "pass", service.StatusActive))
	mock.ExpectRollback()

	account := ollamaCloudUsageRepositoryAccount()
	proxyID := int64(9)
	account.ProxyID = &proxyID
	account.Proxy = &service.Proxy{
		ID: proxyID, Protocol: "http", Host: "old.example", Port: 3128,
		Username: "user", Password: "pass", Status: service.StatusActive,
	}
	repo := newAccountRepositoryWithSQL(client, nil, nil)

	err := repo.SaveOllamaCloudUsageSession(context.Background(), account, "cipher:wos-session=replacement")

	require.ErrorIs(t, err, service.ErrOllamaCloudUsageIdentityChanged)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSaveAndDeleteOllamaCloudUsageSessionKeepCiphertextOutOfSQL(t *testing.T) {
	var capturedSQL []string
	matcher := sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		capturedSQL = append(capturedSQL, actualSQL)
		return sqlmock.QueryMatcherRegexp.Match(expectedSQL, actualSQL)
	})
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	repo := newAccountRepositoryWithSQL(client, db, nil)
	account := ollamaCloudUsageRepositoryAccount()
	const replacement = "cipher:wos-session=browser-cookie-secret"

	mock.ExpectBegin()
	expectOllamaCloudUsageGroupLock(mock, account, true, `"cipher:wos-session=secret"`, `null`)
	mock.ExpectExec(`(?s)UPDATE accounts.*ollama_cloud_usage_session.*ollama_cloud_usage_snapshot`).
		WithArgs(`{"ollama_cloud_usage_session":"cipher:wos-session=browser-cookie-secret"}`, "key", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, repo.SaveOllamaCloudUsageSession(context.Background(), account, replacement))

	account.Extra[service.OllamaCloudUsageSessionExtraKey] = replacement
	mock.ExpectBegin()
	expectOllamaCloudUsageGroupLock(mock, account, true, `"cipher:wos-session=browser-cookie-secret"`, `null`)
	mock.ExpectExec(`(?s)UPDATE accounts.*ollama_cloud_usage_session.*ollama_cloud_usage_snapshot`).
		WithArgs(`{}`, "key", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, repo.DeleteOllamaCloudUsageSession(context.Background(), account))

	require.NotEmpty(t, capturedSQL)
	for _, query := range capturedSQL {
		require.NotContains(t, query, "browser-cookie-secret")
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOllamaCloudBaseURLSQLRegexMatchesServiceSemantics(t *testing.T) {
	for _, baseURL := range []string{
		"https://ollama.com",
		"HTTPS://WWW.OLLAMA.COM:443/v1",
		"https://ollama.com/V1",
		"https://ollama.com/v1/",
		"https://ollama.com.evil.test/v1",
	} {
		t.Run(baseURL, func(t *testing.T) {
			matched, err := regexp.MatchString(ollamaCloudBaseURLRegexSQL, baseURL)
			require.NoError(t, err)
			account := ollamaCloudUsageRepositoryAccount()
			account.Credentials["base_url"] = baseURL
			account.ProtocolEndpoints[service.APIProtocolChatCompletions] = baseURL
			require.Equal(t, service.IsOllamaCloudUsageAccount(account), matched)
		})
	}
}

func TestListOllamaCloudUsageGroupAccountsUsesOneStrictBatchQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	var capturedSQL string
	mock.ExpectQuery("SELECT id").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	repo := newAccountRepositoryWithSQL(nil, captureQuerySQL{db: db, captured: &capturedSQL}, nil)
	first := ollamaCloudUsageRepositoryAccount()
	second := ollamaCloudUsageRepositoryAccount()
	second.ID = 18
	second.Platform = service.PlatformAnthropic
	second.Credentials = map[string]any{"api_key": "key", "base_url": "https://www.ollama.com:443/v1"}
	second.ProtocolEndpoints = map[string]string{
		service.APIProtocolAnthropic: "https://www.ollama.com:443/v1",
	}

	accounts, err := repo.ListOllamaCloudUsageGroupAccounts(context.Background(), []*service.Account{first, second})

	require.NoError(t, err)
	require.Empty(t, accounts)
	query := normalizeSQLWhitespace(capturedSQL)
	require.Contains(t, query, "credentials ->> 'api_key' = ANY($1)")
	require.NotContains(t, query, "platform IN", "Ollama Cloud grouping must not look at the display label")
	require.Contains(t, query, "type = 'apikey'")
	require.Contains(t, query, "jsonb_typeof(credentials -> 'api_key') = 'string'")
	require.Contains(t, query, ollamaCloudBaseURLMatchesSQL(ollamaCloudPrimaryEndpointSQL("protocol_endpoints")))
	// 第三方 key 的地址只在 protocol_endpoints 里，读 credentials.base_url 会让分组静默失效。
	require.NotContains(t, query, "->> 'base_url'")
	require.NotContains(t, query, "~*")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListDueOllamaCloudUsageAccountsFiltersOrdersAndLimits(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	now := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)
	debounce := time.Minute
	maxWait := time.Hour
	var capturedSQL string
	mock.ExpectQuery("WITH eligible AS").
		WithArgs(now.UTC(), debounce.Seconds(), maxWait.Seconds(), 20, service.OllamaCloudUsageMinFetchInterval.Seconds()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "group_last_used_at"}))
	repo := newAccountRepositoryWithSQL(nil, captureQuerySQL{db: db, captured: &capturedSQL}, nil)

	accounts, err := repo.ListDueOllamaCloudUsageAccounts(context.Background(), now, debounce, maxWait, 20)

	require.NoError(t, err)
	require.Empty(t, accounts)
	normalized := normalizeSQLWhitespace(capturedSQL)
	for _, clause := range []string{
		"deleted_at IS NULL",
		"status = 'active'",
		"type = 'apikey'",
		"type = 'apikey'",
		ollamaCloudBaseURLMatchesSQL(ollamaCloudPrimaryEndpointSQL("protocol_endpoints")),
		"jsonb_typeof(extra -> 'ollama_cloud_usage_session') = 'string'",
		"MAX(last_used_at) AS group_last_used_at",
		"PARTITION BY api_key",
		"WHERE group_rank = 1",
		"LIMIT $4",
		"make_interval(secs => $2::double precision)",
		"make_interval(secs => $3::double precision)",
		// Minimum interval floor between successful fetches.
		"make_interval(secs => $5::double precision)",
		// jsonpath .datetime() only accepts the ISO-8601 "Z" designator from
		// PostgreSQL 17 on, and this service writes UTC timestamps. Without this
		// rewrite every parsed_* column is NULL on 14-16 and the due filter
		// collapses into its fail-open branch.
		`regexp_replace( regexp_replace( fetched_at, '(\.[0-9]{6})[0-9]+(Z|[+-][0-9]{2}:[0-9]{2})$', '\1\2' ), 'Z$', '+00:00' )`,
		"group_last_used_at > parsed_fetched_at::timestamptz",
		"group_last_used_at > parsed_last_attempt_at::timestamptz",
		"$1 >= activity_due_at",
		"COALESCE(parsed_next_refresh_at::timestamptz, '-infinity'::timestamptz)",
		"ORDER BY due_class, due_at NULLS FIRST, id",
	} {
		require.Contains(t, normalized, clause)
	}
	require.NotContains(t, normalized, "~*")
	// 配了 Cookie 就刷新（2026-09-28 P5）：不再按渠道级「自动刷新」开关过滤
	require.NotContains(t, normalized, "ollama_cloud_usage_auto_refresh")
	require.NoError(t, mock.ExpectationsWereMet())
}

// 批量更新不改 protocol_endpoints，Ollama 账号的上游地址不会因此变化：
// 只改 credentials.base_url 不得触发 Ollama 托管字段清理，只有 api_key 变化才算组身份变化。
func TestBulkUpdateOllamaIdentityCleanupFollowsAPIKeyOnly(t *testing.T) {
	baseURLOnly := &recordingSQLExecutor{result: rowsAffectedResult(1)}
	_, err := newAccountRepositoryWithSQL(nil, baseURLOnly, nil).BulkUpdate(context.Background(), []int64{17}, service.AccountBulkUpdate{
		Credentials: map[string]any{"base_url": "https://www.ollama.com:443/v1"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, baseURLOnly.execQueries)
	require.NotContains(t, normalizeSQLWhitespace(baseURLOnly.execQueries[0]), "ollama_cloud_usage_session")

	exec := &recordingSQLExecutor{result: rowsAffectedResult(1)}
	repo := newAccountRepositoryWithSQL(nil, exec, nil)

	_, err = repo.BulkUpdate(context.Background(), []int64{17}, service.AccountBulkUpdate{
		Credentials: map[string]any{"api_key": "new-key"},
	})

	require.NoError(t, err)
	require.NotEmpty(t, exec.execQueries)
	query := normalizeSQLWhitespace(exec.execQueries[0])
	require.Contains(t, query, "credentials -> 'api_key' IS DISTINCT FROM $1::jsonb -> 'api_key'")
	require.NotContains(t, query, "->> 'base_url'")
	require.NotContains(t, query, "~*")
	require.NotContains(t, query, "platform IN", "identity cleanup keys off the account type, never the display label")
	require.Contains(t, query, "type = 'apikey' AND (credentials -> 'api_key' IS DISTINCT FROM $1::jsonb -> 'api_key')")
	require.Contains(t, query, "- 'ollama_cloud_usage_session' - 'ollama_cloud_usage_snapshot'")
	payload, ok := exec.execArgs[0][0].([]byte)
	require.True(t, ok)
	require.NotContains(t, string(payload), service.OllamaCloudUsageSnapshotExtraKey)
}

func TestUpdateCredentialsIdentityChangeClearsAllOllamaManagedExtra(t *testing.T) {
	client, mock := newOllamaCloudUsageRepositoryTestClient(t)
	mock.ExpectBegin()
	mock.ExpectExec(`(?s)UPDATE accounts.*credentials -> 'api_key' IS DISTINCT FROM.*ollama_cloud_usage_session.*ollama_cloud_usage_snapshot`).
		WithArgs(`{"api_key":"new-key","base_url":"https://ollama.com"}`, int64(17)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO scheduler_outbox")).
		WithArgs(service.SchedulerOutboxEventAccountChanged, int64(17), nil, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	repo := newAccountRepositoryWithSQL(client, nil, nil)

	err := repo.UpdateCredentials(context.Background(), 17, map[string]any{
		"api_key": "new-key", "base_url": "https://ollama.com",
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// Ollama 清理分支必须带顶层 credentials DISTINCT 守卫：没有它，非 Ollama 的
// openai/anthropic apikey 账号在凭证未变化的持久化上也会误清探测快照。
func TestUpdateCredentialsCleanupBranchRequiresChangedCredentials(t *testing.T) {
	client, mock := newOllamaCloudUsageRepositoryTestClient(t)
	mock.ExpectBegin()
	mock.ExpectExec(`(?s)UPDATE accounts.*CASE.*AND credentials IS DISTINCT FROM \$1::jsonb\s+AND \(\s+credentials -> 'api_key' IS DISTINCT FROM`).
		WithArgs(`{"api_key":"same-key","base_url":"https://relay.example.com/v1"}`, int64(17)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO scheduler_outbox")).
		WithArgs(service.SchedulerOutboxEventAccountChanged, int64(17), nil, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	repo := newAccountRepositoryWithSQL(client, nil, nil)

	err := repo.UpdateCredentials(context.Background(), 17, map[string]any{
		"api_key": "same-key", "base_url": "https://relay.example.com/v1",
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// Ollama Cloud 识别只看地址与类型，平台标签不参与：SQL 侧与 Go 侧对任意标签都同判。
func TestOllamaCloudUsageEligibilityIgnoresPlatformLabel(t *testing.T) {
	require.NotContains(t, ollamaCloudUsageEligibleSQL, "platform")
	for _, platform := range []string{
		service.PlatformOpenAI, service.PlatformAnthropic,
		service.PlatformKimi, service.PlatformZhipu, service.PlatformDeepseek, service.PlatformMiniMax,
		service.PlatformGemini, service.PlatformGrok, service.PlatformAntigravity,
		service.PlatformComposite, "kiro",
	} {
		account := ollamaCloudUsageRepositoryAccount()
		account.Platform = platform
		require.True(t, service.IsOllamaCloudUsageAccount(account), platform)
	}
	oauth := ollamaCloudUsageRepositoryAccount()
	oauth.Type = service.AccountTypeOAuth
	require.False(t, service.IsOllamaCloudUsageAccount(oauth))
}

// 普通（非 ollama）kimi apikey 账号改凭证会落到 Ollama 分支——只减两个它本来就
// 不存在的 ollama 键，不得减其它任何 extra 键。
func TestUpdateCredentialsPlainCNAPIKeyAccountCleanupStaysSemanticallyEquivalent(t *testing.T) {
	var capturedSQL string
	matcher := sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		if strings.Contains(actualSQL, "UPDATE accounts") {
			capturedSQL = actualSQL
		}
		return sqlmock.QueryMatcherRegexp.Match(expectedSQL, actualSQL)
	})
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	mock.ExpectBegin()
	mock.ExpectExec(`(?s)UPDATE accounts.*- 'ollama_cloud_usage_session'.*- 'ollama_cloud_usage_snapshot'`).
		WithArgs(`{"api_key":"rotated-key","base_url":"https://api.moonshot.cn/v1"}`, int64(17)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO scheduler_outbox")).
		WithArgs(service.SchedulerOutboxEventAccountChanged, int64(17), nil, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	repo := newAccountRepositoryWithSQL(client, db, nil)

	err = repo.UpdateCredentials(context.Background(), 17, map[string]any{
		"api_key": "rotated-key", "base_url": "https://api.moonshot.cn/v1",
	})

	require.NoError(t, err)
	query := normalizeSQLWhitespace(capturedSQL)
	require.Contains(t, query, "WHEN type = 'apikey' AND credentials IS DISTINCT FROM $1::jsonb")
	require.NotContains(t, query, "platform IN")
	require.Contains(t, query,
		"THEN COALESCE(extra, '{}'::jsonb) - 'ollama_cloud_usage_session' - 'ollama_cloud_usage_snapshot' ELSE extra END")
	require.NoError(t, mock.ExpectationsWereMet())
}
