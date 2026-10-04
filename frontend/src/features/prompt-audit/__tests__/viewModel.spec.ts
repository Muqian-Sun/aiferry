import { describe, expect, it } from 'vitest'
import type { PromptAuditConfig } from '../types'
import {
  buildUpdateRequest,
  configToDraft,
  draftFingerprint,
  emptyEventFilters,
  eventFilterPayload,
  hasExplicitDeleteRange,
  SCANNER_CATALOG,
} from '../viewModel'

const config = (): PromptAuditConfig => ({
  enabled: true,
  blocking_enabled: false,
  effective_mode: 'async_audit',
  scanners: SCANNER_CATALOG.map((item) => item.id),
  endpoints: [{ id: 'guard-1', name: 'Guard One', base_url: 'http://127.0.0.1:8000', model: 'sileader/qwen3guard:0.6b', has_token: true }],
  config_version: 7,
  updated_at: '2026-07-16T00:00:00Z',
  updated_by: 1,
  change_summary: '{}',
})

describe('Prompt Audit view model', () => {
  it('normalizes legacy null collections from the public config', () => {
    const legacy = { ...config(), scanners: null } as unknown as PromptAuditConfig
    expect(configToDraft(legacy)).toMatchObject({ scanners: [] })
  })

  it('models all nine official input scanners', () => {
    expect(SCANNER_CATALOG).toHaveLength(9)
    expect(SCANNER_CATALOG.map((item) => item.id)).toContain('suicide_and_self_harm')
  })

  // 只发页面上能改的三项；工作线程、队列、节点都不在请求里
  it('sends only the editable fields with the expected config version', () => {
    const draft = configToDraft(config())
    draft.blocking_enabled = true
    expect(buildUpdateRequest(draft, 7)).toEqual({
      expected_config_version: 7, enabled: true, blocking_enabled: true, scanners: SCANNER_CATALOG.map((item) => item.id),
    })
    draft.enabled = false
    expect(buildUpdateRequest(draft, 7).blocking_enabled).toBe(false)
  })

  it('tracks dirty state from the full normalized save payload', () => {
    const original = configToDraft(config())
    const changed = configToDraft(config())
    expect(draftFingerprint(changed)).toBe(draftFingerprint(original))
    changed.scanners = changed.scanners.slice(1)
    expect(draftFingerprint(changed)).not.toBe(draftFingerprint(original))
  })

  it('requires a valid explicit range and sends canonical ISO timestamps for filter deletion', () => {
    const filters = emptyEventFilters()
    expect(hasExplicitDeleteRange(filters)).toBe(false)
    filters.start_at = '2026-07-15T10:00'
    filters.end_at = '2026-07-16T10:00'
    expect(hasExplicitDeleteRange(filters)).toBe(true)
    expect(eventFilterPayload(filters)).toMatchObject({
      start_at: new Date(filters.start_at).toISOString(),
      end_at: new Date(filters.end_at).toISOString(),
    })
  })
})
