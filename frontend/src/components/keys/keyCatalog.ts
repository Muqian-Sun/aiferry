/**
 * 「使用密钥」配置模板里的模型按上架目录生成（2026-10-04 D4，muqian 定）：原来写死了一串目录里没有的模型，
 * 照抄会 404。某个客户端在目录里没有对应厂商的模型时，这个客户端的页签整个不出现。
 *
 * 只取按 Token 计费的对话模型（生图 / 视频 / 按次的不进对话客户端）；厂商按展示名归族（与模型页同一张表），
 * 同一家里「新」的排前面（标识按数字自然序倒排：gpt-6.1-sol 在 gpt-5.5 前），第一个就是默认模型。
 */
import type { PlazaModel } from '@/api/modelPlaza'
import { vendorLabel } from '@/components/modelPlaza/catalog'
import type { UseKeyClientId } from '@/components/user/clients'

export type ClientVendor = 'anthropic' | 'openai' | 'google' | 'xai'

export interface KeyCatalogModel {
  id: string
  name: string
}

export type VendorModels = Record<ClientVendor, KeyCatalogModel[]>

const VENDOR_BY_LABEL: Record<string, ClientVendor> = {
  Anthropic: 'anthropic',
  OpenAI: 'openai',
  Google: 'google',
  xAI: 'xai'
}

/** 每个客户端用哪家的模型：Claude Code 走 Anthropic、Codex 走 OpenAI、Gemini CLI 走 Google、Grok CLI 走 xAI；OpenCode 四家都能配 */
export const CLIENT_VENDORS: Record<UseKeyClientId, ClientVendor[]> = {
  claude: ['anthropic'],
  codex: ['openai'],
  'codex-ws': ['openai'],
  gemini: ['google'],
  grok: ['xai'],
  opencode: ['anthropic', 'openai', 'google', 'xai']
}

/** 新的排前面：标识按数字自然序倒排 */
export function newestFirst<T extends { id: string }>(models: T[]): T[] {
  return [...models].sort((a, b) => b.id.localeCompare(a.id, undefined, { numeric: true }))
}

/** 上架目录里的对话模型按厂商分组，每组新的在前 */
export function groupChatModels(models: PlazaModel[]): VendorModels {
  const groups: VendorModels = { anthropic: [], openai: [], google: [], xai: [] }
  for (const model of models) {
    const billingMode = model.billing_mode || model.pricing?.billing_mode || 'token'
    if (billingMode !== 'token') continue
    const vendor = VENDOR_BY_LABEL[vendorLabel(model.vendor)]
    if (!vendor) continue
    groups[vendor].push({ id: model.model_id, name: model.display_name || model.model_id })
  }
  for (const vendor of Object.keys(groups) as ClientVendor[]) {
    groups[vendor] = newestFirst(groups[vendor])
  }
  return groups
}

/** 目录里有对应厂商模型的客户端（OpenCode 只要四家里有一家就出现） */
export function clientHasModels(client: UseKeyClientId, groups: VendorModels): boolean {
  return CLIENT_VENDORS[client].some((vendor) => groups[vendor].length > 0)
}

/** 某家第一个（新的）生图 / 视频模型标识；目录里没有时为空串（Grok 的生图 / 视频开关据此开关） */
export function newestMediaModel(models: PlazaModel[], vendor: ClientVendor, mode: 'image' | 'video'): string {
  const matches = models
    .filter((model) => (model.billing_mode || model.pricing?.billing_mode) === mode && VENDOR_BY_LABEL[vendorLabel(model.vendor)] === vendor)
    .map((model) => ({ id: model.model_id }))
  return newestFirst(matches)[0]?.id ?? ''
}
