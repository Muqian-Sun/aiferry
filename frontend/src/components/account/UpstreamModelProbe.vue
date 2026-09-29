<template>
  <!--
    「探测模型」（muqian 2026-09-29）：用表单里填好的协议地址与 key 向上游要模型名单，对到模型目录——
    对得上的交给父组件勾成承接模型（按上游支持的重新勾选），目录里没有的可以一键导入（未上架）再勾上。
    结果与出错都在这里就地显示（两站已不用右上角提示条）。
  -->
  <div class="rounded-lg border border-af-hairline px-3 py-2.5" data-testid="upstream-model-probe">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <p class="text-xs text-af-ink-3">{{ t('admin.accounts.probe.hint') }}</p>
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        :disabled="!canProbe || probing"
        data-testid="upstream-model-probe-run"
        @click="probe"
      >
        {{ probing ? t('admin.accounts.probe.running') : t('admin.accounts.probe.run') }}
      </button>
    </div>

    <p v-if="error" class="mt-2 text-xs text-af-danger" role="alert" data-testid="upstream-model-probe-error">{{ error }}</p>

    <div v-else-if="result" class="mt-2 space-y-1.5 text-xs text-af-ink-2" data-testid="upstream-model-probe-result">
      <p>{{ t('admin.accounts.probe.summary', { total: result.length, matched: matchedIds.length }) }}</p>
      <template v-if="unmatched.length">
        <p>
          {{ t('admin.accounts.probe.unmatched', { count: unmatched.length }) }}
          <span class="font-mono text-af-ink-3">{{ unmatched.slice(0, UNMATCHED_PREVIEW).join('、') }}</span>
          <template v-if="unmatched.length > UNMATCHED_PREVIEW">{{ t('admin.accounts.probe.andMore', { count: unmatched.length - UNMATCHED_PREVIEW }) }}</template>
        </p>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="importing"
          data-testid="upstream-model-probe-import"
          @click="importUnmatched"
        >
          {{ importing ? t('admin.accounts.probe.importing') : t('admin.accounts.probe.import', { count: unmatched.length }) }}
        </button>
      </template>
      <p v-if="importedCount > 0" class="text-af-ink-3">{{ t('admin.accounts.probe.imported', { count: importedCount }) }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { ProbedUpstreamModel } from '@/api/admin/modelCatalog'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{
  /** 表单里的协议地址 */
  protocolEndpoints: Record<string, string>
  /** 新填的 key；编辑已有渠道不改 key 时为空，用 accountId 取存着的 */
  apiKey?: string
  accountId?: number
  proxyId?: number | null
}>()

const emit = defineEmits<{
  /** 目录里对得上的条目：父组件按它重新勾选承接模型 */
  matched: [entryIds: number[]]
  /** 一键导入后的条目（新建的与已有的）：父组件先刷新目录再勾上 */
  imported: [entryIds: number[]]
}>()

const { t } = useI18n()
const UNMATCHED_PREVIEW = 8

const probing = ref(false)
const importing = ref(false)
const error = ref('')
const result = ref<ProbedUpstreamModel[] | null>(null)
const importedCount = ref(0)

const trimmedEndpoints = computed(() => {
  const out: Record<string, string> = {}
  for (const [protocol, url] of Object.entries(props.protocolEndpoints ?? {})) {
    if (url?.trim()) out[protocol] = url.trim()
  }
  return out
})
const canProbe = computed(
  () => Object.keys(trimmedEndpoints.value).length > 0 && (!!props.apiKey?.trim() || !!props.accountId)
)
const matchedIds = computed(() => [...new Set((result.value ?? []).flatMap((m) => (m.entry_id ? [m.entry_id] : [])))])
const unmatched = computed(() => (result.value ?? []).filter((m) => !m.entry_id).map((m) => m.id))

// 地址或 key 改了，旧结果就不作数
watch(
  () => [trimmedEndpoints.value, props.apiKey] as const,
  () => {
    result.value = null
    error.value = ''
    importedCount.value = 0
  },
  { deep: true }
)

async function probe() {
  probing.value = true
  error.value = ''
  result.value = null
  importedCount.value = 0
  try {
    const { models } = await adminAPI.accounts.probeUpstreamModels({
      protocol_endpoints: trimmedEndpoints.value,
      api_key: props.apiKey?.trim() || undefined,
      account_id: props.apiKey?.trim() ? undefined : props.accountId,
      proxy_id: props.proxyId ?? undefined
    })
    if (models.length === 0) {
      error.value = t('admin.accounts.probe.empty')
      return
    }
    result.value = await adminAPI.modelCatalog.matchUpstreamModels(models)
    emit('matched', matchedIds.value)
  } catch (err) {
    error.value = extractApiErrorMessage(err, t('admin.accounts.probe.failed'))
  } finally {
    probing.value = false
  }
}

async function importUnmatched() {
  importing.value = true
  error.value = ''
  try {
    const entries = await adminAPI.modelCatalog.importUpstreamModels(unmatched.value)
    const ids = entries.map((entry) => entry.id)
    importedCount.value = ids.length
    // 导入后这些模型都有条目了，结果里补上，列表不再显示「目录里没有」
    const byModel = new Map(entries.map((entry) => [entry.model_id, entry.id]))
    result.value = (result.value ?? []).map((m) => (m.entry_id ? m : { ...m, entry_id: byModel.get(m.id) }))
    emit('imported', [...new Set([...matchedIds.value, ...ids])])
  } catch (err) {
    error.value = extractApiErrorMessage(err, t('admin.accounts.probe.importFailed'))
  } finally {
    importing.value = false
  }
}
</script>
