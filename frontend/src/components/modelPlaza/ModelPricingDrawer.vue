<template>
  <!--
    模型计费详情（muqian 2026-09-30 方案 A）：网格格子只放摘要，点格子在这里按块列全部计费项。
    token 模式：标准价（分段 × 项的表）→ 图片与音频 → 工具 → 其他（最高推理档 / 分时 / 别名）；
    按次 / 图片 / 视频模式：单价（有档位列档位表）→ 工具 → 其他。
    只渲染有数据的块，块之间 hairline 分隔，块标题右侧写单位；价格 = 官方价 × 访问者倍率（scale）。
  -->
  <DetailDrawer
    :show="entry !== null"
    :title="entry?.id ?? ''"
    :eyebrow="entry ? vendorLabel(entry.vendor) : ''"
    :subtitle="entry ? getBillingModeLabel(entry.billingMode, t) : ''"
    @close="emit('close')"
  >
    <div v-if="entry" class="divide-y divide-af-hairline" data-testid="model-pricing-detail">
      <!--
        标准价：该模型实际有的项，分段模型一段一行。
        给最小宽度，窄屏时表格在容器里横向滚动而不是挤压数字。
      -->
      <section
        v-for="block in tokenBlocks"
        :key="block.key"
        class="py-6 first:pt-0"
        :data-testid="`pricing-block-${block.key}`"
      >
        <div class="mb-3 flex items-baseline justify-between gap-4">
          <h3 class="text-13 font-semibold text-af-ink">{{ block.title }}</h3>
          <span class="shrink-0 text-xs text-af-ink-4">{{ t('userUi.models.detail.unitPerMillion') }}</span>
        </div>
        <p v-if="block.hint" class="-mt-1 mb-3 text-xs text-af-ink-3">{{ block.hint }}</p>
        <div class="overflow-x-auto">
          <table class="w-full table-fixed text-13 tabular-nums" :style="{ minWidth: tableMinWidth }">
            <thead>
              <tr class="border-b border-af-hairline text-af-ink-4">
                <th v-if="segmented" scope="col" class="w-[5.5rem] whitespace-nowrap pb-2 pr-3 text-left align-bottom font-normal">
                  {{ t('userUi.models.segmentRange') }}
                </th>
                <th v-for="column in columns" :key="column.key" scope="col" class="pb-2 pl-2 text-right align-bottom font-normal first:pl-0">
                  {{ column.label }}
                  <span v-if="column.ttl" class="block">{{ column.ttl }}</span>
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-af-hairline">
              <tr v-for="row in block.rows" :key="row.min">
                <th v-if="segmented" scope="row" class="whitespace-nowrap py-2 pr-3 text-left font-normal text-af-ink-3">
                  {{ formatSegmentRange(row) }}
                </th>
                <td
                  v-for="column in columns"
                  :key="column.key"
                  class="whitespace-nowrap py-2 pl-2 text-right font-medium text-af-ink first:pl-0"
                  :data-testid="`${block.key}-${column.key}`"
                >
                  {{ formatPrice(row.prices[column.key]) }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section v-if="mediaItems.length" class="py-6 first:pt-0" data-testid="pricing-block-media">
        <div class="mb-3 flex items-baseline justify-between gap-4">
          <h3 class="text-13 font-semibold text-af-ink">{{ t('userUi.models.detail.media') }}</h3>
          <span class="shrink-0 text-xs text-af-ink-4">{{ t('userUi.models.detail.unitPerMillion') }}</span>
        </div>
        <dl class="divide-y divide-af-hairline">
          <DetailField v-for="item in mediaItems" :key="item.key" :label="item.label">
            <span class="font-medium tabular-nums">{{ formatPrice(item.value) }}</span>
          </DetailField>
        </dl>
      </section>

      <!-- 按次 / 图片 / 视频：默认单价，有档位就列档位表（没匹配到档位时按默认单价） -->
      <section v-if="unitBlock" class="py-6 first:pt-0" data-testid="pricing-block-unit">
        <div class="mb-3 flex items-baseline justify-between gap-4">
          <h3 class="text-13 font-semibold text-af-ink">{{ t('userUi.models.detail.unitPrice') }}</h3>
          <span class="shrink-0 text-xs text-af-ink-4">{{ unitBlock.unit }}</span>
        </div>
        <div v-if="unitBlock.tiers.length" class="overflow-x-auto">
          <table class="w-full text-13 tabular-nums">
            <thead>
              <tr class="border-b border-af-hairline text-af-ink-4">
                <th scope="col" class="pb-2 pr-3 text-left font-normal">{{ t('userUi.models.detail.tier') }}</th>
                <th scope="col" class="pb-2 pl-3 text-right font-normal">{{ unitBlock.label }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-af-hairline">
              <tr v-for="tier in unitBlock.tiers" :key="tier.label">
                <th scope="row" class="py-2 pr-3 text-left font-normal text-af-ink-3">{{ tier.label }}</th>
                <td class="whitespace-nowrap py-2 pl-3 text-right font-medium text-af-ink">{{ formatPrice(tier.price) }}</td>
              </tr>
              <tr v-if="unitBlock.price != null">
                <th scope="row" class="py-2 pr-3 text-left font-normal text-af-ink-3">{{ t('userUi.models.detail.defaultTier') }}</th>
                <td class="whitespace-nowrap py-2 pl-3 text-right font-medium text-af-ink">{{ formatPrice(unitBlock.price) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <dl v-else class="divide-y divide-af-hairline">
          <DetailField :label="unitBlock.label">
            <span class="font-medium tabular-nums" data-testid="unit-price">{{ formatPrice(unitBlock.price) }}</span>
          </DetailField>
        </dl>
      </section>

      <!-- 联网搜索按官方原价收，不乘账户倍率（方案第三版）；非 Anthropic 模型另列 Claude Code 联网搜索的计费项 -->
      <section v-if="searchPerThousand != null || claudeCodeSearch" class="py-6 first:pt-0" data-testid="pricing-block-tools">
        <div class="mb-3 flex items-baseline justify-between gap-4">
          <h3 class="text-13 font-semibold text-af-ink">{{ t('userUi.models.detail.tools') }}</h3>
        </div>
        <dl class="divide-y divide-af-hairline">
          <DetailField v-if="searchPerThousand != null" :label="t('userUi.models.detail.search')">
            <span class="font-medium tabular-nums">{{ t('userUi.models.detail.perThousandCalls', { price: formatPrice(searchPerThousand) }) }}</span>
            <span class="block text-xs text-af-ink-3">{{ t('userUi.models.detail.searchOfficialPrice') }}</span>
          </DetailField>
          <DetailField v-if="entry?.xPostPerThousand != null" :label="t('userUi.models.detail.xPosts')">
            <span class="font-medium tabular-nums">{{ t('userUi.models.detail.perThousandPosts', { price: formatPrice(entry.xPostPerThousand) }) }}</span>
          </DetailField>
          <DetailField v-if="entry?.xUserPerThousand != null" :label="t('userUi.models.detail.xUsers')">
            <span class="font-medium tabular-nums">{{ t('userUi.models.detail.perThousandUsers', { price: formatPrice(entry.xUserPerThousand) }) }}</span>
          </DetailField>
          <DetailField v-if="claudeCodeSearch" :label="t('userUi.models.detail.claudeCodeSearch')" data-testid="pricing-claude-code-search">
            <span class="font-medium tabular-nums">{{
              t('userUi.models.detail.claudeCodeSearchPrice', {
                input: formatPrice(claudeCodeSearch.input),
                output: formatPrice(claudeCodeSearch.output),
                search: formatPrice(claudeCodeSearch.perThousand)
              })
            }}</span>
            <span class="block text-xs text-af-ink-3">{{ t('userUi.models.detail.claudeCodeSearchNote') }}</span>
          </DetailField>
        </dl>
      </section>

      <section v-if="hasOther" class="py-6 first:pt-0" data-testid="pricing-block-other">
        <div class="mb-3 flex items-baseline justify-between gap-4">
          <h3 class="text-13 font-semibold text-af-ink">{{ t('userUi.models.detail.other') }}</h3>
        </div>
        <dl class="divide-y divide-af-hairline">
          <DetailField
            v-if="entry.maxReasoningMultiplier != null"
            :label="t('userUi.models.detail.maxReasoning')"
            :value="t('userUi.models.detail.maxReasoningRule', { multiplier: entry.maxReasoningMultiplier })"
          />
          <DetailField v-if="entry.timePricing" :label="t('userUi.models.detail.timePricing')">
            <ul class="space-y-0.5 tabular-nums">
              <li v-for="period in entry.timePricing.periods" :key="period.start_time">
                {{ period.start_time }}–{{ period.end_time }} × {{ period.multiplier }}
              </li>
            </ul>
            <p class="mt-1 text-xs text-af-ink-3">{{ timePricingScope }}</p>
          </DetailField>
          <DetailField v-if="entry.aliases.length" :label="t('userUi.models.detail.aliases')">
            <span class="flex flex-wrap gap-1.5">
              <span v-for="alias in entry.aliases" :key="alias" class="badge badge-gray font-mono">{{ alias }}</span>
            </span>
          </DetailField>
        </dl>
      </section>

      <p v-if="!hasPricing" class="py-6 text-13 text-af-ink-3 first:pt-0" data-testid="pricing-empty">
        {{ t('userUi.models.detail.noPricing') }}
      </p>

      <p v-if="footnote" class="pt-5 text-xs leading-5 text-af-ink-3" data-testid="pricing-footnote">{{ footnote }}</p>
    </div>
  </DetailDrawer>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import DetailDrawer from '@/components/common/DetailDrawer.vue'
import DetailField from '@/components/common/DetailField.vue'
import { getBillingModeLabel } from '@/utils/billingMode'
import { formatSegmentRange, type TokenSegment } from '@/utils/tokenSegments'
import type { PlazaWebSearchBilling } from '@/api/modelPlaza'
import {
  applyMultiplier,
  formatCatalogPrice as formatPrice,
  scaleRows,
  TOKEN_ROW_KEYS,
  vendorLabel,
  type CatalogModel,
  type CatalogTier,
  type TokenRowKey
} from './catalog'

const props = withDefaults(
  defineProps<{
    entry: CatalogModel | null
    /** 价格乘的系数：接口给的是官方价，展示价 = 官方价 × 访问者倍率（登录用账户倍率，未登录用全站默认 1/15） */
    scale: number
    /** 用 Claude Code 配非 Anthropic 模型时那次搜索请求的计费项（官方价）；没有时为 null */
    claudeCodeWebSearch?: PlazaWebSearchBilling | null
  }>(),
  { claudeCodeWebSearch: null }
)

const emit = defineEmits<{ (e: 'close'): void }>()

const { t } = useI18n()

const segmented = computed(() => (props.entry?.rows.length ?? 0) > 1)

/**
 * 表格的列：该模型实际有的项（任一段有价）。有 1 小时缓存写时两列缓存写都注明时长（「缓存写」下一行「5 分钟」/「1 小时」），
 * 分两行写，窄列里不会从词中间折行。
 */
const columns = computed<Array<{ key: TokenRowKey; label: string; ttl: string }>>(() => {
  const rows = props.entry?.rows ?? []
  const present = TOKEN_ROW_KEYS.filter((key) => rows.some((row) => row.prices[key] != null))
  const has1h = present.includes('cacheWrite1h')
  const columnOf: Record<TokenRowKey, { label: string; ttl: string }> = {
    input: { label: t('userUi.models.prices.input'), ttl: '' },
    output: { label: t('userUi.models.prices.output'), ttl: '' },
    cacheWrite: { label: t('userUi.models.prices.cacheWrite'), ttl: has1h ? t('userUi.models.detail.ttl5m') : '' },
    cacheWrite1h: { label: t('userUi.models.prices.cacheWrite'), ttl: t('userUi.models.detail.ttl1h') },
    cacheRead: { label: t('userUi.models.prices.cacheRead'), ttl: '' }
  }
  return present.map((key) => ({ key, ...columnOf[key] }))
})

/**
 * 分段列 5.5rem（放得下「128K–200K」）+ 每项 3.875rem（最长的价「$0.0333」13px 约 54px + 左内边距 8px）：
 * 不分段的五项表在 375px 手机上不用横滑，再窄或分段多列时表格在容器里横滑。
 */
const tableMinWidth = computed(() => `${(segmented.value ? 5.5 : 0) + columns.value.length * 3.875}rem`)

const tokenBlocks = computed<Array<{ key: 'standard'; title: string; hint: string; rows: TokenSegment[] }>>(() => {
  const entry = props.entry
  if (!entry || entry.rows.length === 0) return []
  return [{ key: 'standard', title: t('userUi.models.detail.standard'), hint: '', rows: scaleRows(entry.rows, props.scale) }]
})

const mediaItems = computed(() => {
  const price = applyMultiplier(props.entry?.price ?? null, props.scale)
  if (!price) return []
  const items = [
    { key: 'imageInput', label: t('userUi.models.prices.imageInput'), value: price.imageInput },
    { key: 'imageOutput', label: t('userUi.models.prices.imageOutput'), value: price.imageOutput },
    { key: 'imageCacheRead', label: t('userUi.models.prices.imageCacheRead'), value: price.imageCacheRead },
    { key: 'audioInput', label: t('userUi.models.prices.audioInput'), value: price.audioInput },
    { key: 'audioOutput', label: t('userUi.models.prices.audioOutput'), value: price.audioOutput }
  ]
  return items.filter((item) => item.value != null)
})

/** 非 token 模式的单价块：单位与项名随计费模式（次 / 张 / 秒） */
const unitBlock = computed<{ label: string; unit: string; price: number | null; tiers: CatalogTier[] } | null>(() => {
  const entry = props.entry
  if (!entry || entry.billingMode === 'token') return null
  if (entry.unitPrice == null && entry.tiers.length === 0) return null
  const scale = (value: number | null) => (value == null ? null : value * props.scale)
  const [label, unit] =
    entry.billingMode === 'image'
      ? [t('userUi.models.prices.perImage'), t('userUi.models.detail.unitPerImage')]
      : entry.billingMode === 'video'
        ? [t('userUi.models.prices.perSecond'), t('userUi.models.detail.unitPerSecond')]
        : [t('userUi.models.prices.perRequest'), t('userUi.models.detail.unitPerRequest')]
  return {
    label,
    unit,
    price: scale(entry.unitPrice),
    tiers: entry.tiers.map((tier) => ({ label: tier.label, price: scale(tier.price) }))
  }
})

/** 搜索费按官方原价收，不乘账户倍率 */
const searchPerThousand = computed(() => props.entry?.searchPerThousand ?? null)

/** Claude Code 联网搜索（只给非 Anthropic 模型）：token 价 × 访问者倍率、按每百万 Token；每次搜索按原价、按每千次 */
const claudeCodeSearch = computed(() => {
  const billing = props.claudeCodeWebSearch
  const entry = props.entry
  if (!billing || !entry || entry.vendor === 'anthropic' || entry.vendor === 'bedrock') return null
  const perMillion = (value: number | null) => (value == null ? null : value * 1_000_000 * props.scale)
  return { input: perMillion(billing.input_price), output: perMillion(billing.output_price), perThousand: billing.search_price_per_call * 1000 }
})

const hasOther = computed(() => {
  const entry = props.entry
  return !!entry && (entry.maxReasoningMultiplier != null || entry.timePricing != null || entry.aliases.length > 0)
})

const hasPricing = computed(
  () =>
    tokenBlocks.value.length > 0 ||
    mediaItems.value.length > 0 ||
    unitBlock.value !== null ||
    searchPerThousand.value != null ||
    claudeCodeSearch.value != null
)

const timePricingScope = computed(() => {
  const timePricing = props.entry?.timePricing
  if (!timePricing) return ''
  return timePricing.weekdays_only ? `${timePricing.timezone} · ${t('userUi.models.weekdaysOnly')}` : timePricing.timezone
})

/** 脚注：分段规则（有分段时） */
const footnote = computed(() => (segmented.value ? t('userUi.models.segmentNote') : ''))
</script>
