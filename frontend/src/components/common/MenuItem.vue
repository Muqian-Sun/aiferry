<template>
  <!-- 弹出菜单的一项：普通 / 危险（红字）/ 勾选态。分隔线用 <MenuItem divider />。 -->
  <div v-if="divider" class="my-1 border-t border-af-hairline" role="separator"></div>
  <button
    v-else
    type="button"
    role="menuitem"
    :disabled="disabled"
    :class="[
      'flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm transition-colors disabled:cursor-not-allowed disabled:opacity-40',
      danger ? 'text-af-danger hover:bg-af-danger-tint' : 'text-af-ink-2 hover:bg-af-sunken hover:text-af-ink'
    ]"
  >
    <Icon v-if="icon" :name="icon" size="sm" class="shrink-0" :class="danger ? '' : 'text-af-ink-3'" />
    <span class="min-w-0 flex-1 truncate"><slot /></span>
    <Icon v-if="checked" name="check" size="sm" class="shrink-0 text-af-ink" :stroke-width="2" />
  </button>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'
import type { IconName } from './types'

defineProps<{
  icon?: IconName
  danger?: boolean
  disabled?: boolean
  checked?: boolean
  divider?: boolean
}>()
</script>
