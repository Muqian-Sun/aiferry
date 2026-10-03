<template>
  <!--
    新建 / 编辑模型共用的「模型」字段：模型标识（下面的提示由调用方放进 #model-id-hint）、展示名、厂商。
    厂商：目录里已有的厂商做下拉，另可手填；当前值不在下拉里时也列出来，免得编辑时被清掉。
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
          <option v-for="option in vendorChoices" :key="option" :value="option">{{ option }}</option>
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

const props = defineProps<{
  modelId: string
  displayName: string
  vendor: string
  /** 目录里已有的厂商标签 */
  vendorOptions: string[]
}>()

const emit = defineEmits<{
  'update:modelId': [value: string]
  'update:displayName': [value: string]
  'update:vendor': [value: string]
}>()

const { t } = useI18n()

const CUSTOM_VENDOR = '__custom__'
const customVendor = ref(false)

const vendorChoices = computed(() =>
  [...new Set([...props.vendorOptions, ...(customVendor.value || !props.vendor ? [] : [props.vendor])])].sort()
)

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
