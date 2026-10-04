<template>
  <!--
    订阅（A4 列表模板）：标题右侧「分配订阅」（套餐页签那边是「新建套餐」，同组页头各管各的主操作）；
    工具行 = 用户搜索 + 状态 / 套餐筛选标签 + 刷新 / 用户列显示 / 列设置；
    行尾「调整」图标 + 「⋯」（重置配额、恢复、撤销）；选中行时出现批量条。
  -->
  <AppLayout>
    <template #header-actions>
      <button type="button" class="btn btn-primary btn-md" @click="showAssignModal = true">
        <Icon name="plus" size="md" />
        {{ t('admin.subscriptions.assignSubscription') }}
      </button>
    </template>

    <TablePageLayout>
      <template #filters>
        <ListToolbar>
          <!-- 用户搜索：边输边查（含已删除用户，便于查历史订阅），选中后按用户过滤 -->
          <div class="relative w-full sm:w-64" data-filter-user-search>
            <Icon
              name="search"
              size="sm"
              class="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-af-ink-3"
            />
            <input
              v-model="filterUserKeyword"
              type="text"
              :placeholder="t('admin.usage.searchUserPlaceholder')"
              class="input h-8 py-0 pl-8 pr-8 text-13"
              @input="debounceSearchFilterUsers"
              @focus="showFilterUserDropdown = true"
            />
            <button
              v-if="selectedFilterUser"
              @click="clearFilterUser"
              type="button"
              class="absolute right-2 top-1/2 -translate-y-1/2 text-af-ink-3 hover:text-af-ink-2"
              :title="t('common.clear')"
              :aria-label="t('common.clear')"
            >
              <Icon name="x" size="sm" :stroke-width="2" />
            </button>

            <!-- User Dropdown -->
            <div
              v-if="showFilterUserDropdown && (filterUserResults.length > 0 || filterUserKeyword)"
              class="absolute z-50 mt-1 max-h-60 w-full overflow-auto rounded-lg border border-af-hairline bg-af-sheet py-1 shadow-lg"
            >
              <div
                v-if="filterUserLoading"
                class="px-3 py-2 text-sm text-af-ink-3"
              >
                {{ t('common.loading') }}
              </div>
              <div
                v-else-if="filterUserResults.length === 0 && filterUserKeyword"
                class="px-3 py-2 text-sm text-af-ink-3"
              >
                {{ t('common.noOptionsFound') }}
              </div>
              <button
                v-for="user in filterUserResults"
                :key="user.id"
                type="button"
                @click="selectFilterUser(user)"
                class="w-full px-3 py-1.5 text-left text-sm hover:bg-af-sunken"
              >
                <span class="text-af-ink">{{ user.email }}</span>
                <span v-if="user.deleted" class="ml-1 text-xs text-af-ink-3">（{{ t('admin.usage.userDeletedBadge') }}）</span>
              </button>
            </div>
          </div>

          <FilterChip
            v-model="filters.status"
            :label="t('admin.subscriptions.columns.status')"
            :options="statusOptions"
            test-id="filter-status"
            @change="applyFilters"
          />
          <FilterChip
            v-model="filters.plan_id"
            :label="t('admin.subscriptions.columns.plan')"
            :options="planFilterOptions"
            test-id="filter-plan"
            @change="applyFilters"
          />

          <template #end>
            <button
              type="button"
              class="rounded-md p-2 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink disabled:opacity-40"
              :disabled="loading"
              :title="t('common.refresh')"
              :aria-label="t('common.refresh')"
              @click="loadSubscriptions"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <!-- 「用户」列写邮箱还是用户名（存本机） -->
            <PopoverMenu width-class="w-44">
              <template #trigger="{ open }">
                <button
                  type="button"
                  class="rounded-md p-2 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink"
                  :class="open ? 'bg-af-sunken text-af-ink' : ''"
                  :title="t('admin.subscriptions.userColumnMode')"
                  :aria-label="t('admin.subscriptions.userColumnMode')"
                  data-testid="user-column-mode"
                >
                  <Icon name="user" size="md" />
                </button>
              </template>
              <div class="px-3 pb-1 pt-1.5 text-xs text-af-ink-3">{{ t('admin.subscriptions.userColumnMode') }}</div>
              <MenuItem
                :checked="userColumnMode === 'email'"
                data-testid="user-column-mode-email"
                @click="setUserColumnMode('email')"
              >
                {{ t('admin.users.columns.email') }}
              </MenuItem>
              <MenuItem
                :checked="userColumnMode === 'username'"
                data-testid="user-column-mode-username"
                @click="setUserColumnMode('username')"
              >
                {{ t('admin.users.columns.username') }}
              </MenuItem>
            </PopoverMenu>
            <ColumnSettingsMenu :settings="columnSettings" />
          </template>
        </ListToolbar>
      </template>

      <!-- Subscriptions Table -->
      <template #table>
        <DataTable
          :columns="columns"
          :data="subscriptions"
          :loading="loading"
          row-key="id"
          selectable
          :selected-keys="selectedIds"
          :selection-label="getSubscriptionSelectionLabel"
          :server-side-sort="true"
          default-sort-key="created_at"
          default-sort-order="desc"
          @sort="handleSort"
          @update:selected-keys="handleSelectedKeysUpdate"
        >
          <!-- 用户：点进去看这个用户的用量记录 -->
          <template #cell-user="{ row }">
            <RouterLink
              :to="{ path: '/usage', query: { user_id: row.user_id } }"
              class="block max-w-[15rem] truncate rounded font-medium text-af-ink hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-af-brand focus-visible:ring-offset-2"
            >
              {{ userColumnLabel(row) }}
            </RouterLink>
          </template>

          <template #cell-plan="{ row }">
            <div v-if="row.plan" class="min-w-0">
              <div class="truncate font-medium text-af-ink">{{ row.plan.name }}</div>
              <div class="whitespace-nowrap text-xs text-af-ink-3">{{ formatPlanLimits(row.plan) }}</div>
            </div>
            <span v-else class="text-af-ink-3">-</span>
          </template>

          <template #cell-api_key="{ row }">
            <div v-if="row.api_key" class="min-w-0">
              <div class="truncate text-af-ink-2">{{ row.api_key.name }}</div>
              <code class="font-mono text-xs text-af-ink-3">{{ row.api_key.key_masked }}</code>
            </div>
            <span v-else class="text-af-ink-3">-</span>
          </template>

          <!-- 用量：日 / 周 / 月三条细进度条，常态墨色，≥70% 橙、≥90% 红 -->
          <template #cell-usage="{ row }">
            <div class="min-w-[240px] space-y-1.5">
              <div v-if="row.plan?.daily_limit_usd" class="usage-row">
                <div class="flex items-center gap-2">
                  <span class="usage-label">{{ t('admin.subscriptions.daily') }}</span>
                  <div class="h-1.5 flex-1 overflow-hidden rounded-full bg-af-sunken">
                    <div
                      class="h-full rounded-full"
                      :class="getProgressClass(row.daily_usage_usd, row.plan?.daily_limit_usd)"
                      :style="{ width: getProgressWidth(row.daily_usage_usd, row.plan?.daily_limit_usd) }"
                    ></div>
                  </div>
                  <span class="usage-amount">
                    ${{ row.daily_usage_usd?.toFixed(2) || '0.00' }}
                    <span class="text-af-ink-3">/ ${{ row.plan?.daily_limit_usd?.toFixed(2) }}</span>
                  </span>
                </div>
                <div class="reset-info" v-if="row.daily_window_start">{{ formatDailyUsageWindow(row) }}</div>
              </div>

              <div v-if="row.plan?.weekly_limit_usd" class="usage-row">
                <div class="flex items-center gap-2">
                  <span class="usage-label">{{ t('admin.subscriptions.weekly') }}</span>
                  <div class="h-1.5 flex-1 overflow-hidden rounded-full bg-af-sunken">
                    <div
                      class="h-full rounded-full"
                      :class="getProgressClass(row.weekly_usage_usd, row.plan?.weekly_limit_usd)"
                      :style="{ width: getProgressWidth(row.weekly_usage_usd, row.plan?.weekly_limit_usd) }"
                    ></div>
                  </div>
                  <span class="usage-amount">
                    ${{ row.weekly_usage_usd?.toFixed(2) || '0.00' }}
                    <span class="text-af-ink-3">/ ${{ row.plan?.weekly_limit_usd?.toFixed(2) }}</span>
                  </span>
                </div>
                <div class="reset-info" v-if="row.weekly_window_start">{{ formatResetTime(row.weekly_window_start, 'weekly') }}</div>
              </div>

              <div v-if="row.plan?.monthly_limit_usd" class="usage-row">
                <div class="flex items-center gap-2">
                  <span class="usage-label">{{ t('admin.subscriptions.monthly') }}</span>
                  <div class="h-1.5 flex-1 overflow-hidden rounded-full bg-af-sunken">
                    <div
                      class="h-full rounded-full"
                      :class="getProgressClass(row.monthly_usage_usd, row.plan?.monthly_limit_usd)"
                      :style="{ width: getProgressWidth(row.monthly_usage_usd, row.plan?.monthly_limit_usd) }"
                    ></div>
                  </div>
                  <span class="usage-amount">
                    ${{ row.monthly_usage_usd?.toFixed(2) || '0.00' }}
                    <span class="text-af-ink-3">/ ${{ row.plan?.monthly_limit_usd?.toFixed(2) }}</span>
                  </span>
                </div>
                <div class="reset-info" v-if="row.monthly_window_start">{{ formatResetTime(row.monthly_window_start, 'monthly') }}</div>
              </div>

              <!-- 三档都不限 -->
              <div
                v-if="
                  !row.plan?.daily_limit_usd &&
                  !row.plan?.weekly_limit_usd &&
                  !row.plan?.monthly_limit_usd
                "
                class="text-af-ink-3"
              >
                ∞ {{ t('admin.subscriptions.unlimited') }}
              </div>
            </div>
          </template>

          <!-- 到期：只写日期（悬停看精确时间），下一行剩余时长；7 天内到期标橙 -->
          <template #cell-expires_at="{ value }">
            <div v-if="value" class="whitespace-nowrap">
              <div
                class="tabular-nums"
                :class="isExpiringSoon(value) ? 'text-af-warning' : 'text-af-ink-2'"
                :title="formatDateTimeToMinute(value)"
              >
                {{ formatDateOnly(value) }}
              </div>
              <template
                v-for="remainingExpiry in [formatRemainingExpiry(value)]"
                :key="remainingExpiry ?? 'expired'"
              >
                <div v-if="remainingExpiry" class="text-xs text-af-ink-3">
                  {{ remainingExpiry }}
                </div>
              </template>
            </div>
            <span v-else class="text-af-ink-3">{{
              t('admin.subscriptions.noExpiration')
            }}</span>
          </template>

          <!-- 状态：生效中灰点常态；过期橙、撤销红 -->
          <template #cell-status="{ value }">
            <span class="inline-flex items-center gap-1.5 whitespace-nowrap">
              <span class="inline-block h-2 w-2 rounded-full" :class="statusTone(value).dot"></span>
              <span :class="statusTone(value).text">{{ t(`admin.subscriptions.status.${value}`) }}</span>
            </span>
          </template>

          <template #cell-actions="{ row }">
            <RowActions :actions="rowActions(row)" />
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.subscriptions.noSubscriptionsYet')"
              :description="t('admin.subscriptions.emptyHint')"
            >
              <template #action>
                <div class="flex flex-wrap items-center justify-center gap-4">
                  <button type="button" class="btn btn-primary" @click="showAssignModal = true">
                    <Icon name="plus" size="md" class="mr-2" />
                    {{ t('admin.subscriptions.assignSubscription') }}
                  </button>
                  <RouterLink
                    to="/orders/plans"
                    class="inline-flex items-center gap-1 text-sm font-medium text-af-ink-2 hover:text-af-ink"
                  >
                    {{ t('admin.subscriptions.goToPlans') }}
                    <Icon name="arrowRight" size="xs" />
                  </RouterLink>
                </div>
              </template>
            </EmptyState>
          </template>
        </DataTable>
      </template>

      <!-- 批量条：按钮后的数字是选中里适用该操作的条数（其余状态不处理） -->
      <template #bulk>
        <BulkBar
          :count="selectedCount"
          :title="t('admin.subscriptions.bulk.selectionHint')"
          data-test="subscription-bulk-actions"
          @clear="clearSelection"
        >
          <button
            v-for="action in bulkActions"
            :key="action"
            type="button"
            :class="action === 'revoke' ? 'bulk-btn bulk-btn-danger' : 'bulk-btn'"
            :data-test="`bulk-${action}`"
            :disabled="loading || bulkTargets[action].length === 0"
            @click="openBulkAction(action)"
          >
            {{ bulkActionLabel(action) }}
            <span class="tabular-nums opacity-70">{{ bulkTargets[action].length }}</span>
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

    <BulkSubscriptionActionDialog
      v-if="bulkAction !== null"
      :show="true"
      :action="bulkAction"
      :subscriptions="bulkSubscriptions"
      @close="bulkAction = null"
      @completed="handleBulkCompleted"
    />

    <!-- Assign Subscription Modal -->
    <BaseDialog
      :show="showAssignModal"
      :title="t('admin.subscriptions.assignSubscription')"
      width="normal"
      :show-close-button="!submitting"
      :close-on-escape="!submitting"
      @close="closeAssignModal"
    >
      <form
        id="assign-subscription-form"
        @submit.prevent="handleAssignSubscription"
        class="space-y-5"
      >
        <label class="flex items-center gap-2 text-sm text-af-ink-2">
          <input v-model="batchAssignEnabled" type="checkbox" :disabled="submitting" @change="resetAssignUsers" />
          {{ t('admin.subscriptions.batchAssign.enable') }}
        </label>
        <p v-if="batchAssignEnabled" class="input-hint">{{ t('admin.subscriptions.batchAssign.hint') }}</p>
        <div>
          <label class="input-label">{{ t('admin.subscriptions.form.user') }}</label>
          <div class="relative" data-assign-user-search>
            <input
              v-model="userSearchKeyword"
              type="text"
              :disabled="submitting || (batchAssignEnabled && assignUsers.length >= 100)"
              class="input pr-8"
              :placeholder="t('admin.usage.searchUserPlaceholder')"
              @input="debounceSearchUsers"
              @focus="showUserDropdown = true"
            />
            <button
              v-if="selectedUser"
              @click="clearUserSelection"
              type="button"
              class="absolute right-2 top-1/2 -translate-y-1/2 text-af-ink-3 hover:text-af-ink-2"
            >
              <Icon name="x" size="sm" :stroke-width="2" />
            </button>
            <!-- User Dropdown -->
            <div
              v-if="showUserDropdown && (userSearchResults.length > 0 || userSearchKeyword)"
              class="absolute z-50 mt-1 max-h-60 w-full overflow-auto rounded-lg border border-af-hairline bg-af-sheet shadow-lg"
            >
              <div
                v-if="userSearchLoading"
                class="px-4 py-3 text-sm text-af-ink-3"
              >
                {{ t('common.loading') }}
              </div>
              <div
                v-else-if="userSearchResults.length === 0 && userSearchKeyword"
                class="px-4 py-3 text-sm text-af-ink-3"
              >
                {{ t('common.noOptionsFound') }}
              </div>
              <button
                v-for="user in userSearchResults"
                :key="user.id"
                type="button"
                :disabled="submitting || (batchAssignEnabled && assignUsers.some((selected) => selected.id === user.id))"
                @click="selectUser(user)"
                class="w-full px-4 py-2 text-left text-sm hover:bg-af-sunken disabled:opacity-50"
              >
                <span class="font-medium text-af-ink">{{ user.email }}</span>
              </button>
            </div>
          </div>
          <div v-if="batchAssignEnabled && assignUsers.length > 0" class="mt-2 space-y-2" data-test="assign-users">
            <p class="text-sm text-af-ink-2">
              {{ t('admin.subscriptions.batchAssign.selected', { count: assignUsers.length }) }}
            </p>
            <ul class="max-h-40 space-y-1 overflow-y-auto">
              <li v-for="user in assignUsers" :key="user.id" class="flex items-center justify-between gap-2 rounded-lg bg-af-sunken px-3 py-1 text-sm">
                <span class="truncate">{{ user.email }}</span>
                <button
                  type="button"
                  :disabled="submitting"
                  :aria-label="t('admin.subscriptions.batchAssign.removeUser', { email: user.email })"
                  @click="assignUsers = assignUsers.filter((selected) => selected.id !== user.id)"
                >
                  <Icon name="x" size="sm" />
                </button>
              </li>
            </ul>
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('admin.subscriptions.form.plan') }}</label>
          <Select
            v-model="assignForm.plan_id"
            :disabled="submitting"
            :options="planOptions"
            :placeholder="t('admin.subscriptions.selectPlan')"
          />
          <p class="input-hint">{{ t('admin.subscriptions.planHint') }}</p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.subscriptions.form.validityDays') }}</label>
          <input v-model.number="assignForm.validity_days" type="number" min="1" max="36500" step="1" :disabled="submitting" class="input" />
          <p class="input-hint">{{ t('admin.subscriptions.validityHint') }}</p>
        </div>
        <div v-if="batchAssignResult" class="space-y-2 text-sm" role="status" data-test="batch-assign-result">
          <p>{{ t('admin.subscriptions.batchAssign.result', { success: batchAssignResult.success_count, failed: batchAssignResult.failed_count }) }}</p>
          <ul v-if="batchAssignErrors.length" class="max-h-40 space-y-1 overflow-y-auto text-af-danger">
            <li v-for="(error, index) in batchAssignErrors" :key="index">{{ error }}</li>
          </ul>
          <p v-if="batchAssignResult.failed_count > 0" class="input-hint">{{ t('admin.subscriptions.batchAssign.retryHint') }}</p>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button @click="closeAssignModal" type="button" :disabled="submitting" class="btn btn-secondary">
            {{ batchAssignResult ? t('common.close') : t('common.cancel') }}
          </button>
          <button
            type="submit"
            form="assign-subscription-form"
            :disabled="submitting || (batchAssignEnabled && assignUsers.length === 0)"
            class="btn btn-primary"
          >
            <svg
              v-if="submitting"
              class="-ml-1 mr-2 h-4 w-4 animate-spin"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle
                class="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                stroke-width="4"
              ></circle>
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              ></path>
            </svg>
            {{ submitting ? t('admin.subscriptions.assigning') : t('admin.subscriptions.assign') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Adjust Subscription Modal -->
    <BaseDialog
      :show="showExtendModal"
      :title="t('admin.subscriptions.adjustSubscription')"
      width="narrow"
      @close="closeExtendModal"
    >
      <form
        v-if="extendingSubscription"
        id="extend-subscription-form"
        @submit.prevent="handleExtendSubscription"
        class="space-y-5"
      >
        <div class="rounded-lg bg-af-sunken p-4">
          <p class="text-sm text-af-ink-2">
            {{ t('admin.subscriptions.adjustingFor') }}
            <span class="font-medium text-af-ink">{{
              extendingSubscription.user?.email || t('common.deletedUser')
            }}</span>
          </p>
          <p class="mt-1 text-sm text-af-ink-2">
            {{ t('admin.subscriptions.currentExpiration') }}:
            <span class="font-medium text-af-ink">
              {{
                extendingSubscription.expires_at
                  ? formatDateTimeToMinute(extendingSubscription.expires_at)
                  : t('admin.subscriptions.noExpiration')
              }}
            </span>
          </p>
          <p v-if="extendingSubscription.expires_at" class="mt-1 text-sm text-af-ink-2">
            {{ t('admin.subscriptions.remainingDays') }}:
            <span class="font-medium text-af-ink">
              {{ getDaysRemaining(extendingSubscription.expires_at) ?? 0 }}
            </span>
          </p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.subscriptions.form.adjustDays') }}</label>
          <div class="flex items-center gap-2">
            <input
              v-model.number="extendForm.days"
              type="number"
              required
              class="input text-center"
              :placeholder="t('admin.subscriptions.adjustDaysPlaceholder')"
            />
          </div>
          <p class="input-hint">{{ t('admin.subscriptions.adjustHint') }}</p>
        </div>
      </form>
      <template #footer>
        <div v-if="extendingSubscription" class="flex justify-end gap-3">
          <button @click="closeExtendModal" type="button" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button
            type="submit"
            form="extend-subscription-form"
            :disabled="submitting"
            class="btn btn-primary"
          >
            {{ submitting ? t('admin.subscriptions.adjusting') : t('admin.subscriptions.adjust') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Revoke Confirmation Dialog -->
    <ConfirmDialog
      :show="showRevokeDialog"
      :title="t('admin.subscriptions.revokeSubscription')"
      :message="t('admin.subscriptions.revokeConfirm', { user: revokingSubscription?.user?.email || t('common.deletedUser') })"
      :confirm-text="t('admin.subscriptions.revoke')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmRevoke"
      @cancel="showRevokeDialog = false"
    />

    <!-- Restore Confirmation Dialog -->
    <ConfirmDialog
      :show="showRestoreDialog"
      :title="t('admin.subscriptions.restoreSubscription')"
      :message="t('admin.subscriptions.restoreConfirm', { user: restoringSubscription?.user?.email || t('common.deletedUser') })"
      :confirm-text="t('admin.subscriptions.restore')"
      :cancel-text="t('common.cancel')"
      @confirm="confirmRestore"
      @cancel="showRestoreDialog = false"
    />

    <!-- Reset Quota Confirmation Dialog -->
    <ConfirmDialog
      :show="showResetQuotaConfirm"
      :title="t('admin.subscriptions.resetQuotaTitle')"
      :message="t('admin.subscriptions.resetQuotaConfirm', { user: resettingSubscription?.user?.email || t('common.deletedUser') })"
      :confirm-text="t('admin.subscriptions.resetQuota')"
      :cancel-text="t('common.cancel')"
      @confirm="confirmResetQuota"
      @cancel="showResetQuotaConfirm = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminUser, UserSubscription, UserSubscriptionPlan } from '@/types'
import type { SubscriptionPlan } from '@/types/payment'
import { adminPaymentAPI } from '@/api/admin/payment'
import type { SimpleUser } from '@/api/admin/usage'
import type { SubscriptionBulkAction, SubscriptionBulkActionResult, BulkAssignSubscriptionResult } from '@/api/admin/subscriptions'
import { useTableSelection } from '@/composables/useTableSelection'
import { useColumnSettings } from '@/composables/useColumnSettings'
import BulkSubscriptionActionDialog from '@/components/admin/subscription/BulkSubscriptionActionDialog.vue'
import type { Column } from '@/components/common/types'
import { formatDateOnly, formatDateTimeToMinute } from '@/utils/format'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { BulkBar, ColumnSettingsMenu, FilterChip, ListToolbar, MenuItem, PopoverMenu, RowActions } from '@/components/admin/list'
import type { RowAction } from '@/components/admin/list'
import {
  getRemainingDurationParts,
  getRemainingExpiryDuration,
  isOneTimeDailyQuota,
  type RemainingDurationParts
} from '@/utils/subscriptionQuota'

const { t } = useI18n()

// User column display mode: 'email' or 'username'
const userColumnMode = ref<'email' | 'username'>('email')
const USER_COLUMN_MODE_KEY = 'subscription-user-column-mode'

const loadUserColumnMode = () => {
  try {
    const saved = localStorage.getItem(USER_COLUMN_MODE_KEY)
    if (saved === 'email' || saved === 'username') {
      userColumnMode.value = saved
    }
  } catch (e) {
    console.error('Failed to load user column mode:', e)
  }
}

const saveUserColumnMode = () => {
  try {
    localStorage.setItem(USER_COLUMN_MODE_KEY, userColumnMode.value)
  } catch (e) {
    console.error('Failed to save user column mode:', e)
  }
}

const setUserColumnMode = (mode: 'email' | 'username') => {
  userColumnMode.value = mode
  saveUserColumnMode()
}

// 用户名没填就写邮箱；用户已经不在了写「已删除用户」，不拿内部编号兜底
const userColumnLabel = (subscription: UserSubscription): string => {
  const user = subscription.user
  if (!user) return t('common.deletedUser')
  return userColumnMode.value === 'email' ? user.email : (user.username || user.email)
}

// All available columns
const allColumns = computed<Column[]>(() => [
  {
    key: 'user',
    label: userColumnMode.value === 'email'
      ? t('admin.subscriptions.columns.user')
      : t('admin.users.columns.username'),
    sortable: false
  },
  { key: 'plan', label: t('admin.subscriptions.columns.plan'), sortable: false },
  { key: 'api_key', label: t('admin.subscriptions.columns.apiKey'), sortable: false },
  { key: 'usage', label: t('admin.subscriptions.columns.usage'), sortable: false },
  { key: 'expires_at', label: t('admin.subscriptions.columns.expires'), sortable: true },
  { key: 'status', label: t('admin.subscriptions.columns.status'), sortable: true },
  { key: 'actions', label: t('admin.subscriptions.columns.actions'), sortable: false }
])

// 列设置（A4 共用实现）：用户列与操作列恒显示；订阅密钥名多半就是套餐名，默认收起，表格不用横向滚动
const columnSettings = useColumnSettings({
  storageKey: 'admin-subscriptions-columns',
  version: 1,
  columns: allColumns,
  defaultHidden: ['api_key'],
  alwaysVisible: ['user', 'actions']
})
const columns = columnSettings.visibleColumns

// 筛选标签的选项（「全部」= 清掉标签，不作为选项）
const statusOptions = computed(() => [
  { value: 'active', label: t('admin.subscriptions.status.active') },
  { value: 'expired', label: t('admin.subscriptions.status.expired') },
  { value: 'revoked', label: t('admin.subscriptions.status.revoked') }
])

// 状态：生效中是常态（灰点 + 普通字），过期 / 暂停橙、撤销红
const statusTone = (status: string): { dot: string; text: string } => {
  if (status === 'active') return { dot: 'bg-af-ink-4', text: 'text-af-ink-2' }
  if (status === 'revoked') return { dot: 'bg-af-danger', text: 'text-af-danger' }
  return { dot: 'bg-af-warning', text: 'text-af-warning' }
}

const subscriptions = ref<UserSubscription[]>([])
const plans = ref<SubscriptionPlan[]>([])
const loading = ref(false)
let abortController: AbortController | null = null

const { selectedIds, selectedCount, setSelectedIds, clear: clearSelection, removeMany: removeSelectedIds } =
  useTableSelection<UserSubscription>({ rows: subscriptions, getId: (subscription) => subscription.id })
// 批量条里危险的「撤销」排最后
const bulkActions: SubscriptionBulkAction[] = ['extend', 'reset_quota', 'restore', 'revoke']
const bulkActionLabel = (action: SubscriptionBulkAction): string => ({
  extend: t('admin.subscriptions.bulk.adjustExpiry'),
  reset_quota: t('admin.subscriptions.resetQuota'),
  revoke: t('admin.subscriptions.revoke'),
  restore: t('admin.subscriptions.restore')
})[action]
const bulkAction = ref<SubscriptionBulkAction | null>(null)
const bulkSubscriptions = ref<UserSubscription[]>([])
const bulkTargets = computed(() => {
  const selected = subscriptions.value.filter((subscription) => selectedIds.value.includes(subscription.id))
  return {
    extend: selected.filter((subscription) => ['active', 'expired'].includes(subscription.status)),
    reset_quota: selected.filter((subscription) => subscription.status === 'active'),
    revoke: selected.filter((subscription) => subscription.status === 'active'),
    restore: selected.filter((subscription) => subscription.status === 'revoked')
  }
})
const getSubscriptionSelectionLabel = (subscription: UserSubscription) =>
  t('admin.subscriptions.bulk.selectSubscription', {
    user: subscription.user?.email || t('common.deletedUser'),
    plan: subscription.plan?.name || t('common.deletedPlan')
  })
const handleSelectedKeysUpdate = (keys: Array<string | number>) => {
  const visibleIds = new Set(subscriptions.value.map((subscription) => subscription.id))
  setSelectedIds(keys.filter((key): key is number => typeof key === 'number' && visibleIds.has(key)))
}
const openBulkAction = (action: SubscriptionBulkAction) => {
  if (loading.value || bulkTargets.value[action].length === 0) return
  bulkSubscriptions.value = [...bulkTargets.value[action]]
  bulkAction.value = action
}
const handleBulkCompleted = async (result: SubscriptionBulkActionResult) => {
  removeSelectedIds(result.results.filter((item) => item.success).map((item) => item.subscription_id))
  await loadSubscriptions()
}

// Toolbar user filter (fuzzy search -> select user_id)
const filterUserKeyword = ref('')
const filterUserResults = ref<SimpleUser[]>([])
const filterUserLoading = ref(false)
const showFilterUserDropdown = ref(false)
const selectedFilterUser = ref<SimpleUser | null>(null)
let filterUserSearchTimeout: ReturnType<typeof setTimeout> | null = null

// User search state
const userSearchKeyword = ref('')
const userSearchResults = ref<AdminUser[]>([])
const userSearchLoading = ref(false)
const showUserDropdown = ref(false)
const selectedUser = ref<AdminUser | null>(null)
const batchAssignEnabled = ref(false)
const assignUsers = ref<AdminUser[]>([])
const batchAssignResult = ref<BulkAssignSubscriptionResult | null>(null)
// 提交那一刻的「用户 → 邮箱」：后端的失败原因写成「user 12: 原因」，展示时换成邮箱，不露内部编号
const batchAssignEmails = ref(new Map<number, string>())
const batchAssignErrors = computed(() =>
  (batchAssignResult.value?.errors ?? []).map((error) => {
    const match = /^user (\d+): ([\s\S]*)$/.exec(error)
    if (!match) return error
    const email = batchAssignEmails.value.get(Number(match[1])) ?? t('common.deletedUser')
    return `${email}${t('common.labelSeparator')}${match[2]}`
  })
)
let userSearchTimeout: ReturnType<typeof setTimeout> | null = null

const filters = reactive({
  status: 'active',
  plan_id: '',
  user_id: null as number | null
})

// Sorting state
const sortState = reactive({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc'
})

const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})

const showAssignModal = ref(false)
const showExtendModal = ref(false)
const showRevokeDialog = ref(false)
const showRestoreDialog = ref(false)
const showResetQuotaConfirm = ref(false)
const submitting = ref(false)
const resettingSubscription = ref<UserSubscription | null>(null)
const resettingQuota = ref(false)
const extendingSubscription = ref<UserSubscription | null>(null)
const revokingSubscription = ref<UserSubscription | null>(null)
const restoringSubscription = ref<UserSubscription | null>(null)

const assignForm = reactive({
  user_id: null as number | null,
  plan_id: null as number | null,
  validity_days: 30
})

const extendForm = reactive({
  days: 30
})

// 套餐筛选（全部套餐，含下架）
const planFilterOptions = computed(() =>
  plans.value.map((p) => ({ value: p.id.toString(), label: p.name }))
)

// 分配用：套餐名 + 三档限额
const planOptions = computed(() =>
  plans.value.map((p) => ({ value: p.id, label: `${p.name} · ${formatPlanLimits(p)}` }))
)

/** 三档限额拼串：日 / 周 / 月，null = 不限 */
function formatPlanLimits(plan: Pick<UserSubscriptionPlan, 'daily_limit_usd' | 'weekly_limit_usd' | 'monthly_limit_usd'>): string {
  const fmt = (v: number | null | undefined) => (v == null ? t('payment.admin.unlimited') : `$${v}`)
  return [
    `${t('payment.admin.dailyLimitShort')} ${fmt(plan.daily_limit_usd)}`,
    `${t('payment.admin.weeklyLimitShort')} ${fmt(plan.weekly_limit_usd)}`,
    `${t('payment.admin.monthlyLimitShort')} ${fmt(plan.monthly_limit_usd)}`,
  ].join(' · ')
}

const applyFilters = () => {
  clearSelection()
  pagination.page = 1
  loadSubscriptions()
}

const loadSubscriptions = async () => {
  if (abortController) {
    abortController.abort()
  }
  const requestController = new AbortController()
  abortController = requestController
  const { signal } = requestController

  loading.value = true
  try {
    const response = await adminAPI.subscriptions.list(
      pagination.page,
      pagination.page_size,
      {
        status: (filters.status as any) || undefined,
        plan_id: filters.plan_id ? parseInt(filters.plan_id) : undefined,
        user_id: filters.user_id || undefined,
        sort_by: sortState.sort_by,
        sort_order: sortState.sort_order
      },
      {
        signal
      }
    )
    if (signal.aborted || abortController !== requestController) return
    subscriptions.value = response.items
    const visibleIds = new Set(response.items.map((subscription) => subscription.id))
    setSelectedIds(selectedIds.value.filter((id) => visibleIds.has(id)))
    pagination.total = response.total
    pagination.pages = response.pages
  } catch (error: any) {
    if (signal.aborted || error?.name === 'AbortError' || error?.code === 'ERR_CANCELED') {
      return
    }
    console.error('Error loading subscriptions:', error)
  } finally {
    if (abortController === requestController) {
      loading.value = false
      abortController = null
    }
  }
}

const loadPlans = async () => {
  try {
    const res = await adminPaymentAPI.getPlans()
    plans.value = res.data || []
  } catch (error) {
    console.error('Error loading plans:', error)
  }
}

// Toolbar user filter search with debounce
const debounceSearchFilterUsers = () => {
  if (filterUserSearchTimeout) {
    clearTimeout(filterUserSearchTimeout)
  }
  filterUserSearchTimeout = setTimeout(searchFilterUsers, 300)
}

const searchFilterUsers = async () => {
  const keyword = filterUserKeyword.value.trim()

  // Clear active user filter if user modified the search keyword
  if (selectedFilterUser.value && keyword !== selectedFilterUser.value.email) {
    selectedFilterUser.value = null
    filters.user_id = null
    applyFilters()
  }

  if (!keyword) {
    filterUserResults.value = []
    return
  }

  filterUserLoading.value = true
  try {
    filterUserResults.value = await adminAPI.usage.searchUsers(keyword)
  } catch (error) {
    console.error('Failed to search users:', error)
    filterUserResults.value = []
  } finally {
    filterUserLoading.value = false
  }
}

const selectFilterUser = (user: SimpleUser) => {
  selectedFilterUser.value = user
  filterUserKeyword.value = user.email
  showFilterUserDropdown.value = false
  filters.user_id = user.id
  applyFilters()
}

const clearFilterUser = () => {
  selectedFilterUser.value = null
  filterUserKeyword.value = ''
  filterUserResults.value = []
  showFilterUserDropdown.value = false
  filters.user_id = null
  applyFilters()
}

// User search with debounce
const debounceSearchUsers = () => {
  // Invalidate the assignment target before the debounced search runs.
  if (selectedUser.value && userSearchKeyword.value.trim() !== selectedUser.value.email) {
    selectedUser.value = null
    assignForm.user_id = null
  }
  if (userSearchTimeout) {
    clearTimeout(userSearchTimeout)
  }
  userSearchTimeout = setTimeout(searchUsers, 300)
}

const searchUsers = async () => {
  const keyword = userSearchKeyword.value.trim()

  if (!keyword) {
    userSearchResults.value = []
    return
  }

  userSearchLoading.value = true
  try {
    const result = await adminAPI.users.list(1, 30, {
      search: keyword, sort_by: 'email', sort_order: 'asc'
    })
    userSearchResults.value = result.items
  } catch (error) {
    console.error('Failed to search users:', error)
    userSearchResults.value = []
  } finally {
    userSearchLoading.value = false
  }
}

const selectUser = (user: AdminUser) => {
  if (submitting.value) return
  if (batchAssignEnabled.value) {
    if (assignUsers.value.length < 100 && !assignUsers.value.some((selected) => selected.id === user.id)) {
      assignUsers.value = [...assignUsers.value, user]
    }
    userSearchKeyword.value = ''
    userSearchResults.value = []
    showUserDropdown.value = false
    return
  }
  selectedUser.value = user
  userSearchKeyword.value = user.email
  showUserDropdown.value = false
  assignForm.user_id = user.id
}

const clearUserSelection = () => {
  selectedUser.value = null
  userSearchKeyword.value = ''
  userSearchResults.value = []
  assignForm.user_id = null
}

const resetAssignUsers = () => {
  clearUserSelection()
  assignUsers.value = []
  batchAssignResult.value = null
}

const handlePageChange = (page: number) => {
  clearSelection()
  pagination.page = page
  loadSubscriptions()
}

const handlePageSizeChange = (pageSize: number) => {
  clearSelection()
  pagination.page_size = pageSize
  pagination.page = 1
  loadSubscriptions()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  clearSelection()
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadSubscriptions()
}

const closeAssignModal = () => {
  if (submitting.value) return
  showAssignModal.value = false
  batchAssignEnabled.value = false
  assignUsers.value = []
  batchAssignResult.value = null
  assignForm.user_id = null
  assignForm.plan_id = null
  assignForm.validity_days = 30
  // Clear user search state
  selectedUser.value = null
  userSearchKeyword.value = ''
  userSearchResults.value = []
  showUserDropdown.value = false
}

const handleAssignSubscription = async () => {
  if (submitting.value) return
  if (batchAssignEnabled.value ? assignUsers.value.length === 0 : !assignForm.user_id) {
    console.error(t('admin.subscriptions.pleaseSelectUser'))
    return
  }
  if (!assignForm.plan_id) {
    console.error(t('admin.subscriptions.pleaseSelectPlan'))
    return
  }
  if (!Number.isInteger(assignForm.validity_days) || assignForm.validity_days < 1 || assignForm.validity_days > 36500) {
    console.error(t('admin.subscriptions.validityDaysRequired'))
    return
  }

  submitting.value = true
  try {
    if (batchAssignEnabled.value) {
      batchAssignEmails.value = new Map(assignUsers.value.map((user) => [user.id, user.email]))
      batchAssignResult.value = await adminAPI.subscriptions.bulkAssign({
        user_ids: assignUsers.value.map((user) => user.id),
        plan_id: assignForm.plan_id,
        validity_days: assignForm.validity_days
      })
      const result = batchAssignResult.value
      const successIds = new Set(result.subscriptions.map((subscription) => subscription.user_id))
      assignUsers.value = assignUsers.value.filter((user) => !successIds.has(user.id))
      if (result.success_count > 0) {
        await loadSubscriptions()
      }
      return
    }
    await adminAPI.subscriptions.assign({
      user_id: assignForm.user_id!,
      plan_id: assignForm.plan_id,
      validity_days: assignForm.validity_days
    })
    submitting.value = false
    closeAssignModal()
    loadSubscriptions()
  } catch (error: any) {
    console.error('Error assigning subscription:', error)
  } finally {
    submitting.value = false
  }
}

const handleExtend = (subscription: UserSubscription) => {
  extendingSubscription.value = subscription
  extendForm.days = 30
  showExtendModal.value = true
}

const closeExtendModal = () => {
  showExtendModal.value = false
  extendingSubscription.value = null
}

const handleExtendSubscription = async () => {
  if (!extendingSubscription.value) return

  // 前端验证：调整后的过期时间必须在未来
  if (extendingSubscription.value.expires_at) {
    const expiresAt = new Date(extendingSubscription.value.expires_at)
    const newExpiresAt = new Date(expiresAt.getTime() + extendForm.days * 24 * 60 * 60 * 1000)
    if (newExpiresAt <= new Date()) {
      console.error(t('admin.subscriptions.adjustWouldExpire'))
      return
    }
  }

  submitting.value = true
  try {
    await adminAPI.subscriptions.extend(extendingSubscription.value.id, {
      days: extendForm.days
    })
    closeExtendModal()
    loadSubscriptions()
  } catch (error: any) {
    console.error('Error adjusting subscription:', error)
  } finally {
    submitting.value = false
  }
}

const handleRevoke = (subscription: UserSubscription) => {
  revokingSubscription.value = subscription
  showRevokeDialog.value = true
}

const confirmRevoke = async () => {
  if (!revokingSubscription.value) return

  try {
    await adminAPI.subscriptions.revoke(revokingSubscription.value.id)
    showRevokeDialog.value = false
    revokingSubscription.value = null
    loadSubscriptions()
  } catch (error: any) {
    console.error('Error revoking subscription:', error)
  }
}

const handleRestore = (subscription: UserSubscription) => {
  restoringSubscription.value = subscription
  showRestoreDialog.value = true
}

const confirmRestore = async () => {
  if (!restoringSubscription.value) return

  try {
    await adminAPI.subscriptions.restore(restoringSubscription.value.id)
    showRestoreDialog.value = false
    restoringSubscription.value = null
    loadSubscriptions()
  } catch (error: any) {
    console.error('Error restoring subscription:', error)
  }
}

const handleResetQuota = (subscription: UserSubscription) => {
  resettingSubscription.value = subscription
  showResetQuotaConfirm.value = true
}

const confirmResetQuota = async () => {
  if (!resettingSubscription.value) return
  if (resettingQuota.value) return
  resettingQuota.value = true
  try {
    await adminAPI.subscriptions.resetQuota(resettingSubscription.value.id, { daily: true, weekly: true, monthly: true })
    showResetQuotaConfirm.value = false
    resettingSubscription.value = null
    await loadSubscriptions()
  } catch (error: any) {
    console.error('Error resetting quota:', error)
  } finally {
    resettingQuota.value = false
  }
}

// Helper functions
const getDaysRemaining = (expiresAt: string): number | null => {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  if (diff < 0) return null
  return Math.ceil(diff / (1000 * 60 * 60 * 24))
}

const formatRemainingExpiry = (expiresAt: string): string | null => {
  const duration = getRemainingExpiryDuration(expiresAt)
  if (!duration) return null
  if (duration.unit === 'days') {
    return t('admin.subscriptions.daysRemaining', { days: duration.days })
  }
  if (duration.hours) {
    return t('admin.subscriptions.hoursMinutesRemaining', {
      hours: duration.hours,
      minutes: duration.minutes
    })
  }
  return t('admin.subscriptions.minutesRemaining', { minutes: duration.minutes })
}

const isExpiringSoon = (expiresAt: string): boolean => {
  const days = getDaysRemaining(expiresAt)
  return days !== null && days <= 7
}

const getProgressWidth = (used: number | null | undefined, limit: number | null): string => {
  if (!limit || limit === 0) return '0%'
  const usedValue = used ?? 0
  const percentage = Math.min((usedValue / limit) * 100, 100)
  return `${percentage}%`
}

// 与用户站「我的订阅」同一口径：常态墨色，≥70% 橙、≥90% 红
const getProgressClass = (used: number | null | undefined, limit: number | null): string => {
  if (!limit || limit === 0) return 'bg-af-hairline-strong'
  const usedValue = used ?? 0
  const percentage = (usedValue / limit) * 100
  if (percentage >= 90) return 'bg-af-danger'
  if (percentage >= 70) return 'bg-af-warning'
  return 'bg-af-brand'
}

const formatResetDuration = (parts: RemainingDurationParts): string => {
  if (parts.days > 0) {
    return t('admin.subscriptions.resetInDaysHours', { days: parts.days, hours: parts.hours })
  }

  if (parts.hours > 0) {
    return t('admin.subscriptions.resetInHoursMinutes', { hours: parts.hours, minutes: parts.minutes })
  }

  return t('admin.subscriptions.resetInMinutes', { minutes: parts.minutes })
}

const formatQuotaEndDuration = (parts: RemainingDurationParts): string => {
  if (parts.days > 0) {
    return t('admin.subscriptions.quotaEndsInDaysHours', { days: parts.days, hours: parts.hours })
  }

  if (parts.hours > 0) {
    return t('admin.subscriptions.quotaEndsInHoursMinutes', { hours: parts.hours, minutes: parts.minutes })
  }

  return t('admin.subscriptions.quotaEndsInMinutes', { minutes: parts.minutes })
}

const formatDailyUsageWindow = (subscription: UserSubscription): string => {
  if (isOneTimeDailyQuota(subscription) && subscription.expires_at) {
    const parts = getRemainingDurationParts(subscription.expires_at)
    return parts ? formatQuotaEndDuration(parts) : t('admin.subscriptions.windowNotActive')
  }

  return formatResetTime(subscription.daily_window_start, 'daily')
}

// Format reset time based on window start and period type
const formatResetTime = (windowStart: string | null, period: 'daily' | 'weekly' | 'monthly'): string => {
  if (!windowStart) return t('admin.subscriptions.windowNotActive')

  const start = new Date(windowStart)
  const now = new Date()

  // Calculate reset time based on period
  let resetTime: Date
  switch (period) {
    case 'daily':
      resetTime = new Date(start.getTime() + 24 * 60 * 60 * 1000)
      break
    case 'weekly':
      resetTime = new Date(start.getTime() + 7 * 24 * 60 * 60 * 1000)
      break
    case 'monthly':
      resetTime = new Date(start.getTime() + 30 * 24 * 60 * 60 * 1000)
      break
  }

  const parts = getRemainingDurationParts(resetTime, now)

  return parts ? formatResetDuration(parts) : t('admin.subscriptions.windowNotActive')
}

// 行操作（A4）：「调整」是图标；其余进「⋯」，撤销红字、走确认框
const rowActions = (subscription: UserSubscription): RowAction[] => {
  const actions: RowAction[] = []
  if (subscription.status === 'active' || subscription.status === 'expired') {
    actions.push({
      key: 'adjust',
      label: t('admin.subscriptions.adjust'),
      icon: 'calendar',
      primary: true,
      onSelect: () => handleExtend(subscription)
    })
  }
  if (subscription.status === 'active') {
    actions.push(
      {
        key: 'reset-quota',
        label: t('admin.subscriptions.resetQuota'),
        icon: 'refresh',
        disabled: resettingQuota.value && resettingSubscription.value?.id === subscription.id,
        onSelect: () => handleResetQuota(subscription)
      },
      {
        key: 'revoke',
        label: t('admin.subscriptions.revoke'),
        icon: 'ban',
        danger: true,
        dividerBefore: true,
        onSelect: () => handleRevoke(subscription)
      }
    )
  }
  if (subscription.status === 'revoked') {
    actions.push({
      key: 'restore',
      label: t('admin.subscriptions.restore'),
      icon: 'refresh',
      onSelect: () => handleRestore(subscription)
    })
  }
  return actions
}

// 两个用户搜索下拉：点外面关掉
const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  if (!target.closest('[data-assign-user-search]')) showUserDropdown.value = false
  if (!target.closest('[data-filter-user-search]')) showFilterUserDropdown.value = false
}

onMounted(() => {
  loadUserColumnMode()
  loadSubscriptions()
  loadPlans()
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
  if (filterUserSearchTimeout) {
    clearTimeout(filterUserSearchTimeout)
  }
  if (userSearchTimeout) {
    clearTimeout(userSearchTimeout)
  }
})
</script>

<style scoped>
.usage-row {
  @apply space-y-0.5;
}

.usage-label {
  @apply w-8 flex-shrink-0 text-xs text-af-ink-3;
}

/* 金额定宽右对齐，各行进度条等长 */
.usage-amount {
  @apply min-w-[6.5rem] whitespace-nowrap text-right text-xs tabular-nums text-af-ink-2;
}

/* 重置倒计时：跟在进度条下面、与条左端对齐 */
.reset-info {
  @apply pl-10 text-xs text-af-ink-3;
}
</style>
