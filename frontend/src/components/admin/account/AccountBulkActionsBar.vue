<template>
  <!--
    渠道批量条（A5）：选中行才出现。常用的开 / 关调度、批量编辑直接放；重置状态、刷新令牌、探测倍率、
    按筛选结果批量编辑、删除收进「更多」。计数旁可一键扩到全部筛选结果。
  -->
  <BulkBar :count="selectedIds.length" @clear="$emit('clear')">
    <template #meta>
      <span v-if="allResultsSelected" class="text-13 opacity-80">{{ t('admin.accounts.bulkActions.allResults') }}</span>
      <button
        v-else-if="totalResults > selectedIds.length"
        type="button"
        class="bulk-link"
        :disabled="selectingAll"
        data-testid="bulk-select-all-results"
        @click="$emit('select-all-results')"
      >
        {{
          selectingAll
            ? t('admin.accounts.bulkActions.selectingAll')
            : t('admin.accounts.bulkActions.selectAllResults', { count: totalResults })
        }}
      </button>
    </template>
    <button type="button" class="bulk-btn" data-testid="bulk-enable-scheduling" @click="$emit('toggle-schedulable', true)">
      {{ t('admin.accounts.bulkActions.enableScheduling') }}
    </button>
    <button type="button" class="bulk-btn" data-testid="bulk-disable-scheduling" @click="$emit('toggle-schedulable', false)">
      {{ t('admin.accounts.bulkActions.disableScheduling') }}
    </button>
    <button type="button" class="bulk-btn" data-test="edit-selected" @click="$emit('edit-selected')">
      {{ t('admin.accounts.bulkActions.edit') }}
    </button>
    <PopoverMenu width-class="w-56">
      <template #trigger>
        <button type="button" class="bulk-btn" data-testid="bulk-more">
          {{ t('common.more') }}
          <Icon name="chevronDown" size="xs" />
        </button>
      </template>
      <MenuItem icon="refresh" @click="$emit('reset-status')">{{ t('admin.accounts.bulkActions.resetStatus') }}</MenuItem>
      <MenuItem icon="key" @click="$emit('refresh-token')">{{ t('admin.accounts.bulkActions.refreshToken') }}</MenuItem>
      <MenuItem icon="chart" data-testid="bulk-probe-upstream-billing" @click="$emit('probe-upstream-billing')">
        {{ t('admin.accounts.bulkActions.probeUpstreamBilling') }}
      </MenuItem>
      <MenuItem icon="edit" data-test="edit-filtered" @click="$emit('edit-filtered')">
        {{ t('admin.accounts.bulkActions.editFiltered') }}
      </MenuItem>
      <MenuItem divider />
      <MenuItem icon="trash" danger data-testid="bulk-delete" @click="$emit('delete')">
        {{ t('admin.accounts.bulkActions.delete') }}
      </MenuItem>
    </PopoverMenu>
  </BulkBar>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { BulkBar, MenuItem, PopoverMenu } from '@/components/admin/list'

defineProps<{
  selectedIds: number[]
  totalResults: number
  selectingAll: boolean
  allResultsSelected: boolean
}>()

defineEmits([
  'delete',
  'edit-selected',
  'edit-filtered',
  'clear',
  'select-all-results',
  'toggle-schedulable',
  'reset-status',
  'refresh-token',
  'probe-upstream-billing'
])

const { t } = useI18n()
</script>
