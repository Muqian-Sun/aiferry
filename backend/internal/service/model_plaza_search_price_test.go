//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 模型广场只列这个模型实际会收的搜索费：OpenAI 模型 = /alpha/search 每次价（条目配了用条目的，没配按内置 $0.01），
// grok 模型 = 搜索工具内置 $5 / 千次，其他厂商都不列（走不到这两个计费口子）。
func TestPlazaSearchPrices(t *testing.T) {
	configured := 0.02
	cases := []struct {
		name      string
		entry     ModelCatalogEntry
		web, tool *float64
	}{
		{name: "openai default", entry: ModelCatalogEntry{Vendor: "openai"}, web: testPtrFloat64(0.01)},
		{name: "openai configured", entry: ModelCatalogEntry{Vendor: "openai", SearchPricePerCall: &configured}, web: testPtrFloat64(0.02)},
		{name: "grok tool search", entry: ModelCatalogEntry{Vendor: "xai"}, tool: testPtrFloat64(0.005)},
		{name: "anthropic never charged", entry: ModelCatalogEntry{Vendor: "anthropic", SearchPricePerCall: &configured}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			web, tool := plazaSearchPrices(&tc.entry)
			if tc.web == nil {
				require.Nil(t, web)
			} else {
				require.NotNil(t, web)
				require.InDelta(t, *tc.web, *web, 1e-15)
			}
			if tc.tool == nil {
				require.Nil(t, tool)
			} else {
				require.NotNil(t, tool)
				require.InDelta(t, *tc.tool, *tool, 1e-15)
			}
		})
	}
}
