<template>
  <!--
    编辑模型（一步）：模型标识、展示名、厂商、上架。
    官方价、分段、联网搜索价、承接渠道都在价格页改；计费方式、按次 / 图片 / 视频价不在表单里，保存时按条目原值整条写回。
  -->
  <BaseDialog :show="show" :title="t('admin.modelCatalog.edit')" width="normal" @close="handleClose">
    <form v-if="entry" id="model-edit-form" class="space-y-4" @submit.prevent="save">
      <ModelBasicsFields
        ref="basicsRef"
        v-model:model-id="form.model_id"
        v-model:display-name="form.display_name"
        v-model:vendor="form.vendor"
        :vendor-options="vendorOptions"
      />

      <div>
        <label class="input-label">{{ t('admin.modelCatalog.fields.status') }}</label>
        <select v-model="form.status" class="input" data-testid="model-catalog-status">
          <option value="listed">{{ t('admin.modelCatalog.status.listed') }}</option>
          <option value="unlisted">{{ t('admin.modelCatalog.status.unlisted') }}</option>
        </select>
        <p class="input-hint">{{ t('admin.modelCatalog.dialog.listingRule') }}</p>
      </div>

      <div class="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-af-sunken px-3 py-2 text-13">
        <span class="text-af-ink-2">{{ t('admin.modelCatalog.dialog.pricingElsewhere') }}</span>
        <RouterLink
          :to="{ path: '/pricing', query: { model: entry.model_id } }"
          class="font-medium text-af-ink hover:underline"
          data-testid="model-edit-pricing-link"
        >
          {{ t('admin.modelCatalog.dialog.openPricing') }}
        </RouterLink>
      </div>
    </form>

    <template #footer>
      <div class="flex w-full flex-wrap items-center justify-end gap-3">
        <FormError class="mr-auto min-w-0 flex-1" :message="submitError" />
        <button type="button" class="btn btn-secondary" @click="handleClose">{{ t('common.cancel') }}</button>
        <button type="submit" form="model-edit-form" class="btn btn-primary" :disabled="saving || !entry" data-testid="model-catalog-save">
          <Icon v-if="saving" name="refresh" size="sm" class="-ml-1 mr-2 animate-spin" />
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { ModelCatalogEntry } from '@/api/admin/modelCatalog'
import BaseDialog from '@/components/common/BaseDialog.vue'
import FormError from '@/components/common/FormError.vue'
import Icon from '@/components/icons/Icon.vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import ModelBasicsFields from './ModelBasicsFields.vue'
import { entryToRequest } from './entryRequest'

const props = defineProps<{
  show: boolean
  entry: ModelCatalogEntry | null
  /** 目录里已有的厂商标签 */
  vendorOptions: string[]
}>()

const emit = defineEmits<{ close: []; saved: [entry: ModelCatalogEntry] }>()

const { t } = useI18n()

const basicsRef = ref<InstanceType<typeof ModelBasicsFields> | null>(null)
const saving = ref(false)
const submitError = ref('')

const form = reactive({
  model_id: '',
  display_name: '',
  vendor: '',
  status: 'unlisted' as string
})

watch(
  [() => props.show, () => props.entry],
  ([show, entry]) => {
    if (!show || !entry) return
    submitError.value = ''
    basicsRef.value?.resetCustomVendor()
    Object.assign(form, {
      model_id: entry.model_id,
      display_name: entry.display_name,
      vendor: entry.vendor,
      status: entry.status
    })
  },
  { immediate: true }
)

async function save() {
  const entry = props.entry
  if (!entry) return
  submitError.value = ''
  // 上架要有渠道承接（muqian 2026-10-03）：只拦「这次从未上架改成上架」，已上架的条目改别的字段照常保存
  if (form.status === 'listed' && entry.status !== 'listed' && (entry.bindings?.length ?? 0) === 0) {
    submitError.value = t('admin.modelCatalog.dialog.listingBlocked.channel')
    return
  }
  saving.value = true
  try {
    const updated = await adminAPI.modelCatalog.updateEntry(entry.id, {
      ...entryToRequest(entry),
      model_id: form.model_id.trim(),
      display_name: form.display_name.trim(),
      vendor: form.vendor.trim(),
      status: form.status
    })
    emit('saved', updated)
    emit('close')
  } catch (error) {
    submitError.value = extractApiErrorMessage(error, t('admin.modelCatalog.dialog.saveFailed'))
  } finally {
    saving.value = false
  }
}

function handleClose() {
  emit('close')
}
</script>
