<template>
  <!--
    模型详情抽屉（A5）：模型目录点行打开，看完即关。两个页签：
    概况（标识、厂商、计费、上架、别名、全部标价与分档 / 分时）、渠道（承接的渠道此刻能否调度；「诊断」看各入口协议能否承接）。
    改配置点「编辑」，上下架 / 删除在「⋯」里；动作都 emit 给目录页，由目录页弹原有的对话框。
    抽屉里所有 $ 价共用一个小数位数（priceFormat），不会一行 $3.00、一行 $0.3。
  -->
  <DetailDrawer
    :show="show && !!entry"
    :title="entry?.model_id ?? ''"
    :eyebrow="entry ? t('admin.modelCatalog.drawer.eyebrow', { id: entry.id }) : ''"
    :subtitle="subtitle"
    :tabs="tabs"
    :tab="tab"
    @update:tab="emit('update:tab', $event)"
    @close="emit('close')"
  >
    <template #actions>
      <button type="button" class="btn btn-secondary btn-sm" data-testid="model-catalog-drawer-edit" @click="emit('edit')">
        {{ t('common.edit') }}
      </button>
      <PopoverMenu v-if="entry" width-class="w-40">
        <template #trigger="{ open }">
          <button
            type="button"
            class="rounded-md p-2 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink"
            :class="open ? 'bg-af-sunken text-af-ink' : ''"
            :title="t('common.more')"
            :aria-label="t('common.more')"
            data-testid="model-catalog-drawer-more"
          >
            <Icon name="more" size="md" />
          </button>
        </template>
        <MenuItem
          :icon="entry.status === 'listed' ? 'arrowDown' : 'arrowUp'"
          data-testid="model-catalog-drawer-toggle-status"
          @click="emit('set-status', entry.status === 'listed' ? 'unlisted' : 'listed')"
        >
          {{ entry.status === 'listed' ? t('admin.modelCatalog.bulk.unlist') : t('admin.modelCatalog.bulk.list') }}
        </MenuItem>
        <MenuItem divider />
        <MenuItem icon="trash" danger data-testid="model-catalog-drawer-delete" @click="emit('delete')">
          {{ t('common.delete') }}
        </MenuItem>
      </PopoverMenu>
    </template>

    <!-- 上架却没价 / 没渠道是真异常：用户看不到或调不通 -->
    <template v-if="banners.length" #banner>
      <div class="space-y-1.5">
        <p
          v-for="banner in banners"
          :key="banner.key"
          :class="[
            'flex items-center gap-2 rounded-md px-3 py-2 text-13',
            banner.tone === 'danger' ? 'bg-af-danger-tint text-af-danger' : 'bg-af-warning-tint text-af-warning'
          ]"
          :data-testid="`model-catalog-drawer-banner-${banner.key}`"
        >
          <Icon name="exclamationTriangle" size="sm" class="shrink-0" />
          {{ banner.text }}
        </p>
      </div>
    </template>

    <template v-if="entry">
      <!-- 概况 -->
      <div v-if="tab === 'overview'" class="space-y-8" data-testid="model-catalog-drawer-overview">
        <dl class="divide-y divide-af-hairline">
          <DetailField :label="t('admin.modelCatalog.fields.modelId')">
            <span class="font-mono">{{ entry.model_id }}</span>
          </DetailField>
          <DetailField :label="t('admin.modelCatalog.fields.displayName')" :value="entry.display_name" />
          <DetailField :label="t('admin.modelCatalog.fields.vendor')" :value="vendor" />
          <DetailField :label="t('admin.modelCatalog.fields.billingMode')" :value="t(`admin.modelCatalog.billingModes.${entry.billing_mode || 'token'}`)" />
          <DetailField :label="t('admin.modelCatalog.fields.status')">
            <span class="inline-flex items-center gap-1.5">
              <span :class="['inline-block h-2 w-2 rounded-full', entry.status === 'listed' ? 'bg-af-ink-3' : 'bg-af-hairline-strong']"></span>
              <span :class="entry.status === 'listed' ? 'text-af-ink' : 'text-af-ink-3'">{{ t(`admin.modelCatalog.status.${entry.status}`) }}</span>
            </span>
          </DetailField>
          <DetailField :label="t('admin.modelCatalog.drawer.aliases')">
            <span v-if="entry.aliases?.length" class="font-mono text-13" data-testid="model-catalog-drawer-aliases">
              {{ entry.aliases.map((alias) => alias.alias).join(', ') }}
            </span>
            <template v-else>—</template>
          </DetailField>
          <DetailField :label="t('admin.modelCatalog.drawer.notes')">
            <span v-if="entry.notes" class="whitespace-pre-wrap">{{ entry.notes }}</span>
            <template v-else>—</template>
          </DetailField>
          <DetailField :label="t('admin.modelCatalog.drawer.updatedAt')">
            <span class="tabular-nums">{{ formatDateTime(entry.updated_at) }}</span>
          </DetailField>
        </dl>

        <SheetSection
          :title="t('admin.modelCatalog.drawer.prices')"
          :description="isToken ? t('admin.modelCatalog.drawer.perMillionHint') : undefined"
        >
          <dl class="divide-y divide-af-hairline" data-testid="model-catalog-drawer-prices">
            <DetailField v-for="row in priceRows" :key="row.key" :label="row.label">
              <span class="tabular-nums">{{ row.value }}</span>
            </DetailField>
          </dl>
        </SheetSection>

        <SheetSection
          v-if="tierRows.length"
          :title="t('admin.modelCatalog.drawer.tiers')"
          :description="isToken ? undefined : t('admin.modelCatalog.drawer.mediaTiersHint')"
        >
          <dl class="divide-y divide-af-hairline" data-testid="model-catalog-drawer-tiers">
            <DetailField v-for="row in tierRows" :key="row.key" :label="row.label">
              <span class="tabular-nums">{{ row.value }}</span>
            </DetailField>
          </dl>
        </SheetSection>

        <SheetSection
          v-if="entry.time_pricing?.periods?.length"
          :title="t('admin.modelCatalog.drawer.timePricing')"
          :description="timePricingHint"
        >
          <dl class="divide-y divide-af-hairline">
            <DetailField
              v-for="(period, index) in entry.time_pricing.periods"
              :key="index"
              :label="`${period.start_time} – ${period.end_time}`"
            >
              <span class="tabular-nums">× {{ period.multiplier }}</span>
            </DetailField>
          </dl>
        </SheetSection>
      </div>

      <!-- 渠道：承接的渠道此刻能否调度（一次诊断请求拿全）；各入口协议能否承接看「诊断」 -->
      <SheetSection
        v-else-if="tab === 'channels'"
        :title="t('admin.modelCatalog.bindings.title')"
        :description="t('admin.modelCatalog.drawer.channelsHint')"
        data-testid="model-catalog-drawer-channels"
      >
        <template #actions>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="bindingCount === 0"
            data-testid="model-catalog-drawer-diagnose-button"
            @click="emit('diagnose')"
          >
            <Icon name="beaker" size="sm" />
            {{ t('admin.modelCatalog.diagnose') }}
          </button>
        </template>

        <StatusState
          v-if="bindingCount === 0"
          kind="empty"
          :title="t('admin.modelCatalog.diagnosis.empty')"
          :action-label="t('admin.modelCatalog.drawer.bind')"
          @action="emit('edit')"
        />
        <StatusState v-else-if="channelsLoading" kind="loading" :title="t('common.loading')" />
        <template v-else>
          <p v-if="channelsFailed" class="mb-3 text-13 text-af-ink-3">{{ t('admin.modelCatalog.drawer.channelsFallback') }}</p>
          <ul class="divide-y divide-af-hairline">
            <li
              v-for="channel in channelRows"
              :key="channel.id"
              class="flex items-start justify-between gap-4 py-3 first:pt-0"
              data-testid="model-catalog-drawer-channel"
            >
              <div class="min-w-0">
                <p class="truncate text-sm font-medium text-af-ink">{{ channel.name }}</p>
                <p class="mt-0.5 flex min-w-0 items-center gap-1 text-xs text-af-ink-3">
                  <span class="shrink-0 tabular-nums">#{{ channel.id }}</span>
                  <template v-if="channel.platform">
                    <span aria-hidden="true">·</span>
                    <PlatformTypeBadge
                      variant="plain"
                      :platform="channel.platform as AccountPlatform"
                      :type="channel.type as AccountType"
                      :vendor="channel.vendor"
                    />
                  </template>
                </p>
              </div>
              <div class="shrink-0 text-right text-xs">
                <p v-if="channel.schedulable === true" class="text-af-ink-3">{{ t('admin.modelCatalog.diagnosis.columns.schedulable') }}</p>
                <p v-else-if="channel.schedulable === false" class="text-af-danger" data-testid="model-catalog-drawer-channel-blocked">
                  {{ blockedReasonLabel(channel.blockedReason) }}
                </p>
                <p class="mt-0.5 tabular-nums text-af-ink-3">
                  {{
                    channel.priority === null
                      ? t('admin.modelCatalog.diagnosis.followAccount')
                      : t('admin.modelCatalog.drawer.priority', { value: channel.priority })
                  }}
                </p>
              </div>
            </li>
          </ul>
        </template>
      </SheetSection>
    </template>
  </DetailDrawer>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { ModelCatalogDiagnosisAccount, ModelCatalogEntry, PricingInterval } from '@/api/admin/modelCatalog'
import type { AccountPlatform, AccountType } from '@/types'
import { formatCompactNumber, formatDateTime } from '@/utils/format'
import Icon from '@/components/icons/Icon.vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import SheetSection from '@/components/user/shell/SheetSection.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import type { SectionTab } from '@/components/user/shell/types'
import { DetailDrawer, DetailField, MenuItem, PopoverMenu } from '@/components/admin/list'
import { hasPrice } from './entryRequest'
import { DETAIL_PRICE_MAX_DECIMALS, formatListPrice, perMillion, sharedPriceDecimals } from './priceFormat'
import { catalogVendorLabel } from './vendorLabel'

const props = defineProps<{
  show: boolean
  entry: ModelCatalogEntry | null
  tab: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'update:tab', tab: string): void
  (e: 'edit'): void
  (e: 'diagnose'): void
  (e: 'set-status', status: 'listed' | 'unlisted'): void
  (e: 'delete'): void
}>()

const { t, te } = useI18n()

const bindingCount = computed(() => props.entry?.bindings?.length ?? 0)
const isToken = computed(() => !props.entry?.billing_mode || props.entry.billing_mode === 'token')

const tabs = computed<SectionTab[]>(() => [
  { key: 'overview', label: t('admin.modelCatalog.drawer.tabs.overview') },
  { key: 'channels', label: t('admin.modelCatalog.drawer.tabs.channels'), count: bindingCount.value }
])

const vendor = computed(() => (props.entry ? catalogVendorLabel(props.entry) : ''))

const subtitle = computed(() => {
  const entry = props.entry
  if (!entry) return ''
  return [entry.display_name, vendor.value].filter(Boolean).join(' · ')
})

const banners = computed(() => {
  const entry = props.entry
  if (!entry || entry.status !== 'listed') return []
  const list: { key: string; tone: 'danger' | 'warning'; text: string }[] = []
  if (!hasPrice(entry)) list.push({ key: 'unpriced', tone: 'danger', text: t('admin.modelCatalog.drawer.unpricedBanner') })
  if (bindingCount.value === 0) list.push({ key: 'unbound', tone: 'warning', text: t('admin.modelCatalog.drawer.unboundBanner') })
  return list
})

// ---- 标价 ----
interface Row {
  key: string
  label: string
  value: string
}

type PriceField =
  | 'input_price'
  | 'output_price'
  | 'cache_write_price'
  | 'cache_write_1h_price'
  | 'cache_read_price'
  | 'image_input_price'
  | 'image_output_price'
  | 'image_cache_read_price'
  | 'audio_input_price'
  | 'audio_output_price'
  | 'input_price_priority'
  | 'output_price_priority'
  | 'cache_write_price_priority'
  | 'cache_read_price_priority'

/** 按 Token 计费时列出的单价（$ / 百万 Token）：输入 / 输出始终列出（没配就是「—」），其余配了才列 */
const TOKEN_PRICES: { key: string; labelKey: string; field: PriceField; always?: boolean }[] = [
  { key: 'input', labelKey: 'input', field: 'input_price', always: true },
  { key: 'output', labelKey: 'output', field: 'output_price', always: true },
  { key: 'cache_write', labelKey: 'cacheWrite', field: 'cache_write_price' },
  { key: 'cache_write_1h', labelKey: 'cacheWrite1h', field: 'cache_write_1h_price' },
  { key: 'cache_read', labelKey: 'cacheRead', field: 'cache_read_price' },
  { key: 'image_input', labelKey: 'imageInput', field: 'image_input_price' },
  { key: 'image_output', labelKey: 'imageOutput', field: 'image_output_price' },
  { key: 'image_cache_read', labelKey: 'imageCacheRead', field: 'image_cache_read_price' },
  { key: 'audio_input', labelKey: 'audioInput', field: 'audio_input_price' },
  { key: 'audio_output', labelKey: 'audioOutput', field: 'audio_output_price' },
  { key: 'input_priority', labelKey: 'inputPriority', field: 'input_price_priority' },
  { key: 'output_priority', labelKey: 'outputPriority', field: 'output_price_priority' },
  { key: 'cache_write_priority', labelKey: 'cacheWritePriority', field: 'cache_write_price_priority' },
  { key: 'cache_read_priority', labelKey: 'cacheReadPriority', field: 'cache_read_price_priority' }
]

function sortedIntervals(entry: ModelCatalogEntry): PricingInterval[] {
  return [...(entry.intervals ?? [])].sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0))
}

/** 抽屉里出现的每个 $ 价（标价与分档）共用一个小数位数 */
const decimals = computed(() => {
  const entry = props.entry
  if (!entry) return 2
  const values: Array<number | null | undefined> = [entry.per_request_price]
  if (isToken.value) {
    values.push(...TOKEN_PRICES.map((price) => perMillion(entry[price.field])), entry.search_price_per_call)
  }
  for (const iv of sortedIntervals(entry)) {
    values.push(
      perMillion(iv.input_price),
      perMillion(iv.output_price),
      perMillion(iv.cache_write_price),
      perMillion(iv.cache_read_price),
      iv.per_request_price
    )
  }
  return sharedPriceDecimals(values, DETAIL_PRICE_MAX_DECIMALS)
})

function money(value: number | null | undefined): string {
  return formatListPrice(value, decimals.value)
}

/** 「$0.04 / 次」「$0.039 / 张」「$0.05 / 秒」 */
function perUnit(value: number, mode: string): string {
  return t(`admin.modelCatalog.drawer.price.per.${mode}`, { price: money(value) })
}

/** 200000 → 200K（去掉 formatCompactNumber 的「.0」） */
function compact(value: number): string {
  return formatCompactNumber(value).replace(/\.0(?=[KMB]?$)/, '')
}

const priceRows = computed<Row[]>(() => {
  const entry = props.entry
  if (!entry) return []
  const rows: Row[] = []
  const push = (key: string, label: string, value: string) => rows.push({ key, label, value })

  if (isToken.value) {
    for (const price of TOKEN_PRICES) {
      const value = entry[price.field]
      if (price.always || value != null) push(price.key, t(`admin.modelCatalog.drawer.price.${price.labelKey}`), money(perMillion(value)))
    }
    if (entry.per_request_price != null) {
      push('per_request', t('admin.modelCatalog.drawer.price.perRequest'), perUnit(entry.per_request_price, 'per_request'))
    }
    if (entry.search_price_per_call != null) {
      push('search', t('admin.modelCatalog.drawer.price.searchPerCall'), perUnit(entry.search_price_per_call, 'per_request'))
    }
    if (entry.long_context_input_threshold != null) {
      const parts = [
        entry.long_context_input_multiplier != null ? `${t('admin.modelCatalog.drawer.price.input')} × ${entry.long_context_input_multiplier}` : '',
        entry.long_context_output_multiplier != null ? `${t('admin.modelCatalog.drawer.price.output')} × ${entry.long_context_output_multiplier}` : ''
      ].filter(Boolean)
      push(
        'long_context',
        t('admin.modelCatalog.drawer.price.longContext', {
          op: entry.long_context_threshold_inclusive ? '≥' : '>',
          threshold: compact(entry.long_context_input_threshold)
        }),
        parts.join(' · ') || '—'
      )
    }
  } else {
    push(
      'per_request',
      t('admin.modelCatalog.drawer.price.listPrice'),
      entry.per_request_price == null ? '—' : perUnit(entry.per_request_price, entry.billing_mode)
    )
  }

  const multiplier = (key: string, labelKey: string, value: number | null | undefined) => {
    if (value != null) push(key, t(`admin.modelCatalog.drawer.price.${labelKey}`), `× ${value}`)
  }
  multiplier('fast', 'fast', entry.fast_multiplier)
  multiplier('flex', 'flex', entry.flex_multiplier)
  multiplier('max_reasoning', 'maxReasoning', entry.max_reasoning_effort_multiplier)
  return rows
})

/** 分档：图片 / 视频按档位（每档一个按次价）；按 Token 的是区间分档（每段各自的单价或倍率） */
const tierRows = computed<Row[]>(() => {
  const entry = props.entry
  if (!entry) return []
  return sortedIntervals(entry).map((iv, index) => {
    if (iv.tier_label) {
      return {
        key: `tier-${index}`,
        label: iv.tier_label,
        value: iv.per_request_price == null ? '—' : perUnit(iv.per_request_price, entry.billing_mode)
      }
    }
    const label =
      iv.max_tokens == null
        ? t('admin.modelCatalog.drawer.tokenTierOpen', { min: compact(iv.min_tokens) })
        : t('admin.modelCatalog.drawer.tokenTier', { min: compact(iv.min_tokens), max: compact(iv.max_tokens) })
    return { key: `tier-${index}`, label, value: intervalPrices(iv) }
  })
})

function intervalPrices(iv: PricingInterval): string {
  const parts: string[] = []
  const add = (labelKey: string, price: number | null | undefined, multiplier: number | null | undefined) => {
    const label = t(`admin.modelCatalog.drawer.price.${labelKey}`)
    if (price != null) parts.push(`${label} ${money(perMillion(price))}`)
    else if (multiplier != null) parts.push(`${label} × ${multiplier}`)
  }
  add('input', iv.input_price, iv.input_multiplier)
  add('output', iv.output_price, iv.output_multiplier)
  add('cacheWrite', iv.cache_write_price, iv.cache_write_multiplier)
  add('cacheRead', iv.cache_read_price, iv.cache_read_multiplier)
  if (iv.per_request_price != null) parts.push(perUnit(iv.per_request_price, 'per_request'))
  return parts.join(' · ') || '—'
}

const timePricingHint = computed(() => {
  const tp = props.entry?.time_pricing
  if (!tp) return undefined
  const parts = [t('admin.modelCatalog.drawer.timezone', { timezone: tp.timezone })]
  if (tp.weekdays_only) parts.push(t('admin.modelCatalog.drawer.weekdaysOnly'))
  return parts.join(' · ')
})

// ---- 渠道：打开页签时做一次诊断请求，拿到名称、优先级与此刻能否调度 ----
interface ChannelRow {
  id: number
  name: string
  platform: string
  type: string
  vendor: string
  priority: number | null
  schedulable: boolean | null
  blockedReason?: string
}

const diagnosed = ref<ModelCatalogDiagnosisAccount[] | null>(null)
const channelsLoading = ref(false)
const channelsFailed = ref(false)
let channelsSeq = 0

async function loadChannels(entryId: number) {
  const seq = ++channelsSeq
  channelsLoading.value = true
  channelsFailed.value = false
  try {
    const diagnosis = await adminAPI.modelCatalog.diagnose(entryId)
    if (seq !== channelsSeq) return
    diagnosed.value = diagnosis.accounts ?? []
  } catch (error) {
    if (seq !== channelsSeq) return
    diagnosed.value = null
    channelsFailed.value = true
    console.error('Failed to load catalog entry channels:', error)
  } finally {
    if (seq === channelsSeq) channelsLoading.value = false
  }
}

/** 诊断拿到了就用诊断结果；拿不到就只列目录条目里的渠道 ID */
const channelRows = computed<ChannelRow[]>(() => {
  const entry = props.entry
  if (!entry) return []
  if (diagnosed.value) {
    return diagnosed.value.map((account) => ({
      id: account.id,
      name: account.name,
      platform: account.platform,
      type: account.type,
      vendor: account.vendor,
      priority: account.priority,
      schedulable: account.schedulable,
      blockedReason: account.blocked_reason
    }))
  }
  return (entry.bindings ?? []).map((binding) => ({
    id: binding.account_id,
    name: binding.account?.name ?? `#${binding.account_id}`,
    platform: binding.account?.platform ?? '',
    type: binding.account?.type ?? '',
    vendor: binding.account?.vendor ?? '',
    priority: binding.priority ?? null,
    schedulable: null
  }))
})

function blockedReasonLabel(reason?: string): string {
  if (!reason) return t('admin.modelCatalog.drawer.notSchedulable')
  const key = `admin.modelCatalog.diagnosis.reasons.${reason}`
  return te(key) ? t(key) : reason
}

// 打开、换条目、切到渠道页签、列表重载（编辑保存后条目换了新对象）：重新诊断
watch(
  () => [props.show, props.entry?.id, props.tab, props.entry] as const,
  ([show, entryId, tab], previous) => {
    if (!show || !entryId) return
    if (entryId !== previous?.[1]) diagnosed.value = null
    if (tab === 'channels' && bindingCount.value > 0) void loadChannels(entryId)
  },
  { immediate: true }
)
</script>
