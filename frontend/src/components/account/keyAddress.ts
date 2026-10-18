import type { ProtocolDefaultsResponse } from '@/api/admin/accounts'
import type { ProtocolEndpoints, UpstreamProtocol } from '@/types'
import { platformLabel } from '@/utils/platformLabel'
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
 * 与后端 Account.Vendor() 同一口径：video 地址查视频厂商域名表，其余查对话域名表
 * （火山方舟的对话与 Seedance 同域名，对话 key 仍按中转）。
 */
export function detectKeyVendor(
  endpoints: ProtocolEndpoints,
  vendorHosts: Record<string, string> | undefined,
  videoVendorHosts?: Record<string, string>
): string | null {
  let vendor: string | null = null
  const entries = Object.entries(endpoints).filter((entry): entry is [string, string] => typeof entry[1] === 'string')
  if (entries.length === 0) return null
  for (const [protocol, url] of entries) {
    const table = protocol === 'video' ? videoVendorHosts : vendorHosts
    const found = table?.[hostOf(url)]
    if (!found || (vendor && vendor !== found)) return null
    vendor = found
  }
  return vendor
}

/** 只以 video 协议接入的官方视频厂商（MiniMax 另有对话地址，不在其中）。 */
export const VIDEO_ONLY_VENDORS: ReadonlySet<string> = new Set(['seedance', 'kling'])

/** 官方视频地址的站点模式（后端官方地址表的 cn / intl）。 */
export type VideoSiteMode = 'cn' | 'intl'

export function isVideoSiteMode(mode: string | undefined): mode is VideoSiteMode {
  return mode === 'cn' || mode === 'intl'
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
  /** 地址对应的套餐 / 模式：国产厂商 payg / coding，OpenCode zen / go，官方视频地址的站点 cn / intl */
  mode?: CnAccountMode | 'zen' | 'go' | VideoSiteMode
  protocol: UpstreamProtocol
  url: string
  /** 专名标签（国产预设）；没有时由界面按厂商 / 模式 / 协议拼 */
  label?: string
}

const normalizeUrl = (url: string) => url.trim().replace(/\/+$/, '')

/**
 * 常用官方地址：国产厂商用带国际站的预设表，其余（OpenCode、官方视频厂商）取后端官方地址表；
 * 官方视频地址（video 协议，按站点 cn / intl）一律取后端官方地址表，MiniMax 也是。
 * 同一厂商同一协议同一地址只留一条。后端官方地址表按每个模式各回一份（default / coding / zen / go / cn / intl），
 * OpenCode 的 default、coding 与 zen 是同一组地址：先放有明确模式的，没标模式的同址条目不再重复列出。
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
  const pushVideoSites = (vendor: string, perMode: Record<string, ProtocolEndpoints>) => {
    for (const mode of ['cn', 'intl'] as const) {
      const url = perMode[mode]?.video
      if (url) push({ vendor, mode, protocol: 'video', url })
    }
  }
  for (const [vendor, perMode] of Object.entries(defaults?.defaults ?? {})) {
    if (isCNProviderPlatform(vendor)) {
      for (const preset of CN_BASE_URL_PRESETS[vendor]) {
        push({ vendor, mode: preset.mode, protocol: preset.protocol, url: preset.url, label: preset.label })
      }
      pushVideoSites(vendor, perMode)
      continue
    }
    if (VIDEO_ONLY_VENDORS.has(vendor)) {
      pushVideoSites(vendor, perMode)
      continue
    }
    const listedAddresses = new Set<string>()
    const modeOf = (modeKey: string) => (modeKey === 'zen' || modeKey === 'go' ? modeKey : undefined)
    const byMode = Object.entries(perMode).sort(([a], [b]) => Number(!modeOf(a)) - Number(!modeOf(b)))
    for (const [modeKey, endpoints] of byMode) {
      const mode = modeOf(modeKey)
      for (const [protocol, url] of Object.entries(endpoints)) {
        const address = `${protocol}|${normalizeUrl(url)}`
        if (!url || (!mode && listedAddresses.has(address))) continue
        listedAddresses.add(address)
        push({ vendor, mode, protocol: protocol as UpstreamProtocol, url })
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

/** 视频 key 的地址是哪个站点的官方地址；不是官方预设地址返回 null。 */
export function videoSiteOf(presets: KeyAddressPreset[], vendor: string, endpoints: ProtocolEndpoints): VideoSiteMode | null {
  const mode = modeOfAddress(presets, vendor, endpoints)
  return isVideoSiteMode(mode ?? undefined) ? (mode as VideoSiteMode) : null
}

type Translate = (key: string) => string

/** 只以 video 协议接入的厂商按产品名显示（火山方舟 / Seedance、可灵），其余厂商没有这条 i18n。 */
export function videoOnlyVendorName(vendor: string, t: Translate): string | null {
  return VIDEO_ONLY_VENDORS.has(vendor) ? t(`admin.accounts.keyAddress.videoVendors.${vendor}`) : null
}

/** 视频 key 的厂商提示：产品名（没有就平台名），认得出站点时带上站点。 */
export function videoKeyVendorLabel(vendor: string, site: VideoSiteMode | null, t: Translate): string {
  const name = videoOnlyVendorName(vendor, t) ?? platformLabel(vendor)
  return site ? `${name} · ${t(`admin.accounts.keyAddress.site.${site}`)}` : name
}
