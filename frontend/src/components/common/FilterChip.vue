<template>
  <!--
    筛选标签（A4）：没选时是虚线小标签「状态」，选了变实心「状态：异常」并带 ✕ 一键清掉。
    替代原来一排 160px 宽的下拉框，工具行只占一行、选了什么一眼可见。
  -->
  <PopoverMenu align="start" width-class="w-52" @open="$emit('open')">
    <template #trigger="{ open }">
      <button
        type="button"
        :class="[
          'inline-flex h-8 items-center gap-1.5 rounded-full border px-3 text-13 transition-colors',
          isSet
            ? 'border-af-hairline-strong bg-af-sunken text-af-ink'
            : 'border-dashed border-af-hairline-strong text-af-ink-2 hover:border-af-ink-4 hover:text-af-ink',
          open ? 'border-af-ink-4' : ''
        ]"
        :aria-label="isSet ? `${label}${t('common.labelSeparator')}${selectedLabel}` : label"
        :data-testid="testId"
      >
        <span>{{ label }}</span>
        <template v-if="isSet">
          <span class="-ml-1.5 max-w-[10rem] truncate"><span class="text-af-ink-3">{{ t('common.labelSeparator') }}</span><span class="font-medium">{{ selectedLabel }}</span></span>
          <span
            role="button"
            tabindex="0"
            class="-mr-1 rounded-full p-0.5 text-af-ink-3 hover:bg-af-hairline hover:text-af-ink"
            :aria-label="clearLabel"
            :data-testid="testId ? `${testId}-clear` : undefined"
            @click.stop="select('')"
            @keydown.enter.stop.prevent="select('')"
          >
            <Icon name="x" size="xs" :stroke-width="2" />
          </span>
        </template>
        <Icon v-else name="chevronDown" size="xs" class="text-af-ink-3" />
      </button>
    </template>
    <!-- 选项多（模型、密钥）时面板内滚动，不撑出视口 -->
    <div class="max-h-72 overflow-y-auto">
      <MenuItem
        v-for="option in options"
        :key="String(option.value)"
        :checked="String(option.value) === String(modelValue ?? '')"
        :data-testid="testId ? `${testId}-option-${option.value}` : undefined"
        @click="select(option.value)"
      >
        {{ option.label }}
      </MenuItem>
    </div>
  </PopoverMenu>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PopoverMenu from './PopoverMenu.vue'
import MenuItem from './MenuItem.vue'
import type { FilterOption } from './types'

const props = defineProps<{
  label: string
  modelValue: string | number | null | undefined
  options: FilterOption[]
  testId?: string
  /**
   * 选中值不在选项里时显示的文字（选项还在加载，或那一项已删除）。
   * 用户站按 ID 筛选时必须传，免得把内部 ID 当名字显示；不传时显示原始值（管理站按 ID 筛选保持原样）。
   */
  missingLabel?: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string | number): void
  (e: 'change', value: string | number): void
  (e: 'open'): void
}>()

const { t } = useI18n()
const clearLabel = computed(() => t('common.clear'))

const isSet = computed(() => props.modelValue !== '' && props.modelValue !== null && props.modelValue !== undefined)
const selectedLabel = computed(
  () =>
    props.options.find((o) => String(o.value) === String(props.modelValue))?.label ??
    props.missingLabel ??
    String(props.modelValue)
)

function select(value: string | number) {
  if (String(value) === String(props.modelValue ?? '')) return
  emit('update:modelValue', value)
  emit('change', value)
}
</script>
