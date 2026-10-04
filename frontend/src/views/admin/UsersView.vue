<template>
  <!--
    用户（A4 列表模板）：标题右侧是工具菜单与「创建用户」；数字摘要；工具行 = 搜索 + 筛选标签 + 刷新 / 列设置；
    默认列（管理站瘦身方案）：用户、余额、计费倍率、近 30 天消费、状态、最近活跃，其余进列设置。
    「活跃」在这页只有一个意思：调用过 API（摘要「今日活跃」、列「最近活跃」= last_used_at）；
    登录 / 打开控制台记在 last_active_at，列名写「最近访问控制台」，默认不显示。
    行尾「编辑」图标 + 「⋯」（充值、扣减余额、余额流水、API 密钥、禁用、删除）；选中行时出现批量条。
    点行打开详情抽屉（A5）：概况 / 余额流水 / API 密钥 / 订阅（订阅功能开着才有）/ 用量；
    「余额流水」「API 密钥」和余额数字都直接开到对应页签。
  -->
  <AppLayout>
    <template #header-actions>
      <PopoverMenu width-class="w-48">
        <template #trigger="{ open }">
          <button
            type="button"
            class="btn btn-ghost btn-md px-2.5"
            :class="open ? 'bg-af-sunken text-af-ink' : ''"
            :title="t('common.more')"
            :aria-label="t('common.more')"
            data-testid="users-tools"
          >
            <Icon name="more" size="md" />
          </button>
        </template>
        <MenuItem icon="cog" @click="showAttributesModal = true">
          {{ t('admin.users.attributes.configButton') }}
        </MenuItem>
      </PopoverMenu>
      <button @click="showCreateModal = true" class="btn btn-primary btn-md">
        <Icon name="plus" size="md" />
        {{ t('admin.users.createUser') }}
      </button>
    </template>

    <TablePageLayout>
      <template v-if="summaryItems" #summary>
        <StatRow :items="summaryItems" data-testid="users-summary" />
      </template>

      <template #filters>
        <ListToolbar>
          <SearchInput
            v-model="searchQuery"
            compact
            class="w-full sm:w-64"
            :placeholder="t('admin.users.searchUsers')"
            @update:model-value="handleSearch"
          />
          <FilterChip
            v-model="filters.role"
            :label="t('admin.users.columns.role')"
            :options="[
              { value: 'admin', label: t('admin.users.admin') },
              { value: 'user', label: t('admin.users.user') }
            ]"
            test-id="filter-role"
            @change="applyFilter"
          />
          <FilterChip
            v-model="filters.status"
            :label="t('admin.users.columns.status')"
            :options="[
              { value: 'active', label: t('common.active') },
              { value: 'disabled', label: t('admin.users.disabled') }
            ]"
            test-id="filter-status"
            @change="applyFilter"
          />

          <!-- 自定义属性筛选：从「+ 属性」里挑出来的才显示；单选 / 多选型和角色、状态一样用筛选标签，其他型是输入框（回车生效） -->
          <template v-for="(value, attrId) in activeAttributeFilters" :key="attrId">
            <template v-if="visibleFilters.has(`attr_${attrId}`)">
              <FilterChip
                v-if="['select', 'multi_select'].includes(getAttributeDefinition(Number(attrId))?.type || '')"
                :model-value="value"
                :label="getAttributeDefinitionName(Number(attrId))"
                :options="getAttributeDefinition(Number(attrId))?.options || []"
                :test-id="`filter-attr-${attrId}`"
                @update:model-value="(val) => updateAttributeFilter(Number(attrId), String(val))"
                @change="applyFilter"
              />
              <input
                v-else
                :value="value"
                :type="getAttributeDefinition(Number(attrId))?.type === 'number' ? 'number' : 'text'"
                :placeholder="getAttributeDefinitionName(Number(attrId))"
                class="input h-8 w-full py-0 text-13 sm:w-36"
                @input="(e) => updateAttributeFilter(Number(attrId), (e.target as HTMLInputElement).value)"
                @keyup.enter="applyFilter"
              />
            </template>
          </template>
          <PopoverMenu v-if="filterableAttributes.length > 0" align="start" width-class="w-52" :close-on-select="false">
            <template #trigger>
              <button
                type="button"
                class="inline-flex h-8 items-center gap-1 rounded-full px-2.5 text-13 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink"
                data-testid="filter-attributes"
              >
                <Icon name="plus" size="xs" :stroke-width="2" />
                {{ t('admin.users.attributes.filterButton') }}
              </button>
            </template>
            <MenuItem
              v-for="attr in filterableAttributes"
              :key="attr.id"
              :checked="visibleFilters.has(`attr_${attr.id}`)"
              @click="toggleAttributeFilter(attr)"
            >
              {{ attr.name }}
            </MenuItem>
          </PopoverMenu>

          <template #end>
            <button
              type="button"
              class="rounded-md p-2 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink disabled:opacity-40"
              :disabled="loading"
              :title="t('common.refresh')"
              :aria-label="t('common.refresh')"
              @click="loadUsers"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <ColumnSettingsMenu :settings="columnSettings" />
          </template>
        </ListToolbar>
        <!-- 批量删除部分失败的结果，原来只打控制台 -->
        <FormError class="mt-2" :message="bulkDeleteError" data-testid="users-bulk-delete-error" />
      </template>

      <!-- Users Table -->
      <template #table>
        <DataTable
          :columns="columns"
          :data="sortedUsers"
          :loading="loading"
          row-key="id"
          selectable
          :selected-keys="selectedIds"
          :selection-label="getUserSelectionLabel"
          :server-side-sort="true"
          default-sort-key="created_at"
          default-sort-order="desc"
          :sort-storage-key="USER_SORT_STORAGE_KEY"
          clickable-rows
          @sort="handleSort"
          @update:selected-keys="handleSelectedKeysUpdate"
          @row-click="openDrawer($event)"
        >
          <template #cell-email="{ value, row }">
            <div class="min-w-0">
              <div class="truncate font-medium text-af-ink">{{ value }}</div>
              <div v-if="row.username" class="truncate text-xs text-af-ink-3">{{ row.username }}</div>
            </div>
          </template>

          <template #cell-username="{ value }">
            <span class="text-sm text-af-ink-2">{{ value || '-' }}</span>
          </template>

          <template #cell-notes="{ value }">
            <div class="max-w-xs">
              <span
                v-if="value"
                :title="value.length > 30 ? value : undefined"
                class="block truncate text-sm text-af-ink-2"
              >
                {{ value.length > 30 ? value.substring(0, 25) + '...' : value }}
              </span>
              <span v-else class="text-sm text-af-ink-3">-</span>
            </div>
          </template>

          <!-- Dynamic attribute columns -->
          <template
            v-for="def in attributeDefinitions.filter(d => d.enabled)"
            :key="def.id"
            #[`cell-attr_${def.id}`]="{ row }"
          >
            <div class="max-w-xs">
              <span
                class="block truncate text-sm text-af-ink-2"
                :title="getAttributeValue(row.id, def.id)"
              >
                {{ getAttributeValue(row.id, def.id) }}
              </span>
            </div>
          </template>

          <template #cell-role="{ value }">
            <span :class="value === 'admin' ? 'font-medium text-af-ink' : 'text-af-ink-2'">
              {{ t('admin.users.roles.' + value) }}
            </span>
          </template>

          <!-- 订阅列只在订阅功能开着时进列表（SITE_FEATURES.subscription） -->
          <template #cell-subscriptions="{ row }">
            <div
              v-if="row.subscriptions && row.subscriptions.length > 0"
              class="flex flex-wrap gap-1.5"
            >
              <span
                v-for="sub in row.subscriptions"
                :key="sub.id"
                class="inline-flex items-center gap-1.5 rounded-md bg-af-sunken px-2 py-0.5 text-xs font-medium text-af-ink-2"
                :title="sub.expires_at ? formatDateTime(sub.expires_at) : ''"
              >
                <span class="truncate">{{ sub.plan?.name || t('common.deletedPlan') }}</span>
                <span v-if="sub.expires_at" :class="subscriptionDaysClass(getDaysRemaining(sub.expires_at))">
                  {{ subscriptionDaysLabel(getDaysRemaining(sub.expires_at)) }}
                </span>
              </span>
            </div>
            <span
              v-else
              class="inline-flex items-center gap-1.5 rounded-md bg-af-sunken px-2 py-1 text-xs text-af-ink-3"
            >
              <Icon name="ban" size="xs" class="h-3.5 w-3.5" />
              <span>{{ t('admin.users.noSubscription') }}</span>
            </span>
          </template>

          <template #cell-balance="{ value, row }">
            <button
              type="button"
              class="font-medium tabular-nums underline decoration-dashed decoration-af-ink-4 underline-offset-4 transition-colors hover:text-af-brand-hover"
              :class="balanceTextClass(value)"
              :title="t('admin.users.balanceHistoryTip')"
              @click.stop="openDrawer(row, 'balance')"
            >
              {{ formatBalance(value) }}
            </button>
          </template>

          <!-- 「近 30 天消费」表头：用量是列表出来后另取的、只有本页，所以排序在前端做，只排本页。
               点一下降序 → 再点升序 → 再点取消；指示三角和 DataTable 自带的排序列一个样子。 -->
          <template #header-usage="{ column }">
            <button
              type="button"
              class="inline-flex items-center gap-1 transition-colors hover:text-af-ink"
              :class="usageSort ? 'text-af-ink' : ''"
              :title="t('admin.users.usageSortHint')"
              :aria-label="t('admin.users.usageSortHint')"
              data-test="usage-sort-trigger-usage"
              @click.stop="toggleUsageSort"
            >
              <span>{{ column.label }}</span>
              <span class="inline-flex h-5 w-4 flex-col items-center justify-center" aria-hidden="true">
                <svg
                  class="h-2.5 w-2.5"
                  :class="usageSort === 'asc' ? 'text-af-brand' : 'text-af-ink-3'"
                  fill="currentColor"
                  viewBox="0 0 10 10"
                >
                  <path d="M5 2L1.5 6.5h7L5 2z" />
                </svg>
                <svg
                  class="-mt-0.5 h-2.5 w-2.5"
                  :class="usageSort === 'desc' ? 'text-af-brand' : 'text-af-ink-3'"
                  fill="currentColor"
                  viewBox="0 0 10 10"
                >
                  <path d="M5 8L1.5 3.5h7L5 8z" />
                </svg>
              </span>
            </button>
          </template>

          <!-- 近 30 天消费：收入口径（actual_cost），两位小数 -->
          <template #cell-usage="{ row }">
            <PlatformUsageBreakdown
              :total="usageStats[row.id]?.total_actual_cost ?? 0"
              :by-platform="usageStats[row.id]?.by_platform"
            />
          </template>

          <template #cell-concurrency="{ row }">
            <UserConcurrencyCell
              :current="row.current_concurrency ?? 0"
              :max="row.concurrency"
            />
          </template>

          <template #cell-rate_multiplier="{ row }">
            <span v-if="row.custom_rate_multiplier != null" class="tabular-nums text-af-ink">{{ formatMultiplier(row.rate_multiplier) }}</span>
            <span v-else class="text-af-ink-3">{{ t('admin.users.form.rateMultiplierDefault') }}</span>
          </template>

          <template #cell-status="{ value }">
            <div class="flex items-center gap-1.5">
              <span
                :class="[
                  'inline-block h-2 w-2 rounded-full',
                  value === 'active' ? 'bg-af-ink-4' : 'bg-af-danger'
                ]"
              ></span>
              <span :class="value === 'active' ? 'text-af-ink-2' : 'text-af-danger'">
                {{ value === 'active' ? t('common.active') : t('admin.users.disabled') }}
              </span>
            </div>
          </template>

          <!-- 时间列：最近活跃（调用 API）/ 最近访问控制台写相对时间，悬停看精确时间；创建时间只写日期 -->
          <template #cell-created_at="{ value }">
            <span class="tabular-nums text-af-ink-3" :title="formatDateTime(value)">{{ formatDateOnly(value) }}</span>
          </template>

          <template #header-last_used_at="{ column }">
            <span :title="t('admin.users.columns.lastUsedHint')">{{ column.label }}</span>
          </template>

          <template #cell-last_used_at="{ value }">
            <span v-if="value" class="text-af-ink-2" :title="formatDateTime(value)">{{ formatRelativeTime(value) }}</span>
            <span v-else class="text-af-ink-3">-</span>
          </template>

          <template #cell-last_active_at="{ value }">
            <span v-if="value" class="text-af-ink-2" :title="formatDateTime(value)">{{ formatRelativeTime(value) }}</span>
            <span v-else class="text-af-ink-3">-</span>
          </template>

          <template #cell-actions="{ row }">
            <RowActions :actions="rowActions(row)" />
          </template>

          <template #empty>
            <EmptyState
              :filtered="hasActiveFilters"
              :title="t('admin.users.noUsersYet')"
              :description="t('admin.users.createFirstUser')"
            />
          </template>
        </DataTable>
      </template>

      <template #bulk>
        <BulkBar :count="selectedCount" @clear="clearSelection">
          <button type="button" class="bulk-btn" data-test="bulk-edit-limits" @click="showBulkEditModal = true">
            {{ t('admin.users.bulkLimits.button') }}
          </button>
          <button
            type="button"
            class="bulk-btn bulk-btn-danger"
            data-test="bulk-delete-users"
            :disabled="bulkDeleting"
            @click="bulkDeleteIds = [...selectedIds]"
          >
            {{ t('common.delete') }}
          </button>
        </BulkBar>
      </template>

      <!-- Pagination -->
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

    <ConfirmDialog :show="showDeleteDialog" :title="t('admin.users.deleteUser')" :message="t('admin.users.deleteConfirm', { email: deletingUser?.email })" :confirm-text="t('common.delete')" :danger="true" @confirm="confirmDelete" @cancel="showDeleteDialog = false">
      <FormError :message="deleteError" />
    </ConfirmDialog>
    <ConfirmDialog
      :show="bulkDeleteIds.length > 0"
      :title="t('admin.users.bulkDelete.title')"
      :message="t('admin.users.bulkDelete.confirm', { count: bulkDeleteIds.length })"
      :confirm-text="t('common.delete')"
      danger
      @confirm="confirmBulkDelete"
      @cancel="bulkDeleteIds = []"
    />
    <UserDetailDrawer
      :show="drawerOpen"
      :user="drawerUser"
      v-model:tab="drawerTab"
      :attribute-definitions="filterableAttributes"
      :refresh-key="drawerRefreshKey"
      @close="drawerOpen = false"
      @edit="drawerUser && handleEdit(drawerUser)"
      @deposit="drawerUser && handleDeposit(drawerUser)"
      @withdraw="drawerUser && handleWithdraw(drawerUser)"
      @toggle-status="drawerUser && handleToggleStatus(drawerUser)"
      @delete="drawerUser && handleDelete(drawerUser)"
    />
    <UserCreateModal :show="showCreateModal" @close="showCreateModal = false" @success="loadUsers" />
    <UserEditModal
      :show="showEditModal"
      :user="editingUser"
      @close="closeEditModal"
      @success="handleUserMutated"
      @adjust-balance="handleEditAdjustBalance"
    />
    <BulkEditUserModal
      :show="showBulkEditModal"
      :selected-ids="selectedIds"
      @close="showBulkEditModal = false"
      @success="handleBulkLimitsSuccess"
    />
    <UserBalanceModal :show="showBalanceModal" :user="balanceUser" :operation="balanceOperation" @close="closeBalanceModal" @success="handleBalanceUpdated" />
    <!-- 禁用前先确认（启用不用确认）；报错显示在确认框里 -->
    <ConfirmDialog
      :show="disablingUser !== null"
      :title="t('admin.users.disableTitle')"
      :message="t('admin.users.disableConfirm', { email: disablingUser?.email ?? '' })"
      :confirm-text="t('admin.users.disable')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="confirmDisable"
      @cancel="disablingUser = null"
    >
      <FormError class="mt-2" :message="toggleError" />
    </ConfirmDialog>
    <UserAttributesConfigModal :show="showAttributesModal" @close="handleAttributesModalClose" />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useTableSelection } from '@/composables/useTableSelection'
import { formatDateOnly, formatDateTime, formatRelativeTime } from '@/utils/format'
import { balanceTextClass, formatBalance } from '@/utils/money'
import { SITE_FEATURES } from '@/utils/siteFeatures'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
import { adminAPI } from '@/api/admin'
import type { AdminUser, UserAttributeDefinition } from '@/types'
import type { BatchUserUsageStats } from '@/api/admin/dashboard'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import FormError from '@/components/common/FormError.vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import EmptyState from '@/components/common/EmptyState.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import StatRow from '@/components/user/shell/StatRow.vue'
import type { StatItem } from '@/components/user/shell/types'
import { BulkBar, ColumnSettingsMenu, FilterChip, ListToolbar, MenuItem, PopoverMenu, RowActions } from '@/components/admin/list'
import type { RowAction } from '@/components/admin/list'
import { useColumnSettings } from '@/composables/useColumnSettings'
import type { DashboardStats } from '@/types'
import UserAttributesConfigModal from '@/components/user/UserAttributesConfigModal.vue'
import UserConcurrencyCell from '@/components/user/UserConcurrencyCell.vue'
import PlatformUsageBreakdown from '@/components/user/PlatformUsageBreakdown.vue'
import UserCreateModal from '@/components/admin/user/UserCreateModal.vue'
import UserEditModal from '@/components/admin/user/UserEditModal.vue'
import BulkEditUserModal from '@/components/admin/user/BulkEditUserModal.vue'
import UserBalanceModal from '@/components/admin/user/UserBalanceModal.vue'
import UserDetailDrawer from '@/components/admin/user/UserDetailDrawer.vue'
import { formatAttributeValue } from '@/components/admin/user/attributeValue'
import { formatMultiplier } from '@/utils/formatters'

// 自定义属性定义与本页用户的属性值（放在最前：列定义和排序恢复都要读它）
const attributeDefinitions = ref<UserAttributeDefinition[]>([])
const userAttributeValues = ref<Record<number, Record<number, string>>>({})

// Generate dynamic attribute columns from enabled definitions
const attributeColumns = computed<Column[]>(() =>
  attributeDefinitions.value
    .filter(def => def.enabled)
    .map(def => ({
      key: `attr_${def.id}`,
      label: def.name,
      sortable: false
    }))
)

// Get formatted attribute value for display in table
const getAttributeValue = (userId: number, attrId: number): string => {
  const userAttrs = userAttributeValues.value[userId]
  if (!userAttrs) return '-'
  const value = userAttrs[attrId]
  if (!value) return '-'
  return formatAttributeValue(attributeDefinitions.value.find(d => d.id === attrId), value)
}

// 全部列（列设置菜单按这个顺序列）。默认显示的六列排在最前，顺序同方案：用户、余额、计费倍率、近 30 天消费、状态、最近活跃。
// 原来按平台写死的四个用量子列（Claude / OpenAI / Gemini / Antigravity）已删：按平台看放在「近 30 天消费」的悬停里，平台不写死。
const allColumns = computed<Column[]>(() => [
  { key: 'email', label: t('admin.users.columns.user'), sortable: true },
  { key: 'username', label: t('admin.users.columns.username'), sortable: true },
  { key: 'role', label: t('admin.users.columns.role'), sortable: true },
  { key: 'balance', label: t('admin.users.columns.balance'), sortable: true },
  { key: 'rate_multiplier', label: t('admin.users.columns.rateMultiplier'), sortable: true },
  { key: 'usage', label: t('admin.users.columns.usage'), sortable: false },
  { key: 'concurrency', label: t('admin.users.columns.concurrency'), sortable: true },
  ...(SITE_FEATURES.subscription
    ? [{ key: 'subscriptions', label: t('admin.users.columns.subscriptions'), sortable: false }]
    : []),
  { key: 'status', label: t('admin.users.columns.status'), sortable: true },
  { key: 'last_used_at', label: t('admin.users.columns.lastUsed'), sortable: true },
  { key: 'last_active_at', label: t('admin.users.columns.lastActive'), sortable: true },
  { key: 'created_at', label: t('admin.users.columns.created'), sortable: true },
  { key: 'notes', label: t('admin.users.columns.notes'), sortable: false },
  // 自定义属性：启用了几个就多几列
  ...attributeColumns.value,
  { key: 'actions', label: t('admin.users.columns.actions'), sortable: false }
])

// 列设置（A4 共用实现）：用户列与操作列恒显示；用户名与「用户」列（邮箱 + 用户名小字）重复，默认收起。
// 方案改了默认列，version 加一让本机旧设置作废、回到新默认（v4：内部 ID 列删掉，管理站不显示数字 ID）。
const DEFAULT_HIDDEN_COLUMNS = [
  'username', 'role', 'concurrency', 'subscriptions',
  'last_active_at', 'created_at', 'notes'
]
const columnSettings = useColumnSettings({
  storageKey: 'admin-users-columns',
  version: 4,
  columns: allColumns,
  defaultHidden: DEFAULT_HIDDEN_COLUMNS,
  // 自定义属性列是异步加载后才出现的，按列名规则默认收起（方案：默认只留六列）
  defaultHiddenMatch: (key) => key.startsWith('attr_'),
  alwaysVisible: ['email', 'actions']
})
const isColumnVisible = columnSettings.isVisible
const columns = columnSettings.visibleColumns

const hasVisibleAttributeColumns = computed(() =>
  attributeDefinitions.value.some((def) => def.enabled && isColumnVisible(`attr_${def.id}`))
)
// 订阅列不在表里（功能关着或列被收起）就不让后端顺带查订阅
const hasSubscriptionsColumn = computed(() => columns.value.some((col) => col.key === 'subscriptions'))


const users = ref<AdminUser[]>([])
const loading = ref(false)
const searchQuery = ref('')
const USER_SORT_STORAGE_KEY = 'admin-users-table-sort'
const loadInitialSortState = (): { sort_by: string; sort_order: 'asc' | 'desc' } => {
  const fallback = { sort_by: 'created_at', sort_order: 'desc' as 'asc' | 'desc' }
  // 能恢复的排序键 = 列定义里可排序的列。DataTable 按同一份列定义恢复排序指示，两边不一致会出现
  // 「表头显示按计费倍率排、数据却按创建时间排」（原来手写的清单漏了 rate_multiplier）
  const sortable = new Set(allColumns.value.filter((col) => col.sortable).map((col) => col.key))
  try {
    const raw = localStorage.getItem(USER_SORT_STORAGE_KEY)
    if (!raw) return fallback
    const parsed = JSON.parse(raw) as { key?: string; order?: string }
    const key = typeof parsed.key === 'string' ? parsed.key : ''
    if (!sortable.has(key)) return fallback
    return {
      sort_by: key,
      sort_order: parsed.order === 'asc' ? 'asc' : 'desc'
    }
  } catch {
    return fallback
  }
}
const sortState = reactive(loadInitialSortState())

// Filter values (role, status, and custom attributes)
const filters = reactive({
  role: '',
  status: ''
})
const activeAttributeFilters = reactive<Record<number, string>>({})
// 有搜索或筛选时，空列表说「没有符合条件的结果」，不给「创建第一个用户」的引导
const hasActiveFilters = computed(
  () => !!searchQuery.value.trim() || !!filters.role || !!filters.status || Object.values(activeAttributeFilters).some((value) => !!value)
)

// Visible filters tracking (which filters are shown in the UI)
// Keys: 'role', 'status', 'attr_${id}'
const visibleFilters = reactive<Set<string>>(new Set())

// localStorage keys
const FILTER_VALUES_KEY = 'user-filter-values'
const VISIBLE_FILTERS_KEY = 'user-visible-filters'

// All filterable attribute definitions (enabled attributes)
const filterableAttributes = computed(() =>
  attributeDefinitions.value.filter(def => def.enabled)
)

// Load saved filters from localStorage
const loadSavedFilters = () => {
  try {
    // Load visible filters
    const savedVisible = localStorage.getItem(VISIBLE_FILTERS_KEY)
    if (savedVisible) {
      const parsed = JSON.parse(savedVisible) as string[]
      parsed.forEach(key => visibleFilters.add(key))
    }
    // Load filter values
    const savedValues = localStorage.getItem(FILTER_VALUES_KEY)
    if (savedValues) {
      const parsed = JSON.parse(savedValues)
      if (parsed.role) filters.role = parsed.role
      if (parsed.status) filters.status = parsed.status
      if (parsed.attributes) {
        Object.assign(activeAttributeFilters, parsed.attributes)
      }
    }
  } catch (e) {
    console.error('Failed to load saved filters:', e)
  }
}

// Save filters to localStorage
const saveFiltersToStorage = () => {
  try {
    // Save visible filters
    localStorage.setItem(VISIBLE_FILTERS_KEY, JSON.stringify([...visibleFilters]))
    // Save filter values
    const values = {
      role: filters.role,
      status: filters.status,
      attributes: activeAttributeFilters
    }
    localStorage.setItem(FILTER_VALUES_KEY, JSON.stringify(values))
  } catch (e) {
    console.error('Failed to save filters:', e)
  }
}

// Get attribute definition by ID
const getAttributeDefinition = (attrId: number): UserAttributeDefinition | undefined => {
  return attributeDefinitions.value.find(d => d.id === attrId)
}
const usageStats = ref<Record<string, BatchUserUsageStats>>({})

// 「近 30 天消费」前端排序：DataTable 工作在 server-side-sort 模式，所有 sortable 字段都会触发后端查询，
// 而消费数据是列表出来后按本页用户批量另取的，所以用独立的前端排序状态只排本页。
// 排序状态独立于后端 sortState 持久化（只存 'asc' / 'desc'，读到别的值——含旧版存的 JSON——按未排序）；
// 缺失数据按 0 处理（desc 沉底、asc 置顶）。
type UsageSortOrder = 'asc' | 'desc' | null
const USAGE_SORT_STORAGE_KEY = 'admin-users-usage-sort'

const loadInitialUsageSort = (): UsageSortOrder => {
  try {
    const raw = localStorage.getItem(USAGE_SORT_STORAGE_KEY)
    return raw === 'asc' || raw === 'desc' ? raw : null
  } catch {
    return null
  }
}
const usageSort = ref<UsageSortOrder>(loadInitialUsageSort())
const persistUsageSort = () => {
  try {
    if (usageSort.value) {
      localStorage.setItem(USAGE_SORT_STORAGE_KEY, usageSort.value)
    } else {
      localStorage.removeItem(USAGE_SORT_STORAGE_KEY)
    }
  } catch (e) {
    console.error('Failed to persist usage sort:', e)
  }
}
const clearUsageSort = () => {
  if (!usageSort.value) return
  usageSort.value = null
  persistUsageSort()
}

// 三态循环：desc → asc → off
const toggleUsageSort = () => {
  usageSort.value = usageSort.value === null ? 'desc' : usageSort.value === 'desc' ? 'asc' : null
  persistUsageSort()
}

// 在 server-side 排序结果之上叠加消费列的本地排序；未排序或列被收起时直接透传原数组。
// 稳定排序：等值按原 index 保序，避免拉取新用量数据时表行抖动。
const sortedUsers = computed(() => {
  const order = usageSort.value
  if (!order || !isColumnVisible('usage')) return users.value
  const spend = (userId: number) => usageStats.value[userId]?.total_actual_cost ?? 0
  return [...users.value]
    .map((row, index) => ({ row, index }))
    .sort((a, b) => {
      const av = spend(a.row.id)
      const bv = spend(b.row.id)
      if (av !== bv) return order === 'asc' ? av - bv : bv - av
      return a.index - b.index
    })
    .map((x) => x.row)
})

const {
  selectedIds,
  selectedCount,
  setSelectedIds,
  clear: clearSelection,
  removeMany: removeSelectedIds
} = useTableSelection<AdminUser>({
  rows: sortedUsers,
  getId: (user) => user.id
})

const handleSelectedKeysUpdate = (keys: Array<string | number>) => {
  setSelectedIds(keys.filter((key): key is number => typeof key === 'number'))
}

const getUserSelectionLabel = (user: AdminUser) =>
  t('admin.users.bulkLimits.selectUser', { email: user.email })

const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})

const showCreateModal = ref(false)
const showEditModal = ref(false)
const showBulkEditModal = ref(false)
const showDeleteDialog = ref(false)
const bulkDeleteIds = ref<number[]>([])
const bulkDeleting = ref(false)
const showAttributesModal = ref(false)
const editingUser = ref<AdminUser | null>(null)
const deletingUser = ref<AdminUser | null>(null)
let abortController: AbortController | null = null
let secondaryDataSeq = 0

const loadUsersSecondaryData = async (
  userIds: number[],
  signal?: AbortSignal,
  expectedSeq?: number
) => {
  if (userIds.length === 0) return

  const tasks: Promise<void>[] = []

  if (isColumnVisible('usage')) {
    tasks.push(
      (async () => {
        try {
          const usageResponse = await adminAPI.dashboard.getBatchUsersUsage(userIds)
          if (signal?.aborted) return
          if (typeof expectedSeq === 'number' && expectedSeq !== secondaryDataSeq) return
          usageStats.value = usageResponse.stats
        } catch (e) {
          if (signal?.aborted) return
          console.error('Failed to load usage stats:', e)
        }
      })()
    )
  }

  if (attributeDefinitions.value.length > 0 && hasVisibleAttributeColumns.value) {
    tasks.push(
      (async () => {
        try {
          const attrResponse = await adminAPI.userAttributes.getBatchUserAttributes(userIds)
          if (signal?.aborted) return
          if (typeof expectedSeq === 'number' && expectedSeq !== secondaryDataSeq) return
          userAttributeValues.value = attrResponse.attributes
        } catch (e) {
          if (signal?.aborted) return
          console.error('Failed to load user attribute values:', e)
        }
      })()
    )
  }

  if (tasks.length > 0) {
    await Promise.allSettled(tasks)
  }
}

const refreshCurrentPageSecondaryData = () => {
  const userIds = users.value.map((u) => u.id)
  if (userIds.length === 0) return
  const seq = ++secondaryDataSeq
  void loadUsersSecondaryData(userIds, undefined, seq)
}

// 行操作（A4）：编辑是图标；其余进「⋯」。管理员不能在这里禁用或删除。
const rowActions = (user: AdminUser): RowAction[] => {
  const actions: RowAction[] = [
    { key: 'edit', label: t('common.edit'), icon: 'edit', primary: true, onSelect: () => handleEdit(user) },
    { key: 'deposit', label: t('admin.users.deposit'), icon: 'plus', onSelect: () => handleDeposit(user) },
    { key: 'withdraw', label: t('admin.users.withdraw'), icon: 'arrowDown', onSelect: () => handleWithdraw(user) },
    { key: 'balance-history', label: t('admin.users.balanceHistory'), icon: 'dollar', onSelect: () => openDrawer(user, 'balance') },
    { key: 'api-keys', label: t('admin.users.apiKeys'), icon: 'key', onSelect: () => openDrawer(user, 'keys') }
  ]
  if (user.role !== 'admin') {
    actions.push(
      {
        key: 'toggle-status',
        label: user.status === 'active' ? t('admin.users.disable') : t('admin.users.enable'),
        icon: user.status === 'active' ? 'ban' : 'checkCircle',
        dividerBefore: true,
        onSelect: () => handleToggleStatus(user)
      },
      { key: 'delete', label: t('common.delete'), icon: 'trash', danger: true, onSelect: () => handleDelete(user) }
    )
  }
  return actions
}

// 数字摘要：取仪表盘统计；接口失败就不显示，不摆一排 0。
// 「今日活跃」= 今天调用过 API 的用户（usage_logs 去重），小字写明，和「最近活跃」列同一个意思。
const dashboardStats = ref<DashboardStats | null>(null)
const summaryItems = computed<StatItem[] | null>(() => {
  const stats = dashboardStats.value
  if (!stats) return null
  const fmt = (n: number) => n.toLocaleString()
  return [
    { key: 'total', label: t('admin.users.summary.total'), value: fmt(stats.total_users) },
    { key: 'new', label: t('admin.users.summary.todayNew'), value: fmt(stats.today_new_users) },
    {
      key: 'active',
      label: t('admin.users.summary.todayActive'),
      value: fmt(stats.active_users),
      hint: t('admin.users.summary.todayActiveHint')
    },
    {
      key: 'keys',
      label: t('admin.users.summary.apiKeys'),
      value: fmt(stats.total_api_keys),
      // 全部启用时不附注（同一个数不出现两次）；有停用的才写停用几个
      hint: stats.total_api_keys > stats.active_api_keys
        ? t('admin.users.summary.apiKeysInactive', { count: fmt(stats.total_api_keys - stats.active_api_keys) })
        : undefined
    }
  ]
})
const loadSummary = async () => {
  try {
    dashboardStats.value = await adminAPI.dashboard.getStats()
  } catch {
    dashboardStats.value = null
  }
}

// Balance (Deposit/Withdraw) modal state
const showBalanceModal = ref(false)
const balanceUser = ref<AdminUser | null>(null)
const balanceOperation = ref<'add' | 'subtract'>('add')

// 详情抽屉（A5）：记住打开的是哪个用户；列表刷新后从新数据里取同一个人，不在当前页了就单独取一次
const drawerOpen = ref(false)
const drawerTab = ref('overview')
const drawerUserId = ref<number | null>(null)
const drawerSnapshot = ref<AdminUser | null>(null)
const drawerRefreshKey = ref(0)
const drawerUser = computed<AdminUser | null>(
  () => users.value.find((u) => u.id === drawerUserId.value) ?? drawerSnapshot.value
)

const openDrawer = (user: AdminUser, tab = 'overview') => {
  drawerUserId.value = user.id
  drawerSnapshot.value = user
  drawerTab.value = tab
  drawerOpen.value = true
}

// 改完数据：刷新列表行，再让抽屉重取当前页签（余额流水、密钥等）
const handleUserMutated = async () => {
  await loadUsers()
  const id = drawerUserId.value
  if (!drawerOpen.value || id === null) return
  const fresh = users.value.find((u) => u.id === id)
  if (fresh) {
    drawerSnapshot.value = fresh
  } else {
    try {
      drawerSnapshot.value = await adminAPI.users.getById(id)
    } catch (error) {
      console.error('Failed to refresh user detail:', error)
    }
  }
  drawerRefreshKey.value++
}

// 计算剩余天数
const getDaysRemaining = (expiresAt: string): number => {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diffMs = expires.getTime() - now.getTime()
  return Math.ceil(diffMs / (1000 * 60 * 60 * 24))
}

const subscriptionDaysLabel = (days: number): string =>
  days <= 0 ? t('admin.users.expired') : t('admin.users.daysRemaining', { days })

// 剩余 ≤3 天红、≤7 天橙
const subscriptionDaysClass = (days: number): string => {
  const base = 'rounded px-1 py-0.5 text-xs font-semibold'
  if (days <= 3) return `${base} bg-af-danger-tint/80 text-af-danger`
  if (days <= 7) return `${base} bg-af-warning-tint/80 text-af-warning`
  return `${base} bg-af-hairline`
}

const loadAttributeDefinitions = async () => {
  try {
    attributeDefinitions.value = await adminAPI.userAttributes.listEnabledDefinitions()
  } catch (e) {
    console.error('Failed to load attribute definitions:', e)
  }
}

// Handle attributes modal close - reload definitions and users
const handleAttributesModalClose = async () => {
  showAttributesModal.value = false
  await loadAttributeDefinitions()
  loadUsers()
}

const loadUsers = async () => {
  abortController?.abort()
  const currentAbortController = new AbortController()
  abortController = currentAbortController
  const { signal } = currentAbortController
  loading.value = true
  try {
    // Build attribute filters from active filters
    const attrFilters: Record<number, string> = {}
    for (const [attrId, value] of Object.entries(activeAttributeFilters)) {
      if (value) {
        attrFilters[Number(attrId)] = value
      }
    }

    const response = await adminAPI.users.list(
      pagination.page,
      pagination.page_size,
      {
        role: filters.role as any,
        status: filters.status as any,
        search: searchQuery.value || undefined,
        attributes: Object.keys(attrFilters).length > 0 ? attrFilters : undefined,
        // 后端不传这个参数时默认带订阅，所以显式传 false
        include_subscriptions: hasSubscriptionsColumn.value,
        sort_by: sortState.sort_by,
        sort_order: sortState.sort_order
      },
      { signal }
    )
    if (signal.aborted) {
      return
    }
    users.value = response.items
    pagination.total = response.total
    pagination.pages = response.pages
    usageStats.value = {}
    userAttributeValues.value = {}

    // Defer heavy secondary data so table can render first.
    if (response.items.length > 0) {
      const userIds = response.items.map((u) => u.id)
      const seq = ++secondaryDataSeq
      window.setTimeout(() => {
        if (signal.aborted || seq !== secondaryDataSeq) return
        void loadUsersSecondaryData(userIds, signal, seq)
      }, 50)
    }
  } catch (error: any) {
    const errorInfo = error as { name?: string; code?: string }
    if (errorInfo?.name === 'AbortError' || errorInfo?.name === 'CanceledError' || errorInfo?.code === 'ERR_CANCELED') {
      return
    }
    console.error('Error loading users:', error)
  } finally {
    if (abortController === currentAbortController) {
      loading.value = false
    }
  }
}

const handleBulkLimitsSuccess = async () => {
  clearSelection()
  await loadUsers()
}

let searchTimeout: ReturnType<typeof setTimeout>
const handleSearch = () => {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    loadUsers()
  }, 300)
}

const handlePageChange = (page: number) => {
  // 确保页码在有效范围内
  const validPage = Math.max(1, Math.min(page, pagination.pages || 1))
  pagination.page = validPage
  loadUsers()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  loadUsers()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  clearUsageSort()
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadUsers()
}

// Filter helpers
const getAttributeDefinitionName = (attrId: number): string => {
  const def = attributeDefinitions.value.find(d => d.id === attrId)
  return def?.name || String(attrId)
}

// Toggle a custom attribute filter
const toggleAttributeFilter = (attr: UserAttributeDefinition) => {
  const key = `attr_${attr.id}`
  if (visibleFilters.has(key)) {
    visibleFilters.delete(key)
    delete activeAttributeFilters[attr.id]
  } else {
    visibleFilters.add(key)
    activeAttributeFilters[attr.id] = ''
  }
  saveFiltersToStorage()
  pagination.page = 1
  loadUsers()
}

const updateAttributeFilter = (attrId: number, value: string) => {
  activeAttributeFilters[attrId] = value
}

// Apply filter and save to localStorage
const applyFilter = () => {
  saveFiltersToStorage()
  loadUsers()
}

const handleEdit = (user: AdminUser) => {
  editingUser.value = user
  showEditModal.value = true
}

const closeEditModal = () => {
  showEditModal.value = false
  editingUser.value = null
}

// 禁用要先确认（muqian 2026-09-30 方案第四节：确认用页面里的确认框），启用直接生效
const disablingUser = ref<AdminUser | null>(null)
const toggleError = ref('')

const setUserStatus = async (user: AdminUser, status: 'active' | 'disabled'): Promise<boolean> => {
  toggleError.value = ''
  try {
    await adminAPI.users.toggleStatus(user.id, status)
    void handleUserMutated()
    return true
  } catch (error) {
    toggleError.value = extractApiErrorMessage(error, t('admin.users.toggleStatusFailed'))
    console.error('Error toggling user status:', error)
    return false
  }
}

const handleToggleStatus = (user: AdminUser) => {
  if (user.status === 'active') {
    toggleError.value = ''
    disablingUser.value = user
    return
  }
  void setUserStatus(user, 'active')
}

const confirmDisable = async () => {
  const user = disablingUser.value
  if (!user) return
  if (await setUserStatus(user, 'disabled')) disablingUser.value = null
}

// 编辑弹窗里点「充值 / 扣减」：打开同一个余额弹窗
const handleEditAdjustBalance = (operation: 'add' | 'subtract') => {
  const user = editingUser.value
  if (!user) return
  if (operation === 'add') handleDeposit(user)
  else handleWithdraw(user)
}

// 余额改完：编辑弹窗开着就把它的「当前余额」换成新的（只换余额，正在改的字段不动）
const handleBalanceUpdated = (updated: AdminUser) => {
  if (editingUser.value?.id === updated.id) editingUser.value = { ...editingUser.value, balance: updated.balance }
  void handleUserMutated()
}

const handleDelete = (user: AdminUser) => {
  deletingUser.value = user
  deleteError.value = ''
  showDeleteDialog.value = true
}

const confirmDelete = async () => {
  if (!deletingUser.value) return
  deleteError.value = ''
  try {
    await adminAPI.users.delete(deletingUser.value.id)
    if (drawerUserId.value === deletingUser.value.id) drawerOpen.value = false
    showDeleteDialog.value = false
    deletingUser.value = null
    loadUsers()
  } catch (error: any) {
    deleteError.value = extractApiErrorMessage(error, t('admin.users.failedToDelete'))
  }
}

const deleteError = ref('')
const bulkDeleteError = ref('')
const confirmBulkDelete = async () => {
  const ids = bulkDeleteIds.value
  bulkDeleteIds.value = []
  bulkDeleting.value = true
  bulkDeleteError.value = ''
  const deletedIds: number[] = []
  for (const id of ids) {
    try {
      await adminAPI.users.delete(id)
      deletedIds.push(id)
    } catch {
      // 失败的留在选中里，循环结束后统一报数
    }
  }
  removeSelectedIds(deletedIds)
  if (deletedIds.length > 0) {
    pagination.page = 1
  }
  const failed = ids.length - deletedIds.length
  if (failed > 0) bulkDeleteError.value = t('admin.users.bulkDelete.failed', { count: failed })
  await loadUsers()
  bulkDeleting.value = false
}

const handleDeposit = (user: AdminUser) => {
  balanceUser.value = user
  balanceOperation.value = 'add'
  showBalanceModal.value = true
}

const handleWithdraw = (user: AdminUser) => {
  balanceUser.value = user
  balanceOperation.value = 'subtract'
  showBalanceModal.value = true
}

const closeBalanceModal = () => {
  showBalanceModal.value = false
  balanceUser.value = null
}

// 刚打开的列要补数据：消费 / 属性列按需批量拉取，订阅列随列表一起取
watch(
  () => columns.value.map((col) => col.key),
  (next, prev) => {
    const opened = next.filter((key) => !prev?.includes(key))
    if (opened.some((key) => key === 'usage' || key.startsWith('attr_'))) {
      refreshCurrentPageSecondaryData()
    }
    if (opened.includes('subscriptions')) loadUsers()
  }
)

onMounted(async () => {
  await loadAttributeDefinitions()
  loadSavedFilters()
  loadUsers()
  void loadSummary()
})

onUnmounted(() => {
  clearTimeout(searchTimeout)
  abortController?.abort()
})
</script>
