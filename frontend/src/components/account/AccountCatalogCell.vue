<template>
  <!-- 已上架模型（A5）：前几个模型 ID 一行一个的小字，多出的写「+N」；下架的划线。点模型打开诊断。 -->
  <div v-if="entries.length > 0" class="flex max-w-56 flex-col items-start gap-0.5" data-testid="account-catalog-cell">
    <button
      v-for="entry in displayEntries"
      :key="entry.id"
      type="button"
      class="max-w-full truncate text-left font-mono text-xs transition-colors hover:underline"
      :class="entry.status === 'listed' ? 'text-af-ink-2 hover:text-af-ink' : 'text-af-ink-3 line-through'"
      :title="entry.status === 'listed' ? entry.model_id : `${entry.model_id} · ${t('admin.accounts.catalogUnlisted')}`"
      data-testid="account-catalog-chip"
      @click.stop="emit('diagnose', entry)"
    >
      {{ entry.model_id }}
    </button>
    <span
      v-if="hiddenCount > 0"
      class="text-xs tabular-nums text-af-ink-3"
      :title="hiddenTitle"
      data-testid="account-catalog-more"
    >
      +{{ hiddenCount }}
    </span>
  </div>
  <span v-else class="text-xs text-af-ink-4" :title="t('admin.accounts.catalogNone')" data-testid="account-catalog-none">—</span>
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
