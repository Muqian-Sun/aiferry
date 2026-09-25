import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAppStore } from '@/stores/app'
import { FeatureFlags, isFeatureFlagEnabled, makeSidebarFlag, resolveFeatureFlag } from '@/utils/featureFlags'
import type { PublicSettings } from '@/types'

vi.mock('@/api/auth', () => ({
  getPublicSettings: vi.fn(),
}))

describe('FeatureFlags.payment', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    delete (window as any).__APP_CONFIG__
  })

  it('reads payment_enabled as an opt-out flag: visible before settings load', () => {
    expect(FeatureFlags.payment.key).toBe('payment_enabled')
    expect(FeatureFlags.payment.mode).toBe('opt-out')
    expect(useAppStore().cachedPublicSettings).toBeNull()
    expect(isFeatureFlagEnabled(FeatureFlags.payment)).toBe(true)
  })

  it('hides only when the backend explicitly sends false', () => {
    const store = useAppStore()
    const sidebarFlag = makeSidebarFlag(FeatureFlags.payment)

    store.cachedPublicSettings = { payment_enabled: false } as PublicSettings
    expect(sidebarFlag()).toBe(false)

    store.cachedPublicSettings = { payment_enabled: true } as PublicSettings
    expect(sidebarFlag()).toBe(true)

    store.cachedPublicSettings = {} as PublicSettings
    expect(sidebarFlag()).toBe(true)
  })
})

describe('resolveFeatureFlag', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('reads an explicit boolean from the given settings object', () => {
    expect(resolveFeatureFlag({ payment_enabled: false } as PublicSettings, FeatureFlags.payment)).toBe(false)
    expect(resolveFeatureFlag({ payment_enabled: true } as PublicSettings, FeatureFlags.payment)).toBe(true)
    expect(resolveFeatureFlag({ risk_control_enabled: true } as PublicSettings, FeatureFlags.riskControl)).toBe(true)
  })

  it('falls back to the declared mode when settings are missing or the key is absent', () => {
    expect(resolveFeatureFlag(undefined, FeatureFlags.payment)).toBe(true)
    expect(resolveFeatureFlag(null, FeatureFlags.payment)).toBe(true)
    expect(resolveFeatureFlag({} as PublicSettings, FeatureFlags.payment)).toBe(true)
    expect(resolveFeatureFlag({} as PublicSettings, FeatureFlags.riskControl)).toBe(false)
  })

  it('backs isFeatureFlagEnabled with the same resolution', () => {
    useAppStore().cachedPublicSettings = { payment_enabled: false } as PublicSettings
    expect(isFeatureFlagEnabled(FeatureFlags.payment)).toBe(false)
  })
})
