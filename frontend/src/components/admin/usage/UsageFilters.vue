<template>
  <!--
    一行工具条（2026-10-05 走查：原来一排 192px 宽的大框像卡片，改成与用户站用量页同一套）：
    用户 / API 密钥 / 渠道是按名字搜索的小输入框（EntityPicker compact，界面不出现内部 id），模型与其余维度是筛选标签；
    低频维度（请求类型、计费…；错误页签是错误类型 / 分类 / 状态码）收在「更多筛选」后面，选了就一直露着，免得看不见的条件悄悄起作用。
    有条件时出现「清除筛选」；右端是调用方插槽（列设置）。刷新 / 导出 / 清理是页面级操作，在页头（A7）。
  -->
  <div :class="flat ? 'py-4' : 'card p-6'">
    <div class="flex flex-wrap items-center gap-2" data-testid="usage-filters">
      <EntityPicker
        ref="userPickerRef"
        kind="user"
        compact
        class="usage-filter-dropdown w-full sm:w-44"
        :title="t('admin.usage.userFilter')"
        :model-value="filters.user_id"
        @update:model-value="onUserChange"
      />
      <!-- 先选了用户时只列该用户的密钥 -->
      <EntityPicker
        ref="apiKeyPickerRef"
        kind="apiKey"
        compact
        class="usage-filter-dropdown w-full sm:w-44"
        :title="t('usage.apiKeyFilter')"
        :user-id="filters.user_id"
        :model-value="filters.api_key_id"
        @update:model-value="onApiKeyChange"
      />
      <EntityPicker
        ref="channelPickerRef"
        kind="channel"
        compact
        class="usage-filter-dropdown w-full sm:w-44"
        :title="t('admin.usage.account')"
        :model-value="filters.account_id"
        @update:model-value="onAccountChange"
      />
      <FilterChip v-model="modelChip" :label="t('usage.model')" :options="modelOptions" test-id="admin-usage-filter-model" @change="emitChange" />

      <template v-if="showMore">
        <template v-if="mode === 'errors'">
          <FilterChip v-model="errorPhaseChip" :label="t('admin.ops.errorLog.type')" :options="errorPhaseOptions" test-id="admin-usage-filter-phase" @change="emitChange" />
          <FilterChip v-model="errorCategoryChip" :label="t('usage.errors.category')" :options="errorCategoryOptions" test-id="admin-usage-filter-category" @change="emitChange" />
          <FilterChip v-model="statusCodeChip" :label="t('admin.ops.errorLog.status')" :options="statusCodeOptions" test-id="admin-usage-filter-status" @change="emitChange" />
        </template>
        <template v-else>
          <FilterChip v-model="requestTypeChip" :label="t('usage.type')" :options="requestTypeOptions" test-id="admin-usage-filter-type" @change="emitChange" />
          <FilterChip v-model="compactionChip" :label="t('usage.compactionFilter')" :options="compactionOptions" test-id="admin-usage-filter-compaction" @change="emitChange" />
          <!-- 计费类型只有「余额 / 订阅」两种，订阅隐藏期间这个筛选没有意义 -->
          <FilterChip
            v-if="SITE_FEATURES.subscription"
            v-model="billingTypeChip"
            :label="t('admin.usage.billingType')"
            :options="billingTypeOptions"
            test-id="admin-usage-filter-billing-type"
            @change="emitChange"
          />
          <FilterChip v-model="billingModeChip" :label="t('admin.usage.billingMode')" :options="billingModeOptions" test-id="admin-usage-filter-billing-mode" @change="emitChange" />
          <FilterChip v-model="upstreamMismatchChip" :label="t('admin.usage.upstreamModelAudit')" :options="upstreamModelMismatchOptions" test-id="admin-usage-filter-upstream" @change="emitChange" />
        </template>
      </template>
      <button
        v-else
        type="button"
        class="inline-flex h-8 items-center gap-1 rounded-full px-2 text-13 text-af-ink-3 transition-colors hover:text-af-ink"
        data-testid="usage-filter-more"
        @click="moreOpen = true"
      >
        <Icon name="plus" size="xs" :stroke-width="2" />
        {{ t('admin.usage.moreFilters') }}
      </button>
      <button
        v-if="showActions && anyActive"
        type="button"
        class="px-2 text-13 text-af-ink-3 transition-colors hover:text-af-ink"
        data-testid="usage-filters-clear"
        @click="$emit('reset')"
      >
        {{ t('admin.usage.clearFilters') }}
      </button>
      <div v-if="$slots['after-reset']" class="ml-auto flex items-center gap-0.5">
        <slot name="after-reset" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, toRef, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import FilterChip from '@/components/common/FilterChip.vue'
import type { FilterOption } from '@/components/common/types'
import Icon from '@/components/icons/Icon.vue'
import EntityPicker from '@/components/admin/form/EntityPicker.vue'
import { COMMON_ERROR_STATUS_CODES } from '@/utils/errorBadges'
import { SITE_FEATURES } from '@/utils/siteFeatures'

type ModelValue = Record<string, any>

interface Props {
  modelValue: ModelValue
  startDate: string
  endDate: string
  showActions?: boolean
  modelOptions?: string[]
  /**
   * usage 模式:明细页签的全部条件
   * errors 模式:隐藏用量专属字段,显示错误类型 / 分类 / 状态码(错误页签用)
   */
  mode?: 'usage' | 'errors'
  /** 嵌入页面内使用：去掉自身卡片外观 */
  flat?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  showActions: true,
  mode: 'usage',
  flat: false
})
const emit = defineEmits([
  'update:modelValue',
  'change',
  'reset'
])

const { t } = useI18n()
const filters = toRef(props, 'modelValue')

const userPickerRef = ref<InstanceType<typeof EntityPicker> | null>(null)
const apiKeyPickerRef = ref<InstanceType<typeof EntityPicker> | null>(null)
const channelPickerRef = ref<InstanceType<typeof EntityPicker> | null>(null)

const modelOptions = computed<FilterOption[]>(() => (props.modelOptions ?? []).map((m) => ({ value: m, label: m })))

const requestTypeOptions = computed<FilterOption[]>(() => [
  { value: 'ws_v2', label: t('usage.ws') },
  { value: 'live', label: t('usage.live') },
  { value: 'stream', label: t('usage.stream') },
  { value: 'sync', label: t('usage.sync') },
  { value: 'cyber', label: t('usage.cyber') }
])

// 布尔维度在标签里用字符串值，读写时再换回布尔（见下面的 chip 适配）
const compactionOptions = computed<FilterOption[]>(() => [{ value: 'only', label: t('usage.compactionOnly') }])

const billingTypeOptions = computed<FilterOption[]>(() => [
  { value: 0, label: t('admin.usage.billingTypeBalance') },
  { value: 1, label: t('admin.usage.billingTypeSubscription') }
])

// 错误类型对应后端 phase 参数(与错误表"类型"徽章同语义)
const errorPhaseOptions = computed<FilterOption[]>(() => [
  { value: 'upstream', label: t('admin.ops.errorLog.typeUpstream') },
  { value: 'account_auth', label: t('admin.ops.errorLog.typeAccountAuth') },
  { value: 'request', label: t('admin.ops.errorLog.typeRequest') },
  { value: 'auth', label: t('admin.ops.errorLog.typeAuth') },
  { value: 'routing', label: t('admin.ops.errorLog.typeRouting') },
  { value: 'internal', label: t('admin.ops.errorLog.typeInternal') },
])

// 分类码同用户端 /usage 错误筛选;"other" 无法反查为过滤条件,刻意不列
const errorCategoryCodes = ['auth', 'rate_limit', 'quota', 'invalid_request', 'service_unavailable', 'server', 'internal', 'cyber']

const errorCategoryOptions = computed<FilterOption[]>(() =>
  errorCategoryCodes.map((c) => ({ value: c, label: t('usage.errors.categories.' + c) }))
)

const statusCodeOptions = computed<FilterOption[]>(() => COMMON_ERROR_STATUS_CODES.map((c) => ({ value: c, label: String(c) })))

const billingModeOptions = computed<FilterOption[]>(() => [
  { value: 'token', label: t('admin.usage.billingModeToken') },
  { value: 'per_request', label: t('admin.usage.billingModePerRequest') },
  { value: 'image', label: t('admin.usage.billingModeImage') },
  { value: 'video', label: t('admin.usage.billingModeVideo') }
])

const upstreamModelMismatchOptions = computed<FilterOption[]>(() => [
  { value: 'mismatch', label: t('admin.usage.upstreamModelMismatchOnly') },
  { value: 'matched', label: t('admin.usage.upstreamModelMatchedOnly') }
])

const emitChange = () => emit('change')

const isSet = (value: unknown) => value !== null && value !== undefined && value !== ''

/** 筛选标签的值：空串 = 全部；写回时空串还原成 null，其余按维度换回原类型 */
const chip = (key: string, toFilter: (value: string | number) => unknown = (value) => value, fromFilter: (value: any) => string | number = (value) => value) =>
  computed<string | number>({
    get: () => (isSet(filters.value[key]) ? fromFilter(filters.value[key]) : ''),
    set: (value) => {
      filters.value[key] = value === '' ? null : toFilter(value)
    },
  })
const modelChip = chip('model', String)
const requestTypeChip = chip('request_type', String)
const compactionChip = chip('native_compaction_v2', () => true, () => 'only')
const billingTypeChip = chip('billing_type', Number)
const billingModeChip = chip('billing_mode', String)
const upstreamMismatchChip = chip('upstream_model_mismatch', (value) => value === 'mismatch', (value) => (value ? 'mismatch' : 'matched'))
const errorPhaseChip = chip('error_phase', String)
const errorCategoryChip = chip('error_category', String)
const statusCodeChip = chip('status_code', Number)

// 「更多筛选」：默认收起；里面有条件生效时一直展开
const moreOpen = ref(false)
const MORE_FILTER_KEYS: Record<'usage' | 'errors', string[]> = {
  usage: ['request_type', 'native_compaction_v2', 'billing_type', 'billing_mode', 'upstream_model_mismatch'],
  errors: ['error_phase', 'error_category', 'status_code'],
}
const moreActive = computed(() => MORE_FILTER_KEYS[props.mode].some((key) => isSet(filters.value[key])))
const showMore = computed(() => moreOpen.value || moreActive.value)
const anyActive = computed(() => moreActive.value || ['user_id', 'api_key_id', 'model', 'account_id'].some((key) => isSet(filters.value[key])))

// 换了用户：原来选的密钥不一定属于新用户，一并清掉
const onUserChange = (userId: number | undefined) => {
  filters.value.user_id = userId
  filters.value.api_key_id = undefined
  emitChange()
}

const onApiKeyChange = (apiKeyId: number | undefined) => {
  filters.value.api_key_id = apiKeyId
  emitChange()
}

const onAccountChange = (accountId: number | undefined) => {
  filters.value.account_id = accountId
  emitChange()
}

watch(
  () => props.startDate,
  (value) => {
    filters.value.start_date = value
  },
  { immediate: true }
)

watch(
  () => props.endDate,
  (value) => {
    filters.value.end_date = value
  },
  { immediate: true }
)

// 供外部(如路由带进来的 user_id)在程序化设置 user_id 后回显选中的用户邮箱
const setUserKeyword = (email: string) => userPickerRef.value?.setKeyword(email)

const getUserSearchRevision = () => userPickerRef.value?.getRevision() ?? 0

interface UsageFilterCondition {
  label: string
  value: string
}

const optionLabel = (options: FilterOption[], value: string | number) =>
  options.find((option) => option.value === value)?.label ?? String(value)

/**
 * 明细页签当前生效的条件（不含时间范围），按界面上的名字写：清理弹窗据此告诉管理员要删的是哪批（2026-10-04 D8）。
 * 用户 / 密钥 / 渠道写选中项的名字，不写内部 id。
 */
const describeUsageConditions = (): UsageFilterCondition[] => {
  const f = filters.value
  const conditions: UsageFilterCondition[] = []
  const add = (label: string, value: string) => conditions.push({ label, value })
  if (isSet(f.user_id)) add(t('admin.usage.userFilter'), userPickerRef.value?.getSelectedLabel() || '—')
  if (isSet(f.api_key_id)) add(t('usage.apiKeyFilter'), apiKeyPickerRef.value?.getSelectedLabel() || '—')
  if (isSet(f.model)) add(t('usage.model'), f.model)
  if (isSet(f.account_id)) add(t('admin.usage.account'), channelPickerRef.value?.getSelectedLabel() || '—')
  if (isSet(f.request_type)) add(t('usage.type'), optionLabel(requestTypeOptions.value, requestTypeChip.value))
  if (isSet(f.native_compaction_v2)) add(t('usage.compactionFilter'), optionLabel(compactionOptions.value, compactionChip.value))
  if (isSet(f.billing_type)) add(t('admin.usage.billingType'), optionLabel(billingTypeOptions.value, billingTypeChip.value))
  if (isSet(f.billing_mode)) add(t('admin.usage.billingMode'), optionLabel(billingModeOptions.value, billingModeChip.value))
  if (isSet(f.upstream_model_mismatch)) add(t('admin.usage.upstreamModelAudit'), optionLabel(upstreamModelMismatchOptions.value, upstreamMismatchChip.value))
  return conditions
}

defineExpose({ getUserSearchRevision, setUserKeyword, describeUsageConditions })
</script>
