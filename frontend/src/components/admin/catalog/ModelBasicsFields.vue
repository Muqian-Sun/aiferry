<template>
  <!--
    新建 / 编辑模型共用的「模型」字段：模型标识（下面的提示由调用方放进 #model-id-hint）、展示名、厂商。
    厂商：目录里已有的厂商做下拉（显示公司名，同一厂商族只出一项），另可手填；当前值不在下拉里时也列出来，免得编辑时被清掉。
  -->
  <div class="space-y-4">
    <div>
      <label class="input-label">{{ t('admin.modelCatalog.fields.modelId') }}</label>
      <input
        :value="modelId"
        class="input font-mono"
        required
        autocomplete="off"
        spellcheck="false"
        :placeholder="t('admin.modelCatalog.editor.modelIdPlaceholder')"
        data-testid="model-catalog-model-id"
        @input="emit('update:modelId', ($event.target as HTMLInputElement).value)"
      />
      <slot name="model-id-hint" />
    </div>

    <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <div>
        <label class="input-label">{{ t('admin.modelCatalog.fields.displayName') }}</label>
        <input
          :value="displayName"
          class="input"
          :placeholder="modelId"
          data-testid="model-catalog-display-name"
          @input="emit('update:displayName', ($event.target as HTMLInputElement).value)"
        />
      </div>
      <div>
        <label class="input-label">{{ t('admin.modelCatalog.fields.vendor') }}</label>
        <select v-model="vendorChoice" class="input" data-testid="model-catalog-vendor">
          <option value="">{{ t('admin.modelCatalog.editor.vendorNone') }}</option>
          <option v-for="option in vendorChoices" :key="option.value" :value="option.value">{{ option.label }}</option>
          <option :value="CUSTOM_VENDOR">{{ t('admin.modelCatalog.editor.vendorCustom') }}</option>
        </select>
        <input
          v-if="customVendor"
          :value="vendor"
          class="input mt-2"
          :placeholder="t('admin.modelCatalog.editor.vendorCustomPlaceholder')"
          data-testid="model-catalog-vendor-custom"
          @input="emit('update:vendor', ($event.target as HTMLInputElement).value)"
        />
        <p class="input-hint">{{ t('admin.modelCatalog.editor.vendorHint') }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { vendorLabel } from '@/components/modelPlaza/catalog'
import type { CatalogVendorChoice } from './vendorLabel'

const props = defineProps<{
  modelId: string
  displayName: string
  vendor: string
  /** 目录里已有的厂商（按展示名分组） */
  vendorOptions: CatalogVendorChoice[]
}>()

const emit = defineEmits<{
  'update:modelId': [value: string]
  'update:displayName': [value: string]
  'update:vendor': [value: string]
}>()

const { t } = useI18n()

const CUSTOM_VENDOR = '__custom__'
const customVendor = ref(false)

// 当前值属于哪一族就让那一项取当前值（编辑时不被悄悄换成同族的另一个串）；哪一族都不是（手填过的）单独列一项
const vendorChoices = computed(() => {
  const current = customVendor.value ? '' : props.vendor
  const choices = props.vendorOptions.map((option) =>
    current && option.values.includes(current) ? { label: option.label, value: current } : { label: option.label, value: option.value }
  )
  if (current && !props.vendorOptions.some((option) => option.values.includes(current))) {
    choices.push({ label: vendorLabel(current), value: current })
    choices.sort((a, b) => a.label.localeCompare(b.label))
  }
  return choices
})

const vendorChoice = computed({
  get: () => (customVendor.value ? CUSTOM_VENDOR : props.vendor ?? ''),
  set: (value: string) => {
    if (value === CUSTOM_VENDOR) {
      customVendor.value = true
      emit('update:vendor', '')
      return
    }
    customVendor.value = false
    emit('update:vendor', value)
  }
})

/** 调用方换了一个模型（或按价格文件带出了厂商）时退出手填 */
function resetCustomVendor() {
  customVendor.value = false
}

defineExpose({ resetCustomVendor })
</script>
