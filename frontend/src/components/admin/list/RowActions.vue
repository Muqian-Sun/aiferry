<template>
  <!-- 行尾操作（A4）：常用的一两个是图标按钮，其余进「⋯」。按钮不带文字，靠 title / aria-label 说明。 -->
  <div class="flex items-center justify-end gap-0.5" @click.stop>
    <button
      v-for="action in primaryActions"
      :key="action.key"
      type="button"
      class="rounded-md p-1.5 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink disabled:cursor-not-allowed disabled:opacity-40"
      :title="action.label"
      :aria-label="action.label"
      :disabled="action.disabled"
      :data-testid="`row-action-${action.key}`"
      @click="action.onSelect()"
    >
      <Icon :name="action.icon ?? 'edit'" size="sm" />
    </button>
    <PopoverMenu v-if="menuActions.length" width-class="w-44">
      <template #trigger="{ open }">
        <button
          type="button"
          class="rounded-md p-1.5 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink"
          :class="open ? 'bg-af-sunken text-af-ink' : ''"
          :title="moreLabel"
          :aria-label="moreLabel"
          :aria-expanded="open ? 'true' : 'false'"
          data-testid="row-action-more"
        >
          <Icon name="more" size="sm" />
        </button>
      </template>
      <template v-for="action in menuActions" :key="action.key">
        <MenuItem v-if="action.dividerBefore" divider />
        <MenuItem
          :icon="action.icon"
          :danger="action.danger"
          :disabled="action.disabled"
          :data-testid="`row-action-${action.key}`"
          @click="action.onSelect()"
        >
          {{ action.label }}
        </MenuItem>
      </template>
    </PopoverMenu>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PopoverMenu from './PopoverMenu.vue'
import MenuItem from './MenuItem.vue'
import type { RowAction } from './types'

const props = defineProps<{ actions: RowAction[] }>()

const { t } = useI18n()
const moreLabel = computed(() => t('common.more'))

// 危险操作不允许做成图标直接点，统一进菜单
const primaryActions = computed(() => props.actions.filter((a) => a.primary && !a.danger).slice(0, 2))
const menuActions = computed(() => props.actions.filter((a) => !primaryActions.value.includes(a)))
</script>
