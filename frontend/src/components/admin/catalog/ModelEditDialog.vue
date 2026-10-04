<template>
  <!--
    编辑模型（一步）：模型标识、展示名、厂商、别名、上架、备注。
    别名增删各自一个请求、立即生效（不随「保存」）；其余字段点「保存」整条写回。
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

      <div data-testid="model-catalog-aliases">
        <label class="input-label" for="model-catalog-alias-input">{{ t('admin.modelCatalog.dialog.aliases.label') }}</label>
        <ul v-if="aliases.length" class="mb-2 flex flex-wrap gap-1.5">
          <li
            v-for="alias in aliases"
            :key="alias.id"
            class="inline-flex items-center gap-1 rounded bg-af-sunken py-1 pl-2 pr-1 font-mono text-xs text-af-ink-2"
            :title="alias.notes || undefined"
          >
            {{ alias.alias }}
            <span v-if="alias.source === 'seed'" class="pr-1 font-sans text-af-ink-3" :title="t('admin.modelCatalog.dialog.aliases.seedTitle')">
              {{ t('admin.modelCatalog.dialog.aliases.seed') }}
            </span>
            <button
              v-else
              type="button"
              class="rounded p-0.5 text-af-ink-3 transition-colors hover:bg-af-hairline hover:text-af-ink disabled:cursor-not-allowed disabled:opacity-35"
              :disabled="aliasBusy"
              :aria-label="t('admin.modelCatalog.dialog.aliases.remove', { alias: alias.alias })"
              :title="t('admin.modelCatalog.dialog.aliases.remove', { alias: alias.alias })"
              data-testid="model-catalog-alias-remove"
              @click="removeAlias(alias)"
            >
              <Icon name="x" size="xs" />
            </button>
          </li>
        </ul>
        <div class="flex gap-2">
          <input
            id="model-catalog-alias-input"
            v-model="aliasDraft"
            :class="['input font-mono', aliasError ? 'input-error' : '']"
            autocomplete="off"
            spellcheck="false"
            :placeholder="t('admin.modelCatalog.dialog.aliases.placeholder')"
            :disabled="aliasBusy"
            data-testid="model-catalog-alias-input"
            @keydown.enter.prevent="addAlias"
          />
          <button
            type="button"
            class="btn btn-secondary shrink-0"
            :disabled="aliasBusy || !aliasDraft.trim()"
            data-testid="model-catalog-alias-add"
            @click="addAlias"
          >
            {{ t('admin.modelCatalog.dialog.aliases.add') }}
          </button>
        </div>
        <p v-if="aliasError" class="input-error-text" data-testid="model-catalog-alias-error">{{ aliasError }}</p>
        <p v-else class="input-hint">{{ t('admin.modelCatalog.dialog.aliases.hint') }}</p>
      </div>

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
import type { ModelCatalogAlias, ModelCatalogEntry } from '@/api/admin/modelCatalog'
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
  /** 别名增删已经写进库（不等「保存」）：目录页要重新拉列表 */
  'aliases-changed': []
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

// 别名：弹窗里的这份随增删即时更新，和库里一致
const aliases = ref<ModelCatalogAlias[]>([])
const aliasDraft = ref('')
const aliasError = ref('')
const aliasBusy = ref(false)

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
    aliases.value = [...(entry.aliases ?? [])]
    aliasDraft.value = ''
    aliasError.value = ''
  },
  { immediate: true }
)

async function addAlias() {
  const entry = props.entry
  const alias = aliasDraft.value.trim()
  if (!entry || !alias || aliasBusy.value) return
  aliasError.value = ''
  aliasBusy.value = true
  try {
    const created = await adminAPI.modelCatalog.createAlias({ entry_id: entry.id, alias })
    aliases.value = [...aliases.value, created].sort((a, b) => a.alias.localeCompare(b.alias))
    aliasDraft.value = ''
    emit('aliases-changed')
  } catch (error) {
    aliasError.value = extractApiErrorMessage(error, t('admin.modelCatalog.dialog.aliases.addFailed'), {
      MODEL_CATALOG_ALIAS_EXISTS: t('admin.modelCatalog.dialog.aliases.exists')
    })
  } finally {
    aliasBusy.value = false
  }
}

async function removeAlias(alias: ModelCatalogAlias) {
  if (aliasBusy.value) return
  aliasError.value = ''
  aliasBusy.value = true
  try {
    await adminAPI.modelCatalog.deleteAlias(alias.id)
    aliases.value = aliases.value.filter((item) => item.id !== alias.id)
    emit('aliases-changed')
  } catch (error) {
    aliasError.value = extractApiErrorMessage(error, t('admin.modelCatalog.dialog.aliases.removeFailed'))
  } finally {
    aliasBusy.value = false
  }
}

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
