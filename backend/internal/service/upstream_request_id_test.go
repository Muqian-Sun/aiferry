package service

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// 上游请求标识按固定头名表依次取第一个非空值（2026-09-28 P5 写死，渠道上不再配头名）。
func TestUpstreamRequestIDFromHeaders_PicksFirstNonEmptyInFixedOrder(t *testing.T) {
	h := http.Header{}
	h.Set("X-Client-Request-ID", "sub2api-client")
	h.Set("xai-request-id", "xai-1")
	h.Set("x-goog-request-id", "goog-1")
	h.Set("X-Oneapi-Request-Id", "oneapi-1")
	h.Set("X-Request-ID", "sub2api-local")
	h.Set("Request-Id", " req_official ")

	// 表的顺序（已定取值，不从生产变量里读）：request-id → x-request-id → x-oneapi-request-id →
	// x-goog-request-id → xai-request-id → x-client-request-id；每轮删掉胜出的头，下一个接上
	order := []struct{ header, want string }{
		{"request-id", "req_official"},
		{"x-request-id", "sub2api-local"},
		{"x-oneapi-request-id", "oneapi-1"},
		{"x-goog-request-id", "goog-1"},
		{"xai-request-id", "xai-1"},
		{"x-client-request-id", "sub2api-client"},
	}
	for i, step := range order {
		require.Equal(t, step.want, UpstreamRequestIDFromHeaders(h), "第 %d 个头 %s 应当胜出", i, step.header)
		h.Del(step.header)
	}
	require.Equal(t, "", UpstreamRequestIDFromHeaders(h))
	require.Equal(t, "", UpstreamRequestIDFromHeaders(nil))

	// 空白值跳过，继续往后找；头名大小写不影响（响应头在 net/http 里是规范化的键）
	blank := http.Header{}
	blank.Set("request-id", "   ")
	blank["X-Request-Id"] = []string{"from-canonical-key"}
	require.Equal(t, "from-canonical-key", UpstreamRequestIDFromHeaders(blank))
}

func TestUsageUpstreamRequestIDPtr(t *testing.T) {
	h := http.Header{}
	h.Set("X-Request-ID", strings.Repeat("a", 200))
	require.Nil(t, usageUpstreamRequestIDPtr(h, true), "WS 轮次没有 HTTP 响应头")
	require.Nil(t, usageUpstreamRequestIDPtr(http.Header{}, false))
	require.Nil(t, usageUpstreamRequestIDPtr(nil, false))

	got := usageUpstreamRequestIDPtr(h, false)
	require.NotNil(t, got)
	require.Len(t, *got, maxUsageUpstreamRequestIDLen)
}
