//go:build unit

package server

import (
	"reflect"
	"regexp"
	"sort"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// allocNilPointers 把结构体里为 nil 的指针字段递归分配为零值，只为能完成路由注册。
func allocNilPointers(v reflect.Value) {
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Kind() == reflect.Ptr && f.IsNil() && f.CanSet() && f.Type().Elem().Kind() == reflect.Struct {
			f.Set(reflect.New(f.Type().Elem()))
			if f.Type().Elem().PkgPath() == reflect.TypeOf(handler.Handlers{}).PkgPath() && f.Type().Elem().Name() == "AdminHandlers" {
				allocNilPointers(f.Elem())
			}
		}
	}
}

func siteRouteSet(t *testing.T, site service.Site) map[string]struct{} {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := &handler.Handlers{}
	allocNilPointers(reflect.ValueOf(h).Elem())
	next := func(c *gin.Context) { c.Next() }
	deps := routeDeps{
		handlers:        h,
		jwtAuth:         middleware2.JWTAuthMiddleware(next),
		optionalJWTAuth: middleware2.OptionalJWTAuthMiddleware(next),
		adminAuth:       middleware2.AdminAuthMiddleware(next),
		apiKeyAuth:      middleware2.APIKeyAuthMiddleware(next),
		auditLog:        middleware2.AuditLogMiddleware(next),
		stepUpAuth:      middleware2.StepUpAuthMiddleware(next),
		cfg:             &config.Config{},
	}
	engine := setupSiteRouter(gin.New(), site, deps, func() []string { return nil }, nil)
	out := map[string]struct{}{}
	for _, r := range engine.Routes() {
		out[r.Method+" "+r.Path] = struct{}{}
	}
	return out
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// TestAdminSiteServesOnlyAllowlistedRoutes 管理站路由白名单，fail-closed：
// 管理站上出现白名单之外的任何路由都算失败，新增路由必须有意识地放进来。
func TestAdminSiteServesOnlyAllowlistedRoutes(t *testing.T) {
	allowed := []*regexp.Regexp{
		regexp.MustCompile(`^[A-Z]+ /api/v1/admin(/|$)`),
		regexp.MustCompile(`^POST /api/v1/auth/(login|login/2fa|passkey/login/begin|passkey/login/finish|refresh|logout|revoke-all-sessions)$`),
		regexp.MustCompile(`^GET /api/v1/auth/me$`),
		regexp.MustCompile(`^GET /api/v1/settings/public$`),
		regexp.MustCompile(`^(GET /api/v1/user/profile|PUT /api/v1/user/password|PUT /api/v1/user)$`),
		regexp.MustCompile(`^[A-Z]+ /api/v1/user/(totp|passkeys)(/|$)`),
		regexp.MustCompile(`^GET /api/v1/pages(/|$)`),
		// 插件管理页以 iframe 加载插件界面，资源路径带一次性 token，由 RegisterAdminRoutes 注册。
		regexp.MustCompile(`^GET /api/v1/plugin-ui/:token/\*path$`),
		regexp.MustCompile(`^(GET /health|GET /setup/status|POST /api/event_logging/batch)$`),
	}
	routes := siteRouteSet(t, service.SiteAdmin)
	var unexpected []string
	for _, route := range sortedKeys(routes) {
		ok := false
		for _, re := range allowed {
			if re.MatchString(route) {
				ok = true
				break
			}
		}
		if !ok {
			unexpected = append(unexpected, route)
		}
	}
	require.Empty(t, unexpected, "管理站出现了白名单之外的路由")

	for _, must := range []string{
		"POST /api/v1/auth/login",
		"POST /api/v1/auth/login/2fa",
		"POST /api/v1/auth/refresh",
		"GET /api/v1/auth/me",
		"GET /api/v1/settings/public",
		"POST /api/v1/user/totp/step-up",
		"GET /api/v1/admin/accounts",
		"GET /api/v1/admin/payment/dashboard",
		"GET /api/v1/pages",
	} {
		require.Contains(t, routes, must)
	}
}

// TestUserSiteServesNoAdminRoutes 用户站不得注册任何管理接口。页面列表挂在 /api/v1/pages
// 而非 /admin 前缀下，按路径前缀判断会漏掉它，所以单独断言。
func TestUserSiteServesNoAdminRoutes(t *testing.T) {
	routes := siteRouteSet(t, service.SiteUser)
	for _, route := range sortedKeys(routes) {
		require.NotRegexp(t, `^[A-Z]+ /api/v1/admin(/|$)`, route)
	}
	require.NotContains(t, routes, "GET /api/v1/pages")

	for _, must := range []string{
		"POST /v1/messages",
		"POST /api/v1/auth/register",
		"POST /api/v1/auth/login",
		"GET /api/v1/keys",
		"POST /api/v1/payment/webhook/stripe",
		"GET /api/v1/user/totp/status",
		"GET /api/v1/pages/:slug",
	} {
		require.Contains(t, routes, must)
	}
}
