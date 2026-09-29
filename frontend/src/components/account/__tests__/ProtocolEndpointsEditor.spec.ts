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

// 一个 key 只承接一个上游协议（后端拒绝多协议）：编辑器是「协议 + 地址」一行。
describe('ProtocolEndpointsEditor', () => {
  it('shows the configured protocol and its address', () => {
    const wrapper = mountEditor({ anthropic: 'https://a.example' })

    const select = wrapper.find('[data-testid="protocol-endpoint-protocol"]').element as HTMLSelectElement
    expect(select.value).toBe('anthropic')
    const input = wrapper.find('[data-testid="protocol-endpoint-input-anthropic"]').element as HTMLInputElement
    expect(input.value).toBe('https://a.example')
  })

  it('emits an updated copy when the address is edited', async () => {
    const value = { anthropic: 'https://a.example' }
    const wrapper = mountEditor(value)

    await wrapper.find('[data-testid="protocol-endpoint-input-anthropic"]').setValue('https://relay.example')

    expect(lastEmitted(wrapper)).toEqual({ anthropic: 'https://relay.example' })
    expect(value).toEqual({ anthropic: 'https://a.example' })
  })

  it('switches protocol and keeps an address the admin typed in', async () => {
    const wrapper = mountEditor({ anthropic: 'https://relay.example' }, ALL, { anthropic: 'https://api.moonshot.cn/anthropic' })

    await wrapper.find('[data-testid="protocol-endpoint-protocol"]').setValue('responses')

    expect(lastEmitted(wrapper)).toEqual({ responses: 'https://relay.example' })
  })

  it('switches to the new protocol\'s official address when the current one was untouched', async () => {
    const official = { anthropic: 'https://api.moonshot.cn/anthropic', chat_completions: 'https://api.moonshot.cn/v1' }
    const wrapper = mountEditor({ anthropic: 'https://api.moonshot.cn/anthropic' }, ALL, official)

    await wrapper.find('[data-testid="protocol-endpoint-protocol"]').setValue('chat_completions')

    expect(lastEmitted(wrapper)).toEqual({ chat_completions: 'https://api.moonshot.cn/v1' })
  })

  // 还没选协议也能先填地址（草稿），好用「探测协议」（muqian 2026-09-29）
  it('shows the empty state and keeps the address box usable as a draft when nothing is configured', async () => {
    const wrapper = mountEditor({})

    expect(wrapper.find('[data-testid="protocol-endpoints-empty"]').exists()).toBe(true)
    const input = wrapper.get('[data-testid="protocol-endpoint-input-none"]')
    expect(input.attributes('disabled')).toBeUndefined()
    await input.setValue('https://relay.example.com')
    expect(wrapper.emitted('update:draftUrl')?.at(-1)).toEqual(['https://relay.example.com'])
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('offers the official address only when it differs from the current value', () => {
    const official = { anthropic: 'https://api.moonshot.cn/anthropic' }

    expect(
      mountEditor({ anthropic: 'https://relay.example' }, ALL, official)
        .find('[data-testid="protocol-endpoints-restore-official"]').exists()
    ).toBe(true)
    expect(
      mountEditor({ anthropic: 'https://api.moonshot.cn/anthropic' }, ALL, official)
        .find('[data-testid="protocol-endpoints-restore-official"]').exists()
    ).toBe(false)
    expect(
      mountEditor({ anthropic: 'https://relay.example' }, ALL, {})
        .find('[data-testid="protocol-endpoints-restore-official"]').exists()
    ).toBe(false)
  })

  it('restores only the current protocol\'s official address', async () => {
    const official = { chat_completions: 'https://api.moonshot.cn/v1', responses: 'https://api.moonshot.cn/v1' }
    const wrapper = mountEditor({ responses: 'https://relay.example' }, ALL, official)

    await wrapper.find('[data-testid="protocol-endpoints-restore-official"]').trigger('click')

    expect(lastEmitted(wrapper)).toEqual({ responses: 'https://api.moonshot.cn/v1' })
  })

  it('tells the admin to fill addresses manually when official addresses failed to load', () => {
    const failed = mount(ProtocolEndpointsEditor, {
      props: { modelValue: {}, protocols: ALL, defaultsLoadFailed: true },
      global: { plugins: [i18n], stubs: { Icon: true } }
    })
    expect(failed.find('[data-testid="protocol-defaults-load-failed"]').exists()).toBe(true)
    expect(mountEditor({}).find('[data-testid="protocol-defaults-load-failed"]').exists()).toBe(false)
  })

  it('still shows a stored protocol that is not in the selectable list', () => {
    const wrapper = mountEditor({ gemini: 'https://g.example' }, ['anthropic'])

    expect(wrapper.find('[data-testid="protocol-endpoint-input-gemini"]').exists()).toBe(true)
    const options = wrapper.findAll('[data-testid="protocol-endpoint-protocol"] option').map((option) => option.attributes('value'))
    expect(options).toContain('gemini')
  })
})
