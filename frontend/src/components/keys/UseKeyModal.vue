<template>
  <!-- layout="inline"：放进密钥详情抽屉的「使用方法」页签，只渲染正文（InlineShell 与 BaseDialog 同接口） -->
  <component
    :is="layout === 'inline' ? InlineShell : BaseDialog"
    :show="show"
    :title="t('keys.useKeyModal.title')"
    width="wide"
    @close="emit('close')"
  >
    <div class="space-y-4">
      <!-- 内容按客户端标签页决定（任何 key 都能调四种入站协议） -->
      <!-- Description -->
      <p class="text-sm text-af-ink-2">
        {{ platformDescription }}
      </p>

      <!-- 配置模板的模型来自上架目录（D4）：目录还没到 / 取失败 / 没有对话模型时说清楚 -->
      <p v-if="catalogState === 'loading'" class="text-sm text-af-ink-3" data-testid="use-key-catalog-loading">{{ t('common.loading') }}</p>
      <p v-else-if="catalogState === 'error'" class="text-sm text-af-danger" data-testid="use-key-catalog-error">
        {{ t('keys.useKeyModal.catalog.loadFailed') }}
        <button type="button" class="ml-2 underline" @click="loadCatalog">{{ t('keys.useKeyModal.catalog.retry') }}</button>
      </p>
      <p v-else-if="!clientTabs.length" class="text-sm text-af-ink-3" data-testid="use-key-catalog-empty">{{ t('keys.useKeyModal.catalog.empty') }}</p>

      <!-- Client Tabs -->
      <div v-if="clientTabs.length" class="overflow-x-auto border-b border-af-hairline">
        <nav class="-mb-px flex min-w-max gap-4 sm:gap-6" aria-label="Client">
          <button
            v-for="tab in clientTabs"
            :key="tab.id"
            type="button"
            @click="activeClientTab = tab.id"
            :class="[
              'whitespace-nowrap py-2.5 px-1 border-b-2 font-medium text-sm transition-colors',
              activeClientTab === tab.id
                ? 'border-af-brand text-af-brand'
                : 'border-transparent text-af-ink-3 hover:text-af-ink-2 hover:border-af-hairline-strong'
            ]"
          >
            <span class="flex items-center gap-2">
              <component :is="tab.icon" class="w-4 h-4" />
              {{ tab.label }}
            </span>
          </button>
        </nav>
      </div>

      <!-- Codex Authentication Mode -->
      <div
        v-if="showCodexAuthMode"
        class="rounded-lg border border-af-hairline p-3"
      >
        <div class="mb-2">
          <p class="text-sm font-medium text-af-ink">
            {{ t('keys.useKeyModal.openai.authModeTitle') }}
          </p>
          <p class="mt-0.5 text-xs text-af-ink-3">
            {{ t('keys.useKeyModal.openai.authModeDescription') }}
          </p>
        </div>
        <div
          class="grid grid-cols-2 gap-1 rounded-lg bg-af-sunken p-1"
          role="radiogroup"
          :aria-label="t('keys.useKeyModal.openai.authModeTitle')"
        >
          <button
            type="button"
            role="radio"
            data-testid="codex-auth-mode-legacy"
            :aria-checked="codexAuthMode === 'legacy'"
            :class="[
              'rounded-md px-3 py-2 text-sm font-medium transition-colors',
              codexAuthMode === 'legacy'
                ? 'bg-af-sheet text-af-brand shadow-sm'
                : 'text-af-ink-2 hover:text-af-ink'
            ]"
            @click="codexAuthMode = 'legacy'"
          >
            {{ t('keys.useKeyModal.openai.authModeLegacy') }}
          </button>
          <button
            type="button"
            role="radio"
            data-testid="codex-auth-mode-api-key"
            :aria-checked="codexAuthMode === 'api-key'"
            :class="[
              'rounded-md px-3 py-2 text-sm font-medium transition-colors',
              codexAuthMode === 'api-key'
                ? 'bg-af-sheet text-af-brand shadow-sm'
                : 'text-af-ink-2 hover:text-af-ink'
            ]"
            @click="codexAuthMode = 'api-key'"
          >
            {{ t('keys.useKeyModal.openai.authModeApiKey') }}
          </button>
        </div>
        <div
          v-if="codexAuthMode === 'api-key'"
          data-testid="codex-api-key-restart-notice"
          class="mt-3 flex items-start gap-2 border-l-2 border-af-warning bg-af-warning-tint px-3 py-2 text-xs leading-5 text-af-warning"
        >
          <Icon name="exclamationCircle" size="sm" class="mt-0.5 flex-shrink-0" />
          <p>{{ t('keys.useKeyModal.openai.authModeApiKeyRestartNotice') }}</p>
        </div>
      </div>

      <!-- OS/Shell Tabs -->
      <div v-if="showShellTabs" class="overflow-x-auto border-b border-af-hairline">
        <nav class="-mb-px flex min-w-max gap-4" aria-label="Tabs">
          <button
            v-for="tab in currentTabs"
            :key="tab.id"
            type="button"
            @click="activeTab = tab.id"
            :class="[
              'whitespace-nowrap py-2.5 px-1 border-b-2 font-medium text-sm transition-colors',
              activeTab === tab.id
                ? 'border-af-brand text-af-brand'
                : 'border-transparent text-af-ink-3 hover:text-af-ink-2 hover:border-af-hairline-strong'
            ]"
          >
            <span class="flex items-center gap-2">
              <component :is="tab.icon" class="w-4 h-4" />
              {{ tab.label }}
            </span>
          </button>
        </nav>
      </div>

      <!-- Code Blocks (Stacked for multi-file platforms) -->
      <div class="space-y-4">
        <div
          v-for="(file, index) in currentFiles"
          :key="index"
          class="relative"
        >
          <!-- File Hint (if exists) -->
          <p v-if="file.hint" class="text-xs text-af-warning mb-1.5 flex items-center gap-1">
            <Icon name="exclamationCircle" size="sm" class="flex-shrink-0" />
            {{ file.hint }}
          </p>
          <div class="overflow-hidden rounded-md border border-af-hairline bg-af-sunken">
            <!-- Code Header -->
            <div class="flex items-center justify-between px-4 py-2 bg-af-sunken border-b border-af-hairline">
              <span class="min-w-0 truncate text-xs text-af-ink-3 font-mono">{{ file.path }}</span>
              <button
                type="button"
                @click="copyContent(file.content, index)"
                class="flex flex-shrink-0 items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-lg transition-colors"
                :class="copiedIndex === index
                  ? 'bg-af-success-tint text-af-success'
                  : 'bg-af-sheet hover:bg-af-hairline text-af-ink-3 hover:text-af-ink'"
              >
                <svg v-if="copiedIndex === index" class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                </svg>
                <svg v-else class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M15.666 3.888A2.25 2.25 0 0013.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 01-.75.75H9a.75.75 0 01-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 01-2.25 2.25H6.75A2.25 2.25 0 014.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 011.927-.184" />
                </svg>
                {{ copiedIndex === index ? t('keys.useKeyModal.copied') : t('keys.useKeyModal.copy') }}
              </button>
            </div>
            <!-- Code Content -->
            <pre class="p-4 text-sm font-mono text-af-ink overflow-x-auto"><code v-if="file.highlighted" v-html="file.highlighted"></code><code v-else v-text="file.content"></code></pre>
          </div>
        </div>
      </div>

      <section
        v-if="showCodexModelCatalog"
        data-testid="codex-model-catalog"
        class="overflow-hidden rounded-lg border border-af-hairline bg-af-sunken"
      >
        <div class="flex flex-col gap-3 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
          <div class="min-w-0">
            <h3 class="text-sm font-medium text-af-ink">
              {{ t('keys.useKeyModal.codexModelCatalog.title') }}
            </h3>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('keys.useKeyModal.codexModelCatalog.description') }}
            </p>
            <p class="mt-1 truncate font-mono text-xs text-af-ink-2">
              {{ codexModelCatalogPath }}
            </p>
          </div>
          <button
            v-if="codexModelManifestState === 'ready'"
            type="button"
            class="btn btn-primary min-h-9 flex-shrink-0 px-3 text-xs"
            @click="downloadCodexModelManifest"
          >
            <Icon name="download" size="sm" class="mr-1.5" />
            {{ t('keys.useKeyModal.codexModelCatalog.download') }}
          </button>
          <button
            v-else
            type="button"
            data-testid="codex-model-catalog-fetch"
            class="btn btn-primary min-h-9 flex-shrink-0 px-3 text-xs"
            :disabled="codexModelManifestState === 'loading' || !apiKey"
            @click="loadCodexModelManifest"
          >
            <Icon
              name="refresh"
              size="sm"
              class="mr-1.5"
              :class="codexModelManifestState === 'loading' ? 'animate-spin' : ''"
            />
            {{ codexModelManifestState === 'error'
              ? t('keys.useKeyModal.codexModelCatalog.retry')
              : t('keys.useKeyModal.codexModelCatalog.fetch') }}
          </button>
        </div>
        <p
          v-if="codexModelManifestState === 'ready'"
          class="border-t border-af-hairline px-4 py-2 text-xs text-af-success"
        >
          {{ t('keys.useKeyModal.codexModelCatalog.modelsCount', { count: codexModelManifestModelCount }) }}
        </p>
        <p
          v-else-if="codexModelManifestState === 'error'"
          class="border-t border-af-danger/40 px-4 py-2 text-xs text-af-danger"
        >
          {{ t('keys.useKeyModal.codexModelCatalog.errorDescription') }}
        </p>
      </section>

      <!-- Usage Note -->
      <div v-if="showPlatformNote" class="flex items-start gap-3 p-3 rounded-lg bg-af-brand-tint border border-af-brand/20">
        <Icon name="infoCircle" size="md" class="text-af-brand flex-shrink-0 mt-0.5" />
        <p class="text-sm text-af-brand">
          {{ platformNote }}
        </p>
      </div>

      <!-- 查询可用模型（muqian 2026-09-29：用户拿自己的密钥查 /models）：各客户端通用 -->
      <section class="space-y-2" data-testid="use-key-models-api">
        <h4 class="text-sm font-medium text-af-ink">{{ t('keys.useKeyModal.modelsApi.title') }}</h4>
        <p class="text-xs text-af-ink-3">{{ t('keys.useKeyModal.modelsApi.hint') }}</p>
        <div class="overflow-hidden rounded-md border border-af-hairline bg-af-sunken">
          <div class="flex items-center justify-between border-b border-af-hairline px-4 py-2">
            <span class="min-w-0 truncate font-mono text-xs text-af-ink-3">GET /v1/models</span>
            <button
              type="button"
              class="flex flex-shrink-0 items-center gap-1.5 rounded-lg px-2.5 py-1 text-xs font-medium transition-colors"
              :class="copiedIndex === MODELS_API_COPY_INDEX
                ? 'bg-af-success-tint text-af-success'
                : 'bg-af-sheet text-af-ink-3 hover:bg-af-hairline hover:text-af-ink'"
              @click="copyContent(modelsApiSnippet, MODELS_API_COPY_INDEX)"
            >
              <Icon :name="copiedIndex === MODELS_API_COPY_INDEX ? 'check' : 'copy'" size="xs" />
              {{ copiedIndex === MODELS_API_COPY_INDEX ? t('keys.useKeyModal.copied') : t('keys.useKeyModal.copy') }}
            </button>
          </div>
          <pre class="overflow-x-auto p-4 font-mono text-sm text-af-ink"><code v-text="modelsApiSnippet"></code></pre>
        </div>
      </section>
    </div>

    <template #footer>
      <div class="flex justify-end">
        <button
          @click="emit('close')"
          class="btn btn-secondary"
        >
          {{ t('common.close') }}
        </button>
      </div>
    </template>
  </component>
</template>

<script setup lang="ts">
import { ref, computed, h, watch, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { saveAs } from 'file-saver'
import BaseDialog from '@/components/common/BaseDialog.vue'
import InlineShell from '@/components/common/InlineShell.vue'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { fetchCodexModelsManifest } from '@/api/codex'
import {
  findCodexCatalogModel,
  formatCodexReasoningEffortTomlLine,
  parseCodexCatalogModels,
  selectCodexConfigReasoningEffort
} from '@/utils/codexCatalogConfig'
import { DEFAULT_SITE_NAME } from '@/utils/branding'
import { USE_KEY_CLIENTS } from '@/components/user/clients'
import { getModelPlaza } from '@/api/modelPlaza'
import { clientHasModels, groupChatModels, newestMediaModel, type ClientVendor, type KeyCatalogModel, type VendorModels } from './keyCatalog'

interface Props {
  show: boolean
  apiKey: string
  baseUrl: string
  /** 生成配置里的显示名（provider name / 注释）用站名，不写死品牌 */
  siteName?: string
  layout?: 'dialog' | 'inline'
}

interface Emits {
  (e: 'close'): void
}

interface TabConfig {
  id: string
  label: string
  icon: Component
}

interface FileConfig {
  path: string
  content: string
  hint?: string  // Optional hint message for this file
  highlighted?: string
}

const props = withDefaults(defineProps<Props>(), { siteName: DEFAULT_SITE_NAME, layout: 'dialog' })
const emit = defineEmits<Emits>()
const siteName = computed(() => props.siteName?.trim() || DEFAULT_SITE_NAME)

const { t } = useI18n()
const { copyToClipboard: clipboardCopy } = useClipboard()

const copiedIndex = ref<number | null>(null)
/** 「查询可用模型」代码块的复制状态槽，与配置文件的下标错开 */
const MODELS_API_COPY_INDEX = -1
// 站点根地址（去掉末尾的 /v1），查模型用 {根}/v1/models
const modelsApiSnippet = computed(() => {
  const root = (props.baseUrl || window.location.origin).replace(/\/v1\/?$/, '').replace(/\/+$/, '')
  return `curl ${root}/v1/models \\\n  -H "Authorization: Bearer ${props.apiKey}"`
})
const activeTab = ref<string>('unix')
const activeClientTab = ref<string>('')

// 上架目录里的对话模型（按厂商分组，新的在前）：页签与模板里的模型都从这里来（D4）
type CatalogState = 'loading' | 'ready' | 'error'
const catalogState = ref<CatalogState>('loading')
const vendorModels = ref<VendorModels>({ anthropic: [], openai: [], google: [], xai: [] })
// Grok CLI 的生图 / 视频：目录里有 xAI 的生图 / 视频模型才开
const xaiMedia = ref<{ image: string; video: string }>({ image: '', video: '' })
let catalogRequestID = 0

async function loadCatalog() {
  const requestID = ++catalogRequestID
  catalogState.value = 'loading'
  try {
    const plaza = await getModelPlaza()
    if (requestID !== catalogRequestID) return
    vendorModels.value = groupChatModels(plaza.models ?? [])
    xaiMedia.value = {
      image: newestMediaModel(plaza.models ?? [], 'xai', 'image'),
      video: newestMediaModel(plaza.models ?? [], 'xai', 'video')
    }
    catalogState.value = 'ready'
    if (!clientTabs.value.some((tab) => tab.id === activeClientTab.value)) {
      activeClientTab.value = clientTabs.value[0]?.id ?? ''
    }
  } catch (error) {
    if (requestID !== catalogRequestID) return
    console.error('Failed to load model catalog for key config templates:', error)
    catalogState.value = 'error'
  }
}

function modelsOf(vendor: ClientVendor): KeyCatalogModel[] {
  return vendorModels.value[vendor]
}
/** 这一家的默认模型：新的那个（目录里没有时为空串，页签本来就不出现） */
function defaultModelOf(vendor: ClientVendor): string {
  return vendorModels.value[vendor][0]?.id ?? ''
}
type CodexAuthMode = 'legacy' | 'api-key'
const codexAuthMode = ref<CodexAuthMode>('legacy')
type CodexModelManifestState = 'idle' | 'loading' | 'ready' | 'error'
const codexModelManifestState = ref<CodexModelManifestState>('idle')
const codexModelManifestContent = ref('')
const codexModelManifestModelCount = ref(0)
let codexModelManifestController: AbortController | null = null
let codexModelManifestRequestID = 0

const showCodexModelCatalog = computed(() =>
  props.show && (activeClientTab.value === 'codex' || activeClientTab.value === 'codex-ws')
)

const codexModelCatalogPath = computed(() => {
  const isWindows = activeTab.value === 'windows'
  const configDir = isWindows ? '%userprofile%\\.codex' : '~/.codex'
  return joinConfigPath(configDir, 'codex-models.json', isWindows)
})

const codexManifestContext = computed(() => {
  if (!showCodexModelCatalog.value) return ''
  return `${props.baseUrl}|${props.apiKey}`
})

watch(() => props.show, (show) => {
  if (show) {
    // 目录到了再落到第一个有模型的页签（loadCatalog）
    activeClientTab.value = ''
    activeTab.value = 'unix'
    codexAuthMode.value = 'legacy'
    void loadCatalog()
  } else {
    resetCodexModelManifest()
  }
}, { immediate: true })

watch(codexManifestContext, (context, previousContext) => {
  if (context !== previousContext) {
    resetCodexModelManifest()
  }
})

// Reset shell tab when client changes
watch(activeClientTab, () => {
  activeTab.value = 'unix'
})

// Icon components
const AppleIcon = {
  render() {
    return h('svg', {
      fill: 'currentColor',
      viewBox: '0 0 24 24',
      class: 'w-4 h-4'
    }, [
      h('path', { d: 'M18.71 19.5c-.83 1.24-1.71 2.45-3.05 2.47-1.34.03-1.77-.79-3.29-.79-1.53 0-2 .77-3.27.82-1.31.05-2.3-1.32-3.14-2.53C4.25 17 2.94 12.45 4.7 9.39c.87-1.52 2.43-2.48 4.12-2.51 1.28-.02 2.5.87 3.29.87.78 0 2.26-1.07 3.81-.91.65.03 2.47.26 3.64 1.98-.09.06-2.17 1.28-2.15 3.81.03 3.02 2.65 4.03 2.68 4.04-.03.07-.42 1.44-1.38 2.83M13 3.5c.73-.83 1.94-1.46 2.94-1.5.13 1.17-.34 2.35-1.04 3.19-.69.85-1.83 1.51-2.95 1.42-.15-1.15.41-2.35 1.05-3.11z' })
    ])
  }
}

const WindowsIcon = {
  render() {
    return h('svg', {
      fill: 'currentColor',
      viewBox: '0 0 24 24',
      class: 'w-4 h-4'
    }, [
      h('path', { d: 'M3 12V6.75l6-1.32v6.48L3 12zm17-9v8.75l-10 .15V5.21L20 3zM3 13l6 .09v6.81l-6-1.15V13zm7 .25l10 .15V21l-10-1.91v-5.84z' })
    ])
  }
}

// Terminal icon for Claude Code
const TerminalIcon = {
  render() {
    return h('svg', {
      fill: 'none',
      stroke: 'currentColor',
      viewBox: '0 0 24 24',
      'stroke-width': '1.5',
      class: 'w-4 h-4'
    }, [
      h('path', {
        'stroke-linecap': 'round',
        'stroke-linejoin': 'round',
        d: 'm6.75 7.5 3 2.25-3 2.25m4.5 0h3m-9 8.25h13.5A2.25 2.25 0 0 0 21 17.25V6.75A2.25 2.25 0 0 0 18.75 4.5H5.25A2.25 2.25 0 0 0 3 6.75v10.5A2.25 2.25 0 0 0 5.25 20.25Z'
      })
    ])
  }
}

// Sparkle icon for Gemini
const SparkleIcon = {
  render() {
    return h('svg', {
      fill: 'none',
      stroke: 'currentColor',
      viewBox: '0 0 24 24',
      'stroke-width': '1.5',
      class: 'w-4 h-4'
    }, [
      h('path', {
        'stroke-linecap': 'round',
        'stroke-linejoin': 'round',
        d: 'M9.813 15.904 9 18.75l-.813-2.846a4.5 4.5 0 0 0-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 0 0 3.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 0 0 3.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 0 0-3.09 3.09ZM18.259 8.715 18 9.75l-.259-1.035a3.375 3.375 0 0 0-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 0 0 2.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 0 0 2.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 0 0-2.456 2.456ZM16.894 20.567 16.5 21.75l-.394-1.183a2.25 2.25 0 0 0-1.423-1.423L13.5 18.75l1.183-.394a2.25 2.25 0 0 0 1.423-1.423l.394-1.183.394 1.183a2.25 2.25 0 0 0 1.423 1.423l1.183.394-1.183.394a2.25 2.25 0 0 0-1.423 1.423Z'
      })
    ])
  }
}

// 客户端标签页（清单与首页共用 USE_KEY_CLIENTS）：目录里有这个客户端对应厂商的模型才出现（D4）
const clientTabs = computed((): TabConfig[] =>
  catalogState.value !== 'ready'
    ? []
    : USE_KEY_CLIENTS.filter((client) => clientHasModels(client.id, vendorModels.value)).map((client) => ({
        id: client.id,
        label: t(client.labelKey),
        icon: client.id === 'gemini' ? SparkleIcon : TerminalIcon
      }))
)
const hasActiveClient = computed(() => clientTabs.value.some((tab) => tab.id === activeClientTab.value))

// Shell tabs (3 types for environment variable based configs)
const shellTabs: TabConfig[] = [
  { id: 'unix', label: 'macOS / Linux', icon: AppleIcon },
  { id: 'cmd', label: 'Windows CMD', icon: WindowsIcon },
  { id: 'powershell', label: 'PowerShell', icon: WindowsIcon }
]

// OpenAI tabs (2 OS types)
const openaiTabs: TabConfig[] = [
  { id: 'unix', label: 'macOS / Linux', icon: AppleIcon },
  { id: 'windows', label: 'Windows', icon: WindowsIcon }
]

const showShellTabs = computed(() => hasActiveClient.value && activeClientTab.value !== 'opencode')

const showCodexAuthMode = computed(() =>
  activeClientTab.value === 'codex' || activeClientTab.value === 'codex-ws'
)

const currentTabs = computed(() => {
  if (!showShellTabs.value) return []
  if (activeClientTab.value === 'codex' || activeClientTab.value === 'codex-ws' || activeClientTab.value === 'grok') {
    return openaiTabs
  }
  return shellTabs
})

const platformDescription = computed(() => {
  switch (activeClientTab.value) {
    case 'codex':
    case 'codex-ws':
      return t('keys.useKeyModal.openai.description')
    case 'gemini':
      return t('keys.useKeyModal.gemini.description')
    case 'grok':
      return t('keys.useKeyModal.grok.description')
    default:
      return t('keys.useKeyModal.description')
  }
})

const platformNote = computed(() => {
  switch (activeClientTab.value) {
    case 'codex':
    case 'codex-ws':
      return activeTab.value === 'windows'
        ? t('keys.useKeyModal.openai.noteWindows')
        : t('keys.useKeyModal.openai.note')
    case 'gemini':
      return t('keys.useKeyModal.gemini.note')
    case 'grok':
      // Grok CLI: shell-specific path guidance (env + ~/.grok/config.toml).
      if (activeTab.value === 'cmd' || activeTab.value === 'powershell' || activeTab.value === 'windows') {
        return t('keys.useKeyModal.grok.noteWindows')
      }
      return t('keys.useKeyModal.grok.note')
    default:
      return t('keys.useKeyModal.note')
  }
})

const showPlatformNote = computed(() => activeClientTab.value !== 'opencode')

function resetCodexModelManifest() {
  codexModelManifestController?.abort()
  codexModelManifestController = null
  codexModelManifestRequestID += 1
  codexModelManifestState.value = 'idle'
  codexModelManifestContent.value = ''
  codexModelManifestModelCount.value = 0
}

async function loadCodexModelManifest() {
  if (!showCodexModelCatalog.value || !props.apiKey) return

  codexModelManifestController?.abort()
  const controller = new AbortController()
  const requestID = ++codexModelManifestRequestID
  codexModelManifestController = controller
  codexModelManifestState.value = 'loading'

  try {
    const result = await fetchCodexModelsManifest(props.baseUrl, props.apiKey, controller.signal)
    if (requestID !== codexModelManifestRequestID) return
    codexModelManifestContent.value = result.content
    codexModelManifestModelCount.value = result.modelCount
    codexModelManifestState.value = 'ready'
  } catch (error) {
    const errorName = error && typeof error === 'object' && 'name' in error
      ? String((error as { name?: unknown }).name || '')
      : ''
    if (requestID !== codexModelManifestRequestID || errorName === 'AbortError') return
    codexModelManifestState.value = 'error'
  } finally {
    if (requestID === codexModelManifestRequestID) {
      codexModelManifestController = null
    }
  }
}

function downloadCodexModelManifest() {
  if (!codexModelManifestContent.value) return
  saveAs(
    new Blob([codexModelManifestContent.value], { type: 'application/json;charset=utf-8' }),
    'codex-models.json'
  )
}

const codexCatalogModelSlugs = computed(() =>
  parseCodexCatalogModels(codexModelManifestContent.value).map((model) => model.slug)
)

function selectCodexCatalogModel(preferredModel: string): string {
  if (codexCatalogModelSlugs.value.includes(preferredModel)) return preferredModel
  return codexCatalogModelSlugs.value[0] || preferredModel
}

function codexReasoningEffortTomlLine(modelSlug: string): string {
  return formatCodexReasoningEffortTomlLine(
    selectCodexConfigReasoningEffort(findCodexCatalogModel(codexModelManifestContent.value, modelSlug))
  )
}

const escapeHtml = (value: string) => value
  .replace(/&/g, '&amp;')
  .replace(/</g, '&lt;')
  .replace(/>/g, '&gt;')
  .replace(/"/g, '&quot;')
  .replace(/'/g, '&#39;')

const wrapToken = (className: string, value: string) =>
  `<span class="${className}">${escapeHtml(value)}</span>`

const keyword = (value: string) => wrapToken('text-af-brand', value)
const variable = (value: string) => wrapToken('text-af-ink', value)
const operator = (value: string) => wrapToken('text-af-ink-3', value)
const string = (value: string) => wrapToken('text-af-warning', value)
const comment = (value: string) => wrapToken('text-af-ink-3', value)

// Syntax highlighting helpers
// Generate file configs based on platform and active tab
const currentFiles = computed((): FileConfig[] => {
  const baseUrl = props.baseUrl || window.location.origin
  const apiKey = props.apiKey
  const baseRoot = baseUrl.replace(/\/v1\/?$/, '').replace(/\/+$/, '')
  const ensureV1 = (value: string) => {
    const trimmed = value.replace(/\/+$/, '')
    return trimmed.endsWith('/v1') ? trimmed : `${trimmed}/v1`
  }
  const apiBase = ensureV1(baseRoot)
  const geminiBase = (() => {
    const trimmed = baseRoot.replace(/\/+$/, '')
    return trimmed.endsWith('/v1beta') ? trimmed : `${trimmed}/v1beta`
  })()

  if (!hasActiveClient.value) return []
  switch (activeClientTab.value) {
    case 'opencode': {
      // 一个 key 四种协议都能走：每家一个 provider，目录里没有这家模型的不出
      const providers: Array<[string, ClientVendor, string, string]> = [
        ['anthropic', 'anthropic', apiBase, 'opencode.json (Claude)'],
        ['openai', 'openai', apiBase, 'opencode.json (OpenAI)'],
        ['gemini', 'google', geminiBase, 'opencode.json (Gemini)'],
        ['grok', 'xai', apiBase, 'opencode.json (Grok)']
      ]
      return providers
        .filter(([, vendor]) => modelsOf(vendor).length > 0)
        .map(([platform, vendor, base, label]) => generateOpenCodeConfig(platform, base, apiKey, modelsOf(vendor), label))
    }
    case 'codex':
      return generateOpenAIFiles(baseUrl, apiKey)
    case 'codex-ws':
      return generateOpenAIWsFiles(baseUrl, apiKey)
    case 'gemini':
      return [generateGeminiCliContent(baseUrl, apiKey)]
    case 'grok':
      return generateGrokFiles(apiBase, apiKey)
    default:
      return generateAnthropicFiles(baseUrl, apiKey)
  }
})

function generateAnthropicFiles(baseUrl: string, apiKey: string): FileConfig[] {
  let path: string
  let content: string

  switch (activeTab.value) {
    case 'unix':
      path = 'Terminal'
      content = `export ANTHROPIC_BASE_URL="${baseUrl}"
export ANTHROPIC_AUTH_TOKEN="${apiKey}"
export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`
      break
    case 'cmd':
      path = 'Command Prompt'
      content = `set ANTHROPIC_BASE_URL=${baseUrl}
set ANTHROPIC_AUTH_TOKEN=${apiKey}
set CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`
      break
    case 'powershell':
      path = 'PowerShell'
      content = `$env:ANTHROPIC_BASE_URL="${baseUrl}"
$env:ANTHROPIC_AUTH_TOKEN="${apiKey}"
$env:CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`
      break
    default:
      path = 'Terminal'
      content = ''
  }

  const vscodeSettingsPath = activeTab.value === 'unix'
    ? '~/.claude/settings.json'
    : '%USERPROFILE%\\.claude\\settings.json'

  const vscodeContent = `{
  "$schema": "https://json.schemastore.org/claude-code-settings.json",
  "env": {
    "ANTHROPIC_BASE_URL": "${baseUrl}",
    "ANTHROPIC_AUTH_TOKEN": "${apiKey}",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1"
  }
}`

  return [
    { path, content },
    {
      path: vscodeSettingsPath,
      content: vscodeContent,
      hint: t('keys.useKeyModal.claudeSettingsHint')
    }
  ]
}

function generateGeminiCliContent(baseUrl: string, apiKey: string): FileConfig {
  const model = defaultModelOf('google')
  const modelComment = t('keys.useKeyModal.gemini.modelComment')
  let path: string
  let content: string
  let highlighted: string

  switch (activeTab.value) {
    case 'unix':
      path = 'Terminal'
      content = `export GOOGLE_GEMINI_BASE_URL="${baseUrl}"
export GEMINI_API_KEY="${apiKey}"
export GEMINI_MODEL="${model}"  # ${modelComment}`
      highlighted = `${keyword('export')} ${variable('GOOGLE_GEMINI_BASE_URL')}${operator('=')}${string(`"${baseUrl}"`)}
${keyword('export')} ${variable('GEMINI_API_KEY')}${operator('=')}${string(`"${apiKey}"`)}
${keyword('export')} ${variable('GEMINI_MODEL')}${operator('=')}${string(`"${model}"`)}  ${comment(`# ${modelComment}`)}`
      break
    case 'cmd':
      path = 'Command Prompt'
      content = `set GOOGLE_GEMINI_BASE_URL=${baseUrl}
set GEMINI_API_KEY=${apiKey}
set GEMINI_MODEL=${model}`
      highlighted = `${keyword('set')} ${variable('GOOGLE_GEMINI_BASE_URL')}${operator('=')}${string(baseUrl)}
${keyword('set')} ${variable('GEMINI_API_KEY')}${operator('=')}${string(apiKey)}
${keyword('set')} ${variable('GEMINI_MODEL')}${operator('=')}${string(model)}
${comment(`REM ${modelComment}`)}`
      break
    case 'powershell':
      path = 'PowerShell'
      content = `$env:GOOGLE_GEMINI_BASE_URL="${baseUrl}"
$env:GEMINI_API_KEY="${apiKey}"
$env:GEMINI_MODEL="${model}"  # ${modelComment}`
      highlighted = `${keyword('$env:')}${variable('GOOGLE_GEMINI_BASE_URL')}${operator('=')}${string(`"${baseUrl}"`)}
${keyword('$env:')}${variable('GEMINI_API_KEY')}${operator('=')}${string(`"${apiKey}"`)}
${keyword('$env:')}${variable('GEMINI_MODEL')}${operator('=')}${string(`"${model}"`)}  ${comment(`# ${modelComment}`)}`
      break
    default:
      path = 'Terminal'
      content = ''
      highlighted = ''
  }

  return { path, content, highlighted }
}

function generateOpenAIFiles(baseUrl: string, apiKey: string): FileConfig[] {
  const isWindows = activeTab.value === 'windows'
  const configDir = isWindows ? '%userprofile%\\.codex' : '~/.codex'

  const model = selectCodexCatalogModel(defaultModelOf('openai'))
  const reasoningEffortLine = codexReasoningEffortTomlLine(model)

  // config.toml content
  const configContent = `model_provider = "OpenAI"
model = "${model}"
review_model = "${model}"
${reasoningEffortLine}disable_response_storage = true
model_catalog_json = "${escapeTomlBasicString(codexModelCatalogPath.value)}"
network_access = "enabled"
windows_wsl_setup_acknowledged = true

[model_providers.OpenAI]
name = "OpenAI"
base_url = "${baseUrl}"
wire_api = "responses"
${generateCodexProviderAuthConfig(apiKey)}

[features]
goals = true`

  return buildOpenAICodexFileConfigs(configDir, configContent, apiKey)
}

function generateCodexProviderAuthConfig(apiKey: string): string {
  if (codexAuthMode.value === 'api-key') {
    return `requires_openai_auth = false
experimental_bearer_token = "${escapeTomlBasicString(apiKey)}"
http_headers = { "x-openai-actor-authorization" = "local-image-extension" }`
  }

  return 'requires_openai_auth = true'
}

function buildOpenAICodexFileConfigs(
  configDir: string,
  configContent: string,
  apiKey: string
): FileConfig[] {
  const files: FileConfig[] = [
    {
      path: `${configDir}/config.toml`,
      content: configContent,
      hint: t('keys.useKeyModal.openai.configTomlHint')
    }
  ]

  if (codexAuthMode.value === 'legacy') {
    files.push({
      path: `${configDir}/auth.json`,
      content: JSON.stringify({ OPENAI_API_KEY: apiKey }, null, 2)
    })
  }

  return files
}

function joinConfigPath(dir: string, file: string, windows: boolean): string {
  if (!windows) return `${dir}/${file}`
  return `${dir}\\${file}`
}

function escapeTomlBasicString(value: string): string {
  return value.replace(/\\/g, '\\\\').replace(/"/g, '\\"')
}

function generateGrokFiles(baseUrl: string, apiKey: string): FileConfig[] {
  // Prefer unix/cmd/powershell when shell tabs are shown; fall back to windows tab.
  const shell = activeTab.value
  const isWindowsPath = shell === 'windows' || shell === 'cmd' || shell === 'powershell'
  const configDir = isWindowsPath ? '%userprofile%\\.grok' : '~/.grok'

  let envPath: string
  let envContent: string
  switch (shell) {
    case 'cmd':
      envPath = 'Command Prompt'
      envContent = `set GROK_MODELS_BASE_URL=${baseUrl}
set XAI_API_KEY=${apiKey}`
      break
    case 'powershell':
    case 'windows':
      envPath = 'PowerShell'
      envContent = `$env:GROK_MODELS_BASE_URL="${baseUrl}"
$env:XAI_API_KEY="${apiKey}"`
      break
    default:
      envPath = 'Terminal'
      envContent = `export GROK_MODELS_BASE_URL="${baseUrl}"
export XAI_API_KEY="${apiKey}"`
  }

  // Shape follows Grok Build user guide (~/.grok/docs + custom-models).
  // 文本模型走 Responses；模型清单 = 上架目录里的 xAI 对话模型（新的在前，第一个是默认，D4）。
  // Credential order: api_key field → env_key → signed-in session → XAI_API_KEY global fallback.
  const modelsListUrl = `${baseUrl.replace(/\/+$/, '')}/models`
  const defaultModel = defaultModelOf('xai')
  const modelSections = modelsOf('xai')
    .map(
      (model) => `[model."${model.id}"]
model = "${model.id}"
name = "${model.name}"
env_key = "XAI_API_KEY"                     # or: api_key = "<your key>"  (not recommended)
api_backend = "responses"                   # chat_completions | responses | messages
supports_backend_search = true`
    )
    .join('\n\n')
  const media = xaiMedia.value
  const imageLines = media.image ? `image_gen = true\nimage_gen_model_override = "${media.image}"` : 'image_gen = false'
  const videoLine = media.video ? 'video_gen = true' : 'video_gen = false'
  const configContent = `# Grok Build CLI → ${siteName.value} (API key auth).
# Docs: ~/.grok/docs/user-guide/05-configuration.md + 11-custom-models.md
# Verify after save: grok inspect
#
# IMPORTANT: api_backend must be "responses" (POST /v1/responses).
# If omitted, Grok Build defaults to chat_completions (/v1/chat/completions).
#
# Prefer env_key over hardcoding api_key (never commit secrets).
# Also export GROK_MODELS_BASE_URL + XAI_API_KEY in the shell block above.

# Global inference / catalog endpoints (same role as env GROK_MODELS_BASE_URL).
# When models_base_url is set, Grok uses API-key Bearer auth (no grok login required).
[endpoints]
models_base_url = "${baseUrl}"              # inference base; model list defaults to {base}/models
models_list_url = "${modelsListUrl}"        # optional override (env: GROK_MODELS_LIST_URL)
xai_api_base_url = "${baseUrl}"             # public xAI API base override for gateway routing
cli_chat_proxy_base_url = "${baseUrl}"      # CLI chat-proxy base (env: GROK_CLI_CHAT_PROXY_BASE_URL)

[auth]
preferred_method = "api_key"

# xAI models listed on ${siteName.value}, newest first
${modelSections}

[models]
default = "${defaultModel}"
web_search = "${defaultModel}"              # client-side web_search tool model (must exist as [model.*])
image_description = "${defaultModel}"       # vision/describe-image helper model

[session]
auto_compact_threshold_percent = 80         # auto-compact at this % of context_window (default 85)

# Image / video generation: on only when ${siteName.value} lists an xAI image / video model
[features]
${imageLines}
${videoLine}`

  return [
    { path: envPath, content: envContent },
    {
      path: joinConfigPath(configDir, 'config.toml', isWindowsPath),
      content: configContent,
      hint: t('keys.useKeyModal.grok.configTomlHint')
    }
  ]
}

function generateOpenAIWsFiles(baseUrl: string, apiKey: string): FileConfig[] {
  const isWindows = activeTab.value === 'windows'
  const configDir = isWindows ? '%userprofile%\\.codex' : '~/.codex'
  const model = selectCodexCatalogModel(defaultModelOf('openai'))
  const reasoningEffortLine = codexReasoningEffortTomlLine(model)

  // config.toml content with WebSocket v2
  const configContent = `model_provider = "OpenAI"
model = "${model}"
review_model = "${model}"
${reasoningEffortLine}disable_response_storage = true
model_catalog_json = "${escapeTomlBasicString(codexModelCatalogPath.value)}"
network_access = "enabled"
windows_wsl_setup_acknowledged = true

[model_providers.OpenAI]
name = "OpenAI"
base_url = "${baseUrl}"
wire_api = "responses"
supports_websockets = true
${generateCodexProviderAuthConfig(apiKey)}

[features]
responses_websockets_v2 = true
goals = true`

  return buildOpenAICodexFileConfigs(configDir, configContent, apiKey)
}

function generateOpenCodeConfig(platform: string, baseUrl: string, apiKey: string, models: KeyCatalogModel[], pathLabel?: string): FileConfig {
  // 模型清单就是上架目录里这家的对话模型（D4，原来写死了一串目录里没有的）；OpenAI 走 Responses 不存会话
  const modelEntries = Object.fromEntries(
    models.map((model) => [model.id, platform === 'openai' ? { name: model.name, options: { store: false } } : { name: model.name }])
  )
  const provider: Record<string, any> = {
    [platform]: {
      options: {
        baseURL: baseUrl,
        apiKey
      }
    }
  }

  if (platform === 'gemini') {
    provider[platform].npm = '@ai-sdk/google'
  } else if (platform === 'anthropic') {
    provider[platform].npm = '@ai-sdk/anthropic'
  } else if (platform === 'grok') {
    // 自定义 provider，走站点的 OpenAI 兼容接口
    provider[platform].npm = '@ai-sdk/openai-compatible'
    provider[platform].name = `Grok via ${siteName.value}`
  }
  provider[platform].models = modelEntries

  const agent =
    platform === 'openai'
      ? {
          build: {
            options: {
              store: false
            }
          },
          plan: {
            options: {
              store: false
            }
          }
        }
      : undefined

  const content = JSON.stringify(
    {
      provider,
      ...(agent ? { agent } : {}),
      $schema: 'https://opencode.ai/config.json'
    },
    null,
    2
  )

  return {
    path: pathLabel ?? 'opencode.json',
    content,
    hint: t('keys.useKeyModal.opencode.hint')
  }
}

const copyContent = async (content: string, index: number) => {
  const success = await clipboardCopy(content)
  if (success) {
    copiedIndex.value = index
    setTimeout(() => {
      copiedIndex.value = null
    }, 2000)
  }
}
</script>
