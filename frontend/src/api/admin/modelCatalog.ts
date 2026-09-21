/**
 * Admin model catalog API.
 * Backend: GET/POST/PUT/DELETE /admin/model-catalog/...
 */

import { apiClient } from '../client'
import type { ChannelTimePricing, PricingInterval } from './channels'

export type { ChannelTimePricing, PricingInterval }

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
  /** 条目走哪条网关族；空串表示按 vendor 推导。 */
  route_platform: string
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_write_1h_price: number | null
  cache_read_price: number | null
  image_input_price: number | null
  image_output_price: number | null
  image_cache_read_price: number | null
  input_price_priority: number | null
  output_price_priority: number | null
  cache_write_price_priority: number | null
  cache_read_price_priority: number | null
  per_request_price: number | null
  long_context_input_threshold: number | null
  long_context_threshold_inclusive: boolean
  long_context_input_multiplier: number | null
  long_context_output_multiplier: number | null
  fast_multiplier: number | null
  flex_multiplier: number | null
  max_reasoning_effort_multiplier: number | null
  notes?: string
  intervals: PricingInterval[]
  time_pricing?: ChannelTimePricing | null
  aliases: ModelCatalogAlias[]
  /** 绑定的资源（账号）；上架条目由这些账号承接请求。 */
  bindings: ModelCatalogBinding[]
  created_at: string
  updated_at: string
}

export interface ModelCatalogBinding {
  entry_id: number
  account_id: number
  /** 绑定优先级；null 表示跟随账号自身的优先级。 */
  priority: number | null
  account?: ModelCatalogBindingAccount
}

export interface ModelCatalogBindingAccount {
  id: number
  name: string
  platform: string
  type: string
  vendor: string
  status: string
}

export interface ModelCatalogBindingRequestItem {
  account_id: number
  priority: number | null
}

export interface ModelCatalogEntryRequest {
  model_id: string
  display_name?: string
  vendor?: string
  protocols?: string[]
  billing_mode?: string
  status?: string
  route_platform?: string
  input_price?: number | null
  output_price?: number | null
  cache_write_price?: number | null
  cache_write_1h_price?: number | null
  cache_read_price?: number | null
  image_input_price?: number | null
  image_output_price?: number | null
  image_cache_read_price?: number | null
  input_price_priority?: number | null
  output_price_priority?: number | null
  cache_write_price_priority?: number | null
  cache_read_price_priority?: number | null
  per_request_price?: number | null
  long_context_input_threshold?: number | null
  long_context_threshold_inclusive?: boolean
  long_context_input_multiplier?: number | null
  long_context_output_multiplier?: number | null
  fast_multiplier?: number | null
  flex_multiplier?: number | null
  max_reasoning_effort_multiplier?: number | null
  notes?: string | null
  intervals?: PricingInterval[]
  time_pricing?: ChannelTimePricing | null
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
  getBindings: async (id: number): Promise<ModelCatalogBinding[]> => {
    const { data } = await apiClient.get<ModelCatalogBinding[]>(`/admin/model-catalog/entries/${id}/bindings`)
    return data ?? []
  },
  updateBindings: async (id: number, bindings: ModelCatalogBindingRequestItem[]): Promise<ModelCatalogBinding[]> => {
    const { data } = await apiClient.put<ModelCatalogBinding[]>(`/admin/model-catalog/entries/${id}/bindings`, { bindings })
    return data ?? []
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
  }
}

export default modelCatalogAPI
