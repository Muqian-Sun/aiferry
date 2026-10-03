/**
 * 价卡类型：模型目录条目与分组价卡共用的定价结构。
 * 对应后端 service/pricing_card.go（PricingCard / TimePricing / PricingInterval）。
 *
 * 以及价格页（管理站「供给 › 价格」）的读写接口：官方价与上游价都在价格页改，给模型加一个渠道就是承接；
 * 按模型 / 按渠道整块保存（后端 handler/admin/model_catalog_pricing_handler.go）。单价一律 $/token。
 */

import { apiClient } from '../client'
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
  max_reasoning_effort_multiplier?: number | null
  image_input_price: number | null
  image_output_price: number | null
  per_request_price: number | null
  intervals: PricingInterval[]
  time_pricing: TimePricing | null
}

/** 五项 token 价（$/token）、按 Token 分段与联网搜索价（$/次、$/条） */
export interface PricingPrices {
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_write_1h_price: number | null
  cache_read_price: number | null
  intervals: PricingInterval[]
  /** 每次 web 搜索；官方价为 null = 按厂商公开价收，上游价为 null = 按官方搜索价记成本 */
  search_price_per_call?: number | null
  /** xAI X 搜索按取回条目收：每条帖子、每个主页 */
  x_post_price?: number | null
  x_user_price?: number | null
}

/** 厂商公开的联网搜索价（$/次、$/条），官方价没设时按它收；X 帖子 / 主页价只有 xAI 有 */
export interface PricingSearchDefaults {
  search_price_per_call: number
  x_post_price: number | null
  x_user_price: number | null
}

/** 一条承接关系的上游模型名与上游价 */
export interface PricingBinding extends PricingPrices {
  entry_id: number
  account_id: number
  /** 这个渠道给这个模型用的上游模型名，空 = 与目录模型标识同名 */
  upstream_model: string
  input_price: number
  output_price: number
  /** 上游成本比（上游价 ÷ 官方价，逐项、逐段取最高），与利润门同一个数；官方价没有可比项时为 null */
  cost_ratio: number | null
}

/** 价格页上的一个模型（只有按 Token 计费的模型） */
export interface PricingEntry extends PricingPrices {
  id: number
  model_id: string
  display_name: string
  vendor: string
  status: 'listed' | 'unlisted'
  /** 厂商公开的搜索价；null = 这个厂商没有官方搜索工具，不填搜索价 */
  search_defaults: PricingSearchDefaults | null
  bindings: PricingBinding[]
  /** 能承接这个模型的渠道 */
  bindable_account_ids: number[]
}

/** 价格页上的一个渠道 */
export interface PricingAccount {
  id: number
  name: string
  platform: string
  type: string
  vendor: string
  status: string
  schedulable: boolean
  priority: number
  /** 第三方 key 的上游协议；成品号为空 */
  protocol: string
  /** 第三方 key 上游地址的主机名；成品号为空 */
  upstream_host: string
}

export interface PricingOverview {
  /** 默认售价倍率：售价 = 官方价 × 它；毛利按它算 */
  default_user_rate: number
  /** 利润门的最低毛利率，0 = 关闭 */
  min_margin: number
  entries: PricingEntry[]
  accounts: PricingAccount[]
}

/** 按模型保存一块：官方价 + 这个模型的全部承接关系（整份覆盖） */
export interface PricingModelSaveRequest extends PricingPrices {
  bindings: Array<PricingPrices & { account_id: number; upstream_model: string }>
}

/** 按渠道保存一块：这个渠道承接的全部模型与上游价（整份覆盖） */
export interface PricingChannelSaveRequest {
  bindings: Array<PricingPrices & { entry_id: number; upstream_model: string }>
}

export const pricingAPI = {
  overview: async (): Promise<PricingOverview> => {
    const { data } = await apiClient.get<PricingOverview>('/admin/pricing')
    return data
  },
  saveModel: async (entryId: number, body: PricingModelSaveRequest): Promise<PricingEntry> => {
    const { data } = await apiClient.put<PricingEntry>(`/admin/pricing/models/${entryId}`, body)
    return data
  },
  saveChannel: async (accountId: number, body: PricingChannelSaveRequest): Promise<PricingBinding[]> => {
    const { data } = await apiClient.put<PricingBinding[]>(`/admin/pricing/channels/${accountId}`, body)
    return data
  }
}

export default pricingAPI
