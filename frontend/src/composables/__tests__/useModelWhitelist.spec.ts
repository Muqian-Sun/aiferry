import { describe, expect, it } from 'vitest'

import { getModelsByPlatform } from '../useModelWhitelist'

describe('useModelWhitelist', () => {
  it('openai 模型列表包含 GPT-5.4 官方快照', () => {
    const models = getModelsByPlatform('openai')

    expect(models).toContain('gpt-5.4')
    expect(models).toContain('gpt-5.4-mini')
    expect(models).toContain('gpt-5.4-2026-03-05')
    expect(models).toContain('codex-auto-review')
    expect(models).toContain('gpt-5.6')
    expect(models).toContain('gpt-6')
    expect(models).toContain('gpt-6-astra')
  })

  it('openai 模型列表不再暴露已下线的 ChatGPT 登录 Codex 模型', () => {
    const models = getModelsByPlatform('openai')

    expect(models).not.toContain('gpt-5')
    expect(models).not.toContain('gpt-5.1')
    expect(models).not.toContain('gpt-5.1-codex')
    expect(models).not.toContain('gpt-5.1-codex-max')
    expect(models).not.toContain('gpt-5.1-codex-mini')
    expect(models).not.toContain('gpt-5.2-codex')
  })

  it('antigravity 模型列表包含图片模型兼容项', () => {
    const models = getModelsByPlatform('antigravity')

    expect(models).toContain('gemini-2.5-flash-image')
    expect(models).toContain('gemini-3.1-flash-image')
    expect(models).toContain('gemini-3-pro-image')
  })

  it('Claude 模型列表包含新发布的 Claude 模型', () => {
    expect(getModelsByPlatform('claude')).toContain('claude-fable-5-1')
    expect(getModelsByPlatform('antigravity')).toContain('claude-fable-5-1')
    expect(getModelsByPlatform('claude')).toContain('claude-fable-5')
    expect(getModelsByPlatform('antigravity')).toContain('claude-fable-5')
    expect(getModelsByPlatform('claude')).toContain('claude-opus-4-8')
    expect(getModelsByPlatform('antigravity')).toContain('claude-opus-4-8')
  })

  it('xAI 模型列表包含 Grok 4.5 官方模型和别名', () => {
    const models = getModelsByPlatform('grok')

    expect(models).toContain('grok-4.6')
    expect(models).toContain('grok-4.6-latest')
    expect(models).toContain('grok-4.5')
    expect(models).toContain('grok-4.5-latest')
    expect(models).toContain('grok-build-latest')
    expect(models).toContain('grok-imagine-image-2.0')
    expect(models).toContain('grok-imagine-video-1.5')
  })

  it('grok 模型列表包含 Composer 默认项和兼容别名', () => {
    const models = getModelsByPlatform('grok')

    expect(models).toContain('grok-composer-2.5-fast')
    expect(models).not.toContain('grok-composer')
    expect(models).toContain('composer-2.5')
  })

  it('gemini 模型列表包含原生生图模型', () => {
    const models = getModelsByPlatform('gemini')

    expect(models).toContain('gemini-2.5-flash-image')
    expect(models).toContain('gemini-3.1-flash-image')
    expect(models.indexOf('gemini-3.1-flash-image')).toBeLessThan(models.indexOf('gemini-2.0-flash'))
    expect(models.indexOf('gemini-2.5-flash-image')).toBeLessThan(models.indexOf('gemini-2.5-flash'))
  })

  it('antigravity 模型列表会把新的 Gemini 图片模型排在前面', () => {
    const models = getModelsByPlatform('antigravity')

    expect(models.indexOf('gemini-3.1-flash-image')).toBeLessThan(models.indexOf('gemini-2.5-flash'))
    expect(models.indexOf('gemini-2.5-flash-image')).toBeLessThan(models.indexOf('gemini-2.5-flash-lite'))
  })

  it('antigravity 模型列表包含 Gemini 3.1 Pro 通用别名', () => {
    const models = getModelsByPlatform('antigravity')

    expect(models).toContain('gemini-3.1-pro')
  })
})
