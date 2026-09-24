import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import GrokFreeIcon from '../GrokFreeIcon.vue'
import PlatformTypeBadge from '../PlatformTypeBadge.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

describe('PlatformTypeBadge Grok plans', () => {
  it('renders FREE and BASIC as Grok Free with a lightweight plan icon', async () => {
    const wrapper = mount(PlatformTypeBadge, {
      props: {
        platform: 'grok',
        type: 'oauth',
        planType: 'BASIC',
        subscriptionExpiresAt: '2027-01-01T00:00:00Z',
      },
    })

    expect(wrapper.text()).toContain('Grok Free')
    expect(wrapper.findComponent(GrokFreeIcon).exists()).toBe(true)
    expect(wrapper.find('[data-testid="grok-free-plan-icon"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="grok-plan-icon"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('2027-01-01')

    await wrapper.setProps({ planType: 'FREE' })
    expect(wrapper.text()).toContain('Grok Free')
    expect(wrapper.findComponent(GrokFreeIcon).exists()).toBe(true)
  })

  it('keeps SuperGrok labels compatible and marks paid Grok plans', async () => {
    const wrapper = mount(PlatformTypeBadge, {
      props: {
        platform: 'grok',
        type: 'oauth',
        planType: 'SuperGrok Heavy',
      },
    })

    expect(wrapper.text()).toContain('SuperGrok Heavy')
    expect(wrapper.find('[data-testid="grok-plan-icon"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="grok-free-plan-icon"]').exists()).toBe(false)

    await wrapper.setProps({ platform: 'openai', planType: 'free' })
    expect(wrapper.text()).toContain('Free')
    expect(wrapper.text()).not.toContain('Grok Free')
    expect(wrapper.find('[data-testid="grok-plan-icon"]').exists()).toBe(false)
  })

  it('labels free, SuperGrok, Heavy and Lite plans (plan chips are neutral, no per-tier color)', async () => {
    const free = mount(PlatformTypeBadge, {
      props: { platform: 'grok', type: 'oauth', planType: 'free' },
    })
    expect(free.text()).toContain('Free')

    const superGrok = mount(PlatformTypeBadge, {
      props: { platform: 'grok', type: 'oauth', planType: 'supergrok' },
    })
    expect(superGrok.text()).toContain('SuperGrok')
    expect(superGrok.find('[data-testid="grok-plan-icon"]').exists()).toBe(true)

    const heavy = mount(PlatformTypeBadge, {
      props: { platform: 'grok', type: 'oauth', planType: 'Heavy' },
    })
    expect(heavy.text()).toContain('Heavy')
    expect(heavy.find('[data-testid="grok-plan-icon"]').exists()).toBe(true)

    const lite = mount(PlatformTypeBadge, {
      props: { platform: 'grok', type: 'oauth', planType: 'supergrok_lite' },
    })
    expect(lite.text()).toContain('SuperGrok Lite')
  })

  it('uses a dedicated 12px currentColor Grok mark with a Free sparkle', () => {
    const wrapper = mount(GrokFreeIcon)

    expect(wrapper.element.tagName.toLowerCase()).toBe('svg')
    expect(wrapper.attributes('fill')).toBe('currentColor')
    expect(wrapper.classes()).toEqual(expect.arrayContaining(['h-3', 'w-3']))
    expect(wrapper.findAll('path')).toHaveLength(2)
  })
})

describe('PlatformTypeBadge OpenAI authentication modes', () => {
  it('distinguishes Agent Identity, PAT, and OAuth accounts', async () => {
    const wrapper = mount(PlatformTypeBadge, {
      props: {
        platform: 'openai',
        type: 'oauth',
        authMode: 'agentIdentity',
      },
    })

    expect(wrapper.text()).toContain('Agent Identity')

    await wrapper.setProps({ authMode: 'personalAccessToken' })
    expect(wrapper.text()).toContain('PAT')
    expect(wrapper.text()).not.toContain('Agent Identity')

    await wrapper.setProps({ authMode: undefined })
    expect(wrapper.text()).toContain('OAuth')
  })
})

describe('PlatformTypeBadge MiniMax', () => {
  it('labels official MiniMax API keys as MiniMax, not Gemini', () => {
    const wrapper = mount(PlatformTypeBadge, {
      props: {
        platform: 'minimax',
        type: 'apikey',
        // 第三方 key 按后端识别出的厂商显示；这里模拟官方 MiniMax 地址的 key。
        vendor: 'minimax',
      },
    })

    expect(wrapper.text()).toContain('MiniMax')
    expect(wrapper.text()).toContain('Key')
    expect(wrapper.text()).not.toContain('Gemini')
  })
})
