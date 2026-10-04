<template>
  <!-- 头像 + 上传操作；标题在 ProfileInfoCard 的设置行左栏 -->
  <div>
    <div class="flex items-start gap-4">
      <div
        class="flex h-16 w-16 shrink-0 items-center justify-center overflow-hidden rounded-full bg-af-sunken text-xl font-semibold text-af-ink-2"
      >
        <img
          v-if="avatarPreviewUrl"
          data-testid="profile-avatar-preview"
          :src="avatarPreviewUrl"
          :alt="displayName"
          class="h-full w-full object-cover"
        >
        <span v-else>{{ avatarInitial }}</span>
      </div>

      <div class="min-w-0 flex-1 space-y-3">
        <p class="text-13 text-af-ink-3">
          {{ t('profile.avatar.uploadHint') }}
        </p>

        <div class="flex flex-wrap items-center gap-5">
          <label class="hero-link text-13 font-medium cursor-pointer">
            <input
              data-testid="profile-avatar-file-input"
              type="file"
              accept="image/*"
              class="hidden"
              @change="handleAvatarFileChange"
            >
            {{ t('profile.avatar.uploadAction') }}
          </label>

          <!-- 选了新图才出现「保存」（没有草稿时一个灰掉的按钮只是噪音） -->
          <button
            v-if="avatarDraft"
            data-testid="profile-avatar-save"
            type="button"
            class="btn btn-primary btn-sm"
            :disabled="avatarSaving || !avatarDraft"
            @click="handleAvatarSave"
          >
            {{ t('common.save') }}
          </button>

          <button
            data-testid="profile-avatar-delete"
            type="button"
            class="text-13 font-medium text-af-ink-3 transition-colors hover:text-af-danger disabled:opacity-40"
            :disabled="avatarSaving"
            @click="handleAvatarDelete"
          >
            {{ t('common.delete') }}
          </button>
        </div>
        <FormError :message="avatarError" />
        <FormSuccess :message="avatarSaved.message.value" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { userAPI } from '@/api'
import { useAuthStore } from '@/stores/auth'
import type { User } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'
import FormError from '@/components/common/FormError.vue'
import FormSuccess from '@/components/common/FormSuccess.vue'
import { useTransientMessage } from '@/composables/useTransientMessage'

const props = defineProps<{
  user: User | null
}>()

const { t } = useI18n()
const authStore = useAuthStore()

const targetAvatarUploadBytes = 20 * 1024
const avatarScaleSteps = [1, 0.92, 0.84, 0.76, 0.68, 0.6, 0.52, 0.44, 0.36]
const avatarQualitySteps = [0.92, 0.84, 0.76, 0.68, 0.6, 0.52, 0.44, 0.36]
const avatarDraft = ref('')
const avatarError = ref('')
const avatarSaved = useTransientMessage()
const avatarSaving = ref(false)

const displayName = computed(() => props.user?.username?.trim() || props.user?.email?.trim() || t('profile.user'))
const avatarInitial = computed(() => displayName.value.charAt(0).toUpperCase() || 'U')
const avatarPreviewUrl = computed(() => avatarDraft.value.trim() || props.user?.avatar_url?.trim() || '')

watch(
  () => props.user?.avatar_url,
  () => {
    avatarDraft.value = ''
  }
)

function normalizeUploadedAvatar(value: string): string | null {
  const normalized = value.trim()
  if (!normalized) {
    return null
  }

  if (!/^data:image\/[a-zA-Z0-9.+-]+;base64,/i.test(normalized)) {
    avatarError.value = t('profile.avatar.uploadRequired')
    return null
  }

  return normalized
}

function readFileAsDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : '')
    reader.onerror = () => reject(new Error(t('profile.avatar.readFailed')))
    reader.readAsDataURL(file)
  })
}

function loadImage(dataURL: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image()
    image.onload = () => resolve(image)
    image.onerror = () => reject(new Error(t('profile.avatar.readFailed')))
    image.src = dataURL
  })
}

function canvasToBlob(canvas: HTMLCanvasElement, type: string, quality: number): Promise<Blob> {
  return new Promise((resolve, reject) => {
    canvas.toBlob((blob) => {
      if (!blob) {
        reject(new Error(t('profile.avatar.compressFailed')))
        return
      }
      resolve(blob)
    }, type, quality)
  })
}

async function compressAvatarFile(file: File): Promise<File> {
  const sourceDataURL = await readFileAsDataURL(file)
  const image = await loadImage(sourceDataURL)
  const canvas = document.createElement('canvas')
  const ctx = canvas.getContext('2d')
  if (!ctx) {
    throw new Error(t('profile.avatar.compressFailed'))
  }

  for (const scale of avatarScaleSteps) {
    const width = Math.max(1, Math.round(image.naturalWidth * scale))
    const height = Math.max(1, Math.round(image.naturalHeight * scale))
    canvas.width = width
    canvas.height = height
    ctx.clearRect(0, 0, width, height)
    ctx.drawImage(image, 0, 0, width, height)

    for (const quality of avatarQualitySteps) {
      const blob = await canvasToBlob(canvas, 'image/webp', quality)
      if (blob.size <= targetAvatarUploadBytes) {
        const fileName = file.name.replace(/\.[^.]+$/, '') || 'avatar'
        return new File([blob], `${fileName}.webp`, { type: 'image/webp' })
      }
    }
  }

  throw new Error(t('profile.avatar.compressTooLarge'))
}

async function prepareAvatarUpload(file: File): Promise<File> {
  if (!file.type.startsWith('image/')) {
    throw new Error(t('profile.avatar.invalidType'))
  }
  if (file.type === 'image/gif') {
    if (file.size > targetAvatarUploadBytes) {
      throw new Error(t('profile.avatar.gifTooLarge'))
    }
    return file
  }
  if (file.size <= targetAvatarUploadBytes) {
    return file
  }
  return compressAvatarFile(file)
}

async function handleAvatarFileChange(event: Event) {
  const input = event.target as HTMLInputElement | null
  const file = input?.files?.[0]
  if (input) {
    input.value = ''
  }
  if (!file) {
    return
  }

  avatarError.value = ''
  avatarSaved.clear()
  try {
    const preparedFile = await prepareAvatarUpload(file)
    const dataURL = await readFileAsDataURL(preparedFile)
    const normalized = normalizeUploadedAvatar(dataURL)
    if (!normalized) {
      return
    }
    avatarDraft.value = normalized
  } catch (error: unknown) {
    // 这里的错误都是上面本地化过的（类型不对 / GIF 太大 / 压不到 20KB / 读不出来）
    avatarError.value = extractApiErrorMessage(error, t('profile.avatar.readFailed'))
  }
}

async function handleAvatarSave() {
  const normalized = normalizeUploadedAvatar(avatarDraft.value)
  if (!normalized) {
    return
  }

  avatarSaving.value = true
  avatarError.value = ''
  try {
    const updated = await userAPI.updateProfile({ avatar_url: normalized })
    authStore.user = updated
    avatarDraft.value = updated.avatar_url?.trim() || ''
    avatarSaved.show(t('profile.avatar.saveSuccess'))
  } catch (error: unknown) {
    avatarError.value = extractApiErrorMessage(error, t('profile.avatar.saveFailed'))
  } finally {
    avatarSaving.value = false
  }
}

async function handleAvatarDelete() {
  if (avatarSaving.value) {
    return
  }
  avatarError.value = ''
  avatarSaved.clear()
  if (!avatarDraft.value.trim() && !props.user?.avatar_url?.trim()) {
    avatarError.value = t('profile.avatar.emptyDeleteHint')
    return
  }

  avatarSaving.value = true
  try {
    const updated = await userAPI.updateProfile({ avatar_url: '' })
    authStore.user = updated
    avatarDraft.value = ''
    avatarSaved.show(t('profile.avatar.deleteSuccess'))
  } catch (error: unknown) {
    avatarError.value = extractApiErrorMessage(error, t('profile.avatar.deleteFailed'))
  } finally {
    avatarSaving.value = false
  }
}
</script>
