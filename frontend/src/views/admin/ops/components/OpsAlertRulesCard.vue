<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useMediaQuery } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import FormError from '@/components/common/FormError.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import { opsAPI } from '@/api/admin/ops'
import type { AlertRule, MetricType, Operator } from '../types'
import type { OpsSeverity } from '@/api/admin/ops'
import { formatDateTime } from '../utils/opsFormatters'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()

// 与 DataTable 一致：< 768px 切换为卡片视图，避免宽表在移动端被截断。
const isDesktopViewport = useMediaQuery('(min-width: 768px)')

const loading = ref(false)
const rules = ref<AlertRule[]>([])

async function load() {
  loading.value = true
  try {
    rules.value = await opsAPI.listAlertRules()
  } catch (err: any) {
    console.error('[OpsAlertRulesCard] Failed to load rules', err)
    rules.value = []
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  load()
})

const sortedRules = computed(() => {
  return [...rules.value].sort((a, b) => (b.id || 0) - (a.id || 0))
})

const showEditor = ref(false)
const saving = ref(false)
const editingId = ref<number | null>(null)
const draft = ref<AlertRule | null>(null)
// 校验结果在用户点过保存、或离开过名称输入框之后才显示：新建时名称默认为空，打开就标红是误报
const showValidation = ref(false)
// 前端校验没拦住、被后端拒绝保存时的原因，显示在弹窗底部
const saveError = ref('')

type MetricGroup = 'system' | 'account'

interface MetricDefinition {
  type: MetricType
  group: MetricGroup
  label: string
  description: string
  recommendedOperator: Operator
  recommendedThreshold: number
  unit?: string
}

const metricDefinitions = computed(() => {
  return [
    // System-level metrics
    {
      type: 'success_rate',
      group: 'system',
      label: t('admin.ops.alertRules.metrics.successRate'),
      description: t('admin.ops.alertRules.metricDescriptions.successRate'),
      recommendedOperator: '<',
      recommendedThreshold: 99,
      unit: '%'
    },
    {
      type: 'error_rate',
      group: 'system',
      label: t('admin.ops.alertRules.metrics.errorRate'),
      description: t('admin.ops.alertRules.metricDescriptions.errorRate'),
      recommendedOperator: '>',
      recommendedThreshold: 1,
      unit: '%'
    },
    {
      type: 'upstream_error_rate',
      group: 'system',
      label: t('admin.ops.alertRules.metrics.upstreamErrorRate'),
      description: t('admin.ops.alertRules.metricDescriptions.upstreamErrorRate'),
      recommendedOperator: '>',
      recommendedThreshold: 1,
      unit: '%'
    },
    {
      type: 'cpu_usage_percent',
      group: 'system',
      label: t('admin.ops.alertRules.metrics.cpu'),
      description: t('admin.ops.alertRules.metricDescriptions.cpu'),
      recommendedOperator: '>',
      recommendedThreshold: 80,
      unit: '%'
    },
    {
      type: 'memory_usage_percent',
      group: 'system',
      label: t('admin.ops.alertRules.metrics.memory'),
      description: t('admin.ops.alertRules.metricDescriptions.memory'),
      recommendedOperator: '>',
      recommendedThreshold: 80,
      unit: '%'
    },
    {
      type: 'concurrency_queue_depth',
      group: 'system',
      label: t('admin.ops.alertRules.metrics.queueDepth'),
      description: t('admin.ops.alertRules.metricDescriptions.queueDepth'),
      recommendedOperator: '>',
      recommendedThreshold: 10
    },

    // Account-level metrics
    {
      type: 'account_rate_limited_count',
      group: 'account',
      label: t('admin.ops.alertRules.metrics.accountRateLimitedCount'),
      description: t('admin.ops.alertRules.metricDescriptions.accountRateLimitedCount'),
      recommendedOperator: '>',
      recommendedThreshold: 0
    },
    {
      type: 'account_error_count',
      group: 'account',
      label: t('admin.ops.alertRules.metrics.accountErrorCount'),
      description: t('admin.ops.alertRules.metricDescriptions.accountErrorCount'),
      recommendedOperator: '>',
      recommendedThreshold: 0
    },
    {
      type: 'account_error_ratio',
      group: 'account',
      label: t('admin.ops.alertRules.metrics.accountErrorRatio'),
      description: t('admin.ops.alertRules.metricDescriptions.accountErrorRatio'),
      recommendedOperator: '>',
      recommendedThreshold: 5,
      unit: '%'
    },
    {
      type: 'account_temp_unscheduled_count',
      group: 'account',
      label: t('admin.ops.alertRules.metrics.accountTempUnscheduledCount'),
      description: t('admin.ops.alertRules.metricDescriptions.accountTempUnscheduledCount'),
      recommendedOperator: '>',
      recommendedThreshold: 0
    },
    {
      type: 'overload_account_count',
      group: 'account',
      label: t('admin.ops.alertRules.metrics.overloadAccountCount'),
      description: t('admin.ops.alertRules.metricDescriptions.overloadAccountCount'),
      recommendedOperator: '>',
      recommendedThreshold: 0
    }
  ] satisfies MetricDefinition[]
})

const selectedMetricDefinition = computed(() => {
  const metricType = draft.value?.metric_type
  if (!metricType) return null
  return metricDefinitions.value.find((m) => m.type === metricType) ?? null
})

const metricOptions = computed(() => {
  const buildGroup = (group: MetricGroup): SelectOption[] => {
    const items = metricDefinitions.value.filter((m) => m.group === group)
    if (items.length === 0) return []
    const headerValue = `__group__${group}`
    return [
      {
        value: headerValue,
        label: t(`admin.ops.alertRules.metricGroups.${group}`),
        disabled: true,
        kind: 'group'
      },
      ...items.map((m) => ({ value: m.type, label: m.label }))
    ]
  }

  return [...buildGroup('system'), ...buildGroup('account')]
})

const operatorOptions = computed(() => {
  const ops: Operator[] = ['>', '>=', '<', '<=', '==', '!=']
  return ops.map((o) => ({ value: o, label: o }))
})

const severityOptions = computed(() => {
  const sev: OpsSeverity[] = ['P0', 'P1', 'P2', 'P3']
  return sev.map((s) => ({ value: s, label: s }))
})

const windowOptions = computed(() => {
  const windows = [1, 5, 60]
  return windows.map((m) => ({ value: m, label: `${m}m` }))
})

function newRuleDraft(): AlertRule {
  return {
    name: '',
    description: '',
    enabled: true,
    metric_type: 'error_rate',
    operator: '>',
    threshold: 1,
    window_minutes: 1,
    sustained_minutes: 2,
    severity: 'P1',
    cooldown_minutes: 10,
    notify_email: true
  }
}

function openCreate() {
  editingId.value = null
  draft.value = newRuleDraft()
  showValidation.value = false
  saveError.value = ''
  showEditor.value = true
}

function openEdit(rule: AlertRule) {
  editingId.value = rule.id ?? null
  draft.value = JSON.parse(JSON.stringify(rule))
  showValidation.value = false
  saveError.value = ''
  showEditor.value = true
}

const editorValidation = computed(() => {
  const errors: string[] = []
  const r = draft.value
  if (!r) return { valid: true, errors }
  if (!r.name || !r.name.trim()) errors.push(t('admin.ops.alertRules.validation.nameRequired'))
  if (!r.metric_type) errors.push(t('admin.ops.alertRules.validation.metricRequired'))
  if (!r.operator) errors.push(t('admin.ops.alertRules.validation.operatorRequired'))
  if (!(typeof r.threshold === 'number' && Number.isFinite(r.threshold))) {
    errors.push(t('admin.ops.alertRules.validation.thresholdRequired'))
  } else if (metricDefinitions.value.find((m) => m.type === r.metric_type)?.unit === '%') {
    // 与后端 ops_alerts_handler 的 isPercentOrRateMetric 同一组指标（带 % 单位的那 6 个）：阈值只能在 0–100
    if (r.threshold < 0 || r.threshold > 100) errors.push(t('admin.ops.alertRules.validation.thresholdPercentRange'))
  } else if (r.threshold < 0) {
    errors.push(t('admin.ops.alertRules.validation.thresholdNonNegative'))
  }
  if (!(typeof r.window_minutes === 'number' && Number.isFinite(r.window_minutes) && [1, 5, 60].includes(r.window_minutes))) {
    errors.push(t('admin.ops.alertRules.validation.windowRange'))
  }
  if (!(typeof r.sustained_minutes === 'number' && Number.isFinite(r.sustained_minutes) && r.sustained_minutes >= 1 && r.sustained_minutes <= 1440)) {
    errors.push(t('admin.ops.alertRules.validation.sustainedRange'))
  }
  if (!(typeof r.cooldown_minutes === 'number' && Number.isFinite(r.cooldown_minutes) && r.cooldown_minutes >= 0 && r.cooldown_minutes <= 1440)) {
    errors.push(t('admin.ops.alertRules.validation.cooldownRange'))
  }
  return { valid: errors.length === 0, errors }
})

async function save() {
  if (!draft.value) return
  saveError.value = ''
  showValidation.value = true
  if (!editorValidation.value.valid) return
  saving.value = true
  try {
    if (editingId.value) {
      await opsAPI.updateAlertRule(editingId.value, draft.value)
    } else {
      await opsAPI.createAlertRule(draft.value)
    }
    showEditor.value = false
    draft.value = null
    editingId.value = null
    await load()
  } catch (err: unknown) {
    saveError.value = extractApiErrorMessage(err, t('admin.ops.alertRules.saveFailed'))
    console.error('[OpsAlertRulesCard] Failed to save rule', err)
  } finally {
    saving.value = false
  }
}

const showDeleteConfirm = ref(false)
const pendingDelete = ref<AlertRule | null>(null)
// 删除失败时确认弹窗不关，原因写在弹窗里
const deleteError = ref('')

function requestDelete(rule: AlertRule) {
  pendingDelete.value = rule
  deleteError.value = ''
  showDeleteConfirm.value = true
}

async function confirmDelete() {
  if (!pendingDelete.value?.id) return
  deleteError.value = ''
  try {
    await opsAPI.deleteAlertRule(pendingDelete.value.id)
    showDeleteConfirm.value = false
    pendingDelete.value = null
    await load()
  } catch (err: unknown) {
    deleteError.value = extractApiErrorMessage(err, t('admin.ops.alertRules.deleteFailed'))
    console.error('[OpsAlertRulesCard] Failed to delete rule', err)
  }
}

function cancelDelete() {
  showDeleteConfirm.value = false
  pendingDelete.value = null
}
</script>

<template>
  <div class="rounded-lg bg-af-sheet p-6 ring-1 ring-af-hairline">
    <div class="mb-4 flex flex-wrap items-start justify-between gap-3 sm:gap-4">
      <div>
        <h3 class="text-sm font-bold text-af-ink">{{ t('admin.ops.alertRules.title') }}</h3>
        <p class="mt-1 text-xs text-af-ink-3">{{ t('admin.ops.alertRules.description') }}</p>
      </div>

      <div class="flex items-center gap-2">
        <button class="btn btn-sm btn-primary" :disabled="loading" @click="openCreate">
          {{ t('admin.ops.alertRules.create') }}
        </button>
        <button
          class="flex items-center gap-1.5 rounded-lg bg-af-sunken px-3 py-1.5 text-xs font-bold text-af-ink-2 transition-colors hover:bg-af-hairline disabled:cursor-not-allowed disabled:opacity-50"
          :disabled="loading"
          @click="load"
        >
          <svg class="h-3.5 w-3.5" :class="{ 'animate-spin': loading }" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          {{ t('common.refresh') }}
        </button>
      </div>
    </div>

    <div v-if="loading" class="py-10 text-center text-sm text-af-ink-3">
      {{ t('admin.ops.alertRules.loading') }}
    </div>

    <div v-else-if="sortedRules.length === 0" class="rounded-xl border border-dashed border-af-hairline p-8 text-center text-sm text-af-ink-3">
      {{ t('admin.ops.alertRules.empty') }}
    </div>

    <div v-else class="max-h-[520px] overflow-hidden rounded-xl border border-af-hairline">
      <div class="max-h-[520px] overflow-y-auto">
        <div v-if="!isDesktopViewport" class="divide-y divide-af-hairline">
          <div v-for="row in sortedRules" :key="row.id" class="space-y-2 p-4">
            <div class="flex items-start justify-between gap-2">
              <div class="min-w-0">
                <div class="text-xs font-bold text-af-ink">{{ row.name }}</div>
                <div v-if="row.description" class="mt-0.5 line-clamp-2 text-[11px] text-af-ink-3">
                  {{ row.description }}
                </div>
              </div>
              <span class="shrink-0 text-xs font-bold text-af-ink-2">{{ row.severity }}</span>
            </div>
            <div class="text-xs text-af-ink-2">
              <span class="font-mono">{{ row.metric_type }}</span>
              <span class="mx-1 text-af-ink-3">{{ row.operator }}</span>
              <span class="font-mono">{{ row.threshold }}</span>
            </div>
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs text-af-ink-2">
                {{ row.enabled ? t('common.enabled') : t('common.disabled') }}
              </span>
              <div class="flex items-center gap-2">
                <button class="btn btn-sm btn-secondary" @click="openEdit(row)">{{ t('common.edit') }}</button>
                <button class="btn btn-sm btn-danger" @click="requestDelete(row)">{{ t('common.delete') }}</button>
              </div>
            </div>
            <div v-if="row.updated_at" class="text-[10px] text-af-ink-3">
              {{ formatDateTime(row.updated_at) }}
            </div>
          </div>
        </div>
        <table v-else class="min-w-full divide-y divide-af-hairline">
          <thead class="sticky top-0 z-10 bg-af-sunken">
            <tr>
              <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-af-ink-3">
                {{ t('admin.ops.alertRules.table.name') }}
              </th>
              <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-af-ink-3">
                {{ t('admin.ops.alertRules.table.metric') }}
              </th>
              <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-af-ink-3">
                {{ t('admin.ops.alertRules.table.severity') }}
              </th>
              <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-af-ink-3">
                {{ t('admin.ops.alertRules.table.enabled') }}
              </th>
              <th class="px-4 py-3 text-right text-[11px] font-bold uppercase tracking-wider text-af-ink-3">
                {{ t('admin.ops.alertRules.table.actions') }}
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-af-hairline bg-af-sheet">
            <tr v-for="row in sortedRules" :key="row.id" class="hover:bg-af-sunken">
              <td class="px-4 py-3">
                <div class="text-xs font-bold text-af-ink">{{ row.name }}</div>
                <div v-if="row.description" class="mt-0.5 line-clamp-2 text-[11px] text-af-ink-3">
                  {{ row.description }}
                </div>
                <div v-if="row.updated_at" class="mt-1 text-[10px] text-af-ink-3">
                  {{ formatDateTime(row.updated_at) }}
                </div>
              </td>
              <td class="whitespace-nowrap px-4 py-3 text-xs text-af-ink-2">
                <span class="font-mono">{{ row.metric_type }}</span>
                <span class="mx-1 text-af-ink-3">{{ row.operator }}</span>
                <span class="font-mono">{{ row.threshold }}</span>
              </td>
              <td class="whitespace-nowrap px-4 py-3 text-xs font-bold text-af-ink-2">
                {{ row.severity }}
              </td>
              <td class="whitespace-nowrap px-4 py-3 text-xs text-af-ink-2">
                {{ row.enabled ? t('common.enabled') : t('common.disabled') }}
              </td>
              <td class="whitespace-nowrap px-4 py-3 text-right text-xs">
                <button class="btn btn-sm btn-secondary" @click="openEdit(row)">{{ t('common.edit') }}</button>
                <button class="ml-2 btn btn-sm btn-danger" @click="requestDelete(row)">{{ t('common.delete') }}</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <BaseDialog
      :show="showEditor"
      :title="editingId ? t('admin.ops.alertRules.editTitle') : t('admin.ops.alertRules.createTitle')"
      width="wide"
      @close="showEditor = false"
    >
      <div class="space-y-4">
        <div v-if="showValidation && !editorValidation.valid" class="rounded-xl bg-af-danger-tint p-4 text-xs text-af-danger">
          <div class="font-bold">{{ t('admin.ops.alertRules.validation.title') }}</div>
          <ul class="mt-1 list-disc pl-5">
            <li v-for="e in editorValidation.errors" :key="e">{{ e }}</li>
          </ul>
        </div>

        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div class="md:col-span-2">
            <label class="input-label">{{ t('admin.ops.alertRules.form.name') }}</label>
            <input v-model="draft!.name" class="input" type="text" @blur="showValidation = true" />
          </div>

          <div class="md:col-span-2">
            <label class="input-label">{{ t('admin.ops.alertRules.form.description') }}</label>
            <input v-model="draft!.description" class="input" type="text" />
          </div>

          <div>
            <label class="input-label">{{ t('admin.ops.alertRules.form.metric') }}</label>
            <Select v-model="draft!.metric_type" :options="metricOptions" />
            <div v-if="selectedMetricDefinition" class="mt-1 space-y-0.5 text-xs text-af-ink-3">
              <p>{{ selectedMetricDefinition.description }}</p>
              <p>
                {{
                  t('admin.ops.alertRules.hints.recommended', {
                    operator: selectedMetricDefinition.recommendedOperator,
                    threshold: selectedMetricDefinition.recommendedThreshold,
                    unit: selectedMetricDefinition.unit || ''
                  })
                }}
              </p>
            </div>
          </div>

          <div>
            <label class="input-label">{{ t('admin.ops.alertRules.form.operator') }}</label>
            <Select v-model="draft!.operator" :options="operatorOptions" />
          </div>

          <div>
            <label class="input-label">{{ t('admin.ops.alertRules.form.threshold') }}</label>
            <input v-model.number="draft!.threshold" class="input" type="number" />
          </div>

          <div>
            <label class="input-label">{{ t('admin.ops.alertRules.form.severity') }}</label>
            <Select v-model="draft!.severity" :options="severityOptions" />
          </div>

          <div>
            <label class="input-label">{{ t('admin.ops.alertRules.form.window') }}</label>
            <Select v-model="draft!.window_minutes" :options="windowOptions" />
          </div>

          <div>
            <label class="input-label">{{ t('admin.ops.alertRules.form.sustained') }}</label>
            <input v-model.number="draft!.sustained_minutes" class="input" type="number" min="1" max="1440" />
          </div>

          <div>
            <label class="input-label">{{ t('admin.ops.alertRules.form.cooldown') }}</label>
            <input v-model.number="draft!.cooldown_minutes" class="input" type="number" min="0" max="1440" />
          </div>

          <div class="flex items-center justify-between rounded-xl bg-af-sunken px-4 py-3 md:col-span-2">
            <span class="text-xs font-bold text-af-ink-2">{{ t('admin.ops.alertRules.form.enabled') }}</span>
            <input v-model="draft!.enabled" type="checkbox" class="h-4 w-4 rounded border-af-hairline-strong text-af-brand focus:ring-af-brand" />
          </div>

          <div class="flex items-center justify-between rounded-xl bg-af-sunken px-4 py-3 md:col-span-2">
            <span class="text-xs font-bold text-af-ink-2">{{ t('admin.ops.alertRules.form.notifyEmail') }}</span>
            <input v-model="draft!.notify_email" type="checkbox" class="h-4 w-4 rounded border-af-hairline-strong text-af-brand focus:ring-af-brand" />
          </div>
        </div>
      </div>

      <template #footer>
        <div class="flex w-full flex-wrap items-center justify-end gap-2">
          <FormError class="mr-auto min-w-0 flex-1" :message="saveError" />
          <button class="btn btn-secondary" :disabled="saving" @click="showEditor = false">
            {{ t('common.cancel') }}
          </button>
          <button class="btn btn-primary" :disabled="saving" @click="save">
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="showDeleteConfirm"
      :title="t('admin.ops.alertRules.deleteConfirmTitle')"
      :message="t('admin.ops.alertRules.deleteConfirmMessage')"
      :confirmText="t('common.delete')"
      :cancelText="t('common.cancel')"
      @confirm="confirmDelete"
      @cancel="cancelDelete"
    >
      <FormError :message="deleteError" />
    </ConfirmDialog>
  </div>
</template>
