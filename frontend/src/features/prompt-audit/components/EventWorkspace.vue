<template>
  <!--
    审计事件（2026-10-05 走查）：原来十个带标签的大框平铺成一张表单 + 红色实心「按筛选删除」压在最上面；
    改成与其它列表页同一套——一行 32px 小控件（选了就生效，文字框回车 / 失焦生效），低频的入口 / 请求 ID / Hash 收在「更多筛选」后面；
    删除是次按钮，「删除选中项」有选中才出现；表格不套圆角卡片、表头不上灰底。
  -->
  <section aria-labelledby="prompt-events-title" class="py-6">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 id="prompt-events-title" class="text-base font-semibold text-af-ink">{{ t('admin.promptAudit.events.title') }}</h2>
        <p class="mt-1 text-sm text-af-ink-3">{{ t('admin.promptAudit.events.description') }}</p>
      </div>
      <div class="flex flex-wrap gap-2">
        <button v-if="selectedIds.length > 0" type="button" class="btn btn-secondary btn-sm text-af-danger" @click="$emit('batch-delete')">
          {{ t('admin.promptAudit.events.deleteSelected', { count: selectedIds.length }) }}
        </button>
        <button type="button" class="btn btn-secondary btn-sm" data-test="filter-delete" @click="$emit('preview-delete')">
          {{ t('admin.promptAudit.events.deleteByFilter') }}
        </button>
      </div>
    </div>

    <ListToolbar class="mt-5">
      <SearchInput
        :model-value="localFilters.keyword"
        compact
        class="w-full sm:w-72"
        :placeholder="t('admin.promptAudit.events.keywordPlaceholder')"
        @update:model-value="localFilters.keyword = $event"
        @search="applyFilters"
      />
      <FilterChip v-model="localFilters.decision" :label="t('admin.promptAudit.events.decision')" :options="decisionOptions" test-id="prompt-filter-decision" @change="applyFilters" />
      <FilterChip v-model="localFilters.risk_level" :label="t('admin.promptAudit.events.risk')" :options="riskOptions" test-id="prompt-filter-risk" @change="applyFilters" />
      <!-- 用户 / 密钥按名字选（与用量页筛选同一个选择器），不让人手填内部 id；先选了用户时密钥只列该用户的 -->
      <EntityPicker kind="user" compact class="w-full sm:w-44" :title="t('admin.promptAudit.events.filterUser')" :model-value="idFilterValue(localFilters.user_id)" @update:model-value="setUserFilter" />
      <EntityPicker kind="apiKey" compact class="w-full sm:w-44" :title="t('admin.promptAudit.events.filterApiKey')" :user-id="idFilterValue(localFilters.user_id)" :model-value="idFilterValue(localFilters.api_key_id)" @update:model-value="setApiKeyFilter" />
      <span class="inline-flex items-center gap-1.5 text-13 text-af-ink-3">
        <input v-model="localFilters.start_at" type="datetime-local" class="input h-8 w-auto py-0 text-13" :title="t('admin.promptAudit.events.startAt')" :aria-label="t('admin.promptAudit.events.startAt')" @change="applyFilters" />
        <span aria-hidden="true">–</span>
        <input v-model="localFilters.end_at" type="datetime-local" class="input h-8 w-auto py-0 text-13" :title="t('admin.promptAudit.events.endAt')" :aria-label="t('admin.promptAudit.events.endAt')" @change="applyFilters" />
      </span>
      <template v-if="showMore">
        <input v-model.trim="localFilters.endpoint" type="text" class="input h-8 w-full py-0 text-13 sm:w-44" :placeholder="t('admin.promptAudit.events.endpoint')" :aria-label="t('admin.promptAudit.events.endpoint')" @change="applyFilters" />
        <input v-model.trim="localFilters.request_id" type="text" class="input h-8 w-full py-0 text-13 sm:w-44" :placeholder="t('admin.promptAudit.events.requestId')" :aria-label="t('admin.promptAudit.events.requestId')" @change="applyFilters" />
        <input v-model.trim="localFilters.prompt_hash" type="text" class="input h-8 w-full py-0 font-mono text-13 sm:w-56" :placeholder="t('admin.promptAudit.events.promptHash')" :aria-label="t('admin.promptAudit.events.promptHash')" @change="applyFilters" />
      </template>
      <button
        v-else
        type="button"
        class="inline-flex h-8 items-center gap-1 rounded-full px-2 text-13 text-af-ink-3 transition-colors hover:text-af-ink"
        @click="moreOpen = true"
      >
        <Icon name="plus" size="xs" :stroke-width="2" />
        {{ t('admin.usage.moreFilters') }}
      </button>
      <button v-if="anyActive" type="button" class="px-2 text-13 text-af-ink-3 transition-colors hover:text-af-ink" @click="resetFilters">
        {{ t('admin.usage.clearFilters') }}
      </button>
    </ListToolbar>
    <FormError class="mt-4" :message="error" />
    <div class="mt-4 overflow-x-auto border-t border-af-hairline">
      <table class="min-w-[1120px] w-full text-left text-sm">
        <thead class="text-xs text-af-ink-3">
          <tr>
            <th class="w-10 px-3 py-3"><input type="checkbox" :checked="allSelected" :aria-label="t('admin.promptAudit.events.selectAll')" @change="toggleAll" /></th>
            <th class="px-3 py-3 font-medium">{{ t('admin.promptAudit.events.time') }}</th>
            <th class="px-3 py-3 font-medium">{{ t('admin.promptAudit.events.identity') }}</th>
            <th class="px-3 py-3 font-medium">{{ t('admin.promptAudit.events.route') }}</th>
            <th class="px-3 py-3 font-medium">{{ t('admin.promptAudit.events.result') }}</th>
            <th class="px-3 py-3 font-medium">{{ t('admin.promptAudit.events.preview') }}</th>
            <th class="px-3 py-3 text-right font-medium">{{ t('admin.promptAudit.common.actions') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-af-hairline bg-af-sheet">
          <tr v-if="loading"><td colspan="7" class="px-4 py-12 text-center text-af-ink-3" aria-busy="true">{{ t('common.loading') }}</td></tr>
          <tr v-else-if="events.length === 0"><td colspan="7" class="px-4 py-12 text-center text-af-ink-3">{{ anyActive ? t('admin.promptAudit.events.emptyFiltered') : t('admin.promptAudit.events.empty') }}</td></tr>
          <tr v-for="event in events" v-else :key="event.id" :data-test="`event-${event.id}`" class="align-top hover:bg-af-sunken/70">
            <td class="px-3 py-3"><input type="checkbox" :checked="selectedIds.includes(event.id)" :aria-label="t('admin.promptAudit.events.selectEvent', { time: formatDate(event.created_at) })" @change="toggleOne(event.id)" /></td>
            <td class="whitespace-nowrap px-3 py-3 text-xs text-af-ink-2">{{ formatDate(event.created_at) }}</td>
            <!-- 身份：邮箱（没有就用户名）+ 密钥名，不带标签和复制按钮；要复制去详情里 -->
            <td class="max-w-56 px-3 py-3">
              <p class="truncate text-af-ink" :title="event.snapshot.user_email || event.snapshot.username">{{ event.snapshot.user_email || event.snapshot.username || '—' }}</p>
              <p class="mt-1 truncate text-xs text-af-ink-3" :title="event.snapshot.api_key_name">{{ event.snapshot.api_key_name || '—' }}</p>
            </td>
            <td class="px-3 py-3">
              <p class="font-medium text-af-ink">{{ event.snapshot.endpoint }}</p>
              <p class="mt-1 text-xs text-af-ink-3">{{ event.snapshot.model }} · {{ event.snapshot.protocol }} · {{ event.snapshot.stage || 'http' }}</p>
            </td>
            <td class="px-3 py-3">
              <span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="decisionClass(event.decision)">{{ formatDecisionRisk(event.decision, event.risk_level) }}</span>
              <!-- 请求到底拦没拦：只有同步阻止下判 Block 才真拦；异步审计的事件只是记录，请求已放行 -->
              <p class="mt-1.5 text-xs" :class="event.blocked ? 'font-medium text-af-danger' : 'text-af-ink-3'" data-test="event-outcome">
                {{ event.blocked ? t('admin.promptAudit.events.outcomeBlocked') : t('admin.promptAudit.events.outcomeAllowed') }}
              </p>
              <p class="mt-1 max-w-48 truncate text-xs text-af-ink-3" :title="formatCategories(event.categories)">{{ formatCategories(event.categories) }}</p>
            </td>
            <td class="max-w-xs px-3 py-3"><p class="line-clamp-2 break-words text-af-ink-2">{{ event.snapshot.redacted_preview || '—' }}</p></td>
            <td class="whitespace-nowrap px-3 py-3 text-right">
              <button type="button" class="btn btn-ghost btn-sm" @click="$emit('view', event.id)">{{ t('common.view') }}</button>
              <button type="button" class="btn btn-ghost btn-sm text-af-danger" @click="$emit('delete', event.id)">{{ t('common.delete') }}</button>
            </td>
          </tr>
        </tbody>
      </table>
      <Pagination :total="total" :page="page" :page-size="pageSize" @update:page="$emit('page', $event)" @update:page-size="$emit('page-size', $event)" />
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Pagination from '@/components/common/Pagination.vue'
import EntityPicker from '@/components/admin/form/EntityPicker.vue'
import FilterChip from '@/components/common/FilterChip.vue'
import FormError from '@/components/common/FormError.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Icon from '@/components/icons/Icon.vue'
import { ListToolbar } from '@/components/admin/list'
import type { FilterOption } from '@/components/common/types'
import type { PromptAuditEvent, PromptEventFilters } from '../types'
import { cloneData, emptyEventFilters, SCANNER_CATALOG } from '../viewModel'

const props = defineProps<{
  events: PromptAuditEvent[]; total: number; page: number; pageSize: number
  filters: PromptEventFilters; selectedIds: number[]; loading: boolean; error: string
}>()
const emit = defineEmits<{
  (event: 'filters-change', value: PromptEventFilters): void
  (event: 'search', value: PromptEventFilters): void
  (event: 'selection', value: number[]): void
  (event: 'page', value: number): void
  (event: 'page-size', value: number): void
  (event: 'view', id: number): void
  (event: 'delete', id: number): void
  (event: 'batch-delete'): void
  (event: 'preview-delete'): void
}>()
const { t, locale } = useI18n()
const localFilters = reactive<PromptEventFilters>(cloneData(props.filters))
watch(() => props.filters, (value) => Object.assign(localFilters, cloneData(value)), { deep: true })
const allSelected = computed(() => props.events.length > 0 && props.events.every((event) => props.selectedIds.includes(event.id)))

// 筛选里的用户 / 密钥 id 按字符串存（接口参数由 eventQueryParams 转数字），选择器用数字
function idFilterValue(value: string): number | undefined {
  const id = Number(value)
  return Number.isInteger(id) && id > 0 ? id : undefined
}
function setUserFilter(userId: number | undefined) {
  localFilters.user_id = userId ? String(userId) : ''
  // 换了用户，原来选的密钥不一定属于新用户
  localFilters.api_key_id = ''
  applyFilters()
}
function setApiKeyFilter(apiKeyId: number | undefined) {
  localFilters.api_key_id = apiKeyId ? String(apiKeyId) : ''
  applyFilters()
}

const decisionOptions = computed<FilterOption[]>(() => ['pass', 'flag', 'critical'].map((value) => ({ value, label: t(`admin.promptAudit.decisions.${value}`) })))
const riskOptions = computed<FilterOption[]>(() => ['low', 'medium', 'high', 'critical'].map((value) => ({ value, label: t(`admin.promptAudit.riskLevels.${value}`) })))

// 「更多筛选」：入口 / 请求 ID / Prompt Hash；里面有值时一直展开
const moreOpen = ref(false)
const showMore = computed(() => moreOpen.value || Boolean(localFilters.endpoint || localFilters.request_id || localFilters.prompt_hash))
const anyActive = computed(() => (Object.keys(localFilters) as Array<keyof PromptEventFilters>).some((key) => Boolean(localFilters[key])))

function applyFilters() {
  const value = cloneData(localFilters)
  emit('filters-change', value)
  emit('search', value)
}
function resetFilters() {
  Object.assign(localFilters, emptyEventFilters())
  applyFilters()
}
function toggleOne(id: number) {
  const selected = new Set(props.selectedIds)
  if (selected.has(id)) selected.delete(id)
  else selected.add(id)
  emit('selection', [...selected])
}
function toggleAll() {
  emit('selection', allSelected.value ? [] : props.events.map((event) => event.id))
}
function formatDate(value: string): string {
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'short', timeStyle: 'medium' }).format(new Date(value))
}
function decisionClass(decision: string): string {
  if (decision === 'critical') return 'bg-af-danger-tint text-af-danger'
  if (decision === 'flag') return 'bg-af-warning-tint text-af-warning'
  return 'bg-af-success-tint text-af-success'
}
const DECISIONS = new Set(['pass', 'flag', 'critical'])
const RISK_LEVELS = new Set(['low', 'medium', 'high', 'critical'])

function translateDecision(decision: string): string {
  return DECISIONS.has(decision) ? t(`admin.promptAudit.decisions.${decision}`) : decision
}
function translateRiskLevel(riskLevel: string): string {
  return RISK_LEVELS.has(riskLevel) ? t(`admin.promptAudit.riskLevels.${riskLevel}`) : riskLevel
}
function translateCategory(category: string): string {
  return SCANNER_CATALOG.some((scanner) => scanner.id === category)
    ? t(`admin.promptAudit.scanners.${category}`)
    : category
}
function formatDecisionRisk(decision: string, riskLevel: string): string {
  return `${translateDecision(decision)} · ${translateRiskLevel(riskLevel)}`
}
function formatCategories(categories: string[]): string {
  if (!categories.length) return '—'
  return categories.map(translateCategory).join(', ')
}
</script>
