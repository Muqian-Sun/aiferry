package routes

import "github.com/Wei-Shaw/sub2api/internal/service"

// customRate 测试里给用户单独设的售价折扣（User.RateMultiplier 可空：nil = 不打折）。
func customRate(v float64) *float64 { return &v }

// officialRate 让用户的计费倍率（官方价口径）等于 v 时要设的售价折扣：users.rate_multiplier 存的是「在售价上再打折」
// （muqian 2026-10-06），计费倍率 = 折扣 × 默认售价比例。计费类测试按官方口径写期望值，用它设用户。
func officialRate(v float64) *float64 { d := v / service.DefaultSalePriceRatio; return &d }
