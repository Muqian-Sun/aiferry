<template>
  <!--
    按渠道的一块：这个渠道承接的每个模型一行（上游价），每格下面标官方价作参考；官方价要改请切回「按模型」。
    整块一起保存：这个渠道的承接关系一个事务整份覆盖（删掉不在列表里的、改价、新增），别的渠道不动。
  -->
  <section class="overflow-hidden rounded-lg border border-af-hairline bg-af-sheet" :data-testid="`pricing-channel-${account.id}`">
    <header class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 border-b border-af-hairline bg-af-sunken px-4 py-2.5">
      <div class="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5">
        <span class="text-sm font-semibold text-af-ink">{{ account.name }}</span>
        <span class="text-13 text-af-ink-3">{{ headerMeta }}</span>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <DiscountFillMenu :disabled="draft.rows.length === 0" @apply="fillDiscount" />
        <PopoverMenu v-if="copySources.length > 0" width-class="w-64">
          <template #trigger>
            <button type="button" class="btn btn-ghost btn-sm" data-testid="pricing-channel-copy">
              {{ t('admin.pricing.copyFromSibling') }}<Icon name="chevronDown" size="xs" />
            </button>
          </template>
          <MenuItem v-for="source in copySources" :key="source.id" @click="copyFrom(source.id)">{{ source.name }}</MenuItem>
        </PopoverMenu>
        <PopoverMenu width-class="w-72" @open="addQuery = ''">
          <template #trigger>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="addableEntries.length === 0" data-testid="pricing-channel-add-model">
              <Icon name="plus" size="sm" />{{ t('admin.pricing.addModel') }}
            </button>
          </template>
          <div class="border-b border-af-hairline px-2 pb-1.5 pt-0.5">
            <input
              v-model="addQuery"
              type="text"
              class="input h-8 px-2 py-1 text-13"
              :placeholder="t('admin.pricing.searchModel')"
              data-testid="pricing-channel-add-model-search"
            />
          </div>
          <div class="max-h-72 overflow-y-auto">
            <MenuItem v-for="entry in filteredAddable" :key="entry.id" @click="addModel(entry)">
              <span class="font-mono">{{ entry.model_id }}</span>
            </MenuItem>
            <p v-if="filteredAddable.length === 0" class="px-3 py-2 text-xs text-af-ink-3">{{ t('admin.pricing.noMatch') }}</p>
          </div>
        </PopoverMenu>
      </div>
    </header>

    <div class="relative overflow-x-auto">
      <table class="w-full min-w-[980px] text-13">
        <thead>
          <tr class="border-b border-af-hairline text-left text-xs text-af-ink-3">
            <th class="px-3 py-2 font-medium">{{ t('admin.pricing.columns.model') }}</th>
            <th class="px-2 py-2 font-medium">{{ t('admin.pricing.columns.upstreamModel') }}</th>
            <th v-for="key in PRICE_KEYS" :key="key" class="px-2 py-2 text-right font-medium">{{ t(`admin.pricing.columns.${key}`) }}</th>
            <th class="px-2 py-2 font-medium">{{ t('admin.pricing.columns.segments') }}</th>
            <th class="px-3 py-2 text-right font-medium">{{ t('admin.pricing.columns.margin') }}</th>
            <th class="px-3 py-2 font-medium">{{ t('admin.pricing.columns.status') }}</th>
            <th class="px-3 py-2"><span class="sr-only">{{ t('admin.pricing.columns.actions') }}</span></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-af-hairline">
          <PricingPriceRows
            v-for="row in draft.rows"
            :key="row.id"
            v-model:prices="row.prices"
            :issues="rowIssues(row)"
            :refs="officialOf(row.id)"
            :row-class="isNewRow(row) ? 'bg-af-warning-tint/50' : ''"
            :test-id="`pricing-channel-row-${row.id}`"
            :search-keys="searchKeysOf(entriesById.get(row.id)?.search_defaults)"
            :search-placeholders="upstreamSearchPlaceholders(t, searchKeysOf(entriesById.get(row.id)?.search_defaults), officialOf(row.id))"
            :search-hints="upstreamSearchHints(t, searchKeysOf(entriesById.get(row.id)?.search_defaults), officialOf(row.id), entriesById.get(row.id)?.search_defaults)"
            :search-note="t('admin.pricing.search.upstreamNote')"
          >
            <template #lead>
              <div class="flex items-center gap-1.5">
                <span class="font-mono text-af-ink">{{ entriesById.get(row.id)?.model_id ?? `#${row.id}` }}</span>
                <span v-if="isNewRow(row)" class="rounded-full bg-af-warning-tint px-1.5 text-xs text-af-warning">{{ t('admin.pricing.newRow') }}</span>
              </div>
              <div v-if="entriesById.get(row.id)?.status === 'unlisted'" class="text-xs text-af-ink-3">{{ t('admin.pricing.status.unlisted') }}</div>
            </template>
            <template #upstream>
              <input
                v-model="row.upstreamModel"
                type="text"
                autocomplete="off"
                spellcheck="false"
                :placeholder="t('admin.pricing.sameName')"
                :aria-label="t('admin.pricing.columns.upstreamModel')"
                :title="t('admin.pricing.upstreamModelHint')"
                :class="['input h-8 w-40 px-2 py-1 font-mono text-13', upstreamModelInvalid(row.upstreamModel) ? 'border-af-danger' : '']"
                data-testid="pricing-upstream-model"
              />
            </template>
            <template #margin><MarginCell :margin="savedMargin(row)" :min-margin="minMargin" /></template>
            <template #status><ChannelStatusCell :account="account" :margin="savedMargin(row)" :min-margin="minMargin" /></template>
            <template #actions>
              <button type="button" class="whitespace-nowrap text-13 text-af-ink-3 transition-colors hover:text-af-danger" @click="removeRow(row.id)">
                {{ t('admin.pricing.remove') }}
              </button>
            </template>
          </PricingPriceRows>
          <tr v-if="draft.rows.length === 0">
            <td :colspan="11" class="px-3 py-3 text-13 text-af-ink-3">{{ t('admin.pricing.noModels') }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <footer class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 border-t border-af-hairline px-4 py-2.5">
      <div class="min-w-0 flex-1 space-y-0.5">
        <p v-for="message in issueMessages" :key="message" class="text-xs text-af-danger">{{ message }}</p>
        <FormError :message="saveError" />
        <p v-if="changes === 0 && !saveError" class="text-xs text-af-ink-3">{{ t('admin.pricing.officialReadOnly') }}</p>
      </div>
      <div v-if="changes > 0" class="flex items-center gap-2">
        <span class="text-xs text-af-ink-3">{{ t('admin.pricing.changes', { count: changes }) }}</span>
        <button type="button" class="btn btn-ghost btn-sm" :disabled="saving" @click="reset">{{ t('admin.pricing.undo') }}</button>
        <button
          type="button"
          class="btn btn-primary btn-sm"
          :disabled="saving || issueMessages.length > 0"
          data-testid="pricing-channel-save"
          @click="save"
        >
          {{ saving ? t('admin.pricing.saving') : t('common.save') }}
        </button>
      </div>
    </footer>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { PricingAccount, PricingEntry } from '@/api/admin/pricing'
import Icon from '@/components/icons/Icon.vue'
import FormError from '@/components/common/FormError.vue'
import { MenuItem, PopoverMenu } from '@/components/admin/list'
import { extractApiErrorMessage } from '@/utils/apiError'
import PricingPriceRows from './PricingPriceRows.vue'
import MarginCell from './MarginCell.vue'
import ChannelStatusCell from './ChannelStatusCell.vue'
import DiscountFillMenu from './DiscountFillMenu.vue'
import {
  PRICE_KEYS,
  bindingRowIssues,
  channelDraftChanges,
  cloneChannelDraft,
  emptyPriceRow,
  fillByDiscount,
  hasRowIssues,
  keyedRowUnchanged,
  marginOf,
  priceRowFrom,
  priceRowToRequest,
  searchKeysOf,
  siblingBindingOf,
  upstreamModelInvalid,
  type BlockState,
  type ChannelDraft,
  type KeyedRow,
  type PriceKey,
  type RowIssues,
  type SearchKey
} from './pricingDraft'
import { issueSummary } from './issueSummary'
import { upstreamSearchHints, upstreamSearchPlaceholders } from './searchHints'

const props = defineProps<{
  account: PricingAccount
  /** 这一块的草稿，由页面保管（筛选、翻页不丢） */
  state: BlockState<ChannelDraft>
  accounts: PricingAccount[]
  /** 价格页的全部模型（按 Token 计费） */
  entries: PricingEntry[]
  defaultUserRate: number
  minMargin: number
}>()

const emit = defineEmits<{ saved: [] }>()

const { t } = useI18n()

const entriesById = computed(() => new Map(props.entries.map((entry) => [entry.id, entry])))

function modelIdOf(entryId: number): string {
  return entriesById.value.get(entryId)?.model_id ?? ''
}

// 草稿对象由页面保管，块内原地改
const blockState = computed(() => props.state)
const draft = computed(() => props.state.draft)

const changes = computed(() => channelDraftChanges(props.state))

/** 官方价（搜索价只算显式设了的：没设的上游可不填） */
function officialOf(entryId: number): Record<PriceKey, number | null> & Record<SearchKey, number | null> {
  const entry = entriesById.value.get(entryId)
  return {
    input_price: entry?.input_price ?? null,
    output_price: entry?.output_price ?? null,
    cache_read_price: entry?.cache_read_price ?? null,
    cache_write_price: entry?.cache_write_price ?? null,
    cache_write_1h_price: entry?.cache_write_1h_price ?? null,
    search_price_per_call: entry?.search_price_per_call ?? null,
    x_post_price: entry?.x_post_price ?? null,
    x_user_price: entry?.x_user_price ?? null
  }
}

function rowIssues(row: KeyedRow): RowIssues {
  return bindingRowIssues(row, officialOf(row.id))
}

function isNewRow(row: KeyedRow): boolean {
  return !props.state.initial.rows.some((item) => item.id === row.id)
}

function savedMargin(row: KeyedRow): number | null | undefined {
  if (!keyedRowUnchanged(row, props.state.initial.rows)) return undefined
  const binding = entriesById.value.get(row.id)?.bindings.find((item) => item.account_id === props.account.id)
  return marginOf(binding?.cost_ratio, props.defaultUserRate)
}

const headerMeta = computed(() => {
  const parts: string[] = []
  if (props.account.protocol) parts.push(t(`admin.accounts.protocolEndpoints.protocols.${props.account.protocol}`))
  else parts.push(props.account.platform)
  parts.push(t('admin.pricing.priority', { priority: props.account.priority }))
  parts.push(t('admin.pricing.modelCount', { count: props.state.draft.rows.length }))
  return parts.join(' · ')
})

// ---- 加模型：只列能由这个渠道承接、还没承接的模型
const addQuery = ref('')
const addableEntries = computed(() => {
  const bound = new Set(props.state.draft.rows.map((row) => row.id))
  return props.entries
    .filter((entry) => !bound.has(entry.id) && entry.bindable_account_ids.includes(props.account.id))
    .sort((a, b) => a.model_id.localeCompare(b.model_id))
})
const filteredAddable = computed(() => {
  const query = addQuery.value.trim().toLowerCase()
  return query ? addableEntries.value.filter((entry) => entry.model_id.toLowerCase().includes(query)) : addableEntries.value
})

function addModel(entry: PricingEntry) {
  const sibling = siblingBinding(entry.id)
  draft.value.rows.push({
    id: entry.id,
    upstreamModel: sibling?.upstream_model ?? '',
    prices: sibling ? priceRowFrom(sibling) : emptyPriceRow()
  })
}

// ---- 从同上游的渠道复制价格：同一家上游按协议建了几个渠道（fenno 有 Chat / Responses / Messages）时，
// 把另一个渠道里同一个模型的上游价与分段复制过来；这个渠道还没承接、但能承接的模型一并加上。
const copySources = computed(() => {
  const host = props.account.upstream_host
  if (!host) return []
  return props.accounts.filter(
    (other) => other.id !== props.account.id && other.upstream_host === host && props.entries.some((entry) => entry.bindings.some((b) => b.account_id === other.id))
  )
})

/** 同上游的渠道承接这个模型时的那条承接关系（取第一个） */
function siblingBinding(entryId: number) {
  const entry = entriesById.value.get(entryId)
  return entry ? siblingBindingOf(entry, props.account, props.accounts) : null
}

function copyFrom(sourceId: number) {
  for (const entry of props.entries) {
    const binding = entry.bindings.find((b) => b.account_id === sourceId)
    if (!binding || !entry.bindable_account_ids.includes(props.account.id)) continue
    const prices = priceRowFrom(binding)
    const existing = draft.value.rows.find((row) => row.id === entry.id)
    if (existing) {
      existing.prices = prices
      existing.upstreamModel = binding.upstream_model ?? ''
    } else {
      draft.value.rows.push({ id: entry.id, upstreamModel: binding.upstream_model ?? '', prices })
    }
  }
}

function removeRow(id: number) {
  draft.value.rows = draft.value.rows.filter((row) => row.id !== id)
}

/** 各模型行空着的上游价 = 这个模型的官方价 × ratio（官方价只读，按服务端的值算） */
function fillDiscount(ratio: number) {
  for (const row of draft.value.rows) {
    const entry = entriesById.value.get(row.id)
    if (entry) fillByDiscount(row.prices, priceRowFrom(entry), ratio)
  }
}

const issueMessages = computed(() =>
  props.state.draft.rows.flatMap((row) => {
    const issues = rowIssues(row)
    return hasRowIssues(issues) ? [issueSummary(t, modelIdOf(row.id), issues)] : []
  })
)

function reset() {
  blockState.value.draft = cloneChannelDraft(props.state.initial)
  saveError.value = ''
}

const saving = ref(false)
const saveError = ref('')

async function save() {
  if (issueMessages.value.length > 0) return
  saving.value = true
  saveError.value = ''
  try {
    await adminAPI.pricing.saveChannel(props.account.id, {
      bindings: props.state.draft.rows.map((row) => ({
        entry_id: row.id,
        upstream_model: row.upstreamModel.trim(),
        ...priceRowToRequest(row.prices)
      }))
    })
    // 先记成已保存（这一块变干净），页面重拉后按新数据重建，带上后端重算的毛利
    blockState.value.initial = cloneChannelDraft(props.state.draft)
    emit('saved')
  } catch (error) {
    saveError.value = extractApiErrorMessage(error, t('common.unknownError'))
  } finally {
    saving.value = false
  }
}
</script>
