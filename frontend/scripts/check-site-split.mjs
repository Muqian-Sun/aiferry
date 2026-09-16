#!/usr/bin/env node
/**
 * 构建产物守卫：用户站产物不得包含管理端接口或页面路径（/admin/<path>）。
 *
 * 依赖图测试（src/app/__tests__/siteSplit.spec.ts）按源码判断；本脚本按真正下发给浏览器的产物判断，
 * 覆盖依赖图看不到的情况（例如共享模块里写死的管理端路径字符串）。
 * fail-closed：产物目录缺失或没有脚本文件都判失败，避免「没构建 → 没扫到 → 通过」。
 */
import { existsSync, readdirSync, readFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const dist = resolve(dirname(fileURLToPath(import.meta.url)), '../../backend/internal/web/dist')
const failures = []

for (const app of ['user', 'admin']) {
  if (!existsSync(join(dist, app, 'index.html'))) failures.push(`missing ${app}/index.html (build both sites first)`)
}

const assets = join(dist, 'user', 'assets')
const scripts = existsSync(assets) ? readdirSync(assets).filter((name) => name.endsWith('.js')) : []
if (scripts.length === 0) failures.push('no user-site scripts found under dist/user/assets')

const pattern = /\/admin\/[a-z][a-z0-9_-]*/g
for (const name of scripts) {
  const content = readFileSync(join(assets, name), 'utf8')
  for (const match of content.matchAll(pattern)) {
    const context = content.slice(Math.max(0, match.index - 60), match.index + match[0].length + 40).replace(/\s+/g, ' ')
    failures.push(`${name}: ${match[0]} … ${context}`)
  }
}

if (failures.length > 0) {
  console.error('site split check failed: the user-site bundle must not contain admin console code')
  for (const failure of failures) console.error(`  - ${failure}`)
  process.exit(1)
}
console.log(`site split check passed: ${scripts.length} user-site scripts contain no admin paths`)
