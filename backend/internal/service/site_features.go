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

// 登录条款（2026-09-26 定：条款在代码里配置）。正文是 legal/usage-policy.md、legal/privacy.md。
const (
	// LoginAgreementEnabled 登录 / 注册前要确认条款。
	LoginAgreementEnabled = true
	// LoginAgreementMode 条款的展示形式：modal 弹窗 / checkbox 复选框。
	LoginAgreementMode = "modal"
	// LoginAgreementUpdatedAt 条款更新日期，显示给用户。改了 legal/*.md 的正文要同时改它；
	// 用户要不要重新确认看的是修订号（日期 + 正文哈希），忘改日期也会要求重新确认。
	LoginAgreementUpdatedAt = "2026-09-23"
)

// 通知（2026-09-26 定：通知在代码里写死，跟着 SMTP 走）。
// 配了 SMTP（SMTP_HOST + SMTP_FROM）就开：邮箱验证与忘记密码、余额不足提醒（账号邮箱 + 用户另加的邮箱）、
// 渠道额度提醒（发给管理员账号邮箱）、订阅到期提醒；没配就都不发，用户站也不出现提醒设置。
const (
	// BalanceLowNotifyThreshold 余额低于它时提醒（美元，拍的）；用户可在个人设置里改自己的阈值或关掉。
	BalanceLowNotifyThreshold = 1.0
	// AllowUserViewErrorRequests 用户能不能在用量页看到自己的错误请求（dev 现值：关）。
	AllowUserViewErrorRequests = false
)

// rechargePagePath 用户站充值页，余额提醒邮件里的「立即充值」指向它（前面拼用户站地址）。
const rechargePagePath = "/billing/recharge"

// TablePageSizeOptions 列表可选的每页条数。
func TablePageSizeOptions() []int { return []int{10, 20, 50, 100} }

// 以下站点功能前端不再展示（方案定：删），值固定为空：站点副标题、自定义首页内容、简洁首页、
// 隐藏 CCS 导入按钮、自定义菜单、自定义端点。功能代码保留，公开设置里对应字段恒为空值。
