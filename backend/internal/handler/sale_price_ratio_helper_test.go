package handler

import "github.com/Wei-Shaw/sub2api/internal/service"

// userRateForEffective 测试里按「生效倍率」写场景：想要生效倍率 X 就把用户倍率设成 X / 全站售价系数。
func userRateForEffective(effective float64) float64 {
	return effective / service.SalePriceRatio
}
