<template>
  <!-- 标题统一「错误详情」，不写内部编号；请求 ID 在摘要第一格，可复制 -->
  <BaseDialog :show="show" :title="t('admin.ops.errorDetail.title')" width="full" :close-on-click-outside="true" @close="close">
    <div v-if="loading" class="flex items-center justify-center py-16">
      <div class="flex flex-col items-center gap-3">
        <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-af-brand"></div>
        <div class="text-sm font-medium text-af-ink-3">{{ t('admin.ops.errorDetail.loading') }}</div>
      </div>
    </div>

    <div v-else-if="!detail" class="py-10 text-center text-sm text-af-ink-3">
      {{ emptyText }}
    </div>

    <div v-else class="space-y-6 p-6">
      <!-- Summary -->
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div class="rounded-xl bg-af-sunken p-4">
          <div class="text-xs font-bold uppercase tracking-wider text-af-ink-3">{{ t('admin.ops.errorDetail.requestId') }}</div>
          <div v-if="requestId" class="mt-1 flex items-start gap-1.5">
            <span class="min-w-0 break-all font-mono text-sm font-medium text-af-ink">{{ requestId }}</span>
            <CopyButton
              :text="requestId"
              class="shrink-0 rounded p-0.5 text-af-ink-3 transition-colors hover:bg-af-sheet hover:text-af-ink-2"
              test-id="error-detail-copy-request-id"
            />
          </div>
          <div v-else class="mt-1 text-sm font-medium text-af-ink">—</div>
        </div>

        <div class="rounded-xl bg-af-sunken p-4">
          <div class="text-xs font-bold uppercase tracking-wider text-af-ink-3">{{ t('admin.ops.errorDetail.time') }}</div>
          <div class="mt-1 text-sm font-medium text-af-ink">
            {{ formatDateTime(detail.created_at) }}
          </div>
        </div>

        <div class="rounded-xl bg-af-sunken p-4">
          <div class="text-xs font-bold uppercase tracking-wider text-af-ink-3">
            {{ isUpstreamError(detail) ? t('admin.ops.errorDetail.account') : t('admin.ops.errorDetail.user') }}
          </div>
          <div class="mt-1 text-sm font-medium text-af-ink">
            <!-- 有 id 却查不到名字就是已删除，不回退成数字 id -->
            <template v-if="isUpstreamError(detail)">
              {{ detail.account_name || (detail.account_id != null ? t('common.deletedChannel') : '—') }}
            </template>
            <template v-else>
              {{ detail.user_email || (detail.user_id != null ? t('common.deletedUser') : '—') }}
            </template>
          </div>
        </div>

        <div class="rounded-xl bg-af-sunken p-4">
          <div class="text-xs font-bold uppercase tracking-wider text-af-ink-3">{{ t('admin.ops.errorDetail.platform') }}</div>
          <div class="mt-1 text-sm font-medium text-af-ink">
            {{ detail.platform || '—' }}
          </div>
        </div>


        <div class="rounded-xl bg-af-sunken p-4">
          <div class="text-xs font-bold uppercase tracking-wider text-af-ink-3">{{ t('admin.ops.errorDetail.model') }}</div>
          <div class="mt-1 text-sm font-medium text-af-ink">
            <template v-if="hasModelMapping(detail)">
              <span class="font-mono">{{ detail.requested_model }}</span>
              <span class="mx-1 text-af-ink-3">→</span>
              <span class="font-mono text-af-brand">{{ detail.upstream_model }}</span>
            </template>
            <template v-else>
              {{ displayModel(detail) || '—' }}
            </template>
          </div>
        </div>

        <div class="rounded-xl bg-af-sunken p-4">
          <div class="text-xs font-bold uppercase tracking-wider text-af-ink-3">{{ t('admin.ops.errorDetail.inboundEndpoint') }}</div>
          <div class="mt-1 break-all font-mono text-sm font-medium text-af-ink">
            {{ detail.inbound_endpoint || '—' }}
          </div>
        </div>

        <div class="rounded-xl bg-af-sunken p-4">
          <div class="text-xs font-bold uppercase tracking-wider text-af-ink-3">{{ t('admin.ops.errorDetail.upstreamEndpoint') }}</div>
          <div class="mt-1 break-all font-mono text-sm font-medium text-af-ink">
            {{ detail.upstream_endpoint || '—' }}
          </div>
        </div>

        <div class="rounded-xl bg-af-sunken p-4">
          <div class="text-xs font-bold uppercase tracking-wider text-af-ink-3">{{ t('admin.ops.errorDetail.status') }}</div>
          <div class="mt-1">
            <span :class="['inline-flex items-center rounded-lg px-2 py-1 text-xs font-black ring-1 ring-inset', statusClass]">
              {{ detail.status_code }}
            </span>
          </div>
        </div>

        <div class="rounded-xl bg-af-sunken p-4">
          <div class="text-xs font-bold uppercase tracking-wider text-af-ink-3">{{ t('admin.ops.errorDetail.upstreamStatus') }}</div>
          <div class="mt-1">
            <span :class="['inline-flex items-center rounded-lg px-2 py-1 text-xs font-black ring-1 ring-inset', upstreamStatusClass]">
              {{ detail.upstream_status_code ?? '—' }}
            </span>
          </div>
        </div>

        <div class="rounded-xl bg-af-sunken p-4">
          <div class="text-xs font-bold uppercase tracking-wider text-af-ink-3">{{ t('admin.ops.errorDetail.requestType') }}</div>
          <div class="mt-1 text-sm font-medium text-af-ink">
            {{ formatRequestTypeLabel(detail.request_type) }}
          </div>
        </div>

        <div class="rounded-xl bg-af-sunken p-4">
          <div class="text-xs font-bold uppercase tracking-wider text-af-ink-3">{{ t('admin.ops.errorDetail.message') }}</div>
          <div class="mt-1 break-words text-sm font-medium text-af-ink" :title="rootCauseMessage">
            {{ rootCauseMessage || '—' }}
          </div>
        </div>

        <div v-if="detail.api_key_prefix" class="rounded-xl bg-af-sunken p-4">
          <div class="text-xs font-bold uppercase tracking-wider text-af-ink-3">{{ t('admin.ops.errorDetail.apiKeyPrefix') }}</div>
          <div class="mt-1 font-mono text-sm font-medium text-af-ink">
            {{ detail.api_key_prefix }}
          </div>
        </div>

      </div>

      <div v-if="rootCauseMessage" class="rounded-xl bg-af-warning-tint p-6">
        <h3 class="text-sm font-black uppercase tracking-wider text-af-warning">{{ t('admin.ops.errorDetail.rootCause') }}</h3>
        <div class="mt-3 break-words text-sm font-medium text-af-warning">{{ rootCauseMessage }}</div>
      </div>

      <div class="rounded-xl bg-af-sunken p-6">
        <h3 class="text-sm font-black uppercase tracking-wider text-af-ink">{{ t('admin.ops.errorDetail.diagnosticPayloads') }}</h3>
        <div v-if="!diagnosticPayloadSections.length" class="mt-4 text-sm text-af-ink-3">{{ t('common.noData') }}</div>
        <div v-else class="mt-4 space-y-4">
          <div v-for="section in diagnosticPayloadSections" :key="section.key">
            <div class="mb-2 text-xs font-bold uppercase tracking-wider text-af-ink-3">{{ diagnosticPayloadLabel(section.key) }}</div>
            <pre class="max-h-[520px] overflow-auto rounded-xl border border-af-hairline bg-af-sheet p-4 text-xs text-af-ink"><code>{{ prettyJSON(section.value) }}</code></pre>
          </div>
        </div>
      </div>

      <!-- Upstream errors list (only for request errors) -->
      <div v-if="showUpstreamList" class="rounded-xl bg-af-sunken p-6">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <h3 class="text-sm font-black uppercase tracking-wider text-af-ink">{{ t('admin.ops.errorDetails.upstreamErrors') }}</h3>
          <div class="text-xs text-af-ink-3" v-if="correlatedUpstreamLoading">{{ t('common.loading') }}</div>
        </div>

        <div v-if="!correlatedUpstreamLoading && !correlatedUpstreamErrors.length" class="mt-3 text-sm text-af-ink-3">
          {{ t('common.noData') }}
        </div>

        <div v-else class="mt-4 space-y-3">
          <div
            v-for="(ev, idx) in correlatedUpstreamErrors"
            :key="ev.id"
            class="rounded-xl border border-af-hairline bg-af-sheet p-4"
          >
            <div class="flex flex-wrap items-center justify-between gap-2">
              <div class="text-xs font-black text-af-ink">
                #{{ idx + 1 }}
                <span v-if="ev.type" class="ml-2 rounded-md bg-af-sunken px-2 py-0.5 font-mono text-xs font-bold text-af-ink-2">{{ ev.type }}</span>
              </div>
              <div class="flex items-center gap-2">
                <div class="font-mono text-xs text-af-ink-3">
                  {{ ev.status_code ?? '—' }}
                </div>
                <button
                  type="button"
                  class="inline-flex items-center gap-1.5 rounded-md px-1.5 py-1 text-xs font-bold text-af-brand hover:bg-af-brand-tint disabled:cursor-not-allowed disabled:opacity-60"
                  :disabled="!getUpstreamResponsePreview(ev)"
                  :title="getUpstreamResponsePreview(ev) ? '' : t('common.noData')"
                  @click="toggleUpstreamDetail(ev.id)"
                >
                  <Icon
                    :name="expandedUpstreamDetailIds.has(ev.id) ? 'chevronDown' : 'chevronRight'"
                    size="xs"
                    :stroke-width="2"
                  />
                  <span>
                    {{
                      expandedUpstreamDetailIds.has(ev.id)
                        ? t('admin.ops.errorDetail.responsePreview.collapse')
                        : t('admin.ops.errorDetail.responsePreview.expand')
                    }}
                  </span>
                </button>
              </div>
            </div>

            <div class="mt-3 grid grid-cols-1 gap-2 text-xs text-af-ink-2 sm:grid-cols-2">
              <div>
                <span class="text-af-ink-3">{{ t('admin.ops.errorDetail.upstreamEvent.status') }}:</span>
                <span class="ml-1 font-mono">{{ ev.status_code ?? '—' }}</span>
              </div>
              <div>
                <span class="text-af-ink-3">{{ t('admin.ops.errorDetail.upstreamEvent.requestId') }}:</span>
                <span class="ml-1 font-mono">{{ ev.request_id || ev.client_request_id || '—' }}</span>
              </div>
            </div>

            <div v-if="ev.message" class="mt-3 break-words text-sm font-medium text-af-ink">{{ ev.message }}</div>

            <pre
              v-if="expandedUpstreamDetailIds.has(ev.id)"
              class="mt-3 max-h-[240px] overflow-auto rounded-xl border border-af-hairline bg-af-sunken p-3 text-xs text-af-ink"
            ><code>{{ prettyJSON(getUpstreamResponsePreview(ev)) }}</code></pre>
          </div>
        </div>
      </div>
    </div>
    <template v-if="backToList" #footer>
      <button
        type="button"
        class="btn btn-secondary"
        data-testid="error-detail-back-to-list"
        @click="goBack"
      >
        {{ t('admin.ops.errorDetail.backToList') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import CopyButton from '@/components/common/CopyButton.vue'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { opsAPI, type OpsErrorDetail } from '@/api/admin/ops'
import { formatDateTime } from '@/utils/format'
import { resolveUpstreamPayload } from '../utils/errorDetailResponse'

interface Props {
  show: boolean
  errorId: number | null
  errorType?: 'request' | 'upstream'
  backToList?: boolean
}

interface Emits {
  (e: 'update:show', value: boolean): void
  (e: 'back'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { t } = useI18n()

const loading = ref(false)
const detail = ref<OpsErrorDetail | null>(null)

const showUpstreamList = computed(() => props.errorType === 'request')

const requestId = computed(() => detail.value?.request_id || detail.value?.client_request_id || '')

type DiagnosticPayloadKey = 'client' | 'upstream_message' | 'upstream_detail' | 'upstream_events'

const rootCauseMessage = computed(() => {
  const current = detail.value
  if (!current) return ''
  for (const candidate of [current.upstream_error_message, current.upstream_error_detail, current.message, current.error_body]) {
    const value = meaningfulPayload(candidate)
    if (value) return value
  }
  return ''
})

const diagnosticPayloadSections = computed(() => {
  const current = detail.value
  if (!current) return []
  const candidates: Array<{ key: DiagnosticPayloadKey; value: string }> = [
    { key: 'client', value: meaningfulPayload(current.error_body) },
    { key: 'upstream_message', value: meaningfulPayload(current.upstream_error_message) },
    { key: 'upstream_detail', value: meaningfulPayload(current.upstream_error_detail) },
    { key: 'upstream_events', value: meaningfulPayload(current.upstream_errors) }
  ]
  return candidates.filter((section, index, all) => {
    return section.value && all.findIndex(candidate => candidate.value === section.value) === index
  })
})

function meaningfulPayload(candidate: unknown): string {
  const value = String(candidate || '').trim()
  if (!value || value === '[]' || value === '{}' || value.toLowerCase() === 'null') return ''
  return value
}

function diagnosticPayloadLabel(key: DiagnosticPayloadKey): string {
  return t(`admin.ops.errorDetail.payloads.${key}`)
}

const emptyText = computed(() => t('admin.ops.errorDetail.noErrorSelected'))

function isUpstreamError(d: OpsErrorDetail | null): boolean {
  if (!d) return false
  const phase = String(d.phase || '').toLowerCase()
  const owner = String(d.error_owner || '').toLowerCase()
  return phase === 'upstream' && owner === 'provider'
}

function formatRequestTypeLabel(type: number | null | undefined): string {
  switch (type) {
    case 1: return t('admin.ops.errorDetail.requestTypeSync')
    case 2: return t('admin.ops.errorDetail.requestTypeStream')
    case 3: return t('admin.ops.errorDetail.requestTypeWs')
    default: return t('admin.ops.errorDetail.requestTypeUnknown')
  }
}

function hasModelMapping(d: OpsErrorDetail | null): boolean {
  if (!d) return false
  const requested = String(d.requested_model || '').trim()
  const upstream = String(d.upstream_model || '').trim()
  return !!requested && !!upstream && requested !== upstream
}

function displayModel(d: OpsErrorDetail | null): string {
  if (!d) return ''
  const upstream = String(d.upstream_model || '').trim()
  if (upstream) return upstream
  const requested = String(d.requested_model || '').trim()
  if (requested) return requested
  return String(d.model || '').trim()
}

const correlatedUpstream = ref<OpsErrorDetail[]>([])
const correlatedUpstreamLoading = ref(false)

const correlatedUpstreamErrors = computed<OpsErrorDetail[]>(() => correlatedUpstream.value)

const expandedUpstreamDetailIds = ref(new Set<number>())

function getUpstreamResponsePreview(ev: OpsErrorDetail): string {
  const upstreamPayload = resolveUpstreamPayload(ev)
  if (upstreamPayload) return upstreamPayload
  return String(ev.error_body || '').trim()
}

function toggleUpstreamDetail(id: number) {
  const next = new Set(expandedUpstreamDetailIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  expandedUpstreamDetailIds.value = next
}

async function fetchCorrelatedUpstreamErrors(requestErrorId: number) {
  correlatedUpstreamLoading.value = true
  try {
    const res = await opsAPI.listRequestErrorUpstreamErrors(
      requestErrorId,
      { page: 1, page_size: 100, view: 'all' },
      { include_detail: true }
    )
    correlatedUpstream.value = res.items || []
  } catch (err) {
    console.error('[OpsErrorDetailModal] Failed to load correlated upstream errors', err)
    correlatedUpstream.value = []
  } finally {
    correlatedUpstreamLoading.value = false
  }
}

function close() {
  emit('update:show', false)
}

function goBack() {
  emit('update:show', false)
  emit('back')
}

function prettyJSON(raw?: string): string {
  if (!raw) return 'N/A'
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

async function fetchDetail(id: number) {
  loading.value = true
  try {
    const kind = props.errorType || (detail.value?.phase === 'upstream' ? 'upstream' : 'request')
    const d = kind === 'upstream' ? await opsAPI.getUpstreamErrorDetail(id) : await opsAPI.getRequestErrorDetail(id)
    detail.value = d
  } catch (err: any) {
    detail.value = null
    console.error(err?.message || t('admin.ops.failedToLoadErrorDetail'), err)
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.show, props.errorId] as const,
  ([show, id]) => {
    if (!show) {
      detail.value = null
      return
    }
    if (typeof id === 'number' && id > 0) {
      expandedUpstreamDetailIds.value = new Set()
      fetchDetail(id)
      if (props.errorType === 'request') {
        fetchCorrelatedUpstreamErrors(id)
      } else {
        correlatedUpstream.value = []
      }
    }
  },
  { immediate: true }
)

function statusBadgeClass(code: number): string {
  if (code >= 500) return 'bg-af-danger-tint text-af-danger ring-af-danger/20'
  if (code === 429) return 'bg-af-sunken text-af-ink-2 ring-af-brand/20'
  if (code >= 400) return 'bg-af-warning-tint text-af-warning ring-af-warning/20'
  return 'bg-af-sunken text-af-ink-2 ring-af-hairline-strong/20'
}

const statusClass = computed(() => statusBadgeClass(detail.value?.status_code ?? 0))

const upstreamStatusClass = computed(() => statusBadgeClass(detail.value?.upstream_status_code ?? 0))

</script>
