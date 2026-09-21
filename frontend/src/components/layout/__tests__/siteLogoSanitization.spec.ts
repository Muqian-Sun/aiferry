import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const sidebarSource = readFileSync(resolve(dir, '../sidebar/SidebarFrame.vue'), 'utf8')
const homeViewSource = readFileSync(resolve(dir, '../../../views/HomeView.vue'), 'utf8')
// KeyUsageView 的品牌区已收进用户站顶栏 SiteNav
const siteNavSource = readFileSync(resolve(dir, '../../user/shell/SiteNav.vue'), 'utf8')

describe('site_logo sanitization', () => {
  it('SidebarFrame imports sanitizeUrl and applies it to siteLogo', () => {
    expect(sidebarSource).toContain("import { sanitizeUrl } from '@/utils/url'")
    expect(sidebarSource).toContain('sanitizeUrl(appStore.siteLogo')
  })

  it('HomeView applies sanitizeUrl to siteLogo', () => {
    expect(homeViewSource).toContain('sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo')
  })

  it('SiteNav (user-site top bar) applies sanitizeUrl to siteLogo', () => {
    expect(siteNavSource).toContain('sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo')
  })

  it('all three pass allowRelative and allowDataUrl options', () => {
    for (const src of [sidebarSource, homeViewSource, siteNavSource]) {
      expect(src).toContain('allowRelative: true')
      expect(src).toContain('allowDataUrl: true')
    }
  })
})
