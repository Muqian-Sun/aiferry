<template>
  <!-- 渠道筛选（A5）：搜索 + 一行筛选标签（平台 / 接入方式 / 状态 / 隐私），放在 ListToolbar 里。 -->
  <SearchInput
    :model-value="searchQuery"
    compact
    :placeholder="t('admin.accounts.searchAccounts')"
    class="w-full sm:w-64"
    @update:model-value="$emit('update:searchQuery', $event)"
    @search="$emit('change')"
  />
  <FilterChip :model-value="filters.platform" :label="t('admin.accounts.platform')" :options="pOpts" test-id="filter-platform" @change="update('platform', $event)" />
  <FilterChip :model-value="filters.type" :label="t('admin.accounts.access.label')" :options="tOpts" test-id="filter-type" @change="update('type', $event)" />
  <FilterChip :model-value="filters.status" :label="t('admin.accounts.columns.status')" :options="sOpts" test-id="filter-status" @change="update('status', $event)" />
  <FilterChip :model-value="filters.privacy_mode" :label="t('admin.accounts.privacyFilter')" :options="privacyOpts" test-id="filter-privacy" @change="update('privacy_mode', $event)" />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import SearchInput from '@/components/common/SearchInput.vue'
import { FilterChip } from '@/components/admin/list'
import type { FilterOption } from '@/components/admin/list'
import { CONCRETE_PLATFORM_OPTIONS } from '@/constants/platforms'

const props = defineProps<{ searchQuery: string; filters: Record<string, any> }>()
const emit = defineEmits(['update:searchQuery', 'update:filters', 'change'])
const { t } = useI18n()

function update(key: 'platform' | 'type' | 'status' | 'privacy_mode', value: string | number) {
  emit('update:filters', { ...props.filters, [key]: value })
  emit('change')
}

const pOpts = computed<FilterOption[]>(() => [...CONCRETE_PLATFORM_OPTIONS])
// 接入方式：与列表名称下面那行同一套叫法
const tOpts = computed<FilterOption[]>(() => [
  { value: 'apikey', label: t('admin.accounts.access.apikey') },
  { value: 'oauth', label: t('admin.accounts.access.oauth') },
  { value: 'setup-token', label: t('admin.accounts.access.setupToken') },
  { value: 'bedrock', label: t('admin.accounts.access.bedrock') }
])
const sOpts = computed<FilterOption[]>(() => [
  { value: 'active', label: t('admin.accounts.status.active') },
  { value: 'inactive', label: t('admin.accounts.status.inactive') },
  { value: 'error', label: t('admin.accounts.status.error') },
  { value: 'rate_limited', label: t('admin.accounts.status.rateLimited') },
  { value: 'temp_unschedulable', label: t('admin.accounts.status.tempUnschedulable') },
  { value: 'unschedulable', label: t('admin.accounts.status.unschedulable') }
])
const privacyOpts = computed<FilterOption[]>(() => [
  { value: '__unset__', label: t('admin.accounts.privacyUnset') },
  { value: 'training_off', label: t('admin.accounts.privacyOptions.trainingOff') },
  { value: 'training_set_cf_blocked', label: t('admin.accounts.privacyOptions.cfBlocked') },
  { value: 'training_set_failed', label: t('admin.accounts.privacyOptions.failed') }
])
</script>
