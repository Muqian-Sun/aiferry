package service

import (
	"net/http"
	"strings"
	"unicode/utf8"
)

// maxUsageUpstreamRequestIDLen 与 usage_logs.upstream_request_id VARCHAR(128) 对齐。
const maxUsageUpstreamRequestIDLen = 128

// UpstreamRequestIDFromHeaders 从直接上游的响应头解析请求标识：按 upstreamRequestIDHeaders
// （channel_features.go）的顺序取第一个非空值，头名大小写不敏感；都没有时为空串。
func UpstreamRequestIDFromHeaders(h http.Header) string {
	if len(h) == 0 {
		return ""
	}
	for _, name := range upstreamRequestIDHeaders {
		if id := strings.TrimSpace(h.Get(name)); id != "" {
			return id
		}
	}
	return ""
}

// usageUpstreamRequestIDPtr 生成落库到 usage_logs.upstream_request_id 的值。
// WS 轮次没有 HTTP 响应头，保持 nil；超长时截断到列宽而不是让整条用量行失败。
func usageUpstreamRequestIDPtr(h http.Header, wsMode bool) *string {
	if wsMode {
		return nil
	}
	id := UpstreamRequestIDFromHeaders(h)
	if id == "" {
		return nil
	}
	if len(id) > maxUsageUpstreamRequestIDLen {
		id = id[:maxUsageUpstreamRequestIDLen]
		for len(id) > 0 && !utf8.ValidString(id) {
			id = id[:len(id)-1]
		}
	}
	if id == "" {
		return nil
	}
	return &id
}
