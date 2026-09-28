<template>
  <!--
    模型改名（muqian 2026-09-25 去掉白名单后只剩这一项）：映射只改名，渠道承接哪些模型由「承接的模型」决定。
    提交时由表单给 credentials 打 model_mapping_rename_only 标记。行用下标做 key：每次输入都换新数组，
    下标不变输入框就不丢焦点。
  -->
  <div data-testid="model-rename-editor">
    <label v-if="showTitle" class="input-label">{{ t('admin.accounts.modelRename.title') }}</label>
    <p class="input-hint mb-3">{{ t('admin.accounts.modelRename.hint') }}</p>
    <p v-if="extendsVendorTable" class="input-hint -mt-2 mb-3" data-testid="model-rename-vendor-table-hint">
      {{ t('admin.accounts.modelRename.vendorTableHint') }}
    </p>

    <div v-if="modelValue.length > 0" class="mb-3 space-y-2">
      <div v-for="(mapping, index) in modelValue" :key="index" class="space-y-1">
        <div class="flex items-center gap-2">
          <input
            :value="mapping.from"
            type="text"
            :class="['input flex-1', !isValidWildcardPattern(mapping.from) ? 'border-af-danger' : '']"
            :placeholder="t('admin.accounts.requestModel')"
            :data-testid="`model-rename-from-${index}`"
            @input="updateRow(index, 'from', $event)"
          />
          <Icon name="arrowRight" size="sm" class="shrink-0 text-af-ink-3" />
          <input
            :value="mapping.to"
            type="text"
            :class="['input flex-1', mapping.to.includes('*') ? 'border-af-danger' : '']"
            :placeholder="t('admin.accounts.actualModel')"
            :data-testid="`model-rename-to-${index}`"
            @input="updateRow(index, 'to', $event)"
          />
          <button
            type="button"
            class="rounded-lg p-2 text-af-danger transition-colors hover:bg-af-danger-tint"
            :aria-label="t('common.delete')"
            @click="removeRow(index)"
          >
            <Icon name="trash" size="sm" />
          </button>
        </div>
        <p v-if="!isValidWildcardPattern(mapping.from)" class="text-xs text-af-danger">
          {{ t('admin.accounts.wildcardOnlyAtEnd') }}
        </p>
        <p v-if="mapping.to.includes('*')" class="text-xs text-af-danger">
          {{ t('admin.accounts.targetNoWildcard') }}
        </p>
      </div>
    </div>

    <div class="flex flex-wrap items-center gap-2">
      <button type="button" class="btn btn-secondary btn-sm" data-testid="model-rename-add" @click="addRow('', '')">
        <Icon name="plus" size="sm" />
        {{ t('admin.accounts.addMapping') }}
      </button>
      <slot name="actions" />
    </div>

    <div v-if="presets.length > 0" class="mt-3 flex flex-wrap gap-2">
      <button
        v-for="preset in presets"
        :key="preset.label"
        type="button"
        class="rounded-md border border-af-hairline px-2.5 py-1 text-xs text-af-ink-2 transition-colors hover:border-af-hairline-strong hover:text-af-ink"
        @click="addPreset(preset.from, preset.to)"
      >
        + {{ preset.label }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { isValidWildcardPattern, type ModelMappingEntry } from '@/composables/useModelWhitelist'

const props = withDefaults(
  defineProps<{
    modelValue: ModelMappingEntry[]
    /** 快捷改名项（只放真正改名的，同名项没有意义）。 */
    presets?: { label: string; from: string; to: string }[]
    /** 上游自带模型表（Antigravity / xAI）：表外的模型要加一条改名（可同名）才承接。 */
    extendsVendorTable?: boolean
    /** 外面已有标题（批量编辑的勾选行）时不再重复。 */
    showTitle?: boolean
  }>(),
  { presets: () => [], extendsVendorTable: false, showTitle: true }
)

const emit = defineEmits<{
  'update:modelValue': [value: ModelMappingEntry[]]
}>()

const { t } = useI18n()
const appStore = useAppStore()

function updateRow(index: number, field: 'from' | 'to', event: Event) {
  const value = (event.target as HTMLInputElement).value
  emit('update:modelValue', props.modelValue.map((row, i) => (i === index ? { ...row, [field]: value } : row)))
}

function removeRow(index: number) {
  emit('update:modelValue', props.modelValue.filter((_, i) => i !== index))
}

function addRow(from: string, to: string) {
  emit('update:modelValue', [...props.modelValue, { from, to }])
}

function addPreset(from: string, to: string) {
  if (props.modelValue.some((row) => row.from === from)) {
    appStore.showInfo(t('admin.accounts.mappingExists', { model: from }))
    return
  }
  addRow(from, to)
}
</script>
