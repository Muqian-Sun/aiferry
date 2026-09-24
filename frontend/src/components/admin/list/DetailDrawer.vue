<template>
  <!--
    详情抽屉（A5）：渠道、用户、模型点行打开，看完即关，不离开列表；要改配置再点「编辑」。
    层级在对话框之下（z-40 < BaseDialog z-50），抽屉里再弹的测试 / 确认对话框盖在它上面。
    Esc、点遮罩、点 ✕ 都关；上面还开着对话框时 Esc 只关对话框。
  -->
  <Teleport to="body">
    <div v-if="show" class="fixed inset-0 z-40" data-testid="detail-drawer">
      <div class="drawer-overlay absolute inset-0 bg-af-ink/20" aria-hidden="true" @click="emit('close')"></div>
      <section
        ref="panelEl"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="titleId"
        tabindex="-1"
        :class="[
          'drawer-panel absolute inset-y-0 right-0 flex w-full flex-col border-l border-af-hairline bg-af-sheet shadow-xl focus:outline-none',
          widthClass
        ]"
      >
        <header class="shrink-0 px-6 pt-5">
          <div class="flex items-start justify-between gap-4">
            <div class="min-w-0">
              <p v-if="eyebrow" class="truncate text-xs text-af-ink-3">{{ eyebrow }}</p>
              <h2 :id="titleId" class="mt-0.5 truncate text-lg font-semibold text-af-ink">{{ title }}</h2>
              <p v-if="subtitle" class="mt-0.5 truncate text-13 text-af-ink-3">{{ subtitle }}</p>
            </div>
            <div class="flex shrink-0 items-center gap-2">
              <slot name="actions" />
              <button
                type="button"
                class="-mr-2 rounded-md p-2 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink"
                :aria-label="t('common.close')"
                data-testid="detail-drawer-close"
                @click="emit('close')"
              >
                <Icon name="x" size="md" />
              </button>
            </div>
          </div>
          <div v-if="$slots.banner" class="mt-4"><slot name="banner" /></div>
          <SectionTabs
            v-if="tabs?.length"
            class="-mx-3 mt-3"
            :tabs="tabs"
            :model-value="tab"
            :label="title"
            @update:model-value="emit('update:tab', $event)"
          />
          <div v-else class="mt-5 border-b border-af-hairline"></div>
        </header>
        <div class="min-h-0 flex-1 overflow-y-auto px-6 py-5">
          <StatusState v-if="loading" kind="loading" :title="t('common.loading')" />
          <slot v-else />
        </div>
      </section>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import SectionTabs from '@/components/user/shell/SectionTabs.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import type { SectionTab } from '@/components/user/shell/types'

let counter = 0

const props = withDefaults(
  defineProps<{
    show: boolean
    title: string
    eyebrow?: string
    subtitle?: string
    tabs?: SectionTab[]
    tab?: string
    loading?: boolean
    width?: 'md' | 'lg'
  }>(),
  { eyebrow: '', subtitle: '', tabs: undefined, tab: undefined, loading: false, width: 'md' }
)

const emit = defineEmits<{ (e: 'close'): void; (e: 'update:tab', key: string): void }>()

const { t } = useI18n()
const titleId = `detail-drawer-title-${++counter}`
const panelEl = ref<HTMLElement | null>(null)
const widthClass = computed(() => (props.width === 'lg' ? 'sm:max-w-[720px]' : 'sm:max-w-[560px]'))

let previousFocus: HTMLElement | null = null

function onKey(event: KeyboardEvent) {
  if (event.key !== 'Escape') return
  // 抽屉上面开着对话框（BaseDialog 给 body 加 modal-open）时，Esc 归对话框
  if (document.body.classList.contains('modal-open')) return
  emit('close')
}

watch(
  () => props.show,
  async (open) => {
    if (open) {
      previousFocus = document.activeElement as HTMLElement | null
      document.addEventListener('keydown', onKey)
      await nextTick()
      panelEl.value?.focus()
    } else {
      document.removeEventListener('keydown', onKey)
      previousFocus?.focus?.()
      previousFocus = null
    }
  },
  { immediate: true }
)

onBeforeUnmount(() => document.removeEventListener('keydown', onKey))
</script>

<style scoped>
/* 只做进入动画；关闭立即收起，免得和下一次打开抢状态 */
.drawer-panel {
  animation: drawer-in 180ms ease-out;
}
.drawer-overlay {
  animation: drawer-fade 180ms ease-out;
}
@keyframes drawer-in {
  from {
    transform: translateX(24px);
    opacity: 0;
  }
}
@keyframes drawer-fade {
  from {
    opacity: 0;
  }
}
@media (prefers-reduced-motion: reduce) {
  .drawer-panel,
  .drawer-overlay {
    animation: none;
  }
}
</style>
