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

/** 「探测模型」拿到的一个上游模型，以及它在目录里对上的条目 */
export interface ProbedUpstreamModel {
  id: string
  entry_id?: number
  entry_model_id?: string
  listed?: boolean
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
  search_price_per_call: number | null
  max_reasoning_effort_multiplier: number | null
  notes?: string
  intervals: PricingInterval[]
  time_pricing?: TimePricing | null
  aliases: ModelCatalogAlias[]
  /** 绑定的资源（账号）；上架条目由这些账号承接请求。 */
  bindings: ModelCatalogBinding[]
  /** 厂商族（与渠道平台同一套标识，认不出为空串）：只有列表接口带。 */
  vendor_platform?: string
  /** 经扩展端点（生图 / 视频 / 向量）承接：只有列表接口带；渠道表单的默认勾选不含这类模型。 */
  extension_endpoints?: boolean
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

/** 诊断接口按入站协议逐格报告资源能否承接。 */
export type ModelCatalogInboundProtocol = 'anthropic' | 'chat_completions' | 'responses' | 'gemini'

export interface ModelCatalogDiagnosisAccount extends ModelCatalogBindingAccount {
  /** 绑定优先级；null 表示跟随账号自身的优先级。 */
  priority: number | null
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
  /** 上游模型名对到目录条目（规范化名 / 别名 / 去厂商前缀），没对上的 entry_id 为空 */
  matchUpstreamModels: async (modelIds: string[]): Promise<ProbedUpstreamModel[]> => {
    const { data } = await apiClient.post<{ models: ProbedUpstreamModel[] }>('/admin/model-catalog/entries/match', { model_ids: modelIds })
    return data.models
  },
  /** 目录里没有的上游模型建成未上架条目（内置价格表查得到就带价），已有的原样返回 */
  importUpstreamModels: async (modelIds: string[]): Promise<ModelCatalogEntry[]> => {
    const { data } = await apiClient.post<{ entries: ModelCatalogEntry[] }>('/admin/model-catalog/entries/import', { model_ids: modelIds })
    return data.entries
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
  },
  diagnose: async (id: number): Promise<ModelCatalogDiagnosis> => {
    const { data } = await apiClient.get<ModelCatalogDiagnosis>(`/admin/model-catalog/entries/${id}/diagnosis`)
    return data
  },
  /** 渠道承接的目录条目 ID（渠道表单里直接勾选）。 */
  listAccountEntryIds: async (accountId: number): Promise<number[]> => {
    const { data } = await apiClient.get<{ entry_ids: number[] | null }>(`/admin/accounts/${accountId}/catalog-entries`)
    return data?.entry_ids ?? []
  },
  /** 整份覆盖渠道承接的目录条目；保留的绑定优先级不变，新增的跟随渠道优先级。 */
  replaceAccountEntries: async (accountId: number, entryIds: number[]): Promise<number[]> => {
    const { data } = await apiClient.put<{ entry_ids: number[] | null }>(`/admin/accounts/${accountId}/catalog-entries`, {
      entry_ids: entryIds
    })
    return data?.entry_ids ?? []
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
