package service

import "github.com/Wei-Shaw/sub2api/internal/pkg/timezone"

// 整个 service 测试二进制固定全局时区为 UTC。
//
// 这条以前藏在 group_peak_rate_test.go 的 init() 里；删掉那个文件后暴露出
// payment_fulfillment_test.go 等 sqlite 内存库用例对它的隐式依赖：sqlite 把 time.Time 存成
// 带时区后缀的文本按字典序比较，夹具用 time.Now()（本地时区）、生产代码用
// time.Now().UTC()，时区不一致时 updated_at 的"陈旧 / 新鲜"判定会翻转。
func init() {
	_ = timezone.Init("UTC")
}
