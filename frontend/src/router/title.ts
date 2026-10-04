import { i18n } from '@/i18n'
import type { RouteLocationNormalizedLoaded } from 'vue-router'
import type { CustomMenuItem } from '@/types'
import { DEFAULT_SITE_NAME } from '@/utils/branding'
import { legalDocumentTitle } from '@/utils/legalDocumentTitle'

/**
 * 统一生成页面标题，避免多处写入 document.title 产生覆盖冲突。
 * 优先使用 titleKey 通过 i18n 翻译，fallback 到静态 routeTitle。
 */
export function resolveDocumentTitle(routeTitle: unknown, siteName?: string, titleKey?: string, brandFirst = false): string {
  const normalizedSiteName = typeof siteName === 'string' && siteName.trim() ? siteName.trim() : DEFAULT_SITE_NAME
  const join = (text: string) => (brandFirst ? `${normalizedSiteName} - ${text}` : `${text} - ${normalizedSiteName}`)

  if (typeof titleKey === 'string' && titleKey.trim()) {
    const translated = i18n.global.t(titleKey)
    if (translated && translated !== titleKey) {
      return join(translated)
    }
  }

  if (typeof routeTitle === 'string' && routeTitle.trim()) {
    return join(routeTitle.trim())
  }

  return normalizedSiteName
}

function legalTitleOf(doc: { id: string; title: string } | undefined): string | undefined {
  return doc ? legalDocumentTitle(doc, (key) => i18n.global.t(key)).trim() : undefined
}

export interface RouteTitleOptions {
  /** 条款文档（公开设置 login_agreement_documents）：/legal/:documentId 用文档自己的标题当页签标题 */
  legalDocuments?: Array<{ id: string; title: string }>
}

/** 条款页路由名，与 apps/user/routes.ts 中的声明保持一致。 */
export const LEGAL_DOCUMENT_ROUTE_NAME = 'LegalDocument'

/** 管理站「功能未开启」落地页路由名，与 apps/admin/routes.ts 中的声明保持一致；侧栏灰色入口带 ?name=功能名 */
export const FEATURE_OFF_ROUTE_NAME = 'AdminFeatureOff'

export interface RouteMetaKeys {
  titleKey?: string
  descriptionKey?: string
}

/**
 * 解析路由的 i18n 标题/描述 key（取 meta），页头与 document.title 共用。
 */
export function resolveRouteMetaKeys(route: Pick<RouteLocationNormalizedLoaded, 'meta'>): RouteMetaKeys {
  return {
    titleKey: typeof route.meta.titleKey === 'string' ? route.meta.titleKey : undefined,
    descriptionKey: typeof route.meta.descriptionKey === 'string' ? route.meta.descriptionKey : undefined,
  }
}

export function resolveRouteDocumentTitle(
  route: Pick<RouteLocationNormalizedLoaded, 'name' | 'params' | 'meta'> & { query?: RouteLocationNormalizedLoaded['query'] },
  siteName: string | undefined,
  customMenuItems: CustomMenuItem[] = [],
  options: RouteTitleOptions = {},
): string {
  const id = typeof route.params.id === 'string' ? route.params.id : ''
  const menuItem = route.name === 'CustomPage' && id
    ? customMenuItems.find((item) => item.id === id)
    : undefined
  const menuTitle = menuItem?.label.trim()
  const documentId = typeof route.params.documentId === 'string' ? route.params.documentId : ''
  const legalTitle = route.name === LEGAL_DOCUMENT_ROUTE_NAME && documentId
    ? legalTitleOf(options.legalDocuments?.find((doc) => doc.id === documentId))
    : undefined
  // 功能未开启页：页签写出是哪个功能（「风控 未开启 - AiFerry」），原来只有笼统的「功能未开启」（2026-10-04 走查）
  const featureName = route.name === FEATURE_OFF_ROUTE_NAME && typeof route.query?.name === 'string' ? route.query.name.trim() : ''
  const featureOffTitle = featureName ? i18n.global.t('admin.featureOff.message', { name: featureName }) : undefined
  const exactTitle = menuTitle || legalTitle || featureOffTitle
  const { titleKey } = resolveRouteMetaKeys(route)

  return resolveDocumentTitle(exactTitle || route.meta.title, siteName, exactTitle ? undefined : titleKey, route.meta.titleBrandFirst === true)
}
