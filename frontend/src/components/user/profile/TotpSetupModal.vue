<template>
  <div class="fixed inset-0 z-50 overflow-y-auto" @click.self="$emit('close')">
    <div class="flex min-h-full items-center justify-center p-4">
      <div class="fixed inset-0 bg-black/50 transition-opacity" @click="$emit('close')"></div>

      <div class="relative w-full max-w-md transform rounded-md bg-af-sheet p-6 shadow-xl transition-all">
        <!-- Header -->
        <div class="mb-6 text-center">
          <h3 class="text-xl font-semibold text-af-ink">
            {{ t('profile.totp.setupTitle') }}
          </h3>
          <p class="mt-2 text-sm text-af-ink-3">
            {{ stepDescription }}
          </p>
        </div>

        <!-- Step 0: Identity Verification -->
        <div v-if="step === 0" class="space-y-6">
          <!-- Loading verification method -->
          <div v-if="methodLoading" class="flex items-center justify-center py-8">
            <LoadingSpinner />
          </div>

          <template v-else>
            <!-- Email verification -->
            <div v-if="verificationMethod === 'email'" class="space-y-4">
              <div>
                <label class="input-label">{{ t('profile.totp.emailCode') }}</label>
                <div class="flex gap-2">
                  <input
                    v-model="verifyForm.emailCode"
                    type="text"
                    maxlength="6"
                    inputmode="numeric"
                    class="input flex-1"
                    :placeholder="t('profile.totp.enterEmailCode')"
                  />
                  <button
                    type="button"
                    class="btn btn-secondary whitespace-nowrap"
                    :disabled="sendingCode || codeCooldown > 0"
                    @click="handleSendCode"
                  >
                    {{ codeCooldown > 0 ? `${codeCooldown}s` : (sendingCode ? t('common.sending') : t('profile.totp.sendCode')) }}
                  </button>
                </div>
              </div>
            </div>

            <!-- Password verification -->
            <div v-else class="space-y-4">
              <div>
                <label class="input-label">{{ t('profile.currentPassword') }}</label>
                <input
                  v-model="verifyForm.password"
                  type="password"
                  autocomplete="current-password"
                  class="input"
                  :placeholder="t('profile.totp.enterPassword')"
                />
              </div>
            </div>

            <FormError :message="errorMessage" />

            <div class="flex justify-end gap-3 pt-4">
              <button type="button" class="btn btn-secondary" @click="$emit('close')">
                {{ t('common.cancel') }}
              </button>
              <button
                type="button"
                class="btn btn-primary"
                :disabled="!canProceedFromVerify || setupLoading"
                @click="handleVerifyAndSetup"
              >
                {{ setupLoading ? t('common.loading') : t('common.next') }}
              </button>
            </div>
          </template>
        </div>

        <!-- Step 1: Show QR Code -->
        <div v-if="step === 1" class="space-y-6">
          <!-- QR Code and Secret -->
          <template v-if="setupData">
            <div class="flex justify-center">
              <div class="rounded-lg border border-af-hairline p-4 bg-af-sheet">
                <img :src="qrCodeDataUrl" alt="QR Code" class="h-48 w-48" />
              </div>
            </div>

            <div class="text-center">
              <p class="text-sm text-af-ink-3 mb-2">
                {{ t('profile.totp.manualEntry') }}
              </p>
              <div class="flex items-center justify-center gap-2">
                <code class="rounded bg-af-sunken px-3 py-2 font-mono text-sm">
                  {{ setupData.secret }}
                </code>
                <CopyButton :text="setupData.secret" class="rounded p-1.5 text-af-ink-3 hover:bg-af-sunken" />
              </div>
            </div>
          </template>

          <FormError :message="errorMessage" />

          <div class="flex justify-end gap-3 pt-4">
            <button type="button" class="btn btn-secondary" @click="$emit('close')">
              {{ t('common.cancel') }}
            </button>
            <button
              type="button"
              class="btn btn-primary"
              :disabled="!setupData"
              @click="step = 2"
            >
              {{ t('common.next') }}
            </button>
          </div>
        </div>

        <!-- Step 2: Verify Code -->
        <div v-if="step === 2" class="space-y-6">
          <form @submit.prevent="handleVerify">
            <div class="mb-6">
              <label class="input-label text-center block mb-3">
                {{ t('profile.totp.enterCode') }}
              </label>
              <div class="flex justify-center gap-2">
                <input
                  v-for="(_, index) in 6"
                  :key="index"
                  :ref="(el) => setInputRef(el, index)"
                  type="text"
                  maxlength="1"
                  inputmode="numeric"
                  pattern="[0-9]"
                  class="h-12 w-10 rounded-lg border border-af-hairline-strong text-center text-lg font-semibold focus:border-af-brand focus:ring-af-brand"
                  @input="handleCodeInput($event, index)"
                  @keydown="handleKeydown($event, index)"
                  @paste="handlePaste"
                />
              </div>
              <FormError class="justify-center" :message="errorMessage" />
            </div>

            <div class="flex justify-end gap-3">
              <button type="button" class="btn btn-secondary" @click="step = 1">
                {{ t('common.back') }}
              </button>
              <button
                type="submit"
                class="btn btn-primary"
                :disabled="verifying || code.join('').length !== 6"
              >
                {{ verifying ? t('common.verifying') : t('profile.totp.verify') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import CopyButton from '@/components/common/CopyButton.vue'
import { ref, onMounted, onUnmounted, nextTick, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { totpAPI } from '@/api'
import FormError from '@/components/common/FormError.vue'
import { extractApiErrorMessage, extractI18nErrorMessage } from '@/utils/apiError'
import type { TotpSetupResponse } from '@/types'
import QRCode from 'qrcode'

const emit = defineEmits<{
  close: []
  success: []
}>()

const { t } = useI18n()

// Step: 0 = verify identity, 1 = QR code, 2 = verify TOTP code
const step = ref(0)
const methodLoading = ref(true)
const verificationMethod = ref<'email' | 'password'>('password')
const verifyForm = ref({ emailCode: '', password: '' })
const sendingCode = ref(false)
const codeCooldown = ref(0)
const cooldownTimer = ref<ReturnType<typeof setInterval> | null>(null)

const setupLoading = ref(false)
const setupData = ref<TotpSetupResponse | null>(null)
const verifying = ref(false)
const code = ref<string[]>(['', '', '', '', '', ''])
const inputRefs = ref<(HTMLInputElement | null)[]>([])
const qrCodeDataUrl = ref('')
const errorMessage = ref('')

// 重新输入、切换步骤时清掉报错；验证失败后清空验证码不算重新输入
watch(verifyForm, () => {
  errorMessage.value = ''
}, { deep: true })
watch(step, () => {
  errorMessage.value = ''
})
watch(
  () => code.value.join(''),
  (value) => {
    if (value) errorMessage.value = ''
  }
)

const stepDescription = computed(() => {
  switch (step.value) {
    case 0:
      return verificationMethod.value === 'email'
        ? t('profile.totp.verifyEmailFirst')
        : t('profile.totp.verifyPasswordFirst')
    case 1:
      return t('profile.totp.setupStep1')
    case 2:
      return t('profile.totp.setupStep2')
    default:
      return ''
  }
})

const canProceedFromVerify = computed(() => {
  if (verificationMethod.value === 'email') {
    return verifyForm.value.emailCode.length === 6
  }
  return verifyForm.value.password.length > 0
})

// Generate QR code as base64 when setupData changes
watch(
  () => setupData.value?.qr_code_url,
  async (url) => {
    if (url) {
      try {
        qrCodeDataUrl.value = await QRCode.toDataURL(url, {
          width: 200,
          margin: 2,
          color: {
            dark: '#000000',
            light: '#ffffff'
          }
        })
      } catch (err) {
        console.error('Failed to generate QR code:', err)
      }
    }
  },
  { immediate: true }
)

const setInputRef = (el: any, index: number) => {
  inputRefs.value[index] = el as HTMLInputElement | null
}

const handleCodeInput = (event: Event, index: number) => {
  const input = event.target as HTMLInputElement
  const value = input.value.replace(/[^0-9]/g, '')
  code.value[index] = value

  if (value && index < 5) {
    nextTick(() => {
      inputRefs.value[index + 1]?.focus()
    })
  }
}

const handleKeydown = (event: KeyboardEvent, index: number) => {
  if (event.key === 'Backspace') {
    const input = event.target as HTMLInputElement
    // If current cell is empty and not the first, move to previous cell
    if (!input.value && index > 0) {
      event.preventDefault()
      inputRefs.value[index - 1]?.focus()
    }
    // Otherwise, let the browser handle the backspace naturally
    // The input event will sync code.value via handleCodeInput
  }
}

const handlePaste = (event: ClipboardEvent) => {
  event.preventDefault()
  const pastedData = event.clipboardData?.getData('text') || ''
  const digits = pastedData.replace(/[^0-9]/g, '').slice(0, 6).split('')

  // Update both the ref and the input elements
  digits.forEach((digit, index) => {
    code.value[index] = digit
    if (inputRefs.value[index]) {
      inputRefs.value[index]!.value = digit
    }
  })

  // Clear remaining inputs if pasted less than 6 digits
  for (let i = digits.length; i < 6; i++) {
    code.value[i] = ''
    if (inputRefs.value[i]) {
      inputRefs.value[i]!.value = ''
    }
  }

  const focusIndex = Math.min(digits.length, 5)
  nextTick(() => {
    inputRefs.value[focusIndex]?.focus()
  })
}

const loadVerificationMethod = async () => {
  methodLoading.value = true
  try {
    const method = await totpAPI.getVerificationMethod()
    verificationMethod.value = method.method
  } catch (err: any) {
    console.error(extractApiErrorMessage(err, t('common.error')), err)
    emit('close')
  } finally {
    methodLoading.value = false
  }
}

const handleSendCode = async () => {
  errorMessage.value = ''
  sendingCode.value = true
  try {
    await totpAPI.sendVerifyCode()
    // Start cooldown
    codeCooldown.value = 60
    if (cooldownTimer.value) {
      clearInterval(cooldownTimer.value)
      cooldownTimer.value = null
    }
    cooldownTimer.value = setInterval(() => {
      codeCooldown.value--
      if (codeCooldown.value <= 0) {
        if (cooldownTimer.value) {
          clearInterval(cooldownTimer.value)
          cooldownTimer.value = null
        }
      }
    }, 1000)
  } catch (err: any) {
    errorMessage.value = extractI18nErrorMessage(err, t, 'auth.errors', t('profile.totp.sendCodeFailed'))
    console.error(errorMessage.value, err)
  } finally {
    sendingCode.value = false
  }
}

const handleVerifyAndSetup = async () => {
  setupLoading.value = true

  try {
    const request = verificationMethod.value === 'email'
      ? { email_code: verifyForm.value.emailCode }
      : { password: verifyForm.value.password }

    setupData.value = await totpAPI.initiateSetup(request)
    step.value = 1
  } catch (err: any) {
    errorMessage.value = extractI18nErrorMessage(err, t, 'auth.errors', t('profile.totp.setupFailed'))
    console.error(errorMessage.value, err)
  } finally {
    setupLoading.value = false
  }
}

const handleVerify = async () => {
  const totpCode = code.value.join('')
  if (totpCode.length !== 6 || !setupData.value) return

  verifying.value = true

  try {
    await totpAPI.enable({
      totp_code: totpCode,
      setup_token: setupData.value.setup_token
    })
    emit('success')
  } catch (err: any) {
    errorMessage.value = extractI18nErrorMessage(err, t, 'auth.errors', t('profile.totp.verifyFailed'))
    console.error(errorMessage.value, err)
    code.value = ['', '', '', '', '', '']
    nextTick(() => {
      inputRefs.value[0]?.focus()
    })
  } finally {
    verifying.value = false
  }
}

onMounted(() => {
  loadVerificationMethod()
})

onUnmounted(() => {
  if (cooldownTimer.value) {
    clearInterval(cooldownTimer.value)
    cooldownTimer.value = null
  }
})
</script>
