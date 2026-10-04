//go:build unit

package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 用户看到的请求 ID 去掉计费去重键的内部前缀：local:<id> 就是响应头 X-Request-ID，client:<id> 是用户自己带的；
// 管理站看库里原值（按请求 ID 搜要精确匹配）（2026-10-04 D5 / U15）
func TestUsageLogRequestIDPrefixHiddenFromUsers(t *testing.T) {
	for raw, shown := range map[string]string{
		"local:3f2a-uuid": "3f2a-uuid",
		"client:my-id":    "my-id",
		"resp_upstream":   "resp_upstream",
	} {
		l := &service.UsageLog{ID: 1, RequestID: raw}
		require.Equal(t, shown, UsageLogFromService(l).RequestID, raw)
		require.Equal(t, raw, UsageLogFromServiceAdmin(l).RequestID, raw)
	}
}
