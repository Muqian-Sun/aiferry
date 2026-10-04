<template>
  <!--
    单价输入：条目里存的是 $/token（按 Token 的价）或 $/次，这里按「每百万 Token」或原单位显示和输入
    （muqian 2026-09-25：添加模型时按每百万 Token 填价，不再数零）。
    用文本框 + 本地文本：数字框在输入「0.」这类中间态时读不到值，回写会把用户正在输的内容清掉。
    compact：价格页表格里的窄格，不显示单位（单位写在表头），必填没填时标红。
  -->
  <div class="relative">
    <input
      :value="text"
      type="text"
      inputmode="decimal"
      autocomplete="off"
      :placeholder="placeholder"
      :aria-label="label"
      :class="[
        compact ? 'input h-8 w-24 px-2 py-1 text-right text-13 tabular-nums' : 'input pr-28 tabular-nums',
        invalid || (required && modelValue == null) ? 'border-af-danger' : ''
      ]"
      :data-testid="testId"
      @input="onInput"
      @blur="onBlur"
    />
    <span v-if="!compact" class="pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs text-af-ink-3">{{ unit }}</span>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'

const props = withDefaults(
  defineProps<{
    /** 存储单位的值：$/token 或 $/次；null = 未配置 */
    modelValue: number | null | undefined
    /** 显示值 = 存储值 × scale（每百万 Token 为 1e6，按次为 1） */
    scale?: number
    /** 单位文案，显示在输入框右侧 */
    unit: string
    testId?: string
    /** 表格里的窄格：不显示单位 */
    compact?: boolean
    /** 无障碍标签（窄格没有可见的 label 时用） */
    label?: string
    /** 必填：没填时标红 */
    required?: boolean
    placeholder?: string
  }>(),
  { scale: 1, testId: undefined, compact: false, label: undefined, required: false, placeholder: undefined }
)

const emit = defineEmits<{ 'update:modelValue': [value: number | null] }>()

// 换算后去掉浮点尾巴（3e-6 × 1e6 = 2.9999999999999996）；至少写两位小数，同一列里 3 / 0.3 / 3.75 写成 3.00 / 0.30 / 3.75，
// 更细的价照常写全（0.024、0.1875）（2026-10-04 走查 S13：价格页同一列小数位长短不一）
function format(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) return ''
  const shown = Number((value * props.scale).toPrecision(12))
  const decimals = (String(shown).split('.')[1] ?? '').length
  return decimals >= 2 || String(shown).includes('e') ? String(shown) : shown.toFixed(2)
}

function parse(raw: string): number | null | undefined {
  const trimmed = raw.trim()
  if (trimmed === '') return null
  const n = Number(trimmed)
  return Number.isFinite(n) && n >= 0 ? n : undefined
}

const text = ref(format(props.modelValue))
const invalid = ref(false)

function onInput(event: Event) {
  text.value = (event.target as HTMLInputElement).value
  const parsed = parse(text.value)
  invalid.value = parsed === undefined
  // 填错（负数、不是数字）回写 NaN：原来什么都不回写，保存时提交的是上一次的合法值，看着填的和存的对不上。
  // 用的地方把 NaN 当「格式不对」拦下保存（pricingDraft 的 invalid、模型弹窗的搜索价）
  emit('update:modelValue', parsed === undefined ? Number.NaN : parsed == null ? null : parsed / props.scale)
}

// 输入时保持原样，离开输入框后按同一格式规整（值没变，只改显示）；填错的保留原文，让用户看到哪里错了
function onBlur() {
  if (invalid.value) return
  text.value = format(props.modelValue)
}

// 外部改了值（带价、切换条目）才重写文本；用户自己输入引起的回写不动文本，免得打断输入
watch(
  () => props.modelValue,
  (value) => {
    const typed = parse(text.value)
    const typedStored = typed == null ? typed : typed / props.scale
    const same =
      value != null && Number.isNaN(value)
        ? typedStored === undefined
        : typedStored === null
          ? value == null
          : typedStored !== undefined && value != null && Math.abs(typedStored - value) <= Math.abs(value) * 1e-9
    if (!same) {
      text.value = format(value)
      invalid.value = false
    }
  }
)
</script>
