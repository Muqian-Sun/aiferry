/**
 * 模板 token 守卫的共用部分：禁用模式、扫描面提取、违规收集。
 * 用户站守卫（components/user/shell/__tests__/userTemplateTokens.spec.ts）与管理站壳守卫
 * （components/layout/__tests__/adminShellTokens.spec.ts）各自决定扫描集与白名单。
 */
import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs'
import { join, relative, resolve } from 'node:path'

export interface ForbiddenRule {
  name: string
  re: RegExp
}

/** 禁用模式与含义（只扫 class 属性、:class 表达式与 <style> 里的 @apply）。 */
export const FORBIDDEN: ForbiddenRule[] = [
  { name: 'legacy gray/slate palette', re: /\b(?:bg|text|border|divide|ring|from|to|via|placeholder|accent|fill|stroke|outline)-(?:gray|slate|zinc|neutral|stone)-\d{2,3}\b/ },
  { name: 'legacy dark palette', re: /\b(?:bg|text|border|divide|ring|from|to|via|placeholder)-dark-\d{2,3}\b/ },
  { name: 'dark: variant (tokens switch themselves)', re: /(?:^|[\s"'`(:])dark:[a-z]/ },
  // 唯一获准的渐变是 style.css 的 .text-flow（首屏标题流动渐变，muqian 2026-09-22），模板里不写渐变工具类
  { name: 'gradient', re: /\b(?:bg-gradient-to-\w+|bg-mesh-gradient|text-gradient|gradient-primary|gradient-dark)\b/ },
  { name: 'glass / glow shadows', re: /\b(?:glass(?:-card)?|shadow-glow(?:-lg)?|shadow-glass(?:-sm)?|shadow-card(?:-hover)?)\b/ },
  { name: 'all-caps label', re: /\buppercase\b[^"'`]*\btracking-/ }
]

/**
 * 2026-09-22 muqian 定「学参考站的效果，允许用卡片」：card / rounded-2xl / hover 上浮不再禁，
 * 两站规则现在相同；保留两个名字给各自的 spec 用。
 */
export const FORBIDDEN_ADMIN: ForbiddenRule[] = FORBIDDEN

function walk(dir: string, out: string[]): void {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name)
    if (name === '__tests__') continue
    const st = statSync(full)
    if (st.isDirectory()) walk(full, out)
    else if (name.endsWith('.vue')) out.push(full)
  }
}

/** 收集扫描集（目录递归 + 单文件），相对 src 的路径，排除前缀，去重排序；缺目录 / 缺文件直接抛错。 */
export function collectFiles(src: string, dirs: string[], files: string[], excludePrefixes: string[] = []): string[] {
  const found: string[] = []
  for (const dir of dirs) {
    const full = resolve(src, dir)
    if (!existsSync(full)) throw new Error(`scan dir missing: ${dir}`)
    walk(full, found)
  }
  for (const file of files) {
    const full = resolve(src, file)
    if (!existsSync(full)) throw new Error(`scan file missing: ${file}`)
    found.push(full)
  }
  const rel = found.map((f) => relative(src, f))
  return [...new Set(rel)].filter((f) => !excludePrefixes.some((p) => f.startsWith(p))).sort()
}

/** 只取 class 属性、:class 绑定表达式与 <style> 块里的 @apply，避免误伤脚本字符串或 SVG 颜色。 */
export function styleSurfaces(source: string): string[] {
  const surfaces: string[] = []
  const template = source.match(/<template\b[^>]*>([\s\S]*)<\/template>/)?.[1] ?? ''
  for (const m of template.matchAll(/(?:^|\s)(?::class|class|v-bind:class)\s*=\s*("([^"]*)"|'([^']*)')/g)) {
    surfaces.push(m[2] ?? m[3] ?? '')
  }
  for (const m of source.matchAll(/<style\b[^>]*>([\s\S]*?)<\/style>/g)) {
    for (const apply of m[1].matchAll(/@apply\s+([^;]+);/g)) surfaces.push(apply[1])
  }
  return surfaces
}

export function violationsOf(src: string, file: string, rules: ForbiddenRule[]): string[] {
  const source = readFileSync(resolve(src, file), 'utf8')
  const hits: string[] = []
  for (const surface of styleSurfaces(source)) {
    for (const rule of rules) {
      const m = surface.match(rule.re)
      if (m) hits.push(`${rule.name}: …${m[0]}…`)
    }
  }
  return hits
}
