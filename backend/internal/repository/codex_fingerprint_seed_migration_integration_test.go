//go:build integration

package repository

import (
	"context"
	"testing"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func requireCanonicalUUIDString(t *testing.T, value string) {
	t.Helper()
	parsed, err := uuid.Parse(value)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, parsed)
	require.Equal(t, parsed.String(), value)
}

func TestMigration225BackfillsOnlyEnabledOpenAIOAuthMissingOrMalformedSeeds(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	migrationSQL, err := dbmigrations.FS.ReadFile("225_backfill_codex_fingerprint_seed.sql")
	require.NoError(t, err)

	var missingID, blankID, malformedID, validID, offID, apiKeyID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO accounts (name, platform, type, extra)
VALUES ('migration-225-missing', 'openai', 'oauth', '{"codex_fingerprint_mode":"session"}'::jsonb)
RETURNING id
`).Scan(&missingID))
	require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO accounts (name, platform, type, extra)
VALUES ('migration-225-blank', 'openai', 'oauth', '{"codex_fingerprint_mode":"device","codex_fingerprint_seed":""}'::jsonb)
RETURNING id
`).Scan(&blankID))
	require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO accounts (name, platform, type, extra)
VALUES ('migration-225-malformed', 'openai', 'oauth', '{"codex_fingerprint_mode":"full","codex_fingerprint_seed":"BAD"}'::jsonb)
RETURNING id
`).Scan(&malformedID))
	require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO accounts (name, platform, type, extra)
VALUES ('migration-225-valid', 'openai', 'oauth', '{"codex_fingerprint_mode":"session","codex_fingerprint_seed":"11111111-1111-4111-8111-111111111111"}'::jsonb)
RETURNING id
`).Scan(&validID))
	require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO accounts (name, platform, type, extra)
VALUES ('migration-225-off', 'openai', 'oauth', '{"codex_fingerprint_mode":"off"}'::jsonb)
RETURNING id
`).Scan(&offID))
	require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO accounts (name, platform, type, extra)
VALUES ('migration-225-apikey', 'openai', 'apikey', '{"codex_fingerprint_mode":"session"}'::jsonb)
RETURNING id
`).Scan(&apiKeyID))

	_, err = tx.ExecContext(ctx, string(migrationSQL))
	require.NoError(t, err)

	seedsAfterFirst := map[int64]string{}
	for _, id := range []int64{missingID, blankID, malformedID, validID} {
		var seed string
		require.NoError(t, tx.QueryRowContext(ctx, `SELECT extra->>'codex_fingerprint_seed' FROM accounts WHERE id = $1`, id).Scan(&seed))
		requireCanonicalUUIDString(t, seed)
		seedsAfterFirst[id] = seed
	}
	require.Equal(t, "11111111-1111-4111-8111-111111111111", seedsAfterFirst[validID])

	for _, id := range []int64{offID, apiKeyID} {
		var hasSeed bool
		require.NoError(t, tx.QueryRowContext(ctx, `SELECT extra ? 'codex_fingerprint_seed' FROM accounts WHERE id = $1`, id).Scan(&hasSeed))
		require.False(t, hasSeed)
	}

	_, err = tx.ExecContext(ctx, string(migrationSQL))
	require.NoError(t, err)

	for id, want := range seedsAfterFirst {
		var got string
		require.NoError(t, tx.QueryRowContext(ctx, `SELECT extra->>'codex_fingerprint_seed' FROM accounts WHERE id = $1`, id).Scan(&got))
		require.Equal(t, want, got)
	}
}
