package service

// userRateForEffective 测试里按「生效倍率」写场景：用户价 = 官方价 × SalePriceRatio × 用户倍率，
// 想要生效倍率 X 就把用户倍率设成 X / SalePriceRatio（全站售价系数的语义由 user_rate_multiplier_test 单独验证）。
func userRateForEffective(effective float64) float64 {
	return effective / SalePriceRatio
}
