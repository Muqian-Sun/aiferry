<template>
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
    <div v-if="configured.length > 0" class="space-y-2">
      <div v-for="protocol in configured" :key="protocol" class="flex items-center gap-2">
        <span class="w-44 shrink-0 text-sm text-af-ink-2">{{ protocolLabel(protocol) }}</span>
        <input
          :value="modelValue[protocol]"
          type="text"
          class="input flex-1 font-mono text-sm"
          :placeholder="t('admin.accounts.protocolEndpoints.urlPlaceholder')"
          :data-testid="`protocol-endpoint-input-${protocol}`"
          @input="onInput(protocol, $event)"
        />
        <button
          type="button"
          class="rounded-lg p-2 text-af-danger transition-colors hover:bg-af-danger-tint hover:text-af-danger"
          :aria-label="t('admin.accounts.protocolEndpoints.remove', { protocol: protocolLabel(protocol) })"
          :data-testid="`protocol-endpoint-remove-${protocol}`"
          @click="remove(protocol)"
        >
          <Icon name="trash" size="sm" />
        </button>
      </div>
    </div>
    <p v-else class="text-sm text-af-warning" data-testid="protocol-endpoints-empty">
      {{ t('admin.accounts.protocolEndpoints.empty') }}
    </p>
    <div v-if="unconfigured.length > 0" class="mt-2 flex flex-wrap gap-2">
      <button
        v-for="protocol in unconfigured"
        :key="protocol"
        type="button"
        class="rounded-lg border border-dashed border-af-hairline-strong px-3 py-1 text-xs text-af-ink-2 transition-colors hover:border-af-ink-4 hover:text-af-ink"
        :data-testid="`protocol-endpoint-add-${protocol}`"
        @click="add(protocol)"
      >
        {{ t('admin.accounts.protocolEndpoints.add', { protocol: protocolLabel(protocol) }) }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { ProtocolEndpoints, UpstreamProtocol } from '@/types'
import { sameEndpoints } from './protocolEndpoints'

const props = defineProps<{
  modelValue: ProtocolEndpoints
  /** 可配置的协议，顺序即展示顺序（取自后端 protocol-defaults 的 protocols）。 */
  protocols: readonly UpstreamProtocol[]
  /** 当前平台 / 模式的官方地址；非空且与当前值不同时提供「填入官方地址」。 */
  officialEndpoints?: ProtocolEndpoints
  /** 官方地址加载失败：提示管理员手动填写。 */
  defaultsLoadFailed?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: ProtocolEndpoints]
}>()

const { t } = useI18n()

const configured = computed<UpstreamProtocol[]>(() => {
  const present = Object.keys(props.modelValue) as UpstreamProtocol[]
  const ordered = props.protocols.filter((protocol) => present.includes(protocol))
  // 已存的协议即便不在当前可选列表里也要展示，否则会被静默丢掉。
  return [...ordered, ...present.filter((protocol) => !props.protocols.includes(protocol))]
})

const unconfigured = computed<UpstreamProtocol[]>(() =>
  props.protocols.filter((protocol) => !(protocol in props.modelValue))
)

const canRestoreOfficial = computed(() => {
  const official = props.officialEndpoints
  return !!official && Object.keys(official).length > 0 && !sameEndpoints(props.modelValue, official)
})

function restoreOfficial() {
  emit('update:modelValue', { ...props.officialEndpoints })
}

function protocolLabel(protocol: UpstreamProtocol): string {
  return t(`admin.accounts.protocolEndpoints.protocols.${protocol}`)
}

function onInput(protocol: UpstreamProtocol, event: Event) {
  emit('update:modelValue', { ...props.modelValue, [protocol]: (event.target as HTMLInputElement).value })
}

function add(protocol: UpstreamProtocol) {
  emit('update:modelValue', { ...props.modelValue, [protocol]: '' })
}

function remove(protocol: UpstreamProtocol) {
  const next = { ...props.modelValue }
  delete next[protocol]
  emit('update:modelValue', next)
}
</script>
