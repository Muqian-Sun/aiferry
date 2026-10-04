import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

describe('risk control locale copy', () => {
  it('keeps pre-block audit key summary aware of async worker load', () => {
    expect(zh.admin.riskControl.preBlockAPIKeyLoadSummary).toContain('工作线程：{workerActive} / {workerTotal}')
    expect(en.admin.riskControl.preBlockAPIKeyLoadSummary).toContain('worker: {workerActive} / {workerTotal}')
  })
})
