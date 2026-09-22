/**
 * Model Plaza API（公开端点，对所有人开放，没有开关）。
 * 平铺的上架模型目录：每个条目带目录标价（USD / token）、计费模式、分时倍率与别名。
 * 用户价 = 标价 × 用户倍率（`User.rate_multiplier`），倍率不在这个接口里。
 */

import { apiClient } from './client'
import type { BillingMode } from '@/constants/pricing'

/** 阶梯 / 分档：token 模式按 context 区间，按次模式按 tier_label；单价可给绝对值或相对基础价倍率。 */
export interface UserPricingInterval {
  min_tokens: number
  max_tokens: number | null
  tier_label?: string
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_write_1h_price?: number | null
  cache_read_price: number | null
  input_multiplier?: number | null
  output_multiplier?: number | null
  cache_write_multiplier?: number | null
  cache_read_multiplier?: number | null
  per_request_price: number | null
}

/** 用户侧最小形态的目录标价（/model-plaza 的 pricing，USD / token）。 */
export interface UserSupportedModelPricing {
  billing_mode: BillingMode
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_write_1h_price?: number | null
  cache_read_price: number | null
  max_reasoning_effort_multiplier?: number | null
  image_input_price: number | null
  image_output_price: number | null
  per_request_price: number | null
  search_price_per_call?: number | null
  intervals: UserPricingInterval[]
}

/** 分时倍率时段：配置时区当天 [start_time, end_time) 内整单实付乘 multiplier。 */
export interface PlazaTimePricingPeriod {
  start_time: string
  end_time: string
  multiplier: number
}

/** 计费会生效的分时倍率（仅倍率 ≠ 1 的时段，已按开始时间升序）。 */
export interface PlazaTimePricing {
  /** IANA 时区名，如 Asia/Shanghai。 */
  timezone: string
  /** true 时时段仅周一至周五生效，周末整天按标准价计费。 */
  weekdays_only?: boolean
  periods: PlazaTimePricingPeriod[]
}

/** 一个上架的目录条目。 */
export interface PlazaModel {
  model_id: string
  display_name: string
  /** 厂商标签（anthropic / openai / gemini / …），来自目录，不猜模型名。 */
  vendor: string
  /** token / per_request / image / video；缺省视为 token。 */
  billing_mode: string
  /** 目录标价；上架必有价，但字段可为 null（如按次模式没有 token 价）。 */
  pricing: UserSupportedModelPricing | null
  /** 仅配置了分时倍率的模型返回。 */
  time_pricing?: PlazaTimePricing
  /** 别名（可用别名调用，计费按主 model_id）。 */
  aliases: string[]
}

export interface ModelPlazaResponse {
  /** 管理员配置的全局价格说明（Markdown）。 */
  description: string
  models: PlazaModel[]
}

/** 获取模型广场数据（匿名可访问）。 */
export async function getModelPlaza(options?: { signal?: AbortSignal }): Promise<ModelPlazaResponse> {
  const { data } = await apiClient.get<ModelPlazaResponse>('/model-plaza', {
    signal: options?.signal
  })
  return data
}

export const modelPlazaAPI = { getModelPlaza }

export default modelPlazaAPI
