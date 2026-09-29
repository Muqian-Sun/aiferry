<template>
  <!--
    「探测协议」（muqian 2026-09-29）：用表单里填的地址与 key 逐个试四个上游协议，列出支持 / 不支持 / 不确定；
    还没选协议时用地址草稿（draftUrl）试，不必先随便选一个协议；
    结果旁边的选择框选一个（一个 key 只承接一个协议），协议与地址交给父组件填进表单。结果与出错都在这里就地显示。
  -->
  <div class="rounded-lg border border-af-hairline px-3 py-2.5" data-testid="upstream-protocol-probe">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <p class="text-xs text-af-ink-3">{{ t('admin.accounts.protocolProbe.hint') }}</p>
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        :disabled="!canProbe || probing"
        data-testid="upstream-protocol-probe-run"
        @click="probe"
      >
        {{ probing ? t('admin.accounts.protocolProbe.running') : t('admin.accounts.protocolProbe.run') }}
      </button>
    </div>

    <p v-if="error" class="mt-2 text-xs text-af-danger" role="alert" data-testid="upstream-protocol-probe-error">{{ error }}</p>

    <div v-else-if="results" class="mt-2 space-y-1" role="radiogroup" data-testid="upstream-protocol-probe-result">
      <label
        v-for="result in results"
        :key="result.protocol"
        class="flex items-start gap-2 rounded-md px-1.5 py-1 text-xs"
        :class="selectable(result) ? 'cursor-pointer hover:bg-af-sunken' : 'cursor-not-allowed opacity-60'"
        :data-testid="`upstream-protocol-probe-${result.protocol}`"
      >
        <input
          type="radio"
          name="upstream-protocol-probe"
          class="mt-0.5"
          :value="result.protocol"
          :checked="isCurrent(result)"
          :disabled="!selectable(result)"
          @change="emit('select', result.protocol, result.base_url)"
        />
        <span class="w-36 flex-shrink-0 font-medium text-af-ink">{{ t(`admin.accounts.protocolEndpoints.protocols.${result.protocol}`) }}</span>
        <span class="w-12 flex-shrink-0" :class="STATUS_CLASS[result.status]">{{ t(`admin.accounts.protocolProbe.status.${result.status}`) }}</span>
        <span class="min-w-0 flex-1 text-af-ink-3">{{ describe(result) }}</span>
      </label>
      <p v-if="results.some((r) => r.model)" class="pt-1 text-xs text-af-ink-4">{{ t('admin.accounts.protocolProbe.costNote') }}</p>
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
}>()

const { t } = useI18n()

const STATUS_CLASS: Record<ProtocolProbeStatus, string> = {
  supported: 'text-af-success',
  unsupported: 'text-af-ink-4',
  unknown: 'text-af-warning'
}

const probing = ref(false)
const error = ref('')
const results = ref<ProbedUpstreamProtocol[] | null>(null)

const currentEntry = computed(() => {
  for (const [protocol, url] of Object.entries(props.protocolEndpoints ?? {})) {
    if (url?.trim()) return { protocol: protocol as UpstreamProtocol, url: url.trim() }
  }
  return null
})
// 拿去试的地址：选了协议用协议地址，没选用草稿
const probeBase = computed(() => currentEntry.value?.url ?? props.draftUrl?.trim() ?? '')
const canProbe = computed(() => !!probeBase.value && (!!props.apiKey?.trim() || !!props.accountId))

// 地址改成结果以外的、或 key 改了，旧结果就不作数；在结果里选协议引起的地址变化不清
watch(
  () => [probeBase.value, props.apiKey] as const,
  ([url, apiKey], [, previousApiKey]) => {
    if (!results.value) return
    if (apiKey !== previousApiKey || !results.value.some((r) => r.base_url === url)) {
      results.value = null
      error.value = ''
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

async function probe() {
  const base = probeBase.value
  if (!base) return
  probing.value = true
  error.value = ''
  results.value = null
  try {
    const { protocols } = await adminAPI.accounts.probeUpstreamProtocols({
      base_url: base,
      api_key: props.apiKey?.trim() || undefined,
      account_id: props.apiKey?.trim() ? undefined : props.accountId,
      proxy_id: props.proxyId ?? undefined
    })
    results.value = protocols
  } catch (err) {
    error.value = extractApiErrorMessage(err, t('admin.accounts.protocolProbe.failed'))
  } finally {
    probing.value = false
  }
}
</script>
