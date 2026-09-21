<template>
  <!-- 404：公开壳 + 一段文字与两个动作，不做插画。管理站也会渲染这一页（公开壳在管理站只留品牌与工具） -->
  <SiteShell variant="public">
    <div class="mx-auto max-w-md py-24">
      <p class="font-mono text-13 text-af-ink-4">404</p>
      <h1 class="mt-2 text-xl font-semibold text-af-ink">{{ t('userUi.notFound.title') }}</h1>
      <p class="mt-2 text-sm text-af-ink-3">{{ t('userUi.notFound.description') }}</p>
      <div class="mt-6 flex flex-wrap gap-3">
        <button type="button" class="btn btn-secondary btn-md" @click="goBack">{{ t('userUi.notFound.back') }}</button>
        <RouterLink :to="homePath" class="btn btn-primary btn-md">{{ t('userUi.notFound.home') }}</RouterLink>
      </div>
    </div>
  </SiteShell>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { APP_SITE, IS_ADMIN_SITE } from '@/app/site'
import { defaultAuthedPath } from '@/router/defaultAuthedPath'
import SiteShell from '@/components/user/shell/SiteShell.vue'

const { t } = useI18n()
const router = useRouter()
// 用户站回首页；管理站没有 /home，回仪表盘
const homePath = IS_ADMIN_SITE ? defaultAuthedPath(APP_SITE) : '/home'

function goBack(): void {
  router.back()
}
</script>
