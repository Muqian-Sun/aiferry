<template>
  <!--
    新建渠道第一步：接入方式（第三方 key / 成品号）。v-model 是来源 ID，接入方式由它推出。
    第三方 key 不选平台、不选来源（厂商按地址识别）；成品号要选是哪家的账号（授权流程各家不同）。
  -->
  <div class="space-y-4" data-testid="access-source-picker">
    <div>
      <label class="input-label">{{ t('admin.accounts.accessSource.kindLabel') }}</label>
      <div
        class="mt-2 grid grid-cols-1 gap-3 sm:grid-cols-2"
        role="radiogroup"
        :aria-label="t('admin.accounts.accessSource.kindLabel')"
      >
        <button
          v-for="option in kindOptions"
          :key="option.kind"
          type="button"
          role="radio"
          :aria-checked="currentKind === option.kind"
          :data-testid="`access-kind-${option.kind}`"
          :class="[
            'flex items-start gap-3 rounded-lg border p-3 text-left transition-colors',
            currentKind === option.kind
              ? 'border-af-brand bg-af-brand-tint'
              : 'border-af-hairline hover:border-af-hairline-strong'
          ]"
          @click="selectKind(option.kind)"
        >
          <span
            :class="[
              'flex h-8 w-8 shrink-0 items-center justify-center rounded-md',
              currentKind === option.kind ? 'bg-af-ink text-af-on-brand' : 'bg-af-sunken text-af-ink-3'
            ]"
          >
            <Icon :name="option.icon" size="sm" />
          </span>
          <span class="min-w-0">
            <span class="block text-sm font-medium text-af-ink">{{ t(`admin.accounts.accessSource.kinds.${option.kind}.title`) }}</span>
            <span class="block text-xs text-af-ink-3">{{ t(`admin.accounts.accessSource.kinds.${option.kind}.description`) }}</span>
          </span>
        </button>
      </div>
    </div>

    <div v-if="currentKind === 'subscription'">
      <label class="input-label">{{ t('admin.accounts.accessSource.accountLabel') }}</label>
      <div
        class="mt-2 grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4"
        role="radiogroup"
        :aria-label="t('admin.accounts.accessSource.accountLabel')"
      >
        <button
          v-for="source in sources"
          :key="source.id"
          type="button"
          role="radio"
          :aria-checked="modelValue === source.id"
          :data-testid="`access-source-${source.id}`"
          :class="[
            'flex min-w-0 items-center gap-2.5 rounded-lg border px-3 py-2 text-left transition-colors',
            modelValue === source.id
              ? 'border-af-brand bg-af-brand-tint'
              : 'border-af-hairline hover:border-af-hairline-strong'
          ]"
          @click="emit('update:modelValue', source.id)"
        >
          <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-af-sunken text-af-ink-2">
            <PlatformIcon :platform="source.icon" size="md" />
          </span>
          <span class="min-w-0">
            <span class="block truncate text-sm font-medium text-af-ink">{{ source.name }}</span>
            <span class="block truncate text-xs text-af-ink-3" :title="t(source.hintKey)">{{ t(source.hintKey) }}</span>
          </span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { accessSourcesOf, findAccessSource, type AccessKind } from './accessSources'

const props = defineProps<{
  /** 来源 ID（见 accessSources.ts）。 */
  modelValue: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const { t } = useI18n()

const kindOptions: { kind: AccessKind; icon: 'key' | 'user' }[] = [
  { kind: 'key', icon: 'key' },
  { kind: 'subscription', icon: 'user' }
]

const currentKind = computed(() => findAccessSource(props.modelValue).kind)
const sources = computed(() => accessSourcesOf(currentKind.value))

function selectKind(kind: AccessKind) {
  if (kind === currentKind.value) return
  emit('update:modelValue', accessSourcesOf(kind)[0].id)
}
</script>
