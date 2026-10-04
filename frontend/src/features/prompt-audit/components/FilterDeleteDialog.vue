<template>
  <BaseDialog :show="show" :title="t('admin.promptAudit.events.filterDeleteDialogTitle')" width="wide" @close="$emit('close')">
    <div class="space-y-5 text-sm">
      <p class="text-af-ink-3">{{ t('admin.promptAudit.events.filterDeleteDialogDesc') }}</p>

      <fieldset>
        <legend class="text-xs font-medium text-af-ink-2">{{ t('admin.promptAudit.events.filterTimeRange') }}</legend>
        <div class="mt-2 max-w-full overflow-x-auto">
          <SegmentedControl
            :model-value="preset"
            :options="presetOptions"
            :label="t('admin.promptAudit.events.filterTimeRange')"
            test-id-prefix="range-preset"
            @update:model-value="setPreset"
          />
        </div>
        <p class="mt-2 text-xs text-af-ink-3">{{ t('admin.promptAudit.events.filterTimeRangeHint') }}</p>
        <div v-if="preset === 'custom'" class="mt-3 grid gap-3 sm:grid-cols-2" data-test="custom-range">
          <label class="text-xs text-af-ink-2">
            <span>{{ t('admin.promptAudit.events.startAt') }}</span>
            <input v-model="local.start_at" type="datetime-local" class="input mt-1 w-full" :aria-label="t('admin.promptAudit.events.startAt')" @change="criteriaChanged" />
          </label>
          <label class="text-xs text-af-ink-2">
            <span>{{ t('admin.promptAudit.events.endAt') }}</span>
            <input v-model="local.end_at" type="datetime-local" class="input mt-1 w-full" :aria-label="t('admin.promptAudit.events.endAt')" @change="criteriaChanged" />
          </label>
          <p v-if="!canPreview" class="text-xs text-af-danger sm:col-span-2">{{ t('admin.promptAudit.events.customRangeInvalid') }}</p>
        </div>
      </fieldset>

      <!-- 判定 / 风险等级：与事件列表同一种筛选小控件，不选 = 全部 -->
      <div class="flex flex-wrap items-center gap-2">
        <FilterChip v-model="local.decision" :label="t('admin.promptAudit.events.decision')" :options="decisionOptions" test-id="delete-decision" @change="criteriaChanged" />
        <FilterChip v-model="local.risk_level" :label="t('admin.promptAudit.events.risk')" :options="riskOptions" test-id="delete-risk" @change="criteriaChanged" />
      </div>

      <details data-test="more-conditions">
        <summary class="cursor-pointer select-none text-xs font-medium text-af-ink-2">{{ t('admin.promptAudit.events.moreConditions') }}</summary>
        <div class="mt-3 grid gap-3 sm:grid-cols-2">
          <label class="text-xs text-af-ink-2">
            <span>{{ t('admin.promptAudit.events.endpoint') }}</span>
            <input v-model="local.endpoint" type="text" class="input mt-1 w-full" :aria-label="t('admin.promptAudit.events.endpoint')" @input="criteriaChanged" />
          </label>
          <label class="text-xs text-af-ink-2">
            <span>{{ t('admin.promptAudit.events.keyword') }}</span>
            <input v-model="local.keyword" type="text" class="input mt-1 w-full" :aria-label="t('admin.promptAudit.events.keyword')" @input="criteriaChanged" />
          </label>
          <!-- 按邮箱选用户，不让人手填内部 id -->
          <div class="text-xs text-af-ink-2">
            <span>{{ t('admin.promptAudit.events.filterUser') }}</span>
            <EntityPicker ref="userPickerRef" kind="user" class="mt-1" :model-value="selectedUserId" @update:model-value="setUser" />
          </div>
        </div>
      </details>

      <div v-if="preview" class="space-y-1 border-t border-af-hairline pt-4" data-test="delete-preview-result">
        <p class="text-sm font-semibold text-af-danger">{{ t('admin.promptAudit.events.filterDeleteCount', { count: preview.matched_count }) }}</p>
        <p class="text-xs text-af-ink-3">{{ t('admin.promptAudit.events.expiresAt', { time: formatDate(preview.expires_at) }) }}</p>
        <p class="text-xs text-af-warning">{{ t('admin.promptAudit.events.filterDeleteWarning') }}</p>
      </div>
      <p v-else class="border-t border-af-hairline pt-4 text-xs text-af-ink-3" data-test="delete-preview-empty">
        {{ t('admin.promptAudit.events.filterDeleteNeedPreview') }}
      </p>
    </div>

    <template #footer>
      <div class="flex flex-wrap items-center justify-end gap-3">
        <FormError v-if="error" class="mr-auto" :message="error" />
        <p v-else-if="confirmDisabledReason" class="mr-auto text-xs text-af-ink-3" data-test="confirm-disabled-reason">
          {{ t(confirmDisabledReason) }}
        </p>
        <button type="button" class="btn btn-secondary" @click="$emit('close')">{{ t('common.cancel') }}</button>
        <button type="button" class="btn btn-secondary" :disabled="!canPreview || previewing || deleting" data-test="run-delete-preview" @click="requestPreview">
          {{ previewing ? t('admin.promptAudit.events.filterDeletePreviewing') : t('admin.promptAudit.events.filterDeletePreviewAction') }}
        </button>
        <button
          type="button"
          class="btn btn-danger"
          :disabled="confirmDisabled"
          :title="confirmDisabledReason ? t(confirmDisabledReason) : undefined"
          data-test="confirm-filter-delete"
          @click="requestConfirm"
        >
          {{ deleting ? t('common.deleting') : t('admin.promptAudit.events.confirmFilterDelete') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import FilterChip from '@/components/common/FilterChip.vue'
import FormError from '@/components/common/FormError.vue'
import SegmentedControl from '@/components/common/SegmentedControl.vue'
import type { FilterOption } from '@/components/common/types'
import EntityPicker from '@/components/admin/form/EntityPicker.vue'
import { adminAPI } from '@/api/admin'
import type { PromptDeletePreview, PromptEventFilters } from '../types'
import {
  DELETE_RANGE_PRESETS,
  cloneData,
  emptyEventFilters,
  hasExplicitDeleteRange,
  resolveDeleteRangeFilters,
  type DeleteRangePreset,
} from '../viewModel'

const props = defineProps<{
  show: boolean
  initialFilters: PromptEventFilters
  preview: PromptDeletePreview | null
  previewing: boolean
  deleting: boolean
  /** 预览 / 删除失败的原因，显示在底栏左侧 */
  error?: string
}>()
const emit = defineEmits<{
  (event: 'close'): void
  (event: 'preview', value: PromptEventFilters): void
  (event: 'confirm', value: PromptEventFilters): void
  (event: 'criteria-change'): void
}>()
const { t, locale } = useI18n()

const preset = ref<DeleteRangePreset>('7d')
const local = reactive<PromptEventFilters>(emptyEventFilters())
const presetOptions = computed(() => DELETE_RANGE_PRESETS.map((option) => ({ key: option.id, label: t(`admin.promptAudit.events.timePresets.${option.id}`) })))
const decisionOptions = computed<FilterOption[]>(() => ['pass', 'flag', 'critical'].map((value) => ({ value, label: t(`admin.promptAudit.decisions.${value}`) })))
const riskOptions = computed<FilterOption[]>(() => ['low', 'medium', 'high', 'critical'].map((value) => ({ value, label: t(`admin.promptAudit.riskLevels.${value}`) })))
function setPreset(value: DeleteRangePreset) {
  preset.value = value
  criteriaChanged()
}

// 用户条件：按邮箱选；从列表筛选继承来的只有 id，打开时查出邮箱显示（查不到就是已删除用户），不显示数字
const userPickerRef = ref<InstanceType<typeof EntityPicker> | null>(null)
const selectedUserId = computed(() => {
  const id = Number(local.user_id)
  return Number.isInteger(id) && id > 0 ? id : undefined
})
function setUser(userId: number | undefined) {
  local.user_id = userId ? String(userId) : ''
  criteriaChanged()
}
async function showInheritedUser() {
  const userId = selectedUserId.value
  if (!userId) return
  await nextTick()
  const revision = userPickerRef.value?.getRevision()
  let label: string
  try {
    label = (await adminAPI.users.getById(userId, true)).email
  } catch {
    label = t('common.deletedUser')
  }
  // 查的过程中用户已经改了条件，就不回填
  if (selectedUserId.value !== userId || userPickerRef.value?.getRevision() !== revision) return
  userPickerRef.value?.setKeyword(label)
}

watch(
  () => props.show,
  (visible) => {
    if (!visible) return
    const initial = cloneData(props.initialFilters)
    // Only inherit an explicit list-filter range; otherwise default to the
    // seven-day preset so a careless click can never target everything.
    preset.value = hasExplicitDeleteRange(initial) ? 'custom' : '7d'
    Object.assign(local, initial)
    void showInheritedUser()
  },
  { immediate: true },
)

const canPreview = computed(() => preset.value !== 'custom' || hasExplicitDeleteRange(local))

// One-click flow: a valid criteria selection is enough to confirm — the parent
// mints the server-side confirmation token on the fly. The button stays
// disabled only when the range is invalid, work is in flight, or a fresh
// preview already proved there is nothing to delete.
const confirmDisabled = computed(
  () => !canPreview.value || props.previewing || props.deleting || (props.preview !== null && props.preview.matched_count === 0),
)
const confirmDisabledReason = computed(() => {
  if (props.previewing || props.deleting) return ''
  if (!canPreview.value) return 'admin.promptAudit.events.filterDeleteConfirmInvalidRange'
  if (props.preview && props.preview.matched_count === 0) return 'admin.promptAudit.events.filterDeleteConfirmNoMatches'
  return ''
})

function criteriaChanged() {
  emit('criteria-change')
}
function requestPreview() {
  if (!canPreview.value) return
  emit('preview', resolveDeleteRangeFilters(local, preset.value))
}
function requestConfirm() {
  if (confirmDisabled.value) return
  emit('confirm', resolveDeleteRangeFilters(local, preset.value))
}
function formatDate(value: string): string {
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(value))
}
</script>
