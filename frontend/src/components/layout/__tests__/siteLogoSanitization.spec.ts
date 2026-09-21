import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const sidebarSource = readFileSync(resolve(dir, '../sidebar/SidebarFrame.vue'), 'utf8')
const homeViewSource = readFileSync(resolve(dir, '../../../views/HomeView.vue'), 'utf8')
const keyUsageViewSource = readFileSync(resolve(dir, '../../../views/KeyUsageView.vue'), 'utf8')
const siteNavSource = readFileSync(resolve(dir, '../../user/shell/SiteNav.vue'), 'utf8')

describe('site_logo sanitization', () => {
  it('SidebarFrame imports sanitizeUrl and applies it to siteLogo', () => {
    expect(sidebarSource).toContain("import { sanitizeUrl } from '@/utils/url'")
    expect(sidebarSource).toContain('sanitizeUrl(appStore.siteLogo')
  })

  it('HomeView applies sanitizeUrl to siteLogo', () => {
    expect(homeViewSource).toContain('sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo')
  })

  it('KeyUsageView applies sanitizeUrl to siteLogo', () => {
    expect(keyUsageViewSource).toContain('sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo')
  })

  it('SiteNav (user-site top bar) applies sanitizeUrl to siteLogo', () => {
    expect(siteNavSource).toContain('sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo')
  })

  it('all four pass allowRelative and allowDataUrl options', () => {
    for (const src of [sidebarSource, homeViewSource, keyUsageViewSource, siteNavSource]) {
      expect(src).toContain('allowRelative: true')
      expect(src).toContain('allowDataUrl: true')
    }
  })
})
