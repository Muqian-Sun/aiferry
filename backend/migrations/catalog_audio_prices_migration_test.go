package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// 254 给目录条目加两列可空的音频 token 价，精度与其它价格列（243）一致，可重复执行。
func TestCatalogAudioPricesMigration(t *testing.T) {
	content, err := FS.ReadFile("254_catalog_audio_prices.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ALTER TABLE model_catalog_entries")
	for _, column := range []string{"audio_input_price", "audio_output_price"} {
		require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS "+column+" NUMERIC(20,12)")
	}
	require.NotContains(t, sql, "NOT NULL", "unset audio price must stay NULL so billing falls back to text price")

	entries, err := FS.ReadDir(".")
	require.NoError(t, err)
	seen := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "254_") {
			seen++
		}
	}
	require.Equal(t, 1, seen, "migration number 254 must be unique")
}
