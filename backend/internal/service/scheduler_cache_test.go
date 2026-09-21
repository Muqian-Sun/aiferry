//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 桶键 "%d:%s:%s" 只有目录桶的 Platform 允许为空；注册表 ListBuckets 解析失败会静默跳过，
// 目录桶解析不回来就不再被重建。
func TestParseSchedulerBucket(t *testing.T) {
	cases := []struct {
		raw  string
		want SchedulerBucket
		ok   bool
	}{
		{"27::catalog", SchedulerBucket{GroupID: 27, Platform: "", Mode: SchedulerModeCatalog}, true},
		{"3:anthropic:single", SchedulerBucket{GroupID: 3, Platform: PlatformAnthropic, Mode: SchedulerModeSingle}, true},
		{"3:anthropic:mixed", SchedulerBucket{GroupID: 3, Platform: PlatformAnthropic, Mode: SchedulerModeMixed}, true},
		{"27::single", SchedulerBucket{}, false},
		{"27::mixed", SchedulerBucket{}, false},
		{"27:anthropic:", SchedulerBucket{}, false},
		{"x::catalog", SchedulerBucket{}, false},
		{"27:catalog", SchedulerBucket{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			got, ok := ParseSchedulerBucket(tc.raw)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.want, got)
			if ok {
				require.Equal(t, tc.raw, got.String(), "round-trips through String()")
			}
		})
	}
}
