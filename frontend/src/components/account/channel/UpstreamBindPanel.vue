<template>
  <!--
    建渠道「承接模型」那一步的上游名单（muqian 2026-10-06：承接时列的是整个目录，不是上游这个协议有的模型）。
    名单来自「检测上游」选定协议拉到的 /models，分三组：
    - 目录里有的（含相近名）：默认勾上，勾选 = 承接（下面那一块加一行，名字不同的填上游模型名）；
    - 目录里没有、是官方模型 ID 的：加进目录（有官方价直接加，读不出价的打开新建弹窗填价），加好就到第一组；
    - 目录里没有、不是官方 ID 的：映射到目录里的某个模型（这个渠道的上游模型名），给一个建议由管理员确认。
    联网名单拉不到时判断不了是不是官方，两种做法都给。整个目录的「添加模型」留在下面那一块里，名单不全时用。
  -->
  <section class="space-y-4" data-testid="upstream-bind-panel">
    <header class="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
      <h3 class="text-sm font-semibold text-af-ink">{{ t('admin.accounts.upstreamBind.title') }}</h3>
      <span class="text-13 text-af-ink-3">{{ t('admin.accounts.upstreamBind.meta', { protocol: protocolLabel, count: names.length }) }}</span>
    </header>

    <!-- 目录里有的 -->
    <div v-if="matched.length > 0" class="space-y-1.5" data-testid="upstream-bind-matched">
      <p class="text-xs font-medium text-af-ink-2">{{ t('admin.accounts.upstreamBind.matched', { count: matched.length }) }}</p>
      <div class="flex flex-wrap gap-x-5 gap-y-1.5">
        <label
          v-for="match in matched"
          :key="match.entry.id"
          class="inline-flex items-center gap-1.5 text-13"
          :class="bindable(match.entry.id) ? 'cursor-pointer' : 'cursor-not-allowed opacity-60'"
          :title="bindable(match.entry.id) ? '' : t('admin.accounts.upstreamBind.notBindable')"
          :data-testid="`upstream-bind-matched-${match.entry.model_id}`"
        >
          <input
            type="checkbox"
            :checked="isBound(match.entry.id)"
            :disabled="!bindable(match.entry.id)"
            @change="emit('toggle', match, ($event.target as HTMLInputElement).checked)"
          />
          <span class="font-mono text-af-ink">{{ match.entry.model_id }}</span>
          <span v-if="match.upstreamName !== match.entry.model_id" class="text-xs text-af-ink-3">
            {{ t('admin.accounts.upstreamBind.upstreamName', { name: match.upstreamName }) }}
          </span>
          <span v-if="match.entry.status !== 'listed'" class="text-xs text-af-ink-3">{{ t('admin.accounts.upstreamBind.unlisted') }}</span>
        </label>
      </div>
      <p v-if="hasUnlistedBound" class="text-xs text-af-ink-3">{{ t('admin.accounts.upstreamBind.unlistedHint') }}</p>
    </div>

    <p v-if="lookupState === 'loading'" class="text-13 text-af-ink-3">{{ t('admin.accounts.upstreamBind.lookupLoading') }}</p>

    <!-- 目录里没有、是官方模型 ID 的 -->
    <div v-if="plan.official.length > 0" class="space-y-1.5 border-t border-af-hairline pt-3" data-testid="upstream-bind-official">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <p class="text-xs font-medium text-af-ink-2">{{ t('admin.accounts.upstreamBind.official', { count: plan.official.length }) }}</p>
        <button
          v-if="pricedOfficial.length > 1"
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="busy"
          data-testid="upstream-bind-official-all"
          @click="emit('addAllOfficial', pricedOfficial)"
        >
          {{ t('admin.accounts.upstreamBind.addAll', { count: pricedOfficial.length }) }}
        </button>
      </div>
      <div v-for="candidate in plan.official" :key="candidate.name" class="flex flex-wrap items-center gap-x-3 gap-y-1 text-13">
        <span class="font-mono text-af-ink">{{ candidate.name }}</span>
        <span class="text-xs text-af-ink-3">{{ candidate.priced ? officialPriceText(candidate) : t('admin.accounts.upstreamBind.officialNoPrice') }}</span>
        <button type="button" class="text-13 text-af-brand hover:text-af-brand-hover disabled:opacity-50" :disabled="busy" @click="emit('addOfficial', candidate)">
          {{ candidate.priced ? t('admin.accounts.upstreamBind.addToCatalog') : t('admin.accounts.upstreamBind.addWithPrice') }}
        </button>
      </div>
    </div>

    <!-- 目录里没有、不是官方 ID 的；联网名单拉不到时的也在这里，多一个「加进目录」 -->
    <div v-if="mappable.length > 0" class="space-y-1.5 border-t border-af-hairline pt-3" data-testid="upstream-bind-unofficial">
      <p class="text-xs font-medium text-af-ink-2">
        {{ undeterminedMode ? t('admin.accounts.upstreamBind.undetermined', { count: mappable.length }) : t('admin.accounts.upstreamBind.unofficial', { count: mappable.length }) }}
      </p>
      <p class="text-xs text-af-ink-3">{{ undeterminedMode ? t('admin.accounts.upstreamBind.undeterminedHint') : t('admin.accounts.upstreamBind.unofficialHint') }}</p>
      <div
        v-for="candidate in mappable"
        :key="candidate.name"
        class="flex flex-wrap items-center gap-x-3 gap-y-1 text-13"
        :data-testid="`upstream-bind-map-${candidate.name}`"
      >
        <span class="font-mono text-af-ink">{{ candidate.name }}</span>
        <template v-if="mappedEntry(candidate.name)">
          <span class="text-af-success">{{ t('admin.accounts.upstreamBind.mappedTo', { model: mappedEntry(candidate.name)?.model_id }) }}</span>
          <button type="button" class="text-13 text-af-ink-3 hover:text-af-danger" @click="emit('unmap', candidate.name)">{{ t('admin.accounts.upstreamBind.undo') }}</button>
        </template>
        <template v-else>
          <span class="text-xs text-af-ink-3">{{ t('admin.accounts.upstreamBind.mapTo') }}</span>
          <select
            v-model="mapChoice[candidate.name]"
            class="input h-8 w-56 px-2 py-1 font-mono text-13"
            :aria-label="t('admin.accounts.upstreamBind.mapTo')"
          >
            <option value="">{{ t('admin.accounts.upstreamBind.pickModel') }}</option>
            <option v-for="entry in mapTargets" :key="entry.id" :value="entry.id">{{ entry.model_id }}</option>
          </select>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="!mapChoice[candidate.name]"
            @click="emit('map', candidate.name, Number(mapChoice[candidate.name]))"
          >
            {{ t('admin.accounts.upstreamBind.mapAction') }}
          </button>
          <button v-if="undeterminedMode" type="button" class="text-13 text-af-brand hover:text-af-brand-hover" @click="emit('addOfficial', { name: candidate.name, priced: false, entry: null })">
            {{ t('admin.accounts.upstreamBind.asOfficial') }}
          </button>
        </template>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ModelCatalogEntry, OfficialModelLookupResult } from '@/api/admin/modelCatalog'
import { DETAIL_PRICE_MAX_DECIMALS, formatListPrice, perMillion, sharedPriceDecimals } from '@/components/admin/catalog/priceFormat'
import {
  classifyUpstreamModels,
  planMissingModels,
  type DetectedMatch,
  type OfficialCandidate
} from './upstreamModels'

const props = defineProps<{
  /** 选定协议的显示名 */
  protocolLabel: string
  /** 上游名单（选定协议拉到的） */
  names: string[]
  /** 全部目录条目 */
  catalog: ModelCatalogEntry[]
  /** 这个渠道能承接的目录条目 */
  bindableIds: number[]
  /** 下面那一块当前的承接行（条目 + 上游模型名） */
  rows: Array<{ id: number; upstreamModel: string }>
  /** 官方 ID 查询结果；拉不到为 null */
  lookup: OfficialModelLookupResult | null
  lookupState: 'idle' | 'loading' | 'done'
  /** 正在加进目录 */
  busy: boolean
}>()

const emit = defineEmits<{
  toggle: [match: DetectedMatch, checked: boolean]
  map: [name: string, entryId: number]
  unmap: [name: string]
  addOfficial: [candidate: OfficialCandidate]
  addAllOfficial: [candidates: OfficialCandidate[]]
}>()

const { t } = useI18n()

const detected = computed(() => classifyUpstreamModels(props.names, props.catalog))
const matched = computed(() => [...detected.value.listed, ...detected.value.unlisted])
const bindableSet = computed(() => new Set(props.bindableIds))
const plan = computed(() =>
  planMissingModels(detected.value.missing, props.lookupState === 'done' ? props.lookup : null, props.catalog)
)
// 查询还没回来时先不分组，免得同一个名字先进「判断不了」再跳走
const undeterminedMode = computed(() => props.lookupState === 'done' && !props.lookup?.available)
const mappable = computed(() => (props.lookupState !== 'done' ? [] : [...plan.value.unofficial, ...plan.value.undetermined]))
const pricedOfficial = computed(() => plan.value.official.filter((candidate) => candidate.priced))

function bindable(entryId: number): boolean {
  return bindableSet.value.has(entryId)
}

function isBound(entryId: number): boolean {
  return props.rows.some((row) => row.id === entryId)
}

const hasUnlistedBound = computed(() => matched.value.some((match) => match.entry.status !== 'listed' && isBound(match.entry.id)))

// 映射目标：这个渠道能承接、还没承接的目录模型（已经映射过的那个也留着，换名字时能看到）
const mapTargets = computed(() =>
  props.catalog
    .filter((entry) => bindable(entry.id) && !isBound(entry.id))
    .sort((a, b) => a.model_id.localeCompare(b.model_id))
)

function mappedEntry(name: string): ModelCatalogEntry | null {
  const row = props.rows.find((item) => item.upstreamModel === name)
  return row ? props.catalog.find((entry) => entry.id === row.id) ?? null : null
}

// 每个名字的映射选择，默认是建议的那个（建议的已经承接了就不预选）
const mapChoice = reactive<Record<string, number | ''>>({})
watch(
  mappable,
  (candidates) => {
    for (const candidate of candidates) {
      if (candidate.name in mapChoice) continue
      const suggestion = candidate.suggestion
      mapChoice[candidate.name] = suggestion && bindable(suggestion.id) && !isBound(suggestion.id) ? suggestion.id : ''
    }
  },
  { immediate: true }
)

function officialPriceText(candidate: OfficialCandidate): string {
  const entry = candidate.entry
  if (!entry) return ''
  const input = perMillion(entry.input_price)
  const output = perMillion(entry.output_price)
  const decimals = sharedPriceDecimals([input, output], DETAIL_PRICE_MAX_DECIMALS)
  return t('admin.accounts.upstreamBind.officialPrice', {
    input: formatListPrice(input, decimals),
    output: formatListPrice(output, decimals)
  })
}
</script>
