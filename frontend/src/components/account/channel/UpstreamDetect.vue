<template>
  <!--
    「检测上游」（muqian 2026-10-03，原来的「探测协议」+「探测模型」合成一个按钮）：
    1. 用表单里的地址与 key 逐个试四个上游协议（支持 / 不支持 / 不确定），在结果前面选一个，协议与地址交给父组件填进表单；
       还没选协议时自动选第一个「支持」的；
    2. 再用选定的协议向上游要模型名单，对照模型目录分成「已上架 / 未上架 / 目录没有」，交给父组件（新建时预填承接行）。
    结果与出错都在这里就地显示；地址或 key 改了旧结果就作废。
  -->
  <div class="rounded-lg border border-af-hairline px-3 py-2.5" data-testid="upstream-detect">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <p class="text-xs text-af-ink-3">{{ t('admin.accounts.upstreamDetect.hint') }}</p>
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        :disabled="!canDetect || running"
        data-testid="upstream-detect-run"
        @click="detect"
      >
        {{ running ? t('admin.accounts.upstreamDetect.running') : t('admin.accounts.upstreamDetect.run') }}
      </button>
    </div>

    <p v-if="protocolError" class="mt-2 text-xs text-af-danger" role="alert" data-testid="upstream-detect-error">{{ protocolError }}</p>

    <div v-else-if="protocols" class="mt-2 space-y-1" role="radiogroup" data-testid="upstream-protocol-probe-result">
      <!-- 一句结论：能用哪些；一个都没有就提示看每一项的原因（2026-10-06 生产四个都是不确定，管理员看不出为什么） -->
      <p class="px-1.5 pb-1 text-13 font-medium" :class="usable.length > 0 ? 'text-af-success' : 'text-af-warning'" data-testid="upstream-protocol-probe-conclusion">
        {{ conclusion }}
      </p>
      <label
        v-for="result in protocols"
        :key="result.protocol"
        class="flex items-start gap-2 rounded-md px-1.5 py-1 text-xs"
        :class="selectable(result) ? 'cursor-pointer hover:bg-af-sunken' : 'cursor-not-allowed opacity-60'"
        :data-testid="`upstream-protocol-probe-${result.protocol}`"
      >
        <input
          type="radio"
          name="upstream-detect-protocol"
          class="mt-0.5"
          :value="result.protocol"
          :checked="isCurrent(result)"
          :disabled="!selectable(result) || running"
          @change="pickProtocol(result)"
        />
        <span class="w-36 flex-shrink-0 font-medium text-af-ink">{{ t(`admin.accounts.protocolEndpoints.protocols.${result.protocol}`) }}</span>
        <span class="w-12 flex-shrink-0" :class="STATUS_CLASS[result.status]">{{ t(`admin.accounts.protocolProbe.status.${result.status}`) }}</span>
        <span class="min-w-0 flex-1 text-af-ink-3">
          {{ describe(result) }}
          <span v-if="result.detail" class="mt-0.5 block break-words text-af-ink-2" :data-testid="`upstream-protocol-probe-detail-${result.protocol}`">
            {{ t('admin.accounts.protocolProbe.upstreamSaid', { detail: result.detail }) }}
          </span>
        </span>
      </label>
      <p v-if="protocols.some((r) => r.model)" class="pt-1 text-xs text-af-ink-3">{{ t('admin.accounts.protocolProbe.costNote') }}</p>
    </div>

    <div v-if="modelsState !== 'idle'" class="mt-2 space-y-1 border-t border-af-hairline pt-2 text-xs" data-testid="upstream-detect-models">
      <p v-if="modelsState === 'loading'" class="text-af-ink-3">{{ t('admin.accounts.upstreamDetect.modelsLoading') }}</p>
      <p v-else-if="modelsError" class="text-af-danger" role="alert">{{ modelsError }}</p>
      <template v-else-if="classified">
        <p class="text-af-ink-2" data-testid="upstream-detect-models-summary">{{ summary }}</p>
        <p v-if="classified.listed.length > 0" class="break-words font-mono text-af-ink-3">
          {{ classified.listed.map((match) => match.entry.model_id).join(', ') }}
        </p>
        <p class="text-af-ink-3">{{ t('admin.accounts.upstreamDetect.referenceOnly') }}</p>
        <slot name="models" :classified="classified" />
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { ProbedUpstreamProtocol, ProtocolProbeStatus } from '@/api/admin/accounts'
import type { ProtocolEndpoints, UpstreamProtocol } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'
import { classifyUpstreamModels, type DetectedModels } from './upstreamModels'

const props = defineProps<{
  /** 表单里的协议地址：拿其中填了的那一个地址去试 */
  protocolEndpoints: ProtocolEndpoints
  /** 还没选协议时地址栏里填的地址 */
  draftUrl?: string
  /** 新填的 key；编辑已有渠道不改 key 时为空，用 accountId 取存着的 */
  apiKey?: string
  accountId?: number
  proxyId?: number | null
}>()

const emit = defineEmits<{
  /** 选中一个协议：父组件把表单的协议地址换成 { [protocol]: url } */
  select: [protocol: UpstreamProtocol, url: string]
  /** 模型名单对照目录的结果；检测失败或旧结果作废时为 null */
  models: [result: { names: string[]; classified: DetectedModels } | null]
}>()

const { t } = useI18n()

const STATUS_CLASS: Record<ProtocolProbeStatus, string> = {
  supported: 'text-af-success',
  unsupported: 'text-af-ink-3',
  unknown: 'text-af-warning'
}

const running = ref(false)
const protocolError = ref('')
const protocols = ref<ProbedUpstreamProtocol[] | null>(null)
const modelsState = ref<'idle' | 'loading' | 'done'>('idle')
const modelsError = ref('')
const classified = ref<DetectedModels | null>(null)
const modelCount = ref(0)

const currentEntry = computed(() => {
  for (const [protocol, url] of Object.entries(props.protocolEndpoints ?? {})) {
    if (url?.trim()) return { protocol: protocol as UpstreamProtocol, url: url.trim() }
  }
  return null
})
// 拿去试的地址：选了协议用协议地址，没选用草稿
const probeBase = computed(() => currentEntry.value?.url ?? props.draftUrl?.trim() ?? '')
const canDetect = computed(() => !!probeBase.value && (!!props.apiKey?.trim() || !!props.accountId))

function credentials() {
  const apiKey = props.apiKey?.trim()
  return {
    api_key: apiKey || undefined,
    account_id: apiKey ? undefined : props.accountId,
    proxy_id: props.proxyId ?? undefined
  }
}

function resetModels() {
  modelsState.value = 'idle'
  modelsError.value = ''
  classified.value = null
  modelCount.value = 0
}

// 地址改成结果以外的、或 key 改了，旧结果就不作数；在结果里选协议引起的地址变化不清
watch(
  () => [probeBase.value, props.apiKey] as const,
  ([url, apiKey], [, previousApiKey]) => {
    if (!protocols.value && modelsState.value === 'idle') return
    if (apiKey !== previousApiKey || !(protocols.value ?? []).some((r) => r.base_url === url)) {
      protocols.value = null
      protocolError.value = ''
      resetModels()
      emit('models', null)
    }
  }
)

function selectable(result: ProbedUpstreamProtocol): boolean {
  return result.status !== 'unsupported'
}

function isCurrent(result: ProbedUpstreamProtocol): boolean {
  return currentEntry.value?.protocol === result.protocol && currentEntry.value.url === result.base_url
}

function describe(result: ProbedUpstreamProtocol): string {
  const parts = [t(`admin.accounts.protocolProbe.reason.${result.reason}`)]
  if (result.http_status) parts.push(`HTTP ${result.http_status}`)
  if (result.model) parts.push(t('admin.accounts.protocolProbe.viaModel', { model: result.model }))
  return parts.join(' · ')
}

const usable = computed(() => (protocols.value ?? []).filter((result) => result.status === 'supported'))
const conclusion = computed(() =>
  usable.value.length > 0
    ? t('admin.accounts.protocolProbe.conclusionUsable', {
        protocols: usable.value
          .map((result) => t(`admin.accounts.protocolEndpoints.protocols.${result.protocol}`))
          .join(t('admin.accounts.protocolProbe.listSeparator'))
      })
    : t('admin.accounts.protocolProbe.conclusionNone')
)

const summary = computed(() => {
  const result = classified.value
  if (!result) return ''
  return t('admin.accounts.upstreamDetect.summary', {
    total: modelCount.value,
    listed: result.listed.length,
    unlisted: result.unlisted.length,
    missing: result.missing.length
  })
})

async function probeModels(endpoints: ProtocolEndpoints) {
  modelsState.value = 'loading'
  modelsError.value = ''
  classified.value = null
  try {
    const [{ models }, catalog] = await Promise.all([
      adminAPI.accounts.probeUpstreamModels({ protocol_endpoints: endpoints, ...credentials() }),
      adminAPI.modelCatalog.listEntries()
    ])
    modelCount.value = models.length
    classified.value = classifyUpstreamModels(models, catalog)
    emit('models', { names: models, classified: classified.value })
  } catch (err) {
    // 502 = 上游那边的问题（连不上 / 出错），后端回的是英文原句，换成中文说明
    modelsError.value = (err as { status?: number })?.status === 502
      ? t('admin.accounts.upstreamDetect.modelsUnreachable')
      : extractApiErrorMessage(err, t('admin.accounts.upstreamDetect.modelsFailed'))
    emit('models', null)
  } finally {
    modelsState.value = 'done'
  }
}

async function detect() {
  const base = probeBase.value
  if (!base) return
  running.value = true
  protocolError.value = ''
  protocols.value = null
  resetModels()
  emit('models', null)
  try {
    try {
      protocols.value = (await adminAPI.accounts.probeUpstreamProtocols({ base_url: base, ...credentials() })).protocols
    } catch (err) {
      protocolError.value = extractApiErrorMessage(err, t('admin.accounts.upstreamDetect.protocolFailed'))
      return
    }
    let endpoints: ProtocolEndpoints | null = currentEntry.value ? { [currentEntry.value.protocol]: currentEntry.value.url } : null
    // 还没选协议：自动选第一个「支持」的（顺序与协议下拉一致），不支持 / 拿不准的留给管理员在上面选
    if (!endpoints) {
      const pick = protocols.value.find((result) => result.status === 'supported')
      if (pick) {
        emit('select', pick.protocol, pick.base_url)
        endpoints = { [pick.protocol]: pick.base_url }
      }
    }
    if (endpoints) await probeModels(endpoints)
  } finally {
    running.value = false
  }
}

// 在结果里换了协议：填回表单，模型名单按新协议重查
async function pickProtocol(result: ProbedUpstreamProtocol) {
  emit('select', result.protocol, result.base_url)
  running.value = true
  try {
    await probeModels({ [result.protocol]: result.base_url })
  } finally {
    running.value = false
  }
}
</script>
