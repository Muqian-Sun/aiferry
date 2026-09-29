import type { ProtocolDefaultsResponse } from '@/api/admin/accounts'
import type { ProtocolEndpoints, UpstreamProtocol } from '@/types'
import { CN_BASE_URL_PRESETS, isCNProviderPlatform, type CnAccountMode } from './credentialsBuilder'

// 第三方 key 不选平台（muqian 2026-09-25）：只填协议 + 地址 + Key，常用官方地址做快捷填入，
// 厂商按地址识别——域名表由后端下发（protocol-defaults 的 vendor_hosts），前端不另抄。
// 官方地址与域名表里只有国产厂商与 OpenCode：Anthropic、OpenAI、Gemini、Grok 只认成品号，
// 指向它们官方域名的 key 一律按中转处理（muqian 2026-09-29 定）。

/** 地址的主机名（小写）；解析不了返回空串。只认完整域名，与后端 upstreamHostOf 一致。 */
export function hostOf(url: string): string {
  try {
    return new URL(url.trim()).hostname.toLowerCase()
  } catch {
    return ''
  }
}

/**
 * 按地址识别第三方 key 的厂商：全部地址都是同一厂商的官方域名才算，否则返回 null（中转，只走标准协议）。
 * 与后端 Account.Vendor() 同一口径。
 */
export function detectKeyVendor(endpoints: ProtocolEndpoints, vendorHosts: Record<string, string> | undefined): string | null {
  if (!vendorHosts) return null
  let vendor: string | null = null
  const urls = Object.values(endpoints).filter((url): url is string => typeof url === 'string')
  if (urls.length === 0) return null
  for (const url of urls) {
    const found = vendorHosts[hostOf(url)]
    if (!found || (vendor && vendor !== found)) return null
    vendor = found
  }
  return vendor
}

/**
 * 第三方 key 的 API Key 输入框占位：按地址识别出的厂商给格式提示，智谱的 key 是「id.secret」两段；
 * 其余（含中转）一律 sk-...，不按平台标签猜海外厂商的官方格式——指向它们的 key 都按中转处理。
 */
export function apiKeyPlaceholderFor(vendor: string | null): string {
  return vendor === 'zhipu' ? '<api-key>.<secret>' : 'sk-...'
}

/** 有「按量 / Coding 套餐」两种计费的厂商：套餐要管理员选（MiniMax 两种套餐同一个地址，靠地址分不出来）。 */
export const VENDORS_WITH_CODING_PLAN: ReadonlySet<string> = new Set(['kimi', 'zhipu', 'minimax'])

export interface KeyAddressPreset {
  vendor: string
  /** 地址对应的套餐 / 模式：国产厂商 payg / coding，OpenCode zen / go；其余厂商没有 */
  mode?: CnAccountMode | 'zen' | 'go'
  protocol: UpstreamProtocol
  url: string
  /** 专名标签（国产预设）；没有时由界面按厂商 / 模式 / 协议拼 */
  label?: string
}

const normalizeUrl = (url: string) => url.trim().replace(/\/+$/, '')

/**
 * 常用官方地址：国产厂商用带国际站的预设表，其余（OpenCode）取后端官方地址表。
 * 同一厂商同一协议同一地址只留一条。
 */
export function keyAddressPresets(defaults: ProtocolDefaultsResponse | null): KeyAddressPreset[] {
  const out: KeyAddressPreset[] = []
  const seen = new Set<string>()
  const push = (preset: KeyAddressPreset) => {
    const key = `${preset.vendor}|${preset.mode ?? ''}|${preset.protocol}|${normalizeUrl(preset.url)}`
    if (seen.has(key)) return
    seen.add(key)
    out.push(preset)
  }
  for (const [vendor, perMode] of Object.entries(defaults?.defaults ?? {})) {
    if (isCNProviderPlatform(vendor)) {
      for (const preset of CN_BASE_URL_PRESETS[vendor]) {
        push({ vendor, mode: preset.mode, protocol: preset.protocol, url: preset.url, label: preset.label })
      }
      continue
    }
    for (const [modeKey, endpoints] of Object.entries(perMode)) {
      const mode = modeKey === 'zen' || modeKey === 'go' ? modeKey : undefined
      for (const [protocol, url] of Object.entries(endpoints)) {
        if (url) push({ vendor, mode, protocol: protocol as UpstreamProtocol, url })
      }
    }
  }
  return out
}

/**
 * 地址对应的套餐 / 模式：当前协议地址只命中该厂商一种模式的预设时返回它；
 * 命中多种（MiniMax 按量与套餐同地址）或都不命中返回 null，交给管理员选。
 */
export function modeOfAddress(presets: KeyAddressPreset[], vendor: string, endpoints: ProtocolEndpoints): KeyAddressPreset['mode'] | null {
  const [protocol] = Object.keys(endpoints) as UpstreamProtocol[]
  const url = protocol ? endpoints[protocol] : undefined
  if (!url) return null
  const modes = new Set(
    presets
      .filter((preset) => preset.vendor === vendor && preset.protocol === protocol && normalizeUrl(preset.url) === normalizeUrl(url))
      .map((preset) => preset.mode)
  )
  return modes.size === 1 ? ([...modes][0] ?? null) : null
}
