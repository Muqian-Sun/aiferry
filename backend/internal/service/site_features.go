package service

// 由代码决定的功能开关（muqian 2026-09-25：功能的启用 / 隐藏改代码、不走后台设置）。
// 要改就改这里、重新发版。前端靠公开设置里由这些常量派生的字段显示或隐藏入口；
// 只管前端显示的开关在 frontend/src/utils/siteFeatures.ts。
const (
	// RegistrationOpen 开放注册（2026-09-26 定：默认开放）。
	RegistrationOpen = true
	// InvitationCodeRequired 注册要填邀请码（2026-09-26 定：先不要）。
	InvitationCodeRequired = false
	// SessionBindingEnabled 会话 IP/UA 绑定：登录后 IP 或浏览器标识一变就强制重新登录（方案定：关）。
	SessionBindingEnabled = false
)
