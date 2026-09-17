import { beforeEach, describe, expect, it, vi } from 'vitest'

const { getProtocolDefaults } = vi.hoisted(() => ({ getProtocolDefaults: vi.fn() }))

vi.mock('@/api/admin/accounts', () => ({
  accountsAPI: { getProtocolDefaults }
}))

import type { ProtocolDefaultsResponse } from '@/api/admin/accounts'
import {
  applyPresetUrl,
  describeProtocolEndpointsIssue,
  endpointsAfterDefaultsChange,
  loadProtocolDefaults,
  protocolDefaultsFor,
  protocolDefaultsModeKey,
  resetProtocolDefaultsCacheForTest,
  trimProtocolEndpoints,
  validateProtocolEndpoints
} from '../protocolEndpoints'

const defaults: ProtocolDefaultsResponse = {
  protocols: ['anthropic', 'chat_completions', 'responses', 'gemini'],
  defaults: {
    openai: {
      default: { chat_completions: 'https://api.openai.com', responses: 'https://api.openai.com' }
    },
    kimi: {
      default: { chat_completions: 'https://api.moonshot.cn/v1' },
      coding: { chat_completions: 'https://api.kimi.com/coding/v1' }
    }
  }
}

describe('protocolDefaultsModeKey', () => {
  it.each([
    [undefined, 'default'],
    ['', 'default'],
    ['payg', 'default'],
    ['coding', 'coding'],
    ['zen', 'zen'],
    ['go', 'go']
  ])('maps %s to %s', (mode, key) => {
    expect(protocolDefaultsModeKey(mode)).toBe(key)
  })
})

describe('protocolDefaultsFor', () => {
  it('returns the official endpoints of the platform and mode', () => {
    expect(protocolDefaultsFor(defaults, 'kimi', 'coding')).toEqual({
      chat_completions: 'https://api.kimi.com/coding/v1'
    })
    expect(protocolDefaultsFor(defaults, 'kimi', 'payg')).toEqual({
      chat_completions: 'https://api.moonshot.cn/v1'
    })
  })

  it('returns an empty mapping instead of borrowing another mode or platform', () => {
    expect(protocolDefaultsFor(defaults, 'openai', 'coding')).toEqual({})
    expect(protocolDefaultsFor(defaults, 'antigravity')).toEqual({})
    expect(protocolDefaultsFor(null, 'openai')).toEqual({})
  })

  it('returns a copy so editing the form cannot mutate the cached table', () => {
    const endpoints = protocolDefaultsFor(defaults, 'openai')
    endpoints.chat_completions = 'https://relay.example.com/v1'
    expect(defaults.defaults.openai.default.chat_completions).toBe('https://api.openai.com')
  })
})

describe('endpointsAfterDefaultsChange', () => {
  const previous = { chat_completions: 'https://api.moonshot.cn/v1' }
  const next = { chat_completions: 'https://api.kimi.com/coding/v1' }

  it('switches to the new official endpoints when the admin has not edited them', () => {
    expect(endpointsAfterDefaultsChange({ ...previous }, previous, next)).toEqual(next)
  })

  it('fills the new official endpoints when nothing is configured', () => {
    expect(endpointsAfterDefaultsChange({}, previous, next)).toEqual(next)
  })

  it('keeps endpoints the admin typed in', () => {
    const edited = { chat_completions: 'https://relay.example.com/v1' }
    expect(endpointsAfterDefaultsChange(edited, previous, next)).toBe(edited)
  })

  it('treats an added or removed protocol as an edit', () => {
    const added = { ...previous, anthropic: 'https://relay.example.com' }
    expect(endpointsAfterDefaultsChange(added, previous, next)).toBe(added)
  })
})

describe('validateProtocolEndpoints', () => {
  it('requires at least one protocol', () => {
    expect(validateProtocolEndpoints({})).toEqual({ kind: 'empty' })
  })

  it('rejects a configured protocol whose address is blank', () => {
    expect(validateProtocolEndpoints({ anthropic: 'https://api.anthropic.com', responses: '   ' })).toEqual({
      kind: 'blank',
      protocol: 'responses'
    })
  })

  it('requires the protocol the account is pinned to', () => {
    expect(validateProtocolEndpoints({ chat_completions: 'https://api.deepseek.com' }, 'anthropic')).toEqual({
      kind: 'missingRequired',
      protocol: 'anthropic'
    })
  })

  it('accepts a complete mapping', () => {
    expect(
      validateProtocolEndpoints(
        { chat_completions: 'https://api.deepseek.com', anthropic: 'https://api.deepseek.com/anthropic' },
        'anthropic'
      )
    ).toBeNull()
  })
})

describe('describeProtocolEndpointsIssue', () => {
  const t = (key: string, params?: Record<string, unknown>) => (params ? `${key}|${params.protocol}` : key)

  it('names the protocol with its display label', () => {
    expect(describeProtocolEndpointsIssue({ kind: 'blank', protocol: 'responses' }, t)).toBe(
      'admin.accounts.protocolEndpoints.errors.blank|admin.accounts.protocolEndpoints.protocols.responses'
    )
    expect(describeProtocolEndpointsIssue({ kind: 'missingRequired', protocol: 'anthropic' }, t)).toBe(
      'admin.accounts.protocolEndpoints.errors.missingRequired|admin.accounts.protocolEndpoints.protocols.anthropic'
    )
    expect(describeProtocolEndpointsIssue({ kind: 'empty' }, t)).toBe('admin.accounts.protocolEndpoints.errors.empty')
  })
})

describe('applyPresetUrl', () => {
  const both = ['chat_completions', 'responses'] as const

  it('updates only the target protocols that are configured', () => {
    expect(
      applyPresetUrl({ responses: 'https://api.x.ai/v1', anthropic: 'https://a.example' }, both, 'https://eu-west-1.api.x.ai/v1')
    ).toEqual({ responses: 'https://eu-west-1.api.x.ai/v1', anthropic: 'https://a.example' })
  })

  it('fills every target protocol when none is configured', () => {
    expect(applyPresetUrl({}, both, 'https://api.x.ai/v1')).toEqual({
      chat_completions: 'https://api.x.ai/v1',
      responses: 'https://api.x.ai/v1'
    })
  })
})

describe('trimProtocolEndpoints', () => {
  it('trims addresses without dropping or adding protocols', () => {
    expect(trimProtocolEndpoints({ anthropic: '  https://api.anthropic.com  ', gemini: 'https://g.example' })).toEqual({
      anthropic: 'https://api.anthropic.com',
      gemini: 'https://g.example'
    })
  })
})

describe('loadProtocolDefaults', () => {
  beforeEach(() => {
    resetProtocolDefaultsCacheForTest()
    getProtocolDefaults.mockReset()
  })

  it('fetches once and reuses the result', async () => {
    getProtocolDefaults.mockResolvedValue(defaults)

    await expect(loadProtocolDefaults()).resolves.toBe(defaults)
    await expect(loadProtocolDefaults()).resolves.toBe(defaults)

    expect(getProtocolDefaults).toHaveBeenCalledTimes(1)
  })

  it('does not cache a failure', async () => {
    getProtocolDefaults.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(defaults)

    await expect(loadProtocolDefaults()).rejects.toThrow('offline')
    await expect(loadProtocolDefaults()).resolves.toBe(defaults)

    expect(getProtocolDefaults).toHaveBeenCalledTimes(2)
  })
})
