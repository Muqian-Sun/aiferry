import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

// 渠道定价已不参与计费（价格从模型目录取）：渠道编辑页只保留只读展示，
// 并挂一条「已不参与计费」提示。ChannelsView 挂载依赖太多接口，这里按源码断言：
// 价卡必须包在 disabled 的 fieldset 里、不再绑定编辑事件，也没有新增入口。
describe('Channel pricing section is read-only', () => {
  const source = readFileSync(resolve('src/views/admin/ChannelsView.vue'), 'utf8')
  const section = source.slice(
    source.indexOf('<!-- Model Pricing'),
    source.indexOf('<!-- Account Stats Pricing Rules')
  )

  it('shows the retired-pricing banner', () => {
    expect(section).toContain('data-testid="channel-pricing-retired-banner"')
    expect(section).toContain("t('admin.channels.form.modelPricingRetiredHint')")
  })

  it('renders pricing cards inside a disabled fieldset without edit handlers', () => {
    expect(section).toMatch(/<fieldset[^>]*\bdisabled\b[^>]*>[\s\S]*<PricingEntryCard[\s\S]*<\/fieldset>/)
    expect(section).not.toContain('@update=')
    expect(section).not.toContain('@remove=')
    expect(section).not.toContain('addPricingEntry')
    expect(section).not.toContain('syncLatestModels')
  })

  it('has the banner text in both locales', () => {
    for (const locale of ['zh', 'en']) {
      const messages = readFileSync(resolve(`src/i18n/locales/${locale}/admin/channels.ts`), 'utf8')
      expect(messages).toContain('modelPricingRetiredHint:')
    }
  })
})
