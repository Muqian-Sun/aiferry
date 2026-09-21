import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const headerSource = readFileSync(resolve(dir, '../AppHeader.vue'), 'utf8')
// HomeView 的文档链接已收进用户站顶栏 SiteNav（见下方用例）
// KeyUsageView 的文档链接已收进用户站顶栏 SiteNav
const siteNavSource = readFileSync(resolve(dir, '../../user/shell/SiteNav.vue'), 'utf8')

describe('doc_url sanitization', () => {
  it('AppHeader imports sanitizeUrl', () => {
    expect(headerSource).toContain("import { sanitizeUrl } from '@/utils/url'")
  })

  it('AppHeader applies sanitizeUrl to docUrl', () => {
    expect(headerSource).toContain('sanitizeUrl(appStore.docUrl)')
  })

  it('SiteNav imports sanitizeUrl and applies it to docUrl', () => {
    expect(siteNavSource).toContain("import { sanitizeUrl } from '@/utils/url'")
    expect(siteNavSource).toContain('sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl')
  })
})
