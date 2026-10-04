<template>
  <!--
    编辑渠道弹窗（2026-10-03 由整页改回弹窗）：与新建同一外壳、同一分区顺序 ——
    上游 / 调度与限额 / 高级（默认收起）/ 备注。完整账号由渠道列表按 id 拉好再传进来，拉取期间 account 为 null。
  -->
  <BaseDialog :show="show" :title="dialogTitle" width="wide" @close="handleClose">
    <div v-if="!account" class="py-6" data-testid="account-edit-loading">
      <FormError v-if="loadError" :message="loadError" />
      <p v-else class="flex items-center gap-2 text-sm text-af-ink-3">
        <Icon name="refresh" size="sm" class="animate-spin" />
        {{ t('admin.accounts.dialog.loading') }}
      </p>
    </div>

    <form v-else id="edit-account-form" class="space-y-5" @submit.prevent="handleSubmit">
      <ChannelFormSection section="upstream" :title="t('admin.accounts.dialog.sections.upstream')">
        <!-- 第三方 key：地址（常用官方地址在地址框右边）、套餐、API Key；与新建同一顺序，代理、检测上游、名称在后面 -->
        <template v-if="account.type === 'apikey'">
          <div>
            <ProtocolEndpointsEditor
              v-model="editProtocolEndpoints"
              :protocols="UPSTREAM_PROTOCOLS"
              :official-endpoints="officialProtocolEndpoints"
              :defaults-load-failed="protocolDefaultsLoadFailed"
            >
              <template #url-suffix>
                <KeyAddressPresetMenu
                  v-if="keyPresets.length > 0"
                  class="sm:w-48 sm:shrink-0"
                  :presets="keyPresets"
                  @select="applyKeyAddressPreset"
                />
              </template>
            </ProtocolEndpointsEditor>
            <p v-if="keyVendor" class="input-hint" data-testid="key-vendor-detected">
              {{ t('admin.accounts.keyAddress.detected', { vendor: keyVendorLabel }) }}
            </p>
            <p v-else-if="hasKeyAddress" class="input-hint" data-testid="key-vendor-relay">
              {{ t('admin.accounts.keyAddress.relay') }}
            </p>
          </div>

          <!-- 地址分得出套餐就不问；MiniMax 两种套餐同一个地址、智谱 Anthropic 同地址才要选 -->
          <KeyPlanModePicker v-if="keyPlanNeedsChoice" v-model="editAccountMode" test-id="edit-key-plan-mode" />

          <div>
            <label class="input-label">{{ t('admin.accounts.apiKey') }}</label>
            <input
              v-model="editApiKey"
              type="password"
              class="input font-mono"
              autocomplete="new-password"
              data-1p-ignore
              data-lpignore="true"
              data-bwignore="true"
              :placeholder="apiKeyValuePlaceholder"
            />
            <p class="input-hint">{{ t('admin.accounts.leaveEmptyToKeep') }}</p>
          </div>
        </template>

        <!-- Vertex Service Account：区域（Project ID 由后端从 Service Account JSON 里取） -->
        <div v-if="(account.platform === 'gemini' || account.platform === 'anthropic') && account.type === 'service_account'">
          <label class="input-label">Location</label>
          <select v-model="editVertexLocation" required class="input font-mono">
            <optgroup v-for="group in VERTEX_LOCATION_OPTIONS" :key="group.label" :label="group.label">
              <option v-for="option in group.options" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </optgroup>
          </select>
          <p class="input-hint">{{ t('admin.accounts.vertexLocationHint') }}</p>
          <p class="input-hint">{{ t('admin.accounts.vertexSaJsonEditHint') }}</p>
        </div>

        <!-- Bedrock：凭证（SigV4 与 API Key 两种模式）、区域与全局推理 -->
        <template v-if="account.type === 'bedrock'">
          <template v-if="!isBedrockAPIKeyMode">
            <div>
              <label class="input-label">{{ t('admin.accounts.bedrockAccessKeyId') }}</label>
              <input v-model="editBedrockAccessKeyId" type="text" class="input font-mono" placeholder="AKIA..." />
            </div>
            <div>
              <label class="input-label">{{ t('admin.accounts.bedrockSecretAccessKey') }}</label>
              <input
                v-model="editBedrockSecretAccessKey"
                type="password"
                class="input font-mono"
                :placeholder="t('admin.accounts.bedrockSecretKeyLeaveEmpty')"
              />
              <p class="input-hint">{{ t('admin.accounts.bedrockSecretKeyLeaveEmpty') }}</p>
            </div>
          </template>
          <div v-else>
            <label class="input-label">{{ t('admin.accounts.bedrockApiKeyInput') }}</label>
            <input
              v-model="editBedrockApiKeyValue"
              type="password"
              class="input font-mono"
              :placeholder="t('admin.accounts.bedrockApiKeyLeaveEmpty')"
            />
            <p class="input-hint">{{ t('admin.accounts.bedrockApiKeyLeaveEmpty') }}</p>
          </div>
          <div>
            <label class="input-label">{{ t('admin.accounts.bedrockRegion') }}</label>
            <input v-model="editBedrockRegion" type="text" class="input" placeholder="us-east-1" />
            <p class="input-hint">{{ t('admin.accounts.bedrockRegionHint') }}</p>
          </div>
          <div>
            <label class="flex cursor-pointer items-center gap-2">
              <input
                v-model="editBedrockForceGlobal"
                type="checkbox"
                class="rounded border-af-hairline-strong text-af-brand focus:ring-af-brand"
              />
              <span class="text-sm text-af-ink-2">{{ t('admin.accounts.bedrockForceGlobal') }}</span>
            </label>
            <p class="input-hint mt-1">{{ t('admin.accounts.bedrockForceGlobalHint') }}</p>
          </div>
        </template>
        <!-- Spark 影子号的代理恒继承母账号，不可单独改 -->
        <div v-if="!isSparkShadow">
          <label class="input-label">{{ t('admin.accounts.proxy') }}</label>
          <ProxySelector v-model="form.proxy_id" :proxies="proxies" />
        </div>

        <!-- 检测上游：没改 key 时用存着的 key，地址以表单为准；选中的协议与地址填回上面。承接改在价格页 -->
        <UpstreamDetect
          v-if="account.type === 'apikey'"
          :protocol-endpoints="editProtocolEndpoints"
          :api-key="editApiKey"
          :account-id="account.id"
          :proxy-id="form.proxy_id"
          @select="(protocol, url) => (editProtocolEndpoints = { [protocol]: url })"
        >
          <template #models="{ classified }">
            <p class="flex flex-wrap items-center gap-x-2 text-af-ink-2" data-testid="upstream-detect-unbound">
              {{ t('admin.accounts.upstreamDetect.unbound', { count: unboundCount(classified) }) }}
              <RouterLink :to="{ path: '/pricing', query: { channel: String(account.id) } }" class="font-medium text-af-ink hover:underline">
                {{ t('admin.accounts.upstreamDetect.openPricing') }}
              </RouterLink>
            </p>
          </template>
        </UpstreamDetect>

        <div>
          <label class="input-label">{{ t('common.name') }}</label>
          <input v-model="form.name" type="text" required class="input" />
        </div>

      </ChannelFormSection>

      <ChannelFormSection section="scheduling" :title="t('admin.accounts.dialog.sections.scheduling')">
        <div class="sm:w-1/3">
          <label class="input-label">{{ t('common.status') }}</label>
          <Select v-model="form.status" :options="statusOptions" />
        </div>

        <ChannelLimitsFields
          v-model:priority="form.priority"
          v-model:concurrency="form.concurrency"
          v-model:expires-at="form.expires_at"
        />

        <ChannelQuotaFields
          v-if="account.type === 'apikey' || account.type === 'bedrock'"
          v-model:total-limit="editQuotaLimit"
          v-model:daily-limit="editQuotaDailyLimit"
          v-model:weekly-limit="editQuotaWeeklyLimit"
        />

        <AnthropicSubscriptionLimits
          v-if="account.platform === 'anthropic' && (account.type === 'oauth' || account.type === 'setup-token')"
          v-model:session-limit-enabled="sessionLimitEnabled"
          v-model:max-sessions="maxSessions"
          v-model:rpm-limit-enabled="rpmLimitEnabled"
          v-model:base-rpm="baseRpm"
        />

        <ChannelSettingToggle
          v-if="account.platform === 'openai' && account.type === 'oauth' && !isSparkShadow"
          v-model="autoResetCreditEnabled"
          :label="t('admin.accounts.autoResetCredit.title')"
          :description="t('admin.accounts.autoResetCredit.hint')"
          test-id="auto-reset-credit-settings"
          toggle-test-id="auto-reset-credit-enabled"
        />

        <!-- 超量：Antigravity 成品号（OAuth）专属；第三方 key 按协议调度，没有这一项 -->
        <ChannelSettingToggle
          v-if="account.platform === 'antigravity' && account.type === 'oauth'"
          v-model="allowOverages"
          :label="t('admin.accounts.allowOverages')"
          :description="t('admin.accounts.allowOveragesTooltip')"
          test-id="allow-overages"
        />

        <OllamaCloudUsageSettings
          v-if="account.ollama_cloud_usage?.eligible"
          :account="account"
          @updated="handleOllamaCloudUsageUpdated"
        />
      </ChannelFormSection>

      <ChannelAdvancedSection v-if="advancedItems.length > 0" :summary="advancedItems.join(' · ')">
        <!-- 请求头覆写：任何第三方 key + Grok 成品号 -->
        <HeaderOverrideField
          v-if="headerOverrideCapable"
          v-model:rows="headerOverrideRows"
          test-id="edit-header-override"
        />

        <!-- 池模式：同渠道重试次数与状态码写死在后端（channel_features.go），这里只有开关 -->
        <ChannelSettingToggle
          v-if="account.type === 'apikey'"
          v-model="poolModeEnabled"
          :label="t('admin.accounts.poolMode')"
          :description="t('admin.accounts.poolModeHint')"
          test-id="pool-mode"
        >
          <p class="rounded-lg bg-af-sunken p-3 text-xs text-af-ink-2">
            <Icon name="exclamationCircle" size="sm" class="mr-1 inline" :stroke-width="2" />
            {{ t('admin.accounts.poolModeInfo') }}
          </p>
        </ChannelSettingToggle>

        <!-- 第三方 key 的 Anthropic 协议设置：配了 anthropic 协议地址才展示，不看平台标签 -->
        <AnthropicKeySettings
          v-if="anthropicKeySettingsVisible"
          v-model:auth-scheme="anthropicAPIKeyAuthScheme"
          v-model:bedrock-cc-compat="bedrockCCCompatEnabled"
          test-id-prefix="edit"
        />

        <ChannelSettingToggle
          v-if="interceptWarmupCapable"
          v-model="interceptWarmupRequests"
          :label="t('admin.accounts.interceptWarmupRequests')"
          :description="t('admin.accounts.interceptWarmupRequestsDesc')"
          test-id="intercept-warmup"
        />

        <!-- 智谱团队版 Coding Plan：组织 / 项目 ID（可选，填写后用量查询走团队版端点） -->
        <ZhipuTeamFields
          v-if="zhipuTeamCapable"
          v-model:organization="editZhipuOrganization"
          v-model:project="editZhipuProject"
        />

        <div v-if="antigravityProjectIdCapable">
          <label class="input-label">{{ t('admin.accounts.antigravityProjectIdLabel') }}</label>
          <input
            v-model="antigravityProjectId"
            data-testid="antigravity-project-id-input"
            type="text"
            class="input font-mono"
            :placeholder="t('admin.accounts.antigravityProjectIdPlaceholder')"
          />
          <p class="input-hint">{{ t('admin.accounts.antigravityProjectIdHint') }}</p>
        </div>
      </ChannelAdvancedSection>

      <ChannelFormSection section="notes" :title="t('admin.accounts.dialog.sections.notes')">
        <div>
          <textarea
            v-model="form.notes"
            rows="3"
            class="input"
            :aria-label="t('admin.accounts.notes')"
            :placeholder="t('admin.accounts.notesPlaceholder')"
          ></textarea>
          <p class="input-hint">{{ t('admin.accounts.notesHint') }}</p>
        </div>
      </ChannelFormSection>
    </form>

    <template #footer>
      <div class="flex w-full flex-wrap items-center justify-end gap-3">
        <FormError class="mr-auto min-w-0 flex-1" :message="submitError" />
        <button type="button" class="btn btn-secondary" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button
          v-if="account"
          type="submit"
          form="edit-account-form"
          :disabled="submitting"
          class="btn btn-primary"
        >
          <Icon v-if="submitting" name="refresh" size="sm" class="-ml-1 mr-2 animate-spin" />
          {{ submitting ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminAPI } from '@/api/admin'
import type {
  Account,
  Proxy,
  OllamaCloudUsageState,
  ProtocolEndpoints
} from '@/types'
import type { ProtocolDefaultsResponse } from '@/api/admin/accounts'
import BaseDialog from '@/components/common/BaseDialog.vue'
import FormError from '@/components/common/FormError.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import ProxySelector from '@/components/common/ProxySelector.vue'
import UpstreamDetect from '@/components/account/channel/UpstreamDetect.vue'
import type { DetectedModels } from '@/components/account/channel/upstreamModels'
import KeyAddressPresetMenu from '@/components/account/KeyAddressPresetMenu.vue'
import ProtocolEndpointsEditor from '@/components/account/ProtocolEndpointsEditor.vue'
import ChannelFormSection from '@/components/account/channel/ChannelFormSection.vue'
import ChannelLimitsFields from '@/components/account/channel/ChannelLimitsFields.vue'
import ChannelQuotaFields from '@/components/account/channel/ChannelQuotaFields.vue'
import ChannelSettingToggle from '@/components/account/channel/ChannelSettingToggle.vue'
import ChannelAdvancedSection from '@/components/account/channel/ChannelAdvancedSection.vue'
import AnthropicSubscriptionLimits from '@/components/account/channel/AnthropicSubscriptionLimits.vue'
import AnthropicKeySettings from '@/components/account/channel/AnthropicKeySettings.vue'
import HeaderOverrideField from '@/components/account/channel/HeaderOverrideField.vue'
import KeyPlanModePicker from '@/components/account/channel/KeyPlanModePicker.vue'
import ZhipuTeamFields from '@/components/account/channel/ZhipuTeamFields.vue'
import {
  UPSTREAM_PROTOCOLS,
  describeProtocolEndpointsIssue,
  currentProtocolOf,
  endpointsAfterDefaultsChange,
  hasAnthropicEndpoint,
  loadProtocolDefaults,
  protocolDefaultsFor,
  trimProtocolEndpoints,
  validateProtocolEndpoints
} from '@/components/account/protocolEndpoints'
import {
  VENDORS_WITH_CODING_PLAN,
  apiKeyPlaceholderFor,
  detectKeyVendor,
  keyAddressPresets,
  modeOfAddress,
  type KeyAddressPreset
} from '@/components/account/keyAddress'
import OllamaCloudUsageSettings from '@/components/account/OllamaCloudUsageSettings.vue'
import {
  applyAntigravityProjectID,
  applyHeaderOverride,
  applyInterceptWarmup,
  isHeaderOverrideCapable,
  splitHeaderOverridesObject,
  validateHeaderOverrideRows,
  HEADER_OVERRIDES_CREDENTIAL_KEY,
  type CnAccountMode,
  type HeaderOverrideRow
} from '@/components/account/credentialsBuilder'
import { extractApiErrorMessage } from '@/utils/apiError'
import { platformLabel } from '@/utils/platformLabel'
import { VERTEX_LOCATION_OPTIONS } from '@/constants/account'

interface Props {
  show: boolean
  /** 完整账号（含凭据状态）；列表按 id 拉取期间为 null */
  account: Account | null
  /** 拉取完整账号失败时的提示；有值时弹窗里只显示它 */
  loadError?: string
  proxies: Proxy[]
}

const props = defineProps<Props>()
const emit = defineEmits<{
  close: []
  updated: [account: Account]
}>()

const { t } = useI18n()

// Spark 影子账号(parent_account_id 非空):代理恒继承母账号,不可独立编辑(外审 B/P1),
// 故隐藏代理选择器。
const isSparkShadow = computed(() => props.account?.parent_account_id != null)

// 标题带上渠道名（名称在「上游」分区的最后）
const dialogTitle = computed(() =>
  props.account ? `${t('admin.accounts.editAccount')} · ${props.account.name}` : t('admin.accounts.editAccount')
)

/** 检测到的、目录里已上架、这个渠道还没承接的模型数 */
function unboundCount(classified: DetectedModels): number {
  const id = props.account?.id
  return classified.listed.filter((match) => !(match.entry.bindings ?? []).some((binding) => binding.account_id === id)).length
}

const handleOllamaCloudUsageUpdated = (state: OllamaCloudUsageState) => {
  if (props.account) emit('updated', { ...props.account, ollama_cloud_usage: state })
}


// State
const submitting = ref(false)
// 保存失败 / 校验不过的提示，显示在底部按钮左边；重新打开或换账号时清空
const submitError = ref('')
const editApiKey = ref('')

// 地址分不出套餐时管理员选的计费方式（见下方 keyAccountMode）
const editAccountMode = ref<CnAccountMode>('payg')
// 智谱团队版 Coding Plan：组织/项目 ID，写入 credentials 供额度探测切换团队端点
const editZhipuOrganization = ref('')
const editZhipuProject = ref('')
// 回填窗口标志：syncFormFromAccount 会同步改写 editAccountMode 等字段，
// 而 watcher（pre-flush）在同步代码执行完之后才触发——若不抑制，会把刚恢复的
// 存储版协议地址（可能是中转地址）覆盖为官方地址并在下次保存时持久化。
// nextTick 后解除，此后管理员主动切换模式仍正常联动。
const syncingForm = ref(false)

// ── 第三方 key 协议地址 ──
// 初始值取账号已存的 protocol_endpoints。切换账号模式时只替换没改过的地址；
// 任何时候都可以点「填入官方地址」显式恢复。提交时不补任何默认地址。
const protocolDefaults = ref<ProtocolDefaultsResponse | null>(null)
const protocolDefaultsLoadFailed = ref(false)
const editProtocolEndpoints = ref<ProtocolEndpoints>({})

// ── 计费方式（credentials.account_mode）：与新建同一规则（2026-09-28 P5）──
// 厂商与套餐都按地址识别（官方域名表由后端下发，与后端 Account.Vendor 同口径）；地址分不出套餐
// （MiniMax 按量与套餐同地址、智谱 Anthropic 同地址）才让管理员选；OpenCode 的 Zen / Go 只看地址；中转不写。
const keyPresets = computed(() => keyAddressPresets(protocolDefaults.value))
const keyVendor = computed(() =>
  props.account?.type === 'apikey'
    ? detectKeyVendor(editProtocolEndpoints.value, protocolDefaults.value?.vendor_hosts)
    : null
)
const hasKeyAddress = computed(() => Object.values(editProtocolEndpoints.value).some((url) => !!url?.trim()))
// API Key 占位跟着按地址识别出的厂商走，与新建同一规则（不看平台标签）
const apiKeyValuePlaceholder = computed(() => apiKeyPlaceholderFor(keyVendor.value))
const keyHasCodingPlan = computed(() => !!keyVendor.value && VENDORS_WITH_CODING_PLAN.has(keyVendor.value))
const keyPlanFromAddress = computed(() => {
  const vendor = keyVendor.value
  if (!vendor || !keyHasCodingPlan.value) return null
  const mode = modeOfAddress(keyPresets.value, vendor, editProtocolEndpoints.value)
  return mode === 'payg' || mode === 'coding' ? mode : null
})
const keyPlanNeedsChoice = computed(() => keyHasCodingPlan.value && keyPlanFromAddress.value === null)
// 地址下方的识别提示，与新建同一写法：地址定得了套餐就带上套餐
const keyVendorLabel = computed(() => {
  const vendor = keyVendor.value
  if (!vendor) return ''
  const plan = keyPlanFromAddress.value
  return plan ? `${platformLabel(vendor)} · ${t(`admin.accounts.cnProviders.accountMode.${plan}`)}` : platformLabel(vendor)
})
const keyAccountMode = computed<string | undefined>(() => {
  const vendor = keyVendor.value
  if (!vendor) return undefined
  if (vendor === 'opencode_go') return modeOfAddress(keyPresets.value, vendor, editProtocolEndpoints.value) ?? 'zen'
  if (vendor === 'deepseek') return 'payg'
  return keyHasCodingPlan.value ? (keyPlanFromAddress.value ?? editAccountMode.value) : undefined
})

// 按地址识别出的厂商与套餐取官方地址，与新建同一规则：平台标签只在新建时按地址推导一次，
// 编辑改了地址标签不跟着变，不能拿它判断。认不出厂商（中转）就不给「填入官方地址」。
const officialProtocolEndpoints = computed<ProtocolEndpoints>(() => {
  const vendor = keyVendor.value
  if (!vendor) return {}
  const mode = vendor === 'opencode_go' || keyHasCodingPlan.value ? keyAccountMode.value : undefined
  return protocolDefaultsFor(protocolDefaults.value, vendor, mode)
})
watch(officialProtocolEndpoints, (next, previous) => {
  if (syncingForm.value) return
  editProtocolEndpoints.value = endpointsAfterDefaultsChange(
    editProtocolEndpoints.value,
    previous ?? {},
    next,
    // 编辑时平台不变，换模式保留当前协议；还没配协议时取 Chat Completions（国产厂商与 OpenCode 的默认协议）
    currentProtocolOf(editProtocolEndpoints.value) ?? 'chat_completions'
  )
})
async function ensureProtocolDefaults() {
  try {
    protocolDefaults.value = await loadProtocolDefaults()
    protocolDefaultsLoadFailed.value = false
  } catch {
    protocolDefaultsLoadFailed.value = true
  }
}
watch(
  () => props.show,
  (show) => {
    if (show) void ensureProtocolDefaults()
  },
  { immediate: true }
)
// 提交前校验协议地址，有问题显示在底部并返回 null。
function validatedProtocolEndpoints(): ProtocolEndpoints | null {
  const issue = validateProtocolEndpoints(editProtocolEndpoints.value)
  if (issue) {
    submitError.value = describeProtocolEndpointsIssue(issue, t)
    return null
  }
  return trimProtocolEndpoints(editProtocolEndpoints.value)
}
// 从常用官方地址里选一条：协议与地址一起换掉，预设带套餐的连套餐一起换（与新建同一规则）
function applyKeyAddressPreset(preset: KeyAddressPreset) {
  editProtocolEndpoints.value = { [preset.protocol]: preset.url }
  if (preset.mode === 'payg' || preset.mode === 'coding') editAccountMode.value = preset.mode
}
// Bedrock credentials
const editBedrockAccessKeyId = ref('')
const editBedrockSecretAccessKey = ref('')
const editBedrockRegion = ref('')
const editBedrockForceGlobal = ref(false)
const editBedrockApiKeyValue = ref('')
const editVertexLocation = ref('us-central1')
const isBedrockAPIKeyMode = computed(() =>
  props.account?.type === 'bedrock' &&
  (props.account?.credentials as Record<string, unknown>)?.auth_mode === 'apikey'
)

// 池模式同渠道重试次数与状态码写死在后端（channel_features.go），这里只有开关
const poolModeEnabled = ref(false)
const headerOverrideRows = ref<HeaderOverrideRow[]>([])

const headerOverrideCapable = computed(
  () => !!props.account && isHeaderOverrideCapable(props.account.platform, props.account.type)
)

const interceptWarmupRequests = ref(false)
const autoResetCreditEnabled = ref(false)
const allowOverages = ref(false) // For antigravity accounts: enable AI Credits overages
const antigravityProjectId = ref('')


// Quota control state (Anthropic OAuth/SetupToken only)
// 空闲超时、RPM 策略 / 粘性缓冲、用户消息限速、TLS 指纹、会话 ID 伪装、缓存 TTL 替换已写死在后端
// （channel_features_anthropic.go），表单不再提供；库里的旧值不回填、不改写。
const sessionLimitEnabled = ref(false)
const maxSessions = ref<number | null>(null)
const rpmLimitEnabled = ref(false)
const baseRpm = ref<number | null>(null)

type AnthropicAPIKeyAuthScheme = 'x_api_key' | 'authorization_bearer'
const anthropicAPIKeyAuthScheme = ref<AnthropicAPIKeyAuthScheme>('x_api_key')
const bedrockCCCompatEnabled = ref(false)
// Anthropic 协议上的 key 设置按编辑中的协议地址展示，不看平台标签。
const anthropicKeySettingsVisible = computed(
  () => props.account?.type === 'apikey' && hasAnthropicEndpoint(editProtocolEndpoints.value)
)
const interceptWarmupCapable = computed(
  () => props.account?.platform === 'anthropic' || props.account?.platform === 'antigravity'
)
const zhipuTeamCapable = computed(() => keyVendor.value === 'zhipu' && keyAccountMode.value === 'coding')
const antigravityProjectIdCapable = computed(
  () => props.account?.platform === 'antigravity' && props.account?.type === 'oauth'
)
// 「高级」收起时标题下列出里面有哪几项；条件与模板里各项的 v-if 一一对应（改一处两处一起改）
const advancedItems = computed(() => {
  const account = props.account
  if (!account) return []
  const items: string[] = []
  if (headerOverrideCapable.value) items.push(t('admin.accounts.headerOverride.title'))
  if (account.type === 'apikey') items.push(t('admin.accounts.poolMode'))
  if (anthropicKeySettingsVisible.value) {
    items.push(t('admin.accounts.anthropic.apiKeyAuthScheme'), t('admin.accounts.anthropic.bedrockCCCompat'))
  }
  if (interceptWarmupCapable.value) items.push(t('admin.accounts.interceptWarmupRequests'))
  if (zhipuTeamCapable.value) items.push(t('admin.accounts.cnProviders.zhipuTeam.title'))
  if (antigravityProjectIdCapable.value) items.push(t('admin.accounts.antigravityProjectIdLabel'))
  return items
})

const editQuotaLimit = ref<number | null>(null)
const editQuotaDailyLimit = ref<number | null>(null)
const editQuotaWeeklyLimit = ref<number | null>(null)

const form = reactive({
  name: '',
  notes: '',
  proxy_id: null as number | null,
  concurrency: 1,
  priority: 1,
  status: 'active' as 'active' | 'inactive' | 'error',
  expires_at: null as number | null
})

const statusOptions = computed(() => {
  const options = [
    { value: 'active', label: t('common.active') },
    { value: 'inactive', label: t('common.inactive') }
  ]
  if (form.status === 'error') {
    options.push({ value: 'error', label: t('admin.accounts.status.error') })
  }
  return options
})

// Watchers
const syncFormFromAccount = (newAccount: Account | null) => {
  if (!newAccount) {
    return
  }
  submitError.value = ''
  // 进入回填窗口：抑制模式 watcher 与官方地址联动（见 syncingForm 注释）。
  syncingForm.value = true
  void nextTick(() => {
    syncingForm.value = false
  })
  editProtocolEndpoints.value = { ...(newAccount.protocol_endpoints ?? {}) }
  form.name = newAccount.name
  form.notes = newAccount.notes || ''
  form.proxy_id = newAccount.proxy_id
  form.concurrency = newAccount.concurrency
  form.priority = newAccount.priority
  form.status = (newAccount.status === 'active' || newAccount.status === 'inactive' || newAccount.status === 'error')
    ? newAccount.status
    : 'active'
  form.expires_at = newAccount.expires_at ?? null

  // Load intercept warmup requests setting (applies to all account types)
  const credentials = newAccount.credentials as Record<string, unknown> | undefined
  interceptWarmupRequests.value = credentials?.intercept_warmup_requests === true
  editVertexLocation.value = 'us-central1'
  antigravityProjectId.value =
    newAccount.platform === 'antigravity' &&
    newAccount.type === 'oauth' &&
    typeof credentials?.antigravity_project_id === 'string'
      ? credentials.antigravity_project_id.trim()
      : ''

  allowOverages.value = false
	const extra = newAccount.extra as Record<string, unknown> | undefined
	allowOverages.value = extra?.allow_overages === true
	autoResetCreditEnabled.value = extra?.auto_reset_credit_enabled === true

  anthropicAPIKeyAuthScheme.value = 'x_api_key'
  bedrockCCCompatEnabled.value = false
  // 第三方 key 一律回填：地址行可在弹窗里增删，区块是否展示随地址变化
  if (newAccount.type === 'apikey') {
    anthropicAPIKeyAuthScheme.value = extra?.anthropic_apikey_auth_scheme === 'authorization_bearer'
      ? 'authorization_bearer'
      : 'x_api_key'
    bedrockCCCompatEnabled.value = extra?.bedrock_cc_compat === true
  }

  // Load quota limit for apikey/bedrock accounts (bedrock quota is also loaded in its own branch above)
  if (newAccount.type === 'apikey' || newAccount.type === 'bedrock') {
    const quotaVal = extra?.quota_limit as number | undefined
    editQuotaLimit.value = (quotaVal && quotaVal > 0) ? quotaVal : null
    const dailyVal = extra?.quota_daily_limit as number | undefined
    editQuotaDailyLimit.value = (dailyVal && dailyVal > 0) ? dailyVal : null
    const weeklyVal = extra?.quota_weekly_limit as number | undefined
    editQuotaWeeklyLimit.value = (weeklyVal && weeklyVal > 0) ? weeklyVal : null
  } else {
    editQuotaLimit.value = null
    editQuotaDailyLimit.value = null
    editQuotaWeeklyLimit.value = null
  }


  // Load quota control settings (Anthropic OAuth/SetupToken only)
  loadQuotaControlSettings(newAccount)

  // Load header override state for eligible account platforms/types
  headerOverrideRows.value = []
  if (newAccount.credentials && isHeaderOverrideCapable(newAccount.platform, newAccount.type)) {
    const overrideCreds = newAccount.credentials as Record<string, unknown>
    headerOverrideRows.value = splitHeaderOverridesObject(
      overrideCreds[HEADER_OVERRIDES_CREDENTIAL_KEY]
    )
  }

  // Initialize API Key fields for apikey type
  if (newAccount.type === 'apikey' && newAccount.credentials) {
    const credentials = newAccount.credentials as Record<string, unknown>
    // 计费方式：地址分不出套餐时回填已存的选择（分得出就跟地址走，见 keyAccountMode）
    editAccountMode.value = credentials.account_mode === 'coding' ? 'coding' : 'payg'
    // 智谱团队版 Coding Plan：回填组织/项目 ID（厂商按地址识别，是不是智谱看 keyVendor）
    editZhipuOrganization.value = typeof credentials.zhipu_organization === 'string' ? credentials.zhipu_organization : ''
    editZhipuProject.value = typeof credentials.zhipu_project === 'string' ? credentials.zhipu_project : ''

    // Load pool mode（同渠道重试次数与状态码写死在后端，这里只有开关）
    poolModeEnabled.value = credentials.pool_mode === true
  } else if (newAccount.type === 'bedrock' && newAccount.credentials) {
    const bedrockCreds = newAccount.credentials as Record<string, unknown>
    const authMode = (bedrockCreds.auth_mode as string) || 'sigv4'
    editBedrockRegion.value = (bedrockCreds.aws_region as string) || ''
    editBedrockForceGlobal.value = (bedrockCreds.aws_force_global as string) === 'true'

    if (authMode === 'apikey') {
      editBedrockApiKeyValue.value = ''
    } else {
      editBedrockAccessKeyId.value = (bedrockCreds.aws_access_key_id as string) || ''
      editBedrockSecretAccessKey.value = ''
    }

    // Load quota limits for bedrock
    const bedrockExtra = (newAccount.extra as Record<string, unknown>) || {}
    editQuotaLimit.value = typeof bedrockExtra.quota_limit === 'number' ? bedrockExtra.quota_limit : null
    editQuotaDailyLimit.value = typeof bedrockExtra.quota_daily_limit === 'number' ? bedrockExtra.quota_daily_limit : null
    editQuotaWeeklyLimit.value = typeof bedrockExtra.quota_weekly_limit === 'number' ? bedrockExtra.quota_weekly_limit : null
  } else if ((newAccount.platform === 'gemini' || newAccount.platform === 'anthropic') && newAccount.type === 'service_account' && newAccount.credentials) {
    const credentials = newAccount.credentials as Record<string, unknown>
    editVertexLocation.value = (credentials.location as string) || (credentials.vertex_location as string) || 'us-central1'
  } else {
    poolModeEnabled.value = false
  }
  editApiKey.value = ''
}

watch(
  [() => props.show, () => props.account],
  ([show, newAccount], [wasShow, previousAccount]) => {
    if (!show || !newAccount) {
      return
    }
    if (!wasShow || newAccount !== previousAccount) {
      syncFormFromAccount(newAccount)
    }
  },
  { immediate: true }
)

// Load quota control settings from account (Anthropic OAuth/SetupToken only)
function loadQuotaControlSettings(account: Account) {
  // Reset all quota control state first
  sessionLimitEnabled.value = false
  maxSessions.value = null
  rpmLimitEnabled.value = false
  baseRpm.value = null

  // Remaining quota control settings only apply to Anthropic accounts
  if (account.platform !== 'anthropic') {
    return
  }

  // Session / RPM limit only apply to Anthropic OAuth/SetupToken accounts
  if (account.type !== 'oauth' && account.type !== 'setup-token') {
    return
  }

  // Load from extra field (via backend DTO fields)
  if (account.max_sessions != null && account.max_sessions > 0) {
    sessionLimitEnabled.value = true
    maxSessions.value = account.max_sessions
  }

  // RPM limit
  if (account.base_rpm != null && account.base_rpm > 0) {
    rpmLimitEnabled.value = true
    baseRpm.value = account.base_rpm
  }
}

// Methods
const handleClose = () => {
  emit('close')
}

const submitUpdateAccount = async (accountID: number, updatePayload: Record<string, unknown>) => {
  submitting.value = true
  try {
    const updatedAccount = await adminAPI.accounts.update(accountID, updatePayload)
    emit('updated', updatedAccount)
    handleClose()
  } catch (error) {
    submitError.value = extractApiErrorMessage(error, t('admin.accounts.failedToUpdate'))
  } finally {
    submitting.value = false
  }
}

const handleSubmit = async () => {
  if (!props.account) return
  const accountID = props.account.id
  submitError.value = ''

  if (form.status !== 'active' && form.status !== 'inactive' && form.status !== 'error') {
    submitError.value = t('admin.accounts.pleaseSelectStatus')
    return
  }

  const updatePayload: Record<string, unknown> = { ...form }
  try {
    // 后端期望 proxy_id: 0 表示清除代理，而不是 null
    if (updatePayload.proxy_id === null) {
      updatePayload.proxy_id = 0
    }
    if (form.expires_at === null) {
      updatePayload.expires_at = 0
    }

    // For apikey type, handle credentials update
    if (props.account.type === 'apikey') {
      const apiKeyEndpoints = validatedProtocolEndpoints()
      if (!apiKeyEndpoints) {
        return
      }
      updatePayload.protocol_endpoints = apiKeyEndpoints
      const currentCredentials = (props.account.credentials as Record<string, unknown>) || {}
      const newCredentials: Record<string, unknown> = { ...currentCredentials }
      // 计费方式与新建同一规则：按地址识别出厂商才写 account_mode（决定额度/余额探测），地址指向中转就去掉。
      // 官方域名表没拉到时认不出厂商，保留已存的值，免得一次保存把套餐清掉。
      if (protocolDefaults.value) {
        if (keyAccountMode.value) newCredentials.account_mode = keyAccountMode.value
        else delete newCredentials.account_mode
      }
      // 智谱团队版 Coding Plan：组织/项目 ID 写入凭据（非空才写，清空即移除回落个人版路径）
      if (keyVendor.value === 'zhipu') {
        const org = editZhipuOrganization.value.trim()
        const project = editZhipuProject.value.trim()
        if (org) {
          newCredentials.zhipu_organization = org
          if (project) newCredentials.zhipu_project = project
          else delete newCredentials.zhipu_project
        } else {
          delete newCredentials.zhipu_organization
          delete newCredentials.zhipu_project
        }
      }

      // Handle API key
      // 后端响应已脱敏：currentCredentials 不会再包含 api_key 原文。
      // 用户填入新值则覆盖；留空时优先看 credentials_status.has_api_key；
      // 若后端尚未升级（无 credentials_status），回退读旧结构 currentCredentials.api_key。
      // 两者都无才报错。
      const hasExistingApiKey =
        props.account.credentials_status?.has_api_key ?? Boolean(currentCredentials.api_key)
      if (editApiKey.value.trim()) {
        newCredentials.api_key = editApiKey.value.trim()
      } else if (!hasExistingApiKey) {
        submitError.value = t('admin.accounts.apiKeyIsRequired')
        return
      }

      // 池模式：同渠道重试次数与状态码写死在后端（channel_features.go），这里只写开关
      if (poolModeEnabled.value) {
        newCredentials.pool_mode = true
      } else {
        delete newCredentials.pool_mode
      }

      // 请求头覆写对任何第三方 key 开放，有条目就生效
      const headerError = validateHeaderOverrideRows(headerOverrideRows.value)
      if (headerError) {
        submitError.value = t(`admin.accounts.headerOverride.${headerError}`)
        return
      }
      applyHeaderOverride(newCredentials, headerOverrideRows.value, 'edit')

      // Add intercept warmup requests setting
      applyInterceptWarmup(newCredentials, interceptWarmupRequests.value, 'edit')

      updatePayload.credentials = newCredentials
    } else if ((props.account.platform === 'gemini' || props.account.platform === 'anthropic') && props.account.type === 'service_account') {
      const currentCredentials = (props.account.credentials as Record<string, unknown>) || {}
      const newCredentials: Record<string, unknown> = { ...currentCredentials }

      if (!editVertexLocation.value.trim()) {
        submitError.value = t('admin.accounts.vertexLocationRequired')
        return
      }

      // SA JSON 已脱敏不再随 credentials 返回，存在性优先读 credentials_status。
      // 若后端尚未升级（无 credentials_status），回退读旧结构 service_account_json / service_account。
      const credentialsStatus = props.account.credentials_status
      const hasExistingServiceAccountJson = credentialsStatus
        ? Boolean(
            credentialsStatus.has_service_account_json || credentialsStatus.has_service_account
          )
        : Boolean(currentCredentials.service_account_json || currentCredentials.service_account)
      if (!hasExistingServiceAccountJson) {
        submitError.value = t('admin.accounts.vertexSaJsonRequired')
        return
      }
      newCredentials.location = editVertexLocation.value.trim()
      newCredentials.tier_id = 'vertex'

      applyInterceptWarmup(newCredentials, interceptWarmupRequests.value, 'edit')

      updatePayload.credentials = newCredentials
    } else if (props.account.type === 'bedrock') {
      const currentCredentials = (props.account.credentials as Record<string, unknown>) || {}
      const newCredentials: Record<string, unknown> = { ...currentCredentials }

      newCredentials.aws_region = editBedrockRegion.value.trim()
      if (editBedrockForceGlobal.value) {
        newCredentials.aws_force_global = 'true'
      } else {
        delete newCredentials.aws_force_global
      }

      if (isBedrockAPIKeyMode.value) {
        // API Key mode: only update api_key if user provided new value
        if (editBedrockApiKeyValue.value.trim()) {
          newCredentials.api_key = editBedrockApiKeyValue.value.trim()
        }
      } else {
        // SigV4 mode
        newCredentials.aws_access_key_id = editBedrockAccessKeyId.value.trim()
        if (editBedrockSecretAccessKey.value.trim()) {
          newCredentials.aws_secret_access_key = editBedrockSecretAccessKey.value.trim()
        }
      }

      applyInterceptWarmup(newCredentials, interceptWarmupRequests.value, 'edit')

      updatePayload.credentials = newCredentials
    } else {
      // For oauth/setup-token types, only update intercept_warmup_requests if changed
      const currentCredentials = (props.account.credentials as Record<string, unknown>) || {}
      const newCredentials: Record<string, unknown> = { ...currentCredentials }

      applyInterceptWarmup(newCredentials, interceptWarmupRequests.value, 'edit')

      updatePayload.credentials = newCredentials
    }

    // Grok OAuth: 请求头覆写。成品号只走官方地址，没有可配置的上游地址。
    if (props.account.platform === 'grok' && props.account.type === 'oauth') {
      const currentCredentials =
        (updatePayload.credentials as Record<string, unknown>) ||
        ((props.account.credentials as Record<string, unknown>) || {})
      const newCredentials: Record<string, unknown> = { ...currentCredentials }

      const headerError = validateHeaderOverrideRows(headerOverrideRows.value)
      if (headerError) {
        submitError.value = t(`admin.accounts.headerOverride.${headerError}`)
        return
      }
      applyHeaderOverride(newCredentials, headerOverrideRows.value, 'edit')

      updatePayload.credentials = newCredentials
    }

    // Antigravity 成品号：兜底 project ID
    if (props.account.platform === 'antigravity' && props.account.type === 'oauth') {
      const currentCredentials = (updatePayload.credentials as Record<string, unknown>) ||
        ((props.account.credentials as Record<string, unknown>) || {})
      const newCredentials: Record<string, unknown> = { ...currentCredentials }
      applyAntigravityProjectID(newCredentials, antigravityProjectId.value, 'edit')
      updatePayload.credentials = newCredentials
    }

    // 渠道上的模型改名已删（2026-10-01，改名在价格页每条承接关系的上游模型名里设）：后端拒收这两个键，
    // 上面从账号已存凭据复制来的旧值不能带回去。
    // spark 影子号的 model_mapping 是系统维护的模型列表，表单不改它：影子号只要带了 credentials，
    // 后端就按它整份替换，所以干脆不带。
    if (isSparkShadow.value) {
      delete updatePayload.credentials
    } else {
      const credentials = updatePayload.credentials as Record<string, unknown>
      delete credentials.model_mapping
      delete credentials.model_mapping_rename_only
    }

    // 超量只属于 Antigravity 成品号；第三方 key 不写这个键
    if (props.account.platform === 'antigravity' && props.account.type === 'oauth') {
      const currentExtra = (props.account.extra as Record<string, unknown>) || {}
      const newExtra: Record<string, unknown> = { ...currentExtra }
      if (allowOverages.value) {
        newExtra.allow_overages = true
      } else {
        delete newExtra.allow_overages
      }
      updatePayload.extra = newExtra
    }

    // For Anthropic OAuth/SetupToken accounts, handle quota control settings in extra
    if (props.account.platform === 'anthropic' && (props.account.type === 'oauth' || props.account.type === 'setup-token')) {
      const currentExtra = (updatePayload.extra as Record<string, unknown>) || (props.account.extra as Record<string, unknown>) || {}
      const newExtra: Record<string, unknown> = { ...currentExtra }

      // Session limit settings
      if (sessionLimitEnabled.value && maxSessions.value != null && maxSessions.value > 0) {
        newExtra.max_sessions = maxSessions.value
      } else {
        delete newExtra.max_sessions
      }

      // RPM limit settings
      if (rpmLimitEnabled.value) {
        const DEFAULT_BASE_RPM = 15
        newExtra.base_rpm = (baseRpm.value != null && baseRpm.value > 0)
          ? baseRpm.value
          : DEFAULT_BASE_RPM
      } else {
        delete newExtra.base_rpm
      }

      updatePayload.extra = newExtra
    }

    // 第三方 key 的 Anthropic 协议设置（认证方式 / Bedrock CC 兼容）写入 extra。
    // 区块隐藏（没有 anthropic 地址）时不写界面上的值，账号已存的值原样保留，与其他隐藏区块一致。
    if (anthropicKeySettingsVisible.value) {
      const currentExtra = (updatePayload.extra as Record<string, unknown>) || (props.account.extra as Record<string, unknown>) || {}
      const newExtra: Record<string, unknown> = { ...currentExtra }
      if (anthropicAPIKeyAuthScheme.value === 'authorization_bearer') {
        newExtra.anthropic_apikey_auth_scheme = 'authorization_bearer'
      } else {
        delete newExtra.anthropic_apikey_auth_scheme
      }
      if (bedrockCCCompatEnabled.value) {
        newExtra.bedrock_cc_compat = true
      } else {
        delete newExtra.bedrock_cc_compat
      }
      updatePayload.extra = newExtra
    }

    // 第三方 key 专属、按协议地址判定的 extra：不看平台标签。
    if (props.account.type === 'apikey') {
      const currentExtra = (updatePayload.extra as Record<string, unknown>) || (props.account.extra as Record<string, unknown>) || {}
      const newExtra: Record<string, unknown> = { ...currentExtra }
      // Responses 路由改由协议地址决定，已退役的探测标记与强制模式保存时清掉。
      delete newExtra.openai_responses_mode
      delete newExtra.openai_responses_supported
      updatePayload.extra = newExtra
    }

    // OpenAI 成品号：自动使用重置卡开关（阈值写死 100%，见后端 channel_features_openai.go）
    if (props.account.platform === 'openai' && (props.account.type === 'oauth' || props.account.type === 'setup-token' || props.account.type === 'apikey')) {
      const currentExtra = (updatePayload.extra as Record<string, unknown>) || (props.account.extra as Record<string, unknown>) || {}
      const newExtra: Record<string, unknown> = { ...currentExtra }
      if (props.account.type === 'oauth' && !isSparkShadow.value) {
        newExtra.auto_reset_credit_enabled = autoResetCreditEnabled.value
      }
      // 运行态只允许后端服务更新，账号编辑不得回写旧状态。
      delete newExtra.codex_auto_reset_credit_state
      updatePayload.extra = newExtra
    }

    // For apikey/bedrock accounts, handle quota_limit in extra
    if (props.account.type === 'apikey' || props.account.type === 'bedrock') {
      const currentExtra = (updatePayload.extra as Record<string, unknown>) ||
        (props.account.extra as Record<string, unknown>) || {}
      const newExtra: Record<string, unknown> = { ...currentExtra }
      // Total quota
      if (editQuotaLimit.value != null && editQuotaLimit.value > 0) {
        newExtra.quota_limit = editQuotaLimit.value
      } else {
        delete newExtra.quota_limit
      }
      // Daily quota
      if (editQuotaDailyLimit.value != null && editQuotaDailyLimit.value > 0) {
        newExtra.quota_daily_limit = editQuotaDailyLimit.value
      } else {
        delete newExtra.quota_daily_limit
        delete newExtra.quota_daily_used
        delete newExtra.quota_daily_start
      }
      // Weekly quota
      if (editQuotaWeeklyLimit.value != null && editQuotaWeeklyLimit.value > 0) {
        newExtra.quota_weekly_limit = editQuotaWeeklyLimit.value
      } else {
        delete newExtra.quota_weekly_limit
        delete newExtra.quota_weekly_used
        delete newExtra.quota_weekly_start
      }
      updatePayload.extra = newExtra
    }

    await submitUpdateAccount(accountID, updatePayload)
  } catch (error) {
    submitError.value = extractApiErrorMessage(error, t('admin.accounts.failedToUpdate'))
  }
}
</script>
