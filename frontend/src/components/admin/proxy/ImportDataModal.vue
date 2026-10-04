<template>
  <BaseDialog
    :show="show"
    :title="t('admin.proxies.dataImportTitle')"
    width="normal"
    close-on-click-outside
    @close="handleClose"
  >
    <form id="import-proxy-data-form" class="space-y-4" @submit.prevent="handleImport">
      <div class="text-sm text-af-ink-2">
        {{ t('admin.proxies.dataImportHint') }}
      </div>
      <div
        class="rounded-lg border border-af-warning/30 bg-af-warning-tint p-3 text-xs text-af-warning"
      >
        {{ t('admin.proxies.dataImportWarning') }}
      </div>

      <div>
        <label class="input-label">{{ t('admin.proxies.dataImportFile') }}</label>
        <div
          class="flex items-center justify-between gap-3 rounded-lg border border-dashed border-af-hairline-strong bg-af-sunken px-4 py-3"
        >
          <div class="min-w-0">
            <div class="truncate text-sm text-af-ink-2">
              {{ fileName || t('admin.proxies.dataImportSelectFile') }}
            </div>
            <div class="text-xs text-af-ink-3">JSON (.json)</div>
          </div>
          <button type="button" class="btn btn-secondary shrink-0" @click="openFilePicker">
            {{ t('common.chooseFile') }}
          </button>
        </div>
        <input
          ref="fileInput"
          type="file"
          class="hidden"
          accept="application/json,.json"
          @change="handleFileChange"
        />
      </div>

      <div
        v-if="result"
        class="space-y-2 rounded-xl border border-af-hairline p-4"
      >
        <div class="text-sm font-medium text-af-ink">
          {{ t('admin.proxies.dataImportResult') }}
        </div>
        <div class="text-sm text-af-ink-2">
          {{ t('admin.proxies.dataImportResultSummary', result) }}
        </div>

        <div v-if="errorItems.length" class="mt-2">
          <div class="text-sm font-medium text-af-danger">
            {{ t('admin.proxies.dataImportErrors') }}
          </div>
          <div
            class="mt-2 max-h-48 overflow-auto rounded-lg bg-af-sunken p-3 font-mono text-xs"
          >
            <div v-for="(item, idx) in errorItems" :key="idx" class="whitespace-pre-wrap">
              {{ item.kind }} {{ item.name || item.proxy_key || '-' }} — {{ item.message }}
            </div>
          </div>
        </div>
      </div>
    </form>

    <template #footer>
      <div class="flex w-full flex-wrap items-center justify-end gap-3">
        <FormError class="mr-auto min-w-0 flex-1" :message="importError" />
        <button class="btn btn-secondary" type="button" :disabled="importing" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button
          class="btn btn-primary"
          type="submit"
          form="import-proxy-data-form"
          :disabled="importing"
        >
          {{ importing ? t('admin.proxies.dataImporting') : t('admin.proxies.dataImportButton') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import FormError from '@/components/common/FormError.vue'
import { adminAPI } from '@/api/admin'
import type { AdminDataImportResult } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'

interface Props {
  show: boolean
}

interface Emits {
  (e: 'close'): void
  (e: 'imported'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { t } = useI18n()

const importing = ref(false)
const hasImportedData = ref(false)
const file = ref<File | null>(null)
const result = ref<AdminDataImportResult | null>(null)
// 没选文件、JSON 解析失败、请求失败：显示在弹窗底部，不只进控制台（与渠道导入同一处理）
const importError = ref('')

const fileInput = ref<HTMLInputElement | null>(null)
const fileName = computed(() => file.value?.name || '')

const errorItems = computed(() => result.value?.errors || [])

watch(
  () => props.show,
  (open) => {
    if (open) {
      file.value = null
      hasImportedData.value = false
      result.value = null
      importError.value = ''
      if (fileInput.value) {
        fileInput.value.value = ''
      }
    }
  }
)

const openFilePicker = () => {
  fileInput.value?.click()
}

const handleFileChange = (event: Event) => {
  const target = event.target as HTMLInputElement
  file.value = target.files?.[0] || null
  importError.value = ''
}

const handleClose = () => {
  if (importing.value) return
  if (hasImportedData.value) {
    hasImportedData.value = false
    emit('imported')
  }
  emit('close')
}

const readFileAsText = async (sourceFile: File): Promise<string> => {
  if (typeof sourceFile.text === 'function') {
    return sourceFile.text()
  }

  if (typeof sourceFile.arrayBuffer === 'function') {
    const buffer = await sourceFile.arrayBuffer()
    return new TextDecoder().decode(buffer)
  }

  return await new Promise<string>((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result ?? ''))
    reader.onerror = () => reject(reader.error || new Error(t('common.fileReadFailed')))
    reader.readAsText(sourceFile)
  })
}

const handleImport = async () => {
  importError.value = ''
  if (!file.value) {
    importError.value = t('admin.proxies.dataImportSelectFile')
    return
  }

  importing.value = true
  try {
    const text = await readFileAsText(file.value)
    const dataPayload = JSON.parse(text)

    const res = await adminAPI.proxies.importData({ data: dataPayload })

    result.value = res

    const msgParams: Record<string, unknown> = {
      proxy_created: res.proxy_created,
      proxy_reused: res.proxy_reused,
      proxy_failed: res.proxy_failed
    }

    if (res.proxy_failed > 0) {
      hasImportedData.value ||= res.proxy_created > 0 || res.proxy_reused > 0
      console.error(t('admin.proxies.dataImportCompletedWithErrors', msgParams))
    } else {
      hasImportedData.value = false
      emit('imported')
    }
  } catch (error: unknown) {
    importError.value = error instanceof SyntaxError
      ? t('admin.proxies.dataImportParseFailed')
      : extractApiErrorMessage(error, t('admin.proxies.dataImportFailed'))
    console.error(importError.value, error)
  } finally {
    importing.value = false
  }
}
</script>
