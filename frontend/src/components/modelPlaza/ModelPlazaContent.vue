<template>
  <!--
    模型页「班次表」：每个上架条目一行，标价按百万 Token 列出；搜索 + 厂商筛选。
    embedded=已登录（控制台壳提供页头）；否则公开壳，这里自己画标题。
  -->
  <div class="space-y-6">
    <div v-if="!embedded" class="border-b border-af-hairline pb-4">
      <h1 class="text-xl font-semibold text-af-ink">{{ t('userUi.models.title') }}</h1>
      <p class="mt-1 text-13 text-af-ink-3">{{ t('userUi.models.description') }}</p>
    </div>

    <!-- 管理员配置的全局价格说明（Markdown） -->
    <div v-if="descriptionHtml" class="plaza-description text-sm text-af-ink-2" v-html="descriptionHtml"></div>

    <StatusState v-if="loading" kind="loading" :title="t('userUi.status.loading')" />
    <StatusState v-else-if="error" kind="error" :title="t('userUi.models.loadFailed')" />

    <template v-else>
      <!-- 工具行：搜索 / 厂商 / 计数 -->
      <div class="flex flex-wrap items-center gap-3">
        <SearchInput v-model="searchQuery" :placeholder="t('modelPlaza.filters.searchPlaceholder')" class="w-full sm:w-72" />
        <Select v-model="selectedVendor" :options="vendorOptions" class="w-44" />
        <span class="ml-auto text-13 tabular-nums text-af-ink-3" data-testid="catalog-count">
          {{ t('userUi.models.count', { count: filtered.length }) }}
        </span>
      </div>

      <StatusState
        v-if="filtered.length === 0"
        kind="empty"
        :title="searchActive ? t('userUi.models.noSearchResult') : t('userUi.models.empty')"
      />

      <div v-else class="-mx-6 overflow-x-auto">
        <table class="w-full min-w-[720px] text-13" data-testid="catalog-table">
          <thead>
            <tr class="border-b border-af-hairline text-left text-af-ink-3">
              <th class="py-2 pl-6 pr-4 font-medium">{{ t('userUi.models.columns.model') }}</th>
              <th class="py-2 pr-4 font-medium">{{ t('userUi.models.columns.vendor') }}</th>
              <th class="py-2 pr-4 font-medium">{{ t('userUi.models.columns.billing') }}</th>
              <th class="py-2 pr-4 text-right font-medium" colspan="3">
                {{ t('userUi.models.listPrice') }}
                <span class="ml-1 font-normal text-af-ink-4">{{ t('userUi.models.perMillion') }}</span>
              </th>
            </tr>
            <tr class="border-b border-af-hairline text-left text-xs text-af-ink-4">
              <th class="py-1.5 pl-6 pr-4 font-normal" colspan="3"></th>
              <th class="py-1.5 pr-4 text-right font-normal">{{ t('userUi.models.columns.input') }}</th>
              <th class="py-1.5 pr-4 text-right font-normal">{{ t('userUi.models.columns.output') }}</th>
              <th class="py-1.5 pr-6 text-right font-normal">{{ t('userUi.models.columns.cacheRead') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-af-hairline">
            <tr v-for="entry in filtered" :key="entry.id" class="group h-11 hover:bg-af-sunken" data-testid="catalog-row">
              <td class="pl-6 pr-4">
                <div class="flex items-center gap-2">
                  <span class="font-mono font-medium text-af-ink">{{ entry.id }}</span>
                  <button
                    type="button"
                    class="rounded p-1 text-af-ink-4 opacity-0 transition-opacity hover:text-af-ink focus:opacity-100 group-hover:opacity-100 [@media(hover:none)]:opacity-100"
                    :aria-label="t('userUi.models.copyId')"
                    :title="copiedId === entry.id ? t('userUi.models.copied') : t('userUi.models.copyId')"
                    @click="copyId(entry.id)"
                  >
                    <Icon :name="copiedId === entry.id ? 'check' : 'copy'" size="xs" />
                  </button>
                </div>
              </td>
              <td class="pr-4 text-af-ink-2">{{ vendorLabel(entry.vendor) }}</td>
              <td class="pr-4 text-af-ink-3">{{ getBillingModeLabel(entry.billingMode, t) }}</td>
              <td class="pr-4 text-right tabular-nums text-af-ink">{{ formatPrice(entry.price?.input) }}</td>
              <td class="pr-4 text-right tabular-nums text-af-ink">{{ formatPrice(entry.price?.output) }}</td>
              <td class="pr-6 text-right tabular-nums text-af-ink-2">{{ formatPrice(entry.price?.cacheRead) }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <p class="max-w-3xl text-xs leading-5 text-af-ink-3">{{ t('userUi.models.priceNote') }}</p>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import Icon from '@/components/icons/Icon.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import type { ModelPlazaResponse } from '@/api/modelPlaza'
import { useClipboard } from '@/composables/useClipboard'
import { getBillingModeLabel } from '@/utils/billingMode'
import { buildCatalog, catalogVendors, filterCatalog, formatCatalogPrice as formatPrice, vendorLabel } from './catalog'

const props = defineProps<{
  response: ModelPlazaResponse | null
  loading: boolean
  error: boolean
  embedded?: boolean
}>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()

const searchQuery = ref('')
const selectedVendor = ref('all')
const copiedId = ref<string | null>(null)

const descriptionHtml = computed(() => {
  const md = props.response?.description?.trim()
  if (!md) return ''
  return DOMPurify.sanitize(marked.parse(md) as string)
})

const catalog = computed(() => buildCatalog(props.response?.models ?? []))
const vendors = computed(() => catalogVendors(catalog.value))
const vendorOptions = computed<SelectOption[]>(() => [
  { value: 'all', label: t('userUi.models.allVendors') },
  ...vendors.value.map((v) => ({ value: v, label: vendorLabel(v) }))
])
const searchActive = computed(() => searchQuery.value.trim() !== '')
const filtered = computed(() => filterCatalog(catalog.value, searchQuery.value, selectedVendor.value))

// 数据刷新后失效的厂商筛选回到全部
watch(vendors, (list) => {
  if (selectedVendor.value !== 'all' && !list.includes(selectedVendor.value)) selectedVendor.value = 'all'
})

let copiedTimer: ReturnType<typeof setTimeout> | null = null
async function copyId(id: string) {
  const ok = await copyToClipboard(id)
  if (!ok) return
  copiedId.value = id
  if (copiedTimer) clearTimeout(copiedTimer)
  copiedTimer = setTimeout(() => (copiedId.value = null), 1500)
}
</script>

<style scoped>
.plaza-description :deep(p) {
  @apply mb-2 last:mb-0;
}
.plaza-description :deep(a) {
  @apply text-af-brand underline underline-offset-2 hover:text-af-brand-hover;
}
.plaza-description :deep(ul),
.plaza-description :deep(ol) {
  @apply mb-2 list-disc pl-5;
}
.plaza-description :deep(code) {
  @apply rounded bg-af-sunken px-1 py-0.5 font-mono text-xs text-af-ink;
}
</style>
