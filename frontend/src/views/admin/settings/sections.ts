/**
 * 系统设置的小节（A6，muqian 2026-09-25 定：13 节，比方案多一节「安全」放管理 API Key / 客户端 IP / 面板限流）。
 * 每节一个地址 /settings/<key>；二级导航按组排列。
 */
import type { Component } from 'vue'
import SiteSection from './sections/SiteSection.vue'
import AgreementSection from './sections/AgreementSection.vue'
import RegistrationSection from './sections/RegistrationSection.vue'
import OAuthSection from './sections/OAuthSection.vue'
import DefaultsSection from './sections/DefaultsSection.vue'
import SecuritySection from './sections/SecuritySection.vue'
import CooldownSection from './sections/CooldownSection.vue'
import ForwardingSection from './sections/ForwardingSection.vue'
import ClientsSection from './sections/ClientsSection.vue'
import UpstreamSection from './sections/UpstreamSection.vue'
import PaymentSection from './sections/PaymentSection.vue'
import EmailSection from './sections/EmailSection.vue'
import FeaturesSection from './sections/FeaturesSection.vue'

export const SETTINGS_SECTION_GROUPS = [
  { key: 'site', sections: ['site', 'agreement'] },
  { key: 'users', sections: ['registration', 'oauth', 'defaults'] },
  { key: 'security', sections: ['security'] },
  { key: 'gateway', sections: ['cooldown', 'forwarding', 'clients', 'upstream'] },
  { key: 'payment', sections: ['payment'] },
  { key: 'notify', sections: ['email'] },
  { key: 'features', sections: ['features'] },
] as const

export type SettingsSectionKey = (typeof SETTINGS_SECTION_GROUPS)[number]['sections'][number]

export const SETTINGS_SECTIONS: Array<{ key: SettingsSectionKey }> = SETTINGS_SECTION_GROUPS.flatMap((group) =>
  group.sections.map((key) => ({ key }))
)

export const SECTION_COMPONENTS: Record<SettingsSectionKey, Component> = {
  site: SiteSection,
  agreement: AgreementSection,
  registration: RegistrationSection,
  oauth: OAuthSection,
  defaults: DefaultsSection,
  security: SecuritySection,
  cooldown: CooldownSection,
  forwarding: ForwardingSection,
  clients: ClientsSection,
  upstream: UpstreamSection,
  payment: PaymentSection,
  email: EmailSection,
  features: FeaturesSection,
}

/** 路由参数 → 小节；缺省或不认识的一律落到第一节 */
export function resolveSettingsSection(param: unknown): SettingsSectionKey {
  const value = Array.isArray(param) ? param[0] : param
  return SETTINGS_SECTIONS.some((section) => section.key === value) ? (value as SettingsSectionKey) : SETTINGS_SECTIONS[0].key
}
