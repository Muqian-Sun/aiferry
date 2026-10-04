<template>
  <!--
    渠道列表「已上架模型」列：只写个数（悬停列出模型 ID）；绑定了但没上架的另写「M 个未上架」。
    点它打开详情抽屉的「上架模型」页签，那里逐个列出并能诊断。
  -->
  <button
    v-if="entries.length > 0"
    type="button"
    class="whitespace-nowrap text-left text-sm tabular-nums text-af-ink-2 hover:text-af-ink hover:underline"
    :title="entries.map((entry) => entry.model_id).join('\n')"
    data-testid="account-catalog-cell"
    @click.stop="emit('open')"
  >
    <span data-testid="account-catalog-count">{{ listedCount }}</span>
    <span v-if="unlistedCount > 0" class="ml-1 text-xs text-af-ink-3" data-testid="account-catalog-unlisted">
      {{ t('admin.accounts.catalogUnlistedCount', { count: unlistedCount }) }}
    </span>
  </button>
  <span v-else class="text-sm text-af-ink-3" :title="t('admin.accounts.catalogNone')" data-testid="account-catalog-none">—</span>
</template>

<script setup lang="ts">
/**
 * 这个渠道被哪些目录条目绑定：数据由父组件从 /admin/model-catalog/entries 的 bindings[] 反查得来
 * （不需要后端新接口）。
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ModelCatalogEntry } from '@/api/admin/modelCatalog'

const props = defineProps<{ entries: ModelCatalogEntry[] }>()
const emit = defineEmits<{ (e: 'open'): void }>()

const { t } = useI18n()

const listedCount = computed(() => props.entries.filter((entry) => entry.status === 'listed').length)
const unlistedCount = computed(() => props.entries.length - listedCount.value)
</script>
