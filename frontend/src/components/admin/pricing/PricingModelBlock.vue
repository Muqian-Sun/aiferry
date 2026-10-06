<template>
  <!--
    按模型的一块：官方价一行 + 每个承接渠道一行（上游价）。给模型加一个渠道就是承接（muqian 2026-09-30，D1）。
    整块一起保存：官方价与全部承接关系一个事务整份覆盖；改过的块底部出现「改了 N 处 · 撤销 · 保存」。
  -->
  <!-- 不做卡片（2026-10-04 muqian「一整个卡片」）：块与块之间一条发丝线 + 留白，表头不铺灰 -->
  <section class="border-t border-af-hairline pt-4" :data-testid="`pricing-model-${entry.id}`">
    <header class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 pb-2">
      <div class="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5">
        <span class="font-mono text-base font-semibold text-af-ink">{{ entry.model_id }}</span>
        <span class="text-13 text-af-ink-3">
          {{ headerMeta }}
        </span>
        <span
          v-if="entry.web_search_delegate"
          class="rounded-full bg-af-sheet px-2 text-xs font-medium text-af-ink-2 ring-1 ring-inset ring-af-hairline"
          :title="t('admin.pricing.webSearchDelegateHint')"
          data-testid="pricing-web-search-delegate"
        >{{ t('admin.pricing.webSearchDelegate') }}</span>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <span v-if="lookupMessage" class="text-xs text-af-ink-3">{{ lookupMessage }}</span>
        <button type="button" class="btn btn-ghost btn-sm" :disabled="lookingUp" data-testid="pricing-model-lookup" @click="fillFromPriceFile">
          {{ t('admin.pricing.fillFromPriceFile') }}
        </button>
        <DiscountFillMenu kind="sale" @apply="fillSale" />
        <DiscountFillMenu :disabled="draft.rows.length === 0" @apply="fillDiscount" />
        <PopoverMenu width-class="w-64">
          <template #trigger>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="addableAccounts.length === 0" data-testid="pricing-model-add-channel">
              <Icon name="plus" size="sm" />{{ t('admin.pricing.addChannel') }}
            </button>
          </template>
          <div class="max-h-72 overflow-y-auto">
            <MenuItem v-for="account in addableAccounts" :key="account.id" @click="addChannel(account)">
              {{ account.name }}
            </MenuItem>
          </div>
        </PopoverMenu>
      </div>
    </header>

    <div class="relative overflow-x-auto">
      <table class="w-full min-w-[980px] text-13">
        <thead>
          <tr class="border-b border-af-hairline text-left text-xs text-af-ink-3">
            <th class="py-2 pl-0 pr-3 font-medium">{{ t('admin.pricing.columns.channel') }}</th>
            <th class="px-2 py-2 font-medium">{{ t('admin.pricing.columns.upstreamModel') }}</th>
            <th v-for="key in PRICE_KEYS" :key="key" class="px-2 py-2 text-right font-medium">{{ t(`admin.pricing.columns.${key}`) }}</th>
            <th class="px-2 py-2 font-medium">{{ t('admin.pricing.columns.segments') }}</th>
            <th class="px-3 py-2 text-right font-medium">{{ t('admin.pricing.columns.margin') }}</th>
            <th class="px-3 py-2 font-medium">{{ t('admin.pricing.columns.status') }}</th>
            <th class="py-2 pl-3 pr-0"><span class="sr-only">{{ t('admin.pricing.columns.actions') }}</span></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-af-hairline">
          <PricingPriceRows
            v-model:prices="draft.official"
            :issues="officialRowIssues"
            test-id="pricing-official"
            :search-keys="searchKeys"
            :search-placeholders="officialSearchPlaceholders(t, searchKeys, entry.search_defaults)"
            :search-note="t('admin.pricing.search.officialNote')"
          >
            <template #lead>
              <div class="font-medium text-af-ink">{{ t('admin.pricing.official') }}</div>
            </template>
            <template #upstream><span class="text-xs text-af-ink-3">{{ t('admin.pricing.catalogName') }}</span></template>
            <template #margin><span class="text-af-ink-3">—</span></template>
          </PricingPriceRows>
          <PricingSaleRows v-model:sale="draft.sale" :official="draft.official" :ratio="defaultSaleRatio" test-id="pricing-sale" />
          <PricingPriceRows
            v-for="row in draft.rows"
            :key="row.id"
            v-model:prices="row.prices"
            :issues="rowIssues(row)"
            :row-class="isNewRow(row) ? 'bg-af-warning-tint/50' : ''"
            :test-id="`pricing-binding-${row.id}`"
            :search-keys="searchKeys"
            :search-placeholders="upstreamSearchPlaceholders(t, searchKeys, draft.official)"
            :search-hints="upstreamSearchHints(t, searchKeys, draft.official, entry.search_defaults)"
            :search-note="t('admin.pricing.search.upstreamNote')"
          >
            <template #lead>
              <div class="flex items-center gap-1.5">
                <span class="font-medium text-af-ink">{{ accountName(row.id) }}</span>
                <span v-if="isNewRow(row)" class="rounded-full bg-af-warning-tint px-1.5 text-xs text-af-warning">{{ t('admin.pricing.newRow') }}</span>
              </div>
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
            <template #status><ChannelStatusCell :account="accounts.get(row.id)" :margin="savedMargin(row)" :min-margin="minMargin" /></template>
            <template #actions>
              <button type="button" class="whitespace-nowrap text-13 text-af-ink-3 transition-colors hover:text-af-danger" @click="removeRow(row.id)">
                {{ t('admin.pricing.remove') }}
              </button>
            </template>
          </PricingPriceRows>
          <tr v-if="draft.rows.length === 0">
            <td :colspan="11" class="py-3 pl-0 pr-3 text-13 text-af-ink-3">{{ t('admin.pricing.noChannels') }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <footer
      v-if="changes > 0 || saveError"
      class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 border-t border-af-hairline py-2.5"
    >
      <div class="min-w-0 flex-1 space-y-0.5">
        <p v-for="message in issueMessages" :key="message" class="text-xs text-af-danger">{{ message }}</p>
        <FormError :message="saveError" />
      </div>
      <div class="flex items-center gap-2">
        <span class="text-xs text-af-ink-3">{{ t('admin.pricing.changes', { count: changes }) }}</span>
        <button type="button" class="btn btn-ghost btn-sm" :disabled="saving || changes === 0" @click="reset">{{ t('admin.pricing.undo') }}</button>
        <button
          type="button"
          class="btn btn-primary btn-sm"
          :disabled="saving || changes === 0 || issueMessages.length > 0"
          data-testid="pricing-model-save"
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
import PricingSaleRows from './PricingSaleRows.vue'
import {
  PRICE_KEYS,
  bindingRowIssues,
  cloneModelDraft,
  clonePriceRow,
  emptyPriceRow,
  fillByDiscount,
  fillSaleByRatio,
  hasRowIssues,
  keyedRowUnchanged,
  marginOf,
  modelDraftChanges,
  officialIssues,
  priceRowChanges,
  priceRowFrom,
  priceRowToRequest,
  saleRowChanges,
  saleRowInvalid,
  saleRowToRequest,
  searchKeysOf,
  upstreamModelInvalid,
  type BlockState,
  type KeyedRow,
  type ModelDraft,
  type RowIssues
} from './pricingDraft'
import { issueSummary } from './issueSummary'
import { officialSearchPlaceholders, upstreamSearchHints, upstreamSearchPlaceholders } from './searchHints'

const props = defineProps<{
  entry: PricingEntry
  /** 这一块的草稿，由页面保管（筛选、翻页不丢） */
  state: BlockState<ModelDraft>
  accounts: Map<number, PricingAccount>
  /** 渠道排序键（渠道优先级、再按 ID） */
  accountOrder: (accountId: number) => number
  defaultSaleRatio: number
  minMargin: number
}>()

const emit = defineEmits<{ saved: [] }>()

const { t } = useI18n()

// 草稿对象由页面保管，块内原地改
const blockState = computed(() => props.state)
const draft = computed(() => props.state.draft)

const changes = computed(() => modelDraftChanges(props.state))
// 毛利按售价算（上游成本比由后端按售价口径算）：官方价或售价改了都要保存后重算
const officialChanged = computed(
  () =>
    priceRowChanges(props.state.draft.official, props.state.initial.official) > 0 ||
    saleRowChanges(props.state.draft.sale, props.state.initial.sale) > 0
)

const officialRowIssues = computed(() => officialIssues(props.state.draft.official))

/** 这个模型能填的搜索价（厂商没有官方搜索工具时为空） */
const searchKeys = computed(() => searchKeysOf(props.entry.search_defaults))

function rowIssues(row: KeyedRow): RowIssues {
  return bindingRowIssues(row, props.state.draft.official)
}

function isNewRow(row: KeyedRow): boolean {
  return !props.state.initial.rows.some((item) => item.id === row.id)
}

function accountName(id: number): string {
  return props.accounts.get(id)?.name ?? `#${id}`
}

/** 毛利只对保存过、没改过的行显示（后端按上游价 ÷ 售价口径逐项逐段算，改了价要保存后重算） */
function savedMargin(row: KeyedRow): number | null | undefined {
  if (officialChanged.value || !keyedRowUnchanged(row, props.state.initial.rows)) return undefined
  const binding = props.entry.bindings.find((item) => item.account_id === row.id)
  return marginOf(binding?.cost_ratio, props.defaultSaleRatio)
}

const headerMeta = computed(() =>
  [
    props.entry.vendor || t('admin.pricing.noVendor'),
    t(`admin.pricing.status.${props.entry.status}`),
    t('admin.pricing.channelCount', { count: props.state.draft.rows.length })
  ].join(' · ')
)

const addableAccounts = computed(() => {
  const bound = new Set(props.state.draft.rows.map((row) => row.id))
  return props.entry.bindable_account_ids
    .filter((id) => !bound.has(id))
    .map((id) => props.accounts.get(id))
    .filter((account): account is PricingAccount => account != null)
    .sort((a, b) => props.accountOrder(a.id) - props.accountOrder(b.id))
})

/** 新加的渠道：同一上游（主机名相同）已经承接这个模型的，先带上它的上游模型名与上游价 */
function addChannel(account: PricingAccount) {
  const sibling = account.upstream_host
    ? draft.value.rows.find((row) => props.accounts.get(row.id)?.upstream_host === account.upstream_host)
    : undefined
  draft.value.rows.push({
    id: account.id,
    upstreamModel: sibling?.upstreamModel ?? '',
    prices: sibling ? clonePriceRow(sibling.prices) : emptyPriceRow()
  })
}

function removeRow(id: number) {
  draft.value.rows = draft.value.rows.filter((row) => row.id !== id)
}

/** 各渠道行空着的上游价 = 这一块（可能还没保存的）官方价 × ratio */
function fillDiscount(ratio: number) {
  for (const row of draft.value.rows) fillByDiscount(row.prices, draft.value.official, ratio)
}

/** 空着的售价 = 这一块（可能还没保存的）官方价 × ratio */
function fillSale(ratio: number) {
  fillSaleByRatio(draft.value.sale, draft.value.official, ratio)
}

const issueMessages = computed(() => {
  const messages: string[] = []
  if (hasRowIssues(officialRowIssues.value)) messages.push(issueSummary(t, t('admin.pricing.official'), officialRowIssues.value))
  if (saleRowInvalid(props.state.draft.sale, props.state.draft.official)) messages.push(t('admin.pricing.sale.invalid'))
  for (const row of props.state.draft.rows) {
    const issues = rowIssues(row)
    if (hasRowIssues(issues)) messages.push(issueSummary(t, accountName(row.id), issues))
  }
  return messages
})

function reset() {
  blockState.value.draft = cloneModelDraft(props.state.initial)
  saveError.value = ''
}

const saving = ref(false)
const saveError = ref('')

async function save() {
  if (issueMessages.value.length > 0) return
  saving.value = true
  saveError.value = ''
  try {
    await adminAPI.pricing.saveModel(props.entry.id, {
      ...priceRowToRequest(props.state.draft.official),
      sale_prices: saleRowToRequest(props.state.draft.sale, props.state.draft.official),
      bindings: props.state.draft.rows.map((row) => ({
        account_id: row.id,
        upstream_model: row.upstreamModel.trim(),
        ...priceRowToRequest(row.prices)
      }))
    })
    // 先记成已保存（这一块变干净），页面重拉后按新数据重建，带上后端重算的毛利
    blockState.value.initial = cloneModelDraft(props.state.draft)
    emit('saved')
  } catch (error) {
    saveError.value = extractApiErrorMessage(error, t('common.unknownError'))
  } finally {
    saving.value = false
  }
}

// ---- 按价格文件带官方价（与新建模型时的带价同一个接口）
const lookingUp = ref(false)
const lookupMessage = ref('')

async function fillFromPriceFile() {
  lookingUp.value = true
  lookupMessage.value = ''
  try {
    const found = await adminAPI.modelCatalog.priceLookup(props.entry.model_id)
    if (!found || found.billing_mode !== 'token') {
      lookupMessage.value = t('admin.pricing.priceFileMissing')
      return
    }
    draft.value.official = priceRowFrom({ ...found, cache_write_1h_price: found.cache_write_1h_price ?? null, intervals: found.intervals ?? [] })
  } catch (error) {
    lookupMessage.value = extractApiErrorMessage(error, t('common.unknownError'))
  } finally {
    lookingUp.value = false
  }
}
</script>
