<template>
  <!-- 编辑代理（从 ProxiesView 拆出，行为不变）：密码留空保持不变，只有改过密码框才提交 password -->
  <BaseDialog
    :show="show"
    :title="t('admin.proxies.editProxy')"
    width="normal"
    @close="close"
  >
    <form
      v-if="proxy"
      id="edit-proxy-form"
      @submit.prevent="handleUpdateProxy"
      class="space-y-5"
    >
      <div>
        <label class="input-label">{{ t('admin.proxies.name') }}</label>
        <input v-model="editForm.name" type="text" required class="input" />
      </div>
      <div>
        <label class="input-label">{{ t('admin.proxies.protocol') }}</label>
        <Select v-model="editForm.protocol" :options="protocolSelectOptions" />
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="input-label">{{ t('admin.proxies.host') }}</label>
          <input v-model="editForm.host" type="text" required class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.proxies.port') }}</label>
          <input
            v-model.number="editForm.port"
            type="number"
            required
            min="1"
            max="65535"
            class="input"
          />
        </div>
      </div>
      <div>
        <label class="input-label">{{ t('admin.proxies.username') }}</label>
        <input v-model="editForm.username" type="text" class="input" />
      </div>
      <div>
        <label class="input-label">{{ t('admin.proxies.password') }}</label>
        <div class="relative">
          <input
            v-model="editForm.password"
            :type="editPasswordVisible ? 'text' : 'password'"
            :placeholder="t('admin.proxies.leaveEmptyToKeep')"
            class="input pr-10"
            @input="editPasswordDirty = true"
          />
          <button
            type="button"
            class="absolute right-3 top-1/2 -translate-y-1/2 text-af-ink-3 hover:text-af-ink-2"
            @click="editPasswordVisible = !editPasswordVisible"
          >
            <Icon :name="editPasswordVisible ? 'eyeOff' : 'eye'" size="md" />
          </button>
        </div>
      </div>
      <div>
        <label class="input-label">{{ t('admin.proxies.status') }}</label>
        <Select v-model="editForm.status" :options="editStatusOptions" />
      </div>
      <div>
        <label class="input-label">{{ t('admin.proxies.expiresAt') }}</label>
        <div class="mb-2 flex flex-wrap gap-2">
          <button
            v-for="d in EXPIRY_PRESETS"
            :key="d"
            type="button"
            class="btn btn-sm"
            :class="editForm.expires_at === addDaysToBase(editBaseDate, d) ? 'btn-primary' : 'btn-secondary'"
            @click="editExpiresDays = d"
          >
            {{ t('admin.proxies.nDays', { days: d }) }}
          </button>
        </div>
        <input
          v-model.number="editExpiresDays"
          type="number"
          min="0"
          class="input mb-2"
          :placeholder="t('admin.proxies.expiryDaysPlaceholder')"
        />
        <input v-model="editForm.expires_at" type="date" max="9999-12-31" class="input" />
      </div>
      <div>
        <label class="input-label">{{ t('admin.proxies.fallbackMode') }}</label>
        <Select v-model="editForm.fallback_mode" :options="fallbackModeOptions" />
      </div>
      <div v-if="editForm.fallback_mode === 'proxy'">
        <label class="input-label">{{ t('admin.proxies.backupProxy') }}</label>
        <Select v-model="editForm.backup_proxy_id" :options="backupProxyOptions" />
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <FormError class="mr-auto self-center" :message="errorMessage" />
        <button @click="close" type="button" class="btn btn-secondary">
          {{ t('common.cancel') }}
        </button>
        <button
          v-if="proxy"
          type="submit"
          form="edit-proxy-form"
          :disabled="submitting"
          class="btn btn-primary"
        >
          <Icon v-if="submitting" name="refresh" size="sm" class="animate-spin" />
          {{ submitting ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { Proxy, ProxyProtocol } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import FormError from '@/components/common/FormError.vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { EXPIRY_PRESETS, addDaysToBase, daysFromBase } from './proxyExpiryDays'

const props = defineProps<{
  show: boolean
  proxy: Proxy | null
  /** 「到期回退 → 指定备用代理」的候选（全部启用中的代理，自己除外） */
  backupProxies: Proxy[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'updated'): void
}>()

const { t } = useI18n()

const protocolSelectOptions = computed(() => [
  { value: 'http', label: t('admin.proxies.protocols.http') },
  { value: 'https', label: t('admin.proxies.protocols.https') },
  { value: 'socks5', label: t('admin.proxies.protocols.socks5') },
  { value: 'socks5h', label: t('admin.proxies.protocols.socks5h') }
])

const editStatusOptions = computed(() => [
  { value: 'active', label: t('admin.accounts.status.active') },
  { value: 'inactive', label: t('admin.accounts.status.inactive') }
])

const fallbackModeOptions = computed(() => [
  { label: t('admin.proxies.fallbackNone'), value: 'none' },
  { label: t('admin.proxies.fallbackProxy'), value: 'proxy' },
  { label: t('admin.proxies.fallbackDirect'), value: 'direct' }
])

const backupProxyOptions = computed(() =>
  props.backupProxies
    .filter((p) => p.id !== props.proxy?.id)
    .map((p) => ({ label: `${p.name} (${p.host}:${p.port})`, value: p.id }))
)

const submitting = ref(false)
const errorMessage = ref('')
const editPasswordVisible = ref(false)
const editPasswordDirty = ref(false)

const editForm = reactive({
  name: '',
  protocol: 'http' as ProxyProtocol,
  host: '',
  port: 8080,
  username: '',
  password: '',
  status: 'active' as 'active' | 'inactive' | 'expired',
  expires_at: '' as string,
  fallback_mode: 'none' as 'none' | 'proxy' | 'direct',
  backup_proxy_id: null as number | null,
  expiry_warn_days: 7 as number,
})

// 打开时把代理填进表单
const fillForm = (proxy: Proxy) => {
  editForm.name = proxy.name
  editForm.protocol = proxy.protocol
  editForm.host = proxy.host
  editForm.port = proxy.port
  editForm.username = proxy.username || ''
  editForm.password = proxy.password || ''
  editForm.status = proxy.status === 'expired' ? 'inactive' : proxy.status
  editForm.expires_at = proxy.expires_at ? proxy.expires_at.slice(0, 10) : ''
  editForm.fallback_mode = proxy.fallback_mode || 'none'
  editForm.backup_proxy_id = proxy.backup_proxy_id ?? null
  editForm.expiry_warn_days = proxy.expiry_warn_days ?? 7
  editPasswordVisible.value = false
  editPasswordDirty.value = false
}

watch(
  () => [props.show, props.proxy] as const,
  ([show, proxy]) => {
    if (show && proxy) fillForm(proxy)
  },
  { immediate: true }
)

// 编辑时有效期自「代理创建日」起算
const editBaseDate = computed(() =>
  props.proxy?.created_at ? props.proxy.created_at.slice(0, 10) : '',
)
const editExpiresDays = computed<number | null>({
  get: () => daysFromBase(editBaseDate.value, editForm.expires_at),
  set: (v) => {
    editForm.expires_at = addDaysToBase(editBaseDate.value, v)
  },
})

const close = () => {
  editPasswordVisible.value = false
  editPasswordDirty.value = false
  errorMessage.value = ''
  emit('close')
}

const handleUpdateProxy = async () => {
  if (!props.proxy) return
  errorMessage.value = ''
  if (!editForm.name.trim()) {
    errorMessage.value = t('admin.proxies.nameRequired')
    return
  }
  if (!editForm.host.trim()) {
    errorMessage.value = t('admin.proxies.hostRequired')
    return
  }
  if (editForm.port < 1 || editForm.port > 65535) {
    errorMessage.value = t('admin.proxies.portInvalid')
    return
  }

  submitting.value = true
  try {
    const updateData: any = {
      name: editForm.name.trim(),
      protocol: editForm.protocol,
      host: editForm.host.trim(),
      port: editForm.port,
      username: editForm.username.trim(),
      status: editForm.status,
      expires_at: editForm.expires_at ? Math.floor(new Date(editForm.expires_at).getTime() / 1000) : null,
      fallback_mode: editForm.fallback_mode,
      backup_proxy_id: editForm.fallback_mode === 'proxy' ? editForm.backup_proxy_id : null,
      expiry_warn_days: editForm.expiry_warn_days,
    }

    // Only include password if user actually modified the field
    if (editPasswordDirty.value) {
      updateData.password = editForm.password.trim()
    }

    await adminAPI.proxies.update(props.proxy.id, updateData)
    close()
    emit('updated')
  } catch (error: any) {
    errorMessage.value = extractApiErrorMessage(error, t('admin.proxies.failedToUpdate'))
  } finally {
    submitting.value = false
  }
}
</script>
