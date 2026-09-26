package service

import "testing"

// setGatewayPolicyForTest 在本测试里临时替换 gateway_features.go 里的一个策略变量，测试结束复原。
// 这些变量运行时只读；只能在没调 t.Parallel 的测试里替换，否则并行的测试会读到别人换进去的值。
func setGatewayPolicyForTest[T any](t *testing.T, p *T, v T) {
	t.Helper()
	old := *p
	*p = v
	t.Cleanup(func() { *p = old })
}
