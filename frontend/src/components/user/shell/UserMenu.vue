<template>
  <div ref="rootRef" class="relative">
    <button
      type="button"
      class="flex h-8 items-center gap-2 rounded-md px-1 transition-colors hover:bg-af-sunken focus:outline-none focus-visible:ring-2 focus-visible:ring-af-brand/40"
      :aria-label="t('userUi.topbar.accountMenu')"
      :aria-expanded="open"
      aria-haspopup="menu"
      data-testid="user-menu-trigger"
      @click="open = !open"
    >
      <span class="flex h-7 w-7 items-center justify-center overflow-hidden rounded-full bg-af-brand-tint text-xs font-semibold text-af-brand">
        <img v-if="avatarUrl" :src="avatarUrl" :alt="displayName" class="h-full w-full object-cover" />
        <span v-else>{{ initials }}</span>
      </span>
      <Icon name="chevronDown" size="xs" class="hidden text-af-ink-3 md:block" />
    </button>

    <transition name="user-menu">
      <div
        v-if="open"
        class="absolute right-0 z-50 mt-1 w-64 rounded-lg border border-af-hairline bg-af-sheet py-1 shadow-lg"
        role="menu"
        data-testid="user-menu"
      >
        <div class="border-b border-af-hairline px-4 py-3">
          <div class="truncate text-sm font-medium text-af-ink">{{ displayName }}</div>
          <div class="truncate text-xs text-af-ink-3">{{ user?.email }}</div>
          <div class="mt-0.5 text-xs text-af-ink-3">{{ roleLabel }}</div>
        </div>

        <div class="py-1">
          <RouterLink to="/profile" class="dropdown-item" role="menuitem" @click="close">
            <Icon name="user" size="sm" />
            {{ t('userUi.nav.account') }}
          </RouterLink>
          <RouterLink to="/keys" class="dropdown-item" role="menuitem" @click="close">
            <Icon name="key" size="sm" />
            {{ t('userUi.nav.keys') }}
          </RouterLink>
        </div>

        <!-- 小屏时语言 / 主题收进菜单 -->
        <div class="border-t border-af-hairline py-1 lg:hidden">
          <div class="flex items-center justify-between px-4 py-1.5 text-sm text-af-ink-2">
            <span>{{ t('userUi.topbar.language') }}</span>
            <LocaleSwitcher />
          </div>
          <button type="button" class="dropdown-item w-full" role="menuitem" @click="toggleTheme">
            <Icon :name="isDark ? 'sun' : 'moon'" size="sm" />
            {{ isDark ? t('userUi.topbar.switchToLight') : t('userUi.topbar.switchToDark') }}
          </button>
        </div>

        <div v-if="contactInfo" class="border-t border-af-hairline px-4 py-2.5 text-xs text-af-ink-3">
          {{ t('common.contactSupport') }}:
          <span class="font-medium text-af-ink-2">{{ contactInfo }}</span>
        </div>

        <div v-if="showOnboardingButton" class="border-t border-af-hairline py-1">
          <button type="button" class="dropdown-item w-full" role="menuitem" @click="replayGuide">
            <Icon name="questionCircle" size="sm" />
            {{ t('onboarding.restartTour') }}
          </button>
        </div>

        <div class="border-t border-af-hairline py-1">
          <button type="button" class="dropdown-item w-full text-af-danger hover:text-af-danger" role="menuitem" data-testid="logout" @click="logout">
            <Icon name="login" size="sm" />
            {{ t('nav.logout') }}
          </button>
        </div>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore, useOnboardingStore } from '@/stores'
import { useTheme } from '@/composables/useTheme'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()
const onboardingStore = useOnboardingStore()
const { isDark, toggleTheme } = useTheme()

const open = ref(false)
const rootRef = ref<HTMLElement | null>(null)

const user = computed(() => authStore.user)
const avatarUrl = computed(() => user.value?.avatar_url?.trim() || '')
const contactInfo = computed(() => appStore.contactInfo)
const roleLabel = computed(() => (user.value ? t('admin.users.roles.' + user.value.role) : ''))
// 只在标准模式的管理员下显示新手引导按钮（同旧顶栏）
const showOnboardingButton = computed(() => !authStore.isSimpleMode && user.value?.role === 'admin')

const displayName = computed(() => {
  if (!user.value) return ''
  return user.value.username || user.value.email?.split('@')[0] || ''
})

const initials = computed(() => {
  if (!user.value) return ''
  const source = user.value.username || user.value.email?.split('@')[0] || ''
  return source.substring(0, 2).toUpperCase()
})

function close() {
  open.value = false
}

function replayGuide() {
  close()
  onboardingStore.replay()
}

async function logout() {
  close()
  try {
    await authStore.logout()
  } catch (error) {
    console.error('Logout error:', error)
  }
  await router.push('/login')
}

function onDocumentClick(event: MouseEvent) {
  if (rootRef.value && !rootRef.value.contains(event.target as Node)) close()
}

watch(() => route.fullPath, close)
onMounted(() => document.addEventListener('click', onDocumentClick))
onBeforeUnmount(() => document.removeEventListener('click', onDocumentClick))
</script>

<style scoped>
.user-menu-enter-active,
.user-menu-leave-active {
  transition: opacity 120ms ease-out, transform 120ms ease-out;
}
.user-menu-enter-from,
.user-menu-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
@media (prefers-reduced-motion: reduce) {
  .user-menu-enter-active,
  .user-menu-leave-active {
    transition-duration: 1ms;
  }
}
</style>
