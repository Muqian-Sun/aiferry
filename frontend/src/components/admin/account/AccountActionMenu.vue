<template>
  <!--
    渠道行尾「⋯」菜单（A5）：外观与列表模板的 PopoverMenu / MenuItem 一致；按渠道类型只列能用的操作，删除放最后。
    自己定位（锚点是触发按钮的位置），因为自动刷新要知道菜单是否开着（开着时暂停刷新）。
  -->
  <Teleport to="body">
    <div v-if="show && anchorRect">
      <!-- Backdrop: click anywhere outside to close -->
      <div class="fixed inset-0 z-[9998]" @click="emit('close')"></div>
      <div
        ref="menuRef"
        role="menu"
        class="action-menu-content fixed z-[9999] w-48 overflow-y-auto overscroll-contain rounded-lg border border-af-hairline bg-af-sheet py-1 shadow-lg"
        :style="menuStyle"
        @click.stop
      >
        <template v-if="account">
          <MenuItem icon="play" @click="fire('test')">{{ t('admin.accounts.testConnection') }}</MenuItem>
          <MenuItem icon="chart" @click="fire('stats')">{{ t('admin.accounts.viewStats') }}</MenuItem>
          <MenuItem icon="clock" @click="fire('schedule')">{{ t('admin.scheduledTests.schedule') }}</MenuItem>
          <MenuItem v-if="canDuplicate" icon="copy" @click="fire('duplicate')">{{ t('admin.accounts.duplicateAccount') }}</MenuItem>
          <!-- 影子账号不持凭据:重授权/刷新 token 对其无效(后端拒绝),故隐藏(外审 G4)。 -->
          <template v-if="(account.type === 'oauth' || account.type === 'setup-token') && !isShadow">
            <MenuItem icon="link" @click="fire('reauth')">{{ t('admin.accounts.reAuthorize') }}</MenuItem>
            <MenuItem icon="refresh" @click="fire('refresh-token')">{{ t('admin.accounts.refreshToken') }}</MenuItem>
          </template>
          <MenuItem v-if="isOpenAIOAuthParent" icon="sparkles" @click="fire('create-spark-shadow')">{{ t('admin.accounts.createSparkShadow') }}</MenuItem>
          <MenuItem v-if="supportsPrivacy" icon="shield" @click="fire('set-privacy')">{{ t('admin.accounts.setPrivacy') }}</MenuItem>
          <template v-if="hasRecoverableState || hasQuotaLimit">
            <MenuItem divider />
            <MenuItem v-if="hasRecoverableState" icon="sync" @click="fire('recover-state')">{{ t('admin.accounts.recoverState') }}</MenuItem>
            <MenuItem v-if="hasQuotaLimit" icon="refresh" @click="fire('reset-quota')">{{ t('admin.accounts.resetQuota') }}</MenuItem>
          </template>
          <MenuItem divider />
          <MenuItem icon="trash" danger data-testid="account-menu-delete" @click="fire('delete')">{{ t('common.delete') }}</MenuItem>
        </template>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, ref, watch, onUnmounted } from 'vue'
import { useResizeObserver, useWindowSize } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { MenuItem } from '@/components/admin/list'
import type { Account } from '@/types'

const props = defineProps<{ show: boolean; account: Account | null; anchorRect: DOMRect | null }>()
type MenuEvent = 'test' | 'stats' | 'schedule' | 'duplicate' | 'reauth' | 'refresh-token' | 'recover-state' | 'reset-quota' | 'set-privacy' | 'create-spark-shadow' | 'delete'
const emit = defineEmits(['close', 'test', 'stats', 'schedule', 'duplicate', 'reauth', 'refresh-token', 'recover-state', 'reset-quota', 'set-privacy', 'create-spark-shadow', 'delete'])
const { t } = useI18n()

function fire(event: MenuEvent) {
  if (!props.account) return
  emit(event, props.account)
  emit('close')
}
const menuRef = ref<HTMLElement | null>(null)
const { width: viewportWidth, height: viewportHeight } = useWindowSize()
const viewportPadding = 8
const menuPosition = ref({ top: viewportPadding, left: viewportPadding })
const menuStyle = computed(() => ({
  top: `${menuPosition.value.top}px`,
  left: `${menuPosition.value.left}px`,
  maxWidth: `${Math.max(0, viewportWidth.value - viewportPadding * 2)}px`,
  maxHeight: `${Math.max(0, viewportHeight.value - viewportPadding * 2)}px`
}))

const updatePosition = () => {
  if (!menuRef.value || !props.anchorRect) return

  const { width, height } = menuRef.value.getBoundingClientRect()
  const anchor = props.anchorRect
  const gap = 4
  const maxTop = viewportHeight.value - height - viewportPadding
  const top = anchor.bottom + gap <= maxTop
    ? anchor.bottom + gap
    : anchor.top - height - gap
  const left = viewportWidth.value < 768
    ? anchor.left + anchor.width / 2 - width / 2
    : anchor.right - width

  menuPosition.value.top = Math.max(viewportPadding, Math.min(top, maxTop))
  menuPosition.value.left = Math.max(viewportPadding, Math.min(left, viewportWidth.value - width - viewportPadding))
}

// Measure after rendering; menu items and translated labels can change its size.
watch([menuRef, () => props.anchorRect, viewportWidth, viewportHeight], updatePosition, { flush: 'post' })
useResizeObserver(menuRef, updatePosition)

// 复制渠道只复制 key 和端点、其余走新建流程（muqian 2026-10-07），所以只有第三方 key 渠道能复制
const canDuplicate = computed(() => {
  if (!props.account || props.account.parent_account_id != null) return false
  return props.account.type === 'apikey'
})
const isRateLimited = computed(() => {
  if (props.account?.rate_limit_reset_at && new Date(props.account.rate_limit_reset_at) > new Date()) {
    return true
  }
  const modelLimits = (props.account?.extra as Record<string, unknown> | undefined)?.model_rate_limits as
    | Record<string, { rate_limit_reset_at: string }>
    | undefined
  if (modelLimits) {
    const now = new Date()
    return Object.values(modelLimits).some(info => new Date(info.rate_limit_reset_at) > now)
  }
  return false
})
const isOverloaded = computed(() => props.account?.overload_until && new Date(props.account.overload_until) > new Date())
const isTempUnschedulable = computed(() => props.account?.temp_unschedulable_until && new Date(props.account.temp_unschedulable_until) > new Date())
const hasRecoverableState = computed(() => {
  return props.account?.status === 'error' || Boolean(isRateLimited.value) || Boolean(isOverloaded.value) || Boolean(isTempUnschedulable.value)
})
const isAntigravityOAuth = computed(() => props.account?.platform === 'antigravity' && props.account?.type === 'oauth')
const isOpenAIOAuth = computed(() => props.account?.platform === 'openai' && props.account?.type === 'oauth')
// 影子账号(链接型,持 parent_account_id)不持凭据、type 不可变,凭据/隐私类操作对其无效。
const isShadow = computed(() => props.account?.parent_account_id != null)
// A "parent" OpenAI OAuth account is one that is NOT itself a shadow (parent_account_id == null)
const isOpenAIOAuthParent = computed(() => isOpenAIOAuth.value && !isShadow.value)
const supportsPrivacy = computed(() => (isAntigravityOAuth.value || isOpenAIOAuth.value) && !isShadow.value)
const hasQuotaLimit = computed(() => {
  return (props.account?.type === 'apikey' || props.account?.type === 'bedrock') && (
    (props.account?.quota_limit ?? 0) > 0 ||
    (props.account?.quota_daily_limit ?? 0) > 0 ||
    (props.account?.quota_weekly_limit ?? 0) > 0
  )
})

const handleKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Escape') emit('close')
}

watch(
  () => props.show,
  (visible) => {
    if (visible) {
      window.addEventListener('keydown', handleKeydown)
    } else {
      window.removeEventListener('keydown', handleKeydown)
    }
  },
  { immediate: true }
)

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})
</script>
