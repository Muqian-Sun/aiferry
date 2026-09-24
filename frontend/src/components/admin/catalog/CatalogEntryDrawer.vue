<template>
  <!--
    模型详情抽屉（A5）：模型目录点行打开，看完即关。两个页签：
    概况（标识、厂商、计费、上架、维护方、别名、全部价格与分档 / 分时）、渠道（绑定的渠道此刻能否调度 + 诊断）。
    改配置点「编辑」，上下架 / 诊断 / 删除在「⋯」里；动作都 emit 给目录页，由目录页弹原有的对话框。
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
        <MenuItem icon="beaker" data-testid="model-catalog-drawer-diagnose" @click="emit('diagnose')">
          {{ t('admin.modelCatalog.diagnose') }}
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
          <DetailField :label="t('admin.modelCatalog.fields.vendor')" :value="entry.vendor" />
          <DetailField :label="t('admin.modelCatalog.fields.billingMode')" :value="t(`admin.modelCatalog.billingModes.${entry.billing_mode || 'token'}`)" />
          <DetailField :label="t('admin.modelCatalog.fields.status')">
            <span class="inline-flex items-center gap-1.5">
              <span :class="['inline-block h-2 w-2 rounded-full', entry.status === 'listed' ? 'bg-af-ink-3' : 'bg-af-hairline-strong']"></span>
              <span :class="entry.status === 'listed' ? 'text-af-ink' : 'text-af-ink-3'">{{ t(`admin.modelCatalog.status.${entry.status}`) }}</span>
            </span>
          </DetailField>
          <DetailField :label="t('admin.modelCatalog.fields.managedBy')" :value="t(`admin.modelCatalog.managedBy.${entry.managed_by}`)" />
          <DetailField :label="t('admin.modelCatalog.drawer.protocols')" :value="(entry.protocols ?? []).join(' · ')" />
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

        <SheetSection v-if="tierRows.length" :title="t('admin.modelCatalog.drawer.tiers')">
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

      <!-- 渠道：绑定的渠道此刻能否调度（一次诊断请求拿全）；协议逐格看「诊断」 -->
      <SheetSection
        v-else-if="tab === 'channels'"
        :title="t('admin.modelCatalog.bindings.title')"
        :description="t('admin.modelCatalog.bindings.hint')"
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
                <p class="mt-0.5 truncate text-xs text-af-ink-3">
                  #{{ channel.id }}<template v-if="channel.platform"> · {{ channel.platform }} / {{ channel.type }}</template>
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
import { formatCatalogPrice } from '@/components/modelPlaza/catalog'
import { formatCompactNumber, formatDateTime } from '@/utils/format'
import Icon from '@/components/icons/Icon.vue'
import SheetSection from '@/components/user/shell/SheetSection.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import type { SectionTab } from '@/components/user/shell/types'
import { DetailDrawer, DetailField, MenuItem, PopoverMenu } from '@/components/admin/list'
import { hasPrice } from './entryRequest'

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

const subtitle = computed(() => {
  const entry = props.entry
  if (!entry) return ''
  return [entry.display_name, entry.vendor].filter(Boolean).join(' · ')
})

const banners = computed(() => {
  const entry = props.entry
  if (!entry || entry.status !== 'listed') return []
  const list: { key: string; tone: 'danger' | 'warning'; text: string }[] = []
  if (!hasPrice(entry)) list.push({ key: 'unpriced', tone: 'danger', text: t('admin.modelCatalog.drawer.unpricedBanner') })
  if (bindingCount.value === 0) list.push({ key: 'unbound', tone: 'warning', text: t('admin.modelCatalog.drawer.unboundBanner') })
  return list
})

// ---- 价格 ----
interface Row {
  key: string
  label: string
  value: string
}

/** 目录里按 Token 的价是 $/token，展示换成 $/百万 Token */
function perMillion(value: number | null | undefined): string {
  return value == null ? '—' : formatCatalogPrice(value * 1_000_000)
}

/** 200000 → 200K（去掉 formatCompactNumber 的「.0」） */
function compact(value: number): string {
  return formatCompactNumber(value).replace(/\.0(?=[KMB]?$)/, '')
}

function perCall(value: number): string {
  return `$${Number(value.toFixed(6))}`
}

const priceRows = computed<Row[]>(() => {
  const entry = props.entry
  if (!entry) return []
  const rows: Row[] = []
  const push = (key: string, label: string, value: string) => rows.push({ key, label, value })
  const optional = (key: string, labelKey: string, value: number | null | undefined) => {
    if (value != null) push(key, t(`admin.modelCatalog.drawer.price.${labelKey}`), perMillion(value))
  }

  if (isToken.value) {
    // 输入 / 输出始终列出（没配就是「—」）；其余单价配了才列
    push('input', t('admin.modelCatalog.drawer.price.input'), perMillion(entry.input_price))
    push('output', t('admin.modelCatalog.drawer.price.output'), perMillion(entry.output_price))
    optional('cache_write', 'cacheWrite', entry.cache_write_price)
    optional('cache_write_1h', 'cacheWrite1h', entry.cache_write_1h_price)
    optional('cache_read', 'cacheRead', entry.cache_read_price)
    optional('image_input', 'imageInput', entry.image_input_price)
    optional('image_output', 'imageOutput', entry.image_output_price)
    optional('image_cache_read', 'imageCacheRead', entry.image_cache_read_price)
    optional('audio_input', 'audioInput', entry.audio_input_price)
    optional('audio_output', 'audioOutput', entry.audio_output_price)
    optional('input_priority', 'inputPriority', entry.input_price_priority)
    optional('output_priority', 'outputPriority', entry.output_price_priority)
    optional('cache_write_priority', 'cacheWritePriority', entry.cache_write_price_priority)
    optional('cache_read_priority', 'cacheReadPriority', entry.cache_read_price_priority)
    if (entry.per_request_price != null) {
      push('per_request', t('admin.modelCatalog.drawer.price.perRequest'), t('admin.modelCatalog.drawer.price.perCall', { price: perCall(entry.per_request_price) }))
    }
    if (entry.search_price_per_call != null) {
      push('search', t('admin.modelCatalog.drawer.price.searchPerCall'), t('admin.modelCatalog.drawer.price.perCall', { price: perCall(entry.search_price_per_call) }))
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
    const unit = t(`admin.modelCatalog.columns.perUnit.${entry.billing_mode}`)
    push(
      'per_request',
      t('admin.modelCatalog.drawer.price.defaultPrice'),
      entry.per_request_price == null ? '—' : `${perCall(entry.per_request_price)} · ${unit}`
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
  const intervals = [...(entry.intervals ?? [])].sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0))
  return intervals.map((iv, index) => {
    if (iv.tier_label) {
      return { key: `tier-${index}`, label: iv.tier_label, value: iv.per_request_price == null ? '—' : perCall(iv.per_request_price) }
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
    if (price != null) parts.push(`${label} ${perMillion(price)}`)
    else if (multiplier != null) parts.push(`${label} × ${multiplier}`)
  }
  add('input', iv.input_price, iv.input_multiplier)
  add('output', iv.output_price, iv.output_multiplier)
  add('cacheWrite', iv.cache_write_price, iv.cache_write_multiplier)
  add('cacheRead', iv.cache_read_price, iv.cache_read_multiplier)
  if (iv.per_request_price != null) parts.push(t('admin.modelCatalog.drawer.price.perCall', { price: perCall(iv.per_request_price) }))
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
