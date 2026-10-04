<template>
  <!--
    管理站顶栏：56px（与用户站同高），一张面 + 底部 hairline，无玻璃 / 阴影。
    左：移动端菜单；右：文档、语言、管理员菜单。页面标题在内容区（AdminPageHeader，A3）。
  -->
  <header class="sticky top-0 z-30 border-b border-af-hairline bg-af-sheet">
    <div class="flex h-topbar items-center justify-between gap-3 px-4 sm:px-6">
      <div class="flex min-w-0 items-center gap-3">
        <button
          type="button"
          class="btn btn-ghost btn-icon btn-sm lg:hidden"
          :aria-label="t('common.toggleMenu')"
          @click="toggleMobileSidebar"
        >
          <Icon name="menu" size="md" />
        </button>
      </div>

      <div class="flex shrink-0 items-center gap-1 sm:gap-2">
        <a
          v-if="docUrl"
          :href="docUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="btn btn-ghost btn-sm hidden sm:inline-flex"
        >
          <Icon name="book" size="sm" />
          {{ t('nav.docs') }}
        </a>

        <LocaleSwitcher />

        <div v-if="user" ref="dropdownRef" class="relative">
          <button
            type="button"
            class="flex items-center gap-2 rounded-md py-1 pl-1 pr-2 transition-colors hover:bg-af-sunken"
            :aria-label="t('common.userMenu')"
            :aria-expanded="dropdownOpen ? 'true' : 'false'"
            data-testid="admin-user-menu"
            @click="toggleDropdown"
          >
            <span class="flex h-7 w-7 items-center justify-center overflow-hidden rounded-full bg-af-brand-tint text-xs font-semibold text-af-brand">
              <img v-if="avatarUrl" :src="avatarUrl" :alt="displayName" class="h-full w-full object-cover" />
              <span v-else>{{ userInitials }}</span>
            </span>
            <span class="hidden text-left md:block">
              <span class="block text-sm font-medium leading-tight text-af-ink">{{ displayName }}</span>
              <span class="block text-xs leading-tight text-af-ink-3">{{ t('admin.users.roles.' + user.role) }}</span>
            </span>
            <Icon name="chevronDown" size="sm" class="hidden text-af-ink-3 md:block" />
          </button>

          <transition name="dropdown">
            <div v-if="dropdownOpen" class="dropdown absolute right-0 mt-2 w-56">
              <div class="border-b border-af-hairline px-4 py-3">
                <div class="text-sm font-medium text-af-ink">{{ displayName }}</div>
                <div class="truncate text-xs text-af-ink-3">{{ user.email }}</div>
              </div>

              <div class="py-1">
                <router-link to="/profile" class="dropdown-item" @click="closeDropdown">
                  <Icon name="user" size="sm" />
                  {{ t('nav.accountSecurity') }}
                </router-link>
              </div>

              <div v-if="contactInfo" class="border-t border-af-hairline px-4 py-2.5 text-xs text-af-ink-3">
                {{ t('common.contactSupport') }}:
                <span class="font-medium text-af-ink-2">{{ contactInfo }}</span>
              </div>

              <div class="border-t border-af-hairline py-1">
                <button type="button" class="dropdown-item w-full text-af-danger hover:text-af-danger" @click="handleLogout">
                  <Icon name="login" size="sm" />
                  {{ t('nav.logout') }}
                </button>
              </div>
            </div>
          </transition>
        </div>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeUrl } from '@/utils/url'

const router = useRouter()
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const user = computed(() => authStore.user)
const dropdownOpen = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)
const contactInfo = computed(() => appStore.contactInfo)
const docUrl = computed(() => sanitizeUrl(appStore.docUrl))
const avatarUrl = computed(() => user.value?.avatar_url?.trim() || '')

const userInitials = computed(() => {
  if (!user.value) return '?'
  const source = user.value.username?.trim() || user.value.email?.split('@')[0] || '?'
  return source.slice(0, 2).toUpperCase()
})

const displayName = computed(() => user.value?.username?.trim() || user.value?.email || '')

function toggleMobileSidebar() {
  appStore.toggleMobileSidebar()
}

function toggleDropdown() {
  dropdownOpen.value = !dropdownOpen.value
}

function closeDropdown() {
  dropdownOpen.value = false
}

async function handleLogout() {
  closeDropdown()
  try {
    await authStore.logout()
  } catch (error) {
    // 登出接口失败也回登录页
    console.error('Logout error:', error)
  }
  await router.push('/login')
}

function handleClickOutside(event: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(event.target as Node)) {
    closeDropdown()
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.dropdown-enter-active,
.dropdown-leave-active {
  transition: all 0.15s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
