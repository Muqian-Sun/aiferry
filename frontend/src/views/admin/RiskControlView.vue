<template>
  <AppLayout>
    <template v-if="!loading" #header-actions>
      <button
        type="button"
        class="btn btn-ghost btn-md px-2.5"
        :disabled="statusLoading"
        :title="t('admin.riskControl.refreshStatus')"
        :aria-label="t('admin.riskControl.refreshStatus')"
        @click="loadStatus(false)"
      >
        <Icon name="refresh" size="md" :class="statusLoading ? 'animate-spin' : ''" />
      </button>
      <button type="button" class="btn btn-primary btn-md" @click="openSettings">
        <Icon name="cog" size="sm" />
        {{ t('admin.riskControl.openSettings') }}
      </button>
    </template>

    <div class="space-y-6">
      <!-- 页面加载、日志、解封失败的原因；设置弹窗里的失败在弹窗底栏 -->
      <FormError :message="pageError" data-testid="risk-control-page-error" />
      <div v-if="loading" class="flex items-center justify-center py-16">
        <LoadingSpinner />
      </div>

      <template v-else>
        <StatRow :items="overviewItems" data-testid="risk-overview" />

        <!--
          运行时状态与审核记录（2026-10-05 走查）：原来是卡片套卡片 + 彩色瓷砖，改成与其它页一样的分区（标题 + hairline），
          数字用数字行（黑色）；红绿黄只留在结果徽标与状态点上。
        -->
        <div v-if="showPreBlockRuntimeCard" data-test="pre-block-runtime-cards" class="space-y-8 border-t border-af-hairline pt-6">
          <SheetSection data-test="pre-block-sync-card" :title="t('admin.riskControl.preBlockSyncStatus')" :description="t('admin.riskControl.preBlockSyncHint')">
            <StatRow :items="preBlockMetricItems" data-test="pre-block-metric-grid" />
          </SheetSection>

          <SheetSection data-test="pre-block-api-key-load-card" :title="t('admin.riskControl.preBlockAPIKeyLoad')" :description="t('admin.riskControl.preBlockAPIKeyLoadHint')">
            <template #actions>
              <span class="text-13 tabular-nums text-af-ink-3">{{ preBlockAPIKeyLoadSummaryText }}</span>
            </template>
            <ul
              v-if="preBlockAPIKeyLoads.length > 0"
              data-test="pre-block-api-key-load-list"
              class="max-h-[280px] divide-y divide-af-hairline overflow-y-auto border-y border-af-hairline"
            >
              <li v-for="item in preBlockAPIKeyLoads" :key="item.key_hash || item.index" class="py-3">
                <div class="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                  <div class="min-w-0">
                    <div class="flex min-w-0 items-center gap-2">
                      <span class="h-1.5 w-1.5 flex-shrink-0 rounded-full" :class="apiKeyStatusDotClass(item.status)"></span>
                      <span class="font-mono text-sm font-medium text-af-ink">#{{ item.index + 1 }}</span>
                      <span class="truncate font-mono text-sm text-af-ink-2">{{ item.masked || '-' }}</span>
                    </div>
                    <p class="mt-1 text-xs text-af-ink-3">
                      {{ t('admin.riskControl.preBlockAPIKeyTotals', { total: formatNumber(item.total), success: formatNumber(item.success), errors: formatNumber(item.errors) }) }}
                    </p>
                  </div>
                  <dl class="grid grid-cols-4 gap-4 text-right text-xs text-af-ink-3 sm:min-w-[280px]">
                    <div>
                      <dt>{{ t('admin.riskControl.preBlockKeyActiveShort') }}</dt>
                      <dd class="mt-1 text-sm font-medium tabular-nums text-af-ink">{{ formatNumber(item.active) }}</dd>
                    </div>
                    <div>
                      <dt>{{ t('admin.riskControl.preBlockKeyTotalShort') }}</dt>
                      <dd class="mt-1 text-sm font-medium tabular-nums text-af-ink">{{ formatNumber(item.total) }}</dd>
                    </div>
                    <div>
                      <dt>{{ t('admin.riskControl.preBlockKeyAvgShort') }}</dt>
                      <dd class="mt-1 text-sm font-medium tabular-nums text-af-ink">{{ formatNumber(item.avg_latency_ms) }} ms</dd>
                    </div>
                    <div>
                      <dt>{{ t('admin.riskControl.preBlockKeyLastShort') }}</dt>
                      <dd class="mt-1 text-sm font-medium tabular-nums text-af-ink">{{ formatNumber(item.last_latency_ms) }} ms</dd>
                    </div>
                  </dl>
                </div>
                <!-- 这把密钥的调用量占最多那把的比例 -->
                <div class="mt-2.5 h-1 overflow-hidden rounded-full bg-af-sunken">
                  <div class="h-full rounded-full bg-af-ink" :style="{ width: preBlockAPIKeyLoadWidth(item.total) }"></div>
                </div>
              </li>
            </ul>
            <p v-else class="text-13 text-af-ink-3">
              {{ t('admin.riskControl.preBlockAPIKeyLoadEmpty') }}
            </p>
          </SheetSection>
        </div>

        <SheetSection v-if="showWorkerRuntimeCard" :title="t('admin.riskControl.workerStatus')" :description="t('admin.riskControl.workerStatusHint')">
          <template #actions>
            <span class="text-13 text-af-ink-3">
              {{ t('admin.riskControl.autoRefresh') }}
              <template v-if="status?.last_cleanup_at"> · {{ t('admin.riskControl.lastCleanup', { time: formatDateTime(status.last_cleanup_at) }) }}</template>
            </span>
          </template>

          <div class="space-y-6">
            <StatRow :items="workerMetricItems" />

            <!-- 队列用量：一根细计量条 -->
            <div>
              <div class="flex items-baseline justify-between gap-3 text-13">
                <span class="text-af-ink-3">{{ t('admin.riskControl.queueUsage') }}</span>
                <span class="tabular-nums text-af-ink">
                  {{ formatNumber(status?.queue_length ?? 0) }} / {{ formatNumber(status?.queue_size ?? 0) }}
                  <span class="ml-1 text-af-ink-3">{{ queueUsagePercent }}</span>
                </span>
              </div>
              <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-af-sunken">
                <div class="h-full rounded-full bg-af-ink transition-all duration-300" :style="queueUsageStyle"></div>
              </div>
            </div>

            <div>
              <div class="mb-2 flex items-baseline justify-between gap-3 text-13">
                <span class="text-af-ink-3">{{ t('admin.riskControl.workerPool') }}</span>
                <span class="tabular-nums text-af-ink-3">
                  {{ t('admin.riskControl.workerPoolMeta', { active: status?.active_workers ?? 0, idle: status?.idle_workers ?? 0, total: status?.worker_count ?? 0 }) }}
                </span>
              </div>
              <div class="grid grid-cols-2 gap-2 sm:grid-cols-4 md:grid-cols-6 xl:grid-cols-8 2xl:grid-cols-10">
                <div
                  v-for="worker in workerSlots"
                  :key="worker.id"
                  class="flex h-10 items-center justify-between rounded-md border px-3 transition-colors"
                  :class="workerSlotClass(worker.state)"
                  :title="worker.label"
                >
                  <span class="text-sm font-medium tabular-nums">#{{ worker.id }}</span>
                  <span class="h-2 w-2 rounded-full" :class="workerDotClass(worker.state)"></span>
                </div>
              </div>
            </div>
          </div>
        </SheetSection>

        <SheetSection :title="t('admin.riskControl.records')" :description="t('admin.riskControl.recordsHint')">
          <template #actions>
            <button
              type="button"
              class="rounded-md p-2 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink disabled:opacity-40"
              :disabled="logsLoading"
              :title="t('admin.riskControl.refresh')"
              :aria-label="t('admin.riskControl.refresh')"
              @click="loadLogs"
            >
              <Icon name="refresh" size="md" :class="logsLoading ? 'animate-spin' : ''" />
            </button>
          </template>

          <!-- 一行小控件（与其它列表页同高 32px）；时间仍是起止两个时刻，留空 = 不限 -->
          <ListToolbar class="mb-4">
            <SearchInput
              v-model="filters.search"
              compact
              class="w-full sm:w-56"
              :placeholder="t('admin.riskControl.filters.search')"
              @search="reloadLogsFromFirstPage"
            />
            <FilterChip v-model="filters.result" :label="t('admin.riskControl.table.result')" :options="resultOptions" test-id="risk-filter-result" @change="reloadLogsFromFirstPage" />
            <FilterChip v-model="filters.endpoint" :label="t('admin.riskControl.table.endpoint')" :options="endpointOptions" test-id="risk-filter-endpoint" @change="reloadLogsFromFirstPage" />
            <span class="inline-flex items-center gap-1.5 text-13 text-af-ink-3">
              <input v-model="filters.from" type="datetime-local" class="input h-8 w-auto py-0 text-13" :title="t('admin.riskControl.filters.from')" :aria-label="t('admin.riskControl.filters.from')" @change="reloadLogsFromFirstPage" />
              <span aria-hidden="true">–</span>
              <input v-model="filters.to" type="datetime-local" class="input h-8 w-auto py-0 text-13" :title="t('admin.riskControl.filters.to')" :aria-label="t('admin.riskControl.filters.to')" @change="reloadLogsFromFirstPage" />
            </span>
          </ListToolbar>

          <div class="overflow-x-auto border-t border-af-hairline">
            <table class="min-w-full divide-y divide-af-hairline">
              <thead>
                <tr>
                  <th class="whitespace-nowrap px-3 py-3 text-left text-xs font-medium text-af-ink-3">{{ t('admin.riskControl.table.time') }}</th>
                  <th class="whitespace-nowrap px-3 py-3 text-left text-xs font-medium text-af-ink-3">{{ t('admin.riskControl.table.user') }}</th>
                  <th class="whitespace-nowrap px-3 py-3 text-left text-xs font-medium text-af-ink-3">{{ t('admin.riskControl.table.apiKey') }}</th>
                  <th class="whitespace-nowrap px-3 py-3 text-left text-xs font-medium text-af-ink-3">{{ t('admin.riskControl.table.endpoint') }}</th>
                  <th class="whitespace-nowrap px-3 py-3 text-left text-xs font-medium text-af-ink-3">{{ t('admin.riskControl.table.result') }}</th>
                  <th class="whitespace-nowrap px-3 py-3 text-left text-xs font-medium text-af-ink-3">{{ t('admin.riskControl.table.highest') }}</th>
                  <th class="whitespace-nowrap px-3 py-3 text-left text-xs font-medium text-af-ink-3">{{ t('admin.riskControl.table.actionMeta') }}</th>
                  <th class="whitespace-nowrap px-3 py-3 text-left text-xs font-medium text-af-ink-3">{{ t('admin.riskControl.table.latency') }}</th>
                  <th class="whitespace-nowrap px-3 py-3 text-left text-xs font-medium text-af-ink-3">{{ t('admin.riskControl.table.input') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-af-hairline bg-af-sheet">
                <tr v-if="logsLoading">
                  <td colspan="10" class="px-3 py-12 text-center text-sm text-af-ink-3">{{ t('common.loading') }}</td>
                </tr>
                <tr v-else-if="logs.length === 0">
                  <td colspan="10" class="px-3 py-12 text-center text-sm text-af-ink-3">{{ t('admin.riskControl.emptyLogs') }}</td>
                </tr>
                <template v-else>
                  <tr v-for="row in logs" :key="row.id" class="hover:bg-af-sunken">
                    <td class="whitespace-nowrap px-3 py-4 text-sm text-af-ink-2">{{ formatDateTime(row.created_at) }}</td>
                    <td class="whitespace-nowrap px-3 py-4 text-sm text-af-ink-2">
                      <!-- 不露内部 id：有 id 却没名字就是已删除，没有 id 写 - -->
                      <div>{{ row.user_email || (row.user_id ? t('common.deletedUser') : '-') }}</div>
                    </td>
                    <td class="whitespace-nowrap px-3 py-4 text-sm text-af-ink-2">{{ row.api_key_name || (row.api_key_id ? t('common.deletedKey') : '-') }}</td>
                    <td class="whitespace-nowrap px-3 py-4 text-sm text-af-ink-2">
                      <div>{{ row.endpoint || '-' }}</div>
                      <div class="text-xs text-af-ink-3">{{ row.provider || '-' }} / {{ row.model || '-' }}</div>
                    </td>
                    <td class="whitespace-nowrap px-3 py-4">
                      <span class="inline-flex rounded-md px-2 py-1 text-xs font-medium" :class="resultBadgeClass(row)">
                        {{ resultLabel(row) }}
                      </span>
                      <!-- 审核出错时请求照常放行；原因按错误码给文案 -->
                      <div v-if="errorReason(row)" class="mt-1 text-xs text-af-ink-3">{{ errorReason(row) }}</div>
                    </td>
                    <td class="whitespace-nowrap px-3 py-4 text-sm text-af-ink-2">
                      <template v-if="row.action === 'error'">-</template>
                      <template v-else>
                        <div>{{ row.highest_category || '-' }}</div>
                        <div class="text-xs text-af-ink-3">{{ percent(row.highest_score) }}</div>
                      </template>
                      <div v-if="row.matched_keyword" class="mt-0.5 text-xs font-medium text-af-danger" :title="t('admin.riskControl.matchedKeyword') + ': ' + row.matched_keyword">
                        {{ t('admin.riskControl.matchedKeyword') }}: {{ row.matched_keyword }}
                      </div>
                    </td>
                    <td class="whitespace-nowrap px-3 py-4 text-sm text-af-ink-2">
                      <div>{{ violationCountText(row) }}</div>
                      <div class="text-xs text-af-ink-3">
                        {{ row.email_sent ? t('admin.riskControl.emailSent') : t('admin.riskControl.emailNotSent') }}
                        <span v-if="row.auto_banned"> / {{ t('admin.riskControl.autoBanned') }}</span>
                      </div>
                      <button
                        v-if="canUnbanRow(row)"
                        type="button"
                        class="btn btn-secondary btn-sm mt-2 inline-flex items-center gap-1"
                        :disabled="unbanningUserID === row.user_id"
                        @click="unbanUser(row)"
                      >
                        <Icon name="checkCircle" size="xs" :class="unbanningUserID === row.user_id ? 'animate-spin' : ''" />
                        {{ unbanningUserID === row.user_id ? t('common.processing') : t('admin.riskControl.unbanUser') }}
                      </button>
                      <p v-else-if="isUnbannedRow(row)" class="mt-2 inline-flex items-center gap-1 text-xs text-af-success" data-testid="risk-unbanned">
                        <Icon name="check" size="xs" />
                        {{ t('admin.riskControl.unbanned') }}
                      </p>
                    </td>
                    <td class="whitespace-nowrap px-3 py-4 text-sm text-af-ink-2">
                      <div>{{ latencyText(row.upstream_latency_ms) }}</div>
                      <div v-if="row.queue_delay_ms !== null && row.queue_delay_ms !== undefined" class="text-xs text-af-ink-3">
                        {{ t('admin.riskControl.queueDelay', { ms: row.queue_delay_ms }) }}
                      </div>
                    </td>
                    <td class="w-[320px] max-w-sm px-3 py-4 text-sm text-af-ink-2">
                      <button
                        type="button"
                        class="group flex w-full min-w-0 items-center gap-2 rounded-lg px-2 py-1.5 text-left transition-colors hover:bg-af-sunken"
                        :title="inputSummaryText(row)"
                        @click="openInputDetail(row)"
                      >
                        <span class="min-w-0 flex-1 truncate">{{ inputSummaryText(row) }}</span>
                        <Icon name="eye" size="xs" class="flex-shrink-0 text-af-ink-3 transition-colors group-hover:text-af-brand-hover" />
                      </button>
                    </td>
                  </tr>
                </template>
              </tbody>
            </table>
          </div>

          <Pagination
            v-if="pagination.total > 0"
            :page="pagination.page"
            :total="pagination.total"
            :page-size="pagination.page_size"
            @update:page="onPageChange"
            @update:pageSize="onPageSizeChange"
          />
        </SheetSection>
      </template>

      <!--
        设置弹窗（2026-10-05 瘦身）：页面上只留开关、模式、自动封禁、代理、审核密钥、拦截关键词；
        超时、重试、阈值、封禁次数、通知、保留天数、工作线程等写死在后端（content_moderation.go 常量）。
        分区靠标题与分隔线，不用卡片。
      -->
      <BaseDialog :show="settingsOpen" :title="t('admin.riskControl.settingsTitle')" width="normal" @close="settingsOpen = false">
        <div class="divide-y divide-af-hairline">
          <section class="space-y-5 pb-6">
            <div class="flex items-start justify-between gap-4">
              <div class="min-w-0">
                <p class="text-sm font-medium text-af-ink">{{ t('admin.riskControl.enabled') }}</p>
                <p class="mt-1 text-xs leading-5 text-af-ink-3">{{ t('admin.riskControl.enabledHint') }}</p>
              </div>
              <Toggle v-model="configForm.enabled" />
            </div>
            <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between sm:gap-4">
              <div class="min-w-0">
                <p class="text-sm font-medium text-af-ink">{{ t('admin.riskControl.mode') }}</p>
                <p class="mt-1 text-xs leading-5 text-af-ink-3">{{ modeDescription(configForm.mode) }}</p>
              </div>
              <SegmentedControl v-model="configForm.mode" :options="modeOptions" :label="t('admin.riskControl.mode')" test-id-prefix="risk-mode" />
            </div>
            <div class="flex items-start justify-between gap-4">
              <div class="min-w-0">
                <p class="text-sm font-medium text-af-ink">{{ t('admin.riskControl.autoBan') }}</p>
                <p class="mt-1 text-xs leading-5 text-af-ink-3">{{ t('admin.riskControl.autoBanHint') }}</p>
              </div>
              <Toggle v-model="configForm.auto_ban_enabled" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.riskControl.proxy') }}</label>
              <ProxySelector v-model="configForm.proxy_id" :proxies="proxies" />
              <p class="mt-2 text-xs leading-5 text-af-ink-3">{{ t('admin.riskControl.proxyHint') }}</p>
            </div>
          </section>

          <!-- 审核密钥：已保存的逐个列出（可删），下面追加新的；不再有「增量 / 覆盖」两种写法 -->
          <section class="space-y-3 py-6">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <h3 class="text-sm font-semibold text-af-ink">{{ t('admin.riskControl.apiKeys') }}</h3>
                <p class="mt-1 text-xs leading-5 text-af-ink-3">{{ t('admin.riskControl.apiKeyFreezeRule') }}</p>
              </div>
              <button
                type="button"
                class="btn btn-secondary btn-sm shrink-0"
                :disabled="apiKeyTesting !== '' || effectiveStoredApiKeyCount === 0 || pendingDeletedApiKeyCount > 0"
                @click="testApiKeys(false)"
              >
                {{ apiKeyTesting === 'stored' ? t('admin.riskControl.testingApiKeys') : t('admin.riskControl.testStoredApiKeys') }}
              </button>
            </div>
            <ul v-if="apiKeyRows.length > 0" class="max-h-72 divide-y divide-af-hairline overflow-y-auto border-y border-af-hairline" data-testid="risk-api-key-list">
              <li
                v-for="(row, index) in apiKeyRows"
                :key="apiKeyRowKey(row, index)"
                class="flex items-start justify-between gap-3 py-2.5"
                :class="isStoredApiKeyPendingDelete(row) ? 'opacity-60' : ''"
              >
                <div class="min-w-0">
                  <div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
                    <span class="h-1.5 w-1.5 flex-shrink-0 rounded-full" :class="apiKeyStatusDotClass(row.status)"></span>
                    <span class="truncate font-mono text-sm text-af-ink">{{ row.masked || '-' }}</span>
                    <span class="text-xs text-af-ink-2">{{ apiKeyStatusLabel(row.status) }}</span>
                    <span v-if="apiKeyRowTag(row)" class="text-xs text-af-ink-3">· {{ apiKeyRowTag(row) }}</span>
                  </div>
                  <p class="mt-1 text-xs leading-5 text-af-ink-3">{{ apiKeyStatusMeta(row) }}</p>
                  <p v-if="row.last_error" class="text-xs leading-5 text-af-warning">{{ moderationErrorText(row.last_error) }}</p>
                </div>
                <button
                  v-if="row.configured"
                  type="button"
                  class="inline-flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-md text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink"
                  :title="isStoredApiKeyPendingDelete(row) ? t('admin.riskControl.undoDeleteApiKey') : t('admin.riskControl.deleteApiKey')"
                  :aria-label="isStoredApiKeyPendingDelete(row) ? t('admin.riskControl.undoDeleteApiKey') : t('admin.riskControl.deleteApiKey')"
                  @click="toggleDeleteStoredApiKey(row)"
                >
                  <Icon :name="isStoredApiKeyPendingDelete(row) ? 'refresh' : 'trash'" size="xs" />
                </button>
              </li>
            </ul>
            <p v-else class="text-13 text-af-ink-3">{{ t('admin.riskControl.apiKeyHealthEmpty') }}</p>
            <div>
              <label class="input-label" for="risk-api-keys-input">{{ t('admin.riskControl.apiKeysAdd') }}</label>
              <textarea
                id="risk-api-keys-input"
                v-model="configForm.api_keys_text"
                class="input min-h-20 resize-y font-mono text-sm"
                :placeholder="t('admin.riskControl.apiKeysPlaceholder')"
                autocomplete="new-password"
              ></textarea>
              <div class="mt-2 flex items-start justify-between gap-3">
                <p class="text-xs leading-5 text-af-ink-3">{{ t('admin.riskControl.apiKeysHint') }}</p>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm shrink-0"
                  :disabled="apiKeyTesting !== '' || inputApiKeyCount === 0"
                  @click="testApiKeys(true)"
                >
                  {{ apiKeyTesting === 'input' ? t('admin.riskControl.testingApiKeys') : t('admin.riskControl.testInputApiKeys', { count: inputApiKeyCount }) }}
                </button>
              </div>
            </div>
          </section>

          <!-- 试跑：用已保存的密钥审一段文字或一张图片，看各分类分数；只在填了内容时出结果 -->
          <section class="space-y-3 py-6" @paste="handleModerationImagePaste">
            <div class="min-w-0">
              <h3 class="text-sm font-semibold text-af-ink">{{ t('admin.riskControl.auditTest') }}</h3>
              <p class="mt-1 text-xs leading-5 text-af-ink-3">{{ t('admin.riskControl.auditTestHint') }}</p>
            </div>
            <textarea
              v-model="moderationTestPrompt"
              class="input min-h-20 resize-y text-sm"
              :placeholder="t('admin.riskControl.auditTestPromptPlaceholder')"
              @dragover.prevent
              @drop.prevent="handleModerationImageDrop"
            ></textarea>
            <div class="flex flex-wrap items-center gap-2">
              <div
                v-for="(image, index) in moderationTestImages"
                :key="image.slice(0, 64) + index"
                class="group relative h-12 w-12 overflow-hidden rounded-md border border-af-hairline"
              >
                <img :src="image" alt="" class="h-full w-full object-cover" />
                <button
                  type="button"
                  class="absolute inset-0 flex items-center justify-center bg-black/50 text-af-on-brand opacity-0 transition-opacity group-hover:opacity-100"
                  :aria-label="t('admin.riskControl.removeAuditTestImage')"
                  @click="removeModerationTestImage(index)"
                >
                  <Icon name="x" size="xs" :stroke-width="2" />
                </button>
              </div>
              <label v-if="moderationTestImages.length < maxModerationTestImages" class="btn btn-ghost btn-sm inline-flex cursor-pointer items-center gap-1">
                <Icon name="plus" size="xs" />
                {{ t('admin.riskControl.addAuditTestImage') }}
                <input type="file" accept="image/*" class="sr-only" @change="handleModerationImageUpload" />
              </label>
              <div class="ml-auto flex items-center gap-2">
                <button
                  v-if="hasModerationAuditInput || moderationTestResult"
                  type="button"
                  class="btn btn-ghost btn-sm"
                  @click="clearModerationTestInput"
                >
                  {{ t('admin.riskControl.clearAuditTest') }}
                </button>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="apiKeyTesting !== '' || !hasModerationAuditInput || effectiveStoredApiKeyCount === 0"
                  @click="runAuditTest"
                >
                  {{ apiKeyTesting === 'audit' ? t('admin.riskControl.testingApiKeys') : t('admin.riskControl.auditTestRun') }}
                </button>
              </div>
            </div>
            <div v-if="moderationTestResult" class="space-y-3 border-t border-af-hairline pt-3" data-testid="risk-audit-test-result">
              <div class="flex items-center justify-between gap-3">
                <p class="min-w-0 text-13 text-af-ink-2">
                  {{ t('admin.riskControl.auditTestHighest', { category: moderationTestResult.highest_category || '-', score: percent(moderationTestResult.highest_score) }) }}
                </p>
                <span class="inline-flex shrink-0 items-center gap-1.5 text-xs font-medium" :class="moderationTestResult.flagged ? 'text-af-danger' : 'text-af-success'">
                  <span class="h-1.5 w-1.5 rounded-full" :class="moderationTestResult.flagged ? 'bg-af-danger' : 'bg-af-success'"></span>
                  {{ moderationTestResult.flagged ? t('admin.riskControl.auditTestFlagged') : t('admin.riskControl.auditTestPassed') }}
                </span>
              </div>
              <ul class="max-h-52 space-y-2 overflow-y-auto pr-1">
                <li v-for="score in moderationScoreRows" :key="score.category">
                  <div class="mb-1 flex items-center justify-between gap-3 text-xs">
                    <span class="truncate text-af-ink-2">{{ score.category }}</span>
                    <span class="font-mono tabular-nums text-af-ink-3">{{ percent(score.score) }} / {{ percent(score.threshold) }}</span>
                  </div>
                  <div class="h-1 overflow-hidden rounded-full bg-af-sunken">
                    <div class="h-full rounded-full" :class="score.hit ? 'bg-af-danger' : 'bg-af-ink'" :style="{ width: percentWidth(score.score) }"></div>
                  </div>
                </li>
              </ul>
            </div>
          </section>

          <section class="space-y-2 pt-6">
            <div class="flex items-baseline justify-between gap-3">
              <h3 class="text-sm font-semibold text-af-ink">{{ t('admin.riskControl.blockedKeywords') }}</h3>
              <span class="text-xs tabular-nums text-af-ink-3">{{ t('admin.riskControl.blockedKeywordCount', { count: blockedKeywordCount }) }}</span>
            </div>
            <p class="text-xs leading-5 text-af-ink-3">{{ t('admin.riskControl.blockedKeywordsHint') }}</p>
            <p v-if="configForm.mode !== 'pre_block' && blockedKeywordCount > 0" class="text-xs leading-5 text-af-warning">
              {{ t('admin.riskControl.blockedKeywordsModeWarning') }}
            </p>
            <textarea
              v-model="configForm.blocked_keywords_text"
              class="input min-h-32 resize-y font-mono text-sm"
              :placeholder="t('admin.riskControl.blockedKeywordsPlaceholder')"
            ></textarea>
          </section>
        </div>

        <template #footer>
          <div class="flex justify-end gap-2">
            <FormError class="mr-auto self-center" :message="settingsError" data-testid="risk-control-settings-error" />
            <button type="button" class="btn btn-secondary" @click="settingsOpen = false">{{ t('common.cancel') }}</button>
            <button type="button" class="btn btn-primary inline-flex items-center gap-2" :disabled="saving" @click="saveConfig">
              <Icon v-if="saving" name="refresh" size="sm" class="animate-spin" />
              {{ saving ? t('common.saving') : t('admin.riskControl.saveConfig') }}
            </button>
          </div>
        </template>
      </BaseDialog>

      <BaseDialog
        :show="inputDetailRow !== null"
        :title="t('admin.riskControl.inputDetailTitle')"
        width="wide"
        @close="closeInputDetail"
      >
        <div v-if="inputDetailRow" class="space-y-5">
          <dl class="grid grid-cols-2 gap-x-6 gap-y-4 text-13 sm:grid-cols-4">
            <div class="min-w-0">
              <dt class="text-xs text-af-ink-3">{{ t('admin.riskControl.table.time') }}</dt>
              <dd class="mt-1 truncate text-af-ink">{{ formatDateTime(inputDetailRow.created_at) }}</dd>
            </div>
            <div class="min-w-0">
              <dt class="text-xs text-af-ink-3">{{ t('admin.riskControl.table.user') }}</dt>
              <dd class="mt-1 truncate text-af-ink">{{ inputDetailRow.user_email || (inputDetailRow.user_id ? t('common.deletedUser') : '-') }}</dd>
            </div>
            <div class="min-w-0">
              <dt class="text-xs text-af-ink-3">{{ t('admin.riskControl.table.result') }}</dt>
              <dd class="mt-1">
                <span class="inline-flex rounded-md px-2 py-0.5 text-xs font-medium" :class="resultBadgeClass(inputDetailRow)">
                  {{ resultLabel(inputDetailRow) }}
                </span>
              </dd>
            </div>
            <div class="min-w-0">
              <dt class="text-xs text-af-ink-3">{{ t('admin.riskControl.table.highest') }}</dt>
              <dd class="mt-1 truncate text-af-ink">
                <template v-if="inputDetailRow.action === 'error'">-</template>
                <template v-else>{{ inputDetailRow.highest_category || '-' }} · {{ percent(inputDetailRow.highest_score) }}</template>
              </dd>
            </div>
            <div v-if="inputDetailRow.matched_keyword" class="col-span-2 min-w-0">
              <dt class="text-xs text-af-ink-3">{{ t('admin.riskControl.matchedKeyword') }}</dt>
              <dd class="mt-1 truncate text-af-danger" :title="inputDetailRow.matched_keyword">{{ inputDetailRow.matched_keyword }}</dd>
            </div>
            <div v-if="errorReason(inputDetailRow)" class="col-span-2 min-w-0">
              <dt class="text-xs text-af-ink-3">{{ t('admin.riskControl.errorReasonLabel') }}</dt>
              <dd class="mt-1 text-af-warning">{{ errorReason(inputDetailRow) }}</dd>
            </div>
          </dl>

          <div class="border-t border-af-hairline pt-4">
            <p class="text-xs text-af-ink-3">
              {{ t('admin.riskControl.inputDetailContent') }} · {{ inputDetailRow.endpoint || '-' }} · {{ inputDetailRow.provider || '-' }} / {{ inputDetailRow.model || '-' }}
            </p>
            <p class="mt-3 max-h-[420px] overflow-auto whitespace-pre-wrap break-words text-sm leading-6 text-af-ink">{{ inputDetailText }}</p>
          </div>
        </div>

        <template #footer>
          <div class="flex justify-end">
            <button type="button" class="btn btn-secondary" @click="closeInputDetail">{{ t('common.close') }}</button>
          </div>
        </template>
      </BaseDialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import FormError from '@/components/common/FormError.vue'
import Icon from '@/components/icons/Icon.vue'
import StatRow from '@/components/user/shell/StatRow.vue'
import SheetSection from '@/components/user/shell/SheetSection.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import FilterChip from '@/components/common/FilterChip.vue'
import SegmentedControl from '@/components/common/SegmentedControl.vue'
import { ListToolbar } from '@/components/admin/list'
import type { FilterOption } from '@/components/common/types'
import type { StatItem } from '@/components/user/shell/types'
import Toggle from '@/components/common/Toggle.vue'
import Pagination from '@/components/common/Pagination.vue'
import ProxySelector from '@/components/common/ProxySelector.vue'
import { adminAPI } from '@/api/admin'
import type {
  ContentModerationAPIKeyLoad,
  ContentModerationAPIKeyStatus,
  ContentModerationConfig,
  ContentModerationLog,
  ContentModerationRuntimeStatus,
  ContentModerationTestAuditResult,
  ModerationMode,
  UpdateContentModerationConfig,
} from '@/api/admin/riskControl'
import type { Proxy } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime as formatDateTimeValue } from '@/utils/format'

type WorkerSlotState = 'active' | 'idle' | 'disabled'
/** 正在跑哪一种测试；空串 = 没在测 */
type ApiKeyTestTarget = '' | 'stored' | 'input' | 'audit'
type ModerationScoreRow = {
  category: string
  score: number
  threshold: number
  hit: boolean
}

const maxModerationTestImages = 1
const maxModerationTestImageSize = 8 * 1024 * 1024

const { t } = useI18n()

const loading = ref(true)
const saving = ref(false)
const logsLoading = ref(false)
const statusLoading = ref(false)
const apiKeyTesting = ref<ApiKeyTestTarget>('')
const unbanningUserID = ref<number | null>(null)
const settingsOpen = ref(false)
const pageError = ref('')
const settingsError = ref('')
const proxies = ref<Proxy[]>([])
const logs = ref<ContentModerationLog[]>([])
const status = ref<ContentModerationRuntimeStatus | null>(null)
/** 最近一次从后端拿到的设置；页面概览按它显示，打开设置弹窗时用它重置表单（取消后不留半截改动） */
const savedConfig = ref<ContentModerationConfig | null>(null)
const testedApiKeyStatuses = ref<ContentModerationAPIKeyStatus[]>([])
const pendingDeleteApiKeyHashes = ref<string[]>([])
const moderationTestPrompt = ref('')
const moderationTestImages = ref<string[]>([])
const moderationTestResult = ref<ContentModerationTestAuditResult | null>(null)
const inputDetailRow = ref<ContentModerationLog | null>(null)
let statusTimer: number | null = null

const configForm = reactive({
  enabled: false,
  mode: 'pre_block' as ModerationMode,
  proxy_id: null as number | null,
  api_keys_text: '',
  api_key_count: 0,
  api_key_statuses: [] as ContentModerationAPIKeyStatus[],
  auto_ban_enabled: true,
  blocked_keywords_text: '',
})

const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0,
  pages: 1,
})

const filters = reactive({
  result: '',
  endpoint: '',
  search: '',
  from: '',
  to: '',
})

const modeOptions = computed<Array<{ key: ModerationMode; label: string }>>(() => [
  { key: 'pre_block', label: t('admin.riskControl.modePreBlock') },
  { key: 'observe', label: t('admin.riskControl.modeObserve') },
])

// 未命中的请求不留记录，所以没有「未命中」这一项
const resultOptions = computed<FilterOption[]>(() => [
  { value: 'hit', label: t('admin.riskControl.result.hit') },
  { value: 'blocked', label: t('admin.riskControl.result.blocked') },
  { value: 'error', label: t('admin.riskControl.result.error') },
])

const endpointOptions = computed<FilterOption[]>(() => [
  { value: '/v1/messages', label: '/v1/messages' },
  { value: '/v1/responses', label: '/v1/responses' },
  { value: '/v1/chat/completions', label: '/v1/chat/completions' },
  { value: '/v1beta/models', label: '/v1beta/models' },
  { value: '/v1/images/generations', label: '/v1/images/generations' },
  { value: '/v1/images/edits', label: '/v1/images/edits' },
])

const inputApiKeyCount = computed(() => parseApiKeys(configForm.api_keys_text).length)

const blockedKeywordList = computed(() => parseBlockedKeywords(configForm.blocked_keywords_text))

const blockedKeywordCount = computed(() => blockedKeywordList.value.length)

const pendingDeletedApiKeyCount = computed(() => pendingDeleteApiKeyHashes.value.length)

const effectiveStoredApiKeyCount = computed(() => Math.max(0, configForm.api_key_count - pendingDeletedApiKeyCount.value))

const hasModerationAuditInput = computed(() => {
  return moderationTestPrompt.value.trim() !== '' || moderationTestImages.value.length > 0
})

const savedApiKeyRows = computed<ContentModerationAPIKeyStatus[]>(() => {
  const rows = status.value?.api_key_statuses?.length
    ? status.value.api_key_statuses
    : configForm.api_key_statuses
  return Array.isArray(rows) ? rows : []
})

const apiKeyRows = computed<ContentModerationAPIKeyStatus[]>(() => [
  ...savedApiKeyRows.value,
  ...testedApiKeyStatuses.value,
])

const apiKeyHealthSummary = computed(() => {
  const counts: Record<ContentModerationAPIKeyStatus['status'], number> = { ok: 0, frozen: 0, error: 0, unknown: 0 }
  for (const row of savedApiKeyRows.value) {
    counts[row.status] = (counts[row.status] ?? 0) + 1
  }
  return (['ok', 'frozen', 'error', 'unknown'] as Array<ContentModerationAPIKeyStatus['status']>)
    .filter((item) => counts[item] > 0)
    .map((item) => `${apiKeyStatusLabel(item)} ${counts[item]}`)
    .join(' · ')
})

const overviewItems = computed<StatItem[]>(() => {
  const keyCount = savedConfig.value?.api_key_count ?? 0
  return [
    {
      key: 'status',
      label: t('admin.riskControl.overview.status'),
      value: runtimeStatusText.value,
      hint: modeLabel(savedConfig.value?.mode ?? 'pre_block'),
    },
    {
      key: 'api-key',
      label: t('admin.riskControl.overview.apiKey'),
      value: keyCount > 0 ? t('admin.riskControl.apiKeyCount', { count: keyCount }) : t('admin.riskControl.notConfigured'),
      hint: keyCount > 0 ? apiKeyHealthSummary.value || undefined : undefined,
    },
    {
      key: 'logs',
      label: t('admin.riskControl.overview.logs'),
      value: formatNumber(pagination.total),
      hint: t('admin.riskControl.overview.currentFilter'),
    },
  ]
})

const moderationScoreRows = computed<ModerationScoreRow[]>(() => {
  const result = moderationTestResult.value
  if (!result) return []
  return Object.entries(result.category_scores || {})
    .map(([category, score]) => {
      const threshold = result.thresholds?.[category] ?? 1
      return {
        category,
        score,
        threshold,
        hit: score >= threshold,
      }
    })
    .sort((a, b) => b.score - a.score)
})

const inputDetailText = computed(() => {
  if (!inputDetailRow.value) return '-'
  return inputSummaryText(inputDetailRow.value)
})

const queueUsagePercent = computed(() => `${Math.min(100, Math.max(0, status.value?.queue_usage_percent ?? 0)).toFixed(1)}%`)

const queueUsageStyle = computed(() => ({
  width: queueUsagePercent.value,
}))

const runtimeMode = computed<ModerationMode>(() => status.value?.mode ?? savedConfig.value?.mode ?? 'pre_block')

const showPreBlockRuntimeCard = computed(() => runtimeMode.value === 'pre_block')

const showWorkerRuntimeCard = computed(() => runtimeMode.value === 'observe')

// 数字一律黑色：拦截 / 异常是计数，不是状态，不上红黄底；六个数一行放不下附注，只留标签（2026-10-05）
const preBlockMetricItems = computed<StatItem[]>(() => [
  { key: 'active', label: t('admin.riskControl.preBlockActive'), value: formatNumber(status.value?.pre_block_active ?? 0) },
  { key: 'checked', label: t('admin.riskControl.preBlockChecked'), value: formatNumber(status.value?.pre_block_checked ?? 0) },
  { key: 'allowed', label: t('admin.riskControl.preBlockAllowed'), value: formatNumber(status.value?.pre_block_allowed ?? 0) },
  { key: 'blocked', label: t('admin.riskControl.preBlockBlocked'), value: formatNumber(status.value?.pre_block_blocked ?? 0) },
  { key: 'errors', label: t('admin.riskControl.preBlockErrors'), value: formatNumber(status.value?.pre_block_errors ?? 0) },
  { key: 'latency', label: t('admin.riskControl.preBlockAvgLatency'), value: `${formatNumber(status.value?.pre_block_avg_latency_ms ?? 0)} ms` },
])

const workerMetricItems = computed<StatItem[]>(() => [
  { key: 'active-workers', label: t('admin.riskControl.activeWorkers'), value: String(status.value?.active_workers ?? 0) },
  { key: 'idle-workers', label: t('admin.riskControl.idleWorkers'), value: String(status.value?.idle_workers ?? 0) },
  { key: 'processed', label: t('admin.riskControl.processed'), value: formatNumber(status.value?.processed ?? 0) },
  { key: 'dropped-errors', label: t('admin.riskControl.droppedErrors'), value: formatNumber((status.value?.dropped ?? 0) + (status.value?.errors ?? 0)) },
])

const preBlockAPIKeyLoads = computed<ContentModerationAPIKeyLoad[]>(() => (
  [...(status.value?.pre_block_api_key_loads ?? [])].sort((a, b) => a.index - b.index)
))

const preBlockAPIKeyMaxTotal = computed(() => Math.max(1, ...preBlockAPIKeyLoads.value.map((item) => item.total || 0)))

const preBlockAPIKeyLoadSummaryText = computed(() => t('admin.riskControl.preBlockAPIKeyLoadSummary', {
  active: formatNumber(status.value?.pre_block_api_key_active ?? 0),
  available: formatNumber(status.value?.pre_block_api_key_available_count ?? 0),
  total: formatNumber(status.value?.pre_block_api_key_total_calls ?? 0),
  workerActive: formatNumber(status.value?.active_workers ?? 0),
  workerTotal: formatNumber(status.value?.worker_count ?? 0),
}))

function preBlockAPIKeyLoadWidth(total: number): string {
  return `${Math.min(100, Math.max(0, (total / preBlockAPIKeyMaxTotal.value) * 100)).toFixed(1)}%`
}

const workerSlots = computed(() => {
  const total = Math.max(0, status.value?.worker_count ?? 0)
  const active = Math.max(0, status.value?.active_workers ?? 0)
  const enabled = Boolean(status.value?.risk_control_enabled && status.value?.enabled)
  return Array.from({ length: total }, (_, index) => ({
    id: index + 1,
    state: (!enabled ? 'disabled' : index < active ? 'active' : 'idle') as WorkerSlotState,
    label: !enabled
      ? t('admin.riskControl.workerDisabled')
      : index < active
        ? t('admin.riskControl.workerActive')
        : t('admin.riskControl.workerIdle'),
  }))
})

const runtimeStatusText = computed(() => {
  if (!status.value?.risk_control_enabled) return t('admin.riskControl.riskSwitchOff')
  if (!savedConfig.value?.enabled) return t('admin.riskControl.overview.disabled')
  return t('admin.riskControl.overview.enabled')
})

function applyConfig(config: ContentModerationConfig) {
  savedConfig.value = config
  configForm.enabled = config.enabled
  configForm.mode = config.mode === 'observe' ? 'observe' : 'pre_block'
  configForm.proxy_id = config.proxy_id || null
  configForm.api_keys_text = ''
  configForm.api_key_count = config.api_key_count || 0
  configForm.api_key_statuses = Array.isArray(config.api_key_statuses) ? [...config.api_key_statuses] : []
  configForm.auto_ban_enabled = config.auto_ban_enabled
  configForm.blocked_keywords_text = Array.isArray(config.blocked_keywords) ? config.blocked_keywords.join('\n') : ''
  pendingDeleteApiKeyHashes.value = []
  testedApiKeyStatuses.value = []
}

async function loadAll() {
  loading.value = true
  pageError.value = ''
  try {
    const [config, runtimeStatus, proxyItems] = await Promise.all([
      adminAPI.riskControl.getConfig(),
      adminAPI.riskControl.getStatus(),
      // 代理列表加载失败不阻塞风控页面（仅影响下拉可选项）
      adminAPI.proxies.getAll().catch(() => [] as Proxy[]),
    ])
    applyConfig(config)
    status.value = runtimeStatus
    proxies.value = proxyItems
    if (Array.isArray(runtimeStatus.api_key_statuses)) {
      configForm.api_key_statuses = [...runtimeStatus.api_key_statuses]
      prunePendingDeleteAPIKeyHashes()
    }
    await loadLogs()
  } catch (err: unknown) {
    pageError.value = extractApiErrorMessage(err, t('admin.riskControl.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function loadStatus(silent = true) {
  statusLoading.value = true
  try {
    const runtimeStatus = await adminAPI.riskControl.getStatus()
    status.value = runtimeStatus
    if (Array.isArray(runtimeStatus.api_key_statuses)) {
      configForm.api_key_statuses = [...runtimeStatus.api_key_statuses]
      prunePendingDeleteAPIKeyHashes()
    }
  } catch (err: unknown) {
    if (!silent) {
      pageError.value = extractApiErrorMessage(err, t('admin.riskControl.statusFailed'))
    }
  } finally {
    statusLoading.value = false
  }
}

async function saveConfig() {
  saving.value = true
  settingsError.value = ''
  try {
    const payload: UpdateContentModerationConfig = {
      enabled: configForm.enabled,
      mode: configForm.mode,
      // 后端语义：0 清除代理（直连），>0 指定代理
      proxy_id: configForm.proxy_id ?? 0,
      auto_ban_enabled: configForm.auto_ban_enabled,
      blocked_keywords: blockedKeywordList.value,
    }
    const keys = parseApiKeys(configForm.api_keys_text)
    if (keys.length > 0) {
      payload.api_keys = keys
    }
    if (pendingDeleteApiKeyHashes.value.length > 0) {
      payload.delete_api_key_hashes = [...pendingDeleteApiKeyHashes.value]
    }

    const updated = await adminAPI.riskControl.updateConfig(payload)
    applyConfig(updated)
    settingsOpen.value = false
    await Promise.all([loadStatus(true), loadLogs()])
  } catch (err: unknown) {
    settingsError.value = extractApiErrorMessage(err, t('admin.riskControl.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function loadLogs() {
  logsLoading.value = true
  try {
    const params = {
      page: pagination.page,
      page_size: pagination.page_size,
      result: filters.result || undefined,
      endpoint: filters.endpoint || undefined,
      search: filters.search || undefined,
      from: normalizeDateTimeLocal(filters.from),
      to: normalizeDateTimeLocal(filters.to),
    }
    const result = await adminAPI.riskControl.listLogs(params)
    logs.value = result.items
    pagination.total = result.total
    pagination.page = result.page
    pagination.page_size = result.page_size
    pagination.pages = result.pages
  } catch (err: unknown) {
    pageError.value = extractApiErrorMessage(err, t('admin.riskControl.logsFailed'))
  } finally {
    logsLoading.value = false
  }
}

function canUnbanRow(row: ContentModerationLog): boolean {
  return Boolean(row.auto_banned && row.user_id && row.user_status === 'disabled')
}

/** 被自动封禁过、现在账户已恢复（在这里或用户管理里解封的） */
function isUnbannedRow(row: ContentModerationLog): boolean {
  return Boolean(row.auto_banned && row.user_id && row.user_status && row.user_status !== 'disabled')
}

/** 审核出错的记录里 error 存的是错误码，其余记录（如网络安全策略）存的是上游原文，当摘要显示 */
function inputSummaryText(row: ContentModerationLog): string {
  if (row.input_excerpt) return row.input_excerpt
  if (row.action !== 'error' && row.error) return row.error
  return '-'
}

function errorReason(row: ContentModerationLog): string {
  if (row.action !== 'error') return ''
  return moderationErrorText(row.error)
}

function openInputDetail(row: ContentModerationLog) {
  inputDetailRow.value = row
}

function closeInputDetail() {
  inputDetailRow.value = null
}

async function unbanUser(row: ContentModerationLog) {
  if (!row.user_id || unbanningUserID.value !== null) return
  unbanningUserID.value = row.user_id
  pageError.value = ''
  try {
    const result = await adminAPI.riskControl.unbanUser(row.user_id)
    // 同一用户的所有记录一起改；按钮随之换成「已解封」
    logs.value = logs.value.map((item) => {
      if (item.user_id !== row.user_id) return item
      return { ...item, user_status: result.status }
    })
  } catch (err: unknown) {
    pageError.value = extractApiErrorMessage(err, t('admin.riskControl.unbanFailed'))
  } finally {
    unbanningUserID.value = null
  }
}

function openSettings() {
  if (savedConfig.value) {
    applyConfig(savedConfig.value)
  }
  clearModerationTestInput()
  settingsError.value = ''
  settingsOpen.value = true
}

function reloadLogsFromFirstPage() {
  pagination.page = 1
  void loadLogs()
}

function onPageChange(page: number) {
  pagination.page = page
  void loadLogs()
}

function onPageSizeChange(pageSize: number) {
  pagination.page = 1
  pagination.page_size = pageSize
  void loadLogs()
}

async function testApiKeys(useInputKeys: boolean) {
  const keys = useInputKeys ? parseApiKeys(configForm.api_keys_text) : []
  if (useInputKeys && keys.length === 0) return
  settingsError.value = ''
  apiKeyTesting.value = useInputKeys ? 'input' : 'stored'
  try {
    const result = await adminAPI.riskControl.testAPIKeys({
      api_keys: keys,
      // 与保存语义一致：0 强制直连，>0 指定代理，确保测试与实际审计走同一条链路
      proxy_id: configForm.proxy_id ?? 0,
    })
    if (useInputKeys) {
      testedApiKeyStatuses.value = result.items.map((item) => ({ ...item, configured: false }))
    } else {
      configForm.api_key_statuses = result.items
      testedApiKeyStatuses.value = []
      await loadStatus(true)
    }
  } catch (err: unknown) {
    settingsError.value = extractApiErrorMessage(err, t('admin.riskControl.apiKeyTestFailed'))
  } finally {
    apiKeyTesting.value = ''
  }
}

async function runAuditTest() {
  if (!hasModerationAuditInput.value) return
  settingsError.value = ''
  apiKeyTesting.value = 'audit'
  try {
    const result = await adminAPI.riskControl.testAPIKeys({
      proxy_id: configForm.proxy_id ?? 0,
      prompt: moderationTestPrompt.value,
      images: moderationTestImages.value,
    })
    moderationTestResult.value = result.audit_result ?? null
    mergeConfiguredAPIKeyStatuses(result.items)
    if (!result.audit_result) {
      settingsError.value = t('admin.riskControl.auditTestNoResult')
    }
    await loadStatus(true)
  } catch (err: unknown) {
    settingsError.value = extractApiErrorMessage(err, t('admin.riskControl.apiKeyTestFailed'))
  } finally {
    apiKeyTesting.value = ''
  }
}

/** 试跑只用到一把已保存的密钥，只更新那一把的状态 */
function mergeConfiguredAPIKeyStatuses(items: ContentModerationAPIKeyStatus[]) {
  const updates = new Map(items.map((item) => [item.key_hash, item]))
  configForm.api_key_statuses = configForm.api_key_statuses.map((item) => updates.get(item.key_hash) ?? item)
}

function toggleDeleteStoredApiKey(row: ContentModerationAPIKeyStatus) {
  if (!row.configured || !row.key_hash) return
  const index = pendingDeleteApiKeyHashes.value.indexOf(row.key_hash)
  if (index >= 0) {
    pendingDeleteApiKeyHashes.value.splice(index, 1)
    return
  }
  pendingDeleteApiKeyHashes.value.push(row.key_hash)
}

function isStoredApiKeyPendingDelete(row: ContentModerationAPIKeyStatus): boolean {
  return row.configured && row.key_hash !== '' && pendingDeleteApiKeyHashes.value.includes(row.key_hash)
}

function prunePendingDeleteAPIKeyHashes() {
  const currentHashes = new Set(savedApiKeyRows.value.map((row) => row.key_hash).filter(Boolean))
  pendingDeleteApiKeyHashes.value = pendingDeleteApiKeyHashes.value.filter((hash) => currentHashes.has(hash))
}

function clearModerationTestInput() {
  moderationTestPrompt.value = ''
  moderationTestImages.value = []
  moderationTestResult.value = null
}

function removeModerationTestImage(index: number) {
  moderationTestImages.value.splice(index, 1)
}

async function handleModerationImageUpload(event: Event) {
  const input = event.target as HTMLInputElement
  await addModerationTestFiles(input.files)
  input.value = ''
}

async function handleModerationImageDrop(event: DragEvent) {
  await addModerationTestFiles(event.dataTransfer?.files ?? null)
}

async function handleModerationImagePaste(event: ClipboardEvent) {
  const files = Array.from(event.clipboardData?.files ?? []).filter((file) => file.type.startsWith('image/'))
  if (files.length === 0) return
  event.preventDefault()
  await addModerationTestFiles(files)
}

async function addModerationTestFiles(files: FileList | File[] | null) {
  if (!files) return
  const items = Array.from(files).filter((file) => file.type.startsWith('image/'))
  settingsError.value = ''
  for (const file of items) {
    if (moderationTestImages.value.length >= maxModerationTestImages) {
      settingsError.value = t('admin.riskControl.auditTestImageLimit', { count: maxModerationTestImages })
      return
    }
    if (file.size > maxModerationTestImageSize) {
      settingsError.value = t('admin.riskControl.auditTestImageTooLarge')
      continue
    }
    try {
      moderationTestImages.value.push(await fileToDataURL(file))
    } catch {
      settingsError.value = t('admin.riskControl.auditTestImageReadFailed')
    }
  }
}

function fileToDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result || ''))
    reader.onerror = () => reject(reader.error)
    reader.readAsDataURL(file)
  })
}

function modeLabel(mode: ModerationMode): string {
  const found = modeOptions.value.find((option) => option.key === mode)
  return found?.label ?? mode
}

function modeDescription(mode: ModerationMode): string {
  return mode === 'observe' ? t('admin.riskControl.modeObserveDesc') : t('admin.riskControl.modePreBlockDesc')
}

function resultLabel(row: ContentModerationLog): string {
  if (row.action === 'cyber_policy') return t('admin.riskControl.action.cyberPolicy')
  if (row.action === 'keyword_block') return t('admin.riskControl.action.keywordBlock')
  if (row.action === 'block') return t('admin.riskControl.action.block')
  if (row.action === 'error') return t('admin.riskControl.action.error')
  if (row.flagged) return t('admin.riskControl.action.hitAllowed')
  return t('admin.riskControl.result.pass')
}

function resultBadgeClass(row: ContentModerationLog): string {
  if (row.action === 'block' || row.action === 'keyword_block' || row.action === 'cyber_policy') return 'bg-af-danger-tint text-af-danger'
  if (row.action === 'error') return 'bg-af-warning-tint text-af-warning'
  return 'bg-af-sunken text-af-ink-2'
}

/**
 * 审核出错的原因：后端只存错误码（content_moderation.go moderationErrorCode），这里按码给文案；
 * 认不出的（改版前的旧记录存的是原文）原样显示。
 */
function moderationErrorText(code: string): string {
  if (!code) return ''
  const httpMatch = /^http_(\d{3})$/.exec(code)
  if (httpMatch) {
    const httpStatus = Number(httpMatch[1])
    if (httpStatus === 401 || httpStatus === 403) return t('admin.riskControl.errorReason.httpAuth', { status: httpStatus })
    if (httpStatus === 429) return t('admin.riskControl.errorReason.httpRateLimit', { status: httpStatus })
    return t('admin.riskControl.errorReason.http', { status: httpStatus })
  }
  const known: Record<string, string> = {
    no_api_key: t('admin.riskControl.errorReason.noApiKey'),
    timeout: t('admin.riskControl.errorReason.timeout'),
    network: t('admin.riskControl.errorReason.network'),
    invalid_response: t('admin.riskControl.errorReason.invalidResponse'),
  }
  return known[code] ?? code
}

/** 格子只分忙 / 闲两种底色；空闲是常态，不上绿底，绿色只留在状态点上 */
function workerSlotClass(state: WorkerSlotState): string {
  if (state === 'active') return 'border-af-hairline-strong bg-af-sunken text-af-ink'
  if (state === 'idle') return 'border-af-hairline text-af-ink-2'
  return 'border-af-hairline text-af-ink-3'
}

function workerDotClass(state: WorkerSlotState): string {
  if (state === 'active') return 'bg-af-ink'
  if (state === 'idle') return 'bg-af-success'
  return 'bg-af-ink-4'
}

function percent(value: number): string {
  if (!Number.isFinite(value)) return '-'
  return `${(value * 100).toFixed(1)}%`
}

function percentWidth(value: number): string {
  if (!Number.isFinite(value)) return '0%'
  return `${Math.min(100, Math.max(0, value * 100)).toFixed(1)}%`
}

function latencyText(value: number | null): string {
  if (value === null || value === undefined) return '-'
  return `${value} ms`
}

function apiKeyRowKey(row: ContentModerationAPIKeyStatus, index: number): string {
  return `${row.configured ? 'saved' : 'test'}-${row.key_hash || index}`
}

/** 已保存的不标；刚测过还没保存的标「待保存」；点了删除的标「待删除」 */
function apiKeyRowTag(row: ContentModerationAPIKeyStatus): string {
  if (isStoredApiKeyPendingDelete(row)) return t('admin.riskControl.apiKeyPendingDelete')
  if (!row.configured) return t('admin.riskControl.apiKeyTemporary')
  return ''
}

function apiKeyStatusLabel(statusValue: ContentModerationAPIKeyStatus['status']): string {
  const labels: Record<ContentModerationAPIKeyStatus['status'], string> = {
    ok: t('admin.riskControl.apiKeyStatusOk'),
    error: t('admin.riskControl.apiKeyStatusError'),
    frozen: t('admin.riskControl.apiKeyStatusFrozen'),
    unknown: t('admin.riskControl.apiKeyStatusUnknown'),
  }
  return labels[statusValue] ?? labels.unknown
}

function apiKeyStatusDotClass(statusValue: ContentModerationAPIKeyStatus['status']): string {
  const classes: Record<ContentModerationAPIKeyStatus['status'], string> = {
    ok: 'bg-af-success',
    error: 'bg-af-warning',
    frozen: 'bg-af-danger',
    unknown: 'bg-af-ink-4',
  }
  return classes[statusValue] ?? classes.unknown
}

function apiKeyStatusMeta(row: ContentModerationAPIKeyStatus): string {
  const parts: string[] = []
  parts.push(t('admin.riskControl.apiKeyFailureCount', { count: row.failure_count || 0 }))
  if (row.last_latency_ms > 0) {
    parts.push(t('admin.riskControl.apiKeyLatency', { ms: row.last_latency_ms }))
  }
  if (row.frozen_until) {
    parts.push(t('admin.riskControl.apiKeyFrozenUntil', { time: formatDateTime(row.frozen_until) }))
  } else if (row.last_checked_at) {
    parts.push(t('admin.riskControl.apiKeyLastChecked', { time: formatDateTime(row.last_checked_at) }))
  } else {
    parts.push(t('admin.riskControl.apiKeyNotTested'))
  }
  return parts.join(' · ')
}

function parseApiKeys(value: string): string[] {
  return value
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter((item, index, arr) => item && arr.indexOf(item) === index)
}

function parseBlockedKeywords(value: string): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const line of value.split(/\r?\n/)) {
    const kw = line.trim()
    if (!kw) continue
    const key = kw.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    out.push(kw)
  }
  return out
}

function violationCountText(row: ContentModerationLog): string {
  if (!row.flagged) return '-'
  if (row.violation_count === 0) return t('admin.riskControl.violationNotCounted')
  return t('admin.riskControl.violationCount', { count: row.violation_count || 1 })
}

function normalizeDateTimeLocal(value: string): string | undefined {
  if (!value) return undefined
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return undefined
  return date.toISOString()
}

function formatDateTime(value: string): string {
  return formatDateTimeValue(value) || '-'
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat().format(value)
}

onMounted(() => {
  void loadAll()
  statusTimer = window.setInterval(() => {
    void loadStatus(true)
  }, 15000)
})

onUnmounted(() => {
  if (statusTimer !== null) {
    window.clearInterval(statusTimer)
    statusTimer = null
  }
})
</script>
