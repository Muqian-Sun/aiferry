/**
 * 管理站金额的唯一格式（方案 2026-09-25「金额只剩三个数」）。
 *
 * 全站只显示三种钱：收入（actual_cost，向用户收的）、成本（付给渠道的：标价 × 渠道成本倍率）、
 * 利润（收入 − 成本，为负标红）。这里是它们唯一的格式化入口，页面里不要再各写一份。
 */

const moneyFormatter = new Intl.NumberFormat('en-US', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2
})

/**
 * 汇总金额：美元、两位小数、千分位；0 < |x| < 0.01 写 `<$0.01`（负数 `-<$0.01`），空值写 `$0.00`。
 * 例：1234.5 → `$1,234.50`；-3.2 → `-$3.20`；0.004 → `<$0.01`。
 */
export function formatMoney(amount: number | null | undefined): string {
  const value = Number(amount)
  if (!Number.isFinite(value) || value === 0) return '$0.00'
  const sign = value < 0 ? '-' : ''
  const abs = Math.abs(value)
  if (abs < 0.01) return `${sign}<$0.01`
  return `${sign}$${moneyFormatter.format(abs)}`
}

/**
 * 单笔请求的精确金额（明细详情里的费用构成）：美元、最多 6 位小数、去掉末尾多余的 0，至少两位。
 * 例：0.000123 → `$0.000123`；0.5 → `$0.50`。汇总数字一律用 formatMoney。
 */
export function formatMoneyExact(amount: number | null | undefined): string {
  const value = Number(amount)
  if (!Number.isFinite(value) || value === 0) return '$0.00'
  const sign = value < 0 ? '-' : ''
  let text = Math.abs(value).toFixed(6).replace(/0+$/, '')
  const decimals = text.split('.')[1] ?? ''
  if (decimals.length < 2) text = Math.abs(value).toFixed(2)
  return `${sign}$${text}`
}

/**
 * 余额：正数同汇总金额（formatMoney）；为负是透支的欠款（允许一次请求透支，补上之前不能再用，
 * muqian 2026-09-29），写精确值，不能写成 `-<$0.01` 看不出欠多少。
 */
export function formatBalance(amount: number | null | undefined): string {
  return Number(amount) < 0 ? formatMoneyExact(amount) : formatMoney(amount)
}

/** 余额为负（欠款）时的文字色。 */
export function balanceTextClass(amount: number | null | undefined): string {
  return Number(amount) < 0 ? 'text-af-danger' : ''
}

/** 利润 = 收入 − 成本（两者任一缺失按 0）。 */
export function profitOf(revenue: number | null | undefined, cost: number | null | undefined): number {
  return (Number(revenue) || 0) - (Number(cost) || 0)
}

/** 利润为负时的文字色（状态色只用在这里，正数不上色）。 */
export function profitTextClass(profit: number | null | undefined): string {
  return Number(profit) < 0 ? 'text-af-danger' : ''
}
