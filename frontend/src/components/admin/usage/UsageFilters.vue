<template>
  <div :class="flat ? 'py-4' : 'card p-6'">
    <!--
      第一行只露四个：用户、API 密钥、模型、渠道；其余（请求类型、计费…；错误页签是错误类型 / 分类 / 状态码）
      收在「更多筛选」里，点开才出现，收起时按钮上写着其中生效了几个。清理弹窗要把删除范围全摆出来，不收。
      右：重置 + 调用方插槽（列设置）。刷新 / 导出 / 清理是页面级操作，在页头（A7）。
      控件上方不写标签（A8，与用户站用量页一致）：标签文字进 title；没选时占位写「全部 xx」——值是 undefined 时
      下拉匹配不到 null 那一项，不给占位会显示「请选择」。
    -->
    <div class="flex flex-wrap items-center justify-between gap-3">
      <!-- Left: filters (allowed to wrap to multiple rows) -->
      <div class="flex flex-1 flex-wrap items-center gap-2">
        <!-- User Search -->
        <div ref="userSearchRef" class="usage-filter-dropdown relative w-full sm:w-48" :title="t('admin.usage.userFilter')">
          <input
            v-model="userKeyword"
            type="text"
            class="input pr-8"
            :placeholder="t('admin.usage.searchUserPlaceholder')"
            @input="debounceUserSearch"
            @focus="showUserDropdown = true"
          />
          <button
            v-if="filters.user_id"
            type="button"
            @click="clearUser"
            class="absolute right-2 top-1/2 -translate-y-1/2 text-af-ink-3"
            aria-label="Clear user filter"
          >
            ✕
          </button>
          <div
            v-if="showUserDropdown && (userResults.length > 0 || userKeyword)"
            class="absolute z-50 mt-1 max-h-60 w-full overflow-auto rounded-lg border bg-af-sheet shadow-lg"
          >
            <button
              v-for="u in userResults"
              :key="u.id"
              type="button"
              @click="selectUser(u)"
              class="w-full px-4 py-2 text-left hover:bg-af-sunken"
            >
              <span>{{ u.email }}<span v-if="u.deleted" class="ml-1 text-xs text-af-ink-3">（{{ t('admin.usage.userDeletedBadge') }}）</span></span>
              <span class="ml-2 text-xs text-af-ink-3">#{{ u.id }}</span>
            </button>
          </div>
        </div>

        <!-- API Key Search -->
        <div ref="apiKeySearchRef" class="usage-filter-dropdown relative w-full sm:w-48" :title="t('usage.apiKeyFilter')">
          <input
            v-model="apiKeyKeyword"
            type="text"
            class="input pr-8"
            :placeholder="t('admin.usage.searchApiKeyPlaceholder')"
            @input="debounceApiKeySearch"
            @focus="onApiKeyFocus"
          />
          <button
            v-if="filters.api_key_id"
            type="button"
            @click="onClearApiKey"
            class="absolute right-2 top-1/2 -translate-y-1/2 text-af-ink-3"
            aria-label="Clear API key filter"
          >
            ✕
          </button>
          <div
            v-if="showApiKeyDropdown && apiKeyResults.length > 0"
            class="absolute z-50 mt-1 max-h-60 w-full overflow-auto rounded-lg border bg-af-sheet shadow-lg"
          >
            <button
              v-for="k in apiKeyResults"
              :key="k.id"
              type="button"
              @click="selectApiKey(k)"
              class="w-full px-4 py-2 text-left hover:bg-af-sunken"
            >
              <span class="truncate">{{ k.name || `#${k.id}` }}</span>
              <span class="ml-2 text-xs text-af-ink-3">#{{ k.id }}</span>
            </button>
          </div>
        </div>

        <!-- Model Filter -->
        <div class="w-full sm:w-48" :title="t('usage.model')">
          <Select v-model="filters.model" :options="modelOptions" :placeholder="t('admin.usage.allModels')" searchable @change="emitChange" />
        </div>

        <!-- Channel Filter -->
        <div ref="accountSearchRef" class="usage-filter-dropdown relative w-full sm:w-48" :title="t('admin.usage.account')">
          <input
            v-model="accountKeyword"
            type="text"
            class="input pr-8"
            :placeholder="t('admin.usage.searchAccountPlaceholder')"
            @input="debounceAccountSearch"
            @focus="showAccountDropdown = true"
          />
          <button
            v-if="filters.account_id"
            type="button"
            @click="clearAccount"
            class="absolute right-2 top-1/2 -translate-y-1/2 text-af-ink-3"
            aria-label="Clear account filter"
          >
            ✕
          </button>
          <div
            v-if="showAccountDropdown && (accountResults.length > 0 || accountKeyword)"
            class="absolute z-50 mt-1 max-h-60 w-full overflow-auto rounded-lg border bg-af-sheet shadow-lg"
          >
            <button
              v-for="a in accountResults"
              :key="a.id"
              type="button"
              @click="selectAccount(a)"
              class="w-full px-4 py-2 text-left hover:bg-af-sunken"
            >
              <span class="truncate">{{ a.name }}</span>
              <span class="ml-2 text-xs text-af-ink-3">#{{ a.id }}</span>
            </button>
          </div>
        </div>

        <button
          v-if="mode !== 'cleanup'"
          type="button"
          class="inline-flex h-8 items-center gap-1 rounded-full px-2.5 text-13 transition-colors hover:bg-af-sunken hover:text-af-ink"
          :class="moreOpen || activeMoreCount > 0 ? 'text-af-ink' : 'text-af-ink-3'"
          :aria-expanded="moreOpen"
          data-testid="usage-filter-more"
          @click="moreOpen = !moreOpen"
        >
          <Icon :name="moreOpen ? 'chevronUp' : 'plus'" size="xs" :stroke-width="2" />
          {{ activeMoreCount > 0 ? t('admin.usage.moreFiltersActive', { count: activeMoreCount }) : t('admin.usage.moreFilters') }}
        </button>
      </div>

      <!-- Right: actions -->
      <div v-if="showActions" class="flex w-full flex-wrap items-center justify-end gap-3 sm:w-auto">
        <button type="button" @click="$emit('reset')" class="btn btn-secondary">
          {{ t('common.reset') }}
        </button>
        <slot name="after-reset" />
      </div>
    </div>

    <!-- 更多筛选：点开才出现（清理弹窗一直摆着） -->
    <div v-show="mode === 'cleanup' || moreOpen" class="mt-3 flex flex-wrap items-center gap-2" data-testid="usage-filter-more-row">
      <!-- Request Type Filter (usage only) -->
      <div v-if="mode !== 'errors'" class="w-full sm:w-40" :title="t('usage.type')">
        <Select v-model="filters.request_type" :options="requestTypeOptions" :placeholder="t('admin.usage.allTypes')" @change="emitChange" />
      </div>

      <!-- Native compaction is independent of the transport request type. -->
      <div v-if="mode === 'usage'" class="w-full sm:w-40" :title="t('usage.compactionFilter')">
        <Select v-model="filters.native_compaction_v2" :options="compactionOptions" :placeholder="t('usage.allCompactionTypes')" @change="emitChange" />
      </div>

      <!-- Billing Type Filter (usage only) -->
      <!-- 计费类型只有「余额 / 订阅」两种，订阅隐藏期间这个筛选没有意义 -->
      <div v-if="mode !== 'errors' && SITE_FEATURES.subscription" class="w-full sm:w-44" :title="t('admin.usage.billingType')">
        <Select v-model="filters.billing_type" :options="billingTypeOptions" :placeholder="t('admin.usage.allBillingTypes')" @change="emitChange" />
      </div>

      <!-- Billing Mode Filter (usage only) -->
      <div v-if="mode === 'usage'" class="w-full sm:w-44" :title="t('admin.usage.billingMode')">
        <Select v-model="filters.billing_mode" :options="billingModeOptions" :placeholder="t('admin.usage.allBillingModes')" @change="emitChange" />
      </div>

      <div v-if="mode === 'usage'" class="w-full sm:w-52" :title="t('admin.usage.upstreamModelAudit')">
        <Select v-model="filters.upstream_model_mismatch" :options="upstreamModelMismatchOptions" :placeholder="t('admin.usage.allUpstreamModelAudit')" @change="emitChange" />
      </div>

      <!-- Error Phase Filter (errors only) -->
      <div v-if="mode === 'errors'" class="w-full sm:w-40" :title="t('admin.ops.errorLog.type')">
        <Select v-model="filters.error_phase" :options="errorPhaseOptions" :placeholder="t('admin.usage.allTypes')" @change="emitChange" />
      </div>

      <!-- Error Category Filter (errors only) -->
      <div v-if="mode === 'errors'" class="w-full sm:w-40" :title="t('usage.errors.category')">
        <Select v-model="filters.error_category" :options="errorCategoryOptions" :placeholder="t('usage.errors.allCategories')" @change="emitChange" />
      </div>

      <!-- Status Code Filter (errors only) -->
      <div v-if="mode === 'errors'" class="w-full sm:w-40" :title="t('admin.ops.errorLog.status')">
        <Select v-model="filters.status_code" :options="statusCodeOptions" :placeholder="t('usage.errors.allStatuses')" @change="emitChange" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, toRef, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { COMMON_ERROR_STATUS_CODES } from '@/utils/errorBadges'
import { SITE_FEATURES } from '@/utils/siteFeatures'
import type { SimpleApiKey, SimpleUser } from '@/api/admin/usage'

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
   * cleanup 模式:只留清理接口认的条件(用户 / Key / 模型 / 渠道 / 请求类型 / 计费类型),弹窗里显示的就是要删的范围
   */
  mode?: 'usage' | 'errors' | 'cleanup'
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

const userSearchRef = ref<HTMLElement | null>(null)
const apiKeySearchRef = ref<HTMLElement | null>(null)
const accountSearchRef = ref<HTMLElement | null>(null)

const userKeyword = ref('')
const userResults = ref<SimpleUser[]>([])
const showUserDropdown = ref(false)
let userSearchTimeout: ReturnType<typeof setTimeout> | null = null
let userSearchSequence = 0

const apiKeyKeyword = ref('')
const apiKeyResults = ref<SimpleApiKey[]>([])
const showApiKeyDropdown = ref(false)
let apiKeySearchTimeout: ReturnType<typeof setTimeout> | null = null

interface SimpleAccount {
  id: number
  name: string
}
const accountKeyword = ref('')
const accountResults = ref<SimpleAccount[]>([])
const showAccountDropdown = ref(false)
let accountSearchTimeout: ReturnType<typeof setTimeout> | null = null

const modelOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.usage.allModels') },
  ...(props.modelOptions ?? []).map((m) => ({ value: m, label: m })),
])

const requestTypeOptions = ref<SelectOption[]>([
  { value: null, label: t('admin.usage.allTypes') },
  { value: 'ws_v2', label: t('usage.ws') },
  { value: 'live', label: t('usage.live') },
  { value: 'stream', label: t('usage.stream') },
  { value: 'sync', label: t('usage.sync') },
  { value: 'cyber', label: t('usage.cyber') }
])

const compactionOptions = ref<SelectOption[]>([
  { value: null, label: t('usage.allCompactionTypes') },
  { value: true, label: t('usage.compactionOnly') }
])

const billingTypeOptions = ref<SelectOption[]>([
  { value: null, label: t('admin.usage.allBillingTypes') },
  { value: 0, label: t('admin.usage.billingTypeBalance') },
  { value: 1, label: t('admin.usage.billingTypeSubscription') }
])

// 错误类型对应后端 phase 参数(与错误表"类型"徽章同语义)
const errorPhaseOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.usage.allTypes') },
  { value: 'upstream', label: t('admin.ops.errorLog.typeUpstream') },
  { value: 'account_auth', label: t('admin.ops.errorLog.typeAccountAuth') },
  { value: 'request', label: t('admin.ops.errorLog.typeRequest') },
  { value: 'auth', label: t('admin.ops.errorLog.typeAuth') },
  { value: 'routing', label: t('admin.ops.errorLog.typeRouting') },
  { value: 'internal', label: t('admin.ops.errorLog.typeInternal') },
])

// 分类码同用户端 /usage 错误筛选;"other" 无法反查为过滤条件,刻意不列
const errorCategoryCodes = ['auth', 'rate_limit', 'quota', 'invalid_request', 'service_unavailable', 'upstream', 'internal', 'cyber']

const errorCategoryOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('usage.errors.allCategories') },
  ...errorCategoryCodes.map((c) => ({ value: c, label: t('usage.errors.categories.' + c) })),
])

const statusCodeOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('usage.errors.allStatuses') },
  ...COMMON_ERROR_STATUS_CODES.map((c) => ({ value: c, label: String(c) })),
])

const billingModeOptions = ref<SelectOption[]>([
  { value: null, label: t('admin.usage.allBillingModes') },
  { value: 'token', label: t('admin.usage.billingModeToken') },
  { value: 'per_request', label: t('admin.usage.billingModePerRequest') },
  { value: 'image', label: t('admin.usage.billingModeImage') },
  { value: 'video', label: t('admin.usage.billingModeVideo') }
])

const upstreamModelMismatchOptions = ref<SelectOption[]>([
  { value: null, label: t('admin.usage.allUpstreamModelAudit') },
  { value: true, label: t('admin.usage.upstreamModelMismatchOnly') },
  { value: false, label: t('admin.usage.upstreamModelMatchedOnly') }
])

const emitChange = () => emit('change')

// 「更多筛选」：默认收起；收起时按钮上写着里面生效了几个，免得看不见的条件悄悄起作用
const moreOpen = ref(false)
const MORE_FILTER_KEYS: Record<'usage' | 'errors', string[]> = {
  usage: ['request_type', 'native_compaction_v2', 'billing_type', 'billing_mode', 'upstream_model_mismatch'],
  errors: ['error_phase', 'error_category', 'status_code'],
}
const activeMoreCount = computed(() => {
  if (props.mode === 'cleanup') return 0
  return MORE_FILTER_KEYS[props.mode].filter((key) => {
    const value = filters.value[key]
    return value !== null && value !== undefined && value !== ''
  }).length
})

const clearPendingUserSearch = () => {
  if (userSearchTimeout) {
    clearTimeout(userSearchTimeout)
    userSearchTimeout = null
  }
  userSearchSequence += 1
}

const debounceUserSearch = () => {
  clearPendingUserSearch()
  const query = userKeyword.value.trim()
  if (!query) {
    userResults.value = []
    return
  }

  const sequence = userSearchSequence
  userSearchTimeout = setTimeout(async () => {
    userSearchTimeout = null
    try {
      const results = await adminAPI.usage.searchUsers(query)
      if (sequence === userSearchSequence) {
        userResults.value = results.sort((a, b) => Number(a.deleted) - Number(b.deleted))
      }
    } catch {
      if (sequence === userSearchSequence) {
        userResults.value = []
      }
    }
  }, 300)
}

const debounceApiKeySearch = () => {
  if (apiKeySearchTimeout) clearTimeout(apiKeySearchTimeout)
  apiKeySearchTimeout = setTimeout(async () => {
    try {
      apiKeyResults.value = await adminAPI.usage.searchApiKeys(
        filters.value.user_id,
        apiKeyKeyword.value || ''
      )
    } catch {
      apiKeyResults.value = []
    }
  }, 300)
}

const selectUser = async (u: SimpleUser) => {
  clearPendingUserSearch()
  userKeyword.value = u.email
  showUserDropdown.value = false
  filters.value.user_id = u.id
  clearApiKey()

  // Auto-load API keys for this user
  try {
    apiKeyResults.value = await adminAPI.usage.searchApiKeys(u.id, '')
  } catch {
    apiKeyResults.value = []
  }

  emitChange()
}

const clearUser = () => {
  clearPendingUserSearch()
  userKeyword.value = ''
  userResults.value = []
  showUserDropdown.value = false
  filters.value.user_id = undefined
  clearApiKey()
  emitChange()
}

const selectApiKey = (k: SimpleApiKey) => {
  apiKeyKeyword.value = k.name || String(k.id)
  showApiKeyDropdown.value = false
  filters.value.api_key_id = k.id
  emitChange()
}

const clearApiKey = () => {
  apiKeyKeyword.value = ''
  apiKeyResults.value = []
  showApiKeyDropdown.value = false
  filters.value.api_key_id = undefined
}

const onClearApiKey = () => {
  clearApiKey()
  emitChange()
}

const debounceAccountSearch = () => {
  if (accountSearchTimeout) clearTimeout(accountSearchTimeout)
  accountSearchTimeout = setTimeout(async () => {
    if (!accountKeyword.value) {
      accountResults.value = []
      return
    }
    try {
      const res = await adminAPI.accounts.list(1, 20, { search: accountKeyword.value })
      accountResults.value = res.items.map((a) => ({ id: a.id, name: a.name }))
    } catch {
      accountResults.value = []
    }
  }, 300)
}

const selectAccount = (a: SimpleAccount) => {
  accountKeyword.value = a.name
  showAccountDropdown.value = false
  filters.value.account_id = a.id
  emitChange()
}

const clearAccount = () => {
  accountKeyword.value = ''
  accountResults.value = []
  showAccountDropdown.value = false
  filters.value.account_id = undefined
  emitChange()
}

const onApiKeyFocus = () => {
  showApiKeyDropdown.value = true
  // Trigger search if no results yet
  if (apiKeyResults.value.length === 0) {
    debounceApiKeySearch()
  }
}

const onDocumentClick = (e: MouseEvent) => {
  const target = e.target as Node | null
  if (!target) return

  const clickedInsideUser = userSearchRef.value?.contains(target) ?? false
  const clickedInsideApiKey = apiKeySearchRef.value?.contains(target) ?? false
  const clickedInsideAccount = accountSearchRef.value?.contains(target) ?? false

  if (!clickedInsideUser) showUserDropdown.value = false
  if (!clickedInsideApiKey) showApiKeyDropdown.value = false
  if (!clickedInsideAccount) showAccountDropdown.value = false
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

watch(
  () => filters.value.user_id,
  (userId) => {
    if (!userId) {
      clearPendingUserSearch()
      userKeyword.value = ''
      userResults.value = []
    }
  }
)

watch(
  () => filters.value.api_key_id,
  (apiKeyId) => {
    if (!apiKeyId) {
      apiKeyKeyword.value = ''
      apiKeyResults.value = []
    }
  }
)

watch(
  () => filters.value.account_id,
  (accountId) => {
    if (!accountId) {
      accountKeyword.value = ''
      accountResults.value = []
    }
  }
)

onMounted(() => {
  document.addEventListener('click', onDocumentClick)
})

onUnmounted(() => {
  clearPendingUserSearch()
  document.removeEventListener('click', onDocumentClick)
})

// 供外部(如用户排行下钻)在程序化设置 user_id 后回显选中的用户邮箱
const setUserKeyword = (email: string) => {
  clearPendingUserSearch()
  userKeyword.value = email
  userResults.value = []
  showUserDropdown.value = false
}

const getUserSearchRevision = () => userSearchSequence

defineExpose({ getUserSearchRevision, setUserKeyword })
</script>
