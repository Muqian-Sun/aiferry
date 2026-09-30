package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterServiceStatusRoutes 注册用户站「服务状态」页的两个读数：全站快照与逐模型矩阵。
//
// 与模型广场一样对未登录访客开放（muqian 2026-09-30），按客户端 IP 限流。不挂 JWT：
// handler 走非管理员分支，只回 dto.ServiceStatus* 白名单字段，与访客身份无关。
// 功能开关与模式守卫同登录态的渠道监控接口；BackendModeUserGuard 保证 backend 模式下不对外。
func RegisterServiceStatusRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	settingService *service.SettingService,
	panelRateLimiter *middleware.PanelRateLimiter,
) {
	status := v1.Group("/channel-monitor-v2")
	status.Use(panelRateLimiter.PublicIP())
	status.Use(middleware.BackendModeUserGuard(settingService))
	status.Use(channelMonitorModeV2Guard(settingService))
	{
		status.GET("/snapshot", h.ChannelMonitorV2.Snapshot)
		status.GET("/matrix", h.ChannelMonitorV2.Matrix)
	}
}
