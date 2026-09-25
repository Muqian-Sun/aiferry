import type { ProtocolDefaultsResponse } from '@/api/admin/accounts'
import type { ProtocolEndpoints, UpstreamProtocol } from '@/types'
import { CN_BASE_URL_PRESETS, GROK_BASE_URL_PRESETS, isCNProviderPlatform, type CnAccountMode } from './credentialsBuilder'

// 第三方 key 不选平台（muqian 2026-09-25）：只填协议 + 地址 + Key，常用官方地址做快捷填入，
// 厂商按地址识别——域名表由后端下发（protocol-defaults 的 vendor_hosts），前端不另抄。

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

/** 有「按量 / Coding 套餐」两种计费的厂商：套餐要管理员选（MiniMax 两种套餐同一个地址，靠地址分不出来）。 */
export const VENDORS_WITH_CODING_PLAN: ReadonlySet<string> = new Set(['kimi', 'zhipu', 'minimax'])

export interface KeyAddressPreset {
  vendor: string
  /** 地址对应的套餐 / 模式：国产厂商 payg / coding，OpenCode zen / go；其余厂商没有 */
  mode?: CnAccountMode | 'zen' | 'go'
  protocol: UpstreamProtocol
  url: string
  /** 专名标签（国产预设、Grok 区域）；没有时由界面按厂商 / 模式 / 协议拼 */
  label?: string
}

const normalizeUrl = (url: string) => url.trim().replace(/\/+$/, '')

/**
 * 常用官方地址：国产厂商用带国际站的预设表，Grok 加区域地址，其余取后端官方地址表。
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
    if (vendor === 'grok') {
      for (const preset of GROK_BASE_URL_PRESETS) {
        if (preset.label) push({ vendor, protocol: 'responses', url: preset.url, label: preset.label })
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
