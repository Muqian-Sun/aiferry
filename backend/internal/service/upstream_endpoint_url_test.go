package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJoinUpstreamEndpointURLPreservesURLComponents(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		endpoint string
		want     string
	}{
		{name: "root", base: "https://upstream.example", endpoint: "/v1/models", want: "https://upstream.example/v1/models"},
		{name: "v1", base: "https://upstream.example/v1", endpoint: "/v1/responses", want: "https://upstream.example/v1/responses"},
		{name: "prefix", base: "https://upstream.example/openai", endpoint: "/v1/chat/completions", want: "https://upstream.example/openai/v1/chat/completions"},
		{name: "version", base: "https://upstream.example/openai/v2", endpoint: "/v1/embeddings", want: "https://upstream.example/openai/v2/embeddings"},
		{name: "query", base: "https://upstream.example/v1?redirect=/", endpoint: "/v1/sub2api/billing", want: "https://upstream.example/v1/sub2api/billing?redirect=/"},
		{name: "fragment is removed", base: "https://upstream.example/v1#stale", endpoint: "/v1/alpha/search", want: "https://upstream.example/v1/alpha/search"},
		{name: "ipv6", base: "http://[2001:db8::1]:8080/v1?tenant=a#stale", endpoint: "/v1/responses/input_tokens", want: "http://[2001:db8::1]:8080/v1/responses/input_tokens?tenant=a"},
		{name: "already complete", base: "https://upstream.example/v1/images/generations?tenant=a", endpoint: "/v1/images/generations", want: "https://upstream.example/v1/images/generations?tenant=a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, joinUpstreamEndpointURL(tt.base, tt.endpoint))
		})
	}
}

// TestJoinAnthropicBetaEndpointURL 固定第三方中转 base_url 的拼接口径。
// 关键用例是「base_url 末尾带 /v1」：改用统一拼接前会拼成 /v1/v1/messages，
// 只表现为上游 404，界面上看不出是配置填错。
func TestJoinAnthropicBetaEndpointURL(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		endpoint string
		want     string
	}{
		{
			name:     "官方域名",
			base:     "https://api.anthropic.com",
			endpoint: "/v1/messages",
			want:     "https://api.anthropic.com/v1/messages?beta=true",
		},
		{
			name:     "base 末尾已带 v1",
			base:     "https://relay.example.com/v1",
			endpoint: "/v1/messages",
			want:     "https://relay.example.com/v1/messages?beta=true",
		},
		{
			name:     "base 末尾带 v1 和斜杠",
			base:     "https://relay.example.com/v1/",
			endpoint: "/v1/messages",
			want:     "https://relay.example.com/v1/messages?beta=true",
		},
		{
			name:     "base 带非版本号路径前缀",
			base:     "https://relay.example.com/claude",
			endpoint: "/v1/messages",
			want:     "https://relay.example.com/claude/v1/messages?beta=true",
		},
		{
			name:     "base 自带查询串不会拼出两个问号",
			base:     "https://relay.example.com?token=abc",
			endpoint: "/v1/messages",
			want:     "https://relay.example.com/v1/messages?beta=true&token=abc",
		},
		{
			name:     "count_tokens 端点同样处理 v1 后缀",
			base:     "https://relay.example.com/v1",
			endpoint: "/v1/messages/count_tokens",
			want:     "https://relay.example.com/v1/messages/count_tokens?beta=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, joinAnthropicBetaEndpointURL(tt.base, tt.endpoint))
		})
	}
}
