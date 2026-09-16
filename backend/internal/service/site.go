package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// Site 标识请求所在的站点。用户站与管理站是两个独立监听器，路由集合互不相交。
type Site string

const (
	SiteUser  Site = "user"
	SiteAdmin Site = "admin"
)

// WithSite 把站点写入 context。只应由各监听器最外层的中间件调用。
func WithSite(ctx context.Context, site Site) context.Context {
	return context.WithValue(ctx, ctxkey.Site, site)
}

// SiteFromContext 读取请求所在站点；未设置时返回空串。
func SiteFromContext(ctx context.Context) Site {
	if ctx == nil {
		return ""
	}
	site, _ := ctx.Value(ctxkey.Site).(Site)
	return site
}

// CheckSiteRole 校验账号角色能否在当前站点登录或访问。
//
// 管理员账号只能用管理站，非管理员只能用用户站。两处都要拦：签发 token 时拦住
// 登录，认证中间件拦住拿着另一站点签发的 token 过来的请求——token 本身不绑定站点。
//
// 未设置站点（单元测试直接构造的上下文、安装向导）不做限制；生产监听器必须设置，
// 由路由装配测试保证。
func CheckSiteRole(ctx context.Context, role string) error {
	switch SiteFromContext(ctx) {
	case SiteUser:
		if role == RoleAdmin {
			return infraerrors.Forbidden("SITE_ROLE_FORBIDDEN", "administrator accounts must sign in to the admin console")
		}
	case SiteAdmin:
		if role != RoleAdmin {
			return infraerrors.Forbidden("SITE_ROLE_FORBIDDEN", "only administrators can sign in to the admin console")
		}
	}
	return nil
}
