/**
 * 由代码决定的功能开关：功能的启用 / 隐藏改这里、重新发版，不走后台设置（muqian 2026-09-25）。
 */
export const SITE_FEATURES: Readonly<{ subscription: boolean }> = {
  /**
   * 订阅：先不显示。跟着它走的有两站的订阅页与套餐页、账务「订阅」页签、购买页订阅套餐、
   * 用户站订阅数据的预加载与轮询、用量页「计费类型」筛选。只管显示，已有订阅照常计费。
   */
  subscription: false
}
