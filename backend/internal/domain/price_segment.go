package domain

// PriceSegment 是按 Token 分段价的一段：单次请求的输入 Token（输入 + 缓存写 + 缓存读）
// 落在 (MinTokens, MaxTokens] 时，整条请求按这一段的价计费；MaxTokens 为 nil 表示不封顶。
// 单价是 USD / token；为 nil 的项按服务层的分段规则回落（输入 / 输出沿用基础价，缓存价按
// 本段输入价与基础输入价的比例折算）。承接关系上的上游分段价存成这个形状的 JSONB 数组。
type PriceSegment struct {
	MinTokens         int      `json:"min_tokens"`
	MaxTokens         *int     `json:"max_tokens"`
	InputPrice        *float64 `json:"input_price"`
	OutputPrice       *float64 `json:"output_price"`
	CacheWritePrice   *float64 `json:"cache_write_price"`
	CacheWrite1hPrice *float64 `json:"cache_write_1h_price"`
	CacheReadPrice    *float64 `json:"cache_read_price"`
}
