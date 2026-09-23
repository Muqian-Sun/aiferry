package service

import _ "embed"

// 出厂条款正文（markdown），站点没配置登录协议文档时作为默认值；管理员可在后台改写。
// muqian 2026-09-23 定：只留「禁止做什么」和「隐私政策」两份；书面但简洁，不写成正式合同，不点名模型厂商。
// 隐私政策按代码里的真实行为写（例如失败请求的排障留存期），改动相关逻辑时要同步改这里。
var (
	//go:embed legal/usage-policy.md
	legalUsagePolicyMD string
	//go:embed legal/privacy.md
	legalPrivacyMD string
)
