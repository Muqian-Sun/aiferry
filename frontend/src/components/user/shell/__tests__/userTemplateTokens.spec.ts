/**
 * 用户站模板守卫：新设计只允许 token 工具类（*-af-*）与 style.css 一层原语。
 *
 * 上一轮重构失败的直接原因之一，是旧的 gray 色阶、dark: 变体、card、rounded-2xl 这些装饰类
 * 在模板里残留了 74%，再用一层覆盖 CSS 去"中和"。这里把禁令写成测试：扫描用户站模板与样式块，命中即红。
 *
 * 棘轮式白名单：ALLOWLIST 里的文件是"还没重写"的旧页面。名单只能缩小——
 * 一个文件一旦变干净就必须从名单里删掉，否则本测试失败（防止名单变成永久豁免）。
 * 名单里不存在的文件、扫描不到任何文件，也都判失败（fail-closed）。
 */
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { FORBIDDEN, collectFiles, violationsOf as violationsIn } from '@/__tests__/helpers/templateTokens'

const SRC = resolve(dirname(fileURLToPath(import.meta.url)), '../../../..')

/** 扫描范围：用户站页面、用户站组件、已 token 化的共享控件。目录递归，跳过 __tests__。 */
const SCAN_DIRS = [
  'views/user',
  'views/auth',
  'views/public',
  'components/user',
  'components/keys',
  'components/payment',
  'components/usage',
  'components/modelPlaza',
  'components/auth'
]
const SCAN_FILES = [
  'components/common/Select.vue',
  'components/common/BaseDialog.vue',
  'components/common/ConfirmDialog.vue',
  'components/common/DataTable.vue',
  'components/common/Pagination.vue',
  'components/common/DateRangePicker.vue',
  'components/common/Toast.vue',
  'components/common/LocaleSwitcher.vue',
  'components/common/SearchInput.vue',
  'components/common/AnnouncementBell.vue',
  'components/common/AnnouncementPopup.vue',
  'components/common/EmptyState.vue',
  'components/common/LoadingSpinner.vue',
  'components/common/VendorIcon.vue',
  'components/common/ModelIcon.vue'
]
/** 另一工作流负责删除的页面与组件，不纳入本守卫。 */
const EXCLUDE_PREFIXES = [
  'components/user/monitor/',
  'components/user/MonitorDetailDialog.vue',
  'views/user/ChannelStatus'
]

/**
 * 棘轮白名单：尚未重写的旧文件。每个 PR 只允许删条目，不允许加。
 * S5 时点：只剩 BatchImageGuideView。
 */
const ALLOWLIST = new Set<string>([
  // 批量生图指南：2.7k 行的富文本教程页，内容不在本轮重构范围，只换了壳（长期唯一例外）
  'views/user/BatchImageGuideView.vue'
])

function violationsOf(file: string): string[] {
  return violationsIn(SRC, file, FORBIDDEN)
}

describe('user-site templates use design tokens only', () => {
  const files = collectFiles(SRC, SCAN_DIRS, SCAN_FILES, EXCLUDE_PREFIXES)

  it('scans a non-empty file set (fail-closed)', () => {
    expect(files.length).toBeGreaterThan(20)
  })

  it('every allowlisted file exists and is scanned', () => {
    const scanned = new Set(files)
    const missing = [...ALLOWLIST].filter((f) => !scanned.has(f))
    expect(missing, 'allowlist entries not in scan set').toEqual([])
  })

  it('files outside the allowlist carry no legacy decoration classes', () => {
    const report: Record<string, string[]> = {}
    for (const file of files) {
      if (ALLOWLIST.has(file)) continue
      const hits = violationsOf(file)
      if (hits.length) report[file] = hits
    }
    expect(report).toEqual({})
  })

  it('allowlist ratchet: a file that became clean must leave the allowlist', () => {
    const stale = [...ALLOWLIST].filter((f) => violationsOf(f).length === 0)
    expect(stale, 'remove these from ALLOWLIST').toEqual([])
  })
})
