package service

import (
	"context"
)

// PanelRateLimitSettings 面板 API 限流配置。
// 认证后的面板接口按「用户 ID」维度限流（与客户端 IP 无关，反向代理/共享出口
// 不会被误伤）；无需认证的公开接口按安全客户端 IP 维度限流（内网/回环地址跳过）。
type PanelRateLimitSettings struct {
	// Enabled 总开关
	Enabled bool `json:"enabled"`
	// UserRPM 每用户每分钟请求数上限（认证面板接口全量计数；0 = 不限制）
	UserRPM int `json:"user_rpm"`
	// HeavyRPM 每用户每分钟重查询上限（usage/dashboard 等聚合统计接口；0 = 不限制）
	HeavyRPM int `json:"heavy_rpm"`
	// ExemptAdmin 管理员账号是否豁免按用户限流
	ExemptAdmin bool `json:"exempt_admin"`
	// PublicIPRPM 无需认证的公开接口每 IP 每分钟上限（0 = 不限制）
	PublicIPRPM int `json:"public_ip_rpm"`
}

// DefaultPanelRateLimitSettings 返回默认面板限流配置。
// 默认启用但阈值宽松：正常前端交互远达不到，仅拦截脚本高频刷接口打爆数据库的行为。
func DefaultPanelRateLimitSettings() *PanelRateLimitSettings {
	return &PanelRateLimitSettings{
		Enabled:     true,
		UserRPM:     240,
		HeavyRPM:    60,
		ExemptAdmin: true,
		PublicIPRPM: 300,
	}
}

// GetPanelRateLimitSettingsCached 面板限流配置：由代码决定（DefaultPanelRateLimitSettings），后台不再能改。
// 名字保留 Cached 是因为中间件热路径一直用它；现在就是常量，不读库。
func (s *SettingService) GetPanelRateLimitSettingsCached(ctx context.Context) PanelRateLimitSettings {
	return *DefaultPanelRateLimitSettings()
}
