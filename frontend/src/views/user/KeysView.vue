<template>
  <!--
    密钥（muqian 2026-09-25 定）：回答「每把 key 还能不能用、怎么用」。
    页头一个实心主操作（创建）；「需要处理」只在概览出现（muqian 2026-09-26：一类信息只放一页），这里不再摆一行摘要——
    已过期 / 额度用尽 / 限额将满 / 7 天内到期都是「状态」筛选里的选项，概览的提醒点进来就带着对应筛选；
    接口地址条常驻；表格只放判断能不能用的列（用量与最紧的一项限额合成一列），点行开右侧详情抽屉（趋势、限额与重置、使用方法）。
    配色单色为主：状态用小圆点 + 文字，只有用尽 / 过期 / 超限用橙红。
  -->
  <SiteShell>
    <template #actions>
      <button
        @click="loadApiKeys({ refreshAttention: true })"
        :disabled="loading"
        class="btn btn-ghost btn-md"
        :title="t('common.refresh')"
      >
        <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
      </button>
      <ColumnSettingsMenu :settings="columnSettings" />
      <button @click="showCreateModal = true" class="btn btn-primary btn-md" data-tour="keys-create-btn">
        <Icon name="plus" size="md" class="mr-2" />
        {{ t('keys.createKey') }}
      </button>
    </template>

    <div class="space-y-4">
      <!-- 接口地址条：表格上方常驻；地址与「使用方法」同一口径——设置留空就是当前站点 -->
      <EndpointPopover
        class="border-b border-af-hairline pb-4"
        :api-base-url="apiBaseUrl"
        :custom-endpoints="publicSettings?.custom_endpoints || []"
      />
      <!-- 筛选与批量操作 -->
      <div class="flex flex-col gap-3">
        <div class="keys-filters flex flex-wrap items-center gap-2">
          <SearchInput
            v-model="filterSearch"
            :placeholder="t('keys.searchPlaceholder')"
            class="w-full sm:w-64"
            @search="onFilterChange"
          />
          <FilterChip
            :model-value="attentionFilter || filterStatus"
            :label="t('common.status')"
            :options="statusFilterOptions"
            test-id="keys-filter-status"
            @update:model-value="onStatusFilterChange"
          />
        </div>
        <div v-if="selectedIds.length" class="flex flex-wrap items-center gap-3 text-sm">
          <span class="text-af-ink-2">
            {{ t('keys.bulkEdit.selectedCount', { count: selectedIds.length }) }}
          </span>
          <button
            class="btn btn-primary btn-sm"
            :disabled="loading"
            data-test="bulk-edit-keys"
            @click="showBulkEditModal = true"
          >
            {{ t('keys.bulkEdit.title') }}
          </button>
          <button class="btn btn-secondary btn-sm" @click="selectedIds = []">
            {{ t('keys.bulkEdit.clearSelection') }}
          </button>
        </div>
      </div>
      <!-- 桌面表格出血到页边让行线贯通；窄屏是卡片列表，留页边距 -->
      <div class="md:-mx-6">
      <DataTable
        :columns="columnSettings.visibleColumns.value"
        :data="tableRows"
        :loading="loading"
        selectable
        row-key="id"
        :selected-keys="selectedIds"
        :selection-label="(key: ApiKey) => t('keys.bulkEdit.selectKey', { name: key.name })"
        clickable-rows
        :row-class="(row: ApiKey) => (detailKey?.id === row.id ? 'row-selected bg-af-sunken' : undefined)"
        @update:selected-keys="handleSelectionChange"
        @row-click="openKeyDetail($event, 'overview')"
        :server-side-sort="true"
        default-sort-key="created_at"
        default-sort-order="desc"
        @sort="handleSort"
      >
        <template #cell-key="{ value, row }">
          <div class="flex items-center gap-2">
            <code class="code text-xs">
              {{ maskApiKey(value) }}
            </code>
            <button
              @click.stop="copyToClipboard(value, row.id)"
              class="rounded-lg p-1 transition-colors hover:bg-af-sunken"
              :class="
                copiedKeyId === row.id
                  ? 'text-af-success'
                  : 'text-af-ink-4 hover:text-af-ink'
              "
              :title="copiedKeyId === row.id ? t('keys.copied') : t('keys.copyToClipboard')"
            >
              <Icon
                v-if="copiedKeyId === row.id"
                name="check"
                size="sm"
                :stroke-width="2"
              />
              <Icon v-else name="clipboard" size="sm" />
            </button>
          </div>
        </template>

        <template #cell-name="{ value, row }">
          <div class="flex items-center gap-1.5">
            <span class="font-medium text-af-ink">{{ value }}</span>
            <span
              v-if="row.subscription_id"
              class="badge badge-gray"
              data-testid="subscription-key-badge"
              :title="t('keys.subscriptionKeyProtected')"
            >
              {{ t('keys.subscriptionKey', { plan: row.subscription_plan_name || '' }) }}
            </span>
            <Icon
              v-if="row.ip_whitelist?.length > 0 || row.ip_blacklist?.length > 0"
              name="shield"
              size="sm"
              class="text-af-ink-3"
              :title="t('keys.ipRestrictionEnabled')"
            />
          </div>
        </template>

        <template #cell-current_concurrency="{ value }">
          <span class="text-sm tabular-nums" :class="(value ?? 0) > 0 ? 'font-semibold text-af-ink' : 'text-af-ink-4'">
            {{ value ?? 0 }}
          </span>
        </template>

        <!-- 用量 / 限额：今日与近 30 天实付；下面只画用得最满的一项限额，没设就写「不限额」，全部限额在抽屉里 -->
        <template #cell-usage="{ row }">
          <div class="min-w-[180px] text-sm" data-testid="key-usage-cell">
            <div class="flex items-baseline gap-x-3 whitespace-nowrap tabular-nums">
              <span><span class="text-af-ink-3">{{ t('keys.today') }}</span> <span class="font-medium text-af-ink">{{ formatCurrency(usageStats[row.id]?.today_actual_cost ?? 0) }}</span></span>
              <span><span class="text-af-ink-3">{{ t('keys.total') }}</span> <span class="font-medium text-af-ink">{{ formatCurrency(usageStats[row.id]?.total_actual_cost ?? 0) }}</span></span>
            </div>
            <KeyLimitInline class="mt-1.5" :meter="rowLimits.get(row.id) ?? null" />
          </div>
        </template>

        <template #cell-expires_at="{ value, row }">
          <span v-if="!value" class="text-sm text-af-ink-4">{{ t('keys.noExpiration') }}</span>
          <span v-else-if="new Date(value) < now" class="text-sm text-af-danger">{{ formatDateTime(value) }}</span>
          <span v-else-if="isExpiringSoon(row, now)" class="text-sm text-af-warning" :title="formatDateTime(value)">
            {{ t('keys.expiresInDaysShort', { days: daysUntilExpiry(row, now) }) }}
          </span>
          <span v-else class="text-sm text-af-ink-3">{{ formatDateTime(value) }}</span>
        </template>

        <template #cell-status="{ value }">
          <span class="inline-flex items-center gap-1.5 whitespace-nowrap text-sm" :class="value === 'inactive' ? 'text-af-ink-4' : 'text-af-ink-2'">
            <span
              class="h-1.5 w-1.5 shrink-0 rounded-full"
              :class="
                value === 'active' ? 'bg-af-ink' :
                value === 'quota_exhausted' ? 'bg-af-warning' :
                value === 'expired' ? 'bg-af-danger' :
                'bg-af-ink-4'
              "
              aria-hidden="true"
            />
            {{ t('keys.status.' + value) }}
          </span>
        </template>

        <!-- 最近使用写相对时间（能看出这把 key 还有没有人在用），悬停看具体时间 -->
        <template #cell-last_used_at="{ value }">
          <span v-if="value" class="whitespace-nowrap text-sm text-af-ink-2" :title="formatDateTime(value)">
            {{ formatRelativeTime(value) }}
          </span>
          <span v-else class="text-sm text-af-ink-4">{{ t('keys.detail.neverUsed') }}</span>
        </template>

        <template #cell-last_used_ip="{ value }">
          <span v-if="value" class="text-sm text-af-ink-3">
            {{ value }}
          </span>
          <span v-else class="text-sm text-af-ink-4">-</span>
        </template>

        <template #cell-created_at="{ value }">
          <span class="text-sm text-af-ink-3">{{ formatDateTime(value) }}</span>
        </template>

        <template #cell-actions="{ row }">
          <!-- 两个常用动作是图标按钮，其余进「更多」菜单，行高不再被五个带字按钮撑开 -->
          <div class="flex items-center gap-0.5">
            <button
              type="button"
              class="rounded-md p-1.5 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink"
              :title="t('keys.useKey')"
              :aria-label="t('keys.useKey')"
              data-testid="use-key"
              @click.stop="openKeyDetail(row, 'use')"
            >
              <Icon name="terminal" size="sm" />
            </button>
            <button
              type="button"
              class="rounded-md p-1.5 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink"
              :title="t('common.edit')"
              :aria-label="t('common.edit')"
              data-testid="edit-key"
              @click.stop="editKey(row)"
            >
              <Icon name="edit" size="sm" />
            </button>
            <button
              type="button"
              class="rounded-md p-1.5 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink"
              :class="openMenu?.key.id === row.id ? 'bg-af-sunken text-af-ink' : ''"
              :title="t('keys.moreActions')"
              :aria-label="t('keys.moreActions')"
              :aria-expanded="openMenu?.key.id === row.id ? 'true' : 'false'"
              data-testid="key-menu"
              @click.stop="toggleKeyMenu(row, $event)"
            >
              <Icon name="more" size="sm" />
            </button>
          </div>
        </template>

        <template #empty>
          <StatusState
            kind="empty"
            :title="t('keys.noKeysYet')"
            :description="t('keys.createFirstKey')"
            :action-label="t('keys.createKey')"
            @action="showCreateModal = true"
          />
        </template>
      </DataTable>
      <Pagination
        v-if="!attentionFilter && pagination.total > 0"
        :page="pagination.page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        @update:page="handlePageChange"
        @update:pageSize="handlePageSizeChange"
      />
      </div>
    </div>

    <KeyDetailDrawer
      :show="detailKey !== null"
      :api-key="detailKey"
      :usage="detailKey ? usageStats[detailKey.id] : undefined"
      :tab="detailTab"
      :base-url="publicSettings?.api_base_url || ''"
      :site-name="publicSettings?.site_name || ''"
      :now="now"
      @update:tab="detailTab = $event"
      @close="detailKey = null"
      @edit="editKey"
      @reset-quota="confirmResetQuota"
      @reset-rate-limit="confirmResetRateLimit"
    />

    <!-- 行内「更多」菜单：Teleport 到 body 固定定位，不被表格的 sticky 列裁掉 -->
    <Teleport to="body">
      <div
        v-if="openMenu"
        ref="keyMenuRef"
        class="dropdown fixed z-50 w-44 py-1"
        :style="{ top: `${openMenu.top}px`, right: `${openMenu.right}px` }"
        data-testid="key-menu-items"
        @click.stop
      >
        <button
          v-if="!publicSettings?.hide_ccs_import_button"
          type="button"
          class="dropdown-item w-full text-left"
          @click="runMenuAction(() => importToCcswitch(openMenu!.key))"
        >
          {{ t('keys.importToCcSwitch') }}
        </button>
        <button type="button" class="dropdown-item w-full text-left" @click="runMenuAction(() => toggleKeyStatus(openMenu!.key))">
          {{ openMenu.key.status === 'active' ? t('keys.disable') : t('keys.enable') }}
        </button>
        <!-- 订阅 key 是订阅的访问凭证，不能删 -->
        <button
          v-if="!openMenu.key.subscription_id"
          type="button"
          class="dropdown-item w-full text-left text-af-danger hover:text-af-danger"
          data-testid="delete-key"
          @click="runMenuAction(() => confirmDelete(openMenu!.key))"
        >
          {{ t('common.delete') }}
        </button>
      </div>
    </Teleport>

    <!-- 新建 / 编辑：新建默认只露名称，其余收进「更多设置」（muqian 2026-09-25，与渠道表单同一思路）；编辑时全部展开。
         已用多少、重置这些状态信息在详情抽屉里，表单只放设置。 -->
    <BaseDialog
      :show="showCreateModal || showEditModal"
      :title="showEditModal ? t('keys.editKey') : t('keys.createKey')"
      width="normal"
      @close="closeModals"
    >
      <form id="key-form" @submit.prevent="handleSubmit" class="space-y-5">
        <div>
          <label class="input-label">{{ t('keys.nameLabel') }}</label>
          <input
            v-model="formData.name"
            type="text"
            required
            class="input"
            :placeholder="t('keys.namePlaceholder')"
            data-tour="key-form-name"
          />
        </div>

        <button
          v-if="!showEditModal"
          type="button"
          class="flex w-full items-center justify-between gap-3 border-t border-af-hairline pt-4 text-left"
          :aria-expanded="showMoreSettings ? 'true' : 'false'"
          data-testid="key-form-more"
          @click="showMoreSettings = !showMoreSettings"
        >
          <span>
            <span class="block text-sm font-medium text-af-ink">{{ t('keys.moreSettings') }}</span>
            <span class="block text-13 text-af-ink-3">{{ t('keys.moreSettingsHint') }}</span>
          </span>
          <Icon :name="showMoreSettings ? 'chevronUp' : 'chevronDown'" size="sm" class="shrink-0 text-af-ink-3" />
        </button>

        <template v-if="showEditModal || showMoreSettings">
          <!-- Custom Key Section (only for create) -->
          <div v-if="!showEditModal" class="space-y-3">
            <div class="flex items-center justify-between">
              <label class="input-label mb-0">{{ t('keys.customKeyLabel') }}</label>
              <Toggle v-model="formData.use_custom_key" />
            </div>
            <div v-if="formData.use_custom_key">
              <input
                v-model="formData.custom_key"
                type="text"
                class="input font-mono"
                :placeholder="t('keys.customKeyPlaceholder')"
                :class="{ 'border-af-danger': customKeyError }"
              />
              <p v-if="customKeyError" class="mt-1 text-sm text-af-danger">{{ customKeyError }}</p>
              <p v-else class="input-hint">{{ t('keys.customKeyHint') }}</p>
            </div>
          </div>

          <div v-if="showEditModal">
            <label class="input-label">{{ t('keys.statusLabel') }}</label>
            <Select
              v-model="formData.status"
              :options="statusOptions"
              :placeholder="t('keys.selectStatus')"
            />
          </div>

          <!-- 额度：留空 / 0 = 不限 -->
          <div>
            <label class="input-label">{{ t('keys.quotaLimit') }}</label>
            <div class="relative">
              <span class="absolute left-3 top-1/2 -translate-y-1/2 text-af-ink-3">$</span>
              <input
                v-model.number="formData.quota"
                type="number"
                step="0.01"
                min="0"
                class="input pl-7"
                :placeholder="t('keys.quotaAmountPlaceholder')"
              />
            </div>
            <p class="input-hint">{{ t('keys.quotaAmountHint') }}</p>
          </div>

          <!-- 速率限制：5h / 1d / 7d 三档 -->
          <div class="space-y-3">
            <div class="flex items-center justify-between">
              <label class="input-label mb-0">{{ t('keys.rateLimitSection') }}</label>
              <Toggle v-model="formData.enable_rate_limit" />
            </div>
            <div v-if="formData.enable_rate_limit" class="space-y-4 pt-1">
              <p class="input-hint -mt-2">{{ t('keys.rateLimitHint') }}</p>
              <div v-for="window in RATE_WINDOWS" :key="window.field">
                <label class="input-label">{{ t(window.labelKey) }}</label>
                <div class="relative">
                  <span class="absolute left-3 top-1/2 -translate-y-1/2 text-af-ink-3">$</span>
                  <input
                    v-model.number="formData[window.field]"
                    type="number"
                    step="0.01"
                    min="0"
                    class="input pl-7"
                    placeholder="0"
                  />
                </div>
              </div>
            </div>
          </div>

          <!-- 到期 -->
          <div class="space-y-3">
            <div class="flex items-center justify-between">
              <label class="input-label mb-0">{{ t('keys.expiration') }}</label>
              <Toggle v-model="formData.enable_expiration" />
            </div>

            <div v-if="formData.enable_expiration" class="space-y-4 pt-1">
              <div class="flex flex-wrap gap-2">
                <button
                  v-for="days in ['7', '30', '90']"
                  :key="days"
                  type="button"
                  @click="setExpirationDays(parseInt(days))"
                  :class="[
                    'rounded-lg px-3 py-1.5 text-sm transition-colors',
                    formData.expiration_preset === days
                      ? 'bg-af-brand-tint text-af-brand'
                      : 'bg-af-sunken text-af-ink-2 hover:bg-af-hairline'
                  ]"
                >
                  {{ showEditModal ? t('keys.extendDays', { days }) : t('keys.expiresInDays', { days }) }}
                </button>
                <button
                  type="button"
                  @click="formData.expiration_preset = 'custom'"
                  :class="[
                    'rounded-lg px-3 py-1.5 text-sm transition-colors',
                    formData.expiration_preset === 'custom'
                      ? 'bg-af-brand-tint text-af-brand'
                      : 'bg-af-sunken text-af-ink-2 hover:bg-af-hairline'
                  ]"
                >
                  {{ t('keys.customDate') }}
                </button>
              </div>

              <div>
                <label class="input-label">{{ t('keys.expirationDate') }}</label>
                <input
                  v-model="formData.expiration_date"
                  type="datetime-local"
                  class="input"
                  @input="formData.expiration_preset = 'custom'"
                />
                <p class="input-hint">{{ t('keys.expirationDateHint') }}</p>
              </div>

              <div v-if="showEditModal && selectedKey?.expires_at" class="text-sm">
                <span class="text-af-ink-3">{{ t('keys.currentExpiration') }}: </span>
                <span class="font-medium text-af-ink">
                  {{ formatDateTime(selectedKey.expires_at) }}
                </span>
              </div>
            </div>
          </div>

          <!-- IP 限制 -->
          <div class="space-y-3">
            <div class="flex items-center justify-between">
              <label class="input-label mb-0">{{ t('keys.ipRestriction') }}</label>
              <Toggle v-model="formData.enable_ip_restriction" />
            </div>

            <div v-if="formData.enable_ip_restriction" class="space-y-4 pt-1">
              <div>
                <label class="input-label">{{ t('keys.ipWhitelist') }}</label>
                <textarea
                  v-model="formData.ip_whitelist"
                  rows="3"
                  class="input font-mono text-sm"
                  :placeholder="t('keys.ipWhitelistPlaceholder')"
                />
                <p class="input-hint">{{ t('keys.ipWhitelistHint') }}</p>
              </div>

              <div>
                <label class="input-label">{{ t('keys.ipBlacklist') }}</label>
                <textarea
                  v-model="formData.ip_blacklist"
                  rows="3"
                  class="input font-mono text-sm"
                  :placeholder="t('keys.ipBlacklistPlaceholder')"
                />
                <p class="input-hint">{{ t('keys.ipBlacklistHint') }}</p>
              </div>
            </div>
          </div>
        </template>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button @click="closeModals" type="button" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button
            form="key-form"
            type="submit"
            :disabled="submitting"
            class="btn btn-primary"
            data-tour="key-form-submit"
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
            {{
              submitting
                ? t('keys.saving')
                : showEditModal
                  ? t('common.update')
                  : t('common.create')
            }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BulkEditKeysModal
      :show="showBulkEditModal"
      :selected-keys="selectedApiKeys"
      @close="showBulkEditModal = false"
      @updated="handleBulkUpdated"
    />

    <!-- Delete Confirmation Dialog -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('keys.deleteKey')"
      :message="t('keys.deleteConfirmMessage', { name: selectedKey?.name })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="handleDelete"
      @cancel="showDeleteDialog = false"
    />

    <!-- Reset Quota Confirmation Dialog -->
    <ConfirmDialog
      :show="showResetQuotaDialog"
      :title="t('keys.resetQuotaTitle')"
      :message="t('keys.resetQuotaConfirmMessage', { name: resetTarget?.name, used: resetTarget?.quota_used?.toFixed(4) })"
      :confirm-text="t('keys.reset')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="resetQuotaUsed"
      @cancel="showResetQuotaDialog = false"
    />

    <!-- Reset Rate Limit Confirmation Dialog -->
    <ConfirmDialog
      :show="showResetRateLimitDialog"
      :title="t('keys.resetRateLimitTitle')"
      :message="t('keys.resetRateLimitConfirmMessage', { name: resetTarget?.name })"
      :confirm-text="t('keys.reset')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="resetRateLimitUsage"
      @cancel="showResetRateLimitDialog = false"
    />

    <!-- CCS Client Selection Dialog：导入哪个客户端由用户选 -->
    <BaseDialog
      :show="showCcsClientSelect"
      :title="t('keys.ccsClientSelect.title')"
      width="narrow"
      @close="closeCcsClientSelect"
    >
      <div class="space-y-4">
        <p class="text-sm text-af-ink-2">
          {{ t('keys.ccsClientSelect.description') }}
	        </p>
	        <div class="grid grid-cols-2 gap-3">
	          <button
	            v-for="client in ccsClients"
	            :key="client.type"
	            type="button"
	            :data-testid="`ccs-client-${client.type}`"
	            @click="handleCcsClientSelect(client.type)"
	            class="flex flex-col items-center gap-2 p-4 rounded-md border-2 border-af-hairline hover:border-af-brand hover:bg-af-brand-tint transition-all"
	          >
	            <Icon :name="client.icon" size="xl" class="text-af-ink-2" />
	            <span class="font-medium text-af-ink">{{ t(`keys.ccsClientSelect.${client.type}`) }}</span>
	            <span class="text-xs text-af-ink-3">{{ t(`keys.ccsClientSelect.${client.type}Desc`) }}</span>
	          </button>
	        </div>
	      </div>
      <template #footer>
        <div class="flex justify-end">
          <button @click="closeCcsClientSelect" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
        </div>
      </template>
    </BaseDialog>

  </SiteShell>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { onClickOutside } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { useAppStore } from '@/stores/app'
import { useOnboardingStore } from '@/stores/onboarding'
import { useClipboard } from '@/composables/useClipboard'
import { useColumnSettings } from '@/composables/useColumnSettings'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { DEFAULT_SITE_NAME } from '@/utils/branding'
import { keysAPI, authAPI, usageAPI } from '@/api'
import { adoptPreloaded } from '@/router/routePreload'
import { KEY_STATUSES, KEYS_DEFAULT_SORT, keysListFilters, keysRequestKey, keysStatusFromQuery, keysUsageIds } from './keysQuery'
import SiteShell from '@/components/user/shell/SiteShell.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import BulkEditKeysModal from '@/components/keys/BulkEditKeysModal.vue'
import EndpointPopover from '@/components/keys/EndpointPopover.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import ColumnSettingsMenu from '@/components/common/ColumnSettingsMenu.vue'
import FilterChip from '@/components/common/FilterChip.vue'
import Select from '@/components/common/Select.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Toggle from '@/components/common/Toggle.vue'
import type { Column, FilterOption } from '@/components/common/types'
import Icon from '@/components/icons/Icon.vue'
import KeyDetailDrawer, { type KeyDrawerTab } from '@/components/user/keys/KeyDetailDrawer.vue'
import KeyLimitInline from '@/components/user/keys/KeyLimitInline.vue'
import {
  daysUntilExpiry,
  isExpiringSoon,
  isNearLimit,
  loadAllKeys,
  tightestLimit,
  type KeyLimitMeter
} from '@/components/user/keys/keyAttention'
import type { BatchApiKeyUsageStats } from '@/api/usage'
import type { ApiKey, PublicSettings, UpdateApiKeyRequest } from '@/types'
import { formatCurrency, formatDateTime, formatRelativeTime } from '@/utils/format'
import { maskApiKey } from '@/utils/maskApiKey'
import {
  buildCcSwitchImportDeeplink,
  type CcSwitchClientType
} from '@/utils/ccswitchImport'

const { t } = useI18n()

// Helper to format date for datetime-local input
const formatDateTimeLocal = (isoDate: string): string => {
  const date = new Date(isoDate)
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

const appStore = useAppStore()
const onboardingStore = useOnboardingStore()
const { copyToClipboard: clipboardCopy } = useClipboard()
// 单测里不装路由：拿不到就当没有地址栏参数
const route = useRoute() as ReturnType<typeof useRoute> | undefined

// ---------- 列 ----------
const allColumns = computed<Column[]>(() => [
  { key: 'name', label: t('common.name'), sortable: true },
  { key: 'key', label: t('keys.apiKey'), sortable: false },
  { key: 'status', label: t('common.status'), sortable: true },
  { key: 'usage', label: t('keys.usageAndLimit'), sortable: false },
  { key: 'current_concurrency', label: t('keys.currentConcurrency'), sortable: true },
  { key: 'last_used_at', label: t('keys.lastUsedAt'), sortable: true },
  { key: 'last_used_ip', label: t('keys.lastUsedIP'), sortable: false },
  { key: 'expires_at', label: t('keys.expiresAt'), sortable: true },
  { key: 'created_at', label: t('keys.created'), sortable: true },
  { key: 'actions', label: t('common.actions'), sortable: false }
])

// 默认只留判断「能不能用」的列；并发 / 最近 IP / 创建时间在详情抽屉里（muqian 2026-09-25）。
// 用户站不显示内部 ID：ID 列已删，version 2 让本机存的旧列设置回到新默认。
const columnSettings = useColumnSettings({
  storageKey: 'user-keys-columns',
  version: 2,
  columns: allColumns,
  defaultHidden: ['current_concurrency', 'last_used_ip', 'created_at'],
  alwaysVisible: ['name', 'actions']
})

const apiKeys = ref<ApiKey[]>([])
const selectedIds = ref<number[]>([])
const showBulkEditModal = ref(false)
const selectedApiKeys = computed(() => tableRows.value.filter((key) => selectedIds.value.includes(key.id)))

const handleSelectionChange = (ids: Array<string | number>) => {
  const visibleIds = new Set(tableRows.value.map((key) => key.id))
  selectedIds.value = [...new Set(ids.map(Number))].filter((id) => visibleIds.has(id))
}

const handleBulkUpdated = (succeededIds: number[]) => {
  const succeeded = new Set(succeededIds)
  selectedIds.value = selectedIds.value.filter((id) => !succeeded.has(id))
  loadApiKeys({ refreshAttention: true })
}

const loading = ref(false)
const submitting = ref(false)
const now = ref(new Date())
let nowTimer: ReturnType<typeof setInterval> | null = null
const usageStats = ref<Record<string, BatchApiKeyUsageStats>>({})

const pagination = ref({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})
const sortState = ref({ ...KEYS_DEFAULT_SORT })

// Filter state
const filterSearch = ref('')
const filterStatus = ref<string | number>('')

const showCreateModal = ref(false)
const showEditModal = ref(false)
const showMoreSettings = ref(false)
const showDeleteDialog = ref(false)
const showResetQuotaDialog = ref(false)
const showResetRateLimitDialog = ref(false)
const showCcsClientSelect = ref(false)
const pendingCcsRow = ref<ApiKey | null>(null)
const ccsClients = [
  { type: 'claude', icon: 'terminal' },
  { type: 'codex', icon: 'terminal' },
  { type: 'gemini', icon: 'sparkles' },
  { type: 'grokbuild', icon: 'terminal' }
] as const satisfies ReadonlyArray<{ type: CcSwitchClientType; icon: 'terminal' | 'sparkles' }>
const selectedKey = ref<ApiKey | null>(null)
/** 正要重置额度 / 速率用量的那把（从详情抽屉发起） */
const resetTarget = ref<ApiKey | null>(null)
const copiedKeyId = ref<number | null>(null)
const publicSettings = ref<PublicSettings | null>(null)
// 设置里的 API 端点地址留空 = 当前站点（与「使用方法」的回落一致）
const apiBaseUrl = computed(() => publicSettings.value?.api_base_url || window.location.origin)

// ---------- 限额将满 / 7 天内到期 ----------
// 这两种没有后端筛选：在全部密钥（最多 500 把）里挑出来；拉不全就不提供这两个选项
const allKeys = ref<ApiKey[] | null>(null)

type AttentionFilter = 'near_limit' | 'expiring'
const attentionFilter = ref<AttentionFilter | ''>('')

function setAttentionFilter(value: AttentionFilter) {
  selectedIds.value = []
  attentionFilter.value = value
  filterSearch.value = ''
  filterStatus.value = ''
  void loadUsageStats(attentionRows.value.map((key) => key.id))
}

const attentionRows = computed<ApiKey[]>(() => {
  if (!attentionFilter.value || !allKeys.value) return []
  const test = attentionFilter.value === 'near_limit' ? isNearLimit : isExpiringSoon
  return allKeys.value.filter((key) => test(key, now.value))
})

/** 表格数据：「限额将满 / 即将到期」时是从全部密钥里挑出来的几把（不分页），否则是服务端分页列表 */
const tableRows = computed(() => (attentionFilter.value ? attentionRows.value : apiKeys.value))
const rowLimits = computed(() => new Map<number, KeyLimitMeter | null>(tableRows.value.map((key) => [key.id, tightestLimit(key, now.value)])))

async function loadAttentionKeys() {
  try {
    const { keys, complete } = await loadAllKeys()
    allKeys.value = complete ? keys : null
    if (attentionFilter.value) void loadUsageStats(attentionRows.value.map((key) => key.id))
  } catch (error) {
    console.error('Failed to load keys for attention:', error)
    allKeys.value = null
  }
}

// ---------- 详情抽屉 ----------
const detailKey = ref<ApiKey | null>(null)
const detailTab = ref<KeyDrawerTab>('overview')
const openKeyDetail = (key: ApiKey, tab: KeyDrawerTab) => {
  closeKeyMenu()
  detailKey.value = key
  detailTab.value = tab
}
/** 列表刷新后，抽屉里的那把换成新数据（被删了就关掉） */
function syncDetailKey() {
  if (!detailKey.value) return
  const fresh = tableRows.value.find((key) => key.id === detailKey.value!.id) ?? allKeys.value?.find((key) => key.id === detailKey.value!.id)
  if (fresh) detailKey.value = fresh
}

// 行内「更多」菜单：一次只开一个；按钮的视口坐标定位，点外面 / 滚动 / 选完动作即关
const openMenu = ref<{ key: ApiKey; top: number; right: number } | null>(null)
const keyMenuRef = ref<HTMLElement | null>(null)
const toggleKeyMenu = (key: ApiKey, event: MouseEvent) => {
  if (openMenu.value?.key.id === key.id) {
    openMenu.value = null
    return
  }
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
  openMenu.value = { key, top: rect.bottom + 4, right: window.innerWidth - rect.right }
}
const closeKeyMenu = () => {
  openMenu.value = null
}
const runMenuAction = (action: () => void | Promise<void>) => {
  closeKeyMenu()
  void action()
}
onClickOutside(keyMenuRef, closeKeyMenu)
let abortController: AbortController | null = null

const RATE_WINDOWS = [
  { field: 'rate_limit_5h', labelKey: 'keys.rateLimit5h' },
  { field: 'rate_limit_1d', labelKey: 'keys.rateLimit1d' },
  { field: 'rate_limit_7d', labelKey: 'keys.rateLimit7d' }
] as const

const emptyForm = () => ({
  name: '',
  status: 'active' as 'active' | 'inactive',
  use_custom_key: false,
  custom_key: '',
  enable_ip_restriction: false,
  ip_whitelist: '',
  ip_blacklist: '',
  // Quota settings (empty = unlimited)
  quota: null as number | null,
  // Rate limit settings
  enable_rate_limit: false,
  rate_limit_5h: null as number | null,
  rate_limit_1d: null as number | null,
  rate_limit_7d: null as number | null,
  enable_expiration: false,
  expiration_preset: '30' as '7' | '30' | '90' | 'custom',
  expiration_date: ''
})

const formData = ref(emptyForm())

// 自定义Key验证
const customKeyError = computed(() => {
  if (!formData.value.use_custom_key || !formData.value.custom_key) {
    return ''
  }
  const key = formData.value.custom_key
  if (key.length < 16) {
    return t('keys.customKeyTooShort')
  }
  // 检查字符：只允许字母、数字、下划线、连字符
  if (!/^[a-zA-Z0-9_-]+$/.test(key)) {
    return t('keys.customKeyInvalidChars')
  }
  return ''
})

const statusOptions = computed(() => [
  { value: 'active', label: t('common.active') },
  { value: 'inactive', label: t('common.inactive') }
])

const shouldSubmitEditStatus = (key: ApiKey, status: 'active' | 'inactive') => {
  if (key.status === 'quota_exhausted' || key.status === 'expired') {
    return status === 'active'
  }
  return true
}

const statusFilterOptions = computed<FilterOption[]>(() => [
  ...KEY_STATUSES.map((value) => ({ value, label: t(`keys.status.${value}`) })),
  ...(allKeys.value
    ? [
        { value: 'near_limit', label: t('keys.attention.nearLimit') },
        { value: 'expiring', label: t('keys.attention.expiringSoon') }
      ]
    : [])
])

const onFilterChange = () => {
  selectedIds.value = []
  attentionFilter.value = ''
  pagination.value.page = 1
  loadApiKeys()
}

const onStatusFilterChange = (value: string | number) => {
  if (value === 'near_limit' || value === 'expiring') {
    setAttentionFilter(value)
    return
  }
  filterStatus.value = value
  onFilterChange()
}

const copyToClipboard = async (text: string, keyId: number) => {
  const success = await clipboardCopy(text, t('keys.copied'))
  if (success) {
    copiedKeyId.value = keyId
    setTimeout(() => {
      copiedKeyId.value = null
    }, 800)
  }
}

const isAbortError = (error: unknown) => {
  if (!error || typeof error !== 'object') return false
  const { name, code } = error as { name?: string; code?: string }
  return name === 'AbortError' || code === 'ERR_CANCELED'
}

async function loadUsageStats(ids: number[], signal?: AbortSignal) {
  if (ids.length === 0) return
  try {
    const batch = keysUsageIds(ids)
    const usageResponse = await adoptPreloaded(keysRequestKey.usage(batch), () => usageAPI.getDashboardApiKeysUsage(batch, { signal }))
    if (signal?.aborted) return
    usageStats.value = { ...usageStats.value, ...usageResponse.stats }
  } catch (e) {
    if (!isAbortError(e)) {
      console.error('Failed to load usage stats:', e)
    }
  }
}

/**
 * 拉当前页。refreshAttention：同时重拉全部密钥，给「限额将满 / 7 天内到期」筛选用（首次进入、刷新、任何改动之后）；
 * 没有筛选且一页就装得下时直接用这一页，不再多拉一次。
 */
const loadApiKeys = async (options: { refreshAttention?: boolean } = {}) => {
  abortController?.abort()
  const controller = new AbortController()
  abortController = controller
  const { signal } = controller
  loading.value = true
  try {
    const filters = keysListFilters(filterSearch.value, filterStatus.value ? String(filterStatus.value) : '', sortState.value)
    const { page, page_size: pageSize } = pagination.value
    const response = await adoptPreloaded(keysRequestKey.list(page, pageSize, filters), () =>
      keysAPI.list(page, pageSize, filters, { signal })
    )
    if (signal.aborted) return
    apiKeys.value = response.items
    handleSelectionChange(selectedIds.value)
    pagination.value.total = response.total
    pagination.value.pages = response.pages

    if (options.refreshAttention) {
      const unfiltered = !filters.search && !filters.status
      if (unfiltered && pagination.value.page === 1 && (response.pages ?? 0) <= 1) {
        allKeys.value = response.items
      } else {
        void loadAttentionKeys()
      }
    }
    syncDetailKey()

    // Load usage stats for all API keys in the list
    await loadUsageStats(response.items.map((k) => k.id), signal)
  } catch (error) {
    if (isAbortError(error)) {
      return
    }
    appStore.showError(t('keys.failedToLoad'))
  } finally {
    if (abortController === controller) {
      loading.value = false
    }
  }
}

const loadPublicSettings = async () => {
  try {
    publicSettings.value = await adoptPreloaded(keysRequestKey.publicSettings, () => authAPI.getPublicSettings())
  } catch (error) {
    console.error('Failed to load public settings:', error)
  }
}

const handlePageChange = (page: number) => {
  selectedIds.value = []
  pagination.value.page = page
  loadApiKeys()
}

const handlePageSizeChange = (pageSize: number) => {
  selectedIds.value = []
  pagination.value.page_size = pageSize
  pagination.value.page = 1
  loadApiKeys()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  selectedIds.value = []
  sortState.value.sort_by = key
  sortState.value.sort_order = order
  pagination.value.page = 1
  loadApiKeys()
}

const editKey = (key: ApiKey) => {
  closeKeyMenu()
  selectedKey.value = key
  const hasIPRestriction = (key.ip_whitelist?.length > 0) || (key.ip_blacklist?.length > 0)
  const hasExpiration = !!key.expires_at
  formData.value = {
    name: key.name,
    status: key.status === 'quota_exhausted' || key.status === 'expired' ? 'inactive' : key.status,
    use_custom_key: false,
    custom_key: '',
    enable_ip_restriction: hasIPRestriction,
    ip_whitelist: (key.ip_whitelist || []).join('\n'),
    ip_blacklist: (key.ip_blacklist || []).join('\n'),
    quota: key.quota > 0 ? key.quota : null,
    enable_rate_limit: (key.rate_limit_5h > 0) || (key.rate_limit_1d > 0) || (key.rate_limit_7d > 0),
    rate_limit_5h: key.rate_limit_5h || null,
    rate_limit_1d: key.rate_limit_1d || null,
    rate_limit_7d: key.rate_limit_7d || null,
    enable_expiration: hasExpiration,
    expiration_preset: 'custom',
    expiration_date: key.expires_at ? formatDateTimeLocal(key.expires_at) : ''
  }
  showEditModal.value = true
}

const toggleKeyStatus = async (key: ApiKey) => {
  const newStatus = key.status === 'active' ? 'inactive' : 'active'
  try {
    await keysAPI.toggleStatus(key.id, newStatus)
    appStore.showSuccess(
      newStatus === 'active' ? t('keys.keyEnabledSuccess') : t('keys.keyDisabledSuccess')
    )
    loadApiKeys({ refreshAttention: true })
  } catch (error) {
    appStore.showError(t('keys.failedToUpdateStatus'))
  }
}

const confirmDelete = (key: ApiKey) => {
  selectedKey.value = key
  showDeleteDialog.value = true
}

const handleSubmit = async () => {
  // Validate custom key if enabled
  if (!showEditModal.value && formData.value.use_custom_key) {
    if (!formData.value.custom_key) {
      appStore.showError(t('keys.customKeyRequired'))
      return
    }
    if (customKeyError.value) {
      appStore.showError(customKeyError.value)
      return
    }
  }

  // 打开了有效期却没有日期（比如手动清空了），不能悄悄建成永久有效
  if (formData.value.enable_expiration && !formData.value.expiration_date) {
    appStore.showError(t('keys.expirationDateRequired'))
    return
  }

  // Parse IP lists only if IP restriction is enabled
  const parseIPList = (text: string): string[] =>
    text.split('\n').map(ip => ip.trim()).filter(ip => ip.length > 0)
  const ipWhitelist = formData.value.enable_ip_restriction ? parseIPList(formData.value.ip_whitelist) : []
  const ipBlacklist = formData.value.enable_ip_restriction ? parseIPList(formData.value.ip_blacklist) : []

  // Calculate quota value (null/empty/0 = unlimited, stored as 0)
  const quota = formData.value.quota && formData.value.quota > 0 ? formData.value.quota : 0

  // Calculate expiration
  let expiresInDays: number | undefined
  let expiresAt: string | null | undefined
  if (formData.value.enable_expiration && formData.value.expiration_date) {
    if (!showEditModal.value) {
      // Create mode: calculate days from date
      const expDate = new Date(formData.value.expiration_date)
      const now = new Date()
      const diffDays = Math.ceil((expDate.getTime() - now.getTime()) / (1000 * 60 * 60 * 24))
      expiresInDays = diffDays > 0 ? diffDays : 1
    } else {
      // Edit mode: use custom date directly
      expiresAt = new Date(formData.value.expiration_date).toISOString()
    }
  } else if (showEditModal.value) {
    // Edit mode: if expiration disabled or date cleared, send empty string to clear
    expiresAt = ''
  }

  // Calculate rate limit values (send 0 when toggle is off)
  const rateLimitData = formData.value.enable_rate_limit ? {
    rate_limit_5h: formData.value.rate_limit_5h && formData.value.rate_limit_5h > 0 ? formData.value.rate_limit_5h : 0,
    rate_limit_1d: formData.value.rate_limit_1d && formData.value.rate_limit_1d > 0 ? formData.value.rate_limit_1d : 0,
    rate_limit_7d: formData.value.rate_limit_7d && formData.value.rate_limit_7d > 0 ? formData.value.rate_limit_7d : 0,
  } : { rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0 }

  submitting.value = true
  try {
    if (showEditModal.value && selectedKey.value) {
      const updates: UpdateApiKeyRequest = {
        name: formData.value.name,
        ip_whitelist: ipWhitelist,
        ip_blacklist: ipBlacklist,
        quota: quota,
        expires_at: expiresAt,
        rate_limit_5h: rateLimitData.rate_limit_5h,
        rate_limit_1d: rateLimitData.rate_limit_1d,
        rate_limit_7d: rateLimitData.rate_limit_7d,
      }
      if (shouldSubmitEditStatus(selectedKey.value, formData.value.status)) {
        updates.status = formData.value.status
      }
      await keysAPI.update(selectedKey.value.id, updates)
      appStore.showSuccess(t('keys.keyUpdatedSuccess'))
    } else {
      const customKey = formData.value.use_custom_key ? formData.value.custom_key : undefined
      await keysAPI.create(
        formData.value.name,
        customKey,
        ipWhitelist,
        ipBlacklist,
        quota,
        expiresInDays,
        rateLimitData
      )
      appStore.showSuccess(t('keys.keyCreatedSuccess'))
      // Only advance tour if active, on submit step, and creation succeeded
      if (onboardingStore.isCurrentStep('[data-tour="key-form-submit"]')) {
        onboardingStore.nextStep(500)
      }
    }
    closeModals()
    loadApiKeys({ refreshAttention: true })
  } catch (error: any) {
    const errorMsg = error.response?.data?.detail || t('keys.failedToSave')
    appStore.showError(errorMsg)
    // Don't advance tour on error
  } finally {
    submitting.value = false
  }
}

/**
 * 处理删除 API Key 的操作
 * 优化：错误处理改进，优先显示后端返回的具体错误消息（如权限不足等），
 * 若后端未返回消息则显示默认的国际化文本
 */
const handleDelete = async () => {
  if (!selectedKey.value) return

  try {
    await keysAPI.delete(selectedKey.value.id)
    appStore.showSuccess(t('keys.keyDeletedSuccess'))
    showDeleteDialog.value = false
    if (detailKey.value?.id === selectedKey.value.id) detailKey.value = null
    loadApiKeys({ refreshAttention: true })
  } catch (error: any) {
    // 优先使用后端返回的错误消息，提供更具体的错误信息给用户
    const errorMsg = error?.message || t('keys.failedToDelete')
    appStore.showError(errorMsg)
  }
}

const closeModals = () => {
  showCreateModal.value = false
  showEditModal.value = false
  showMoreSettings.value = false
  selectedKey.value = null
  formData.value = emptyForm()
}

// 快捷天数：新建时从现在算；编辑时按钮写「+N 天」，从原到期时间往后延（已过期或原来永久有效则从现在算）
const setExpirationDays = (days: number) => {
  formData.value.expiration_preset = days.toString() as '7' | '30' | '90'
  const now = new Date()
  const currentExpiry = showEditModal.value && selectedKey.value?.expires_at
    ? new Date(selectedKey.value.expires_at)
    : null
  const expDate = currentExpiry && currentExpiry > now ? currentExpiry : now
  expDate.setDate(expDate.getDate() + days)
  formData.value.expiration_date = formatDateTimeLocal(expDate.toISOString())
}

// 新建时打开有效期，按默认高亮的天数（30 天）直接填好日期；之前只高亮不填，提交出去是永久有效
watch(
  () => formData.value.enable_expiration,
  (enabled) => {
    if (!enabled || formData.value.expiration_date) return
    const preset = formData.value.expiration_preset
    if (preset !== 'custom') setExpirationDays(parseInt(preset))
  }
)

// ---------- 重置已用（详情抽屉里发起，先确认） ----------
const confirmResetQuota = (key: ApiKey) => {
  resetTarget.value = key
  showResetQuotaDialog.value = true
}

const confirmResetRateLimit = (key: ApiKey) => {
  resetTarget.value = key
  showResetRateLimitDialog.value = true
}

/** 用接口返回的新数据替换列表、全部密钥与抽屉里的同一把 */
function applyKeyUpdate(updated: ApiKey) {
  const replace = (list: ApiKey[]) => list.map((key) => (key.id === updated.id ? { ...key, ...updated } : key))
  apiKeys.value = replace(apiKeys.value)
  if (allKeys.value) allKeys.value = replace(allKeys.value)
  if (detailKey.value?.id === updated.id) detailKey.value = { ...detailKey.value, ...updated }
}

const resetQuotaUsed = async () => {
  const key = resetTarget.value
  if (!key) return
  showResetQuotaDialog.value = false
  try {
    const updatedKey = await keysAPI.update(key.id, { reset_quota: true })
    appStore.showSuccess(t('keys.quotaResetSuccess'))
    applyKeyUpdate({ ...key, quota_used: updatedKey.quota_used, status: updatedKey.status })
  } catch (error: any) {
    const errorMsg = error.response?.data?.detail || t('keys.failedToResetQuota')
    appStore.showError(errorMsg)
  } finally {
    resetTarget.value = null
  }
}

const resetRateLimitUsage = async () => {
  const key = resetTarget.value
  if (!key) return
  showResetRateLimitDialog.value = false
  try {
    await keysAPI.update(key.id, { reset_rate_limit_usage: true })
    appStore.showSuccess(t('keys.rateLimitResetSuccess'))
    await loadApiKeys({ refreshAttention: true })
  } catch (error: any) {
    const errorMsg = error.response?.data?.detail || t('keys.failedToResetRateLimit')
    appStore.showError(errorMsg)
  } finally {
    resetTarget.value = null
  }
}

// 没有分组就没有「分组平台」：导入 CC-Switch 时由用户选客户端（Claude Code / Codex / Gemini CLI / Grok Build）。
const importToCcswitch = (row: ApiKey) => {
  pendingCcsRow.value = row
  showCcsClientSelect.value = true
}

const executeCcsImport = (row: ApiKey, clientType: CcSwitchClientType) => {
  const baseUrl = publicSettings.value?.api_base_url || window.location.origin

  const usageScript = `({
    request: {
      url: "{{baseUrl}}/v1/usage",
      method: "GET",
      headers: { "Authorization": "Bearer {{apiKey}}" }
    },
    extractor: function(response) {
      const remaining = response?.remaining ?? response?.quota?.remaining ?? response?.balance;
      const unit = response?.unit ?? response?.quota?.unit ?? "USD";
      return {
        isValid: response?.is_active ?? response?.isValid ?? true,
        remaining,
        unit
      };
    }
  })`
  const providerName = (publicSettings.value?.site_name || DEFAULT_SITE_NAME).trim() || DEFAULT_SITE_NAME
  const deeplink = buildCcSwitchImportDeeplink({
    baseUrl,
    clientType,
    providerName,
    apiKey: row.key,
    usageScript
  })

  try {
    window.open(deeplink, '_self')

    // Check if the protocol handler worked by detecting if we're still focused
    setTimeout(() => {
      if (document.hasFocus()) {
        // Still focused means the protocol handler likely failed
        appStore.showError(t('keys.ccSwitchNotInstalled'))
      }
    }, 100)
  } catch (error) {
    appStore.showError(t('keys.ccSwitchNotInstalled'))
  }
}

const handleCcsClientSelect = (clientType: CcSwitchClientType) => {
  if (pendingCcsRow.value) {
    executeCcsImport(pendingCcsRow.value, clientType)
  }
  showCcsClientSelect.value = false
  pendingCcsRow.value = null
}

const closeCcsClientSelect = () => {
  showCcsClientSelect.value = false
  pendingCcsRow.value = null
}

/** 概览「需要处理」带过来的条件：?status=expired|quota_exhausted 走后端筛选，?attention=near_limit|expiring 在全部密钥里挑 */
function applyRouteQuery() {
  const status = keysStatusFromQuery(route?.query ?? {})
  if (status) filterStatus.value = status
  const focus = route?.query.attention
  if (focus === 'near_limit' || focus === 'expiring') attentionFilter.value = focus
}

onMounted(() => {
  window.addEventListener('scroll', closeKeyMenu, true)
  applyRouteQuery()
  loadApiKeys({ refreshAttention: true })
  loadPublicSettings()
  nowTimer = setInterval(() => { now.value = new Date() }, 60000)
})

onUnmounted(() => {
  window.removeEventListener('scroll', closeKeyMenu, true)
  if (nowTimer) clearInterval(nowTimer)
})
</script>

<style scoped>
/* 筛选行：通用搜索框 / 下拉是 42px，这一页压到 34px、13px 字 */
.keys-filters :deep(.input),
.keys-filters :deep(.select-trigger) {
  @apply py-1.5 text-13;
}
</style>
