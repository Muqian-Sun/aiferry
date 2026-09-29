import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(
  resolve(process.cwd(), 'src/components/account/CreateAccountModal.vue'),
  'utf8'
)
// 接入方式（2026-09-25 起替代平台 / 类型按钮）：成品号选哪家的账号；第三方 key 不选平台
const accessSources = readFileSync(
  resolve(process.cwd(), 'src/components/account/accessSources.ts'),
  'utf8'
)

describe('CreateAccountModal Grok account types', () => {
  it('offers Grok as a subscription account', () => {
    // Grok 只认成品号：指向 xAI 官方地址的 key 按中转处理（muqian 2026-09-29），常用地址里没有 xAI
    expect(accessSources).toContain("id: 'grok', kind: 'subscription', platform: 'grok', category: 'oauth-based'")
  })

  it('offers no custom upstream URL for the OAuth create flow', () => {
    // 成品号只走官方地址；要走中转请按第三方 key 建号。请求头覆写的展示见 CreateAccountModal.spec.ts
    expect(source).not.toContain('data-testid="grok-custom-base-url-toggle"')
    expect(source).not.toContain('data-testid="grok-custom-base-url-input"')
  })

  it('validates and applies upstream config on Grok OAuth create paths', () => {
    // 授权码兑换 / RT 批量 / SSO 批量（密码授权已隐藏）
    expect(source.match(/validateHeaderOverrideForm\(\)/g)?.length).toBeGreaterThanOrEqual(3)
    expect(source.match(/applyGrokOAuthUpstreamConfig\(credentials\)/g)?.length).toBeGreaterThanOrEqual(3)
  })

  it('hides Grok password authorize option in the create flow', () => {
    expect(source).toContain(':show-email-password-option="false"')
  })
})
