<template>
  <!-- 小标题 + 状态；外层容器由调用方决定。用户站的两栏设置行自己画标题（headless），管理端卡片里保留小标题 -->
  <div class="space-y-4">
    <div v-if="!headless">
      <h3 class="text-sm font-semibold text-af-ink">
        {{ t('profile.totp.title') }}
      </h3>
      <p class="mt-1 text-13 text-af-ink-3">
        {{ t('profile.totp.description') }}
      </p>
    </div>
    <div>
      <!-- Loading state -->
      <div v-if="loading" class="flex items-center justify-center py-8">
        <LoadingSpinner />
      </div>

      <!-- Feature disabled globally -->
      <div v-else-if="status && !status.feature_enabled" class="flex items-center gap-4 py-4">
        <div>
          <p class="font-medium text-af-ink-2">
            {{ t('profile.totp.featureDisabled') }}
          </p>
          <p class="text-sm text-af-ink-3">
            {{ t('profile.totp.featureDisabledHint') }}
          </p>
        </div>
      </div>

      <!-- 2FA Enabled -->
      <div v-else-if="status?.enabled" class="flex items-center justify-between">
        <div class="flex items-center gap-4">
          <div>
            <p class="font-medium text-af-ink">
              {{ t('profile.totp.enabled') }}
            </p>
            <p v-if="status.enabled_at" class="text-sm text-af-ink-3">
              {{ t('profile.totp.enabledAt') }}: {{ formatDate(status.enabled_at) }}
            </p>
          </div>
        </div>
        <button
          type="button"
          class="btn btn-ghost btn-sm text-af-danger hover:text-af-danger"
          @click="showDisableDialog = true"
        >
          {{ t('profile.totp.disable') }}
        </button>
      </div>

      <!-- 2FA Not Enabled -->
      <div v-else class="flex items-center justify-between">
        <div class="flex items-center gap-4">
          <div>
            <p class="font-medium text-af-ink-2">
              {{ t('profile.totp.notEnabled') }}
            </p>
            <p class="text-sm text-af-ink-3">
              {{ t('profile.totp.notEnabledHint') }}
            </p>
          </div>
        </div>
        <button
          type="button"
          class="btn btn-primary"
          @click="showSetupModal = true"
        >
          {{ t('profile.totp.enable') }}
        </button>
      </div>
    </div>

    <!-- Setup Modal -->
    <TotpSetupModal
      v-if="showSetupModal"
      @close="showSetupModal = false"
      @success="handleSetupSuccess"
    />

    <!-- Disable Dialog -->
    <TotpDisableDialog
      v-if="showDisableDialog"
      @close="showDisableDialog = false"
      @success="handleDisableSuccess"
    />
  </div>
</template>

<script setup lang="ts">
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { totpAPI } from '@/api'
import type { TotpStatus } from '@/types'
import TotpSetupModal from './TotpSetupModal.vue'
import TotpDisableDialog from './TotpDisableDialog.vue'

/** 用户站两栏设置行里不画自带小标题 */
defineProps<{ headless?: boolean }>()

const { t } = useI18n()

const loading = ref(true)
const status = ref<TotpStatus | null>(null)
const showSetupModal = ref(false)
const showDisableDialog = ref(false)

const loadStatus = async () => {
  loading.value = true
  try {
    status.value = await totpAPI.getStatus()
  } catch (error) {
    console.error('Failed to load TOTP status:', error)
  } finally {
    loading.value = false
  }
}

const handleSetupSuccess = () => {
  showSetupModal.value = false
  loadStatus()
}

const handleDisableSuccess = () => {
  showDisableDialog.value = false
  loadStatus()
}

const formatDate = (timestamp: number) => {
  // Backend returns Unix timestamp in seconds, convert to milliseconds
  const date = new Date(timestamp * 1000)
  return date.toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

onMounted(() => {
  loadStatus()
})
</script>
