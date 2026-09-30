package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestKeyPrefix(t *testing.T) {
	if got := keyPrefix("sk-3f2a9c7e", 8); got != "sk-3f2a9" {
		t.Errorf("keyPrefix=%q want %q", got, "sk-3f2a9")
	}
	if got := keyPrefix("abc", 8); got != "abc" {
		t.Errorf("short key should be returned as-is, got %q", got)
	}
}

// 没到上游的错误（余额不足、利润门全排除等调度前就被拒）不写上游端点；到过上游的照写（2026-09-29 E2E）。
func TestClearOpsUpstreamEndpointWithoutUpstream(t *testing.T) {
	accountID := int64(7)
	status := 502
	cases := []struct {
		name  string
		entry service.OpsInsertErrorLogInput
		keep  bool
	}{
		{name: "调度前被拒", entry: service.OpsInsertErrorLogInput{UpstreamEndpoint: "/v1/responses"}, keep: false},
		{name: "选到了渠道", entry: service.OpsInsertErrorLogInput{UpstreamEndpoint: "/v1/responses", AccountID: &accountID}, keep: true},
		{name: "有上游状态码", entry: service.OpsInsertErrorLogInput{UpstreamEndpoint: "/v1/responses", UpstreamStatusCode: &status}, keep: true},
		{name: "有上游尝试", entry: service.OpsInsertErrorLogInput{UpstreamEndpoint: "/v1/responses", UpstreamErrors: []*service.OpsUpstreamErrorEvent{{}}}, keep: true},
	}
	for _, tc := range cases {
		entry := tc.entry
		clearOpsUpstreamEndpointWithoutUpstream(&entry)
		if tc.keep {
			require.Equal(t, "/v1/responses", entry.UpstreamEndpoint, tc.name)
		} else {
			require.Empty(t, entry.UpstreamEndpoint, tc.name)
		}
	}
}
