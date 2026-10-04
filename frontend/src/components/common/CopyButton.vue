<template>
  <!--
    复制按钮：点了要看得见结果（2026-10-04 体验诊断：十几处复制点了没有任何反馈）。
    成功时图标换成对勾 / 文字换成「已复制」，1.5 秒后复原；失败时换成「复制失败」并保持 3 秒。
    外观由调用方的 class 决定（图标小按钮或文字按钮），这里只管状态。
  -->
  <button
    type="button"
    :class="[stateClass]"
    :title="currentLabel"
    :aria-label="currentLabel"
    :data-testid="testId"
    @click.stop="copy"
  >
    <template v-if="variant === 'text'">{{ currentLabel }}</template>
    <Icon v-else :name="state === 'copied' ? 'check' : state === 'failed' ? 'x' : 'copy'" size="sm" />
  </button>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'

const props = withDefaults(
  defineProps<{
    text: string | null | undefined
    /** icon：小图标按钮；text：文字按钮（「复制」） */
    variant?: 'icon' | 'text'
    /** 平时的文字 / 提示，默认「复制」 */
    label?: string
    testId?: string
  }>(),
  { variant: 'icon', label: undefined, testId: undefined }
)

const { t } = useI18n()
const { copyToClipboard } = useClipboard()
const state = ref<'idle' | 'copied' | 'failed'>('idle')
let timer: ReturnType<typeof setTimeout> | undefined

const currentLabel = computed(() => {
  if (state.value === 'copied') return t('common.copied')
  if (state.value === 'failed') return t('common.copyFailed')
  return props.label ?? t('common.copy')
})

const stateClass = computed(() => {
  if (state.value === 'copied') return '!text-af-success'
  if (state.value === 'failed') return '!text-af-danger'
  return ''
})

async function copy() {
  if (!props.text) return
  const ok = await copyToClipboard(props.text)
  state.value = ok ? 'copied' : 'failed'
  clearTimeout(timer)
  timer = setTimeout(() => (state.value = 'idle'), ok ? 1500 : 3000)
}

onBeforeUnmount(() => clearTimeout(timer))
</script>
