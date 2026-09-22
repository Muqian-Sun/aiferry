import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import VendorStrip from '../VendorStrip.vue'
import { modelIconKey, vendorIconKey } from '@/components/common/modelIconData'
import { HOME_CLIENTS, USE_KEY_CLIENTS } from '@/components/user/clients'

describe('VendorStrip', () => {
  it('renders a mono icon for vendors that have one and plain text for the rest', () => {
    const wrapper = mount(VendorStrip, { props: { vendors: ['openai', 'anthropic', 'volcengine', ''] } })
    const items = wrapper.findAll('li')
    expect(items.map((item) => item.text())).toEqual(['OpenAI', 'Anthropic', 'Volcengine', '—'])
    expect(items.map((item) => item.find('svg').exists())).toEqual([true, true, false, false])
    expect(items[0].find('svg').attributes('fill')).toBe('currentColor')
  })
})

describe('modelIconData keys', () => {
  it('maps catalog vendor tags to icon keys and leaves unknown vendors without one', () => {
    expect(vendorIconKey('anthropic')).toBe('claude')
    expect(vendorIconKey('google')).toBe('gemini')
    expect(vendorIconKey('vertex_ai-language-models')).toBe('gemini')
    expect(vendorIconKey('some-new-provider')).toBeNull()
  })

  it('keeps model-name detection working after the extraction', () => {
    expect(modelIconKey('gpt-5.6')).toBe('openai')
    expect(modelIconKey('claude-opus-5')).toBe('claude')
    expect(modelIconKey('something-else')).toBeNull()
  })
})

describe('use-key client list', () => {
  it('shows every client with a config snippet on the home page except the Codex WebSocket variant', () => {
    expect(USE_KEY_CLIENTS.map((c) => c.id)).toEqual(['claude', 'codex', 'codex-ws', 'gemini', 'grok', 'opencode'])
    expect(HOME_CLIENTS.map((c) => c.id)).toEqual(['claude', 'codex', 'gemini', 'grok', 'opencode'])
  })
})
