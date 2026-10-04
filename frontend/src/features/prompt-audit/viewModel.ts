import type {
  PromptAuditConfig,
  PromptAuditDraft,
  PromptAuditUpdateRequest,
  PromptEventFilters,
} from './types'

export const SCANNER_CATALOG = [
  { id: 'violent', label: 'Violent' },
  { id: 'non_violent_illegal_acts', label: 'Non-violent Illegal Acts' },
  { id: 'sexual_content_or_sexual_acts', label: 'Sexual Content or Sexual Acts' },
  { id: 'pii', label: 'PII' },
  { id: 'suicide_and_self_harm', label: 'Suicide & Self-Harm' },
  { id: 'unethical_acts', label: 'Unethical Acts' },
  { id: 'politically_sensitive_topics', label: 'Politically Sensitive Topics' },
  { id: 'copyright_violation', label: 'Copyright Violation' },
  { id: 'jailbreak', label: 'Jailbreak' },
] as const

// Vue props/refs are proxies and cannot be passed to structuredClone in every
// browser. Prompt Audit state is JSON-only, so this produces a detached draft
// without retaining reactive proxies or browser storage references.
export function cloneData<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T
}

export function configToDraft(config: PromptAuditConfig): PromptAuditDraft {
  return {
    enabled: config.enabled,
    blocking_enabled: config.blocking_enabled,
    scanners: [...(config.scanners ?? [])],
  }
}

export function buildUpdateRequest(draft: PromptAuditDraft, configVersion: number): PromptAuditUpdateRequest {
  return {
    expected_config_version: configVersion,
    enabled: draft.enabled,
    blocking_enabled: draft.enabled && draft.blocking_enabled,
    scanners: [...draft.scanners],
  }
}

export function draftFingerprint(draft: PromptAuditDraft | null): string {
  if (!draft) return ''
  return JSON.stringify({ ...buildUpdateRequest(draft, 0), scanners: [...draft.scanners].sort() })
}

export function emptyEventFilters(): PromptEventFilters {
  return {
    decision: '',
    risk_level: '',
    endpoint: '',
    user_id: '',
    api_key_id: '',
    request_id: '',
    prompt_hash: '',
    keyword: '',
    start_at: '',
    end_at: '',
  }
}

function toISO(value: string): string | undefined {
  if (!value.trim()) return undefined
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString()
}

export function eventQueryParams(filters: PromptEventFilters): Record<string, string | number> {
  const result: Record<string, string | number> = {}
  for (const key of ['decision', 'risk_level', 'endpoint', 'request_id', 'prompt_hash', 'keyword'] as const) {
    const value = filters[key].trim()
    if (value) result[key] = value
  }
  for (const key of ['user_id', 'api_key_id'] as const) {
    const value = Number(filters[key])
    if (Number.isInteger(value) && value > 0) result[key] = value
  }
  const start = toISO(filters.start_at)
  const end = toISO(filters.end_at)
  if (start) result.start_at = start
  if (end) result.end_at = end
  return result
}

export function eventFilterPayload(filters: PromptEventFilters): Record<string, unknown> {
  return eventQueryParams(filters)
}

export function hasExplicitDeleteRange(filters: PromptEventFilters): boolean {
  const start = toISO(filters.start_at)
  const end = toISO(filters.end_at)
  return Boolean(start && end && new Date(start).getTime() < new Date(end).getTime())
}

export type DeleteRangePreset = '1d' | '7d' | '30d' | '90d' | 'all' | 'custom'

export const DELETE_RANGE_PRESETS: ReadonlyArray<{ id: DeleteRangePreset; days: number | null }> = [
  { id: '1d', days: 1 },
  { id: '7d', days: 7 },
  { id: '30d', days: 30 },
  { id: '90d', days: 90 },
  { id: 'all', days: null },
  { id: 'custom', days: null },
]

const DAY_MS = 24 * 60 * 60 * 1000

// Presets delete events older than the chosen cutoff: the range always starts
// at the epoch and ends at (now - days) so the backend's explicit-range
// requirement is satisfied without asking the user for a begin date.
export function resolveDeleteRangeFilters(
  filters: PromptEventFilters,
  preset: DeleteRangePreset,
  now: number = Date.now(),
): PromptEventFilters {
  const resolved = cloneData(filters)
  if (preset === 'custom') return resolved
  const days = DELETE_RANGE_PRESETS.find((item) => item.id === preset)?.days ?? null
  resolved.start_at = new Date(0).toISOString()
  resolved.end_at = new Date(days === null ? now : now - days * DAY_MS).toISOString()
  return resolved
}

// 守卫节点探测、审计线程最近一次出错的错误码（prompt_service.go Probe / prompt_worker.go setLastError）；
// 页面按码给文案，不显示后端原文。认不出的码走通用文案并带上码。
const GUARD_ERROR_KEYS: Record<string, string> = {
  prompt_guard_unavailable: 'admin.promptAudit.guardErrors.unavailable',
  prompt_guard_invalid_response: 'admin.promptAudit.guardErrors.invalidResponse',
  connection_failed: 'admin.promptAudit.guardErrors.connectionFailed',
  timeout: 'admin.promptAudit.guardErrors.timeout',
  authentication_failed: 'admin.promptAudit.guardErrors.authenticationFailed',
  probe_http_error: 'admin.promptAudit.guardErrors.httpError',
  response_read_failed: 'admin.promptAudit.guardErrors.responseReadFailed',
  response_too_large: 'admin.promptAudit.guardErrors.responseTooLarge',
  endpoint_not_found: 'admin.promptAudit.guardErrors.endpointNotFound',
  endpoint_unsafe: 'admin.promptAudit.guardErrors.endpointUnsafe',
  probe_request_invalid: 'admin.promptAudit.guardErrors.probeRequestInvalid',
  database_unavailable: 'admin.promptAudit.guardErrors.databaseUnavailable',
  payload_store_unavailable: 'admin.promptAudit.guardErrors.payloadStoreUnavailable',
  payload_missing: 'admin.promptAudit.guardErrors.payloadMissing',
  claim_job_failed: 'admin.promptAudit.guardErrors.claimJobFailed',
  reclaim_failed: 'admin.promptAudit.guardErrors.reclaimFailed',
  worker_panic: 'admin.promptAudit.guardErrors.workerPanic',
}

export function guardErrorText(t: (key: string, params?: Record<string, unknown>) => string, code: string): string {
  const key = GUARD_ERROR_KEYS[code]
  return key ? t(key) : t('admin.promptAudit.guardErrors.unknown', { code })
}
