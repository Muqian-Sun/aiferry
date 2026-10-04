<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="show"
        class="modal-overlay"
        :style="zIndexStyle"
        :aria-labelledby="dialogId"
        role="dialog"
        aria-modal="true"
        @click.self="handleClose"
      >
        <!-- Modal panel -->
        <div ref="dialogRef" tabindex="-1" :class="['modal-content focus:outline-none', widthClasses]" @click.stop>
          <!-- Header -->
          <div class="modal-header">
            <h3 :id="dialogId" class="modal-title">
              {{ title }}
            </h3>
            <button
              v-if="showCloseButton"
              @click="emit('close')"
              class="-mr-2 rounded-md p-2 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink-2 focus:outline-none focus-visible:ring-2 focus-visible:ring-af-brand/40 focus-visible:ring-offset-2 focus-visible:ring-offset-af-sheet"
              :aria-label="t('common.close')"
            >
              <Icon name="x" size="md" />
            </button>
          </div>

          <!-- Body -->
          <div ref="modalBodyRef" class="modal-body">
            <slot></slot>
          </div>

          <!-- Footer -->
          <div v-if="$slots.footer" class="modal-footer">
            <slot name="footer"></slot>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script lang="ts">
// 模块级计数：写在 <script setup> 里每个实例都从 0 开始，所有弹窗的标题 ID 都成了 modal-title-1
let dialogIdCounter = 0
</script>

<script setup lang="ts">
import { computed, watch, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useModalLayer } from '@/composables/useModalLayer'

const dialogId = `modal-title-${++dialogIdCounter}`

const dialogRef = ref<HTMLElement | null>(null)
const modalBodyRef = ref<HTMLElement | null>(null)

type DialogWidth = 'narrow' | 'normal' | 'wide' | 'extra-wide' | 'full'

interface Props {
  show: boolean
  title: string
  width?: DialogWidth
  closeOnEscape?: boolean
  closeOnClickOutside?: boolean
  showCloseButton?: boolean
  zIndex?: number
}

interface Emits {
  (e: 'close'): void
}

const props = withDefaults(defineProps<Props>(), {
  width: 'normal',
  closeOnEscape: true,
  closeOnClickOutside: false,
  showCloseButton: true,
  zIndex: 50
})

const emit = defineEmits<Emits>()
const { t } = useI18n()

// Custom z-index style (overrides the default z-50 from CSS)
const zIndexStyle = computed(() => {
  return props.zIndex !== 50 ? { zIndex: props.zIndex } : undefined
})

const widthClasses = computed(() => {
  // Width guidance: narrow=confirm/short prompts, normal=standard forms,
  // wide=multi-section forms or rich content, extra-wide=analytics/tables,
  // full=full-screen or very dense layouts.
  const widths: Record<DialogWidth, string> = {
    narrow: 'max-w-md',
    normal: 'max-w-lg',
    wide: 'w-full sm:max-w-2xl md:max-w-3xl lg:max-w-4xl',
    'extra-wide': 'w-full sm:max-w-3xl md:max-w-4xl lg:max-w-5xl xl:max-w-6xl',
    full: 'w-full sm:max-w-4xl md:max-w-5xl lg:max-w-6xl xl:max-w-7xl'
  }
  return widths[props.width]
})

const handleClose = () => {
  if (props.closeOnClickOutside) {
    emit('close')
  }
}

// 叠层、Esc、滚动锁、焦点陷阱与归还交给弹层栈（useModalLayer）
useModalLayer({
  open: () => props.show,
  panel: () => dialogRef.value,
  onEscape: () => {
    if (props.closeOnEscape) emit('close')
  },
  // 先聚焦标了 data-autofocus 的元素，再是正文里第一个输入框；都没有就聚焦面板本身（不落到右上角的 ✕）
  initialFocus: () =>
    dialogRef.value?.querySelector<HTMLElement>('[data-autofocus]') ??
    modalBodyRef.value?.querySelector<HTMLElement>(
      'input:not([type="hidden"]):not([disabled]):not([readonly]), select:not([disabled]), textarea:not([disabled]):not([readonly])'
    ) ??
    null
})

// 每次打开都从正文顶部开始
watch(
  () => props.show,
  (isOpen) => {
    if (isOpen) requestAnimationFrame(() => modalBodyRef.value?.scrollTo({ top: 0 }))
  }
)
</script>
