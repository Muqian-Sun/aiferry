<template>
  <!--
    认证页外壳（用户站与管理站登录共用），与首页同一套视觉（muqian 2026-09-23「登录注册页和首页风格匹配」）：
    公开站顶栏 + 页脚（SiteShell public），白底，表单直接放在页面上，不套卡片框。
    用户站 lg 起左右两栏：左边是首页首屏的标语（大字 + 流动光泽 + 三条要点），右边是表单，中间一道竖向细线；
    窄屏与管理站只有表单一栏（管理员不需要看产品标语）。进场动效同首页：左栏逐行淡入上浮，表单晚 150ms。
  -->
  <SiteShell variant="public" flush>
    <div
      class="mx-auto grid w-full max-w-site flex-1 items-center gap-12 px-6 py-12 sm:py-16 lg:py-10"
      :class="adminSite ? '' : 'lg:grid-cols-[1.1fr_1fr] lg:gap-0'"
    >
      <div v-if="!adminSite" v-reveal.stagger class="hidden text-left lg:block lg:pb-16 lg:pr-16" data-testid="auth-aside">
        <p class="text-[3.25rem] font-semibold leading-[1.06] tracking-[-0.02em] text-af-ink">
          {{ t('userUi.home.hero.title') }}<br />
          <span class="text-flow">{{ t('userUi.home.hero.titleAccent') }}</span>
        </p>
        <p class="mt-6 max-w-md text-[17px] leading-8 text-af-ink-2">{{ t('userUi.home.hero.description') }}</p>
        <ul class="mt-9 space-y-3 text-sm text-af-ink-2">
          <li v-for="point in POINTS" :key="point" class="flex items-center gap-2">
            <span class="flex h-5 w-5 items-center justify-center rounded-full bg-af-brand-tint text-af-brand"><Icon name="check" size="xs" /></span>
            {{ t(`userUi.home.hero.points.${point}`) }}
          </li>
        </ul>
      </div>

      <div v-reveal="adminSite ? undefined : 150" :class="adminSite ? '' : 'lg:border-l lg:border-af-hairline lg:py-6 lg:pl-16'">
        <div class="mx-auto w-full max-w-[380px]" data-testid="auth-form-column">
          <slot />
          <div v-if="$slots.footer" class="mt-8 border-t border-af-hairline pt-6 text-sm text-af-ink-2">
            <slot name="footer" />
          </div>
        </div>
      </div>
    </div>
  </SiteShell>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { IS_ADMIN_SITE } from '@/app/site'
import { useAppStore } from '@/stores'
import { vReveal } from '@/directives/reveal'
import Icon from '@/components/icons/Icon.vue'
import SiteShell from '@/components/user/shell/SiteShell.vue'

/** 左栏三条要点，与首页首屏同一组文案 */
const POINTS = ['protocol', 'sameModel', 'ledger'] as const

const { t } = useI18n()
const appStore = useAppStore()
const adminSite = IS_ADMIN_SITE

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>
