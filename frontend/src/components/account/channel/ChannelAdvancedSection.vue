<template>
  <!-- 渠道弹窗的「高级」：默认收起，标题下列出里面有哪几项，点开再改 -->
  <section class="rounded-lg border border-af-hairline" data-testid="channel-advanced">
    <button
      type="button"
      class="flex w-full items-start gap-2 px-4 py-3 text-left"
      :aria-expanded="open ? 'true' : 'false'"
      data-testid="channel-advanced-toggle"
      @click="open = !open"
    >
      <Icon name="chevronRight" size="sm" :class="['mt-0.5 shrink-0 text-af-ink-3 transition-transform', open ? 'rotate-90' : '']" />
      <span class="min-w-0">
        <span class="block text-sm font-medium text-af-ink">{{ t('admin.accounts.dialog.sections.advanced') }}</span>
        <span v-if="summary" class="mt-0.5 block text-xs text-af-ink-3">{{ summary }}</span>
      </span>
    </button>
    <!-- v-show：收起时字段仍在表单里，已填的值不丢 -->
    <div v-show="open" class="space-y-5 border-t border-af-hairline px-4 py-4">
      <slot />
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

defineProps<{
  /** 收起时显示的里面有哪几项，如「请求头覆写 · 池模式 · 拦截预热请求」 */
  summary?: string
}>()

const { t } = useI18n()
const open = ref(false)
</script>
