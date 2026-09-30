<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.bulkEdit.title')"
    width="wide"
    @close="handleClose"
  >
    <form id="bulk-edit-account-form" class="space-y-5" @submit.prevent="() => handleSubmit()">
      <!-- Info -->
      <div class="rounded-lg bg-af-sunken p-4">
        <p class="text-sm text-af-ink-2">
          <svg class="mr-1.5 inline h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
          {{ t('admin.accounts.bulkEdit.selectionInfo', { count: targetMode === 'filtered' ? targetPreviewCount : accountIds.length }) }}
        </p>
      </div>

      <!-- Mixed platform warning -->
      <div v-if="isMixedPlatform" class="rounded-lg bg-af-warning-tint p-4">
        <p class="text-sm text-af-warning">
          <svg class="mr-1.5 inline h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
          {{ t('admin.accounts.bulkEdit.mixedPlatformWarning', { platforms: targetSelectedPlatforms.join(', ') }) }}
        </p>
      </div>

      <!-- 模型改名（映射只改名，muqian 2026-09-25 去掉白名单） -->
      <div class="border-t border-af-hairline pt-4">
        <div class="mb-3 flex items-center justify-between">
          <label
            id="bulk-edit-model-restriction-label"
            class="input-label mb-0"
            for="bulk-edit-model-restriction-enabled"
          >
            {{ t('admin.accounts.modelRename.title') }}
          </label>
          <input
            v-model="enableModelRestriction"
            id="bulk-edit-model-restriction-enabled"
            type="checkbox"
            aria-controls="bulk-edit-model-restriction-body"
            class="rounded border-af-hairline-strong text-af-brand focus:ring-af-brand"
          />
        </div>

        <div
          id="bulk-edit-model-restriction-body"
          :class="!enableModelRestriction && 'pointer-events-none opacity-50'"
          role="group"
          aria-labelledby="bulk-edit-model-restriction-label"
        >
          <ModelRenameEditor
            v-model="modelMappings"
            :show-title="false"
            :presets="renamePresets"
          />
        </div>
      </div>

      <!-- Intercept warmup requests (Anthropic only) -->
      <div class="border-t border-af-hairline pt-4">
        <div class="flex items-center justify-between gap-4">
          <div class="flex-1 pr-4">
            <label
              id="bulk-edit-intercept-warmup-label"
              class="input-label mb-0"
              for="bulk-edit-intercept-warmup-enabled"
            >
              {{ t('admin.accounts.interceptWarmupRequests') }}
            </label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.interceptWarmupRequestsDesc') }}
            </p>
          </div>
          <input
            v-model="enableInterceptWarmup"
            id="bulk-edit-intercept-warmup-enabled"
            type="checkbox"
            aria-controls="bulk-edit-intercept-warmup-body"
            class="rounded border-af-hairline-strong text-af-brand focus:ring-af-brand"
          />
        </div>
        <div v-if="enableInterceptWarmup" id="bulk-edit-intercept-warmup-body" class="mt-3">
          <button
            type="button"
            :class="[
              'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
              interceptWarmupRequests ? 'bg-af-brand' : 'bg-af-hairline'
            ]"
            @click="interceptWarmupRequests = !interceptWarmupRequests"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                interceptWarmupRequests ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
        </div>
      </div>

      <!-- Header Override (eligible API-key platforms + grok OAuth) -->
      <div v-if="allHeaderOverrideCapable" class="border-t border-af-hairline pt-4">
        <div class="flex items-center justify-between gap-4">
          <div class="flex-1 pr-4">
            <label
              id="bulk-edit-header-override-label"
              class="input-label mb-0"
              for="bulk-edit-header-override-enabled"
            >
              {{ t('admin.accounts.headerOverride.title') }}
            </label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.headerOverride.hint') }}
            </p>
          </div>
          <input
            v-model="enableHeaderOverride"
            id="bulk-edit-header-override-enabled"
            type="checkbox"
            aria-controls="bulk-edit-header-override-body"
            class="rounded border-af-hairline-strong text-af-brand focus:ring-af-brand"
          />
        </div>
        <div v-if="enableHeaderOverride" id="bulk-edit-header-override-body" class="mt-3 space-y-3">
          <button
            type="button"
            :class="[
              'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
              headerOverrideEnabled ? 'bg-af-brand' : 'bg-af-hairline'
            ]"
            @click="headerOverrideEnabled = !headerOverrideEnabled"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                headerOverrideEnabled ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>

          <div v-if="headerOverrideEnabled" class="space-y-3">
            <div class="rounded-lg bg-af-sunken p-3">
              <p class="text-xs text-af-ink-2">
                <Icon name="exclamationCircle" size="sm" class="mr-1 inline" :stroke-width="2" />
                {{ t('admin.accounts.headerOverride.info') }}
              </p>
            </div>

            <p class="text-xs text-af-warning">
              {{ t('admin.accounts.headerOverride.bulkReplaceHint') }}
            </p>

            <HeaderOverrideEditor
              :rows="headerOverrideRows"
              @update:rows="headerOverrideRows = $event"
            />
          </div>
          <p v-else class="text-xs text-af-ink-3">
            {{ t('admin.accounts.headerOverride.bulkDisableHint') }}
          </p>
        </div>
      </div>

      <!-- Proxy -->
      <div class="border-t border-af-hairline pt-4">
        <div class="mb-3 flex items-center justify-between">
          <label
            id="bulk-edit-proxy-label"
            class="input-label mb-0"
            for="bulk-edit-proxy-enabled"
          >
            {{ t('admin.accounts.proxy') }}
          </label>
          <input
            v-model="enableProxy"
            id="bulk-edit-proxy-enabled"
            type="checkbox"
            aria-controls="bulk-edit-proxy-body"
            class="rounded border-af-hairline-strong text-af-brand focus:ring-af-brand"
          />
        </div>
        <div id="bulk-edit-proxy-body" :class="!enableProxy && 'pointer-events-none opacity-50'">
          <ProxySelector
            v-model="proxyId"
            :proxies="proxies"
            aria-labelledby="bulk-edit-proxy-label"
          />
        </div>
      </div>

      <!-- Concurrency & Priority -->
      <div class="grid grid-cols-2 gap-4 border-t border-af-hairline pt-4">
        <div>
          <div class="mb-3 flex items-center justify-between">
            <label
              id="bulk-edit-concurrency-label"
              class="input-label mb-0"
              for="bulk-edit-concurrency-enabled"
            >
              {{ t('admin.accounts.concurrency') }}
            </label>
            <input
              v-model="enableConcurrency"
              id="bulk-edit-concurrency-enabled"
              type="checkbox"
              aria-controls="bulk-edit-concurrency"
              class="rounded border-af-hairline-strong text-af-brand focus:ring-af-brand"
            />
          </div>
          <input
            v-model.number="concurrency"
            id="bulk-edit-concurrency"
            type="number"
            min="1"
            :disabled="!enableConcurrency"
            class="input"
            :class="!enableConcurrency && 'cursor-not-allowed opacity-50'"
            aria-labelledby="bulk-edit-concurrency-label"
            @input="concurrency = Math.max(1, concurrency || 1)"
          />
        </div>
        <div>
          <div class="mb-3 flex items-center justify-between">
            <label
              id="bulk-edit-priority-label"
              class="input-label mb-0"
              for="bulk-edit-priority-enabled"
            >
              {{ t('admin.accounts.priority') }}
            </label>
            <input
              v-model="enablePriority"
              id="bulk-edit-priority-enabled"
              type="checkbox"
              aria-controls="bulk-edit-priority"
              class="rounded border-af-hairline-strong text-af-brand focus:ring-af-brand"
            />
          </div>
          <input
            v-model.number="priority"
            id="bulk-edit-priority"
            type="number"
            min="1"
            :disabled="!enablePriority"
            class="input"
            :class="!enablePriority && 'cursor-not-allowed opacity-50'"
            aria-labelledby="bulk-edit-priority-label"
          />
        </div>
      </div>

      <!-- Status -->
      <div class="border-t border-af-hairline pt-4">
        <div class="mb-3 flex items-center justify-between">
          <label
            id="bulk-edit-status-label"
            class="input-label mb-0"
            for="bulk-edit-status-enabled"
          >
            {{ t('common.status') }}
          </label>
          <input
            v-model="enableStatus"
            id="bulk-edit-status-enabled"
            type="checkbox"
            aria-controls="bulk-edit-status"
            class="rounded border-af-hairline-strong text-af-brand focus:ring-af-brand"
          />
        </div>
        <div id="bulk-edit-status" :class="!enableStatus && 'pointer-events-none opacity-50'">
          <Select
            v-model="status"
            :options="statusOptions"
            aria-labelledby="bulk-edit-status-label"
          />
        </div>
      </div>

      <!-- RPM Limit (仅全部为 Anthropic OAuth/SetupToken 时显示) -->
      <div v-if="allAnthropicOAuthOrSetupToken" class="border-t border-af-hairline pt-4">
        <div class="mb-3 flex items-center justify-between">
          <label
            id="bulk-edit-rpm-limit-label"
            class="input-label mb-0"
            for="bulk-edit-rpm-limit-enabled"
          >
            {{ t('admin.accounts.quotaControl.rpmLimit.label') }}
          </label>
          <input
            v-model="enableRpmLimit"
            id="bulk-edit-rpm-limit-enabled"
            type="checkbox"
            aria-controls="bulk-edit-rpm-limit-body"
            class="rounded border-af-hairline-strong text-af-brand focus:ring-af-brand"
          />
        </div>

        <div
          id="bulk-edit-rpm-limit-body"
          :class="!enableRpmLimit && 'pointer-events-none opacity-50'"
          role="group"
          aria-labelledby="bulk-edit-rpm-limit-label"
        >
          <div class="mb-3 flex items-center justify-between">
            <span class="text-sm text-af-ink-2">{{ t('admin.accounts.quotaControl.rpmLimit.hint') }}</span>
            <button
              type="button"
              @click="rpmLimitEnabled = !rpmLimitEnabled"
              :class="[
                'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
                rpmLimitEnabled ? 'bg-af-brand' : 'bg-af-hairline'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                  rpmLimitEnabled ? 'translate-x-5' : 'translate-x-0'
                ]"
              />
            </button>
          </div>

          <div v-if="rpmLimitEnabled" class="space-y-3">
            <div>
              <label class="input-label text-xs">{{ t('admin.accounts.quotaControl.rpmLimit.baseRpm') }}</label>
              <input
                v-model.number="bulkBaseRpm"
                type="number"
                min="1"
                max="1000"
                step="1"
                class="input"
                :placeholder="t('admin.accounts.quotaControl.rpmLimit.baseRpmPlaceholder')"
              />
              <p class="input-hint">{{ t('admin.accounts.quotaControl.rpmLimit.baseRpmHint') }}</p>
            </div>
          </div>
        </div>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="bulk-edit-account-form"
          :disabled="submitting"
          class="btn btn-primary"
        >
          <svg
            v-if="submitting"
            class="-ml-1 mr-2 h-4 w-4 animate-spin"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              class="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              stroke-width="4"
            />
            <path
              class="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            />
          </svg>
          {{
            submitting ? t('admin.accounts.bulkEdit.updating') : t('admin.accounts.bulkEdit.submit')
          }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  Proxy as ProxyConfig,
  AccountPlatform,
  AccountType
} from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import ProxySelector from '@/components/common/ProxySelector.vue'
import ModelRenameEditor from '@/components/account/ModelRenameEditor.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  buildModelMappingObject as buildModelMappingPayload,
  renamePresetsFor
} from '@/composables/useModelWhitelist'
import HeaderOverrideEditor from '@/components/account/HeaderOverrideEditor.vue'
import {
  buildHeaderOverridesObject,
  isHeaderOverrideCapable,
  validateHeaderOverrideRows,
  HEADER_OVERRIDES_CREDENTIAL_KEY,
  type HeaderOverrideRow
} from '@/components/account/credentialsBuilder'
interface Props {
  show: boolean
  accountIds: number[]
  selectedPlatforms: AccountPlatform[]
  selectedTypes: AccountType[]
  target?: {
    mode: 'selected' | 'filtered'
    filters?: Record<string, unknown>
    previewCount?: number
    selectedPlatforms?: AccountPlatform[]
    selectedTypes?: AccountType[]
  }
  proxies: ProxyConfig[]
}

const props = defineProps<Props>()
const emit = defineEmits<{
  close: []
  updated: []
}>()

const { t } = useI18n()

// Platform awareness
const targetMode = computed(() => props.target?.mode ?? 'selected')
const targetPreviewCount = computed(() => props.target?.previewCount ?? props.accountIds.length)
const targetSelectedPlatforms = computed(() => props.target?.selectedPlatforms ?? props.selectedPlatforms)
const targetSelectedTypes = computed(() => props.target?.selectedTypes ?? props.selectedTypes)
const isMixedPlatform = computed(() => targetSelectedPlatforms.value.length > 1)

// 是否全部为支持请求头覆写的平台/账号类型
// 所选平台 × 所选类型的全组合均需具备覆写资格（实际选中账号是该组合的子集，
// 按交叉积判定偏保守但绝不放行不合资格的账号）
const allHeaderOverrideCapable = computed(() => {
  return (
    targetSelectedPlatforms.value.length > 0 &&
    targetSelectedTypes.value.length > 0 &&
    targetSelectedPlatforms.value.every(p =>
      targetSelectedTypes.value.every(ty => isHeaderOverrideCapable(p, ty))
    )
  )
})

// 是否全部为 Anthropic OAuth/SetupToken（RPM 配置仅在此条件下显示）
const allAnthropicOAuthOrSetupToken = computed(() => {
  return (
    targetSelectedPlatforms.value.length === 1 &&
    targetSelectedPlatforms.value[0] === 'anthropic' &&
    targetSelectedTypes.value.every(t => t === 'oauth' || t === 'setup-token')
  )
})

// 改名快捷项（同名预设只对自带模型表的上游保留，见 renamePresetsFor）
const renamePresets = computed(() => {
  if (targetSelectedPlatforms.value.length === 0) return []

  const dedupedPresets = new Map<string, ReturnType<typeof renamePresetsFor>[number]>()
  for (const platform of targetSelectedPlatforms.value) {
    for (const preset of renamePresetsFor(platform)) {
      const key = `${preset.from}=>${preset.to}`
      if (!dedupedPresets.has(key)) {
        dedupedPresets.set(key, preset)
      }
    }
  }

  return Array.from(dedupedPresets.values())
})

// Model mapping type
interface ModelMapping {
  from: string
  to: string
}

// State - field enable flags
const enableModelRestriction = ref(false)
const enableInterceptWarmup = ref(false)
const enableHeaderOverride = ref(false)
const enableProxy = ref(false)
const enableConcurrency = ref(false)
const enablePriority = ref(false)
const enableStatus = ref(false)
const enableRpmLimit = ref(false)

// State - field values
const submitting = ref(false)
const modelMappings = ref<ModelMapping[]>([])
const interceptWarmupRequests = ref(false)
// 请求头覆写：覆写表里有条目就生效（渠道级开关已删）。开 = 用下方条目整表替换，关 = 清空所选渠道的覆写表
const headerOverrideEnabled = ref(false)
const headerOverrideRows = ref<HeaderOverrideRow[]>([])
const proxyId = ref<number | null>(null)
const concurrency = ref(1)
const priority = ref(1)
const status = ref<'active' | 'inactive'>('active')
const rpmLimitEnabled = ref(false)
const bulkBaseRpm = ref<number | null>(null)

const statusOptions = computed(() => [
  { value: 'active', label: t('common.active') },
  { value: 'inactive', label: t('common.inactive') }
])
const buildModelMappingObject = (): Record<string, string> | null => {
  return buildModelMappingPayload('mapping', [], modelMappings.value)
}

const buildUpdatePayload = (): Record<string, unknown> | null => {
  const updates: Record<string, unknown> = {}
  const credentials: Record<string, unknown> = {}
  let credentialsChanged = false
  const ensureExtra = (): Record<string, unknown> => {
    if (!updates.extra) {
      updates.extra = {}
    }
    return updates.extra as Record<string, unknown>
  }

  if (enableProxy.value) {
    // 后端期望 proxy_id: 0 表示清除代理，而不是 null
    updates.proxy_id = proxyId.value === null ? 0 : proxyId.value
  }

  if (enableConcurrency.value) {
    updates.concurrency = concurrency.value
  }

  if (enablePriority.value) {
    updates.priority = priority.value
  }

  if (enableStatus.value) {
    updates.status = status.value
  }

  if (enableModelRestriction.value) {
    // 映射只改名：空配置显式发空对象，覆盖各账号已有的映射；批量路径按 JSONB 顶层键合并，标记一并写入
    credentials.model_mapping = buildModelMappingObject() ?? {}
    credentials.model_mapping_rename_only = true
    credentialsChanged = true
  }

  if (enableInterceptWarmup.value) {
    credentials.intercept_warmup_requests = interceptWarmupRequests.value
    credentialsChanged = true
  }

  if (enableHeaderOverride.value) {
    // 后端使用 JSONB || merge 语义：「关」显式写入空对象，清空各渠道已有的覆写表
    credentials[HEADER_OVERRIDES_CREDENTIAL_KEY] = headerOverrideEnabled.value
      ? buildHeaderOverridesObject(headerOverrideRows.value)
      : {}
    credentialsChanged = true
  }

  // RPM limit settings (写入 extra 字段；RPM 策略与粘性缓冲写死在后端)
  if (enableRpmLimit.value) {
    const extra = ensureExtra()
    // 关闭 RPM 限制时显式写 0（后端 JSONB || merge 不会删除已有 key）
    extra.base_rpm = rpmLimitEnabled.value && bulkBaseRpm.value != null && bulkBaseRpm.value > 0
      ? bulkBaseRpm.value
      : 0
  }

  if (credentialsChanged) {
    updates.credentials = credentials
  }

  return Object.keys(updates).length > 0 ? updates : null
}

const handleClose = () => {
  emit('close')
}

const handleSubmit = async () => {
  if (targetMode.value === 'selected' && props.accountIds.length === 0) {
    console.error(t('admin.accounts.bulkEdit.noSelection'))
    return
  }

  const hasAnyFieldEnabled =
    enableModelRestriction.value ||
    enableInterceptWarmup.value ||
    enableHeaderOverride.value ||
    enableProxy.value ||
    enableConcurrency.value ||
    enablePriority.value ||
    enableStatus.value ||
    enableRpmLimit.value

  if (!hasAnyFieldEnabled) {
    console.error(t('admin.accounts.bulkEdit.noFieldsSelected'))
    return
  }

  if (enableHeaderOverride.value && headerOverrideEnabled.value) {
    // 批量保存对 header_overrides 是整键替换：开启但没有任何有效行会把所选账号的
    // 既有覆写配置静默清空，必须显式拦截（清空请走关闭开关的路径，有专门提示）
    if (!headerOverrideRows.value.some((row) => row.name.trim())) {
      console.error(t('admin.accounts.headerOverride.bulkEmptyRows'))
      return
    }
    const headerError = validateHeaderOverrideRows(headerOverrideRows.value)
    if (headerError) {
      console.error(t(`admin.accounts.headerOverride.${headerError}`))
      return
    }
  }

  const built = buildUpdatePayload()
  if (!built) {
    console.error(t('admin.accounts.bulkEdit.noFieldsSelected'))
    return
  }

  await submitBulkUpdate(built)
}

const submitBulkUpdate = async (updates: Record<string, unknown>) => {
  submitting.value = true

  try {
    const res = targetMode.value === 'filtered' && props.target?.filters
      ? await adminAPI.accounts.bulkUpdate({
        filters: props.target.filters,
        ...updates
      })
      : await adminAPI.accounts.bulkUpdate(props.accountIds, updates)
    const success = res.success || 0
    const failed = res.failed || 0

    if (success > 0 && failed > 0) {
      console.error(t('admin.accounts.bulkEdit.partialSuccess', { success, failed }))
    } else if (success === 0) {
      console.error(t('admin.accounts.bulkEdit.failed'))
    }

    if (success > 0) {
      emit('updated')
      handleClose()
    }
  } catch (error: any) {
    console.error('Error bulk updating accounts:', error)
  } finally {
    submitting.value = false
  }
}

// Reset form when modal closes
watch(
  () => props.show,
  (newShow) => {
    if (!newShow) {
      // Reset all enable flags
      enableModelRestriction.value = false
      enableInterceptWarmup.value = false
      enableHeaderOverride.value = false
      enableProxy.value = false
      enableConcurrency.value = false
      enablePriority.value = false
      enableStatus.value = false
      enableRpmLimit.value = false

      // Reset all values
      modelMappings.value = []
      interceptWarmupRequests.value = false
      headerOverrideEnabled.value = false
      headerOverrideRows.value = []
      proxyId.value = null
      concurrency.value = 1
      priority.value = 1
      status.value = 'active'
      rpmLimitEnabled.value = false
      bulkBaseRpm.value = null
    }
  }
)
</script>
