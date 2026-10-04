//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 首字延迟 P99 告警线默认 20 秒，与服务状态的「异常」线一致（muqian 2026-10-04）；库里没存时按它标色。
func TestOpsMetricThresholds_DefaultTTFTIsTwentySeconds(t *testing.T) {
	got, err := (&OpsService{}).GetMetricThresholds(context.Background())
	require.NoError(t, err)
	require.NotNil(t, got.TTFTp99MsMax)
	require.Equal(t, 20000.0, *got.TTFTp99MsMax)
}
