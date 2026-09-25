<template>
  <section class="mx-auto w-full max-w-6xl space-y-5 px-1 py-2 sm:px-2">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <p class="min-w-0 max-w-3xl text-sm text-af-ink-3">
        {{ t('channelMonitorV2.settings.description') }}
      </p>
      <button
        type="button"
        class="btn btn-primary"
        :disabled="saving || !dirty"
        @click="save"
      >
        <Icon name="check" size="sm" />
        {{ t('channelMonitorV2.settings.save') }}
      </button>
    </header>

    <div
      v-if="!featureEnabled"
      class="rounded-lg border border-af-warning/30 bg-af-warning-tint/90 px-4 py-3 text-sm text-af-warning"
      role="status"
    >
      {{ t('channelMonitorV2.settings.disabledBanner') }}
      <router-link class="ml-1 font-medium underline" to="/settings/features">{{ t('admin.featureOff.goSettings') }}</router-link>
    </div>

    <div
      v-if="loading"
      class="card flex min-h-[200px] items-center justify-center !rounded-lg !border-0 text-sm text-af-ink-3 ring-1 ring-af-hairline"
    >
      <span class="animate-pulse">{{ t('channelMonitorV2.settings.loading') }}</span>
    </div>

    <template v-else-if="draft">
      <!-- 用户端展示（A6-4 从设置页挪来）：存在全局设置里，保存时只发这两项 -->
      <div
        v-if="visibility"
        class="card divide-y divide-af-hairline !rounded-lg !border-0 ring-1 ring-af-hairline"
        data-testid="monitor-user-visibility"
      >
        <div class="px-5 py-4">
          <strong class="text-sm font-semibold text-af-ink">{{ t('channelMonitorV2.settings.visibility.title') }}</strong>
          <p class="mt-0.5 text-xs text-af-ink-3">{{ t('channelMonitorV2.settings.visibility.description') }}</p>
        </div>
        <div class="flex items-start justify-between gap-4 px-5 py-4">
          <div class="min-w-0">
            <p class="text-sm font-medium text-af-ink">{{ t('channelMonitorV2.settings.visibility.hideThroughput') }}</p>
            <p class="mt-1 text-xs text-af-ink-3">{{ t('channelMonitorV2.settings.visibility.hideThroughputHint') }}</p>
          </div>
          <Toggle v-model="visibility.hide_throughput" />
        </div>
        <div class="flex items-start justify-between gap-4 px-5 py-4">
          <div class="min-w-0">
            <p class="text-sm font-medium text-af-ink">{{ t('channelMonitorV2.settings.visibility.hideUserRanking') }}</p>
            <p class="mt-1 text-xs text-af-ink-3">{{ t('channelMonitorV2.settings.visibility.hideUserRankingHint') }}</p>
          </div>
          <Toggle v-model="visibility.hide_user_ranking" />
        </div>
      </div>

      <div class="card divide-y divide-af-hairline !rounded-lg !border-0 ring-1 ring-af-hairline">
        <div class="flex flex-wrap items-center justify-between gap-4 px-5 py-4">
          <div>
            <strong class="text-sm font-semibold text-af-ink">{{ t('channelMonitorV2.settings.enableTitle') }}</strong>
            <p class="mt-0.5 text-xs text-af-ink-3">
              {{ t('channelMonitorV2.settings.enableHint') }}
            </p>
          </div>
          <Toggle v-model="draft.enabled" />
        </div>
        <div class="flex flex-wrap items-center justify-between gap-4 px-5 py-4">
          <div>
            <strong class="text-sm font-semibold text-af-ink">{{ t('channelMonitorV2.settings.refreshTitle') }}</strong>
            <p class="mt-0.5 text-xs text-af-ink-3">{{ t('channelMonitorV2.settings.refreshHint') }}</p>
          </div>
          <div class="tabs inline-flex w-auto" role="group" :aria-label="t('channelMonitorV2.settings.refreshAria')">
            <button
              type="button"
              class="tab"
              :class="draft.refresh_interval_seconds === 60 ? 'tab-active' : ''"
              @click="draft.refresh_interval_seconds = 60"
            >
              1 min
            </button>
            <button
              type="button"
              class="tab"
              :class="draft.refresh_interval_seconds === 300 ? 'tab-active' : ''"
              @click="draft.refresh_interval_seconds = 300"
            >
              5 min
            </button>
          </div>
        </div>
      </div>

      <div class="card overflow-hidden !rounded-lg !border-0 ring-1 ring-af-hairline">
        <div class="card-header !py-3">
          <h3 class="text-sm font-semibold text-af-ink">{{ t('channelMonitorV2.settings.platformsTitle') }}</h3>
          <p class="mt-0.5 text-xs text-af-ink-3">
            {{ t('channelMonitorV2.settings.platformsHint') }}
          </p>
        </div>
        <div class="divide-y divide-af-hairline">
          <div
            v-for="platform in draft.platforms"
            :key="platform.platform"
            class="grid grid-cols-1 items-center gap-3 px-5 py-3 sm:grid-cols-[auto_7rem_minmax(0,1fr)_auto]"
          >
            <Toggle v-model="platform.enabled" />
            <strong class="text-sm font-medium text-af-ink">{{ platformLabel(platform.platform) }}</strong>
            <input
              class="input"
              :value="platform.models.join(', ')"
              type="text"
              :placeholder="t('channelMonitorV2.settings.modelsPlaceholder')"
              @change="setModels(platform, $event)"
            />
            <span
              class="badge justify-self-start sm:justify-self-end"
              :class="platform.models.length ? 'badge-gray' : 'badge badge-primary'"
            >
              {{ platform.models.length ? t('channelMonitorV2.settings.badgeOther') : t('channelMonitorV2.settings.badgeAllModels') }}
            </span>
          </div>
        </div>
      </div>

      <div class="card overflow-hidden !rounded-lg !border-0 ring-1 ring-af-hairline">
        <div class="card-header !py-3">
          <h3 class="text-sm font-semibold text-af-ink">{{ t('channelMonitorV2.settings.errorsTitle') }}</h3>
          <p class="mt-0.5 text-xs text-af-ink-3">
            {{ t('channelMonitorV2.settings.errorsHint') }}
          </p>
        </div>
        <div class="max-h-[min(40vh,320px)] overflow-y-auto px-3 py-2 sm:px-4">
          <div class="grid grid-cols-1 gap-1 sm:grid-cols-2">
            <label
              v-for="category in errorCategories"
              :key="category"
              class="flex cursor-pointer items-center gap-3 rounded-xl px-3 py-2.5 text-sm transition hover:bg-af-sunken"
            >
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-af-hairline-strong text-af-brand focus:ring-af-brand/40"
                :checked="isCategoryIgnored(category)"
                @change="toggleIgnoredCategory(category)"
              />
              <span class="min-w-0 flex-1 truncate font-medium text-af-ink">
                {{ categoryLabel(category) }}
              </span>
              <small class="shrink-0 font-mono text-[10px] text-af-ink-3">{{ category }}</small>
            </label>
          </div>
        </div>
        <div class="border-t border-af-hairline px-5 py-3 text-xs text-af-ink-3">
          {{
            t('channelMonitorV2.settings.ignoredSummary', {
              ignored: draft.ignored_error_categories?.length || 0,
              counted: countedErrorCategoryCount,
            })
          }}
        </div>
      </div>

      <div class="card overflow-hidden !rounded-lg !border-0 ring-1 ring-af-hairline">
        <div class="card-header !py-3">
          <h3 class="text-sm font-semibold text-af-ink">{{ t('channelMonitorV2.settings.healthTitle') }}</h3>
          <p class="mt-0.5 text-xs text-af-ink-3">
            {{ t('channelMonitorV2.settings.healthHint') }}
          </p>
        </div>
        <div class="grid grid-cols-1 gap-4 px-5 py-4 sm:grid-cols-2 lg:grid-cols-4">
          <label class="block">
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.minimumSample') }}</span>
            <input v-model.number="draft.health_thresholds.minimum_sample" class="input" type="number" min="1" max="10000" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.warningError') }}</span>
            <input v-model.number="warningErrorPercent" class="input" type="number" min="0" max="100" step="0.1" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.criticalError') }}</span>
            <input v-model.number="criticalErrorPercent" class="input" type="number" min="0" max="100" step="0.1" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.targetTtft') }}</span>
            <input v-model.number="draft.health_thresholds.target_ttft_ms" class="input" type="number" min="1" step="100" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.warningTtft') }}</span>
            <input v-model.number="draft.health_thresholds.warning_ttft_ms" class="input" type="number" min="1" step="100" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.criticalTtft') }}</span>
            <input v-model.number="draft.health_thresholds.critical_ttft_ms" class="input" type="number" min="1" step="100" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.warningCache') }}</span>
            <input v-model.number="warningCachePercent" class="input" type="number" min="0" max="100" step="0.1" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.criticalCache') }}</span>
            <input v-model.number="criticalCachePercent" class="input" type="number" min="0" max="100" step="0.1" />
          </label>
        </div>
      </div>

      <div class="space-y-2">
        <div class="rounded-lg border border-af-hairline-strong bg-af-brand-tint/80 px-4 py-3 text-sm text-af-brand">
          <template v-if="namedModelCount === 0">
            {{ t('channelMonitorV2.settings.namedModelsEmpty') }}
          </template>
          <template v-else>
            {{ t('channelMonitorV2.settings.namedModelsCount', { count: namedModelCount }) }}
          </template>
        </div>
        <div class="rounded-lg border border-af-hairline bg-af-sunken/80 px-4 py-3 text-xs text-af-ink-2">
          <p class="font-medium text-af-ink">{{ t('channelMonitorV2.settings.userContractTitle') }}</p>
          <ul class="mt-1.5 list-disc space-y-0.5 pl-4">
            <li>{{ t('channelMonitorV2.settings.userContract.health') }}</li>
            <li>{{ t('channelMonitorV2.settings.userContract.trend') }}</li>
            <li>{{ t('channelMonitorV2.settings.userContract.latency') }}</li>
            <li>{{ t('channelMonitorV2.settings.userContract.models') }}</li>
          </ul>
        </div>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { extractApiErrorMessage } from '@/utils/apiError'
import { isChannelMonitorRouteEnabled } from '@/utils/featureFlags'
import {
  getConfig,
  updateConfig,
  MONITOR_ERROR_CATEGORIES,
  type MonitorConfig,
} from '@/api/channelMonitorV2'

const { t, te } = useI18n()
const appStore = useAppStore()
const loading = ref(true)
const saving = ref(false)
const draft = ref<MonitorConfig | null>(null)
const original = ref('')

/** 用户端展示的两个开关存在全局设置里（channel_monitor_hide_*），与汇总配置分开存 */
interface MonitorVisibility {
  hide_throughput: boolean
  hide_user_ranking: boolean
}
const visibility = ref<MonitorVisibility | null>(null)
const visibilityOriginal = ref('')
const visibilityDirty = computed(() => (visibility.value ? JSON.stringify(visibility.value) !== visibilityOriginal.value : false))
const configDirty = computed(() => (draft.value ? JSON.stringify(draft.value) !== original.value : false))
const dirty = computed(() => configDirty.value || visibilityDirty.value)

function applyVisibility(settings: { channel_monitor_hide_throughput?: boolean; channel_monitor_hide_user_ranking?: boolean }) {
  const value: MonitorVisibility = {
    hide_throughput: Boolean(settings.channel_monitor_hide_throughput),
    hide_user_ranking: Boolean(settings.channel_monitor_hide_user_ranking),
  }
  visibility.value = { ...value }
  visibilityOriginal.value = JSON.stringify(value)
}
const namedModelCount = computed(
  () => draft.value?.platforms.filter((p) => p.enabled).reduce((sum, p) => sum + p.models.length, 0) || 0
)
const errorCategories = MONITOR_ERROR_CATEGORIES
const countedErrorCategoryCount = computed(
  () => errorCategories.length - (draft.value?.ignored_error_categories?.length || 0)
)
/** 功能开关（设置 › 功能开关）关着时汇总不跑；配置仍可先保存。 */
const featureEnabled = computed(() => isChannelMonitorRouteEnabled())
const defaultThresholds = {
  minimum_sample: 50,
  warning_error_rate: 0.05,
  critical_error_rate: 0.20,
  target_ttft_ms: 3000,
  warning_ttft_ms: 3000,
  critical_ttft_ms: 10000,
  // Higher is better: below 85% watch, below 60% critical.
  warning_cache_rate: 0.85,
  critical_cache_rate: 0.60,
  error_weight: 0.60,
  ttft_weight: 0.20,
  cache_weight: 0.20,
}

/** Factory ignored categories (matches backend DefaultChannelMonitorV2IgnoredErrorCategories). */
const defaultIgnoredErrorCategories = [
  'authentication',
  'client_cancelled',
  'content_policy',
  'context_limit',
  'model_unsupported',
  'not_found',
  'quota_or_balance',
] as const
function percentModel(key: 'warning_error_rate' | 'critical_error_rate' | 'warning_cache_rate' | 'critical_cache_rate') {
  return computed({
    get: () => ((draft.value?.health_thresholds?.[key] ?? defaultThresholds[key]) * 100),
    set: (value: number) => {
      if (!draft.value) return
      draft.value.health_thresholds[key] = Math.max(0, Math.min(100, Number(value) || 0)) / 100
    },
  })
}
const warningErrorPercent = percentModel('warning_error_rate')
const criticalErrorPercent = percentModel('critical_error_rate')
const warningCachePercent = percentModel('warning_cache_rate')
const criticalCachePercent = percentModel('critical_cache_rate')

function setModels(platform: MonitorConfig['platforms'][number], event: Event) {
  platform.models = [
    ...new Set(
      (event.target as HTMLInputElement).value
        .split(',')
        .map((v) => v.trim())
        .filter(Boolean)
    ),
  ].sort()
}

function isCategoryIgnored(category: string): boolean {
  return Boolean(draft.value?.ignored_error_categories?.includes(category))
}

function toggleIgnoredCategory(category: string) {
  if (!draft.value) return
  const current = new Set(draft.value.ignored_error_categories || [])
  if (current.has(category)) current.delete(category)
  else current.add(category)
  draft.value.ignored_error_categories = [...current].sort()
}

function categoryLabel(category: string) {
  const key = `channelMonitorV2.errorCategories.${category}`
  return te(key) ? t(key) : category
}

function platformLabel(value: string) {
  return (
    {
      anthropic: 'Claude',
      openai: 'OpenAI',
      grok: 'Grok',
      kiro: 'Kiro',
      gemini: 'Gemini',
      antigravity: 'Antigravity',
      kimi: 'Kimi',
      zhipu: 'Zhipu GLM',
      deepseek: 'DeepSeek',
      minimax: 'MiniMax',
      composite: 'Composite',
    } as Record<string, string>
  )[value] || value
}

function normalizeConfig(value: MonitorConfig): MonitorConfig {
  const ignored = value.ignored_error_categories
  return {
    ...value,
    health_thresholds: { ...defaultThresholds, ...(value.health_thresholds || {}) },
    // Preserve explicit empty arrays from the server (operator cleared all).
    ignored_error_categories: [
      ...(ignored == null ? [...defaultIgnoredErrorCategories] : ignored),
    ].sort(),
  }
}

async function load() {
  loading.value = true
  try {
    const [value, settings] = await Promise.all([getConfig(), adminAPI.settings.getSettings()])
    const normalized = normalizeConfig(value)
    draft.value = structuredClone(normalized)
    original.value = JSON.stringify(normalized)
    applyVisibility(settings)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('channelMonitorV2.settings.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!draft.value) return
  saving.value = true
  try {
    if (configDirty.value) {
      const payload = normalizeConfig(draft.value)
      const value = await updateConfig(payload)
      const normalized = normalizeConfig(value)
      draft.value = structuredClone(normalized)
      original.value = JSON.stringify(normalized)
    }
    if (visibility.value && visibilityDirty.value) {
      // 设置接口只写请求里带了的字段，这里只发这两项
      const updated = await adminAPI.settings.updateSettings({
        channel_monitor_hide_throughput: visibility.value.hide_throughput,
        channel_monitor_hide_user_ranking: visibility.value.hide_user_ranking,
      })
      applyVisibility(updated)
    }
    appStore.showSuccess(t('channelMonitorV2.settings.saveSuccess'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('channelMonitorV2.settings.saveFailed')))
    await load()
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>
