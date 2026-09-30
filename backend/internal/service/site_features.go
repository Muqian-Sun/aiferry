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
	// LoginAgreementUpdatedAt 条款更新日期，显示在条款正文页。改了 legal/*.md 的正文要同时改它。
	// 登录 / 注册每次都要勾选同意（默认不勾、不记住），没有「同意过哪一版」的记录。
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

// 注册与安全（2026-09-26 定：注册在代码里配置）。
const (
	// RegistrationEmailDomainQuotaEnabled 白名单之外的域名按主域名限量注册（方案定：删，写死关）。
	RegistrationEmailDomainQuotaEnabled = false
	// StepUpEnabled 敏感操作（导出、备份、提升管理员等）要求二次验证（拍板第 9 条：用不到，写死关）。
	StepUpEnabled = false
	// ForceEmailOnThirdPartySignup 第三方注册强制补邮箱：写死关（Google / GitHub 本来就带邮箱，微信注册用合成邮箱）。
	ForceEmailOnThirdPartySignup = false
)

// ChannelMonitorEnabled 渠道监控（用户站「服务状态」）写死开（2026-09-26 定），后台不再有开关。
const ChannelMonitorEnabled = true

// PaymentEnabled 在线支付写死关（2026-09-26 定）：下单、支付页、后台支付设置都不出现，
// 支付代码与支付配置接口保留，开支付时改这里再把后台那一节接回来。
const PaymentEnabled = false

// 新用户默认值（2026-09-26 定：写进代码，后台不再能改；不按注册来源区分，也不在首次绑定第三方时额外发放）。
const (
	// NewUserConcurrency 新用户默认并发。
	NewUserConcurrency = 5
	// NewUserBalance 新用户初始余额（美元）：注册不送余额。
	NewUserBalance = 0.0
	// NewUserRPMLimit 新用户默认每分钟请求数上限，0 = 不限。
	NewUserRPMLimit = 0
	// NewUserRateMultiplier 全站默认计费倍率：官方价的十五分之一（muqian 2026-09-29 / 30）。
	// 用户倍率直接相对官方价：实付 = 目录官方价 × 用户倍率。没单独设倍率的用户（绝大多数）都按它，
	// 未登录时模型广场的展示价也按它；只有少数用户由管理员单独设（users.rate_multiplier 非空）。
	NewUserRateMultiplier = 1.0 / 15
)

// newUserDefaultSubscriptions 新用户（自助注册、后台新建）自动赠送的订阅：写死为空（2026-09-26 定）。
// 赠送代码保留，订阅功能打开后要送再在这里填；测试里临时改它来覆盖赠送逻辑。
var newUserDefaultSubscriptions []DefaultSubscriptionSetting

// NewUserDefaultSubscriptions 新用户自动赠送的订阅（副本）。
func NewUserDefaultSubscriptions() []DefaultSubscriptionSetting {
	return append([]DefaultSubscriptionSetting(nil), newUserDefaultSubscriptions...)
}

// registrationEmailSuffixWhitelist 注册邮箱域名白名单：不在单里的直接拒绝注册（2026-09-26 定，名单是我拟的、muqian 选用）。
// 启动时按注册校验的规则规整一遍，写错了直接启动失败。
var registrationEmailSuffixWhitelist = mustNormalizeRegistrationEmailSuffixWhitelist([]string{
	"@gmail.com", "@outlook.com", "@hotmail.com", "@live.com", "@icloud.com",
	"@qq.com", "@foxmail.com", "@163.com", "@126.com", "@yeah.net", "@sina.com", "@aliyun.com",
})

// RegistrationEmailSuffixWhitelist 注册邮箱域名白名单（副本）。
func RegistrationEmailSuffixWhitelist() []string {
	return append([]string(nil), registrationEmailSuffixWhitelist...)
}

func mustNormalizeRegistrationEmailSuffixWhitelist(raw []string) []string {
	normalized, err := NormalizeRegistrationEmailSuffixWhitelist(raw)
	if err != nil {
		panic("site_features: invalid registration email suffix whitelist: " + err.Error())
	}
	return normalized
}

// TablePageSizeOptions 列表可选的每页条数。
func TablePageSizeOptions() []int { return []int{10, 20, 50, 100} }

// 以下站点功能前端不再展示（方案定：删），值固定为空：站点副标题、自定义首页内容、简洁首页、
// 隐藏 CCS 导入按钮、自定义菜单、自定义端点。功能代码保留，公开设置里对应字段恒为空值。
