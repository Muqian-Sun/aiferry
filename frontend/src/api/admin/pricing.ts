/**
 * 价卡类型：模型目录条目与分组价卡共用的定价结构。
 * 对应后端 service/pricing_card.go（PricingCard / TimePricing / PricingInterval）。
 */

import type { BillingMode } from '@/constants/pricing'

export type { BillingMode }

export interface PricingInterval {
  id?: number
  min_tokens: number
  max_tokens: number | null
  tier_label: string
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_write_1h_price?: number | null
  cache_read_price: number | null
  input_multiplier: number | null
  output_multiplier: number | null
  cache_write_multiplier: number | null
  cache_read_multiplier: number | null
  per_request_price: number | null
  sort_order: number
}

export interface TimePricingPeriod {
  start_time: string
  end_time: string
  multiplier: number
}

export interface TimePricing {
  timezone: string
  weekdays_only?: boolean
  periods: TimePricingPeriod[]
}

export interface PricingCard {
  id?: number
  /** 只有渠道实体在用，PR-5b 随渠道删。 */
  platform: string
  models: string[]
  billing_mode: BillingMode
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_write_1h_price?: number | null
  cache_read_price: number | null
  fast_multiplier?: number | null
  flex_multiplier?: number | null
  max_reasoning_effort_multiplier?: number | null
  image_input_price: number | null
  image_output_price: number | null
  per_request_price: number | null
  intervals: PricingInterval[]
  time_pricing: TimePricing | null
}
