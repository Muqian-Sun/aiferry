#!/usr/bin/env node
/**
 * 首页图标云的厂商 logo 生成器：把 @lobehub/icons 的官方组件渲染成静态 SVG，写成数据文件（用时再编成 data URI）。
 * 图标路径与配色全部来自这个包本身，不手画、不手抄。
 *
 * 用法：node scripts/gen-vendor-logos.mjs
 * 产物：src/components/user/home/vendorLogos.ts（勿手改，改清单后重跑本脚本）
 *
 * @lobehub/icons 是 React 组件、且不是 ESM 包，所以借 vite 自带的 esbuild 把组件连同 react-dom/server
 * 打成一个临时 CJS 文件再执行。缺组件、重复键、渲染不出 <svg> 一律报错退出（fail-closed）。
 */
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'

const FRONTEND = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const OUT = path.join(FRONTEND, 'src/components/user/home/vendorLogos.ts')
const requireHere = createRequire(path.join(FRONTEND, 'package.json'))
const esbuild = createRequire(requireHere.resolve('vite/package.json'))('esbuild')
const ICONS_DIR = fs.realpathSync(path.join(FRONTEND, 'node_modules/@lobehub/icons'))
const ICONS_VERSION = JSON.parse(fs.readFileSync(path.join(ICONS_DIR, 'package.json'), 'utf8')).version

/**
 * 清单：[组件目录名, 变体]。muqian 2026-09-23 点名的 11 家「用户最关注的大模型」：
 * 美国 Claude / OpenAI / Grok / Gemini，中国 智谱（Z.ai 字标）/ Kimi / DeepSeek / MiniMax / MiMo / Qwen / 豆包。
 * 有官方全彩（Color）的用全彩，只有单色的用 Mono。顺序即首屏图标云里的摆放顺序（中美交错）。
 */
const LOGOS = [
  ['Claude', 'Color'],
  ['DeepSeek', 'Color'],
  ['OpenAI', 'Mono'],
  ['Qwen', 'Color'],
  ['Gemini', 'Color'],
  ['Kimi', 'Color'],
  ['Grok', 'Mono'],
  ['Doubao', 'Color'],
  ['ZAI', 'Mono'],
  ['Minimax', 'Color'],
  ['XiaomiMiMo', 'Mono']
]

function fail(message) {
  console.error(`gen-vendor-logos: ${message}`)
  process.exit(1)
}

// ---- 清单自检 ----
const seen = new Set()
for (const [key, variant] of LOGOS) {
  if (seen.has(key)) fail(`重复的键 ${key}`)
  seen.add(key)
  const file = path.join(ICONS_DIR, 'es', key, 'components', `${variant}.js`)
  if (!fs.existsSync(file)) fail(`@lobehub/icons@${ICONS_VERSION} 里没有 ${key}/${variant}`)
}

// ---- 打包 + 渲染 ----
const entry = [
  "const { createElement } = require('react')",
  "const { renderToStaticMarkup } = require('react-dom/server')",
  ...LOGOS.map(
    ([key, variant], i) =>
      `const I${i} = require('./es/${key}/components/${variant}.js').default\nconst S${i} = require('./es/${key}/style.js')`
  ),
  'module.exports = [',
  ...LOGOS.map(
    ([key, variant], i) =>
      `  { key: ${JSON.stringify(key)}, variant: ${JSON.stringify(variant)}, title: S${i}.TITLE, color: S${i}.COLOR_PRIMARY, svg: renderToStaticMarkup(createElement(I${i}, { size: 24 })) },`
  ),
  ']'
].join('\n')

const bundle = await esbuild.build({
  stdin: { contents: entry, resolveDir: ICONS_DIR, loader: 'js' },
  bundle: true,
  format: 'cjs',
  platform: 'node',
  write: false,
  logLevel: 'error',
  define: { 'process.env.NODE_ENV': '"production"' }
})
const tmp = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'vendor-logos-')), 'render.cjs')
fs.writeFileSync(tmp, bundle.outputFiles[0].text)
const rendered = createRequire(tmp)(tmp)

// ---- 整理 ----
/** 相对亮度（sRGB），用来判断单色图标的品牌色是不是接近纯黑 / 纯白 */
function luminance(hex) {
  const h = hex.replace('#', '')
  const full = h.length === 3 ? h.split('').map((c) => c + c).join('') : h
  const [r, g, b] = [0, 2, 4].map((i) => parseInt(full.slice(i, i + 2), 16) / 255)
  const lin = (c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4)
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b)
}

const entries = rendered.map(({ key, variant, title, color, svg }) => {
  if (!svg.startsWith('<svg')) fail(`${key} 没渲染出 <svg>`)
  let clean = svg
    .replace(/<title>[^<]*<\/title>/g, '')
    .replace(/\sstyle="[^"]*"/g, '')
  const mono = variant === 'Mono'
  // 单色图标当遮罩用：形状要不透明，颜色由使用处决定
  if (mono) clean = clean.replace(/currentColor/g, '#000')
  // 品牌色太接近黑 / 白（OpenAI、Grok、Midjourney…）就不指定，让使用处用墨色 token，深色模式自动翻
  const tint = mono && /^#[0-9a-f]{3,6}$/i.test(color ?? '') && luminance(color) > 0.08 && luminance(color) < 0.8 ? color : null
  // 全彩图标里有近黑色块（Kimi 的 K、Yi 的 Y…）：深色底上看不见，打标让使用处在深色模式下反相
  const fills = [...clean.matchAll(/(?:fill|stop-color)="(#[0-9a-f]{3,6})"/gi)].map((m) => m[1])
  const inkOnDark = !mono && fills.some((hex) => luminance(hex) < 0.02)
  return {
    key: key.toLowerCase(),
    title,
    kind: mono ? 'mono' : 'color',
    tint,
    inkOnDark,
    svg: clean
  }
})

const header = `/**
 * 首页图标云用的厂商 logo：由 scripts/gen-vendor-logos.mjs 从 @lobehub/icons@${ICONS_VERSION} 生成，勿手改。
 * 改清单请改脚本里的 LOGOS 后重跑 \`node scripts/gen-vendor-logos.mjs\`。
 *
 * - kind = color：官方全彩 SVG，编成 data URI 当 <img> 画。
 * - kind = mono：官方单色 SVG，编成 data URI 当 CSS 遮罩用；颜色取 tint，没有 tint（品牌色是黑 / 白）就用墨色 token。
 * - inkOnDark：全彩图标里含近黑色块，深色模式下要反相才看得见。
 * 存原始 SVG、用时再编码：percent-encoding 会让文件大一倍。
 */
export interface VendorLogo {
  key: string
  title: string
  kind: 'color' | 'mono'
  tint: string | null
  inkOnDark: boolean
  svg: string
}

export const VENDOR_LOGOS: readonly VendorLogo[] = `
/** 单引号字面量（与仓库的 lint 风格一致）；反斜杠与单引号转义 */
const str = (value) => (value === null ? 'null' : `'${String(value).replace(/\\/g, '\\\\').replace(/'/g, "\\'")}'`)
const body = entries
  .map((e) => `  { key: ${str(e.key)}, title: ${str(e.title)}, kind: ${str(e.kind)}, tint: ${str(e.tint)}, inkOnDark: ${e.inkOnDark}, svg: ${str(e.svg)} }`)
  .join(',\n')
fs.writeFileSync(OUT, `${header}[\n${body}\n]\n`)
console.log(`gen-vendor-logos: ${entries.length} 个 logo（color ${entries.filter((e) => e.kind === 'color').length} / mono ${entries.filter((e) => e.kind === 'mono').length}）→ ${path.relative(FRONTEND, OUT)}，${(fs.statSync(OUT).size / 1024).toFixed(1)} KB`)
