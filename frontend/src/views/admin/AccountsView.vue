<template>
  <!--
    渠道（A5 列表模板）：标题右侧是工具菜单（导入导出、按筛选批量编辑、错误透传、TLS 指纹）与「添加渠道」；
    数字摘要（渠道 / 可调度 / 异常 / 限流中，异常与限流可一键筛选）；工具行 = 搜索 + 筛选标签 + 自动刷新 / 刷新 / 列设置。
    默认 6 列（方案 2026-09-25）：名称（一行名字 + 一行「厂商 · 接入方式」）、状态（一行，异常写原因）、调度、
    今日（请求 · 收入 · 利润）、已上架模型数、最近使用；其余列在列设置里。上游用量窗口和容量列表放不下，在详情抽屉「用量」页签。
    行尾「编辑」图标 + 「⋯」；点整行打开详情抽屉；选中行时出现批量条。新建 / 编辑是弹窗；别处带 ?edit=<id> 跳过来直接打开该渠道的编辑弹窗。
  -->
  <AppLayout>
    <template #header-actions>
      <PopoverMenu width-class="w-56" @open="toolsMenuOpen = true" @close="toolsMenuOpen = false">
        <template #trigger="{ open }">
          <button
            type="button"
            class="btn btn-ghost btn-md px-2.5"
            :class="open ? 'bg-af-sunken text-af-ink' : ''"
            :title="t('common.more')"
            :aria-label="t('common.more')"
            data-testid="accounts-tools"
          >
            <Icon name="more" size="md" />
          </button>
        </template>
        <MenuItem icon="upload" @click="showImportData = true">{{ t('admin.accounts.dataImport') }}</MenuItem>
        <MenuItem icon="download" @click="openExportDataDialog">
          {{ selIds.length ? t('admin.accounts.dataExportSelected') : t('admin.accounts.dataExport') }}
        </MenuItem>
        <MenuItem icon="edit" data-testid="accounts-edit-filtered" @click="openBulkEditFiltered">
          {{ t('admin.accounts.bulkActions.editFiltered') }}
        </MenuItem>
      </PopoverMenu>
      <button type="button" class="btn btn-primary btn-md" data-testid="accounts-create" @click="openCreate">
        <Icon name="plus" size="md" />
        {{ t('admin.accounts.createAccount') }}
      </button>
    </template>

    <TablePageLayout>
      <template v-if="summaryItems" #summary>
        <StatRow :items="summaryItems" data-testid="accounts-summary" />
      </template>

      <template #filters>
        <ListToolbar>
          <AccountTableFilters
            v-model:searchQuery="params.search"
            :filters="params"
            @update:filters="(newFilters) => Object.assign(params, newFilters)"
            @change="debouncedReload"
            @update:searchQuery="debouncedReload"
          />
          <template #end>
            <PopoverMenu
              width-class="w-48"
              :close-on-select="false"
              @open="autoRefreshMenuOpen = true"
              @close="autoRefreshMenuOpen = false"
            >
              <template #trigger="{ open }">
                <button
                  type="button"
                  class="inline-flex h-9 items-center gap-1 rounded-md px-2 text-13 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink"
                  :class="open ? 'bg-af-sunken text-af-ink' : ''"
                  :title="t('admin.accounts.autoRefresh')"
                  :aria-label="t('admin.accounts.autoRefresh')"
                  data-testid="accounts-auto-refresh"
                >
                  <Icon name="clock" size="md" />
                  <span v-if="autoRefreshEnabled" class="tabular-nums">{{ autoRefreshCountdown }}s</span>
                </button>
              </template>
              <MenuItem :checked="autoRefreshEnabled" @click="setAutoRefreshEnabled(!autoRefreshEnabled)">
                {{ t('admin.accounts.enableAutoRefresh') }}
              </MenuItem>
              <MenuItem divider />
              <MenuItem
                v-for="sec in autoRefreshIntervals"
                :key="sec"
                :checked="autoRefreshIntervalSeconds === sec"
                @click="setAutoRefreshInterval(sec)"
              >
                {{ autoRefreshIntervalLabel(sec) }}
              </MenuItem>
            </PopoverMenu>
            <button
              type="button"
              class="rounded-md p-2 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink disabled:opacity-40"
              :disabled="loading"
              :title="t('common.refresh')"
              :aria-label="t('common.refresh')"
              data-testid="accounts-refresh"
              @click="handleManualRefresh"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <ColumnSettingsMenu :settings="columnSettings" />
          </template>
        </ListToolbar>
        <p v-if="hasPendingListSync" class="mt-2 flex flex-wrap items-center gap-2 text-13 text-af-ink-3">
          <span>{{ t('admin.accounts.listPendingSyncHint') }}</span>
          <button type="button" class="font-medium text-af-brand hover:text-af-brand-hover" @click="syncPendingListChanges">
            {{ t('admin.accounts.listPendingSyncAction') }}
          </button>
        </p>
      </template>

      <template #table>
        <div ref="accountTableRef" class="flex min-h-0 flex-1 flex-col overflow-hidden">
        <DataTable
          ref="dataTableRef"
          :columns="cols"
          :data="accounts"
          :loading="loading"
          row-key="id"
          :server-side-sort="true"
          clickable-rows
          :row-class="(row: AccountListItem) => (isSelected(row.id) ? 'row-selected bg-af-sunken' : undefined)"
          @row-click="openDetail"
          @sort="handleSort"
          default-sort-key="name"
          default-sort-order="asc"
          :sort-storage-key="ACCOUNT_SORT_STORAGE_KEY"
          :estimate-row-height="56"
          :overscan="5"
          :virtualize-threshold="50"
        >
          <template #header-select>
            <input
              type="checkbox"
              class="h-4 w-4 cursor-pointer rounded border-af-hairline-strong accent-af-brand focus:ring-af-brand"
              :checked="allVisibleSelected"
              :aria-label="t('common.selectAll')"
              @click.stop
              @change="toggleSelectAllVisible($event)"
            />
          </template>
          <template #cell-select="{ row }">
            <input
              type="checkbox"
              :checked="isSelected(row.id)"
              :aria-label="row.name"
              class="h-4 w-4 cursor-pointer rounded border-af-hairline-strong accent-af-brand focus:ring-af-brand"
              @click.stop
              @change="toggleSel(row.id)"
            />
          </template>
          <template #cell-name="{ row, value }">
            <div class="min-w-0 max-w-[22rem]">
              <a
                v-if="accountHomepageUrl(row)"
                :href="accountHomepageUrl(row)"
                target="_blank"
                rel="noopener noreferrer"
                class="block truncate font-medium text-af-ink hover:underline"
                :title="accountHomepageUrl(row)"
                @click.stop
              >
                {{ value }}
              </a>
              <span v-else class="block truncate font-medium text-af-ink">{{ value }}</span>
              <!-- 厂商 · 接入方式；套餐、隐私、到期、邮箱、协议地址在详情抽屉里 -->
              <div class="mt-0.5 flex min-w-0 items-center gap-1 text-xs text-af-ink-3" data-testid="account-vendor-line">
                <PlatformIcon :platform="accountVendor(row).icon" size="xs" />
                <span class="truncate">{{ vendorLabel(row) }} · {{ t(accountAccessKey(row)) }}</span>
              </div>
            </div>
          </template>
          <template #cell-notes="{ value }">
            <span v-if="value" :title="value" class="block max-w-xs truncate text-sm text-af-ink-2">{{ value }}</span>
            <span v-else class="text-sm text-af-ink-4">-</span>
          </template>
          <template #cell-status="{ row }">
            <AccountStatusIndicator :account="row" @show-temp-unsched="handleShowTempUnsched" />
          </template>
          <template #cell-schedulable="{ row }">
            <MiniSwitch
              :model-value="row.schedulable"
              :disabled="togglingSchedulable === row.id"
              :label="row.schedulable ? t('admin.accounts.schedulableEnabled') : t('admin.accounts.schedulableDisabled')"
              data-testid="account-schedulable-toggle"
              @toggle="handleToggleSchedulable(row)"
            />
          </template>
          <template #cell-today="{ row }">
            <AccountTodayStatsCell
              :stats="todayStatsByAccountId[String(row.id)] ?? null"
              :loading="todayStatsLoading"
              :error="todayStatsError"
            />
          </template>
          <template #cell-catalog="{ row }">
            <AccountCatalogCell :entries="catalogEntriesForAccount(row.id)" @open="openDetail(row, 'models')" />
          </template>
          <template #cell-proxy="{ row }">
            <div class="flex flex-col gap-0.5">
              <span v-if="row.proxy" class="text-sm text-af-ink-2">
                {{ row.proxy.name }}<span v-if="row.proxy.country_code" class="text-xs text-af-ink-3"> ({{ row.proxy.country_code }})</span>
              </span>
              <span v-else class="text-sm text-af-ink-4">-</span>
              <span v-if="row.proxy && row.proxy.expires_at" :class="['text-xs', proxyExpiryBadge(row.proxy)]" :title="formatDateTime(row.proxy.expires_at)">
                {{ proxyExpiryText(row.proxy) }}
              </span>
              <span v-if="row.proxy_fallback_origin_id" class="flex items-center gap-1.5 text-xs">
                <span class="text-af-warning" :title="t('admin.accounts.fallbackActiveTip', { origin: row.proxy_fallback_origin_name })">
                  {{ t('admin.accounts.fallbackActive') }}
                </span>
                <button type="button" class="text-af-brand hover:text-af-brand-hover" @click.stop="onRevertFallback(row)">{{ t('admin.accounts.revertProxy') }}</button>
              </span>
            </div>
          </template>
          <template #cell-priority="{ value }">
            <span class="text-sm tabular-nums text-af-ink-2">{{ value }}</span>
          </template>
          <template #cell-last_used_at="{ value }">
            <span class="text-sm text-af-ink-3" :title="value ? formatDateTime(value) : undefined">{{ formatRelativeTime(value) }}</span>
          </template>
          <template #cell-created_at="{ value }">
            <span class="text-sm text-af-ink-3" :title="formatDateTime(value)">{{ formatDateOnly(value) }}</span>
          </template>
          <template #cell-expires_at="{ value }">
            <div class="flex flex-col items-start gap-0.5">
              <span :class="['text-sm', isExpired(value) ? 'text-af-warning' : 'text-af-ink-3']">{{ formatExpiresAt(value) }}</span>
              <span v-if="isExpired(value)" class="text-xs text-af-warning">{{ t('admin.accounts.expired') }}</span>
            </div>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex items-center justify-end gap-0.5" @click.stop>
              <button
                type="button"
                class="rounded-md p-1.5 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink"
                :title="t('common.edit')"
                :aria-label="t('common.edit')"
                data-testid="row-action-edit"
                @click="handleEdit(row)"
              >
                <Icon name="edit" size="sm" />
              </button>
              <button
                type="button"
                class="rounded-md p-1.5 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink"
                :class="menu.show && menu.acc?.id === row.id ? 'bg-af-sunken text-af-ink' : ''"
                :title="t('common.more')"
                :aria-label="t('common.more')"
                :aria-expanded="menu.show && menu.acc?.id === row.id ? 'true' : 'false'"
                data-testid="row-action-more"
                @click="openMenu(row, $event)"
              >
                <Icon name="more" size="sm" />
              </button>
            </div>
          </template>
        </DataTable>
        </div>
      </template>

      <template #bulk>
        <AccountBulkActionsBar
          :selected-ids="selIds"
          :total-results="pagination.total"
          :selecting-all="selectingAllResults"
          :all-results-selected="allResultsSelected"
          @delete="handleBulkDelete"
          @reset-status="handleBulkResetStatus"
          @refresh-token="handleBulkRefreshToken"
          @edit-selected="openBulkEditSelected"
          @edit-filtered="openBulkEditFiltered"
          @clear="clearSelection"
          @select-all-results="handleSelectAllResults"
          @toggle-schedulable="handleBulkToggleSchedulable"
        />
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

    <AccountDetailDrawer
      v-model:tab="detailTab"
      :account="detailAccount"
      :catalog-entries="detailAccount ? catalogEntriesForAccount(detailAccount.id) : []"
      :menu-open="menu.show"
      @close="closeDetail"
      @edit="handleEdit"
      @test="handleTest"
      @toggle-schedulable="handleToggleSchedulable"
      @open-menu="openMenu"
      @diagnose="openCatalogDiagnosis"
      @show-temp-unsched="handleShowTempUnsched"
      @account-updated="handleAccountUpdated"
    />
    <CatalogEntryDiagnosisModal
      :show="diagnosisEntry !== null"
      :entry-id="diagnosisEntry?.id ?? null"
      :model-id="diagnosisEntry?.model_id ?? ''"
      @close="diagnosisEntry = null"
    />
    <CreateAccountModal :show="showCreate" :proxies="proxies" @close="showCreate = false" @created="handleCreated" />
    <EditAccountModal
      :show="showEdit"
      :account="editingAccount"
      :load-error="editLoadError"
      :proxies="proxies"
      @close="closeEdit"
      @updated="handleAccountUpdated"
    />
    <ReAuthAccountModal :show="showReAuth" :account="reAuthAcc" @close="closeReAuthModal" @reauthorized="handleAccountUpdated" />
    <AccountTestModal :show="showTest" :account="testingAcc" @close="closeTestModal" />
    <AccountActionMenu
      :show="menu.show"
      :account="menu.acc"
      :anchor-rect="menu.anchorRect"
      @close="menu.show = false"
      @test="handleTest"
      @stats="handleViewStats"
      @schedule="handleSchedule"
      @duplicate="handleDuplicateAccount"
      @reauth="handleReAuth"
      @refresh-token="handleRefresh"
      @recover-state="handleRecoverState"
      @reset-quota="handleResetQuota"
      @set-privacy="handleSetPrivacy"
      @create-spark-shadow="handleCreateSparkShadow"
      @delete="handleDelete"
    />
    <ImportDataModal :show="showImportData" @close="showImportData = false" @imported="handleDataImported" />
    <BulkEditAccountModal
      :show="showBulkEdit"
      :account-ids="selIds"
      :selected-platforms="selPlatforms"
      :selected-types="selTypes"
      :target="bulkEditTarget ?? undefined"
      :proxies="proxies"
      @close="showBulkEdit = false"
      @updated="handleBulkUpdated"
    />
    <TempUnschedStatusModal :show="showTempUnsched" :account="tempUnschedAcc" @close="showTempUnsched = false" @reset="handleTempUnschedReset" />
    <ConfirmDialog :show="showDeleteDialog" :title="t('admin.accounts.deleteAccount')" :message="t('admin.accounts.deleteConfirm', { name: deletingAcc?.name })" :confirm-text="t('common.delete')" :cancel-text="t('common.cancel')" :danger="true" @confirm="confirmDelete" @cancel="showDeleteDialog = false" />
    <ConfirmDialog :show="showCreateShadowDialog" :title="t('admin.accounts.createSparkShadow')" :message="t('admin.accounts.createSparkShadowConfirm', { name: creatingShadowAcc?.name })" @confirm="confirmCreateSparkShadow" @cancel="showCreateShadowDialog = false" />
    <ConfirmDialog :show="showExportDataDialog" :title="t('admin.accounts.dataExport')" :message="t('admin.accounts.dataExportConfirmMessage')" :confirm-text="t('admin.accounts.dataExportConfirm')" :cancel-text="t('common.cancel')" @confirm="handleExportData" @cancel="showExportDataDialog = false">
      <label class="flex items-center gap-2 text-sm text-af-ink-2">
        <input type="checkbox" class="h-4 w-4 rounded border-af-hairline-strong text-af-brand focus:ring-af-brand" v-model="includeProxyOnExport" />
        <span>{{ t('admin.accounts.dataExportIncludeProxies') }}</span>
      </label>
    </ConfirmDialog>
    <TotpStepUpDialog :controller="accountExportStepUp" />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, toRaw, watch } from 'vue'
import { useIntervalFn } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useRoute, useRouter } from 'vue-router'
import { adminAPI } from '@/api/admin'
import { useTableLoader } from '@/composables/useTableLoader'
import { useSwipeSelect, type SwipeSelectVirtualContext } from '@/composables/useSwipeSelect'
import { useTableSelection } from '@/composables/useTableSelection'
import { useStepUp, isStepUpBlocked, isStepUpCancelled, stepUpBlockReason } from '@/composables/useStepUp'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { BulkEditAccountModal, CreateAccountModal, EditAccountModal, TempUnschedStatusModal } from '@/components/account'
import AccountTableFilters from '@/components/admin/account/AccountTableFilters.vue'
import AccountBulkActionsBar from '@/components/admin/account/AccountBulkActionsBar.vue'
import AccountActionMenu from '@/components/admin/account/AccountActionMenu.vue'
import ImportDataModal from '@/components/admin/account/ImportDataModal.vue'
import ReAuthAccountModal from '@/components/admin/account/ReAuthAccountModal.vue'
import AccountTestModal from '@/components/admin/account/AccountTestModal.vue'
import AccountDetailDrawer from '@/components/admin/account/AccountDetailDrawer.vue'
import type { AccountDetailTab } from '@/components/admin/account/accountDetail'
import { accountHomepageUrl, accountVendor } from '@/components/admin/account/accountDisplay'
import { accountAccessKey } from '@/components/admin/account/accountAccess'
import AccountStatusIndicator from '@/components/account/AccountStatusIndicator.vue'
import AccountTodayStatsCell from '@/components/account/AccountTodayStatsCell.vue'
import AccountCatalogCell from '@/components/account/AccountCatalogCell.vue'
import CatalogEntryDiagnosisModal from '@/components/admin/catalog/CatalogEntryDiagnosisModal.vue'
import type { ModelCatalogEntry } from '@/api/admin/modelCatalog'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import { fetchAllAccountIds } from '@/utils/accountSelection'
import { buildGrokUsageRefreshKey, buildOpenAIUsageRefreshKey } from '@/utils/accountUsageRefresh'
import { formatDateOnly, formatDateTime, formatRelativeTime } from '@/utils/format'
import { proxyExpiryBadgeClass, proxyExpiryLabelKey } from '@/utils/proxyExpiry'
import type { Account, AccountListItem, AccountPlatform, AccountType, DashboardStats, Proxy as AccountProxy, WindowStats } from '@/types'
import StatRow from '@/components/user/shell/StatRow.vue'
import type { StatItem } from '@/components/user/shell/types'
import { ColumnSettingsMenu, ListToolbar, MenuItem, MiniSwitch, PopoverMenu } from '@/components/admin/list'
import { useColumnSettings } from '@/composables/useColumnSettings'

const { t } = useI18n()
const router = useRouter()

const proxies = ref<AccountProxy[]>([])
// 已上架模型：目录条目自带 bindings[]，按 account_id 反查，不需要后端新接口。
const catalogEntries = ref<ModelCatalogEntry[]>([])
const catalogEntriesByAccountID = computed(() => {
  const byAccount = new Map<number, ModelCatalogEntry[]>()
  for (const entry of catalogEntries.value) {
    for (const binding of entry.bindings ?? []) {
      const list = byAccount.get(binding.account_id)
      if (list) list.push(entry)
      else byAccount.set(binding.account_id, [entry])
    }
  }
  return byAccount
})
const catalogEntriesForAccount = (accountID: number): ModelCatalogEntry[] => catalogEntriesByAccountID.value.get(accountID) ?? []

// 新建弹窗第二步会写承接关系：除了重拉渠道列表，「已上架模型」列用的目录也一起重拉
async function reloadCatalogEntries() {
  try {
    catalogEntries.value = await adminAPI.modelCatalog.listEntries()
  } catch (error) {
    console.error('Failed to load model catalog:', error)
  }
}
const handleCreated = () => {
  void reload()
  void reloadCatalogEntries()
}
const vendorLabel = (row: AccountListItem): string => {
  const vendor = accountVendor(row)
  return vendor.labelKey ? t(vendor.labelKey) : vendor.label ?? ''
}
const diagnosisEntry = ref<ModelCatalogEntry | null>(null)
const openCatalogDiagnosis = (entry: ModelCatalogEntry) => {
  diagnosisEntry.value = entry
}
const accountTableRef = ref<HTMLElement | null>(null)
const dataTableRef = ref<InstanceType<typeof DataTable> | null>(null)
type AccountBulkEditTarget =
  | {
      mode: 'selected'
      accountIds: number[]
      selectedPlatforms: AccountPlatform[]
      selectedTypes: AccountType[]
    }
  | {
      mode: 'filtered'
      filters: {
        platform?: string
        type?: string
        status?: string
        search?: string
        privacy_mode?: string
        sort_by?: string
        sort_order?: AccountSortOrder
      }
      previewCount: number
      selectedPlatforms: AccountPlatform[]
      selectedTypes: AccountType[]
    }
const selPlatforms = computed<AccountPlatform[]>(() => {
  const platforms = new Set(
    accounts.value
      .filter(a => isSelected(a.id))
      .map(a => a.platform)
  )
  return [...platforms]
})
const selTypes = computed<AccountType[]>(() => {
  const types = new Set(
    accounts.value
      .filter(a => isSelected(a.id))
      .map(a => a.type)
  )
  return [...types]
})
const showImportData = ref(false)
const showExportDataDialog = ref(false)
const includeProxyOnExport = ref(true)
const showBulkEdit = ref(false)
const bulkEditTarget = ref<AccountBulkEditTarget | null>(null)
const showTempUnsched = ref(false)
const showDeleteDialog = ref(false)
const showCreateShadowDialog = ref(false)
const showReAuth = ref(false)
const showTest = ref(false)
const showCreate = ref(false)
const showEdit = ref(false)
// 编辑弹窗的完整账号（列表行是精简版）：按 id 拉取期间为 null，拉取失败时 editLoadError 有值
const editingAccount = ref<Account | null>(null)
const editLoadError = ref('')
const tempUnschedAcc = ref<Account | null>(null)
const deletingAcc = ref<Account | null>(null)
const creatingShadowAcc = ref<Account | null>(null)
const reAuthAcc = ref<Account | null>(null)
const testingAcc = ref<Account | null>(null)
const togglingSchedulable = ref<number | null>(null)
const menu = reactive<{show:boolean, acc:Account|null, anchorRect:DOMRect|null}>({ show: false, acc: null, anchorRect: null })
const exportingData = ref(false)

// 页头工具菜单 / 自动刷新菜单开着时暂停自动刷新（PopoverMenu 的 open / close 事件回写）
const toolsMenuOpen = ref(false)
const autoRefreshMenuOpen = ref(false)

// Sorting settings
const ACCOUNT_SORT_STORAGE_KEY = 'account-table-sort'
type AccountSortOrder = 'asc' | 'desc'
type AccountSortState = {
  sort_by: string
  sort_order: AccountSortOrder
}
const ACCOUNT_SORTABLE_KEYS = new Set([
  'name',
  'status',
  'schedulable',
  'priority',
  'last_used_at',
  'created_at',
  'expires_at'
])
const loadInitialAccountSortState = (): AccountSortState => {
  const fallback: AccountSortState = { sort_by: 'name', sort_order: 'asc' }
  try {
    const raw = localStorage.getItem(ACCOUNT_SORT_STORAGE_KEY)
    if (!raw) return fallback
    const parsed = JSON.parse(raw) as { key?: string; order?: string }
    const key = typeof parsed.key === 'string' ? parsed.key : ''
    if (!ACCOUNT_SORTABLE_KEYS.has(key)) return fallback
    return {
      sort_by: key,
      sort_order: parsed.order === 'desc' ? 'desc' : 'asc'
    }
  } catch {
    return fallback
  }
}
const sortState = reactive<AccountSortState>(loadInitialAccountSortState())

// Auto refresh settings
const AUTO_REFRESH_STORAGE_KEY = 'account-auto-refresh'
const autoRefreshIntervals = [5, 10, 15, 30] as const
const autoRefreshEnabled = ref(false)
const autoRefreshIntervalSeconds = ref<(typeof autoRefreshIntervals)[number]>(30)
const autoRefreshCountdown = ref(0)
const autoRefreshETag = ref<string | null>(null)
const autoRefreshFetching = ref(false)
const AUTO_REFRESH_SILENT_WINDOW_MS = 15000
const autoRefreshSilentUntil = ref(0)
const hasPendingListSync = ref(false)
const todayStatsByAccountId = ref<Record<string, WindowStats>>({})
const todayStatsLoading = ref(false)
const todayStatsError = ref<string | null>(null)
const todayStatsReqSeq = ref(0)
const pendingTodayStatsRefresh = ref(false)

// 「今日」列（请求 · 收入 · 利润）的数据：按当前页批量拉；列被藏起来时不拉
const refreshTodayStatsBatch = async () => {
  if (!columnSettings.isVisible('today')) {
    todayStatsLoading.value = false
    todayStatsError.value = null
    return
  }

  const accountIDs = accounts.value.map(account => account.id)
  const reqSeq = ++todayStatsReqSeq.value
  if (accountIDs.length === 0) {
    todayStatsByAccountId.value = {}
    todayStatsError.value = null
    todayStatsLoading.value = false
    return
  }

  todayStatsLoading.value = true
  todayStatsError.value = null

  try {
    const result = await adminAPI.accounts.getBatchTodayStats(accountIDs)
    if (reqSeq !== todayStatsReqSeq.value) return
    // 今天没有请求的渠道后端不返回，单元格写「—」
    todayStatsByAccountId.value = result.stats ?? {}
  } catch (error) {
    if (reqSeq !== todayStatsReqSeq.value) return
    todayStatsError.value = t('admin.accounts.today.loadFailed')
    console.error('Failed to load account today stats:', error)
  } finally {
    if (reqSeq === todayStatsReqSeq.value) {
      todayStatsLoading.value = false
    }
  }
}

const autoRefreshIntervalLabel = (sec: number) => {
  if (sec === 5) return t('admin.accounts.refreshInterval5s')
  if (sec === 10) return t('admin.accounts.refreshInterval10s')
  if (sec === 15) return t('admin.accounts.refreshInterval15s')
  if (sec === 30) return t('admin.accounts.refreshInterval30s')
  return `${sec}s`
}

const loadSavedAutoRefresh = () => {
  try {
    const saved = localStorage.getItem(AUTO_REFRESH_STORAGE_KEY)
    if (!saved) return
    const parsed = JSON.parse(saved) as { enabled?: boolean; interval_seconds?: number }
    autoRefreshEnabled.value = parsed.enabled === true
    const interval = Number(parsed.interval_seconds)
    if (autoRefreshIntervals.includes(interval as any)) {
      autoRefreshIntervalSeconds.value = interval as any
    }
  } catch (e) {
    console.error('Failed to load saved auto refresh settings:', e)
  }
}

const saveAutoRefreshToStorage = () => {
  try {
    localStorage.setItem(
      AUTO_REFRESH_STORAGE_KEY,
      JSON.stringify({
        enabled: autoRefreshEnabled.value,
        interval_seconds: autoRefreshIntervalSeconds.value
      })
    )
  } catch (e) {
    console.error('Failed to save auto refresh settings:', e)
  }
}

if (typeof window !== 'undefined') {
  loadSavedAutoRefresh()
}

const setAutoRefreshEnabled = (enabled: boolean) => {
  autoRefreshEnabled.value = enabled
  saveAutoRefreshToStorage()
  if (enabled) {
    autoRefreshCountdown.value = autoRefreshIntervalSeconds.value
    resumeAutoRefresh()
  } else {
    pauseAutoRefresh()
    autoRefreshCountdown.value = 0
  }
}

const setAutoRefreshInterval = (seconds: (typeof autoRefreshIntervals)[number]) => {
  autoRefreshIntervalSeconds.value = seconds
  saveAutoRefreshToStorage()
  if (autoRefreshEnabled.value) {
    autoRefreshCountdown.value = seconds
  }
}

// 仪表盘「需要处理」带 ?status=error / rate_limited 跳过来：用作初始状态筛选（只认筛选下拉里有的值）
const route = useRoute()
const ACCOUNT_STATUS_FILTER_VALUES = ['active', 'inactive', 'error', 'rate_limited', 'temp_unschedulable', 'unschedulable']
function initialStatusFromQuery(): string {
  const value = route.query.status
  return typeof value === 'string' && ACCOUNT_STATUS_FILTER_VALUES.includes(value) ? value : ''
}

const {
  items: accounts,
  loading,
  params,
  pagination,
  load: baseLoad,
  reload: baseReload,
  debouncedReload: baseDebouncedReload,
  handlePageChange: baseHandlePageChange,
  handlePageSizeChange: baseHandlePageSizeChange
} = useTableLoader<AccountListItem, any>({
  fetchFn: adminAPI.accounts.list,
  initialParams: {
    platform: '',
    type: '',
    status: initialStatusFromQuery(),
    privacy_mode: '',
    search: '',
    lite: '1',
    sort_by: sortState.sort_by,
    sort_order: sortState.sort_order
  }
})

const {
  selectedSet,
  selectedIds: selIds,
  allVisibleSelected,
  isSelected,
  setSelectedIds,
  select,
  deselect,
  toggle: toggleSel,
  clear: clearSelectedIds,
  removeMany: removeSelectedAccounts,
  toggleVisible,
  batchUpdate
} = useTableSelection<AccountListItem>({
  rows: accounts,
  getId: (account) => account.id
})

const selectingAllResults = ref(false)
const selectedAllResultIDs = ref<Set<number> | null>(null)
const selectionRequestVersion = ref(0)
const allResultsSelected = computed(() => {
  const snapshot = selectedAllResultIDs.value
  if (!snapshot || snapshot.size === 0 || snapshot.size !== selectedSet.value.size) return false
  return Array.from(snapshot).every(id => selectedSet.value.has(id))
})

const clearSelection = () => {
  selectionRequestVersion.value++
  selectingAllResults.value = false
  selectedAllResultIDs.value = null
  clearSelectedIds()
}

const swipeVirtualContext: SwipeSelectVirtualContext = {
  getVirtualizer: () => dataTableRef.value?.virtualizer ?? null,
  getSortedData: () => dataTableRef.value?.sortedData ?? accounts.value,
  getRowId: (row: any) => row.id,
}

const { isDragging: swipeDragging } = useSwipeSelect(accountTableRef, {
  isSelected,
  select,
  deselect,
  batchUpdate
}, swipeVirtualContext)
// 拖选结束时鼠标松开会在行上补一次 click，记下时间让 openDetail 忽略它
let swipeDragEndedAt = 0
watch(swipeDragging, (dragging) => {
  if (!dragging) swipeDragEndedAt = Date.now()
})

const resetAutoRefreshCache = () => {
  autoRefreshETag.value = null
}

type AccountLoadOptions = {
  refreshTodayStats?: boolean
}

const load = async (options: AccountLoadOptions = {}) => {
  const requestParams = params as any
  hasPendingListSync.value = false
  resetAutoRefreshCache()
  pendingTodayStatsRefresh.value = false
  requestParams.lite = '1'
  await baseLoad()
  if (options.refreshTodayStats !== false) await refreshTodayStatsBatch()
}

// 增删改之后的重拉：数字摘要一起刷新
const reload = async () => {
  hasPendingListSync.value = false
  resetAutoRefreshCache()
  pendingTodayStatsRefresh.value = false
  void loadSummary()
  await baseReload()
  await refreshTodayStatsBatch()
}

const debouncedReload = () => {
  clearSelection()
  hasPendingListSync.value = false
  resetAutoRefreshCache()
  pendingTodayStatsRefresh.value = true
  baseDebouncedReload()
}

const handlePageChange = (page: number) => {
  hasPendingListSync.value = false
  resetAutoRefreshCache()
  pendingTodayStatsRefresh.value = true
  baseHandlePageChange(page)
}

const handlePageSizeChange = (size: number) => {
  hasPendingListSync.value = false
  resetAutoRefreshCache()
  pendingTodayStatsRefresh.value = true
  baseHandlePageSizeChange(size)
}

const handleSort = (key: string, order: AccountSortOrder) => {
  sortState.sort_by = key
  sortState.sort_order = order
  const requestParams = params as any
  requestParams.sort_by = key
  requestParams.sort_order = order
  pagination.page = 1
  hasPendingListSync.value = false
  resetAutoRefreshCache()
  pendingTodayStatsRefresh.value = true
  load()
}

watch(loading, (isLoading, wasLoading) => {
  if (wasLoading && !isLoading && pendingTodayStatsRefresh.value) {
    pendingTodayStatsRefresh.value = false
    refreshTodayStatsBatch().catch((error) => {
      console.error('Failed to refresh account today stats after table load:', error)
    })
  }
})

const isAnyModalOpen = computed(() => {
  return (
    showImportData.value ||
    showExportDataDialog.value ||
    showBulkEdit.value ||
    showTempUnsched.value ||
    showDeleteDialog.value ||
    showReAuth.value ||
    showTest.value ||
    showCreate.value ||
    showEdit.value
  )
})

const enterAutoRefreshSilentWindow = () => {
  autoRefreshSilentUntil.value = Date.now() + AUTO_REFRESH_SILENT_WINDOW_MS
  autoRefreshCountdown.value = autoRefreshIntervalSeconds.value
}

const inAutoRefreshSilentWindow = () => {
  return Date.now() < autoRefreshSilentUntil.value
}

const shouldReplaceAutoRefreshRow = (current: Account, next: Account) => {
  return (
    current.updated_at !== next.updated_at ||
    current.current_concurrency !== next.current_concurrency ||
    current.active_sessions !== next.active_sessions ||
    current.schedulable !== next.schedulable ||
    current.status !== next.status ||
    current.rate_limit_reset_at !== next.rate_limit_reset_at ||
    current.overload_until !== next.overload_until ||
    current.temp_unschedulable_until !== next.temp_unschedulable_until ||
    buildOpenAIUsageRefreshKey(current) !== buildOpenAIUsageRefreshKey(next) ||
    buildGrokUsageRefreshKey(current) !== buildGrokUsageRefreshKey(next)
  )
}

const syncAccountRefs = (nextAccount: Account) => {
  if (reAuthAcc.value?.id === nextAccount.id) reAuthAcc.value = nextAccount
  if (tempUnschedAcc.value?.id === nextAccount.id) tempUnschedAcc.value = nextAccount
  if (deletingAcc.value?.id === nextAccount.id) deletingAcc.value = nextAccount
  if (menu.acc?.id === nextAccount.id) menu.acc = nextAccount
}

const mergeAccountsIncrementally = (nextRows: Account[]) => {
  const currentRows = accounts.value
  const currentByID = new Map(currentRows.map(row => [row.id, row]))
  let changed = nextRows.length !== currentRows.length
  const mergedRows = nextRows.map((nextRow) => {
    const currentRow = currentByID.get(nextRow.id)
    if (!currentRow) {
      changed = true
      return nextRow
    }
    if (shouldReplaceAutoRefreshRow(currentRow, nextRow)) {
      changed = true
      syncAccountRefs(nextRow)
      return nextRow
    }
    return currentRow
  })
  if (!changed) {
    for (let i = 0; i < mergedRows.length; i += 1) {
      if (mergedRows[i].id !== currentRows[i]?.id) {
        changed = true
        break
      }
    }
  }
  if (changed) {
    accounts.value = mergedRows
  }
}

const refreshAccountsIncrementally = async () => {
  if (autoRefreshFetching.value) return
  autoRefreshFetching.value = true
  try {
    const result = await adminAPI.accounts.listWithEtag(
      pagination.page,
      pagination.page_size,
      toRaw(params) as {
        platform?: string
        type?: string
        status?: string
        privacy_mode?: string
        search?: string
        sort_by?: string
        sort_order?: AccountSortOrder

      },
      { etag: autoRefreshETag.value }
    )

    if (result.etag) {
      autoRefreshETag.value = result.etag
    }
    if (!result.notModified && result.data) {
      pagination.total = result.data.total || 0
      pagination.pages = result.data.pages || 0
      mergeAccountsIncrementally(result.data.items || [])
      hasPendingListSync.value = false
    }

    await Promise.all([refreshTodayStatsBatch(), loadSummary()])
  } catch (error) {
    console.error('Auto refresh failed:', error)
  } finally {
    autoRefreshFetching.value = false
  }
}

// 手动刷新：列表 + 今日 + 数字摘要
const handleManualRefresh = async () => {
  await Promise.all([load(), loadSummary()])
}

const syncPendingListChanges = async () => {
  hasPendingListSync.value = false
  await Promise.all([load(), loadSummary()])
}

const { pause: pauseAutoRefresh, resume: resumeAutoRefresh } = useIntervalFn(
  async () => {
    if (!autoRefreshEnabled.value) return
    if (document.hidden) return
    if (loading.value || autoRefreshFetching.value) return
    if (isAnyModalOpen.value) return
    if (menu.show || toolsMenuOpen.value || autoRefreshMenuOpen.value) return
    if (inAutoRefreshSilentWindow()) {
      autoRefreshCountdown.value = Math.max(
        0,
        Math.ceil((autoRefreshSilentUntil.value - Date.now()) / 1000)
      )
      return
    }

    if (autoRefreshCountdown.value <= 0) {
      autoRefreshCountdown.value = autoRefreshIntervalSeconds.value
      await refreshAccountsIncrementally()
      return
    }

    autoRefreshCountdown.value -= 1
  },
  1000,
  { immediate: false }
)

// 全部列。默认 6 个数据列（方案 2026-09-25）：名称 / 状态 / 调度 / 今日 / 已上架模型 / 最近使用；
// 其余进「列设置」（存在本机）。上游用量窗口、容量在详情抽屉「用量」页签，不再占列。
const allColumns = computed(() => [
  { key: 'select', label: '', sortable: false },
  { key: 'name', label: t('admin.accounts.columns.name'), sortable: true },
  { key: 'status', label: t('admin.accounts.columns.status'), sortable: true },
  { key: 'schedulable', label: t('admin.accounts.columns.schedulable'), sortable: true },
  { key: 'today', label: t('admin.accounts.columns.today'), sortable: false },
  { key: 'catalog', label: t('admin.accounts.columns.catalog'), sortable: false },
  { key: 'last_used_at', label: t('admin.accounts.columns.lastUsed'), sortable: true },
  { key: 'priority', label: t('admin.accounts.columns.priority'), sortable: true },
  { key: 'proxy', label: t('admin.accounts.columns.proxy'), sortable: false },
  { key: 'created_at', label: t('admin.accounts.columns.createdAt'), sortable: true },
  { key: 'expires_at', label: t('admin.accounts.columns.expiresAt'), sortable: true },
  { key: 'notes', label: t('admin.accounts.columns.notes'), sortable: false },
  { key: 'actions', label: t('admin.accounts.columns.actions'), sortable: false }
])

// 版本 2（2026-09-25 默认列改成 6 个）：本机旧设置作废，回到新默认
// 版本 3（2026-09-26 删掉内部 ID 列，管理站不显示数字 ID）
const columnSettings = useColumnSettings({
  storageKey: 'admin-accounts-columns',
  version: 3,
  columns: allColumns,
  defaultHidden: ['priority', 'proxy', 'created_at', 'expires_at', 'notes'],
  alwaysVisible: ['select', 'name', 'actions']
})
const cols = columnSettings.visibleColumns

// 「今日」列藏着时不拉今日统计；重新打开时补拉
watch(
  () => columnSettings.isVisible('today'),
  (visible, wasVisible) => {
    if (visible && !wasVisible) {
      refreshTodayStatsBatch().catch((error) => {
        console.error('Failed to load account today stats after showing column:', error)
      })
    }
  }
)

// 数字摘要：全站渠道计数（仪表盘统计接口）；异常 / 限流有数时可一键筛选
const dashboardStats = ref<DashboardStats | null>(null)
const loadSummary = async () => {
  try {
    dashboardStats.value = await adminAPI.dashboard.getStats()
  } catch {
    dashboardStats.value = null
  }
}
const applyStatusFilter = (status: string) => {
  params.status = status
  debouncedReload()
}
const summaryItems = computed<StatItem[] | null>(() => {
  const stats = dashboardStats.value
  if (!stats) return null
  const fmt = (n: number) => n.toLocaleString()
  const filterAction = (status: string, count: number) =>
    count > 0 && params.status !== status
      ? { label: t('admin.accounts.summary.filter'), onClick: () => applyStatusFilter(status) }
      : undefined
  return [
    { key: 'total', label: t('admin.accounts.summary.total'), value: fmt(stats.total_accounts) },
    { key: 'normal', label: t('admin.accounts.summary.normal'), value: fmt(stats.normal_accounts) },
    {
      key: 'error',
      label: t('admin.accounts.summary.error'),
      value: fmt(stats.error_accounts),
      action: filterAction('error', stats.error_accounts)
    },
    {
      key: 'rate_limited',
      label: t('admin.accounts.summary.rateLimited'),
      value: fmt(stats.ratelimit_accounts),
      action: filterAction('rate_limited', stats.ratelimit_accounts)
    }
  ]
})

const accountDetailLoading = new Set<number>()
const loadAccountDetails = async (account: Pick<AccountListItem, 'id'>): Promise<Account | null> => {
  if (accountDetailLoading.has(account.id)) return null
  accountDetailLoading.add(account.id)
  try {
    return await adminAPI.accounts.getById(account.id)
  } catch (error) {
    console.error('Failed to load account details:', error)
    return null
  } finally {
    accountDetailLoading.delete(account.id)
  }
}

// 新建 / 编辑渠道是弹窗（2026-10-03 由整页改回）；编辑先开弹窗，再按 id 拉完整账号
const openCreate = () => {
  showCreate.value = true
}
let editLoadSeq = 0
const handleEdit = async (a: Pick<AccountListItem, 'id'>) => {
  const seq = ++editLoadSeq
  editingAccount.value = null
  editLoadError.value = ''
  showEdit.value = true
  try {
    const account = await adminAPI.accounts.getById(a.id)
    if (seq === editLoadSeq) editingAccount.value = account
  } catch (error) {
    if (seq !== editLoadSeq) return
    editLoadError.value =
      (error as { status?: number } | null)?.status === 404
        ? t('admin.accounts.dialog.notFound')
        : t('admin.accounts.dialog.loadFailed', { message: extractApiErrorMessage(error, t('common.error')) })
  }
}
const closeEdit = () => {
  editLoadSeq++
  showEdit.value = false
  editingAccount.value = null
  editLoadError.value = ''
}
// 渠道状态页等处带 ?edit=<id> 跳过来：打开该渠道的编辑弹窗，并把参数从地址栏去掉（刷新不再弹）
function openEditFromQuery() {
  const raw = route.query.edit
  if (typeof raw !== 'string') return
  const query = { ...route.query }
  delete query.edit
  void router.replace({ query })
  const id = Number(raw)
  if (Number.isInteger(id) && id > 0) void handleEdit({ id })
}

// 详情抽屉：跟着列表行走（自动刷新 / 本地修补后抽屉里同步变化）；行被筛掉时保留打开时的快照
const detailAccountId = ref<number | null>(null)
const detailSnapshot = ref<AccountListItem | null>(null)
const detailTab = ref<AccountDetailTab>('overview')
const detailAccount = computed<AccountListItem | null>(() => {
  if (detailAccountId.value === null) return null
  return accounts.value.find((account) => account.id === detailAccountId.value) ?? detailSnapshot.value
})
watch(detailAccountId, (id) => {
  if (id === null) detailSnapshot.value = null
})
const openDetail = (row: AccountListItem, tab: AccountDetailTab = 'overview') => {
  // 拖选行、选中文字后的那次 click 不算点行
  if (swipeDragEndedAt && Date.now() - swipeDragEndedAt < 300) return
  if (typeof window !== 'undefined' && window.getSelection()?.toString()) return
  detailSnapshot.value = row
  detailTab.value = tab
  detailAccountId.value = row.id
}
const closeDetail = () => {
  detailAccountId.value = null
}
const openMenu = (a: Account, e: MouseEvent) => {
  menu.acc = a
  const target = e.currentTarget as HTMLElement
  menu.anchorRect = target.getBoundingClientRect()
  menu.show = true
}
const toggleSelectAllVisible = (event: Event) => {
  const target = event.target as HTMLInputElement
  toggleVisible(target.checked)
}
const handleBulkDelete = async () => {
  const accountIds = [...selIds.value]
  if (!confirm(t('admin.accounts.bulkActions.confirmDelete', { count: accountIds.length }))) return
  try {
    const result = await adminAPI.accounts.batchDelete(accountIds)
    if (result.failed > 0) {
      console.error(t('admin.accounts.bulkActions.partialSuccess', {
        success: result.success,
        failed: result.failed
      }))
      setSelectedIds(result.failed_ids?.length ? result.failed_ids : accountIds)
    } else {
      clearSelection()
    }
    await reload()
  } catch (error) {
    console.error('Failed to bulk delete accounts:', error)
  }
}
const handleBulkResetStatus = async () => {
  if (!confirm(t('common.confirm'))) return
  try {
    const result = await adminAPI.accounts.batchClearError(selIds.value)
    if (result.failed > 0) {
      console.error(t('admin.accounts.bulkActions.partialSuccess', { success: result.success, failed: result.failed }))
    } else {
      clearSelection()
    }
    reload()
  } catch (error) {
    console.error('Failed to bulk reset status:', error)
  }
}
const handleBulkRefreshToken = async () => {
  if (!confirm(t('common.confirm'))) return
  const accountIds = [...selIds.value]
  try {
    const result = await adminAPI.accounts.batchRefresh(accountIds)
    if (result.failed > 0) {
      console.error(t('admin.accounts.bulkActions.partialSuccess', { success: result.success, failed: result.failed }))
      const failedIds = result.errors?.map(error => error.account_id) ?? []
      setSelectedIds(failedIds.length > 0 ? failedIds : accountIds)
    } else {
      clearSelection()
    }
    reload()
  } catch (error) {
    console.error('Failed to bulk refresh token:', error)
  }
}
const updateSchedulableInList = (accountIds: number[], schedulable: boolean) => {
  if (accountIds.length === 0) return
  const idSet = new Set(accountIds)
  accounts.value = accounts.value.map((account) => (idSet.has(account.id) ? { ...account, schedulable } : account))
}
const normalizeBulkSchedulableResult = (
  result: {
    success?: number
    failed?: number
    success_ids?: number[]
    failed_ids?: number[]
    results?: Array<{ account_id: number; success: boolean }>
  },
  accountIds: number[]
) => {
  const responseSuccessIds = Array.isArray(result.success_ids) ? result.success_ids : []
  const responseFailedIds = Array.isArray(result.failed_ids) ? result.failed_ids : []
  if (responseSuccessIds.length > 0 || responseFailedIds.length > 0) {
    return {
      successIds: responseSuccessIds,
      failedIds: responseFailedIds,
      successCount: typeof result.success === 'number' ? result.success : responseSuccessIds.length,
      failedCount: typeof result.failed === 'number' ? result.failed : responseFailedIds.length,
      hasIds: true,
      hasCounts: true
    }
  }

  const results = Array.isArray(result.results) ? result.results : []
  if (results.length > 0) {
    const successIds = results.filter(item => item.success).map(item => item.account_id)
    const failedIds = results.filter(item => !item.success).map(item => item.account_id)
    return {
      successIds,
      failedIds,
      successCount: typeof result.success === 'number' ? result.success : successIds.length,
      failedCount: typeof result.failed === 'number' ? result.failed : failedIds.length,
      hasIds: true,
      hasCounts: true
    }
  }

  const hasExplicitCounts = typeof result.success === 'number' || typeof result.failed === 'number'
  const successCount = typeof result.success === 'number' ? result.success : 0
  const failedCount = typeof result.failed === 'number' ? result.failed : 0
  if (hasExplicitCounts && failedCount === 0 && successCount === accountIds.length && accountIds.length > 0) {
    return {
      successIds: accountIds,
      failedIds: [],
      successCount,
      failedCount,
      hasIds: true,
      hasCounts: true
    }
  }

  return {
    successIds: [],
    failedIds: [],
    successCount,
    failedCount,
    hasIds: false,
    hasCounts: hasExplicitCounts
  }
}
const handleBulkToggleSchedulable = async (schedulable: boolean) => {
  const accountIds = [...selIds.value]
  try {
    const result = await adminAPI.accounts.bulkUpdate(accountIds, { schedulable })
    const { successIds, failedIds, successCount, failedCount, hasIds, hasCounts } = normalizeBulkSchedulableResult(result, accountIds)
    if (!hasIds && !hasCounts) {
      console.error(t('admin.accounts.bulkSchedulableResultUnknown'))
      setSelectedIds(accountIds)
      load().catch((error) => {
        console.error('Failed to refresh accounts:', error)
      })
      return
    }
    if (successIds.length > 0) {
      updateSchedulableInList(successIds, schedulable)
    }
    if (failedCount > 0) {
      const message = hasCounts || hasIds
        ? t('admin.accounts.bulkSchedulablePartial', { success: successCount, failed: failedCount })
        : t('admin.accounts.bulkSchedulableResultUnknown')
      console.error(message)
      setSelectedIds(failedIds.length > 0 ? failedIds : accountIds)
    } else {
      if (hasIds) clearSelection()
      else setSelectedIds(accountIds)
    }
  } catch (error) {
    console.error('Failed to bulk toggle schedulable:', error)
  }
}
const buildBulkEditFilterSnapshot = () => {
  const rawParams = toRaw(params) as Record<string, unknown>
  const sortOrder: AccountSortOrder = rawParams.sort_order === 'desc' ? 'desc' : 'asc'
  return {
    platform: typeof rawParams.platform === 'string' ? rawParams.platform : '',
    type: typeof rawParams.type === 'string' ? rawParams.type : '',
    status: typeof rawParams.status === 'string' ? rawParams.status : '',
    search: typeof rawParams.search === 'string' ? rawParams.search : '',
    privacy_mode: typeof rawParams.privacy_mode === 'string' ? rawParams.privacy_mode : '',
    sort_by: typeof rawParams.sort_by === 'string' ? rawParams.sort_by : '',
    sort_order: sortOrder
  }
}

const handleSelectAllResults = async () => {
  if (selectingAllResults.value || pagination.total === 0) return

  const requestVersion = ++selectionRequestVersion.value
  const filters = buildBulkEditFilterSnapshot()
  selectingAllResults.value = true
  try {
    const ids = await fetchAllAccountIds(
      (page, pageSize, requestFilters) => adminAPI.accounts.list(page, pageSize, requestFilters),
      filters
    )
    if (requestVersion !== selectionRequestVersion.value) return

    setSelectedIds(ids)
    selectedAllResultIDs.value = new Set(ids)
  } catch (error) {
    if (requestVersion !== selectionRequestVersion.value) return
    console.error('Failed to select all account results:', error)
  } finally {
    if (requestVersion === selectionRequestVersion.value) {
      selectingAllResults.value = false
    }
  }
}

const collectSelectionMetadata = (rows: Account[]) => {
  const selectedPlatforms = Array.from(new Set(rows.map(account => account.platform)))
  const selectedTypes = Array.from(new Set(rows.map(account => account.type)))
  return { selectedPlatforms, selectedTypes }
}

const openBulkEditSelected = () => {
  bulkEditTarget.value = {
    mode: 'selected',
    accountIds: [...selIds.value],
    selectedPlatforms: [...selPlatforms.value],
    selectedTypes: [...selTypes.value]
  }
  showBulkEdit.value = true
}

const openBulkEditFiltered = async () => {
  const filters = buildBulkEditFilterSnapshot()
  const preview = await adminAPI.accounts.list(1, 100, filters)
  const { selectedPlatforms, selectedTypes } = collectSelectionMetadata(preview.items)
  bulkEditTarget.value = {
    mode: 'filtered',
    filters,
    previewCount: preview.total,
    selectedPlatforms,
    selectedTypes
  }
  showBulkEdit.value = true
}

const handleBulkUpdated = () => {
  showBulkEdit.value = false
  bulkEditTarget.value = null
  clearSelection()
  reload()
}
const handleDataImported = () => { showImportData.value = false; reload() }
const ACCOUNT_PRIVACY_MODE_UNSET_QUERY_VALUE = '__unset__'
const buildAccountQueryFilters = () => ({
  platform: params.platform || '',
  type: params.type || '',
  status: params.status || '',
  privacy_mode: params.privacy_mode || '',
  search: params.search || '',
  sort_by: sortState.sort_by,
  sort_order: sortState.sort_order
})
const accountMatchesCurrentFilters = (account: Account) => {
  const filters = buildAccountQueryFilters()
  if (filters.platform && account.platform !== filters.platform) return false
  if (filters.type && account.type !== filters.type) return false
  if (filters.status) {
    const now = Date.now()
    const rateLimitResetAt = account.rate_limit_reset_at ? new Date(account.rate_limit_reset_at).getTime() : Number.NaN
    const isRateLimited = Number.isFinite(rateLimitResetAt) && rateLimitResetAt > now
    const tempUnschedUntil = account.temp_unschedulable_until ? new Date(account.temp_unschedulable_until).getTime() : Number.NaN
    const isTempUnschedulable = Number.isFinite(tempUnschedUntil) && tempUnschedUntil > now

    if (filters.status === 'active') {
      if (account.status !== 'active' || isRateLimited || isTempUnschedulable || !account.schedulable) return false
    } else if (filters.status === 'rate_limited') {
      if (account.status !== 'active' || !isRateLimited || isTempUnschedulable) return false
    } else if (filters.status === 'temp_unschedulable') {
      if (account.status !== 'active' || !isTempUnschedulable) return false
    } else if (filters.status === 'unschedulable') {
      if (account.status !== 'active' || account.schedulable || isRateLimited || isTempUnschedulable) return false
    } else if (account.status !== filters.status) {
      return false
    }
  }
  const privacyMode = typeof account.extra?.privacy_mode === 'string' ? account.extra.privacy_mode : ''
  if (filters.privacy_mode) {
    if (filters.privacy_mode === ACCOUNT_PRIVACY_MODE_UNSET_QUERY_VALUE) {
      if (privacyMode.trim() !== '') return false
    } else if (privacyMode !== filters.privacy_mode) {
      return false
    }
  }
  const search = String(filters.search || '').trim().toLowerCase()
  if (search && !account.name.toLowerCase().includes(search)) return false
  return true
}
const mergeRuntimeFields = (oldAccount: Account, updatedAccount: Account): Account => ({
  ...updatedAccount,
  current_concurrency: updatedAccount.current_concurrency ?? oldAccount.current_concurrency,
  active_sessions: updatedAccount.active_sessions ?? oldAccount.active_sessions
})

const syncPaginationAfterLocalRemoval = () => {
  const nextTotal = Math.max(0, pagination.total - 1)
  pagination.total = nextTotal
  pagination.pages = nextTotal > 0 ? Math.ceil(nextTotal / pagination.page_size) : 0

  const maxPage = Math.max(1, pagination.pages || 1)

  if (pagination.page > maxPage) {
    pagination.page = maxPage
  }
  // 行被本地移除后不立刻全量补页，改为提示用户手动同步。
  hasPendingListSync.value = nextTotal > 0
}

const patchAccountInList = (updatedAccount: Account) => {
  const index = accounts.value.findIndex(account => account.id === updatedAccount.id)
  if (index === -1) return
  const mergedAccount = mergeRuntimeFields(accounts.value[index], updatedAccount)
  if (!accountMatchesCurrentFilters(mergedAccount)) {
    accounts.value = accounts.value.filter(account => account.id !== mergedAccount.id)
    syncPaginationAfterLocalRemoval()
    removeSelectedAccounts([mergedAccount.id])
    if (menu.acc?.id === mergedAccount.id) {
      menu.show = false
      menu.acc = null
    }
    return
  }
  const nextAccounts = [...accounts.value]
  nextAccounts[index] = mergedAccount
  accounts.value = nextAccounts
  syncAccountRefs(mergedAccount)
}
const handleAccountUpdated = (updatedAccount: Account) => {
  patchAccountInList(updatedAccount)
  enterAutoRefreshSilentWindow()
}
const formatExportTimestamp = () => {
  const now = new Date()
  const pad2 = (value: number) => String(value).padStart(2, '0')
  return `${now.getFullYear()}${pad2(now.getMonth() + 1)}${pad2(now.getDate())}${pad2(now.getHours())}${pad2(now.getMinutes())}${pad2(now.getSeconds())}`
}
const openExportDataDialog = () => {
  includeProxyOnExport.value = true
  showExportDataDialog.value = true
}
const handleExportData = async () => {
  if (exportingData.value) return
  exportingData.value = true
  try {
    const dataPayload = await accountExportStepUp.run(() => adminAPI.accounts.exportData(
      selIds.value.length > 0
        ? { ids: selIds.value, includeProxies: includeProxyOnExport.value }
        : {
            includeProxies: includeProxyOnExport.value,
            filters: buildAccountQueryFilters()
          }
    ))
    const timestamp = formatExportTimestamp()
    const filename = `aiferry-channels-${timestamp}.json`
    const blob = new Blob([JSON.stringify(dataPayload, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = filename
    link.click()
    URL.revokeObjectURL(url)
    // spark 影子账号被后端排除出备份(其凭据透传母账号、调度配置不可经凭据型导入重建);
    // 跳过非零时记一条日志,避免「下载成功但少了账号」无声无息。
    if (dataPayload.skipped_shadows && dataPayload.skipped_shadows > 0) {
      console.warn(t('admin.accounts.dataExportedSkippedShadows', { count: dataPayload.skipped_shadows }))
    }
  } catch (error: any) {
    if (isStepUpCancelled(error)) {
      // 用户主动取消 step-up 验证，静默返回，不弹错误提示。
    } else if (isStepUpBlocked(error)) {
      console.error(
        stepUpBlockReason(error) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN'
          ? t('stepUp.adminApiKeyForbidden')
          : t('stepUp.notEnabled'),
        error
      )
    } else {
      console.error(error?.message || t('admin.accounts.dataExportFailed'), error)
    }
  } finally {
    exportingData.value = false
    showExportDataDialog.value = false
  }
}
const accountExportStepUp = useStepUp()
const closeTestModal = () => { showTest.value = false; testingAcc.value = null }
const closeReAuthModal = () => { showReAuth.value = false; reAuthAcc.value = null }
const handleTest = async (a: AccountListItem) => {
  const account = await loadAccountDetails(a)
  if (!account) return
  testingAcc.value = account
  showTest.value = true
}
// 「查看统计」「定时测试」在详情抽屉的页签里（A5），不再各开一个对话框
const handleViewStats = (a: AccountListItem) => {
  openDetail(a, 'usage')
}
const handleSchedule = (a: AccountListItem) => {
  openDetail(a, 'schedule')
}
const handleReAuth = (a: Account) => { reAuthAcc.value = a; showReAuth.value = true }
const duplicatingAccountIDs = new Set<number>()
const handleDuplicateAccount = async (a: Account) => {
  if (duplicatingAccountIDs.has(a.id)) return
  duplicatingAccountIDs.add(a.id)
  try {
    await adminAPI.accounts.duplicate(a.id)
    reload()
  } catch (error: any) {
    console.error('Failed to duplicate account:', error)
  } finally {
    duplicatingAccountIDs.delete(a.id)
  }
}
const handleRefresh = async (a: Account) => {
  try {
    const result = await adminAPI.accounts.refreshCredentials(a.id)
    patchAccountInList(result.account)
    enterAutoRefreshSilentWindow()
    if (result.warning) console.warn(result.message)
  } catch (error) {
    console.error('Failed to refresh credentials:', error)
  }
}
const handleRecoverState = async (a: Account) => {
  try {
    const updated = await adminAPI.accounts.recoverState(a.id)
    patchAccountInList(updated)
    enterAutoRefreshSilentWindow()
  } catch (error: any) {
    console.error('Failed to recover account state:', error)
  }
}
const handleResetQuota = async (a: Account) => {
  try {
    const updated = await adminAPI.accounts.resetAccountQuota(a.id)
    patchAccountInList(updated)
    enterAutoRefreshSilentWindow()
  } catch (error) {
    console.error('Failed to reset quota:', error)
  }
}

const privacyResultMessageKey = (account: Account): { type: 'success' | 'error'; key: string } => {
  const mode = typeof account.extra?.privacy_mode === 'string' ? account.extra.privacy_mode : ''
  if (account.platform === 'openai') {
    switch (mode) {
      case 'training_off':
        return { type: 'success', key: 'admin.accounts.privacyTrainingOff' }
      case 'training_set_cf_blocked':
        return { type: 'error', key: 'admin.accounts.privacyCfBlocked' }
      default:
        return { type: 'error', key: 'admin.accounts.privacyFailed' }
    }
  }
  if (account.platform === 'antigravity') {
    if (mode === 'privacy_set') {
      return { type: 'success', key: 'admin.accounts.privacyAntigravitySet' }
    }
    return { type: 'error', key: 'admin.accounts.privacyAntigravityFailed' }
  }
  return { type: 'error', key: 'admin.accounts.privacyFailed' }
}

const handleSetPrivacy = async (a: Account) => {
  try {
    const updated = await adminAPI.accounts.setPrivacy(a.id)
    patchAccountInList(updated)
    enterAutoRefreshSilentWindow()
    const result = privacyResultMessageKey(updated)
    if (result.type === 'error') {
      console.error(t(result.key))
    }
  } catch (error: any) {
    console.error('Failed to set privacy:', error)
  }
}
const onRevertFallback = async (a: Account) => {
  try {
    await adminAPI.accounts.revertProxyFallback(a.id)
    reload()
  } catch (error: any) {
    console.error('Failed to revert proxy fallback:', error)
  }
}
const handleCreateSparkShadow = (a: Account) => {
  creatingShadowAcc.value = a
  showCreateShadowDialog.value = true
}
const confirmCreateSparkShadow = async () => {
  const a = creatingShadowAcc.value
  if (!a) return
  try {
    await adminAPI.accounts.createSparkShadow(a.id, { name: `${a.name} (Spark)` })
    showCreateShadowDialog.value = false
    creatingShadowAcc.value = null
    reload()
  } catch (error: any) {
    console.error('Failed to create spark shadow:', error)
  }
}
const handleDelete = (a: Account) => { deletingAcc.value = a; showDeleteDialog.value = true }
const confirmDelete = async () => { if(!deletingAcc.value) return; try { await adminAPI.accounts.delete(deletingAcc.value.id); showDeleteDialog.value = false; deletingAcc.value = null; reload() } catch (error) { console.error('Failed to delete account:', error) } }
const handleToggleSchedulable = async (a: Account) => {
  const nextSchedulable = !a.schedulable
  togglingSchedulable.value = a.id
  try {
    const updated = await adminAPI.accounts.setSchedulable(a.id, nextSchedulable)
    updateSchedulableInList([a.id], updated?.schedulable ?? nextSchedulable)
    enterAutoRefreshSilentWindow()
  } catch (error) {
    console.error('Failed to toggle schedulable:', error)
  } finally {
    togglingSchedulable.value = null
  }
}
const handleShowTempUnsched = (a: Account) => { tempUnschedAcc.value = a; showTempUnsched.value = true }
const handleTempUnschedReset = async (updated: Account) => {
  showTempUnsched.value = false
  tempUnschedAcc.value = null
  patchAccountInList(updated)
  enterAutoRefreshSilentWindow()
}
const formatExpiresAt = (value: number | null) => {
  if (!value) return '-'
  return formatDateTime(
    new Date(value * 1000),
    {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false
    },
    'sv-SE'
  )
}
const isExpired = (value: number | null) => {
  if (!value) return false
  return value * 1000 <= Date.now()
}
// 所绑定代理的有效期(逻辑同 /admin/proxies,见 utils/proxyExpiry)
const proxyExpiryBadge = (p: AccountProxy): string => proxyExpiryBadgeClass(p.expires_at, p.status)
const proxyExpiryText = (p: AccountProxy): string => {
  const { key, params } = proxyExpiryLabelKey(p.expires_at, p.status)
  return params ? t(key, params) : t(key)
}

// 表格滚动时关闭行操作菜单，并让顶部工具菜单继续贴紧触发按钮。
const handleScroll = (event: Event) => {
  if (event.target instanceof Element && event.target.closest('.action-menu-content')) return
  menu.show = false
}

onMounted(async () => {
  openEditFromQuery()
  load()
  loadSummary()
  const [proxiesResult, catalogResult] = await Promise.allSettled([
    adminAPI.proxies.getAll(),
    adminAPI.modelCatalog.listEntries()
  ])
  if (proxiesResult.status === 'fulfilled') {
    proxies.value = proxiesResult.value
  } else {
    console.error('Failed to load proxies:', proxiesResult.reason)
  }
  if (catalogResult.status === 'fulfilled') {
    catalogEntries.value = catalogResult.value
  } else {
    console.error('Failed to load model catalog:', catalogResult.reason)
  }
  window.addEventListener('scroll', handleScroll, true)

  if (autoRefreshEnabled.value) {
    autoRefreshCountdown.value = autoRefreshIntervalSeconds.value
    resumeAutoRefresh()
  } else {
    pauseAutoRefresh()
  }
})

onUnmounted(() => {
  window.removeEventListener('scroll', handleScroll, true)
})
</script>
