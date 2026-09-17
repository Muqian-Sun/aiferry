//go:build integration

package repository

import (
	"context"
	"testing"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

// TestMigration242MergesLegacyUpstreamAccountsIntoAPIKey 覆盖 052 漏掉的几类 upstream 行
// （非 antigravity 平台、已软删除、source_kind 为空），并确认其它类型的行不被改动、
// 迁移可重复执行。
func TestMigration242MergesLegacyUpstreamAccountsIntoAPIKey(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	migrationSQL, err := dbmigrations.FS.ReadFile("242_merge_legacy_upstream_accounts_into_apikey.sql")
	require.NoError(t, err)

	insert := func(name, platform, accountType string, sourceKind any, deleted bool) int64 {
		t.Helper()
		var id int64
		require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO accounts (name, platform, type, source_kind, deleted_at)
VALUES ($1, $2, $3, $4, CASE WHEN $5 THEN NOW() ELSE NULL END)
RETURNING id
`, name, platform, accountType, sourceKind, deleted).Scan(&id))
		return id
	}

	antigravityUpstream := insert("m242-antigravity-upstream", "antigravity", "upstream", "api_key", false)
	anthropicUpstream := insert("m242-anthropic-upstream", "anthropic", "upstream", "api_key", false)
	deletedUpstream := insert("m242-deleted-upstream", "antigravity", "upstream", "api_key", true)
	unclassifiedUpstream := insert("m242-unclassified-upstream", "openai", "upstream", nil, false)
	oauthAccount := insert("m242-oauth", "anthropic", "oauth", "subscription", false)
	apiKeyAccount := insert("m242-apikey", "openai", "apikey", "api_key", false)

	for i := 0; i < 2; i++ {
		_, err = tx.ExecContext(ctx, string(migrationSQL))
		require.NoError(t, err)
	}

	type row struct {
		accountType string
		sourceKind  string
	}
	read := func(id int64) row {
		t.Helper()
		var r row
		require.NoError(t, tx.QueryRowContext(ctx,
			`SELECT type, COALESCE(source_kind, '') FROM accounts WHERE id = $1`, id,
		).Scan(&r.accountType, &r.sourceKind))
		return r
	}

	for _, id := range []int64{antigravityUpstream, anthropicUpstream, deletedUpstream, unclassifiedUpstream} {
		require.Equal(t, row{accountType: "apikey", sourceKind: "api_key"}, read(id))
	}
	require.Equal(t, row{accountType: "oauth", sourceKind: "subscription"}, read(oauthAccount))
	require.Equal(t, row{accountType: "apikey", sourceKind: "api_key"}, read(apiKeyAccount))

	var remaining int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts WHERE type = 'upstream'`).Scan(&remaining))
	require.Zero(t, remaining)
}
