<template>
  <!--
    唯一的顶栏：公开站与控制台只是页签集合不同。
    56px、粘性；底色半透明 + 背景模糊，滚动时内容从下面透过来；<lg 时页签下沉为第二行横向滚动，语言 / 主题收进头像菜单。
    页签是胶囊式高亮（当前页一块浅底），图标按钮圆形。
  -->
  <header class="sticky top-0 z-30 border-b border-af-hairline/70 bg-af-sheet/80 backdrop-blur-md">
    <div class="mx-auto flex h-topbar max-w-site items-center gap-8 px-6">
      <RouterLink :to="brandPath" class="flex shrink-0 items-center gap-2.5" data-testid="site-brand">
        <img :src="logoSrc" :alt="siteName" class="h-7 w-7 rounded-lg object-contain" />
        <span class="text-base font-semibold tracking-[-0.01em] text-af-ink">{{ siteName }}</span>
      </RouterLink>

      <div class="hidden h-full min-w-0 flex-1 items-center lg:flex">
        <NavTabs :tabs="tabs" :more="more" />
      </div>

      <div class="ml-auto flex shrink-0 items-center gap-1">
        <template v-if="variant === 'console'">
          <BalanceLink v-if="user && !isSimpleMode" class="hidden sm:block" />
          <AnnouncementBell v-if="user" />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="hidden h-8 items-center rounded-md px-2 text-13 font-medium text-af-ink-2 transition-colors hover:bg-af-sunken hover:text-af-ink md:flex"
          >
            {{ t('userUi.nav.docs') }}
          </a>
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
          <!-- 语言与主题收进一个细边胶囊，和右边的账户按钮分开 -->
          <div class="flex items-center gap-0.5 rounded-full border border-af-hairline bg-af-sheet p-0.5">
            <LocaleSwitcher />
            <button
              type="button"
              class="flex h-8 w-8 items-center justify-center rounded-full text-af-ink-2 transition-colors hover:bg-af-sunken hover:text-af-ink"
              :aria-label="isDark ? t('userUi.topbar.switchToLight') : t('userUi.topbar.switchToDark')"
              @click="toggleTheme"
            >
              <Icon :name="isDark ? 'sun' : 'moon'" size="sm" />
            </button>
          </div>
          <template v-if="!adminSite">
            <RouterLink v-if="authenticated" :to="CONSOLE_HOME_PATH" class="nav-account-btn ml-2" data-testid="nav-console">
              {{ t('userUi.nav.console') }}
              <Icon name="arrowRight" size="xs" />
            </RouterLink>
            <template v-else>
              <RouterLink to="/login" class="nav-account-btn ml-2" data-testid="nav-login">
                {{ t('userUi.nav.login') }}
              </RouterLink>
              <RouterLink v-if="registrationEnabled" to="/register" class="nav-account-btn nav-account-btn-solid" data-testid="nav-register">
                {{ t('userUi.footer.register') }}
              </RouterLink>
            </template>
          </template>
        </template>
      </div>
    </div>

    <!-- 小屏：页签第二行，横向滚动 -->
    <div v-if="tabs.length || more.length" class="border-t border-af-hairline lg:hidden">
      <div class="mx-auto h-11 max-w-site overflow-x-auto px-3 scrollbar-hide">
        <NavTabs :tabs="tabs" :more="more" />
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
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'
import { sanitizeUrl } from '@/utils/url'
import AnnouncementBell from '@/components/common/AnnouncementBell.vue'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import BalanceLink from './BalanceLink.vue'
import NavTabs from './NavTabs.vue'
import UserMenu from './UserMenu.vue'
import { CONSOLE_HOME_PATH, buildConsoleNav, buildPublicNav } from './navItems'

const props = defineProps<{ variant: 'public' | 'console' }>()

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { isDark, toggleTheme } = useTheme()
const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()
const adminSite = IS_ADMIN_SITE

// 批量生图页签按用户的密钥权限出现：只有控制台壳需要探测（已登录时才发请求）
onMounted(() => {
  if (props.variant === 'console') void refreshBatchImageAccess()
})

const user = computed(() => authStore.user)
const authenticated = computed(() => authStore.isAuthenticated)
const isSimpleMode = computed(() => authStore.isSimpleMode)
const siteName = computed(() => appStore.siteName)
const logoSrc = computed(
  () => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo, { allowRelative: true, allowDataUrl: true }) || '/logo.svg'
)
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl))
const brandPath = computed(() => (props.variant === 'console' ? CONSOLE_HOME_PATH : '/home'))
const registrationEnabled = computed(() => Boolean(appStore.cachedPublicSettings?.registration_enabled))

const customItems = computed(() => appStore.cachedPublicSettings?.custom_menu_items ?? [])

const consoleNav = computed(() =>
  buildConsoleNav({
    t,
    simpleMode: isSimpleMode.value,
    backendMode: appStore.backendModeEnabled,
    batchImageEnabled: canUseBatchImage.value,
    customItems: customItems.value
  })
)

const tabs = computed(() =>
  props.variant === 'console'
    ? consoleNav.value.tabs
    : buildPublicNav({ t, adminSite, docUrl: docUrl.value })
)
const more = computed(() => (props.variant === 'console' ? consoleNav.value.more : []))
</script>
