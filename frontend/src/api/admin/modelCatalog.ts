/**
 * Admin model catalog API.
 * Backend: GET/POST/PUT/DELETE /admin/model-catalog/...
 */

import { apiClient } from '../client'
import type { TimePricing, PricingInterval } from './pricing'

export type { TimePricing, PricingInterval }

export interface ModelCatalogAlias {
  id: number
  entry_id: number
  alias: string
  source: 'manual' | 'seed' | string
  notes?: string
  created_at: string
  updated_at: string
}

export interface ModelCatalogEntry {
  id: number
  model_id: string
  display_name: string
  vendor: string
  protocols: string[]
  billing_mode: string
  status: 'listed' | 'unlisted' | string
  managed_by: 'seed' | 'admin' | string
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_write_1h_price: number | null
  cache_read_price: number | null
  image_input_price: number | null
  image_output_price: number | null
  image_cache_read_price: number | null
  /** 音频 Token 单价：较新的后端才有；旧后端不返回这两个字段 */
  audio_input_price?: number | null
  audio_output_price?: number | null
  per_request_price: number | null
  /** 联网搜索官方价（$/次、$/条）：在价格页改；X 帖子 / 主页只有 xAI 模型有 */
  search_price_per_call: number | null
  x_post_price?: number | null
  x_user_price?: number | null
  max_reasoning_effort_multiplier: number | null
  notes?: string
  intervals: PricingInterval[]
  time_pricing?: TimePricing | null
  aliases: ModelCatalogAlias[]
  /** 绑定的资源（账号）；上架条目由这些账号承接请求。 */
  bindings: ModelCatalogBinding[]
  /** 厂商族（与渠道平台同一套标识，认不出为空串）：只有列表接口带。 */
  vendor_platform?: string
  /** 经扩展端点（生图 / 视频 / 向量）承接：只有列表接口带。 */
  extension_endpoints?: boolean
  created_at: string
  updated_at: string
}

/**
 * 渠道 × 模型的绑定，带这个渠道给这个模型的上游价（美元 / Token，与目录条目的价同单位；intervals 是按 Token 分段的上游价）。
 * 渠道成本 = 用量 × 上游价，由后端逐条算进用量记录的 account_cost。
 */
export interface ModelCatalogBinding {
  entry_id: number
  account_id: number
  input_price: number
  output_price: number
  cache_write_price: number | null
  cache_write_1h_price: number | null
  cache_read_price: number | null
  intervals: PricingInterval[]
  created_at: string
  updated_at: string
}

export interface ModelCatalogBindingAccount {
  id: number
  name: string
  platform: string
  type: string
  vendor: string
  status: string
}

/** 诊断接口按入站协议逐格报告资源能否承接。 */
export type ModelCatalogInboundProtocol = 'anthropic' | 'chat_completions' | 'responses' | 'gemini'

export interface ModelCatalogDiagnosisAccount extends ModelCatalogBindingAccount {
  /** 此刻能否进入调度；不能时 blocked_reason 给第一个原因。 */
  schedulable: boolean
  blocked_reason?: string
  serves: Record<ModelCatalogInboundProtocol, boolean>
}

export interface ModelCatalogDiagnosis {
  entry_id: number
  accounts: ModelCatalogDiagnosisAccount[]
}

export interface ModelCatalogEntryRequest {
  model_id: string
  display_name?: string
  vendor?: string
  protocols?: string[]
  billing_mode?: string
  status?: string
  input_price?: number | null
  output_price?: number | null
  cache_write_price?: number | null
  cache_write_1h_price?: number | null
  cache_read_price?: number | null
  image_input_price?: number | null
  image_output_price?: number | null
  image_cache_read_price?: number | null
  /** 只在有值时发送：旧后端不认识这两个字段，会直接忽略 */
  audio_input_price?: number | null
  audio_output_price?: number | null
  per_request_price?: number | null
  search_price_per_call?: number | null
  x_post_price?: number | null
  x_user_price?: number | null
  max_reasoning_effort_multiplier?: number | null
  notes?: string | null
  intervals?: PricingInterval[]
  time_pricing?: TimePricing | null
}

export interface ModelCatalogSeedResult {
  inserted: number
  refreshed: number
  skipped_admin: number
  skipped_invalid: number
  /** 写库失败的条目数；单条失败不拖垮整批，前几条原因在 errors 里。 */
  failed: number
  errors?: string[]
}

export interface ModelCatalogAliasRequest {
  alias: string
  entry_id: number
  notes?: string | null
}

const modelCatalogAPI = {
  listEntries: async (): Promise<ModelCatalogEntry[]> => {
    const { data } = await apiClient.get<ModelCatalogEntry[]>('/admin/model-catalog/entries')
    return data ?? []
  },
  getEntry: async (id: number): Promise<ModelCatalogEntry> => {
    const { data } = await apiClient.get<ModelCatalogEntry>(`/admin/model-catalog/entries/${id}`)
    return data
  },
  createEntry: async (body: ModelCatalogEntryRequest): Promise<ModelCatalogEntry> => {
    const { data } = await apiClient.post<ModelCatalogEntry>('/admin/model-catalog/entries', body)
    return data
  },
  updateEntry: async (id: number, body: ModelCatalogEntryRequest): Promise<ModelCatalogEntry> => {
    const { data } = await apiClient.put<ModelCatalogEntry>(`/admin/model-catalog/entries/${id}`, body)
    return data
  },
  deleteEntry: async (id: number): Promise<void> => {
    await apiClient.delete(`/admin/model-catalog/entries/${id}`)
  },
  createAlias: async (body: ModelCatalogAliasRequest): Promise<ModelCatalogAlias> => {
    const { data } = await apiClient.post<ModelCatalogAlias>('/admin/model-catalog/aliases', body)
    return data
  },
  deleteAlias: async (id: number): Promise<void> => {
    await apiClient.delete(`/admin/model-catalog/aliases/${id}`)
  },
  seed: async (): Promise<ModelCatalogSeedResult> => {
    const { data } = await apiClient.post<ModelCatalogSeedResult>('/admin/model-catalog/seed')
    return data
  },
  diagnose: async (id: number): Promise<ModelCatalogDiagnosis> => {
    const { data } = await apiClient.get<ModelCatalogDiagnosis>(`/admin/model-catalog/entries/${id}/diagnosis`)
    return data
  },
  /** 按模型 ID 从价格文件带出厂商与价格；找不到返回 null。 */
  priceLookup: async (modelId: string): Promise<ModelCatalogEntry | null> => {
    const { data } = await apiClient.get<{ found: boolean; entry?: ModelCatalogEntry }>('/admin/model-catalog/price-lookup', {
      params: { model_id: modelId }
    })
    return data?.found && data.entry ? data.entry : null
  }
}

export default modelCatalogAPI
