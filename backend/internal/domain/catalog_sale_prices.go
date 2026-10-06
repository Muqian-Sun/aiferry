package domain

// CatalogSalePrices 是目录条目上我们自己定的售价（USD / token，muqian 2026-10-06：每项单独填售价）：
// 基础价五项，加上按 Token 分段的各段售价。为 nil 的项没单独定，按「官方价 × 默认售价比例」收。
// 分段按 MinTokens 对上官方价的分段（官方价分段的下界）。整份存成 model_catalog_entries.sale_prices 的 JSONB，
// 只有价格页保存售价时写，播种刷新官方价、编辑模型都不碰它。
type CatalogSalePrices struct {
	InputPrice        *float64             `json:"input_price"`
	OutputPrice       *float64             `json:"output_price"`
	CacheWritePrice   *float64             `json:"cache_write_price"`
	CacheWrite1hPrice *float64             `json:"cache_write_1h_price"`
	CacheReadPrice    *float64             `json:"cache_read_price"`
	Segments          []CatalogSaleSegment `json:"segments"`
}

// CatalogSaleSegment 是一段的售价：输入侧 Token 落在官方价那一段（下界 MinTokens）时用。
type CatalogSaleSegment struct {
	MinTokens         int      `json:"min_tokens"`
	InputPrice        *float64 `json:"input_price"`
	OutputPrice       *float64 `json:"output_price"`
	CacheWritePrice   *float64 `json:"cache_write_price"`
	CacheWrite1hPrice *float64 `json:"cache_write_1h_price"`
	CacheReadPrice    *float64 `json:"cache_read_price"`
}

// IsZero 报告一项售价都没填。
func (p CatalogSalePrices) IsZero() bool {
	if p.InputPrice != nil || p.OutputPrice != nil || p.CacheWritePrice != nil || p.CacheWrite1hPrice != nil || p.CacheReadPrice != nil {
		return false
	}
	for _, s := range p.Segments {
		if s.InputPrice != nil || s.OutputPrice != nil || s.CacheWritePrice != nil || s.CacheWrite1hPrice != nil || s.CacheReadPrice != nil {
			return false
		}
	}
	return true
}
