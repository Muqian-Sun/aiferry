<template>
  <!-- 开关 + 阈值 + 通知邮箱；标题与说明在 ProfileView 的设置行左栏 -->
  <div>
    <div class="space-y-6">
      <!-- Enable toggle -->
      <div class="flex items-center justify-between">
        <label class="input-label mb-0">{{ t('profile.balanceNotify.enabled') }}</label>
        <label class="relative inline-flex items-center cursor-pointer">
          <input type="checkbox" v-model="notifyEnabled" :disabled="togglingEnabled" @change="handleToggle" class="sr-only peer" />
          <div class="w-11 h-6 bg-af-hairline peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-af-brand/30 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-af-sheet after:border-af-hairline-strong after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-af-brand"></div>
        </label>
      </div>
      <FormError :message="toggleError" />

      <template v-if="notifyEnabled">
        <!-- Custom threshold with save button -->
        <div>
          <label class="input-label">
            {{ t('profile.balanceNotify.threshold') }}
            <span class="text-xs text-af-ink-3 ml-2">{{ t('profile.balanceNotify.thresholdHint') }}</span>
          </label>
          <div class="flex items-center gap-2">
            <span class="text-af-ink-3">$</span>
            <input
              v-model.number="customThreshold"
              type="number"
              min="0"
              step="0.01"
              class="input flex-1"
              :placeholder="systemDefaultThreshold > 0 ? `${t('profile.balanceNotify.systemDefault')} $${systemDefaultThreshold}` : t('profile.balanceNotify.thresholdPlaceholder')"
            />
            <button
              @click="handleThresholdUpdate"
              :disabled="savingThreshold"
              class="btn btn-primary btn-sm whitespace-nowrap"
            >
              {{ savingThreshold ? t('common.saving') : t('common.save') }}
            </button>
          </div>
          <FormError class="mt-2" :message="thresholdError" />
          <FormSuccess class="mt-2" :message="thresholdSaved.message.value" />
        </div>

        <!-- Email list with toggles -->
        <div>
          <label class="input-label">{{ t('profile.balanceNotify.extraEmails') }}</label>
          <p class="mb-2 text-xs text-af-ink-3">{{ t('profile.balanceNotify.extraEmailsHint') }}</p>

          <!-- Saved email entries -->
          <div v-if="emailEntries.length > 0" class="space-y-2 mb-3">
            <div v-for="(entry, idx) in emailEntries" :key="idx"
              class="flex items-center justify-between px-3 py-2 bg-af-sunken rounded-lg">
              <div class="flex items-center gap-2 min-w-0 flex-1">
                <label class="relative inline-flex items-center cursor-pointer shrink-0">
                  <input type="checkbox" :checked="!entry.disabled" @change="handleEmailToggle(entry, $event)" class="sr-only peer" />
                  <div class="w-9 h-5 bg-af-hairline peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-af-sheet after:border-af-hairline-strong after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-af-brand"></div>
                </label>
                <span class="text-sm text-af-ink-2 truncate">{{ entry.email }}</span>
              </div>
              <div class="flex items-center gap-2 shrink-0">
                <template v-if="!entry.verified">
                  <!-- Inline verify flow for saved unverified emails -->
                  <template v-if="verifyingEmail === entry.email">
                    <input
                      v-model="verifyCode"
                      type="text"
                      maxlength="6"
                      class="w-20 rounded border border-af-hairline-strong px-2 py-1 text-xs"
                      :placeholder="t('profile.balanceNotify.codePlaceholder')"
                    />
                    <button @click="verifySavedEmail(entry.email)" :disabled="!verifyCode || verifyCode.length !== 6 || verifyingSaved" class="text-xs text-af-brand hover:text-af-brand">
                      {{ t('profile.balanceNotify.verify') }}
                    </button>
                    <span v-if="verifyCountdown > 0" class="text-xs text-af-ink-3">{{ verifyCountdown }}s</span>
                    <button v-else @click="sendCodeForSaved(entry.email)" :disabled="sendingSavedCode" class="text-xs text-af-ink-3 hover:text-af-ink-2">
                      {{ t('profile.balanceNotify.resend') }}
                    </button>
                    <button @click="verifyingEmail = ''" class="text-xs text-af-ink-3 hover:text-af-ink-2">
                      {{ t('common.cancel') }}
                    </button>
                  </template>
                  <template v-else>
                    <button @click="sendCodeForSaved(entry.email)" :disabled="sendingSavedCode" class="text-xs text-af-brand hover:text-af-brand">
                      {{ t('profile.balanceNotify.verify') }}
                    </button>
                    <span class="text-xs text-af-warning">{{ t('profile.balanceNotify.unverified') }}</span>
                  </template>
                </template>
                <span v-else class="text-xs text-af-success">{{ t('profile.balanceNotify.verified') }}</span>
                <button @click="handleRemoveEmail(entry.email)" class="text-af-danger hover:text-af-danger text-xs">
                  {{ t('profile.balanceNotify.removeEmail') }}
                </button>
              </div>
            </div>
          </div>

          <!-- Pending (unverified) emails in verification flow -->
          <div v-if="pendingEmails.length > 0" class="space-y-2 mb-3">
            <div v-for="(pe, idx) in pendingEmails" :key="pe.email"
              data-testid="pending-email-row"
              class="flex items-center gap-2 rounded-md border border-af-warning/40 bg-af-warning-tint px-3 py-2">
              <span class="flex-1 text-sm text-af-ink-2">{{ pe.email }}</span>
              <div v-if="!pe.codeSent" class="flex items-center gap-1">
                <button @click="sendCodeFor(idx)" :disabled="pe.sending" class="text-xs text-af-brand hover:text-af-brand">
                  {{ t('profile.balanceNotify.sendCode') }}
                </button>
                <button @click="pendingEmails.splice(idx, 1)" class="text-xs text-af-danger hover:text-af-danger ml-1">
                  {{ t('profile.balanceNotify.removeEmail') }}
                </button>
              </div>
              <div v-else class="flex items-center gap-1">
                <input
                  v-model="pe.code"
                  type="text"
                  maxlength="6"
                  class="w-20 rounded border border-af-hairline-strong px-2 py-1 text-xs"
                  :placeholder="t('profile.balanceNotify.codePlaceholder')"
                />
                <button @click="verifyPending(idx)" :disabled="!pe.code || pe.code.length !== 6 || pe.verifying" class="text-xs text-af-brand hover:text-af-brand">
                  {{ t('profile.balanceNotify.verify') }}
                </button>
                <span v-if="pe.countdown > 0" class="text-xs text-af-ink-3">{{ pe.countdown }}s</span>
                <button v-else @click="sendCodeFor(idx)" :disabled="pe.sending" class="text-xs text-af-ink-3 hover:text-af-ink-2">
                  {{ t('profile.balanceNotify.resend') }}
                </button>
              </div>
            </div>
          </div>

          <!-- Add new email input (hidden when at limit) -->
          <div v-if="canAddMore" class="flex gap-2">
            <input
              v-model="newEmail"
              type="email"
              class="input flex-1"
              :placeholder="t('profile.balanceNotify.emailPlaceholder')"
              @keyup.enter="addPendingEmail"
            />
            <button
              @click="addPendingEmail"
              :disabled="!newEmail"
              class="btn btn-secondary whitespace-nowrap"
            >
              {{ t('common.add') }}
            </button>
          </div>
          <p v-else class="text-xs text-af-ink-3">
            {{ t('profile.balanceNotify.maxEmailsReached') }}
          </p>
          <FormError class="mt-2" :message="emailError" />
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { userAPI } from '@/api'
import { extractI18nErrorMessage } from '@/utils/apiError'
import FormError from '@/components/common/FormError.vue'
import FormSuccess from '@/components/common/FormSuccess.vue'
import { useTransientMessage } from '@/composables/useTransientMessage'
import type { NotifyEmailEntry } from '@/types'

const maxTotalEmails = 3

interface PendingEmail {
  email: string
  codeSent: boolean
  code: string
  sending: boolean
  verifying: boolean
  countdown: number
  timer: ReturnType<typeof setInterval> | null
}

const props = defineProps<{
  enabled: boolean
  threshold: number | null
  extraEmails: NotifyEmailEntry[]
  systemDefaultThreshold: number
  userEmail: string
}>()

const { t } = useI18n()
const authStore = useAuthStore()

const notifyEnabled = ref(props.enabled)
const customThreshold = ref<number | null>(props.threshold)
const emailEntries = ref<NotifyEmailEntry[]>([...props.extraEmails])
const pendingEmails = ref<PendingEmail[]>([])
const newEmail = ref('')
const savingThreshold = ref(false)
const togglingEnabled = ref(false)
const toggleError = ref('')
const thresholdError = ref('')
const thresholdSaved = useTransientMessage()
// 邮箱区（加 / 发码 / 验证 / 开关 / 移除）共用一条报错，放在邮箱区末尾
const emailError = ref('')

// 验证码类错误（INVALID_VERIFY_CODE 等）的中文在 auth.errors；其余用调用处给的兜底
function failure(err: unknown, fallback: string): string {
  return extractI18nErrorMessage(err, t, 'auth.errors', fallback)
}

// State for verifying saved unverified emails
const verifyingEmail = ref('')
const verifyCode = ref('')
const verifyingSaved = ref(false)
const sendingSavedCode = ref(false)
const verifyCountdown = ref(0)
let verifyTimer: ReturnType<typeof setInterval> | null = null

const canAddMore = computed(() => {
  return emailEntries.value.length + pendingEmails.value.length < maxTotalEmails
})

watch(() => props.enabled, (val) => { notifyEnabled.value = val })
watch(() => props.threshold, (val) => { customThreshold.value = val })
watch(() => props.extraEmails, (val) => { emailEntries.value = [...val] })

// When list is empty on mount, pre-fill the add input with user's email
onMounted(() => {
  if (emailEntries.value.length === 0 && props.userEmail) {
    newEmail.value = props.userEmail
  }
})

onUnmounted(() => {
  for (const pe of pendingEmails.value) {
    if (pe.timer) clearInterval(pe.timer)
  }
  if (verifyTimer) clearInterval(verifyTimer)
})

const handleToggle = async () => {
  toggleError.value = ''
  togglingEnabled.value = true
  try {
    const updated = await userAPI.updateProfile({ balance_notify_enabled: notifyEnabled.value })
    authStore.user = updated
  } catch (err: unknown) {
    toggleError.value = failure(err, t('profile.balanceNotify.saveFailed'))
    notifyEnabled.value = !notifyEnabled.value
  } finally {
    togglingEnabled.value = false
  }
}

const handleThresholdUpdate = async () => {
  savingThreshold.value = true
  thresholdError.value = ''
  thresholdSaved.clear()
  try {
    const threshold = customThreshold.value && customThreshold.value > 0 ? customThreshold.value : 0
    const updated = await userAPI.updateProfile({ balance_notify_threshold: threshold })
    authStore.user = updated
    thresholdSaved.show(t('common.saved'))
  } catch (err: unknown) {
    thresholdError.value = failure(err, t('profile.balanceNotify.saveFailed'))
  } finally {
    savingThreshold.value = false
  }
}

async function handleEmailToggle(entry: NotifyEmailEntry, event: Event) {
  const newDisabled = !entry.disabled
  emailError.value = ''
  try {
    const updated = await userAPI.toggleNotifyEmail(entry.email, newDisabled)
    authStore.user = updated
    emailEntries.value = [...updated.balance_notify_extra_emails]
  } catch (err: unknown) {
    emailError.value = failure(err, t('profile.balanceNotify.saveFailed'))
    // 开关只单向绑定 :checked，数据没变 Vue 不会重画：失败时手动拨回原状态
    ;(event.target as HTMLInputElement).checked = !entry.disabled
  }
}

function addPendingEmail() {
  const email = newEmail.value.trim()
  if (!email) return
  // Check duplicates
  const isDuplicate = emailEntries.value.some(e => e.email.toLowerCase() === email.toLowerCase())
    || pendingEmails.value.some(p => p.email.toLowerCase() === email.toLowerCase())
  emailError.value = ''
  if (isDuplicate) {
    emailError.value = t('profile.balanceNotify.emailDuplicate')
    return
  }
  pendingEmails.value.push({ email, codeSent: false, code: '', sending: false, verifying: false, countdown: 0, timer: null })
  newEmail.value = ''
}

async function sendCodeFor(idx: number) {
  const pe = pendingEmails.value[idx]
  if (!pe) return
  pe.sending = true
  emailError.value = ''
  try {
    await userAPI.sendNotifyEmailCode(pe.email)
    pe.codeSent = true
    pe.countdown = 60
    pe.timer = setInterval(() => {
      pe.countdown--
      if (pe.countdown <= 0 && pe.timer) {
        clearInterval(pe.timer)
        pe.timer = null
      }
    }, 1000)
  } catch (err: unknown) {
    emailError.value = failure(err, t('profile.balanceNotify.sendCodeFailed'))
  } finally {
    pe.sending = false
  }
}

async function verifyPending(idx: number) {
  const pe = pendingEmails.value[idx]
  if (!pe || !pe.code || pe.code.length !== 6) return
  pe.verifying = true
  emailError.value = ''
  try {
    await userAPI.verifyNotifyEmail(pe.email, pe.code)
    if (pe.timer) clearInterval(pe.timer)
    pendingEmails.value = pendingEmails.value.filter(entry => entry !== pe)
    const updated = await userAPI.getProfile()
    authStore.user = updated
    emailEntries.value = [...updated.balance_notify_extra_emails]
  } catch (err: unknown) {
    emailError.value = failure(err, t('profile.balanceNotify.verifyFailed'))
  } finally {
    pe.verifying = false
  }
}

const handleRemoveEmail = async (email: string) => {
  emailError.value = ''
  try {
    await userAPI.removeNotifyEmail(email)
    const updated = await userAPI.getProfile()
    authStore.user = updated
    emailEntries.value = [...updated.balance_notify_extra_emails]
  } catch (err: unknown) {
    emailError.value = failure(err, t('profile.balanceNotify.removeFailed'))
  }
}

// Verify saved unverified emails
async function sendCodeForSaved(email: string) {
  sendingSavedCode.value = true
  emailError.value = ''
  try {
    await userAPI.sendNotifyEmailCode(email)
    verifyingEmail.value = email
    verifyCode.value = ''
    verifyCountdown.value = 60
    if (verifyTimer) clearInterval(verifyTimer)
    verifyTimer = setInterval(() => {
      verifyCountdown.value--
      if (verifyCountdown.value <= 0 && verifyTimer) {
        clearInterval(verifyTimer)
        verifyTimer = null
      }
    }, 1000)
  } catch (err: unknown) {
    emailError.value = failure(err, t('profile.balanceNotify.sendCodeFailed'))
  } finally {
    sendingSavedCode.value = false
  }
}

async function verifySavedEmail(email: string) {
  if (!verifyCode.value || verifyCode.value.length !== 6) return
  verifyingSaved.value = true
  emailError.value = ''
  try {
    await userAPI.verifyNotifyEmail(email, verifyCode.value)
    verifyingEmail.value = ''
    verifyCode.value = ''
    if (verifyTimer) { clearInterval(verifyTimer); verifyTimer = null }
    const updated = await userAPI.getProfile()
    authStore.user = updated
    emailEntries.value = [...updated.balance_notify_extra_emails]
  } catch (err: unknown) {
    emailError.value = failure(err, t('profile.balanceNotify.verifyFailed'))
  } finally {
    verifyingSaved.value = false
  }
}
</script>
