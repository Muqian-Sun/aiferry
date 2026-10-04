package timezone

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func mustQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	q, err := url.ParseQuery(raw)
	require.NoError(t, err)
	return q
}

// 「近 24 小时」给精确时刻：原样用作 [start, end)，不按自然日展开（2026-10-04 走查：按日期传查成了两个自然日）
func TestParseQueryRangePreciseTimes(t *testing.T) {
	start, end, err := ParseQueryRange(mustQuery(t, "start_time=2026-10-03T02:45:00.000Z&end_time=2026-10-04T02:45:00.000Z&timezone=Asia/Shanghai"))
	require.NoError(t, err)
	require.NotNil(t, start)
	require.NotNil(t, end)
	require.True(t, start.Equal(time.Date(2026, 10, 3, 2, 45, 0, 0, time.UTC)), "start=%s", start)
	require.True(t, end.Equal(time.Date(2026, 10, 4, 2, 45, 0, 0, time.UTC)), "end=%s", end)
}

// 按天：按 timezone 参数取当天零点，end_date 含当天（取次日零点）；起止同一天是合法的单日范围
func TestParseQueryRangeDatesInUserTimezone(t *testing.T) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)

	start, end, err := ParseQueryRange(mustQuery(t, "start_date=2026-10-04&end_date=2026-10-04&timezone=Asia/Shanghai"))
	require.NoError(t, err)
	require.True(t, start.Equal(time.Date(2026, 10, 4, 0, 0, 0, 0, shanghai)), "start=%s", start)
	require.True(t, end.Equal(time.Date(2026, 10, 5, 0, 0, 0, 0, shanghai)), "end=%s", end)
}

func TestParseQueryRangeAbsentBoundsAreNil(t *testing.T) {
	start, end, err := ParseQueryRange(mustQuery(t, "timezone=UTC"))
	require.NoError(t, err)
	require.Nil(t, start)
	require.Nil(t, end)
}

func TestParseQueryRangeRejectsInvalidRanges(t *testing.T) {
	cases := map[string]string{
		"起止日期颠倒":                 "start_date=2026-10-05&end_date=2026-10-03",
		"起止时刻颠倒":                 "start_time=2026-10-04T02:00:00Z&end_time=2026-10-03T02:00:00Z",
		"起止时刻相同（空区间）":            "start_time=2026-10-04T02:00:00Z&end_time=2026-10-04T02:00:00Z",
		"同一端两种写法":                "start_time=2026-10-03T02:00:00Z&start_date=2026-10-03",
		"start_time 不是 RFC3339":  "start_time=2026-10-03",
		"end_date 不是 YYYY-MM-DD": "end_date=2026/10/03",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			start, end, err := ParseQueryRange(mustQuery(t, raw))
			require.Error(t, err)
			require.Nil(t, start)
			require.Nil(t, end)
		})
	}
}
