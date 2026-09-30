package usagestats

// AccountStats 账号使用统计
//
// cost: 渠道成本（使用 account_cost：用量 × 承接关系上的上游价）
// standard_cost: 标价（使用 total_cost，目录官方价，不含倍率）
// user_cost: 用户/API Key 口径费用（使用 actual_cost：标价 × 用户倍率）
type AccountStats struct {
	Requests     int64   `json:"requests"`
	Tokens       int64   `json:"tokens"`
	Cost         float64 `json:"cost"`
	StandardCost float64 `json:"standard_cost"`
	UserCost     float64 `json:"user_cost"`
}
