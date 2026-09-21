//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

const modelCatalogMigration = "243_model_catalog.sql"

// applyModelCatalogMigration 在事务里重放 243，用于验证迁移可重复执行。
func applyModelCatalogMigration(ctx context.Context, t *testing.T, tx *sql.Tx) {
	t.Helper()

	migrationSQL, err := dbmigrations.FS.ReadFile(modelCatalogMigration)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migrationSQL))
	require.NoError(t, err)
}

func insertCatalogEntry(ctx context.Context, t *testing.T, tx *sql.Tx, modelID string) int64 {
	t.Helper()

	var id int64
	require.NoError(t, tx.QueryRowContext(ctx,
		"INSERT INTO model_catalog_entries (model_id) VALUES ($1) RETURNING id", modelID).Scan(&id))
	return id
}

// TestMigration243CreatesCatalogTablesAndIsRepeatable 覆盖「迁移可执行 + 可重复执行」。
// 243 已由 TestMain 的 ApplyMigrations 跑过一次，这里再跑一次必须不报错，
// 也不能把已有数据抹掉。
func TestMigration243CreatesCatalogTablesAndIsRepeatable(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	entryID := insertCatalogEntry(ctx, t, tx, "migration-243-repeat")

	applyModelCatalogMigration(ctx, t, tx)

	var surviving int64
	require.NoError(t, tx.QueryRowContext(ctx,
		"SELECT id FROM model_catalog_entries WHERE id = $1", entryID).Scan(&surviving))
	require.Equal(t, entryID, surviving)

	for _, table := range []string{
		"model_catalog_entries",
		"model_catalog_aliases",
		"model_catalog_price_intervals",
		"model_catalog_time_pricing",
	} {
		var exists bool
		require.NoError(t, tx.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1 FROM information_schema.tables
  WHERE table_schema = 'public' AND table_name = $1
)`, table).Scan(&exists))
		require.Truef(t, exists, "table %s must exist after migration", table)
	}
}

// TestMigration243ModelIDIsUniqueCaseInsensitively 钉住「模型标识大小写不敏感唯一」：
// 计费查表前一律 lower()，两条只差大小写的记录会让命中哪条取决于行序。
func TestMigration243ModelIDIsUniqueCaseInsensitively(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	insertCatalogEntry(ctx, t, tx, "Migration-243-Case")

	_, err := tx.ExecContext(ctx,
		"INSERT INTO model_catalog_entries (model_id) VALUES ($1)", "migration-243-case")
	require.Error(t, err, "lower(model_id) must be unique")
}

// TestMigration243AliasIsUniqueCaseInsensitively 钉住「同一个别名不能指向两个模型」。
func TestMigration243AliasIsUniqueCaseInsensitively(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	first := insertCatalogEntry(ctx, t, tx, "migration-243-alias-a")
	second := insertCatalogEntry(ctx, t, tx, "migration-243-alias-b")

	_, err := tx.ExecContext(ctx,
		"INSERT INTO model_catalog_aliases (alias, entry_id) VALUES ($1, $2)", "Migration-243-Alias", first)
	require.NoError(t, err)

	_, err = tx.ExecContext(ctx,
		"INSERT INTO model_catalog_aliases (alias, entry_id) VALUES ($1, $2)", "migration-243-alias", second)
	require.Error(t, err, "lower(alias) must be unique across entries")
}

// TestMigration243RejectsInvalidEnums 钉住三个 CHECK 约束。
func TestMigration243RejectsInvalidEnums(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name   string
		column string
		value  string
	}{
		{name: "billing_mode", column: "billing_mode", value: "per_minute"},
		{name: "status", column: "status", value: "draft"},
		{name: "managed_by", column: "managed_by", value: "importer"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx := testTx(t)
			_, err := tx.ExecContext(ctx,
				"INSERT INTO model_catalog_entries (model_id, "+tc.column+") VALUES ($1, $2)",
				"migration-243-enum-"+tc.name, tc.value)
			require.Errorf(t, err, "%s must reject %q", tc.column, tc.value)
		})
	}
}

// TestMigration243CascadesChildRowsOnEntryDelete 钉住三张子表的 ON DELETE CASCADE：
// 删条目必须把别名、分档、分时一起带走，否则孤儿行会让下次播种撞唯一约束。
func TestMigration243CascadesChildRowsOnEntryDelete(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	entryID := insertCatalogEntry(ctx, t, tx, "migration-243-cascade")

	_, err := tx.ExecContext(ctx,
		"INSERT INTO model_catalog_aliases (alias, entry_id) VALUES ($1, $2)", "migration-243-cascade-alias", entryID)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx,
		"INSERT INTO model_catalog_price_intervals (entry_id, min_tokens, max_tokens) VALUES ($1, 0, 100)", entryID)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx,
		"INSERT INTO model_catalog_time_pricing (entry_id, timezone) VALUES ($1, 'UTC')", entryID)
	require.NoError(t, err)

	_, err = tx.ExecContext(ctx, "DELETE FROM model_catalog_entries WHERE id = $1", entryID)
	require.NoError(t, err)

	for _, table := range []string{
		"model_catalog_aliases",
		"model_catalog_price_intervals",
		"model_catalog_time_pricing",
	} {
		var remaining int
		require.NoError(t, tx.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM "+table+" WHERE entry_id = $1", entryID).Scan(&remaining))
		require.Zerof(t, remaining, "%s rows must cascade with the entry", table)
	}
}

// TestMigration243RejectsInvertedInterval 钉住 max_tokens > min_tokens 的 CHECK：
// 倒挂的区间永远匹配不到，会静默退回基准价。
func TestMigration243RejectsInvertedInterval(t *testing.T) {
	ctx := context.Background()

	// 约束违例会让整个事务进入 aborted 状态，所以正反两例各用一个事务。
	t.Run("inverted is rejected", func(t *testing.T) {
		tx := testTx(t)
		entryID := insertCatalogEntry(ctx, t, tx, "migration-243-interval-bad")
		_, err := tx.ExecContext(ctx,
			"INSERT INTO model_catalog_price_intervals (entry_id, min_tokens, max_tokens) VALUES ($1, 200, 100)", entryID)
		require.Error(t, err, "max_tokens must be greater than min_tokens")
	})

	t.Run("unbounded is allowed", func(t *testing.T) {
		tx := testTx(t)
		entryID := insertCatalogEntry(ctx, t, tx, "migration-243-interval-ok")
		_, err := tx.ExecContext(ctx,
			"INSERT INTO model_catalog_price_intervals (entry_id, min_tokens, max_tokens) VALUES ($1, 0, NULL)", entryID)
		require.NoError(t, err, "unbounded interval must be allowed")
	})
}

// TestMigration243TimePricingIsOnePerEntry 钉住「一个条目至多一份分时配置」。
func TestMigration243TimePricingIsOnePerEntry(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	entryID := insertCatalogEntry(ctx, t, tx, "migration-243-time-pricing")

	_, err := tx.ExecContext(ctx,
		"INSERT INTO model_catalog_time_pricing (entry_id, timezone) VALUES ($1, 'UTC')", entryID)
	require.NoError(t, err)

	_, err = tx.ExecContext(ctx,
		"INSERT INTO model_catalog_time_pricing (entry_id, timezone) VALUES ($1, 'Asia/Shanghai')", entryID)
	require.Error(t, err, "entry_id must be unique in model_catalog_time_pricing")
}
