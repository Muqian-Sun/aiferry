<template>
  <!--
    Token 构成（muqian 2026-09-24）：区间内输入 / 输出 / 缓存读 / 缓存写各占多少——一条 100% 横条，四段墨色由深到浅、段间留 2px 缝，
    下面图例写占比与数量；右侧缓存命中率 = 缓存读 ÷（输入 + 缓存读 + 缓存写）。四类 token 在记账时是互斥的（OpenAI 的输入已扣掉缓存部分）。
  -->
  <div v-if="loading" class="flex h-24 items-center justify-center">
    <span class="spinner text-af-ink-3" />
  </div>
  <div
    v-else-if="total > 0"
    class="grid gap-x-12 gap-y-6 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center"
    data-testid="token-composition"
  >
    <div class="min-w-0">
      <div class="flex h-3 gap-[2px] overflow-hidden rounded-full" role="img" :aria-label="ariaLabel">
        <span
          v-for="segment in visibleSegments"
          :key="segment.key"
          class="h-full min-w-[3px]"
          :class="segment.swatch"
          :style="{ flexGrow: segment.value, flexBasis: 0 }"
        />
      </div>
      <dl class="mt-4 grid grid-cols-2 gap-x-6 gap-y-3 sm:grid-cols-4">
        <div v-for="segment in segments" :key="segment.key" class="min-w-0">
          <dt class="flex items-center gap-2 text-13 text-af-ink-3">
            <span class="h-2 w-2 shrink-0 rounded-full" :class="segment.swatch" aria-hidden="true" />
            {{ segment.label }}
          </dt>
          <dd class="mt-1 truncate text-base font-semibold tabular-nums text-af-ink">
            {{ segment.percentText }}<span class="ml-1.5 text-13 font-normal text-af-ink-4">{{ formatTokensK(segment.value) }}</span>
          </dd>
        </div>
      </dl>
    </div>
    <div class="sm:border-l sm:border-af-hairline sm:pl-12" data-testid="cache-hit-rate">
      <p class="text-13 text-af-ink-3">{{ t('userUi.overview.composition.hitRate') }}</p>
      <p class="mt-2 text-[2rem] font-semibold leading-none tabular-nums text-af-ink">{{ hitRateText }}</p>
    </div>
  </div>
  <div v-else class="flex h-24 items-center justify-center text-sm text-af-ink-3">
    {{ t('userUi.overview.composition.empty') }}
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatTokensK } from '@/utils/format'

const props = defineProps<{
  totals: { input: number; output: number; cacheRead: number; cacheWrite: number }
  loading?: boolean
}>()

const { t } = useI18n()

const percent = (share: number) => `${(share * 100).toFixed(share > 0 && share < 0.1 ? 1 : 0)}%`

const total = computed(() => props.totals.input + props.totals.output + props.totals.cacheRead + props.totals.cacheWrite)

const segments = computed(() =>
  (
    [
      { key: 'input', value: props.totals.input, swatch: 'bg-af-ink' },
      { key: 'output', value: props.totals.output, swatch: 'bg-af-ink-2' },
      { key: 'cacheRead', value: props.totals.cacheRead, swatch: 'bg-af-ink-3' },
      { key: 'cacheWrite', value: props.totals.cacheWrite, swatch: 'bg-af-ink-4' }
    ] as const
  ).map((segment) => ({
    ...segment,
    label: t(`userUi.overview.composition.${segment.key}`),
    percentText: percent(total.value > 0 ? segment.value / total.value : 0)
  }))
)

const visibleSegments = computed(() => segments.value.filter((segment) => segment.value > 0))

const ariaLabel = computed(() => segments.value.map((segment) => `${segment.label} ${segment.percentText}`).join('，'))

/** 命中率的分母是全部输入侧 token（输入 + 缓存读 + 缓存写）；没有输入侧 token 时显示「—」 */
const hitRateText = computed(() => {
  const { input, cacheRead, cacheWrite } = props.totals
  const denominator = input + cacheRead + cacheWrite
  return denominator > 0 ? percent(cacheRead / denominator) : '—'
})
</script>
