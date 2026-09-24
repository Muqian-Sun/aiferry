<template>
  <!--
    用户（A4 列表模板）：标题右侧是工具菜单与「创建用户」；数字摘要；工具行 = 搜索 + 筛选标签 + 刷新 / 列设置；
    行尾「编辑」图标 + 「⋯」（充值、扣款、余额记录、API 密钥、禁用、删除）；选中行时出现批量条。
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

          <!-- 自定义属性筛选：从「+ 属性」里挑出来的才显示 -->
          <template v-for="(value, attrId) in activeAttributeFilters" :key="attrId">
            <div v-if="visibleFilters.has(`attr_${attrId}`)" class="w-full sm:w-36">
              <Select
                v-if="['select', 'multi_select'].includes(getAttributeDefinition(Number(attrId))?.type || '')"
                :model-value="value"
                :options="[
                  { value: '', label: getAttributeDefinitionName(Number(attrId)) },
                  ...(getAttributeDefinition(Number(attrId))?.options || [])
                ]"
                @update:model-value="(val) => { updateAttributeFilter(Number(attrId), String(val ?? '')); applyFilter() }"
              />
              <input
                v-else
                :value="value"
                :type="getAttributeDefinition(Number(attrId))?.type === 'number' ? 'number' : 'text'"
                :placeholder="getAttributeDefinitionName(Number(attrId))"
                class="input h-8 py-0 text-13"
                @input="(e) => updateAttributeFilter(Number(attrId), (e.target as HTMLInputElement).value)"
                @keyup.enter="applyFilter"
              />
            </div>
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
          @sort="handleSort"
          @update:selected-keys="handleSelectedKeysUpdate"
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
                <span class="truncate">{{ sub.plan?.name || `#${sub.plan_id}` }}</span>
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
              class="font-medium tabular-nums text-af-ink underline decoration-dashed decoration-af-ink-4 underline-offset-4 transition-colors hover:text-af-brand-hover"
              :title="t('admin.users.balanceHistoryTip')"
              @click="handleBalanceHistory(row)"
            >
              ${{ value.toFixed(2) }}
            </button>
          </template>

          <!-- 用量列自定义表头：列名 + 单个排序图标按钮，点击展开"今日/近30天"菜单。
               column.sortable=false，DataTable 内置点击逻辑不会触发；
               菜单项三态循环：desc → asc → off。 -->
          <template
            v-for="usageKey in USAGE_COLUMN_KEYS"
            :key="usageKey"
            #[`header-${usageKey}`]="{ column }"
          >
            <div class="flex items-center gap-1.5">
              <span>{{ column.label }}</span>
              <div class="usage-sort-trigger relative">
                <button
                  type="button"
                  class="flex items-center gap-1 rounded px-1 py-0.5 transition-colors hover:bg-af-hairline"
                  :class="usageSort && usageSort.key === usageKey
                    ? 'text-af-brand'
                    : 'text-af-ink-3'"
                  :title="t('admin.users.sortBy')"
                  :data-test="`usage-sort-trigger-${usageKey}`"
                  @click.stop="toggleUsageSortMenu(usageKey)"
                >
                  <span
                    v-if="usageSort && usageSort.key === usageKey"
                    class="text-[10px] normal-case font-medium tracking-normal"
                  >{{ usageSort.metric === 'today' ? t('admin.users.today') : t('admin.users.total') }}</span>
                  <svg
                    v-if="usageSort && usageSort.key === usageKey"
                    class="h-3.5 w-3.5"
                    :class="{ 'rotate-180': usageSort.order === 'desc' }"
                    fill="currentColor"
                    viewBox="0 0 20 20"
                  >
                    <path
                      fill-rule="evenodd"
                      d="M14.707 12.707a1 1 0 01-1.414 0L10 9.414l-3.293 3.293a1 1 0 01-1.414-1.414l4-4a1 1 0 011.414 0l4 4a1 1 0 010 1.414z"
                      clip-rule="evenodd"
                    />
                  </svg>
                  <svg v-else class="h-3.5 w-3.5" fill="currentColor" viewBox="0 0 20 20">
                    <path d="M10 3l-4 5h8l-4-5zM10 17l4-5H6l4 5z" />
                  </svg>
                </button>
                <!-- 弹出菜单：今日 / 近30天，点击进行三态循环切换。 -->
                <div
                  v-if="openUsageSortMenu === usageKey"
                  class="absolute right-0 top-full z-50 mt-1 min-w-[120px] rounded-lg border border-af-hairline bg-af-sheet py-1 shadow-lg"
                >
                  <button
                    v-for="metric in (['today', 'total'] as const)"
                    :key="metric"
                    type="button"
                    class="flex w-full items-center justify-between gap-3 px-3 py-1.5 text-left text-xs normal-case tracking-normal hover:bg-af-sunken"
                    :class="isUsageSortActive(usageKey, metric)
                      ? 'font-medium text-af-brand'
                      : 'text-af-ink-2'"
                    :data-test="`usage-sort-${usageKey}-${metric}`"
                    @click.stop="toggleUsageSort(usageKey, metric)"
                  >
                    <span>{{ metric === 'today' ? t('admin.users.today') : t('admin.users.total') }}</span>
                    <svg
                      v-if="getUsageSortOrder(usageKey, metric)"
                      class="h-3 w-3"
                      :class="{ 'rotate-180': getUsageSortOrder(usageKey, metric) === 'desc' }"
                      fill="currentColor"
                      viewBox="0 0 20 20"
                    >
                      <path
                        fill-rule="evenodd"
                        d="M14.707 12.707a1 1 0 01-1.414 0L10 9.414l-3.293 3.293a1 1 0 01-1.414-1.414l4-4a1 1 0 011.414 0l4 4a1 1 0 010 1.414z"
                        clip-rule="evenodd"
                      />
                    </svg>
                  </button>
                  <div class="mt-1 border-t border-af-hairline px-3 py-1 text-[10px] normal-case tracking-normal text-af-ink-3">
                    {{ t('admin.users.sortCurrentPageOnly') }}
                  </div>
                </div>
              </div>
            </div>
          </template>

          <template #cell-usage="{ row }">
            <PlatformUsageBreakdown
              :today="usageStats[row.id]?.today_actual_cost ?? 0"
              :total="usageStats[row.id]?.total_actual_cost ?? 0"
              :by-platform="usageStats[row.id]?.by_platform"
            />
          </template>

          <template #cell-usage_anthropic="{ row }">
            <PlatformCostCell :usage="getPlatformUsage(row.id, 'anthropic')" />
          </template>

          <template #cell-usage_openai="{ row }">
            <PlatformCostCell :usage="getPlatformUsage(row.id, 'openai')" />
          </template>

          <template #cell-usage_gemini="{ row }">
            <PlatformCostCell :usage="getPlatformUsage(row.id, 'gemini')" />
          </template>

          <template #cell-usage_antigravity="{ row }">
            <PlatformCostCell :usage="getPlatformUsage(row.id, 'antigravity')" />
          </template>

          <template #cell-concurrency="{ row }">
            <UserConcurrencyCell
              :current="row.current_concurrency ?? 0"
              :max="row.concurrency"
            />
          </template>

          <template #cell-rate_multiplier="{ row }">
            <span class="tabular-nums text-af-ink-2">× {{ row.rate_multiplier }}</span>
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

          <!-- 时间列：最近活跃 / 最近使用写相对时间，悬停看精确时间；注册时间只写日期 -->
          <template #cell-created_at="{ value }">
            <span class="tabular-nums text-af-ink-3" :title="formatDateTime(value)">{{ formatDateOnly(value) }}</span>
          </template>

          <template #cell-last_used_at="{ value }">
            <span v-if="value" class="text-af-ink-2" :title="formatDateTime(value)">{{ formatRelativeTime(value) }}</span>
            <span v-else class="text-af-ink-4">-</span>
          </template>

          <template #cell-last_active_at="{ value }">
            <span v-if="value" class="text-af-ink-2" :title="formatDateTime(value)">{{ formatRelativeTime(value) }}</span>
            <span v-else class="text-af-ink-4">-</span>
          </template>

          <template #cell-actions="{ row }">
            <RowActions :actions="rowActions(row)" />
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.users.noUsersYet')"
              :description="t('admin.users.createFirstUser')"
              :action-text="t('admin.users.createUser')"
              @action="showCreateModal = true"
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

    <ConfirmDialog :show="showDeleteDialog" :title="t('admin.users.deleteUser')" :message="t('admin.users.deleteConfirm', { email: deletingUser?.email })" :danger="true" @confirm="confirmDelete" @cancel="showDeleteDialog = false" />
    <ConfirmDialog
      :show="bulkDeleteIds.length > 0"
      :title="t('admin.users.bulkDelete.title')"
      :message="t('admin.users.bulkDelete.confirm', { count: bulkDeleteIds.length })"
      :confirm-text="t('common.delete')"
      danger
      @confirm="confirmBulkDelete"
      @cancel="bulkDeleteIds = []"
    />
    <UserCreateModal :show="showCreateModal" @close="showCreateModal = false" @success="loadUsers" />
    <UserEditModal :show="showEditModal" :user="editingUser" @close="closeEditModal" @success="loadUsers" />
    <BulkEditUserModal
      :show="showBulkEditModal"
      :selected-ids="selectedIds"
      @close="showBulkEditModal = false"
      @success="handleBulkLimitsSuccess"
    />
    <UserApiKeysModal :show="showApiKeysModal" :user="viewingUser" @close="closeApiKeysModal" />
    <UserBalanceModal :show="showBalanceModal" :user="balanceUser" :operation="balanceOperation" @close="closeBalanceModal" @success="loadUsers" />
    <UserBalanceHistoryModal :show="showBalanceHistoryModal" :user="balanceHistoryUser" @close="closeBalanceHistoryModal" @deposit="handleDepositFromHistory" @withdraw="handleWithdrawFromHistory" />
    <UserAttributesConfigModal :show="showAttributesModal" @close="handleAttributesModalClose" />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useTableSelection } from '@/composables/useTableSelection'
import { formatDateOnly, formatDateTime, formatRelativeTime } from '@/utils/format'
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
import EmptyState from '@/components/common/EmptyState.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import StatRow from '@/components/user/shell/StatRow.vue'
import type { StatItem } from '@/components/user/shell/types'
import { BulkBar, ColumnSettingsMenu, FilterChip, ListToolbar, MenuItem, PopoverMenu, RowActions } from '@/components/admin/list'
import type { RowAction } from '@/components/admin/list'
import { useColumnSettings } from '@/composables/useColumnSettings'
import type { DashboardStats } from '@/types'
import Select from '@/components/common/Select.vue'
import UserAttributesConfigModal from '@/components/user/UserAttributesConfigModal.vue'
import UserConcurrencyCell from '@/components/user/UserConcurrencyCell.vue'
import PlatformUsageBreakdown from '@/components/user/PlatformUsageBreakdown.vue'
import PlatformCostCell from '@/components/user/PlatformCostCell.vue'
import UserCreateModal from '@/components/admin/user/UserCreateModal.vue'
import UserEditModal from '@/components/admin/user/UserEditModal.vue'
import BulkEditUserModal from '@/components/admin/user/BulkEditUserModal.vue'
import UserApiKeysModal from '@/components/admin/user/UserApiKeysModal.vue'
import UserBalanceModal from '@/components/admin/user/UserBalanceModal.vue'
import UserBalanceHistoryModal from '@/components/admin/user/UserBalanceHistoryModal.vue'

const appStore = useAppStore()

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

  // Find definition for this attribute
  const def = attributeDefinitions.value.find(d => d.id === attrId)
  if (!def) return value

  // Format based on type
  if (def.type === 'multi_select' && value) {
    try {
      const arr = JSON.parse(value)
      if (Array.isArray(arr)) {
        // Map values to labels
        return arr.map(v => {
          const opt = def.options?.find(o => o.value === v)
          return opt?.label || v
        }).join(', ')
      }
    } catch {
      return value
    }
  }

  if (def.type === 'select' && value && def.options) {
    const opt = def.options.find(o => o.value === value)
    return opt?.label || value
  }

  return value
}

// All possible columns (for column settings)
const allColumns = computed<Column[]>(() => [
  { key: 'email', label: t('admin.users.columns.user'), sortable: true },
  { key: 'id', label: t('admin.users.columns.id'), sortable: true },
  { key: 'username', label: t('admin.users.columns.username'), sortable: true },
  { key: 'notes', label: t('admin.users.columns.notes'), sortable: false },
  // Dynamic attribute columns
  ...attributeColumns.value,
  { key: 'role', label: t('admin.users.columns.role'), sortable: true },
  { key: 'subscriptions', label: t('admin.users.columns.subscriptions'), sortable: false },
  { key: 'balance', label: t('admin.users.columns.balance'), sortable: true },
  { key: 'usage', label: t('admin.users.columns.usage'), sortable: false },
  { key: 'usage_anthropic', label: t('admin.users.columns.usageAnthropic'), sortable: false },
  { key: 'usage_openai', label: t('admin.users.columns.usageOpenAI'), sortable: false },
  { key: 'usage_gemini', label: t('admin.users.columns.usageGemini'), sortable: false },
  { key: 'usage_antigravity', label: t('admin.users.columns.usageAntigravity'), sortable: false },
  { key: 'concurrency', label: t('admin.users.columns.concurrency'), sortable: true },
  { key: 'rate_multiplier', label: t('admin.users.columns.rateMultiplier'), sortable: true },
  { key: 'status', label: t('admin.users.columns.status'), sortable: true },
  { key: 'last_active_at', label: t('admin.users.columns.lastActive'), sortable: true },
  { key: 'last_used_at', label: t('admin.users.columns.lastUsed'), sortable: true },
  { key: 'created_at', label: t('admin.users.columns.created'), sortable: true },
  { key: 'actions', label: t('admin.users.columns.actions'), sortable: false }
])

// 列设置（A4 共用实现）：用户列与操作列恒显示；ID、用户名与「用户」列（邮箱 + 用户名小字）重复，默认收起
const DEFAULT_HIDDEN_COLUMNS = [
  'notes', 'subscriptions', 'usage', 'concurrency',
  'usage_anthropic', 'usage_openai', 'usage_gemini', 'usage_antigravity',
  'id', 'username'
]
const columnSettings = useColumnSettings({
  storageKey: 'admin-users-columns',
  version: 1,
  columns: allColumns,
  defaultHidden: DEFAULT_HIDDEN_COLUMNS,
  alwaysVisible: ['email', 'actions']
})
const isColumnVisible = columnSettings.isVisible
const columns = columnSettings.visibleColumns

// usage 主列或任意 usage_<platform> 子列可见时都需要批量拉取用量数据
// 列 key → 平台名（'usage' 主列汇总所有平台时为 null）
// 显式数组取代 Object.keys()：保证迭代顺序（决定列头排序按钮渲染顺序）
// 不会因 JS 引擎差异或 USAGE_COLUMN_PLATFORMS 属性顺序调整而静默变化。
const USAGE_COLUMN_KEYS: readonly string[] = ['usage', 'usage_anthropic', 'usage_openai', 'usage_gemini', 'usage_antigravity']
const USAGE_COLUMN_PLATFORMS: Record<string, string | null> = {
  usage: null,
  usage_anthropic: 'anthropic',
  usage_openai: 'openai',
  usage_gemini: 'gemini',
  usage_antigravity: 'antigravity'
}
const PLATFORM_USAGE_COLUMNS = USAGE_COLUMN_KEYS.filter((k) => k !== 'usage')
const hasVisibleUsageColumn = computed(
  () => isColumnVisible('usage') || PLATFORM_USAGE_COLUMNS.some((k) => isColumnVisible(k))
)
const hasVisibleAttributeColumns = computed(() =>
  attributeDefinitions.value.some((def) => def.enabled && isColumnVisible(`attr_${def.id}`))
)


const users = ref<AdminUser[]>([])
const loading = ref(false)
const searchQuery = ref('')
const USER_SORT_STORAGE_KEY = 'admin-users-table-sort'
const loadInitialSortState = (): { sort_by: string; sort_order: 'asc' | 'desc' } => {
  const fallback = { sort_by: 'created_at', sort_order: 'desc' as 'asc' | 'desc' }
  const sortable = new Set(['email', 'id', 'username', 'role', 'balance', 'concurrency', 'status', 'last_used_at', 'last_active_at', 'created_at'])
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

const getPlatformUsage = (userId: number, platform: string) =>
  usageStats.value[userId]?.by_platform?.find((p) => p.platform === platform)

// 用量列前端排序：DataTable 工作在 server-side-sort 模式，所有 sortable
// 字段都会触发后端查询，而用量列数据是异步批量拉取后再合并到当前页，
// 因此采用独立的前端排序状态对当前页 users 做本地排序。
// 排序状态独立于后端 sortState 持久化；缺失数据按 0 处理（desc 沉底、asc 置顶）。
type UsageMetric = 'today' | 'total'
type UsageSortState = { key: string; metric: UsageMetric; order: 'asc' | 'desc' } | null
const USAGE_SORT_STORAGE_KEY = 'admin-users-usage-sort'
// 列头排序按钮点击后弹出的"今日/近30天"选择菜单，同时只允许一个列展开。
const openUsageSortMenu = ref<string | null>(null)

const loadInitialUsageSort = (): UsageSortState => {
  try {
    const raw = localStorage.getItem(USAGE_SORT_STORAGE_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<{ key: string; metric: string; order: string }>
    if (!parsed.key || !USAGE_COLUMN_KEYS.includes(parsed.key)) return null
    const metric: UsageMetric = parsed.metric === 'total' ? 'total' : 'today'
    const order: 'asc' | 'desc' = parsed.order === 'asc' ? 'asc' : 'desc'
    return { key: parsed.key, metric, order }
  } catch {
    return null
  }
}
const usageSort = ref<UsageSortState>(loadInitialUsageSort())
const persistUsageSort = () => {
  try {
    if (usageSort.value) {
      localStorage.setItem(USAGE_SORT_STORAGE_KEY, JSON.stringify(usageSort.value))
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
  openUsageSortMenu.value = null
  persistUsageSort()
}

const isUsageSortActive = (key: string, metric: UsageMetric) =>
  !!usageSort.value && usageSort.value.key === key && usageSort.value.metric === metric
const getUsageSortOrder = (key: string, metric: UsageMetric): 'asc' | 'desc' | null =>
  isUsageSortActive(key, metric) ? usageSort.value!.order : null

// 三态循环：desc → asc → off。选完即关闭菜单（用户大多希望"选中即应用"，
// 想再切换 order 时重新打开菜单点同一项即可）。
const toggleUsageSort = (key: string, metric: UsageMetric) => {
  const cur = usageSort.value
  if (cur && cur.key === key && cur.metric === metric) {
    usageSort.value = cur.order === 'desc' ? { key, metric, order: 'asc' } : null
  } else {
    usageSort.value = { key, metric, order: 'desc' }
  }
  persistUsageSort()
  openUsageSortMenu.value = null
}

// 点击图标本身不触发排序，仅开关菜单；首次排序由用户在菜单内选择 metric 触发（默认 desc，详见 toggleUsageSort）。
const toggleUsageSortMenu = (key: string) => {
  openUsageSortMenu.value = openUsageSortMenu.value === key ? null : key
}

const getUsageValue = (userId: number, key: string, metric: UsageMetric): number => {
  const stats = usageStats.value[userId]
  if (!stats) return 0
  const platform = USAGE_COLUMN_PLATFORMS[key]
  if (platform === null) {
    return metric === 'today' ? stats.today_actual_cost ?? 0 : stats.total_actual_cost ?? 0
  }
  const p = stats.by_platform?.find((x) => x.platform === platform)
  if (!p) return 0
  return metric === 'today' ? p.today_actual_cost ?? 0 : p.total_actual_cost ?? 0
}

// 在 server-side 排序结果之上叠加用量列的本地排序；无 usageSort 时直接透传原数组。
// 稳定排序：等值按原 index 保序，避免拉取新用量数据时表行抖动。
const sortedUsers = computed(() => {
  const s = usageSort.value
  if (!s) return users.value
  return [...users.value]
    .map((row, index) => ({ row, index }))
    .sort((a, b) => {
      const av = getUsageValue(a.row.id, s.key, s.metric)
      const bv = getUsageValue(b.row.id, s.key, s.metric)
      if (av !== bv) return s.order === 'asc' ? av - bv : bv - av
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

// User attribute definitions and values
const attributeDefinitions = ref<UserAttributeDefinition[]>([])
const userAttributeValues = ref<Record<number, Record<number, string>>>({})
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
const showApiKeysModal = ref(false)
const showAttributesModal = ref(false)
const editingUser = ref<AdminUser | null>(null)
const deletingUser = ref<AdminUser | null>(null)
const viewingUser = ref<AdminUser | null>(null)
let abortController: AbortController | null = null
let secondaryDataSeq = 0

const loadUsersSecondaryData = async (
  userIds: number[],
  signal?: AbortSignal,
  expectedSeq?: number
) => {
  if (userIds.length === 0) return

  const tasks: Promise<void>[] = []

  if (hasVisibleUsageColumn.value) {
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
    { key: 'balance-history', label: t('admin.users.balanceHistory'), icon: 'dollar', onSelect: () => handleBalanceHistory(user) },
    { key: 'api-keys', label: t('admin.users.apiKeys'), icon: 'key', onSelect: () => handleViewApiKeys(user) }
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

// 用量列表头的「今日 / 近 30 天」排序菜单：点外面关掉
const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  if (openUsageSortMenu.value !== null && !target.closest('.usage-sort-trigger')) {
    openUsageSortMenu.value = null
  }
}

// 数字摘要：取仪表盘统计；接口失败就不显示，不摆一排 0
const dashboardStats = ref<DashboardStats | null>(null)
const summaryItems = computed<StatItem[] | null>(() => {
  const stats = dashboardStats.value
  if (!stats) return null
  const fmt = (n: number) => n.toLocaleString()
  return [
    { key: 'total', label: t('admin.users.summary.total'), value: fmt(stats.total_users) },
    { key: 'new', label: t('admin.users.summary.todayNew'), value: fmt(stats.today_new_users) },
    { key: 'active', label: t('admin.users.summary.todayActive'), value: fmt(stats.active_users) },
    {
      key: 'keys',
      label: t('admin.users.summary.apiKeys'),
      value: fmt(stats.total_api_keys),
      hint: t('admin.users.summary.apiKeysActive', { count: fmt(stats.active_api_keys) })
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

// Balance History modal state
const showBalanceHistoryModal = ref(false)
const balanceHistoryUser = ref<AdminUser | null>(null)

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
  const base = 'rounded px-1 py-0.5 text-[10px] font-semibold'
  if (days <= 3) return `${base} bg-af-danger-tint/80 text-af-danger`
  if (days <= 7) return `${base} bg-af-warning-tint/80 text-af-warning`
  return `${base} bg-black/10`
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
        include_subscriptions: true,
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
    const message = error.response?.data?.detail || error.message || t('admin.users.failedToLoad')
    appStore.showError(message)
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

const handleToggleStatus = async (user: AdminUser) => {
  const newStatus = user.status === 'active' ? 'disabled' : 'active'
  try {
    await adminAPI.users.toggleStatus(user.id, newStatus)
    appStore.showSuccess(
      newStatus === 'active' ? t('admin.users.userEnabled') : t('admin.users.userDisabled')
    )
    loadUsers()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.users.failedToToggle'))
    console.error('Error toggling user status:', error)
  }
}

const handleViewApiKeys = (user: AdminUser) => {
  viewingUser.value = user
  showApiKeysModal.value = true
}

const closeApiKeysModal = () => {
  showApiKeysModal.value = false
  viewingUser.value = null
}

const handleDelete = (user: AdminUser) => {
  deletingUser.value = user
  showDeleteDialog.value = true
}

const confirmDelete = async () => {
  if (!deletingUser.value) return
  try {
    await adminAPI.users.delete(deletingUser.value.id)
    appStore.showSuccess(t('common.success'))
    showDeleteDialog.value = false
    deletingUser.value = null
    loadUsers()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.users.failedToDelete'))
    console.error('Error deleting user:', error)
  }
}

const confirmBulkDelete = async () => {
  const ids = bulkDeleteIds.value
  bulkDeleteIds.value = []
  bulkDeleting.value = true
  const deletedIds: number[] = []
  for (const id of ids) {
    try {
      await adminAPI.users.delete(id)
      deletedIds.push(id)
    } catch (error) {
      console.error('Error deleting user:', error)
    }
  }
  removeSelectedIds(deletedIds)
  if (deletedIds.length > 0) {
    appStore.showSuccess(t('admin.users.bulkDelete.success', { count: deletedIds.length }))
    pagination.page = 1
  }
  const failed = ids.length - deletedIds.length
  if (failed > 0) appStore.showError(t('admin.users.bulkDelete.failed', { count: failed }))
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

const handleBalanceHistory = (user: AdminUser) => {
  balanceHistoryUser.value = user
  showBalanceHistoryModal.value = true
}

const closeBalanceHistoryModal = () => {
  showBalanceHistoryModal.value = false
  balanceHistoryUser.value = null
}

// Handle deposit from balance history modal
const handleDepositFromHistory = () => {
  if (balanceHistoryUser.value) {
    handleDeposit(balanceHistoryUser.value)
  }
}

// Handle withdraw from balance history modal
const handleWithdrawFromHistory = () => {
  if (balanceHistoryUser.value) {
    handleWithdraw(balanceHistoryUser.value)
  }
}

// 刚打开的列要补数据：用量 / 属性列按需批量拉取，订阅列随列表一起取
watch(
  () => columns.value.map((col) => col.key),
  (next, prev) => {
    const opened = next.filter((key) => !prev?.includes(key))
    if (opened.some((key) => key === 'usage' || key.startsWith('usage_') || key.startsWith('attr_'))) {
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
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
  clearTimeout(searchTimeout)
  abortController?.abort()
})
</script>
