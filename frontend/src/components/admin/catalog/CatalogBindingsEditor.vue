<template>
  <!-- 绑定资源：搜索账号 → 添加；每条一个可空优先级。工作副本由父组件持有（v-model），保存时整份覆盖。 -->
  <div class="space-y-3">
    <div>
      <div class="text-sm font-medium text-af-ink">{{ t('admin.modelCatalog.bindings.title') }}</div>
      <p class="mt-1 text-xs text-af-ink-3">{{ t('admin.modelCatalog.bindings.hint') }}</p>
    </div>
    <div>
      <input
        v-model="resourceQuery"
        type="text"
        class="input"
        :placeholder="t('admin.modelCatalog.bindings.search')"
        data-testid="model-catalog-resource-search"
        @input="scheduleResourceSearch"
      />
      <ul
        v-if="resourceResults.length > 0"
        class="mt-2 max-h-48 divide-y divide-af-hairline overflow-y-auto rounded-md border border-af-hairline"
        data-testid="model-catalog-resource-results"
      >
        <li v-for="account in resourceResults" :key="account.id" class="flex items-center justify-between gap-3 px-3 py-2 text-sm">
          <span class="flex min-w-0 items-center gap-2">
            <PlatformTypeBadge :platform="account.platform" :type="account.type" :vendor="account.vendor" />
            <span class="truncate">{{ account.name }}</span>
          </span>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="isBound(account.id)"
            data-testid="model-catalog-resource-add"
            @click="addBinding(account)"
          >
            {{ t('admin.modelCatalog.bindings.add') }}
          </button>
        </li>
      </ul>
      <p v-else-if="resourceSearched" class="mt-2 text-xs text-af-ink-3">
        {{ t('admin.modelCatalog.bindings.noResults') }}
      </p>
    </div>
    <ul v-if="modelValue.length > 0" class="divide-y divide-af-hairline rounded-md border border-af-hairline" data-testid="model-catalog-bindings">
      <li v-for="binding in modelValue" :key="binding.account_id" class="flex flex-wrap items-center gap-3 px-3 py-2">
        <span class="flex min-w-0 flex-1 items-center gap-2 text-sm">
          <PlatformTypeBadge
            v-if="binding.account"
            :platform="badgePlatform(binding.account.platform)"
            :type="badgeType(binding.account.type)"
            :vendor="binding.account.vendor"
          />
          <span class="truncate">{{ binding.account?.name ?? `#${binding.account_id}` }}</span>
        </span>
        <label class="flex items-center gap-2 text-xs text-af-ink-3">
          {{ t('admin.modelCatalog.bindings.priority') }}
          <input
            :value="binding.priority"
            type="number"
            class="input w-24"
            data-testid="model-catalog-binding-priority"
            @input="setPriority(binding.account_id, ($event.target as HTMLInputElement).value)"
          />
        </label>
        <button
          type="button"
          class="btn btn-ghost btn-sm text-af-danger hover:text-af-danger"
          data-testid="model-catalog-binding-remove"
          @click="removeBinding(binding.account_id)"
        >
          {{ t('admin.modelCatalog.bindings.remove') }}
        </button>
      </li>
    </ul>
    <p v-else class="text-xs text-af-ink-3">{{ t('admin.modelCatalog.bindings.empty') }}</p>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { ModelCatalogBinding, ModelCatalogBindingAccount } from '@/api/admin/modelCatalog'
import type { AccountListItem, AccountPlatform, AccountType } from '@/types'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import { numberOrNull } from './entryRequest'

const props = defineProps<{
  modelValue: ModelCatalogBinding[]
  /** 新建条目时还没有 id，绑定先挂 0，保存后由父组件用真实 id 提交 */
  entryId: number | null
}>()
const emit = defineEmits<{ 'update:modelValue': [bindings: ModelCatalogBinding[]] }>()

const { t } = useI18n()
const appStore = useAppStore()

const resourceQuery = ref('')
const resourceResults = ref<AccountListItem[]>([])
const resourceSearched = ref(false)
let resourceSearchTimer: ReturnType<typeof setTimeout> | null = null

function isBound(accountId: number): boolean {
  return props.modelValue.some((binding) => binding.account_id === accountId)
}

// 绑定摘要里的 platform / type 是后端字符串，徽章按前端联合类型接收。
function badgePlatform(platform: string): AccountPlatform {
  return platform as AccountPlatform
}

function badgeType(type: string): AccountType {
  return type as AccountType
}

function toBindingAccount(account: AccountListItem): ModelCatalogBindingAccount {
  return {
    id: account.id,
    name: account.name,
    platform: account.platform,
    type: account.type,
    vendor: account.vendor ?? '',
    status: account.status
  }
}

function addBinding(account: AccountListItem) {
  if (isBound(account.id)) return
  emit('update:modelValue', [
    ...props.modelValue,
    { entry_id: props.entryId ?? 0, account_id: account.id, priority: null, account: toBindingAccount(account) }
  ])
}

function removeBinding(accountId: number) {
  emit('update:modelValue', props.modelValue.filter((binding) => binding.account_id !== accountId))
}

function setPriority(accountId: number, raw: string) {
  const priority = raw === '' ? null : numberOrNull(Number(raw))
  emit(
    'update:modelValue',
    props.modelValue.map((binding) => (binding.account_id === accountId ? { ...binding, priority } : binding))
  )
}

function scheduleResourceSearch() {
  if (resourceSearchTimer) clearTimeout(resourceSearchTimer)
  resourceSearchTimer = setTimeout(() => {
    resourceSearchTimer = null
    void searchResources()
  }, 300)
}

async function searchResources() {
  const query = resourceQuery.value.trim()
  if (!query) {
    resourceResults.value = []
    resourceSearched.value = false
    return
  }
  try {
    const result = await adminAPI.accounts.list(1, 20, { search: query, lite: 'true' })
    resourceResults.value = result.items ?? []
    resourceSearched.value = true
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.unknownError')))
  }
}

/** 父组件换条目时清空搜索状态 */
function reset() {
  resourceQuery.value = ''
  resourceResults.value = []
  resourceSearched.value = false
}
defineExpose({ reset })

onBeforeUnmount(() => {
  if (resourceSearchTimer) clearTimeout(resourceSearchTimer)
})
</script>
