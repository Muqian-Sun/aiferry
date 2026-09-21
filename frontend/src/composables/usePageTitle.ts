/**
 * 当前路由的页标题 / 说明（与 document.title 共用 resolveRouteMetaKeys 的解析）。
 * 自定义页取管理员配置的菜单 label；充值页标题随站点计费模式切换。
 */
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { getSiteContext } from '@/app/siteContext'
import { useAppStore } from '@/stores/app'
import { resolveRouteMetaKeys } from '@/router/title'
import { resolveSiteBillingMode } from '@/utils/siteBillingMode'

export function usePageTitle() {
  const route = useRoute()
  const { t } = useI18n()
  const appStore = useAppStore()

  const metaKeys = computed(() =>
    resolveRouteMetaKeys(route, { billingMode: resolveSiteBillingMode(appStore.cachedPublicSettings) })
  )

  const title = computed(() => {
    if (route.name === 'CustomPage') {
      const id = route.params.id as string
      const menuItem = getSiteContext().getCustomMenuItems().find((item) => item.id === id)
      if (menuItem?.label) return menuItem.label
    }
    const key = metaKeys.value.titleKey
    if (key) return t(key)
    return (route.meta.title as string) || ''
  })

  const description = computed(() => {
    const key = metaKeys.value.descriptionKey
    if (key) return t(key)
    return (route.meta.description as string) || ''
  })

  return { title, description }
}
