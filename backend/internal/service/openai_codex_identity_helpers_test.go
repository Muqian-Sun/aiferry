package service

// deriveStableUUIDv4 / extractClientSessionID 原在 openai_codex_fingerprint.go（Codex 指纹收敛
// 2026-09-28 P5 写死关后删了），两个辅助函数仍被账号身份隔离与 turn-state 溯源使用，测试随之迁到这里。

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeriveStableUUIDv4_Deterministic(t *testing.T) {
	a := deriveStableUUIDv4("test-seed-1")
	b := deriveStableUUIDv4("test-seed-1")
	assert.Equal(t, a, b, "同一种子应返回相同结果")
}

func TestDeriveStableUUIDv4_DifferentSeeds(t *testing.T) {
	a := deriveStableUUIDv4("seed-a")
	b := deriveStableUUIDv4("seed-b")
	assert.NotEqual(t, a, b, "不同种子应返回不同结果")
}

func TestDeriveStableUUIDv4_ValidFormat(t *testing.T) {
	result := deriveStableUUIDv4("test-seed")
	parsed, err := uuid.Parse(result)
	require.NoError(t, err, "应返回合法 UUID 格式")
	assert.Equal(t, uuid.Version(4), parsed.Version(), "应为 UUIDv4")
	assert.Equal(t, uuid.RFC4122, parsed.Variant(), "应为 RFC4122 变体")
}

func TestExtractClientSessionID(t *testing.T) {
	tests := []struct {
		name     string
		headers  http.Header
		expected string
	}{
		{"连字符形式优先", func() http.Header {
			h := http.Header{}
			h.Set("session-id", "hyphen-form")
			h.Set("session_id", "underscore-form")
			return h
		}(), "hyphen-form"},
		{"回退到下划线形式", func() http.Header {
			h := http.Header{}
			h.Set("session_id", "underscore-form")
			return h
		}(), "underscore-form"},
		{"都没有", http.Header{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, extractClientSessionID(tt.headers))
		})
	}
}
