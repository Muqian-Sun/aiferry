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

        <div
          v-if="showPreBlockRuntimeCard"
          data-test="pre-block-runtime-cards"
          class="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,520px)_minmax(0,1fr)]"
        >
          <div data-test="pre-block-sync-card" class="card">
            <div class="flex flex-col gap-4 border-b border-af-hairline px-6 py-4 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <h2 class="text-lg font-semibold text-af-ink">{{ t('admin.riskControl.preBlockSyncStatus') }}</h2>
                <p class="mt-1 text-sm text-af-ink-3">{{ t('admin.riskControl.preBlockSyncHint') }}</p>
              </div>
              <span class="inline-flex w-fit items-center rounded-full bg-af-sunken px-2.5 py-1 text-xs font-medium text-af-ink-2">
                {{ modeLabel(status?.mode ?? configForm.mode) }}
              </span>
            </div>

            <div class="p-6">
              <div data-test="pre-block-metric-grid" class="grid grid-cols-2 gap-3 md:grid-cols-3">
                <div
                  v-for="item in preBlockMetricItems"
                  :key="item.key"
                  class="rounded-lg p-4"
                  :class="item.class"
                >
                  <p class="text-xs text-af-ink-3">{{ item.label }}</p>
                  <p class="mt-2 truncate text-2xl font-semibold leading-8" :class="item.valueClass">{{ item.value }}</p>
                  <p v-if="item.meta" class="mt-1 truncate text-xs text-af-ink-3">{{ item.meta }}</p>
                </div>
              </div>
            </div>
          </div>

          <div data-test="pre-block-api-key-load-card" class="card">
            <div class="flex flex-col gap-4 border-b border-af-hairline px-6 py-4 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <h2 class="text-lg font-semibold text-af-ink">{{ t('admin.riskControl.preBlockAPIKeyLoad') }}</h2>
                <p class="mt-1 text-sm text-af-ink-3">
                  {{ t('admin.riskControl.preBlockAPIKeyLoadHint') }}
                </p>
              </div>
              <span class="inline-flex w-fit items-center rounded-full bg-af-sunken px-2.5 py-1 text-xs font-medium text-af-ink-2">
                {{ preBlockAPIKeyLoadSummaryText }}
              </span>
            </div>

            <div class="p-6">
              <div
                v-if="preBlockAPIKeyLoads.length > 0"
                data-test="pre-block-api-key-load-list"
                class="max-h-[280px] space-y-3 overflow-y-auto pr-1"
              >
                <div
                  v-for="item in preBlockAPIKeyLoads"
                  :key="item.key_hash || item.index"
                  class="rounded-lg bg-af-sunken p-3"
                >
                  <div class="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                    <div class="min-w-0">
                      <div class="flex min-w-0 items-center gap-2">
                        <span class="font-mono text-sm font-semibold text-af-ink">#{{ item.index + 1 }}</span>
                        <span class="truncate font-mono text-sm text-af-ink-2">{{ item.masked || '-' }}</span>
                        <span class="h-2 w-2 flex-shrink-0 rounded-full" :class="apiKeyStatusDotClass(item.status)"></span>
                      </div>
                      <p class="mt-1 text-xs text-af-ink-3">
                        {{ t('admin.riskControl.preBlockAPIKeyTotals', { total: formatNumber(item.total), success: formatNumber(item.success), errors: formatNumber(item.errors) }) }}
                      </p>
                    </div>
                    <div class="grid grid-cols-4 gap-2 text-right text-xs text-af-ink-3 sm:min-w-[280px]">
                      <div>
                        <p>{{ t('admin.riskControl.preBlockKeyActiveShort') }}</p>
                        <p class="mt-1 text-sm font-semibold text-af-ink-2">{{ formatNumber(item.active) }}</p>
                      </div>
                      <div>
                        <p>{{ t('admin.riskControl.preBlockKeyTotalShort') }}</p>
                        <p class="mt-1 text-sm font-semibold text-af-ink">{{ formatNumber(item.total) }}</p>
                      </div>
                      <div>
                        <p>{{ t('admin.riskControl.preBlockKeyAvgShort') }}</p>
                        <p class="mt-1 text-sm font-semibold text-af-ink">{{ formatNumber(item.avg_latency_ms) }} ms</p>
                      </div>
                      <div>
                        <p>{{ t('admin.riskControl.preBlockKeyLastShort') }}</p>
                        <p class="mt-1 text-sm font-semibold text-af-ink">{{ formatNumber(item.last_latency_ms) }} ms</p>
                      </div>
                    </div>
                  </div>
                  <div class="mt-3 h-1.5 overflow-hidden rounded-full bg-af-sheet">
                    <div class="h-full rounded-full bg-af-ink" :style="{ width: preBlockAPIKeyLoadWidth(item.total) }"></div>
                  </div>
                </div>
              </div>
              <p v-else class="rounded-lg bg-af-sunken p-4 text-sm text-af-ink-3">
                {{ t('admin.riskControl.preBlockAPIKeyLoadEmpty') }}
              </p>
            </div>
          </div>
        </div>

        <div v-if="showWorkerRuntimeCard" class="card">
          <div class="flex flex-col gap-4 border-b border-af-hairline px-6 py-4 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <h2 class="text-lg font-semibold text-af-ink">{{ t('admin.riskControl.workerStatus') }}</h2>
              <p class="mt-1 text-sm text-af-ink-3">{{ t('admin.riskControl.workerStatusHint') }}</p>
            </div>
            <div class="flex flex-wrap items-center gap-2 text-sm text-af-ink-3">
              <span>{{ t('admin.riskControl.autoRefresh') }}</span>
              <span v-if="status?.last_cleanup_at">
                {{ t('admin.riskControl.lastCleanup', { time: formatDateTime(status.last_cleanup_at) }) }}
              </span>
            </div>
          </div>

          <div class="grid grid-cols-1 gap-6 p-6 xl:grid-cols-[minmax(0,360px)_1fr]">
            <div class="space-y-4">
              <div class="rounded-lg border border-af-hairline p-4">
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <p class="text-sm font-medium text-af-ink">{{ t('admin.riskControl.queueUsage') }}</p>
                    <p class="mt-1 text-xs text-af-ink-3">
                      {{ formatNumber(status?.queue_length ?? 0) }} / {{ formatNumber(status?.queue_size ?? configForm.queue_size) }}
                    </p>
                  </div>
                  <span class="text-sm font-semibold text-af-ink">{{ queueUsagePercent }}</span>
                </div>
                <div class="mt-4 h-2 overflow-hidden rounded-full bg-af-sunken">
                  <div class="h-full rounded-full bg-af-brand transition-all duration-300" :style="queueUsageStyle"></div>
                </div>
              </div>

              <div class="grid grid-cols-2 gap-3">
                <div class="rounded-lg bg-af-sunken p-4">
                  <p class="text-xs text-af-ink-3">{{ t('admin.riskControl.activeWorkers') }}</p>
                  <p class="mt-2 text-2xl font-semibold text-af-ink">{{ status?.active_workers ?? 0 }}</p>
                </div>
                <div class="rounded-lg bg-af-success-tint p-4">
                  <p class="text-xs text-af-ink-3">{{ t('admin.riskControl.idleWorkers') }}</p>
                  <p class="mt-2 text-2xl font-semibold text-af-success">{{ status?.idle_workers ?? configForm.worker_count }}</p>
                </div>
                <div class="rounded-lg bg-af-sunken p-4">
                  <p class="text-xs text-af-ink-3">{{ t('admin.riskControl.processed') }}</p>
                  <p class="mt-2 text-2xl font-semibold text-af-ink">{{ formatNumber(status?.processed ?? 0) }}</p>
                </div>
                <div class="rounded-lg bg-af-sunken p-4">
                  <p class="text-xs text-af-ink-3">{{ t('admin.riskControl.droppedErrors') }}</p>
                  <p class="mt-2 text-2xl font-semibold text-af-ink">{{ formatNumber((status?.dropped ?? 0) + (status?.errors ?? 0)) }}</p>
                </div>
              </div>
            </div>

            <div>
              <div class="mb-3 flex items-center justify-between gap-3">
                <div>
                  <p class="text-sm font-medium text-af-ink">{{ t('admin.riskControl.workerPool') }}</p>
                  <p class="mt-1 text-xs text-af-ink-3">
                    {{ t('admin.riskControl.workerPoolMeta', { active: status?.active_workers ?? 0, idle: status?.idle_workers ?? configForm.worker_count, total: status?.worker_count ?? configForm.worker_count }) }}
                  </p>
                </div>
                <span class="inline-flex items-center rounded-full bg-af-sunken px-2.5 py-1 text-xs font-medium text-af-ink-2">
                  {{ modeLabel(status?.mode ?? configForm.mode) }}
                </span>
              </div>
              <div class="grid grid-cols-2 gap-2 sm:grid-cols-4 md:grid-cols-6 xl:grid-cols-8 2xl:grid-cols-10">
                <div
                  v-for="worker in workerSlots"
                  :key="worker.id"
                  class="flex h-12 items-center justify-between rounded-lg border px-3 transition-colors"
                  :class="workerSlotClass(worker.state)"
                  :title="worker.label"
                >
                  <span class="text-sm font-semibold">#{{ worker.id }}</span>
                  <span class="h-2.5 w-2.5 rounded-full" :class="workerDotClass(worker.state)"></span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="card">
          <div class="flex flex-col gap-4 border-b border-af-hairline px-6 py-4">
            <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <h2 class="text-lg font-semibold text-af-ink">{{ t('admin.riskControl.records') }}</h2>
                <p class="mt-1 text-sm text-af-ink-3">{{ t('admin.riskControl.recordsHint') }}</p>
              </div>
              <button type="button" class="btn btn-secondary inline-flex items-center gap-2" :disabled="logsLoading" @click="loadLogs">
                <Icon name="refresh" size="sm" :class="logsLoading ? 'animate-spin' : ''" />
                {{ t('admin.riskControl.refresh') }}
              </button>
            </div>

            <div class="flex flex-col gap-2 rounded-lg border border-af-hairline bg-af-sunken px-3 py-2 sm:flex-row sm:items-center sm:justify-between">
              <div class="flex min-w-0 items-center gap-2 text-sm text-af-ink-2">
                <Icon name="filter" size="sm" class="flex-shrink-0 text-af-ink-3" />
                <span class="font-medium">{{ t('admin.riskControl.modelFilter') }}</span>
                <span class="truncate text-af-ink-3">{{ modelFilterSummary }}</span>
              </div>
              <div v-if="modelFilterPreviewModels.length > 0" class="flex flex-wrap gap-1.5">
                <span
                  v-for="model in modelFilterPreviewModels"
                  :key="model"
                  class="inline-flex max-w-[180px] items-center truncate rounded-md bg-af-sheet px-2 py-1 font-mono text-xs text-af-ink-2"
                >
                  {{ model }}
                </span>
                <span v-if="hiddenModelFilterModelCount > 0" class="inline-flex rounded-md bg-af-sheet px-2 py-1 text-xs text-af-ink-3">
                  +{{ hiddenModelFilterModelCount }}
                </span>
              </div>
            </div>

            <div class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-5">
              <Select v-model="filters.result" :options="resultOptions" @change="reloadLogsFromFirstPage" />
              <Select v-model="filters.endpoint" :options="endpointOptions" @change="reloadLogsFromFirstPage" />
              <input v-model.trim="filters.search" type="search" class="input" :placeholder="t('admin.riskControl.filters.search')" @keyup.enter="reloadLogsFromFirstPage" />
              <input v-model="filters.from" type="datetime-local" class="input" :title="t('admin.riskControl.filters.from')" @change="reloadLogsFromFirstPage" />
              <input v-model="filters.to" type="datetime-local" class="input" :title="t('admin.riskControl.filters.to')" @change="reloadLogsFromFirstPage" />
            </div>
          </div>

          <div class="overflow-x-auto">
            <table class="min-w-full divide-y divide-af-hairline">
              <thead class="bg-af-sunken">
                <tr>
                  <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-af-ink-3">{{ t('admin.riskControl.table.time') }}</th>
                  <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-af-ink-3">{{ t('admin.riskControl.table.user') }}</th>
                  <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-af-ink-3">{{ t('admin.riskControl.table.apiKey') }}</th>
                  <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-af-ink-3">{{ t('admin.riskControl.table.endpoint') }}</th>
                  <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-af-ink-3">{{ t('admin.riskControl.table.result') }}</th>
                  <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-af-ink-3">{{ t('admin.riskControl.table.highest') }}</th>
                  <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-af-ink-3">{{ t('admin.riskControl.table.actionMeta') }}</th>
                  <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-af-ink-3">{{ t('admin.riskControl.table.latency') }}</th>
                  <th class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-af-ink-3">{{ t('admin.riskControl.table.input') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-af-hairline bg-af-sheet">
                <tr v-if="logsLoading">
                  <td colspan="10" class="px-5 py-12 text-center text-sm text-af-ink-3">{{ t('common.loading') }}</td>
                </tr>
                <tr v-else-if="logs.length === 0">
                  <td colspan="10" class="px-5 py-12 text-center text-sm text-af-ink-3">{{ t('admin.riskControl.emptyLogs') }}</td>
                </tr>
                <template v-else>
                  <tr v-for="row in logs" :key="row.id" class="hover:bg-af-sunken">
                    <td class="whitespace-nowrap px-5 py-4 text-sm text-af-ink-2">{{ formatDateTime(row.created_at) }}</td>
                    <td class="whitespace-nowrap px-5 py-4 text-sm text-af-ink-2">
                      <!-- 不露内部 id：有 id 却没名字就是已删除，没有 id 写 - -->
                      <div>{{ row.user_email || (row.user_id ? t('common.deletedUser') : '-') }}</div>
                    </td>
                    <td class="whitespace-nowrap px-5 py-4 text-sm text-af-ink-2">{{ row.api_key_name || (row.api_key_id ? t('common.deletedKey') : '-') }}</td>
                    <td class="whitespace-nowrap px-5 py-4 text-sm text-af-ink-2">
                      <div>{{ row.endpoint || '-' }}</div>
                      <div class="text-xs text-af-ink-3">{{ row.provider || '-' }} / {{ row.model || '-' }}</div>
                    </td>
                    <td class="whitespace-nowrap px-5 py-4">
                      <span class="inline-flex rounded-md px-2 py-1 text-xs font-medium" :class="resultBadgeClass(row)">
                        {{ resultLabel(row) }}
                      </span>
                    </td>
                    <td class="whitespace-nowrap px-5 py-4 text-sm text-af-ink-2">
                      <div>{{ row.highest_category || '-' }}</div>
                      <div class="text-xs text-af-ink-3">{{ percent(row.highest_score) }}</div>
                      <div v-if="row.matched_keyword" class="mt-0.5 text-xs font-medium text-af-danger" :title="t('admin.riskControl.matchedKeyword') + ': ' + row.matched_keyword">
                        {{ t('admin.riskControl.matchedKeyword') }}: {{ row.matched_keyword }}
                      </div>
                    </td>
                    <td class="whitespace-nowrap px-5 py-4 text-sm text-af-ink-2">
                      <div>{{ violationCountText(row) }}</div>
                      <div class="text-xs text-af-ink-3">
                        {{ row.email_sent ? t('admin.riskControl.emailSent') : t('admin.riskControl.emailNotSent') }}
                        <span v-if="row.auto_banned"> / {{ t('admin.riskControl.autoBanned') }}</span>
                      </div>
                      <button
                        v-if="canUnbanRow(row)"
                        type="button"
                        class="mt-2 inline-flex items-center gap-1 rounded-md border border-af-success/30 bg-af-success-tint px-2 py-1 text-xs font-medium text-af-success transition-colors hover:bg-af-success-tint disabled:cursor-not-allowed disabled:opacity-60"
                        :disabled="unbanningUserID === row.user_id"
                        @click="unbanUser(row)"
                      >
                        <Icon name="checkCircle" size="xs" :class="unbanningUserID === row.user_id ? 'animate-spin' : ''" />
                        {{ unbanningUserID === row.user_id ? t('common.processing') : t('admin.riskControl.unbanUser') }}
                      </button>
                    </td>
                    <td class="whitespace-nowrap px-5 py-4 text-sm text-af-ink-2">
                      <div>{{ latencyText(row.upstream_latency_ms) }}</div>
                      <div v-if="row.queue_delay_ms !== null && row.queue_delay_ms !== undefined" class="text-xs text-af-ink-3">
                        {{ t('admin.riskControl.queueDelay', { ms: row.queue_delay_ms }) }}
                      </div>
                    </td>
                    <td class="w-[320px] max-w-sm px-5 py-4 text-sm text-af-ink-2">
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
        </div>
      </template>

      <BaseDialog :show="settingsOpen" :title="t('admin.riskControl.settingsTitle')" width="extra-wide" @close="settingsOpen = false">
        <div class="space-y-6">
          <div class="flex gap-2 overflow-x-auto border-b border-af-hairline pb-3">
            <button
              v-for="tab in settingsTabs"
              :key="tab.id"
              type="button"
              class="inline-flex whitespace-nowrap rounded-lg px-3 py-2 text-sm font-medium transition-colors"
              :class="activeSettingsTab === tab.id ? 'bg-af-brand-tint text-af-brand' : 'text-af-ink-3 hover:bg-af-sunken hover:text-af-ink'"
              @click="activeSettingsTab = tab.id"
            >
              {{ tab.label }}
            </button>
          </div>

          <div v-if="activeSettingsTab === 'basic'" class="space-y-5">
            <div class="grid grid-cols-1 gap-5 lg:grid-cols-2">
              <div class="flex items-center justify-between rounded-lg border border-af-hairline p-4">
                <div>
                  <p class="text-sm font-medium text-af-ink">{{ t('admin.riskControl.enabled') }}</p>
                  <p class="mt-1 text-xs text-af-ink-3">{{ t('admin.riskControl.enabledHint') }}</p>
                </div>
                <Toggle v-model="configForm.enabled" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.riskControl.mode') }}</label>
                <Select v-model="configForm.mode" :options="modeOptions" />
                <p class="mt-2 text-xs leading-5 text-af-ink-3">{{ modeDescription(configForm.mode) }}</p>
              </div>
              <div>
                <label class="input-label">{{ t('admin.riskControl.baseUrl') }}</label>
                <input v-model.trim="configForm.base_url" type="url" class="input" placeholder="https://api.openai.com" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.riskControl.model') }}</label>
                <input v-model.trim="configForm.model" type="text" class="input" placeholder="omni-moderation-latest" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.riskControl.timeoutMs') }}</label>
                <input v-model.number="configForm.timeout_ms" type="number" min="500" max="30000" class="input" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.riskControl.retryCount') }}</label>
                <input v-model.number="configForm.retry_count" type="number" min="0" max="5" class="input" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.riskControl.sampleRate') }}</label>
                <div class="relative">
                  <input v-model.number="configForm.sample_rate" type="number" min="0" max="100" step="1" class="input pr-8" />
                  <span class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-af-ink-3">%</span>
                </div>
              </div>
              <div>
                <label class="input-label">{{ t('admin.riskControl.proxy') }}</label>
                <ProxySelector v-model="configForm.proxy_id" :proxies="proxies" />
                <p class="mt-2 text-xs leading-5 text-af-ink-3">{{ t('admin.riskControl.proxyHint') }}</p>
              </div>
            </div>

            <div class="overflow-hidden rounded-xl border border-af-hairline bg-af-sheet">
              <div class="flex flex-col gap-4 border-b border-af-hairline bg-af-sunken px-4 py-4 lg:flex-row lg:items-center lg:justify-between">
                <div class="flex items-start gap-3">
                  <span class="mt-0.5 flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg bg-af-brand-tint text-af-brand">
                    <Icon name="key" size="md" />
                  </span>
                  <div>
                    <label class="text-sm font-semibold text-af-ink">{{ t('admin.riskControl.apiKeys') }}</label>
                    <p class="mt-1 max-w-3xl text-xs leading-5 text-af-ink-3">
                      {{ t('admin.riskControl.apiKeysHint', { count: configForm.api_key_count }) }}
                    </p>
                  </div>
                </div>
                <div class="flex flex-wrap items-center gap-2">
                  <button
                    type="button"
                    class="btn btn-secondary inline-flex items-center gap-2"
                    :disabled="apiKeyTesting || inputApiKeyCount === 0 || configForm.clear_api_key"
                    @click="testApiKeys(true)"
                  >
                    <Icon name="beaker" size="sm" :class="apiKeyTesting ? 'animate-pulse' : ''" />
                    {{ apiKeyTesting ? t('admin.riskControl.testingApiKeys') : t('admin.riskControl.testInputApiKeys') }}
                  </button>
                  <button
                    type="button"
                    class="btn btn-secondary inline-flex items-center gap-2"
                    :disabled="apiKeyTesting || effectiveStoredApiKeyCount === 0 || pendingDeletedApiKeyCount > 0 || configForm.clear_api_key || configForm.api_keys_mode === 'replace'"
                    @click="testApiKeys(false)"
                  >
                    <Icon name="shield" size="sm" />
                    {{ storedApiKeyTestButtonText }}
                  </button>
                  <button
                    v-if="configForm.api_key_configured"
                    type="button"
                    class="btn btn-secondary inline-flex items-center gap-2"
                    @click="toggleClearApiKey"
                  >
                    <Icon :name="configForm.clear_api_key ? 'x' : 'trash'" size="sm" />
                    {{ configForm.clear_api_key ? t('admin.riskControl.keepApiKey') : t('admin.riskControl.clearApiKey') }}
                  </button>
                </div>
              </div>

              <div class="grid grid-cols-1 gap-4 p-4 xl:grid-cols-[minmax(0,1fr)_minmax(360px,440px)]">
                <div class="space-y-3">
                  <div class="flex flex-col gap-2 rounded-lg border border-af-hairline bg-af-sunken p-2 sm:flex-row sm:items-center sm:justify-between">
                    <div class="text-xs leading-5 text-af-ink-3">
                      <span class="font-medium text-af-ink-2">{{ t('admin.riskControl.apiKeysWriteMode') }}</span>
                      <span class="ml-2">{{ apiKeysModeHint }}</span>
                    </div>
                    <div class="inline-flex rounded-lg bg-af-sheet p-1">
                      <button
                        type="button"
                        class="rounded-md px-3 py-1.5 text-xs font-medium transition-colors"
                        :class="configForm.api_keys_mode === 'append' ? 'bg-af-brand text-af-on-brand' : 'text-af-ink-2 hover:bg-af-sunken'"
                        :disabled="configForm.clear_api_key"
                        @click="setAPIKeysMode('append')"
                      >
                        {{ t('admin.riskControl.apiKeysModeAppend') }}
                      </button>
                      <button
                        type="button"
                        class="rounded-md px-3 py-1.5 text-xs font-medium transition-colors"
                        :class="configForm.api_keys_mode === 'replace' ? 'bg-af-warning text-af-on-brand' : 'text-af-ink-2 hover:bg-af-sunken'"
                        :disabled="configForm.clear_api_key"
                        @click="setAPIKeysMode('replace')"
                      >
                        {{ t('admin.riskControl.apiKeysModeReplace') }}
                      </button>
                    </div>
                  </div>
                  <textarea
                    v-model="configForm.api_keys_text"
                    class="input min-h-44 resize-y font-mono text-sm"
                    :placeholder="apiKeysPlaceholder"
                    autocomplete="new-password"
                    :disabled="configForm.clear_api_key"
                  ></textarea>
                  <div class="flex flex-wrap items-center gap-2 text-xs text-af-ink-3">
                    <span class="inline-flex rounded-md bg-af-sunken px-2 py-1">
                      {{ t('admin.riskControl.inputApiKeyCount', { count: inputApiKeyCount }) }}
                    </span>
                    <span v-if="configForm.api_key_configured" class="inline-flex rounded-md bg-af-sunken px-2 py-1">
                      {{ t('admin.riskControl.storedApiKeyCount', { count: configForm.api_key_count }) }}
                    </span>
                    <span v-if="configForm.clear_api_key" class="inline-flex rounded-md bg-af-danger-tint px-2 py-1 text-af-danger">
                      {{ t('admin.riskControl.apiKeyWillClear') }}
                    </span>
                    <span v-else-if="pendingDeletedApiKeyCount > 0" class="inline-flex rounded-md bg-af-warning-tint px-2 py-1 text-af-warning">
                      {{ t('admin.riskControl.apiKeyPendingDeleteCount', { count: pendingDeletedApiKeyCount }) }}
                    </span>
                    <span v-if="configForm.api_keys_mode === 'replace'" class="inline-flex rounded-md bg-af-warning-tint px-2 py-1 text-af-warning">
                      {{ t('admin.riskControl.apiKeysReplaceWarning') }}
                    </span>
                  </div>

                  <div class="rounded-lg border border-af-hairline bg-af-sunken p-3" @paste="handleModerationImagePaste">
                    <div class="mb-3 flex items-center justify-between gap-3">
                      <div>
                        <p class="text-sm font-semibold text-af-ink">{{ t('admin.riskControl.auditTestInput') }}</p>
                        <p class="mt-1 text-xs text-af-ink-3">{{ t('admin.riskControl.auditTestInputHint') }}</p>
                      </div>
                      <button
                        v-if="moderationTestPrompt || moderationTestImages.length > 0 || moderationTestResult"
                        type="button"
                        class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-af-ink-3 hover:bg-af-sheet hover:text-af-ink"
                        @click="clearModerationTestInput"
                      >
                        <Icon name="x" size="xs" />
                        {{ t('admin.riskControl.clearAuditTest') }}
                      </button>
                    </div>
                    <textarea
                      v-model="moderationTestPrompt"
                      class="input min-h-24 resize-y text-sm"
                      :placeholder="t('admin.riskControl.auditTestPromptPlaceholder')"
                    ></textarea>
                    <div
                      class="mt-3 rounded-lg border border-dashed border-af-hairline bg-af-sheet p-3"
                      @dragover.prevent
                      @drop.prevent="handleModerationImageDrop"
                    >
                      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                        <div class="flex items-start gap-2">
                          <Icon name="upload" size="md" class="mt-0.5 text-af-ink-3" />
                          <div>
                            <p class="text-sm font-medium text-af-ink">{{ t('admin.riskControl.auditTestImages') }}</p>
                            <p class="mt-1 text-xs text-af-ink-3">{{ t('admin.riskControl.auditTestImagesHint') }}</p>
                          </div>
                        </div>
                        <label class="btn btn-secondary inline-flex cursor-pointer items-center gap-2">
                          <Icon name="plus" size="sm" />
                          {{ t('admin.riskControl.addAuditTestImage') }}
                          <input type="file" accept="image/*" multiple class="sr-only" @change="handleModerationImageUpload" />
                        </label>
                      </div>
                      <div v-if="moderationTestImages.length > 0" class="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-4">
                        <div
                          v-for="(image, index) in moderationTestImages"
                          :key="image.slice(0, 64) + index"
                          class="group relative aspect-square overflow-hidden rounded-lg border border-af-hairline bg-af-sunken"
                        >
                          <img :src="image" alt="" class="h-full w-full object-cover" />
                          <button
                            type="button"
                            class="absolute right-1.5 top-1.5 flex h-7 w-7 items-center justify-center rounded-full bg-black/60 text-af-on-brand opacity-0 transition-opacity group-hover:opacity-100"
                            @click="removeModerationTestImage(index)"
                          >
                            <Icon name="x" size="xs" :stroke-width="2" />
                          </button>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <div class="rounded-lg border border-af-hairline bg-af-sunken p-3">
                  <div class="mb-3 flex items-start justify-between gap-3">
                    <div class="min-w-0">
                      <p class="text-sm font-semibold text-af-ink">{{ t('admin.riskControl.apiKeyHealth') }}</p>
                      <p class="mt-1 text-xs text-af-ink-3">{{ t('admin.riskControl.apiKeyFreezeRule') }}</p>
                    </div>
                    <span class="inline-flex shrink-0 items-center whitespace-nowrap rounded-full bg-af-sheet px-2 py-0.5 text-xs font-medium leading-5 text-af-ink-2">
                      {{ t('admin.riskControl.apiKeyRows', { count: apiKeyRows.length }) }}
                    </span>
                  </div>

                  <div v-if="apiKeyRows.length === 0" class="flex min-h-32 flex-col items-center justify-center rounded-lg border border-dashed border-af-hairline bg-af-sheet px-4 py-6 text-center">
                    <Icon name="infoCircle" size="lg" class="text-af-ink-3" />
                    <p class="mt-2 text-sm font-medium text-af-ink-2">{{ t('admin.riskControl.apiKeyHealthEmpty') }}</p>
                    <p class="mt-1 text-xs text-af-ink-3">{{ t('admin.riskControl.apiKeyHealthEmptyHint') }}</p>
                  </div>
                  <div v-else class="space-y-2">
                    <div class="space-y-2" :class="apiKeyRowsExpanded ? 'max-h-72 overflow-y-auto pr-1' : ''">
                      <div
                        v-for="(row, index) in visibleApiKeyRows"
                        :key="apiKeyRowKey(row, index)"
                        class="rounded-lg border bg-af-sheet p-2.5"
                        :class="isStoredApiKeyPendingDelete(row) ? 'border-af-warning/30 opacity-70' : 'border-af-hairline'"
                      >
                        <div class="flex items-start justify-between gap-2">
                          <div class="min-w-0">
                            <div class="flex min-w-0 flex-wrap items-center gap-2">
                              <span class="truncate font-mono text-sm font-semibold text-af-ink">{{ row.masked || '-' }}</span>
                              <span
                                class="inline-flex rounded-md px-1.5 py-0.5 text-xs font-medium"
                                :class="row.configured ? 'bg-af-brand-tint text-af-brand' : 'bg-af-sunken text-af-ink-2'"
                              >
                                {{ isStoredApiKeyPendingDelete(row) ? t('admin.riskControl.apiKeyPendingDelete') : row.configured ? t('admin.riskControl.apiKeyConfigured') : t('admin.riskControl.apiKeyTemporary') }}
                              </span>
                            </div>
                            <p class="mt-1 text-xs leading-5 text-af-ink-3">{{ apiKeyStatusMeta(row) }}</p>
                          </div>
                          <div class="flex flex-shrink-0 items-center gap-1.5">
                            <span class="inline-flex items-center gap-1.5 whitespace-nowrap rounded-full px-2 py-0.5 text-xs font-medium" :class="apiKeyStatusBadgeClass(row.status)">
                              <span class="h-1.5 w-1.5 rounded-full" :class="apiKeyStatusDotClass(row.status)"></span>
                              {{ apiKeyStatusLabel(row.status) }}
                            </span>
                            <button
                              v-if="row.configured && !configForm.clear_api_key"
                              type="button"
                              class="inline-flex h-7 w-7 items-center justify-center rounded-md text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink-2"
                              :title="isStoredApiKeyPendingDelete(row) ? t('admin.riskControl.undoDeleteApiKey') : t('admin.riskControl.deleteApiKey')"
                              @click="toggleDeleteStoredApiKey(row)"
                            >
                              <Icon :name="isStoredApiKeyPendingDelete(row) ? 'refresh' : 'trash'" size="xs" />
                            </button>
                          </div>
                        </div>
                        <p v-if="row.last_error" class="mt-1.5 rounded-md bg-af-warning-tint px-2 py-1.5 text-xs leading-5 text-af-warning">
                          {{ row.last_error }}
                        </p>
                      </div>
                    </div>

                    <div v-if="canToggleApiKeyRows" class="flex items-center justify-between gap-3 rounded-lg border border-dashed border-af-hairline bg-af-sheet px-3 py-2 text-xs text-af-ink-3">
                      <span class="min-w-0 truncate">
                        {{ apiKeyRowsExpanded ? t('admin.riskControl.apiKeyRowsExpanded', { count: apiKeyRows.length }) : t('admin.riskControl.apiKeyRowsCollapsed', { count: hiddenApiKeyRowCount }) }}
                      </span>
                      <button
                        type="button"
                        class="inline-flex shrink-0 items-center gap-1 rounded-md px-2 py-1 font-medium text-af-brand transition-colors hover:bg-af-brand-tint hover:text-af-brand-hover"
                        @click="apiKeyRowsExpanded = !apiKeyRowsExpanded"
                      >
                        <Icon :name="apiKeyRowsExpanded ? 'chevronUp' : 'chevronDown'" size="xs" />
                        {{ apiKeyRowsExpanded ? t('admin.riskControl.collapseApiKeyRows') : t('admin.riskControl.expandApiKeyRows') }}
                      </button>
                    </div>
                  </div>

                  <div v-if="moderationTestResult" class="mt-4 rounded-lg border border-af-hairline bg-af-sheet p-3">
                    <div class="flex items-start justify-between gap-3">
                      <div>
                        <p class="text-sm font-semibold text-af-ink">{{ t('admin.riskControl.auditTestResult') }}</p>
                        <p class="mt-1 text-xs text-af-ink-3">
                          {{ t('admin.riskControl.auditTestHighest', { category: moderationTestResult.highest_category || '-', score: percent(moderationTestResult.highest_score) }) }}
                        </p>
                      </div>
                      <span class="inline-flex rounded-full px-2 py-1 text-xs font-medium" :class="moderationTestResult.flagged ? 'bg-af-danger-tint text-af-danger' : 'bg-af-success-tint text-af-success'">
                        {{ moderationTestResult.flagged ? t('admin.riskControl.auditTestFlagged') : t('admin.riskControl.auditTestPassed') }}
                      </span>
                    </div>
                    <div class="mt-3">
                      <div class="mb-2 flex items-center justify-between text-xs text-af-ink-3">
                        <span>{{ t('admin.riskControl.auditTestComposite') }}</span>
                        <span class="font-semibold text-af-ink">{{ percent(moderationTestResult.composite_score) }}</span>
                      </div>
                      <div class="h-2 overflow-hidden rounded-full bg-af-sunken">
                        <div class="h-full rounded-full" :class="moderationTestResult.flagged ? 'bg-af-danger' : 'bg-af-success'" :style="{ width: percentWidth(moderationTestResult.composite_score) }"></div>
                      </div>
                    </div>
                    <div class="mt-3 max-h-52 space-y-2 overflow-y-auto pr-1">
                      <div v-for="score in moderationScoreRows" :key="score.category">
                        <div class="mb-1 flex items-center justify-between gap-3 text-xs">
                          <span class="truncate text-af-ink-2">{{ score.category }}</span>
                          <span class="font-mono text-af-ink-3">{{ percent(score.score) }} / {{ percent(score.threshold) }}</span>
                        </div>
                        <div class="h-1.5 overflow-hidden rounded-full bg-af-sunken">
                          <div class="h-full rounded-full" :class="score.hit ? 'bg-af-danger' : 'bg-af-brand'" :style="{ width: percentWidth(score.score) }"></div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div v-else-if="activeSettingsTab === 'scope'" class="space-y-5">
            <div class="space-y-4 rounded-lg border border-af-hairline p-4">
              <div class="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
                <div>
                  <h3 class="text-base font-semibold text-af-ink">{{ t('admin.riskControl.modelFilter') }}</h3>
                  <p class="mt-1 text-sm text-af-ink-3">{{ t('admin.riskControl.modelFilterHint') }}</p>
                </div>
                <span class="inline-flex w-fit rounded-md bg-af-sunken px-2.5 py-1 text-xs font-medium text-af-ink-2">
                  {{ modelFilterSummary }}
                </span>
              </div>

              <div class="grid grid-cols-1 gap-2 md:grid-cols-3">
                <button
                  v-for="option in modelFilterOptions"
                  :key="option.value"
                  type="button"
                  class="rounded-lg border p-3 text-left transition-colors"
                  :class="configForm.model_filter_type === option.value
                    ? 'border-af-hairline-strong bg-af-brand-tint text-af-brand'
                    : 'border-af-hairline hover:bg-af-sunken'"
                  @click="setModelFilterType(option.value)"
                >
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-sm font-semibold">{{ option.label }}</span>
                    <span
                      class="flex h-4 w-4 flex-shrink-0 items-center justify-center rounded-full border"
                      :class="configForm.model_filter_type === option.value
                        ? 'border-af-brand bg-af-brand text-af-on-brand'
                        : 'border-af-hairline-strong text-transparent'"
                    >
                      <Icon name="check" size="xs" :stroke-width="2" />
                    </span>
                  </div>
                  <p class="mt-1 text-xs leading-5 text-af-ink-3">{{ option.description }}</p>
                </button>
              </div>

              <div v-if="configForm.model_filter_type !== 'all'" class="space-y-2">
                <label class="input-label">{{ t('admin.riskControl.modelFilterModels') }}</label>
                <ModelWhitelistSelector v-model="configForm.model_filter_models" />
                <p class="text-xs text-af-ink-3">
                  {{ t('admin.riskControl.modelFilterModelCount', { count: modelFilterModelCount }) }}
                </p>
              </div>
            </div>
          </div>

          <div v-else-if="activeSettingsTab === 'runtime'" class="grid grid-cols-1 gap-5 lg:grid-cols-2">
            <div>
              <label class="input-label">{{ t('admin.riskControl.workerCount') }}</label>
              <input v-model.number="configForm.worker_count" type="number" min="1" max="32" class="input" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.riskControl.queueSize') }}</label>
              <input v-model.number="configForm.queue_size" type="number" min="100" max="100000" class="input" />
            </div>
            <div class="flex items-center justify-between rounded-lg border border-af-hairline p-4 lg:col-span-2">
              <div>
                <p class="text-sm font-medium text-af-ink">{{ t('admin.riskControl.recordNonHits') }}</p>
                <p class="mt-1 text-xs text-af-ink-3">{{ t('admin.riskControl.recordNonHitsHint') }}</p>
              </div>
              <Toggle v-model="configForm.record_non_hits" />
            </div>
            <div class="space-y-4 rounded-lg border border-af-hairline p-4 lg:col-span-2">
              <div class="flex items-center justify-between gap-4">
                <div>
                  <p class="text-sm font-medium text-af-ink">{{ t('admin.riskControl.preHashCheck') }}</p>
                  <p class="mt-1 text-xs text-af-ink-3">{{ t('admin.riskControl.preHashCheckHint') }}</p>
                </div>
                <Toggle v-model="configForm.pre_hash_check_enabled" />
              </div>
              <div class="rounded-lg bg-af-sunken p-3">
                <div class="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
                  <div>
                    <p class="text-sm font-medium text-af-ink">
                      {{ t('admin.riskControl.flaggedHashCount', { count: formatNumber(status?.flagged_hash_count ?? 0) }) }}
                    </p>
                    <p class="mt-1 text-xs text-af-ink-3">{{ t('admin.riskControl.flaggedHashHint') }}</p>
                  </div>
                  <button
                    type="button"
                    class="btn btn-secondary inline-flex items-center justify-center gap-2 text-af-danger hover:text-af-danger"
                    :disabled="hashActionLoading || (status?.flagged_hash_count ?? 0) === 0"
                    @click="clearHashesConfirm = true"
                  >
                    <Icon name="trash" size="sm" :class="hashActionLoading ? 'animate-pulse' : ''" />
                    {{ t('admin.riskControl.clearFlaggedHashes') }}
                  </button>
                </div>
                <div class="mt-3 flex flex-col gap-2 sm:flex-row">
                  <input
                    v-model.trim="flaggedHashInput"
                    type="text"
                    class="input font-mono text-sm"
                    :placeholder="t('admin.riskControl.flaggedHashPlaceholder')"
                  />
                  <button
                    type="button"
                    class="btn btn-secondary inline-flex items-center justify-center gap-2"
                    :disabled="hashActionLoading || !isFlaggedHashInputValid"
                    @click="deleteFlaggedHash"
                  >
                    <Icon name="trash" size="sm" />
                    {{ t('admin.riskControl.deleteFlaggedHash') }}
                  </button>
                </div>
              </div>
            </div>
          </div>

          <div v-else-if="activeSettingsTab === 'response'" class="space-y-5">
            <div class="grid grid-cols-1 gap-5 lg:grid-cols-2">
              <div>
                <label class="input-label">{{ t('admin.riskControl.blockStatus') }}</label>
                <input v-model.number="configForm.block_status" type="number" min="400" max="599" class="input" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.riskControl.blockMessage') }}</label>
                <input v-model.trim="configForm.block_message" type="text" class="input" />
              </div>
              <div class="flex items-center justify-between rounded-lg border border-af-hairline p-4">
                <div>
                  <p class="text-sm font-medium text-af-ink">{{ t('admin.riskControl.emailOnHit') }}</p>
                  <p class="mt-1 text-xs text-af-ink-3">{{ t('admin.riskControl.emailOnHitHint') }}</p>
                </div>
                <Toggle v-model="configForm.email_on_hit" />
              </div>
              <div class="flex items-center justify-between rounded-lg border border-af-hairline p-4">
                <div>
                  <p class="text-sm font-medium text-af-ink">{{ t('admin.riskControl.autoBan') }}</p>
                  <p class="mt-1 text-xs text-af-ink-3">{{ t('admin.riskControl.autoBanHint') }}</p>
                </div>
                <Toggle v-model="configForm.auto_ban_enabled" />
              </div>
              <div class="flex items-center justify-between rounded-lg border border-af-hairline p-4 lg:col-span-2">
                <div>
                  <p class="text-sm font-medium text-af-ink">{{ t('admin.riskControl.cyberPolicyExcludeBan') }}</p>
                  <p class="mt-1 text-xs text-af-ink-3">{{ t('admin.riskControl.cyberPolicyExcludeBanHint') }}</p>
                </div>
                <Toggle v-model="configForm.cyber_policy_exclude_from_ban_count" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.riskControl.banThreshold') }}</label>
                <input v-model.number="configForm.ban_threshold" type="number" min="1" max="1000" class="input" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.riskControl.violationWindowHours') }}</label>
                <input v-model.number="configForm.violation_window_hours" type="number" min="1" max="8760" class="input" />
              </div>
              <!-- cyber 会话自动屏蔽（A6-4 从设置页挪来）：存在全局设置里，保存时只发这两项 -->
              <template v-if="sessionBlock">
                <div class="flex items-center justify-between rounded-lg border border-af-hairline p-4 lg:col-span-2" data-testid="risk-cyber-session-block">
                  <div>
                    <p class="text-sm font-medium text-af-ink">{{ t('admin.riskControl.cyberSessionBlock') }}</p>
                    <p class="mt-1 text-xs text-af-ink-3">{{ t('admin.riskControl.cyberSessionBlockHint') }}</p>
                  </div>
                  <Toggle v-model="sessionBlock.enabled" />
                </div>
                <div v-if="sessionBlock.enabled">
                  <label class="input-label">{{ t('admin.riskControl.cyberSessionBlockTTL') }}</label>
                  <input v-model.number="sessionBlock.ttl_seconds" type="number" min="1" class="input" />
                </div>
              </template>
            </div>
          </div>

          <div v-else-if="activeSettingsTab === 'riskThresholds'" class="space-y-5">
            <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <h3 class="text-base font-semibold text-af-ink">{{ t('admin.riskControl.riskThresholds') }}</h3>
                <p class="mt-1 text-sm text-af-ink-3">{{ t('admin.riskControl.riskThresholdsHint') }}</p>
              </div>
              <button
                type="button"
                class="btn btn-secondary inline-flex items-center justify-center gap-2"
                @click="resetRiskThresholds"
              >
                <Icon name="refresh" size="sm" />
                {{ t('admin.riskControl.riskThresholdReset') }}
              </button>
            </div>

            <div class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
              <div
                v-for="row in riskThresholdRows"
                :key="row.category"
                class="rounded-lg border border-af-hairline bg-af-sunken p-4"
              >
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <label class="block truncate text-sm font-semibold text-af-ink" :for="`risk-threshold-${row.category}`">
                      {{ row.category }}
                    </label>
                    <p class="mt-1 text-xs text-af-ink-3">
                      {{ t('admin.riskControl.riskThresholdDefault', { value: formatThresholdPercent(row.defaultValue) }) }}
                    </p>
                  </div>
                  <span class="inline-flex shrink-0 rounded-md bg-af-sheet px-2 py-1 font-mono text-xs font-medium text-af-ink-2">
                    {{ formatThresholdPercent(row.value) }}
                  </span>
                </div>
                <div class="mt-3">
                  <label class="sr-only" :for="`risk-threshold-${row.category}`">
                    {{ t('admin.riskControl.riskThresholdPercent') }}
                  </label>
                  <div class="relative">
                    <input
                      :id="`risk-threshold-${row.category}`"
                      v-model.number="configForm.thresholds[row.category]"
                      :data-test="`risk-threshold-${row.category}`"
                      type="number"
                      min="0"
                      max="100"
                      step="0.1"
                      class="input pr-8 font-mono"
                    />
                    <span class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-af-ink-3">%</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div v-else-if="activeSettingsTab === 'keywords'" class="space-y-5">
            <div
              class="flex items-start gap-3 rounded-lg border p-4"
              :class="keywordNotice.toneClass"
            >
              <Icon
                :name="keywordNotice.icon"
                size="md"
                :class="keywordNotice.iconClass"
              />
              <div class="text-sm leading-6">
                <p class="font-medium" :class="keywordNotice.titleClass">{{ keywordNotice.title }}</p>
                <p class="mt-1 text-xs text-af-ink-3">{{ keywordNotice.description }}</p>
              </div>
            </div>

            <div class="space-y-2">
              <label class="input-label">{{ t('admin.riskControl.keywordBlockingMode') }}</label>
              <div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
                <button
                  v-for="option in keywordBlockingModeOptions"
                  :key="option.value"
                  type="button"
                  class="rounded-lg border p-3 text-left transition-colors"
                  :class="configForm.keyword_blocking_mode === option.value
                    ? 'border-af-hairline-strong bg-af-brand-tint text-af-brand'
                    : 'border-af-hairline hover:bg-af-sunken'"
                  @click="configForm.keyword_blocking_mode = option.value"
                >
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-sm font-semibold">{{ option.label }}</span>
                    <span
                      class="flex h-4 w-4 flex-shrink-0 items-center justify-center rounded-full border"
                      :class="configForm.keyword_blocking_mode === option.value
                        ? 'border-af-brand bg-af-brand text-af-on-brand'
                        : 'border-af-hairline-strong text-transparent'"
                    >
                      <Icon name="check" size="xs" :stroke-width="2" />
                    </span>
                  </div>
                  <p class="mt-1 text-xs leading-5 text-af-ink-3">{{ option.description }}</p>
                </button>
              </div>
            </div>

            <div>
              <div class="mb-2 flex items-center justify-between">
                <label class="input-label mb-0">{{ t('admin.riskControl.blockedKeywords') }}</label>
                <span class="inline-flex rounded-md bg-af-sunken px-2 py-1 text-xs text-af-ink-3">
                  {{ t('admin.riskControl.blockedKeywordCount', { count: blockedKeywordCount }) }}
                </span>
              </div>
              <textarea
                v-model="configForm.blocked_keywords_text"
                class="input min-h-52 resize-y font-mono text-sm"
                :placeholder="t('admin.riskControl.blockedKeywordsPlaceholder')"
                :disabled="configForm.keyword_blocking_mode === 'api_only'"
              ></textarea>
              <p class="mt-2 text-xs text-af-ink-3">
                {{ t('admin.riskControl.blockedKeywordsLimit', { max: blockedKeywordMax }) }}
              </p>
            </div>
          </div>

          <div v-else class="grid grid-cols-1 gap-5 lg:grid-cols-2">
            <div>
              <label class="input-label">{{ t('admin.riskControl.hitRetentionDays') }}</label>
              <input v-model.number="configForm.hit_retention_days" type="number" min="1" max="3650" class="input" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.riskControl.nonHitRetentionDays') }}</label>
              <input v-model.number="configForm.non_hit_retention_days" type="number" min="1" max="3" class="input" />
            </div>
            <div class="rounded-lg border border-af-hairline p-4 text-sm text-af-ink-3 lg:col-span-2">
              <div class="flex flex-wrap items-center gap-3">
                <Icon name="database" size="md" class="text-af-ink-3" />
                <span>{{ t('admin.riskControl.cleanupStats', { hit: status?.last_cleanup_deleted_hit ?? 0, nonHit: status?.last_cleanup_deleted_non_hit ?? 0 }) }}</span>
              </div>
            </div>
          </div>
        </div>

        <template #footer>
          <div class="flex justify-end gap-2">
            <FormError class="mr-auto self-center" :message="settingsError" data-testid="risk-control-settings-error" />
            <button type="button" class="btn btn-secondary" @click="settingsOpen = false">{{ t('common.cancel') }}</button>
            <button type="button" class="btn btn-primary inline-flex items-center gap-2" :disabled="saving" @click="saveConfig">
              <Icon v-if="saving" name="refresh" size="sm" class="animate-spin" />
              <Icon v-else name="check" size="sm" />
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
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <div class="rounded-lg border border-af-hairline bg-af-sunken p-4">
              <p class="text-xs font-medium text-af-ink-3">{{ t('admin.riskControl.table.time') }}</p>
              <p class="mt-1 truncate text-sm font-semibold text-af-ink">{{ formatDateTime(inputDetailRow.created_at) }}</p>
            </div>
            <div class="rounded-lg border border-af-hairline bg-af-sunken p-4">
              <p class="text-xs font-medium text-af-ink-3">{{ t('admin.riskControl.table.user') }}</p>
              <p class="mt-1 truncate text-sm font-semibold text-af-ink">{{ inputDetailRow.user_email || '-' }}</p>
            </div>
            <div class="rounded-lg border border-af-hairline bg-af-sunken p-4">
              <p class="text-xs font-medium text-af-ink-3">{{ t('admin.riskControl.table.result') }}</p>
              <span class="mt-1 inline-flex rounded-md px-2 py-1 text-xs font-medium" :class="resultBadgeClass(inputDetailRow)">
                {{ resultLabel(inputDetailRow) }}
              </span>
            </div>
            <div class="rounded-lg border border-af-hairline bg-af-sunken p-4">
              <p class="text-xs font-medium text-af-ink-3">{{ t('admin.riskControl.table.highest') }}</p>
              <p class="mt-1 truncate text-sm font-semibold text-af-ink">
                {{ inputDetailRow.highest_category || '-' }} / {{ percent(inputDetailRow.highest_score) }}
              </p>
            </div>
            <div v-if="inputDetailRow.matched_keyword" class="rounded-lg border border-af-danger/30 bg-af-danger-tint p-4">
              <p class="text-xs font-medium text-af-danger">{{ t('admin.riskControl.matchedKeyword') }}</p>
              <p class="mt-1 truncate text-sm font-semibold text-af-danger" :title="inputDetailRow.matched_keyword">{{ inputDetailRow.matched_keyword }}</p>
            </div>
          </div>

          <div class="rounded-xl border border-af-hairline bg-af-sheet p-4">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p class="text-sm font-semibold text-af-ink">{{ t('admin.riskControl.inputDetailContent') }}</p>
                <p class="mt-1 text-xs text-af-ink-3">
                  {{ inputDetailRow.endpoint || '-' }} · {{ inputDetailRow.provider || '-' }} / {{ inputDetailRow.model || '-' }}
                </p>
              </div>
            </div>
            <pre class="mt-4 max-h-[420px] overflow-auto whitespace-pre-wrap break-words rounded-lg bg-af-ink p-4 text-sm leading-6 text-af-on-brand">{{ inputDetailText }}</pre>
          </div>
        </div>

        <template #footer>
          <div class="flex justify-end">
            <button type="button" class="btn btn-secondary" @click="closeInputDetail">{{ t('common.close') }}</button>
          </div>
        </template>
      </BaseDialog>
    </div>
    <!-- 清空风险哈希：站内确认框（原来是浏览器原生 confirm），失败就地写出来 -->
    <ConfirmDialog
      :show="clearHashesConfirm"
      :title="t('admin.riskControl.clearFlaggedHashesTitle')"
      :message="t('admin.riskControl.clearFlaggedHashesConfirm')"
      :confirm-text="t('admin.riskControl.clearFlaggedHashes')"
      danger
      :loading="hashActionLoading"
      @confirm="clearFlaggedHashes"
      @cancel="clearHashesConfirm = false"
    >
      <FormError :message="clearHashesError" />
    </ConfirmDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import FormError from '@/components/common/FormError.vue'
import Icon from '@/components/icons/Icon.vue'
import StatRow from '@/components/user/shell/StatRow.vue'
import type { StatItem } from '@/components/user/shell/types'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import Pagination from '@/components/common/Pagination.vue'
import ModelWhitelistSelector from '@/components/account/ModelWhitelistSelector.vue'
import ProxySelector from '@/components/common/ProxySelector.vue'
import { adminAPI } from '@/api/admin'
import type {
  ContentModerationAPIKeyLoad,
  ContentModerationAPIKeyStatus,
  ContentModerationConfig,
  ContentModerationLog,
  ContentModerationModelFilter,
  ContentModerationModelFilterType,
  ContentModerationRuntimeStatus,
  ContentModerationTestAuditResult,
  KeywordBlockingMode,
  ModerationMode,
  UpdateContentModerationConfig,
} from '@/api/admin/riskControl'
import type { Proxy, SelectOption } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime as formatDateTimeValue } from '@/utils/format'

type SettingsTab = 'basic' | 'scope' | 'runtime' | 'response' | 'riskThresholds' | 'retention' | 'keywords'
type WorkerSlotState = 'active' | 'idle' | 'disabled'
type APIKeysWriteMode = 'append' | 'replace'
type ModerationScoreRow = {
  category: string
  score: number
  threshold: number
  hit: boolean
}
type RiskThresholdRow = {
  category: string
  value: number
  defaultValue: number
}

const maxModerationTestImages = 1
const maxModerationTestImageSize = 8 * 1024 * 1024
const maxVisibleApiKeyRows: number = 3
const blockedKeywordMax = 10000
const riskThresholdDefaults: Record<string, number> = {
  harassment: 98,
  'harassment/threatening': 90,
  hate: 65,
  'hate/threatening': 65,
  illicit: 95,
  'illicit/violent': 95,
  'self-harm': 65,
  'self-harm/intent': 85,
  'self-harm/instructions': 65,
  sexual: 65,
  'sexual/minors': 65,
  violence: 95,
  'violence/graphic': 95,
}
const riskThresholdCategories = Object.keys(riskThresholdDefaults)

const { t } = useI18n()
const defaultBlockMessage = () => t('admin.riskControl.defaultBlockMessage')

const loading = ref(true)
const saving = ref(false)
const logsLoading = ref(false)
const statusLoading = ref(false)
const apiKeyTesting = ref(false)
const hashActionLoading = ref(false)
const unbanningUserID = ref<number | null>(null)
const settingsOpen = ref(false)
const pageError = ref('')
const settingsError = ref('')
/** cyber 会话自动屏蔽：存在全局设置里（cyber_session_block_*），打开设置时读取 */
const sessionBlock = ref<{ enabled: boolean; ttl_seconds: number } | null>(null)
const sessionBlockOriginal = ref('')
const activeSettingsTab = ref<SettingsTab>('basic')
const flaggedHashInput = ref('')
const proxies = ref<Proxy[]>([])
const logs = ref<ContentModerationLog[]>([])
const status = ref<ContentModerationRuntimeStatus | null>(null)
const testedApiKeyStatuses = ref<ContentModerationAPIKeyStatus[]>([])
const pendingDeleteApiKeyHashes = ref<string[]>([])
const apiKeyRowsExpanded = ref<boolean>(false)
const moderationTestPrompt = ref('')
const moderationTestImages = ref<string[]>([])
const moderationTestResult = ref<ContentModerationTestAuditResult | null>(null)
const inputDetailRow = ref<ContentModerationLog | null>(null)
let statusTimer: number | null = null

const configForm = reactive({
  enabled: false,
  mode: 'pre_block' as ModerationMode,
  base_url: 'https://api.openai.com',
  model: 'omni-moderation-latest',
  proxy_id: null as number | null,
  api_keys_text: '',
  api_key_configured: false,
  api_key_masked: '',
  api_key_count: 0,
  api_key_masks: [] as string[],
  api_key_statuses: [] as ContentModerationAPIKeyStatus[],
  api_keys_mode: 'append' as APIKeysWriteMode,
  clear_api_key: false,
  timeout_ms: 3000,
  retry_count: 2,
  sample_rate: 100,
  record_non_hits: false,
  worker_count: 4,
  queue_size: 32768,
  block_status: 403,
  block_message: defaultBlockMessage(),
  email_on_hit: true,
  auto_ban_enabled: true,
  cyber_policy_exclude_from_ban_count: false,
  ban_threshold: 10,
  violation_window_hours: 720,
  hit_retention_days: 180,
  non_hit_retention_days: 3,
  pre_hash_check_enabled: false,
  thresholds: { ...riskThresholdDefaults } as Record<string, number>,
  blocked_keywords_text: '',
  keyword_blocking_mode: 'keyword_and_api' as KeywordBlockingMode,
  model_filter_type: 'all' as ContentModerationModelFilterType,
  model_filter_models: [] as string[],
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

const settingsTabs = computed<Array<{ id: SettingsTab; label: string }>>(() => [
  { id: 'basic', label: t('admin.riskControl.tabs.basic') },
  { id: 'scope', label: t('admin.riskControl.tabs.scope') },
  { id: 'runtime', label: t('admin.riskControl.tabs.runtime') },
  { id: 'response', label: t('admin.riskControl.tabs.response') },
  { id: 'riskThresholds', label: t('admin.riskControl.tabs.riskThresholds') },
  { id: 'keywords', label: t('admin.riskControl.tabs.keywords') },
  { id: 'retention', label: t('admin.riskControl.tabs.retention') },
])

const modeOptions = computed<SelectOption[]>(() => [
  { value: 'pre_block', label: t('admin.riskControl.modePreBlock') },
  { value: 'observe', label: t('admin.riskControl.modeObserve') },
  { value: 'off', label: t('admin.riskControl.modeOff') },
])

const keywordBlockingModeOptions = computed<Array<{ value: KeywordBlockingMode; label: string; description: string }>>(() => [
  {
    value: 'keyword_and_api',
    label: t('admin.riskControl.keywordModeKeywordAndApi'),
    description: t('admin.riskControl.keywordModeKeywordAndApiDesc'),
  },
  {
    value: 'keyword_only',
    label: t('admin.riskControl.keywordModeKeywordOnly'),
    description: t('admin.riskControl.keywordModeKeywordOnlyDesc'),
  },
  {
    value: 'api_only',
    label: t('admin.riskControl.keywordModeApiOnly'),
    description: t('admin.riskControl.keywordModeApiOnlyDesc'),
  },
])

const modelFilterOptions = computed<Array<{ value: ContentModerationModelFilterType; label: string; description: string }>>(() => [
  {
    value: 'all',
    label: t('admin.riskControl.modelFilterAll'),
    description: t('admin.riskControl.modelFilterAllDesc'),
  },
  {
    value: 'include',
    label: t('admin.riskControl.modelFilterInclude'),
    description: t('admin.riskControl.modelFilterIncludeDesc'),
  },
  {
    value: 'exclude',
    label: t('admin.riskControl.modelFilterExclude'),
    description: t('admin.riskControl.modelFilterExcludeDesc'),
  },
])

type KeywordNoticeView = {
  title: string
  description: string
  icon: 'infoCircle' | 'exclamationTriangle'
  toneClass: string
  iconClass: string
  titleClass: string
}

const keywordNoticeTones = {
  info: {
    icon: 'infoCircle' as const,
    toneClass: 'border-af-hairline-strong bg-af-brand-tint/60',
    iconClass: 'mt-0.5 flex-shrink-0 text-af-brand',
    titleClass: 'text-af-brand',
  },
  warning: {
    icon: 'exclamationTriangle' as const,
    toneClass: 'border-af-warning/30 bg-af-warning-tint',
    iconClass: 'mt-0.5 flex-shrink-0 text-af-warning',
    titleClass: 'text-af-warning',
  },
}

const keywordNotice = computed<KeywordNoticeView>(() => {
  const strategy = configForm.keyword_blocking_mode
  if (strategy === 'api_only') {
    return {
      ...keywordNoticeTones.info,
      title: t('admin.riskControl.keywordModeApiOnlyNotice'),
      description: t('admin.riskControl.keywordModeApiOnlyDesc'),
    }
  }
  if (configForm.mode !== 'pre_block') {
    return {
      ...keywordNoticeTones.warning,
      title: t('admin.riskControl.blockedKeywordsModeWarning', { mode: modeLabel(configForm.mode) }),
      description: t('admin.riskControl.blockedKeywordsDescription'),
    }
  }
  if (strategy === 'keyword_only') {
    return {
      ...keywordNoticeTones.info,
      title: t('admin.riskControl.keywordModeKeywordOnlyNotice'),
      description: t('admin.riskControl.keywordModeKeywordOnlyDesc'),
    }
  }
  return {
    ...keywordNoticeTones.info,
    title: t('admin.riskControl.blockedKeywordsPreBlockHint'),
    description: t('admin.riskControl.blockedKeywordsDescription'),
  }
})

const resultOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('admin.riskControl.result.all') },
  { value: 'hit', label: t('admin.riskControl.result.hit') },
  { value: 'blocked', label: t('admin.riskControl.result.blocked') },
  { value: 'pass', label: t('admin.riskControl.result.pass') },
  { value: 'error', label: t('admin.riskControl.result.error') },
])

const endpointOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('admin.riskControl.filters.allEndpoints') },
  { value: '/v1/messages', label: '/v1/messages' },
  { value: '/v1/responses', label: '/v1/responses' },
  { value: '/v1/chat/completions', label: '/v1/chat/completions' },
  { value: '/v1beta/models', label: '/v1beta/models' },
  { value: '/v1/images/generations', label: '/v1/images/generations' },
  { value: '/v1/images/edits', label: '/v1/images/edits' },
])

const modelFilterModelCount = computed(() => configForm.model_filter_models.length)

const modelFilterSummary = computed(() => {
  if (configForm.model_filter_type === 'include') {
    return t('admin.riskControl.modelFilterIncludeSummary', { count: modelFilterModelCount.value })
  }
  if (configForm.model_filter_type === 'exclude') {
    return t('admin.riskControl.modelFilterExcludeSummary', { count: modelFilterModelCount.value })
  }
  return t('admin.riskControl.modelFilterAllSummary')
})

const modelFilterPreviewModels = computed(() => configForm.model_filter_models.slice(0, 6))

const hiddenModelFilterModelCount = computed(() => Math.max(0, configForm.model_filter_models.length - modelFilterPreviewModels.value.length))

const inputApiKeyCount = computed(() => parseApiKeys(configForm.api_keys_text).length)

const blockedKeywordList = computed(() => parseBlockedKeywords(configForm.blocked_keywords_text))

const blockedKeywordCount = computed(() => blockedKeywordList.value.length)

const pendingDeletedApiKeyCount = computed(() => pendingDeleteApiKeyHashes.value.length)

const effectiveStoredApiKeyCount = computed(() => Math.max(0, configForm.api_key_count - pendingDeletedApiKeyCount.value))

const apiKeysPlaceholder = computed(() => (
  configForm.api_keys_mode === 'replace'
    ? t('admin.riskControl.apiKeysPlaceholderReplace')
    : t('admin.riskControl.apiKeysPlaceholder')
))

const apiKeysModeHint = computed(() => (
  configForm.api_keys_mode === 'replace'
    ? t('admin.riskControl.apiKeysModeReplaceHint')
    : t('admin.riskControl.apiKeysModeAppendHint')
))

const hasModerationAuditInput = computed(() => {
  return moderationTestPrompt.value.trim() !== '' || moderationTestImages.value.length > 0
})

const isFlaggedHashInputValid = computed(() => /^[a-fA-F0-9]{64}$/.test(flaggedHashInput.value.trim()))

const storedApiKeyTestButtonText = computed(() => {
  if (apiKeyTesting.value) return t('admin.riskControl.testingApiKeys')
  if (hasModerationAuditInput.value) return t('admin.riskControl.testContentWithStoredApiKey')
  return t('admin.riskControl.testStoredApiKeys')
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

const visibleApiKeyRows = computed<ContentModerationAPIKeyStatus[]>(() => {
  if (apiKeyRowsExpanded.value) return apiKeyRows.value
  return apiKeyRows.value.slice(0, maxVisibleApiKeyRows)
})

const hiddenApiKeyRowCount = computed<number>(() => Math.max(0, apiKeyRows.value.length - visibleApiKeyRows.value.length))

const canToggleApiKeyRows = computed<boolean>(() => apiKeyRows.value.length > maxVisibleApiKeyRows)

const activeSavedApiKeyRows = computed<ContentModerationAPIKeyStatus[]>(() => (
  savedApiKeyRows.value.filter((row) => !isStoredApiKeyPendingDelete(row))
))

const apiKeyHealthBadges = computed<Array<{ status: ContentModerationAPIKeyStatus['status']; count: number }>>(() => {
  const counts: Record<ContentModerationAPIKeyStatus['status'], number> = {
    ok: 0,
    error: 0,
    frozen: 0,
    unknown: 0,
  }
  for (const row of activeSavedApiKeyRows.value) {
    counts[row.status] = (counts[row.status] ?? 0) + 1
  }
  if (activeSavedApiKeyRows.value.length === 0 && effectiveStoredApiKeyCount.value > 0) {
    counts.unknown = effectiveStoredApiKeyCount.value
  }
  return (['ok', 'frozen', 'error', 'unknown'] as Array<ContentModerationAPIKeyStatus['status']>)
    .map((item) => ({ status: item, count: counts[item] }))
    .filter((item) => item.count > 0)
})

const apiKeyHealthSummary = computed(() => {
  if (!configForm.api_key_configured) return ''
  if (apiKeyHealthBadges.value.length === 0) return t('admin.riskControl.apiKeyStatusUnknown')
  return apiKeyHealthBadges.value
    .map((badge) => `${apiKeyStatusLabel(badge.status)} ${badge.count}`)
    .join(' · ')
})

const overviewItems = computed<StatItem[]>(() => [
  {
    key: 'status',
    label: t('admin.riskControl.overview.status'),
    value: runtimeStatusText.value,
    hint: modeLabel(configForm.mode),
  },
  {
    key: 'api-key',
    label: t('admin.riskControl.overview.apiKey'),
    value: configForm.api_key_configured ? t('admin.riskControl.apiKeyCount', { count: configForm.api_key_count }) : t('admin.riskControl.notConfigured'),
    hint: configForm.api_key_configured ? apiKeyHealthSummary.value || configForm.model || '-' : configForm.model || '-',
  },
  {
    key: 'scope',
    label: t('admin.riskControl.tabs.scope'),
    value: modelFilterSummary.value,
    hint: t('admin.riskControl.modelFilterModelCount', { count: modelFilterModelCount.value }),
  },
  {
    key: 'logs',
    label: t('admin.riskControl.overview.logs'),
    value: formatNumber(pagination.total),
    hint: t('admin.riskControl.overview.currentFilter'),
  },
])

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

const riskThresholdRows = computed<RiskThresholdRow[]>(() => (
  riskThresholdCategories.map((category) => ({
    category,
    value: configForm.thresholds[category] ?? riskThresholdDefaults[category],
    defaultValue: riskThresholdDefaults[category],
  }))
))

const inputDetailText = computed(() => {
  if (!inputDetailRow.value) return '-'
  return inputDetailRow.value.input_excerpt || inputDetailRow.value.error || '-'
})

const queueUsagePercent = computed(() => `${Math.min(100, Math.max(0, status.value?.queue_usage_percent ?? 0)).toFixed(1)}%`)

const queueUsageStyle = computed(() => ({
  width: queueUsagePercent.value,
}))

const runtimeMode = computed<ModerationMode>(() => status.value?.mode ?? configForm.mode)

const showPreBlockRuntimeCard = computed(() => runtimeMode.value === 'pre_block')

const showWorkerRuntimeCard = computed(() => runtimeMode.value === 'observe')

const preBlockMetricItems = computed(() => [
  {
    key: 'active',
    label: t('admin.riskControl.preBlockActive'),
    value: formatNumber(status.value?.pre_block_active ?? 0),
    meta: t('admin.riskControl.preBlockActiveHint'),
    class: 'bg-af-sunken',
    valueClass: 'text-af-ink-2',
  },
  {
    key: 'checked',
    label: t('admin.riskControl.preBlockChecked'),
    value: formatNumber(status.value?.pre_block_checked ?? 0),
    meta: t('admin.riskControl.preBlockCheckedHint'),
    class: 'bg-af-sunken',
    valueClass: 'text-af-ink',
  },
  {
    key: 'allowed',
    label: t('admin.riskControl.preBlockAllowed'),
    value: formatNumber(status.value?.pre_block_allowed ?? 0),
    meta: t('admin.riskControl.preBlockAllowedHint'),
    class: 'bg-af-success-tint',
    valueClass: 'text-af-success',
  },
  {
    key: 'blocked',
    label: t('admin.riskControl.preBlockBlocked'),
    value: formatNumber(status.value?.pre_block_blocked ?? 0),
    meta: t('admin.riskControl.preBlockBlockedHint'),
    class: 'bg-af-danger-tint',
    valueClass: 'text-af-danger',
  },
  {
    key: 'errors',
    label: t('admin.riskControl.preBlockErrors'),
    value: formatNumber(status.value?.pre_block_errors ?? 0),
    meta: t('admin.riskControl.preBlockErrorsHint'),
    class: 'bg-af-warning-tint',
    valueClass: 'text-af-warning',
  },
  {
    key: 'latency',
    label: t('admin.riskControl.preBlockAvgLatency'),
    value: `${formatNumber(status.value?.pre_block_avg_latency_ms ?? 0)} ms`,
    meta: t('admin.riskControl.preBlockAvgLatencyHint'),
    class: 'bg-af-sunken',
    valueClass: 'text-af-ink-2',
  },
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
  workerTotal: formatNumber(status.value?.worker_count ?? configForm.worker_count),
}))

function preBlockAPIKeyLoadWidth(total: number): string {
  return `${Math.min(100, Math.max(0, (total / preBlockAPIKeyMaxTotal.value) * 100)).toFixed(1)}%`
}

const workerSlots = computed(() => {
  const total = Math.max(0, status.value?.worker_count ?? configForm.worker_count)
  const active = Math.max(0, status.value?.active_workers ?? 0)
  const enabled = Boolean(status.value?.risk_control_enabled && status.value?.enabled && status.value?.mode !== 'off')
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
  if (!configForm.enabled || configForm.mode === 'off') return t('admin.riskControl.overview.disabled')
  return t('admin.riskControl.overview.enabled')
})

function applyConfig(config: ContentModerationConfig) {
  configForm.enabled = config.enabled
  configForm.mode = config.mode
  configForm.base_url = config.base_url || 'https://api.openai.com'
  configForm.model = config.model || 'omni-moderation-latest'
  configForm.proxy_id = config.proxy_id || null
  configForm.api_keys_text = ''
  configForm.api_key_configured = config.api_key_configured
  configForm.api_key_masked = config.api_key_masked || ''
  configForm.api_key_count = config.api_key_count || 0
  configForm.api_key_masks = Array.isArray(config.api_key_masks) ? [...config.api_key_masks] : []
  configForm.api_key_statuses = Array.isArray(config.api_key_statuses) ? [...config.api_key_statuses] : []
  configForm.api_keys_mode = 'append'
  configForm.clear_api_key = false
  pendingDeleteApiKeyHashes.value = []
  testedApiKeyStatuses.value = []
  apiKeyRowsExpanded.value = false
  configForm.timeout_ms = config.timeout_ms || 3000
  configForm.retry_count = config.retry_count ?? 2
  configForm.sample_rate = config.sample_rate ?? 100
  configForm.record_non_hits = config.record_non_hits
  configForm.worker_count = config.worker_count || 4
  configForm.queue_size = config.queue_size || 32768
  configForm.block_status = config.block_status || 403
  configForm.block_message = config.block_message || defaultBlockMessage()
  configForm.email_on_hit = config.email_on_hit ?? true
  configForm.auto_ban_enabled = config.auto_ban_enabled ?? true
  configForm.cyber_policy_exclude_from_ban_count = config.cyber_policy_exclude_from_ban_count ?? false
  configForm.ban_threshold = config.ban_threshold || 10
  configForm.violation_window_hours = config.violation_window_hours || 720
  configForm.hit_retention_days = config.hit_retention_days || 180
  configForm.non_hit_retention_days = Math.min(Math.max(config.non_hit_retention_days || 3, 1), 3)
  configForm.pre_hash_check_enabled = config.pre_hash_check_enabled ?? false
  configForm.thresholds = riskThresholdsFromConfig(config.thresholds)
  configForm.blocked_keywords_text = Array.isArray(config.blocked_keywords) ? config.blocked_keywords.join('\n') : ''
  configForm.keyword_blocking_mode = normalizeKeywordBlockingMode(config.keyword_blocking_mode)
  const modelFilter = normalizeModelFilter(config.model_filter)
  configForm.model_filter_type = modelFilter.type
  configForm.model_filter_models = modelFilter.models
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
    const modelFilterPayload = buildModelFilterPayload()
    if (modelFilterPayload.type !== 'all' && modelFilterPayload.models.length === 0) {
      settingsError.value = t('admin.riskControl.modelFilterModelsRequired')
      return
    }
    const payload: UpdateContentModerationConfig = {
      enabled: configForm.enabled,
      mode: configForm.mode,
      base_url: configForm.base_url,
      model: configForm.model,
      // 后端语义：0 清除代理（直连），>0 指定代理
      proxy_id: configForm.proxy_id ?? 0,
      timeout_ms: Number(configForm.timeout_ms) || 3000,
      retry_count: Number(configForm.retry_count) || 0,
      sample_rate: Number(configForm.sample_rate) || 0,
      record_non_hits: configForm.record_non_hits,
      clear_api_key: configForm.clear_api_key,
      worker_count: Number(configForm.worker_count) || 4,
      queue_size: Number(configForm.queue_size) || 32768,
      block_status: Number(configForm.block_status) || 403,
      block_message: configForm.block_message || defaultBlockMessage(),
      email_on_hit: configForm.email_on_hit,
      auto_ban_enabled: configForm.auto_ban_enabled,
      cyber_policy_exclude_from_ban_count: configForm.cyber_policy_exclude_from_ban_count,
      ban_threshold: Number(configForm.ban_threshold) || 10,
      violation_window_hours: Number(configForm.violation_window_hours) || 720,
      hit_retention_days: Number(configForm.hit_retention_days) || 180,
      non_hit_retention_days: Math.min(Math.max(Number(configForm.non_hit_retention_days) || 3, 1), 3),
      pre_hash_check_enabled: configForm.pre_hash_check_enabled,
      thresholds: buildRiskThresholdPayload(),
      blocked_keywords: blockedKeywordList.value,
      keyword_blocking_mode: configForm.keyword_blocking_mode,
      model_filter: modelFilterPayload,
    }
    const keys = parseApiKeys(configForm.api_keys_text)
    if (!payload.clear_api_key && configForm.api_keys_mode === 'replace' && keys.length === 0) {
      settingsError.value = t('admin.riskControl.apiKeysReplaceNoInput')
      return
    }
    if (keys.length > 0) {
      payload.api_keys = keys
      payload.api_keys_mode = configForm.api_keys_mode
      payload.clear_api_key = false
    }
    if (!payload.clear_api_key && configForm.api_keys_mode !== 'replace' && pendingDeleteApiKeyHashes.value.length > 0) {
      payload.delete_api_key_hashes = [...pendingDeleteApiKeyHashes.value]
    }

    const updated = await adminAPI.riskControl.updateConfig(payload)
    applyConfig(updated)
    if (sessionBlock.value && JSON.stringify(sessionBlock.value) !== sessionBlockOriginal.value) {
      // 设置接口只写请求里带了的字段，这里只发这两项
      await adminAPI.settings.updateSettings({
        cyber_session_block_enabled: sessionBlock.value.enabled,
        cyber_session_block_ttl_seconds: Number(sessionBlock.value.ttl_seconds) || 3600,
      })
      sessionBlockOriginal.value = JSON.stringify(sessionBlock.value)
    }
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

function inputSummaryText(row: ContentModerationLog): string {
  return row.input_excerpt || row.error || '-'
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

async function deleteFlaggedHash() {
  if (!isFlaggedHashInputValid.value || hashActionLoading.value) return
  hashActionLoading.value = true
  settingsError.value = ''
  try {
    await adminAPI.riskControl.deleteFlaggedHash(flaggedHashInput.value)
    flaggedHashInput.value = ''
    await loadStatus(true)
  } catch (err: unknown) {
    settingsError.value = extractApiErrorMessage(err, t('admin.riskControl.flaggedHashDeleteFailed'))
  } finally {
    hashActionLoading.value = false
  }
}

const clearHashesConfirm = ref(false)
const clearHashesError = ref('')
watch(clearHashesConfirm, (open) => { if (open) clearHashesError.value = '' })

async function clearFlaggedHashes() {
  if (hashActionLoading.value) return
  hashActionLoading.value = true
  try {
    await adminAPI.riskControl.clearFlaggedHashes()
    clearHashesConfirm.value = false
    await loadStatus(true)
  } catch (err: unknown) {
    clearHashesError.value = extractApiErrorMessage(err, t('admin.riskControl.flaggedHashesClearFailed'))
  } finally {
    hashActionLoading.value = false
  }
}

function openSettings() {
  activeSettingsTab.value = 'basic'
  settingsError.value = ''
  settingsOpen.value = true
  void loadSessionBlock()
}

async function loadSessionBlock() {
  try {
    const settings = await adminAPI.settings.getSettings()
    const value = {
      enabled: Boolean(settings.cyber_session_block_enabled),
      ttl_seconds: Number(settings.cyber_session_block_ttl_seconds) || 3600,
    }
    sessionBlock.value = { ...value }
    sessionBlockOriginal.value = JSON.stringify(value)
  } catch (err: unknown) {
    sessionBlock.value = null
    settingsError.value = extractApiErrorMessage(err, t('admin.riskControl.loadFailed'))
  }
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

function toggleClearApiKey() {
  configForm.clear_api_key = !configForm.clear_api_key
  if (configForm.clear_api_key) {
    configForm.api_keys_text = ''
    configForm.api_keys_mode = 'append'
    testedApiKeyStatuses.value = []
    pendingDeleteApiKeyHashes.value = []
  }
}

function setAPIKeysMode(mode: APIKeysWriteMode) {
  configForm.api_keys_mode = mode
  if (mode === 'replace') {
    pendingDeleteApiKeyHashes.value = []
  }
}

function setModelFilterType(type: ContentModerationModelFilterType) {
  configForm.model_filter_type = type
  if (type === 'all') {
    configForm.model_filter_models = []
  }
}

async function testApiKeys(useInputKeys: boolean) {
  const keys = useInputKeys ? parseApiKeys(configForm.api_keys_text) : []
  settingsError.value = ''
  if (useInputKeys && keys.length === 0) {
    settingsError.value = t('admin.riskControl.apiKeyTestNoInput')
    return
  }
  apiKeyTesting.value = true
  try {
    const result = await adminAPI.riskControl.testAPIKeys({
      api_keys: keys,
      base_url: configForm.base_url,
      model: configForm.model,
      timeout_ms: Number(configForm.timeout_ms) || 3000,
      // 与保存语义一致：0 强制直连，>0 指定代理，确保测试与实际审计走同一条链路
      proxy_id: configForm.proxy_id ?? 0,
      prompt: moderationTestPrompt.value,
      images: moderationTestImages.value,
    })
    moderationTestResult.value = result.audit_result ?? null
    if (useInputKeys) {
      testedApiKeyStatuses.value = result.items.map((item) => ({ ...item, configured: false }))
    } else {
      mergeConfiguredAPIKeyStatuses(result.items)
      testedApiKeyStatuses.value = []
      await loadStatus(true)
    }
  } catch (err: unknown) {
    settingsError.value = extractApiErrorMessage(err, t('admin.riskControl.apiKeyTestFailed'))
  } finally {
    apiKeyTesting.value = false
  }
}

function mergeConfiguredAPIKeyStatuses(items: ContentModerationAPIKeyStatus[]) {
  if (!hasModerationAuditInput.value || configForm.api_key_statuses.length === 0) {
    configForm.api_key_statuses = items
    return
  }
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
    } catch (error) {
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
  const found = modeOptions.value.find((option) => option.value === mode)
  return found?.label ?? mode
}

function modeDescription(mode: ModerationMode): string {
  const descriptions: Record<ModerationMode, string> = {
    pre_block: t('admin.riskControl.modePreBlockDesc'),
    observe: t('admin.riskControl.modeObserveDesc'),
    off: t('admin.riskControl.modeOffDesc'),
  }
  return descriptions[mode] ?? ''
}

function resultLabel(row: ContentModerationLog): string {
  if (row.action === 'cyber_policy') return t('admin.riskControl.action.cyberPolicy')
  if (row.action === 'keyword_block') return t('admin.riskControl.action.keywordBlock')
  if (row.action === 'block') return t('admin.riskControl.action.block')
  if (row.action === 'error' || row.error) return t('admin.riskControl.action.error')
  if (row.flagged) return t('admin.riskControl.result.hit')
  return t('admin.riskControl.result.pass')
}

function resultBadgeClass(row: ContentModerationLog): string {
  if (row.action === 'block' || row.action === 'keyword_block' || row.action === 'cyber_policy') return 'bg-af-danger-tint text-af-danger'
  if (row.action === 'error' || row.error) return 'bg-af-warning-tint text-af-warning'
  if (row.flagged) return 'bg-af-sunken text-af-ink-2'
  return 'bg-af-success-tint text-af-success'
}

function workerSlotClass(state: WorkerSlotState): string {
  if (state === 'active') {
    return 'border-af-hairline bg-af-sunken text-af-ink-2'
  }
  if (state === 'idle') {
    return 'border-af-success/30 bg-af-success-tint text-af-success'
  }
  return 'border-af-hairline bg-af-sheet text-af-ink-3'
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

function apiKeyStatusLabel(statusValue: ContentModerationAPIKeyStatus['status']): string {
  const labels: Record<ContentModerationAPIKeyStatus['status'], string> = {
    ok: t('admin.riskControl.apiKeyStatusOk'),
    error: t('admin.riskControl.apiKeyStatusError'),
    frozen: t('admin.riskControl.apiKeyStatusFrozen'),
    unknown: t('admin.riskControl.apiKeyStatusUnknown'),
  }
  return labels[statusValue] ?? labels.unknown
}

function apiKeyStatusBadgeClass(statusValue: ContentModerationAPIKeyStatus['status']): string {
  const classes: Record<ContentModerationAPIKeyStatus['status'], string> = {
    ok: 'bg-af-success-tint text-af-success',
    error: 'bg-af-warning-tint text-af-warning',
    frozen: 'bg-af-danger-tint text-af-danger',
    unknown: 'bg-af-sunken text-af-ink-2',
  }
  return classes[statusValue] ?? classes.unknown
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
  if (row.last_http_status > 0) {
    parts.push(t('admin.riskControl.apiKeyHTTPStatus', { status: row.last_http_status }))
  }
  if (row.frozen_until) {
    parts.push(t('admin.riskControl.apiKeyFrozenUntil', { time: formatDateTime(row.frozen_until) }))
  } else if (row.last_checked_at) {
    parts.push(t('admin.riskControl.apiKeyLastChecked', { time: formatDateTime(row.last_checked_at) }))
  } else {
    parts.push(t('admin.riskControl.apiKeyNotTested'))
  }
  return parts.join(' / ')
}

function parseApiKeys(value: string): string[] {
  return value
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter((item, index, arr) => item && arr.indexOf(item) === index)
}

function normalizeKeywordBlockingMode(value: unknown): KeywordBlockingMode {
  if (value === 'keyword_only' || value === 'api_only' || value === 'keyword_and_api') {
    return value
  }
  return 'keyword_and_api'
}

function normalizeModelFilter(value: unknown): ContentModerationModelFilter {
  if (!value || typeof value !== 'object') {
    return { type: 'all', models: [] }
  }
  const raw = value as Partial<ContentModerationModelFilter>
  const type = normalizeModelFilterType(raw.type)
  const models = type === 'all' ? [] : normalizeModelNames(raw.models)
  return { type, models }
}

function normalizeModelFilterType(value: unknown): ContentModerationModelFilterType {
  if (value === 'include' || value === 'exclude' || value === 'all') {
    return value
  }
  return 'all'
}

function normalizeModelNames(models: unknown): string[] {
  if (!Array.isArray(models)) return []
  const seen = new Set<string>()
  const out: string[] = []
  for (const item of models) {
    const model = String(item ?? '').trim()
    if (!model) continue
    const key = model.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    out.push(model)
  }
  return out
}

function buildModelFilterPayload(): ContentModerationModelFilter {
  const type = normalizeModelFilterType(configForm.model_filter_type)
  if (type === 'all') {
    return { type: 'all', models: [] }
  }
  return {
    type,
    models: normalizeModelNames(configForm.model_filter_models),
  }
}

function riskThresholdsFromConfig(thresholds: Record<string, number> | null | undefined): Record<string, number> {
  const out: Record<string, number> = { ...riskThresholdDefaults }
  for (const category of riskThresholdCategories) {
    const value = thresholds?.[category]
    if (Number.isFinite(value)) {
      out[category] = clampPercent(Number(value) * 100)
    }
  }
  return out
}

function buildRiskThresholdPayload(): Record<string, number> {
  const payload: Record<string, number> = {}
  for (const category of riskThresholdCategories) {
    payload[category] = Number((clampPercent(configForm.thresholds[category]) / 100).toFixed(4))
  }
  return payload
}

function resetRiskThresholds() {
  configForm.thresholds = { ...riskThresholdDefaults }
}

function clampPercent(value: unknown): number {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) {
    return 0
  }
  return Math.min(100, Math.max(0, numeric))
}

function formatThresholdPercent(value: number): string {
  return `${clampPercent(value).toFixed(1)}%`
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
