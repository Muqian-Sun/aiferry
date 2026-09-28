<template>
  <!--
    按名字选用户 / 密钥 / 渠道（管理站不让站长手填内部 id）。从用量页筛选（UsageFilters）抽出来，交互与接口不变：
    输入后 300ms 搜索（用户按邮箱、密钥按名称、渠道按名称），下拉点选，✕ 清除；v-model 是选中对象的 id，界面上不出现 id。
    下拉里同名密钥写所属用户邮箱、渠道写厂商，用来区分。
  -->
  <div ref="rootRef" class="relative">
    <input
      v-model="keyword"
      type="text"
      class="input pr-8"
      autocomplete="off"
      :placeholder="placeholderText"
      :aria-label="placeholderText"
      @input="onInput"
      @focus="onFocus"
    />
    <button
      v-if="modelValue"
      type="button"
      class="absolute right-2 top-1/2 -translate-y-1/2 text-af-ink-3 transition-colors hover:text-af-ink"
      :title="t('common.clear')"
      :aria-label="t('common.clear')"
      @click="clear"
    >
      ✕
    </button>
    <div
      v-if="open && (results.length > 0 || searched)"
      class="absolute z-50 mt-1 max-h-60 w-full overflow-auto rounded-lg border border-af-hairline bg-af-sheet shadow-lg"
    >
      <button
        v-for="option in results"
        :key="option.id"
        type="button"
        class="block w-full px-4 py-2 text-left hover:bg-af-sunken"
        @click="select(option)"
      >
        <span class="block truncate text-sm text-af-ink">{{ option.label }}<span v-if="option.badge" class="ml-1 text-xs text-af-ink-3">（{{ option.badge }}）</span></span>
        <span v-if="option.hint" class="block truncate text-xs text-af-ink-3">{{ option.hint }}</span>
      </button>
      <p v-if="results.length === 0" class="px-4 py-2 text-sm text-af-ink-3">{{ t('admin.entity.noMatch') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { platformLabel } from '@/utils/platformLabel'

interface PickerOption {
  id: number
  label: string
  /** 第二行灰字：密钥写所属用户邮箱，渠道写厂商 */
  hint?: string
  /** 名字后的括注：已删除用户 */
  badge?: string
}

const props = withDefaults(defineProps<{
  kind: 'user' | 'apiKey' | 'channel'
  modelValue?: number | null
  /** 只对密钥：限定在这个用户名下（先选了用户时），此时下拉不再重复写用户邮箱 */
  userId?: number | null
  placeholder?: string
}>(), {
  modelValue: undefined,
  userId: undefined,
  placeholder: ''
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: number | undefined): void
}>()

const { t } = useI18n()

const rootRef = ref<HTMLElement | null>(null)
const keyword = ref('')
const results = ref<PickerOption[]>([])
const open = ref(false)
/** 当前关键字已经搜过一次（没结果时显示「没有匹配项」，而不是空框） */
const searched = ref(false)
let searchTimer: ReturnType<typeof setTimeout> | null = null
/** 每次输入 / 选中 / 清除都加一：晚到的旧搜索结果据此丢弃；外部回填标签时也据此判断用户是否已经动过输入框 */
let revision = 0

const placeholderText = computed(() => {
  if (props.placeholder) return props.placeholder
  if (props.kind === 'user') return t('admin.usage.searchUserPlaceholder')
  if (props.kind === 'apiKey') return t('admin.usage.searchApiKeyPlaceholder')
  return t('admin.usage.searchAccountPlaceholder')
})

// 密钥在没选用户时也能列（按名称或列出最近的），用户 / 渠道要有关键字才搜
const searchesWithoutKeyword = computed(() => props.kind === 'apiKey')

const cancelPendingSearch = () => {
  if (searchTimer) {
    clearTimeout(searchTimer)
    searchTimer = null
  }
  revision += 1
}

const fetchOptions = async (query: string): Promise<PickerOption[]> => {
  if (props.kind === 'user') {
    const users = await adminAPI.usage.searchUsers(query)
    return users
      .sort((a, b) => Number(a.deleted) - Number(b.deleted))
      .map((u) => ({ id: u.id, label: u.email, badge: u.deleted ? t('admin.usage.userDeletedBadge') : undefined }))
  }
  if (props.kind === 'apiKey') {
    const keys = await adminAPI.usage.searchApiKeys(props.userId ?? undefined, query)
    return keys.map((k) => ({ id: k.id, label: k.name, hint: props.userId ? undefined : k.user_email }))
  }
  const res = await adminAPI.accounts.list(1, 20, { search: query })
  return res.items.map((a) => ({ id: a.id, label: a.name, hint: platformLabel(a.platform) }))
}

const scheduleSearch = () => {
  cancelPendingSearch()
  searched.value = false
  const query = keyword.value.trim()
  if (!query && !searchesWithoutKeyword.value) {
    results.value = []
    return
  }
  const current = revision
  searchTimer = setTimeout(async () => {
    searchTimer = null
    try {
      const options = await fetchOptions(query)
      if (current !== revision) return
      results.value = options
    } catch {
      if (current !== revision) return
      results.value = []
    }
    searched.value = true
  }, 300)
}

const onInput = () => {
  open.value = true
  scheduleSearch()
}

const onFocus = () => {
  open.value = true
  if (searchesWithoutKeyword.value && results.value.length === 0 && !searchTimer) scheduleSearch()
}

const select = (option: PickerOption) => {
  cancelPendingSearch()
  keyword.value = option.label
  open.value = false
  emit('update:modelValue', option.id)
}

const resetInput = () => {
  cancelPendingSearch()
  keyword.value = ''
  results.value = []
  searched.value = false
  open.value = false
}

const clear = () => {
  resetInput()
  emit('update:modelValue', undefined)
}

// 外部清掉了值（重置筛选等）：输入框跟着清空
watch(
  () => props.modelValue,
  (value) => {
    if (!value) resetInput()
  }
)

// 密钥：换了所属用户，旧的候选作废
watch(
  () => props.userId,
  () => {
    if (props.kind !== 'apiKey') return
    cancelPendingSearch()
    results.value = []
    searched.value = false
  }
)

const onDocumentClick = (e: MouseEvent) => {
  const target = e.target as Node | null
  if (target && !rootRef.value?.contains(target)) open.value = false
}

onMounted(() => document.addEventListener('click', onDocumentClick))
onUnmounted(() => {
  cancelPendingSearch()
  document.removeEventListener('click', onDocumentClick)
})

/** 调用方用 id 程序化选中（如路由带进来的用户）后，回填显示的名字 */
const setKeyword = (label: string) => {
  cancelPendingSearch()
  keyword.value = label
  results.value = []
  searched.value = false
  open.value = false
}

const getRevision = () => revision

defineExpose({ setKeyword, getRevision })
</script>
