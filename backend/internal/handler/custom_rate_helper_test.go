package handler

// customRate 测试里给用户单独设的计费倍率（User.RateMultiplier 可空：nil = 跟全站默认倍率）。
func customRate(v float64) *float64 { return &v }
