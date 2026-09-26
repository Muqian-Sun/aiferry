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
              <span class="min-w-0 truncate text-xs text-af-ink-4 font-mono">{{ file.path }}</span>
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
const activeTab = ref<string>('unix')
const activeClientTab = ref<string>('claude')
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
    activeClientTab.value = 'claude'
    activeTab.value = 'unix'
    codexAuthMode.value = 'legacy'
  } else {
    resetCodexModelManifest()
  }
})

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

// 客户端标签页固定（清单与首页共用 USE_KEY_CLIENTS）：没有分组就没有「分组平台」，任何 key 都能走四种入站协议。
const clientTabs = computed((): TabConfig[] =>
  USE_KEY_CLIENTS.map((client) => ({
    id: client.id,
    label: t(client.labelKey),
    icon: client.id === 'gemini' ? SparkleIcon : TerminalIcon
  }))
)

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

const showShellTabs = computed(() => activeClientTab.value !== 'opencode')

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
const operator = (value: string) => wrapToken('text-af-ink-4', value)
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

  switch (activeClientTab.value) {
    case 'opencode':
      // 一个 key 四种协议都能走：一份 opencode.json 带四个 provider
      return [
        generateOpenCodeConfig('anthropic', apiBase, apiKey, 'opencode.json (Claude)'),
        generateOpenCodeConfig('openai', apiBase, apiKey, 'opencode.json (OpenAI)'),
        generateOpenCodeConfig('gemini', geminiBase, apiKey, 'opencode.json (Gemini)'),
        generateOpenCodeConfig('grok', apiBase, apiKey, 'opencode.json (Grok)')
      ]
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
  const model = 'gemini-2.0-flash'
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

  const model = selectCodexCatalogModel('gpt-5.5')
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

  // Shape follows Grok Build user guide (~/.grok/docs + custom-models) and production-ready Sub2API setups.
  // Text models only (Responses). Image/video: Imagine model IDs on media endpoints / feature overrides.
  // Credential order: api_key field → env_key → signed-in session → XAI_API_KEY global fallback.
  const modelsListUrl = `${baseUrl.replace(/\/+$/, '')}/models`
  const configContent = `# Grok Build CLI → ${siteName.value} Grok group (API key auth).
# Docs: ~/.grok/docs/user-guide/05-configuration.md + 11-custom-models.md
# Verify after save: grok inspect
#
# IMPORTANT: api_backend must be "responses" for the ${siteName.value} Grok group (POST /v1/responses).
# If omitted, Grok Build defaults to chat_completions (/v1/chat/completions).
# Keep api_backend = "responses" on every model entry.
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

# Prefer API key when using a custom gateway (matches ${siteName.value}).
# Requires XAI_API_KEY env or per-model env_key / api_key.
[auth]
preferred_method = "api_key"

[model."grok-4.5"]
model = "grok-4.5"                          # id sent to the API
name = "Grok 4.5"                           # shown in /model picker
description = "Grok 4.5 via ${siteName.value} (Responses)"
# base_url inherits from [endpoints].models_base_url; override only if needed:
# base_url = "${baseUrl}"
env_key = "XAI_API_KEY"                     # or: api_key = "${apiKey}"  (not recommended)
api_backend = "responses"                   # chat_completions | responses | messages
context_window = 500000                     # drives auto-compaction timing
# Optional sampling (global defaults can live under [models] instead):
# temperature = 0.7
# top_p = 0.95
# max_completion_tokens = 8192
# Server-side (backend) web_search tools — only if your gateway exposes them:
supports_backend_search = true

[model."grok-build-0.1"]
model = "grok-build-0.1"
name = "Grok Build"
description = "Coding / agent sessions (xAI recommends grok-build* for coding)"
env_key = "XAI_API_KEY"
api_backend = "responses"
context_window = 256000
supports_backend_search = true

# Text multi-agent / client web_search sub-agent (NOT Imagine image/video).
[model."grok-4.20-multi-agent-0309"]
model = "grok-4.20-multi-agent-0309"
name = "Grok 4.20 Multi Agent (text / web_search)"
description = "Text multi-agent; use for web_search sub-agent, not image/video"
env_key = "XAI_API_KEY"
api_backend = "responses"
context_window = 1000000
supports_backend_search = true

[model."grok-4.3"]
model = "grok-4.3"
name = "Grok 4.3"
env_key = "XAI_API_KEY"
api_backend = "responses"
context_window = 1000000
supports_backend_search = true

# Optional short alias for /model grok:
# [model."grok"]
# model = "grok-4.5"
# name = "Grok"
# env_key = "XAI_API_KEY"
# api_backend = "responses"
# context_window = 1000000
# supports_backend_search = true

[models]
# xAI recommends grok-build* for coding/agent sessions; use grok-4.5 for general chat.
default = "grok-4.5"
web_search = "grok-4.5"                     # client-side web_search tool model (must exist as [model.*])
image_description = "grok-4.5"              # vision/describe-image helper model
# Optional environment-wide sampling defaults (per-model values win):
# temperature = 0.7
# top_p = 0.95
# max_completion_tokens = 8192
# max_retries = 8

[session]
auto_compact_threshold_percent = 80         # auto-compact at this % of context_window (default 85)

# Imagine tools: model IDs go to the ${siteName.value} media endpoints (not the text [model.*] catalog).
# Enable only if the Grok group allows image/video generation.
[features]
image_gen = true
video_gen = true
image_gen_model_override = "grok-imagine-image-quality"   # or grok-imagine-image
image_edit_model_override = "grok-imagine-edit"
# Optional feature flags (defaults shown in docs):
# telemetry = false
# remote_fetch = true                         # set false for air-gapped / pure-gateway catalogs
# lsp_tools = false`

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
  const model = selectCodexCatalogModel('gpt-5.5')
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

function generateOpenCodeConfig(platform: string, baseUrl: string, apiKey: string, pathLabel?: string): FileConfig {
  const provider: Record<string, any> = {
    [platform]: {
      options: {
        baseURL: baseUrl,
        apiKey
      }
    }
  }
  const openaiModels = {
    'gpt-6': {
      name: 'GPT-6 (Astra)',
      limit: {
        context: 1050000,
        output: 128000
      },
      options: {
        store: false
      },
      variants: {
        low: {},
        medium: {},
        high: {},
        xhigh: {},
        max: {}
      }
    },
    'gpt-6-astra': {
      name: 'GPT-6 Astra',
      limit: {
        context: 1050000,
        output: 128000
      },
      options: {
        store: false
      },
      variants: {
        low: {},
        medium: {},
        high: {},
        xhigh: {},
        max: {}
      }
    },
    'gpt-5.2': {
      name: 'GPT-5.2',
      limit: {
        context: 400000,
        output: 128000
      },
      options: {
        store: false
      },
      variants: {
        low: {},
        medium: {},
        high: {},
        xhigh: {}
      }
    },
    'gpt-5.6': {
      name: 'GPT-5.6 (Sol)',
      limit: {
        context: 1050000,
        output: 128000
      },
      options: {
        store: false
      },
      variants: {
        low: {},
        medium: {},
        high: {},
        xhigh: {},
        max: {}
      }
    },
    'gpt-5.6-sol': {
      name: 'GPT-5.6 Sol',
      limit: {
        context: 1050000,
        output: 128000
      },
      options: {
        store: false
      },
      variants: {
        low: {},
        medium: {},
        high: {},
        xhigh: {},
        max: {}
      }
    },
    'gpt-5.6-terra': {
      name: 'GPT-5.6 Terra',
      limit: {
        context: 1050000,
        output: 128000
      },
      options: {
        store: false
      },
      variants: {
        low: {},
        medium: {},
        high: {},
        xhigh: {},
        max: {}
      }
    },
    'gpt-5.6-luna': {
      name: 'GPT-5.6 Luna',
      limit: {
        context: 1050000,
        output: 128000
      },
      options: {
        store: false
      },
      variants: {
        low: {},
        medium: {},
        high: {},
        xhigh: {},
        max: {}
      }
    },
    'gpt-5.5': {
      name: 'GPT-5.5',
      limit: {
        context: 1050000,
        output: 128000
      },
      options: {
        store: false
      },
      variants: {
        low: {},
        medium: {},
        high: {},
        xhigh: {}
      }
    },
    'gpt-5.4': {
      name: 'GPT-5.4',
      limit: {
        context: 1050000,
        output: 128000
      },
      options: {
        store: false
      },
      variants: {
        low: {},
        medium: {},
        high: {},
        xhigh: {}
      }
    },
    'gpt-5.4-mini': {
      name: 'GPT-5.4 Mini',
      limit: {
        context: 400000,
        output: 128000
      },
      options: {
        store: false
      },
      variants: {
        low: {},
        medium: {},
        high: {},
        xhigh: {}
      }
    },
    'gpt-5.3-codex-spark': {
      name: 'GPT-5.3 Codex Spark',
      limit: {
        context: 128000,
        output: 32000
      },
      options: {
        store: false
      },
      variants: {
        low: {},
        medium: {},
        high: {},
        xhigh: {}
      }
    },
    'codex-mini-latest': {
      name: 'Codex Mini',
      limit: {
        context: 200000,
        output: 100000
      },
      options: {
        store: false
      },
      variants: {
        low: {},
        medium: {},
        high: {}
      }
    }
  }
  const geminiModels = {
    'gemini-2.0-flash': {
      name: 'Gemini 2.0 Flash',
      limit: {
        context: 1048576,
        output: 65536
      },
      modalities: {
        input: ['text', 'image', 'pdf'],
        output: ['text']
      }
    },
    'gemini-2.5-flash': {
      name: 'Gemini 2.5 Flash',
      limit: {
        context: 1048576,
        output: 65536
      },
      modalities: {
        input: ['text', 'image', 'pdf'],
        output: ['text']
      }
    },
    'gemini-2.5-pro': {
      name: 'Gemini 2.5 Pro',
      limit: {
        context: 2097152,
        output: 65536
      },
      modalities: {
        input: ['text', 'image', 'pdf'],
        output: ['text']
      },
      options: {
        thinking: {
          budgetTokens: 24576,
          type: 'enabled'
        }
      }
    },
    'gemini-3.5-flash': {
      name: 'Gemini 3.5 Flash',
      limit: {
        context: 1048576,
        output: 65536
      },
      modalities: {
        input: ['text', 'image', 'pdf'],
        output: ['text']
      }
    },
    'gemini-3-flash-preview': {
      name: 'Gemini 3 Flash Preview',
      limit: {
        context: 1048576,
        output: 65536
      },
      modalities: {
        input: ['text', 'image', 'pdf'],
        output: ['text']
      }
    },
    'gemini-3-pro-preview': {
      name: 'Gemini 3 Pro Preview',
      limit: {
        context: 1048576,
        output: 65536
      },
      modalities: {
        input: ['text', 'image', 'pdf'],
        output: ['text']
      },
      options: {
        thinking: {
          budgetTokens: 24576,
          type: 'enabled'
        }
      }
    },
    'gemini-3.1-pro-preview': {
      name: 'Gemini 3.1 Pro Preview',
      limit: {
        context: 1048576,
        output: 65536
      },
      modalities: {
        input: ['text', 'image', 'pdf'],
        output: ['text']
      },
      options: {
        thinking: {
          budgetTokens: 24576,
          type: 'enabled'
        }
      }
    }
  }

  const grokModels = {
    'grok-4.5': {
      name: 'Grok 4.5',
      limit: { context: 500000, output: 64000 }
    },
    'grok-build-0.1': {
      name: 'Grok Build 0.1',
      limit: { context: 256000, output: 64000 }
    },
    'grok-4.20-multi-agent-0309': {
      name: 'Grok 4.20 Multi Agent (text / web_search)',
      limit: { context: 1000000, output: 64000 }
    },
    'grok-4.3': {
      name: 'Grok 4.3',
      limit: { context: 1000000, output: 64000 }
    },
    'grok-composer-2.5-fast': {
      name: 'Grok Composer 2.5 Fast',
      limit: { context: 500000, output: 64000 }
    }
  }

  if (platform === 'gemini') {
    provider[platform].npm = '@ai-sdk/google'
    provider[platform].models = geminiModels
  } else if (platform === 'anthropic') {
    provider[platform].npm = '@ai-sdk/anthropic'
  } else if (platform === 'openai') {
    provider[platform].models = openaiModels
  } else if (platform === 'grok') {
    // Custom provider pointing at Sub2API OpenAI-compatible Responses/Chat endpoints.
    provider[platform].npm = '@ai-sdk/openai-compatible'
    provider[platform].name = `Grok via ${siteName.value}`
    provider[platform].models = grokModels
  }

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
  const success = await clipboardCopy(content, t('keys.copied'))
  if (success) {
    copiedIndex.value = index
    setTimeout(() => {
      copiedIndex.value = null
    }, 2000)
  }
}
</script>
