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
  /** 这些日期（YYYY-MM-DD，按 timezone 的本地日期）全天按平时：法定节假日 */
  exclude_dates?: string[]
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
  /** 每次 web 搜索；官方价为 null = 不收搜索费（计费只认目录），上游价为 null = 上游不收 */
  search_price_per_call?: number | null
  /** xAI X 搜索按取回条目收：每条帖子、每个主页 */
  x_post_price?: number | null
  x_user_price?: number | null
  /** 最高推理档（effort = max）整单乘的倍数：官方价为 null = 不加价，上游价为 null = 跟官方 */
  max_reasoning_effort_multiplier?: number | null
}

/** 厂商公开的联网搜索价（$/次、$/条）：播种时写进官方价，价格页当参考；X 帖子 / 主页价只有 xAI 有 */
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
  /** 上游成本比（上游价 ÷ 售价口径，逐项、逐段取最高；售价口径 = 定了售价的项按售价 ÷ 默认售价比例，没定的按官方价），与利润门同一个数；没有可比项时为 null。平时的值 */
  cost_ratio: number | null
  /** 一周里最差的上游成本比（上游忙时涨、我们没涨的时段）；不比平时差时为 null */
  peak_cost_ratio: number | null
  /** 最高推理档最差的上游成本比（上游在 max 档比售价涨得多，含忙时）；不比平时差时为 null */
  max_reasoning_cost_ratio: number | null
  /** 上游忙闲时：渠道成本按请求时刻整单 × 倍数；null = 上游不分忙闲时 */
  time_pricing: TimePricing | null
}

/** 价格页上的一个模型（只有按 Token 计费的模型） */
/** 售价的一段：输入侧 Token 落在官方价那一段（下界 min_tokens）时用 */
export interface PricingSaleSegment {
  min_tokens: number
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_write_1h_price: number | null
  cache_read_price: number | null
}

/** 我们自己定的售价（$/token）：五项 + 各段；null = 没单独定，按官方价 × 默认售价比例收 */
export interface PricingSalePrices {
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_write_1h_price: number | null
  cache_read_price: number | null
  segments: PricingSaleSegment[]
  /** 售价忙闲时：不带 / null = 跟官方忙闲时；periods 为空 = 全天一个价；有时段 = 向用户收钱按它整单乘倍数 */
  time_pricing?: TimePricing | null
  /** 售价的最高推理倍率：不带 / null = 跟官方；有值 = effort = max 时整单乘它（1 = 不加价） */
  max_reasoning_effort_multiplier?: number | null
}

export interface PricingEntry extends PricingPrices {
  id: number
  model_id: string
  display_name: string
  vendor: string
  status: 'listed' | 'unlisted'
  /** 厂商公开的搜索价（播种时已写进官方价，这里只作参考）；null = 这个厂商没有官方搜索工具，不填搜索价 */
  search_defaults: PricingSearchDefaults | null
  /** 「联网搜索」计费项：Claude Code 配第三方模型时代执行搜索的模型（它的官方价就是计费项） */
  web_search_delegate?: boolean
  /** 售价（muqian 2026-10-06：每项单独填，没填的按官方价 × default_sale_ratio） */
  sale_prices: PricingSalePrices
  /** 官方忙闲时（目录条目的分时，如 DeepSeek 工作日高峰 × 2）：向用户收钱整单乘倍数；新加承接默认带上它。null = 不分忙闲时 */
  time_pricing: TimePricing | null
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
  /** 默认售价比例：没单独定售价的项 = 官方价 × 它；毛利 = 1 − 上游成本比 ÷ 它 */
  default_sale_ratio: number
  /** 利润门的最低毛利率，0 = 不能亏本（利润门一直开着） */
  min_margin: number
  entries: PricingEntry[]
  accounts: PricingAccount[]
}

/** 按模型保存一块：官方价、售价 + 这个模型的全部承接关系（整份覆盖） */
export interface PricingModelSaveRequest extends PricingPrices {
  sale_prices: PricingSalePrices
  /** 官方忙闲时（null = 不分忙闲时） */
  time_pricing: TimePricing | null
  bindings: Array<PricingPrices & { account_id: number; upstream_model: string; time_pricing: TimePricing | null }>
}

/** 按渠道保存一块：这个渠道承接的全部模型与上游价（整份覆盖） */
export interface PricingChannelSaveRequest {
  bindings: Array<PricingPrices & { entry_id: number; upstream_model: string; time_pricing: TimePricing | null }>
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
