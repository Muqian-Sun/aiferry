<template>
  <!-- 列设置（A4）：工具行右侧一个图标，勾选即生效、存在本机；底部「恢复默认」。 -->
  <PopoverMenu width-class="w-52" :close-on-select="false">
    <template #trigger="{ open }">
      <button
        type="button"
        class="rounded-md p-2 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink"
        :class="open ? 'bg-af-sunken text-af-ink' : ''"
        :title="t('common.columnSettings')"
        :aria-label="t('common.columnSettings')"
        data-testid="column-settings"
      >
        <Icon name="columns" size="md" />
      </button>
    </template>
    <div class="max-h-80 overflow-y-auto">
      <MenuItem
        v-for="col in settings.toggleableColumns.value"
        :key="col.key"
        :checked="settings.isVisible(col.key)"
        :data-testid="`column-toggle-${col.key}`"
        @click="settings.toggle(col.key)"
      >
        {{ col.label }}
      </MenuItem>
    </div>
    <MenuItem divider />
    <MenuItem :disabled="settings.isDefault.value" data-testid="column-reset" @click="settings.reset()">
      {{ t('common.restoreDefault') }}
    </MenuItem>
  </PopoverMenu>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PopoverMenu from './PopoverMenu.vue'
import MenuItem from './MenuItem.vue'
import type { ColumnSettings } from '@/composables/useColumnSettings'

defineProps<{ settings: ColumnSettings }>()

const { t } = useI18n()
</script>
