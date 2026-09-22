<template>
  <BaseDialog :show="show" :title="dialogTitle" width="wide" @close="emit('close')">
    <div v-if="loading" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400" data-testid="model-catalog-diagnosis-loading">
      {{ t('common.loading') }}
    </div>
    <div v-else-if="error" class="py-6 text-center text-sm text-red-600 dark:text-red-400" data-testid="model-catalog-diagnosis-error">
      {{ error }}
    </div>
    <div v-else-if="accounts.length === 0" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400" data-testid="model-catalog-diagnosis-empty">
      {{ t('admin.modelCatalog.diagnosis.empty') }}
    </div>
    <div v-else class="overflow-x-auto">
      <table class="min-w-full text-sm">
        <thead>
          <tr class="text-left text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">
            <th class="px-3 py-2">{{ t('admin.modelCatalog.diagnosis.columns.account') }}</th>
            <th class="px-3 py-2">{{ t('admin.modelCatalog.diagnosis.columns.priority') }}</th>
            <th class="px-3 py-2">{{ t('admin.modelCatalog.diagnosis.columns.schedulable') }}</th>
            <th v-for="inbound in inboundProtocols" :key="inbound" class="px-3 py-2 text-center">
              {{ t(`admin.modelCatalog.diagnosis.inbound.${inbound}`) }}
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="account in accounts"
            :key="account.id"
            class="border-t border-gray-100 dark:border-dark-700"
            data-testid="model-catalog-diagnosis-row"
          >
            <td class="px-3 py-2">
              <div class="font-medium text-gray-900 dark:text-white">{{ account.name }}</div>
              <div class="text-xs text-gray-500 dark:text-gray-400">
                #{{ account.id }} · {{ account.platform }} / {{ account.type }}
                <span v-if="account.vendor" class="ml-1 rounded bg-gray-100 px-1.5 py-0.5 text-[11px] dark:bg-dark-600">{{ account.vendor }}</span>
              </div>
            </td>
            <td class="px-3 py-2 text-gray-700 dark:text-gray-300">
              {{ account.priority === null ? t('admin.modelCatalog.diagnosis.followAccount') : account.priority }}
            </td>
            <td class="px-3 py-2">
              <span v-if="account.schedulable" class="text-emerald-600 dark:text-emerald-400" data-testid="model-catalog-diagnosis-schedulable">✓</span>
              <span v-else class="text-red-600 dark:text-red-400" data-testid="model-catalog-diagnosis-blocked">
                ✗ {{ blockedReasonLabel(account.blocked_reason) }}
              </span>
            </td>
            <td
              v-for="inbound in inboundProtocols"
              :key="inbound"
              class="px-3 py-2 text-center"
              :data-testid="`model-catalog-diagnosis-serves-${inbound}`"
              :data-serves="account.serves?.[inbound] ? 'true' : 'false'"
            >
              <span v-if="account.serves?.[inbound]" class="text-emerald-600 dark:text-emerald-400">✓</span>
              <span v-else class="text-gray-400 dark:text-gray-500">✗</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <template #footer>
      <button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.close') }}</button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
/**
 * 目录条目诊断：一条上架模型绑定了哪些资源、每个此刻能不能调度、四个入站协议各能不能承接。
 * 数据来自 GET /admin/model-catalog/entries/:id/diagnosis；目录页行操作与渠道页的模型 chip 共用。
 */
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import modelCatalogAPI, {
  type ModelCatalogDiagnosisAccount,
  type ModelCatalogInboundProtocol
} from '@/api/admin/modelCatalog'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{
  show: boolean
  entryId: number | null
  /** 只用于标题；不传就只显示条目 ID。 */
  modelId?: string
}>()
const emit = defineEmits<{ (e: 'close'): void }>()

const { t, te } = useI18n()

const inboundProtocols: ModelCatalogInboundProtocol[] = ['anthropic', 'chat_completions', 'responses', 'gemini']
const loading = ref(false)
const error = ref('')
const accounts = ref<ModelCatalogDiagnosisAccount[]>([])

const dialogTitle = computed(() =>
  t('admin.modelCatalog.diagnosis.title', { model: props.modelId || (props.entryId !== null ? `#${props.entryId}` : '') })
)

const blockedReasonLabel = (reason?: string): string => {
  if (!reason) return ''
  const key = `admin.modelCatalog.diagnosis.reasons.${reason}`
  return te(key) ? t(key) : reason
}

const load = async (entryId: number) => {
  loading.value = true
  error.value = ''
  accounts.value = []
  try {
    const diagnosis = await modelCatalogAPI.diagnose(entryId)
    accounts.value = diagnosis.accounts ?? []
  } catch (err) {
    error.value = extractApiErrorMessage(err, t('common.error'))
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.show, props.entryId] as const,
  ([show, entryId]) => {
    if (show && entryId !== null) void load(entryId)
  },
  { immediate: true }
)
</script>
