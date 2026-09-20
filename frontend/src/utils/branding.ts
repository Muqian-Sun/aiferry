import { sanitizeUrl } from '@/utils/url'

/** 未配置站名时的产品默认值；管理员在后台配置的 site_name 始终优先。 */
export const DEFAULT_SITE_NAME = 'AiFerry'
/** 默认副标题（登录页品牌区）。 */
export const DEFAULT_SITE_SUBTITLE = 'AI Model API Platform'

export function updateFavicon(logoUrl: string): void {
  const sanitizedLogoUrl = sanitizeUrl(logoUrl, {
    allowRelative: true,
    allowDataUrl: true,
  })
  if (!sanitizedLogoUrl) {
    return
  }

  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    document.head.appendChild(link)
  }

  link.type = sanitizedLogoUrl.endsWith('.svg') ? 'image/svg+xml' : 'image/x-icon'
  link.href = sanitizedLogoUrl
}
