<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import QuotaDimensionRow from './QuotaDimensionRow.vue'

const { t } = useI18n()

const props = defineProps<{
  totalLimit: number | null
  dailyLimit: number | null
  weeklyLimit: number | null
}>()

const emit = defineEmits<{
  'update:totalLimit': [value: number | null]
  'update:dailyLimit': [value: number | null]
  'update:weeklyLimit': [value: number | null]
}>()

const enabled = computed(() =>
  (props.totalLimit != null && props.totalLimit > 0) ||
  (props.dailyLimit != null && props.dailyLimit > 0) ||
  (props.weeklyLimit != null && props.weeklyLimit > 0)
)

const localEnabled = ref(enabled.value)
const collapsed = ref(false)

// Sync when props change externally
watch(enabled, (val) => {
  localEnabled.value = val
})

// When toggle is turned off, clear all values and expand
watch(localEnabled, (val) => {
  if (!val) {
    collapsed.value = false
    emit('update:totalLimit', null)
    emit('update:dailyLimit', null)
    emit('update:weeklyLimit', null)
  }
})
</script>

<template>
  <div class="rounded-lg border border-af-hairline">
      <!-- Header: toggle + collapse -->
      <div class="flex items-center justify-between p-4" :class="{ 'pb-0': localEnabled && !collapsed }">
        <div class="flex items-center gap-2 flex-1 cursor-pointer" @click="localEnabled && (collapsed = !collapsed)">
          <svg v-if="localEnabled" class="h-4 w-4 text-af-ink-3 transition-transform" :class="{ '-rotate-90': collapsed }" viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.938a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z" clip-rule="evenodd" />
          </svg>
          <div>
            <label class="input-label mb-0 cursor-pointer">{{ t('admin.accounts.quotaLimitToggle') }}</label>
            <p class="mt-0.5 text-xs text-af-ink-3">
              {{ t('admin.accounts.quotaLimitToggleHint') }}
            </p>
          </div>
        </div>
        <button
          type="button"
          @click="localEnabled = !localEnabled"
          :class="[
            'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
            localEnabled ? 'bg-af-brand' : 'bg-af-hairline'
          ]"
        >
          <span
            :class="[
              'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
              localEnabled ? 'translate-x-5' : 'translate-x-0'
            ]"
          />
        </button>
      </div>

      <!-- Collapsible content -->
      <div v-if="localEnabled && !collapsed" class="space-y-2 p-4 pt-3">
        <!-- 日 / 周限额一律滚动窗口；设了限额的维度用到 80% 时给管理员发一次提醒（配了 SMTP 时） -->
        <QuotaDimensionRow
          :label="t('admin.accounts.quotaDailyLimit')"
          :limit="dailyLimit"
          :hint="t('admin.accounts.quotaDailyLimitHint')"
          @update:limit="emit('update:dailyLimit', $event)"
        />
        <QuotaDimensionRow
          :label="t('admin.accounts.quotaWeeklyLimit')"
          :limit="weeklyLimit"
          :hint="t('admin.accounts.quotaWeeklyLimitHint')"
          @update:limit="emit('update:weeklyLimit', $event)"
        />
        <QuotaDimensionRow
          :label="t('admin.accounts.quotaTotalLimit')"
          :limit="totalLimit"
          :hint="t('admin.accounts.quotaTotalLimitHint')"
          @update:limit="emit('update:totalLimit', $event)"
        />
      </div>
  </div>
</template>
