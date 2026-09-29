/**
 * 管理站壳的模板守卫（A0）：壳组件只允许 token 工具类与 style.css 原语；
 * 管理站的 card 是 token 化的分区容器，所以规则里不禁 card。
 *
 * 扫描集只有壳（AppLayout / AppHeader / SidebarFrame / AdminSidebar / TablePageLayout）与已重做的管理页。
 * 全站旧调色类由 scripts/codemod-af-tokens.mjs --check 盯住（pnpm check:admin-tokens，已接进 build），不在这里配白名单。
 * 另外盯住 style.css：壳用到的原语（card / sidebar / page / table / tour）不能再带旧调色。
 */
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { FORBIDDEN_ADMIN, collectFiles, violationsOf } from '@/__tests__/helpers/templateTokens'

const SRC = resolve(dirname(fileURLToPath(import.meta.url)), '../../..')

const SCAN_FILES = [
  'components/layout/AppLayout.vue',
  'components/layout/AppHeader.vue',
  'components/layout/TablePageLayout.vue',
  'components/layout/sidebar/SidebarFrame.vue',
  'components/admin/layout/AdminSidebar.vue',
  // A2：已重做的管理页与它的组件，一并盯住
  'views/admin/ModelCatalogView.vue',
  'components/admin/catalog/CatalogEntryEditor.vue',
  'components/admin/catalog/CatalogChannelPicker.vue',
  'components/admin/catalog/PriceInput.vue',
  'components/admin/catalog/CatalogPriceCell.vue',
  'components/admin/catalog/CatalogEntryDiagnosisModal.vue'
]

/** style.css 里壳依赖的原语块名；每块的 @apply 都不能带旧调色 */
const STYLE_BLOCKS = ['card', 'card-hover', 'card-header', 'card-body', 'card-footer', 'stat-card', 'sidebar', 'sidebar-header', 'sidebar-nav', 'sidebar-link', 'sidebar-link-active', 'sidebar-section-title', 'page-header', 'page-title', 'page-description', 'table-container', 'table th', 'table td', 'empty-state-icon', 'empty-state-title', 'empty-state-description', 'tour-step-description', 'tour-info-box', 'tour-success-box', 'tour-warning-box', 'tour-error-box']

function styleBlock(css: string, selector: string): string {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const m = css.match(new RegExp(`\\n {2}\\.${escaped} \\{([\\s\\S]*?)\\n {2}\\}`))
  if (!m) throw new Error(`style block missing: .${selector}`)
  return m[1]
}

describe('admin shell templates use design tokens only', () => {
  const files = collectFiles(SRC, [], SCAN_FILES)

  it('scans exactly the shell files (fail-closed)', () => {
    expect(files).toEqual([...SCAN_FILES].sort())
  })

  it('shell components carry no legacy decoration classes', () => {
    const report: Record<string, string[]> = {}
    for (const file of files) {
      const hits = violationsOf(SRC, file, FORBIDDEN_ADMIN)
      if (hits.length) report[file] = hits
    }
    expect(report).toEqual({})
  })

  it('the shell primitives in style.css are token-only and the glass helpers are gone', () => {
    const css = readFileSync(resolve(SRC, 'style.css'), 'utf8')
    const report: Record<string, string[]> = {}
    for (const block of STYLE_BLOCKS) {
      const body = styleBlock(css, block)
      const hits: string[] = []
      for (const apply of body.matchAll(/@apply\s+([^;]+);/g)) {
        for (const rule of FORBIDDEN_ADMIN) {
          const m = apply[1].match(rule.re)
          if (m) hits.push(`${rule.name}: …${m[0]}…`)
        }
      }
      if (hits.length) report[block] = hits
    }
    expect(report).toEqual({})
    expect(css).not.toMatch(/\n {2}\.glass(?:-card)? \{/)
    expect(css).not.toMatch(/\n {2}\.card-glass \{/)
  })
})
