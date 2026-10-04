<template>
  <!--
    清理删的就是用量页当前列出的这批：同样的时间范围（近 24 小时按精确时刻）、同样的筛选（含用户）。
    弹窗不再自带一套筛选，只把范围原样写出来、先数出条数，管理员看清「哪段时间、谁的、多少条」再删（2026-10-04 D8）。
  -->
  <BaseDialog :show="show" :title="t('admin.usage.cleanup.title')" width="wide" @close="handleClose">
    <div class="space-y-6">
      <section class="space-y-3" data-testid="usage-cleanup-scope">
        <p class="text-sm text-af-ink-2">{{ t('admin.usage.cleanup.scopeIntro') }}</p>
        <dl class="grid grid-cols-[auto_1fr] gap-x-6 gap-y-2 text-sm">
          <dt class="text-af-ink-3">{{ t('admin.usage.cleanup.range') }}</dt>
          <dd class="tabular-nums text-af-ink">{{ rangeText }}</dd>
          <template v-for="condition in conditions" :key="condition.label">
            <dt class="text-af-ink-3">{{ condition.label }}</dt>
            <dd class="break-all text-af-ink">{{ condition.value }}</dd>
          </template>
        </dl>
        <p v-if="conditions.length === 0" class="text-13 text-af-ink-3">{{ t('admin.usage.cleanup.noOtherConditions') }}</p>

        <p class="text-sm" aria-live="polite" data-testid="usage-cleanup-count">
          <span v-if="previewLoading" class="text-af-ink-3">{{ t('admin.usage.cleanup.counting') }}</span>
          <span v-else-if="previewError" class="text-af-danger">{{ previewError }}</span>
          <span v-else-if="previewCount === 0" class="text-af-ink-2">{{ t('admin.usage.cleanup.nothingToDelete') }}</span>
          <span v-else-if="previewCount !== null" class="text-af-ink-2">
            <i18n-t keypath="admin.usage.cleanup.willDelete" tag="span">
              <template #count>
                <span class="font-semibold tabular-nums text-af-ink">{{ previewCount.toLocaleString() }}</span>
              </template>
            </i18n-t>
          </span>
        </p>
        <FormError :message="submitError" />
        <p v-if="submitted" class="text-sm text-af-ink-2">{{ t('admin.usage.cleanup.submitted') }}</p>
      </section>

      <div class="rounded-xl border border-af-hairline p-4">
        <div class="flex items-center justify-between">
          <h4 class="text-sm font-semibold text-af-ink-2">
            {{ t('admin.usage.cleanup.recentTasks') }}
          </h4>
          <button type="button" class="btn btn-ghost btn-sm" @click="loadTasks">
            {{ t('common.refresh') }}
          </button>
        </div>

        <div class="mt-3 space-y-2">
          <FormError :message="tasksError" />
          <div v-if="tasksLoading" class="text-sm text-af-ink-3">
            {{ t('admin.usage.cleanup.loadingTasks') }}
          </div>
          <div v-else-if="tasks.length === 0" class="text-sm text-af-ink-3">
            {{ t('admin.usage.cleanup.noTasks') }}
          </div>
          <div v-else class="space-y-2">
            <div
              v-for="task in tasks"
              :key="task.id"
              class="flex flex-col gap-2 rounded-lg border border-af-hairline px-3 py-2 text-sm text-af-ink-2"
            >
              <div class="flex flex-wrap items-center justify-between gap-2">
                <div class="flex items-center gap-2">
                  <span :class="statusClass(task.status)" class="rounded-full px-2 py-0.5 text-xs font-semibold">
                    {{ statusLabel(task.status) }}
                  </span>
                  <button
                    v-if="canCancel(task)"
                    type="button"
                    class="btn btn-ghost btn-xs text-af-danger hover:text-af-danger"
                    @click="openCancelConfirm(task)"
                  >
                    {{ t('admin.usage.cleanup.cancel') }}
                  </button>
                </div>
                <div class="text-xs text-af-ink-3">
                  {{ formatDateTime(task.created_at) }}
                </div>
              </div>
              <div class="flex flex-wrap items-center gap-4 text-xs text-af-ink-3">
                <span>{{ t('admin.usage.cleanup.range') }}: {{ formatRange(task) }}</span>
                <span>{{ t('admin.usage.cleanup.deletedRows') }}: {{ task.deleted_rows.toLocaleString() }}</span>
              </div>
              <div v-if="task.error_message" class="text-xs text-af-danger">
                {{ task.error_message }}
              </div>
            </div>
          </div>
        </div>

        <Pagination
          v-if="tasksTotal > tasksPageSize"
          class="mt-4"
          :total="tasksTotal"
          :page="tasksPage"
          :page-size="tasksPageSize"
          :page-size-options="[5]"
          :show-page-size-selector="false"
          :show-jump="true"
          @update:page="handleTaskPageChange"
          @update:pageSize="handleTaskPageSizeChange"
        />
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="handleClose">
          {{ submitted ? t('common.close') : t('common.cancel') }}
        </button>
        <button
          type="button"
          class="btn btn-danger"
          :disabled="!canSubmit"
          data-testid="usage-cleanup-submit"
          @click="submitCleanup"
        >
          {{ submitLabel }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <ConfirmDialog
    :show="cancelConfirmVisible"
    :title="t('admin.usage.cleanup.cancelConfirmTitle')"
    :message="t('admin.usage.cleanup.cancelConfirmMessage')"
    :confirm-text="t('admin.usage.cleanup.cancelConfirm')"
    danger
    @confirm="cancelTask"
    @cancel="cancelConfirmVisible = false"
  />
</template>

<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import FormError from '@/components/common/FormError.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import { adminUsageAPI } from '@/api/admin/usage'
import type { UsageCleanupRequest, UsageCleanupTask } from '@/api/admin/usage'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatDateTimeToMinute } from '@/utils/format'

interface Props {
  show: boolean
  /** 用量页当前的时间范围与筛选（打开弹窗时定格）：显示、预览计数、最后删除都用这一份 */
  request: UsageCleanupRequest | null
  /** 时间范围以外的生效条件，按界面上的名字写（UsageFilters.describeUsageConditions） */
  conditions: Array<{ label: string; value: string }>
}

const props = defineProps<Props>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submitted'): void
}>()

const { t } = useI18n()

const tasks = ref<UsageCleanupTask[]>([])
const tasksLoading = ref(false)
const tasksPage = ref(1)
const tasksPageSize = ref(5)
const tasksTotal = ref(0)
const previewCount = ref<number | null>(null)
const previewLoading = ref(false)
const previewError = ref('')
const submitting = ref(false)
const submitted = ref(false)
const submitError = ref('')
// 任务列表加载 / 停止任务的失败显示在任务列表上方
const tasksError = ref('')
const cancelConfirmVisible = ref(false)
const canceling = ref(false)
const cancelTarget = ref<UsageCleanupTask | null>(null)
let pollTimer: number | null = null
let previewSeq = 0

/** 近 24 小时写到分钟；按天的范围写日期（含首尾两天） */
const rangeText = computed(() => {
  const request = props.request
  if (!request) return ''
  if (request.start_time && request.end_time) {
    return t('admin.usage.cleanup.timeRange', {
      start: formatDateTimeToMinute(request.start_time),
      end: formatDateTimeToMinute(request.end_time)
    })
  }
  return t('admin.usage.cleanup.dayRange', { start: request.start_date, end: request.end_date })
})

const submitLabel = computed(() => {
  if (submitting.value) return t('admin.usage.cleanup.submitting')
  const count = previewCount.value ?? 0
  return count > 0 ? t('admin.usage.cleanup.deleteCount', { count: count.toLocaleString() }) : t('admin.usage.cleanup.delete')
})

const canSubmit = computed(() =>
  !previewLoading.value && !previewError.value && (previewCount.value ?? 0) > 0 && !submitting.value && !submitted.value
)

const errorMessage = (error: unknown, fallbackKey: string) =>
  extractI18nErrorMessage(error, t, 'admin.usage.cleanup.errors', t(fallbackKey))

const loadPreview = async () => {
  const request = props.request
  if (!request) return
  const seq = ++previewSeq
  previewLoading.value = true
  previewError.value = ''
  previewCount.value = null
  try {
    const res = await adminUsageAPI.previewCleanupTask(request)
    if (seq !== previewSeq) return
    previewCount.value = res.count
  } catch (error) {
    if (seq !== previewSeq) return
    previewError.value = errorMessage(error, 'admin.usage.cleanup.countFailed')
  } finally {
    if (seq === previewSeq) previewLoading.value = false
  }
}

const startPolling = () => {
  stopPolling()
  pollTimer = window.setInterval(() => {
    loadTasks()
  }, 10000)
}

const stopPolling = () => {
  if (pollTimer !== null) {
    window.clearInterval(pollTimer)
    pollTimer = null
  }
}

const handleClose = () => {
  stopPolling()
  cancelConfirmVisible.value = false
  canceling.value = false
  cancelTarget.value = null
  emit('close')
}
const statusLabel = (status: string) => {
  const map: Record<string, string> = {
    pending: t('admin.usage.cleanup.status.pending'),
    running: t('admin.usage.cleanup.status.running'),
    succeeded: t('admin.usage.cleanup.status.succeeded'),
    failed: t('admin.usage.cleanup.status.failed'),
    canceled: t('admin.usage.cleanup.status.canceled')
  }
  return map[status] || status
}

const statusClass = (status: string) => {
  const map: Record<string, string> = {
    pending: 'bg-af-warning-tint text-af-warning',
    running: 'bg-af-sunken text-af-ink-2',
    succeeded: 'bg-af-success-tint text-af-success',
    failed: 'bg-af-danger-tint text-af-danger',
    canceled: 'bg-af-hairline text-af-ink-2'
  }
  return map[status] || 'bg-af-sunken text-af-ink-2'
}

const formatDateTime = (value?: string | null) => {
  if (!value) return '--'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

const formatRange = (task: UsageCleanupTask) => {
  const start = formatDateTime(task.filters.start_time)
  const end = formatDateTime(task.filters.end_time)
  return `${start} ~ ${end}`
}

const loadTasks = async () => {
  if (!props.show) return
  tasksLoading.value = true
  tasksError.value = ''
  try {
    const res = await adminUsageAPI.listCleanupTasks({
      page: tasksPage.value,
      page_size: tasksPageSize.value
    })
    tasks.value = res.items || []
    tasksTotal.value = res.total || 0
    if (res.page) {
      tasksPage.value = res.page
    }
    if (res.page_size) {
      tasksPageSize.value = res.page_size
    }
  } catch (error) {
    tasksError.value = errorMessage(error, 'admin.usage.cleanup.loadFailed')
  } finally {
    tasksLoading.value = false
  }
}

const handleTaskPageChange = (page: number) => {
  tasksPage.value = page
  loadTasks()
}

const handleTaskPageSizeChange = (size: number) => {
  if (!Number.isFinite(size) || size <= 0) return
  tasksPageSize.value = size
  tasksPage.value = 1
  loadTasks()
}

const canCancel = (task: UsageCleanupTask) => {
  return task.status === 'pending' || task.status === 'running'
}

const openCancelConfirm = (task: UsageCleanupTask) => {
  cancelTarget.value = task
  cancelConfirmVisible.value = true
}

const submitCleanup = async () => {
  const request = props.request
  if (!request || !canSubmit.value) return
  submitting.value = true
  submitError.value = ''
  try {
    await adminUsageAPI.createCleanupTask(request)
    submitted.value = true
    emit('submitted')
    loadTasks()
  } catch (error) {
    submitError.value = errorMessage(error, 'admin.usage.cleanup.submitFailed')
  } finally {
    submitting.value = false
  }
}

const cancelTask = async () => {
  const task = cancelTarget.value
  if (!task) {
    cancelConfirmVisible.value = false
    return
  }
  canceling.value = true
  cancelConfirmVisible.value = false
  tasksError.value = ''
  try {
    await adminUsageAPI.cancelCleanupTask(task.id)
    loadTasks()
  } catch (error) {
    tasksError.value = errorMessage(error, 'admin.usage.cleanup.cancelFailed')
  } finally {
    canceling.value = false
    cancelTarget.value = null
  }
}

watch(
  () => props.show,
  (show) => {
    if (show) {
      tasksPage.value = 1
      tasksTotal.value = 0
      submitted.value = false
      submitError.value = ''
      loadPreview()
      loadTasks()
      startPolling()
    } else {
      stopPolling()
    }
  }
)

onUnmounted(() => {
  stopPolling()
})
</script>
