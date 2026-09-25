<template>
  <!--
    第三方 key 的上游地址：一个 key 只承接一个上游协议（后端拒绝多协议），所以是「协议 + 地址」一行。
    切换协议时，地址没改过（空或仍是原协议的官方地址）就换成新协议的官方地址。
  -->
  <div data-testid="protocol-endpoints-editor">
    <div class="flex items-center justify-between gap-2">
      <label class="input-label mb-0">{{ t('admin.accounts.protocolEndpoints.title') }}</label>
      <button
        v-if="canRestoreOfficial"
        type="button"
        class="text-xs text-af-brand hover:text-af-brand-hover"
        data-testid="protocol-endpoints-restore-official"
        @click="restoreOfficial"
      >
        {{ t('admin.accounts.protocolEndpoints.restoreOfficial') }}
      </button>
    </div>
    <p class="input-hint mb-2">{{ t('admin.accounts.protocolEndpoints.hint') }}</p>
    <p
      v-if="defaultsLoadFailed"
      class="mb-2 text-sm text-af-warning"
      data-testid="protocol-defaults-load-failed"
    >
      {{ t('admin.accounts.protocolEndpoints.loadFailed') }}
    </p>
    <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
      <select
        :value="protocol ?? ''"
        class="input sm:w-52 sm:shrink-0"
        data-testid="protocol-endpoint-protocol"
        :aria-label="t('admin.accounts.protocolEndpoints.protocolLabel')"
        @change="onProtocolChange"
      >
        <option v-if="!protocol" value="" disabled>{{ t('admin.accounts.protocolEndpoints.choose') }}</option>
        <option v-for="option in protocolOptions" :key="option" :value="option">{{ protocolLabel(option) }}</option>
      </select>
      <input
        :value="protocol ? modelValue[protocol] : ''"
        type="text"
        class="input flex-1 font-mono text-sm"
        :disabled="!protocol"
        :placeholder="t('admin.accounts.protocolEndpoints.urlPlaceholder')"
        :data-testid="`protocol-endpoint-input-${protocol ?? 'none'}`"
        @input="onInput"
      />
    </div>
    <p v-if="!protocol" class="mt-2 text-sm text-af-warning" data-testid="protocol-endpoints-empty">
      {{ t('admin.accounts.protocolEndpoints.empty') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ProtocolEndpoints, UpstreamProtocol } from '@/types'

const props = defineProps<{
  modelValue: ProtocolEndpoints
  /** 可选的协议，顺序即展示顺序。 */
  protocols: readonly UpstreamProtocol[]
  /** 当前平台 / 模式各协议的官方地址；当前协议有官方地址且与当前值不同时提供「填入官方地址」。 */
  officialEndpoints?: ProtocolEndpoints
  /** 官方地址加载失败：提示管理员手动填写。 */
  defaultsLoadFailed?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: ProtocolEndpoints]
}>()

const { t } = useI18n()

const protocol = computed<UpstreamProtocol | null>(() => (Object.keys(props.modelValue)[0] as UpstreamProtocol) ?? null)

// 已存的协议即便不在当前可选列表里也要能看到，否则会被静默丢掉。
const protocolOptions = computed<UpstreamProtocol[]>(() =>
  protocol.value && !props.protocols.includes(protocol.value) ? [...props.protocols, protocol.value] : [...props.protocols]
)

const officialUrl = computed(() => (protocol.value ? props.officialEndpoints?.[protocol.value] : undefined))
const canRestoreOfficial = computed(
  () => !!protocol.value && !!officialUrl.value && props.modelValue[protocol.value] !== officialUrl.value
)

function restoreOfficial() {
  if (protocol.value && officialUrl.value) emit('update:modelValue', { [protocol.value]: officialUrl.value })
}

function protocolLabel(value: UpstreamProtocol): string {
  return t(`admin.accounts.protocolEndpoints.protocols.${value}`)
}

function onProtocolChange(event: Event) {
  const next = (event.target as HTMLSelectElement).value as UpstreamProtocol
  const current = protocol.value
  const currentUrl = current ? (props.modelValue[current] ?? '') : ''
  const untouched = !currentUrl.trim() || (!!current && currentUrl === props.officialEndpoints?.[current])
  emit('update:modelValue', { [next]: untouched ? (props.officialEndpoints?.[next] ?? '') : currentUrl })
}

function onInput(event: Event) {
  if (protocol.value) emit('update:modelValue', { [protocol.value]: (event.target as HTMLInputElement).value })
}
</script>
