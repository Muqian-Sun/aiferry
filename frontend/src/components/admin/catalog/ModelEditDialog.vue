<template>
  <!--
    编辑模型（一步）：模型标识、展示名、厂商、上架、备注，点「保存」整条写回。
    目录不存别名（muqian 2026-10-06）：上游名字不同在渠道承接行的上游模型名里配。
    官方价、分段、联网搜索价、承接渠道都在价格页改；计费方式、按次 / 图片 / 视频价不在表单里，保存时按条目原值整条写回。
  -->
  <BaseDialog :show="show" :title="t('admin.modelCatalog.edit')" width="normal" @close="handleClose">
    <form v-if="entry" id="model-edit-form" class="space-y-4" @submit.prevent="save()">
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

      <div>
        <label class="input-label" for="model-catalog-notes">{{ t('admin.modelCatalog.dialog.notes') }}</label>
        <textarea
          id="model-catalog-notes"
          v-model="form.notes"
          class="input"
          rows="2"
          :placeholder="t('admin.modelCatalog.dialog.notesPlaceholder')"
          data-testid="model-catalog-notes"
        ></textarea>
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

  <!-- 有承接但没有能调度的渠道：不拦上架，先确认（D6）；没有承接的照旧拦在上面 -->
  <ConfirmDialog
    :show="confirmListing"
    :title="t('admin.modelCatalog.unschedulable.confirmTitle')"
    :message="t('admin.modelCatalog.unschedulable.confirmMessage', { models: entry ? unschedulableListText([entry], t) : '' })"
    :confirm-text="t('admin.modelCatalog.unschedulable.confirm')"
    :cancel-text="t('common.cancel')"
    @confirm="onConfirmListing"
    @cancel="confirmListing = false"
  />
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { ModelCatalogEntry } from '@/api/admin/modelCatalog'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import FormError from '@/components/common/FormError.vue'
import Icon from '@/components/icons/Icon.vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import ModelBasicsFields from './ModelBasicsFields.vue'
import { entryToRequest } from './entryRequest'
import type { CatalogVendorChoice } from './vendorLabel'
import { hasNoSchedulableChannel, unschedulableListText } from './schedulable'

const props = defineProps<{
  show: boolean
  entry: ModelCatalogEntry | null
  /** 目录里已有的厂商（按展示名分组） */
  vendorOptions: CatalogVendorChoice[]
}>()

const emit = defineEmits<{
  close: []
  saved: [entry: ModelCatalogEntry]
}>()

const { t } = useI18n()

const basicsRef = ref<InstanceType<typeof ModelBasicsFields> | null>(null)
const saving = ref(false)
const submitError = ref('')
const confirmListing = ref(false)

const form = reactive({
  model_id: '',
  display_name: '',
  vendor: '',
  status: 'unlisted' as string,
  notes: ''
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
      status: entry.status,
      notes: entry.notes ?? ''
    })
  },
  { immediate: true }
)

async function save(confirmed = false) {
  const entry = props.entry
  if (!entry) return
  submitError.value = ''
  const listing = form.status === 'listed' && entry.status !== 'listed'
  // 上架要有渠道承接（muqian 2026-10-03）：只拦「这次从未上架改成上架」，已上架的条目改别的字段照常保存
  if (listing && (entry.bindings?.length ?? 0) === 0) {
    submitError.value = t('admin.modelCatalog.dialog.listingBlocked.channel')
    return
  }
  if (listing && !confirmed && hasNoSchedulableChannel(entry)) {
    confirmListing.value = true
    return
  }
  saving.value = true
  try {
    const notes = form.notes.trim()
    const updated = await adminAPI.modelCatalog.updateEntry(entry.id, {
      ...entryToRequest(entry),
      model_id: form.model_id.trim(),
      display_name: form.display_name.trim(),
      vendor: form.vendor.trim(),
      status: form.status,
      notes: notes === '' ? null : notes
    })
    emit('saved', updated)
    emit('close')
  } catch (error) {
    submitError.value = extractApiErrorMessage(error, t('admin.modelCatalog.dialog.saveFailed'), {
      MODEL_CATALOG_ENTRY_EXISTS: t('admin.modelCatalog.dialog.idTaken')
    })
  } finally {
    saving.value = false
  }
}

function onConfirmListing() {
  confirmListing.value = false
  void save(true)
}

function handleClose() {
  emit('close')
}
</script>
