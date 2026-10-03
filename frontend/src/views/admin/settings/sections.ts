/**
 * 系统设置的小节（A6，muqian 2026-09-25 定：13 节，比方案多一节「安全」放管理 API Key / 客户端 IP / 面板限流；
 * 2026-09-26 删「第三方登录」一节——第三方登录只认部署配置，后台不再能配）。
 * 2026-09-27 删「品牌与首页」「条款」「注册与登录」「新用户默认值」「访问与限流」「支付方式」「邮件」七节：
 * 站点、条款、注册、新用户默认值、访问限流、通知写进后端代码，SMTP 与人机验证挪到部署配置，在线支付写死关，
 * /admin/settings 不再返回也不接受这些字段；管理 API Key 前端不再给入口（后端接口保留）。
 * 「开关」一节只剩风控：渠道监控写死开，运维监控只认部署配置 OPS_ENABLED。
 * 2026-09-27（上线收口 P4）：网关的「重试与冷却」「转发行为」「Claude Code · Codex」「上游余额探测」四节收成「其它」一节：
 * 冷却、流超时、整流、Beta / Fast 策略、转发细节、客户端版本、调度阈值、上游余额探测都写进后端代码，
 * 后台只留最低毛利率与联网搜索模拟；2026-10-04 联网搜索模拟也删了（方案页第三版）。
 * 每节一个地址 /settings/<key>；二级导航按组排列。
 */
import type { Component } from 'vue'
import OtherSection from './sections/OtherSection.vue'
import FeaturesSection from './sections/FeaturesSection.vue'

export const SETTINGS_SECTION_GROUPS = [
  { key: 'gateway', sections: ['other'] },
  { key: 'features', sections: ['features'] },
] as const

export type SettingsSectionKey = (typeof SETTINGS_SECTION_GROUPS)[number]['sections'][number]

export const SETTINGS_SECTIONS: Array<{ key: SettingsSectionKey }> = SETTINGS_SECTION_GROUPS.flatMap((group) =>
  group.sections.map((key) => ({ key }))
)

export const SECTION_COMPONENTS: Record<SettingsSectionKey, Component> = {
  other: OtherSection,
  features: FeaturesSection,
}

/** 路由参数 → 小节；缺省或不认识的一律落到第一节 */
export function resolveSettingsSection(param: unknown): SettingsSectionKey {
  const value = Array.isArray(param) ? param[0] : param
  return SETTINGS_SECTIONS.some((section) => section.key === value) ? (value as SettingsSectionKey) : SETTINGS_SECTIONS[0].key
}
