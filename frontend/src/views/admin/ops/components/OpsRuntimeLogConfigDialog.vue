<script setup lang="ts">
/**
 * 运行时日志配置（2026-10-04 运维页重排）：原来直接摊在系统日志上面，挪进这个弹窗。
 * 保存方式不变：「保存并应用」立即生效；去留按 P6（方案页 2c5jp1…）再定。
 */
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select from '@/components/common/Select.vue'
import { opsAPI, type OpsRuntimeLogConfig } from '@/api/admin/ops'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>()

const { t } = useI18n()

const loading = ref(false)
const saving = ref(false)
const errorMessage = ref('')
const confirmReset = ref(false)
const config = reactive<OpsRuntimeLogConfig>({
  level: 'info',
  persist_access_logs: false,
  enable_sampling: false,
  sampling_initial: 100,
  sampling_thereafter: 100,
  caller: true,
  stacktrace_level: 'error',
  retention_days: 30
})

const levelOptions = ['debug', 'info', 'warn', 'error'].map((v) => ({ value: v, label: v }))
const stacktraceOptions = ['none', 'error', 'fatal'].map((v) => ({ value: v, label: v }))

async function load() {
  loading.value = true
  errorMessage.value = ''
  try {
    Object.assign(config, await opsAPI.getRuntimeLogConfig())
  } catch (err) {
    console.error('[OpsRuntimeLogConfigDialog] load failed', err)
    errorMessage.value = t('admin.ops.page.loadFailed')
  } finally {
    loading.value = false
  }
}

watch(
  () => props.show,
  (show) => {
    if (show) void load()
  },
  { immediate: true }
)

async function save() {
  saving.value = true
  errorMessage.value = ''
  try {
    Object.assign(config, await opsAPI.updateRuntimeLogConfig({ ...config }))
    emit('saved')
    emit('close')
  } catch (err) {
    console.error('[OpsRuntimeLogConfigDialog] save failed', err)
    errorMessage.value = t('admin.ops.page.saveFailed')
  } finally {
    saving.value = false
  }
}

async function reset() {
  confirmReset.value = false
  saving.value = true
  errorMessage.value = ''
  try {
    Object.assign(config, await opsAPI.resetRuntimeLogConfig())
    emit('saved')
  } catch (err) {
    console.error('[OpsRuntimeLogConfigDialog] reset failed', err)
    errorMessage.value = t('admin.ops.page.saveFailed')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <BaseDialog :show="props.show" :title="t('admin.ops.systemLogs.runtimeConfig')" width="wide" @close="emit('close')">
    <div class="space-y-4">
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <label class="text-xs text-af-ink-2">
          {{ t('admin.ops.systemLogs.level') }}
          <Select v-model="config.level" class="mt-1" :options="levelOptions" />
        </label>
        <label class="text-xs text-af-ink-2">
          {{ t('admin.ops.systemLogs.stacktraceThreshold') }}
          <Select v-model="config.stacktrace_level" class="mt-1" :options="stacktraceOptions" />
        </label>
        <label class="text-xs text-af-ink-2">
          {{ t('admin.ops.systemLogs.retentionDays') }}
          <input v-model.number="config.retention_days" type="number" min="1" max="3650" class="input mt-1" />
          <span class="mt-1 block text-xs text-af-ink-3">{{ t('admin.ops.systemLogs.retentionDaysHint') }}</span>
        </label>
        <label class="text-xs text-af-ink-2">
          {{ t('admin.ops.systemLogs.samplingInitial') }}
          <input v-model.number="config.sampling_initial" type="number" min="1" class="input mt-1" />
        </label>
        <label class="text-xs text-af-ink-2">
          {{ t('admin.ops.systemLogs.samplingThereafter') }}
          <input v-model.number="config.sampling_thereafter" type="number" min="1" class="input mt-1" />
        </label>
      </div>
      <div class="flex flex-wrap items-center gap-x-4 gap-y-2">
        <label class="inline-flex items-center gap-2 text-xs text-af-ink-2">
          <input v-model="config.caller" type="checkbox" />
          {{ t('admin.ops.systemLogs.caller') }}
        </label>
        <label class="inline-flex items-center gap-2 text-xs text-af-ink-2">
          <input v-model="config.enable_sampling" type="checkbox" />
          {{ t('admin.ops.systemLogs.sampling') }}
        </label>
        <label class="inline-flex items-center gap-2 text-xs text-af-ink-2">
          <input v-model="config.persist_access_logs" type="checkbox" />
          {{ t('admin.ops.systemLogs.persistAccessLogs') }}
        </label>
      </div>
      <p class="text-xs text-af-ink-3">{{ t('admin.ops.systemLogs.persistAccessLogsHint') }}</p>
      <p v-if="errorMessage" class="text-sm text-af-danger">{{ errorMessage }}</p>
      <div class="flex flex-wrap justify-end gap-2">
        <button type="button" class="btn btn-secondary" :disabled="saving || loading" @click="confirmReset = true">
          {{ t('admin.ops.systemLogs.resetDefaults') }}
        </button>
        <button type="button" class="btn btn-primary" :disabled="saving || loading" @click="save">
          {{ saving ? t('common.saving') : t('admin.ops.systemLogs.saveAndApply') }}
        </button>
      </div>
    </div>
    <ConfirmDialog
      :show="confirmReset"
      :title="t('admin.ops.systemLogs.resetDefaults')"
      :message="t('admin.ops.systemLogs.resetRuntimeConfigConfirm')"
      @confirm="reset"
      @cancel="confirmReset = false"
    />
  </BaseDialog>
</template>
