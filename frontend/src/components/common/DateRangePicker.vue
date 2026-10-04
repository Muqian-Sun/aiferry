<template>
  <div class="relative" ref="containerRef">
    <button
      type="button"
      @click="toggle"
      :class="['date-picker-trigger', isOpen && 'date-picker-trigger-open', plain && 'date-picker-trigger-plain']"
    >
      <span class="date-picker-icon">
        <Icon name="calendar" size="sm" />
      </span>
      <span class="date-picker-value">
        {{ displayValue }}
      </span>
      <span class="date-picker-chevron">
        <Icon
          name="chevronDown"
          size="sm"
          :class="['transition-transform duration-200', isOpen && 'rotate-180']"
        />
      </span>
    </button>

    <Transition name="date-picker-dropdown">
      <div v-if="isOpen" ref="dropdownRef" :class="['date-picker-dropdown', alignEnd && 'date-picker-dropdown-end']">
        <!-- Quick presets -->
        <div class="date-picker-presets">
          <button
            v-for="preset in presets"
            :key="preset.value"
            @click="selectPreset(preset)"
            :class="['date-picker-preset', isPresetActive(preset) && 'date-picker-preset-active']"
          >
            {{ t(preset.labelKey) }}
          </button>
        </div>

        <div class="date-picker-divider"></div>

        <!-- Custom date range inputs -->
        <div class="date-picker-custom">
          <div class="date-picker-field">
            <label class="date-picker-label">{{ t('dates.startDate') }}</label>
            <input
              type="date"
              v-model="localStartDate"
              :max="localEndDate || tomorrow"
              class="date-picker-input"
              :aria-invalid="rangeReversed"
              @change="onDateInput"
            />
          </div>
          <div class="date-picker-separator">
            <Icon name="arrowRight" size="sm" class="text-af-ink-3" />
          </div>
          <div class="date-picker-field">
            <label class="date-picker-label">{{ t('dates.endDate') }}</label>
            <input
              type="date"
              v-model="localEndDate"
              :min="localStartDate"
              :max="tomorrow"
              class="date-picker-input"
              :aria-invalid="rangeReversed"
              @change="onDateInput"
            />
          </div>
        </div>

        <!-- 起止颠倒：日期框的 min / max 拦不住手输，应用前再查一次，不让一个必然为空的范围发出去 -->
        <p v-if="rangeReversed" class="date-picker-error" role="alert" data-testid="date-range-reversed">
          {{ t('dates.rangeReversed') }}
        </p>

        <!-- Apply button -->
        <div class="date-picker-actions">
          <button @click="apply" class="date-picker-apply" :disabled="rangeReversed">
            {{ t('dates.apply') }}
          </button>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { IS_ADMIN_SITE } from '@/app/site'
import { LAST_24_HOURS_PRESET, isReversedDateRange } from '@/utils/dateRange'

interface DatePreset {
  labelKey: string
  value: string
  getRange: () => { start: string; end: string }
}

interface Props {
  startDate: string
  endDate: string
  /**
   * 当前生效的预设，由页面给：「近 24 小时」按精确时刻查，它的两个日期和自定义的「昨天 → 今天」长得一样，
   * 不能靠日期反推（反推会把自定义的两个自然日认成近 24 小时）。给 null 时按日期认按天的预设，认不出就显示日期。
   */
  preset: string | null
}

interface Emits {
  (e: 'update:startDate', value: string): void
  (e: 'update:endDate', value: string): void
  (e: 'change', range: { startDate: string; endDate: string; preset: string | null }): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { t, locale } = useI18n()

/** 用户站触发器不画框（见样式区 .date-picker-trigger-plain） */
const plain = !IS_ADMIN_SITE

const isOpen = ref(false)
const containerRef = ref<HTMLElement | null>(null)
const dropdownRef = ref<HTMLElement | null>(null)

/** 面板默认与触发器左对齐；触发器靠右（如放在页头右侧）时放不下，就改成右对齐，免得溢出视口撑出横向滚动条 */
const VIEWPORT_MARGIN = 8
const alignEnd = ref(false)
watch(isOpen, async (open) => {
  if (!open) return
  alignEnd.value = false
  await nextTick()
  const panel = dropdownRef.value
  if (panel) alignEnd.value = panel.getBoundingClientRect().right > window.innerWidth - VIEWPORT_MARGIN
})
const localStartDate = ref(props.startDate)
const localEndDate = ref(props.endDate)
const activePreset = ref<string | null>(null)
const rangeReversed = computed(() => isReversedDateRange(localStartDate.value, localEndDate.value))

const today = computed(() => {
  // Use local timezone to avoid UTC timezone issues
  const now = new Date()
  const year = now.getFullYear()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
})

// Tomorrow's date - used for max date to handle timezone differences
// When user is in a timezone behind the server, "today" on server might be "tomorrow" locally
const tomorrow = computed(() => {
  const d = new Date()
  d.setDate(d.getDate() + 1)
  return formatDateToString(d)
})

// Helper function to format date to YYYY-MM-DD using local timezone
const formatDateToString = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const presets: DatePreset[] = [
  {
    labelKey: 'dates.today',
    value: 'today',
    getRange: () => {
      const t = today.value
      return { start: t, end: t }
    }
  },
  {
    labelKey: 'dates.yesterday',
    value: 'yesterday',
    getRange: () => {
      const d = new Date()
      d.setDate(d.getDate() - 1)
      const yesterday = formatDateToString(d)
      return { start: yesterday, end: yesterday }
    }
  },
  {
    labelKey: 'dates.last24Hours',
    value: LAST_24_HOURS_PRESET,
    getRange: () => {
      const end = new Date()
      const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
      return {
        start: formatDateToString(start),
        end: formatDateToString(end)
      }
    }
  },
  {
    labelKey: 'dates.last7Days',
    value: '7days',
    getRange: () => {
      const end = today.value
      const d = new Date()
      d.setDate(d.getDate() - 6)
      const start = formatDateToString(d)
      return { start, end }
    }
  },
  {
    labelKey: 'dates.last14Days',
    value: '14days',
    getRange: () => {
      const end = today.value
      const d = new Date()
      d.setDate(d.getDate() - 13)
      const start = formatDateToString(d)
      return { start, end }
    }
  },
  {
    labelKey: 'dates.last30Days',
    value: '30days',
    getRange: () => {
      const end = today.value
      const d = new Date()
      d.setDate(d.getDate() - 29)
      const start = formatDateToString(d)
      return { start, end }
    }
  },
  {
    labelKey: 'dates.thisMonth',
    value: 'thisMonth',
    getRange: () => {
      const now = new Date()
      const start = formatDateToString(new Date(now.getFullYear(), now.getMonth(), 1))
      return { start, end: today.value }
    }
  },
  {
    labelKey: 'dates.lastMonth',
    value: 'lastMonth',
    getRange: () => {
      const now = new Date()
      const start = formatDateToString(new Date(now.getFullYear(), now.getMonth() - 1, 1))
      const end = formatDateToString(new Date(now.getFullYear(), now.getMonth(), 0))
      return { start, end }
    }
  }
]

const displayValue = computed(() => {
  if (activePreset.value) {
    const preset = presets.find((p) => p.value === activePreset.value)
    if (preset) return t(preset.labelKey)
  }

  if (localStartDate.value && localEndDate.value) {
    if (localStartDate.value === localEndDate.value) {
      return formatDate(localStartDate.value)
    }
    return `${formatDate(localStartDate.value)} - ${formatDate(localEndDate.value)}`
  }

  return t('dates.selectDateRange')
})

const formatDate = (dateStr: string): string => {
  const date = new Date(dateStr + 'T00:00:00')
  const dateLocale = locale.value === 'zh' ? 'zh-CN' : 'en-US'
  return date.toLocaleDateString(dateLocale, { month: 'short', day: 'numeric' })
}

const isPresetActive = (preset: DatePreset): boolean => {
  return activePreset.value === preset.value
}

const selectPreset = (preset: DatePreset) => {
  const range = preset.getRange()
  localStartDate.value = range.start
  localEndDate.value = range.end
  activePreset.value = preset.value
}

/** 按日期认按天的预设（今天、近 7 天……）；近 24 小时不参与：它的日期对上了也可能是自定义的两个自然日 */
const matchDayPreset = (start: string, end: string): string | null => {
  for (const preset of presets) {
    if (preset.value === LAST_24_HOURS_PRESET) continue
    const range = preset.getRange()
    if (range.start === start && range.end === end) return preset.value
  }
  return null
}

/** 手动改了日期：不再是近 24 小时，只可能是某个按天的预设或自定义 */
const onDateInput = () => {
  activePreset.value = matchDayPreset(localStartDate.value, localEndDate.value)
}

/** 页面给的范围：有预设就用，没有就按日期认 */
const syncFromProps = () => {
  localStartDate.value = props.startDate
  localEndDate.value = props.endDate
  activePreset.value = props.preset ?? matchDayPreset(props.startDate, props.endDate)
}

const toggle = () => {
  isOpen.value = !isOpen.value
}

const apply = () => {
  if (rangeReversed.value) return
  emit('update:startDate', localStartDate.value)
  emit('update:endDate', localEndDate.value)
  emit('change', {
    startDate: localStartDate.value,
    endDate: localEndDate.value,
    preset: activePreset.value
  })
  isOpen.value = false
}

const handleClickOutside = (event: MouseEvent) => {
  if (containerRef.value && !containerRef.value.contains(event.target as Node)) {
    isOpen.value = false
  }
}

const handleEscape = (event: KeyboardEvent) => {
  if (event.key === 'Escape' && isOpen.value) {
    // 告诉外层弹层这次 Esc 已经处理过，只收起面板、不关弹窗（useModalLayer 会跳过 defaultPrevented）
    event.preventDefault()
    isOpen.value = false
  }
}

// 首次渲染就显示页面给的范围；之后页面的范围变了（应用、重置、地址栏带入）就跟上；
// 关掉面板也回到页面当前的范围，没应用的改动和颠倒提示一起丢掉
syncFromProps()
watch(() => [props.startDate, props.endDate, props.preset], syncFromProps)
watch(isOpen, (open) => {
  if (!open) syncFromProps()
})

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  document.addEventListener('keydown', handleEscape)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
  document.removeEventListener('keydown', handleEscape)
})
</script>

<style scoped>
.date-picker-trigger {
  @apply flex items-center gap-2;
  @apply rounded-md px-3 py-2 text-sm;
  @apply bg-af-sheet;
  @apply border border-af-hairline-strong;
  @apply text-af-ink-2;
  @apply transition-colors duration-150;
  @apply focus:border-af-brand focus:outline-none focus:ring-2 focus:ring-af-brand/30;
  @apply hover:border-af-ink-4;
  @apply cursor-pointer;
}

.date-picker-trigger-open {
  @apply border-af-brand ring-2 ring-af-brand/30;
}

.date-picker-icon {
  @apply text-af-ink-3;
}

.date-picker-value {
  @apply font-medium;
}

.date-picker-chevron {
  @apply text-af-ink-3;
}

.date-picker-dropdown {
  @apply absolute left-0 z-[100] mt-2;
  @apply bg-af-sheet;
  @apply rounded-lg;
  @apply border border-af-hairline;
  @apply shadow-lg shadow-af-ink/10;
  @apply overflow-hidden;
  @apply min-w-[320px];
}

.date-picker-dropdown-end {
  @apply left-auto right-0;
}

.date-picker-presets {
  @apply grid grid-cols-2 gap-1 p-2;
}

.date-picker-preset {
  @apply rounded-md px-3 py-1.5 text-xs font-medium;
  @apply text-af-ink-2;
  @apply hover:bg-af-sunken;
  @apply transition-colors duration-150;
}

.date-picker-preset-active {
  @apply bg-af-brand-tint;
  @apply text-af-brand;
}

.date-picker-divider {
  @apply border-t border-af-hairline;
}

.date-picker-custom {
  @apply flex items-end gap-2 p-3;
}

.date-picker-field {
  @apply flex-1;
}

.date-picker-label {
  @apply mb-1 block text-xs font-medium text-af-ink-3;
}

.date-picker-input {
  @apply w-full rounded-md px-2 py-1.5 text-sm;
  @apply bg-af-sheet;
  @apply border border-af-hairline-strong;
  @apply text-af-ink;
  @apply focus:border-af-brand focus:outline-none focus:ring-2 focus:ring-af-brand/30;
}

.date-picker-input::-webkit-calendar-picker-indicator {
  @apply cursor-pointer opacity-60 hover:opacity-100;
  filter: invert(0.5);
}

.dark .date-picker-input::-webkit-calendar-picker-indicator {
  filter: none;
}

.date-picker-separator {
  @apply flex items-center justify-center pb-1;
}

.date-picker-actions {
  @apply flex justify-end p-2 pt-0;
}

.date-picker-error {
  @apply px-3 pb-2 text-xs text-af-danger;
}

.date-picker-apply {
  @apply rounded-md px-4 py-1.5 text-sm font-medium;
  @apply bg-af-brand text-af-on-brand;
  @apply hover:bg-af-brand-hover;
  @apply transition-colors duration-150;
  @apply disabled:cursor-not-allowed disabled:opacity-50;
}

/* Dropdown animation */
.date-picker-dropdown-enter-active,
.date-picker-dropdown-leave-active {
  transition: all 0.2s ease;
}

.date-picker-dropdown-enter-from,
.date-picker-dropdown-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}

/* 用户站：触发器不画框，只有图标 + 文字 + 箭头（与语言切换同一套，muqian 2026-09-23 不要方块）；管理端保持描边 */
.date-picker-trigger-plain {
  @apply border-transparent bg-transparent px-1 font-medium text-af-ink;
  @apply hover:border-transparent hover:text-af-ink-2 focus:border-transparent focus:ring-0 focus-visible:underline;
}
</style>
