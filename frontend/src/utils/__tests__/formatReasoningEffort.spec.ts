import { describe, expect, it } from 'vitest'
import {
  formatReasoningEffort,
  formatReasoningEffortMapping,
  reasoningEffortValuesEqual,
} from '@/utils/format'

// 推理强度按界面语言显示（common.reasoningEffort.*）；测试环境不编译文案，t 原样返回 key
describe('formatReasoningEffort', () => {
  it('maps known effort values to their labels', () => {
    expect(formatReasoningEffort('max')).toBe('common.reasoningEffort.max')
    expect(formatReasoningEffort('x-high')).toBe('common.reasoningEffort.xhigh')
    expect(formatReasoningEffort(null)).toBe('-')
  })
})

describe('formatReasoningEffortMapping', () => {
  it('shows a single value when requested and forwarded match', () => {
    expect(formatReasoningEffortMapping('max', 'max')).toBe('common.reasoningEffort.max')
    expect(formatReasoningEffortMapping(null, 'high')).toBe('common.reasoningEffort.high')
  })

  it('shows requested then forwarded when mapping changed the value', () => {
    expect(formatReasoningEffortMapping('max', 'xhigh')).toBe('common.reasoningEffort.max → common.reasoningEffort.xhigh')
    expect(formatReasoningEffortMapping('high', 'medium')).toBe('common.reasoningEffort.high → common.reasoningEffort.medium')
  })
})

describe('reasoningEffortValuesEqual', () => {
  it('treats x-high aliases as equal', () => {
    expect(reasoningEffortValuesEqual('x-high', 'xhigh')).toBe(true)
    expect(reasoningEffortValuesEqual('max', 'xhigh')).toBe(false)
  })
})
