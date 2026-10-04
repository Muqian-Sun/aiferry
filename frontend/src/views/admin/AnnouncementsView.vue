<template>
  <!--
    公告（A4 列表模板）：标题右侧「创建公告」；工具行 = 搜索 + 状态筛选标签 + 刷新；
    行尾「编辑」图标 +「⋯」（预览、已读情况、删除）。
  -->
  <AppLayout>
    <template #header-actions>
      <button type="button" class="btn btn-primary btn-md" @click="openCreateDialog">
        <Icon name="plus" size="md" />
        {{ t('admin.announcements.createAnnouncement') }}
      </button>
    </template>

    <TablePageLayout>
      <template #filters>
        <ListToolbar>
          <SearchInput
            v-model="searchQuery"
            compact
            class="w-full sm:w-64"
            :placeholder="t('admin.announcements.searchAnnouncements')"
            @update:model-value="handleSearch"
          />
          <FilterChip
            v-model="filters.status"
            :label="t('admin.announcements.columns.status')"
            :options="statusOptions"
            test-id="filter-status"
            @change="handleStatusChange"
          />

          <template #end>
            <button
              type="button"
              class="rounded-md p-2 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink disabled:opacity-40"
              :disabled="loading"
              :title="t('common.refresh')"
              :aria-label="t('common.refresh')"
              @click="loadAnnouncements"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
          </template>
        </ListToolbar>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="announcements"
          :loading="loading"
          :server-side-sort="true"
          default-sort-key="created_at"
          default-sort-order="desc"
          @sort="handleSort"
        >
          <template #cell-title="{ value }">
            <div class="min-w-0 max-w-md">
              <div class="truncate font-medium text-af-ink" :title="value">{{ value }}</div>
            </div>
          </template>

          <!-- 状态：展示中是常态（灰点）；草稿 / 已归档不在展示，淡色空心点 -->
          <template #cell-status="{ value }">
            <div class="flex items-center gap-1.5">
              <span
                :class="[
                  'inline-block h-2 w-2 rounded-full',
                  value === 'active' ? 'bg-af-ink-4' : 'border border-af-ink-4'
                ]"
              ></span>
              <span :class="value === 'active' ? 'text-af-ink-2' : 'text-af-ink-3'">{{ statusLabel(value) }}</span>
            </div>
          </template>

          <template #cell-targeting="{ row }">
            <span class="text-af-ink-2">{{ targetingSummary(row.targeting) }}</span>
          </template>

          <!-- 有效期：一行「开始 → 结束」，精确到分钟（维护通知这类公告的时刻有意义） -->
          <template #cell-timeRange="{ row }">
            <span class="tabular-nums text-af-ink-2">
              {{ row.starts_at ? formatDateTimeToMinute(row.starts_at) : t('admin.announcements.timeImmediate') }}
              <span class="mx-1 text-af-ink-4">→</span>
              {{ row.ends_at ? formatDateTimeToMinute(row.ends_at) : t('admin.announcements.timeNever') }}
            </span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="tabular-nums text-af-ink-3" :title="formatDateTime(value)">{{ formatDateOnly(value) }}</span>
          </template>

          <template #cell-actions="{ row }">
            <RowActions :actions="rowActions(row)" />
          </template>

          <template #empty>
            <EmptyState
              :filtered="hasActiveFilters"
              :title="t('empty.noData')"
              :description="t('admin.announcements.createFirstAnnouncement')"
              :action-text="t('admin.announcements.createAnnouncement')"
              @action="openCreateDialog"
            />
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <!-- Create/Edit Dialog -->
    <BaseDialog
      :show="showEditDialog"
      :title="isEditing ? t('admin.announcements.editAnnouncement') : t('admin.announcements.createAnnouncement')"
      width="wide"
      @close="closeEdit"
    >
      <form id="announcement-form" @submit.prevent="handleSave" class="space-y-4">
        <div>
          <label class="input-label">{{ t('admin.announcements.form.title') }}</label>
          <input v-model="form.title" type="text" class="input" required />
        </div>

        <div>
          <label class="input-label">{{ t('admin.announcements.form.content') }}</label>
          <textarea v-model="form.content" rows="6" class="input" required></textarea>
        </div>

        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.announcements.form.status') }}</label>
            <Select v-model="form.status" :options="statusOptions" />
          </div>
        </div>

        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.announcements.form.startsAt') }}</label>
            <input v-model="form.starts_at_str" type="datetime-local" max="9999-12-31T23:59" class="input" />
            <p class="input-hint">{{ t('admin.announcements.form.startsAtHint') }}</p>
          </div>
          <div>
            <label class="input-label">{{ t('admin.announcements.form.endsAt') }}</label>
            <input v-model="form.ends_at_str" type="datetime-local" max="9999-12-31T23:59" class="input" />
            <p class="input-hint">{{ t('admin.announcements.form.endsAtHint') }}</p>
          </div>
        </div>

        <AnnouncementTargetingEditor
          v-model="form.targeting"
          :plans="subscriptionPlans"
        />
      </form>

      <template #footer>
        <div class="flex w-full flex-wrap items-center justify-end gap-3">
          <FormError class="mr-auto min-w-0 flex-1" :message="saveError" />
          <button type="button" @click="closeEdit" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button type="submit" form="announcement-form" :disabled="saving" class="btn btn-primary">
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Delete Confirmation -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.announcements.deleteAnnouncement')"
      :message="t('admin.announcements.deleteConfirm', { title: deletingAnnouncement?.title ?? '' })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />

    <!-- Read Status Dialog -->
    <AnnouncementReadStatusDialog
      :show="showReadStatusDialog"
      :announcement-id="readStatusAnnouncementId"
      @close="showReadStatusDialog = false"
    />

    <AnnouncementPopup
      :announcement="previewAnnouncement"
      @close="previewAnnouncement = null"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { adminAPI } from '@/api/admin'
import {
  formatDateOnly,
  formatDateTime,
  formatDateTimeLocalInput,
  formatDateTimeToMinute,
  parseDateTimeLocalInput
} from '@/utils/format'
import type { Announcement, AnnouncementTargeting } from '@/types'
import type { SubscriptionPlan } from '@/types/payment'
import { adminPaymentAPI } from '@/api/admin/payment'
import { SITE_FEATURES } from '@/utils/siteFeatures'
import { extractI18nErrorMessage } from '@/utils/apiError'
import type { Column } from '@/components/common/types'

import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select from '@/components/common/Select.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import FormError from '@/components/common/FormError.vue'
import Icon from '@/components/icons/Icon.vue'
import { FilterChip, ListToolbar, RowActions } from '@/components/admin/list'
import type { RowAction } from '@/components/admin/list'

import AnnouncementTargetingEditor from '@/components/admin/announcements/AnnouncementTargetingEditor.vue'
import AnnouncementReadStatusDialog from '@/components/admin/announcements/AnnouncementReadStatusDialog.vue'
import AnnouncementPopup from '@/components/common/AnnouncementPopup.vue'

const { t } = useI18n()

const announcements = ref<Announcement[]>([])
const loading = ref(false)

const filters = reactive({
  status: '',
})
const searchQuery = ref('')
/** 带着搜索 / 状态筛选却没有结果时，空状态说「没有符合条件的」，不给「创建第一条」 */
const hasActiveFilters = computed(() => Boolean(searchQuery.value.trim() || filters.status))

const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})

const sortState = reactive({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc'
})

// 编辑表单的状态选项，也用作「状态」筛选标签的选项（筛选标签自带「全部」）
const statusOptions = computed(() => [
  { value: 'draft', label: t('admin.announcements.statusLabels.draft') },
  { value: 'active', label: t('admin.announcements.statusLabels.active') },
  { value: 'archived', label: t('admin.announcements.statusLabels.archived') }
])

const columns = computed<Column[]>(() => [
  { key: 'title', label: t('admin.announcements.columns.title'), sortable: true },
  { key: 'status', label: t('admin.announcements.columns.status'), sortable: true },
  { key: 'targeting', label: t('admin.announcements.columns.targeting') },
  { key: 'timeRange', label: t('admin.announcements.columns.timeRange') },
  { key: 'created_at', label: t('admin.announcements.columns.createdAt'), sortable: true },
  { key: 'actions', label: t('admin.announcements.columns.actions') }
])

const statusLabel = (status: string) => {
  if (status === 'draft') return t('admin.announcements.statusLabels.draft')
  if (status === 'active') return t('admin.announcements.statusLabels.active')
  if (status === 'archived') return t('admin.announcements.statusLabels.archived')
  return status
}

const targetingSummary = (targeting: AnnouncementTargeting) => {
  const anyOf = targeting?.any_of ?? []
  if (!anyOf || anyOf.length === 0) return t('admin.announcements.targetingSummaryAll')
  return t('admin.announcements.targetingSummaryCustom', { groups: anyOf.length })
}

// 行操作（A4）：编辑是图标；预览、已读情况、删除进「⋯」，删除红字
const rowActions = (row: Announcement): RowAction[] => [
  { key: 'edit', label: t('common.edit'), icon: 'edit', primary: true, onSelect: () => openEditDialog(row) },
  { key: 'preview', label: t('admin.announcements.preview'), icon: 'eye', onSelect: () => openPreview(row) },
  { key: 'read-status', label: t('admin.announcements.readStatus'), icon: 'chartBar', onSelect: () => openReadStatus(row) },
  { key: 'delete', label: t('common.delete'), icon: 'trash', danger: true, dividerBefore: true, onSelect: () => handleDelete(row) }
]

// ===== CRUD / list =====
let currentController: AbortController | null = null

async function loadAnnouncements() {
  currentController?.abort()
  const requestController = new AbortController()
  currentController = requestController
  const { signal } = requestController

  try {
    loading.value = true
    const res = await adminAPI.announcements.list(pagination.page, pagination.page_size, {
      status: filters.status || undefined,
      search: searchQuery.value || undefined,
      sort_by: sortState.sort_by,
      sort_order: sortState.sort_order
    }, { signal })

    if (signal.aborted || currentController !== requestController) return

    announcements.value = res.items
    pagination.total = res.total
    pagination.pages = res.pages
    pagination.page = res.page
    pagination.page_size = res.page_size
  } catch (error: any) {
    if (
      signal.aborted ||
      currentController !== requestController ||
      error?.name === 'AbortError' ||
      error?.code === 'ERR_CANCELED'
    ) {
      return
    }
    console.error('Error loading announcements:', error)
  } finally {
    if (currentController === requestController) {
      loading.value = false
      currentController = null
    }
  }
}

function handlePageChange(page: number) {
  pagination.page = page
  loadAnnouncements()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  loadAnnouncements()
}

function handleStatusChange() {
  pagination.page = 1
  loadAnnouncements()
}

function handleSort(key: string, order: 'asc' | 'desc') {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadAnnouncements()
}

let searchDebounceTimer: number | null = null
function handleSearch() {
  if (searchDebounceTimer) window.clearTimeout(searchDebounceTimer)
  searchDebounceTimer = window.setTimeout(() => {
    pagination.page = 1
    loadAnnouncements()
  }, 300)
}

// ===== Create/Edit dialog =====
const showEditDialog = ref(false)
const saving = ref(false)
// 保存失败（含后端校验，如开始时间晚于结束时间）显示在弹窗底部
const saveError = ref('')
const editingAnnouncement = ref<Announcement | null>(null)

const isEditing = computed(() => !!editingAnnouncement.value)

const form = reactive({
  title: '',
  content: '',
  status: 'draft',
  starts_at_str: '',
  ends_at_str: '',
  targeting: { any_of: [] } as AnnouncementTargeting
})

const subscriptionPlans = ref<SubscriptionPlan[]>([])

async function loadSubscriptionPlans() {
  try {
    const res = await adminPaymentAPI.getPlans()
    subscriptionPlans.value = res.data || []
  } catch (error: any) {
    console.error('Error loading plans:', error)
    // not fatal
  }
}

function resetForm() {
  form.title = ''
  form.content = ''
  form.status = 'draft'
  form.starts_at_str = ''
  form.ends_at_str = ''
  form.targeting = { any_of: [] }
}

function fillFormFromAnnouncement(a: Announcement) {
  form.title = a.title
  form.content = a.content
  form.status = a.status

  // Backend returns RFC3339 strings
  form.starts_at_str = a.starts_at ? formatDateTimeLocalInput(Math.floor(new Date(a.starts_at).getTime() / 1000)) : ''
  form.ends_at_str = a.ends_at ? formatDateTimeLocalInput(Math.floor(new Date(a.ends_at).getTime() / 1000)) : ''

  form.targeting = a.targeting ?? { any_of: [] }
}

function openCreateDialog() {
  editingAnnouncement.value = null
  resetForm()
  saveError.value = ''
  showEditDialog.value = true
}

function openEditDialog(row: Announcement) {
  editingAnnouncement.value = row
  fillFormFromAnnouncement(row)
  saveError.value = ''
  showEditDialog.value = true
}

function closeEdit() {
  showEditDialog.value = false
  editingAnnouncement.value = null
}

function buildCreatePayload() {
  const startsAt = parseDateTimeLocalInput(form.starts_at_str)
  const endsAt = parseDateTimeLocalInput(form.ends_at_str)

  return {
    title: form.title,
    content: form.content,
    status: form.status as any,
    targeting: form.targeting,
    starts_at: startsAt ?? undefined,
    ends_at: endsAt ?? undefined
  }
}

function buildUpdatePayload(original: Announcement) {
  const payload: any = {}

  if (form.title !== original.title) payload.title = form.title
  if (form.content !== original.content) payload.content = form.content
  if (form.status !== original.status) payload.status = form.status

  // starts_at / ends_at: distinguish unchanged vs clear(0) vs set
  const originalStarts = original.starts_at ? Math.floor(new Date(original.starts_at).getTime() / 1000) : null
  const originalEnds = original.ends_at ? Math.floor(new Date(original.ends_at).getTime() / 1000) : null

  const newStarts = parseDateTimeLocalInput(form.starts_at_str)
  const newEnds = parseDateTimeLocalInput(form.ends_at_str)

  if (newStarts !== originalStarts) {
    payload.starts_at = newStarts === null ? 0 : newStarts
  }
  if (newEnds !== originalEnds) {
    payload.ends_at = newEnds === null ? 0 : newEnds
  }

  // targeting: do shallow compare by JSON
  if (JSON.stringify(form.targeting ?? {}) !== JSON.stringify(original.targeting ?? {})) {
    payload.targeting = form.targeting
  }

  return payload
}

async function handleSave() {
  saveError.value = ''
  const failedKey = editingAnnouncement.value ? 'admin.announcements.failedToUpdate' : 'admin.announcements.failedToCreate'
  // Frontend validation for targeting (to avoid ANNOUNCEMENT_INVALID_TARGET)
  const anyOf = form.targeting?.any_of ?? []
  if (anyOf.length > 50) {
    saveError.value = t(failedKey)
    return
  }
  for (const g of anyOf) {
    const allOf = g?.all_of ?? []
    if (allOf.length > 50) {
      saveError.value = t(failedKey)
      return
    }
  }

  saving.value = true
  try {
    if (!editingAnnouncement.value) {
      const payload = buildCreatePayload()
      await adminAPI.announcements.create(payload)
      showEditDialog.value = false
      await loadAnnouncements()
      return
    }

    const original = editingAnnouncement.value
    const payload = buildUpdatePayload(original)
    await adminAPI.announcements.update(original.id, payload)
    showEditDialog.value = false
    editingAnnouncement.value = null
    await loadAnnouncements()
  } catch (error: unknown) {
    saveError.value = extractI18nErrorMessage(error, t, 'admin.announcements.errors', t(failedKey))
    console.error('Failed to save announcement:', error)
  } finally {
    saving.value = false
  }
}

// ===== Delete =====
const showDeleteDialog = ref(false)
const deletingAnnouncement = ref<Announcement | null>(null)

function handleDelete(row: Announcement) {
  deletingAnnouncement.value = row
  showDeleteDialog.value = true
}

async function confirmDelete() {
  if (!deletingAnnouncement.value) return

  try {
    await adminAPI.announcements.delete(deletingAnnouncement.value.id)
    showDeleteDialog.value = false
    deletingAnnouncement.value = null
    await loadAnnouncements()
  } catch (error: any) {
    console.error('Failed to delete announcement:', error)
  }
}

// ===== Read status =====
const showReadStatusDialog = ref(false)
const readStatusAnnouncementId = ref<number | null>(null)
const previewAnnouncement = ref<Announcement | null>(null)

function openPreview(row: Announcement) {
  previewAnnouncement.value = row
}

function openReadStatus(row: Announcement) {
  readStatusAnnouncementId.value = row.id
  showReadStatusDialog.value = true
}

onMounted(async () => {
  if (SITE_FEATURES.subscription) await loadSubscriptionPlans()
  await loadAnnouncements()
})

onUnmounted(() => {
  if (searchDebounceTimer) window.clearTimeout(searchDebounceTimer)
  currentController?.abort()
})
</script>
