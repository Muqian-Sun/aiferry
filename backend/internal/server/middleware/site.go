package middleware

import (
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// SiteContext 把监听器所属站点写入请求 context，供认证中间件与 token 签发做站点角色校验。
func SiteContext(site service.Site) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(service.WithSite(c.Request.Context(), site))
		c.Next()
	}
}
