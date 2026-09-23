//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 桶键 "%d:%s:%s" 只认两种形态：目录桶（条目 ID + 空平台）与平台池（PoolID 0 + 平台）。
// 注册表 ListBuckets 解析失败会静默跳过：旧的分组 / mixed / forced 桶就此不再被重建。
func TestParseSchedulerBucket(t *testing.T) {
	cases := []struct {
		raw  string
		want SchedulerBucket
		ok   bool
	}{
		{"27::catalog", SchedulerBucket{PoolID: 27, Mode: SchedulerModeCatalog}, true},
		{"0:anthropic:single", SchedulerBucket{Platform: PlatformAnthropic, Mode: SchedulerModeSingle}, true},
		// 旧的分组桶 / mixed / forced 成员一律拒绝：注册表里的残留不再被重建
		{"3:anthropic:single", SchedulerBucket{}, false},
		{"0:anthropic:mixed", SchedulerBucket{}, false},
		{"0:anthropic:forced", SchedulerBucket{}, false},
		{"27::single", SchedulerBucket{}, false},
		{"0::catalog", SchedulerBucket{}, false},
		{"27:anthropic:catalog", SchedulerBucket{}, false},
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
