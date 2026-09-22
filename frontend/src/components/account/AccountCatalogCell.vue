<template>
  <div v-if="entries.length > 0" class="flex max-w-56 flex-wrap gap-1" data-testid="account-catalog-cell">
    <button
      v-for="entry in displayEntries"
      :key="entry.id"
      type="button"
      class="inline-flex max-w-32 items-center truncate rounded-md px-1.5 py-0.5 text-xs font-medium transition-colors"
      :class="entry.status === 'listed'
        ? 'bg-sky-50 text-sky-700 hover:bg-sky-100 dark:bg-sky-900/30 dark:text-sky-300 dark:hover:bg-sky-900/50'
        : 'bg-gray-100 text-gray-500 line-through hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-400'"
      :title="entry.status === 'listed' ? entry.model_id : `${entry.model_id} · ${t('admin.accounts.catalogUnlisted')}`"
      data-testid="account-catalog-chip"
      @click.stop="emit('diagnose', entry)"
    >
      {{ entry.model_id }}
    </button>
    <span
      v-if="hiddenCount > 0"
      class="inline-flex items-center rounded-md bg-gray-100 px-1.5 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-600 dark:text-gray-300"
      :title="hiddenTitle"
      data-testid="account-catalog-more"
    >
      +{{ hiddenCount }}
    </span>
  </div>
  <span v-else class="text-xs text-gray-400 dark:text-gray-500" :title="t('admin.accounts.catalogNone')" data-testid="account-catalog-none">—</span>
</template>

<script setup lang="ts">
/**
 * 渠道页「已上架模型」列：这个资源被哪些目录条目绑定。数据由父组件从 /admin/model-catalog/entries
 * 的 bindings[] 反查得来（不需要后端新接口）；点 chip 打开该条目的诊断弹窗。
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ModelCatalogEntry } from '@/api/admin/modelCatalog'

const props = withDefaults(defineProps<{
  entries: ModelCatalogEntry[]
  maxDisplay?: number
}>(), { maxDisplay: 4 })
const emit = defineEmits<{ (e: 'diagnose', entry: ModelCatalogEntry): void }>()

const { t } = useI18n()

const displayEntries = computed(() => props.entries.slice(0, props.maxDisplay))
const hiddenCount = computed(() => Math.max(0, props.entries.length - props.maxDisplay))
const hiddenTitle = computed(() => props.entries.slice(props.maxDisplay).map((entry) => entry.model_id).join(', '))
</script>
