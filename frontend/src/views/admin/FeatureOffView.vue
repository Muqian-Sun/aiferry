<template>
  <!-- 侧栏灰色入口的落地页（A3）：功能没开时不再让入口消失，点进来告诉管理员去哪儿打开 -->
  <AppLayout>
    <div class="max-w-xl" data-testid="feature-off">
      <p class="text-base text-af-ink">{{ t('admin.featureOff.message', { name }) }}</p>
      <p class="mt-1 text-sm text-af-ink-3">{{ t('admin.featureOff.hint') }}</p>
      <RouterLink to="/settings/features" class="btn btn-primary mt-5 inline-flex">{{ t('admin.featureOff.goSettings') }}</RouterLink>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'

const route = useRoute()
const { t } = useI18n()
const name = computed(() => (typeof route.query.name === 'string' && route.query.name.trim()) || t('admin.featureOff.fallbackName'))
</script>
