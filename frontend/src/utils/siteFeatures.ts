/**
 * 由代码决定的功能开关：功能的启用 / 隐藏改这里、重新发版，不走后台设置（muqian 2026-09-25）。
 * 路由用 meta.siteFeature 挂到某个开关上，关着时守卫拦回首页（router/siteGuard.ts）。
 */
export type SiteFeature = 'subscription' | 'batchImage' | 'accountSecurity' | 'promptAudit'

export const SITE_FEATURES: Readonly<Record<SiteFeature, boolean>> = {
  /**
   * 订阅：先不显示。跟着它走的有两站的订阅页与套餐页、账务「订阅」页签、购买页订阅套餐、
   * 用户站订阅数据的预加载与轮询、用量页「计费类型」筛选。只管显示，已有订阅照常计费。
   */
  subscription: false,
  /** 用户站批量生图：先不显示（muqian 2026-09-26）。侧栏条目、页面，以及侧栏按密钥探测权限的请求。 */
  batchImage: false,
  /**
   * 用户站账户「安全」页：先不显示（muqian 2026-09-26）。第三方绑定、改密码、两步验证、通行密钥整页不可见；
   * 已开两步验证的用户照常登录。
   */
  accountSecurity: false,
  /**
   * 管理站「审查 · 提示词」页：先不显示（muqian 2026-10-05：要调守卫模型，有成本）。页签与页面都不可见；
   * 功能本身默认关、部署配置里没配守卫节点也开不起来。
   */
  promptAudit: false
}
