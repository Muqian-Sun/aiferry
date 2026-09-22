import { describe, expect, it } from 'vitest'
import {
  CC_SWITCH_CLIENT_TYPES,
  GROK_CC_SWITCH_MODEL,
  OPENAI_CC_SWITCH_CODEX_MODEL,
  buildCcSwitchImportDeeplink
} from '@/utils/ccswitchImport'

function paramsFromDeeplink(deeplink: string): URLSearchParams {
  const query = deeplink.split('?')[1] || ''
  return new URLSearchParams(query)
}

// 没有分组就没有「分组平台」：导入哪个客户端由用户选，app / endpoint / model 只看 clientType。
describe('ccswitchImport utils', () => {
  it('defaults OpenAI CC Switch imports to the current Codex model', () => {
    expect(OPENAI_CC_SWITCH_CODEX_MODEL).toBe('gpt-5.5')
  })

  it('defaults Grok Build imports to the current Grok model', () => {
    expect(GROK_CC_SWITCH_MODEL).toBe('grok-4.5')
  })

  it('offers the four clients a key can be used with', () => {
    expect(CC_SWITCH_CLIENT_TYPES).toEqual(['claude', 'codex', 'gemini', 'grokbuild'])
  })

  const baseInput = {
    baseUrl: 'https://api.example.com',
    providerName: 'Sub2API',
    apiKey: 'sk-test',
    usageScript: 'return true'
  }

  it('adds the Codex model parameter for Codex imports', () => {
    const params = paramsFromDeeplink(buildCcSwitchImportDeeplink({ ...baseInput, clientType: 'codex' }))

    expect(params.get('resource')).toBe('provider')
    expect(params.get('app')).toBe('codex')
    expect(params.get('endpoint')).toBe(baseInput.baseUrl)
    expect(params.get('model')).toBe(OPENAI_CC_SWITCH_CODEX_MODEL)
    expect(atob(params.get('usageScript') || '')).toBe(baseInput.usageScript)
  })

  it.each([
    'https://api.example.com',
    'https://api.example.com/',
    'https://api.example.com/v1',
    'https://api.example.com/v1/'
  ])('imports Grok Build with one /v1 suffix for base URL %s', (baseUrl) => {
    const params = paramsFromDeeplink(buildCcSwitchImportDeeplink({ ...baseInput, baseUrl, clientType: 'grokbuild' }))

    expect(params.get('app')).toBe('grokbuild')
    expect(params.get('endpoint')).toBe('https://api.example.com/v1')
    expect(params.get('model')).toBe(GROK_CC_SWITCH_MODEL)
  })

  it.each([
    { clientType: 'claude' as const, app: 'claude' },
    { clientType: 'gemini' as const, app: 'gemini' }
  ])('does not add a model parameter for $clientType imports', ({ clientType, app }) => {
    const params = paramsFromDeeplink(buildCcSwitchImportDeeplink({ ...baseInput, clientType }))

    expect(params.get('app')).toBe(app)
    expect(params.get('endpoint')).toBe(baseInput.baseUrl)
    expect(params.has('model')).toBe(false)
  })
})
