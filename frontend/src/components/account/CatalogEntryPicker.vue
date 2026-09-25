<template>
  <!--
    渠道承接的目录模型（muqian 2026-09-25：渠道表单里直接绑定模型）。按厂商族分组、可搜索；
    勾选结果是目录条目 ID，由表单在保存渠道后整份写入（PUT /admin/accounts/:id/catalog-entries）。
  -->
  <div data-testid="catalog-entry-picker">
    <div class="flex flex-wrap items-baseline justify-between gap-2">
      <label class="input-label mb-0">{{ t('admin.accounts.catalogEntries.title') }}</label>
      <span class="text-xs text-af-ink-3" data-testid="catalog-entry-picker-count">
        {{ t('admin.accounts.catalogEntries.selected', { count: modelValue.length }) }}
        <button
          v-if="modelValue.length > 0"
          type="button"
          class="ml-2 text-af-brand hover:text-af-brand-hover"
          @click="emit('update:modelValue', [])"
        >
          {{ t('admin.accounts.catalogEntries.clear') }}
        </button>
      </span>
    </div>
    <p class="input-hint mb-2">{{ t('admin.accounts.catalogEntries.hint') }}</p>

    <div class="mb-2 flex flex-col gap-2 sm:flex-row sm:items-center">
      <input
        v-model="query"
        type="search"
        class="input sm:flex-1"
        :placeholder="t('admin.accounts.catalogEntries.searchPlaceholder')"
        data-testid="catalog-entry-picker-search"
      />
      <label class="flex shrink-0 cursor-pointer items-center gap-2 text-sm text-af-ink-2">
        <input v-model="listedOnly" type="checkbox" class="rounded border-af-hairline-strong text-af-brand focus:ring-af-brand" />
        {{ t('admin.accounts.catalogEntries.listedOnly') }}
      </label>
    </div>

    <div class="max-h-96 overflow-y-auto rounded-lg border border-af-hairline">
      <p v-if="loading" class="px-3 py-4 text-sm text-af-ink-3">{{ t('admin.accounts.catalogEntries.loading') }}</p>
      <div v-else-if="loadFailed" class="flex items-center justify-between gap-2 px-3 py-4 text-sm text-af-warning">
        <span>{{ t('admin.accounts.catalogEntries.loadFailed') }}</span>
        <button type="button" class="btn btn-secondary btn-sm" @click="load">{{ t('admin.accounts.catalogEntries.retry') }}</button>
      </div>
      <p v-else-if="groups.length === 0" class="px-3 py-4 text-sm text-af-ink-3">
        {{ entries.length === 0 ? t('admin.accounts.catalogEntries.emptyCatalog') : t('admin.accounts.catalogEntries.noMatch') }}
      </p>
      <template v-else>
      <section
        v-for="group in groups"
        :key="group.key"
        class="border-b border-af-hairline last:border-b-0"
        :data-testid="`catalog-entry-group-${group.key || 'other'}`"
      >
        <div class="flex items-center justify-between gap-2 bg-af-sunken px-3 py-2">
          <button
            type="button"
            class="flex min-w-0 items-center gap-2 text-sm font-medium text-af-ink"
            :aria-expanded="isExpanded(group.key)"
            @click="toggleGroup(group.key)"
          >
            <Icon :name="isExpanded(group.key) ? 'chevronDown' : 'chevronRight'" size="sm" class="text-af-ink-3" />
            <PlatformIcon :platform="group.icon" size="sm" />
            <span class="truncate">{{ group.label }}</span>
            <span class="text-xs font-normal text-af-ink-3">{{ group.selectedCount }}/{{ group.entries.length }}</span>
          </button>
          <button
            type="button"
            class="shrink-0 text-xs text-af-brand hover:text-af-brand-hover"
            :data-testid="`catalog-entry-group-toggle-all-${group.key || 'other'}`"
            @click="toggleAll(group.entries)"
          >
            {{ group.selectedCount === group.entries.length
              ? t('admin.accounts.catalogEntries.deselectAll')
              : t('admin.accounts.catalogEntries.selectAll') }}
          </button>
        </div>
        <ul v-if="isExpanded(group.key)">
          <li v-for="entry in group.entries" :key="entry.id">
            <label class="flex cursor-pointer items-center gap-3 px-3 py-1.5 hover:bg-af-sunken/60">
              <input
                type="checkbox"
                class="rounded border-af-hairline-strong text-af-brand focus:ring-af-brand"
                :checked="selectedSet.has(entry.id)"
                :data-testid="`catalog-entry-${entry.id}`"
                @change="toggleEntry(entry.id)"
              />
              <span class="min-w-0 flex-1 truncate font-mono text-sm text-af-ink">{{ entry.model_id }}</span>
              <span
                v-if="entry.display_name && entry.display_name !== entry.model_id"
                class="hidden min-w-0 truncate text-xs text-af-ink-3 sm:inline"
              >{{ entry.display_name }}</span>
              <span
                v-if="entry.status !== 'listed'"
                class="shrink-0 rounded bg-af-sunken px-1.5 py-0.5 text-[11px] text-af-ink-3"
              >{{ t('admin.accounts.catalogEntries.unlisted') }}</span>
            </label>
          </li>
        </ul>
      </section>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { adminAPI } from '@/api/admin'
import type { ModelCatalogEntry } from '@/api/admin/modelCatalog'
import type { AccountPlatform } from '@/types'
import { platformLabel } from '@/utils/platformLabel'

const props = defineProps<{
  /** 勾选的目录条目 ID。 */
  modelValue: number[]
  /** 来源对应的厂商族：排在最前并默认展开；中转没有。 */
  suggestedPlatform?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: number[]]
}>()

const { t } = useI18n()

const entries = ref<ModelCatalogEntry[]>([])
const loading = ref(false)
const loadFailed = ref(false)
const query = ref('')
const listedOnly = ref(false)
const expanded = ref(new Set<string>())

// 分组顺序：来源厂商族在最前，其余按渠道来源的顺序，认不出厂商的归「其他」放最后。
const FAMILY_ORDER = ['anthropic', 'openai', 'gemini', 'grok', 'kimi', 'zhipu', 'deepseek', 'minimax', 'opencode_go']
const KNOWN_ICONS = new Set<string>([...FAMILY_ORDER, 'antigravity'])

const selectedSet = computed(() => new Set(props.modelValue))

async function load() {
  loading.value = true
  loadFailed.value = false
  try {
    entries.value = await adminAPI.modelCatalog.listEntries()
    expandInitialGroups()
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

// 初始展开：来源厂商族 + 已有勾选的组；搜索时另按命中展开（见 isExpanded）。
function expandInitialGroups() {
  const next = new Set<string>()
  if (props.suggestedPlatform) next.add(props.suggestedPlatform)
  for (const entry of entries.value) {
    if (selectedSet.value.has(entry.id)) next.add(entry.vendor_platform ?? '')
  }
  expanded.value = next
}

const normalizedQuery = computed(() => query.value.trim().toLowerCase())

interface EntryGroup {
  key: string
  label: string
  icon: AccountPlatform | 'relay'
  entries: ModelCatalogEntry[]
  selectedCount: number
}

const groups = computed<EntryGroup[]>(() => {
  const byFamily = new Map<string, ModelCatalogEntry[]>()
  for (const entry of entries.value) {
    if (listedOnly.value && entry.status !== 'listed') continue
    if (normalizedQuery.value) {
      const haystack = `${entry.model_id} ${entry.display_name ?? ''}`.toLowerCase()
      if (!haystack.includes(normalizedQuery.value)) continue
    }
    const key = entry.vendor_platform ?? ''
    const list = byFamily.get(key)
    if (list) list.push(entry)
    else byFamily.set(key, [entry])
  }
  const rank = (key: string) => {
    if (key && key === props.suggestedPlatform) return -1
    const index = FAMILY_ORDER.indexOf(key)
    if (index >= 0) return index
    return key ? FAMILY_ORDER.length : FAMILY_ORDER.length + 1
  }
  return [...byFamily.entries()]
    .sort(([a], [b]) => rank(a) - rank(b) || a.localeCompare(b))
    .map(([key, list]) => {
      const sorted = [...list].sort(
        (a, b) => Number(b.status === 'listed') - Number(a.status === 'listed') || a.model_id.localeCompare(b.model_id)
      )
      return {
        key,
        label: key ? platformLabel(key) : t('admin.accounts.catalogEntries.otherVendors'),
        icon: KNOWN_ICONS.has(key) ? (key as AccountPlatform) : 'relay',
        entries: sorted,
        selectedCount: sorted.filter((entry) => selectedSet.value.has(entry.id)).length
      }
    })
})

function isExpanded(key: string): boolean {
  return normalizedQuery.value !== '' || expanded.value.has(key)
}

function toggleGroup(key: string) {
  const next = new Set(expanded.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  expanded.value = next
}

function toggleEntry(id: number) {
  const next = new Set(props.modelValue)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  emit('update:modelValue', [...next])
}

// 组内全选 / 全不选只作用于当前可见（筛选、搜索后）的条目。
function toggleAll(groupEntries: ModelCatalogEntry[]) {
  const next = new Set(props.modelValue)
  const allSelected = groupEntries.every((entry) => next.has(entry.id))
  for (const entry of groupEntries) {
    if (allSelected) next.delete(entry.id)
    else next.add(entry.id)
  }
  emit('update:modelValue', [...next])
}

// 换了来源（新建表单）就把新来源的厂商族展开，已展开的保持不动。
watch(
  () => props.suggestedPlatform,
  (platform) => {
    if (platform && !expanded.value.has(platform)) expanded.value = new Set([...expanded.value, platform])
  }
)

onMounted(load)
</script>
