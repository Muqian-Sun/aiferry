<template>
  <!-- 添加代理（从 ProxiesView 拆出，行为不变）：标准添加 / 快捷添加（每行一个代理 URL，批量导入）两个页签 -->
  <BaseDialog
    :show="show"
    :title="t('admin.proxies.createProxy')"
    width="normal"
    @close="close"
  >
    <!-- Tab Switch -->
    <div class="mb-6 border-b border-af-hairline">
      <div class="flex min-w-0 shrink-0">
        <button
          type="button"
          @click="createMode = 'standard'"
          :class="[
            '-mb-px border-b-2 px-4 py-2 text-sm font-medium transition-colors',
            createMode === 'standard'
              ? 'border-af-brand text-af-brand'
              : 'border-transparent text-af-ink-3 hover:text-af-ink-2'
          ]"
        >
          <Icon name="plus" size="sm" class="mr-1.5 inline" />
          {{ t('admin.proxies.standardAdd') }}
        </button>
        <button
          type="button"
          @click="createMode = 'batch'"
          :class="[
            '-mb-px border-b-2 px-4 py-2 text-sm font-medium transition-colors',
            createMode === 'batch'
              ? 'border-af-brand text-af-brand'
              : 'border-transparent text-af-ink-3 hover:text-af-ink-2'
          ]"
        >
          <svg
            class="mr-1.5 inline h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            stroke-width="1.5"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M3.75 12h16.5m-16.5 3.75h16.5M3.75 19.5h16.5M5.625 4.5h12.75a1.875 1.875 0 010 3.75H5.625a1.875 1.875 0 010-3.75z"
            />
          </svg>
          {{ t('admin.proxies.batchAdd') }}
        </button>
      </div>
    </div>

    <!-- Standard Add Form -->
    <form
      v-if="createMode === 'standard'"
      id="create-proxy-form"
      @submit.prevent="handleCreateProxy"
      class="space-y-5"
    >
      <div>
        <label class="input-label">{{ t('admin.proxies.name') }}</label>
        <input
          v-model="createForm.name"
          type="text"
          required
          class="input"
          :placeholder="t('admin.proxies.enterProxyName')"
        />
      </div>
      <div>
        <label class="input-label">{{ t('admin.proxies.protocol') }}</label>
        <Select v-model="createForm.protocol" :options="protocolSelectOptions" />
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="input-label">{{ t('admin.proxies.host') }}</label>
          <input
            v-model="createForm.host"
            type="text"
            required
            :placeholder="t('admin.proxies.form.hostPlaceholder')"
            class="input"
          />
        </div>
        <div>
          <label class="input-label">{{ t('admin.proxies.port') }}</label>
          <input
            v-model.number="createForm.port"
            type="number"
            required
            min="1"
            max="65535"
            :placeholder="t('admin.proxies.form.portPlaceholder')"
            class="input"
          />
        </div>
      </div>
      <div>
        <label class="input-label">{{ t('admin.proxies.username') }}</label>
        <input
          v-model="createForm.username"
          type="text"
          class="input"
          :placeholder="t('admin.proxies.optionalAuth')"
        />
      </div>
      <div>
        <label class="input-label">{{ t('admin.proxies.password') }}</label>
        <div class="relative">
          <input
            v-model="createForm.password"
            :type="createPasswordVisible ? 'text' : 'password'"
            class="input pr-10"
            :placeholder="t('admin.proxies.optionalAuth')"
          />
          <button
            type="button"
            class="absolute right-3 top-1/2 -translate-y-1/2 text-af-ink-3 hover:text-af-ink-2"
            @click="createPasswordVisible = !createPasswordVisible"
          >
            <Icon :name="createPasswordVisible ? 'eyeOff' : 'eye'" size="md" />
          </button>
        </div>
      </div>
      <div>
        <label class="input-label">{{ t('admin.proxies.expiresAt') }}</label>
        <div class="mb-2 flex flex-wrap gap-2">
          <button
            v-for="d in EXPIRY_PRESETS"
            :key="d"
            type="button"
            class="btn btn-sm"
            :class="createForm.expires_at === addDaysToBase('', d) ? 'btn-primary' : 'btn-secondary'"
            @click="createExpiresDays = d"
          >
            {{ t('admin.proxies.nDays', { days: d }) }}
          </button>
        </div>
        <input
          v-model.number="createExpiresDays"
          type="number"
          min="0"
          class="input mb-2"
          :placeholder="t('admin.proxies.expiryDaysPlaceholder')"
        />
        <input v-model="createForm.expires_at" type="date" max="9999-12-31" class="input" />
      </div>
      <div>
        <label class="input-label">{{ t('admin.proxies.fallbackMode') }}</label>
        <Select v-model="createForm.fallback_mode" :options="fallbackModeOptions" />
      </div>
      <div v-if="createForm.fallback_mode === 'proxy'">
        <label class="input-label">{{ t('admin.proxies.backupProxy') }}</label>
        <Select v-model="createForm.backup_proxy_id" :options="backupProxyOptions" />
      </div>
    </form>

    <!-- Batch Add Form -->
    <div v-else class="space-y-5">
      <div>
        <label class="input-label">{{ t('admin.proxies.batchInput') }}</label>
        <textarea
          v-model="batchInput"
          rows="10"
          class="input font-mono text-sm"
          :placeholder="t('admin.proxies.batchInputPlaceholder')"
          @input="parseBatchInput"
        ></textarea>
        <p class="input-hint mt-2">
          {{ t('admin.proxies.batchInputHint') }}
        </p>
      </div>

      <!-- Parse Result -->
      <div v-if="batchParseResult.total > 0" class="rounded-lg bg-af-sunken p-4">
        <div class="flex items-center gap-4 text-sm">
          <div class="flex items-center gap-1.5">
            <Icon name="checkCircle" size="sm" :stroke-width="2" class="text-af-brand" />
            <span class="text-af-ink-2">
              {{ t('admin.proxies.parsedCount', { count: batchParseResult.valid }) }}
            </span>
          </div>
          <div v-if="batchParseResult.invalid > 0" class="flex items-center gap-1.5">
            <Icon
              name="exclamationCircle"
              size="sm"
              :stroke-width="2"
              class="text-af-warning"
            />
            <span class="text-af-warning">
              {{ t('admin.proxies.invalidCount', { count: batchParseResult.invalid }) }}
            </span>
          </div>
          <div v-if="batchParseResult.duplicate > 0" class="flex items-center gap-1.5">
            <Icon name="copy" size="sm" :stroke-width="2" class="text-af-ink-3" />
            <span class="text-af-ink-3">
              {{ t('admin.proxies.duplicateCount', { count: batchParseResult.duplicate }) }}
            </span>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button @click="close" type="button" class="btn btn-secondary">
          {{ t('common.cancel') }}
        </button>
        <button
          v-if="createMode === 'standard'"
          type="submit"
          form="create-proxy-form"
          :disabled="submitting"
          class="btn btn-primary"
        >
          <Icon v-if="submitting" name="refresh" size="sm" class="animate-spin" />
          {{ submitting ? t('admin.proxies.creating') : t('common.create') }}
        </button>
        <button
          v-else
          @click="handleBatchCreate"
          type="button"
          :disabled="submitting || batchParseResult.valid === 0"
          class="btn btn-primary"
        >
          <Icon v-if="submitting" name="refresh" size="sm" class="animate-spin" />
          {{
            submitting
              ? t('admin.proxies.importing')
              : t('admin.proxies.importProxies', { count: batchParseResult.valid })
          }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { Proxy, ProxyProtocol } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { EXPIRY_PRESETS, addDaysToBase, daysFromBase } from './proxyExpiryDays'

const props = defineProps<{
  show: boolean
  /** 「到期回退 → 指定备用代理」的候选（全部启用中的代理） */
  backupProxies: Proxy[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'created'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()

const protocolSelectOptions = computed(() => [
  { value: 'http', label: t('admin.proxies.protocols.http') },
  { value: 'https', label: t('admin.proxies.protocols.https') },
  { value: 'socks5', label: t('admin.proxies.protocols.socks5') },
  { value: 'socks5h', label: t('admin.proxies.protocols.socks5h') }
])

const fallbackModeOptions = computed(() => [
  { label: t('admin.proxies.fallbackNone'), value: 'none' },
  { label: t('admin.proxies.fallbackProxy'), value: 'proxy' },
  { label: t('admin.proxies.fallbackDirect'), value: 'direct' }
])

const backupProxyOptions = computed(() =>
  props.backupProxies.map((p) => ({ label: `${p.name} (${p.host}:${p.port})`, value: p.id }))
)

const submitting = ref(false)
const createPasswordVisible = ref(false)

// Batch import state
const createMode = ref<'standard' | 'batch'>('standard')
const batchInput = ref('')
const batchParseResult = reactive({
  total: 0,
  valid: 0,
  invalid: 0,
  duplicate: 0,
  proxies: [] as Array<{
    protocol: ProxyProtocol
    host: string
    port: number
    username: string
    password: string
  }>
})

const createForm = reactive({
  name: '',
  protocol: 'http' as ProxyProtocol,
  host: '',
  port: 8080,
  username: '',
  password: '',
  expires_at: '' as string,
  fallback_mode: 'none' as 'none' | 'proxy' | 'direct',
  backup_proxy_id: null as number | null,
  expiry_warn_days: 7 as number,
})

// 创建时无 created_at → base='' 用今天
const createExpiresDays = computed<number | null>({
  get: () => daysFromBase('', createForm.expires_at),
  set: (v) => {
    createForm.expires_at = addDaysToBase('', v)
  },
})

const close = () => {
  emit('close')
  createMode.value = 'standard'
  createForm.name = ''
  createForm.protocol = 'http'
  createForm.host = ''
  createForm.port = 8080
  createForm.username = ''
  createForm.password = ''
  createForm.expires_at = ''
  createForm.fallback_mode = 'none'
  createForm.backup_proxy_id = null
  createForm.expiry_warn_days = 7
  createPasswordVisible.value = false
  batchInput.value = ''
  batchParseResult.total = 0
  batchParseResult.valid = 0
  batchParseResult.invalid = 0
  batchParseResult.duplicate = 0
  batchParseResult.proxies = []
}

// Parse proxy URL: protocol://user:pass@host:port or protocol://host:port
// Host may be a domain, IPv4, or bracketed IPv6 ([2001:db8::1]).
const parseProxyUrl = (
  line: string
): {
  protocol: ProxyProtocol
  host: string
  port: number
  username: string
  password: string
} | null => {
  const trimmed = line.trim()
  if (!trimmed) return null

  // Regex to parse proxy URL (supports http, https, socks5, socks5h).
  // Host alternatives: [bracketed-IPv6] | hostname/IPv4 (colon-free, so the
  // match stops before the final :port).
  const regex =
    /^(https?|socks5h?):\/\/(?:([^:@\[\]]+):([^@\[\]]+)@)?(\[[0-9a-f:.]+\]|[^:\[\]]+):(\d+)$/i
  const match = trimmed.match(regex)

  if (!match) return null

  const [, protocol, username, password, rawHost, port] = match
  const portNum = parseInt(port, 10)

  if (portNum < 1 || portNum > 65535) return null

  // Strip brackets from IPv6 literals; the backend re-brackets via net.JoinHostPort.
  const host = rawHost.replace(/^\[|\]$/g, '').trim()

  return {
    protocol: protocol.toLowerCase() as ProxyProtocol,
    host,
    port: portNum,
    username: username?.trim() || '',
    password: password?.trim() || ''
  }
}

const parseBatchInput = () => {
  const lines = batchInput.value.split('\n').filter((l) => l.trim())
  const seen = new Set<string>()
  const proxies: typeof batchParseResult.proxies = []
  let invalid = 0
  let duplicate = 0

  for (const line of lines) {
    const parsed = parseProxyUrl(line)
    if (!parsed) {
      invalid++
      continue
    }

    // Check for duplicates (same host:port:username:password)
    const key = `${parsed.host}:${parsed.port}:${parsed.username}:${parsed.password}`
    if (seen.has(key)) {
      duplicate++
      continue
    }
    seen.add(key)
    proxies.push(parsed)
  }

  batchParseResult.total = lines.length
  batchParseResult.valid = proxies.length
  batchParseResult.invalid = invalid
  batchParseResult.duplicate = duplicate
  batchParseResult.proxies = proxies
}

const handleBatchCreate = async () => {
  if (batchParseResult.valid === 0) return

  submitting.value = true
  try {
    const result = await adminAPI.proxies.batchCreate(batchParseResult.proxies)
    const created = result.created || 0
    const skipped = result.skipped || 0

    if (created > 0) {
      appStore.showSuccess(t('admin.proxies.batchImportSuccess', { created, skipped }))
    } else {
      appStore.showInfo(t('admin.proxies.batchImportAllSkipped', { skipped }))
    }

    close()
    emit('created')
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.failedToImport'))
    console.error('Error batch creating proxies:', error)
  } finally {
    submitting.value = false
  }
}

const handleCreateProxy = async () => {
  if (!createForm.name.trim()) {
    appStore.showError(t('admin.proxies.nameRequired'))
    return
  }
  if (!createForm.host.trim()) {
    appStore.showError(t('admin.proxies.hostRequired'))
    return
  }
  if (createForm.port < 1 || createForm.port > 65535) {
    appStore.showError(t('admin.proxies.portInvalid'))
    return
  }
  submitting.value = true
  try {
    await adminAPI.proxies.create({
      name: createForm.name.trim(),
      protocol: createForm.protocol,
      host: createForm.host.trim(),
      port: createForm.port,
      username: createForm.username.trim() || null,
      password: createForm.password.trim() || null,
      expires_at: createForm.expires_at ? Math.floor(new Date(createForm.expires_at).getTime() / 1000) : null,
      fallback_mode: createForm.fallback_mode,
      backup_proxy_id: createForm.fallback_mode === 'proxy' ? createForm.backup_proxy_id : null,
      expiry_warn_days: createForm.expiry_warn_days,
    })
    appStore.showSuccess(t('admin.proxies.proxyCreated'))
    close()
    emit('created')
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.failedToCreate'))
    console.error('Error creating proxy:', error)
  } finally {
    submitting.value = false
  }
}
</script>
