#!/usr/bin/env node
/**
 * 管理站换皮 codemod（A1）：把旧调色类换成 af-* 设计 token。
 *
 * 用法：node scripts/codemod-af-tokens.mjs [--write] <文件或目录>...
 *   不带 --write 只报告；带 --write 才写回。映射不了的类逐条打印（文件:行 类名），存在时退出码 1——fail-closed，不静默跳过。
 *
 * 只动这些位置（全部走 AST，不做全文正则）：
 *   - 模板里的静态 class 与 :class 绑定表达式中的字符串字面量 / 模板字符串 / 对象字符串键
 *   - <style> 块里的 @apply
 *   - <script> 与 .ts 文件里「整串都是类名」且含旧类的字符串字面量
 *
 * 配色规则（muqian 2026-09-24）：灰系 → 墨色 / 纸面 / 细线 token；红 → danger、绿 → success、黄橙 → warning；
 * primary → brand；蓝 / 紫 / 天蓝 / 青 / 粉 / 靛等装饰色一律收成墨色。dark: 变体删除（token 自带明暗）。
 */
import { readFileSync, readdirSync, statSync, writeFileSync } from 'node:fs'
import { extname, join, relative, resolve } from 'node:path'
import { babelParse, parse as parseSfc } from 'vue/compiler-sfc'
import postcss from 'postcss'

const args = process.argv.slice(2)
const WRITE = args.includes('--write')
const targets = args.filter((a) => a !== '--write')
if (!targets.length) {
  console.error('usage: codemod-af-tokens.mjs [--write] <file|dir>...')
  process.exit(2)
}

// ---------------------------------------------------------------------------
// 类名映射
// ---------------------------------------------------------------------------
const NEUTRAL = new Set(['gray', 'slate', 'zinc', 'neutral', 'stone', 'dark'])
const SEMANTIC = {
  red: 'danger', rose: 'danger',
  green: 'success', emerald: 'success', lime: 'success',
  amber: 'warning', yellow: 'warning', orange: 'warning'
}
const BRAND = new Set(['primary'])
const MONO = new Set(['blue', 'sky', 'indigo', 'violet', 'purple', 'fuchsia', 'pink', 'cyan', 'teal'])
const PALETTES = [...NEUTRAL, ...Object.keys(SEMANTIC), ...BRAND, ...MONO]

// 长的前缀在前，免得 border-t 被 border 抢先匹配
const UTILS = ['ring-offset', 'border-t', 'border-r', 'border-b', 'border-l', 'border-x', 'border-y', 'placeholder', 'decoration', 'outline', 'divide', 'border', 'accent', 'shadow', 'caret', 'stroke', 'fill', 'text', 'ring', 'from', 'via', 'bg', 'to']
const UTIL_RE = UTILS.join('|')
const PALETTE_TOKEN = new RegExp(`^(!?)(${UTIL_RE})-(${PALETTES.join('|')})-(\\d{2,3})(/\\d+)?$`)
const BW_TOKEN = new RegExp(`^(!?)(${UTIL_RE})-(white|black)(/\\d+)?$`)
const TEXT_LIKE = new Set(['text', 'placeholder', 'fill', 'stroke', 'decoration', 'caret', 'accent'])
const BORDER_LIKE = new Set(['border', 'border-t', 'border-r', 'border-b', 'border-l', 'border-x', 'border-y', 'divide', 'outline'])

/** 灰系按色阶落到 token 名（不含前缀） */
function neutralToken(util, shade, opacity) {
  if (util === 'placeholder') return 'af-ink-4'
  if (TEXT_LIKE.has(util)) {
    if (shade >= 800) return 'af-ink'
    if (shade >= 600) return 'af-ink-2'
    if (shade >= 400) return 'af-ink-3'
    if (shade >= 200) return 'af-ink-4'
    return 'af-on-brand' // 50 / 100 的字只出现在深色底上
  }
  if (util === 'bg') {
    if (shade <= 100) return 'af-sunken'
    if (shade <= 200) return 'af-hairline'
    if (shade <= 400) return 'af-ink-4'
    if (shade <= 600) return 'af-ink-3'
    if (shade <= 700) return 'af-ink-2'
    return 'af-ink'
  }
  if (util === 'ring') {
    if (opacity && Number(opacity.slice(1)) <= 10) return 'af-hairline' // ring-gray-900/5 这类描边
    if (shade <= 300) return 'af-hairline'
    return 'af-hairline-strong'
  }
  if (util === 'ring-offset') return 'af-sheet'
  if (BORDER_LIKE.has(util)) {
    if (shade <= 200) return 'af-hairline'
    if (shade <= 400) return 'af-hairline-strong'
    if (shade <= 600) return 'af-ink-4'
    return 'af-ink-3'
  }
  return null
}

function semanticToken(util, shade, name) {
  if (TEXT_LIKE.has(util)) return shade >= 300 ? `af-${name}` : null
  if (util === 'bg') return shade <= 200 ? `af-${name}-tint` : `af-${name}`
  if (BORDER_LIKE.has(util) || util === 'ring') return shade <= 300 ? `af-${name}/30` : `af-${name}`
  if (util === 'ring-offset') return 'af-sheet'
  return null
}

function brandToken(util, shade, hover) {
  if (TEXT_LIKE.has(util)) return shade >= 400 ? (hover ? 'af-brand-hover' : 'af-brand') : null
  if (util === 'bg') return shade <= 200 ? 'af-brand-tint' : hover && shade >= 600 ? 'af-brand-hover' : 'af-brand'
  if (BORDER_LIKE.has(util)) return shade <= 300 ? 'af-hairline-strong' : 'af-brand'
  if (util === 'ring') return 'af-brand'
  if (util === 'ring-offset') return 'af-sheet'
  return null
}

function monoToken(util, shade, hover) {
  if (TEXT_LIKE.has(util)) {
    if (shade >= 400) return 'af-ink-2'
    if (shade >= 300) return 'af-ink-3'
    return null
  }
  if (util === 'bg') return shade <= 200 ? 'af-sunken' : hover ? 'af-ink-2' : 'af-ink'
  if (BORDER_LIKE.has(util)) return shade <= 300 ? 'af-hairline' : 'af-ink-3'
  if (util === 'ring') return shade <= 300 ? 'af-hairline' : 'af-brand'
  if (util === 'ring-offset') return 'af-sheet'
  return null
}

const COLOR_BASE = new RegExp(`^!?(?:(?:${UTIL_RE})-(?:${PALETTES.join('|')})-\\d{2,3}(?:/\\d+)?|(?:${UTIL_RE})-(?:white|black|transparent|current|inherit)(?:/\\d+)?|shadow(?:-\\w+)?|bg-gradient-to-\\w+|(?:bg|border|ring|text|divide|placeholder)-opacity-\\d+|ring-\\d|ring|border)$`)

const LEGACY_FLAG = new RegExp(`(?:^|:)!?(?:(?:${UTIL_RE})-(?:${PALETTES.join('|')})-\\d{2,3}|(?:bg|text|border|ring|ring-offset|divide|fill|stroke)-(?:white|black)|shadow(?:-sm|-inner)?$|rounded(?:-[trbl]{1,2})?-(?:2xl|3xl)$|bg-gradient-to-|bg-mesh-gradient|text-gradient|gradient-primary|gradient-dark|glass|shadow-glow|shadow-glass|shadow-card)|(?:^|:)dark:`)

/**
 * 单个类 → 结果：{ out: string|null（null = 删除）, unmapped?: true }
 * 没有旧成分的类原样返回。
 */
export function mapClass(cls) {
  const parts = cls.split(':')
  const base = parts.pop()
  const variants = parts
  if (!base) return { out: cls }

  if (variants.includes('dark')) {
    // 颜色 / 阴影 / 渐变类的 dark: 变体直接删（token 自带明暗）；其余（dark:hidden、dark:invert…）要人看
    return COLOR_BASE.test(base) ? { out: null } : { out: cls, unmapped: true }
  }
  const prefix = variants.length ? `${variants.join(':')}:` : ''
  const hover = variants.some((v) => v === 'hover' || v === 'group-hover')

  // 阴影：小阴影删；浮层（md 以上）保留
  if (/^!?shadow(?:-sm|-inner)?$/.test(base)) return { out: null }
  // 圆角：2xl / 3xl 收到 lg
  const round = base.match(/^(!?rounded(?:-[trbl]{1,2})?)-(?:2xl|3xl)$/)
  if (round) return { out: `${prefix}${round[1]}-lg` }
  // 渐变、玻璃：没有机械映射，人来改
  if (/^(?:bg-gradient-to-\w+|bg-mesh-gradient|text-gradient|gradient-primary|gradient-dark|glass(?:-card)?|shadow-glow(?:-lg)?|shadow-glass(?:-sm)?|shadow-card(?:-hover)?)$/.test(base)) {
    return { out: cls, unmapped: true }
  }

  const bw = base.match(BW_TOKEN)
  if (bw) {
    const [, bang, util, color, opacity = ''] = bw
    if (util === 'from' || util === 'via' || util === 'to') return { out: cls, unmapped: true }
    let token = null
    if (color === 'white') {
      token = TEXT_LIKE.has(util) ? 'af-on-brand' : 'af-sheet'
    } else {
      if (util === 'bg') return { out: cls } // 遮罩的 bg-black/50 两种模式都成立，保留
      token = TEXT_LIKE.has(util) ? 'af-ink' : 'af-ink'
    }
    return { out: `${prefix}${bang}${util}-${token}${opacity}` }
  }

  const m = base.match(PALETTE_TOKEN)
  if (!m) return { out: cls }
  const [, bang, util, palette, shadeText, opacity = ''] = m
  const shade = Number(shadeText)
  if (util === 'from' || util === 'via' || util === 'to') return { out: cls, unmapped: true }
  if (util === 'shadow') return { out: null } // 彩色阴影直接去掉

  let token = null
  if (NEUTRAL.has(palette)) token = neutralToken(util, shade, opacity)
  else if (SEMANTIC[palette]) token = semanticToken(util, shade, SEMANTIC[palette])
  else if (BRAND.has(palette)) token = brandToken(util, shade, hover)
  else if (MONO.has(palette)) token = monoToken(util, shade, hover)
  if (!token) return { out: cls, unmapped: true }

  // token 自带透明度（如 af-danger/30）时不再叠原透明度；ring-gray-900/5 这类已折成 hairline 的也丢掉原透明度
  const keepOpacity = !token.includes('/') && !(util === 'ring' && token === 'af-hairline')
  return { out: `${prefix}${bang}${util}-${token}${keepOpacity ? opacity : ''}` }
}

export function isLegacyClass(cls) {
  return LEGACY_FLAG.test(cls)
}

const CLASSY = /^[!a-z0-9\-:/[\].%#_]+$/

/** 悬停色与常态色撞成同一个 token 时，悬停用的下一档（按工具类分） */
const HOVER_STEP = {
  bg: {
    'af-sheet': 'af-sunken',
    'af-sunken': 'af-hairline',
    'af-hairline': 'af-hairline-strong',
    'af-brand-tint': 'af-hairline',
    'af-brand': 'af-brand-hover',
    'af-ink': 'af-ink-2'
  },
  text: {
    'af-ink-4': 'af-ink-3',
    'af-ink-3': 'af-ink-2',
    'af-ink-2': 'af-ink',
    'af-brand': 'af-brand-hover'
  },
  border: {
    'af-hairline': 'af-hairline-strong',
    'af-hairline-strong': 'af-ink-4'
  }
}

/** 一串类名 → { text, unmapped[] , changed } ；去重、保持顺序、压空白 */
function mapClassList(text) {
  const tokens = text.split(/\s+/).filter(Boolean)
  const out = []
  const unmapped = []
  let changed = false
  for (const t of tokens) {
    const r = mapClass(t)
    if (r.unmapped) unmapped.push(t)
    if (r.out !== t) changed = true
    if (r.out !== null && !out.includes(r.out)) out.push(r.out)
    else if (r.out !== null) changed = true
  }
  if (!changed) return { text, unmapped, changed: false }
  // 常态与悬停落到同一个 token 时（bg-gray-50 hover:bg-gray-100 → 都是 sunken），悬停往上加深一档，保住悬停反馈
  for (let i = 0; i < out.length; i++) {
    const hover = out[i].match(/^((?:[a-z-]+:)*(?:hover|group-hover):)(!?)(bg|text|border)-(af-[a-z0-9-]+)$/)
    if (!hover || !out.includes(`${hover[2]}${hover[3]}-${hover[4]}`)) continue
    const next = HOVER_STEP[hover[3]][hover[4]]
    if (next) out[i] = `${hover[1]}${hover[2]}${hover[3]}-${next}`
  }
  const lead = text.match(/^\s*/)[0]
  const trail = text.match(/\s*$/)[0]
  return { text: lead + out.join(' ') + trail, unmapped, changed: true }
}

/** 脚本里的字符串：整串都长得像类名、且至少有一个旧类，才当作类名串处理 */
function looksLikeLegacyClassString(text) {
  const tokens = text.split(/\s+/).filter(Boolean)
  return tokens.length > 0 && tokens.every((t) => CLASSY.test(t)) && tokens.some(isLegacyClass)
}

// ---------------------------------------------------------------------------
// 收集改动
// ---------------------------------------------------------------------------
function walkBabel(node, visit) {
  if (!node || typeof node.type !== 'string') return
  visit(node)
  for (const key of Object.keys(node)) {
    if (key === 'loc' || key === 'start' || key === 'end' || key === 'leadingComments' || key === 'trailingComments' || key === 'innerComments' || key === 'extra') continue
    const value = node[key]
    if (Array.isArray(value)) value.forEach((v) => walkBabel(v, visit))
    else if (value && typeof value === 'object' && typeof value.type === 'string') walkBabel(value, visit)
  }
}

/** 表达式里的类名字符串（:class）：字符串字面量、模板字符串片段；base = 表达式在全文中的起点 */
function classStringsInExpression(code, base, onlyLegacy) {
  const found = []
  const ast = babelParse(`(${code})`, { plugins: ['typescript'] })
  walkBabel(ast, (node) => {
    if (node.type === 'StringLiteral') {
      if (!onlyLegacy || looksLikeLegacyClassString(node.value)) found.push({ start: base + node.start - 1 + 1, end: base + node.end - 1 - 1, text: node.value })
    } else if (node.type === 'TemplateLiteral') {
      for (const q of node.quasis) {
        const raw = q.value.raw
        if (!onlyLegacy || looksLikeLegacyClassString(raw)) found.push({ start: base + q.start - 1, end: base + q.end - 1, text: raw })
      }
    }
  })
  return found
}

function classStringsInScript(code, base) {
  const found = []
  const ast = babelParse(code, { sourceType: 'module', plugins: ['typescript'] })
  walkBabel(ast.program ?? ast, (node) => {
    if (node.type === 'ImportDeclaration' || node.type === 'ExportAllDeclaration') return
    if (node.type === 'StringLiteral' && looksLikeLegacyClassString(node.value)) {
      found.push({ start: base + node.start + 1, end: base + node.end - 1, text: node.value })
    } else if (node.type === 'TemplateLiteral') {
      for (const q of node.quasis) {
        if (looksLikeLegacyClassString(q.value.raw)) found.push({ start: base + q.start, end: base + q.end, text: q.value.raw })
      }
    }
  })
  return found
}

function lineOf(source, offset) {
  let line = 1
  for (let i = 0; i < offset && i < source.length; i++) if (source.charCodeAt(i) === 10) line++
  return line
}

function processVue(source, file) {
  const { descriptor, errors } = parseSfc(source, { filename: file })
  if (errors.length) throw new Error(`SFC parse error: ${errors[0].message ?? errors[0]}`)
  const spans = []

  const tpl = descriptor.template?.ast
  if (tpl) {
    const visit = (node) => {
      if (node.type === 1) {
        for (const p of node.props) {
          if (p.type === 6 && p.name === 'class' && p.value) {
            const { start, end } = p.value.loc
            const q = source[start.offset]
            if (q !== '"' && q !== "'") throw new Error(`unquoted class at ${lineOf(source, start.offset)}`)
            spans.push({ start: start.offset + 1, end: end.offset - 1, text: p.value.content, kind: 'class' })
          }
          if (p.type === 7 && p.name === 'bind' && p.arg?.type === 4 && p.arg.content === 'class' && p.exp) {
            const expStart = p.exp.loc.start.offset
            if (source.slice(expStart, p.exp.loc.end.offset) !== p.exp.content) throw new Error(`exp offset mismatch at ${lineOf(source, expStart)}`)
            for (const s of classStringsInExpression(p.exp.content, expStart, false)) spans.push({ ...s, kind: ':class' })
          }
        }
      }
      for (const c of node.children ?? []) visit(c)
      for (const b of node.branches ?? []) visit(b)
    }
    visit(tpl)
  }

  for (const block of [descriptor.script, descriptor.scriptSetup]) {
    if (!block) continue
    for (const s of classStringsInScript(block.content, block.loc.start.offset)) spans.push({ ...s, kind: 'script' })
  }

  const edits = []
  const unmapped = []
  for (const span of spans) {
    if (source.slice(span.start, span.end) !== span.text) throw new Error(`span mismatch at line ${lineOf(source, span.start)}: ${JSON.stringify(span.text).slice(0, 60)}`)
    const r = mapClassList(span.text)
    r.unmapped.forEach((u) => unmapped.push({ line: lineOf(source, span.start), cls: u, kind: span.kind }))
    if (r.changed) edits.push({ start: span.start, end: span.end, text: r.text })
  }

  // <style> 的 @apply：postcss 改 params，整块内容替换
  for (const style of descriptor.styles) {
    const root = postcss.parse(style.content)
    let changed = false
    root.walkAtRules('apply', (rule) => {
      const r = mapClassList(rule.params)
      r.unmapped.forEach((u) => unmapped.push({ line: lineOf(source, style.loc.start.offset) + (rule.source?.start?.line ?? 1) - 1, cls: u, kind: '@apply' }))
      if (r.changed) {
        changed = true
        if (r.text.trim()) rule.params = r.text.trim()
        else rule.remove()
      }
    })
    if (changed) edits.push({ start: style.loc.start.offset, end: style.loc.end.offset, text: root.toString() })
  }
  return { edits, unmapped }
}

function processTs(source) {
  const edits = []
  const unmapped = []
  for (const s of classStringsInScript(source, 0)) {
    const r = mapClassList(s.text)
    r.unmapped.forEach((u) => unmapped.push({ line: lineOf(source, s.start), cls: u, kind: 'ts' }))
    if (r.changed) edits.push({ start: s.start, end: s.end, text: r.text })
  }
  return { edits, unmapped }
}

function applyEdits(source, edits) {
  const sorted = [...edits].sort((a, b) => b.start - a.start)
  for (let i = 1; i < sorted.length; i++) {
    if (sorted[i].end > sorted[i - 1].start) throw new Error(`overlapping edits at ${sorted[i].start}`)
  }
  let out = source
  for (const e of sorted) out = out.slice(0, e.start) + e.text + out.slice(e.end)
  return out
}

// ---------------------------------------------------------------------------
// 主流程
// ---------------------------------------------------------------------------
const cwd = process.cwd()
const files = []
for (const target of targets) {
  const full = resolve(cwd, target)
  const st = statSync(full)
  if (st.isDirectory()) {
    const walk = (dir) => {
      for (const name of readdirSync(dir)) {
        const p = join(dir, name)
        if (name === '__tests__' || name === 'node_modules') continue
        if (statSync(p).isDirectory()) walk(p)
        else if (['.vue', '.ts'].includes(extname(name)) && !name.endsWith('.d.ts')) files.push(p)
      }
    }
    walk(full)
  } else files.push(full)
}
if (!files.length) {
  console.error('no files matched')
  process.exit(2)
}

let totalEdits = 0
let changedFiles = 0
const allUnmapped = []
const failures = []
for (const file of [...new Set(files)].sort()) {
  const rel = relative(cwd, file)
  const source = readFileSync(file, 'utf8')
  try {
    const { edits, unmapped } = file.endsWith('.vue') ? processVue(source, file) : processTs(source)
    unmapped.forEach((u) => allUnmapped.push(`${rel}:${u.line}  [${u.kind}]  ${u.cls}`))
    if (edits.length) {
      changedFiles++
      totalEdits += edits.length
      if (WRITE) writeFileSync(file, applyEdits(source, edits))
    }
  } catch (err) {
    failures.push(`${rel}: ${err.message}`)
  }
}

console.log(`${WRITE ? 'wrote' : 'would change'} ${changedFiles} files, ${totalEdits} class strings; scanned ${files.length}`)
if (failures.length) {
  console.log(`\n${failures.length} files FAILED to process:`)
  failures.forEach((f) => console.log(`  ${f}`))
}
if (allUnmapped.length) {
  console.log(`\n${allUnmapped.length} unmapped classes (fix by hand):`)
  allUnmapped.forEach((u) => console.log(`  ${u}`))
}
process.exit(failures.length || allUnmapped.length ? 1 : 0)
