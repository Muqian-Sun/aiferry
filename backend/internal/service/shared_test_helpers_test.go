package service

import (
	"github.com/gin-gonic/gin"
)

// 无 build tag：被 unit 与 integration 两种标签下都要编译的测试文件引用。
// 放在带 //go:build unit 的文件里会让 -tags integration 编译不过（无标签的测试文件在
// 两种标签下都编译，却找不到只在 unit 下存在的符号）。

// withVendorRoute 给请求挂一条目录路由，条目厂商决定请求的厂商平台（xai → grok）。
func withVendorRoute(c *gin.Context, vendor string) {
	c.Request = c.Request.WithContext(WithCatalogRoute(c.Request.Context(), CatalogRoute{
		EntryID: 1, CanonicalModel: "m", RequestedModel: "m", Entry: &ModelCatalogEntry{ID: 1, ModelID: "m", Vendor: vendor},
	}))
}
