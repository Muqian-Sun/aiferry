<template>
  <!--
    Token 构成（概览；muqian 2026-09-25 改成与模型用量同一种画法）：输入 / 输出 / 缓存读 / 缓存写各一行，墨色占比条 + 数量；
    下面一行缓存命中率 = 缓存读 ÷（输入 + 缓存读 + 缓存写）。四类 token 在记账时是互斥的（OpenAI 的输入已扣掉缓存部分）。
  -->
  <div v-if="loading" class="flex h-40 items-center justify-center">
    <span class="spinner text-af-ink-3" />
  </div>
  <div v-else-if="total > 0" data-testid="token-composition">
    <table class="w-full text-13">
      <thead>
        <tr class="border-b border-af-hairline text-left text-af-ink-3">
          <th class="py-2 pr-4 font-medium">{{ t('userUi.overview.composition.kind') }}</th>
          <th class="w-[45%] py-2 pr-4 font-medium">{{ t('userUi.overview.share') }}</th>
          <th class="py-2 text-right font-medium">{{ t('userUi.usage.stats.tokens') }}</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-af-hairline">
        <tr v-for="segment in segments" :key="segment.key" class="h-11">
          <td class="pr-4 text-af-ink">{{ segment.label }}</td>
          <td class="pr-4"><ShareBar :value="segment.value" :total="total" /></td>
          <td class="text-right tabular-nums text-af-ink-2">{{ formatTokensK(segment.value) }}</td>
        </tr>
      </tbody>
    </table>
    <div class="mt-4 flex items-baseline justify-between gap-4 border-t border-af-hairline pt-4" data-testid="cache-hit-rate">
      <span class="text-13 text-af-ink-3">{{ t('userUi.overview.composition.hitRate') }}</span>
      <span class="text-2xl font-semibold text-af-ink">{{ hitRateText }}</span>
    </div>
  </div>
  <div v-else class="flex h-40 items-center justify-center text-sm text-af-ink-3">
    {{ t('userUi.overview.composition.empty') }}
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import ShareBar from '@/components/charts/ShareBar.vue'
import { formatTokensK } from '@/utils/format'

const props = defineProps<{
  totals: { input: number; output: number; cacheRead: number; cacheWrite: number }
  loading?: boolean
}>()

const { t } = useI18n()

const percent = (share: number) => `${(share * 100).toFixed(share > 0 && share < 0.1 ? 1 : 0)}%`

const total = computed(() => props.totals.input + props.totals.output + props.totals.cacheRead + props.totals.cacheWrite)

const segments = computed(() =>
  (['input', 'output', 'cacheRead', 'cacheWrite'] as const).map((key) => ({
    key,
    value: props.totals[key],
    label: t(`userUi.overview.composition.${key}`)
  }))
)

/** 命中率的分母是全部输入侧 token（输入 + 缓存读 + 缓存写）；没有输入侧 token 时显示「—」 */
const hitRateText = computed(() => {
  const { input, cacheRead, cacheWrite } = props.totals
  const denominator = input + cacheRead + cacheWrite
  return denominator > 0 ? percent(cacheRead / denominator) : '—'
})
</script>
