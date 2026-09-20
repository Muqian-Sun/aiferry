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
import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs'
import { dirname, join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

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
  'components/common/LocaleSwitcher.vue'
]
/** 另一工作流负责删除的页面与组件，不纳入本守卫。 */
const EXCLUDE_PREFIXES = [
  'components/user/monitor/',
  'components/user/MonitorDetailDialog.vue',
  'views/user/ChannelStatus',
  'views/user/AvailableChannelsView.vue'
]

/** 禁用模式与含义（只扫 class 属性、:class 表达式与 <style> 里的 @apply）。 */
const FORBIDDEN: Array<{ name: string; re: RegExp }> = [
  { name: 'legacy gray/slate palette', re: /\b(?:bg|text|border|divide|ring|from|to|via|placeholder|accent|fill|stroke|outline)-(?:gray|slate|zinc|neutral|stone)-\d{2,3}\b/ },
  { name: 'legacy dark palette', re: /\b(?:bg|text|border|divide|ring|from|to|via|placeholder)-dark-\d{2,3}\b/ },
  { name: 'dark: variant (tokens switch themselves)', re: /(?:^|[\s"'`(:])dark:[a-z]/ },
  { name: 'card class', re: /(?:^|[\s"'`])card(?:-glass|-hover|-header|-body|-footer)?(?=$|[\s"'`])/ },
  { name: 'rounded-2xl / rounded-3xl', re: /\brounded-(?:2xl|3xl|4xl)\b/ },
  { name: 'gradient', re: /\b(?:bg-gradient-to-\w+|bg-mesh-gradient|text-gradient|gradient-primary|gradient-dark)\b/ },
  { name: 'glass / glow shadows', re: /\b(?:glass(?:-card)?|shadow-glow(?:-lg)?|shadow-glass(?:-sm)?|shadow-card(?:-hover)?)\b/ },
  { name: 'all-caps label', re: /\buppercase\b[^"'`]*\btracking-/ },
  { name: 'hover lift', re: /hover:-translate-y/ }
]

/**
 * 棘轮白名单：尚未重写的旧文件。每个 PR 只允许删条目，不允许加。
 * S1 时点：所有旧用户页与用户组件；已 token 化的共享控件不在名单里。
 */
const ALLOWLIST = new Set<string>([
  // views
  'views/user/AffiliateView.vue',
  'views/user/AirwallexPaymentView.vue',
  'views/user/BatchImageGuideView.vue',
  'views/user/CustomPageView.vue',
  'views/user/KeysView.vue',
  'views/user/PaymentQRCodeView.vue',
  'views/user/PaymentResultView.vue',
  'views/user/PaymentView.vue',
  'views/user/RedeemView.vue',
  'views/user/StripePaymentView.vue',
  'views/user/StripePopupView.vue',
  'views/user/SubscriptionsView.vue',
  'views/user/UserOrdersView.vue',
  // layout
  // components
  'components/keys/BulkEditKeysModal.vue',
  'components/keys/EndpointPopover.vue',
  'components/keys/UseKeyModal.vue',
  'components/modelPlaza/ModelPlazaContent.vue',
  'components/modelPlaza/PlazaFilterBar.vue',
  'components/modelPlaza/PlazaGroupSection.vue',
  'components/modelPlaza/PlazaModelPricingTable.vue',
  'components/payment/AmountInput.vue',
  'components/payment/OrderTable.vue',
  'components/payment/PaymentMethodSelector.vue',
  'components/payment/PaymentProviderDialog.vue',
  'components/payment/PaymentProviderList.vue',
  'components/payment/PaymentQRDialog.vue',
  'components/payment/PaymentStatusPanel.vue',
  'components/payment/ProviderCard.vue',
  'components/payment/StripePaymentInline.vue',
  'components/payment/SubscriptionPlanCard.vue',
  'components/payment/ToggleSwitch.vue',
  'components/usage/UsageStatsCards.vue',
  'components/usage/UsageTable.vue',
  'components/user/PlatformCostCell.vue',
  'components/user/PlatformUsageBreakdown.vue',
  'components/user/UserAttributeForm.vue',
  'components/user/UserAttributesConfigModal.vue',
  'components/user/UserConcurrencyCell.vue',
  'components/user/UserErrorDetailModal.vue',
  'components/user/UserErrorRequestsTable.vue',
  'components/user/profile/ProfileAvatarCard.vue',
  'components/user/profile/ProfileBalanceNotifyCard.vue',
  'components/user/profile/ProfileEditForm.vue',
  'components/user/profile/ProfileIdentityBindingsSection.vue',
  'components/user/profile/ProfileInfoCard.vue',
  'components/user/profile/ProfilePasskeyCard.vue',
  'components/user/profile/ProfilePasswordForm.vue',
  'components/user/profile/ProfileTotpCard.vue',
  'components/user/profile/TotpDisableDialog.vue',
  'components/user/profile/TotpSetupModal.vue'
])

function walk(dir: string, out: string[]): void {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name)
    if (name === '__tests__') continue
    const st = statSync(full)
    if (st.isDirectory()) walk(full, out)
    else if (name.endsWith('.vue')) out.push(full)
  }
}

function collectFiles(): string[] {
  const files: string[] = []
  for (const dir of SCAN_DIRS) {
    const full = resolve(SRC, dir)
    if (!existsSync(full)) throw new Error(`scan dir missing: ${dir}`)
    walk(full, files)
  }
  for (const file of SCAN_FILES) {
    const full = resolve(SRC, file)
    if (!existsSync(full)) throw new Error(`scan file missing: ${file}`)
    files.push(full)
  }
  const rel = files.map((f) => relative(SRC, f))
  return [...new Set(rel)].filter((f) => !EXCLUDE_PREFIXES.some((p) => f.startsWith(p))).sort()
}

/** 只取 class 属性、:class 绑定表达式与 <style> 块里的 @apply，避免误伤脚本字符串或 SVG 颜色。 */
function styleSurfaces(source: string): string[] {
  const surfaces: string[] = []
  const template = source.match(/<template\b[^>]*>([\s\S]*)<\/template>/)?.[1] ?? ''
  for (const m of template.matchAll(/(?:^|\s)(?::class|class|v-bind:class)\s*=\s*("([^"]*)"|'([^']*)')/g)) {
    surfaces.push(m[2] ?? m[3] ?? '')
  }
  for (const m of source.matchAll(/<style\b[^>]*>([\s\S]*?)<\/style>/g)) {
    for (const apply of m[1].matchAll(/@apply\s+([^;]+);/g)) surfaces.push(apply[1])
  }
  return surfaces
}

function violationsOf(file: string): string[] {
  const source = readFileSync(resolve(SRC, file), 'utf8')
  const hits: string[] = []
  for (const surface of styleSurfaces(source)) {
    for (const rule of FORBIDDEN) {
      const m = surface.match(rule.re)
      if (m) hits.push(`${rule.name}: …${m[0]}…`)
    }
  }
  return hits
}

describe('user-site templates use design tokens only', () => {
  const files = collectFiles()

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
