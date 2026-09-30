<template>
  <!--
    唯一的顶栏：公开站与控制台是同一组页签（产品 / 模型与价格 / 服务状态 / 文档），控制台自己的导航在左侧栏（ConsoleSidebar）。
    模型与服务状态的入口只在这里（muqian 2026-09-30），侧栏不重复；控制台里点它们留在控制台（页签带 CONSOLE_SHELL_STATE）。
    56px、粘性；底色半透明 + 背景模糊，滚动时内容从下面透过来。控制台宽度与侧栏布局对齐（max-w-console）。
    <lg 时第二行横向滚动：公开站放页签，控制台放侧栏的全部条目（窄屏没有侧栏）；语言 / 主题收进头像菜单。
  -->
  <header class="sticky top-0 z-30 border-b border-af-hairline/70 bg-af-sheet/80 backdrop-blur-md">
    <div class="mx-auto flex h-topbar items-center gap-8 px-6" :class="variant === 'console' ? 'max-w-console' : 'max-w-site'">
      <RouterLink :to="brandPath" class="flex shrink-0 items-center gap-2.5" data-testid="site-brand">
        <BrandLogo :src="customLogo" :alt="siteName" class="h-7 w-7 text-af-ink" />
        <span class="text-base font-semibold tracking-[-0.01em] text-af-ink">{{ siteName }}</span>
      </RouterLink>

      <div class="hidden h-full min-w-0 flex-1 items-center lg:flex">
        <NavTabs :tabs="tabs" />
      </div>

      <div class="ml-auto flex shrink-0 items-center gap-1">
        <template v-if="variant === 'console'">
          <BalanceLink v-if="user && !isSimpleMode" class="hidden sm:block" />
          <AnnouncementBell v-if="user" />
          <div class="hidden lg:block"><LocaleSwitcher /></div>
          <button
            type="button"
            class="hidden h-8 w-8 items-center justify-center rounded-md text-af-ink-2 transition-colors hover:bg-af-sunken hover:text-af-ink lg:flex"
            :aria-label="isDark ? t('userUi.topbar.switchToLight') : t('userUi.topbar.switchToDark')"
            @click="toggleTheme"
          >
            <Icon :name="isDark ? 'sun' : 'moon'" size="sm" />
          </button>
          <UserMenu v-if="user" />
        </template>

        <template v-else>
          <!-- 无框（muqian：胶囊框和浅底按钮丑）：语言 / 主题是纯图标文字，一道细竖线隔开账户入口 -->
          <div class="flex items-center gap-4">
            <LocaleSwitcher />
            <button
              type="button"
              class="flex h-8 w-6 items-center justify-center text-af-ink-2 transition-colors hover:text-af-ink"
              :aria-label="isDark ? t('userUi.topbar.switchToLight') : t('userUi.topbar.switchToDark')"
              @click="toggleTheme"
            >
              <Icon :name="isDark ? 'sun' : 'moon'" size="sm" />
            </button>
            <template v-if="!adminSite">
              <span class="h-4 w-px bg-af-hairline-strong" aria-hidden="true" />
              <RouterLink v-if="authenticated" :to="CONSOLE_HOME_PATH" class="nav-text-link text-af-brand hover:text-af-brand-hover" data-testid="nav-console">
                {{ t('userUi.nav.console') }}
                <Icon name="arrowRight" size="xs" />
              </RouterLink>
              <template v-else>
                <RouterLink to="/login" :class="['nav-text-link', registrationEnabled ? '' : 'text-af-brand hover:text-af-brand-hover']" data-testid="nav-login">
                  {{ t('userUi.nav.login') }}
                </RouterLink>
                <RouterLink v-if="registrationEnabled" to="/register" class="nav-text-link text-af-brand hover:text-af-brand-hover" data-testid="nav-register">
                  {{ t('userUi.footer.register') }}
                </RouterLink>
              </template>
            </template>
          </div>
        </template>
      </div>
    </div>

    <!-- 小屏：第二行横向滚动（控制台 = 侧栏全部条目 + 顶栏页签，公开站 = 页签） -->
    <div v-if="mobileTabs.length" class="border-t border-af-hairline lg:hidden">
      <div class="mx-auto h-11 max-w-site overflow-x-auto px-3 scrollbar-hide">
        <NavTabs :tabs="mobileTabs" />
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { IS_ADMIN_SITE } from '@/app/site'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useTheme } from '@/composables/useTheme'
import { sanitizeUrl } from '@/utils/url'
import { SITE_FEATURES } from '@/utils/siteFeatures'
import { FeatureFlags, resolveFeatureFlag } from '@/utils/featureFlags'
import AnnouncementBell from '@/components/common/AnnouncementBell.vue'
import BrandLogo from '@/components/common/BrandLogo.vue'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import BalanceLink from './BalanceLink.vue'
import NavTabs from './NavTabs.vue'
import UserMenu from './UserMenu.vue'
import { CONSOLE_HOME_PATH, buildPublicNav } from './navItems'
import { useConsoleNav } from './useConsoleNav'
import { CONSOLE_SHELL_STATE } from './consoleShell'

const props = defineProps<{ variant: 'public' | 'console' }>()

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { isDark, toggleTheme } = useTheme()
const { flatItems: consoleItems, refreshBatchImageAccess } = useConsoleNav()
const adminSite = IS_ADMIN_SITE

// 批量生图条目按用户的密钥权限出现：只有控制台壳、且批量生图没被代码关掉时才探测（已登录时才发请求）
onMounted(() => {
  if (props.variant === 'console' && SITE_FEATURES.batchImage) void refreshBatchImageAccess()
})

const user = computed(() => authStore.user)
const authenticated = computed(() => authStore.isAuthenticated)
const isSimpleMode = computed(() => authStore.isSimpleMode)
const siteName = computed(() => appStore.siteName)
/** 后台配置的自定义 logo；没配时 BrandLogo 画内置的无框标 */
const customLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo, { allowRelative: true, allowDataUrl: true }))
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl))
const brandPath = computed(() => (props.variant === 'console' ? CONSOLE_HOME_PATH : '/home'))
const registrationEnabled = computed(() => Boolean(appStore.cachedPublicSettings?.registration_enabled))

const serviceStatusEnabled = computed(() => resolveFeatureFlag(appStore.cachedPublicSettings, FeatureFlags.channelMonitor))
// 控制台里的顶栏页签带上控制台标记：点「模型与价格」「服务状态」留在控制台（consoleShell.ts）
const tabs = computed(() => {
  const publicTabs = buildPublicNav({ t, adminSite, docUrl: docUrl.value, serviceStatusEnabled: serviceStatusEnabled.value })
  return props.variant === 'console' ? publicTabs.map((tab) => (tab.external ? tab : { ...tab, state: CONSOLE_SHELL_STATE })) : publicTabs
})
// 窄屏行在 DOM 里排在侧栏前面：新手引导按第一个 [data-tour] 定位，这一行不带锚点，免得桌面端指向隐藏元素。
// 控制台窄屏没有第一行页签：侧栏条目后面接上顶栏页签，模型 / 服务状态才有入口
const mobileTabs = computed(() =>
  props.variant === 'console'
    ? [...consoleItems.value.map(({ dataTour: _tour, icon: _icon, ...item }) => item), ...tabs.value]
    : tabs.value
)
</script>
