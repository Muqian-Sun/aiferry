package repository

import (
	"context"
	"database/sql/driver"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// recordingValueConverter 记录查询参数，校验 SQL 口径里的常量值。
type recordingValueConverter struct {
	values *[]driver.Value
}

func (c recordingValueConverter) ConvertValue(v any) (driver.Value, error) {
	converted, err := driver.DefaultParameterConverter.ConvertValue(v)
	if err == nil {
		*c.values = append(*c.values, converted)
	}
	return converted, err
}

// 调度候选查询的平台过滤只约束成品号：WHERE 里平台条件必须与「第三方 key」条件
// （source_kind = api_key，或 source_kind 为 NULL 时类型为 apikey）取 OR。
// 真实数据库行为见 account_repo_scheduling_candidates_integration_test.go。
func TestListSchedulingCandidates_PlatformFilterOnlyConstrainsSubscriptions(t *testing.T) {
	queries := map[string]func(*accountRepository) error{
		"all": func(repo *accountRepository) error {
			_, err := repo.ListSchedulingCandidates(context.Background(), []string{service.PlatformAnthropic})
			return err
		},
		"by group": func(repo *accountRepository) error {
			_, err := repo.ListSchedulingCandidatesByGroupID(context.Background(), 42, []string{service.PlatformAnthropic})
			return err
		},
		"ungrouped": func(repo *accountRepository) error {
			_, err := repo.ListSchedulingCandidatesUngrouped(context.Background(), []string{service.PlatformAnthropic})
			return err
		},
	}
	for name, query := range queries {
		t.Run(name, func(t *testing.T) {
			var capturedSQL string
			var args []driver.Value
			db, mock, err := sqlmock.New(
				sqlmock.QueryMatcherOption(captureEntQueryMatcher{actual: &capturedSQL}),
				sqlmock.ValueConverterOption(recordingValueConverter{values: &args}),
			)
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			t.Cleanup(func() { _ = client.Close() })
			repo := newAccountRepositoryWithSQL(client, db, nil)

			mock.ExpectQuery("scheduling candidates").WillReturnRows(sqlmock.NewRows([]string{"id"}))
			require.NoError(t, query(repo))
			require.NoError(t, mock.ExpectationsWereMet())

			normalized := normalizeSQLWhitespace(capturedSQL)
			require.Contains(t, normalized, `"platform" IN (`)
			platformAt := strings.Index(normalized, `"platform" IN (`)
			keyClause := normalized[platformAt:]
			require.Regexp(t, `^"platform" IN \(\$\d+\) OR \("accounts"\."source_kind" = \$\d+ OR \("accounts"\."source_kind" IS NULL AND "accounts"\."type" = \$\d+\)\)`, keyClause)
			require.Contains(t, args, driver.Value(service.PlatformAnthropic))
			require.Contains(t, args, driver.Value(service.AccountSourceAPIKey))
			require.Contains(t, args, driver.Value(service.AccountTypeAPIKey))
		})
	}
}
