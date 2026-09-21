import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import PlatformTypeBadge from '../PlatformTypeBadge.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

// 第三方 key 的 platform 只是展示标签：徽章按后端按地址识别出的 vendor 显示，
// 没识别出厂商的 key 显示为中转；成品号仍按 platform。
describe('PlatformTypeBadge vendor display for third-party keys', () => {
  it('shows the identified vendor instead of the label', () => {
    const wrapper = mount(PlatformTypeBadge, {
      props: { platform: 'openai', type: 'apikey', vendor: 'deepseek' },
    })
    const badge = wrapper.get('[data-testid="platform-badge"]')
    expect(badge.text()).toContain('DeepSeek')
    expect(badge.text()).not.toContain('OpenAI')
    expect(badge.attributes('title')).toContain('label: OpenAI')
  })

  it('shows Relay when no official vendor was identified', () => {
    const wrapper = mount(PlatformTypeBadge, {
      props: { platform: 'kimi', type: 'apikey', vendor: '' },
    })
    const badge = wrapper.get('[data-testid="platform-badge"]')
    expect(badge.text()).toContain('Relay')
    expect(badge.text()).not.toContain('Kimi')
    expect(badge.attributes('title')).toContain('label: Kimi')
  })

  it('keeps subscription accounts on their platform', () => {
    const wrapper = mount(PlatformTypeBadge, {
      props: { platform: 'anthropic', type: 'oauth', vendor: 'anthropic' },
    })
    const badge = wrapper.get('[data-testid="platform-badge"]')
    expect(badge.text()).toContain('Anthropic')
    expect(badge.attributes('title')).toBeUndefined()
  })
})
