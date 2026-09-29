<template>
  <!--
    区块内的 加载 / 空 / 错误 三态：一段居中文字 + 至多一个文字按钮，不做插画卡片。
    error 用 role=alert 让辅助技术立即播报；loading 用 role=status + aria-busy。
    组件不负责请求、路由或权限，点击动作只 emit('action')。
  -->
  <section
    class="flex flex-col items-center justify-center px-4 py-12 text-center"
    :role="kind === 'error' ? 'alert' : 'status'"
    :aria-busy="kind === 'loading' ? 'true' : undefined"
    :data-testid="`status-${kind}`"
  >
    <span v-if="kind === 'loading'" class="spinner mb-3 text-af-ink-4" data-testid="status-spinner" aria-hidden="true"></span>
    <Icon v-else :name="kind === 'error' ? 'exclamationCircle' : 'inbox'" size="lg" class="mb-3 text-af-ink-4" aria-hidden="true" />
    <p class="text-sm font-medium text-af-ink">{{ title }}</p>
    <p v-if="description" class="mt-1 max-w-sm text-13 text-af-ink-3">{{ description }}</p>
    <button
      v-if="actionLabel && kind !== 'loading'"
      type="button"
      class="btn btn-ghost btn-sm mt-3 text-af-brand hover:text-af-brand-hover"
      @click="emit('action')"
    >
      {{ actionLabel }}
    </button>
  </section>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'

withDefaults(
  defineProps<{
    kind: 'loading' | 'empty' | 'error'
    title: string
    description?: string
    actionLabel?: string
  }>(),
  { description: '', actionLabel: '' }
)

const emit = defineEmits<{ action: [] }>()
</script>
