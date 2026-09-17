import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import ProtocolEndpointsEditor from '../ProtocolEndpointsEditor.vue'
import type { ProtocolEndpoints, UpstreamProtocol } from '@/types'

const i18n = createI18n({
  legacy: false,
  locale: 'en',
  missingWarn: false,
  fallbackWarn: false,
  messages: { en: {} }
})

const ALL: UpstreamProtocol[] = ['anthropic', 'chat_completions', 'responses', 'gemini']

function mountEditor(
  modelValue: ProtocolEndpoints,
  protocols: UpstreamProtocol[] = ALL,
  officialEndpoints?: ProtocolEndpoints
) {
  return mount(ProtocolEndpointsEditor, {
    props: { modelValue, protocols, officialEndpoints },
    global: {
      plugins: [i18n],
      stubs: { Icon: true }
    }
  })
}

function lastEmitted(wrapper: ReturnType<typeof mountEditor>): ProtocolEndpoints {
  const events = wrapper.emitted('update:modelValue')
  expect(events).toBeTruthy()
  return events![events!.length - 1][0] as ProtocolEndpoints
}

describe('ProtocolEndpointsEditor', () => {
  it('renders configured protocols in the given protocol order', () => {
    const wrapper = mountEditor({ responses: 'https://r.example', anthropic: 'https://a.example' })

    const inputs = wrapper.findAll('input')
    expect(inputs.map((input) => input.attributes('data-testid'))).toEqual([
      'protocol-endpoint-input-anthropic',
      'protocol-endpoint-input-responses'
    ])
    expect((inputs[0].element as HTMLInputElement).value).toBe('https://a.example')
  })

  it('offers only the protocols that are not configured yet', () => {
    const wrapper = mountEditor({ anthropic: 'https://a.example' })

    expect(wrapper.find('[data-testid="protocol-endpoint-add-anthropic"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="protocol-endpoint-add-chat_completions"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="protocol-endpoint-add-gemini"]').exists()).toBe(true)
  })

  it('emits an updated copy when an address is edited', async () => {
    const value = { anthropic: 'https://a.example' }
    const wrapper = mountEditor(value)

    await wrapper.find('[data-testid="protocol-endpoint-input-anthropic"]').setValue('https://relay.example')

    expect(lastEmitted(wrapper)).toEqual({ anthropic: 'https://relay.example' })
    expect(value).toEqual({ anthropic: 'https://a.example' })
  })

  it('adds a protocol with an empty address so validation can flag it', async () => {
    const wrapper = mountEditor({ anthropic: 'https://a.example' })

    await wrapper.find('[data-testid="protocol-endpoint-add-gemini"]').trigger('click')

    expect(lastEmitted(wrapper)).toEqual({ anthropic: 'https://a.example', gemini: '' })
  })

  it('removes a protocol entirely instead of leaving an empty address', async () => {
    const wrapper = mountEditor({ anthropic: 'https://a.example', responses: 'https://r.example' })

    await wrapper.find('[data-testid="protocol-endpoint-remove-responses"]').trigger('click')

    const next = lastEmitted(wrapper)
    expect(next).toEqual({ anthropic: 'https://a.example' })
    expect('responses' in next).toBe(false)
  })

  it('shows the empty state when nothing is configured', () => {
    const wrapper = mountEditor({})

    expect(wrapper.find('[data-testid="protocol-endpoints-empty"]').exists()).toBe(true)
    expect(wrapper.findAll('input')).toHaveLength(0)
  })

  it('offers the official addresses only when they differ from the current value', () => {
    const official = { anthropic: 'https://api.anthropic.com' }

    expect(
      mountEditor({ anthropic: 'https://relay.example' }, ALL, official)
        .find('[data-testid="protocol-endpoints-restore-official"]').exists()
    ).toBe(true)
    expect(
      mountEditor({ anthropic: 'https://api.anthropic.com' }, ALL, official)
        .find('[data-testid="protocol-endpoints-restore-official"]').exists()
    ).toBe(false)
    expect(
      mountEditor({ anthropic: 'https://relay.example' }, ALL, {})
        .find('[data-testid="protocol-endpoints-restore-official"]').exists()
    ).toBe(false)
  })

  it('replaces the whole mapping with a copy of the official addresses', async () => {
    const official = { chat_completions: 'https://api.openai.com', responses: 'https://api.openai.com' }
    const wrapper = mountEditor({ anthropic: 'https://relay.example' }, ALL, official)

    await wrapper.find('[data-testid="protocol-endpoints-restore-official"]').trigger('click')

    const next = lastEmitted(wrapper)
    expect(next).toEqual(official)
    // 发出的必须是副本：之后在表单里改地址不能改到官方地址表。
    next.chat_completions = 'https://relay.example/v1'
    expect(official.chat_completions).toBe('https://api.openai.com')
  })

  it('still shows a stored protocol that is not in the selectable list', () => {
    const wrapper = mountEditor({ gemini: 'https://g.example' }, ['anthropic'])

    expect(wrapper.find('[data-testid="protocol-endpoint-input-gemini"]').exists()).toBe(true)
  })
})
