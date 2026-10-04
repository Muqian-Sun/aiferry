<template>
  <!--
    渠道测试（测试连接 / 定时测试）的模型下拉下面那一行：模型只来自这个渠道的承接（价格页加的行），
    加载失败写原因；没有可选的模型就说清楚，并给去价格页的入口。不回退到任何写死的模型表。
  -->
  <p v-if="error" class="mt-1 text-xs text-af-danger" data-testid="test-models-error">
    {{ t('admin.accounts.testModelsLoadFailed', { reason: error }) }}
  </p>
  <p v-else-if="emptyText" class="mt-1 text-xs text-af-ink-3" data-testid="test-models-empty">
    {{ emptyText }}
    <RouterLink
      :to="{ path: '/pricing', query: { channel: String(accountId) } }"
      class="font-medium text-af-ink hover:underline"
    >
      {{ t('admin.accounts.testModelsGoToPricing') }}
    </RouterLink>
  </p>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{
  accountId: number
  /** 加载失败的原因；有值时只显示它 */
  error?: string
  /** 下拉为空时的说明；空串 = 有模型可选，不显示 */
  emptyText?: string
}>()

const { t } = useI18n()
</script>
