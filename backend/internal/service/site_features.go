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

// 站点（muqian 2026-09-26：站点相关的在代码里配置，后台不再有「站点」设置）。
// 这些值经公开设置下发给两站前端，字段名不变。
const (
	// SiteName 站点名称：页面标题、邮件标题与落款都用它。
	SiteName = "AiFerry"
	// SiteLogo 站点 Logo 地址；留空用前端内置的 Logo。
	SiteLogo = ""
	// SiteContactInfo 客服联系方式，显示在用户站；空着不显示。
	// 2026-09-26 定「要有、先留占位」——上线前要填。
	SiteContactInfo = ""
	// SiteDocURL 文档链接；空着不显示。
	SiteDocURL = ""
	// ModelPlazaDescription 模型页顶部的价格说明（Markdown）；空着不显示。
	ModelPlazaDescription = ""
	// TableDefaultPageSize 列表默认每页条数。
	TableDefaultPageSize = 20
	// BackendModeEnabled「只留管理站」模式（关掉整个用户站）：两站已拆开，不再需要（方案定：删）。
	BackendModeEnabled = false
)

// TablePageSizeOptions 列表可选的每页条数。
func TablePageSizeOptions() []int { return []int{10, 20, 50, 100} }

// 以下站点功能前端不再展示（方案定：删），值固定为空：站点副标题、自定义首页内容、简洁首页、
// 隐藏 CCS 导入按钮、自定义菜单、自定义端点。功能代码保留，公开设置里对应字段恒为空值。
