<template>
  <!--
    代理（A4 列表模板）：标题右侧「⋯」（导入、导出、全部测试连接、全部质量检测）+「添加代理」；
    工具行 = 搜索 + 协议 / 状态筛选标签 + 刷新；行尾「编辑」图标 +「⋯」（测试连接、质量检测、删除）；
    选中行时出现批量条（测试连接、质量检测、删除）。添加 / 编辑对话框拆到 components/admin/proxy。
  -->
  <AppLayout>
    <template #header-actions>
      <PopoverMenu width-class="w-52">
        <template #trigger="{ open }">
          <button
            type="button"
            class="btn btn-ghost btn-md px-2.5"
            :class="open ? 'bg-af-sunken text-af-ink' : ''"
            :title="t('common.more')"
            :aria-label="t('common.more')"
            data-testid="proxies-tools"
          >
            <Icon name="more" size="md" />
          </button>
        </template>
        <MenuItem icon="upload" data-testid="proxies-import" @click="showImportData = true">
          {{ t('admin.proxies.dataImport') }}
        </MenuItem>
        <MenuItem icon="download" data-testid="proxies-export" @click="showExportDataDialog = true">
          {{ selectedCount > 0 ? t('admin.proxies.dataExportSelected') : t('admin.proxies.dataExport') }}
        </MenuItem>
        <MenuItem divider />
        <MenuItem
          icon="play"
          :disabled="batchTesting || loading"
          data-testid="proxies-test-all"
          @click="handleBatchTest('all')"
        >
          {{ t('admin.proxies.testAll') }}
        </MenuItem>
        <MenuItem
          icon="shield"
          :disabled="batchQualityChecking || loading"
          data-testid="proxies-quality-all"
          @click="handleBatchQualityCheck('all')"
        >
          {{ t('admin.proxies.qualityCheckAll') }}
        </MenuItem>
      </PopoverMenu>
      <button type="button" class="btn btn-primary btn-md" @click="showCreateModal = true">
        <Icon name="plus" size="md" />
        {{ t('admin.proxies.createProxy') }}
      </button>
    </template>

    <TablePageLayout>
      <template #filters>
        <ListToolbar>
          <SearchInput
            v-model="searchQuery"
            compact
            class="w-full sm:w-64"
            :placeholder="t('admin.proxies.searchProxies')"
            @update:model-value="handleSearch"
          />
          <FilterChip
            v-model="filters.protocol"
            :label="t('admin.proxies.columns.protocol')"
            :options="protocolOptions"
            test-id="filter-protocol"
            @change="handleFilterChange"
          />
          <FilterChip
            v-model="filters.status"
            :label="t('admin.proxies.columns.status')"
            :options="statusOptions"
            test-id="filter-status"
            @change="handleFilterChange"
          />

          <template #end>
            <button
              type="button"
              class="rounded-md p-2 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink disabled:opacity-40"
              :disabled="loading"
              :title="t('common.refresh')"
              :aria-label="t('common.refresh')"
              @click="loadProxies"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
          </template>
        </ListToolbar>
      </template>

      <template #table>
        <div ref="proxyTableRef" class="flex min-h-0 flex-1 flex-col overflow-hidden">
        <DataTable
          :columns="columns"
          :data="proxies"
          :loading="loading"
          row-key="id"
          selectable
          :selected-keys="selectedIds"
          :selection-label="getProxySelectionLabel"
          :server-side-sort="true"
          default-sort-key="id"
          default-sort-order="desc"
          @sort="handleSort"
          @update:selected-keys="handleSelectedKeysUpdate"
        >
          <!--
            名称下一行写协议和状态（协议、状态两列并进来，靠筛选标签筛）：正常不写，
            停用写「· 停用」，已过期红字「· 已过期」——常态不占地方，异常才出声
          -->
          <template #cell-name="{ value, row }">
            <div class="min-w-0 max-w-[10rem]">
              <div class="truncate font-medium text-af-ink" :title="value">{{ value }}</div>
              <div class="flex items-center gap-1 text-xs text-af-ink-3">
                <span v-if="row.protocol">{{ row.protocol.toUpperCase() }}</span>
                <template v-if="row.status && row.status !== 'active'">
                  <span v-if="row.protocol" class="text-af-ink-4">·</span>
                  <span :class="row.status === 'expired' ? 'text-af-danger' : 'text-af-ink-2'">{{ statusLabel(row.status) }}</span>
                </template>
              </div>
            </div>
          </template>

          <!-- 地址：等宽 + 复制（左键复制完整 URL，右键选格式）；下一行写认证（用户名 + 可显示的密码） -->
          <template #cell-address="{ row }">
            <div class="min-w-0">
              <div class="flex items-center gap-1.5">
                <code class="max-w-[14rem] truncate font-mono text-13 text-af-ink" :title="`${row.host}:${row.port}`">{{ row.host }}:{{ row.port }}</code>
                <PopoverMenu
                  :ref="(el) => registerCopyMenu(row.id, el)"
                  align="start"
                  width-class="w-auto min-w-[180px] max-w-md"
                >
                  <template #trigger>
                    <button
                      type="button"
                      class="rounded p-0.5 text-af-ink-4 transition-colors hover:text-af-ink-2"
                      :title="t('admin.proxies.copyProxyUrl')"
                      :aria-label="t('admin.proxies.copyProxyUrl')"
                      @click.stop="copyProxyUrl(row)"
                      @contextmenu.prevent="openCopyMenu(row.id)"
                    >
                      <Icon name="copy" size="sm" />
                    </button>
                  </template>
                  <MenuItem v-for="fmt in getCopyFormats(row)" :key="fmt.label" @click="copyFormat(fmt.value)">
                    <span class="font-mono text-xs">{{ fmt.label }}</span>
                  </MenuItem>
                </PopoverMenu>
              </div>
              <div v-if="row.username || row.password" class="mt-0.5 flex items-center gap-1.5 text-xs text-af-ink-3">
                <span v-if="row.username">{{ row.username }}</span>
                <span v-if="row.username && row.password" class="text-af-ink-4">·</span>
                <span v-if="row.password" class="font-mono">
                  {{ visiblePasswordIds.has(row.id) ? row.password : '••••••' }}
                </span>
                <button
                  v-if="row.password"
                  type="button"
                  class="rounded p-0.5 text-af-ink-4 hover:text-af-ink-2"
                  :aria-label="t('admin.proxies.password')"
                  @click.stop="visiblePasswordIds.has(row.id) ? visiblePasswordIds.delete(row.id) : visiblePasswordIds.add(row.id)"
                >
                  <Icon :name="visiblePasswordIds.has(row.id) ? 'eyeOff' : 'eye'" size="xs" />
                </button>
              </div>
            </div>
          </template>

          <template #cell-location="{ row }">
            <div v-if="formatLocation(row) || row.country_code" class="flex max-w-[8rem] items-center gap-2">
              <img
                v-if="row.country_code"
                :src="flagUrl(row.country_code)"
                :alt="row.country || row.country_code"
                class="h-3.5 w-5 shrink-0 rounded-sm"
              />
              <span class="truncate text-af-ink-2" :title="formatLocation(row)">{{ formatLocation(row) }}</span>
            </div>
            <span v-else class="text-af-ink-4">-</span>
          </template>

          <!-- 账号数：有账号时可点开看是哪些账号 -->
          <template #cell-account_count="{ row, value }">
            <button
              v-if="(value || 0) > 0"
              type="button"
              class="tabular-nums text-af-ink underline decoration-dashed decoration-af-ink-4 underline-offset-4 transition-colors hover:text-af-brand-hover"
              :title="t('admin.proxies.accountsTitle', { name: row.name })"
              @click="openAccountsModal(row)"
            >
              {{ value }}
            </button>
            <span v-else class="tabular-nums text-af-ink-4">0</span>
          </template>

          <!-- 延迟：失败红字、≥200ms 黄字，其余常态；下一行是质量检测结果 -->
          <template #cell-latency="{ row }">
            <div class="flex flex-col gap-0.5">
              <span
                v-if="testingProxyIds.has(row.id) || qualityCheckingProxyIds.has(row.id)"
                class="inline-flex items-center gap-1.5 text-af-ink-3"
              >
                <Icon name="refresh" size="xs" class="animate-spin" />
                {{ t('admin.proxies.testing') }}
              </span>
              <span
                v-else-if="row.latency_status === 'failed'"
                class="text-af-danger"
                :title="row.latency_message || undefined"
              >
                {{ t('admin.proxies.latencyFailed') }}
              </span>
              <span
                v-else-if="typeof row.latency_ms === 'number'"
                :class="['tabular-nums', row.latency_ms < 200 ? 'text-af-ink-2' : 'text-af-warning']"
              >
                {{ row.latency_ms }}ms
              </span>
              <span v-else class="text-af-ink-4">-</span>
              <span
                v-if="typeof row.quality_checked === 'number'"
                class="text-xs"
                :class="qualityOverallTextClass(row.quality_status)"
                :title="row.quality_summary || undefined"
              >
                {{ t('admin.proxies.qualityInline', { grade: row.quality_grade || '-', score: row.quality_score ?? '-' }) }}
                · {{ qualityOverallLabel(row.quality_status) }}
              </span>
            </div>
          </template>

          <template #cell-expiry="{ row }">
            <span v-if="!row.expires_at" class="text-af-ink-3">{{ t('admin.proxies.neverExpires') }}</span>
            <div v-else class="flex flex-col">
              <span class="tabular-nums text-af-ink-2" :title="formatDateTime(row.expires_at)">{{ formatDateOnly(row.expires_at) }}</span>
              <span class="text-xs" :class="expiryTextClass(row)">{{ expiryLabel(row) }}</span>
            </div>
          </template>

          <template #cell-created_at="{ row }">
            <span class="tabular-nums text-af-ink-3" :title="formatDateTime(row.created_at)">{{ formatDateOnly(row.created_at) }}</span>
          </template>

          <template #cell-actions="{ row }">
            <RowActions :actions="rowActions(row)" />
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.proxies.noProxiesYet')"
              :description="t('admin.proxies.createFirstProxy')"
              :action-text="t('admin.proxies.createProxy')"
              @action="showCreateModal = true"
            />
          </template>
        </DataTable>
        </div>
      </template>

      <template #bulk>
        <BulkBar :count="selectedCount" @clear="clearSelectedProxies">
          <button
            type="button"
            class="bulk-btn"
            data-test="bulk-test-proxies"
            :disabled="batchTesting || loading"
            @click="handleBatchTest('selected')"
          >
            <Icon v-if="batchTesting" name="refresh" size="xs" class="animate-spin" />
            {{ t('admin.proxies.testConnection') }}
          </button>
          <button
            type="button"
            class="bulk-btn"
            data-test="bulk-quality-proxies"
            :disabled="batchQualityChecking || loading"
            @click="handleBatchQualityCheck('selected')"
          >
            <Icon v-if="batchQualityChecking" name="refresh" size="xs" class="animate-spin" />
            {{ t('admin.proxies.qualityCheck') }}
          </button>
          <button
            type="button"
            class="bulk-btn bulk-btn-danger"
            data-test="bulk-delete-proxies"
            @click="openBatchDelete"
          >
            {{ t('common.delete') }}
          </button>
        </BulkBar>
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

    <ProxyCreateDialog
      :show="showCreateModal"
      :backup-proxies="allProxiesForBackup"
      @close="showCreateModal = false"
      @created="loadProxies"
    />

    <ProxyEditDialog
      :show="showEditModal"
      :proxy="editingProxy"
      :backup-proxies="allProxiesForBackup"
      @close="closeEditModal"
      @updated="loadProxies"
    />

    <!-- Delete Confirmation Dialog -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.proxies.deleteProxy')"
      :message="t('admin.proxies.deleteConfirm', { name: deletingProxy?.name })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />

    <!-- Batch Delete Confirmation Dialog -->
    <ConfirmDialog
      :show="showBatchDeleteDialog"
      :title="t('admin.proxies.batchDelete')"
      :message="t('admin.proxies.batchDeleteConfirm', { count: selectedCount })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmBatchDelete"
      @cancel="showBatchDeleteDialog = false"
    />
    <ConfirmDialog
      :show="showExportDataDialog"
      :title="t('admin.proxies.dataExport')"
      :message="t('admin.proxies.dataExportConfirmMessage')"
      :confirm-text="t('admin.proxies.dataExportConfirm')"
      :cancel-text="t('common.cancel')"
      @confirm="handleExportData"
      @cancel="showExportDataDialog = false"
    />

    <ImportDataModal
      :show="showImportData"
      @close="showImportData = false"
      @imported="handleDataImported"
    />

    <BaseDialog
      :show="showQualityReportDialog"
      :title="t('admin.proxies.qualityReportTitle')"
      width="normal"
      @close="closeQualityReportDialog"
    >
      <div v-if="qualityReport" class="space-y-4">
        <div class="rounded-lg border border-af-hairline bg-af-sunken p-4">
          <div class="flex items-center justify-between gap-4">
            <div>
              <div class="text-sm text-af-ink-3">
                {{ qualityReportProxy?.name || '-' }}
              </div>
              <div class="mt-1 text-sm text-af-ink-2">
                {{ qualityReport.summary }}
              </div>
            </div>
            <div class="text-right">
              <div class="text-2xl font-semibold text-af-ink">
                {{ qualityReport.score }}
              </div>
              <div class="text-xs text-af-ink-3">
                {{ t('admin.proxies.qualityGrade', { grade: qualityReport.grade }) }}
              </div>
            </div>
          </div>
          <div class="mt-3 grid grid-cols-2 gap-2 text-xs text-af-ink-2">
            <div>{{ t('admin.proxies.qualityExitIP') }}: {{ qualityReport.exit_ip || '-' }}</div>
            <div>{{ t('admin.proxies.qualityCountry') }}: {{ qualityReport.country || '-' }}</div>
            <div>
              {{ t('admin.proxies.qualityBaseLatency') }}:
              {{ typeof qualityReport.base_latency_ms === 'number' ? `${qualityReport.base_latency_ms}ms` : '-' }}
            </div>
            <div>{{ t('admin.proxies.qualityCheckedAt') }}: {{ new Date(qualityReport.checked_at * 1000).toLocaleString() }}</div>
          </div>
        </div>

        <div class="max-h-80 overflow-auto rounded-lg border border-af-hairline">
          <table class="min-w-full divide-y divide-af-hairline text-sm">
            <thead class="bg-af-sunken text-xs uppercase text-af-ink-3">
              <tr>
                <th class="whitespace-nowrap px-3 py-2 text-left">{{ t('admin.proxies.qualityTableTarget') }}</th>
                <th class="whitespace-nowrap px-3 py-2 text-left">{{ t('admin.proxies.qualityTableStatus') }}</th>
                <th class="whitespace-nowrap px-3 py-2 text-left">HTTP</th>
                <th class="whitespace-nowrap px-3 py-2 text-left">{{ t('admin.proxies.qualityTableLatency') }}</th>
                <th class="px-3 py-2 text-left">{{ t('admin.proxies.qualityTableMessage') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-af-hairline bg-af-sheet">
              <tr v-for="item in qualityReport.items" :key="item.target">
                <td class="whitespace-nowrap px-3 py-2 text-af-ink">{{ qualityTargetLabel(item.target) }}</td>
                <td class="whitespace-nowrap px-3 py-2">
                  <span class="badge whitespace-nowrap" :class="qualityStatusClass(item.status)">{{ qualityStatusLabel(item.status) }}</span>
                </td>
                <td class="whitespace-nowrap px-3 py-2 text-af-ink-2">{{ item.http_status ?? '-' }}</td>
                <td class="whitespace-nowrap px-3 py-2 text-af-ink-2">
                  {{ typeof item.latency_ms === 'number' ? `${item.latency_ms}ms` : '-' }}
                </td>
                <td class="px-3 py-2 text-af-ink-2">
                  <span>{{ item.message || '-' }}</span>
                  <span v-if="item.cf_ray" class="ml-1 text-xs text-af-ink-3">(cf-ray: {{ item.cf_ray }})</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end">
          <button @click="closeQualityReportDialog" class="btn btn-secondary">
            {{ t('common.close') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Proxy Accounts Dialog -->
    <BaseDialog
      :show="showAccountsModal"
      :title="t('admin.proxies.accountsTitle', { name: accountsProxy?.name || '' })"
      width="normal"
      @close="closeAccountsModal"
    >
      <div v-if="accountsLoading" class="flex items-center justify-center py-8 text-sm text-af-ink-3">
        <Icon name="refresh" size="md" class="mr-2 animate-spin" />
        {{ t('common.loading') }}
      </div>
      <div v-else-if="proxyAccounts.length === 0" class="py-6 text-center text-sm text-af-ink-3">
        {{ t('admin.proxies.accountsEmpty') }}
      </div>
      <div v-else class="max-h-80 overflow-auto">
        <table class="min-w-full divide-y divide-af-hairline text-sm">
          <thead class="bg-af-sunken text-xs uppercase text-af-ink-3">
            <tr>
              <th class="px-4 py-2 text-left">{{ t('admin.proxies.accountName') }}</th>
              <th class="px-4 py-2 text-left">{{ t('admin.accounts.columns.platformType') }}</th>
              <th class="px-4 py-2 text-left">{{ t('admin.proxies.accountNotes') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-af-hairline bg-af-sheet">
            <tr v-for="account in proxyAccounts" :key="account.id">
              <td class="px-4 py-2 font-medium text-af-ink">{{ account.name }}</td>
              <td class="px-4 py-2">
                <PlatformTypeBadge :platform="account.platform" :type="account.type" />
              </td>
              <td class="px-4 py-2 text-af-ink-2">
                {{ account.notes || '-' }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <template #footer>
        <div class="flex justify-end">
          <button @click="closeAccountsModal" class="btn btn-secondary">
            {{ t('common.close') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { Proxy, ProxyAccountSummary, ProxyQualityCheckResult } from '@/types'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import ImportDataModal from '@/components/admin/proxy/ImportDataModal.vue'
import ProxyCreateDialog from '@/components/admin/proxy/ProxyCreateDialog.vue'
import ProxyEditDialog from '@/components/admin/proxy/ProxyEditDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import { BulkBar, FilterChip, ListToolbar, MenuItem, PopoverMenu, RowActions } from '@/components/admin/list'
import type { RowAction } from '@/components/admin/list'
import { useClipboard } from '@/composables/useClipboard'
import { useSwipeSelect } from '@/composables/useSwipeSelect'
import { useTableSelection } from '@/composables/useTableSelection'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { formatDateOnly, formatDateTime } from '@/utils/format'
import { EXPIRY_DANGER_DAYS, EXPIRY_WARN_DAYS, daysUntil, proxyExpiryLabelKey } from '@/utils/proxyExpiry'

const { t } = useI18n()
const { copyToClipboard } = useClipboard()

// 协议、状态并进名称列，认证并进地址列（A4：默认列在 1440 宽下不用横向滚动）
const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.proxies.columns.name'), sortable: true },
  { key: 'address', label: t('admin.proxies.columns.address'), sortable: false },
  { key: 'location', label: t('admin.proxies.columns.location'), sortable: false },
  { key: 'account_count', label: t('admin.proxies.columns.accounts'), sortable: true },
  { key: 'latency', label: t('admin.proxies.columns.latency'), sortable: false },
  { key: 'expiry', label: t('admin.proxies.columns.expiry'), sortable: true },
  { key: 'created_at', label: t('admin.proxies.columns.createdAt'), sortable: true },
  { key: 'actions', label: t('admin.proxies.columns.actions'), sortable: false }
])

// 筛选标签的选项（标签自带「全部」）
const protocolOptions = computed(() => [
  { value: 'http', label: 'HTTP' },
  { value: 'https', label: 'HTTPS' },
  { value: 'socks5', label: 'SOCKS5' },
  { value: 'socks5h', label: 'SOCKS5H' }
])

const statusOptions = computed(() => [
  { value: 'active', label: t('admin.accounts.status.active') },
  { value: 'inactive', label: t('admin.accounts.status.inactive') },
  { value: 'expired', label: t('admin.proxies.expired') }
])

const statusLabel = (status: string) =>
  status === 'expired' ? t('admin.proxies.expired') : t('admin.accounts.status.' + status)

const proxies = ref<Proxy[]>([])
const visiblePasswordIds = reactive(new Set<number>())
const loading = ref(false)
const searchQuery = ref('')
const filters = reactive({
  protocol: '',
  status: ''
})
const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})
const sortState = reactive({
  sort_by: 'id',
  sort_order: 'desc' as 'asc' | 'desc'
})

const showCreateModal = ref(false)
const showEditModal = ref(false)
const showImportData = ref(false)
const showDeleteDialog = ref(false)
const showBatchDeleteDialog = ref(false)
const showExportDataDialog = ref(false)
const showAccountsModal = ref(false)
const exportingData = ref(false)
const testingProxyIds = ref<Set<number>>(new Set())
const qualityCheckingProxyIds = ref<Set<number>>(new Set())
const batchTesting = ref(false)
const batchQualityChecking = ref(false)
const proxyTableRef = ref<HTMLElement | null>(null)
const {
  selectedSet: selectedProxyIds,
  selectedIds,
  selectedCount,
  isSelected,
  select,
  deselect,
  setSelectedIds,
  clear: clearSelectedProxies,
  removeMany: removeSelectedProxies,
  batchUpdate
} = useTableSelection<Proxy>({
  rows: proxies,
  getId: (proxy) => proxy.id
})
useSwipeSelect(proxyTableRef, {
  isSelected,
  select,
  deselect,
  batchUpdate
})

const handleSelectedKeysUpdate = (keys: Array<string | number>) => {
  setSelectedIds(keys.filter((key): key is number => typeof key === 'number'))
}

const getProxySelectionLabel = (proxy: Proxy) => proxy.name

const accountsProxy = ref<Proxy | null>(null)
const proxyAccounts = ref<ProxyAccountSummary[]>([])
const accountsLoading = ref(false)
const editingProxy = ref<Proxy | null>(null)
const deletingProxy = ref<Proxy | null>(null)
const showQualityReportDialog = ref(false)
const qualityReportProxy = ref<Proxy | null>(null)
const qualityReport = ref<ProxyQualityCheckResult | null>(null)

// 「到期回退 → 指定备用代理」的候选：全部启用中的代理
const allProxiesForBackup = ref<Proxy[]>([])
const loadBackupProxyOptions = async () => {
  allProxiesForBackup.value = await adminAPI.proxies.getAllWithCount()
}

let abortController: AbortController | null = null

const isAbortError = (error: unknown) => {
  if (!error || typeof error !== 'object') return false
  const maybeError = error as { name?: string; code?: string }
  return maybeError.name === 'AbortError' || maybeError.code === 'ERR_CANCELED'
}

const buildProxyQueryFilters = () => ({
  protocol: filters.protocol || undefined,
  status: (filters.status || undefined) as 'active' | 'inactive' | 'expired' | undefined,
  search: searchQuery.value || undefined,
  sort_by: sortState.sort_by,
  sort_order: sortState.sort_order
})

const loadProxies = async () => {
  if (abortController) {
    abortController.abort()
  }
  const currentAbortController = new AbortController()
  abortController = currentAbortController
  loading.value = true
  try {
    const response = await adminAPI.proxies.list(
      pagination.page,
      pagination.page_size,
      buildProxyQueryFilters(),
      { signal: currentAbortController.signal }
    )
    if (currentAbortController.signal.aborted || abortController !== currentAbortController) {
      return
    }
    proxies.value = response.items
    pagination.total = response.total
    pagination.pages = response.pages
  } catch (error) {
    if (isAbortError(error)) {
      return
    }
    console.error('Error loading proxies:', error)
  } finally {
    if (abortController === currentAbortController) {
      loading.value = false
      abortController = null
    }
  }
}

const handleFilterChange = () => {
  pagination.page = 1
  loadProxies()
}

let searchTimeout: ReturnType<typeof setTimeout>
const handleSearch = () => {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    loadProxies()
  }, 300)
}

const handlePageChange = (page: number) => {
  pagination.page = page
  loadProxies()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  loadProxies()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadProxies()
}

// 行操作（A4）：编辑是图标；测试连接、质量检测、删除进「⋯」，删除红字（已有账号在用时照旧提示不能删）
const rowActions = (proxy: Proxy): RowAction[] => [
  { key: 'edit', label: t('common.edit'), icon: 'edit', primary: true, onSelect: () => handleEdit(proxy) },
  {
    key: 'test',
    label: t('admin.proxies.testConnection'),
    icon: 'play',
    disabled: testingProxyIds.value.has(proxy.id),
    onSelect: () => handleTestConnection(proxy)
  },
  {
    key: 'quality',
    label: t('admin.proxies.qualityCheck'),
    icon: 'shield',
    disabled: qualityCheckingProxyIds.value.has(proxy.id),
    onSelect: () => handleQualityCheck(proxy)
  },
  { key: 'delete', label: t('common.delete'), icon: 'trash', danger: true, dividerBefore: true, onSelect: () => handleDelete(proxy) }
]

const handleDataImported = () => {
  showImportData.value = false
  loadProxies()
}

const handleEdit = (proxy: Proxy) => {
  editingProxy.value = proxy
  showEditModal.value = true
}

const closeEditModal = () => {
  showEditModal.value = false
  editingProxy.value = null
}

const applyLatencyResult = (
  proxyId: number,
  result: {
    success: boolean
    latency_ms?: number
    message?: string
    ip_address?: string
    country?: string
    country_code?: string
    region?: string
    city?: string
  }
) => {
  const target = proxies.value.find((proxy) => proxy.id === proxyId)
  if (!target) return
  if (result.success) {
    target.latency_status = 'success'
    target.latency_ms = result.latency_ms
    target.ip_address = result.ip_address
    target.country = result.country
    target.country_code = result.country_code
    target.region = result.region
    target.city = result.city
  } else {
    target.latency_status = 'failed'
    target.latency_ms = undefined
    target.ip_address = undefined
    target.country = undefined
    target.country_code = undefined
    target.region = undefined
    target.city = undefined
  }
  target.latency_message = result.message
}

const summarizeQualityStatus = (result: ProxyQualityCheckResult): Proxy['quality_status'] => {
  if (result.challenge_count > 0) return 'challenge'
  if (result.failed_count > 0) return 'failed'
  if (result.warn_count > 0) return 'warn'
  return 'healthy'
}

const applyQualityResult = (proxyId: number, result: ProxyQualityCheckResult) => {
  const target = proxies.value.find((proxy) => proxy.id === proxyId)
  if (!target) return
  target.quality_status = summarizeQualityStatus(result)
  target.quality_score = result.score
  target.quality_grade = result.grade
  target.quality_summary = result.summary
  target.quality_checked = result.checked_at
}

const formatLocation = (proxy: Proxy) => {
  const parts = [proxy.country, proxy.city].filter(Boolean) as string[]
  return parts.join(' · ')
}

const flagUrl = (code: string) =>
  `https://unpkg.com/flag-icons/flags/4x3/${code.toLowerCase()}.svg`

const startTestingProxy = (proxyId: number) => {
  testingProxyIds.value = new Set([...testingProxyIds.value, proxyId])
}

const stopTestingProxy = (proxyId: number) => {
  const next = new Set(testingProxyIds.value)
  next.delete(proxyId)
  testingProxyIds.value = next
}

const startQualityCheckingProxy = (proxyId: number) => {
  qualityCheckingProxyIds.value = new Set([...qualityCheckingProxyIds.value, proxyId])
}

const stopQualityCheckingProxy = (proxyId: number) => {
  const next = new Set(qualityCheckingProxyIds.value)
  next.delete(proxyId)
  qualityCheckingProxyIds.value = next
}

const runProxyTest = async (proxyId: number, notify: boolean) => {
  startTestingProxy(proxyId)
  try {
    const result = await adminAPI.proxies.testProxy(proxyId)
    applyLatencyResult(proxyId, result)
    if (notify && !result.success) {
      console.error(result.message || t('admin.proxies.proxyTestFailed'))
    }
    return result
  } catch (error: any) {
    const message = error.response?.data?.detail || t('admin.proxies.failedToTest')
    applyLatencyResult(proxyId, { success: false, message })
    console.error('Error testing proxy:', error)
    return null
  } finally {
    stopTestingProxy(proxyId)
  }
}

const handleTestConnection = async (proxy: Proxy) => {
  await runProxyTest(proxy.id, true)
}

const handleQualityCheck = async (proxy: Proxy) => {
  startQualityCheckingProxy(proxy.id)
  try {
    const result = await adminAPI.proxies.checkProxyQuality(proxy.id)
    qualityReportProxy.value = proxy
    qualityReport.value = result
    showQualityReportDialog.value = true

    const baseStep = result.items.find((item) => item.target === 'base_connectivity')
    if (baseStep && baseStep.status === 'pass') {
      applyLatencyResult(proxy.id, {
        success: true,
        latency_ms: result.base_latency_ms,
        message: result.summary,
        ip_address: result.exit_ip,
        country: result.country,
        country_code: result.country_code
      })
    }
    applyQualityResult(proxy.id, result)
  } catch (error: any) {
    console.error('Error checking proxy quality:', error)
  } finally {
    stopQualityCheckingProxy(proxy.id)
  }
}

const runBatchProxyQualityChecks = async (ids: number[]) => {
  if (ids.length === 0) return

  const concurrency = 3
  let index = 0

  const worker = async () => {
    while (index < ids.length) {
      const current = ids[index]
      index++
      startQualityCheckingProxy(current)
      try {
        const result = await adminAPI.proxies.checkProxyQuality(current)
        const target = proxies.value.find((proxy) => proxy.id === current)
        if (target) {
          const baseStep = result.items.find((item) => item.target === 'base_connectivity')
          if (baseStep && baseStep.status === 'pass') {
            applyLatencyResult(current, {
              success: true,
              latency_ms: result.base_latency_ms,
              message: result.summary,
              ip_address: result.exit_ip,
              country: result.country,
              country_code: result.country_code
            })
          }
        }
        applyQualityResult(current, result)
      } catch (error) {
        console.error(t('admin.proxies.qualityCheckFailed'), error)
      } finally {
        stopQualityCheckingProxy(current)
      }
    }
  }

  const workers = Array.from({ length: Math.min(concurrency, ids.length) }, () => worker())
  await Promise.all(workers)
}

const closeQualityReportDialog = () => {
  showQualityReportDialog.value = false
  qualityReportProxy.value = null
  qualityReport.value = null
}

const qualityStatusClass = (status: string) => {
  if (status === 'pass') return 'badge-success'
  if (status === 'warn') return 'badge-warning'
  if (status === 'challenge') return 'badge-danger'
  return 'badge-danger'
}

const qualityStatusLabel = (status: string) => {
  if (status === 'pass') return t('admin.proxies.qualityStatusPass')
  if (status === 'warn') return t('admin.proxies.qualityStatusWarn')
  if (status === 'challenge') return t('admin.proxies.qualityStatusChallenge')
  return t('admin.proxies.qualityStatusFail')
}

const expiryLabel = (row: Proxy): string => {
  const { key, params } = proxyExpiryLabelKey(row.expires_at, row.status)
  return params ? t(key, params) : t(key)
}

// 到期紧迫度（与 utils/proxyExpiry 同一口径）：已过期 / ≤3 天红字、≤7 天黄字，其余灰字——不做徽章
const expiryTextClass = (row: Proxy): string => {
  if (row.status === 'expired') return 'text-af-danger'
  const d = row.expires_at ? daysUntil(row.expires_at) : Infinity
  if (d <= EXPIRY_DANGER_DAYS) return 'text-af-danger'
  if (d <= EXPIRY_WARN_DAYS) return 'text-af-warning'
  return 'text-af-ink-3'
}

// 行内质量结果：优质是常态（灰字），告警黄字，挑战 / 失败红字
const qualityOverallTextClass = (status?: string) => {
  if (status === 'healthy') return 'text-af-ink-3'
  if (status === 'warn') return 'text-af-warning'
  return 'text-af-danger'
}

const qualityOverallLabel = (status?: string) => {
  if (status === 'healthy') return t('admin.proxies.qualityStatusHealthy')
  if (status === 'warn') return t('admin.proxies.qualityStatusWarn')
  if (status === 'challenge') return t('admin.proxies.qualityStatusChallenge')
  return t('admin.proxies.qualityStatusFail')
}

const qualityTargetLabel = (target: string) => {
  switch (target) {
    case 'base_connectivity':
      return t('admin.proxies.qualityTargetBase')
    case 'openai':
      return 'OpenAI'
    case 'anthropic':
      return 'Anthropic'
    case 'gemini':
      return 'Gemini'
    case 'grok':
      return 'Grok'
    case 'kimi':
      return 'Kimi'
    case 'zhipu':
      return 'Zhipu GLM'
    case 'deepseek':
      return 'DeepSeek'
    case 'minimax':
      return 'MiniMax'
    default:
      return target
  }
}

const fetchAllProxiesForBatch = async (): Promise<Proxy[]> => {
  const pageSize = 200
  const result: Proxy[] = []
  let page = 1
  let totalPages = 1

  while (page <= totalPages) {
    const response = await adminAPI.proxies.list(
      page,
      pageSize,
      {
        protocol: filters.protocol || undefined,
        status: filters.status as any,
        search: searchQuery.value || undefined,
        sort_by: sortState.sort_by,
        sort_order: sortState.sort_order
      }
    )
    result.push(...response.items)
    totalPages = response.pages || 1
    page++
  }

  return result
}

const runBatchProxyTests = async (ids: number[]) => {
  if (ids.length === 0) return
  const concurrency = 5
  let index = 0

  const worker = async () => {
    while (index < ids.length) {
      const current = ids[index]
      index++
      await runProxyTest(current, false)
    }
  }

  const workers = Array.from({ length: Math.min(concurrency, ids.length) }, () => worker())
  await Promise.all(workers)
}

/**
 * 批量测试 / 质量检测的对象：批量条上的按钮只管选中的行；标题右侧「⋯」里的「全部…」
 * 管当前筛选条件下的全部代理（原来工具行按钮「没选中就测全部」的那半）。
 */
type BatchScope = 'selected' | 'all'
const resolveBatchIds = async (scope: BatchScope): Promise<number[]> => {
  if (scope === 'selected') return Array.from(selectedProxyIds.value)
  const allProxies = await fetchAllProxiesForBatch()
  return allProxies.map((proxy) => proxy.id)
}

const handleBatchTest = async (scope: BatchScope) => {
  if (batchTesting.value) return

  batchTesting.value = true
  try {
    const ids = await resolveBatchIds(scope)

    if (ids.length === 0) {
      return
    }

    await runBatchProxyTests(ids)
    loadProxies()
  } catch (error: any) {
    console.error('Error batch testing proxies:', error)
  } finally {
    batchTesting.value = false
  }
}

const handleBatchQualityCheck = async (scope: BatchScope) => {
  if (batchQualityChecking.value) return

  batchQualityChecking.value = true
  try {
    const ids = await resolveBatchIds(scope)

    if (ids.length === 0) {
      return
    }

    await runBatchProxyQualityChecks(ids)
    loadProxies()
  } catch (error: any) {
    console.error('Error batch checking quality:', error)
  } finally {
    batchQualityChecking.value = false
  }
}

const formatExportTimestamp = () => {
  const now = new Date()
  const pad2 = (value: number) => String(value).padStart(2, '0')
  return `${now.getFullYear()}${pad2(now.getMonth() + 1)}${pad2(now.getDate())}${pad2(now.getHours())}${pad2(now.getMinutes())}${pad2(now.getSeconds())}`
}

const handleExportData = async () => {
  if (exportingData.value) return
  exportingData.value = true
  try {
    const dataPayload = await adminAPI.proxies.exportData(
      selectedCount.value > 0
        ? { ids: Array.from(selectedProxyIds.value) }
        : {
            filters: buildProxyQueryFilters()
          }
    )
    const timestamp = formatExportTimestamp()
    const filename = `aiferry-proxy-${timestamp}.json`
    const blob = new Blob([JSON.stringify(dataPayload, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = filename
    link.click()
    URL.revokeObjectURL(url)
  } catch (error: any) {
    console.error(error?.message || t('admin.proxies.dataExportFailed'), error)
  } finally {
    exportingData.value = false
    showExportDataDialog.value = false
  }
}

const handleDelete = (proxy: Proxy) => {
  if ((proxy.account_count || 0) > 0) {
    console.error(t('admin.proxies.deleteBlockedInUse'))
    return
  }
  deletingProxy.value = proxy
  showDeleteDialog.value = true
}

const openBatchDelete = () => {
  if (selectedCount.value === 0) {
    return
  }
  showBatchDeleteDialog.value = true
}

const confirmDelete = async () => {
  if (!deletingProxy.value) return

  try {
    await adminAPI.proxies.delete(deletingProxy.value.id)
    showDeleteDialog.value = false
    removeSelectedProxies([deletingProxy.value.id])
    deletingProxy.value = null
    loadProxies()
  } catch (error: any) {
    console.error('Error deleting proxy:', error)
  }
}

const confirmBatchDelete = async () => {
  const ids = Array.from(selectedProxyIds.value)
  if (ids.length === 0) {
    showBatchDeleteDialog.value = false
    return
  }

  try {
    await adminAPI.proxies.batchDelete(ids)
    clearSelectedProxies()
    showBatchDeleteDialog.value = false
    loadProxies()
  } catch (error: any) {
    console.error('Error batch deleting proxies:', error)
  }
}

const openAccountsModal = async (proxy: Proxy) => {
  accountsProxy.value = proxy
  proxyAccounts.value = []
  accountsLoading.value = true
  showAccountsModal.value = true

  try {
    proxyAccounts.value = await adminAPI.proxies.getProxyAccounts(proxy.id)
  } catch (error: any) {
    console.error('Error loading proxy accounts:', error)
  } finally {
    accountsLoading.value = false
  }
}

const closeAccountsModal = () => {
  showAccountsModal.value = false
  accountsProxy.value = null
  proxyAccounts.value = []
}

// ── Proxy URL copy ──
// 复制按钮：左键复制完整 URL；右键弹出格式菜单（共用 PopoverMenu，挂 body、点外面 / 滚动即关）
function buildAuthPart(row: any): string {
  const user = row.username ? encodeURIComponent(row.username) : ''
  const pass = row.password ? encodeURIComponent(row.password) : ''
  if (user && pass) return `${user}:${pass}@`
  if (user) return `${user}@`
  if (pass) return `:${pass}@`
  return ''
}

function buildProxyUrl(row: any): string {
  return `${row.protocol}://${buildAuthPart(row)}${row.host}:${row.port}`
}

function getCopyFormats(row: any) {
  const hasAuth = row.username || row.password
  const fullUrl = buildProxyUrl(row)
  const formats = [
    { label: fullUrl, value: fullUrl },
  ]
  if (hasAuth) {
    const withoutProtocol = fullUrl.replace(/^[^:]+:\/\//, '')
    formats.push({ label: withoutProtocol, value: withoutProtocol })
  }
  formats.push({ label: `${row.host}:${row.port}`, value: `${row.host}:${row.port}` })
  return formats
}

type CopyMenuHandle = { open: boolean; close: () => void }
const copyMenus = new Map<number, CopyMenuHandle>()
function registerCopyMenu(id: number, el: unknown) {
  if (el) copyMenus.set(id, el as CopyMenuHandle)
  else copyMenus.delete(id)
}

function openCopyMenu(id: number) {
  const menu = copyMenus.get(id)
  if (menu) menu.open = true
}

function copyProxyUrl(row: any) {
  copyToClipboard(buildProxyUrl(row))
  copyMenus.get(row.id)?.close()
}

function copyFormat(value: string) {
  copyToClipboard(value)
}

onMounted(() => {
  loadProxies()
  loadBackupProxyOptions()
})

onUnmounted(() => {
  clearTimeout(searchTimeout)
  abortController?.abort()
})
</script>
