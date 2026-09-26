export const TOKENS_PER_MILLION = 1_000_000

interface TokenPriceFormatOptions {
  fractionDigits?: number
  withCurrencySymbol?: boolean
  emptyValue?: string
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value)
}

export function calculateTokenUnitPrice(
  cost: number | null | undefined,
  tokens: number | null | undefined
): number | null {
  if (!isFiniteNumber(cost) || !isFiniteNumber(tokens) || tokens <= 0) {
    return null
  }

  return cost / tokens
}

export function calculateTokenPricePerMillion(
  cost: number | null | undefined,
  tokens: number | null | undefined
): number | null {
  const unitPrice = calculateTokenUnitPrice(cost, tokens)
  if (unitPrice == null) {
    return null
  }

  return unitPrice * TOKENS_PER_MILLION
}

export function formatTokenPricePerMillion(
  cost: number | null | undefined,
  tokens: number | null | undefined,
  options: TokenPriceFormatOptions = {}
): string {
  const pricePerMillion = calculateTokenPricePerMillion(cost, tokens)
  if (pricePerMillion == null) {
    return options.emptyValue ?? '-'
  }

  const fractionDigits = options.fractionDigits ?? 4
  const formatted = pricePerMillion.toFixed(fractionDigits)
  return options.withCurrencySymbol == false ? formatted : `$${formatted}`
}

/** 账号成本 = total_cost × 账号倍率（渠道统计价卡已删，没有别的基数）。 */
export function accountBilled(row: { total_cost?: number | null; account_rate_multiplier?: number | null }): number {
  const result = (row.total_cost ?? 0) * (row.account_rate_multiplier ?? 1)
  return Number.isNaN(result) ? 0 : result
}
