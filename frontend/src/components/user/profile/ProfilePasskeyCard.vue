<template>
  <!-- 小标题 + 列表；外层容器由调用方决定。用户站的两栏设置行自己画标题（headless，只留「添加」按钮），管理端卡片里保留小标题 -->
  <div class="space-y-4">
    <div class="flex items-start justify-between gap-4" :class="headless && !(enabled && supported && !showAddForm) ? 'hidden' : ''">
      <div v-if="!headless">
        <h3 class="text-sm font-semibold text-af-ink">
          {{ t('profile.passkey.title') }}
        </h3>
        <p class="mt-1 text-13 text-af-ink-3">
          {{ t('profile.passkey.description') }}
        </p>
      </div>
      <button
        v-if="enabled && supported && !showAddForm"
        type="button"
        class="btn btn-secondary btn-sm shrink-0"
        :disabled="busy"
        @click="showAddForm = true"
      >
        {{ t('profile.passkey.add') }}
      </button>
    </div>

    <div>
      <div v-if="!enabled" class="mb-5 text-sm text-af-ink-3">
        {{ t('profile.passkey.featureDisabled') }}
      </div>
      <div v-if="enabled && !supported" class="mb-5 text-sm text-af-warning">
        {{ t('profile.passkey.unsupported') }}
      </div>
      <div>
        <form
          v-if="enabled && supported && showAddForm"
          class="mb-5 flex flex-col gap-3 rounded-lg border border-af-hairline p-4"
          @submit.prevent="addPasskey"
        >
          <div class="grid gap-3 sm:grid-cols-2">
            <div>
              <label for="passkey-name" class="input-label">{{ t('profile.passkey.name') }}</label>
              <input
                id="passkey-name"
                v-model="newName"
                class="input"
                maxlength="100"
                :placeholder="t('profile.passkey.namePlaceholder')"
                autofocus
              />
            </div>
            <div>
              <label for="passkey-add-password" class="input-label">{{
                t('profile.currentPassword')
              }}</label>
              <input
                id="passkey-add-password"
                v-model="newPassword"
                type="password"
                autocomplete="current-password"
                class="input"
                :placeholder="t('profile.passkey.passwordPlaceholder')"
              />
            </div>
          </div>
          <FormError :message="addError" />
          <div class="flex justify-end gap-2">
            <button type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="cancelAdd">
              {{ t('common.cancel') }}
            </button>
            <button type="submit" class="btn btn-primary btn-sm" :disabled="busy || newPassword.length === 0">
              {{ busy ? t('common.processing') : t('profile.passkey.continue') }}
            </button>
          </div>
        </form>

        <div v-if="loading" class="flex justify-center py-6">
          <LoadingSpinner />
        </div>

        <FormError v-else-if="loadError" :message="loadError" />

        <p v-else-if="credentials.length === 0 && enabled" class="text-13 text-af-ink-3">
          {{ t('profile.passkey.empty') }}
        </p>

        <div v-else class="divide-y divide-af-hairline">
          <div
            v-for="credential in credentials"
            :key="credential.id"
            class="flex items-center justify-between gap-4 py-4 first:pt-0 last:pb-0"
          >
            <div class="min-w-0 flex-1">
              <form
                v-if="renamingId === credential.id"
                class="flex items-center gap-2"
                @submit.prevent="saveRename(credential)"
              >
                <input
                  v-model="renameDraft"
                  class="input min-w-0 flex-1"
                  maxlength="100"
                  :aria-label="t('profile.passkey.name')"
                  autofocus
                />
                <button type="submit" class="btn btn-primary btn-sm" :disabled="busy || !renameDraft.trim()">
                  {{ busy ? t('common.saving') : t('common.save') }}
                </button>
                <button type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="cancelRename">
                  {{ t('common.cancel') }}
                </button>
              </form>
              <div v-else class="flex items-center gap-2">
                <Icon name="key" size="md" class="shrink-0 text-af-brand" />
                <p class="truncate font-medium text-af-ink">
                  {{ credential.name }}
                </p>
                <span
                  v-if="credential.backup"
                  class="rounded-full bg-af-sunken px-2 py-0.5 text-xs text-af-ink-2"
                >
                  {{ t('profile.passkey.synced') }}
                </span>
              </div>
              <FormError v-if="renamingId === credential.id" class="mt-1" :message="renameError" />
              <p class="mt-1 text-xs text-af-ink-3">
                {{ t('profile.passkey.createdAt', { date: formatDate(credential.created_at) }) }}
                <template v-if="credential.last_used_at">
                  · {{ t('profile.passkey.lastUsed', { date: formatDate(credential.last_used_at) }) }}
                </template>
              </p>
            </div>
            <div v-if="renamingId !== credential.id" class="flex shrink-0 gap-2">
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="busy"
                @click="startRename(credential)"
              >
                {{ t('common.edit') }}
              </button>
              <button
                type="button"
                class="btn btn-ghost btn-sm text-af-danger hover:bg-af-danger-tint"
                :disabled="busy"
                @click="deletePasskey(credential)"
              >
                {{ t('common.delete') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 删除确认：吊销凭据需验证当前密码，防止被窃会话静默移除 Passkey -->
    <BaseDialog
      :show="deleteTarget !== null"
      :title="t('profile.passkey.deleteTitle')"
      width="narrow"
      :close-on-escape="!busy"
      @close="closeDeleteDialog"
    >
      <form id="passkey-delete-form" class="space-y-4" @submit.prevent="confirmDelete">
        <p class="text-sm text-af-ink-2">
          {{ t('profile.passkey.deleteConfirm', { name: deleteTarget?.name ?? '' }) }}
        </p>
        <div>
          <label for="passkey-delete-password" class="input-label">{{ t('profile.currentPassword') }}</label>
          <input
            id="passkey-delete-password"
            v-model="deletePassword"
            type="password"
            autocomplete="current-password"
            class="input"
            :placeholder="t('profile.passkey.passwordPlaceholder')"
            autofocus
          />
        </div>
        <FormError :message="deleteError" />
      </form>
      <template #footer>
        <button type="button" class="btn btn-secondary" :disabled="busy" @click="closeDeleteDialog">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="passkey-delete-form"
          class="btn btn-danger"
          :disabled="busy || deletePassword.length === 0"
        >
          {{ busy ? t('common.deleting') : t('common.delete') }}
        </button>
      </template>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import FormError from '@/components/common/FormError.vue'
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { passkeyAPI, type PasskeyCredentialSummary } from '@/api'
import { Icon } from '@/components/icons'
import { extractI18nErrorMessage } from '@/utils/apiError'

const props = defineProps<{ enabled: boolean; headless?: boolean }>()

const { t } = useI18n()
const supported = passkeyAPI.isSupported()
const loading = ref(false)
const busy = ref(false)
const showAddForm = ref(false)
const newName = ref('')
const newPassword = ref('')
const deleteTarget = ref<PasskeyCredentialSummary | null>(null)
const deletePassword = ref('')
const credentials = ref<PasskeyCredentialSummary[]>([])
const loadError = ref('')
const addError = ref('')
const deleteError = ref('')
const renamingId = ref<number | null>(null)
const renameDraft = ref('')
const renameError = ref('')

// 密码错误等后端原因按 auth.errors 翻译；浏览器 / 设备侧的失败（DOMException，消息是浏览器的英文）用兜底文案
function failureMessage(error: unknown, fallback: string): string {
  if (error instanceof DOMException) return fallback
  return extractI18nErrorMessage(error, t, 'auth.errors', fallback)
}

async function loadCredentials(): Promise<void> {
  if (!props.enabled) {
    credentials.value = []
    return
  }
  loading.value = true
  loadError.value = ''
  try {
    credentials.value = await passkeyAPI.list()
  } catch (error) {
    // 字符串错误码在 reason 字段（code 是数字状态码）；
    // 设置变更竞态下后端仍可能返回 PASSKEY_DISABLED，静默处理
    const reason = (error as { reason?: string }).reason
    if (reason !== 'PASSKEY_DISABLED') {
      loadError.value = failureMessage(error, t('profile.passkey.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

async function addPasskey(): Promise<void> {
  if (newPassword.value.length === 0) return
  busy.value = true
  addError.value = ''
  try {
    await passkeyAPI.register(newName.value.trim(), newPassword.value)
    cancelAdd()
    await loadCredentials()
  } catch (error) {
    // 用户在系统弹窗里点了取消：不算失败
    if (!(error instanceof DOMException && error.name === 'NotAllowedError')) {
      addError.value = failureMessage(error, t('profile.passkey.addFailed'))
    }
  } finally {
    busy.value = false
  }
}

function cancelAdd(): void {
  showAddForm.value = false
  newName.value = ''
  newPassword.value = ''
  addError.value = ''
}

function startRename(credential: PasskeyCredentialSummary): void {
  renamingId.value = credential.id
  renameDraft.value = credential.name
  renameError.value = ''
}

function cancelRename(): void {
  renamingId.value = null
  renameError.value = ''
}

async function saveRename(credential: PasskeyCredentialSummary): Promise<void> {
  const name = renameDraft.value.trim()
  if (!name) return
  if (name === credential.name) {
    cancelRename()
    return
  }
  busy.value = true
  renameError.value = ''
  try {
    await passkeyAPI.rename(credential.id, name)
    credential.name = name
    cancelRename()
  } catch (error) {
    renameError.value = failureMessage(error, t('profile.passkey.renameFailed'))
  } finally {
    busy.value = false
  }
}

function deletePasskey(credential: PasskeyCredentialSummary): void {
  deleteTarget.value = credential
  deletePassword.value = ''
  deleteError.value = ''
}

function closeDeleteDialog(): void {
  if (busy.value) return
  deleteTarget.value = null
  deletePassword.value = ''
  deleteError.value = ''
}

async function confirmDelete(): Promise<void> {
  const credential = deleteTarget.value
  if (!credential || deletePassword.value.length === 0) return
  busy.value = true
  deleteError.value = ''
  try {
    await passkeyAPI.remove(credential.id, deletePassword.value)
    credentials.value = credentials.value.filter((item) => item.id !== credential.id)
    busy.value = false
    closeDeleteDialog()
  } catch (error) {
    // 密码错误等失败保持对话框打开，允许重试
    deleteError.value = failureMessage(error, t('profile.passkey.deleteFailed'))
  } finally {
    busy.value = false
  }
}

function formatDate(value: string): string {
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric'
  }).format(new Date(value))
}

watch(
  () => props.enabled,
  () => {
    void loadCredentials()
  },
  { immediate: true }
)
</script>
