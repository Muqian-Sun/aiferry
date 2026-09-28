<template>
  <!--
    承接的渠道（muqian 2026-09-25：模型页添加表单改成能直接浏览、勾选的渠道列表，不再先搜再加）。
    列出全部渠道，已绑定的排在前面；勾上即绑定，绑定行可填优先级（留空跟随渠道自身）。工作副本由父组件持有，保存时整份覆盖。
  -->
  <div data-testid="catalog-channel-picker">
    <div class="flex flex-wrap items-baseline justify-between gap-2">
      <div class="text-sm font-medium text-af-ink">{{ t('admin.modelCatalog.bindings.title') }}</div>
      <span class="text-xs text-af-ink-3">{{ t('admin.modelCatalog.bindings.selected', { count: modelValue.length }) }}</span>
    </div>
    <p class="mt-1 text-xs text-af-ink-3">{{ t('admin.modelCatalog.bindings.hint') }}</p>

    <div class="mt-3 flex flex-col gap-2 sm:flex-row sm:items-center">
      <input
        v-model="query"
        type="search"
        class="input sm:flex-1"
        :placeholder="t('admin.modelCatalog.bindings.search')"
        data-testid="model-catalog-resource-search"
      />
      <label class="flex shrink-0 cursor-pointer items-center gap-2 text-sm text-af-ink-2">
        <input v-model="boundOnly" type="checkbox" class="rounded border-af-hairline-strong text-af-brand focus:ring-af-brand" />
        {{ t('admin.modelCatalog.bindings.boundOnly') }}
      </label>
    </div>

    <div class="mt-2 max-h-96 overflow-y-auto rounded-lg border border-af-hairline">
      <p v-if="loading" class="px-3 py-4 text-sm text-af-ink-3">{{ t('admin.modelCatalog.bindings.loading') }}</p>
      <div v-else-if="loadFailed" class="flex items-center justify-between gap-2 px-3 py-4 text-sm text-af-warning">
        <span>{{ t('admin.modelCatalog.bindings.loadFailed') }}</span>
        <button type="button" class="btn btn-secondary btn-sm" @click="load">{{ t('admin.modelCatalog.bindings.retry') }}</button>
      </div>
      <p v-else-if="rows.length === 0" class="px-3 py-4 text-sm text-af-ink-3">
        {{ query || boundOnly ? t('admin.modelCatalog.bindings.noResults') : t('admin.modelCatalog.bindings.noChannels') }}
      </p>
      <ul v-else class="divide-y divide-af-hairline" data-testid="model-catalog-bindings">
        <li v-for="row in rows" :key="row.id" class="flex flex-wrap items-center gap-3 px-3 py-2">
          <label class="flex min-w-0 flex-1 cursor-pointer items-center gap-3">
            <input
              type="checkbox"
              class="rounded border-af-hairline-strong text-af-brand focus:ring-af-brand"
              :checked="boundIds.has(row.id)"
              :data-testid="`model-catalog-channel-${row.id}`"
              @change="toggle(row)"
            />
            <PlatformTypeBadge :platform="badgePlatform(row.platform)" :type="badgeType(row.type)" :vendor="row.vendor" />
            <span class="min-w-0 truncate text-sm text-af-ink">{{ row.name }}</span>
            <span v-if="row.status !== 'active'" class="shrink-0 text-xs text-af-ink-3">{{ t('admin.modelCatalog.bindings.inactive') }}</span>
          </label>
          <label v-if="boundIds.has(row.id)" class="flex items-center gap-2 text-xs text-af-ink-3">
            {{ t('admin.modelCatalog.bindings.priority') }}
            <input
              :value="priorityOf(row.id)"
              type="number"
              class="input w-24"
              :placeholder="t('admin.modelCatalog.bindings.priorityFollow')"
              data-testid="model-catalog-binding-priority"
              @input="setPriority(row.id, ($event.target as HTMLInputElement).value)"
            />
          </label>
        </li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { ModelCatalogBinding, ModelCatalogBindingAccount } from '@/api/admin/modelCatalog'
import type { AccountPlatform, AccountType } from '@/types'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import { numberOrNull } from './entryRequest'

const props = defineProps<{
  modelValue: ModelCatalogBinding[]
  /** 新建条目时还没有 id，绑定先挂 0，保存后由父组件用真实 id 提交 */
  entryId: number | null
}>()
const emit = defineEmits<{ 'update:modelValue': [bindings: ModelCatalogBinding[]] }>()

const { t } = useI18n()

const channels = ref<ModelCatalogBindingAccount[]>([])
const loading = ref(false)
const loadFailed = ref(false)
const query = ref('')
const boundOnly = ref(false)

// 全部渠道按页取完（后端单页上限 1000）；取不全就算失败，不静默少列
const PAGE_SIZE = 500
async function load() {
  loading.value = true
  loadFailed.value = false
  try {
    const all: ModelCatalogBindingAccount[] = []
    for (let page = 1; ; page++) {
      const result = await adminAPI.accounts.list(page, PAGE_SIZE, { lite: 'true' })
      const items = result.items ?? []
      for (const account of items) {
        all.push({
          id: account.id,
          name: account.name,
          platform: account.platform,
          type: account.type,
          vendor: account.vendor ?? '',
          status: account.status
        })
      }
      if (items.length < PAGE_SIZE || all.length >= result.total) break
    }
    channels.value = all
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

const boundIds = computed(() => new Set(props.modelValue.map((binding) => binding.account_id)))

// 已绑定但不在列表里的渠道（已被删除）也要列出来，否则取消不了；没有名字就写「已删除渠道」
const rows = computed<ModelCatalogBindingAccount[]>(() => {
  const byId = new Map(channels.value.map((channel) => [channel.id, channel]))
  for (const binding of props.modelValue) {
    if (!byId.has(binding.account_id)) {
      byId.set(binding.account_id, binding.account ?? {
        id: binding.account_id,
        name: t('common.deletedChannel'),
        platform: '',
        type: '',
        vendor: '',
        status: ''
      })
    }
  }
  const q = query.value.trim().toLowerCase()
  return [...byId.values()]
    .filter((row) => !boundOnly.value || boundIds.value.has(row.id))
    .filter((row) => !q || row.name.toLowerCase().includes(q))
    .sort((a, b) => Number(boundIds.value.has(b.id)) - Number(boundIds.value.has(a.id)) || a.name.localeCompare(b.name))
})

// 绑定摘要里的 platform / type 是后端字符串，徽章按前端联合类型接收。
function badgePlatform(platform: string): AccountPlatform {
  return platform as AccountPlatform
}

function badgeType(type: string): AccountType {
  return type as AccountType
}

function priorityOf(accountId: number): number | null {
  return props.modelValue.find((binding) => binding.account_id === accountId)?.priority ?? null
}

function toggle(row: ModelCatalogBindingAccount) {
  if (boundIds.value.has(row.id)) {
    emit('update:modelValue', props.modelValue.filter((binding) => binding.account_id !== row.id))
    return
  }
  emit('update:modelValue', [
    ...props.modelValue,
    { entry_id: props.entryId ?? 0, account_id: row.id, priority: null, account: row }
  ])
}

function setPriority(accountId: number, raw: string) {
  const priority = raw === '' ? null : numberOrNull(Number(raw))
  emit(
    'update:modelValue',
    props.modelValue.map((binding) => (binding.account_id === accountId ? { ...binding, priority } : binding))
  )
}

onMounted(load)
</script>
