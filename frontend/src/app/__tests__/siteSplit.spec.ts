/**
 * 用户站与管理后台分离的依赖守卫：从用户站入口出发，沿 import 依赖图不得触达任何管理端模块。
 *
 * 依赖图用 TypeScript AST 构建（不是正则）：覆盖静态 import、export ... from、动态 import()；
 * 跳过纯类型导入（构建时被擦除，不进入产物）。解析不到的相对 / 别名导入直接判失败，
 * 避免「解析失败 → 静默少一条边 → 守卫更绿」。
 */
import { existsSync, readFileSync, statSync } from 'node:fs'
import { dirname, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'
import { describe, expect, it } from 'vitest'

const SRC = resolve(dirname(fileURLToPath(import.meta.url)), '../..')

const ADMIN_CODE = [
  /^views\/admin\//,
  /^components\/admin\//,
  /^api\/admin\//,
  /^apps\/admin\//,
  /^features\/prompt-audit\//,
  /^stores\/admin[A-Z]\w*\.ts$/,
]

function scriptOf(file: string): string {
  const text = readFileSync(file, 'utf8')
  if (!file.endsWith('.vue')) return text
  return [...text.matchAll(/<script\b[^>]*>([\s\S]*?)<\/script>/g)].map((m) => m[1]).join('\n')
}

function resolveImport(from: string, spec: string): string | null {
  let base: string
  if (spec.startsWith('@/')) base = resolve(SRC, spec.slice(2))
  else if (spec.startsWith('.')) base = resolve(dirname(from), spec)
  else return null // 第三方包
  for (const candidate of [base, `${base}.ts`, `${base}.vue`, resolve(base, 'index.ts')]) {
    if (existsSync(candidate) && statSync(candidate).isFile()) return candidate
  }
  // 静态资源（样式、图片、markdown）不参与判断
  if (/\.(css|scss|svg|png|jpe?g|gif|webp|json|md)(\?raw)?$/.test(spec)) return null
  throw new Error(`unresolved import ${spec} in ${relative(SRC, from)}`)
}

function importsOf(file: string): string[] {
  const source = ts.createSourceFile(file, scriptOf(file), ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)
  const out: string[] = []
  const add = (spec: string) => {
    const target = resolveImport(file, spec)
    if (target) out.push(target)
  }
  const visit = (node: ts.Node) => {
    if (ts.isImportDeclaration(node) && ts.isStringLiteral(node.moduleSpecifier)) {
      const clause = node.importClause
      const typeOnly =
        clause?.isTypeOnly ||
        (!!clause && !clause.name && !!clause.namedBindings && ts.isNamedImports(clause.namedBindings) &&
          clause.namedBindings.elements.length > 0 && clause.namedBindings.elements.every((e) => e.isTypeOnly))
      if (!typeOnly) add(node.moduleSpecifier.text)
    } else if (ts.isExportDeclaration(node) && node.moduleSpecifier && ts.isStringLiteral(node.moduleSpecifier)) {
      if (!node.isTypeOnly) add(node.moduleSpecifier.text)
    } else if (
      ts.isCallExpression(node) &&
      node.expression.kind === ts.SyntaxKind.ImportKeyword &&
      node.arguments[0] &&
      ts.isStringLiteral(node.arguments[0])
    ) {
      add(node.arguments[0].text)
    }
    ts.forEachChild(node, visit)
  }
  visit(source)
  return out
}

function reachableFrom(entry: string): Map<string, string | null> {
  const parent = new Map<string, string | null>([[entry, null]])
  const stack = [entry]
  while (stack.length) {
    const file = stack.pop() as string
    for (const next of importsOf(file)) {
      if (!parent.has(next)) {
        parent.set(next, file)
        stack.push(next)
      }
    }
  }
  return parent
}

function chain(parent: Map<string, string | null>, file: string): string {
  const out: string[] = []
  for (let cur: string | null | undefined = file; cur; cur = parent.get(cur)) out.push(relative(SRC, cur))
  return out.join(' <- ')
}

describe('user site does not bundle admin console code', () => {
  it('reaches no admin module from the user entry', () => {
    const parent = reachableFrom(resolve(SRC, 'apps/user/main.ts'))
    // 守卫自检：入口确实展开了（避免入口路径写错而空转）
    expect(parent.size).toBeGreaterThan(100)
    const leaked = [...parent.keys()]
      .filter((file) => ADMIN_CODE.some((pattern) => pattern.test(relative(SRC, file))))
      .map((file) => chain(parent, file))
    expect(leaked).toEqual([])
  })

  it('does reach admin modules from the admin entry (the classifier is not vacuous)', () => {
    const parent = reachableFrom(resolve(SRC, 'apps/admin/main.ts'))
    const admin = [...parent.keys()].filter((file) => ADMIN_CODE.some((pattern) => pattern.test(relative(SRC, file))))
    expect(admin.length).toBeGreaterThan(100)
  })
})
